#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${GCP04_W4_BUNDLE_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
if [[ ! "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; then
  echo 'GCP04_W4_BUNDLE_RUN_ID must contain 1-64 safe filename characters.' >&2
  exit 2
fi
LOCAL_DIR="$ROOT_DIR/.local/testing"
BUNDLE_DIR="$LOCAL_DIR/gcp04-w4-baseline-${RUN_ID,,}"
mkdir -p -- "$LOCAL_DIR"
chmod 0700 "$ROOT_DIR/.local" "$LOCAL_DIR"
[[ ! -e "$BUNDLE_DIR" && ! -L "$BUNDLE_DIR" ]] || { echo 'Refusing to overwrite an existing private baseline bundle.' >&2; exit 1; }
umask 077
GCP04_W3_RUN_ID="w4-${RUN_ID,,}" \
GCP04_W3_RETAIN_BUNDLE_DIR="$BUNDLE_DIR" \
  bash "$ROOT_DIR/scripts/test-gcp04-w3-logical-baseline-postgres.sh"
"$ROOT_DIR/scripts/gcp04_verify_baseline_bundle.py" "$BUNDLE_DIR" --repo-root "$ROOT_DIR"
