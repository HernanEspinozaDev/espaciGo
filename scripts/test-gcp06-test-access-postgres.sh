#!/usr/bin/env bash
set -euo pipefail

image='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
name="gcp06-test-access-$$-$RANDOM"
tmpdir="$(mktemp -d)"
chmod 700 "$tmpdir"
cleanup() {
  docker rm -f "$name" >/dev/null 2>&1 || true
  chmod -R u+w "$tmpdir" >/dev/null 2>&1 || true
  rm -rf "$tmpdir" >/dev/null 2>&1 || true
}
trap cleanup EXIT

socket_dir="$tmpdir/socket"
mkdir -m 777 "$socket_dir"
docker run -d --name "$name" --network none \
  --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=1g \
  -v "$socket_dir:/var/run/postgresql" \
  -e POSTGRES_PASSWORD=throwaway-local-only \
  "$image" -c listen_addresses='' -c unix_socket_directories=/var/run/postgresql >/dev/null

for attempt in $(seq 1 60); do
  if docker exec "$name" pg_isready -U postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$name" pg_isready -U postgres >/dev/null
docker exec "$name" psql -U postgres -v ON_ERROR_STOP=1 -c 'CREATE DATABASE "espacigo_gcp06_candidate_20261020-gcp06a"' >/dev/null

go_env="postgres://postgres@/espacigo_gcp06_candidate_20261020-gcp06a?host=$socket_dir&sslmode=disable"
TEST_DATABASE_URL="$go_env" go test -p 1 -count=1 -run '^TestGCP06ApplyHashesAndRevokesExactCandidateCohort$' ./internal/gcp06testaccess
