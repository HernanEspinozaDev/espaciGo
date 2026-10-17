#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d /tmp/espacigo-gcp03-origin.XXXXXX)"
SOCKET_DIR="$TMP_DIR/socket"
CONTAINER="espacigo-gcp03-origin-$$"
MEMBERSHIP_CONTAINER="espacigo-gcp03-origin-membership-$$"
MEMBERSHIP_SOCKET_DIR="$TMP_DIR/membership-socket"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'

mkdir -m 0777 "$SOCKET_DIR" "$MEMBERSHIP_SOCKET_DIR"
if docker inspect "$CONTAINER" >/dev/null 2>&1; then
  echo "refusing to reuse existing container $CONTAINER" >&2
  exit 2
fi
cleanup() {
  status=$?
  trap - EXIT INT TERM
  docker stop "$CONTAINER" >/dev/null 2>&1 || true
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  docker stop "$MEMBERSHIP_CONTAINER" >/dev/null 2>&1 || true
  docker rm -f "$MEMBERSHIP_CONTAINER" >/dev/null 2>&1 || true
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

cd "$ROOT_DIR"
PYTHONPATH="$ROOT_DIR/scripts${PYTHONPATH:+:$PYTHONPATH}" \
  GCP03_ORIGIN_EXECUTOR_TEST_SOCKET="$SOCKET_DIR" \
  python3 -m unittest scripts.test_gcp03_origin_cloud_import_postgres -v

# A second disposable database gives `postgres` Cloud SQL's documented role
# attributes from initial creation. PostgreSQL protects the bootstrap
# superuser from later demotion, so this fixture starts with a different
# superuser and never runs bootstrap or import.
if docker inspect "$MEMBERSHIP_CONTAINER" >/dev/null 2>&1; then
  echo "refusing to reuse existing container $MEMBERSHIP_CONTAINER" >&2
  exit 2
fi
docker run -d --name "$MEMBERSHIP_CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --tmpfs /docker-entrypoint-initdb.d:rw,noexec,nosuid,size=1m \
  --mount "type=bind,src=$MEMBERSHIP_SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_USER=fixture_control \
  -e POSTGRES_DB=espacigo_scratch \
  -e POSTGRES_HOST_AUTH_METHOD=trust \
  "$IMAGE" >/dev/null

membership_ready=false
for _ in $(seq 1 160); do
  if docker exec "$MEMBERSHIP_CONTAINER" pg_isready -U fixture_control -d espacigo_scratch >/dev/null 2>&1 && [[ -S "$MEMBERSHIP_SOCKET_DIR/.s.PGSQL.5432" ]]; then
    sleep 2
    if docker exec "$MEMBERSHIP_CONTAINER" pg_isready -U fixture_control -d espacigo_scratch >/dev/null 2>&1 && [[ -S "$MEMBERSHIP_SOCKET_DIR/.s.PGSQL.5432" ]]; then
      membership_ready=true
      break
    fi
  fi
  sleep 0.25
done
[[ "$membership_ready" == true ]] || { echo "disposable PostgreSQL 18 membership fixture did not become ready" >&2; exit 2; }
docker exec "$MEMBERSHIP_CONTAINER" psql -U fixture_control -d espacigo_scratch -v ON_ERROR_STOP=1 -c \
  "CREATE ROLE postgres LOGIN NOSUPERUSER CREATEROLE CREATEDB INHERIT; CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT; CREATE ROLE membership_admin NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT; CREATE ROLE membership_observer LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE; ALTER DATABASE espacigo_scratch OWNER TO postgres; GRANT espacigo_runtime TO membership_admin WITH ADMIN TRUE, INHERIT TRUE, SET TRUE; GRANT membership_admin TO postgres WITH ADMIN FALSE, INHERIT TRUE, SET TRUE;"
: > "$MEMBERSHIP_SOCKET_DIR/.gcp03-origin-membership-isolated-test"
chmod 0600 "$MEMBERSHIP_SOCKET_DIR/.gcp03-origin-membership-isolated-test"
env -u DATABASE_URL \
  GCP03_ORIGIN_MEMBERSHIP_TEST_DATABASE_URL="postgres://postgres@localhost/espacigo_scratch?host=$MEMBERSHIP_SOCKET_DIR&sslmode=disable" \
  go test -count=1 -v -run '^TestOriginMembershipPostgres18$' ./internal/gcp03origin
