#!/usr/bin/env bash
# Build locally; inspect inventory and rejection paths without network or SQL.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-migrator-image.XXXXXX")"
IMAGE="espacigo-runtime-migrator:local-check-$$"
owned_image=false
OWNED_CONTAINER_ID=''
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ -n "$OWNED_CONTAINER_ID" ]]; then docker rm -f "$OWNED_CONTAINER_ID" >/dev/null 2>&1 || true; fi
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
SOURCE_SHA="$(git rev-parse HEAD)"
docker build --platform=linux/amd64 --file=Dockerfile.runtime-migrator --build-arg="SOURCE_SHA=$SOURCE_SHA" --tag="$IMAGE" .
OWNED_CONTAINER_ID="$(docker create --network none "$IMAGE" -inventory-only)"
docker cp "$OWNED_CONTAINER_ID:/app/db/query/img01_schema_release_evidence.sql" "$TMP_DIR/schema-query.sql"
cmp db/query/img01_schema_release_evidence.sql "$TMP_DIR/schema-query.sql"
docker rm "$OWNED_CONTAINER_ID" >/dev/null
OWNED_CONTAINER_ID=''
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
# Validate the compiled source binding without a network or database. A correct
# SHA reaches the target guard, while a different SHA fails invocation first.
FINGERPRINT="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))["inventory_sha256"])' "$TMP_DIR/source.json")"
for candidate in "$SOURCE_SHA" 0000000000000000000000000000000000000000; do
  if docker run --rm --network none --read-only --cap-drop ALL \
    --security-opt=no-new-privileges -e DATABASE_URL='postgres://unused@localhost/unused?sslmode=disable' \
    "$IMAGE" -release -release-source-sha "$candidate" -release-inventory-sha256 "$FINGERPRINT" \
    -release-expected-applied 83 > "$TMP_DIR/release-$candidate.log" 2>&1; then
    echo 'release accepted an unreviewed local target' >&2
    exit 1
  fi
done
grep -Fq 'release requires the reviewed private Cloud SQL target' "$TMP_DIR/release-$SOURCE_SHA.log"
grep -Fq 'release requires the embedded source SHA' "$TMP_DIR/release-0000000000000000000000000000000000000000.log"
if docker run --rm --network none --read-only --cap-drop ALL --security-opt=no-new-privileges \
  "$IMAGE" -schema-evidence-only -plan-only > "$TMP_DIR/schema-negative.log" 2>&1; then
  echo 'catalog mode accepted another execution mode' >&2; exit 1
fi
grep -Fq -- '-schema-evidence-only cannot combine' "$TMP_DIR/schema-negative.log"
echo 'PASS: runtime inventory, embedded release SHA and target/mode guards tested without database access.'
