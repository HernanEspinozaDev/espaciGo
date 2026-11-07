#!/usr/bin/env bash
# Build locally; run only the inventory path, without network, SQL or secrets.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-migrator-image.XXXXXX")"
IMAGE="espacigo-runtime-migrator:local-check-$$"
owned_image=false
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ "$owned_image" == true ]]; then docker image rm "$IMAGE" >/dev/null 2>&1 || true; fi
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if docker image inspect "$IMAGE" >/dev/null 2>&1; then
  echo 'refusing to reuse an existing image tag' >&2
  exit 1
fi
# Claim only our unique tag; failed builds must not remove shared images/cache.
owned_image=true
docker build --platform=linux/amd64 --file=Dockerfile.runtime-migrator --tag="$IMAGE" .
go run ./cmd/dbmigrate -inventory-only -dir db/migrations > "$TMP_DIR/source.json"
docker run --rm --network none --read-only --cap-drop ALL \
  --security-opt=no-new-privileges "$IMAGE" -inventory-only > "$TMP_DIR/image.json"
python3 scripts/gcp/verify_migration_inventory.py "$TMP_DIR/source.json" "$TMP_DIR/image.json"
# Invalid execution-mode combinations must be refused before a connection.
if docker run --rm --network none --read-only --cap-drop ALL \
  --security-opt=no-new-privileges "$IMAGE" -inventory-only -plan-only > "$TMP_DIR/negative.log" 2>&1; then
  echo 'offline inventory accepted a conflicting SQL mode' >&2
  exit 1
fi
grep -Fq -- '-inventory-only cannot combine' "$TMP_DIR/negative.log"
echo 'PASS: runtime image inventory and conflicting-mode rejection tested without database access.'
