#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-m04-test-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-m04-test.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
	docker stop "$NAME" >/dev/null 2>&1 || true
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  docker run --rm --network none --user 0:0 \
    --mount "type=bind,src=$SOCKET_DIR,dst=/socket" \
    --entrypoint /bin/sh "$IMAGE" -c 'find /socket -mindepth 1 -maxdepth 1 -delete' >/dev/null 2>&1 || true
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
}
trap cleanup EXIT INT TERM

docker run -d --name "$NAME" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 60); do
  if docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null
if (($#)); then TEST_PACKAGES=("$@"); else TEST_PACKAGES=(./...); fi
env -u DATABASE_URL TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR" \
  go test -p 1 -count=1 -v "${TEST_PACKAGES[@]}"
