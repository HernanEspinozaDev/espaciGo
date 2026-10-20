package gcp06testaccess_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06testaccess"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

func TestGCP06ApplyHashesAndRevokesExactCandidateCohort(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; requires disposable PostgreSQL 18 with the exact GCP-06 candidate database name")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Path != "/"+gcp06testaccess.Database {
		t.Fatal("TEST_DATABASE_URL must target only the disposable GCP-06 candidate-named test database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal("connect disposable PostgreSQL test database")
	}
	defer admin.Close(context.Background())
	if _, err := admin.Exec(ctx, `DO $$ BEGIN IF NOT EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT; END IF; END $$`); err != nil {
		t.Fatal("prepare disposable runtime group role")
	}
	_, sourceFile, _, _ := runtime.Caller(0)
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", "..", "db", "migrations"))
	if _, err := migrator.Run(ctx, databaseURL, migrationDir); err != nil {
		t.Fatalf("apply migrations to disposable PostgreSQL database: %v", err)
	}
	const (
		ownerID                         = "00000000-0000-4000-8000-000000000001"
		userAID                         = "00000000-0000-4000-8000-000000000002"
		userBID                         = "00000000-0000-4000-8000-000000000003"
		desidentifiedID                 = "00000000-0000-4000-8000-000000000004"
		operationID                     = "00000000-0000-4000-8000-000000000099"
		login                           = "gcp06_test_access_012345abcdef"
		sharedPassword  identity.Secret = "FixturePass1!"
	)
	if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES
		($1,$2,$2,'owner-hash','activo'),($3,'fixture-a@example.invalid','fixture-a@example.invalid','old-a','activo'),($4,'fixture-b@example.invalid','fixture-b@example.invalid','old-b','activo'),($5,'fixture-desidentified@example.invalid','fixture-desidentified@example.invalid','untouched-hash','desidentificado')`, ownerID, gcp06testaccess.OwnerEmail, userAID, userBID, desidentifiedID); err != nil {
		t.Fatal("insert synthetic candidate account cohort")
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'bootstrap_propietario')`, ownerID); err != nil {
		t.Fatal("mark the synthetic owner as bootstrapped")
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index, id := range []string{ownerID, userAID, userBID} {
		if _, err := admin.Exec(ctx, `INSERT INTO public.sesion(id,usuario_id,token_hash,creada_en,ultima_actividad_en,expira_en) VALUES($1,$2,$3,$4,$4,$5)`, uuidForTest(index+10), id, hashForTest(index+10), now, now.Add(time.Hour)); err != nil {
			t.Fatal("insert synthetic active session")
		}
		if _, err := admin.Exec(ctx, `INSERT INTO public.token_accion(id,usuario_id,proposito,token_hash,creado_en,expira_en) VALUES($1,$2,'recuperar_clave',$3,$4,$5)`, uuidForTest(index+20), id, hashForTest(index+20), now, now.Add(time.Hour)); err != nil {
			t.Fatal("insert synthetic action token")
		}
	}
	if _, err := admin.Exec(ctx, `CREATE ROLE `+pgx.Identifier{login}.Sanitize()+` LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT`); err != nil {
		t.Fatal("create one-purpose synthetic login")
	}
	if _, err := admin.Exec(ctx, `GRANT espacigo_gcp06_test_access TO `+pgx.Identifier{login}.Sanitize()+` WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`); err != nil {
		t.Fatal("grant only the restricted GCP-06 test-access role")
	}

	loginURL := *parsed
	loginURL.User = url.User(login)
	conn, err := pgx.Connect(ctx, loginURL.String())
	if err != nil {
		t.Fatal("connect using synthetic restricted login")
	}
	defer conn.Close(context.Background())
	if err := gcp06testaccess.ValidateRestrictedConnection(ctx, conn, login); err != nil {
		t.Fatalf("restricted SQL contract rejected: %v", err)
	}
	if _, err := admin.Exec(ctx, `DELETE FROM public.administrador_global WHERE usuario_id=$1`, ownerID); err != nil {
		t.Fatal("prepare missing-owner-role guard case")
	}
	if _, err := gcp06testaccess.Preview(ctx, conn); err == nil {
		t.Fatal("reset preview proceeded without the unique global owner role")
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'bootstrap_propietario')`, ownerID); err != nil {
		t.Fatal("restore synthetic global owner role")
	}
	if _, err := admin.Exec(ctx, `UPDATE public.usuario SET estado='correo_pendiente' WHERE id=$1`, ownerID); err != nil {
		t.Fatal("prepare inactive-owner guard case")
	}
	if _, err := gcp06testaccess.Preview(ctx, conn); err == nil {
		t.Fatal("reset preview proceeded for an inactive owner")
	}
	if _, err := admin.Exec(ctx, `UPDATE public.usuario SET estado='activo' WHERE id=$1`, ownerID); err != nil {
		t.Fatal("restore active synthetic owner")
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.bloqueo_cuenta_administrativo_local(cuenta_id,bloqueada_por,motivo_codigo,bloqueada_en,correlacion_id) VALUES($1,$2,'revision_administrativa',now(),'gcp06-test-owner-guard')`, ownerID, userAID); err != nil {
		t.Fatal("prepare administratively-blocked owner guard case")
	}
	if _, err := gcp06testaccess.Preview(ctx, conn); err == nil {
		t.Fatal("reset preview proceeded for an administratively blocked owner")
	}
	if _, err := admin.Exec(ctx, `DELETE FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id=$1`, ownerID); err != nil {
		t.Fatal("restore unblocked synthetic owner")
	}
	preview, err := gcp06testaccess.Preview(ctx, conn)
	if err != nil || preview.TargetCount != 3 {
		t.Fatalf("unexpected non-PII preview: count=%d err=%v", preview.TargetCount, err)
	}
	if _, err := gcp06testaccess.Apply(ctx, conn, operationID, preview.TargetCount, "0000000000000000000000000000000000000000000000000000000000000000", sharedPassword); err == nil {
		t.Fatal("stale cohort digest unexpectedly applied")
	}
	if result, err := gcp06testaccess.Apply(ctx, conn, operationID, preview.TargetCount, preview.TargetSHA256, sharedPassword); err != nil || result.TargetCount != 3 {
		t.Fatalf("reviewed candidate reset failed: result=%+v err=%v", result, err)
	}
	if result, err := gcp06testaccess.Apply(ctx, conn, operationID, preview.TargetCount, preview.TargetSHA256, sharedPassword); err == nil || !result.AlreadyApplied {
		t.Fatalf("committed operation replay was not blocked: result=%+v err=%v", result, err)
	}
	var ownerHash string
	if err := admin.QueryRow(ctx, `SELECT hash_clave FROM public.usuario WHERE id=$1`, ownerID).Scan(&ownerHash); err != nil {
		t.Fatal("read synthetic owner hash")
	}
	if matched, err := (bcrypt.Hasher{}).Matches(identity.Secret(ownerHash), sharedPassword); err != nil || !matched {
		t.Fatal("owner was not included in the shared bcrypt reset")
	}
	var ownerRoleCount int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.administrador_global WHERE usuario_id=$1`, ownerID).Scan(&ownerRoleCount); err != nil || ownerRoleCount != 1 {
		t.Fatal("global owner role was not preserved by the credential reset")
	}
	var resetAccounts, revokedSessions, invalidatedTokens, distinctHashes int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.usuario WHERE id IN ($1,$2,$3)`, ownerID, userAID, userBID).Scan(&resetAccounts); err != nil {
		t.Fatal("read fixture credential hashes")
	}
	if err := admin.QueryRow(ctx, `SELECT count(DISTINCT hash_clave) FROM public.usuario WHERE id IN ($1,$2,$3)`, ownerID, userAID, userBID).Scan(&distinctHashes); err != nil {
		t.Fatal("check fixture credential hash equality")
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.sesion WHERE usuario_id IN ($1,$2,$3) AND revocada_en IS NOT NULL`, ownerID, userAID, userBID).Scan(&revokedSessions); err != nil {
		t.Fatal("read fixture session revocations")
	}
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.token_accion WHERE usuario_id IN ($1,$2,$3) AND invalidado_en IS NOT NULL`, ownerID, userAID, userBID).Scan(&invalidatedTokens); err != nil {
		t.Fatal("read fixture token invalidations")
	}
	var desidentifiedHash string
	if err := admin.QueryRow(ctx, `SELECT hash_clave FROM public.usuario WHERE id=$1`, desidentifiedID).Scan(&desidentifiedHash); err != nil || desidentifiedHash != "untouched-hash" {
		t.Fatal("desidentified account was included in the reset cohort")
	}
	if resetAccounts != 3 || distinctHashes != 1 || revokedSessions != 3 || invalidatedTokens != 3 {
		t.Fatalf("unexpected reset side effects: accounts=%d distinct_hashes=%d revoked_sessions=%d invalidated_tokens=%d", resetAccounts, distinctHashes, revokedSessions, invalidatedTokens)
	}
	var commonHash string
	if err := admin.QueryRow(ctx, `SELECT min(hash_clave) FROM public.usuario WHERE id IN ($1,$2,$3)`, ownerID, userAID, userBID).Scan(&commonHash); err != nil {
		t.Fatal("read synthetic common hash")
	}
	if matched, err := (bcrypt.Hasher{}).Matches(identity.Secret(commonHash), sharedPassword); err != nil || !matched {
		t.Fatal("reset hash does not verify with the synthetic shared password")
	}
	var auditCount int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.gcp06_test_access_batch_audit WHERE id=$1 AND target_count=3 AND target_set_sha256=$2`, operationID, preview.TargetSHA256).Scan(&auditCount); err != nil || auditCount != 1 {
		t.Fatal("atomic candidate reset audit row is missing")
	}
	if _, err := conn.Exec(ctx, `SELECT hash_clave FROM public.usuario LIMIT 1`); err == nil {
		t.Fatal("restricted reset login can read password hashes")
	}
}

func uuidForTest(n int) string {
	return "00000000-0000-4000-8000-0000000000" + fmt.Sprintf("%02d", n)
}

func hashForTest(n int) string {
	return fmt.Sprintf("%064x", n)
}
