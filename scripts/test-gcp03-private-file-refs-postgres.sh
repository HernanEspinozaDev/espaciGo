#!/usr/bin/env bash
set -euo pipefail

if [[ "${RUN_G03O_PRIVATE_FILE_REFS_POSTGRES_TEST:-}" != "YES" ]]; then
  echo 'Set RUN_G03O_PRIVATE_FILE_REFS_POSTGRES_TEST=YES to run the disposable PostgreSQL 18 integration.' >&2
  exit 2
fi

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
IMAGE_ID="$(docker image inspect "$IMAGE" --format '{{.Id}}')"
[[ "$IMAGE_ID" == 'sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677' ]] || {
  echo 'Pinned disposable PostgreSQL image is unavailable or has a different image ID.' >&2
  exit 2
}

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
CONTAINER="espacigo-g03o-file-refs-${RUN_ID}"
TMP_DIR="$(mktemp -d /tmp/espacigo-g03o-file-refs.XXXXXX)"
SOCKET_DIR="$TMP_DIR/socket"
mkdir -m 0777 "$SOCKET_DIR"

cleanup() {
  local status=$?
  trap - EXIT INT TERM
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  chmod -t "$SOCKET_DIR" 2>/dev/null || true
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
  if docker container inspect "$CONTAINER" >/dev/null 2>&1; then
    echo 'Disposable PostgreSQL container remains after cleanup.' >&2
    exit 1
  fi
  exit "$status"
}
trap cleanup EXIT INT TERM

if docker inspect "$CONTAINER" >/dev/null 2>&1; then
  echo 'Refusing to reuse an existing container name.' >&2
  exit 2
fi
docker run -d --name "$CONTAINER" --network none \
  --user "$(id -u):$(id -g)" \
  --tmpfs "/var/lib/postgresql:rw,noexec,nosuid,size=1g,uid=$(id -u),gid=$(id -g),mode=0700" \
  --tmpfs /docker-entrypoint-initdb.d:rw,noexec,nosuid,size=1m \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_USER=postgres -e POSTGRES_DB=espacigo_scratch \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null

ready=false
for _ in $(seq 1 160); do
  if docker exec "$CONTAINER" pg_isready -U postgres -d espacigo_scratch >/dev/null 2>&1 && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]]; then
    ready=true
    break
  fi
  sleep 0.25
done
[[ "$ready" == true ]] || { echo 'Disposable PostgreSQL 18 did not become ready.' >&2; exit 2; }
: > "$SOCKET_DIR/.gcp03-private-file-refs-isolated-test"
chmod 0600 "$SOCKET_DIR/.gcp03-private-file-refs-isolated-test"

DATABASE_URL="postgres://postgres@localhost/espacigo_scratch?host=$SOCKET_DIR&sslmode=disable" \
  go run ./cmd/dbmigrate -dir "$ROOT_DIR/db/migrations"
cd "$ROOT_DIR"
PYTHONPATH="$ROOT_DIR/scripts${PYTHONPATH:+:$PYTHONPATH}" \
  G03O_PRIVATE_FILE_REFS_TEST_SOCKET="$SOCKET_DIR" \
  python3 -m unittest scripts.test_gcp03_origin_private_file_refs scripts.test_gcp03_origin_private_file_refs_postgres -v

docker rm -f "$CONTAINER" >/dev/null
if docker container inspect "$CONTAINER" >/dev/null 2>&1; then
  echo 'Disposable PostgreSQL container was not removed.' >&2
  exit 1
fi
trap - EXIT INT TERM
chmod -t "$SOCKET_DIR" 2>/dev/null || true
python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1])' "$TMP_DIR"
printf 'PASS: V1–V61 private-file-reference query integration; Cloud access none.\n'
