package adminglobal

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
)

var retirementReasons = map[string]bool{
	"moderacion_contenido":    true,
	"riesgo_seguridad":        true,
	"solicitud_titular":       true,
	"revision_administrativa": true,
}

type PublicationFilter struct {
	OwnerID  string `json:"owner_id,omitempty"`
	Category string `json:"category,omitempty"`
	State    string `json:"state,omitempty"`
	Search   string `json:"search,omitempty"`
	Visible  *bool  `json:"publicly_visible,omitempty"`
	PageSize int    `json:"page_size"`
	Cursor   string `json:"cursor,omitempty"`
}
type Publication struct {
	ID                     string         `json:"publication_id"`
	OwnerID                string         `json:"owner_id"`
	OwnerName              *string        `json:"owner_name"`
	OwnerEmail             string         `json:"owner_email"`
	Category               string         `json:"category_code"`
	CategoryName           string         `json:"category_name"`
	Title                  string         `json:"title"`
	Description            string         `json:"description"`
	AreaM2                 string         `json:"area_m2"`
	Capacity               int32          `json:"capacity"`
	UsageRules             string         `json:"usage_rules"`
	Address                string         `json:"address"`
	State                  string         `json:"state"`
	RateUnit               string         `json:"rate_unit"`
	BasePriceCLP           string         `json:"base_price_clp"`
	UpdatedAt              time.Time      `json:"updated_at"`
	PubliclyVisible        bool           `json:"publicly_visible"`
	PublicCoverAvailable   bool           `json:"public_cover_available"`
	AttributeSchemaVersion int            `json:"attribute_schema_version"`
	Attributes             map[string]any `json:"attributes"`
	AttributeSchema        spaces.Profile `json:"attribute_schema"`
}
type PublicationPage struct {
	Items         []Publication `json:"items"`
	NextCursor    string        `json:"next_cursor"`
	MatchingTotal int64         `json:"matching_total"`
	GeneratedAt   time.Time     `json:"generated_at"`
}
type PublicationStateChange struct {
	PublicationID string    `json:"publication_id"`
	OwnerID       string    `json:"owner_id"`
	State         string    `json:"state"`
	ChangedAt     time.Time `json:"changed_at"`
}
type PublicationStateInput struct {
	State             string    `json:"state"`
	ExpectedUpdatedAt time.Time `json:"expected_updated_at"`
	Reason            string    `json:"reason"`
}
type PublicationContentUpdate struct {
	Title                  *string        `json:"title,omitempty"`
	Description            *string        `json:"description,omitempty"`
	Capacity               *int32         `json:"capacity,omitempty"`
	UsageRules             *string        `json:"usage_rules,omitempty"`
	BasePriceCLP           *string        `json:"base_price_clp,omitempty"`
	RegionCode             *string        `json:"region_code,omitempty"`
	CommuneCode            *string        `json:"comuna_code,omitempty"`
	CoverPhotoID           *string        `json:"cover_photo_id,omitempty"`
	AttributeSchemaVersion *int           `json:"attribute_schema_version,omitempty"`
	Attributes             map[string]any `json:"attributes,omitempty"`
	ExpectedUpdatedAt      time.Time      `json:"expected_updated_at"`
	Reason                 string         `json:"reason"`
}

func (s *Service) UpdatePublication(ctx context.Context, actor, id, correlation string, in PublicationContentUpdate) (Publication, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(correlation) || in.ExpectedUpdatedAt.IsZero() || !accountReasons[in.Reason] {
		return Publication{}, ErrInvalid
	}
	var price *int64
	if in.BasePriceCLP != nil {
		n, err := strconv.ParseInt(*in.BasePriceCLP, 10, 64)
		if err != nil || n <= 5000 {
			return Publication{}, ErrInvalid
		}
		price = &n
	}
	content := spaces.PublishedContentInput{Title: in.Title, Description: in.Description, Capacity: in.Capacity, UsageRules: in.UsageRules, BasePriceCLP: price, RegionCode: in.RegionCode, CommuneCode: in.CommuneCode}
	hasContent := in.Title != nil || in.Description != nil || in.Capacity != nil || in.UsageRules != nil || in.BasePriceCLP != nil || in.RegionCode != nil || in.CommuneCode != nil || in.Attributes != nil
	if !hasContent && in.CoverPhotoID == nil {
		return Publication{}, ErrInvalid
	}
	if (in.Title != nil || in.Description != nil || in.Capacity != nil || in.UsageRules != nil || in.BasePriceCLP != nil || in.RegionCode != nil || in.CommuneCode != nil) && content.Validate() != nil {
		return Publication{}, ErrInvalid
	}
	if in.CoverPhotoID != nil && *in.CoverPhotoID != "" && !canonicalUUID.MatchString(*in.CoverPhotoID) {
		return Publication{}, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return Publication{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var owner string
	if err = tx.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, id).Scan(&owner); errors.Is(err, pgx.ErrNoRows) {
		return Publication{}, ErrNotFound
	} else if err != nil {
		return Publication{}, err
	}
	if actor == owner { return Publication{}, ErrNotFound }
	if err = lockManagedAccounts(ctx, tx, actor, owner); err != nil {
		return Publication{}, err
	}
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return Publication{}, err
	}
	var state, unit string
	var updated time.Time
	var oldPrice int64
	err = tx.QueryRow(ctx, `SELECT estado,actualizado_en,modalidad_tarifa,precio_base_clp FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, id, owner).Scan(&state, &updated, &unit, &oldPrice)
	if errors.Is(err, pgx.ErrNoRows) {
		return Publication{}, ErrNotFound
	}
	if err != nil {
		return Publication{}, err
	}
	if state != "activa" && state != "oculta" && state != "borrador" {
		return Publication{}, ErrConflict
	}
	if !updated.Equal(in.ExpectedUpdatedAt.UTC()) {
		return Publication{}, ErrConflict
	}
	if in.CoverPhotoID != nil {
		if *in.CoverPhotoID != "" {
			var valid bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.espacio_galeria_sintetica_local WHERE id=$1 AND espacio_id=$2 AND propietario_id=$3 AND estado='activa' AND publicada_en_catalogo AND archivo_id IS NOT NULL AND mime_type='image/png' AND size_bytes BETWEEN 1 AND 1048576)`, *in.CoverPhotoID, id, owner).Scan(&valid); err != nil {
				return Publication{}, err
			}
			if !valid {
				return Publication{}, ErrInvalid
			}
		}
	}
	var profile spaces.Profile
	if in.Attributes != nil {
		var raw []byte
		var currentVersion int
		err = tx.QueryRow(ctx, `SELECT c.categoria_codigo,c.perfil_version,p.perfil FROM public.espacio_caracteristicas c JOIN public.categoria_perfil_atributos p ON p.categoria_codigo=c.categoria_codigo AND p.version=c.perfil_version WHERE c.espacio_id=$1 FOR UPDATE OF c`, id).Scan(&profile.CategoryCode, &currentVersion, &raw)
		if errors.Is(err, pgx.ErrNoRows) {
			return Publication{}, ErrConflict
		}
		if err != nil {
			return Publication{}, err
		}
		if json.Unmarshal(raw, &profile) != nil || in.AttributeSchemaVersion == nil || *in.AttributeSchemaVersion != currentVersion || profile.ValidateAttributes(in.Attributes) != nil {
			return Publication{}, ErrInvalid
		}
	}
	at := s.now().UTC()
	if price != nil && *price != oldPrice {
		var version int32
		if err = tx.QueryRow(ctx, `SELECT COALESCE(max(version),0)+1 FROM public.tarifa_espacio WHERE espacio_id=$1`, id).Scan(&version); err != nil {
			return Publication{}, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,$2,$3,$4)`, id, version, unit, *price); err != nil {
			return Publication{}, err
		}
	}
	if in.Attributes != nil {
		if _, err = tx.Exec(ctx, `UPDATE public.espacio_caracteristicas SET valores=$2::jsonb,actualizado_en=$3 WHERE espacio_id=$1 AND perfil_version=$4`, id, in.Attributes, at, *in.AttributeSchemaVersion); err != nil {
			return Publication{}, err
		}
	}
	var cover any
	if in.CoverPhotoID != nil && *in.CoverPhotoID != "" {
		cover = *in.CoverPhotoID
	}
	_, err = tx.Exec(ctx, `UPDATE public.espacio SET titulo=COALESCE($3,titulo),descripcion=COALESCE($4,descripcion),capacidad_maxima=COALESCE($5,capacidad_maxima),reglas_uso=COALESCE($6,reglas_uso),precio_base_clp=COALESCE($7,precio_base_clp),ubicacion_catalogo_version=CASE WHEN $8 THEN CASE WHEN $9='' THEN NULL ELSE 'SUBDERE_DPA_2018' END ELSE ubicacion_catalogo_version END,region_codigo=CASE WHEN $8 THEN NULLIF($9,'') ELSE region_codigo END,comuna_codigo=CASE WHEN $8 THEN NULLIF($10,'') ELSE comuna_codigo END,portada_foto_id=CASE WHEN $11 THEN $12::uuid ELSE portada_foto_id END,actualizado_en=$13 WHERE id=$1 AND propietario_id=$2 AND actualizado_en=$14`, id, owner, in.Title, in.Description, in.Capacity, in.UsageRules, price, in.RegionCode != nil, coalesceString(in.RegionCode), coalesceString(in.CommuneCode), in.CoverPhotoID != nil, cover, at, in.ExpectedUpdatedAt.UTC())
	if err != nil {
		return Publication{}, err
	}
	if err = s.audit(ctx, tx, actor, id, "admin.global.publications.update", "exito", in.Reason, correlation, at); err != nil {
		return Publication{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Publication{}, err
	}
	return Publication{ID: id, OwnerID: owner, UpdatedAt: at}, nil
}
func coalesceString(v *string) string {
	if v == nil {
		return ""
	}
	return strings.TrimSpace(*v)
}

func (s *Service) RetirePublication(ctx context.Context, actor, id, owner, correlation string, expectedUpdatedAt time.Time, reason string) (spaces.Deletion, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || actor == owner || !canonicalUUID.MatchString(correlation) || expectedUpdatedAt.IsZero() || !retirementReasons[reason] {
		return spaces.Deletion{}, ErrInvalid
	}
	if s.publicationRetirer == nil {
		return spaces.Deletion{}, ErrConflict
	}
	var actualOwner string
	if err := s.pool.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, id).Scan(&actualOwner); errors.Is(err, pgx.ErrNoRows) {
		return spaces.Deletion{}, ErrNotFound
	} else if err != nil {
		return spaces.Deletion{}, err
	}
	if actualOwner != owner {
		return spaces.Deletion{}, ErrConflict
	}
	return s.publicationRetirer.DeleteAsAdministrator(ctx, actor, owner, id, correlation, expectedUpdatedAt.UTC(), reason)
}

func (s *Service) PublicationSchedule(ctx context.Context, actor, id, owner, correlation string) (booking.WeeklyHours, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || !canonicalUUID.MatchString(correlation) || actor == owner || s.weeklyHoursManager == nil {
		return booking.WeeklyHours{}, ErrInvalid
	}
	var actualOwner string
	if err := s.pool.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, id).Scan(&actualOwner); errors.Is(err, pgx.ErrNoRows) {
		return booking.WeeklyHours{}, ErrNotFound
	} else if err != nil {
		return booking.WeeklyHours{}, err
	}
	if actualOwner != owner {
		return booking.WeeklyHours{}, ErrConflict
	}
	return s.weeklyHoursManager.WeeklyHoursAsAdministrator(ctx, actor, owner, id, correlation)
}

func (s *Service) SavePublicationSchedule(ctx context.Context, actor, id, owner, correlation string, expectedUpdatedAt time.Time, reason string, value booking.WeeklyHours) (booking.WeeklyHours, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || !canonicalUUID.MatchString(correlation) || expectedUpdatedAt.IsZero() || !accountReasons[reason] || s.weeklyHoursManager == nil {
		return booking.WeeklyHours{}, ErrInvalid
	}
	return s.weeklyHoursManager.SaveWeeklyHoursAsAdministrator(ctx, actor, owner, id, value, expectedUpdatedAt.UTC(), correlation, reason)
}

func (s *Service) PublicationGallery(ctx context.Context, actor, id, owner, correlation string) ([]gallery.Photo, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || !canonicalUUID.MatchString(correlation) || s.publicationGallery == nil {
		return nil, ErrInvalid
	}
	var actualOwner string
	if err := s.pool.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, id).Scan(&actualOwner); errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, err
	}
	if actualOwner != owner {
		return nil, ErrConflict
	}
	return s.publicationGallery.ListAsAdministrator(ctx, actor, owner, id, correlation)
}

func (s *Service) SetPublicationPhotoVisibility(ctx context.Context, actor, id, owner, photoID, correlation string, enabled bool, expected time.Time, reason string) (gallery.Photo, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || !canonicalUUID.MatchString(photoID) || !canonicalUUID.MatchString(correlation) || expected.IsZero() || !accountReasons[reason] || s.publicationGallery == nil {
		return gallery.Photo{}, ErrInvalid
	}
	return s.publicationGallery.SetPublicInCatalogAsAdministrator(ctx, actor, owner, id, photoID, enabled, expected.UTC(), correlation, reason)
}

func (s *Service) SelectPublicationCover(ctx context.Context, actor, id, owner, photoID, correlation string, expected time.Time, reason string) (gallery.Cover, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || !canonicalUUID.MatchString(photoID) || !canonicalUUID.MatchString(correlation) || expected.IsZero() || !accountReasons[reason] || s.publicationGallery == nil {
		return gallery.Cover{}, ErrInvalid
	}
	return s.publicationGallery.SelectCoverAsAdministrator(ctx, actor, owner, id, photoID, expected.UTC(), correlation, reason)
}

func (s *Service) RemovePublicationPhoto(ctx context.Context, actor, id, owner, photoID, correlation string, expected time.Time, reason string) (gallery.Removal, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(owner) || !canonicalUUID.MatchString(photoID) || !canonicalUUID.MatchString(correlation) || expected.IsZero() || !retirementReasons[reason] || s.publicationGallery == nil {
		return gallery.Removal{}, ErrInvalid
	}
	return s.publicationGallery.RemoveAsAdministrator(ctx, actor, owner, id, photoID, expected.UTC(), correlation, reason)
}

func (s *Service) SetPublicationState(ctx context.Context, actor, id, correlation string, in PublicationStateInput) (PublicationStateChange, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(id) || !canonicalUUID.MatchString(correlation) || in.ExpectedUpdatedAt.IsZero() || !accountReasons[in.Reason] || (in.State != "activa" && in.State != "oculta") {
		return PublicationStateChange{}, ErrInvalid
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
	if err != nil {
		return PublicationStateChange{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var owner string
	if err = tx.QueryRow(ctx, `SELECT propietario_id::text FROM public.espacio WHERE id=$1`, id).Scan(&owner); errors.Is(err, pgx.ErrNoRows) {
		return PublicationStateChange{}, ErrNotFound
	} else if err != nil {
		return PublicationStateChange{}, err
	}
	if actor == owner { return PublicationStateChange{}, ErrNotFound }
	if err = lockManagedAccounts(ctx, tx, actor, owner); err != nil {
		return PublicationStateChange{}, err
	}
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return PublicationStateChange{}, err
	}
	var current string
	var updated time.Time
	if err = tx.QueryRow(ctx, `SELECT estado,actualizado_en FROM public.espacio WHERE id=$1 AND propietario_id=$2 FOR UPDATE`, id, owner).Scan(&current, &updated); err != nil {
		return PublicationStateChange{}, ErrNotFound
	}
	if !updated.Equal(in.ExpectedUpdatedAt.UTC()) {
		return PublicationStateChange{}, ErrConflict
	}
	var fixture bool
	err = tx.QueryRow(ctx, `SELECT habilitada FROM public.reserva_ensayo_local_fixture WHERE espacio_id=$1 FOR UPDATE`, id).Scan(&fixture)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return PublicationStateChange{}, err
	}
	if fixture {
		return PublicationStateChange{}, ErrConflict
	}
	if in.State == "activa" {
		if current != "borrador" && current != "oculta" {
			return PublicationStateChange{}, ErrConflict
		}
		var eligible bool
		if err = tx.QueryRow(ctx, `SELECT u.estado='activo' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id) AND EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local e JOIN public.verificacion v ON v.id=e.verificacion_id AND v.usuario_id=e.usuario_id AND v.tipo=e.tipo AND v.estado='aprobada' WHERE e.usuario_id=u.id AND e.tipo='kyc' AND e.estado='elegible') FROM public.usuario u WHERE u.id=$1`, owner).Scan(&eligible); err != nil {
			return PublicationStateChange{}, err
		}
		if !eligible {
			return PublicationStateChange{}, ErrConflict
		}
	} else if current != "activa" {
		return PublicationStateChange{}, ErrConflict
	}
	at := s.now().UTC()
	if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_publicacion_historial_local(espacio_id,actor_id,estado_anterior,estado_nuevo,ocurrida_en,correlacion_id) VALUES($1,$2,$3,$4,$5,$6)`, id, actor, current, in.State, at, correlation); err != nil {
		return PublicationStateChange{}, err
	}
	tag, err := tx.Exec(ctx, `UPDATE public.espacio SET estado=$2,actualizado_en=$3 WHERE id=$1 AND actualizado_en=$4`, id, in.State, at, in.ExpectedUpdatedAt.UTC())
	if err != nil {
		return PublicationStateChange{}, err
	}
	if tag.RowsAffected() != 1 {
		return PublicationStateChange{}, ErrConflict
	}
	if err = s.audit(ctx, tx, actor, id, "admin.global.publications.state", "exito", in.Reason, correlation, at); err != nil {
		return PublicationStateChange{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PublicationStateChange{}, err
	}
	return PublicationStateChange{PublicationID: id, OwnerID: owner, State: in.State, ChangedAt: at}, nil
}

type publicationCursor struct {
	Version    int       `json:"v"`
	FilterHash string    `json:"f"`
	Snapshot   time.Time `json:"s"`
	UpdatedAt  time.Time `json:"u"`
	ID         string    `json:"i"`
}

func (s *Service) ListPublications(ctx context.Context, actor, correlation string, f PublicationFilter) (PublicationPage, error) {
	if !canonicalUUID.MatchString(actor) || !canonicalUUID.MatchString(correlation) {
		return PublicationPage{}, ErrInvalid
	}
	if f.PageSize == 0 {
		f.PageSize = 25
	}
	if f.PageSize < 1 || f.PageSize > 50 || len(f.Cursor) > 2048 || len(f.Search) > 120 || len(f.Category) > 80 || len(f.State) > 20 {
		return PublicationPage{}, ErrInvalid
	}
	f.Search = strings.TrimSpace(f.Search)
	if f.OwnerID != "" && !canonicalUUID.MatchString(f.OwnerID) {
		return PublicationPage{}, ErrInvalid
	}
	if f.State != "" && f.State != "borrador" && f.State != "activa" && f.State != "oculta" && f.State != "eliminada" {
		return PublicationPage{}, ErrInvalid
	}
	var exists bool
	if f.Category != "" {
		if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.categoria_espacio WHERE codigo=$1)`, f.Category).Scan(&exists); err != nil {
			return PublicationPage{}, err
		}
		if !exists {
			return PublicationPage{}, ErrInvalid
		}
	}
	hashInput := f
	hashInput.Cursor = ""
	data, _ := json.Marshal(hashInput)
	digest := sha256.Sum256(data)
	filterHash := hex.EncodeToString(digest[:])
	now := s.now().UTC()
	cursor := publicationCursor{Version: 1, FilterHash: filterHash, Snapshot: now}
	if f.Cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(f.Cursor)
		if err != nil || len(raw) > 1024 {
			return PublicationPage{}, ErrInvalid
		}
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if dec.Decode(&cursor) != nil || cursor.Version != 1 || cursor.FilterHash != filterHash || cursor.Snapshot.IsZero() || cursor.UpdatedAt.IsZero() || !canonicalUUID.MatchString(cursor.ID) {
			return PublicationPage{}, ErrInvalid
		}
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadWrite})
	if err != nil {
		return PublicationPage{}, err
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if err = requireGlobalActor(ctx, tx, actor); err != nil {
		return PublicationPage{}, err
	}
	args := []any{cursor.Snapshot}
	where := []string{"e.actualizado_en <= $1"}
	add := func(v any) string { args = append(args, v); return fmt.Sprintf("$%d", len(args)) }
	if f.OwnerID != "" {
		where = append(where, "e.propietario_id="+add(f.OwnerID))
	}
	if f.Category != "" {
		where = append(where, "e.categoria_codigo="+add(f.Category))
	}
	if f.State != "" {
		where = append(where, "e.estado="+add(f.State))
	}
	if f.Search != "" {
		v := add("%" + escapeLikePrefix(strings.ToLower(f.Search)) + "%")
		where = append(where, "(lower(e.titulo) LIKE "+v+" ESCAPE E'\\\\' OR lower(u.correo_normalizado) LIKE "+v+" ESCAPE E'\\\\')")
	}
	visibleExpr := `(e.estado='activa' AND c.activa AND u.estado='activo' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local bl WHERE bl.cuenta_id=u.id) AND EXISTS(SELECT 1 FROM public.tarifa_espacio tr WHERE tr.espacio_id=e.id) AND EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local el JOIN public.verificacion v ON v.id=el.verificacion_id AND v.usuario_id=el.usuario_id AND v.tipo=el.tipo AND v.estado='aprobada' WHERE el.usuario_id=u.id AND el.tipo='kyc' AND el.estado='elegible') AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local_fixture fx WHERE fx.espacio_id=e.id AND fx.anfitrion_id=e.propietario_id AND fx.habilitada))`
	if f.Visible != nil {
		where = append(where, visibleExpr+"="+add(*f.Visible))
	}
	filterSQL := strings.Join(where, " AND ")
	countSQL := `SELECT count(*) FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo JOIN public.usuario u ON u.id=e.propietario_id WHERE ` + filterSQL
	var total int64
	if err = tx.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return PublicationPage{}, err
	}
	pageArgs := append([]any(nil), args...)
	pageWhere := filterSQL
	if f.Cursor != "" {
		pageArgs = append(pageArgs, cursor.UpdatedAt.UTC(), cursor.ID)
		pageWhere += fmt.Sprintf(" AND (e.actualizado_en,e.id)<($%d::timestamptz,$%d::uuid)", len(pageArgs)-1, len(pageArgs))
	}
	pageArgs = append(pageArgs, f.PageSize+1)
	limit := len(pageArgs)
	query := fmt.Sprintf(`SELECT e.id::text,e.propietario_id::text,p.nombre_visible,u.correo_original,e.categoria_codigo,c.nombre,e.titulo,e.descripcion,e.superficie_m2::text,e.capacidad_maxima,e.reglas_uso,e.direccion,e.estado,COALESCE(rt.modalidad,e.modalidad_tarifa),COALESCE(rt.precio_base_clp,e.precio_base_clp)::text,e.actualizado_en,%s,
	EXISTS(SELECT 1 FROM public.espacio_galeria_sintetica_local g WHERE g.espacio_id=e.id AND g.propietario_id=e.propietario_id AND g.id=e.portada_foto_id AND g.estado='activa' AND g.publicada_en_catalogo AND g.archivo_id IS NOT NULL AND g.mime_type='image/png' AND g.size_bytes BETWEEN 1 AND 1048576)
	,COALESCE(ec.perfil_version,0),COALESCE(ec.valores,'{}'::jsonb),COALESCE(cp.perfil,'{}'::jsonb)
	FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo JOIN public.usuario u ON u.id=e.propietario_id LEFT JOIN public.perfil_usuario p ON p.usuario_id=u.id LEFT JOIN public.espacio_caracteristicas ec ON ec.espacio_id=e.id LEFT JOIN public.categoria_perfil_atributos cp ON cp.categoria_codigo=ec.categoria_codigo AND cp.version=ec.perfil_version LEFT JOIN LATERAL(SELECT modalidad,precio_base_clp FROM public.tarifa_espacio WHERE espacio_id=e.id ORDER BY version DESC LIMIT 1)rt ON true WHERE %s ORDER BY e.actualizado_en DESC,e.id DESC LIMIT $%d`, visibleExpr, pageWhere, limit)
	rows, err := tx.Query(ctx, query, pageArgs...)
	if err != nil {
		return PublicationPage{}, err
	}
	out := PublicationPage{Items: []Publication{}, MatchingTotal: total, GeneratedAt: now}
	for rows.Next() {
		var p Publication
		var attributeValues, schemaRaw []byte
		if err = rows.Scan(&p.ID, &p.OwnerID, &p.OwnerName, &p.OwnerEmail, &p.Category, &p.CategoryName, &p.Title, &p.Description, &p.AreaM2, &p.Capacity, &p.UsageRules, &p.Address, &p.State, &p.RateUnit, &p.BasePriceCLP, &p.UpdatedAt, &p.PubliclyVisible, &p.PublicCoverAvailable, &p.AttributeSchemaVersion, &attributeValues, &schemaRaw); err != nil {
			rows.Close()
			return PublicationPage{}, err
		}
		p.Attributes = map[string]any{}
		if err = json.Unmarshal(attributeValues, &p.Attributes); err != nil {
			rows.Close()
			return PublicationPage{}, err
		}
		if err = json.Unmarshal(schemaRaw, &p.AttributeSchema); err != nil {
			rows.Close()
			return PublicationPage{}, err
		}
		out.Items = append(out.Items, p)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return PublicationPage{}, err
	}
	rows.Close()
	if len(out.Items) > f.PageSize {
		last := out.Items[f.PageSize-1]
		out.Items = out.Items[:f.PageSize]
		encoded, e := json.Marshal(publicationCursor{Version: 1, FilterHash: filterHash, Snapshot: cursor.Snapshot, UpdatedAt: last.UpdatedAt.UTC(), ID: last.ID})
		if e != nil {
			return PublicationPage{}, e
		}
		out.NextCursor = base64.RawURLEncoding.EncodeToString(encoded)
	}
	if err = s.audit(ctx, tx, actor, collectionID, "admin.global.publications.list", "exito", "consulta_administrativa", correlation, now); err != nil {
		return PublicationPage{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PublicationPage{}, err
	}
	return out, nil
}
