#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="$(date -u +%Y%m%d%H%M%S)-$$"
OUTPUT_REL=".local/testing/gcp03-restore-executor/$RUN_ID"
OUTPUT_DIR="$ROOT_DIR/$OUTPUT_REL"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp03-restore-executor.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
CONTAINER="espacigo-gcp03-restore-executor-$$"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
mkdir -m 0777 "$SOCKET_DIR"
mkdir -p "$(dirname -- "$OUTPUT_DIR")"
chmod 700 "$(dirname -- "$OUTPUT_DIR")"
if [[ -e "$OUTPUT_DIR" ]]; then
  echo "refusing to overwrite an existing GCP-03 integration run" >&2
  exit 2
fi
mkdir -m 700 "$OUTPUT_DIR"
cleanup() {
  status=$?
  if [[ "$status" -ne 0 ]]; then
    docker logs "$CONTAINER" >&2 2>/dev/null || true
  fi
  docker stop "$CONTAINER" >/dev/null 2>&1 || true
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
}
trap cleanup EXIT INT TERM

python3 "$ROOT_DIR/scripts/gcp03-restore-fixture.py" --output-dir "$OUTPUT_REL"
docker run -d --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_USER=localcontrol -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 120); do
  if docker exec "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U localcontrol -d postgres -Atqc 'SELECT 1' >/dev/null 2>&1 && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]]; then
    # The official image starts a temporary server for init scripts, then
    # shuts it down and starts the long-running server. Wait through that
    # transition before letting the host integration client connect.
    sleep 2
    if docker exec "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U localcontrol -d postgres -Atqc 'SELECT 1' >/dev/null 2>&1 && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]]; then break; fi
  fi
  sleep 0.25
done
docker exec "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U localcontrol -d postgres -Atqc 'SELECT 1' >/dev/null
[[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]] || { echo "PostgreSQL socket did not appear in the host-mounted directory" >&2; exit 2; }
docker exec "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U localcontrol -d postgres -c \
  'CREATE ROLE postgres LOGIN NOSUPERUSER CREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS' >/dev/null
cd "$ROOT_DIR"
env -u DATABASE_URL \
  GCP03_EXECUTOR_TEST_DATABASE_URL="postgres://localcontrol@localhost/postgres?host=$SOCKET_DIR" \
  GCP03_EXECUTOR_FIXTURE_PATH="$OUTPUT_DIR/fixture.sql" \
  go test -count=1 -v ./cmd/gcp03restore
