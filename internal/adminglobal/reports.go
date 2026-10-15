package adminglobal

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// ReportPeriod uses an explicit IANA zone for the calendar window. Instants
// are normalized to UTC after validating that the interval is at most 31 days.
type ReportPeriod struct {
	From     time.Time `json:"from"`
	Until    time.Time `json:"until"`
	TimeZone string    `json:"time_zone"`
}

func (p ReportPeriod) Validate() error {
	if _, err := time.LoadLocation(p.TimeZone); err != nil || p.From.IsZero() || p.Until.IsZero() || !p.From.Before(p.Until) || p.Until.Sub(p.From) > 31*24*time.Hour {
		return ErrInvalid
	}
	return nil
}

type KPIValue struct {
	Key         string `json:"key,omitempty"`
	Category    string `json:"category,omitempty"`
	State       string `json:"state,omitempty"`
	Source      string `json:"source"`
	Definition  string `json:"definition"`
	Value       int64  `json:"value"`
	Available   bool   `json:"available"`
	Reason      string `json:"reason,omitempty"`
	Denominator int64  `json:"denominator,omitempty"`
}
type KPIReport struct {
	GeneratedAt                  time.Time        `json:"generated_at"`
	Period                       ReportPeriod     `json:"period"`
	Accounts                     map[string]int64 `json:"accounts"`
	AccountRoles                 map[string]int64 `json:"account_roles"`
	Registrations                int64            `json:"registrations_in_period"`
	PublicationOwnerDistribution []KPIValue       `json:"publication_owner_distribution"`
	Publications                 []KPIValue       `json:"publications"`
	Reservations                 []KPIValue       `json:"reservations"`
	Operations                   []KPIValue       `json:"operations"`
	Finance                      []FinanceKPI     `json:"finance"`
	Queues                       []KPIValue       `json:"queues"`
}
type FinanceKPI struct {
	Kind       string `json:"kind"`
	State      string `json:"state"`
	Count      int64  `json:"count"`
	AmountCLP  string `json:"amount_clp"`
	Source     string `json:"source"`
	Definition string `json:"definition"`
}

func (s *Service) KPIs(ctx context.Context, actor, correlation string, p ReportPeriod) (KPIReport, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(correlation) || p.Validate() != nil {
		return KPIReport{}, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadWrite})
	if err != nil {
		return KPIReport{}, err
	}
	defer tx.Rollback(context.Background())
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return KPIReport{}, err
	}
	p.From = p.From.UTC()
	p.Until = p.Until.UTC()
	out := KPIReport{GeneratedAt: s.now().UTC(), Period: p, Accounts: map[string]int64{}, AccountRoles: map[string]int64{}, PublicationOwnerDistribution: []KPIValue{}, Publications: []KPIValue{}, Reservations: []KPIValue{}, Operations: []KPIValue{}, Finance: []FinanceKPI{}, Queues: []KPIValue{}}
	var cohortCount int64
	rows, err := tx.Query(ctx, `SELECT estado,count(*) FROM usuario GROUP BY estado ORDER BY estado`)
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var state string
		var n int64
		if err = rows.Scan(&state, &n); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		out.Accounts[state] = n
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM reserva_ensayo_local WHERE creada_en >= $1 AND creada_en < $2`, p.From, p.Until).Scan(&cohortCount); err != nil {
		return KPIReport{}, err
	}
	rows, err = tx.Query(ctx, `SELECT role,count(*) FROM (SELECT rol::text role,usuario_id FROM rol_usuario UNION ALL SELECT 'administrador_global',usuario_id FROM administrador_global) r GROUP BY role ORDER BY role`)
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var role string
		var n int64
		if err = rows.Scan(&role, &n); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		out.AccountRoles[role] = n
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM usuario WHERE creado_en >= $1 AND creado_en < $2`, p.From, p.Until).Scan(&out.Registrations); err != nil {
		return KPIReport{}, err
	}
	// Publication visibility intentionally mirrors the public catalog policy.
	rows, err = tx.Query(ctx, fmt.Sprintf(`SELECT e.categoria_codigo,e.estado,count(*),count(*) FILTER (WHERE %s) FROM espacio e JOIN categoria_espacio c ON c.codigo=e.categoria_codigo JOIN usuario u ON u.id=e.propietario_id GROUP BY e.categoria_codigo,e.estado ORDER BY e.categoria_codigo,e.estado`, publicCatalogVisibleExpr))
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var cat, state string
		var n, visible int64
		if err = rows.Scan(&cat, &state, &n, &visible); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		out.Publications = append(out.Publications, KPIValue{Key: "inventario", Category: cat, State: state, Source: "espacio/categoria_espacio", Definition: fmt.Sprintf("Publicaciones de categoría %s en estado %s.", cat, state), Value: n, Available: true})
		out.Publications = append(out.Publications, KPIValue{Key: "efectivamente_visibles", Category: cat, State: state, Source: "política compartida de catálogo público", Definition: fmt.Sprintf("Visibilidad efectiva para categoría %s/estado %s según misma expresión del catálogo.", cat, state), Value: visible, Available: true})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT bucket,count(*) FROM (SELECT CASE WHEN n=0 THEN '0' WHEN n=1 THEN '1' WHEN n BETWEEN 2 AND 4 THEN '2-4' ELSE '5+' END bucket FROM (SELECT u.id,count(e.id) n FROM usuario u LEFT JOIN espacio e ON e.propietario_id=u.id GROUP BY u.id) per_owner) d GROUP BY bucket ORDER BY bucket`)
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var bucket string
		var n int64
		if err = rows.Scan(&bucket, &n); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		out.PublicationOwnerDistribution = append(out.PublicationOwnerDistribution, KPIValue{Key: bucket, Source: "espacio agrupado por propietario", Definition: "Cantidad de titulares por número de publicaciones; distribución agregada, sin identidad.", Value: n, Available: true})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `SELECT estado,count(*) FROM reserva_ensayo_local WHERE creada_en >= $1 AND creada_en < $2 GROUP BY estado ORDER BY estado`, p.From, p.Until)
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var state string
		var n int64
		if err = rows.Scan(&state, &n); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		out.Reservations = append(out.Reservations, KPIValue{Key: state, State: state, Denominator: cohortCount, Source: "reserva_ensayo_local.creada_en", Definition: "Reservas creadas en el período, agrupadas por estado vigente; denominador = total de reservas creadas en el período. No reconstruye estado anterior.", Value: n, Available: true})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	rows, err = tx.Query(ctx, `WITH decisions AS (
		SELECT estado_nuevo,count(DISTINCT reserva_id) AS total
		FROM reserva_ensayo_transicion
		WHERE creada_en >= $1 AND creada_en < $2 AND estado_nuevo IN ('aprobada_host','rechazada_arrendador')
		GROUP BY estado_nuevo
	), cancellations AS (
		SELECT estado_nuevo,count(DISTINCT reserva_id) AS total
		FROM reserva_ensayo_transicion
		WHERE creada_en >= $1 AND creada_en < $2 AND estado_nuevo IN ('cancelada_por_pago','cancelada_por_firma','cancelada_arrendatario')
		GROUP BY estado_nuevo
	), unioned AS (
		SELECT 'decision_anfitrion'::text AS group_key,estado_nuevo,total FROM decisions
		UNION ALL SELECT 'cancelacion',estado_nuevo,total FROM cancellations
	)
	SELECT group_key,estado_nuevo,total,
		CASE WHEN group_key='decision_anfitrion' THEN (SELECT count(DISTINCT reserva_id) FROM reserva_ensayo_transicion WHERE creada_en >= $1 AND creada_en < $2 AND estado_nuevo IN ('aprobada_host','rechazada_arrendador'))
		ELSE (SELECT count(DISTINCT reserva_id) FROM reserva_ensayo_transicion WHERE creada_en >= $1 AND creada_en < $2 AND estado_nuevo IN ('cancelada_por_pago','cancelada_por_firma','cancelada_arrendatario')) END
	FROM unioned ORDER BY group_key,estado_nuevo`, p.From, p.Until)
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var groupKey, state string
		var n, den int64
		if err = rows.Scan(&groupKey, &state, &n, &den); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		definition := fmt.Sprintf("%s: reservas únicas con estado terminal %s en el período; denominador = %s terminales del mismo grupo.", groupKey, state, groupKey)
		out.Reservations = append(out.Reservations, KPIValue{Key: groupKey + ":" + state, Category: groupKey, State: state, Denominator: den, Source: "reserva_ensayo_transicion.creada_en", Definition: definition, Value: n, Available: true})
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	var approvedDecisionCount, decisionDenominator int64
	if err = tx.QueryRow(ctx, `SELECT count(DISTINCT reserva_id) FILTER (WHERE estado_nuevo='aprobada_host'),count(DISTINCT reserva_id) FROM reserva_ensayo_transicion WHERE creada_en >= $1 AND creada_en < $2 AND estado_nuevo IN ('aprobada_host','rechazada_arrendador')`, p.From, p.Until).Scan(&approvedDecisionCount, &decisionDenominator); err != nil {
		return KPIReport{}, err
	}
	if decisionDenominator > 0 {
		out.Reservations = append(out.Reservations, KPIValue{Key: "approval_rate", Category: "decision_anfitrion", State: "aprobada_host", Denominator: decisionDenominator, Source: "reserva_ensayo_transicion", Definition: "Aprobaciones del anfitrión / decisiones terminales del anfitrión (aprobadas + rechazadas); no incluye cancelaciones.", Value: approvedDecisionCount, Available: true})
	} else {
		out.Reservations = append(out.Reservations, KPIValue{Key: "approval_rate", Category: "decision_anfitrion", Source: "reserva_ensayo_transicion", Definition: "Aprobaciones del anfitrión / decisiones terminales del anfitrión (aprobadas + rechazadas); no incluye cancelaciones.", Available: false, Reason: "sin_decisiones_terminales"})
	}
	for _, q := range []struct {
		label, sql, source, def string
		period                  bool
	}{{"operaciones_registradas", `SELECT count(*) FROM operacion_arriendo_ensayo_local WHERE ocurrio_en >= $1 AND ocurrio_en < $2 AND tipo IN ('checkout','recepcion')`, "operacion_arriendo_ensayo_local", "Eventos check-out/recepción registrados en el período; no equivalen a reservas cerradas ni a liquidaciones financieras.", true}, {"kyc_pendiente", `SELECT count(*) FROM verificacion WHERE estado='en_revision'`, "verificacion", "Cola vigente al generar; sin historial de entradas.", false}, {"privacidad_pendiente", `SELECT count(*) FROM solicitud_titular WHERE estado IN ('recibida','identidad_pendiente','en_revision')`, "solicitud_titular", "Solicitudes en estados pendientes vigentes.", false}, {"reclamos_abiertos", `SELECT count(*) FROM reclamo_dano_ensayo_local WHERE estado='abierto'`, "reclamo_dano_ensayo_local", "Reclamos vigentes cuyo estado de dominio es abierto; excluye resuelta.", false}, {"avisos_pendientes", `SELECT count(*) FROM aviso_local WHERE estado IN ('pendiente','procesando','fallo_terminal')`, "aviso_local", "Avisos en cola o fallo terminal al generar.", false}, {"reconciliacion_pendiente", `SELECT count(*) FROM reserva_pago_ensayo_operacion WHERE estado IN ('pendiente','vencida')`, "reserva_pago_ensayo_operacion", "Operaciones de pago pendientes/vencidas; sin sumar garantías.", false}} {
		var n int64
		if q.period {
			err = tx.QueryRow(ctx, q.sql, p.From, p.Until).Scan(&n)
		} else {
			err = tx.QueryRow(ctx, q.sql).Scan(&n)
		}
		if err != nil {
			return KPIReport{}, err
		}
		v := KPIValue{Source: q.source, Definition: q.def, Value: n, Available: true}
		if q.label == "operaciones_registradas" {
			out.Operations = append(out.Operations, v)
		} else {
			v.Definition = q.label + ": " + v.Definition
			out.Queues = append(out.Queues, v)
		}
	}
	rows, err = tx.Query(ctx, `SELECT kind,state,count(*)::bigint,coalesce(sum(amount),0)::text,source,definition FROM (
	 SELECT 'cobro_arriendo' kind,'confirmado' state,importe_clp amount,'reserva_pago_ensayo' source,'Cobros simulados confirmados por fecha de creación; excluye garantías.' definition FROM reserva_pago_ensayo WHERE resultado='exito_simulado' AND creada_en >= $1 AND creada_en < $2
	 UNION ALL SELECT 'devolucion','confirmado',importe_clp,'reserva_devolucion_ensayo','Devoluciones simuladas confirmadas por fecha de completitud.' FROM reserva_devolucion_ensayo WHERE estado='completada' AND ultimo_resultado='exito_simulado' AND completada_en >= $1 AND completada_en < $2
	 UNION ALL SELECT 'liquidacion','confirmado',transferencia_clp,'reserva_liquidacion_ensayo_local','Liquidaciones simuladas; transferencia neta en CLP y fecha de completitud.' FROM reserva_liquidacion_ensayo_local WHERE estado='liquidada' AND ultimo_resultado='exito_simulado' AND completada_en >= $1 AND completada_en < $2
	 UNION ALL SELECT 'comision_liquidacion','confirmado',comision_clp,'reserva_liquidacion_ensayo_local','Comisión contabilizada en la misma liquidación simulada; no equivale a ingreso real.' FROM reserva_liquidacion_ensayo_local WHERE estado='liquidada' AND ultimo_resultado='exito_simulado' AND completada_en >= $1 AND completada_en < $2
	) f GROUP BY kind,state,source,definition ORDER BY kind,state`, p.From, p.Until)
	if err != nil {
		return KPIReport{}, err
	}
	for rows.Next() {
		var k FinanceKPI
		if err = rows.Scan(&k.Kind, &k.State, &k.Count, &k.AmountCLP, &k.Source, &k.Definition); err != nil {
			rows.Close()
			return KPIReport{}, err
		}
		out.Finance = append(out.Finance, k)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return KPIReport{}, err
	}
	rows.Close()
	if err = s.audit(ctx, tx, actor, collectionID, "admin.global.kpi.read", "exito", "consulta_administrativa", correlation, out.GeneratedAt); err != nil {
		return KPIReport{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return KPIReport{}, err
	}
	return out, nil
}

type AuditFilter struct {
	From           time.Time  `json:"from"`
	Until          time.Time  `json:"until"`
	ActorID        string     `json:"actor_id,omitempty"`
	TargetID       string     `json:"target_account_id,omitempty"`
	ResourceType   string     `json:"resource_type,omitempty"`
	ResourceID     string     `json:"resource_id,omitempty"`
	Action         string     `json:"action,omitempty"`
	Result         string     `json:"result,omitempty"`
	ReasonCode     string     `json:"reason_code,omitempty"`
	CorrelationID  string     `json:"correlation_id,omitempty"`
	PageSize       int        `json:"page_size"`
	Cursor         string     `json:"cursor,omitempty"`
	SnapshotCutoff *time.Time `json:"snapshot_cutoff,omitempty"`
	Format         string     `json:"format,omitempty"`
}
type AuditEntry struct {
	ID            string    `json:"id"`
	ActorID       *string   `json:"actor_id"`
	TargetID      *string   `json:"target_account_id"`
	ResourceType  string    `json:"resource_type"`
	ResourceID    string    `json:"resource_id"`
	Action        string    `json:"action"`
	Result        string    `json:"result"`
	Reason        string    `json:"reason_code"`
	CorrelationID string    `json:"correlation_id"`
	OccurredAt    time.Time `json:"occurred_at"`
	RecordedAt    time.Time `json:"recorded_at"`
}
type AuditPage struct {
	Items          []AuditEntry `json:"items"`
	NextCursor     string       `json:"next_cursor"`
	From           time.Time    `json:"from"`
	Until          time.Time    `json:"until"`
	GeneratedAt    time.Time    `json:"generated_at"`
	SnapshotCutoff time.Time    `json:"snapshot_cutoff"`
	AppliedFilters AuditFilter  `json:"applied_filters"`
}

type auditCursor struct {
	Snapshot time.Time `json:"s"`
	Occurred time.Time `json:"o"`
	ID       string    `json:"i"`
	Filter   string    `json:"f"`
}

func (s *Service) Audit(ctx context.Context, actor, correlation string, f AuditFilter, export bool) (AuditPage, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(correlation) || f.From.IsZero() || !f.From.Before(f.Until) || f.Until.Sub(f.From) > 31*24*time.Hour || f.PageSize < 1 || f.PageSize > 100 || len(f.Cursor) > 2048 || !optionalUUID(f.ActorID) || !optionalUUID(f.TargetID) || !optionalUUID(f.ResourceID) || !optionalUUID(f.CorrelationID) || len(f.ResourceType) > 40 || len(f.Action) > 60 || len(f.ReasonCode) > 80 || (f.Result != "" && f.Result != "exito" && f.Result != "rechazo") || (f.Format != "" && f.Format != "json" && f.Format != "csv") {
		return AuditPage{}, ErrInvalid
	}
	f.From = f.From.UTC()
	f.Until = f.Until.UTC()
	requestedCursor := f.Cursor
	f.Cursor = ""
	now := s.now().UTC()
	cutoff := now
	if f.SnapshotCutoff != nil {
		cutoff = f.SnapshotCutoff.UTC()
		if cutoff.After(now) {
			return AuditPage{}, ErrInvalid
		}
	}
	if export && (requestedCursor != "" || f.SnapshotCutoff == nil) {
		return AuditPage{}, ErrInvalid
	}
	filterForHash := f
	filterForHash.PageSize = 0
	filterForHash.Format = ""
	filterForHash.SnapshotCutoff = nil
	fingerprintBytes, _ := json.Marshal(filterForHash)
	fingerprint := fmt.Sprintf("%x", fingerprintBytes)
	var cursor auditCursor
	if requestedCursor != "" {
		data, e := base64.RawURLEncoding.DecodeString(requestedCursor)
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if e != nil || len(data) > 2048 || decoder.Decode(&cursor) != nil || cursor.Filter != fingerprint || cursor.Snapshot.IsZero() || !canonicalUUID.MatchString(cursor.ID) || cursor.Occurred.IsZero() {
			return AuditPage{}, ErrInvalid
		}
		if f.SnapshotCutoff != nil && !f.SnapshotCutoff.UTC().Equal(cursor.Snapshot.UTC()) {
			return AuditPage{}, ErrInvalid
		}
		cutoff = cursor.Snapshot.UTC()
	} else if export && f.SnapshotCutoff != nil {
		cutoff = f.SnapshotCutoff.UTC()
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadWrite})
	if err != nil {
		return AuditPage{}, err
	}
	defer tx.Rollback(context.Background())
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return AuditPage{}, err
	}
	// Pages and exports are pinned to registro time, independently from when the file is generated.
	query := `SELECT id::text,actor_id::text,cuenta_objetivo_id::text,recurso_tipo,recurso_id::text,accion,resultado,motivo_codigo,correlacion_id::text,ocurrido_en,registrado_en FROM evento_auditoria_local WHERE ocurrido_en >= $1 AND ocurrido_en < $2 AND registrado_en <= $3 AND ($4='' OR actor_id::text=$4) AND ($5='' OR cuenta_objetivo_id::text=$5) AND ($6='' OR recurso_tipo=$6) AND ($7::uuid IS NULL OR recurso_id=$7::uuid) AND ($8='' OR accion=$8) AND ($9='' OR resultado=$9) AND ($10='' OR motivo_codigo=$10) AND ($11='' OR correlacion_id::text=$11) AND ($13::timestamptz IS NULL OR (ocurrido_en,id)<($13,$14::uuid)) ORDER BY ocurrido_en DESC,id DESC LIMIT $12`
	limit := f.PageSize + 1
	if export {
		limit = 10001
	}
	var lastAt any
	var lastID any
	if requestedCursor != "" && !export {
		lastAt = cursor.Occurred
		lastID = cursor.ID
	}
	rows, err := tx.Query(ctx, query, f.From, f.Until, cutoff, f.ActorID, f.TargetID, f.ResourceType, nullableUUID(f.ResourceID), f.Action, f.Result, f.ReasonCode, f.CorrelationID, limit, lastAt, lastID)
	if err != nil {
		return AuditPage{}, err
	}
	f.SnapshotCutoff = &cutoff
	out := AuditPage{Items: []AuditEntry{}, From: f.From, Until: f.Until, GeneratedAt: now, SnapshotCutoff: cutoff, AppliedFilters: f}
	for rows.Next() {
		var e AuditEntry
		var actorID, target *string
		if err = rows.Scan(&e.ID, &actorID, &target, &e.ResourceType, &e.ResourceID, &e.Action, &e.Result, &e.Reason, &e.CorrelationID, &e.OccurredAt, &e.RecordedAt); err != nil {
			rows.Close()
			return AuditPage{}, err
		}
		e.ActorID = actorID
		e.TargetID = target
		out.Items = append(out.Items, e)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return AuditPage{}, err
	}
	rows.Close()
	if export && len(out.Items) > 10000 {
		if err = s.audit(ctx, tx, actor, collectionID, "admin.global.audit.export", "rechazo", "limite_filas_excedido", correlation, cutoff); err != nil {
			return AuditPage{}, err
		}
		if err = tx.Commit(ctx); err != nil {
			return AuditPage{}, err
		}
		return AuditPage{}, ErrInvalid
	}
	if export {
		var body []byte
		if f.Format == "csv" {
			var b bytes.Buffer
			if err = WriteAuditCSV(&b, out); err != nil {
				return AuditPage{}, err
			}
			body = b.Bytes()
		} else {
			body, err = json.Marshal(map[string]any{"data": out})
			if err != nil {
				return AuditPage{}, err
			}
		}
		if len(body) > 8*1024*1024 {
			if err = s.audit(ctx, tx, actor, collectionID, "admin.global.audit.export", "rechazo", "limite_bytes_excedido", correlation, cutoff); err != nil {
				return AuditPage{}, err
			}
			if err = tx.Commit(ctx); err != nil {
				return AuditPage{}, err
			}
			return AuditPage{}, ErrInvalid
		}
	}
	if !export && len(out.Items) > f.PageSize {
		out.Items = out.Items[:f.PageSize]
		cur, _ := json.Marshal(auditCursor{Snapshot: cutoff, Occurred: out.Items[len(out.Items)-1].OccurredAt, ID: out.Items[len(out.Items)-1].ID, Filter: fingerprint})
		out.NextCursor = base64.RawURLEncoding.EncodeToString(cur)
	}
	if err = s.audit(ctx, tx, actor, collectionID, "admin.global.audit."+map[bool]string{true: "export", false: "read"}[export], "exito", "consulta_administrativa", correlation, out.GeneratedAt); err != nil {
		return AuditPage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AuditPage{}, err
	}
	return out, nil
}

func optionalUUID(s string) bool { return s == "" || canonicalUUID.MatchString(s) }
func nullableUUID(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func WriteAuditCSV(w io.Writer, p AuditPage) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{"id", "actor_id", "target_account_id", "resource_type", "resource_id", "action", "result", "reason_code", "correlation_id", "occurred_at", "recorded_at", "query_from", "query_until", "snapshot_cutoff", "generated_at", "applied_filters"}); err != nil {
		return err
	}
	filterBytes, _ := json.Marshal(p.AppliedFilters)
	metadata := []string{p.From.UTC().Format(time.RFC3339Nano), p.Until.UTC().Format(time.RFC3339Nano), p.SnapshotCutoff.UTC().Format(time.RFC3339Nano), p.GeneratedAt.UTC().Format(time.RFC3339Nano), string(filterBytes)}
	items := p.Items
	if len(items) == 0 {
		items = []AuditEntry{{}}
	}
	for _, e := range items {
		vals := []string{e.ID, pstr(e.ActorID), pstr(e.TargetID), e.ResourceType, e.ResourceID, e.Action, e.Result, e.Reason, e.CorrelationID, e.OccurredAt.UTC().Format(time.RFC3339Nano), e.RecordedAt.UTC().Format(time.RFC3339Nano), metadata[0], metadata[1], metadata[2], metadata[3], metadata[4]}
		for i, v := range vals {
			if i >= 3 && i <= 8 && len(v) > 0 && strings.ContainsRune("=+-@\t\r", rune(v[0])) {
				vals[i] = "'" + v
			}
		}
		if err := cw.Write(vals); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}
func pstr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
