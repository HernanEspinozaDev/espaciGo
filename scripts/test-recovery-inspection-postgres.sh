#!/usr/bin/env bash
# Own networkless disposable cluster; never accepts an external DSN.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-recovery-ci-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:?private scratch TMPDIR required}/espacigo-recovery-ci.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
OWNED_CONTAINER_ID=''
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ -n "$OWNED_CONTAINER_ID" ]]; then
    # The image initializes/chowns its socket mount. Restore ownership of
    # this test's exact bind mount before removal; never touch other mounts.
    if ! docker exec "$OWNED_CONTAINER_ID" chown -R "$(id -u):$(id -g)" /var/run/postgresql; then
      printf '%s\n' 'FAIL: restore own socket mount ownership' >&2
      status=1
    fi
    if ! docker rm -f "$OWNED_CONTAINER_ID" >/dev/null; then
      printf '%s\n' 'FAIL: remove own recovery container' >&2
      status=1
    elif docker inspect "$OWNED_CONTAINER_ID" >/dev/null 2>&1; then
      printf '%s\n' 'FAIL: recovery container still exists' >&2
      status=1
    else
      printf '%s\n' 'PASS: own recovery container absence verified'
    fi
  fi
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1])' "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if docker inspect "$NAME" >/dev/null 2>&1; then
  printf '%s\n' 'refusing existing container' >&2
  exit 1
fi
docker image inspect "$IMAGE" >/dev/null
OWNED_CONTAINER_ID="$(docker create --name "$NAME" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --tmpfs /docker-entrypoint-initdb.d:rw,noexec,nosuid,size=1m \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE")"
docker start "$OWNED_CONTAINER_ID" >/dev/null
ready=false
for _ in $(seq 1 160); do
  count="$(docker logs "$NAME" 2>&1 | grep -cF 'database system is ready to accept connections' || true)"
  if [[ "${count:-0}" -ge 2 ]] && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]] && docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.25
done
[[ "$ready" == true ]] || { printf '%s\n' 'disposable PostgreSQL not ready' >&2; exit 1; }
: > "$SOCKET_DIR/.espacigo-recovery-ci"
chmod 0600 "$SOCKET_DIR/.espacigo-recovery-ci"
python3 - "$NAME" <<'PY'
import json,subprocess,sys
c=json.loads(subprocess.check_output(['docker','inspect',sys.argv[1]],text=True))[0]
assert c['HostConfig']['NetworkMode']=='none'
assert not c['HostConfig'].get('PortBindings')
assert not any(m['Type']=='volume' for m in c['Mounts'])
print('ISOLATED RECOVERY TARGET:',c['Name'],c['Config']['Image'],'network=none ports=none volumes=none PGDATA=tmpfs',flush=True)
PY
env -u DATABASE_URL -u DATABASE_SET_ROLE -u PGPASSWORD -u PGPASSFILE -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE \
  TEST_RECOVERY_INSPECTION_ISOLATED=1 \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR&sslmode=disable" \
  go test -v -count=1 -timeout=90s -run '^TestRecoveryInspectionPostgresReadonlySnapshot$' ./internal/migrator > "$TMP_DIR/result.txt" 2>&1 || {
    python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).read_text())' "$TMP_DIR/result.txt"
    exit 1
  }
PYTHONDONTWRITEBYTECODE=1 PYTHONPATH="$ROOT_DIR/scripts/gcp" /usr/bin/python3 - "$TMP_DIR/result.txt" <<'PY'
import json,pathlib,sys
import recovery_inspection as recovery
s=pathlib.Path(sys.argv[1]).read_text(); print(s)
assert '--- PASS: TestRecoveryInspectionPostgresReadonlySnapshot' in s
assert 'SKIP' not in s and 'FAIL' not in s
reports=[json.loads(line.split('RECOVERY_REPORT_JSON=',1)[1]) for line in s.splitlines() if 'RECOVERY_REPORT_JSON=' in line]
assert len(reports)==1, 'actual PostgreSQL report must reach the production Python validator'
recovery.validate_report(reports[0], recovery.local_migrations(pathlib.Path('.'))[:2])
print('PASS: actual PostgreSQL -> Go JSON -> production Python report validation')
PY
