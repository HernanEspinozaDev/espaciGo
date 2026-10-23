#!/usr/bin/env bash
# Real Chromium→Go→PostgreSQL auth test. The caller must explicitly authorize
# this fixture and provide an already-running local Angular frontend.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
if [[ "${AUTH_BROWSER_FIXTURE_AUTHORIZED:-}" != "1" ]]; then
  echo 'Set AUTH_BROWSER_FIXTURE_AUTHORIZED=1 only after confirming local fixture scope.' >&2
  exit 2
fi
FRONTEND_URL="${AUTH_BROWSER_FRONTEND_URL:-}"
if [[ "$FRONTEND_URL" != http://127.0.0.1:* ]]; then
  echo 'AUTH_BROWSER_FRONTEND_URL must use the local 127.0.0.1 address.' >&2
  exit 2
fi
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-auth-browser-$$"
TMP_DIR="$(mktemp -d /tmp/espacigo-auth-browser.XXXXXX)"
SOCKET_DIR="$TMP_DIR/socket"
chmod 0700 "$TMP_DIR"
mkdir -m 0777 "$SOCKET_DIR"
install -m 0600 /dev/null "$TMP_DIR/.disposable-postgres"
cleanup() {
  docker exec --user root "$NAME" chmod 0777 /var/run/postgresql >/dev/null 2>&1 || true
  docker rm -fv "$NAME" >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT INT TERM
unset PGPASSWORD PGUSER PGHOST PGPORT PGDATABASE DATABASE_URL GCP04_REPLAY_CLOUD GCP04_REPLAY_CLOUD_SQL_PASSWORD_FILE
install -m 0600 /dev/null "$TMP_DIR/no-credentials"
export PGPASSFILE="$TMP_DIR/no-credentials"
docker run -d --name "$NAME" --network none --tmpfs /var/lib/postgresql:rw,size=1g \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_USER=fixture_admin -e POSTGRES_HOST_AUTH_METHOD=trust -e PGDATA=/var/lib/postgresql/auth-browser-data "$IMAGE" >/dev/null
for _ in $(seq 1 120); do
  if docker exec "$NAME" pg_isready -U fixture_admin >/dev/null 2>&1; then break; fi
  sleep 0.25
done
docker exec "$NAME" pg_isready -U fixture_admin >/dev/null
DATA_DIR="$(docker exec "$NAME" psql -X -U fixture_admin -d postgres -qAt -v ON_ERROR_STOP=1 -c 'SHOW data_directory')"
if [[ "$DATA_DIR" != /var/lib/postgresql/auth-browser-data ]]; then
  echo 'PostgreSQL data directory is outside the test tmpfs.' >&2
  exit 1
fi
printf 'espacigo-auth-browser-disposable-v1\n%s\n' "$NAME" > "$TMP_DIR/.disposable-postgres"
docker exec "$NAME" psql -X -U fixture_admin -d postgres -v ON_ERROR_STOP=1 -q -c 'CREATE ROLE postgres LOGIN SUPERUSER'
export TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR"
export AUTH_BROWSER_DISPOSABLE_ROOT="$TMP_DIR"
export AUTH_BROWSER_FRONTEND_URL="$FRONTEND_URL"
export PLAYWRIGHT_EXECUTABLE_PATH="${PLAYWRIGHT_EXECUTABLE_PATH:-/home/nandev/.cache/ms-playwright/chromium-1248/chrome-linux64/chrome}"
go test ./internal/adapters/postgres/identity -run '^TestAuth(BrowserDatabaseTargetValidationFailsBeforeConnect|RealBrowserLogin)$' -count=1 -v
