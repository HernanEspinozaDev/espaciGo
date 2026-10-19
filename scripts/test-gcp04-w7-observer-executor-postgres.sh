#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W7_EXECUTOR_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo 'Unsafe run id.' >&2; exit 2; }
PROJECT="espacigo-gcp04-w7-executor-${RUN_ID,,}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w7-executor.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
mkdir -m 0777 "$SOCKET_DIR"
chmod 0700 "$TMP_DIR"
compose() { docker compose --project-directory "$ROOT_DIR" --project-name "$PROJECT" -f "$COMPOSE_FILE" "$@"; }
cleanup() { compose down --volumes --remove-orphans >/dev/null 2>&1 || true; rm -rf -- "$TMP_DIR"; }
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}')"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL 18 image is not present; refusing pull/substitute.' >&2; exit 1; }
[[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]] || { echo 'Disposable Compose project already exists.' >&2; exit 1; }

python3 - "$COMPOSE_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" "$RUN_ID" <<'PY'
import json, sys
from pathlib import Path
compose_file, image, container, socket_dir, run_id = sys.argv[1:]
Path(compose_file).write_text(json.dumps({"services":{"postgres":{
    "image":image,"container_name":container,"network_mode":"none",
    "command":["postgres","-c","listen_addresses=*"],
    "tmpfs":["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
    "volumes":[{"type":"bind","source":socket_dir,"target":"/var/run/postgresql","bind":{"create_host_path":False}}],
    "environment":{"POSTGRES_HOST_AUTH_METHOD":"trust"},
    "labels":{"com.espacigo.scope":"gcp04-w7-observer-executor-test","com.espacigo.run":run_id}
}}}),encoding="utf-8")
PY
compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" <<'PY'
import json, sys
from pathlib import Path
s=json.loads(Path(sys.argv[1]).read_text())["services"]["postgres"]
assert s["image"]==sys.argv[2] and s["container_name"]==sys.argv[3]
assert s["network_mode"]=="none" and s["command"]==["postgres","-c","listen_addresses=*"]
assert any(x.startswith("/var/lib/postgresql:") for x in s["tmpfs"])
assert s["volumes"][0]["source"]==sys.argv[4] and s["volumes"][0]["target"]=="/var/run/postgresql"
assert not s.get("ports")
print("Disposable PostgreSQL 18 fixture resolved: pinned image, isolated network, tmpfs data, private Unix socket.")
PY

compose up -d postgres >/dev/null
for _ in $(seq 1 240); do
  ready="$(compose exec -T postgres psql -X -At -U postgres -d postgres -c "SELECT current_setting('listen_addresses')" 2>/dev/null || true)"
  [[ "$ready" == '*' ]] && break
  sleep 0.25
done
[[ "$ready" == '*' ]] || { echo 'Final PostgreSQL server did not become ready.' >&2; exit 1; }
psql_local() { compose exec -T postgres psql -X -v ON_ERROR_STOP=1 "$@"; }
psql_local -U postgres -d postgres <<'SQL' >/dev/null
CREATE DATABASE espacigo;
CREATE ROLE gcp04_w7_actor LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE gcp04_w7_unprivileged LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
GRANT CONNECT ON DATABASE espacigo TO gcp04_w7_actor WITH GRANT OPTION;
GRANT CONNECT ON DATABASE espacigo TO gcp04_w7_unprivileged;
SQL
psql_local -U postgres -d espacigo <<'SQL' >/dev/null
GRANT USAGE ON SCHEMA public TO gcp04_w7_actor WITH GRANT OPTION;
CREATE TABLE public.schema_migrations(version text PRIMARY KEY);
GRANT SELECT ON TABLE public.schema_migrations TO gcp04_w7_actor WITH GRANT OPTION;
SQL

# Model the stricter W7 post-bootstrap contract. The fresh-cluster W9 baseline
# has PUBLIC TEMP enabled, and prepare must refuse it without changing ACLs.
if psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_executor_prepare_candidate.sql" > "$TMP_DIR/public-temp.log" 2>&1; then
  echo 'W7 prepare unexpectedly accepted the fresh-cluster PUBLIC TEMP baseline.' >&2; exit 1
fi
grep -Fq 'W7 PUBLIC TEMP baseline is not satisfied' "$TMP_DIR/public-temp.log"
psql_local -U postgres -d espacigo -c 'REVOKE TEMP ON DATABASE espacigo FROM PUBLIC' >/dev/null
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT has_database_privilege('public','espacigo','TEMP')")" == f ]]

expect_failure() {
  local who="$1" expected="$2" output="$TMP_DIR/failure.log" status=0
  compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U "$who" -d espacigo \
    < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_executor_prepare_candidate.sql" > "$output" 2>&1 || status=$?
  if [[ "$status" -eq 0 ]]; then cat "$output" >&2; echo 'Expected W7 prepare failure.' >&2; exit 1; fi
  if ! grep -Fq "$expected" "$output"; then cat "$output" >&2; echo "Failure did not match guard: $expected" >&2; exit 1; fi
}
expect_verify_failure() {
  local expected="$1" output="$TMP_DIR/verify-failure.log" status=0
  psql_local -U gcp04_w7_actor -d espacigo \
    < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_verify_api_membership_candidate.sql" > "$output" 2>&1 || status=$?
  if [[ "$status" -eq 0 ]]; then cat "$output" >&2; echo 'Expected W7 verifier failure.' >&2; exit 1; fi
  if ! grep -Fq "$expected" "$output"; then cat "$output" >&2; echo "Verifier failure did not match: $expected" >&2; exit 1; fi
}

# Missing grant options fail before creating either reserved role.
expect_failure gcp04_w7_unprivileged 'W7 actor lacks database CONNECT or public schema USAGE grant option'
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor')")" == 0 ]]

# Prepare transaction creates the group under a temporary executor. Keep the
# executor only across the separately reconciled users.insert API boundary.
psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_executor_prepare_candidate.sql"
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor'")" == 1 ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid WHERE r.rolname='espacigo_db_observer'")" == 1 ]]
expect_verify_failure 'W7 API-created login is absent'

# If the API call is reconciled as absent, compensate while the executor still
# exists; the group is empty and only the original actor's ACL grants remain.
psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_compensate_candidate.sql"
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_db_observer','espacigo_gcp04_w7_observer_executor')")" == 0 ]]
expect_verify_failure 'W7 API-created login is absent'

psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_executor_prepare_candidate.sql"

# The API call is intentionally simulated. First model the ordinary PostgreSQL
# default (SET=true); the read-only W7 verifier must reject it. Only a fixture
# superuser then sets SET=false to test the verifier's positive path. This does
# not prove the Cloud SQL API creates the same membership flags.
psql_local -U postgres -d espacigo <<'SQL' >/dev/null
CREATE ROLE espacigo_gcp04_w7_observer LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
GRANT espacigo_db_observer TO espacigo_gcp04_w7_observer;
SQL
if compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U gcp04_w7_actor -d espacigo > "$TMP_DIR/executor-grant-dependency.log" 2>&1 <<'SQL'
BEGIN;
SET ROLE espacigo_gcp04_w7_observer_executor;
GRANT espacigo_db_observer TO espacigo_gcp04_w7_observer WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
RESET ROLE;
DROP ROLE espacigo_gcp04_w7_observer_executor;
COMMIT;
SQL
then
  echo 'Executor unexpectedly retired after becoming login membership grantor.' >&2; exit 1
fi
grep -Fq 'privileges for membership of role espacigo_gcp04_w7_observer in role espacigo_db_observer' "$TMP_DIR/executor-grant-dependency.log"
if psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_verify_api_membership_candidate.sql" > "$TMP_DIR/default-membership.log" 2>&1; then
  echo 'W7 verifier unexpectedly accepted default API-style SET=true membership.' >&2; exit 1
fi
grep -Fq 'W7 API membership must be exactly ADMIN=false INHERIT=true SET=false' "$TMP_DIR/default-membership.log"
psql_local -U postgres -d espacigo -c 'GRANT espacigo_db_observer TO espacigo_gcp04_w7_observer WITH ADMIN FALSE, INHERIT TRUE, SET FALSE' >/dev/null
psql_local -U postgres -d espacigo -c 'ALTER ROLE espacigo_gcp04_w7_observer NOINHERIT' >/dev/null
expect_verify_failure 'W7 API-created login must be LOGIN INHERIT with elevated attributes disabled'
psql_local -U postgres -d espacigo -c 'ALTER ROLE espacigo_gcp04_w7_observer INHERIT' >/dev/null
psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_verify_api_membership_candidate.sql"

psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_executor_retire_success_candidate.sql"
psql_local -U gcp04_w7_actor -d espacigo < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_observer_verify_api_membership_candidate.sql"
psql_local -U postgres -d espacigo <<'SQL'
DO $$
DECLARE membership record;
BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor') THEN
        RAISE EXCEPTION 'temporary executor survived successful prepare';
    END IF;
    SELECT m.admin_option, m.inherit_option, m.set_option, g.rolname AS granted, u.rolname AS member
      INTO membership
      FROM pg_catalog.pg_auth_members m
      JOIN pg_catalog.pg_roles g ON g.oid=m.roleid
      JOIN pg_catalog.pg_roles u ON u.oid=m.member
     WHERE g.rolname='espacigo_db_observer';
    IF NOT FOUND OR membership.granted <> 'espacigo_db_observer'
       OR membership.member <> 'espacigo_gcp04_w7_observer'
       OR membership.admin_option OR NOT membership.inherit_option OR membership.set_option THEN
        RAISE EXCEPTION 'final observer membership is not exact: %', row_to_json(membership);
    END IF;
    IF has_database_privilege('espacigo_db_observer','espacigo','TEMP')
       OR NOT has_database_privilege('espacigo_gcp04_w7_observer','espacigo','CONNECT')
       OR has_database_privilege('espacigo_gcp04_w7_observer','espacigo','CREATE')
       OR NOT has_schema_privilege('espacigo_gcp04_w7_observer','public','USAGE')
       OR has_schema_privilege('espacigo_gcp04_w7_observer','public','CREATE')
       OR NOT has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','SELECT') THEN
        RAISE EXCEPTION 'observer effective ACL differs from the read-only contract';
    END IF;
    IF has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','INSERT')
       OR has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','UPDATE')
       OR has_table_privilege('espacigo_gcp04_w7_observer','public.schema_migrations','DELETE') THEN
        RAISE EXCEPTION 'observer gained ledger write access';
    END IF;
END $$;
SQL

# After users.delete, the original CREATEROLE actor cannot drop the group: its
# ADMIN membership belonged to the removed executor. Preserve this as a real
# lifecycle limitation, rather than claiming full cleanup.
psql_local -U postgres -d espacigo -c 'DROP ROLE espacigo_gcp04_w7_observer' >/dev/null
expect_verify_failure 'W7 API-created login is absent'
if psql_local -U gcp04_w7_actor -d espacigo -c 'DROP ROLE espacigo_db_observer' > "$TMP_DIR/post-success-drop.log" 2>&1; then
  echo 'Actor unexpectedly dropped observer after executor retirement.' >&2; exit 1
fi
grep -Fq 'may drop this role' "$TMP_DIR/post-success-drop.log"

echo 'PASS: verifier handles missing login with executor present/absent, checks LOGIN INHERIT, and passes before/after executor retirement; absent-API compensation works; executor-granted login edge blocks executor retirement; later group retirement needs an ADMIN steward.'
