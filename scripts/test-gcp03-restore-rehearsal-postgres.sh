#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
OUTPUT_REL=".local/testing/gcp03-restore-rehearsal-20261016"
OUTPUT_DIR="$ROOT_DIR/$OUTPUT_REL"
mkdir -p "$OUTPUT_DIR"
chmod 700 "$OUTPUT_DIR"
python3 "$ROOT_DIR/scripts/gcp03-restore-fixture.py" --output-dir "$OUTPUT_REL"

export GCP03_RESTORE_FIXTURE_PATH="$OUTPUT_DIR/fixture.sql"
export GCP03_RESTORE_EVIDENCE_DIR="$OUTPUT_DIR"
export GO_TEST_RUN='^TestSyntheticRestoreRehearsalPostgres18$'
exec bash "$ROOT_DIR/scripts/test-m04-attributes-postgres.sh" ./internal/gcp03scratch
