package gallerypg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/gallery"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type deleteFaultStore struct {
	*evidencefs.Store
	mu        sync.Mutex
	fail      int
	lastPutID string
}

func (s *deleteFaultStore) Put(ctx context.Context, id string, blob []byte) error {
	if err := s.Store.Put(ctx, id, blob); err != nil {
		return err
	}
	s.mu.Lock()
	s.lastPutID = id
	s.mu.Unlock()
	return nil
}

func (s *deleteFaultStore) failNextDelete() {
	s.mu.Lock()
	s.fail++
	s.mu.Unlock()
}

func (s *deleteFaultStore) lastPut() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPutID
}

func (s *deleteFaultStore) Delete(ctx context.Context, id string) error {
	s.mu.Lock()
	if s.fail > 0 {
		s.fail--
		s.mu.Unlock()
		return errors.New("synthetic one-time delete failure")
	}
	s.mu.Unlock()
	return s.Store.Delete(ctx, id)
}

func TestSyntheticGalleryOwnerLimitIdempotencyAndRecoverableCleanup(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is required for disposable PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	db := fmt.Sprintf("gallery_test_%d", time.Now().UnixNano())
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{db}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP DATABASE "+pgx.Identifier{db}.Sanitize()+" WITH (FORCE)")
		_, _ = admin.Exec(context.Background(), `DROP ROLE IF EXISTS espacigo_runtime`)
	})
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + db
	if _, err = migrator.Run(ctx, u.String(), "../../../../db/migrations"); err != nil {
		t.Fatal(err)
	}
	adminPool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer adminPool.Close()
	if _, err = adminPool.Exec(ctx, `DO $$ BEGIN CREATE ROLE espacigo_runtime LOGIN PASSWORD 'runtime-test-only'; EXCEPTION WHEN duplicate_object THEN ALTER ROLE espacigo_runtime LOGIN PASSWORD 'runtime-test-only'; END $$;`); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	if err = dbbootstrap.GrantRuntimePermissions(ctx, bootstrap); err != nil {
		t.Fatal(err)
	}
	_ = bootstrap.Close(ctx)
	runtimeURL := *u
	runtimeURL.User = url.UserPassword("espacigo_runtime", "runtime-test-only")
	pool, err := pgxpool.New(ctx, runtimeURL.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	owner, other := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	for i, id := range []string{owner, other} {
		email := fmt.Sprintf("gallery-%d@example.test", i)
		if _, err = adminPool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, id, email); err != nil {
			t.Fatal(err)
		}
	}
	spaceA, spaceB := "cccccccc-cccc-4ccc-8ccc-cccccccccccc", "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	for _, entry := range []struct{ id, owner string }{{spaceA, owner}, {spaceB, other}} {
		_, err = adminPool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
		VALUES($1,$2,'oficina','Gallery space',repeat('Synthetic gallery acceptance description. ',4),20,2,'Synthetic rules','hora',8000,'Synthetic private address','borrador','UTC')`, entry.id, entry.owner)
		if err != nil {
			t.Fatal(err)
		}
	}
	root, err := os.MkdirTemp("", "espacigo-gallery-private-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if err = os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	base, err := evidencefs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	files := &deleteFaultStore{Store: base}
	svc, err := gallery.NewService(New(pool), files, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	first, reused, err := svc.AddSynthetic(ctx, owner, spaceA, "gallery-key-0001")
	if err != nil || reused {
		t.Fatalf("first add reused=%v err=%v", reused, err)
	}
	retry, reused, err := svc.AddSynthetic(ctx, owner, spaceA, "gallery-key-0001")
	if err != nil || !reused || retry.ID != first.ID {
		t.Fatalf("idempotent add=%+v reused=%v err=%v", retry, reused, err)
	}
	retryCandidate := files.lastPut()
	files.failNextDelete()
	if err = svc.CleanRetiredOnce(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err = base.Get(ctx, retryCandidate); err != nil {
		t.Fatalf("failed deletion after idempotent retry should preserve candidate file: %v", err)
	}
	var candidateState string
	if err = adminPool.QueryRow(ctx, `SELECT estado FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id=$1`, retryCandidate).Scan(&candidateState); err != nil || candidateState != "pendiente_limpieza" {
		t.Fatalf("retry candidate cleanup state=%q err=%v", candidateState, err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local SET proximo_intento_en=clock_timestamp() WHERE archivo_id=$1`, retryCandidate); err != nil {
		t.Fatal(err)
	}
	restarted, err := gallery.NewService(New(pool), files, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	if err = restarted.CleanRetiredOnce(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err = base.Get(ctx, retryCandidate); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("idempotent retry candidate was not removed after recovery: %v", err)
	}
	metadata, err := svc.Metadata(ctx, owner, spaceA, strings.ToUpper(first.ID))
	if err != nil || metadata.ID != first.ID {
		t.Fatalf("uppercase UUID metadata=%+v err=%v", metadata, err)
	}
	_, blob, err := svc.Content(ctx, owner, spaceA, first.ID)
	if err != nil || len(blob) < 8 || !strings.HasPrefix(string(blob[:8]), "\x89PNG\r\n\x1a\n") {
		t.Fatalf("own PNG content unavailable: len=%d err=%v", len(blob), err)
	}
	if _, err = svc.List(ctx, other, spaceA); !errors.Is(err, gallery.ErrNotFound) {
		t.Fatalf("foreign owner list error=%v", err)
	}
	if _, _, err = svc.Content(ctx, other, spaceA, first.ID); !errors.Is(err, gallery.ErrNotFound) {
		t.Fatalf("foreign owner content error=%v", err)
	}
	for i := 2; i <= gallery.MaxPhotosPerSpace; i++ {
		key := fmt.Sprintf("gallery-key-%04d", i)
		if _, _, err = svc.AddSynthetic(ctx, owner, spaceA, key); err != nil {
			t.Fatalf("add %d: %v", i, err)
		}
	}
	items, err := svc.List(ctx, owner, spaceA)
	if err != nil || len(items) != gallery.MaxPhotosPerSpace {
		t.Fatalf("gallery items=%d err=%v", len(items), err)
	}
	files.failNextDelete()
	if _, _, err = svc.AddSynthetic(ctx, owner, spaceA, "gallery-key-0011"); !errors.Is(err, gallery.ErrLimit) {
		t.Fatalf("gallery limit error=%v", err)
	}
	rejectedCandidate := files.lastPut()
	var pendingCandidates int
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id=$1 AND estado='pendiente_limpieza'`, rejectedCandidate).Scan(&pendingCandidates); err != nil || pendingCandidates != 1 {
		t.Fatalf("rejected candidate cleanup count=%d err=%v", pendingCandidates, err)
	}
	if err = svc.CleanRetiredOnce(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err = base.Get(ctx, rejectedCandidate); err != nil {
		t.Fatalf("failed deletion after rejected add should remain retryable: %v", err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio_galeria_archivo_candidato_local SET proximo_intento_en=clock_timestamp() WHERE archivo_id=$1`, rejectedCandidate); err != nil {
		t.Fatal(err)
	}
	if err = restarted.CleanRetiredOnce(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err = base.Get(ctx, rejectedCandidate); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rejected candidate was not removed after recovery: %v", err)
	}
	removed, err := svc.Remove(ctx, owner, spaceA, first.ID)
	if err != nil || !removed.Removed || removed.Reused {
		t.Fatalf("remove=%+v err=%v", removed, err)
	}
	removed, err = svc.Remove(ctx, owner, spaceA, first.ID)
	if err != nil || !removed.Reused {
		t.Fatalf("repeat remove=%+v err=%v", removed, err)
	}
	files.fail = 1
	if err = svc.CleanRetiredOnce(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err = base.Get(ctx, first.ID); err != nil {
		t.Fatalf("failed deletion should preserve recoverable file: %v", err)
	}
	if _, err = adminPool.Exec(ctx, `UPDATE public.espacio_galeria_sintetica_local SET proximo_intento_en=clock_timestamp() WHERE id=$1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if err = svc.CleanRetiredOnce(ctx, 50); err != nil {
		t.Fatal(err)
	}
	if _, err = base.Get(ctx, first.ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("gallery file not removed after retry: %v", err)
	}
	var active int
	if err = adminPool.QueryRow(ctx, `SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE espacio_id=$1 AND estado='activa'`, spaceA).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != gallery.MaxPhotosPerSpace-1 {
		t.Fatalf("active count after remove=%d", active)
	}
	// At nine active images, two concurrent uploads must serialize on the space
	// row; exactly one may fill slot ten.
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			<-start
			_, _, e := svc.AddSynthetic(ctx, owner, spaceA, fmt.Sprintf("gallery-race-%04d", i))
			errs <- e
		}(i)
	}
	close(start)
	successes := 0
	limits := 0
	for i := 0; i < 2; i++ {
		e := <-errs
		if e == nil {
			successes++
		} else if errors.Is(e, gallery.ErrLimit) {
			limits++
		} else {
			t.Fatalf("concurrent gallery add: %v", e)
		}
	}
	if successes != 1 || limits != 1 {
		t.Fatalf("concurrent limit result success=%d limit=%d", successes, limits)
	}
	if _, err = svc.List(ctx, other, spaceB); err != nil {
		t.Fatalf("owner cannot query own empty gallery: %v", err)
	}
}
