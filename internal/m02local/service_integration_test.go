package m02local

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/accountlock"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/ownerexport"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func m02TestDB(t *testing.T) (context.Context, *pgxpool.Pool, string) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL required; disposable PostgreSQL only")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	t.Cleanup(cancel)
	admin, e := pgx.Connect(ctx, adminURL)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(context.Background())
	db := fmt.Sprintf("m02_local_%d", time.Now().UnixNano())
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{db}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		cleanupConn, openErr := pgx.Connect(context.Background(), adminURL)
		if openErr != nil {
			t.Errorf("open cleanup connection: %v", openErr)
			return
		}
		defer cleanupConn.Close(context.Background())
		_, e := cleanupConn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{db}.Sanitize())
		if e != nil {
			t.Errorf("drop disposable database: %v", e)
		}
	})
	u, e := url.Parse(adminURL)
	if e != nil {
		t.Fatal(e)
	}
	u.Path = "/" + db
	if _, e = migrator.Run(ctx, u.String(), filepath.Join("..", "..", "db", "migrations")); e != nil {
		t.Fatalf("apply migrations: %v", e)
	}
	pool, e := pgxpool.New(ctx, u.String())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	return ctx, pool, u.String()
}

func runtimePoolForM02(t *testing.T, ctx context.Context, databaseURL string) *pgxpool.Pool {
	t.Helper()
	admin, e := pgx.Connect(ctx, databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = admin.Exec(ctx, `DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT; END IF; END $$`); e != nil {
		_ = admin.Close(ctx)
		t.Fatal(e)
	}
	if e = dbbootstrap.GrantRuntimePermissions(ctx, admin); e != nil {
		_ = admin.Close(ctx)
		t.Fatalf("grant runtime permissions: %v", e)
	}
	_ = admin.Close(ctx)
	cfg, e := pgxpool.ParseConfig(databaseURL)
	if e != nil {
		t.Fatal(e)
	}
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		_, e := c.Exec(ctx, "SET ROLE espacigo_runtime")
		return e
	}
	pool, e := pgxpool.NewWithConfig(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestSyntheticPhotoPayoutOwnershipIdempotencyAndEligibility(t *testing.T) {
	ctx, adminPool, databaseURL := m02TestDB(t)
	pool := runtimePoolForM02(t, ctx, databaseURL)
	a := "11111111-1111-4111-8111-111111111111"
	b := "22222222-2222-4222-8222-222222222222"
	admin := "33333333-3333-4333-8333-333333333333"
	caseID := "44444444-4444-4444-8444-444444444444"
	for _, id := range []string{a, b, admin} {
		if _, e := adminPool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'not-for-export-hash','activo')`, id, id+"@test.invalid"); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := adminPool.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,resuelta_en) VALUES($1,$2,'kyc','aprobada','fixture:55555555-5555-4555-8555-555555555555','approval-key',$3,now())`, caseID, a, admin); e != nil {
		t.Fatal(e)
	}
	if _, e := adminPool.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en) VALUES($1,'kyc',$2,'elegible',now())`, a, caseID); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	if e := os.Chmod(dir, 0o700); e != nil {
		t.Fatal(e)
	}
	store, e := evidencefs.New(dir)
	if e != nil {
		t.Fatal(e)
	}
	svc, e := New(pool, store, time.Now)
	if e != nil {
		t.Fatal(e)
	}
	photo, reused, e := svc.SetPhoto(ctx, a, "photo-idem-key-1")
	if e != nil || reused {
		t.Fatalf("set synthetic photo reused=%v err=%v", reused, e)
	}
	same, reused, e := svc.SetPhoto(ctx, a, "photo-idem-key-1")
	if e != nil || !reused || same.ID != photo.ID {
		t.Fatalf("photo retry mismatch: %#v reused=%v err=%v", same, reused, e)
	}
	if _, e = svc.RemovePhoto(ctx, a, "photo-idem-key-1"); !errors.Is(e, ErrConflict) {
		t.Fatalf("cross-action photo idempotency reuse: %v", e)
	}
	priorPhotoID := photo.ID
	photo, reused, e = svc.SetPhoto(ctx, a, "photo-replace-key-2")
	if e != nil || reused || photo.ID == priorPhotoID {
		t.Fatalf("photo replacement reused=%v id=%s err=%v", reused, photo.ID, e)
	}
	if e = svc.CleanRetiredPhotos(ctx, 10); e != nil {
		t.Fatal(e)
	}
	if _, e = store.Get(ctx, priorPhotoID); e == nil {
		t.Fatal("replaced private photo was not cleaned")
	}
	if _, _, e = svc.PhotoContent(ctx, b); !errors.Is(e, ErrNotFound) {
		t.Fatalf("other account photo access: %v", e)
	}
	got, content, e := svc.PhotoContent(ctx, a)
	if e != nil || got.ID != photo.ID || len(content) == 0 {
		t.Fatalf("own photo unavailable: %v", e)
	}
	payout, reused, e := svc.SetPayout(ctx, a, "payout-idem-key-1")
	if e != nil || reused {
		t.Fatalf("set fake payout reused=%v err=%v", reused, e)
	}
	again, reused, e := svc.SetPayout(ctx, a, "payout-idem-key-1")
	if e != nil || !reused || again.ID != payout.ID {
		t.Fatalf("payout retry mismatch: %#v reused=%v err=%v", again, reused, e)
	}
	if _, e = svc.RevokePayout(ctx, a, "payout-idem-key-1"); !errors.Is(e, ErrConflict) {
		t.Fatalf("cross-action payout idempotency reuse: %v", e)
	}
	payout, reused, e = svc.SetPayout(ctx, a, "payout-change-key-2")
	if e != nil || reused || payout.ID == again.ID {
		t.Fatalf("payout change reused=%v id=%s err=%v", reused, payout.ID, e)
	}
	privacySvc, e := privacy.NewService(identitypg.NewIdentityRepository(pool), ownerexport.New(pool, store))
	if e != nil {
		t.Fatal(e)
	}
	archive, e := privacySvc.ExportOwnArchive(ctx, a)
	if e != nil {
		t.Fatalf("export own ZIP: %v", e)
	}
	zipReader, e := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if e != nil {
		t.Fatal(e)
	}
	archiveFiles := map[string][]byte{}
	for _, f := range zipReader.File {
		rc, e := f.Open()
		if e != nil {
			t.Fatal(e)
		}
		b, e := io.ReadAll(rc)
		_ = rc.Close()
		if e != nil {
			t.Fatal(e)
		}
		archiveFiles[f.Name] = b
	}
	if _, ok := archiveFiles["files/profile/photo-"+photo.ID+".png"]; !ok {
		t.Fatalf("ZIP lacks own photo fixture; entries=%v", mapKeys(archiveFiles))
	}
	if !bytes.HasPrefix(archiveFiles["files/profile/photo-"+photo.ID+".png"], []byte("\x89PNG\r\n\x1a\n")) {
		t.Fatal("ZIP photo is not a PNG")
	}
	if !bytes.Contains(archiveFiles["data.json"], []byte(payout.Reference)) || bytes.Contains(archiveFiles["data.json"], []byte("not-for-export-hash")) || bytes.Contains(archiveFiles["data.json"], []byte(b+"@test.invalid")) {
		t.Fatal("ZIP does not include fake payout reference or includes credential hash")
	}
	if _, _, e = svc.SetPayout(ctx, b, "payout-idem-key-2"); !errors.Is(e, ErrIneligible) {
		t.Fatalf("payout without KYC: %v", e)
	}
	if _, e = svc.RevokePayout(ctx, a, "payout-revoke-key"); e != nil {
		t.Fatal(e)
	}
	if _, e = svc.CurrentPayout(ctx, a); !errors.Is(e, ErrNotFound) {
		t.Fatalf("revoked payout remains active: %v", e)
	}
	if _, e = svc.RemovePhoto(ctx, a, "photo-remove-key"); e != nil {
		t.Fatal(e)
	}
	if e = svc.CleanRetiredPhotos(ctx, 10); e != nil {
		t.Fatal(e)
	}
	if _, e = store.Get(ctx, photo.ID); e == nil {
		t.Fatal("removed photo file still exists")
	}
}

func mapKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestPhotoWriteLosesRaceWithSuppressionAccountLock(t *testing.T) {
	ctx, adminPool, databaseURL := m02TestDB(t)
	pool := runtimePoolForM02(t, ctx, databaseURL)
	owner := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	if _, e := adminPool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,'race@test.invalid','race@test.invalid','test','activo')`, owner); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	_ = os.Chmod(dir, 0o700)
	store, e := evidencefs.New(dir)
	if e != nil {
		t.Fatal(e)
	}
	svc, _ := New(pool, store, time.Now)
	tx, e := pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	active, e := accountlock.LockActive(ctx, tx, owner)
	if e != nil || !active {
		t.Fatalf("lock account: active=%v err=%v", active, e)
	}
	results := make(chan error, 2)
	go func() { _, _, err := svc.SetPhoto(ctx, owner, "race-photo-key-1"); results <- err }()
	go func() { _, _, err := svc.SetPayout(ctx, owner, "race-payout-key-1"); results <- err }()
	deadline := time.Now().Add(3 * time.Second)
	for {
		var waiting int
		if e = adminPool.QueryRow(ctx, `SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid() AND wait_event_type='Lock' AND query LIKE '%SELECT estado FROM public.usuario WHERE id=$1 FOR UPDATE%'`).Scan(&waiting); e != nil {
			t.Fatal(e)
		}
		if waiting == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected both M02 writes to wait on account lock; waiting=%d", waiting)
		}
		time.Sleep(5 * time.Millisecond)
	}
	if _, e = tx.Exec(ctx, `UPDATE public.usuario SET estado='desidentificado' WHERE id=$1`, owner); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 2; i++ {
		if e = <-results; !errors.Is(e, identity.ErrForbidden) {
			t.Fatalf("expected suppression race rejection, got %v", e)
		}
	}
	var photos, payouts int
	if e = adminPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.foto_perfil_sintetica_local WHERE usuario_id=$1),(SELECT count(*) FROM public.cuenta_cobro_sintetica_local WHERE usuario_id=$1)`, owner).Scan(&photos, &payouts); e != nil || photos != 0 || payouts != 0 {
		t.Fatalf("resources inserted after suppression: photos=%d payout=%d err=%v", photos, payouts, e)
	}
}
