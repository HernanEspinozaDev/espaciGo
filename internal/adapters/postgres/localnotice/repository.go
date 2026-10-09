package localnotice

import (
	"context"
	"errors"
	"time"

	domain "github.com/HernanEspinozaDev/espaciGo/internal/localnotice"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) Claim(ctx context.Context, at time.Time) (domain.Notice, bool, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.Notice{}, false, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var id, recipient string
	e = tx.QueryRow(ctx, `SELECT id::text,destinatario_id::text FROM public.aviso_local WHERE (estado='pendiente' AND proximo_intento_en<=$1) OR (estado='procesando' AND lease_hasta<=$1) ORDER BY proximo_intento_en,id LIMIT 1`, at).Scan(&id, &recipient)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Notice{}, false, nil
	}
	if e != nil {
		return domain.Notice{}, false, e
	}
	var userState string
	var email pgtype.Text
	e = tx.QueryRow(ctx, `SELECT estado,correo_original FROM public.usuario WHERE id=$1 FOR UPDATE`, recipient).Scan(&userState, &email)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Notice{}, false, tx.Commit(ctx)
	}
	if e != nil {
		return domain.Notice{}, false, e
	}
	var n domain.Notice
	var state string
	var lease *time.Time
	e = tx.QueryRow(ctx, `SELECT id::text,tipo_evento,agregado_tipo,agregado_id::text,destinatario_id::text,estado,ciclo,intentos_ciclo,lease_hasta FROM public.aviso_local WHERE id=$1 FOR UPDATE`, id).Scan(&n.ID, &n.Type, &n.AggregateType, &n.AggregateID, &n.RecipientID, &state, &n.Cycle, &n.Attempt, &lease)
	if e != nil {
		return domain.Notice{}, false, e
	}
	if (state != "pendiente" && state != "procesando") || (state == "pendiente" && n.Attempt > 8) || (state == "procesando" && lease != nil && lease.After(at)) {
		return domain.Notice{}, false, tx.Commit(ctx)
	}
	if userState != "activo" || !email.Valid {
		_, e = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='cancelada',cancelada_en=$2,retirar_en=$2::timestamptz+interval '30 days',lease_hasta=NULL,codigo_error='destinatario_no_activo' WHERE id=$1`, id, at)
		if e != nil {
			return domain.Notice{}, false, e
		}
		_, e = tx.Exec(ctx, `UPDATE public.aviso_local_ciclo SET estado='cancelada',finalizada_en=$3,codigo_resultado='destinatario_no_activo' WHERE aviso_id=$1 AND ciclo=$2 AND estado='pendiente'`, id, n.Cycle, at)
		if e != nil {
			return domain.Notice{}, false, e
		}
		return domain.Notice{}, false, tx.Commit(ctx)
	}
	n.Email = email.String
	n.Attempt++
	_, e = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='procesando',intentos_total=intentos_total+1,intentos_ciclo=intentos_ciclo+1,lease_hasta=$2::timestamptz+interval '30 seconds' WHERE id=$1`, id, at)
	if e != nil {
		return domain.Notice{}, false, e
	}
	_, e = tx.Exec(ctx, `UPDATE public.aviso_local_ciclo SET intentos=intentos+1 WHERE aviso_id=$1 AND ciclo=$2 AND estado='pendiente'`, id, n.Cycle)
	if e != nil {
		return domain.Notice{}, false, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.Notice{}, false, e
	}
	return n, true, nil
}
func (r *Repository) Finish(ctx context.Context, n domain.Notice, success bool, code string, at time.Time) error {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var state string
	var cycle, attempt int
	e = tx.QueryRow(ctx, `SELECT estado,ciclo,intentos_ciclo FROM public.aviso_local WHERE id=$1 FOR UPDATE`, n.ID).Scan(&state, &cycle, &attempt)
	if errors.Is(e, pgx.ErrNoRows) {
		return nil
	}
	if e != nil {
		return e
	}
	if state != "procesando" || cycle != n.Cycle || attempt != n.Attempt {
		return domain.ErrInvalid
	}
	if success {
		_, e = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='entregada',entregada_en=$2,retirar_en=$2::timestamptz+interval '30 days',lease_hasta=NULL,codigo_error=NULL WHERE id=$1`, n.ID, at)
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE public.aviso_local_ciclo SET estado='entregada',finalizada_en=$3,codigo_resultado='entregada' WHERE aviso_id=$1 AND ciclo=$2`, n.ID, n.Cycle, at)
		}
	} else if attempt >= 8 {
		_, e = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='fallo_terminal',fallo_terminal_en=$2,retirar_en=$2::timestamptz+interval '30 days',lease_hasta=NULL,codigo_error=$3 WHERE id=$1`, n.ID, at, code)
		if e == nil {
			_, e = tx.Exec(ctx, `UPDATE public.aviso_local_ciclo SET estado='fallo_terminal',finalizada_en=$3,codigo_resultado=$4 WHERE aviso_id=$1 AND ciclo=$2`, n.ID, n.Cycle, at, code)
		}
	} else {
		delay := time.Second << uint(min(attempt-1, 12))
		_, e = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='pendiente',proximo_intento_en=$2,lease_hasta=NULL,codigo_error=$3 WHERE id=$1`, n.ID, at.Add(delay), code)
	}
	if e != nil {
		return e
	}
	return tx.Commit(ctx)
}
func (r *Repository) ListTerminal(ctx context.Context, limit int) ([]domain.TerminalNotice, error) {
	rows, e := r.pool.Query(ctx, `SELECT id::text,tipo_evento,estado,COALESCE(codigo_error,''),ciclo,intentos_total,creada_en,COALESCE(entregada_en,fallo_terminal_en,cancelada_en) FROM public.aviso_local WHERE estado='fallo_terminal' ORDER BY fallo_terminal_en,id LIMIT $1`, limit)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.TerminalNotice{}
	for rows.Next() {
		var v domain.TerminalNotice
		if e = rows.Scan(&v.ID, &v.Type, &v.State, &v.Code, &v.Cycle, &v.Attempts, &v.CreatedAt, &v.TerminalAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) Reopen(ctx context.Context, id, admin, reason, key, correlation string, at time.Time) (domain.Recovery, error) {
	tx, e := r.pool.Begin(ctx)
	if e != nil {
		return domain.Recovery{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var recipient string
	e = tx.QueryRow(ctx, `SELECT destinatario_id::text FROM public.aviso_local WHERE id=$1`, id).Scan(&recipient)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Recovery{}, domain.ErrInvalid
	}
	if e != nil {
		return domain.Recovery{}, e
	}
	rows, e := tx.Query(ctx, `SELECT id FROM public.usuario WHERE id=ANY($1::uuid[]) ORDER BY id FOR UPDATE`, []string{admin, recipient})
	if e != nil {
		return domain.Recovery{}, e
	}
	for rows.Next() {
		var discard string
		if e = rows.Scan(&discard); e != nil {
			rows.Close()
			return domain.Recovery{}, e
		}
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return domain.Recovery{}, e
	}
	rows.Close()
	var allowed, active bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.usuario u JOIN public.rol_usuario x ON x.usuario_id=u.id WHERE u.id=$1 AND u.estado='activo' AND x.rol='administrador'),EXISTS(SELECT 1 FROM public.usuario WHERE id=$2 AND estado='activo')`, admin, recipient).Scan(&allowed, &active); e != nil {
		return domain.Recovery{}, e
	}
	if !allowed || !active {
		return domain.Recovery{}, domain.ErrInvalid
	}
	var existing int
	e = tx.QueryRow(ctx, `SELECT a.ciclo FROM public.aviso_local_recuperacion_auditoria x JOIN public.aviso_local a ON a.id=x.aviso_id WHERE x.aviso_id=$1 AND x.clave_idempotencia=$2`, id, key).Scan(&existing)
	if e == nil {
		return domain.Recovery{NoticeID: id, Cycle: existing, Reused: true}, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return domain.Recovery{}, e
	}
	var state string
	var cycle int
	e = tx.QueryRow(ctx, `SELECT estado,ciclo FROM public.aviso_local WHERE id=$1 FOR UPDATE`, id).Scan(&state, &cycle)
	if e != nil {
		return domain.Recovery{}, e
	}
	if state != "fallo_terminal" {
		return domain.Recovery{}, domain.ErrInvalid
	}
	cycle++
	_, e = tx.Exec(ctx, `UPDATE public.aviso_local SET estado='pendiente',ciclo=$2,intentos_ciclo=0,proximo_intento_en=$3,lease_hasta=NULL,fallo_terminal_en=NULL,codigo_error=NULL,retirar_en=NULL WHERE id=$1`, id, cycle, at)
	if e != nil {
		return domain.Recovery{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.aviso_local_ciclo(aviso_id,ciclo,estado,iniciada_en,actor_reapertura_id,motivo_reapertura_codigo,correlacion_id,clave_idempotencia) VALUES($1,$2,'pendiente',$3,$4,$5,$6,$7)`, id, cycle, at, admin, reason, correlation, key)
	if e != nil {
		return domain.Recovery{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.aviso_local_recuperacion_auditoria(id,aviso_id,actor_id,motivo_codigo,ocurrida_en,correlacion_id,clave_idempotencia,retirar_en) VALUES(gen_random_uuid(),$1,$2,$3,$4,$5,$6,$4::timestamptz+interval '5 years')`, id, admin, reason, at, correlation, key)
	if e != nil {
		return domain.Recovery{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.Recovery{}, e
	}
	return domain.Recovery{NoticeID: id, Cycle: cycle}, nil
}
func (r *Repository) Purge(ctx context.Context, at time.Time, limit int) (int64, error) {
	var reviews, reports, notices int64
	e := r.pool.QueryRow(ctx, `SELECT reviews_purged,reports_purged,notices_purged FROM public.purge_expired_local_communication($1,$2)`, at, limit).Scan(&reviews, &reports, &notices)
	return reports + reviews + notices, e
}
