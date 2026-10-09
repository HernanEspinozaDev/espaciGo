#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
PG_IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
MAILPIT_IMAGE='axllent/mailpit:v1.31.4@sha256:b68349e3a014b90c5610bfb26b2ae36f3892d7b8cf25ee140c6c71c98d2fcf48'
SUFFIX="$$-$RANDOM"
NETWORK="espacigo-comm-test-$SUFFIX"
DB="espacigo-comm-db-$SUFFIX"
MAILPIT="espacigo-comm-mailpit-$SUFFIX"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-comm-test.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  docker rm -f "$DB" "$MAILPIT" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
}
trap cleanup EXIT INT TERM

docker network create "$NETWORK" >/dev/null
docker run -d --name "$DB" --network none --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  --tmpfs /var/lib/postgresql:rw,size=1g -e POSTGRES_HOST_AUTH_METHOD=trust \
  "$PG_IMAGE" >/dev/null
docker run -d --name "$MAILPIT" --network "$NETWORK" --publish 127.0.0.1::1025 \
  --publish 127.0.0.1::8025 --tmpfs /data:rw,size=64m \
  -e MP_DATABASE=/data/mailpit.db -e MP_MAX_MESSAGES=50 "$MAILPIT_IMAGE" >/dev/null

for _ in $(seq 1 120); do
  ready_count="$(docker logs "$DB" 2>&1 | grep -c 'database system is ready to accept connections' || true)"
  if [[ "$ready_count" -ge 2 ]] && docker exec "$DB" pg_isready -U postgres -d postgres >/dev/null 2>&1; then break; fi
  sleep 0.25
done
docker exec "$DB" pg_isready -U postgres -d postgres >/dev/null
SMTP_PORT="$(docker port "$MAILPIT" 1025/tcp | sed -E 's/.*:([0-9]+)$/\1/')"
MAILPIT_PORT="$(docker port "$MAILPIT" 8025/tcp | sed -E 's/.*:([0-9]+)$/\1/')"
for _ in $(seq 1 120); do
  if python3 -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:$MAILPIT_PORT/api/v1/info',timeout=.5).read()" >/dev/null 2>&1; then break; fi
  sleep 0.25
done
python3 -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:$MAILPIT_PORT/api/v1/info',timeout=2).read()" >/dev/null

TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR" \
LOCAL_SMTP_ADDR="127.0.0.1:$SMTP_PORT" \
LOCAL_MAILPIT_API="http://127.0.0.1:$MAILPIT_PORT" \
GO_TEST_RUN='TestSyntheticReviewReciprocityModerationRetentionAndRuntimePermissions|TestV37BackfillsV36ReviewAuthorshipMarker' \
  go test -p 1 -count=1 -v ./internal/adapters/postgres/reputation
