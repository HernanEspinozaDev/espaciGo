#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W7_B1_OBSERVER_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo 'Unsafe run id.' >&2; exit 2; }
PROJECT="espacigo-gcp04-w7-b1-observer-${RUN_ID,,}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w7-b1-observer.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
mkdir -m 0777 "$SOCKET_DIR"
chmod 0700 "$TMP_DIR"

compose() { docker compose --project-directory "$ROOT_DIR" --project-name "$PROJECT" -f "$COMPOSE_FILE" "$@"; }
cleanup() {
  local status=$?
  if [[ "$status" -ne 0 ]]; then compose logs postgres >&2 || true; fi
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  python3 - "$TMP_DIR" <<'PY'
import shutil, sys
shutil.rmtree(sys.argv[1], ignore_errors=True)
PY
  return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}')"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL 18/PostGIS image unavailable; refusing pull or substitute.' >&2; exit 1; }
[[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]] || { echo 'Disposable Compose project already exists.' >&2; exit 1; }

python3 - "$COMPOSE_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" "$RUN_ID" <<'PY'
import json, sys
from pathlib import Path
compose_file, image, container, socket_dir, run_id = sys.argv[1:]
Path(compose_file).write_text(json.dumps({"services":{"postgres":{
    "image":image,"container_name":container,"network_mode":"none",
    "command":["postgres","-c","listen_addresses=*","-c","max_connections=40"],
    "tmpfs":["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
    "volumes":[{"type":"bind","source":socket_dir,"target":"/var/run/postgresql","bind":{"create_host_path":False}}],
    "environment":{"POSTGRES_HOST_AUTH_METHOD":"trust"},
    "labels":{"com.espacigo.scope":"gcp04-w7-b1-observer-preflight-test","com.espacigo.run":run_id}
}}}),encoding="utf-8")
PY
compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" <<'PY'
import json, sys
from pathlib import Path
s=json.loads(Path(sys.argv[1]).read_text())["services"]["postgres"]
assert s["image"]==sys.argv[2] and s["container_name"]==sys.argv[3]
assert s["network_mode"]=="none" and not s.get("ports")
assert any(x.startswith("/var/lib/postgresql:") for x in s["tmpfs"])
assert s["volumes"][0]["source"]==sys.argv[4] and s["volumes"][0]["target"]=="/var/run/postgresql"
print("Disposable W7 B1 observer fixture resolved: pinned image, network none, tmpfs data, private Unix socket.")
PY

compose up -d postgres >/dev/null
ready=""
pid1=""
for _ in $(seq 1 240); do
  ready="$(compose exec -T postgres psql -X -At -U postgres -d postgres -c "SELECT current_setting('listen_addresses')" 2>/dev/null || true)"
  pid1="$(compose exec -T postgres cat /proc/1/comm 2>/dev/null || true)"
  [[ "$ready" == '*' && "$pid1" == 'postgres' ]] && break
  sleep 0.25
done
[[ "$ready" == '*' && "$pid1" == 'postgres' ]] || { echo 'Final PostgreSQL 18 server did not become ready.' >&2; exit 1; }
psql_local() { compose exec -T postgres psql -X -v ON_ERROR_STOP=1 "$@"; }

# Synthetic roles model only the reviewed B1 contract and inherited grant
# options. cloudsqlsuperuser is deliberately NOT a PostgreSQL SUPERUSER here.
psql_local -U postgres -d postgres <<'SQL' >/dev/null
CREATE ROLE cloudsqlsuperuser NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
CREATE ROLE cloudsqlsuperuser_base NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
GRANT cloudsqlsuperuser_base TO cloudsqlsuperuser WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
CREATE ROLE espacigo_gcp04_w7_bootstrap LOGIN NOSUPERUSER CREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
GRANT cloudsqlsuperuser TO espacigo_gcp04_w7_bootstrap WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
CREATE DATABASE espacigo;
SQL
psql_local -U postgres -d postgres -c 'GRANT CONNECT ON DATABASE espacigo TO cloudsqlsuperuser_base WITH GRANT OPTION' >/dev/null
psql_local -U postgres -d espacigo <<'SQL' >/dev/null
REVOKE TEMP ON DATABASE espacigo FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO cloudsqlsuperuser_base WITH GRANT OPTION;
CREATE TABLE public.schema_migrations(version integer PRIMARY KEY);
GRANT SELECT ON TABLE public.schema_migrations TO cloudsqlsuperuser_base WITH GRANT OPTION;
CREATE TABLE public.usuario(id integer);
CREATE EXTENSION btree_gist;
CREATE EXTENSION postgis;
CREATE EXTENSION unaccent;
CREATE ROLE espacigo_schema_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_privacy_cleanup_worker NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
SQL

psql_local -U espacigo_gcp04_w7_bootstrap -d espacigo \
  < "$ROOT_DIR/planning/gcp/sql_candidates/gcp04_w7_b1_observer_group_prepare_candidate.sql" >/dev/null
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname='espacigo_gcp04_w7_observer_executor'")" == 0 ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles u ON u.oid=m.member WHERE r.rolname='espacigo_db_observer' OR u.rolname='espacigo_db_observer'")" == 0 ]]

# Model the later API-created observer membership locally. This is a fixture,
# not evidence that Cloud SQL users.insert sets these membership options.
psql_local -U postgres -d postgres <<'SQL' >/dev/null
CREATE ROLE espacigo_gcp04_w7_observer LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
SQL
psql_local -U postgres -d espacigo \
  -c 'GRANT espacigo_db_observer TO espacigo_gcp04_w7_observer WITH ADMIN FALSE, INHERIT TRUE, SET FALSE' >/dev/null

GCP04_W7_TEST_DATABASE_USER=espacigo_gcp04_w7_observer \
GCP04_W7_TEST_DSN="postgres://espacigo_gcp04_w7_observer@localhost/espacigo?host=$SOCKET_DIR" \
GCP04_W7_TEST_QUERY_FILE="$ROOT_DIR/db/query/gcp04_w4_target_preflight.sql" \
GCP04_W7_TEST_REQUIRED_CONNECTIONS=1 \
  go test ./cmd/gcp04preflight -run TestLocalObserverReadOnlyIntegration -count=1 -v

compose down --volumes --remove-orphans >/dev/null
[[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]]
[[ -z "$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT")" ]]
echo 'W7 B1 observer bootstrap and exact W4 preflight passed on the isolated PostgreSQL 18 fixture.'
