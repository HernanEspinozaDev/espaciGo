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

	clamavadapter "github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
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
	if cfg.ConnConfig.Host != "127.0.0.1" || cfg.ConnConfig.Port != 15434 || cfg.ConnConfig.Database != "espacigo_f5_gate_20261101" {
		t.Fatal("refusing integration target outside the authorized F5 synthetic endpoint")
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
	var database string
	if err := pool.QueryRow(ctx, `SELECT current_database()`).Scan(&database); err != nil || database != "espacigo_f5_gate_20261101" {
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
	actor, reviewer := uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		if err := outer.Rollback(context.Background()); err != nil {
			t.Errorf("rollback upload fixture failed: %v", err)
		}
		var remaining int
		if err := pool.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM public.usuario WHERE id IN($1,$2))+(SELECT count(*) FROM public.document_verification_case WHERE subject_id=$1)+(SELECT count(*) FROM public.document_verification_file f JOIN public.document_verification_case c ON c.id=f.case_id WHERE c.subject_id=$1)+(SELECT count(*) FROM public.document_verification_job j JOIN public.document_verification_file f ON f.id=j.file_id JOIN public.document_verification_case c ON c.id=f.case_id WHERE c.subject_id=$1)+(SELECT count(*) FROM public.document_verification_event WHERE subject_id=$1)+(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE reviewer_id=$2 OR scope_id=$1 OR granted_by=$1)+(SELECT count(*) FROM public.document_verification_reviewer_grant_event WHERE reviewer_id=$2 AND scope_id=$1)`, actor, reviewer).Scan(&remaining); err != nil {
			t.Errorf("verify upload fixture rollback failed: %v", err)
		} else if remaining != 0 {
			t.Errorf("upload fixture rollback left %d rows", remaining)
		}
	})
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, actor, actor+"@example.invalid"); err != nil {
		t.Fatal(err)
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic','activo')`, reviewer, reviewer+"@example.invalid"); err != nil {
		t.Fatal("unable to seed synthetic reviewer")
	}
	if _, err := outer.Exec(ctx, `INSERT INTO public.document_verification_reviewer_grant(reviewer_id,capability,scope_kind,scope_id,granted_by,granted_reason_code) VALUES($1,'review_identity','identity_subject',$2,$2,'role_assignment')`, reviewer, actor); err != nil {
		t.Fatal("unable to seed rollback-scoped reviewer capability")
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

	t.Run("real ClamAV worker persists clean and EICAR outcomes", func(t *testing.T) {
		address := os.Getenv("ESPACIGO_CLAMAV_TEST_ADDRESS")
		if address == "" {
			t.Skip("set ESPACIGO_CLAMAV_TEST_ADDRESS for the authorized local F5 scanner")
		}
		if address != "172.25.0.2:3310" {
			t.Fatal("refusing scanner outside the authorized local F5 endpoint")
		}

		// Hold the shared scan queue exclusively for this rollback-scoped fixture.
		lockF5JobQueueForTest(t, ctx, outer, "scan", receipt.File.ID)
		if _, err := outer.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
			t.Fatal("unable to restore restricted runtime role after queue isolation")
		}
		scanner, err := clamavadapter.New(clamavadapter.Config{
			Address: address, Timeout: 5 * time.Second,
			MaxBytes: dv.MaxUploadBytes, MaxSignatureAge: 48 * time.Hour,
		})
		if err != nil {
			t.Fatal("unable to configure the authorized synthetic ClamAV scanner")
		}
		jobs := &scanWorkerClaimRecorder{scanProcessorJobs: repo}
		processor := NewScanProcessor(jobs, store, scanner, 2*time.Minute, 30*time.Second, 48*time.Hour)
		if processor == nil {
			t.Fatal("unable to create scan processor")
		}
		// The rollback fixture uses one pgx transaction; avoid the monitor's
		// background query racing that connection while this fast local scan runs.
		processor.claimPollInterval = 10 * time.Second
		if err := processor.ProcessOne(ctx); err != nil {
			t.Fatalf("process uploaded clean document with real ClamAV: %v", err)
		}
		if len(jobs.claims) != 1 || jobs.claims[0].FileID != receipt.File.ID {
			t.Fatalf("scan worker claimed unexpected fixture: %+v", jobs.claims)
		}
		var state, errorCode, scannerVersion, jobState, jobError string
		var signaturesAt time.Time
		var inspected, retentionSet, leaseCleared bool
		if err := outer.QueryRow(ctx, `SELECT f.state,COALESCE(f.last_error_code,''),COALESCE(f.scanner_version,''),f.signatures_at,f.inspected_at IS NOT NULL,f.retention_at IS NOT NULL,j.state,j.lease_until IS NULL,COALESCE(j.last_error_code,'') FROM public.document_verification_file f JOIN public.document_verification_job j ON j.file_id=f.id AND j.kind='scan' WHERE f.id=$1`, receipt.File.ID).Scan(&state, &errorCode, &scannerVersion, &signaturesAt, &inspected, &retentionSet, &jobState, &leaseCleared, &jobError); err != nil {
			t.Fatal("real clean scan outcome was not readable")
		}
		if state != string(dv.FileClean) || errorCode != "" || scannerVersion == "" || signaturesAt.IsZero() || !inspected || retentionSet || jobState != "completado" || !leaseCleared || jobError != "" {
			t.Fatalf("unexpected persisted clean scan: state=%q error=%q scanner=%q inspected=%t retention=%t job=%q lease_cleared=%t job_error=%q", state, errorCode, scannerVersion, inspected, retentionSet, jobState, leaseCleared, jobError)
		}
		ownerCase, err := repo.LoadOwnCase(ctx, actor, created.ID)
		if err != nil {
			t.Fatal("unable to load clean case for submission")
		}
		if err := cases.Submit(ctx, actor, created.ID, dv.PurposeIdentity, ownerCase.Revision); err != nil {
			t.Fatalf("submit clean identity case: %v", err)
		}
		submittedCase, err := repo.LoadOwnCase(ctx, actor, created.ID)
		if err != nil || submittedCase.State != dv.CaseSubmitted {
			t.Fatalf("submitted case state=%q err=%v", submittedCase.State, err)
		}
		if err := cases.Review(ctx, reviewer, created.ID, dv.CapabilityIdentity, dv.DecisionApprove, time.Time{}, submittedCase.Revision); err != nil {
			t.Fatalf("approve persisted identity case with scoped reviewer grant: %v", err)
		}
		approvedCase, err := repo.LoadOwnCase(ctx, actor, created.ID)
		if err != nil || approvedCase.State != dv.CaseApproved || approvedCase.VerifiedAt.IsZero() || approvedCase.VerifiedUntil.IsZero() {
			t.Fatalf("approved case state/verification=%q/%t/%t err=%v", approvedCase.State, !approvedCase.VerifiedAt.IsZero(), !approvedCase.VerifiedUntil.IsZero(), err)
		}
		if err := cases.Revoke(ctx, reviewer, created.ID, dv.PurposeIdentity, dv.ReasonOwnerRequest, approvedCase.Revision); err != nil {
			t.Fatalf("revoke persisted identity case: %v", err)
		}
		revokedCase, err := repo.LoadOwnCase(ctx, actor, created.ID)
		if err != nil || revokedCase.State != dv.CaseRevoked || revokedCase.RevokedAt == nil {
			t.Fatalf("revoked case state/timestamp=%q/%t err=%v", revokedCase.State, revokedCase.RevokedAt != nil, err)
		}
		var submittedEvents, approvedEvents, revokedEvents int
		if err := outer.QueryRow(ctx, `SELECT count(*) FILTER(WHERE action='case_transitioned' AND result_code='enviada'),count(*) FILTER(WHERE action='review_decision' AND result_code='aprobada'),count(*) FILTER(WHERE action='revoked' AND result_code='revocada' AND reason_code='solicitud_titular') FROM public.document_verification_event WHERE case_id=$1`, created.ID).Scan(&submittedEvents, &approvedEvents, &revokedEvents); err != nil {
			t.Fatal("unable to verify persisted submit/review/revoke audit events")
		}
		if submittedEvents != 1 || approvedEvents != 1 || revokedEvents != 1 {
			t.Fatalf("submit/review/revoke audit counts=%d/%d/%d", submittedEvents, approvedEvents, revokedEvents)
		}

		// Seed EICAR at the persisted-worker boundary, not through upload inspection:
		// this specifically proves real scanner→DB behavior for a blocked result.
		const eicarTestSignature = `X5O!P%@AP[4\PZX54(P^)7CC)7}$EICAR-STANDARD-ANTIVIRUS-TEST-FILE!$H+H*`
		infectedContent := []byte(eicarTestSignature)
		malwareCase, err := cases.Start(ctx, actor, dv.CreateInput{Purpose: dv.PurposeIdentity, DocumentExpiresAt: time.Now().UTC().AddDate(1, 0, 0)})
		if err != nil {
			t.Fatal("unable to create separate synthetic malware case")
		}
		infectedFileID := uuid.NewString()
		if err := store.PutIdentityDocument(ctx, actor, malwareCase.ID, infectedFileID, infectedContent); err != nil {
			t.Fatal("unable to store encrypted synthetic EICAR fixture")
		}
		malwareSnapshot, err := repo.LoadOwnCase(ctx, actor, malwareCase.ID)
		if err != nil {
			t.Fatal("unable to reload malware case before adding EICAR worker fixture")
		}
		infectedFile := dv.File{ID: infectedFileID, State: dv.FilePending, Size: int64(len(infectedContent)), MIME: "image/png", ContentSHA256: sha256.Sum256(infectedContent)}
		if err := cases.AddFile(ctx, actor, malwareCase.ID, dv.PurposeIdentity, malwareSnapshot.Revision, infectedFile); err != nil {
			t.Fatal("unable to persist synthetic EICAR scan fixture")
		}
		if err := processor.ProcessOne(ctx); err != nil {
			t.Fatalf("process EICAR fixture with real ClamAV: %v", err)
		}
		if len(jobs.claims) != 2 || jobs.claims[1].FileID != infectedFileID {
			t.Fatalf("scan worker did not claim the EICAR fixture: %+v", jobs.claims)
		}
		var blocked, malwareRetentionScheduled bool
		var infectedState, infectedError, infectedReason, infectedJobState string
		var infectedScanner string
		var infectedLeaseCleared bool
		if err := outer.QueryRow(ctx, `SELECT f.state,f.last_error_code,COALESCE(f.retention_reason_code,''),COALESCE(f.scanner_version,''),f.inspected_at IS NOT NULL,f.retention_at>=clock_timestamp()+interval '23 hours' AND f.retention_at<=clock_timestamp()+interval '25 hours',j.state,j.lease_until IS NULL FROM public.document_verification_file f JOIN public.document_verification_job j ON j.file_id=f.id AND j.kind='scan' WHERE f.id=$1`, infectedFileID).Scan(&infectedState, &infectedError, &infectedReason, &infectedScanner, &blocked, &malwareRetentionScheduled, &infectedJobState, &infectedLeaseCleared); err != nil {
			t.Fatal("real EICAR scan outcome was not readable")
		}
		if infectedState != string(dv.FileBlocked) || infectedError != "malware_detected" || infectedReason != "malicious_file" || infectedScanner == "" || !blocked || !malwareRetentionScheduled || infectedJobState != "completado" || !infectedLeaseCleared {
			t.Fatalf("unexpected persisted EICAR scan: state=%q error=%q reason=%q scanner=%q inspected=%t retention_24h=%t job=%q lease_cleared=%t", infectedState, infectedError, infectedReason, infectedScanner, blocked, malwareRetentionScheduled, infectedJobState, infectedLeaseCleared)
		}
		var retentionState, retentionError string
		if err := outer.QueryRow(ctx, `SELECT state,COALESCE(last_error_code,'') FROM public.document_verification_job WHERE file_id=$1 AND kind='retention_delete'`, infectedFileID).Scan(&retentionState, &retentionError); err != nil {
			t.Fatal("EICAR retention job was not persisted")
		}
		if retentionState != "pendiente" || retentionError != "" {
			t.Fatalf("unexpected EICAR retention job: state=%q error=%q", retentionState, retentionError)
		}
	})
}

type scanWorkerClaimRecorder struct {
	scanProcessorJobs
	claims []ScanClaim
}

func (r *scanWorkerClaimRecorder) ClaimReadyScanJob(ctx context.Context, lease time.Duration) (*ScanClaim, error) {
	claim, err := r.scanProcessorJobs.ClaimReadyScanJob(ctx, lease)
	if claim != nil {
		r.claims = append(r.claims, *claim)
	}
	return claim, err
}
