#!/usr/bin/env bash
# Prove a failed docker run cannot clean up a pre-existing same-name container.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
TMP_DIR="$(mktemp -d /tmp/espacigo-auth-browser-guard.XXXXXX)"
cleanup() { python3 -c 'import pathlib,shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"; }
trap cleanup EXIT INT TERM
mkdir "$TMP_DIR/bin"
cat > "$TMP_DIR/bin/docker" <<'DOCKER'
#!/usr/bin/env bash
printf '%s\n' "$1" >> "$DOCKER_CALL_LOG"
if [[ "$1" == "run" ]]; then exit 1; fi
exit 0
DOCKER
chmod 0700 "$TMP_DIR/bin/docker"
export DOCKER_CALL_LOG="$TMP_DIR/docker-calls"
if AUTH_BROWSER_FIXTURE_AUTHORIZED=1 AUTH_BROWSER_FRONTEND_URL=http://127.0.0.1:1 PATH="$TMP_DIR/bin:$PATH" scripts/test-auth-real-browser-postgres.sh > "$TMP_DIR/output" 2>&1; then
  echo 'expected injected docker run failure' >&2
  exit 1
fi
python3 - "$DOCKER_CALL_LOG" <<'PY'
from pathlib import Path
import sys
calls=Path(sys.argv[1]).read_text().splitlines()
if calls != ['run']:
    raise SystemExit('runner attempted cleanup after failed container creation')
print('PASS failed docker run never execs into or removes a same-name container')
PY
