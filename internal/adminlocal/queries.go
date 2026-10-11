package adminlocal

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
)

var adminLocalUUID = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

const adminCollectionID = "00000000-0000-0000-0000-000000000000"

type AccountLookup struct {
	AccountID string    `json:"account_id"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
	Blocked   bool      `json:"blocked"`
	Roles     []string  `json:"roles"`
}

type PublishedSpace struct {
	SpaceID      string    `json:"space_id"`
	OwnerID      *string   `json:"owner_id"`
	Title        string    `json:"title"`
	Category     string    `json:"category_code"`
	CategoryName string    `json:"category_name"`
	State        string    `json:"state"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type PublishedSpaces struct {
	Items []PublishedSpace `json:"items"`
}

type DisputeStateCount struct {
	State string `json:"state"`
	Count int64  `json:"count"`
}

type DisputesReport struct {
	GeneratedAt time.Time           `json:"generated_at"`
	Period      Period              `json:"period"`
	Notice      string              `json:"notice"`
	Items       []DisputeStateCount `json:"items"`
}

func (s *Service) LookupAccount(ctx context.Context, actor, target, correlation string) (AccountLookup, error) {
	if !adminLocalUUID.MatchString(actor) || !adminLocalUUID.MatchString(target) || correlation == "" || len(correlation) > 120 {
		return AccountLookup{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return AccountLookup{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAdminAndOptionalAccount(ctx, tx, actor, target); err != nil {
		return AccountLookup{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return AccountLookup{}, err
	}
	var out AccountLookup
	var found bool
	err = tx.QueryRow(ctx, `SELECT u.id::text,u.estado,u.creado_en,
		EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id),
		COALESCE(array_agg(r.rol ORDER BY r.rol) FILTER (WHERE r.rol IS NOT NULL),'{}'::text[])
		FROM public.usuario u LEFT JOIN public.rol_usuario r ON r.usuario_id=u.id
		WHERE u.id=$1 GROUP BY u.id`, target).Scan(&out.AccountID, &out.State, &out.CreatedAt, &out.Blocked, &out.Roles)
	if err == nil {
		found = true
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return AccountLookup{}, err
	}
	at := s.now().UTC()
	result, reason := "exito", "consulta_administrativa"
	if !found {
		result, reason = "rechazo", "cuenta_no_encontrada"
	}
	if err = writeAudit(ctx, tx, s.ids, actor, "cuenta", target, "admin.accounts.lookup", result, reason, correlation, at); err != nil {
		return AccountLookup{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return AccountLookup{}, err
	}
	if !found {
		return AccountLookup{}, ErrNotFound
	}
	if out.Roles == nil {
		out.Roles = []string{}
	}
	return out, nil
}

func lockAdminAndOptionalAccount(ctx context.Context, tx pgx.Tx, actor, target string) error {
	rows, err := tx.Query(ctx, `SELECT id::text FROM public.usuario WHERE id=$1 OR id=$2 ORDER BY id FOR UPDATE`, actor, target)
	if err != nil {
		return err
	}
	actorFound := false
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		if id == actor {
			actorFound = true
		}
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	if !actorFound {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ListPublishedSpaces(ctx context.Context, actor, correlation string) (PublishedSpaces, error) {
	if !adminLocalUUID.MatchString(actor) || correlation == "" || len(correlation) > 120 {
		return PublishedSpaces{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublishedSpaces{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor); err != nil {
		return PublishedSpaces{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return PublishedSpaces{}, err
	}
	at := s.now().UTC()
	if err = writeAudit(ctx, tx, s.ids, actor, "coleccion_publicaciones_locales", adminCollectionID, "admin.publications.list", "exito", "consulta_administrativa", correlation, at); err != nil {
		return PublishedSpaces{}, err
	}
	rows, err := tx.Query(ctx, `SELECT e.id::text,e.propietario_id::text,e.titulo,e.categoria_codigo,c.nombre,e.estado,e.actualizado_en
		FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo
		WHERE e.estado='activa' ORDER BY e.actualizado_en DESC,e.id DESC`)
	if err != nil {
		return PublishedSpaces{}, err
	}
	items := make([]PublishedSpace, 0)
	for rows.Next() {
		var item PublishedSpace
		var owner sql.NullString
		if err = rows.Scan(&item.SpaceID, &owner, &item.Title, &item.Category, &item.CategoryName, &item.State, &item.UpdatedAt); err != nil {
			rows.Close()
			return PublishedSpaces{}, err
		}
		if owner.Valid {
			id := owner.String
			item.OwnerID = &id
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return PublishedSpaces{}, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return PublishedSpaces{}, err
	}
	return PublishedSpaces{Items: items}, nil
}

func (s *Service) GetPublishedSpace(ctx context.Context, actor, spaceID, correlation string) (PublishedSpace, error) {
	if !adminLocalUUID.MatchString(actor) || !adminLocalUUID.MatchString(spaceID) || correlation == "" || len(correlation) > 120 {
		return PublishedSpace{}, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PublishedSpace{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor); err != nil {
		return PublishedSpace{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return PublishedSpace{}, err
	}
	var out PublishedSpace
	var owner sql.NullString
	err = tx.QueryRow(ctx, `SELECT e.id::text,e.propietario_id::text,e.titulo,e.categoria_codigo,c.nombre,e.estado,e.actualizado_en
		FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo
		WHERE e.id=$1 AND e.estado='activa'`, spaceID).Scan(&out.SpaceID, &owner, &out.Title, &out.Category, &out.CategoryName, &out.State, &out.UpdatedAt)
	found := err == nil
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return PublishedSpace{}, err
	}
	if owner.Valid {
		id := owner.String
		out.OwnerID = &id
	}
	at := s.now().UTC()
	result, reason := "exito", "consulta_administrativa"
	if !found {
		result, reason = "rechazo", "publicacion_no_encontrada"
	}
	if err = writeAudit(ctx, tx, s.ids, actor, "publicacion_local", spaceID, "admin.publications.read", result, reason, correlation, at); err != nil {
		return PublishedSpace{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PublishedSpace{}, err
	}
	if !found {
		return PublishedSpace{}, ErrNotFound
	}
	return out, nil
}

func (s *Service) DisputesReport(ctx context.Context, actor, correlation string, p Period) (DisputesReport, error) {
	if !adminLocalUUID.MatchString(actor) || correlation == "" || len(correlation) > 120 || p.Validate() != nil {
		return DisputesReport{}, ErrInvalid
	}
	p = p.normalizedUTC()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return DisputesReport{}, err
	}
	defer tx.Rollback(ctx)
	if err = lockAccounts(ctx, tx, actor); err != nil {
		return DisputesReport{}, err
	}
	if err = requireAdmin(ctx, tx, actor); err != nil {
		return DisputesReport{}, err
	}
	at := s.now().UTC()
	if err = writeAudit(ctx, tx, s.ids, actor, "coleccion_reclamos_dano_ensayo_local", adminCollectionID, "admin.reports.disputes", "exito", "consulta_solo_lectura", correlation, at); err != nil {
		return DisputesReport{}, err
	}
	rows, err := tx.Query(ctx, `SELECT estado,count(*) FROM public.reclamo_dano_ensayo_local WHERE abierto_en >= $1 AND abierto_en < $2 GROUP BY estado ORDER BY estado`, p.From, p.Until)
	if err != nil {
		return DisputesReport{}, err
	}
	items := make([]DisputeStateCount, 0)
	for rows.Next() {
		var item DisputeStateCount
		if err = rows.Scan(&item.State, &item.Count); err != nil {
			rows.Close()
			return DisputesReport{}, err
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return DisputesReport{}, err
	}
	rows.Close()
	if err = tx.Commit(ctx); err != nil {
		return DisputesReport{}, err
	}
	return DisputesReport{GeneratedAt: at, Period: p, Notice: SafetyNotice, Items: items}, nil
}
