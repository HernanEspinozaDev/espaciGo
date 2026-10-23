#!/usr/bin/env bash
# Captures an operator-attested read-only report. Hashing the result and
# storing it in GCS later protects integrity/freshness, not DB provenance.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
QUERY_FILE="$ROOT_DIR/db/query/img01_schema_release_evidence.sql"
OUTPUT_DIR="${IMG01_SCHEMA_EVIDENCE_DIR:-$ROOT_DIR/.local/testing}"

if [[ "${PGPASSWORD+x}" == x ]]; then
  echo "PGPASSWORD is not accepted; provide an owner-controlled PGPASSFILE." >&2
  exit 2
fi
if [[ "${PGDATABASE:-}" != "espacigo_dms_stage_20261020" ]]; then
  echo "PGDATABASE must name the reviewed test database." >&2
  exit 2
fi
if [[ -z "${PGHOST:-}" || -z "${PGUSER:-}" || -z "${PGPASSFILE:-}" ]]; then
  echo "PGHOST, PGUSER, and PGPASSFILE are required." >&2
  exit 2
fi
if [[ ! -f "$PGPASSFILE" || -L "$PGPASSFILE" || "$(stat -c '%a' "$PGPASSFILE")" != "600" ]]; then
  echo "PGPASSFILE must be a regular, non-symlink file with mode 0600." >&2
  exit 2
fi
if [[ "$PGUSER" == "espacigo_runtime" || "$PGUSER" == "espacigo_api_login" ]]; then
  echo "Use an approved read-only/catalog-capable operator session, not the application runtime." >&2
  exit 2
fi

umask 077
mkdir -p "$OUTPUT_DIR"
chmod 700 "$OUTPUT_DIR"
tmp_report="$(mktemp "$OUTPUT_DIR/.img01-schema-evidence.XXXXXX")"
tmp_json="$(mktemp "$OUTPUT_DIR/.img01-schema-evidence-json.XXXXXX")"
trap 'rm -- "$tmp_report" "$tmp_json" 2>/dev/null || true' EXIT

psql -X -qAt -v ON_ERROR_STOP=1 -f "$QUERY_FILE" > "$tmp_report"
python3 - "$tmp_report" "$QUERY_FILE" "$tmp_json" <<'PY'
import hashlib
import json
import pathlib
import sys

report_path, query_path, output_path = map(pathlib.Path, sys.argv[1:])
lines = [line for line in report_path.read_text(encoding="utf-8").splitlines() if line.strip()]
if len(lines) != 1:
    raise SystemExit("expected exactly one JSON evidence row")
report = json.loads(lines[0])
if not isinstance(report, dict):
    raise SystemExit("schema evidence row is not a JSON object")
report["query_sha256"] = hashlib.sha256(query_path.read_bytes()).hexdigest()
output_path.write_text(json.dumps(report, sort_keys=True, separators=(",", ":")) + "\n", encoding="utf-8")
PY

evidence_sha="$(sha256sum "$tmp_json" | awk '{print $1}')"
final_path="$OUTPUT_DIR/img01-schema-evidence-${evidence_sha}.json"
if [[ -e "$final_path" ]]; then
  echo "matching evidence file already exists; refusing to overwrite it." >&2
  exit 1
fi
mv "$tmp_json" "$final_path"
chmod 600 "$final_path"
printf 'Read-only schema evidence saved locally. sha256=%s file=%s\n' "$evidence_sha" "$final_path"
python3 "$ROOT_DIR/scripts/verify_img01_schema_release_evidence.py" "$final_path" --migrations "$ROOT_DIR/db/migrations" --query "$QUERY_FILE"
