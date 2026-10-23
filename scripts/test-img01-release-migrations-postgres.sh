#!/usr/bin/env bash
# Disposable PostgreSQL only: never consumes an external DATABASE_URL or volume.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-img01-release-test-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-img01-release.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  docker exec --user root "$NAME" chmod 0777 /var/run/postgresql >/dev/null 2>&1 || true
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT INT TERM
unset PGPASSWORD PGPASSFILE PGUSER PGHOST PGPORT PGDATABASE DATABASE_PASSWORD_FILE DATABASE_SET_ROLE GCP04_REPLAY_CLOUD
export PGPASSFILE="$TMP_DIR/no-credentials"
export DATABASE_CONNECTION_MODE=direct
export DBMIGRATE_TIMEOUT=10m

docker run -d --name "$NAME" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_USER=fixture_admin -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 120); do
  ready_count="$(docker logs "$NAME" 2>&1 | rg -c 'database system is ready to accept connections' || true)"
  if [[ "${ready_count:-0}" -ge 2 ]] && docker exec "$NAME" pg_isready -U fixture_admin >/dev/null 2>&1; then break; fi
  sleep 0.25
done
docker exec "$NAME" pg_isready -U fixture_admin >/dev/null
docker exec "$NAME" psql -X -U fixture_admin -d postgres -v ON_ERROR_STOP=1 -q -c 'CREATE ROLE postgres LOGIN SUPERUSER'
go build -o "$TMP_DIR/dbmigrate" ./cmd/dbmigrate
mkdir "$TMP_DIR/v67" "$TMP_DIR/v74"
for migration in db/migrations/V*.sql; do
  version="${migration##*/V}"
  version="${version%%__*}"
  if (( 10#$version <= 67 )); then cp "$migration" "$TMP_DIR/v67/"; fi
  if (( 10#$version <= 74 )); then cp "$migration" "$TMP_DIR/v74/"; fi
done
sql() { docker exec -i "$NAME" psql -X -U postgres -d "$DB" -v ON_ERROR_STOP=1 "$@"; }
ledger_hash() { sql -qAt -c "SELECT md5(jsonb_agg(to_jsonb(m) ORDER BY version)::text) FROM public.schema_migrations m WHERE version<=67"; }
expect_failure() {
  local expected="$1"
  shift
  if "$@" >"$TMP_DIR/expected-failure.log" 2>&1; then
    echo "expected rejection: $expected" >&2; exit 1
  fi
  if ! rg -q -- "$expected" "$TMP_DIR/expected-failure.log"; then
    cat "$TMP_DIR/expected-failure.log" >&2; exit 1
  fi
}
DB=espacigo_dms_stage_20261020
docker exec "$NAME" createdb -U postgres "$DB"
export DATABASE_URL="postgres://postgres@localhost/$DB?host=$SOCKET_DIR"
sql -q -c 'CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT'
sql -q <<'SQL'
CREATE ROLE espacigo_api_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT espacigo_runtime TO espacigo_api_login WITH ADMIN FALSE, SET TRUE, INHERIT FALSE;
SQL
"$TMP_DIR/dbmigrate" -dir "$TMP_DIR/v67"
before="$(ledger_hash)"
# Preserve the real V1-V67 ledger. Only remove role ACLs/role in this disposable
# fixture to reproduce reported drift, without attributing its remote cause.
sql -q <<'SQL'
DROP OWNED BY espacigo_privacy_cleanup_worker;
DROP ROLE espacigo_privacy_cleanup_worker;
CREATE ROLE img01_reader LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT CONNECT ON DATABASE espacigo_dms_stage_20261020 TO img01_reader;
GRANT USAGE ON SCHEMA public TO img01_reader;
GRANT SELECT ON public.schema_migrations TO img01_reader;
SQL
expect_failure 'V000064 cleanup worker role' "$TMP_DIR/dbmigrate" -plan-only -dir db/migrations
expect_failure 'V000064 cleanup worker role' "$TMP_DIR/dbmigrate" -dir db/migrations
[[ "$(ledger_hash)" == "$before" ]]
# Wrong DB, wrong session and missing grant target must all fail atomically.
DB=postgres
expect_failure 'unexpected database' sql -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
DB=espacigo_dms_stage_20261020
{ echo 'SET SESSION AUTHORIZATION img01_reader;'; cat db/repair/reconcile_v64_cleanup_worker_role.sql; } > "$TMP_DIR/wrong-session.sql"
expect_failure 'reviewed postgres session' sql -f /dev/stdin < "$TMP_DIR/wrong-session.sql"
sql -q -c 'ALTER TABLE public.foto_perfil_sintetica_local RENAME TO repair_missing_target'
expect_failure 'grant targets are missing' sql -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
sql -q -c 'ALTER TABLE public.repair_missing_target RENAME TO foto_perfil_sintetica_local'
sql -q -c "ALTER FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamptz) SET search_path = public"
expect_failure 'owner/security settings differ' sql -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
sql -q -c 'ALTER FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamptz) SET search_path = pg_catalog, public, pg_temp'
expect_failure 'owner/security settings differ' sql -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
sql -q -c 'ALTER FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamptz) SET search_path = pg_catalog, public'
sql -q -c "ALTER FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamptz) SET timezone = 'America/Santiago'"
expect_failure 'owner/security settings differ' sql -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
sql -q -c "ALTER FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamptz) SET timezone = 'UTC'"
# Reproduce Cloud SQL owner authority, rather than a local postgres superuser.
# fixture_admin is local to this networkless cluster and only restores test setup.
sql -q -c 'ALTER ROLE postgres NOSUPERUSER CREATEROLE CREATEDB'
sql -q -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
expect_failure 'cleanup worker role exists' sql -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
[[ "$(ledger_hash)" == "$before" ]]
DB=postgres
expect_failure 'unexpected database/session' sql -f /dev/stdin < db/repair/reconcile_img01_database_temporary_acl.sql
DB=espacigo_dms_stage_20261020
sql -q -c 'GRANT TEMPORARY ON DATABASE espacigo_dms_stage_20261020 TO img01_reader'
expect_failure 'TEMP/CONNECT ACL differs' sql -f /dev/stdin < db/repair/reconcile_img01_database_temporary_acl.sql
sql -q -c 'REVOKE TEMPORARY ON DATABASE espacigo_dms_stage_20261020 FROM img01_reader'
sql -q -f /dev/stdin < db/repair/reconcile_img01_database_temporary_acl.sql
expect_failure 'TEMP/CONNECT ACL differs' sql -f /dev/stdin < db/repair/reconcile_img01_database_temporary_acl.sql
[[ "$(ledger_hash)" == "$before" ]]
sql -q <<'SQL'
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_privacy_cleanup_worker'
   AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit))
 THEN RAISE EXCEPTION 'unsafe cleanup attributes'; END IF;
 IF (SELECT count(*) FROM pg_auth_members WHERE roleid='espacigo_privacy_cleanup_worker'::regrole OR member='espacigo_privacy_cleanup_worker'::regrole) <> 1
 OR NOT EXISTS (SELECT 1 FROM pg_auth_members WHERE roleid='espacigo_privacy_cleanup_worker'::regrole
   AND member='postgres'::regrole AND grantor=10 AND admin_option AND NOT inherit_option AND NOT set_option)
 THEN RAISE EXCEPTION 'expected only creator ADMIN membership without SET/INHERIT'; END IF;
 IF has_table_privilege('espacigo_privacy_cleanup_worker','public.usuario','SELECT')
 OR has_table_privilege('espacigo_privacy_cleanup_worker','public.schema_migrations','SELECT')
 OR has_table_privilege('espacigo_privacy_cleanup_worker','public.baja_archivo_pendiente_local','UPDATE')
 OR NOT has_column_privilege('espacigo_privacy_cleanup_worker','public.baja_archivo_pendiente_local','claim_owner','UPDATE')
 OR NOT has_function_privilege('espacigo_privacy_cleanup_worker','public.finalize_pending_suppression_cleanup(uuid,timestamptz)','EXECUTE')
 THEN RAISE EXCEPTION 'repair grants missing or overbroad'; END IF;
END $$;
SQL
DATABASE_URL="postgres://img01_reader@localhost/$DB?host=$SOCKET_DIR" "$TMP_DIR/dbmigrate" -plan-only -dir db/migrations > "$TMP_DIR/plan.json"
python3 - "$TMP_DIR/plan.json" <<'PY'
import json,sys
p=json.load(open(sys.argv[1]))
assert p['read_only'] and len(p['applied'])==67
assert [m['version'] for m in p['pending']]==list(range(68,76)), p
PY
expect_failure 'permission denied' env DATABASE_URL="postgres://img01_reader@localhost/$DB?host=$SOCKET_DIR" "$TMP_DIR/dbmigrate" -dir db/migrations
[[ "$(ledger_hash)" == "$before" ]]
"$TMP_DIR/dbmigrate" -dir db/migrations
"$TMP_DIR/dbmigrate" -dir db/migrations
[[ "$(ledger_hash)" == "$before" ]]
"$TMP_DIR/dbmigrate" -plan-only -dir db/migrations > "$TMP_DIR/postcheck.json"
python3 - "$TMP_DIR/postcheck.json" <<'PY'
import json,sys
p=json.load(open(sys.argv[1]))
assert p['read_only'] and len(p['applied'])==75 and p['pending']==[], p
PY
sql -q <<'SQL'
DO $$ BEGIN
 IF has_database_privilege('espacigo_runtime', current_database(), 'TEMPORARY')
 OR NOT has_database_privilege('img01_reader',current_database(),'CONNECT')
 OR NOT has_function_privilege('espacigo_privacy_cleanup_worker','public.claim_private_media_cleanup(integer,timestamptz)','EXECUTE')
 OR has_function_privilege('espacigo_runtime','public.claim_private_media_cleanup(integer,timestamptz)','EXECUTE')
 OR has_table_privilege('espacigo_privacy_cleanup_worker','public.media_asset','UPDATE')
 OR NOT has_column_privilege('espacigo_privacy_cleanup_worker','public.media_asset','fencing_token','SELECT')
 THEN RAISE EXCEPTION 'V73 worker separation failed'; END IF;
END $$;
SQL
# Exercise the actual SQL/JSON serialization boundary, not a Python-only mock.
sql -qAt -f /dev/stdin < db/query/img01_schema_release_evidence.sql > "$TMP_DIR/schema-report.json"
python3 - "$TMP_DIR/schema-report.json" <<'PYREPORT'
from pathlib import Path
import hashlib,json,sys
p=Path(sys.argv[1])
report=json.loads(p.read_text())
report['query_sha256']=hashlib.sha256(Path('db/query/img01_schema_release_evidence.sql').read_bytes()).hexdigest()
worker_edges=[m for m in report['memberships'] if m['granted_role']=='espacigo_privacy_cleanup_worker']
assert len(worker_edges)==1 and type(worker_edges[0]['grantor_oid']) is int, worker_edges
p.write_text(json.dumps(report))
PYREPORT
python3 scripts/verify_img01_schema_release_evidence.py "$TMP_DIR/schema-report.json" --migrations db/migrations --query db/query/img01_schema_release_evidence.sql
docker exec "$NAME" psql -X -U fixture_admin -d postgres -v ON_ERROR_STOP=1 -q -c 'ALTER ROLE postgres SUPERUSER'
# Also exercise the explicitly supported absent-V64-routine repair variant.
# Recreate only this test DB, without changing any retained resource.
docker exec "$NAME" dropdb -U postgres "$DB"
docker exec "$NAME" psql -X -U fixture_admin -d postgres -v ON_ERROR_STOP=1 -q -c 'DROP ROLE espacigo_privacy_cleanup_worker'
docker exec "$NAME" createdb -U postgres "$DB"
"$TMP_DIR/dbmigrate" -dir "$TMP_DIR/v67"
before="$(ledger_hash)"
sql -q <<'SQL'
DROP FUNCTION public.finalize_pending_suppression_cleanup(uuid,timestamptz);
DROP OWNED BY espacigo_privacy_cleanup_worker;
DROP ROLE espacigo_privacy_cleanup_worker;
SQL
sql -q -f /dev/stdin < db/repair/reconcile_v64_cleanup_worker_role.sql
"$TMP_DIR/dbmigrate" -dir db/migrations
[[ "$(ledger_hash)" == "$before" ]]
"$TMP_DIR/dbmigrate" -plan-only -dir db/migrations > /dev/null
docker exec "$NAME" dropdb -U postgres "$DB"
docker exec "$NAME" psql -X -U fixture_admin -d postgres -v ON_ERROR_STOP=1 -q -c 'DROP ROLE espacigo_privacy_cleanup_worker'
DB=img01_fresh_install
docker exec "$NAME" createdb -U postgres "$DB"
export DATABASE_URL="postgres://postgres@localhost/$DB?host=$SOCKET_DIR"
"$TMP_DIR/dbmigrate" -dir db/migrations
"$TMP_DIR/dbmigrate" -plan-only -dir db/migrations > "$TMP_DIR/fresh.json"
python3 - "$TMP_DIR/fresh.json" <<'PY'
import json,sys
p=json.load(open(sys.argv[1]))
assert len(p['applied'])==75 and p['pending']==[], p
PY
# Exact upgrade from the current remote baseline, including old consent history.
DB=auth_v74_upgrade
docker exec "$NAME" createdb -U postgres "$DB"
export DATABASE_URL="postgres://postgres@localhost/$DB?host=$SOCKET_DIR"
"$TMP_DIR/dbmigrate" -dir "$TMP_DIR/v74"
before74="$(sql -qAt -c "SELECT md5(jsonb_agg(to_jsonb(m) ORDER BY version)::text) FROM public.schema_migrations m")"
sql -q <<'SQL'
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
VALUES ('90000000-0000-4000-8000-000000000001','history@example.invalid','history@example.invalid','synthetic-hash','activo');
INSERT INTO public.aceptacion_terminos(id,usuario_id,version_id,canal)
VALUES ('90000000-0000-4000-8000-000000000002','90000000-0000-4000-8000-000000000001','00000000-0000-4000-8000-000000000001','web');
SQL
"$TMP_DIR/dbmigrate" -dir db/migrations
[[ "$(sql -qAt -c "SELECT md5(jsonb_agg(to_jsonb(m) ORDER BY version)::text) FROM public.schema_migrations m WHERE version<=74")" == "$before74" ]]
sql -q <<'SQL'
DO $$ BEGIN
 IF (SELECT count(*) FROM public.aceptacion_terminos WHERE id='90000000-0000-4000-8000-000000000002' AND version_id='00000000-0000-4000-8000-000000000001')<>1
 OR (SELECT count(*) FROM public.version_terminos WHERE codigo IN ('evaluacion-terminos-20261023-v1','evaluacion-privacidad-20261023-v1'))<>2
 THEN RAISE EXCEPTION 'consent version/history upgrade failed'; END IF;
END $$;
SQL
# Existing integration tests cover plan-only without ledger and history guards.
export TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR"
go test ./internal/migrator -run 'Test(RunnerAppliesFromEmptyAndRepeatIsNoop|RunnerRefusesToInitializeLedgerBesideExistingApplicationObjects|PlanOnlyDoesNotInitializeLedgerOrExecutePendingMigrations)$' -count=1
# Account recovery/verification regressions use a shared NOLOGIN role and own
# fresh databases. This cluster is removed only by this runner's EXIT trap.
go test ./internal/adapters/postgres/identity -run 'Test(AuthRegistrationTermsDuplicateAndUnverifiedLogin|RegistrationAPIRequiresNoBusinessPreferenceAndGrantsNoRole|AuthVerificationReissueFailureLimitTTLAndReplay|AuthConcurrentVerificationHasOneWinnerAndLoginCountsAllFailures|AuthVerificationActivationRollsBackAndDeliveryFailureIsRetryable|AuthRecoveryAndPasswordChangeRevokeSessions|AuthRecoveryTokenStopsAfterFiveFailuresWithoutBlockingLogin|PasswordResetRevalidatesExpiredTokenAfterAccountLockWait|RecoveryDeliveryFailureIsObservableWithoutDisclosingAccount|ReadableConsentEndpointChecksPublishedContentHash)$' -count=1
echo 'PASS: isolated V1-V75, V67 upgrade with V64 drift (routine present/absent), repair rejection guards, immutable V1-V67 ledger, read-only plan, DDL denial for reader, worker privileges, repeat no-op.'
