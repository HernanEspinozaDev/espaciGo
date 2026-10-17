#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d /tmp/espacigo-gcp03-origin.XXXXXX)"
SOCKET_DIR="$TMP_DIR/socket"
CONTAINER="espacigo-gcp03-origin-$$"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'

mkdir -m 0777 "$SOCKET_DIR"
if docker inspect "$CONTAINER" >/dev/null 2>&1; then
  echo "refusing to reuse existing container $CONTAINER" >&2
  exit 2
fi
cleanup() {
  status=$?
  trap - EXIT INT TERM
  docker stop "$CONTAINER" >/dev/null 2>&1 || true
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT INT TERM

# The image includes a postgis init script that preinstalls extensions. Hide
# it and use initdb's built-in plpgsql only, matching the Cloud SQL baseline.
docker run -d --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --tmpfs /docker-entrypoint-initdb.d:rw,noexec,nosuid,size=1m \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=espacigo_scratch \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  "$IMAGE" >/dev/null

ready=false
for _ in $(seq 1 160); do
  if docker exec "$CONTAINER" pg_isready -U postgres -d espacigo_scratch >/dev/null 2>&1 && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]]; then
    sleep 2
    if docker exec "$CONTAINER" pg_isready -U postgres -d espacigo_scratch >/dev/null 2>&1 && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]]; then
      ready=true
      break
    fi
  fi
  sleep 0.25
done
[[ "$ready" == true ]] || { echo "disposable PostgreSQL 18 did not become ready" >&2; exit 2; }
: > "$SOCKET_DIR/.gcp03-origin-isolated-test"
chmod 0600 "$SOCKET_DIR/.gcp03-origin-isolated-test"

cd "$ROOT_DIR"
env -u DATABASE_URL \
  GCP03_ORIGIN_TEST_DATABASE_URL="postgres://postgres@localhost/espacigo_scratch?host=$SOCKET_DIR&sslmode=disable" \
  go test -count=1 -v ./internal/gcp03origin
