package reputation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"regexp"
	"sort"
	"strings"
	"time"

	domain "github.com/HernanEspinozaDev/espaciGo/internal/reputation"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

var publicEmailPattern = regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`)
var publicUUIDPattern = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}\b`)
var publicRUTPattern = regexp.MustCompile(`(?i)\b\d{1,2}(?:\.\d{3}){2}-[\dk]\b`)
var publicPhonePattern = regexp.MustCompile(`(?:\+?56[ .-]?)?\b9[ .-]?\d{4}[ .-]?\d{4}\b`)

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) CreateReview(ctx context.Context, id, actor, reservation string, in domain.ReviewInput, key string, hash []byte, at time.Time) (domain.Review, error) {
	tx, e := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return domain.Review{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var host, renter, space, state string
	var linksRetireAt *time.Time
	e = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text,espacio_id::text,estado,vinculos_retirar_en FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&host, &renter, &space, &state, &linksRetireAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Review{}, domain.ErrNotFound
	}
	if e != nil {
		return domain.Review{}, e
	}
	targetType, targetID := "", ""
	if actor == renter {
		targetType = "espacio"
	} else if actor == host {
		targetType, targetID = "arrendatario", renter
	} else {
		return domain.Review{}, domain.ErrNotFound
	}
	if e = lockActiveAccounts(ctx, tx, host, renter); e != nil {
		return domain.Review{}, e
	}
	var old domain.Review
	var oldHash []byte
	var oldComment string
	var oldTarget string
	var oldState string
	e = tx.QueryRow(ctx, `SELECT id::text,puntuacion,comentario,creada_en,huella_solicitud,destinatario_tipo,estado FROM public.resena_ensayo_local WHERE reserva_id=$1 AND autor_id=$2`, reservation, actor).Scan(&old.ID, &old.Rating, &oldComment, &old.CreatedAt, &oldHash, &oldTarget, &oldState)
	if e == nil {
		if string(oldHash) != string(hash) {
			return domain.Review{}, domain.ErrConflict
		}
		old.Comment = oldComment
		old.TargetType = oldTarget
		old.State = oldState
		old.Reused = true
		return old, tx.Commit(ctx)
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return domain.Review{}, e
	}
	if state != "finalizada" {
		return domain.Review{}, domain.ErrConflict
	}
	if linksRetireAt != nil && !linksRetireAt.After(at) {
		return domain.Review{}, domain.ErrConflict
	}
	marker := reviewAuthorshipMarker(reservation, actor)
	var marked bool
	if e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.resena_autoria_marca_local WHERE huella_autoria=$1 AND retirar_en>$2)`, marker[:], at).Scan(&marked); e != nil {
		return domain.Review{}, e
	}
	if marked {
		return domain.Review{}, domain.ErrConflict
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.resena_ensayo_local(id,reserva_id,espacio_id,autor_id,destinatario_tipo,destinatario_id,puntuacion,comentario,creada_en,retirar_en,huella_solicitud,clave_idempotencia) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::uuid,$7,$8,$9,$9::timestamptz+interval '24 months',$10,$11)`, id, reservation, space, actor, targetType, targetID, in.Rating, in.Comment, at, hash, key)
	if e != nil {
		return domain.Review{}, mapReviewError(e)
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.resena_autoria_marca_local(huella_autoria,creada_en,retirar_en) VALUES($1,$2,GREATEST($2::timestamptz+interval '24 months',COALESCE($3,$2::timestamptz+interval '24 months')))`, marker[:], at, linksRetireAt)
	if e != nil {
		return domain.Review{}, mapReviewError(e)
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.Review{}, e
	}
	return domain.Review{ID: id, TargetType: targetType, Rating: in.Rating, Comment: in.Comment, State: "publicada", CreatedAt: at}, nil
}

func (r *Repository) ListSpaceReviews(ctx context.Context, space string) (domain.SpaceReviews, error) {
	var out domain.SpaceReviews
	out.Items = []domain.Review{}
	e := r.pool.QueryRow(ctx, `SELECT count(*),COALESCE(avg(puntuacion),0) FROM public.resena_ensayo_local r JOIN public.espacio s ON s.id=r.espacio_id WHERE r.espacio_id=$1 AND s.estado='activa' AND r.destinatario_tipo='espacio' AND r.estado IN ('publicada','reportada')`, space).Scan(&out.Count, &out.Average)
	if e != nil {
		return out, e
	}
	if out.Count == 0 {
		var active bool
		e = r.pool.QueryRow(ctx, `SELECT estado='activa' FROM public.espacio WHERE id=$1`, space).Scan(&active)
		if errors.Is(e, pgx.ErrNoRows) || !active {
			return out, domain.ErrNotFound
		}
		if e != nil {
			return out, e
		}
	}
	rows, e := r.pool.Query(ctx, `SELECT r.id::text,r.destinatario_tipo,r.puntuacion,r.comentario,r.estado,r.creada_en FROM public.resena_ensayo_local r JOIN public.espacio s ON s.id=r.espacio_id WHERE r.espacio_id=$1 AND s.estado='activa' AND r.destinatario_tipo='espacio' AND r.estado IN ('publicada','reportada') ORDER BY r.creada_en DESC,r.id`, space)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.Review
		if e = rows.Scan(&v.ID, &v.TargetType, &v.Rating, &v.Comment, &v.State, &v.CreatedAt); e != nil {
			return out, e
		}
		v.Comment = sanitizePublicComment(v.Comment)
		v.State = "" // Moderation/report status is participant/admin metadata, not public listing data.
		out.Items = append(out.Items, v)
	}
	return out, rows.Err()
}
func (r *Repository) ListReservationReviews(ctx context.Context, actor, reservation string) ([]domain.Review, error) {
	var member bool
	e := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local WHERE id=$1 AND $2::uuid IN(anfitrion_id,arrendatario_id))`, reservation, actor).Scan(&member)
	if e != nil {
		return nil, e
	}
	if !member {
		return nil, domain.ErrNotFound
	}
	rows, e := r.pool.Query(ctx, `SELECT id::text,destinatario_tipo,puntuacion,comentario,estado,creada_en FROM public.resena_ensayo_local WHERE reserva_id=$1 AND estado<>'oculta' ORDER BY creada_en,id`, reservation)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	items := []domain.Review{}
	for rows.Next() {
		var v domain.Review
		if e = rows.Scan(&v.ID, &v.TargetType, &v.Rating, &v.Comment, &v.State, &v.CreatedAt); e != nil {
			return nil, e
		}
		items = append(items, v)
	}
	return items, rows.Err()
}
func (r *Repository) Report(ctx context.Context, id, actor, reservationExpected, review, reason, key string, hash []byte, at time.Time) (domain.Report, error) {
	tx, e := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return domain.Report{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var reservation, space, host, state, targetType string
	e = tx.QueryRow(ctx, `SELECT rv.id::text,rv.espacio_id::text,rv.anfitrion_id::text,rv.estado,x.destinatario_tipo FROM public.resena_ensayo_local x JOIN public.reserva_ensayo_local rv ON rv.id=x.reserva_id WHERE x.id=$1`, review).Scan(&reservation, &space, &host, &state, &targetType)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Report{}, domain.ErrNotFound
	}
	if e != nil {
		return domain.Report{}, e
	}
	if reservation != reservationExpected || targetType != "espacio" {
		return domain.Report{}, domain.ErrNotFound
	}
	var lock string
	e = tx.QueryRow(ctx, `SELECT id::text FROM public.reserva_ensayo_local WHERE id=$1 FOR UPDATE`, reservation).Scan(&lock)
	if e != nil {
		return domain.Report{}, e
	}
	if e = lockActiveAccounts(ctx, tx, host); e != nil {
		return domain.Report{}, e
	}
	var reviewSpace, reviewState string
	e = tx.QueryRow(ctx, `SELECT espacio_id::text,estado FROM public.resena_ensayo_local WHERE id=$1 FOR UPDATE`, review).Scan(&reviewSpace, &reviewState)
	if e != nil {
		return domain.Report{}, e
	}
	if actor != host || reviewSpace != space {
		return domain.Report{}, domain.ErrNotFound
	}
	var old domain.Report
	var oldHash []byte
	var oldResolution pgtype.Text
	e = tx.QueryRow(ctx, `SELECT id::text,resena_id::text,espacio_id::text,motivo_codigo,estado,creada_en,resolver_en,motivo_resolucion_codigo,huella_solicitud FROM public.reporte_resena_ensayo_local WHERE resena_id=$1`, review).Scan(&old.ID, &old.ReviewID, &old.SpaceID, &old.Reason, &old.State, &old.CreatedAt, &old.ResolvedAt, &oldResolution, &oldHash)
	if e == nil {
		if oldResolution.Valid {
			old.ResolutionReason = oldResolution.String
		}
		if actor == host && reviewSpace == space && string(oldHash) == string(hash) {
			old.Reused = true
			return old, tx.Commit(ctx)
		}
		return domain.Report{}, domain.ErrConflict
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return domain.Report{}, e
	}
	if reviewState == "oculta" {
		return domain.Report{}, domain.ErrNotFound
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.reporte_resena_ensayo_local(id,resena_id,espacio_id,anfitrion_id,motivo_codigo,estado,creada_en,clave_idempotencia,huella_solicitud) VALUES($1,$2,$3,$4,$5,'pendiente',$6,$7,$8)`, id, review, space, actor, reason, at, key, hash)
	if e != nil {
		return domain.Report{}, mapReviewError(e)
	}
	_, e = tx.Exec(ctx, `UPDATE public.resena_ensayo_local SET estado='reportada' WHERE id=$1 AND estado='publicada'`, review)
	if e != nil {
		return domain.Report{}, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.reporte_resena_historial_local(reporte_id,estado_anterior,estado_nuevo,actor_id,motivo_codigo,ocurrida_en) VALUES($1,NULL,'pendiente',$2,$3,$4)`, id, actor, reason, at)
	if e != nil {
		return domain.Report{}, e
	}
	if e = tx.Commit(ctx); e != nil {
		return domain.Report{}, e
	}
	return domain.Report{ID: id, ReviewID: review, SpaceID: space, Reason: reason, State: "pendiente", CreatedAt: at}, nil
}
func (r *Repository) ListReports(ctx context.Context) ([]domain.Report, error) {
	rows, e := r.pool.Query(ctx, `SELECT q.id::text,q.resena_id::text,q.espacio_id::text,q.motivo_codigo,q.estado,q.creada_en,q.resolver_en,q.motivo_resolucion_codigo,x.comentario FROM public.reporte_resena_ensayo_local q JOIN public.resena_ensayo_local x ON x.id=q.resena_id WHERE q.estado='pendiente' ORDER BY q.creada_en,q.id`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []domain.Report{}
	for rows.Next() {
		var v domain.Report
		var resolution pgtype.Text
		if e = rows.Scan(&v.ID, &v.ReviewID, &v.SpaceID, &v.Reason, &v.State, &v.CreatedAt, &v.ResolvedAt, &resolution, &v.Comment); e != nil {
			return nil, e
		}
		if resolution.Valid {
			v.ResolutionReason = resolution.String
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (r *Repository) Moderate(ctx context.Context, id, admin, report, decision, reason, key, correlation string, at time.Time) (domain.Report, error) {
	tx, e := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if e != nil {
		return domain.Report{}, e
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var allowed bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.usuario u JOIN public.rol_usuario rr ON rr.usuario_id=u.id WHERE u.id=$1 AND u.estado='activo' AND rr.rol='administrador')`, admin).Scan(&allowed)
	if e != nil {
		return domain.Report{}, e
	}
	if !allowed {
		return domain.Report{}, domain.ErrNotFound
	}
	var reviewID, space, state string
	e = tx.QueryRow(ctx, `SELECT resena_id::text,espacio_id::text,estado FROM public.reporte_resena_ensayo_local WHERE id=$1 FOR UPDATE`, report).Scan(&reviewID, &space, &state)
	if errors.Is(e, pgx.ErrNoRows) {
		return domain.Report{}, domain.ErrNotFound
	}
	if e != nil {
		return domain.Report{}, e
	}
	var old domain.Report
	var oldResolution pgtype.Text
	e = tx.QueryRow(ctx, `SELECT id::text,resena_id::text,espacio_id::text,motivo_codigo,estado,creada_en,resolver_en,motivo_resolucion_codigo FROM public.reporte_resena_ensayo_local WHERE id=$1`, report).Scan(&old.ID, &old.ReviewID, &old.SpaceID, &old.Reason, &old.State, &old.CreatedAt, &old.ResolvedAt, &oldResolution)
	if e != nil {
		return old, e
	}
	if oldResolution.Valid {
		old.ResolutionReason = oldResolution.String
	}
	if state != "pendiente" {
		if old.State == mapDecision(decision) && old.ResolutionReason == reason {
			old.Reused = true
			return old, tx.Commit(ctx)
		}
		return old, domain.ErrConflict
	}
	next := "desestimada"
	reviewState := "publicada"
	if decision == "ocultar" {
		next = "oculta"
		reviewState = "oculta"
	}
	_, e = tx.Exec(ctx, `UPDATE public.reporte_resena_ensayo_local SET estado=$2,resolver_en=$3,resuelta_por=$4,motivo_resolucion_codigo=$5,retirar_en=$3::timestamptz+interval '24 months' WHERE id=$1`, report, next, at, admin, reason)
	if e != nil {
		return old, e
	}
	_, e = tx.Exec(ctx, `UPDATE public.resena_ensayo_local SET estado=$2 WHERE id=$1`, reviewID, reviewState)
	if e != nil {
		return old, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.reporte_resena_historial_local(reporte_id,estado_anterior,estado_nuevo,actor_id,motivo_codigo,ocurrida_en) VALUES($1,'pendiente',$2,$3,$4,$5)`, report, next, admin, reason, at)
	if e != nil {
		return old, e
	}
	encoded, _ := json.Marshal(map[string]any{"obligations_detected": []string{}, "pending_checks": []string{}})
	_, e = tx.Exec(ctx, `INSERT INTO public.evento_auditoria_local(id,actor_id,recurso_tipo,recurso_id,accion,resultado,motivo_codigo,correlacion_id,ocurrido_en,retirar_en,clave_idempotencia,detalle_codigos) VALUES($1,$2,'reporte_resena',$3,'reputation.review.moderate','exito',$4,$5,$6,$6::timestamptz+interval '5 years',$7,$8::jsonb)`, id, admin, report, reason, correlation, at, key, string(encoded))
	if e != nil {
		return old, e
	}
	if e = tx.Commit(ctx); e != nil {
		return old, e
	}
	old.State = next
	old.ResolvedAt = &at
	old.ResolutionReason = reason
	return old, nil
}
func mapDecision(s string) string {
	if s == "ocultar" {
		return "oculta"
	}
	return "desestimada"
}

func lockActiveAccounts(ctx context.Context, tx pgx.Tx, ids ...string) error {
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	last := ""
	for _, id := range ids {
		if id == last {
			continue
		}
		last = id
		var active bool
		err := tx.QueryRow(ctx, `SELECT estado='activo' FROM public.usuario WHERE id=$1 FOR UPDATE`, id).Scan(&active)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && !active) {
			return domain.ErrNotFound
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func reviewAuthorshipMarker(reservation, author string) [sha256.Size]byte {
	return sha256.Sum256([]byte(reservation + ":" + author))
}

func (r *Repository) MyRenterReputation(ctx context.Context, owner string) (domain.SpaceReviews, error) {
	var out domain.SpaceReviews
	out.Items = []domain.Review{}
	e := r.pool.QueryRow(ctx, `SELECT count(*),COALESCE(avg(puntuacion),0) FROM public.resena_ensayo_local WHERE destinatario_tipo='arrendatario' AND destinatario_id=$1 AND estado IN ('publicada','reportada')`, owner).Scan(&out.Count, &out.Average)
	if e != nil {
		return out, e
	}
	rows, e := r.pool.Query(ctx, `SELECT id::text,destinatario_tipo,puntuacion,comentario,creada_en FROM public.resena_ensayo_local WHERE destinatario_tipo='arrendatario' AND destinatario_id=$1 AND estado IN ('publicada','reportada') ORDER BY creada_en DESC,id`, owner)
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v domain.Review
		if e = rows.Scan(&v.ID, &v.TargetType, &v.Rating, &v.Comment, &v.CreatedAt); e != nil {
			return out, e
		}
		out.Items = append(out.Items, v)
	}
	return out, rows.Err()
}
func (r *Repository) PurgeExpired(ctx context.Context, at time.Time, limit int) (domain.RetentionCounts, error) {
	var out domain.RetentionCounts
	e := r.pool.QueryRow(ctx, `SELECT reviews_purged,reports_purged,notices_purged FROM public.purge_expired_local_communication($1,$2)`, at, limit).Scan(&out.ReviewsPurged, &out.ReportsPurged, &out.NoticesPurged)
	if e != nil {
		return domain.RetentionCounts{}, e
	}
	return out, nil
}
func mapReviewError(err error) error {
	var pgerr *pgconn.PgError
	if errors.As(err, &pgerr) && pgerr.Code == "23505" {
		return domain.ErrConflict
	}
	return err
}

func sanitizePublicComment(s string) string {
	s = publicEmailPattern.ReplaceAllString(s, "[correo omitido]")
	s = publicUUIDPattern.ReplaceAllString(s, "[identificador omitido]")
	s = publicRUTPattern.ReplaceAllString(s, "[identificador omitido]")
	s = publicPhonePattern.ReplaceAllString(s, "[teléfono omitido]")
	return strings.TrimSpace(s)
}
