#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${1:-$(date -u +%Y%m%dT%H%M%S)-$$}"
if [[ ! "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; then
	echo "run id must contain 1-64 safe filename characters" >&2
	exit 2
fi
OUTPUT_REL=".local/testing/gcp03-restore-rehearsal-20261016/$RUN_ID"
OUTPUT_DIR="$ROOT_DIR/$OUTPUT_REL"
mkdir -p "$(dirname -- "$OUTPUT_DIR")"
chmod 700 "$(dirname -- "$OUTPUT_DIR")"
if [[ -e "$OUTPUT_DIR" ]]; then
	echo "refusing to overwrite existing run evidence: $OUTPUT_REL" >&2
	exit 2
fi
mkdir -m 700 "$OUTPUT_DIR"
python3 "$ROOT_DIR/scripts/gcp03-restore-fixture.py" --output-dir "$OUTPUT_REL"

export GCP03_RESTORE_FIXTURE_PATH="$OUTPUT_DIR/fixture.sql"
export GCP03_RESTORE_EVIDENCE_DIR="$OUTPUT_DIR"
export GO_TEST_RUN='^TestSyntheticRestoreRehearsalPostgres18$'
exec bash "$ROOT_DIR/scripts/test-m04-attributes-postgres.sh" ./internal/gcp03scratch
