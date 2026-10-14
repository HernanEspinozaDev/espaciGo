#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="$(python3 -c 'import secrets; print(secrets.token_hex(6))')"
PROJECT="espacigo_localclose02_demo_${RUN_ID}"
STATE_DIR="$ROOT_DIR/.local/testing/local-close-02-demo/$RUN_ID"
BASE_PORT="${LOCAL_CLOSE02_DEMO_PORT_BASE:-20870}"

if [[ ! "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] || (( BASE_PORT < 1024 || BASE_PORT > 65000 )); then
  printf 'LOCAL_CLOSE02_DEMO_PORT_BASE must be an unprivileged TCP port from 1024 to 65000\n' >&2
  exit 2
fi
python3 - "$BASE_PORT" <<'PY'
import socket, sys
base=int(sys.argv[1])
for port in range(base, base+4):
    with socket.socket() as sock:
        try: sock.bind(('127.0.0.1', port))
        except OSError: raise SystemExit(f'LOCAL-CLOSE-02 demo port {port} is already in use')
PY
for resource in "$PROJECT" "${PROJECT}_pgdata"; do
  if docker ps --all --filter "name=^/${resource}" --format '{{.Names}}' | grep -Fxq "$resource" \
    || docker volume ls --format '{{.Name}}' | grep -Fxq "$resource"; then
    printf 'LOCAL-CLOSE-02 demo resource already exists: %s\n' "$resource" >&2
    exit 2
  fi
done
[[ ! -e "$STATE_DIR" ]] || { printf 'LOCAL-CLOSE-02 demo state path already exists\n' >&2; exit 2; }
install -d -m 0700 "$ROOT_DIR/.local/testing/local-close-02-demo"
printf 'Starting isolated LOCAL-CLOSE-02 review demo. Project=%s, ports=%s-%s, volume=%s_pgdata\n' "$PROJECT" "$BASE_PORT" "$((BASE_PORT + 3))" "$PROJECT"
cd "$ROOT_DIR"
FE14_RUN_ID="$RUN_ID" \
FE14_PROJECT_NAME="$PROJECT" \
FE14_STATE_DIR="$STATE_DIR" \
FE14_RETAIN_STACK=1 \
FE14_RETAIN_KIND=local-close-02 \
FE14_PUBLIC_ONLY=1 \
FE14_LOCAL_CLOSE02_CLOCK=1 \
FE14_PORT_BASE="$BASE_PORT" \
FE14_EVIDENCE_OUTPUT_DIR="$ROOT_DIR/planning/cierre_local/evidencia/local-close-02" \
FE14_PUBLIC_TEST_GREP='MPV2-07 serves public catalog|Anonymous visitor explores the Go-backed public catalog and minimized detail|MPV2-13 serves real 48-item keyset pages|MPV2-13 renders a fixed four-column desktop catalog and a one-column mobile list|MPV2-04 host declares a photo public|Go-backed role navigation, private JSON/ZIP exports|LOCAL-CLOSE-02 serves a signed reservation through its financial close' \
  bash "$ROOT_DIR/scripts/test-frontend-e2e.sh"
printf 'Demo LOCAL-CLOSE-02 disponible en http://127.0.0.1:%s; el smoke SSR independiente se detuvo al finalizar. Credenciales privadas: %s/accounts.json.\n' "$((BASE_PORT + 2))" "$STATE_DIR"
