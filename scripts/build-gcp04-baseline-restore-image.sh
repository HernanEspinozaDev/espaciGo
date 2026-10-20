#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
BUNDLE_INPUT="${1:-}"
IMAGE_TAG="${2:-}"

if [[ -z "$BUNDLE_INPUT" || -z "$IMAGE_TAG" ]]; then
  echo 'Usage: scripts/build-gcp04-baseline-restore-image.sh <private-W6-bundle> <local-image-tag>' >&2
  exit 2
fi
if [[ ! "$IMAGE_TAG" =~ ^[a-z0-9./_-]+:[a-zA-Z0-9._-]+$ ]]; then
  echo 'Image tag has an invalid shape.' >&2
  exit 2
fi

BUNDLE_DIR="$(realpath -e -- "$BUNDLE_INPUT")"
if [[ ! -d "$BUNDLE_DIR" || "$BUNDLE_DIR" != */.local/testing/gcp04-w4-baseline-* ]]; then
  echo 'The named baseline context must be a retained private W6 bundle under .local/testing.' >&2
  exit 2
fi
if [[ "$(stat -c '%a' -- "$BUNDLE_DIR")" != 700 ]]; then
  echo 'The retained bundle directory must have mode 0700.' >&2
  exit 2
fi

for file in baseline.dump baseline.manifest.json baseline.toc.original baseline.toc.restore baseline.acl.restore.sql; do
  path="$BUNDLE_DIR/$file"
  if [[ ! -f "$path" || -L "$path" || "$(stat -c '%a' -- "$path")" != 600 ]]; then
    echo "Required private baseline artifact is missing or has unsafe permissions: $file" >&2
    exit 2
  fi
done

docker buildx build --load \
  --file "$ROOT_DIR/Dockerfile.gcp04-baseline-restore" \
  --build-context "baseline=$BUNDLE_DIR" \
  --tag "$IMAGE_TAG" \
  "$ROOT_DIR"

MANIFEST_SHA256="$(sha256sum -- "$BUNDLE_DIR/baseline.manifest.json" | cut -d ' ' -f 1)"
DUMP_SHA256="$(sha256sum -- "$BUNDLE_DIR/baseline.dump" | cut -d ' ' -f 1)"
python3 - "$IMAGE_TAG" "$MANIFEST_SHA256" "$DUMP_SHA256" <<'PY'
import json
import sys

print(json.dumps({
    "format": "espacigo.gcp04.baseline-restore-image.v1",
    "local_image_tag": sys.argv[1],
    "manifest_sha256": sys.argv[2],
    "dump_sha256": sys.argv[3],
    "published": False,
}, separators=(",", ":")))
PY
