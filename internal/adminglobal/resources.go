package adminglobal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

type AccountResource struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	SpaceID       string    `json:"space_id,omitempty"`
	ReservationID string    `json:"reservation_id,omitempty"`
	State         string    `json:"state"`
	Participant   string    `json:"participant_role,omitempty"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type AccountResourcePage struct {
	Items         []AccountResource `json:"items"`
	NextCursor    string            `json:"next_cursor"`
	MatchingTotal int64             `json:"matching_total"`
	GeneratedAt   time.Time         `json:"generated_at"`
}

type accountResourceCursor struct {
	Version int       `json:"v"`
	Filter  string    `json:"f"`
	At      time.Time `json:"a"`
	ID      string    `json:"i"`
}

func (s *Service) ListAccountResources(ctx context.Context, actor, accountID, correlation, kind, encodedCursor string, pageSize int) (AccountResourcePage, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(accountID) || !canonicalUUID.MatchString(correlation) || len(encodedCursor) > 2048 || pageSize < 1 || pageSize > 50 {
		return AccountResourcePage{}, ErrInvalid
	}
	query, valid := accountResourceQuery(kind)
	if !valid {
		return AccountResourcePage{}, ErrInvalid
	}
	filterBytes, _ := json.Marshal(struct{ Account, Kind string }{accountID, kind})
	filterHashBytes := sha256.Sum256(filterBytes)
	filterHash := hex.EncodeToString(filterHashBytes[:])
	var cursor accountResourceCursor
	if encodedCursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(encodedCursor)
		if err != nil || len(raw) > 1024 {
			return AccountResourcePage{}, ErrInvalid
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&cursor) != nil || cursor.Version != 1 || cursor.Filter != filterHash || cursor.At.IsZero() || !canonicalUUID.MatchString(cursor.ID) {
			return AccountResourcePage{}, ErrInvalid
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return AccountResourcePage{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return AccountResourcePage{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.usuario WHERE id=$1)`, accountID).Scan(&exists); err != nil {
		return AccountResourcePage{}, err
	}
	if !exists {
		return AccountResourcePage{}, ErrNotFound
	}
	var total int64
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM (`+query+`) resources`, accountID).Scan(&total); err != nil {
		return AccountResourcePage{}, err
	}
	args := []any{accountID}
	pageQuery := `SELECT id,kind,space_id,reservation_id,state,participant,occurred_at FROM (` + query + `) resources WHERE ($2::timestamptz IS NULL OR (occurred_at,id)<($2,$3::text)) ORDER BY occurred_at DESC,id DESC LIMIT $4`
	var lastAt any
	var lastID any
	if cursor.Version == 1 {
		lastAt = cursor.At.UTC()
		lastID = cursor.ID
	}
	args = append(args, lastAt, lastID, pageSize+1)
	rows, err := tx.Query(ctx, pageQuery, args...)
	if err != nil {
		return AccountResourcePage{}, err
	}
	items := make([]AccountResource, 0, pageSize+1)
	for rows.Next() {
		var item AccountResource
		if err = rows.Scan(&item.ID, &item.Kind, &item.SpaceID, &item.ReservationID, &item.State, &item.Participant, &item.OccurredAt); err != nil {
			rows.Close()
			return AccountResourcePage{}, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return AccountResourcePage{}, err
	}
	rows.Close()
	page := AccountResourcePage{Items: items, MatchingTotal: total, GeneratedAt: s.now().UTC()}
	if len(items) > pageSize {
		page.Items = items[:pageSize]
		last := page.Items[len(page.Items)-1]
		encoded, marshalErr := json.Marshal(accountResourceCursor{Version: 1, Filter: filterHash, At: last.OccurredAt.UTC(), ID: last.ID})
		if marshalErr != nil {
			return AccountResourcePage{}, marshalErr
		}
		page.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	if err = s.auditWithTarget(ctx, tx, actor, accountID, "admin.global.accounts.resources.list", "exito", "consulta_recurso", correlation, page.GeneratedAt, accountID); err != nil {
		return AccountResourcePage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AccountResourcePage{}, err
	}
	return page, nil
}

func (s *Service) GetAccountResource(ctx context.Context, actor, accountID, resourceID, correlation, kind string) (map[string]any, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(accountID) || !canonicalUUID.MatchString(resourceID) || !canonicalUUID.MatchString(correlation) {
		return nil, ErrInvalid
	}
	query, valid := accountResourceDetailQuery(kind)
	if !valid {
		return nil, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err = tx.QueryRow(ctx, query, resourceID, accountID).Scan(&raw); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	var detail map[string]any
	if err = json.Unmarshal(raw, &detail); err != nil {
		return nil, err
	}
	if err = s.auditWithTarget(ctx, tx, actor, resourceID, "admin.global.accounts.resources.read", "exito", "consulta_recurso", correlation, s.now().UTC(), accountID); err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return detail, nil
}

func accountResourceQuery(kind string) (string, bool) {
	switch kind {
	case "reservations":
		return `SELECT id::text AS id,'reservation'::text AS kind,espacio_id::text AS space_id,id::text AS reservation_id,estado AS state,CASE WHEN anfitrion_id=$1 THEN 'arrendador' ELSE 'arrendatario' END AS participant,creada_en AS occurred_at FROM public.reserva_ensayo_local WHERE anfitrion_id=$1 OR arrendatario_id=$1`, true
	case "incidents":
		return `SELECT c.id::text AS id,'damage_claim'::text AS kind,r.espacio_id::text AS space_id,r.id::text AS reservation_id,CASE WHEN EXISTS(SELECT 1 FROM public.reclamo_dano_resolucion_ensayo_local x WHERE x.reclamo_id=c.id) THEN 'resuelta' ELSE c.estado END AS state,CASE WHEN c.anfitrion_id=$1 THEN 'arrendador' ELSE 'arrendatario' END AS participant,c.abierto_en AS occurred_at FROM public.reclamo_dano_ensayo_local c JOIN public.reserva_ensayo_local r ON r.id=c.reserva_id WHERE c.anfitrion_id=$1 OR c.arrendatario_id=$1 UNION ALL SELECT d.id::text AS id,'dispute'::text AS kind,r.espacio_id::text AS space_id,r.id::text AS reservation_id,d.estado AS state,CASE WHEN d.anfitrion_id=$1 THEN 'arrendador' ELSE 'arrendatario' END AS participant,d.abierta_en AS occurred_at FROM public.disputa_ensayo_local d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE d.anfitrion_id=$1 OR d.arrendatario_id=$1`, true
	default:
		return "", false
	}
}

func accountResourceDetailQuery(kind string) (string, bool) {
	switch kind {
	case "reservations":
		return `SELECT jsonb_build_object('id',id,'space_id',espacio_id,'host_id',anfitrion_id,'renter_id',arrendatario_id,'state',estado,'start_at',inicio,'end_at',termino,'subtotal_clp',subtotal_clp::text,'currency',moneda,'created_at',creada_en,'updated_at',actualizada_en) FROM public.reserva_ensayo_local WHERE id=$1 AND (anfitrion_id=$2 OR arrendatario_id=$2)`, true
	case "incidents":
		return `SELECT detail FROM (SELECT jsonb_build_object('id',c.id,'kind','damage_claim','reservation_id',c.reserva_id,'space_id',r.espacio_id,'state',CASE WHEN x.reclamo_id IS NULL THEN c.estado ELSE 'resuelta' END,'description',c.descripcion,'occurred_at',c.abierto_en,'host_id',c.anfitrion_id,'renter_id',c.arrendatario_id) AS detail FROM public.reclamo_dano_ensayo_local c JOIN public.reserva_ensayo_local r ON r.id=c.reserva_id LEFT JOIN public.reclamo_dano_resolucion_ensayo_local x ON x.reclamo_id=c.id WHERE c.id=$1 AND (c.anfitrion_id=$2 OR c.arrendatario_id=$2) UNION ALL SELECT jsonb_build_object('id',d.id,'kind','dispute','reservation_id',d.reserva_id,'space_id',r.espacio_id,'state',d.estado,'reason_code',d.motivo_codigo,'occurred_at',d.abierta_en,'host_id',d.anfitrion_id,'renter_id',d.arrendatario_id) FROM public.disputa_ensayo_local d JOIN public.reserva_ensayo_local r ON r.id=d.reserva_id WHERE d.id=$1 AND (d.anfitrion_id=$2 OR d.arrendatario_id=$2)) details(detail) LIMIT 1`, true
	default:
		return "", false
	}
}
