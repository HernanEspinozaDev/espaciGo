#!/usr/bin/env bash
# Creates its own networkless disposable cluster; never accepts an external DSN.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-migration-ci-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-migration-ci.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
OWNED_CONTAINER_ID=''
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ -n "$OWNED_CONTAINER_ID" ]]; then docker rm -f "$OWNED_CONTAINER_ID" >/dev/null 2>&1 || true; fi
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if docker inspect "$NAME" >/dev/null 2>&1; then
  echo 'refusing to reuse an existing migration test container' >&2
  exit 1
fi
OWNED_CONTAINER_ID="$(docker create --name "$NAME" --network none \
  --tmpfs /var/lib/postgresql:rw,size=2g \
  --tmpfs /docker-entrypoint-initdb.d:rw,noexec,nosuid,size=1m \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE")"
docker start "$OWNED_CONTAINER_ID" >/dev/null
ready=false
for _ in $(seq 1 160); do
  ready_count="$(docker logs "$NAME" 2>&1 | grep -cF 'database system is ready to accept connections' || true)"
  if [[ "${ready_count:-0}" -ge 2 ]] && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]] && docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.25
done
[[ "$ready" == true ]] || { docker logs --tail=30 "$NAME" >&2; echo 'disposable PostgreSQL did not become ready' >&2; exit 1; }
: > "$SOCKET_DIR/.espacigo-migration-ci"
chmod 0600 "$SOCKET_DIR/.espacigo-migration-ci"
# Assert the resolved resource boundaries before any test can create/drop DBs.
python3 - "$NAME" <<'PY'
import json,subprocess,sys
c=json.loads(subprocess.check_output(['docker','inspect',sys.argv[1]],text=True))[0]
assert c['HostConfig']['NetworkMode']=='none'
assert not c['HostConfig'].get('PortBindings')
assert not any(m['Type']=='volume' for m in c['Mounts'])
PY
env -u DATABASE_URL -u DATABASE_SET_ROLE -u PGPASSWORD -u PGPASSFILE -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE \
  TEST_MIGRATOR_CI_ISOLATED=1 \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR&sslmode=disable" \
  go test -json -count=1 -timeout=3m \
  -run '^Test(Runner|PlanOnly|MigratorSerializes|CurrentSchemaMigration)' \
  ./internal/migrator > "$TMP_DIR/results.jsonl" 2>&1 || {
    cat "$TMP_DIR/results.jsonl" >&2
    exit 1
  }
python3 scripts/gcp/check_migration_ci.py "$TMP_DIR/results.jsonl"
echo 'PASS: disposable PostgreSQL runner, fresh V1-V84, V75 upgrade, V84 authority recovery/finalization, read-only plan, no-op, failure recovery, checksum and concurrency tests; own resources removed on exit.'
