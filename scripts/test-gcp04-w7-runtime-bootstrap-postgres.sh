#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cmp -s "$ROOT_DIR/db/bootstrap/gcp04-runtime/005_cloud_bootstrap.sql" "$ROOT_DIR/cmd/gcp04w7bootstrap/bootstrap.sql" || { echo 'Embedded and reviewed bootstrap SQL differ.' >&2; exit 1; }
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W7_BOOTSTRAP_TEST_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo 'Unsafe run id.' >&2; exit 2; }
PROJECT="espacigo-gcp04-w7-bootstrap-${RUN_ID,,}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w7-bootstrap.XXXXXX")"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
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
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL image is not present; refusing pull/substitute.' >&2; exit 1; }
[[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]] || { echo 'Disposable Compose project already exists.' >&2; exit 1; }

python3 - "$COMPOSE_FILE" "$IMAGE" "$CONTAINER" "$RUN_ID" <<'PY'
import json, sys
from pathlib import Path
compose_file, image, container, run_id = sys.argv[1:]
Path(compose_file).write_text(json.dumps({"services":{"postgres":{
    "image":image,"container_name":container,"network_mode":"none",
    "command":["postgres","-c","listen_addresses=*"],
    "tmpfs":["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
    "environment":{"POSTGRES_HOST_AUTH_METHOD":"trust"},
    "labels":{"com.espacigo.scope":"gcp04-w7-runtime-bootstrap-test","com.espacigo.run":run_id}
}}}),encoding="utf-8")
PY
compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$CONTAINER" <<'PY'
import json, sys
from pathlib import Path
s=json.loads(Path(sys.argv[1]).read_text())["services"]["postgres"]
assert s["image"]==sys.argv[2] and s["container_name"]==sys.argv[3]
assert s["network_mode"]=="none" and not s.get("ports")
assert any(x.startswith("/var/lib/postgresql:") for x in s["tmpfs"])
print("Disposable PostgreSQL 18 fixture resolved: pinned image, isolated network, tmpfs data.")
PY

compose up -d postgres >/dev/null
for _ in $(seq 1 240); do
  ready="$(compose exec -T postgres psql -X -At -U postgres -d postgres -c "SELECT current_setting('listen_addresses')" 2>/dev/null || true)"
  pid1="$(compose exec -T postgres cat /proc/1/comm 2>/dev/null || true)"
  [[ "$ready" == '*' && "$pid1" == 'postgres' ]] && break
  sleep 0.25
done
[[ "$ready" == '*' && "$pid1" == 'postgres' ]] || { echo 'Final PostgreSQL server did not become ready.' >&2; exit 1; }
psql_local() { compose exec -T postgres psql -X -v ON_ERROR_STOP=1 "$@"; }
psql_local -U postgres -d postgres <<'SQL' >/dev/null
-- Local fixture models Cloud SQL's managed extension capability with a
-- synthetic SUPERUSER role; Cloud SQL never grants that PostgreSQL attribute.
CREATE ROLE cloudsqlsuperuser NOLOGIN SUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
CREATE ROLE espacigo_gcp04_w7_bootstrap LOGIN NOSUPERUSER CREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
GRANT cloudsqlsuperuser TO espacigo_gcp04_w7_bootstrap WITH ADMIN FALSE, INHERIT TRUE, SET TRUE;
CREATE DATABASE espacigo OWNER cloudsqlsuperuser;
SQL
psql_local -U postgres -d espacigo <<'SQL' >/dev/null
SQL
psql_local -U espacigo_gcp04_w7_bootstrap -d espacigo < "$ROOT_DIR/db/bootstrap/gcp04-runtime/005_cloud_bootstrap.sql"

[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT pg_catalog.pg_get_userbyid(datdba) FROM pg_catalog.pg_database WHERE datname='espacigo'")" == 'cloudsqlsuperuser' ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT pg_catalog.pg_get_userbyid(nspowner) FROM pg_catalog.pg_namespace WHERE nspname='public'")" == 'pg_database_owner' ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname='espacigo_w7_bootstrap_executor'")" == '0' ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_auth_members m ON m.roleid=r.oid WHERE r.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_gadm_bootstrap')")" == '0' ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_database d,LATERAL aclexplode(COALESCE(d.datacl,acldefault('d',d.datdba))) a WHERE d.datname='espacigo' AND a.grantee=0 AND a.privilege_type='TEMPORARY'")" == '0' ]]
[[ "$(psql_local -U postgres -d espacigo -Atc "SELECT count(*) FROM pg_catalog.pg_extension e JOIN pg_catalog.pg_roles r ON r.oid=e.extowner WHERE e.extname IN ('postgis','btree_gist','unaccent') AND r.rolname='cloudsqlsuperuser'")" == '3' ]]
echo 'W7 runtime bootstrap passed on the isolated PostgreSQL 18 fixture.'
