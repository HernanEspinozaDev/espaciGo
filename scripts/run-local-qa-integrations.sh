#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
MANIFEST="${1:-scripts/local-required-integrations.txt}"
REPORT="${2:?provide the aggregate go test JSON report path}"
ERRORS="${3:?provide the integration stderr path}"

mapfile -t PACKAGES < <(awk -F: '!/^#/ && NF >= 3 {print $1}' "$MANIFEST" | sort -u)
if ((${#PACKAGES[@]} == 0)); then
  printf 'required integration manifest has no packages: %s\n' "$MANIFEST" >&2
  exit 2
fi

: >"$REPORT"
: >"$ERRORS"
for package in "${PACKAGES[@]}"; do
  printf '== PostgreSQL/PostGIS + Mailpit: %s ==\n' "$package"
  if ! env -u GO_TEST_RUN LOCAL_QA_GO_JSON=1 GO_TEST_RUN='.' \
    bash scripts/test-local-comm-mailpit.sh "./$package" >>"$REPORT" 2>>"$ERRORS"; then
    cat "$ERRORS" >&2
    python3 - "$REPORT" <<'PY'
import json, sys
from pathlib import Path
for line in Path(sys.argv[1]).read_text(encoding="utf-8").splitlines():
    try:
        event = json.loads(line)
    except json.JSONDecodeError:
        continue
    if event.get("Action") == "fail" or event.get("OutputType") == "error":
        package = event.get("Package", "?")
        test = event.get("Test", "")
        output = event.get("Output", "").rstrip()
        print(f"Go integration failure: {package} {test} {output}", file=sys.stderr)
PY
    exit 1
  fi
done

python3 scripts/check_local_qa_results.py "$REPORT" "$MANIFEST"
