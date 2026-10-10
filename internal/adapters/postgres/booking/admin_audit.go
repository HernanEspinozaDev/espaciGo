package bookingpg

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/jackc/pgx/v5"
)

const auditCollectionID = "00000000-0000-0000-0000-000000000000"

var adminAuditCursorAEAD = func() cipher.AEAD {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("initialize local audit cursor encryption")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic("initialize local audit cursor cipher")
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic("initialize local audit cursor cipher")
	}
	return aead
}()

type auditCursor struct {
	Version    int       `json:"v"`
	Actor      string    `json:"a"`
	Filter     string    `json:"f"`
	PageSize   int       `json:"s"`
	SnapshotAt time.Time `json:"h_at"`
	LastAt     time.Time `json:"l_at"`
	LastID     string    `json:"l_id"`
}

func auditFilterHash(f booking.AdminAuditFilter) string {
	b, _ := json.Marshal(f)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}
func decodeAuditCursor(encoded, actor, filter string, size int) (*auditCursor, error) {
	if encoded == "" {
		return nil, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(b) > 4096 || len(b) < adminAuditCursorAEAD.NonceSize()+adminAuditCursorAEAD.Overhead() {
		return nil, booking.ErrInvalid
	}
	plain, err := adminAuditCursorAEAD.Open(nil, b[:adminAuditCursorAEAD.NonceSize()], b[adminAuditCursorAEAD.NonceSize():], []byte("admin-audit-cursor-v1"))
	if err != nil {
		return nil, booking.ErrInvalid
	}
	var c auditCursor
	if json.Unmarshal(plain, &c) != nil || c.Version != 1 || c.Actor != actor || c.Filter != filter || c.PageSize != size || c.SnapshotAt.IsZero() || c.LastAt.IsZero() || !bookingUUID.MatchString(c.LastID) {
		return nil, booking.ErrInvalid
	}
	return &c, nil
}
func encodeAuditCursor(c auditCursor) (string, error) {
	b, _ := json.Marshal(c)
	nonce := make([]byte, adminAuditCursorAEAD.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := adminAuditCursorAEAD.Seal(nil, nonce, b, []byte("admin-audit-cursor-v1"))
	return base64.RawURLEncoding.EncodeToString(append(nonce, sealed...)), nil
}

func (r *Repository) ListAdminAudit(ctx context.Context, actor, correlation string, filter booking.AdminAuditFilter, pageSize int, encoded string) (booking.AdminAuditPage, error) {
	if !bookingUUID.MatchString(actor) || pageSize < 1 || pageSize > 100 || correlation == "" || len(correlation) > 120 {
		return booking.AdminAuditPage{}, booking.ErrInvalid
	}
	hash := auditFilterHash(filter)
	cursor, err := decodeAuditCursor(encoded, actor, hash, pageSize)
	if err != nil {
		return booking.AdminAuditPage{}, err
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return booking.AdminAuditPage{}, err
	}
	defer tx.Rollback(ctx)
	var snapshotAt time.Time
	if cursor == nil {
		err = tx.QueryRow(ctx, `SELECT statement_timestamp()`).Scan(&snapshotAt)
		if err != nil {
			return booking.AdminAuditPage{}, err
		}
	} else {
		snapshotAt = cursor.SnapshotAt
	}
	if _, err = insertAuditCollectionRead(ctx, tx, actor, "admin.audit.events.list", "exito", "consulta_administrativa", correlation); err != nil {
		return booking.AdminAuditPage{}, err
	}
	page := booking.AdminAuditPage{Items: []booking.AdminAuditEntry{}}
	{
		var lastAt *time.Time
		var lastID any
		if cursor != nil {
			v := cursor.LastAt
			lastAt = &v
			lastID = cursor.LastID
		}
		args := append(auditArgs(filter), snapshotAt, lastAt, lastID, pageSize+1)
		rows, e := tx.Query(ctx, `SELECT id::text,ocurrido_en,actor_id::text,recurso_tipo,recurso_id::text,accion,resultado,motivo_codigo,correlacion_id FROM public.evento_auditoria_local WHERE `+auditWhere+` AND ocurrido_en<$8 AND ($9::timestamptz IS NULL OR (ocurrido_en,id)<($9,$10::uuid)) ORDER BY ocurrido_en DESC,id DESC LIMIT $11`, args...)
		if e != nil {
			return page, e
		}
		for rows.Next() {
			var v booking.AdminAuditEntry
			var actorID sql.NullString
			if e = rows.Scan(&v.ID, &v.OccurredAt, &actorID, &v.ResourceType, &v.ResourceID, &v.Action, &v.Result, &v.ReasonCode, &v.CorrelationID); e != nil {
				rows.Close()
				return page, e
			}
			if actorID.Valid {
				x := actorID.String
				v.ActorID = &x
			}
			page.Items = append(page.Items, v)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return page, e
		}
		rows.Close()
		if len(page.Items) > pageSize {
			page.Items = page.Items[:pageSize]
			last := page.Items[len(page.Items)-1]
			page.NextCursor, err = encodeAuditCursor(auditCursor{Version: 1, Actor: actor, Filter: hash, PageSize: pageSize, SnapshotAt: snapshotAt, LastAt: last.OccurredAt, LastID: last.ID})
			if err != nil {
				return booking.AdminAuditPage{}, err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.AdminAuditPage{}, err
	}
	return page, nil
}

func (r *Repository) ExportAdminAudit(ctx context.Context, actor, correlation string, filter booking.AdminAuditFilter) (booking.AdminAuditExport, error) {
	if !bookingUUID.MatchString(actor) || correlation == "" || len(correlation) > 120 {
		return booking.AdminAuditExport{}, booking.ErrInvalid
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return booking.AdminAuditExport{}, err
	}
	defer tx.Rollback(ctx)
	var snapshotAt time.Time
	err = tx.QueryRow(ctx, `SELECT statement_timestamp()`).Scan(&snapshotAt)
	if err != nil {
		return booking.AdminAuditExport{}, err
	}
	generated := snapshotAt.UTC()
	args := append(auditArgs(filter), snapshotAt)
	count := int64(0)
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.evento_auditoria_local WHERE `+auditWhere+` AND ocurrido_en<$8`, args...).Scan(&count); err != nil {
		return booking.AdminAuditExport{}, err
	}
	result, reason := "exito", "consulta_administrativa"
	if count > 10000 {
		result, reason = "rechazo", "limite_exportacion"
	}
	if _, err = insertAuditCollectionRead(ctx, tx, actor, "admin.audit.events.export", result, reason, correlation); err != nil {
		return booking.AdminAuditExport{}, err
	}
	if count > 10000 {
		if err = tx.Commit(ctx); err != nil {
			return booking.AdminAuditExport{}, err
		}
		return booking.AdminAuditExport{}, booking.ErrAdminAuditExportLimit
	}
	events := []booking.AdminAuditEntry{}
	{
		args := append(auditArgs(filter), snapshotAt, 10001)
		rows, e := tx.Query(ctx, `SELECT id::text,ocurrido_en,actor_id::text,recurso_tipo,recurso_id::text,accion,resultado,motivo_codigo,correlacion_id FROM public.evento_auditoria_local WHERE `+auditWhere+` AND ocurrido_en<$8 ORDER BY ocurrido_en DESC,id DESC LIMIT $9`, args...)
		if e != nil {
			return booking.AdminAuditExport{}, e
		}
		for rows.Next() {
			var v booking.AdminAuditEntry
			var actorID sql.NullString
			if e = rows.Scan(&v.ID, &v.OccurredAt, &actorID, &v.ResourceType, &v.ResourceID, &v.Action, &v.Result, &v.ReasonCode, &v.CorrelationID); e != nil {
				rows.Close()
				return booking.AdminAuditExport{}, e
			}
			if actorID.Valid {
				x := actorID.String
				v.ActorID = &x
			}
			events = append(events, v)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return booking.AdminAuditExport{}, e
		}
		rows.Close()
	}
	if err = tx.Commit(ctx); err != nil {
		return booking.AdminAuditExport{}, err
	}
	return booking.AdminAuditExport{Version: 1, GeneratedAt: generated, Filters: filter, Events: events}, nil
}

const auditWhere = `ocurrido_en >= $1 AND ocurrido_en < $2 AND retirar_en > clock_timestamp() AND ($3='' OR actor_id::text=lower($3)) AND ($4='' OR recurso_tipo=$4) AND ($5='' OR recurso_id::text=lower($5)) AND ($6='' OR accion=$6) AND ($7='' OR resultado=$7)`

func auditArgs(f booking.AdminAuditFilter) []any {
	return []any{f.From.UTC(), f.Until.UTC(), strings.ToLower(f.ActorID), f.ResourceType, strings.ToLower(f.ResourceID), f.Action, f.Result}
}
func insertAuditCollectionRead(ctx context.Context, tx pgx.Tx, actor, action, result, reason, correlation string) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en) VALUES(gen_random_uuid(),$1,'coleccion_eventos_auditoria_local',$2,$3,$4,$5,$6,statement_timestamp(),statement_timestamp()+interval '5 years') RETURNING id::text`, actor, auditCollectionID, action, result, reason, correlation).Scan(&id)
	return id, err
}

func (r *Repository) RecordAdminAuditAttempt(ctx context.Context, actor, correlation, action, result, reason string) error {
	if !bookingUUID.MatchString(actor) || correlation == "" || len(correlation) > 120 {
		return booking.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err = insertAuditCollectionRead(ctx, tx, actor, action, result, reason, correlation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
