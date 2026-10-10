package adminlocal

import (
	"context"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const SafetyNotice = "ENSAYO LOCAL — SIN MOVIMIENTOS REALES"

var (
	ErrInvalid   = errors.New("adminlocal: invalid input")
	ErrForbidden = errors.New("adminlocal: forbidden")
	ErrNotFound  = errors.New("adminlocal: not found")
	ErrConflict  = errors.New("adminlocal: conflict")
)

type Service struct {
	pool *pgxpool.Pool
	now  func() time.Time
	ids  credentials.Generator
}

func New(pool *pgxpool.Pool, now func() time.Time) (*Service, error) {
	if pool == nil || now == nil {
		return nil, ErrInvalid
	}
	return &Service{pool: pool, now: now, ids: credentials.Generator{}}, nil
}

type BlockResult struct {
	AccountID string    `json:"account_id"`
	Blocked   bool      `json:"blocked"`
	ChangedAt time.Time `json:"changed_at"`
}

var blockReasons = map[string]bool{"riesgo_seguridad": true, "uso_indebido": true, "revision_administrativa": true}
var unblockReasons = map[string]bool{"revision_concluida": true, "error_bloqueo": true, "riesgo_resuelto": true}

func (s *Service) BlockAccount(ctx context.Context, actor, accountID, reason, correlation string) (BlockResult, error) {
	if actor == "" || accountID == "" || correlation == "" || len(correlation) > 120 || !blockReasons[reason] {
		return BlockResult{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return BlockResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor, accountID); err != nil {
		return BlockResult{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return BlockResult{}, err
	}
	var state string
	if err = tx.QueryRow(ctx, `SELECT estado FROM public.usuario WHERE id=$1`, accountID).Scan(&state); err != nil {
		return BlockResult{}, ErrNotFound
	}
	if state != "activo" {
		return BlockResult{}, ErrConflict
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, accountID).Scan(&exists); err != nil {
		return BlockResult{}, err
	}
	if exists {
		return BlockResult{}, ErrConflict
	}
	at := s.now().UTC() // read after both account locks
	id, err := s.ids.ID()
	if err != nil {
		return BlockResult{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.bloqueo_cuenta_administrativo_local(cuenta_id,bloqueada_por,motivo_codigo,bloqueada_en,correlacion_id) VALUES($1,$2,$3,$4,$5)`, accountID, actor, reason, at, correlation); err != nil {
		return BlockResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.sesion SET revocada_en=$2 WHERE usuario_id=$1 AND revocada_en IS NULL`, accountID, at); err != nil {
		return BlockResult{}, err
	}
	if _, err = tx.Exec(ctx, `UPDATE public.token_accion SET invalidado_en=$2 WHERE usuario_id=$1 AND invalidado_en IS NULL AND consumido_en IS NULL`, accountID, at); err != nil {
		return BlockResult{}, err
	}
	if err = writeBlockHistory(ctx, tx, id, accountID, actor, "bloquear", reason, correlation, at); err != nil {
		return BlockResult{}, err
	}
	if err = writeAudit(ctx, tx, s.ids, actor, "cuenta", accountID, "admin.account.block", "exito", reason, correlation, at); err != nil {
		return BlockResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return BlockResult{}, err
	}
	return BlockResult{AccountID: accountID, Blocked: true, ChangedAt: at}, nil
}

func (s *Service) UnblockAccount(ctx context.Context, actor, accountID, reason, correlation string) (BlockResult, error) {
	if actor == "" || accountID == "" || correlation == "" || len(correlation) > 120 || !unblockReasons[reason] {
		return BlockResult{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return BlockResult{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor, accountID); err != nil {
		return BlockResult{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return BlockResult{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1)`, accountID).Scan(&exists); err != nil {
		return BlockResult{}, err
	}
	if !exists {
		return BlockResult{}, ErrConflict
	}
	at := s.now().UTC()
	// A restricted login is also revoked on unblock. The account must establish
	// a fresh normal session; revoked normal sessions are never restored.
	if _, err = tx.Exec(ctx, `UPDATE public.sesion SET revocada_en=$2 WHERE usuario_id=$1 AND revocada_en IS NULL`, accountID, at); err != nil {
		return BlockResult{}, err
	}
	id, err := s.ids.ID()
	if err != nil {
		return BlockResult{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1`, accountID); err != nil {
		return BlockResult{}, err
	}
	if err = writeBlockHistory(ctx, tx, id, accountID, actor, "desbloquear", reason, correlation, at); err != nil {
		return BlockResult{}, err
	}
	if err = writeAudit(ctx, tx, s.ids, actor, "cuenta", accountID, "admin.account.unblock", "exito", reason, correlation, at); err != nil {
		return BlockResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return BlockResult{}, err
	}
	return BlockResult{AccountID: accountID, Blocked: false, ChangedAt: at}, nil
}

func lockAccounts(ctx context.Context, tx pgx.Tx, ids ...string) error {
	// One ordered query acquires the same usuario row locks as booking creation
	// and KYC/suppression, preventing a block/new-reservation race.
	if len(ids) == 1 {
		ids = append(ids, ids[0])
	}
	if len(ids) != 2 {
		return ErrInvalid
	}
	rows, err := tx.Query(ctx, `SELECT id::text FROM public.usuario WHERE id=$1 OR id=$2 ORDER BY id FOR UPDATE`, ids[0], ids[1])
	if err != nil {
		return err
	}
	defer rows.Close()
	found := 0
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		found++
	}
	if err = rows.Err(); err != nil {
		return err
	}
	if found != len(unique(ids)) {
		return ErrNotFound
	}
	return nil
}
func unique(ids []string) []string {
	out := []string{}
	for _, id := range ids {
		seen := false
		for _, old := range out {
			if old == id {
				seen = true
			}
		}
		if !seen {
			out = append(out, id)
		}
	}
	return out
}
func requireAdmin(ctx context.Context, tx pgx.Tx, id string) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.usuario u JOIN public.rol_usuario r ON r.usuario_id=u.id WHERE u.id=$1 AND u.estado='activo' AND r.rol='administrador' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id))`, id).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrForbidden
	}
	return nil
}
func writeBlockHistory(ctx context.Context, tx pgx.Tx, id, account, actor, action, reason, correlation string, at time.Time) error {
	_, err := tx.Exec(ctx, `INSERT INTO public.bloqueo_cuenta_historial_local(id,cuenta_id,actor_id,accion,motivo_codigo,ocurrida_en,correlacion_id) VALUES($1,$2,$3,$4,$5,$6,$7)`, id, account, actor, action, reason, at, correlation)
	return err
}
func writeAudit(ctx context.Context, tx pgx.Tx, ids credentials.Generator, actor, resourceType, resourceID, action, result, reason, correlation string, at time.Time) error {
	id, err := ids.ID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::timestamptz,$9::timestamptz+interval '5 years')`, id, actor, resourceType, resourceID, action, result, reason, correlation, at)
	return err
}

type Period struct {
	From     time.Time `json:"from"`
	Until    time.Time `json:"until"`
	TimeZone string    `json:"timezone"`
}

func (p Period) normalizedUTC() Period {
	p.From = p.From.UTC()
	p.Until = p.Until.UTC()
	return p
}

func (p Period) Validate() error {
	if p.TimeZone == "" || p.From.IsZero() || p.Until.IsZero() || !p.Until.After(p.From) {
		return ErrInvalid
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return ErrInvalid
	}
	from, until := p.From.In(loc), p.Until.In(loc)
	if until.After(from.AddDate(0, 0, 31)) {
		return ErrInvalid
	}
	return nil
}

type ReservationStateCount struct {
	State string `json:"state"`
	Count int64  `json:"count"`
}
type FinancialLine struct {
	Kind      string `json:"kind"`
	State     string `json:"state"`
	Count     int64  `json:"count"`
	AmountCLP int64  `json:"amount_clp"`
}
type Report struct {
	GeneratedAt time.Time `json:"generated_at"`
	Period      Period    `json:"period"`
	Currency    string    `json:"currency"`
	Notice      string    `json:"notice"`
}
type ReservationsReport struct {
	Report
	Items []ReservationStateCount `json:"items"`
}
type FinanceReport struct {
	Report
	Confirmed []FinancialLine `json:"confirmed_movements"`
	Pending   []FinancialLine `json:"pending_obligations"`
}

func (s *Service) ReservationReport(ctx context.Context, actor, correlation string, p Period) (ReservationsReport, error) {
	if actor == "" || correlation == "" || len(correlation) > 120 || p.Validate() != nil {
		return ReservationsReport{}, ErrInvalid
	}
	p = p.normalizedUTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ReservationsReport{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor); err != nil {
		return ReservationsReport{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return ReservationsReport{}, err
	}
	at := s.now().UTC()
	if err = writeAudit(ctx, tx, s.ids, actor, "coleccion_reservas_ensayo_local", "00000000-0000-4000-8000-000000000000", "admin.reports.reservations", "exito", "consulta_solo_lectura", correlation, at); err != nil {
		return ReservationsReport{}, err
	}
	rows, err := tx.Query(ctx, `SELECT estado,count(*) FROM public.reserva_ensayo_local WHERE creada_en >= $1 AND creada_en < $2 GROUP BY estado ORDER BY estado`, p.From.UTC(), p.Until.UTC())
	if err != nil {
		return ReservationsReport{}, err
	}
	defer rows.Close()
	items := []ReservationStateCount{}
	for rows.Next() {
		var x ReservationStateCount
		if err = rows.Scan(&x.State, &x.Count); err != nil {
			return ReservationsReport{}, err
		}
		items = append(items, x)
	}
	if err = rows.Err(); err != nil {
		return ReservationsReport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return ReservationsReport{}, err
	}
	return ReservationsReport{Report: Report{GeneratedAt: at, Period: p, Currency: "CLP", Notice: SafetyNotice}, Items: items}, nil
}

func (s *Service) FinanceReport(ctx context.Context, actor, correlation string, p Period) (FinanceReport, error) {
	if actor == "" || correlation == "" || len(correlation) > 120 || p.Validate() != nil {
		return FinanceReport{}, ErrInvalid
	}
	p = p.normalizedUTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FinanceReport{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor); err != nil {
		return FinanceReport{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return FinanceReport{}, err
	}
	at := s.now().UTC()
	if err = writeAudit(ctx, tx, s.ids, actor, "coleccion_finanzas_ensayo_local", "00000000-0000-4000-8000-000000000000", "admin.reports.finance", "exito", "consulta_solo_lectura", correlation, at); err != nil {
		return FinanceReport{}, err
	}
	const query = `WITH lines AS (
 SELECT 'cobro_arriendo'::text kind,'confirmado'::text state,count(*)::bigint n,COALESCE(sum(importe_clp),0)::bigint amount FROM public.reserva_pago_ensayo WHERE resultado='exito_simulado' AND creada_en >= $1 AND creada_en < $2
 UNION ALL SELECT 'devolucion','confirmado',count(*)::bigint,COALESCE(sum(importe_clp),0)::bigint FROM public.reserva_devolucion_ensayo WHERE estado='completada' AND ultimo_resultado='exito_simulado' AND completada_en >= $1 AND completada_en < $2
 UNION ALL SELECT CASE tipo WHEN 'autorizacion' THEN 'garantia_autorizada' WHEN 'captura' THEN 'garantia_capturada' ELSE 'garantia_liberada' END,'confirmado',count(*)::bigint,COALESCE(sum(importe_clp),0)::bigint FROM public.reserva_garantia_operacion_ensayo_local WHERE estado='confirmada' AND completada_en >= $1 AND completada_en < $2 GROUP BY tipo
 UNION ALL SELECT 'cobro_arriendo',o.estado,count(*)::bigint,COALESCE(sum(r.subtotal_clp),0)::bigint FROM public.reserva_pago_ensayo_operacion o JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id WHERE o.estado='pendiente' GROUP BY o.estado
 UNION ALL SELECT kind,state,count(*)::bigint,COALESCE(sum(subtotal_clp),0)::bigint FROM (
   SELECT DISTINCT o.id,r.subtotal_clp,'cobro_arriendo'::text kind,'pendiente_conciliacion'::text state
   FROM public.reserva_pago_ensayo_operacion o
   JOIN public.reserva_ensayo_local r ON r.id=o.reserva_id
   WHERE o.estado='vencida' AND (
     EXISTS (SELECT 1 FROM public.reserva_pago_fake_resultado_ensayo f WHERE f.operacion_id=o.id AND f.estado='resultado' AND NOT EXISTS (
       SELECT 1 FROM public.reserva_pago_evento_ensayo e JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE e.operacion_id=o.id AND a.estado IN ('aplicada','ignorada','vencida')
     ))
     OR EXISTS (SELECT 1 FROM public.reserva_pago_evento_ensayo e LEFT JOIN public.reserva_pago_evento_aplicacion_ensayo a ON a.evento_id=e.id WHERE e.operacion_id=o.id AND (a.estado IS NULL OR a.estado IN ('pendiente','pendiente_conciliacion')))
   )
 ) unresolved GROUP BY kind,state
 UNION ALL SELECT 'devolucion',estado,count(*)::bigint,COALESCE(sum(importe_clp),0)::bigint FROM public.reserva_devolucion_ensayo WHERE estado='pendiente' GROUP BY estado
 UNION ALL SELECT CASE tipo WHEN 'autorizacion' THEN 'garantia_autorizada' WHEN 'captura' THEN 'garantia_capturada' ELSE 'garantia_liberada' END,estado,count(*)::bigint,COALESCE(sum(importe_clp),0)::bigint FROM public.reserva_garantia_operacion_ensayo_local WHERE estado IN ('pendiente','por_conciliar') GROUP BY tipo,estado
) SELECT kind,state,n,amount FROM lines WHERE n>0 ORDER BY state,kind`
	rows, err := tx.Query(ctx, query, p.From.UTC(), p.Until.UTC())
	if err != nil {
		return FinanceReport{}, err
	}
	defer rows.Close()
	confirmed, pending := []FinancialLine{}, []FinancialLine{}
	for rows.Next() {
		var x FinancialLine
		if err = rows.Scan(&x.Kind, &x.State, &x.Count, &x.AmountCLP); err != nil {
			return FinanceReport{}, err
		}
		if x.State == "confirmado" {
			confirmed = append(confirmed, x)
		} else {
			pending = append(pending, x)
		}
	}
	if err = rows.Err(); err != nil {
		return FinanceReport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return FinanceReport{}, err
	}
	return FinanceReport{Report: Report{GeneratedAt: at, Period: p, Currency: "CLP", Notice: SafetyNotice}, Confirmed: confirmed, Pending: pending}, nil
}
