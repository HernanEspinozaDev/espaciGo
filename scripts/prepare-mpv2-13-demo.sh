#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="$(python3 -c 'import secrets; print(secrets.token_hex(6))')"
PROJECT="espacigo_mpv2_13_demo_${RUN_ID}"
STATE_DIR="$ROOT_DIR/.local/testing/mpv2-13-demo/$RUN_ID"
BASE_PORT="${MPV2_13_DEMO_PORT_BASE:-19900}"
EVIDENCE_DIR="$ROOT_DIR/planning/marketplace_publico_v2/evidencia/mpv2_13"

if [[ ! "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] || (( BASE_PORT < 1024 || BASE_PORT > 65000 )); then
  printf 'MPV2_13_DEMO_PORT_BASE must be an unprivileged TCP port from 1024 to 65000\n' >&2
  exit 2
fi

python3 - "$BASE_PORT" <<'PY'
import socket, sys

base=int(sys.argv[1])
for port in range(base, base+4):
    with socket.socket() as sock:
        try:
            sock.bind(('127.0.0.1', port))
        except OSError:
            raise SystemExit(f'review demo port {port} is already in use')
PY

for resource in "$PROJECT" "${PROJECT}_pgdata"; do
  if docker ps --all --filter "name=^/${resource}" --format '{{.Names}}' | grep -Fxq "$resource" \
    || docker volume ls --format '{{.Name}}' | grep -Fxq "$resource"; then
    printf 'review demo resource already exists: %s\n' "$resource" >&2
    exit 2
  fi
done
[[ ! -e "$STATE_DIR" ]] || { printf 'review demo state path already exists: %s\n' "$STATE_DIR" >&2; exit 2; }
install -d -m 0700 "$ROOT_DIR/.local/testing/mpv2-13-demo" "$EVIDENCE_DIR"

printf 'Preparing new isolated MPV2-13 demo. Project: %s; ports: %s-%s; dedicated volume: %s_pgdata\n' "$PROJECT" "$BASE_PORT" "$((BASE_PORT + 2))" "$PROJECT"
printf 'The script validates the resolved Compose project and loopback binds before starting it. Existing MPV2-09 and espacigo_pgdata are not inputs.\n'

cd "$ROOT_DIR"
FE14_RUN_ID="$RUN_ID" \
FE14_PROJECT_NAME="$PROJECT" \
FE14_STATE_DIR="$STATE_DIR" \
FE14_RETAIN_STACK=1 \
FE14_PUBLIC_ONLY=1 \
FE14_PORT_BASE="$BASE_PORT" \
FE14_EVIDENCE_OUTPUT_DIR="$EVIDENCE_DIR" \
FE14_PUBLIC_TEST_GREP='MPV2-07 serves public catalog|Anonymous visitor explores the Go-backed public catalog and minimized detail|MPV2-13 renders a fixed four-column desktop catalog and a one-column mobile list|MPV2-13 serves real 48-item keyset pages|MPV2-04 host declares a photo public' \
  bash "$ROOT_DIR/scripts/test-frontend-e2e.sh"
