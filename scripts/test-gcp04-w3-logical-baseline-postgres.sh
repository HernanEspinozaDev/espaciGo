#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RETAIN_DIR="${GCP04_W3_RETAIN_BUNDLE_DIR:-}"
if [[ -n "$RETAIN_DIR" ]]; then
  [[ "$RETAIN_DIR" == /* ]] || { echo 'GCP04_W3_RETAIN_BUNDLE_DIR must be an absolute private .local/testing path.' >&2; exit 2; }
  TESTING_DIR="$(realpath -m -- "$ROOT_DIR/.local/testing")"
  RETAIN_DIR="$(realpath -m -- "$RETAIN_DIR")"
  case "$RETAIN_DIR" in
    "$TESTING_DIR"/gcp04-w4-baseline-*) ;;
    *) echo 'GCP04_W3_RETAIN_BUNDLE_DIR must be a new gcp04-w4-baseline-* directory under .local/testing.' >&2; exit 2 ;;
  esac
  [[ "$(basename -- "$RETAIN_DIR")" =~ ^gcp04-w4-baseline-[A-Za-z0-9._-]{1,64}$ ]] || { echo 'Private bundle directory name is invalid.' >&2; exit 2; }
  [[ ! -e "$RETAIN_DIR" && ! -L "$RETAIN_DIR" ]] || { echo 'Refusing to overwrite an existing retained baseline bundle.' >&2; exit 2; }
fi
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W3_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
if [[ ! "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; then
  echo 'GCP04_W3_RUN_ID must contain 1-64 safe filename characters.' >&2
  exit 2
fi
RUN_SLUG="${RUN_ID,,}"
RUN_SLUG="${RUN_SLUG//./-}"
PROJECT="espacigo-gcp04-w3-baseline-${RUN_SLUG}"
SOURCE_CONTAINER="${PROJECT}-source"
TARGET_CONTAINER="${PROJECT}-target"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w3-baseline.XXXXXX")"
SOURCE_SOCKET="$TMP_DIR/source-socket"
TARGET_SOCKET="$TMP_DIR/target-socket"
ARTIFACTS="$TMP_DIR/artifacts"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
mkdir -m 0777 "$SOURCE_SOCKET" "$TARGET_SOCKET"
mkdir -m 0700 "$ARTIFACTS"
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

python3 - "$COMPOSE_FILE" "$IMAGE" "$SOURCE_CONTAINER" "$TARGET_CONTAINER" "$SOURCE_SOCKET" "$TARGET_SOCKET" "$ARTIFACTS" "$RUN_ID" <<'PY'
import json
import sys
from pathlib import Path

compose_file, image, source_container, target_container, source_socket, target_socket, artifacts, run_id = sys.argv[1:]
def service(container, socket, side):
    return {
        "image": image,
        "container_name": container,
        "network_mode": "none",
        "command": ["postgres", "-c", "listen_addresses=*", "-c", "max_connections=40"],
        "tmpfs": ["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
        "volumes": [
            {"type": "bind", "source": socket, "target": "/var/run/postgresql", "bind": {"create_host_path": False}},
            {"type": "bind", "source": artifacts, "target": "/w3-artifacts", "bind": {"create_host_path": False}},
        ],
        "environment": {"POSTGRES_HOST_AUTH_METHOD": "trust"},
        "labels": {"com.espacigo.scope": "gcp04-w3-logical-baseline", "com.espacigo.run": run_id, "com.espacigo.side": side},
    }

Path(compose_file).write_text(json.dumps({"services": {
    "source": service(source_container, source_socket, "canonical-source"),
    "target": service(target_container, target_socket, "restricted-restore-target"),
}}), encoding="utf-8")
PY

compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$SOURCE_CONTAINER" "$TARGET_CONTAINER" "$SOURCE_SOCKET" "$TARGET_SOCKET" "$ARTIFACTS" <<'PY'
import json
import sys
from pathlib import Path

resolved = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
image, source_container, target_container, source_socket, target_socket, artifacts = sys.argv[2:]
assert set(resolved["services"]) == {"source", "target"}
for name, container, socket in (("source", source_container, source_socket), ("target", target_container, target_socket)):
    service = resolved["services"][name]
    assert service["image"] == image and service["container_name"] == container
    assert service["network_mode"] == "none" and service["command"] == ["postgres", "-c", "listen_addresses=*", "-c", "max_connections=40"]
    assert any(item.startswith("/var/lib/postgresql:") for item in service["tmpfs"])
    assert len(service["volumes"]) == 2 and all(item["type"] == "bind" for item in service["volumes"])
    assert {item["source"] for item in service["volumes"]} == {socket, artifacts}
    assert not service.get("ports")
    assert service["environment"]["POSTGRES_HOST_AUTH_METHOD"] == "trust"
print("Resolved baseline Compose: two pinned PostgreSQL 18 clusters; no network or TCP ports; data in tmpfs; private run-scoped Unix sockets.")
PY

compose up -d source target >/dev/null
for service in source target; do
  container_id="$(compose ps -q "$service")"
  [[ -n "$container_id" ]] || { echo "Disposable $service PostgreSQL container did not start." >&2; exit 1; }
  docker inspect "$container_id" | python3 -c 'import json,sys; c=json.load(sys.stdin)[0]; assert c["HostConfig"]["NetworkMode"] == "none"; assert "/var/lib/postgresql" in c["HostConfig"]["Tmpfs"]; assert len(c["Mounts"]) == 2 and all(m["Type"] == "bind" for m in c["Mounts"]); assert not c["HostConfig"].get("PortBindings"); print("Runtime isolation verified.")'
  for _ in $(seq 1 240); do
    listen_addresses="$(compose exec -T "$service" psql -X -At -U postgres -d postgres -c "SELECT quote_literal(current_setting('listen_addresses'))" 2>/dev/null || true)"
    if [[ "$listen_addresses" == "'*'" ]]; then break; fi
    sleep 0.25
  done
  [[ "$listen_addresses" == "'*'" ]] || { echo "PostgreSQL $service final server listen_addresses was [$listen_addresses], expected ['*']." >&2; exit 1; }
done

BOOTSTRAP_ACTOR='gcp04_w6_bootstrap_actor'
UNPRIVILEGED_ACTOR='gcp04_w6_unprivileged_actor'
OVERPRIVILEGED_ACTOR='gcp04_w6_overprivileged_actor'
BOOTSTRAP_SQL="$ROOT_DIR/db/bootstrap/gcp04-runtime/004_gadm_global_role.sql"

expect_helper_failure() {
  local side="$1" actor="$2" expected="$3" input="$BOOTSTRAP_SQL"
  local output="$ARTIFACTS/${side}-${actor}-bootstrap-failure.log" status=0
  if [[ "$4" != '-' ]]; then input="$4"; fi
  compose exec -T "$side" psql -X -v ON_ERROR_STOP=1 -U "$actor" -d espacigo \
    < "$input" > "$output" 2>&1 || status=$?
  if [[ "$status" -eq 0 ]] || ! grep -Fq "$expected" "$output"; then
    cat "$output" >&2
    echo "Expected GADM bootstrap rejection [$expected] for $actor on $side." >&2
    return 1
  fi
}

compose exec -T source psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres -c 'CREATE DATABASE espacigo' >/dev/null
compose exec -T source psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres \
  -c "CREATE ROLE $BOOTSTRAP_ACTOR LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT" >/dev/null
compose exec -T source psql -X -v ON_ERROR_STOP=1 -U "$BOOTSTRAP_ACTOR" -d espacigo < "$BOOTSTRAP_SQL" >/dev/null
compose exec -T source psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  < "$ROOT_DIR/db/bootstrap/gcp04-runtime/001_pre_migrate_roles.sql" >/dev/null

cd "$ROOT_DIR"
env -u DATABASE_SET_ROLE DATABASE_URL="postgres://postgres@localhost:5432/espacigo?host=$SOURCE_SOCKET" \
  go run ./cmd/dbmigrate -dir db/migrations

compose exec -T source psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  < "$ROOT_DIR/db/bootstrap/gcp04-runtime/003_local_source_owner_normalization.sql" >/dev/null
DATABASE_URL="postgres://postgres@localhost:5432/espacigo?host=$SOURCE_SOCKET" \
DATABASE_SET_ROLE=espacigo_schema_owner go run ./cmd/dbmigrate -dir db/migrations | tee "$ARTIFACTS/source-no-changes.txt"
grep -Fxq 'sin cambios' "$ARTIFACTS/source-no-changes.txt"
compose exec -T source psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c 'REVOKE espacigo_schema_owner FROM postgres' >/dev/null
compose exec -T source psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  < "$ROOT_DIR/db/bootstrap/gcp04-runtime/002_local_restore_role_contract.sql" >/dev/null
compose exec -T source psql -X -At -U postgres -d espacigo -c "
  SELECT c.relname || ':' || COALESCE(c.relacl::text, '<null>')
  FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND c.relname='verificacion_evidencia_sintetica'"

compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres -c 'CREATE DATABASE espacigo' >/dev/null
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres <<SQL >/dev/null
CREATE ROLE $BOOTSTRAP_ACTOR LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE $UNPRIVILEGED_ACTOR LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE $OVERPRIVILEGED_ACTOR LOGIN NOSUPERUSER CREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
SQL

# Wrong identities fail before any global role change.
expect_helper_failure target postgres 'GADM bootstrap requires a direct LOGIN NOSUPERUSER CREATEROLE actor with other elevated attributes disabled' -
expect_helper_failure target "$UNPRIVILEGED_ACTOR" 'GADM bootstrap requires a direct LOGIN NOSUPERUSER CREATEROLE actor with other elevated attributes disabled' -
expect_helper_failure target "$OVERPRIVILEGED_ACTOR" 'GADM bootstrap requires a direct LOGIN NOSUPERUSER CREATEROLE actor with other elevated attributes disabled' -

# A reserved-name collision is not repaired or adopted.
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo -c \
  'CREATE ROLE espacigo_gadm_bootstrap_executor NOLOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT' >/dev/null
expect_helper_failure target "$BOOTSTRAP_ACTOR" 'unexpected pre-existing GADM bootstrap executor role' -
[[ "$(compose exec -T target psql -X -At -U postgres -d espacigo -c "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap'")" == 0 ]]
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo -c 'DROP ROLE espacigo_gadm_bootstrap_executor' >/dev/null

# Inject a failure after helper creation/assertions but before COMMIT. Closing
# the failed psql transaction must roll back GADM and the transient executor.
python3 - "$BOOTSTRAP_SQL" "$ARTIFACTS/gadm-bootstrap-failure-injected.sql" <<'PY'
import sys
from pathlib import Path
source, target = map(Path, sys.argv[1:])
sql=source.read_text(encoding="utf-8")
assert sql.rstrip().endswith("COMMIT;")
sql=sql.rstrip()[:-len("COMMIT;")] + "SELECT 1 / 0;\nCOMMIT;\n"
target.write_text(sql, encoding="utf-8")
PY
expect_helper_failure target "$BOOTSTRAP_ACTOR" 'division by zero' "$ARTIFACTS/gadm-bootstrap-failure-injected.sql"
ROLLBACK_ROLE_COUNT="$(compose exec -T target psql -X -At -U postgres -d espacigo -c "SELECT count(*) FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_gadm_bootstrap','espacigo_gadm_bootstrap_executor')")"
ROLLBACK_EDGE_COUNT="$(compose exec -T target psql -X -At -U postgres -d espacigo -c "SELECT count(*) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles u ON u.oid=m.member WHERE r.rolname IN ('espacigo_gadm_bootstrap','espacigo_gadm_bootstrap_executor') OR u.rolname IN ('espacigo_gadm_bootstrap','espacigo_gadm_bootstrap_executor')")"
[[ "$ROLLBACK_ROLE_COUNT" == 0 && "$ROLLBACK_EDGE_COUNT" == 0 ]] || { echo 'Injected pre-COMMIT failure left a GADM/transient role or membership.' >&2; exit 1; }

# Actual target role bootstrap runs with the CREATEROLE actor before the
# superuser-owned local role/extension fixture. The fixture may validate and
# grant DB/schema ACLs, but cannot create or comment GADM.
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U "$BOOTSTRAP_ACTOR" -d espacigo < "$BOOTSTRAP_SQL" >/dev/null
GADM_COUNTS_BEFORE_RERUN="$(compose exec -T target psql -X -At -U postgres -d espacigo -c "SELECT (SELECT count(*) FROM pg_catalog.pg_roles)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_auth_members)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_db_role_setting)::text")"
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U "$BOOTSTRAP_ACTOR" -d espacigo < "$BOOTSTRAP_SQL" >/dev/null
GADM_COUNTS_AFTER_RERUN="$(compose exec -T target psql -X -At -U postgres -d espacigo -c "SELECT (SELECT count(*) FROM pg_catalog.pg_roles)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_auth_members)::text || ':' || (SELECT count(*) FROM pg_catalog.pg_db_role_setting)::text")"
[[ "$GADM_COUNTS_BEFORE_RERUN" == "$GADM_COUNTS_AFTER_RERUN" ]] || { echo 'Exact helper rerun broadened role, membership, or role-setting state.' >&2; exit 1; }

# Per-role settings are unexpected state and must remain untouched on abort.
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c "ALTER ROLE espacigo_gadm_bootstrap SET statement_timeout = '5s'" >/dev/null
expect_helper_failure target "$BOOTSTRAP_ACTOR" 'existing espacigo_gadm_bootstrap has role settings' -
[[ "$(compose exec -T target psql -X -At -U postgres -d espacigo -c "SELECT count(*) FROM pg_catalog.pg_db_role_setting s JOIN pg_catalog.pg_roles r ON r.oid=s.setrole WHERE r.rolname='espacigo_gadm_bootstrap'")" == 1 ]]
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c 'ALTER ROLE espacigo_gadm_bootstrap RESET statement_timeout' >/dev/null

compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  < "$ROOT_DIR/db/bootstrap/gcp04-runtime/002_local_restore_role_contract.sql" >/dev/null
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL' >/dev/null
CREATE EXTENSION btree_gist;
CREATE EXTENSION postgis;
CREATE EXTENSION unaccent;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE TEMP ON DATABASE espacigo FROM PUBLIC;
GRANT TEMP ON DATABASE espacigo TO espacigo_schema_owner;
SQL

# pg_dump is deliberately database-scoped; roles and memberships are created
# independently above. The custom archive is kept only in this private tmpfs-backed run.
compose exec -T source pg_dump -U postgres -d espacigo -Fc -f /w3-artifacts/baseline.dump
compose exec -T source pg_restore --list /w3-artifacts/baseline.dump > "$ARTIFACTS/baseline.toc.original"
compose exec -T source psql -X -At -U postgres -d espacigo -c "
  SELECT DISTINCT n.nspname || '.' || c.relname
  FROM pg_catalog.pg_depend d
  JOIN pg_catalog.pg_extension e ON d.refclassid = 'pg_catalog.pg_extension'::regclass AND d.refobjid = e.oid
  JOIN pg_catalog.pg_class c ON d.classid = 'pg_catalog.pg_class'::regclass AND d.objid = c.oid
  JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
  WHERE d.deptype = 'e' AND c.relkind IN ('r', 'p')
  ORDER BY 1" > "$ARTIFACTS/extension-config-tables.txt"
python3 - "$ARTIFACTS/baseline.toc.original" "$ARTIFACTS/extension-config-tables.txt" "$ARTIFACTS/baseline.toc.restore" "$ARTIFACTS/baseline.toc.acl" "$ARTIFACTS/baseline.toc.bootstrap-acl" <<'PY'
import re
import sys
from pathlib import Path

toc_path, config_path, output_path, acl_path, bootstrap_acl_path = map(Path, sys.argv[1:])
extension_tables = {line.strip().split(".", 1)[1] for line in config_path.read_text(encoding="utf-8").splitlines() if line.strip()}
lines = toc_path.read_text(encoding="utf-8").splitlines()
kept, acl, skipped, bootstrap_acl = [], [], [], []
for line in lines:
    if line.lstrip().startswith(";"):
        kept.append(line)
        acl.append(line)
        continue
    extension_item = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+EXTENSION\s+-\s+", line))
    extension_comment = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+COMMENT\s+-\s+EXTENSION\s+", line))
    table_data = re.search(r"\bTABLE DATA\s+\S+\s+(\S+)", line)
    extension_data = bool(table_data and table_data.group(1) in extension_tables)
    acl_entry = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+(?:ACL|DEFAULT ACL)(?:\s+-|\s+)", line))
    schema_acl = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+ACL\s+\S+\s+SCHEMA\s+public\s+", line))
    if extension_item or extension_comment or extension_data:
        skipped.append(line)
    elif schema_acl:
        bootstrap_acl.append(line)
    elif acl_entry:
        acl.append(line)
    else:
        kept.append(line)
assert any(re.search(r"\bEXTENSION\s+-\s+postgis\b", line) for line in skipped), "expected extension TOC item was not filtered"
assert any(re.search(r"\bACL(?:\s+-|\s+)\S+", line) for line in acl), "archive contains no ACL TOC entries to preserve"
assert any(re.search(r"\bACL\s+\S+\s+SCHEMA\s+public\b", line) for line in bootstrap_acl), "expected schema ACL to be supplied by the explicit role contract"
assert not any(re.search(r"\b\d+;\s+\d+\s+\d+\s+(?:ROLE|USER)\s+", line, re.I) for line in lines), "global role entry must not be present in the archive TOC"
output_path.write_text("\n".join(kept) + "\n", encoding="utf-8")
acl_path.write_text("\n".join(acl) + "\n", encoding="utf-8")
Path(str(output_path) + ".skipped").write_text("\n".join(skipped) + "\n", encoding="utf-8")
bootstrap_acl_path.write_text("\n".join(bootstrap_acl) + "\n", encoding="utf-8")
PY

run_target_preflight() {
  local name="$1" expected="$2"
  shift 2
  set +e
  psql -X -qAt -v ON_ERROR_STOP=1 "$@" -h "$TARGET_SOCKET" -U espacigo_observer_login -d espacigo \
    -f "$ROOT_DIR/db/query/gcp04_w4_target_preflight.sql" \
    > "$ARTIFACTS/$name.json" 2> "$ARTIFACTS/$name.stderr"
  local status=$?
  set -e
  if [[ "$expected" == success && "$status" -ne 0 ]]; then
    cat "$ARTIFACTS/$name.stderr" >&2
    return "$status"
  elif [[ "$expected" == failure && "$status" -eq 0 ]]; then
    echo "preflight unexpectedly accepted invalid input: $name" >&2
    cat "$ARTIFACTS/$name.json" >&2
    return 1
  fi
}

# Reject missing, zero, and non-numeric connection budgets before querying catalog state.
run_target_preflight missing-budget failure
run_target_preflight zero-budget failure -v required_connections=0
run_target_preflight invalid-budget failure -v required_connections=abc
grep -Fq 'required_connections is required' "$ARTIFACTS/missing-budget.stderr"
grep -Fq 'required_connections must be a positive integer' "$ARTIFACTS/zero-budget.stderr"
grep -Fq 'required_connections must be a positive integer' "$ARTIFACTS/invalid-budget.stderr"

# Read-only preflight report against the empty restore target, through its isolated Unix socket.
run_target_preflight pre-restore-report success -v required_connections=24
python3 - "$ARTIFACTS/pre-restore-report.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
assert report["database"]["transaction_read_only"] is True
assert report["database"]["user"] == "espacigo_observer_login"
empty_counts = {key: report["destination"][key] for key in ("relations_non_extension", "routines_non_extension", "types_non_extension", "other_schema_objects_non_extension")}
if any(empty_counts.values()):
    raise AssertionError(f"empty destination has non-extension objects: {empty_counts}; relations={[row for row in report['destination']['relation_inventory'] if not row['extension_owned']]}")
assert report["destination"]["non_public_schemas_non_extension"] == 0
assert report["destination"]["non_public_objects_non_extension"] == 0
assert report["destination"]["unapproved_extensions"] == 0
assert report["destination"]["empty_destination_candidate"] is True
assert report["schema_migrations"]["present"] is False
assert report["capacity"]["required_connections"] == 24
PY

compose exec -T target pg_restore --exit-on-error --single-transaction --no-owner --no-acl \
  --role=espacigo_schema_owner -U espacigo_migrator_login -d espacigo \
  --use-list=/w3-artifacts/baseline.toc.restore /w3-artifacts/baseline.dump

compose exec -T source psql -X -At -U postgres -d espacigo -c \
  "SELECT count(*) FROM pg_catalog.pg_default_acl WHERE defaclrole='postgres'::regrole" > "$ARTIFACTS/default-acl-postgres-count.txt"
[[ "$(cat "$ARTIFACTS/default-acl-postgres-count.txt")" == 0 ]] || { echo 'Non-owner default privileges require an explicit reviewed role translation.' >&2; exit 1; }
compose exec -T source pg_restore \
  --use-list=/w3-artifacts/baseline.toc.acl \
  --file=/w3-artifacts/baseline.acl.raw.sql /w3-artifacts/baseline.dump
python3 - "$ARTIFACTS/baseline.acl.raw.sql" "$ARTIFACTS/baseline.acl.restore.sql" <<'PY'
import re
import sys
from pathlib import Path

source, target = map(Path, sys.argv[1:])
sql = source.read_text(encoding="utf-8")
roles = re.findall(r"(?im)^SET SESSION AUTHORIZATION\s+([^;]+);", sql)
if set(role.strip().strip('"') for role in roles) - {"espacigo_schema_owner"}:
    raise SystemExit("archive ACL uses a non-owner session authorization that has no reviewed non-superuser translation")
translated = "SET ROLE espacigo_schema_owner;\n" + re.sub(r"(?im)^SET SESSION AUTHORIZATION\s+([^;]+);", r"SET ROLE \1;", sql)
translated = re.sub(r"(?im)^RESET SESSION AUTHORIZATION;", "SET ROLE espacigo_schema_owner;", translated)
if re.search(r"(?i)SESSION AUTHORIZATION", translated):
    raise SystemExit("could not safely translate every ACL session authorization")
target.write_text(translated, encoding="utf-8")
PY
set +e
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U espacigo_migrator_login -d espacigo \
  -f /w3-artifacts/baseline.acl.restore.sql > "$ARTIFACTS/acl-restore.log" 2>&1
ACL_STATUS=$?
set -e
if [[ "$ACL_STATUS" -ne 0 ]]; then
  tail -n 24 "$ARTIFACTS/acl-restore.log" >&2
  exit "$ACL_STATUS"
fi

DATABASE_URL="postgres://espacigo_migrator_login@localhost:5432/espacigo?host=$TARGET_SOCKET" \
DATABASE_SET_ROLE=espacigo_schema_owner go run ./cmd/dbmigrate -dir db/migrations | tee "$ARTIFACTS/target-no-changes.txt"
grep -Fxq 'sin cambios' "$ARTIFACTS/target-no-changes.txt"

run_target_preflight post-restore-report success -v required_connections=24
python3 - "$ARTIFACTS/post-restore-report.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
assert report["database"]["transaction_read_only"] is True
assert report["destination"]["schema_migrations_present"] is True
assert report["schema_migrations"]["present"] is True
assert report["destination"]["relations_non_extension"] >= 98
assert report["destination"]["non_public_schemas_non_extension"] == 0
assert report["destination"]["non_public_objects_non_extension"] == 0
assert report["destination"]["unapproved_extensions"] == 0
assert report["destination"]["empty_destination_candidate"] is False
assert report["capacity"]["required_connections"] == 24
assert all(not row["missing_effective_privileges"] for row in report["owner_effective_privileges"])
PY

run_target_preflight insufficient-budget success -v required_connections=999999
python3 - "$ARTIFACTS/insufficient-budget.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
assert report["capacity"]["passes"] is False
PY

run_observer_preflight() {
  local name="$1"
  psql -X -qAt -v ON_ERROR_STOP=1 -v required_connections=24 \
    -h "$TARGET_SOCKET" -U espacigo_observer_login -d espacigo \
    -f "$ROOT_DIR/db/query/gcp04_w4_target_preflight.sql" \
    > "$ARTIFACTS/$name.json" 2> "$ARTIFACTS/$name.stderr"
}

# An unapproved schema and table outside public must be inventoried and close the empty-target gate.
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
CREATE SCHEMA foreign_app;
CREATE TABLE foreign_app.real_data (id integer PRIMARY KEY);
INSERT INTO foreign_app.real_data VALUES (7);
SQL
run_observer_preflight observer-foreign-schema
python3 - "$ARTIFACTS/observer-foreign-schema.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
destination = report["destination"]
assert destination["non_public_schemas_non_extension"] == 1
assert destination["non_public_objects_non_extension"] >= 1
assert destination["empty_destination_candidate"] is False
assert any(row["identity"] == "foreign_app.real_data" for row in destination["non_public_object_inventory"])
assert any(row["name"] == "foreign_app" and not row["extension_owned"] for row in destination["schema_inventory"])
PY
set +e
psql -X -q -v ON_ERROR_STOP=1 -h "$TARGET_SOCKET" -U espacigo_observer_login -d espacigo \
  -c 'SELECT * FROM foreign_app.real_data' > "$ARTIFACTS/observer-foreign-data-denial.log" 2>&1
FOREIGN_DATA_STATUS=$?
set -e
[[ "$FOREIGN_DATA_STATUS" -ne 0 ]] && grep -Fq 'permission denied for schema foreign_app' "$ARTIFACTS/observer-foreign-data-denial.log"

# Direct membership of a login in a powerful catalog role must be visible to the restricted observer.
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c 'GRANT pg_read_all_stats TO espacigo_observer_login' >/dev/null
run_observer_preflight observer-direct-stats-membership
python3 - "$ARTIFACTS/observer-direct-stats-membership.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
assert any(edge["app_login"] == "espacigo_observer_login" and edge["member_role"] == "espacigo_observer_login"
           and edge["granted_role"] == "pg_read_all_stats" and edge["edge_context"] == "transitive-login-membership"
           and edge["inherit_option"]
           for edge in report["memberships"])
PY
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c 'REVOKE pg_read_all_stats FROM espacigo_observer_login' >/dev/null

# The closure must also report a powerful role reached through a settable intermediate group.
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
CREATE ROLE gcp04_w4_inherit_group NOLOGIN;
CREATE ROLE gcp04_w4_set_group NOLOGIN;
GRANT pg_read_all_stats TO gcp04_w4_inherit_group;
GRANT pg_read_all_stats TO gcp04_w4_set_group;
GRANT gcp04_w4_inherit_group TO espacigo_observer_login WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
GRANT gcp04_w4_set_group TO espacigo_observer_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
SQL
run_observer_preflight observer-transitive-stats-membership
python3 - "$ARTIFACTS/observer-transitive-stats-membership.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding="utf-8"))
edges = report["memberships"]
assert any(edge["app_login"] == "espacigo_observer_login" and edge["member_role"] == "espacigo_observer_login"
           and edge["granted_role"] == "gcp04_w4_inherit_group" and edge["inherit_option"] and not edge["set_option"] for edge in edges)
assert any(edge["app_login"] == "espacigo_observer_login" and edge["member_role"] == "gcp04_w4_inherit_group"
           and edge["granted_role"] == "pg_read_all_stats" for edge in edges)
assert any(edge["app_login"] == "espacigo_observer_login" and edge["member_role"] == "espacigo_observer_login"
           and edge["granted_role"] == "gcp04_w4_set_group" and not edge["inherit_option"] and edge["set_option"] for edge in edges)
assert any(edge["app_login"] == "espacigo_observer_login" and edge["member_role"] == "gcp04_w4_set_group"
           and edge["granted_role"] == "pg_read_all_stats" for edge in edges)
PY
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
REVOKE gcp04_w4_inherit_group FROM espacigo_observer_login;
REVOKE gcp04_w4_set_group FROM espacigo_observer_login;
REVOKE pg_read_all_stats FROM gcp04_w4_inherit_group;
REVOKE pg_read_all_stats FROM gcp04_w4_set_group;
DROP ROLE gcp04_w4_inherit_group;
DROP ROLE gcp04_w4_set_group;
DROP SCHEMA foreign_app CASCADE;
SQL

# Checksum drift is tested by altering an existing ledger value, confirming the
# restricted migrator aborts, then restoring that exact prior value in this throwaway DB.
ORIGINAL_CHECKSUM="$(compose exec -T target psql -X -At -U postgres -d espacigo -c 'SELECT checksum FROM public.schema_migrations WHERE version=64')"
[[ "$ORIGINAL_CHECKSUM" =~ ^[0-9a-f]{64}$ ]]
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c "UPDATE public.schema_migrations SET checksum = repeat('0', 64) WHERE version = 64" >/dev/null
set +e
DRIFT_OUTPUT="$(DATABASE_URL="postgres://espacigo_migrator_login@localhost:5432/espacigo?host=$TARGET_SOCKET" \
  DATABASE_SET_ROLE=espacigo_schema_owner go run ./cmd/dbmigrate -dir db/migrations 2>&1)"
DRIFT_STATUS=$?
set -e
if [[ "$DRIFT_STATUS" -eq 0 || "$DRIFT_OUTPUT" != *'checksum drift for applied migration V000064'* ]]; then
  echo 'Restricted migrator did not reject the synthetic V64 checksum discrepancy.' >&2
  exit 1
fi
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo \
  -c "UPDATE public.schema_migrations SET checksum = '$ORIGINAL_CHECKSUM' WHERE version = 64" >/dev/null

compose exec -T target psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
DO $checks$
DECLARE gadm_oid oid;
BEGIN
    IF (SELECT count(*) FROM public.schema_migrations) <> 64
       OR (SELECT min(version) FROM public.schema_migrations) <> 1
       OR (SELECT max(version) FROM public.schema_migrations) <> 64 THEN
        RAISE EXCEPTION 'restored canonical history is not contiguous V1-V64';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted_role ON granted_role.oid=m.roleid
        JOIN pg_catalog.pg_roles member_role ON member_role.oid=m.member
        WHERE granted_role.rolname='espacigo_gadm_bootstrap' OR member_role.rolname='espacigo_gadm_bootstrap'
           OR m.admin_option
    ) THEN
        RAISE EXCEPTION 'target has ADMIN OPTION or GADM role membership';
    END IF;
    SELECT oid INTO gadm_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap';
    IF gadm_oid IS NULL OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
         WHERE oid=gadm_oid AND (rolcanlogin OR rolsuper OR rolcreatedb OR rolcreaterole
                                 OR rolreplication OR rolbypassrls OR rolinherit OR rolconnlimit <> -1)
    ) THEN
        RAISE EXCEPTION 'target GADM attributes differ from the helper contract';
    END IF;
    IF shobj_description(gadm_oid,'pg_authid') IS DISTINCT FROM
       'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' THEN
        RAISE EXCEPTION 'target GADM comment differs from V60';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_db_role_setting WHERE setrole=gadm_oid)
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_gadm_bootstrap_executor')
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_shdepend d WHERE d.refclassid='pg_catalog.pg_authid'::regclass
                   AND d.refobjid=gadm_oid AND d.deptype='o') THEN
        RAISE EXCEPTION 'target GADM has settings, transient executor, or owned objects';
    END IF;
    IF (SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls
        FROM pg_catalog.pg_roles WHERE rolname='espacigo_migrator_login') THEN
        RAISE EXCEPTION 'migrator login has elevated global attributes';
    END IF;
    IF NOT has_table_privilege('espacigo_schema_owner', 'public.schema_migrations', 'SELECT') THEN
        RAISE EXCEPTION 'schema owner cannot verify restored migration ledger';
    END IF;
END
$checks$;
SQL

# Prove all runtime identities use their individual LOGIN and SET ROLE edges,
# and that the most sensitive neighboring permissions remain denied.
compose exec -T target psql -X -At -U postgres -d espacigo -c "
  SELECT c.relname || ':' || COALESCE(c.relacl::text, '<null>')
  FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
  WHERE n.nspname='public' AND c.relname IN ('verificacion_evidencia_sintetica','schema_migrations')
  ORDER BY c.relname"
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U espacigo_api_login -d espacigo <<'SQL'
SET ROLE espacigo_runtime;
DO $checks$
BEGIN
  IF session_user <> 'espacigo_api_login' OR current_user <> 'espacigo_runtime' THEN RAISE EXCEPTION 'API login/role identity mismatch'; END IF;
  IF NOT has_table_privilege(current_user, 'public.usuario', 'SELECT') THEN RAISE EXCEPTION 'API runtime SELECT grant missing'; END IF;
  IF has_table_privilege(current_user, 'public.schema_migrations', 'SELECT') THEN RAISE EXCEPTION 'API runtime can read migration ledger'; END IF;
  IF has_database_privilege(current_user, current_database(), 'CREATE') THEN RAISE EXCEPTION 'API runtime can create database objects'; END IF;
END
$checks$;
SELECT count(*) FROM public.usuario;
RESET ROLE;
SQL
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U espacigo_privacy_worker_login -d espacigo <<'SQL'
SET ROLE espacigo_privacy_cleanup_worker;
DO $checks$
BEGIN
  IF session_user <> 'espacigo_privacy_worker_login' OR current_user <> 'espacigo_privacy_cleanup_worker' THEN RAISE EXCEPTION 'worker login/role identity mismatch'; END IF;
  IF NOT has_table_privilege(current_user, 'public.verificacion_evidencia_sintetica', 'DELETE') THEN RAISE EXCEPTION 'worker cleanup DELETE grant missing'; END IF;
  IF has_table_privilege(current_user, 'public.usuario', 'SELECT') THEN RAISE EXCEPTION 'worker can read account table'; END IF;
END
$checks$;
RESET ROLE;
SQL
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U espacigo_observer_login -d espacigo <<'SQL'
DO $checks$
BEGIN
  IF session_user <> 'espacigo_observer_login' OR current_user <> 'espacigo_observer_login' THEN RAISE EXCEPTION 'observer login identity mismatch'; END IF;
  IF NOT has_table_privilege(current_user, 'public.schema_migrations', 'SELECT') THEN RAISE EXCEPTION 'observer cannot read ledger'; END IF;
  IF has_table_privilege(current_user, 'public.usuario', 'SELECT') THEN RAISE EXCEPTION 'observer can read business data'; END IF;
END
$checks$;
SELECT count(*) FROM public.schema_migrations;
SQL
# Keep enough concurrent API LOGIN sessions to make a 24-connection proposed
# budget impossible on this disposable max_connections=40 server. PostgreSQL
# 18 hides backend_type for these other-user rows unless broad stats privileges
# are granted; the observer must count them as unclassified and subtract them.
capacity_probe_pids=()
for probe in $(seq 1 14); do
  compose exec -e PGAPPNAME=gcp04_w3_capacity_probe -T target psql -X -q -v ON_ERROR_STOP=1 \
    -U espacigo_api_login -d espacigo -c 'SELECT pg_catalog.pg_sleep(20)' \
    > "$ARTIFACTS/capacity-probe-${probe}.log" 2>&1 &
  capacity_probe_pids+=("$!")
done
for _ in $(seq 1 80); do
  probe_count="$(compose exec -T target psql -X -At -U postgres -d espacigo \
    -c "SELECT count(*) FROM pg_catalog.pg_stat_activity WHERE application_name = 'gcp04_w3_capacity_probe'" 2>/dev/null || true)"
  [[ "$probe_count" == 14 ]] && break
  sleep 0.25
done
[[ "$probe_count" == 14 ]] || { echo "Expected 14 concurrent API login probes, observed [$probe_count]." >&2; exit 1; }
compose exec -T target psql -X -q -A -t -F '|' -v ON_ERROR_STOP=1 -U espacigo_observer_login -d espacigo \
  < "$ROOT_DIR/db/query/gcp04_runtime_private_preflight.sql" > "$ARTIFACTS/target-private-preflight.txt"
python3 - "$ARTIFACTS/target-private-preflight.txt" <<'PY'
import sys
from pathlib import Path

rows = [line.split("|") for line in Path(sys.argv[1]).read_text(encoding="utf-8").splitlines()]
capacity = next((row for row in rows if len(row) == 6 and row[0].isdigit()), None)
assert capacity is not None, "preflight capacity row was not emitted"
maximum, reserved, super_reserved, visible, unclassified, lower_bound = map(int, capacity)
assert maximum == 40, f"unexpected isolated capacity fixture: {maximum}"
assert visible >= 1, f"observer did not see its own client backend: {visible}"
assert unclassified >= 14, f"observer omitted concurrent other-login rows: {unclassified}"
assert lower_bound == max(maximum - reserved - super_reserved - visible - unclassified, 0)
assert lower_bound < 24, f"capacity gate would incorrectly admit a 24-connection budget: {lower_bound}"
print(f"PG18 observer capacity regression passed: visible={visible}, unclassified={unclassified}, conservative_free_lower_bound={lower_bound} (<24).")
PY
for probe_pid in "${capacity_probe_pids[@]}"; do
  wait "$probe_pid"
done
compose exec -T target psql -X -v ON_ERROR_STOP=1 -U espacigo_migrator_login -d espacigo <<'SQL'
SET ROLE espacigo_schema_owner;
DO $checks$
BEGIN
  IF session_user <> 'espacigo_migrator_login' OR current_user <> 'espacigo_schema_owner' THEN RAISE EXCEPTION 'migrator login/role identity mismatch'; END IF;
  IF (SELECT rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls FROM pg_catalog.pg_roles WHERE rolname=current_user) THEN
    RAISE EXCEPTION 'effective migrator role has elevated global attributes';
  END IF;
END
$checks$;
CREATE TABLE public.w3_restore_ddl_probe (id integer);
DROP TABLE public.w3_restore_ddl_probe;
SQL

expect_denied() {
  local label="$1" login="$2" expected="$3" sql="$4" status=0
  set +e
  compose exec -T target psql -X -v ON_ERROR_STOP=1 -U "$login" -d espacigo -c "$sql" \
    > "$ARTIFACTS/${label}.denial.log" 2>&1
  status=$?
  set -e
  if [[ "$status" -eq 0 ]] || ! grep -Fq "$expected" "$ARTIFACTS/${label}.denial.log"; then
    tail -n 20 "$ARTIFACTS/${label}.denial.log" >&2
    echo "Expected denial did not occur for $label." >&2
    exit 1
  fi
}
expect_denied api-ledger espacigo_api_login 'permission denied for table schema_migrations' \
  'SET ROLE espacigo_runtime; SELECT * FROM public.schema_migrations'
expect_denied worker-account espacigo_privacy_worker_login 'permission denied for table usuario' \
  'SET ROLE espacigo_privacy_cleanup_worker; SELECT * FROM public.usuario'
expect_denied observer-business espacigo_observer_login 'permission denied for table usuario' \
  'SELECT * FROM public.usuario'
expect_denied migrator-create-role espacigo_migrator_login 'permission denied to create role' \
  'SET ROLE espacigo_schema_owner; CREATE ROLE gcp04_w3_forbidden_role'

# A schema-only archive comparison includes owners and ACL statements. Data
# is hashed table-by-table so extension configuration and application seed rows
# are accounted for separately.
compose exec -T source pg_dump -U postgres -d espacigo --schema-only > "$ARTIFACTS/source.schema.sql"
compose exec -T target pg_dump -U postgres -d espacigo --schema-only > "$ARTIFACTS/target.schema.sql"

python3 - "$ROOT_DIR" "$PROJECT" "$COMPOSE_FILE" "$ARTIFACTS" "$SOURCE_SOCKET" "$TARGET_SOCKET" <<'PY'
import hashlib
import json
import re
import subprocess
import sys
from pathlib import Path

root, project, compose_file, artifacts, source_socket, target_socket = sys.argv[1:]
artifacts = Path(artifacts)
def compose(*args):
    return ["docker", "compose", "--project-directory", root, "--project-name", project, "-f", compose_file, *args]

def psql(service, sql):
    result = subprocess.run(compose("exec", "-T", service, "psql", "-X", "-A", "-t", "-v", "ON_ERROR_STOP=1", "-U", "postgres", "-d", "espacigo", "-c", sql), capture_output=True)
    if result.returncode:
        raise SystemExit(f"catalog inventory query failed on {service}: {result.stderr.decode(errors='replace')}")
    return result.stdout.decode().strip()

def data_inventory(service):
    names = psql(service, "SELECT tablename FROM pg_catalog.pg_tables WHERE schemaname='public' ORDER BY tablename").splitlines()
    inventory = {}
    commands = []
    for index, name in enumerate(names):
        if not name or any(ch in name for ch in "\n\r\x00"):
            raise SystemExit("unexpected relation name in synthetic DB")
        escaped = name.replace('"', '""')
        outfile = f"/w3-artifacts/{service}-table-{index:04d}.copy"
        command = f'\\copy (SELECT row_to_json(t)::text FROM ONLY public."{escaped}" t ORDER BY 1) TO \'{outfile}\''
        commands.append(command)
    script = artifacts / f"{service}-inventory.psql"
    script.write_text("\n".join(commands) + "\n", encoding="utf-8")
    subprocess.run(compose("exec", "-T", service, "psql", "-X", "-U", "postgres", "-d", "espacigo", "-v", "ON_ERROR_STOP=1", "-f", f"/w3-artifacts/{script.name}"), check=True, stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
    for index, name in enumerate(names):
        local = artifacts / f"{service}-table-{index:04d}.copy"
        data = local.read_bytes()
        inventory[name] = {"rows": len(data.splitlines()), "sha256": hashlib.sha256(data).hexdigest()}
        local.unlink()
    script.unlink()
    return inventory

source_data = data_inventory("source")
target_data = data_inventory("target")
if source_data != target_data:
    source_only = sorted(set(source_data) - set(target_data))
    target_only = sorted(set(target_data) - set(source_data))
    changed = sorted(name for name in set(source_data) & set(target_data) if source_data[name] != target_data[name])
    raise SystemExit(f"source/destination table data inventory differs: missing={source_only} extra={target_only} changed={changed}")

migration_history_sql = """
SELECT jsonb_agg(jsonb_build_object('version',version,'name',name,'checksum',checksum,'runner_checksum',runner_checksum)
  ORDER BY version)::text
FROM public.schema_migrations
"""
migration_history = {service: json.loads(psql(service, migration_history_sql)) for service in ("source", "target")}
if migration_history["source"] != migration_history["target"] or len(migration_history["source"]) != 64:
    raise SystemExit("source/destination V1-V64 migration ledgers/checksums differ")

catalog_sql = r"""
SELECT jsonb_build_object(
  'database', (SELECT jsonb_build_object('name', datname, 'owner', pg_catalog.pg_get_userbyid(datdba)) FROM pg_catalog.pg_database WHERE datname=current_database()),
  'schema', (SELECT jsonb_build_object('name', nspname, 'owner', pg_catalog.pg_get_userbyid(nspowner)) FROM pg_catalog.pg_namespace WHERE nspname='public'),
  'roles', (SELECT jsonb_agg(jsonb_build_object('name',r.rolname,'login',r.rolcanlogin,'super',r.rolsuper,'createdb',r.rolcreatedb,'createrole',r.rolcreaterole,'replication',r.rolreplication,'bypassrls',r.rolbypassrls,'inherit',r.rolinherit,'comment',(SELECT d.description FROM pg_catalog.pg_shdescription d WHERE d.objoid=r.oid AND d.classoid='pg_catalog.pg_authid'::regclass)) ORDER BY r.rolname) FROM pg_catalog.pg_roles r WHERE left(r.rolname,9)='espacigo_'),
  'public_object_kinds', (SELECT jsonb_agg(jsonb_build_object('identity',n.nspname||'.'||c.relname,'kind',c.relkind,'owner',pg_catalog.pg_get_userbyid(c.relowner)) ORDER BY n.nspname,c.relname) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f','S')),
  'memberships', (SELECT jsonb_agg(jsonb_build_object('granted',r.rolname,'member',mbr.rolname,'admin',m.admin_option,'inherit',m.inherit_option,'set',m.set_option) ORDER BY r.rolname,mbr.rolname) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles r ON r.oid=m.roleid JOIN pg_catalog.pg_roles mbr ON mbr.oid=m.member),
  'extensions', (SELECT jsonb_agg(jsonb_build_object('name',extname,'version',extversion) ORDER BY extname) FROM pg_catalog.pg_extension),
  'security_definers', (SELECT jsonb_agg(jsonb_build_object('signature',p.oid::regprocedure::text,'owner',pg_catalog.pg_get_userbyid(p.proowner),'search_path',p.proconfig) ORDER BY p.oid::regprocedure::text) FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.prosecdef),
  'acl_entries', (SELECT jsonb_agg(jsonb_build_object('kind',a.kind,'identity',a.identity,'object_owner',pg_catalog.pg_get_userbyid(a.object_owner),'grantor',pg_catalog.pg_get_userbyid(a.grantor),'grantee',CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_catalog.pg_get_userbyid(a.grantee) END,'privilege',a.privilege_type,'grantable',a.is_grantable) ORDER BY a.kind,a.identity,a.object_owner,a.grantor,a.grantee,a.privilege_type,a.is_grantable) FROM (
    SELECT 'database'::text AS kind, d.datname::text AS identity, d.datdba AS object_owner, x.* FROM pg_catalog.pg_database d CROSS JOIN LATERAL pg_catalog.aclexplode(d.datacl) x WHERE d.datname=current_database() AND d.datacl IS NOT NULL
    UNION ALL
    SELECT 'schema', n.nspname, n.nspowner, x.* FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) x WHERE n.nspname='public' AND n.nspacl IS NOT NULL
    UNION ALL
    SELECT 'relation', n.nspname||'.'||c.relname, c.relowner, x.* FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) x WHERE n.nspname='public' AND c.relacl IS NOT NULL
    UNION ALL
    SELECT 'column', n.nspname||'.'||c.relname||'.'||a.attname, c.relowner, x.* FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) x WHERE n.nspname='public' AND a.attnum>0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
    UNION ALL
    SELECT 'routine', n.nspname||'.'||p.oid::regprocedure::text, p.proowner, x.* FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) x WHERE n.nspname='public' AND p.proacl IS NOT NULL
    UNION ALL
    SELECT 'type', n.nspname||'.'||t.typname, t.typowner, x.* FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(t.typacl) x WHERE n.nspname='public' AND t.typacl IS NOT NULL
    UNION ALL
    SELECT 'default', coalesce(n.nspname,'')||':'||d.defaclobjtype::text, d.defaclrole, x.* FROM pg_catalog.pg_default_acl d LEFT JOIN pg_catalog.pg_namespace n ON n.oid=d.defaclnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(d.defaclacl) x
  ) a)
)::text;
"""
catalogs = {}
for service in ("source", "target"):
    catalogs[service] = json.loads(psql(service, catalog_sql))
    (artifacts / f"{service}.catalog.json").write_text(json.dumps(catalogs[service], sort_keys=True, indent=2) + "\n", encoding="utf-8")
owner_acl_representation = {}
normalized_catalogs = {}
for service in ("source", "target"):
    normalized_catalogs[service] = dict(catalogs[service])
    acl_entries = catalogs[service]["acl_entries"] or []
    owner_entries = [entry for entry in acl_entries if entry["kind"] in {"relation", "column", "routine", "type"}
                     and entry["grantor"] == entry["object_owner"] and entry["grantee"] == entry["object_owner"]]
    effective_entries = [entry for entry in acl_entries if entry not in owner_entries]
    normalized_catalogs[service]["acl_entries"] = effective_entries
    owner_acl_representation[service] = owner_entries

owner_effective_sql = r"""
SELECT jsonb_agg(jsonb_build_object('kind',kind,'identity',identity,'owner',owner_name,
  'required_privileges',required_privileges,'missing_effective_privileges',missing_effective_privileges)
  ORDER BY kind,identity)::text
FROM (
  SELECT 'relation'::text AS kind, n.nspname||'.'||c.relname AS identity,
         pg_catalog.pg_get_userbyid(c.relowner) AS owner_name,
         spec.required_privileges,
         ARRAY(SELECT privilege FROM unnest(spec.required_privileges) privilege
               WHERE NOT CASE WHEN c.relkind='S'
                 THEN has_sequence_privilege(c.relowner,c.oid,privilege)
                 ELSE has_table_privilege(c.relowner,c.oid,privilege)
               END ORDER BY privilege) AS missing_effective_privileges
  FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
  CROSS JOIN LATERAL (SELECT CASE WHEN c.relkind='S'
    THEN ARRAY['SELECT','USAGE','UPDATE']::text[]
    ELSE ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER','MAINTAIN']::text[] END AS required_privileges) spec
  WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f','S')
  UNION ALL
  SELECT 'routine',n.nspname||'.'||p.oid::regprocedure::text,
         pg_catalog.pg_get_userbyid(p.proowner),ARRAY['EXECUTE']::text[],
         ARRAY(SELECT 'EXECUTE' WHERE NOT has_function_privilege(p.proowner,p.oid,'EXECUTE'))
  FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
  WHERE n.nspname='public'
  UNION ALL
  SELECT 'type',n.nspname||'.'||t.typname,pg_catalog.pg_get_userbyid(t.typowner),ARRAY['USAGE']::text[],
         ARRAY(SELECT 'USAGE' WHERE NOT has_type_privilege(t.typowner,t.oid,'USAGE'))
  FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
  WHERE n.nspname='public' AND t.typrelid=0 AND t.typtype IN ('b','d','e','r','m')
  UNION ALL
  SELECT 'schema',n.nspname,pg_catalog.pg_get_userbyid(n.nspowner),ARRAY['USAGE','CREATE']::text[],
         ARRAY(SELECT privilege FROM unnest(ARRAY['USAGE','CREATE']::text[]) privilege
               WHERE NOT has_schema_privilege(n.nspowner,n.oid,privilege) ORDER BY privilege)
  FROM pg_catalog.pg_namespace n WHERE n.nspname='public'
  UNION ALL
  SELECT 'database',d.datname,pg_catalog.pg_get_userbyid(d.datdba),ARRAY['CONNECT','CREATE','TEMP']::text[],
         ARRAY(SELECT privilege FROM unnest(ARRAY['CONNECT','CREATE','TEMP']::text[]) privilege
               WHERE NOT has_database_privilege(d.datdba,d.oid,privilege) ORDER BY privilege)
  FROM pg_catalog.pg_database d WHERE d.datname=current_database()
) owner_privileges
"""
owner_acl_effective = {}
for service in ("source", "target"):
    owner_acl_effective[service] = json.loads(psql(service, owner_effective_sql)) or []
    incomplete = [entry for entry in owner_acl_effective[service] if entry["missing_effective_privileges"]]
    if incomplete:
        raise SystemExit(f"owner lacks effective full privileges on {service} objects: {[entry['identity'] for entry in incomplete]}")

object_kinds = {item["identity"]: item["kind"] for item in catalogs["source"]["public_object_kinds"] or []}
owner_acl_groups = {}
for service in ("source", "target"):
    groups = {}
    for entry in owner_acl_representation[service]:
        key = (entry["kind"], entry["identity"], entry["object_owner"])
        groups.setdefault(key, []).append(entry)
    for (kind, identity, owner), entries in groups.items():
        privilege_names = {entry["privilege"] for entry in entries}
        if kind == "relation":
            required = ({"SELECT", "USAGE", "UPDATE"} if object_kinds.get(identity) == "S" else
                        {"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"})
        elif kind == "column":
            required = {"SELECT", "INSERT", "UPDATE", "REFERENCES"}
        elif kind == "routine":
            required = {"EXECUTE"}
        elif kind == "type":
            required = {"USAGE"}
        else:
            raise SystemExit(f"unexpected explicit owner ACL kind: {kind}")
        if privilege_names != required:
            raise SystemExit(f"explicit owner ACL is partial or unexpected for {service} {identity}: {sorted(privilege_names)}")
    owner_acl_groups[service] = groups

def owner_entry_key(entry):
    return (entry["kind"], entry["identity"], entry["object_owner"], entry["grantor"],
            entry["grantee"], entry["privilege"], entry["grantable"])
source_owner_entries = {owner_entry_key(entry) for entry in owner_acl_representation["source"]}
target_owner_entries = {owner_entry_key(entry) for entry in owner_acl_representation["target"]}
owner_acl_differences = {
    "source_only": [entry for entry in owner_acl_representation["source"] if owner_entry_key(entry) not in target_owner_entries],
    "target_only": [entry for entry in owner_acl_representation["target"] if owner_entry_key(entry) not in source_owner_entries],
}
owner_acl_effective_normalized = {}
for service in ("source", "target"):
    owner_acl_effective_normalized[service] = [
        {key: value for key, value in item.items() if key != "owner"}
        for item in owner_acl_effective[service]
    ]
if owner_acl_effective_normalized["source"] != owner_acl_effective_normalized["target"]:
    raise SystemExit("source/destination effective owner privileges differ")
if normalized_catalogs["source"] != normalized_catalogs["target"]:
    source_json = json.dumps(normalized_catalogs["source"], sort_keys=True, indent=2).splitlines()
    target_json = json.dumps(normalized_catalogs["target"], sort_keys=True, indent=2).splitlines()
    diff = "\n".join(__import__("difflib").unified_diff(source_json, target_json, fromfile="source-catalog", tofile="target-catalog")) + "\n"
    print(diff[:10000], file=sys.stderr)
    raise SystemExit("source/destination role, membership, owner, ACL, extension, or SECURITY DEFINER inventories differ")
gadm_role = next((role for role in normalized_catalogs["target"]["roles"]
                  if role["name"] == "espacigo_gadm_bootstrap"), None)
expected_gadm_comment = "NOLOGIN GADM local bootstrap group; temporary isolated login membership only."
if gadm_role is None or gadm_role["comment"] != expected_gadm_comment:
    raise SystemExit("target GADM role comment differs from the exact V60 contract")
(artifacts / "source.catalog.json").write_text(
    json.dumps(normalized_catalogs["source"], sort_keys=True, indent=2) + "\n", encoding="utf-8")

constraint_sql = """
SELECT jsonb_agg(jsonb_build_object('identity',n.nspname||'.'||c.relname||'.'||k.conname,
  'definition',pg_catalog.pg_get_constraintdef(k.oid,true), 'node_tree',k.conbin::text)
  ORDER BY n.nspname,c.relname,k.conname)::text
FROM pg_catalog.pg_constraint k
JOIN pg_catalog.pg_class c ON c.oid=k.conrelid
JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
WHERE n.nspname='public' AND k.contype='c'
"""
constraints = {service: json.loads(psql(service, constraint_sql)) for service in ("source", "target")}
location_re = re.compile(r":location -?\d+")
for rows in constraints.values():
    for row in rows:
        row["node_tree"] = location_re.sub(":location 0", row["node_tree"] or "")
source_constraints = {row["identity"]: row for row in constraints["source"]}
target_constraints = {row["identity"]: row for row in constraints["target"]}
if set(source_constraints) != set(target_constraints):
    raise SystemExit("source/destination check-constraint inventory differs")
allowed_deparser_differences = {
    "public.evento_auditoria_local.evento_auditoria_idempotencia_ck",
    "public.mensaje_reserva_ensayo.mensaje_reserva_ensayo_cuerpo_check",
    "public.perfil_usuario.perfil_usuario_nombre_ck",
}
for identity in sorted(source_constraints):
    source_row, target_row = source_constraints[identity], target_constraints[identity]
    if source_row["definition"] != target_row["definition"]:
        raise SystemExit(f"source/destination catalog check definition differs: {identity}")

def named_check_lines(path):
    found = {identity.split(".")[-1]: [] for identity in allowed_deparser_differences}
    for line in path.read_text(encoding="utf-8").splitlines():
        for name in found:
            if f"CONSTRAINT {name} CHECK" in line:
                found[name].append(line)
    if any(len(lines) != 1 for lines in found.values()):
        raise SystemExit("could not uniquely locate the reviewed CHECK constraints in schema dump")
    return {name: lines[0] for name, lines in found.items()}

source_check_lines = named_check_lines(artifacts / "source.schema.sql")
target_check_lines = named_check_lines(artifacts / "target.schema.sql")
constraint_deparser_differences = []
for identity in sorted(allowed_deparser_differences):
    name = identity.split(".")[-1]
    source_row, target_row = source_constraints[identity], target_constraints[identity]
    if source_check_lines[name] != target_check_lines[name]:
        if source_row["definition"] != target_row["definition"]:
            raise SystemExit(f"pg_dump check text differs and catalog definitions are not equal: {identity}")
        constraint_deparser_differences.append({
            "identity": identity,
            "source_dump_line_sha256": hashlib.sha256(source_check_lines[name].encode()).hexdigest(),
            "target_dump_line_sha256": hashlib.sha256(target_check_lines[name].encode()).hexdigest(),
            "catalog_definition_sha256_equal": hashlib.sha256(source_row["definition"].encode()).hexdigest(),
            "source_node_tree_sha256": hashlib.sha256(source_row["node_tree"].encode()).hexdigest(),
            "target_node_tree_sha256": hashlib.sha256(target_row["node_tree"].encode()).hexdigest(),
            "reason": "PostgreSQL 18 pg_dump emitted different redundant boolean-check parentheses; pg_get_constraintdef(..., true) is identical on source and destination",
        })

def structural_dump(path):
    lines = path.read_text(encoding="utf-8").splitlines()
    result, acl_section = [], False
    for line in lines:
        if re.match(r"^-- Name: .*; Type: (?:ACL|DEFAULT ACL);", line):
            acl_section = True
            continue
        if acl_section and (line.startswith("-- Name:") or line.startswith("-- PostgreSQL database dump complete")):
            acl_section = False
        if acl_section:
            continue
        if line.startswith(("GRANT ", "REVOKE ", "ALTER DEFAULT PRIVILEGES ", "SET SESSION AUTHORIZATION", "RESET SESSION AUTHORIZATION")):
            continue
        if line.startswith("\\restrict ") or line.startswith("\\unrestrict "):
            line = line.split(" ", 1)[0] + " [dump-auth-token]"
        for difference in constraint_deparser_differences:
            name = difference["identity"].split(".")[-1]
            if f"CONSTRAINT {name} CHECK" in line:
                line = f"    CONSTRAINT {name} CHECK [validated-equivalent-node-tree]"
        result.append(line)
    return "\n".join(result) + "\n"

source_structure = structural_dump(artifacts / "source.schema.sql")
target_structure = structural_dump(artifacts / "target.schema.sql")
if source_structure != target_structure:
    diff = "\n".join(__import__("difflib").unified_diff(source_structure.splitlines(), target_structure.splitlines(), fromfile="source", tofile="target")) + "\n"
    (artifacts / "schema-diff.txt").write_text(diff, encoding="utf-8")
    print(diff[:12000], file=sys.stderr)
    raise SystemExit("source/destination structural schema differs beyond validated ACL and constraint-node normalization")
(artifacts / "source.structure.sql").write_text(source_structure, encoding="utf-8")
(artifacts / "target.structure.sql").write_text(target_structure, encoding="utf-8")
schema_sha = hashlib.sha256(source_structure.encode()).hexdigest()
constraint_differences_sha = hashlib.sha256(json.dumps(constraint_deparser_differences, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
body = {
    "format": "espacigo-gcp04-w4-baseline-package-v2",
    "postgres_image": "postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677",
    "source": "canonical empty ephemeral database; V1-V64 applied by cmd/dbmigrate as local postgres; ownership/grants normalized locally after replay",
    "destination": "separate ephemeral PostgreSQL 18 cluster; global roles provisioned separately; pg_restore as espacigo_migrator_login SET ROLE espacigo_schema_owner",
    "gadm_bootstrap": {
        "helper": "db/bootstrap/gcp04-runtime/004_gadm_global_role.sql",
        "actor_contract": "LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT",
        "created_and_commented_by_controlled_actor": True,
        "exact_comment": gadm_role["comment"],
        "zero_memberships_and_settings_after_restore": True,
        "executor_absent_after_commit": True,
        "failure_injection_rollback_clean": True,
        "reserved_executor_collision_rejected": True,
        "invalid_actor_and_setting_rejected": True
    },
    "target_preflight": {"login": "espacigo_observer_login", "read_only": True,
        "executed_before_and_after_restore": True},
    "target_restore": {"login": "espacigo_migrator_login", "set_role": "espacigo_schema_owner"},
    "repo_head": subprocess.check_output(["git", "-C", root, "rev-parse", "HEAD"], text=True).strip(),
    "migrations": {"count": 64, "first": 1, "last": 64, "source_target_checksums_equal": True,
        "source_ledger": migration_history["source"],
        "canonical_files": [{"name": path.name, "sha256": hashlib.sha256(path.read_bytes()).hexdigest()}
                             for path in sorted((Path(root) / "db/migrations").glob("V*.sql"))]},
    "extension_config_tables": (artifacts / "extension-config-tables.txt").read_text(encoding="utf-8").splitlines(),
    "archive_sha256": hashlib.sha256((artifacts / "baseline.dump").read_bytes()).hexdigest(),
    "toc_sha256": hashlib.sha256((artifacts / "baseline.toc.restore").read_bytes()).hexdigest(),
    "toc_original_sha256": hashlib.sha256((artifacts / "baseline.toc.original").read_bytes()).hexdigest(),
    "toc_skipped_sha256": hashlib.sha256((artifacts / "baseline.toc.restore.skipped").read_bytes()).hexdigest(),
    "toc_bootstrap_acl_sha256": hashlib.sha256((artifacts / "baseline.toc.bootstrap-acl").read_bytes()).hexdigest(),
    "toc_acl_sha256": hashlib.sha256((artifacts / "baseline.toc.acl").read_bytes()).hexdigest(),
    "acl_raw_sql_sha256": hashlib.sha256((artifacts / "baseline.acl.raw.sql").read_bytes()).hexdigest(),
    "acl_restore_sql_sha256": hashlib.sha256((artifacts / "baseline.acl.restore.sql").read_bytes()).hexdigest(),
    "source_schema_dump_sha256": hashlib.sha256((artifacts / "source.schema.sql").read_bytes()).hexdigest(),
    "source_structure_sha256": hashlib.sha256((artifacts / "source.structure.sql").read_bytes()).hexdigest(),
    "source_catalog_sha256": hashlib.sha256((artifacts / "source.catalog.json").read_bytes()).hexdigest(),
    "target_schema_dump_sha256": hashlib.sha256((artifacts / "target.schema.sql").read_bytes()).hexdigest(),
    "normalized_schema_sha256_equal": schema_sha,
    "acl_inventory_sha256_equal": hashlib.sha256(json.dumps(normalized_catalogs["source"]["acl_entries"], sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "explicit_owner_acl_representation_sha256": hashlib.sha256(json.dumps(owner_acl_representation, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "explicit_owner_acl_representation": owner_acl_representation,
    "explicit_owner_acl_representation_differences_sha256": hashlib.sha256(json.dumps(owner_acl_differences, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "explicit_owner_acl_representation_differences": owner_acl_differences,
    "owner_acl_effective_sha256_equal": hashlib.sha256(json.dumps(owner_acl_effective_normalized["source"], sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "owner_acl_effective_full_privileges_checked": True,
    "owner_acl_effective_inventory": owner_acl_effective,
    "constraint_deparser_differences_sha256": constraint_differences_sha,
    "constraint_deparser_differences": constraint_deparser_differences,
    "catalog_manifest_sha256_equal": hashlib.sha256(json.dumps(normalized_catalogs["source"], sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "data_inventory_sha256_equal": hashlib.sha256(json.dumps(source_data, sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "migration_history_sha256_equal": hashlib.sha256(json.dumps(migration_history["source"], sort_keys=True, separators=(",", ":")).encode()).hexdigest(),
    "migration_history_count": len(migration_history["source"]),
    "tables": source_data,
    "catalog": normalized_catalogs["source"],
    "toc_entries_skipped": (artifacts / "baseline.toc.restore.skipped").read_text(encoding="utf-8").splitlines(),
    "role_definitions_and_memberships_in_dump": False,
    "toc_bootstrap_acl_entries": (artifacts / "baseline.toc.bootstrap-acl").read_text(encoding="utf-8").splitlines(),
    "expected_differences": constraint_deparser_differences,
}
canonical = json.dumps(body, sort_keys=True, separators=(",", ":")).encode()
manifest = {**body, "manifest_body_sha256": hashlib.sha256(canonical).hexdigest()}
(artifacts / "baseline.manifest.json").write_text(json.dumps(manifest, sort_keys=True, indent=2) + "\n", encoding="utf-8")
print("Baseline manifest: " + manifest["manifest_body_sha256"])
print("Archive SHA-256: " + manifest["archive_sha256"])
print("Original TOC SHA-256: " + manifest["toc_original_sha256"])
print("Filtered TOC SHA-256: " + manifest["toc_sha256"])
print("Source schema SHA-256: " + manifest["source_schema_dump_sha256"])
print("Target schema SHA-256: " + manifest["target_schema_dump_sha256"])
print("Normalized schema SHA-256: " + manifest["normalized_schema_sha256_equal"])
print("ACL inventory SHA-256: " + manifest["acl_inventory_sha256_equal"])
print("Per-table inventory SHA-256: " + manifest["data_inventory_sha256_equal"])
print("V1-V64 ledger SHA-256: " + manifest["migration_history_sha256_equal"])
print("Documented pg_dump check deparser differences: " + str(len(constraint_deparser_differences)))
print(f"Matched {len(source_data)} public tables and all source/destination row checksums.")
PY

umask 077
mkdir -p "$ROOT_DIR/.local/testing"
chmod 0700 "$ROOT_DIR/.local/testing"
MANIFEST_PATH="$ROOT_DIR/.local/testing/gcp04-w3-logical-baseline-${RUN_SLUG}.json"
install -m 0600 "$ARTIFACTS/baseline.manifest.json" "$MANIFEST_PATH"
echo "Private manifest: $MANIFEST_PATH"

if [[ -n "$RETAIN_DIR" ]]; then
  mkdir -m 0700 -- "$RETAIN_DIR"
  bundle_files=(
    baseline.dump baseline.manifest.json baseline.toc.original baseline.toc.restore
    baseline.toc.restore.skipped baseline.toc.bootstrap-acl baseline.toc.acl
    baseline.acl.raw.sql baseline.acl.restore.sql source.schema.sql source.structure.sql
    source.catalog.json
  )
  for file in "${bundle_files[@]}"; do
    [[ -f "$ARTIFACTS/$file" && ! -L "$ARTIFACTS/$file" ]] || { echo "Required bundle input missing: $file" >&2; exit 1; }
    install -m 0600 -- "$ARTIFACTS/$file" "$RETAIN_DIR/$file"
  done
  (cd -- "$RETAIN_DIR" && sha256sum "${bundle_files[@]}" | LC_ALL=C sort -k2) > "$RETAIN_DIR/SHA256SUMS"
  chmod 0600 "$RETAIN_DIR/SHA256SUMS"
  echo "Private baseline package retained: $RETAIN_DIR"
fi

echo 'V1-V64 canonical baseline dump/restore, restricted no-change verification, checksum-drift rejection, runtime role identities/denials, private preflight, and inventory checks passed locally.'
cleanup_verified
trap - EXIT
