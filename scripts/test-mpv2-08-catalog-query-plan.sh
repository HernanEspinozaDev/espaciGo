#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
OUTPUT="$ROOT_DIR/planning/marketplace_publico_v2/evidencia/mpv2_08/query-plan.json"
TEMP_LOG="$(mktemp "${TMPDIR:-/tmp}/mpv2-08-catalog-plan.XXXXXX")"
trap 'rm -f -- "$TEMP_LOG"' EXIT
umask 077

if ! GO_TEST_RUN='^TestLocalPublicCatalogQueryPlanAndSitemapPaging$' LOCAL_QA_GO_JSON=1 \
  bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/booking >"$TEMP_LOG" 2>&1; then
  cat "$TEMP_LOG" >&2
  exit 1
fi
python3 - "$TEMP_LOG" "$OUTPUT" <<'PY'
import json
import pathlib
import sys

log_path, output_path = map(pathlib.Path, sys.argv[1:])
events = []
for line in log_path.read_text(encoding="utf-8").splitlines():
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    if event.get("Test") == "TestLocalPublicCatalogQueryPlanAndSitemapPaging":
        events.append(event.get("Output", ""))
transcript = "".join(events)
marker = "MPV2-08_PUBLIC_QUERY_PLAN="
if marker not in transcript:
    raise SystemExit("integration passed without emitting the expected real-query plan report")
start = transcript.index(marker) + len(marker)
end = transcript.find("\n--- PASS:", start)
if end < 0:
    raise SystemExit("integration query-plan report did not end with a passing test event")
report = json.loads(transcript[start:end].strip())
output_path.parent.mkdir(parents=True, exist_ok=True)
output_path.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
output_path.chmod(0o644)
print(f"Wrote reproducible query-plan report: {output_path}")
print(f"Recorded code HEAD: {report['git_head']}")
PY
