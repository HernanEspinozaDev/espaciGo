package verificationpg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func nowForMigration() time.Time { return time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC) }

func TestVerificationRepositoryOwnershipIdempotencyReviewAndRetry(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	database := fmt.Sprintf("kyc_repo_test_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, err := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize())
		if err != nil {
			t.Errorf("drop disposable database: %v", err)
		}
	}()
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	parsed.Path = "/" + database
	testURL := parsed.String()
	migrationRoot := "../../../../db/migrations"
	preV28 := t.TempDir()
	migrationFiles, err := os.ReadDir(migrationRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range migrationFiles {
		if file.Name() == "V000028__local_kyc_eligibility_and_history.sql" {
			continue
		}
		contents, readErr := os.ReadFile(filepath.Join(migrationRoot, file.Name()))
		if readErr != nil {
			t.Fatal(readErr)
		}
		if writeErr := os.WriteFile(filepath.Join(preV28, file.Name()), contents, 0o600); writeErr != nil {
			t.Fatal(writeErr)
		}
	}
	if _, err := migrator.Run(ctx, testURL, preV28); err != nil {
		t.Fatal(err)
	}
	migrationConn, err := pgx.Connect(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	defer migrationConn.Close(context.Background())
	preexistingOwner := "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
	if _, err := migrationConn.Exec(ctx, "INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-hash', 'desidentificado')", preexistingOwner, preexistingOwner+"@ejemplo.invalid"); err != nil {
		t.Fatal(err)
	}
	preexistingReviewer := "d3d3d3d3-d3d3-43d3-83d3-d3d3d3d3d3d3"
	legacyOwner := "d8d8d8d8-d8d8-48d8-88d8-d8d8d8d8d8d8"
	for _, id := range []string{preexistingReviewer, legacyOwner} {
		if _, err := migrationConn.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-hash','activo')`, id, id+"@ejemplo.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := migrationConn.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,proveedor_ref,referencia_evidencia,clave_idempotencia,motivo_codigo,creada_en,resuelta_en,retirada_privacidad_en)
VALUES('dededede-dede-4ede-8ede-dededededede',$1,'kyc','retirada_privacidad','local-fixture-v1','fixture:dfdfdfdf-dfdf-4fdf-8fdf-dfdfdfdfdfdf','privacy-case-0001','baja_privacidad',$2,$3,$4)`, preexistingOwner, nowForMigration(), nowForMigration(), nowForMigration()); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationConn.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,proveedor_ref,referencia_evidencia,clave_idempotencia,revisor_id,motivo_codigo,creada_en,resuelta_en)
VALUES('d4d4d4d4-d4d4-44d4-84d4-d4d4d4d4d4d4',$1,'kyc','rechazada','local-fixture-v1','fixture:d5d5d5d5-d5d5-45d5-85d5-d5d5d5d5d5d5','legacy-reject-0001',$2,'antecedentes_incompletos',$3,$4)`, legacyOwner, preexistingReviewer, nowForMigration(), nowForMigration()); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationConn.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,proveedor_ref,referencia_evidencia,clave_idempotencia,reintento_de,creada_en)
VALUES('d6d6d6d6-d6d6-46d6-86d6-d6d6d6d6d6d6',$1,'kyc','en_revision','local-fixture-v1','fixture:d7d7d7d7-d7d7-47d7-87d7-d7d7d7d7d7d7','legacy-retry-0001','d4d4d4d4-d4d4-44d4-84d4-d4d4d4d4d4d4',$2)`, legacyOwner, nowForMigration()); err != nil {
		t.Fatal(err)
	}
	if _, err := migrator.Run(ctx, testURL, migrationRoot); err != nil {
		t.Fatal(err)
	}
	if _, err := migrationConn.Exec(ctx, `CREATE ROLE espacigo_runtime NOLOGIN`); err != nil {
		t.Fatal(err)
	}
	if err := dbbootstrap.GrantRuntimePermissions(ctx, migrationConn); err != nil {
		t.Fatal(err)
	}
	var migratedAction string
	if err := migrationConn.QueryRow(ctx, `SELECT accion FROM public.verificacion_historial_local WHERE verificacion_id='dededede-dede-4ede-8ede-dededededede' ORDER BY id DESC LIMIT 1`).Scan(&migratedAction); err != nil || migratedAction != "estado_observado_en_migracion" {
		t.Fatalf("privacy verification history migration action=%q err=%v", migratedAction, err)
	}
	var legacyCorrection string
	if err := migrationConn.QueryRow(ctx, `SELECT correccion_codigo FROM public.verificacion WHERE id='d6d6d6d6-d6d6-46d6-86d6-d6d6d6d6d6d6'`).Scan(&legacyCorrection); err != nil || legacyCorrection != "legado_pre_v28_sin_codigo" {
		t.Fatalf("legacy retry correction=%q err=%v", legacyCorrection, err)
	}
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	owner := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	other := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	reviewer := "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	runtimeOwner := "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee"
	for _, id := range []string{owner, other, reviewer, runtimeOwner} {
		if _, err := pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-hash','activo')`, id, id+"@ejemplo.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	runtimeConfig, err := pgxpool.ParseConfig(testURL)
	if err != nil {
		t.Fatal(err)
	}
	runtimeConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, `SET ROLE espacigo_runtime`)
		return err
	}
	runtimePool, err := pgxpool.NewWithConfig(ctx, runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimePool.Close()
	runtimeRepo := New(runtimePool)
	runtimeCase, err := runtimeRepo.Create(ctx, verification.Case{
		ID: "d1d1d1d1-d1d1-41d1-81d1-d1d1d1d1d1d1", OwnerID: runtimeOwner, Type: "kyc", State: "en_revision", Provider: "local-fixture-v1",
		EvidenceRef: "fixture:d2d2d2d2-d2d2-42d2-82d2-d2d2d2d2d2d2", Idempotency: "runtime-case-0001", CreatedAt: nowForMigration(),
	})
	if err != nil {
		t.Fatalf("runtime role create/history insert: %v", err)
	}
	runtimeHistory, err := runtimeRepo.ListHistory(ctx, runtimeOwner, runtimeCase.ID)
	if err != nil || len(runtimeHistory) != 1 || runtimeHistory[0].Action != "solicitada" {
		t.Fatalf("runtime history=%+v err=%v", runtimeHistory, err)
	}
	if _, err = runtimeRepo.Review(ctx, runtimeCase.ID, reviewer, true, "", time.Now); err != nil {
		t.Fatalf("runtime role review/eligibility/audit write: %v", err)
	}
	runtimeEligibility, err := runtimeRepo.ListEligibility(ctx, runtimeOwner)
	if err != nil || len(runtimeEligibility) != 2 || !runtimeEligibility[1].Eligible {
		t.Fatalf("runtime eligibility=%+v err=%v", runtimeEligibility, err)
	}
	var historyUpdate, historyDelete bool
	if err = pool.QueryRow(ctx, `SELECT has_table_privilege('espacigo_runtime','public.verificacion_historial_local','UPDATE'),has_table_privilege('espacigo_runtime','public.verificacion_historial_local','DELETE')`).Scan(&historyUpdate, &historyDelete); err != nil || historyUpdate || historyDelete {
		t.Fatalf("runtime history can mutate/delete: update=%v delete=%v err=%v", historyUpdate, historyDelete, err)
	}
	r := New(pool)
	now := time.Now().UTC()
	first := verification.Case{ID: "11111111-1111-4111-8111-111111111111", OwnerID: owner, Type: "kyb", State: "en_revision", Provider: "local-fixture-v1", EvidenceRef: "fixture:22222222-2222-4222-8222-222222222222", Idempotency: "kyb-request-0001", CreatedAt: now}
	created, err := r.Create(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	privateRoot := t.TempDir()
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	privateStore, err := evidencefs.New(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	evidenceService, err := verification.NewEvidenceService(r, r, privateStore, credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	persistedEvidence, err := evidenceService.Upload(ctx, owner, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	readEvidence, blob, err := evidenceService.OwnContent(ctx, owner, created.ID, persistedEvidence.ID)
	checksum := sha256.Sum256(blob)
	if err != nil || readEvidence.ID != persistedEvidence.ID || readEvidence.SHA256 != hex.EncodeToString(checksum[:]) || readEvidence.SizeBytes != int64(len(blob)) {
		t.Fatalf("persisted evidence=%+v err=%v", readEvidence, err)
	}
	ownEvidence, err := r.ListOwnEvidence(ctx, owner, created.ID)
	if err != nil || len(ownEvidence) != 1 {
		t.Fatalf("owner evidence=%+v err=%v", ownEvidence, err)
	}
	if _, _, err := evidenceService.OwnContent(ctx, other, created.ID, persistedEvidence.ID); err != verification.ErrNotFound {
		t.Fatalf("foreign evidence read err=%v; want not found", err)
	}
	if foreignEvidence, err := r.ListOwnEvidence(ctx, other, created.ID); err != nil || len(foreignEvidence) != 0 {
		t.Fatalf("foreign evidence list=%+v err=%v", foreignEvidence, err)
	}
	if _, _, err := evidenceService.ReviewContent(ctx, created.ID, persistedEvidence.ID); err != nil {
		t.Fatalf("review evidence read: %v", err)
	}
	if reviewedEvidence, err := r.ListReviewEvidence(ctx, created.ID); err != nil || len(reviewedEvidence) != 1 {
		t.Fatalf("review evidence list=%+v err=%v", reviewedEvidence, err)
	}
	if err := evidenceService.DeleteOwn(ctx, owner, created.ID, persistedEvidence.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(privateRoot, persistedEvidence.ID+".png")); !os.IsNotExist(err) {
		t.Fatalf("blob remains after explicit cleanup: %v", err)
	}
	if _, err := r.GetOwnEvidence(ctx, owner, created.ID, persistedEvidence.ID); err != verification.ErrNotFound {
		t.Fatalf("evidence remains after explicit cleanup: %v", err)
	}
	replayed := first
	replayed.ID = "33333333-3333-4333-8333-333333333333"
	replayed.EvidenceRef = "fixture:44444444-4444-4444-8444-444444444444"
	duplicate, err := r.Create(ctx, replayed)
	if err != nil || duplicate.ID != created.ID {
		t.Fatalf("idempotent create=%+v err=%v", duplicate, err)
	}
	if _, err := r.GetOwn(ctx, other, created.ID); err != verification.ErrNotFound {
		t.Fatalf("foreign owner read err=%v", err)
	}
	if items, err := r.ListPending(ctx); err != nil || len(items) != 2 {
		t.Fatalf("pending count=%d err=%v", len(items), err)
	}
	rejected, err := r.Review(ctx, created.ID, reviewer, false, "antecedentes_incompletos", time.Now)
	if err != nil || rejected.State != "rechazada" || rejected.ResolvedAt == nil {
		t.Fatalf("reject=%+v err=%v", rejected, err)
	}
	if _, err := r.Review(ctx, created.ID, reviewer, true, "", time.Now); err != verification.ErrConflict {
		t.Fatalf("second review err=%v", err)
	}
	retry := verification.Case{ID: "55555555-5555-4555-8555-555555555555", OwnerID: owner, Provider: "local-fixture-v1", EvidenceRef: "fixture:66666666-6666-4666-8666-666666666666", State: "en_revision", Idempotency: "kyb-retry-0001", CreatedAt: now, CorrectionCode: "antecedentes_fixture_actualizados"}
	newCase, err := r.Retry(ctx, owner, created.ID, retry, time.Now)
	if err != nil || newCase.RetryOf != created.ID || newCase.Type != "kyb" {
		t.Fatalf("retry=%+v err=%v", newCase, err)
	}
	retry.ID = "77777777-7777-4777-8777-777777777777"
	again, err := r.Retry(ctx, owner, created.ID, retry, time.Now)
	if err != nil || again.ID != newCase.ID {
		t.Fatalf("idempotent retry=%+v err=%v", again, err)
	}
	if newCase.CorrectionCode != "antecedentes_fixture_actualizados" {
		t.Fatalf("retry did not retain typed correction: %+v", newCase)
	}
	retryHistory, err := r.ListHistory(ctx, owner, created.ID)
	if err != nil || len(retryHistory) != 3 || retryHistory[0].Action != "solicitada" || retryHistory[1].Action != "revision_rechazada" || retryHistory[2].Action != "subsanacion_solicitada" {
		t.Fatalf("retry history=%+v err=%v", retryHistory, err)
	}

	// Two requests for the same owner, rejected case, and key can both pass an
	// initial lookup before either inserts. They must converge on one retry row.
	raceParent, err := r.Create(ctx, verification.Case{
		ID: "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", OwnerID: owner, Type: "kyb",
		State: "en_revision", Provider: "local-fixture-v1",
		EvidenceRef: "fixture:ffffffff-ffff-4fff-8fff-ffffffffffff",
		Idempotency: "kyb-race-parent-0001", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Review(ctx, raceParent.ID, reviewer, false, "antecedentes_incompletos", time.Now); err != nil {
		t.Fatal(err)
	}
	const concurrentKey = "kyb-retry-race-0001"
	start := make(chan struct{})
	results := make(chan verification.Case, 2)
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, id := range []string{
		"88888888-8888-4888-8888-888888888888",
		"99999999-9999-4999-8999-999999999999",
	} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			<-start
			result, retryErr := r.Retry(ctx, owner, raceParent.ID, verification.Case{
				ID: id, OwnerID: owner, Provider: "local-fixture-v1",
				EvidenceRef: "fixture:" + id, State: "en_revision",
				Idempotency: concurrentKey, CreatedAt: time.Now().UTC(), CorrectionCode: "antecedentes_fixture_actualizados",
			}, time.Now)
			results <- result
			errs <- retryErr
		}(id)
	}
	close(start)
	wg.Wait()
	close(results)
	close(errs)
	var retryIDs []string
	for result := range results {
		retryIDs = append(retryIDs, result.ID)
	}
	for retryErr := range errs {
		if retryErr != nil {
			t.Fatalf("concurrent retry err=%v", retryErr)
		}
	}
	if len(retryIDs) != 2 || retryIDs[0] == "" || retryIDs[0] != retryIDs[1] {
		t.Fatalf("concurrent retry IDs=%v; want same non-empty case", retryIDs)
	}
	var retryCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM public.verificacion WHERE usuario_id=$1 AND reintento_de=$2::text::uuid AND clave_idempotencia=$3`, owner, raceParent.ID, concurrentKey).Scan(&retryCount); err != nil {
		t.Fatal(err)
	}
	if retryCount != 1 {
		t.Fatalf("concurrent retry rows=%d; want exactly one", retryCount)
	}

	// Reusing the same owner/key for a different parent remains a conflict.
	otherCase, err := r.Create(ctx, verification.Case{
		ID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaab", OwnerID: owner, Type: "kyb",
		State: "en_revision", Provider: "local-fixture-v1",
		EvidenceRef: "fixture:bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbc",
		Idempotency: "kyb-request-0002", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Review(ctx, otherCase.ID, reviewer, false, "antecedentes_incompletos", time.Now); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Retry(ctx, owner, otherCase.ID, verification.Case{
		ID: "cccccccc-cccc-4ccc-8ccc-cccccccccccd", OwnerID: owner,
		EvidenceRef: "fixture:dddddddd-dddd-4ddd-8ddd-ddddddddddde",
		State:       "en_revision", Idempotency: concurrentKey, CreatedAt: now, CorrectionCode: "antecedentes_fixture_actualizados",
	}, time.Now); err != verification.ErrConflict {
		t.Fatalf("incompatible retry key reuse err=%v; want conflict", err)
	}
	// Eligibility is type-scoped and persists across newer pending/rejected cases.
	approvedCase, err := r.Create(ctx, verification.Case{
		ID: "abababab-abab-4bab-8bab-abababababab", OwnerID: owner, Type: "kyb", State: "en_revision",
		Provider: "local-fixture-v1", EvidenceRef: "fixture:acacacac-acac-4cac-8cac-acacacacacac",
		Idempotency: "kyb-approved-0001", CreatedAt: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	approvedCase, err = r.Review(ctx, approvedCase.ID, reviewer, true, "", time.Now)
	if err != nil || approvedCase.State != "aprobada" {
		t.Fatalf("approve case=%+v err=%v", approvedCase, err)
	}
	eligibility, err := r.ListEligibility(ctx, owner)
	if err != nil || len(eligibility) != 2 || !eligibility[0].Eligible || eligibility[0].VerificationID != approvedCase.ID {
		t.Fatalf("approved type eligibility=%+v err=%v", eligibility, err)
	}
	newer, err := r.Create(ctx, verification.Case{
		ID: "adadadad-adad-4dad-8dad-adadadadadad", OwnerID: owner, Type: "kyb", State: "en_revision",
		Provider: "local-fixture-v1", EvidenceRef: "fixture:aeaeaeae-aeae-4eae-8eae-aeaeaeaeaeae",
		Idempotency: "kyb-newer-pending-01", CreatedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	eligibility, err = r.ListEligibility(ctx, owner)
	if err != nil || !eligibility[0].Eligible || eligibility[0].VerificationID != approvedCase.ID {
		t.Fatalf("pending case removed eligibility: %+v err=%v", eligibility, err)
	}
	if _, err = r.Review(ctx, newer.ID, reviewer, false, "documento_vencido", time.Now); err != nil {
		t.Fatal(err)
	}
	eligibility, err = r.ListEligibility(ctx, owner)
	if err != nil || !eligibility[0].Eligible || eligibility[0].VerificationID != approvedCase.ID {
		t.Fatalf("rejected newer case removed eligibility: %+v err=%v", eligibility, err)
	}
	if eligibility[1].Eligible {
		t.Fatalf("KYB approval implicitly granted KYC: %+v", eligibility)
	}
	var originalRetentionDeadline time.Time
	if err := pool.QueryRow(ctx, `SELECT retirar_en FROM public.verificacion WHERE id=$1`, approvedCase.ID).Scan(&originalRetentionDeadline); err != nil {
		t.Fatal(err)
	}

	startRevoke := make(chan struct{})
	revokes := make(chan verification.Case, 2)
	revokeErrors := make(chan error, 2)
	var revokeWG sync.WaitGroup
	for _, correlation := range []string{"test-revoke-1", "test-revoke-2"} {
		revokeWG.Add(1)
		go func(correlation string) {
			defer revokeWG.Done()
			<-startRevoke
			result, revokeErr := r.Revoke(ctx, approvedCase.ID, reviewer, "revision_fixture_actualizada", "revoke-key-0001", correlation, time.Now)
			revokes <- result
			revokeErrors <- revokeErr
		}(correlation)
	}
	close(startRevoke)
	revokeWG.Wait()
	close(revokes)
	close(revokeErrors)
	for result := range revokes {
		if result.State != "revocada" || result.RevokedAt == nil {
			t.Fatalf("concurrent revoke=%+v", result)
		}
	}
	for revokeErr := range revokeErrors {
		if revokeErr != nil {
			t.Fatalf("concurrent idempotent revoke err=%v", revokeErr)
		}
	}
	var revokeHistoryCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.verificacion_historial_local WHERE verificacion_id=$1 AND accion='elegibilidad_revocada' AND clave_idempotencia='revoke-key-0001'`, approvedCase.ID).Scan(&revokeHistoryCount); err != nil || revokeHistoryCount != 1 {
		t.Fatalf("concurrent revoke history count=%d err=%v", revokeHistoryCount, err)
	}
	if _, err := r.Revoke(ctx, approvedCase.ID, reviewer, "revision_fixture_actualizada", "revoke-key-0002", "test-revoke-3", time.Now); err != verification.ErrConflict {
		t.Fatalf("different revoke key err=%v; want conflict", err)
	}
	eligibility, err = r.ListEligibility(ctx, owner)
	if err != nil || eligibility[0].Eligible || eligibility[0].RevokedAt == nil || eligibility[0].RevocationReason != "revision_fixture_actualizada" {
		t.Fatalf("revoked eligibility=%+v err=%v", eligibility, err)
	}
	var revokedRetentionDeadline time.Time
	if err := pool.QueryRow(ctx, `SELECT retirar_en FROM public.verificacion WHERE id=$1`, approvedCase.ID).Scan(&revokedRetentionDeadline); err != nil || !revokedRetentionDeadline.Equal(originalRetentionDeadline) {
		t.Fatalf("revocation changed original terminal retention: before=%s after=%s err=%v", originalRetentionDeadline, revokedRetentionDeadline, err)
	}
	history, err := r.ListHistory(ctx, owner, approvedCase.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 3 || history[0].Action != "solicitada" || history[1].Action != "revision_aprobada" || history[2].Action != "elegibilidad_revocada" {
		t.Fatalf("verification history=%+v", history)
	}

	// A later explicit approval restores only its matching type and records a new source.
	reapproved, err := r.Create(ctx, verification.Case{
		ID: "afafafaf-afaf-4faf-8faf-afafafafafaf", OwnerID: owner, Type: "kyb", State: "en_revision",
		Provider: "local-fixture-v1", EvidenceRef: "fixture:b0b0b0b0-b0b0-40b0-80b0-b0b0b0b0b0b0",
		Idempotency: "kyb-reapproved-0001", CreatedAt: now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Review(ctx, reapproved.ID, reviewer, true, "", time.Now); err != nil {
		t.Fatal(err)
	}
	eligibility, err = r.ListEligibility(ctx, owner)
	if err != nil || !eligibility[0].Eligible || eligibility[0].VerificationID != reapproved.ID || eligibility[0].RevokedAt != nil {
		t.Fatalf("reapproved eligibility=%+v err=%v", eligibility, err)
	}

	// A revocation racing with a newer approval cannot revoke the newer source.
	raceApproval, err := r.Create(ctx, verification.Case{
		ID: "b1b1b1b1-b1b1-41b1-81b1-b1b1b1b1b1b1", OwnerID: owner, Type: "kyb", State: "en_revision",
		Provider: "local-fixture-v1", EvidenceRef: "fixture:b2b2b2b2-b2b2-42b2-82b2-b2b2b2b2b2b2",
		Idempotency: "kyb-race-approval-01", CreatedAt: now.Add(3 * time.Second),
	})
	if err != nil {
		t.Fatal(err)
	}
	raceStart := make(chan struct{})
	raceReviewErr := make(chan error, 1)
	raceRevokeErr := make(chan error, 1)
	var raceWG sync.WaitGroup
	raceWG.Add(2)
	go func() {
		defer raceWG.Done()
		<-raceStart
		_, e := r.Review(ctx, raceApproval.ID, reviewer, true, "", time.Now)
		raceReviewErr <- e
	}()
	go func() {
		defer raceWG.Done()
		<-raceStart
		_, e := r.Revoke(ctx, reapproved.ID, reviewer, "revision_fixture_actualizada", "revoke-race-0001", "test-revoke-race", time.Now)
		raceRevokeErr <- e
	}()
	close(raceStart)
	raceWG.Wait()
	close(raceReviewErr)
	close(raceRevokeErr)
	if e := <-raceReviewErr; e != nil {
		t.Fatalf("concurrent newer approval err=%v", e)
	}
	if e := <-raceRevokeErr; e != nil && e != verification.ErrConflict {
		t.Fatalf("concurrent stale revocation err=%v", e)
	}
	eligibility, err = r.ListEligibility(ctx, owner)
	if err != nil || !eligibility[0].Eligible || eligibility[0].VerificationID != raceApproval.ID || eligibility[0].RevokedAt != nil {
		t.Fatalf("concurrent approval/revoke final eligibility=%+v err=%v", eligibility, err)
	}

}
