#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W3_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
if [[ ! "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; then
  echo 'GCP04_W3_RUN_ID must contain 1-64 safe filename characters.' >&2
  exit 2
fi
RUN_SLUG="${RUN_ID,,}"
RUN_SLUG="${RUN_SLUG//./-}"
PROJECT="espacigo-gcp04-w3-role-${RUN_SLUG}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w3.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
mkdir -m 0777 "$SOCKET_DIR"
chmod 0700 "$TMP_DIR"

compose() {
  docker compose --project-directory "$ROOT_DIR" --project-name "$PROJECT" -f "$COMPOSE_FILE" "$@"
}
cleanup_best_effort() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
cleanup_verified() {
  compose down --volumes --remove-orphans >/dev/null
  if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]]; then
    echo "Disposable Compose containers remain for project $PROJECT." >&2
    return 1
  fi
  if [[ -n "$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT")" ]]; then
    echo "Disposable Compose volumes remain for project $PROJECT." >&2
    return 1
  fi
  rm -rf -- "$TMP_DIR"
}
trap cleanup_best_effort EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}')"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL 18/PostGIS image ID mismatch; refusing to pull or substitute an image.' >&2; exit 1; }
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]]; then
  echo "Compose project already exists; refusing to reuse it: $PROJECT" >&2
  exit 1
fi

python3 - "$COMPOSE_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" "$RUN_ID" <<'PY'
import json
import sys
from pathlib import Path

compose_file, image, container, socket_dir, run_id = sys.argv[1:]
config = {
    "services": {
        "postgres": {
            "image": image,
            "container_name": container,
            "network_mode": "none",
            "tmpfs": ["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
            "volumes": [{
                "type": "bind",
                "source": socket_dir,
                "target": "/var/run/postgresql",
                "bind": {"create_host_path": False},
            }],
            "environment": {"POSTGRES_HOST_AUTH_METHOD": "trust"},
            "labels": {"com.espacigo.scope": "gcp04-w3-role-test", "com.espacigo.run": run_id},
        }
    }
}
Path(compose_file).write_text(json.dumps(config), encoding="utf-8")
PY

compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" <<'PY'
import json
import sys
from pathlib import Path

resolved = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
service = resolved["services"]["postgres"]
expected_image, expected_container, expected_socket = sys.argv[2:]
assert service["image"] == expected_image
assert service["container_name"] == expected_container
assert service["network_mode"] == "none"
assert any(item.startswith("/var/lib/postgresql:") for item in service["tmpfs"])
assert len(service["volumes"]) == 1 and service["volumes"][0]["type"] == "bind"
assert service["volumes"][0]["source"] == expected_socket
assert service["volumes"][0]["target"] == "/var/run/postgresql"
assert not service.get("ports")
assert service["environment"]["POSTGRES_HOST_AUTH_METHOD"] == "trust"
print("Resolved W3 test Compose: pinned PostgreSQL 18 image; no TCP ports/network; data in container tmpfs; run-scoped Unix socket only.")
PY

compose up -d postgres >/dev/null
CONTAINER_ID="$(compose ps -q postgres)"
[[ -n "$CONTAINER_ID" ]] || { echo 'Disposable PostgreSQL container did not start.' >&2; exit 1; }
docker inspect "$CONTAINER_ID" | python3 -c 'import json,sys; c=json.load(sys.stdin)[0]; assert c["HostConfig"]["NetworkMode"] == "none"; assert "/var/lib/postgresql" in c["HostConfig"]["Tmpfs"]; assert all(m["Type"] == "bind" for m in c["Mounts"]); assert not c["HostConfig"].get("PortBindings"); print("Runtime isolation verified.")'
for _ in $(seq 1 120); do
  if compose exec -T postgres pg_isready -U postgres -d postgres >/dev/null 2>&1; then
    break
  fi
  sleep 0.25
done
compose exec -T postgres pg_isready -U postgres -d postgres >/dev/null

compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres -c 'CREATE DATABASE espacigo' >/dev/null
compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  < "$ROOT_DIR/db/bootstrap/gcp04-runtime/001_pre_migrate_roles.sql"

cd "$ROOT_DIR"
timeout --signal=TERM --kill-after=10s 120s env -u DATABASE_URL \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR" \
  go test -p 1 -count=1 -v \
  -run '^TestRunnerAppliesDDLUnderRestrictedSetRoleMembership$' \
  ./internal/migrator

MIGRATIONS_V059="$TMP_DIR/migrations-v059"
mkdir -m 0700 "$MIGRATIONS_V059"
python3 - "$ROOT_DIR/db/migrations" "$MIGRATIONS_V059" <<'PY'
import re
import shutil
import sys
from pathlib import Path

source, target = map(Path, sys.argv[1:])
files = sorted(source.glob("V*.sql"))
assert len(files) == 64
for path in files:
    match = re.match(r"V(\d{6})__", path.name)
    assert match, path.name
    if int(match.group(1)) <= 59:
        shutil.copyfile(path, target / path.name)
PY

DATABASE_URL="postgres://postgres@localhost:5432/espacigo?host=$SOCKET_DIR" \
DATABASE_SET_ROLE=espacigo_schema_owner \
  go run ./cmd/dbmigrate -dir "$MIGRATIONS_V059"

set +e
MIGRATION_FAILURE="$(DATABASE_URL="postgres://postgres@localhost:5432/espacigo?host=$SOCKET_DIR" \
DATABASE_SET_ROLE=espacigo_schema_owner \
  go run ./cmd/dbmigrate -dir db/migrations 2>&1)"
MIGRATION_STATUS=$?
set -e
if [[ "$MIGRATION_STATUS" -eq 0 ]] || [[ "$MIGRATION_FAILURE" != *V000060* ]] || [[ "$MIGRATION_FAILURE" != *"permission denied"* ]]; then
  echo 'Expected PostgreSQL 18 restricted-role failure at V60 was not reproduced.' >&2
  exit 1
fi
echo 'Confirmed: restricted schema-owner execution stops at V60 COMMENT ON ROLE; full GCP bootstrap remains gated.'

compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
CREATE ROLE espacigo_api_login NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_worker_login NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_migrator_login NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_observer_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
GRANT espacigo_db_observer TO espacigo_observer_login WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;

CREATE ROLE w3_cloudsql_style_creator CREATEROLE;
SET ROLE w3_cloudsql_style_creator;
CREATE ROLE w3_cloudsql_style_gadm_probe NOLOGIN;
RESET ROLE;
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_catalog.pg_auth_members m
    JOIN pg_catalog.pg_roles granted_role ON granted_role.oid=m.roleid
    JOIN pg_catalog.pg_roles member_role ON member_role.oid=m.member
    WHERE granted_role.rolname='w3_cloudsql_style_gadm_probe'
      AND member_role.rolname='w3_cloudsql_style_creator' AND m.admin_option
  ) THEN
    RAISE EXCEPTION 'PG18 CREATEROLE self-admin membership behavior was not observed';
  END IF;
END
$$;
DROP ROLE w3_cloudsql_style_gadm_probe;
DROP ROLE w3_cloudsql_style_creator;
SQL

compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
DO $$
BEGIN
  IF (SELECT tableowner FROM pg_catalog.pg_tables WHERE schemaname='public' AND tablename='schema_migrations') <> 'espacigo_schema_owner' THEN
    RAISE EXCEPTION 'migration ledger is not owned by schema owner';
  END IF;
  IF NOT has_table_privilege('espacigo_runtime', 'public.usuario', 'SELECT') THEN
    RAISE EXCEPTION 'runtime table grants are missing';
  END IF;
  IF has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'SELECT') THEN
    RAISE EXCEPTION 'runtime can read migration history';
  END IF;
  IF NOT has_table_privilege('espacigo_db_observer', 'public.schema_migrations', 'SELECT')
     OR has_table_privilege('espacigo_db_observer', 'public.usuario', 'SELECT') THEN
    RAISE EXCEPTION 'observer grants exceed the migration metadata allowlist';
  END IF;
  IF has_database_privilege('espacigo_runtime', 'espacigo', 'TEMP')
     OR has_database_privilege('espacigo_privacy_cleanup_worker', 'espacigo', 'TEMP') THEN
    RAISE EXCEPTION 'runtime or worker retains TEMP privilege';
  END IF;
  IF NOT has_database_privilege('espacigo_schema_owner', 'espacigo', 'TEMP') THEN
    RAISE EXCEPTION 'migration owner lacks the TEMP privilege required by V57';
  END IF;
  IF NOT has_database_privilege('espacigo_schema_owner', 'espacigo', 'TEMP') THEN
    RAISE EXCEPTION 'migration owner lacks V57 TEMP capability';
  END IF;
END
$$;
SQL

compose exec -T postgres psql -X -v ON_ERROR_STOP=1 -U espacigo_observer_login -d espacigo \
  < "$ROOT_DIR/db/query/gcp04_runtime_private_preflight.sql"

cleanup_verified
trap - EXIT
