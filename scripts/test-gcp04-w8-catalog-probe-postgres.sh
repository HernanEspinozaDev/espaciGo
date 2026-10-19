#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
CONTAINER="espacigo-gcp04-w8-pg18-${RUN_ID}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w8.XXXXXX")"
chmod 0700 "$TMP_DIR"
TEST_BINARY="$TMP_DIR/gcp04w8.test"

cleanup() {
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}' 2>/dev/null || true)"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || {
  echo 'Pinned PostgreSQL 18/PostGIS image is not available locally; refusing to pull or substitute.' >&2
  exit 1
}

CGO_ENABLED=0 go test -c -trimpath -o "$TEST_BINARY" ./cmd/gcp04w8catalogprobe
docker run --rm -d --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=1g \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  "$IMAGE" postgres -c "listen_addresses=" >/dev/null

ready=0
for _ in $(seq 1 40); do
  if docker exec "$CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
[[ "$ready" == 1 ]] || { echo 'Isolated PostgreSQL 18 fixture did not become ready.' >&2; exit 1; }

docker exec -i "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres <<'SQL'
CREATE DATABASE espacigo;
CREATE ROLE espacigo_gcp04_w8_catalog_probe LOGIN INHERIT;
GRANT pg_read_all_settings TO espacigo_gcp04_w8_catalog_probe;
SQL
docker cp "$TEST_BINARY" "$CONTAINER:/tmp/gcp04w8.test" >/dev/null
docker cp "$ROOT_DIR/db/query/gcp04_w8_catalog_probe.sql" "$CONTAINER:/tmp/gcp04_w8_catalog_probe.sql" >/dev/null

docker exec "$CONTAINER" env \
  GCP04_W8_TEST_DSN='host=/var/run/postgresql port=5432 user=espacigo_gcp04_w8_catalog_probe dbname=espacigo sslmode=disable' \
  GCP04_W8_TEST_QUERY_FILE=/tmp/gcp04_w8_catalog_probe.sql \
  /tmp/gcp04w8.test -test.v -test.run '^TestLocalPostgres18GoRunnerPath$'

docker rm -f "$CONTAINER" >/dev/null
trap - EXIT
rm -rf -- "$TMP_DIR"
echo 'W8 Go runner-path integration passed on pinned PostgreSQL 18; network none, no published ports, database on tmpfs.'
