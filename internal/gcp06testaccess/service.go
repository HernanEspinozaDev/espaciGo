package gcp06testaccess

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/jackc/pgx/v5"
)

const (
	RunID           = "20261020-gcp06a"
	Instance        = "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg"
	Database        = "espacigo_gcp06_candidate_20261020-gcp06a"
	OwnerEmail      = "hernan.espinoza.dev@gmail.com"
	GroupRole       = "espacigo_gcp06_test_access"
	SQLPasswordFile = "/var/run/secrets/gcp06-test-access-db/password"
	AppPasswordFile = "/var/run/secrets/gcp06-test-account/password"
)

var (
	loginPattern  = regexp.MustCompile(`^gcp06_test_access_[a-f0-9]{12}$`)
	canonicalUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	sha256Hex     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Result struct {
	TargetCount    int
	TargetSHA256   string
	AlreadyApplied bool
}

// ValidateTarget pins this operator to the sole imported test candidate.
func ValidateTarget(runID, instance, database, login string) error {
	if runID != RunID || instance != Instance || database != Database || !loginPattern.MatchString(login) {
		return errors.New("target must be the pinned GCP-06 candidate and one-time restricted login")
	}
	return nil
}

// ValidateRestrictedConnection switches the physical login to the NOLOGIN
// allowlist role and checks that the login has no direct grants or extra roles.
func ValidateRestrictedConnection(ctx context.Context, conn *pgx.Conn, login string) error {
	if conn == nil || !loginPattern.MatchString(login) {
		return errors.New("restricted candidate connection is required")
	}
	if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{GroupRole}.Sanitize()); err != nil {
		return errors.New("candidate test-access role is unavailable")
	}
	var safe bool
	err := conn.QueryRow(ctx, `
SELECT session_user=$1 AND current_user=$2
 AND NOT s.rolsuper AND NOT s.rolcreatedb AND NOT s.rolcreaterole AND NOT s.rolreplication AND NOT s.rolbypassrls AND NOT s.rolinherit
 AND NOT r.rolcanlogin AND NOT r.rolsuper AND NOT r.rolcreatedb AND NOT r.rolcreaterole AND NOT r.rolreplication AND NOT r.rolbypassrls AND NOT r.rolinherit
 AND (SELECT count(*)=1 FROM pg_catalog.pg_auth_members m WHERE m.member=s.oid AND m.roleid=r.oid AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option)
 AND (SELECT count(*)=1 FROM pg_catalog.pg_auth_members m WHERE m.member=s.oid)
 AND NOT EXISTS(SELECT 1 FROM information_schema.table_privileges p WHERE p.grantee=session_user)
 AND NOT EXISTS(SELECT 1 FROM information_schema.column_privileges p WHERE p.grantee=session_user)
 AND NOT EXISTS(SELECT 1 FROM information_schema.routine_privileges p WHERE p.grantee=session_user)
	AND NOT EXISTS(SELECT 1 FROM information_schema.usage_privileges p WHERE p.grantee=session_user)
 AND has_database_privilege(current_user,current_database(),'CONNECT')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND NOT has_schema_privilege(current_user,'public','CREATE')
FROM pg_catalog.pg_roles s CROSS JOIN pg_catalog.pg_roles r
WHERE s.rolname=session_user AND r.rolname=current_user`, login, GroupRole).Scan(&safe)
	if err != nil || !safe {
		return errors.New("candidate test-access login does not match the restricted role contract")
	}
	if err := validateExactColumnGrants(ctx, conn); err != nil {
		return err
	}
	var dbName string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&dbName); err != nil || dbName != Database {
		return errors.New("connected database differs from the pinned candidate")
	}
	return nil
}

func validateExactColumnGrants(ctx context.Context, conn *pgx.Conn) error {
	rows, err := conn.Query(ctx, `SELECT table_name,column_name,privilege_type
		FROM information_schema.column_privileges WHERE grantee=current_user`)
	if err != nil {
		return errors.New("candidate test-access grants could not be checked")
	}
	defer rows.Close()
	allowed := map[string]struct{}{
		"usuario.id:SELECT": {}, "usuario.hash_clave:UPDATE": {}, "usuario.actualizado_en:UPDATE": {},
		"sesion.usuario_id:SELECT": {}, "sesion.revocada_en:SELECT": {}, "sesion.revocada_en:UPDATE": {},
		"token_accion.usuario_id:SELECT": {}, "token_accion.invalidado_en:SELECT": {},
		"token_accion.invalidado_en:UPDATE": {}, "token_accion.consumido_en:SELECT": {},
		"gcp06_test_access_batch_audit.id:INSERT": {}, "gcp06_test_access_batch_audit.target_count:INSERT": {},
		"gcp06_test_access_batch_audit.target_set_sha256:INSERT": {}, "gcp06_test_access_batch_audit.occurred_at:INSERT": {},
	}
	seen := make(map[string]struct{}, len(allowed))
	for rows.Next() {
		var table, column, privilege string
		if err := rows.Scan(&table, &column, &privilege); err != nil {
			return errors.New("candidate test-access grants could not be read")
		}
		key := table + "." + column + ":" + privilege
		if _, ok := allowed[key]; !ok {
			return errors.New("candidate test-access role has a privilege outside its reviewed allowlist")
		}
		seen[key] = struct{}{}
	}
	if err := rows.Err(); err != nil || len(seen) != len(allowed) {
		return errors.New("candidate test-access role does not match its exact column allowlist")
	}
	var exactRoutines bool
	if err := conn.QueryRow(ctx, `SELECT
		(SELECT count(*)=3 FROM information_schema.routine_privileges WHERE grantee=current_user)
		AND (SELECT count(*)=3 FROM information_schema.routine_privileges WHERE grantee=current_user AND privilege_type='EXECUTE'
		 AND routine_name IN ('gcp06_test_access_target_ids','gcp06_test_access_owner_ready','gcp06_test_access_operation_exists'))
		AND NOT EXISTS (SELECT 1 FROM information_schema.table_privileges WHERE grantee=current_user)
		AND NOT EXISTS (SELECT 1 FROM information_schema.role_table_grants WHERE grantee=current_user AND privilege_type IN ('DELETE','TRUNCATE'))
		AND NOT EXISTS (SELECT 1 FROM information_schema.usage_privileges WHERE grantee=current_user)
		AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_shdepend sd JOIN pg_catalog.pg_roles r ON r.oid=sd.refobjid
		 WHERE sd.refclassid='pg_catalog.pg_authid'::regclass AND r.rolname=current_user AND sd.deptype='o')`).Scan(&exactRoutines); err != nil || !exactRoutines {
		return errors.New("candidate test-access role has unexpected table, routine, or sequence privileges")
	}
	return nil
}

// Preview returns only a count and an order-independent digest of eligible
// candidate account UUIDs, including the active, verified global owner.
func Preview(ctx context.Context, conn *pgx.Conn) (Result, error) {
	if err := requireBootstrappedOwner(ctx, conn); err != nil {
		return Result{}, err
	}
	ids, err := targetAccountIDs(ctx, conn)
	if err != nil {
		return Result{}, errors.New("could not inspect the candidate test-account cohort")
	}
	if len(ids) == 0 {
		return Result{}, errors.New("candidate test-account cohort is empty")
	}
	return Result{TargetCount: len(ids), TargetSHA256: TargetSetDigest(ids)}, nil
}

// Apply changes all currently eligible candidate accounts in one transaction.
// A preview count and target-set digest bind the apply to the exact reviewed cohort.
func Apply(ctx context.Context, conn *pgx.Conn, operationID string, expectedCount int, expectedDigest string, password identity.Secret) (Result, error) {
	if conn == nil || !canonicalUUID.MatchString(operationID) || expectedCount < 1 || expectedCount > 10000 || !sha256Hex.MatchString(expectedDigest) {
		return Result{}, errors.New("reviewed candidate reset bounds are invalid")
	}
	if err := identity.ValidatePassword(password); err != nil {
		return Result{}, errors.New("Secret Manager application password does not meet backend policy")
	}
	hash, err := (bcrypt.Hasher{}).Hash(password)
	if err != nil {
		return Result{}, errors.New("could not hash candidate test password")
	}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return Result{}, errors.New("could not begin candidate reset transaction")
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(1195469901,1397056562)`); err != nil {
		return Result{}, errors.New("could not acquire candidate reset lock")
	}
	var prior bool
	if err := tx.QueryRow(ctx, `SELECT public.gcp06_test_access_operation_exists($1)`, operationID).Scan(&prior); err != nil {
		return Result{}, errors.New("could not check candidate reset operation id")
	}
	if prior {
		return Result{AlreadyApplied: true}, errors.New("candidate reset operation id was already committed; reconcile its audit before retry")
	}
	if err := requireBootstrappedOwnerTx(ctx, tx); err != nil {
		return Result{}, err
	}
	ids, err := targetAccountIDsTx(ctx, tx)
	if err != nil {
		return Result{}, errors.New("could not lock candidate test-account cohort")
	}
	result := Result{TargetCount: len(ids), TargetSHA256: TargetSetDigest(ids)}
	if result.TargetCount != expectedCount || result.TargetSHA256 != expectedDigest {
		return result, errors.New("candidate cohort differs from the reviewed preview; no accounts changed")
	}
	now := time.Now().UTC()
	for _, id := range ids {
		if _, err := tx.Exec(ctx, `UPDATE public.usuario SET hash_clave=$2, actualizado_en=$3 WHERE id=$1`, id, string(hash), now); err != nil {
			return Result{}, errors.New("candidate credential update failed; transaction rolled back")
		}
		if _, err := tx.Exec(ctx, `UPDATE public.sesion SET revocada_en=$2 WHERE usuario_id=$1 AND revocada_en IS NULL`, id, now); err != nil {
			return Result{}, errors.New("candidate session revocation failed; transaction rolled back")
		}
		if _, err := tx.Exec(ctx, `UPDATE public.token_accion SET invalidado_en=$2 WHERE usuario_id=$1 AND invalidado_en IS NULL AND consumido_en IS NULL`, id, now); err != nil {
			return Result{}, errors.New("candidate action-token revocation failed; transaction rolled back")
		}
	}
	if _, err := tx.Exec(ctx, `INSERT INTO public.gcp06_test_access_batch_audit(id, target_count, target_set_sha256, occurred_at) VALUES($1,$2,$3,$4)`, operationID, result.TargetCount, result.TargetSHA256, now); err != nil {
		return Result{}, errors.New("candidate reset audit failed; transaction rolled back")
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, errors.New("candidate reset outcome is uncertain; reconcile the operation id before any retry")
	}
	clear([]byte(hash))
	return result, nil
}

func TargetSetDigest(ids []string) string {
	canonical := append([]string(nil), ids...)
	sort.Strings(canonical)
	h := sha256.New()
	for _, id := range canonical {
		_, _ = fmt.Fprintln(h, id)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func targetAccountIDs(ctx context.Context, conn *pgx.Conn) ([]string, error) {
	if conn == nil {
		return nil, errors.New("candidate connection is required")
	}
	return queryTargetIDs(ctx, conn, `SELECT id::text FROM public.gcp06_test_access_target_ids() AS targets(id) ORDER BY id`)
}

func targetAccountIDsTx(ctx context.Context, tx pgx.Tx) ([]string, error) {
	return queryTargetIDs(ctx, tx, `SELECT u.id::text FROM public.usuario u JOIN public.gcp06_test_access_target_ids() selected(id) ON selected.id=u.id ORDER BY u.id FOR UPDATE OF u`)
}

func requireBootstrappedOwner(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return errors.New("candidate connection is required")
	}
	var eligible bool
	err := conn.QueryRow(ctx, `SELECT public.gcp06_test_access_owner_ready()`).Scan(&eligible)
	if err != nil || !eligible {
		return errors.New("verified owner must be the active global administrator before the shared test credential operation")
	}
	return nil
}

func requireBootstrappedOwnerTx(ctx context.Context, tx pgx.Tx) error {
	var eligible bool
	err := tx.QueryRow(ctx, `SELECT public.gcp06_test_access_owner_ready()`).Scan(&eligible)
	if err != nil || !eligible {
		return errors.New("verified owner must be the active global administrator before the shared test credential operation")
	}
	return nil
}

type queryRower interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func queryTargetIDs(ctx context.Context, q queryRower, sql string, args ...any) ([]string, error) {
	rows, err := q.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func ReadSecret(path string) (identity.Secret, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0400 || info.Size() == 0 || info.Size() > 72 {
		return "", errors.New("secret must be a bounded mode-0400 Secret Manager mount")
	}
	value, err := os.ReadFile(path)
	if err != nil || len(value) == 0 || len(value) > 72 {
		clear(value)
		return "", errors.New("Secret Manager mount is empty or invalid")
	}
	return identity.Secret(value), nil
}
