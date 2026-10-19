#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W5_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo 'Unsafe run id.' >&2; exit 2; }
PROJECT="espacigo-gcp04-w5-${RUN_ID,,}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w5.XXXXXX")"
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
    "labels":{"com.espacigo.scope":"gcp04-w5-role-test","com.espacigo.run":run_id}
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
CREATE ROLE gcp04_w5_actor LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
SQL

# Run the exact desired operation inside a disposable transaction. PostgreSQL
# creates the role and permits its V60 comment, but records the implicit ADMIN
# membership as granted by the bootstrap superuser. Neither ordinary REVOKE nor
# a CREATEROLE actor impersonating that grantor can remove that edge.
compose exec -T postgres psql -X -U gcp04_w5_actor -d espacigo <<'SQL'
\set ON_ERROR_STOP on
BEGIN;
CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
COMMENT ON ROLE espacigo_gadm_bootstrap IS 'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.';
DO $$
DECLARE m record;
BEGIN
  SELECT gr.rolname AS granted, mr.rolname AS member, ar.rolname AS grantor, am.admin_option
    INTO m
    FROM pg_catalog.pg_auth_members am
    JOIN pg_catalog.pg_roles gr ON gr.oid=am.roleid
    JOIN pg_catalog.pg_roles mr ON mr.oid=am.member
    JOIN pg_catalog.pg_roles ar ON ar.oid=am.grantor
   WHERE gr.rolname='espacigo_gadm_bootstrap' AND mr.rolname=session_user;
  IF NOT FOUND OR NOT m.admin_option OR m.grantor <> 'postgres' THEN
    RAISE EXCEPTION 'unexpected PostgreSQL 18 CREATEROLE membership semantics: %', row_to_json(m);
  END IF;
  IF shobj_description((SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'),'pg_authid') IS DISTINCT FROM
     'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' THEN
    RAISE EXCEPTION 'COMMENT ON ROLE failed to set the V60 contract';
  END IF;
END $$;
REVOKE espacigo_gadm_bootstrap FROM gcp04_w5_actor;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_catalog.pg_auth_members m
    JOIN pg_catalog.pg_roles r ON r.oid=m.roleid
    JOIN pg_catalog.pg_roles u ON u.oid=m.member
    WHERE r.rolname='espacigo_gadm_bootstrap' AND u.rolname='gcp04_w5_actor' AND m.admin_option
  ) THEN
    RAISE EXCEPTION 'ordinary REVOKE unexpectedly removed the bootstrap-granted ADMIN edge';
  END IF;
END $$;
\set ON_ERROR_STOP off
REVOKE espacigo_gadm_bootstrap FROM gcp04_w5_actor GRANTED BY postgres;
\if :ERROR
\echo 'PASS: explicit GRANTED BY postgres is rejected for NOSUPERUSER CREATEROLE actor.'
\else
\echo 'FAIL: actor unexpectedly revoked bootstrap-granted edge.'
\quit 1
\endif
\set ON_ERROR_STOP on
ROLLBACK;
SQL

psql_local -U postgres -d espacigo <<'SQL'
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap') THEN
    RAISE EXCEPTION 'failed attempted bootstrap transaction left GADM role behind';
  END IF;
  IF (SELECT rolsuper OR NOT rolcreaterole FROM pg_catalog.pg_roles WHERE rolname='gcp04_w5_actor') THEN
    RAISE EXCEPTION 'controlled test actor was not NOSUPERUSER CREATEROLE';
  END IF;
END $$;
SQL

expect_failure() {
  local expected="$1" output="$TMP_DIR/failure.log" status=0
  compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U gcp04_w5_actor -d espacigo \
    < "$ROOT_DIR/db/bootstrap/gcp04-runtime/004_gadm_global_role.sql" > "$output" 2>&1 || status=$?
  if [[ "$status" -eq 0 ]]; then cat "$output" >&2; echo 'Expected existing-role contract failure.' >&2; exit 1; fi
  if ! grep -Fq "$expected" "$output"; then cat "$output" >&2; echo "Failure did not match expected contract guard: $expected" >&2; exit 1; fi
  echo 'Expected fail-closed existing-role case confirmed.'
}

# Successful candidate: atomic temporary CREATEROLE intermediary, exact
# comment, no residual memberships/ownership/settings, and exact rerun no-op.
psql_local -U gcp04_w5_actor -d espacigo < "$ROOT_DIR/db/bootstrap/gcp04-runtime/004_gadm_global_role.sql"
BEFORE_RERUN="$(psql_local -U postgres -d espacigo -Atc "SELECT (SELECT count(*) FROM pg_catalog.pg_roles)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_auth_members)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_db_role_setting)::text")"
psql_local -U gcp04_w5_actor -d espacigo < "$ROOT_DIR/db/bootstrap/gcp04-runtime/004_gadm_global_role.sql"
AFTER_RERUN="$(psql_local -U postgres -d espacigo -Atc "SELECT (SELECT count(*) FROM pg_catalog.pg_roles)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_auth_members)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_db_role_setting)::text")"
[[ "$BEFORE_RERUN" == "$AFTER_RERUN" ]] || { echo 'Exact rerun changed role, membership, or role-setting counts.' >&2; exit 1; }

psql_local -U postgres -d espacigo -c 'ALTER ROLE espacigo_gadm_bootstrap LOGIN' >/dev/null
expect_failure 'existing espacigo_gadm_bootstrap has unexpected role attributes'
psql_local -U postgres -d espacigo -Atc "SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'" | grep -qx t
psql_local -U postgres -d espacigo -c 'ALTER ROLE espacigo_gadm_bootstrap NOLOGIN' >/dev/null

psql_local -U postgres -d espacigo -c "COMMENT ON ROLE espacigo_gadm_bootstrap IS 'unexpected fixture comment'" >/dev/null
expect_failure 'existing espacigo_gadm_bootstrap has unexpected comment'
psql_local -U postgres -d espacigo -Atc "SELECT shobj_description(oid,'pg_authid') FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'" | grep -qx 'unexpected fixture comment'
psql_local -U postgres -d espacigo -c "COMMENT ON ROLE espacigo_gadm_bootstrap IS 'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.'" >/dev/null

psql_local -U postgres -d espacigo <<'SQL' >/dev/null
CREATE ROLE gcp04_w5_member_fixture NOLOGIN;
GRANT gcp04_w5_member_fixture TO espacigo_gadm_bootstrap;
SQL
expect_failure 'existing espacigo_gadm_bootstrap has role memberships'
psql_local -U postgres -d espacigo -c 'REVOKE gcp04_w5_member_fixture FROM espacigo_gadm_bootstrap' >/dev/null
psql_local -U postgres -d espacigo -c 'GRANT espacigo_gadm_bootstrap TO gcp04_w5_member_fixture' >/dev/null
expect_failure 'existing espacigo_gadm_bootstrap has role memberships'
psql_local -U postgres -d espacigo -c 'REVOKE espacigo_gadm_bootstrap FROM gcp04_w5_member_fixture' >/dev/null
psql_local -U postgres -d espacigo -c 'CREATE SCHEMA gcp04_w5_owned AUTHORIZATION espacigo_gadm_bootstrap' >/dev/null
expect_failure 'existing espacigo_gadm_bootstrap owns database objects'
psql_local -U postgres -d espacigo -c 'DROP SCHEMA gcp04_w5_owned' >/dev/null
psql_local -U postgres -d espacigo -c 'DROP ROLE gcp04_w5_member_fixture' >/dev/null

echo 'PASS: PG18 direct REVOKE limitation reproduced; intermediary helper creates/comments GADM atomically, removes temporary memberships, passes exact rerun, and rejects unexpected attributes/comment/memberships/ownership.'
