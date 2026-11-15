#!/usr/bin/env bash
# Creates its own networkless disposable cluster; never accepts an external DSN.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
SCOPE="${1:-full}"
case "$SCOPE" in
  full) GO_TEST_PATTERN='^Test(Runner|PlanOnly|MigratorSerializes|CurrentSchemaMigration|RentalCombinedFixtureOptIn)' ;;
  v19) GO_TEST_PATTERN='^TestCurrentSchemaMigrationV19WeeklyHours$' ;;
  v76) GO_TEST_PATTERN='^TestGalleryCapacityTransitionV68ToV76$' ;;
  authority-window) GO_TEST_PATTERN='^TestNativeKYCAuthorityWindowPostgres$' ;;
  *) echo 'usage: test-migration-runner-postgres.sh [full|v19|v76|authority-window]' >&2; exit 2 ;;
esac
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
if [[ "$SCOPE" == authority-window ]]; then
  IMAGE='postgres@sha256:c7fedce47eff6a78cb35dad6b627f0d684107f75ab15c0870e40c093766ba452'
fi
NAME="espacigo-migration-ci-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:?set TMPDIR to your scratch directory}/espacigo-migration-ci.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
OWNED_CONTAINER_ID=''
OWNER_UID="$(id -u)"
OWNER_GID="$(id -g)"
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ -n "$OWNED_CONTAINER_ID" ]]; then
    if [[ "$(docker inspect --format '{{.State.Running}}' "$OWNED_CONTAINER_ID")" == true ]]; then
      if ! docker exec --user 0 "$OWNED_CONTAINER_ID" chown "$OWNER_UID:$OWNER_GID" /var/run/postgresql; then
        echo 'failed to restore ownership of own socket directory' >&2
        exit 1
      fi
    fi
    if ! docker rm -f "$OWNED_CONTAINER_ID" >/dev/null; then
      echo 'failed to remove own migration container; preserving socket directory' >&2
      exit 1
    fi
    if ! python3 - "$OWNED_CONTAINER_ID" <<'PY'
import subprocess,sys
r=subprocess.run(['docker','inspect',sys.argv[1]],capture_output=True,text=True)
assert r.returncode!=0 and r.stderr.strip().lower()==('error: no such object: '+sys.argv[1]), 'owned container absence not verified'
print('PASS: owned migration container removed; exact-ID absence verified')
PY
    then exit 1; fi
  fi
  python3 -c 'import pathlib,shutil,sys; p=pathlib.Path(sys.argv[1]); shutil.rmtree(p); assert not p.exists(); print("PASS: own migration scratch removed and absence verified")' "$TMP_DIR"
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
python3 - "$TMP_DIR/container-receipt.json" "$OWNED_CONTAINER_ID" "$NAME" <<'PY'
import json,os,sys
fd=os.open(sys.argv[1],os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
with os.fdopen(fd,'w') as f:json.dump({'id':sys.argv[2],'name':sys.argv[3]},f)
PY
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
ISOLATION_TOKEN="$(python3 -c 'import secrets; print(secrets.token_hex(32))')"
printf '%s\n' "$ISOLATION_TOKEN" > "$SOCKET_DIR/.espacigo-migration-ci"
chmod 0600 "$SOCKET_DIR/.espacigo-migration-ci"
# Assert the resolved resource boundaries before any test can create/drop DBs.
python3 - "$NAME" <<'PY'
import json,subprocess,sys
c=json.loads(subprocess.check_output(['docker','inspect',sys.argv[1]],text=True))[0]
assert c['HostConfig']['NetworkMode']=='none'
assert not c['HostConfig'].get('PortBindings')
assert not any(m['Type']=='volume' for m in c['Mounts'])
socket=next(m['Source'] for m in c['Mounts'] if m['Destination']=='/var/run/postgresql')
print('ISOLATED TARGET: container='+c['Name'].lstrip('/')+' image='+c['Config']['Image']+' network=none ports=none volumes=none PGDATA=tmpfs socket='+socket)
PY
env -u DATABASE_URL -u DATABASE_SET_ROLE -u PGPASSWORD -u PGPASSFILE -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE \
  TEST_MIGRATOR_CI_ISOLATED=1 \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR&sslmode=disable" \
  TEST_MIGRATION_AUTHORITY_REPORT_DIR="$TMP_DIR" \
  go test -race -json -count=1 -timeout=3m \
  -run "$GO_TEST_PATTERN" \
  ./internal/migrator > "$TMP_DIR/results.jsonl" 2>&1 || {
    cat "$TMP_DIR/results.jsonl" >&2
    exit 1
  }
if [[ "$SCOPE" == full ]]; then
  python3 scripts/gcp/test_identity_tail_plan_wire.py "$TMP_DIR"
fi
python3 scripts/gcp/check_migration_ci.py --scope "$SCOPE" "$TMP_DIR/results.jsonl"
if [[ "$SCOPE" == v19 ]]; then
  echo 'PASS: isolated disposable PostgreSQL V1-V19 weekly-hours migration and trigger tests.'
elif [[ "$SCOPE" == v76 ]]; then
  echo 'PASS: isolated disposable PostgreSQL V1-V68-to-V76 gallery capacity transition (10 to 30), concurrent 30th slot, and 31st-slot rejection.'
elif [[ "$SCOPE" == authority-window ]]; then
  echo 'PASS: isolated PostgreSQL 18.7 target-pinned V95/V96 preparation/finalization, wrong-target no-DDL, separate temporary CREATE closure, successful migration boundaries, retry and readback; no persistent database touched.'
else
  env -u DATABASE_URL -u DATABASE_SET_ROLE -u PGPASSWORD -u PGPASSFILE -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE \
  TEST_RENTALFLOW_POSTGRES_ISOLATED=1 \
  TEST_RENTALFLOW_POSTGRES_ISOLATION_TOKEN="$ISOLATION_TOKEN" \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR&sslmode=disable" \
  go test -race -count=1 -timeout=3m ./internal/adapters/postgres/rentalflow > "$TMP_DIR/rentalflow.log" 2>&1 || {
    cat "$TMP_DIR/rentalflow.log" >&2
    exit 1
  }
cat "$TMP_DIR/rentalflow.log"
echo 'PASS: disposable PostgreSQL runner (V1-V97, V75 upgrade, V96-to-V97 authority lifecycle, V84 legacy recovery/finalization, quotes/suppression, plan/no-op/failure/checksum/concurrency) and rentalflow availability integration on V1-V61; cleanup checked separately on exit.'
fi
