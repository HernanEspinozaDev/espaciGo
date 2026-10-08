// Package m02local contains local-only M02 resources. The payout destination is
// a fake adapter reference; no bank or provider credentials are accepted.
package m02local

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrConflict = errors.New("m02 local: conflict")
var ErrNotFound = errors.New("m02 local: not found")
var ErrIneligible = errors.New("m02 local: synthetic KYC approval required")

type FileStore interface {
	Put(context.Context, string, []byte) error
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}
type Photo struct {
	ID        string    `json:"id"`
	Fixture   string    `json:"fixture_code"`
	MIME      string    `json:"mime_type"`
	SHA256    string    `json:"sha256"`
	Size      int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}
type Payout struct {
	ID        string    `json:"id"`
	Adapter   string    `json:"adapter"`
	Reference string    `json:"reference"`
	State     string    `json:"state"`
	UpdatedAt time.Time `json:"updated_at"`
}
type Service struct {
	pool  *pgxpool.Pool
	files FileStore
	now   func() time.Time
}

func operationHash(operation string) string {
	sum := sha256.Sum256([]byte(operation))
	return hex.EncodeToString(sum[:])
}

func New(pool *pgxpool.Pool, files FileStore, now func() time.Time) (*Service, error) {
	if pool == nil || files == nil || now == nil {
		return nil, identity.ErrInvalid
	}
	return &Service{pool: pool, files: files, now: now}, nil
}

func (s *Service) CurrentPhoto(ctx context.Context, owner string) (Photo, error) {
	var p Photo
	err := s.pool.QueryRow(ctx, `SELECT photo.id::text,photo.fixture_code,photo.mime_type,photo.sha256,photo.size_bytes,photo.creada_en FROM public.foto_perfil_sintetica_local photo JOIN public.usuario owner ON owner.id=photo.usuario_id AND owner.estado='activo' WHERE photo.usuario_id=$1 AND photo.estado='activa'`, owner).Scan(&p.ID, &p.Fixture, &p.MIME, &p.SHA256, &p.Size, &p.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Photo{}, ErrNotFound
	}
	return p, err
}
func (s *Service) PhotoContent(ctx context.Context, owner string) (Photo, []byte, error) {
	p, e := s.CurrentPhoto(ctx, owner)
	if e != nil {
		return p, nil, e
	}
	b, e := s.files.Get(ctx, p.ID)
	return p, b, e
}

func (s *Service) SetPhoto(ctx context.Context, owner, key string) (Photo, bool, error) {
	if len(key) < 8 || len(key) > 128 {
		return Photo{}, false, identity.ErrInvalid
	}
	blob, err := verification.SyntheticPNG()
	if err != nil {
		return Photo{}, false, err
	}
	sum := sha256.Sum256(blob)
	hash := hex.EncodeToString(sum[:])
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		return Photo{}, false, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Photo{}, false, err
	}
	defer tx.Rollback(context.Background())
	active, err := accountlock.LockActive(ctx, tx, owner)
	if err != nil {
		return Photo{}, false, err
	}
	if !active {
		return Photo{}, false, identity.ErrForbidden
	}
	var raw []byte
	var priorHash string
	err = tx.QueryRow(ctx, `SELECT respuesta,solicitud_sha256 FROM public.m02_operacion_idempotente_local WHERE usuario_id=$1 AND recurso='foto_perfil' AND clave_idempotencia=$2`, owner, key).Scan(&raw, &priorHash)
	if err == nil {
		if priorHash != operationHash("PUT-profile-photo-v1") {
			return Photo{}, false, ErrConflict
		}
		var p Photo
		if e := json.Unmarshal(raw, &p); e != nil {
			return p, false, e
		}
		if err = tx.Commit(ctx); err != nil {
			return Photo{}, false, err
		}
		return p, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Photo{}, false, err
	}
	at := s.now().UTC().Truncate(time.Microsecond)
	if err = s.files.Put(ctx, id, blob); err != nil {
		return Photo{}, false, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = s.files.Delete(context.Background(), id)
		}
	}()
	if _, err = tx.Exec(ctx, `UPDATE public.foto_perfil_sintetica_local SET estado='reemplazada',retirada_en=$2,proximo_intento_en=$2 WHERE usuario_id=$1 AND estado='activa'`, owner, at); err != nil {
		return Photo{}, false, err
	}
	p := Photo{ID: id, Fixture: verification.SyntheticEvidenceFixture, MIME: "image/png", SHA256: hash, Size: int64(len(blob)), CreatedAt: at}
	if _, err = tx.Exec(ctx, `INSERT INTO public.foto_perfil_sintetica_local(id,usuario_id,archivo_id,mime_type,fixture_code,sha256,size_bytes,estado,creada_en,clave_idempotencia) VALUES($1,$2,$1,'image/png',$3,$4,$5,'activa',$6,$7)`, id, owner, p.Fixture, hash, len(blob), at, key); err != nil {
		return Photo{}, false, err
	}
	body, _ := json.Marshal(p)
	if _, err = tx.Exec(ctx, `INSERT INTO public.m02_operacion_idempotente_local(usuario_id,recurso,clave_idempotencia,solicitud_sha256,respuesta,creada_en) VALUES($1,'foto_perfil',$2,$3,$4::jsonb,$5)`, owner, key, operationHash("PUT-profile-photo-v1"), string(body), at); err != nil {
		return Photo{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Photo{}, false, err
	}
	cleanup = false
	return p, false, nil
}
func (s *Service) RemovePhoto(ctx context.Context, owner, key string) (bool, error) {
	if len(key) < 8 || len(key) > 128 {
		return false, identity.ErrInvalid
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return false, e
	}
	defer tx.Rollback(context.Background())
	active, e := accountlock.LockActive(ctx, tx, owner)
	if e != nil {
		return false, e
	}
	if !active {
		return false, identity.ErrForbidden
	}
	var priorHash string
	e = tx.QueryRow(ctx, `SELECT solicitud_sha256 FROM public.m02_operacion_idempotente_local WHERE usuario_id=$1 AND recurso='foto_perfil' AND clave_idempotencia=$2`, owner, key).Scan(&priorHash)
	if e == nil {
		if priorHash != operationHash("DELETE-profile-photo-v1") {
			return false, ErrConflict
		}
		if e = tx.Commit(ctx); e != nil {
			return false, e
		}
		return true, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return false, e
	}
	at := s.now().UTC()
	tag, e := tx.Exec(ctx, `UPDATE public.foto_perfil_sintetica_local SET estado='retirada',retirada_en=$2,proximo_intento_en=$2 WHERE usuario_id=$1 AND estado='activa'`, owner, at)
	if e != nil {
		return false, e
	}
	if tag.RowsAffected() == 0 {
		return false, ErrNotFound
	}
	resp := []byte(`{"removed":true}`)
	_, e = tx.Exec(ctx, `INSERT INTO public.m02_operacion_idempotente_local(usuario_id,recurso,clave_idempotencia,solicitud_sha256,respuesta,creada_en) VALUES($1,'foto_perfil',$2,$3,$4::jsonb,$5)`, owner, key, operationHash("DELETE-profile-photo-v1"), resp, at)
	if e != nil {
		return false, e
	}
	return false, tx.Commit(ctx)
}

func (s *Service) CurrentPayout(ctx context.Context, owner string) (Payout, error) {
	var p Payout
	e := s.pool.QueryRow(ctx, `SELECT payout.id::text,payout.adaptador,payout.referencia_ficticia,payout.estado,payout.actualizada_en FROM public.cuenta_cobro_sintetica_local payout JOIN public.usuario owner ON owner.id=payout.usuario_id AND owner.estado='activo' WHERE payout.usuario_id=$1 AND payout.estado='activa'`, owner).Scan(&p.ID, &p.Adapter, &p.Reference, &p.State, &p.UpdatedAt)
	if errors.Is(e, pgx.ErrNoRows) {
		return Payout{}, ErrNotFound
	}
	return p, e
}
func (s *Service) SetPayout(ctx context.Context, owner, key string) (Payout, bool, error) {
	if len(key) < 8 || len(key) > 128 {
		return Payout{}, false, identity.ErrInvalid
	}
	id, e := (credentials.Generator{}).ID()
	if e != nil {
		return Payout{}, false, e
	}
	refID, e := (credentials.Generator{}).ID()
	if e != nil {
		return Payout{}, false, e
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return Payout{}, false, e
	}
	defer tx.Rollback(context.Background())
	active, e := accountlock.LockActive(ctx, tx, owner)
	if e != nil {
		return Payout{}, false, e
	}
	if !active {
		return Payout{}, false, identity.ErrForbidden
	}
	var raw []byte
	var priorHash string
	e = tx.QueryRow(ctx, `SELECT respuesta,solicitud_sha256 FROM public.m02_operacion_idempotente_local WHERE usuario_id=$1 AND recurso='cuenta_cobro' AND clave_idempotencia=$2`, owner, key).Scan(&raw, &priorHash)
	if e == nil {
		if priorHash != operationHash("PUT-payout-account-v1") {
			return Payout{}, false, ErrConflict
		}
		var p Payout
		if e = json.Unmarshal(raw, &p); e != nil {
			return p, false, e
		}
		if e = tx.Commit(ctx); e != nil {
			return p, false, e
		}
		return p, true, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return Payout{}, false, e
	}
	var eligible bool
	e = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local e JOIN public.verificacion v ON v.id=e.verificacion_id AND v.usuario_id=e.usuario_id AND v.tipo=e.tipo JOIN public.usuario u ON u.id=e.usuario_id WHERE e.usuario_id=$1 AND e.tipo='kyc' AND e.estado='elegible' AND v.estado='aprobada' AND u.estado='activo')`, owner).Scan(&eligible)
	if e != nil {
		return Payout{}, false, e
	}
	if !eligible {
		return Payout{}, false, ErrIneligible
	}
	at := s.now().UTC().Truncate(time.Microsecond)
	updated, e := tx.Exec(ctx, `UPDATE public.cuenta_cobro_sintetica_local SET estado='reemplazada',actualizada_en=$2 WHERE usuario_id=$1 AND estado='activa'`, owner, at)
	if e != nil {
		return Payout{}, false, e
	}
	action := "creada"
	if updated.RowsAffected() > 0 {
		action = "actualizada"
	}
	p := Payout{ID: id, Adapter: "fake-local-v1", Reference: "demo_" + refID, State: "activa", UpdatedAt: at}
	if _, e = tx.Exec(ctx, `INSERT INTO public.cuenta_cobro_sintetica_local(id,usuario_id,adaptador,referencia_ficticia,estado,creada_en,actualizada_en) VALUES($1,$2,$3,$4,'activa',$5,$5)`, id, owner, p.Adapter, p.Reference, at); e != nil {
		return Payout{}, false, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.cuenta_cobro_sintetica_historial_local(cuenta_id,usuario_id,accion,ocurrida_en,correlacion_id,clave_idempotencia) VALUES($1,$2,$3,$4,$5,$6)`, id, owner, action, at, id, key)
	if e != nil {
		return Payout{}, false, e
	}
	b, _ := json.Marshal(p)
	_, e = tx.Exec(ctx, `INSERT INTO public.m02_operacion_idempotente_local(usuario_id,recurso,clave_idempotencia,solicitud_sha256,respuesta,creada_en) VALUES($1,'cuenta_cobro',$2,$3,$4::jsonb,$5)`, owner, key, operationHash("PUT-payout-account-v1"), string(b), at)
	if e != nil {
		return Payout{}, false, e
	}
	return p, false, tx.Commit(ctx)
}
func (s *Service) RevokePayout(ctx context.Context, owner, key string) (bool, error) {
	if len(key) < 8 || len(key) > 128 {
		return false, identity.ErrInvalid
	}
	tx, e := s.pool.Begin(ctx)
	if e != nil {
		return false, e
	}
	defer tx.Rollback(context.Background())
	active, e := accountlock.LockActive(ctx, tx, owner)
	if e != nil {
		return false, e
	}
	if !active {
		return false, identity.ErrForbidden
	}
	var priorHash string
	e = tx.QueryRow(ctx, `SELECT solicitud_sha256 FROM public.m02_operacion_idempotente_local WHERE usuario_id=$1 AND recurso='cuenta_cobro' AND clave_idempotencia=$2`, owner, key).Scan(&priorHash)
	if e == nil {
		if priorHash != operationHash("DELETE-payout-account-v1") {
			return false, ErrConflict
		}
		_ = tx.Commit(ctx)
		return true, nil
	}
	if !errors.Is(e, pgx.ErrNoRows) {
		return false, e
	}
	at := s.now().UTC()
	var id string
	e = tx.QueryRow(ctx, `UPDATE public.cuenta_cobro_sintetica_local SET estado='revocada',revocada_en=$2,actualizada_en=$2 WHERE usuario_id=$1 AND estado='activa' RETURNING id::text`, owner, at).Scan(&id)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if e != nil {
		return false, e
	}
	if _, e = tx.Exec(ctx, `INSERT INTO public.cuenta_cobro_sintetica_historial_local(cuenta_id,usuario_id,accion,ocurrida_en,correlacion_id,clave_idempotencia) VALUES($1,$2,'revocada',$3,$4,$5)`, id, owner, at, id, key); e != nil {
		return false, e
	}
	_, e = tx.Exec(ctx, `INSERT INTO public.m02_operacion_idempotente_local(usuario_id,recurso,clave_idempotencia,solicitud_sha256,respuesta,creada_en) VALUES($1,'cuenta_cobro',$2,$3,'{"revoked":true}'::jsonb,$4)`, owner, key, operationHash("DELETE-payout-account-v1"), at)
	if e != nil {
		return false, e
	}
	return false, tx.Commit(ctx)
}

// CleanRetiredPhotos is an idempotent, bounded worker. File deletion happens
// outside the DB transaction; a failed delete remains retryable.
func (s *Service) CleanRetiredPhotos(ctx context.Context, limit int) error {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	rows, e := s.pool.Query(ctx, `SELECT id::text FROM public.foto_perfil_sintetica_local WHERE estado<>'activa' AND archivo_id IS NOT NULL AND proximo_intento_en<=now() ORDER BY proximo_intento_en,id LIMIT $1`, limit)
	if e != nil {
		return e
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, id := range ids {
		if e = s.files.Delete(ctx, id); e != nil {
			_, _ = s.pool.Exec(ctx, `UPDATE public.foto_perfil_sintetica_local SET intentos_limpieza=intentos_limpieza+1,proximo_intento_en=now()+interval '30 seconds',ultimo_codigo_error='archivo_sintetico_no_eliminado' WHERE id=$1`, id)
			continue
		}
		_, e = s.pool.Exec(ctx, `UPDATE public.foto_perfil_sintetica_local SET archivo_id=NULL,mime_type=NULL,sha256=NULL,size_bytes=NULL,limpia_en=now(),proximo_intento_en=NULL,ultimo_codigo_error=NULL WHERE id=$1 AND estado<>'activa'`, id)
		if e != nil {
			return e
		}
	}
	return nil
}
