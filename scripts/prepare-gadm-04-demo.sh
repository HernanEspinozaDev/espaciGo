#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="$(python3 -c 'import secrets; print(secrets.token_hex(6))')"
PROJECT="espacigo_gadm04_demo_${RUN_ID}"
STATE_DIR="$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID"
BASE_PORT="${GADM04_DEMO_PORT_BASE:-20940}"
[[ "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] && (( BASE_PORT >= 1024 && BASE_PORT <= 64997 )) || { printf 'GADM04_DEMO_PORT_BASE must leave four unprivileged ports available\n' >&2; exit 2; }
python3 - "$BASE_PORT" <<'PY'
import socket,sys
for port in range(int(sys.argv[1]),int(sys.argv[1])+4):
 with socket.socket() as s:
  try: s.bind(('127.0.0.1',port))
  except OSError: raise SystemExit(f'GADM-04 demo port {port} is in use')
PY
for resource in "$PROJECT" "${PROJECT}_pgdata"; do
  if docker ps --all --filter "name=^/${resource}" --format '{{.Names}}' | grep -Fxq "$resource" || docker volume ls --format '{{.Name}}' | grep -Fxq "$resource"; then
    printf 'GADM-04 resource already exists: %s\n' "$resource" >&2; exit 2
  fi
done
[[ ! -e "$STATE_DIR" ]] || { printf 'GADM-04 state already exists\n' >&2; exit 2; }
install -d -m 0700 "$ROOT_DIR/.local/testing/gadm-04-demo"
printf 'Creating disposable, retained GADM-04 review demo. Project=%s, PostGIS volume=%s_pgdata, ports=%s-%s\n' "$PROJECT" "$PROJECT" "$BASE_PORT" "$((BASE_PORT+3))"
cd "$ROOT_DIR"
FE14_RUN_ID="$RUN_ID" FE14_PROJECT_NAME="$PROJECT" FE14_STATE_DIR="$STATE_DIR" \
FE14_RETAIN_STACK=1 FE14_RETAIN_KIND=gadm-04 FE14_PORT_BASE="$BASE_PORT" \
FE14_EVIDENCE_OUTPUT_DIR="$ROOT_DIR/planning/administracion_global/evidencia/gadm_04" \
FE14_TEST_GREP='GADM-04 verifies global-only legacy access, legacy-only access, and outsider denial' \
  bash "$ROOT_DIR/scripts/test-frontend-e2e.sh"
cat >"$STATE_DIR/demo-record.txt" <<EOF
owner=HernanMEC
reason=user-requested isolated GADM-04 review/demo
project=$PROJECT
volume=${PROJECT}_pgdata
ports=$BASE_PORT-$((BASE_PORT+3))
state=$STATE_DIR
EOF
chmod 0600 "$STATE_DIR/demo-record.txt"
printf '\nGADM-04 demo ready: http://127.0.0.1:%s/ (health: http://127.0.0.1:%s/healthz)\n' "$((BASE_PORT+2))" "$((BASE_PORT+2))"
printf 'Synthetic login credentials: %s/accounts.json (mode 0600); private state: %s\n' "$STATE_DIR" "$STATE_DIR"
printf 'Stop: scripts/stop-gadm-04-demo.sh %s\nRemove: scripts/remove-gadm-04-demo.sh %s\n' "$RUN_ID" "$RUN_ID"
