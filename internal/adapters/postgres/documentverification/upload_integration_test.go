package documentverificationpg

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/documentfs"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification/inspection"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This opt-in test exercises the upload service, encrypted filesystem adapter,
// PostgreSQL repository, scan-job trigger, audit event, and runtime grants in
// one rollback-scoped synthetic transaction. It never applies migrations.
func TestPostgresUploadPersistsEncryptedObjectAndPendingScanJob(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL is unset")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal("invalid integration DSN")
	}
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15433 || cfg.ConnConfig.Database != "espacigo_f5_synth_20261029" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	var database string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil || database != "espacigo_f5_synth_20261029" {
		t.Fatalf("refusing unexpected database %q (query error: %v)", database, err)
	}
	var setupRole string
	if err := pool.QueryRow(ctx, `SELECT current_user`).Scan(&setupRole); err != nil {
		t.Fatal(err)
	}
	if setupRole == "espacigo_runtime" {
		t.Fatal("refusing to seed synthetic fixtures through the runtime role")
	}
	outer, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal(err)
	}
	actor := uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Errorf("rollback upload fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id=$1)+(SELECT count(*) FROM public.document_verification_case WHERE subject_id=$1)+(SELECT count(*) FROM public.document_verification_file f JOIN public.document_verification_case c ON c.id=f.case_id WHERE c.subject_id=$1)+(SELECT count(*) FROM public.document_verification_job j JOIN public.document_verification_file f ON f.id=j.file_id JOIN public.document_verification_case c ON c.id=f.case_id WHERE c.subject_id=$1)+(SELECT count(*) FROM public.document_verification_event WHERE subject_id=$1)`, actor).Scan(&remaining); err != nil {
			t.Errorf("verify upload fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("upload fixture rollback left %d rows", remaining)
		}
	})
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatalf("cannot enter the non-superuser runtime role: %v", err)
	}
	var role string
	var superuser, bypassRLS bool
	if err := outer.QueryRow(ctx, `SELECT current_user,r.rolsuper,r.rolbypassrls FROM pg_catalog.pg_roles r WHERE r.rolname=current_user`).Scan(&role, &superuser, &bypassRLS); err != nil {
		t.Fatal(err)
	}
	if role != "espacigo_runtime" || superuser || bypassRLS {
		t.Fatalf("upload repository is not running with the restricted runtime role: role=%q superuser=%t bypassRLS=%t", role, superuser, bypassRLS)
	}

	repo := newWithTransactionBeginner(savepointBeginner{tx: outer})
	cases, err := dv.NewService(repo)
	if err != nil {
		t.Fatal(err)
	}
	created, err := cases.Start(ctx, actor, dv.CreateInput{Purpose: dv.PurposeIdentity, DocumentExpiresAt: time.Now().UTC().AddDate(1, 0, 0)})
	if err != nil {
		t.Fatalf("create synthetic identity case as runtime: %v", err)
	}

	inspector := inspection.New(inspection.Config{PDFInfoPath: "/usr/bin/pdfinfo", PDFDetachPath: "/usr/bin/pdfdetach"})
	if inspector == nil {
		t.Fatal("trusted Poppler executables are required for the document inspector")
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	storageRoot := filepath.Join(t.TempDir(), "private-documents")
	store, err := documentfs.New(storageRoot, key)
	if err != nil {
		t.Fatal(err)
	}
	uploader, err := dv.NewUploadService(cases, inspector, store)
	if err != nil {
		t.Fatal(err)
	}
	var raw bytes.Buffer
	if err := png.Encode(&raw, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	receipt, err := uploader.Upload(ctx, actor, created.ID, created.Revision, raw.Bytes())
	if err != nil {
		t.Fatalf("upload through runtime repository failed: %v", err)
	}
	if receipt.File.State != dv.FilePending || receipt.Revision != created.Revision+1 {
		t.Fatalf("unexpected upload receipt: %+v", receipt)
	}
	loaded, err := repo.LoadOwnCase(ctx, actor, created.ID)
	if err != nil || loaded.Revision != receipt.Revision || len(loaded.Files) != 1 || loaded.Files[0] != receipt.File {
		t.Fatalf("persisted owner case=%+v, err=%v", loaded, err)
	}
	content, err := store.ReadIdentityDocument(ctx, actor, created.ID, receipt.File.ID)
	if err != nil || int64(len(content)) != receipt.File.Size || sha256.Sum256(content) != receipt.File.ContentSHA256 {
		t.Fatalf("rehydrated private object does not match persisted metadata (size=%d, err=%v)", len(content), err)
	}
	objectPath := filepath.Join(storageRoot, "users", actor, "identity", created.ID, receipt.File.ID+".bin")
	sealed, err := os.ReadFile(objectPath)
	if err != nil || bytes.Equal(sealed, content) || !bytes.HasPrefix(sealed, []byte("ESPACIGODOC1")) {
		t.Fatalf("private object is not stored in the encrypted envelope: err=%v", err)
	}
	var pendingScans, fileAddedEvents int
	if err := outer.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.document_verification_job WHERE file_id=$1 AND kind='scan' AND state='pendiente'),(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND file_id=$1 AND actor_id=$3 AND action='file_added' AND result_code='pendiente')`, receipt.File.ID, created.ID, actor).Scan(&pendingScans, &fileAddedEvents); err != nil {
		t.Fatal(err)
	}
	if pendingScans != 1 || fileAddedEvents != 1 {
		t.Fatalf("upload side effects: pending scan jobs=%d file-added events=%d", pendingScans, fileAddedEvents)
	}
}
