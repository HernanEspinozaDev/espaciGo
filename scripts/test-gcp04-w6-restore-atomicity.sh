#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BUNDLE_INPUT="${1:-}"
if [[ -z "$BUNDLE_INPUT" ]]; then
  echo 'Usage: scripts/test-gcp04-w6-restore-atomicity.sh <retained-private-W6-bundle>' >&2
  exit 2
fi

BUNDLE_DIR="$(realpath -e -- "$BUNDLE_INPUT")"
if [[ ! -d "$BUNDLE_DIR" || "$BUNDLE_DIR" != */.local/testing/gcp04-w4-baseline-* || -L "$BUNDLE_DIR" ]]; then
  echo 'Expected the retained private W6 bundle under .local/testing; the test will not copy or regenerate it.' >&2
  exit 2
fi
if [[ "$(stat -c '%a' -- "$BUNDLE_DIR")" != 700 ]]; then
  echo 'The retained bundle directory must have mode 0700.' >&2
  exit 2
fi
for file in baseline.dump baseline.manifest.json baseline.toc.original baseline.toc.restore baseline.acl.restore.sql; do
  path="$BUNDLE_DIR/$file"
  if [[ ! -f "$path" || -L "$path" || "$(stat -c '%a' -- "$path")" != 600 ]]; then
    echo "Required retained artifact is missing or has unsafe permissions: $file" >&2
    exit 2
  fi
done
if [[ "$(sha256sum -- "$BUNDLE_DIR/baseline.dump" | cut -d ' ' -f 1)" != "1d31721c1a3fee15903f3a5cfbb348e5803b567727187c0a8871313b94f7faee" ]]; then
  echo 'The retained W6 archive SHA-256 does not match the reviewed pin.' >&2
  exit 2
fi

IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NETWORK="espacigo-gcp04-w6-atomicity-$$-$RANDOM"
CONTAINER="espacigo-gcp04-w6-atomicity-$$-$RANDOM"
PORT="$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')"
cleanup() {
  docker stop "$CONTAINER" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM

docker network create "$NETWORK" >/dev/null
docker run -d --rm \
  --name "$CONTAINER" \
  --network "$NETWORK" \
  --publish "127.0.0.1:${PORT}:5432" \
  --tmpfs /var/lib/postgresql:rw,nosuid,nodev,size=2g \
  --env POSTGRES_HOST_AUTH_METHOD=trust \
  "$IMAGE" >/dev/null

STORAGE="$(docker inspect --format '{{index .HostConfig.Tmpfs "/var/lib/postgresql"}}|{{len .Mounts}}' "$CONTAINER")"
TMPFS_SETTING="${STORAGE%%|*}"
MOUNT_COUNT="${STORAGE##*|}"
if [[ -z "$TMPFS_SETTING" || "$TMPFS_SETTING" == '<no value>' || "$MOUNT_COUNT" != 0 ]]; then
  echo 'Disposable PostgreSQL fixture must use tmpfs only and have no Docker volumes.' >&2
  exit 1
fi

for _ in $(seq 1 60); do
  if pg_isready -h 127.0.0.1 -p "$PORT" -U postgres >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
if ! pg_isready -h 127.0.0.1 -p "$PORT" -U postgres >/dev/null 2>&1; then
  echo 'Disposable PostgreSQL 18 fixture did not become ready.' >&2
  exit 1
fi

GCP04_W6_TEST_EPHEMERAL=1 \
GCP04_W6_TEST_BUNDLE="$BUNDLE_DIR" \
GCP04_W6_TEST_ADMIN_DSN="postgres://postgres@127.0.0.1:${PORT}/postgres?sslmode=disable" \
  go -C "$ROOT_DIR" test ./cmd/gcp04baseline-restore -run '^TestTransactionalRestoreAtomicityFromPrivateW6Bundle$' -count=1 -v
