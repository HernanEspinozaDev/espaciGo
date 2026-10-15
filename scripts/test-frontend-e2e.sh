#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${FE14_RUN_ID:-$(python3 -c 'import secrets; print(secrets.token_hex(6))')}"
PROJECT="${FE14_PROJECT_NAME:-espacigo_fe14_${RUN_ID}}"
STATE_DIR="${FE14_STATE_DIR:-${TMPDIR:-/tmp}/espacigo-fe14-${RUN_ID}}"
STATE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).expanduser().resolve(strict=False))' "$STATE_DIR")"
BASE_PORT="${FE14_PORT_BASE:-18780}"
LOCAL_CLOSE02_CLOCK="${FE14_LOCAL_CLOSE02_CLOCK:-0}"
if [[ ! "$RUN_ID" =~ ^[0-9a-f]{12}$ ]]; then
  printf 'FE14_RUN_ID must be 12 lowercase hexadecimal characters\n' >&2
  exit 2
fi
if [[ "$LOCAL_CLOSE02_CLOCK" == "1" ]]; then
  if [[ "${FE14_RETAIN_STACK:-0}" == "1" && "${FE14_RETAIN_KIND:-}" == "local-close-02" ]]; then
    LOCAL_CLOSE02_CLOCK_KIND=demo
    expected_state="$ROOT_DIR/.local/testing/local-close-02-demo/$RUN_ID"
    expected_project="espacigo_localclose02_demo_${RUN_ID}"
  else
    LOCAL_CLOSE02_CLOCK_KIND=run
    expected_state="$ROOT_DIR/.local/testing/local-close-02/$RUN_ID"
    expected_project="espacigo_localclose02_${RUN_ID}"
  fi
  [[ "$PROJECT" == "$expected_project" && "$STATE_DIR" == "$expected_state" ]] || {
    printf 'manual clock is restricted to this run\x27s disposable LOCAL-CLOSE-02 project/state path or its retained LOCAL-CLOSE-02 demo path\n' >&2
    exit 2
  }
  export LOCAL_CLOSE02_CLOCK_KIND
fi
if [[ "${FE14_RETAIN_STACK:-0}" == "1" ]]; then
  case "${FE14_RETAIN_KIND:-mpv2-13}" in
    mpv2-13)
      expected_state="$ROOT_DIR/.local/testing/mpv2-13-demo/$RUN_ID"
      expected_project="espacigo_mpv2_13_demo_${RUN_ID}"
      retain_label='MPV2-13'
      ;;
    local-close-02)
      expected_state="$ROOT_DIR/.local/testing/local-close-02-demo/$RUN_ID"
      expected_project="espacigo_localclose02_demo_${RUN_ID}"
      retain_label='LOCAL-CLOSE-02'
      ;;
    gadm-04)
      expected_state="$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID"
      expected_project="espacigo_gadm04_demo_${RUN_ID}"
      retain_label='GADM-04'
      ;;
    *) printf 'unsupported retained demo kind\n' >&2; exit 2 ;;
  esac
  if [[ "${FE14_RETAIN_KIND:-}" == "gadm-04" ]]; then
    [[ "$PROJECT" == "$expected_project" && "$STATE_DIR" == "$expected_state" && "${FE14_TEST_GREP:-}" == *GADM-04* ]] || {
      printf 'retained GADM-04 stack is restricted to its dedicated wrapper and state path\n' >&2; exit 2;
    }
  elif [[ "${FE14_PUBLIC_ONLY:-0}" != "1" || "$PROJECT" != "$expected_project" || "$STATE_DIR" != "$expected_state" ]]; then
    printf 'retained FE-14 stack is restricted to a dedicated demo wrapper and state path\n' >&2
    exit 2
  fi
  [[ ! -e "$STATE_DIR" ]] || { printf 'retained demo state path already exists\n' >&2; exit 2; }
fi
if [[ ! "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] || (( BASE_PORT < 1024 || BASE_PORT > 64997 )); then
  printf 'FE14_PORT_BASE must leave four unprivileged TCP ports available (1024–64997)\n' >&2
  exit 2
fi
LOCAL_SSR_PORT="$((BASE_PORT + 3))"
python3 - "$BASE_PORT" <<'PY'
import re
import socket
import subprocess
import sys

base = int(sys.argv[1])
ports = range(base, base + 4)
try:
    published = subprocess.run(
        ["docker", "ps", "--format", "{{.Ports}}"],
        check=True, capture_output=True, text=True,
    ).stdout
except (OSError, subprocess.CalledProcessError) as exc:
    raise SystemExit(f"unable to inspect published Docker ports before FE-14: {exc}")
for port in ports:
    if any(re.search(rf"(?:127\.0\.0\.1|0\.0\.0\.0|\[::\]|::):{port}->", line) for line in published.splitlines()):
        raise SystemExit(f"FE-14 host port {port} is already published by a running container; choose a free FE14_PORT_BASE")
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    try:
        sock.bind(("127.0.0.1", port))
    except OSError as exc:
        raise SystemExit(f"FE-14 host port {port} is already in use; choose a free FE14_PORT_BASE: {exc}")
    finally:
        sock.close()
PY

export LOCAL_DOCKER_PROJECT_NAME="$PROJECT"
export LOCAL_DOCKER_IMAGE_PREFIX="$PROJECT"
export LOCAL_DOCKER_STATE_DIR="$STATE_DIR"
export LOCAL_DOCKER_PGDATA_VOLUME="${PROJECT}_pgdata"
export LOCAL_DOCKER_API_PORT="$BASE_PORT"
export LOCAL_DOCKER_MAILPIT_PORT="$((BASE_PORT + 1))"
export LOCAL_DOCKER_ANGULAR_PORT="$((BASE_PORT + 2))"
export LOCAL_DOCKER_SSR_PORT="$LOCAL_SSR_PORT"
export LOCAL_COMPOSE_PROJECT_NAME="$PROJECT"
export LOCAL_IMAGE_PREFIX="$PROJECT"
export LOCAL_API_PORT="$LOCAL_DOCKER_API_PORT"
export LOCAL_MAILPIT_PORT="$LOCAL_DOCKER_MAILPIT_PORT"
export LOCAL_ANGULAR_PORT="$LOCAL_DOCKER_ANGULAR_PORT"
export LOCAL_PGDATA_VOLUME="$LOCAL_DOCKER_PGDATA_VOLUME"
export LOCAL_DATABASE_NAME="espacigo_local"
export LOCAL_DATABASE_HOST="database"
export LOCAL_DATABASE_PORT="5432"
export LOCAL_DATABASE_ADMIN_USER="espacigo_admin"
export LOCAL_DATABASE_RUNTIME_USER="espacigo_runtime"
export DATABASE_SSLMODE="disable"
export LOCAL_FRONTEND_E2E=1
export FE14_HEAD="$(git -C "$ROOT_DIR" rev-parse HEAD)"
export FE14_ACCOUNT_FILE_HOST="$STATE_DIR/accounts.json"
export FE14_ARTIFACT_DIR_HOST="$STATE_DIR/artifacts"
export FE14_RESULTS_DIR_HOST="$STATE_DIR/results"
FE14_EVIDENCE_PREFIX=""
case "${FE14_EVIDENCE_OUTPUT_DIR:-}" in
  */mpv2_03) FE14_EVIDENCE_PREFIX='fe14_mpv2_03_' ;;
  */mpv2_04) FE14_EVIDENCE_PREFIX='fe14_mpv2_04_' ;;
  */mpv2_05) FE14_EVIDENCE_PREFIX='fe14_mpv2_05_' ;;
  */mpv2_06) FE14_EVIDENCE_PREFIX='fe14_mpv2_06_' ;;
  */mpv2_07) FE14_EVIDENCE_PREFIX='fe14_mpv2_07_' ;;
  */mpv2_10) FE14_EVIDENCE_PREFIX='fe14_mpv2_10_' ;;
  */mpv2_11) FE14_EVIDENCE_PREFIX='fe14_mpv2_11_' ;;
  */mpv2_12) FE14_EVIDENCE_PREFIX='fe14_mpv2_12_' ;;
  */mpv2_13) FE14_EVIDENCE_PREFIX='fe14_mpv2_13_' ;;
  */gadm_02) FE14_EVIDENCE_PREFIX='fe14_gadm_02_' ;;
  */gadm_03) FE14_EVIDENCE_PREFIX='fe14_gadm_03_' ;;
  */gadm_04) FE14_EVIDENCE_PREFIX='fe14_gadm_' ;;
  */local-close-01) FE14_EVIDENCE_PREFIX='fe14_local_close_01_' ;;
  */local-close-02) FE14_EVIDENCE_PREFIX='fe14_local_close_02_' ;;
esac
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
export LOCAL_SECRETS_DIR="$STATE_DIR/secrets"
export LOCAL_M03_EVIDENCE_DIR="$STATE_DIR/private-evidence"
export LOCAL_PRIVACY_REPLAY_DIR="$STATE_DIR/privacy-replay"
export LOCAL_DATABASE_TLS_DIR="$STATE_DIR/database-tls"
umask 077
mkdir -p "$STATE_DIR/artifacts"
mkdir -p "$STATE_DIR/results"
chmod 0700 "$STATE_DIR" "$STATE_DIR/artifacts" "$STATE_DIR/results"
export LOCAL_DOCKER_COMPOSE_OVERRIDE="$STATE_DIR/network-pools.yaml"
if [[ "$LOCAL_CLOSE02_CLOCK" == "1" ]]; then export LOCAL_CLOSE02_CLOCK_HOST="$STATE_DIR/business-clock"; fi

mapfile -t FE14_SUBNETS < <(python3 - <<'PY'
import ipaddress
import json
import subprocess

ids = subprocess.check_output(['docker', 'network', 'ls', '-q'], text=True).split()
used = []
if ids:
    for network in json.loads(subprocess.check_output(['docker', 'network', 'inspect', *ids], text=True)):
        for item in (network.get('IPAM') or {}).get('Config') or []:
            if item.get('Subnet'):
                try:
                    used.append(ipaddress.ip_network(item['Subnet'], strict=False))
                except ValueError:
                    pass
for octet in range(240, 255):
    candidates = [ipaddress.ip_network(f'10.{octet}.{third}.0/24') for third in range(3)]
    if all(not candidate.overlaps(existing) for candidate in candidates for existing in used):
        print(*candidates, sep='\n')
        break
else:
    raise SystemExit('No free FE-14 subnet triplet found')
PY
)
if (( ${#FE14_SUBNETS[@]} != 3 )); then
  printf 'Could not select three free FE-14 Docker subnets\n' >&2
  exit 1
fi
cat > "$LOCAL_DOCKER_COMPOSE_OVERRIDE" <<EOF
networks:
  api:
    ipam:
      config:
        - subnet: ${FE14_SUBNETS[0]}
  mail:
    ipam:
      config:
        - subnet: ${FE14_SUBNETS[1]}
  data:
    ipam:
      config:
        - subnet: ${FE14_SUBNETS[2]}
EOF
if [[ "$LOCAL_CLOSE02_CLOCK" == "1" ]]; then
  mkdir -m 0700 -p "$STATE_DIR/business-clock"
  LOCAL_CLOSE02_CLOCK_KIND="${LOCAL_CLOSE02_CLOCK_KIND:-run}" python3 "$ROOT_DIR/scripts/local-close-02-business-clock" "$RUN_ID" init >/dev/null
  cat >> "$LOCAL_DOCKER_COMPOSE_OVERRIDE" <<EOF
services:
  backend:
    environment:
      LOCAL_DEMO_BUSINESS_CLOCK_FILE: /run/fe14-demo/business-clock.json
    volumes:
      - type: bind
        source: ${LOCAL_CLOSE02_CLOCK_HOST}
        target: /run/fe14-demo
        read_only: true
EOF
fi
compose=(docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" -f "$ROOT_DIR/compose.frontend.yaml" -f "$ROOT_DIR/compose.frontend-e2e.yaml" -f "$LOCAL_DOCKER_COMPOSE_OVERRIDE" -p "$PROJECT")
"${compose[@]}" config --format json >"$STATE_DIR/compose.resolved.json"
python3 - "$STATE_DIR/compose.resolved.json" "$PROJECT" "$LOCAL_DOCKER_PGDATA_VOLUME" <<'PY'
import json, pathlib, sys
config=json.loads(pathlib.Path(sys.argv[1]).read_text())
project, expected_volume=sys.argv[2:]
volumes=config.get('volumes') or {}
if not volumes or expected_volume not in {item.get('name') for item in volumes.values()}:
    raise SystemExit('resolved FE-14 Compose does not use its unique database volume')
if any(item.get('name') == 'espacigo_pgdata' or not item.get('name','').startswith(project+'_') for item in volumes.values()):
    raise SystemExit('resolved FE-14 Compose references an unexpected named volume')
for service in (config.get('services') or {}).values():
    for port in service.get('ports') or []:
        if port.get('host_ip') != '127.0.0.1':
            raise SystemExit('resolved FE-14 Compose publishes a service outside loopback')
    for mount in service.get('volumes') or []:
        if mount.get('type') == 'volume' and mount.get('source') == 'espacigo_pgdata':
            raise SystemExit('resolved FE-14 Compose references persistent espacigo_pgdata')
print('PASS resolved FE-14 Compose uses only run-specific named volumes')
if (config.get('services') or {}).get('database',{}).get('ports'):
    raise SystemExit('resolved FE-14 Compose unexpectedly exposes the database port')
PY
E2E_PID=""
LOCAL_SSR_PID=""
print_sanitized_e2e_log() {
  python3 - "$STATE_DIR/accounts.json" "$STATE_DIR/e2e.log" <<'PY'
import json, pathlib, re, sys
accounts=json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='utf-8'))
text=pathlib.Path(sys.argv[2]).read_text(errors='replace')
for item in accounts.values():
    text=text.replace(item['email'],'[synthetic-email]').replace(item['password'],'[synthetic-password]')
text=re.sub(r'(?i)(authorization|bearer|access_token|token)([ :=]+)[^\s,;]+',r'\1\2[redacted]',text)
print(text[-9000:])
PY
}
cleanup() {
  local status=$?
  if [[ -n "$E2E_PID" ]]; then
    kill "$E2E_PID" >/dev/null 2>&1 || true
    wait "$E2E_PID" >/dev/null 2>&1 || true
  fi
  if [[ -n "$LOCAL_SSR_PID" ]]; then
    kill -TERM "$LOCAL_SSR_PID" >/dev/null 2>&1 || true
    wait "$LOCAL_SSR_PID" >/dev/null 2>&1 || true
  fi
  if [[ "${FE14_RETAIN_STACK:-0}" == "1" && "$status" == "0" ]]; then
    if [[ -n "${FE14_EVIDENCE_OUTPUT_DIR:-}" && "${FE14_PUBLIC_ONLY:-0}" == "1" ]]; then
      mkdir -p "$FE14_EVIDENCE_OUTPUT_DIR"
      case "$FE14_EVIDENCE_PREFIX" in
        fe14_mpv2_13_) evidence_artifacts=(fe14_mpv2_13_catalog_desktop.jpg fe14_mpv2_13_catalog_desktop.json fe14_mpv2_13_catalog_mobile.jpg fe14_mpv2_13_catalog_mobile.json) ;;
        fe14_local_close_02_) evidence_artifacts=(fe14_local_close_02_closed_finance_desktop.jpg fe14_local_close_02_closed_finance_desktop.json fe14_local_close_02_closed_history_renter_desktop.jpg fe14_local_close_02_closed_history_renter_desktop.json fe14_local_close_02_closed_history_renter_mobile.jpg fe14_local_close_02_closed_history_renter_mobile.json fe14_local_close_02_resolved_receipt_renter_desktop.jpg fe14_local_close_02_resolved_receipt_renter_desktop.json fe14_local_close_02_resolved_receipt_renter_mobile.jpg fe14_local_close_02_resolved_receipt_renter_mobile.json) ;;
        *) evidence_artifacts=() ;;
      esac
      for artifact in "${evidence_artifacts[@]}"; do
        if [[ -f "$STATE_DIR/artifacts/$artifact" ]]; then
          evidence_name="$artifact"
          if [[ "${FE14_RETAIN_STACK:-0}" == "1" ]]; then evidence_name="${artifact%.*}_${RUN_ID}.${artifact##*.}"; fi
          cp -- "$STATE_DIR/artifacts/$artifact" "$FE14_EVIDENCE_OUTPUT_DIR/$evidence_name"
          chmod 0644 "$FE14_EVIDENCE_OUTPUT_DIR/$evidence_name"
        fi
      done
    fi
    cat >"$STATE_DIR/runtime.env" <<EOF
LOCAL_DOCKER_PROJECT_NAME=$PROJECT
LOCAL_DOCKER_IMAGE_PREFIX=$PROJECT
LOCAL_DOCKER_STATE_DIR=$STATE_DIR
LOCAL_DOCKER_PGDATA_VOLUME=${PROJECT}_pgdata
LOCAL_DOCKER_API_PORT=$BASE_PORT
LOCAL_DOCKER_MAILPIT_PORT=$((BASE_PORT + 1))
LOCAL_DOCKER_ANGULAR_PORT=$((BASE_PORT + 2))
EOF
    chmod 0600 "$STATE_DIR/runtime.env"
    rm -rf -- "$STATE_DIR/results"
    rm -f -- "$STATE_DIR/stack-build.log" "$STATE_DIR/e2e-build.log" "$STATE_DIR/e2e.log" "$STATE_DIR/local-ssr.log"
    printf 'RETAINED %s review demo: project=%s volume=%s ports=%s-%s state=%s\n' "$retain_label" "$PROJECT" "${PROJECT}_pgdata" "$BASE_PORT" "$((BASE_PORT + 2))" "$STATE_DIR"
    printf 'Owner: HernanMEC. Reason: user-requested isolated review demo; retain pending the user product decision. Synthetic account credentials: %s/accounts.json (mode 0600).\n' "$STATE_DIR"
    return "$status"
  fi
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" down-frontend >/dev/null 2>&1 || true
  local -a ids=()
  mapfile -t ids < <(docker ps --all --quiet --filter "label=com.docker.compose.project=${PROJECT}")
  if (( ${#ids[@]} > 0 )); then docker rm --force "${ids[@]}" >/dev/null 2>&1 || true; fi
  mapfile -t ids < <(docker network ls --quiet --filter "label=com.docker.compose.project=${PROJECT}")
  if (( ${#ids[@]} > 0 )); then docker network rm "${ids[@]}" >/dev/null 2>&1 || true; fi
  docker volume rm "$LOCAL_DOCKER_PGDATA_VOLUME" >/dev/null 2>&1 || true
  if [[ -n "${FE14_EVIDENCE_OUTPUT_DIR:-}" && "${FE14_PUBLIC_ONLY:-0}" == "1" ]]; then
    mkdir -p "$FE14_EVIDENCE_OUTPUT_DIR"
    case "$FE14_EVIDENCE_PREFIX" in
      fe14_mpv2_05_) evidence_artifacts=(fe14_mpv2_05_catalog_desktop.jpg fe14_mpv2_05_catalog_desktop.json fe14_mpv2_05_filters_mobile.jpg fe14_mpv2_05_filters_mobile.json) ;;
      fe14_mpv2_06_) evidence_artifacts=(fe14_mpv2_06_context_desktop.jpg fe14_mpv2_06_context_desktop.json fe14_mpv2_06_context_mobile.jpg fe14_mpv2_06_context_mobile.json) ;;
      fe14_mpv2_07_) evidence_artifacts=(fe14_mpv2_07_catalog_desktop.jpg fe14_mpv2_07_catalog_desktop.json fe14_mpv2_07_catalog_mobile.jpg fe14_mpv2_07_catalog_mobile.json fe14_mpv2_07_detail_desktop.jpg fe14_mpv2_07_detail_desktop.json fe14_mpv2_07_detail_mobile.jpg fe14_mpv2_07_detail_mobile.json) ;;
      fe14_mpv2_10_) evidence_artifacts=(fe14_mpv2_10_sort_desktop.jpg fe14_mpv2_10_sort_desktop.json fe14_mpv2_10_sort_mobile.jpg fe14_mpv2_10_sort_mobile.json) ;;
      fe14_mpv2_11_) evidence_artifacts=(fe14_mpv2_11_typed_filters_desktop.jpg fe14_mpv2_11_typed_filters_desktop.json fe14_mpv2_11_typed_filters_mobile.jpg fe14_mpv2_11_typed_filters_mobile.json) ;;
      fe14_mpv2_12_) evidence_artifacts=(fe14_mpv2_12_grid_4_desktop.jpg fe14_mpv2_12_grid_4_desktop.json fe14_mpv2_12_grid_2_mobile.jpg fe14_mpv2_12_grid_2_mobile.json) ;;
      fe14_mpv2_13_) evidence_artifacts=(fe14_mpv2_13_catalog_desktop.jpg fe14_mpv2_13_catalog_desktop.json fe14_mpv2_13_catalog_mobile.jpg fe14_mpv2_13_catalog_mobile.json) ;;
      fe14_local_close_02_) evidence_artifacts=(fe14_local_close_02_closed_finance_desktop.jpg fe14_local_close_02_closed_finance_desktop.json fe14_local_close_02_closed_history_renter_desktop.jpg fe14_local_close_02_closed_history_renter_desktop.json fe14_local_close_02_closed_history_renter_mobile.jpg fe14_local_close_02_closed_history_renter_mobile.json fe14_local_close_02_resolved_receipt_renter_desktop.jpg fe14_local_close_02_resolved_receipt_renter_desktop.json fe14_local_close_02_resolved_receipt_renter_mobile.jpg fe14_local_close_02_resolved_receipt_renter_mobile.json) ;;
      *) evidence_artifacts=() ;;
    esac
    for artifact in "${evidence_artifacts[@]}"; do
      if [[ -f "$STATE_DIR/artifacts/$artifact" ]]; then
        evidence_name="$artifact"
        if [[ "${FE14_RETAIN_STACK:-0}" == "1" ]]; then evidence_name="${artifact%.*}_${RUN_ID}.${artifact##*.}"; fi
        cp -- "$STATE_DIR/artifacts/$artifact" "$FE14_EVIDENCE_OUTPUT_DIR/$evidence_name"
        chmod 0644 "$FE14_EVIDENCE_OUTPUT_DIR/$evidence_name"
      fi
    done
  fi
  if (( status != 0 )) && [[ "${FE14_PRESERVE_FAILURE_STATE:-0}" == "1" ]]; then
    printf 'Preserved failed FE-14 state for inspection: %s\n' "$STATE_DIR" >&2
  else
    rm -rf -- "$STATE_DIR/results"
    rm -rf -- "$STATE_DIR"
  fi
  return "$status"
}
trap cleanup EXIT

printf 'FE-14 disposable project: %s\n' "$PROJECT"
printf 'API/Angular/Mailpit ports: %s/%s/%s\n' "$LOCAL_DOCKER_API_PORT" "$LOCAL_DOCKER_ANGULAR_PORT" "$LOCAL_DOCKER_MAILPIT_PORT"
printf 'New database volume: %s; no persistent volume is referenced\n' "$LOCAL_DOCKER_PGDATA_VOLUME"
printf 'E2E bind mounts: read-only accounts file, artifacts directory, results directory only\n'
bash "$ROOT_DIR/scripts/local-docker-stack.sh" up-frontend >"$STATE_DIR/stack-build.log" 2>&1 || { tail -n 120 "$STATE_DIR/stack-build.log" >&2; "${compose[@]}" ps --all >&2 || true; "${compose[@]}" logs --no-color migrate >&2 || true; exit 1; }
curl --silent --show-error --fail --max-time 5 "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null
curl --silent --show-error --fail --max-time 5 "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/healthz" >/dev/null
printf 'PASS Angular and Go readiness are HTTP 200 before authenticated E2E\n'

LOCAL_FRONTEND_HOST=0.0.0.0 \
  LOCAL_FRONTEND_PORT="$LOCAL_DOCKER_SSR_PORT" \
  API_INTERNAL_BASE_URL="http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/api/v1" \
  API_PUBLIC_BASE_URL=http://localhost:8080/api/v1 \
  FRONTEND_PUBLIC_ORIGIN="http://127.0.0.1:${LOCAL_DOCKER_SSR_PORT}" \
  node "$ROOT_DIR/frontend/tooling/start-local-ssr.mjs" >"$STATE_DIR/local-ssr.log" 2>&1 &
  LOCAL_SSR_PID=$!
  local_ssr_ready=0
  for _ in $(seq 1 120); do
    if curl --silent --fail --max-time 2 "http://127.0.0.1:${LOCAL_DOCKER_SSR_PORT}/healthz" >/dev/null; then
      local_ssr_ready=1
      break
    fi
    if ! kill -0 "$LOCAL_SSR_PID" >/dev/null 2>&1; then
      tail -n 100 "$STATE_DIR/local-ssr.log" >&2
      exit 1
    fi
    sleep 1
  done
  if (( local_ssr_ready != 1 )); then
    tail -n 100 "$STATE_DIR/local-ssr.log" >&2
    printf 'Timed out waiting for npm start local SSR smoke server\n' >&2
    exit 1
  fi
printf 'PASS npm start local SSR command serves /healthz with explicit local API/origin configuration\n'

python3 "$ROOT_DIR/scripts/provision-fe14-accounts.py" \
  "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}" \
  "http://127.0.0.1:${LOCAL_DOCKER_MAILPIT_PORT}" \
  "$RUN_ID" "$STATE_DIR/accounts.json"

"${compose[@]}" exec -T database psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 \
  --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local <<'SQL' >/dev/null
INSERT INTO public.rol_usuario (usuario_id, rol)
SELECT id, 'arrendador' FROM public.usuario
WHERE correo_normalizado = 'fe14-' || :'run_id' || '-host@ejemplo.invalid'
ON CONFLICT (usuario_id, rol) DO NOTHING;
INSERT INTO public.rol_usuario (usuario_id, rol)
SELECT id, 'administrador' FROM public.usuario
WHERE correo_normalizado = 'fe14-' || :'run_id' || '-admin@ejemplo.invalid'
ON CONFLICT (usuario_id, rol) DO NOTHING;
INSERT INTO public.perfil_usuario(usuario_id,nombre_visible)
SELECT id,'Arrendatario Sintetico' FROM public.usuario
WHERE correo_normalizado='fe14-'||:'run_id'||'-renter@ejemplo.invalid'
ON CONFLICT(usuario_id) DO UPDATE SET nombre_visible=EXCLUDED.nombre_visible,actualizado_en=clock_timestamp();
SQL
admin_password="$(<"$LOCAL_SECRETS_DIR/db_admin_password")"
mkdir -m 0700 "$STATE_DIR/gadm04-local-testing"
python3 - "$STATE_DIR/gadm04-local-testing/activate.env" "$admin_password" <<'PY'
import os, pathlib, sys
path = pathlib.Path(sys.argv[1])
content = f"DATABASE_URL=postgres://espacigo_admin:{sys.argv[2]}@127.0.0.1:5432/espacigo_local\nEXPECTED_DATABASE=espacigo_local\n"
fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
with os.fdopen(fd, 'w') as stream: stream.write(content)
PY
global_admin_id="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only \
  --username espacigo_admin --dbname espacigo_local \
  --command "SELECT id::text FROM public.usuario WHERE correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid'")"
global_admin_id="${global_admin_id//[[:space:]]/}"
CGO_ENABLED=0 go -C "$ROOT_DIR" build -o "$STATE_DIR/gadm-admin" ./cmd/gadm-admin
docker run --rm --network "container:${PROJECT}-database-1" \
  --volume "$STATE_DIR/gadm-admin:/tmp/gadm-admin:ro" \
  --volume "$STATE_DIR/gadm04-local-testing:$ROOT_DIR/.local/testing" \
  --workdir "$ROOT_DIR" golang:1.27.1-alpine3.24 /bin/sh -c "GADM_BOOTSTRAP_ENV=local-activation /tmp/gadm-admin activate-local --action bootstrap --account-id '$global_admin_id' --database espacigo_local --confirm 'ACTIVATE bootstrap $global_admin_id IN espacigo_local' --credentials '$ROOT_DIR/.local/testing/activate.env'"
unset admin_password
ROLE_COUNT="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only \
  --username espacigo_admin --dbname espacigo_local \
  --command "SELECT count(*) FROM public.usuario u JOIN public.rol_usuario r ON r.usuario_id=u.id WHERE u.correo_normalizado LIKE 'fe14-${RUN_ID}-%' AND r.rol IN ('arrendatario','arrendador','administrador')")"
[[ "${ROLE_COUNT//[[:space:]]/}" == "7" ]] || { printf 'Synthetic role grant verification failed\n' >&2; exit 1; }
GLOBAL_ROLE_CHECK="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local --command "SELECT count(*) FROM public.usuario u WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND EXISTS(SELECT 1 FROM public.administrador_global g WHERE g.usuario_id=u.id) AND NOT EXISTS(SELECT 1 FROM public.rol_usuario r WHERE r.usuario_id=u.id AND r.rol='administrador')")"
[[ "${GLOBAL_ROLE_CHECK//[[:space:]]/}" == "1" ]] || { printf 'Global-only fixture unexpectedly has an administrator role or lacks global membership\n' >&2; exit 1; }
LEGACY_ROLE_CHECK="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local --command "SELECT count(*) FROM public.usuario u WHERE u.correo_normalizado='fe14-${RUN_ID}-admin@ejemplo.invalid' AND EXISTS(SELECT 1 FROM public.rol_usuario r WHERE r.usuario_id=u.id AND r.rol='administrador') AND NOT EXISTS(SELECT 1 FROM public.administrador_global g WHERE g.usuario_id=u.id)")"
[[ "${LEGACY_ROLE_CHECK//[[:space:]]/}" == "1" ]] || { printf 'Legacy-only fixture unexpectedly has global membership\n' >&2; exit 1; }
printf 'PASS Go verified five synthetic accounts; global-only, legacy-only, and ordinary actors are separated\n'

if [[ "${FE14_TEST_GREP:-}" == *GADM-03* ]]; then
  "${compose[@]}" exec -T database psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 \
    --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local <<'SQL' >/dev/null
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
SELECT md5(:'run_id'||'-gadm03-account-'||n)::uuid,
       'gadm03-'||:'run_id'||'-'||n||'@ejemplo.invalid',
       'gadm03-'||:'run_id'||'-'||n||'@ejemplo.invalid','synthetic-no-login','activo'
FROM generate_series(1,1000) n;
SQL
  COUNT_1000="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local --command "SELECT count(*) FROM public.usuario WHERE correo_normalizado LIKE 'gadm03-${RUN_ID}-%@ejemplo.invalid'")"
  [[ "${COUNT_1000//[[:space:]]/}" == "1000" ]] || { printf 'GADM-03 synthetic account seed did not contain exactly 1000 rows\n' >&2; exit 1; }
  "${compose[@]}" exec -T database psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 \
    --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local <<'SQL' >"$STATE_DIR/artifacts/gadm03-account-query-plan.txt"
EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON)
SELECT u.id,u.correo_original,p.nombre_visible,u.estado,u.creado_en
FROM public.usuario u LEFT JOIN public.perfil_usuario p ON p.usuario_id=u.id
WHERE u.correo_normalizado LIKE 'gadm03-'||:'run_id'||'-%@ejemplo.invalid'
ORDER BY u.creado_en DESC,u.id DESC LIMIT 25;
SQL
  printf 'PASS GADM-03 seeded 1000 synthetic accounts in disposable PostGIS and captured the real prefix-page EXPLAIN ANALYZE plan\n'
  admin_password="$(<"$LOCAL_SECRETS_DIR/db_admin_password")"
  CGO_ENABLED=0 go -C "$ROOT_DIR" test -c -o "$STATE_DIR/migrator-gadm03.test" ./internal/migrator
  docker run --rm --network "${PROJECT}_data" \
    --volume "$STATE_DIR/migrator-gadm03.test:/tmp/migrator-gadm03.test:ro" \
    --volume "$ROOT_DIR:$ROOT_DIR:ro" --workdir "$ROOT_DIR" \
    --env "TEST_DATABASE_URL=postgres://espacigo_admin:${admin_password}@database:5432/espacigo_local?sslmode=disable" \
    golang:1.27.1-alpine3.24 /tmp/migrator-gadm03.test -test.run '^TestGADM03MigrationWorksFromEmptyAndV56WithoutChangingHistory$' -test.count=1 -test.v
  unset admin_password
  printf 'PASS GADM-03 fresh, V56, and V58 migration integration scenarios ran against disposable PostgreSQL 18\n'
  admin_password="$(<"$LOCAL_SECRETS_DIR/db_admin_password")"
  CGO_ENABLED=0 go -C "$ROOT_DIR" test -c -o "$STATE_DIR/adminglobal-gadm03.test" ./internal/adminglobal
  mkdir -m 0700 "$STATE_DIR/gadm03-local-testing"
  docker run --rm --network "container:${PROJECT}-database-1" \
    --volume "$ROOT_DIR:$ROOT_DIR:ro" \
    --volume "$STATE_DIR/gadm03-local-testing:$ROOT_DIR/.local/testing" \
    --volume "$(go env GOMODCACHE):/opt/gomod:ro" \
    --volume "$STATE_DIR/adminglobal-gadm03.test:/tmp/adminglobal-gadm03.test:ro" \
    --workdir "$ROOT_DIR" --env GOMODCACHE=/opt/gomod \
    --env "TEST_DATABASE_URL=postgres://espacigo_admin:${admin_password}@127.0.0.1:5432/espacigo_local?sslmode=disable" \
    golang:1.27.1-alpine3.24 /tmp/adminglobal-gadm03.test -test.run '^TestGlobalBootstrapDirectoryAndLastMasterConcurrency$' -test.count=1 -test.v
  unset admin_password
  printf 'PASS required GADM-03/PostGIS service and HTTP integration suite ran; no TEST_DATABASE_URL skips\n'
fi

"${compose[@]}" exec -T database psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 \
  --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local <<'SQL' >/dev/null
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
VALUES(gen_random_uuid(),'fe14-'||:'run_id'||'-public-owner@ejemplo.invalid',
       'fe14-'||:'run_id'||'-public-owner@ejemplo.invalid','not-a-login-account','activo');
WITH approved AS (
  INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,resuelta_en)
  SELECT gen_random_uuid(),host.id,'kyc','aprobada','fixture:'||gen_random_uuid(),
         'fe14-public-'||:'run_id'||'-'||host.id::text,admin.id,now()
  FROM public.usuario host CROSS JOIN public.usuario admin
  WHERE host.correo_normalizado IN ('fe14-'||:'run_id'||'-public-owner@ejemplo.invalid',
                                    'fe14-'||:'run_id'||'-host@ejemplo.invalid',
                                    'fe14-'||:'run_id'||'-renter@ejemplo.invalid')
    AND admin.correo_normalizado='fe14-'||:'run_id'||'-admin@ejemplo.invalid'
  RETURNING id,usuario_id
)
INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en)
SELECT usuario_id,'kyc',id,'elegible',now() FROM approved;

WITH host AS (
  SELECT id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-host@ejemplo.invalid'
), space AS (
  INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
      reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria,
      ubicacion_catalogo_version,region_codigo,comuna_codigo)
  SELECT gen_random_uuid(),host.id,'oficina','Espacio sintético público FE-14',
      'Publicación sintética recreable para comprobar el catálogo público anónimo, el detalle minimizado y sus filtros desde el navegador en el entorno FE-14 desechable.',
      32,6,'Uso de prueba recreable','hora',18000,'Dirección privada de prueba 123, Santiago','activa','America/Santiago',
      'SUBDERE_DPA_2018','13','13123'
  FROM host RETURNING id
)
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
SELECT id,1,'hora',18000 FROM space;
INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores)
SELECT id,'oficina',1,'{"wifi":true,"puestos_trabajo":6,"tipo_uso_oficina":"privada","climatizacion":["aire_acondicionado"]}'::jsonb FROM public.espacio WHERE titulo='Espacio sintético público FE-14';
INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
SELECT gen_random_uuid(),id,propietario_id,gen_random_uuid(),'synthetic-png-v1','image/png',repeat('a',64),100,'activa',now(),'fe14-gadm02-gallery'
FROM public.espacio WHERE titulo='Espacio sintético público FE-14';
WITH host AS (
  SELECT id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-host@ejemplo.invalid'
), fixtures(title, description) AS (VALUES
  ('Retiro permitido FE-14','Publicación sintética recreable y sin reservas ni obligaciones abiertas, destinada al ensayo de una operación de retiro administrativo permitida en este entorno descartable.'),
  ('Retiro con obligación FE-14','Publicación sintética recreable con una reserva activa y pagada de ensayo, que debe impedir el retiro administrativo mientras conserve esa obligación abierta.')
), inserted AS (
  INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
      reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
  SELECT gen_random_uuid(),host.id,'oficina',fixtures.title,fixtures.description,24,4,'Uso de prueba','hora',12000,
      'Dirección privada sintética','activa','America/Santiago'
  FROM host CROSS JOIN fixtures RETURNING id,titulo
)
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
SELECT id,1,'hora',12000 FROM inserted;
INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores)
SELECT id,'oficina',1,'{}'::jsonb FROM public.espacio WHERE titulo IN ('Retiro permitido FE-14','Retiro con obligación FE-14');
WITH host AS (
  SELECT id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-host@ejemplo.invalid'
), schedule_space AS (
  INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
      reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria)
  SELECT gen_random_uuid(),host.id,'oficina','Horario FE14','Publicación sintética borrador con fixture horario habilitado para validar el guard de horario semanal en FE-14.',24,4,'Uso de prueba','hora',12000,'Dirección privada sintética','borrador','America/Santiago'
  FROM host RETURNING id,propietario_id
)
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
SELECT id,1,'hora',12000 FROM schedule_space;
INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores)
SELECT id,'oficina',1,'{}'::jsonb FROM public.espacio WHERE titulo='Horario FE14';
INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada)
SELECT e.id,e.propietario_id,u.id,true FROM public.espacio e CROSS JOIN public.usuario u
WHERE e.titulo='Horario FE14' AND u.correo_normalizado='fe14-'||:'run_id'||'-renter@ejemplo.invalid';
WITH host AS (
  SELECT id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-public-owner@ejemplo.invalid'
), space AS (
  INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
      reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria,
      ubicacion_catalogo_version,region_codigo,comuna_codigo)
  SELECT gen_random_uuid(),host.id,'oficina','Segundo espacio público FE-14',
      'Segunda publicación sintética recreable para validar cursores y navegación de páginas en el catálogo público desde la interfaz anónima.',
      24,4,'Uso de prueba recreable','hora',35000,'Dirección privada de prueba 456, Santiago','activa','America/Santiago',
      'SUBDERE_DPA_2018','13','13123'
  FROM host RETURNING id
)
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
SELECT id,1,'hora',35000 FROM space;
WITH host AS (
  SELECT id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-public-owner@ejemplo.invalid'
), fixtures(categoria_codigo, valores) AS (VALUES
  ('oficina','{"wifi":true,"mobiliario_incluido":true,"puestos_trabajo":6,"tipo_uso_oficina":"privada","escritorios":4,"climatizacion":["aire_acondicionado"]}'::jsonb),
  ('sala_multiproposito','{"wifi":true,"proyector":true,"disposiciones_admitidas":["auditorio","aula"],"sillas_disponibles":40,"mesas_disponibles":8,"pantalla":true,"videoconferencia":true}'::jsonb),
  ('bodega','{"carro_carga_disponible":true,"muelle_carga":false,"altura_util_m":3.5,"volumen_util_m3":120.25,"acceso_carga":"vehiculo_liviano","ancho_acceso_m":2.5,"altura_acceso_m":3.0,"tipo_almacenamiento":"seco"}'::jsonb),
  ('estacionamiento','{"techado":true,"carga_electrica":false,"largo_plaza_m":5.0,"ancho_plaza_m":2.5,"vehiculos_admitidos":["automovil","suv"],"altura_maxima_m":2.1,"tipo_acceso":"porton_automatico"}'::jsonb),
  ('local_flexible','{"acceso_nivel_calle":true,"vitrina":true,"frente_m":4.5,"probadores":2,"bodega_apoyo_m2":6.25,"climatizacion":["aire_acondicionado"]}'::jsonb),
  ('stand','{"montaje_incluido":true,"wifi":true,"ancho_stand_m":3.0,"fondo_stand_m":2.0,"lados_abiertos":2,"altura_maxima_m":2.5,"potencia_disponible_w":1500,"mesas_incluidas":1,"sillas_incluidas":2}'::jsonb),
  ('quincho','{"techado":true,"agua_potable":true,"tipo_parrilla":"carbon","asientos_disponibles":12,"parrillas_disponibles":1,"mesas_disponibles":2,"lavaplatos":true,"refrigerador":true}'::jsonb),
  ('parcela_eventos','{"salon_cubierto":true,"quincho_disponible":true,"superficie_exterior_util_m2":100.0,"superficie_cubierta_util_m2":45.5,"estacionamientos_disponibles":12,"cocina_disponible":true,"piscina_disponible":false,"acceso_bus":true}'::jsonb)
), inserted AS (
  INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
      reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria,
      ubicacion_catalogo_version,region_codigo,comuna_codigo)
  SELECT gen_random_uuid(),host.id,fixtures.categoria_codigo,
      'MPV2-12 '||fixtures.categoria_codigo||' fixture '||series.n,
      'Publicación sintética comercial para filtros tipados, recreable en el Compose desechable de FE-14. Solo datos ficticios para verificar la búsqueda pública.',
      30,20,'Uso sintético','hora',12000,'Dirección privada sintética','activa','America/Santiago',
      'SUBDERE_DPA_2018','13','13123'
  FROM host CROSS JOIN fixtures CROSS JOIN generate_series(1,7) AS series(n)
  RETURNING id,categoria_codigo,titulo
)
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
SELECT id,1,CASE right(titulo,1) WHEN '3' THEN 'dia' WHEN '4' THEN 'mes' ELSE 'hora' END,
       CASE right(titulo,1) WHEN '3' THEN 54000 WHEN '4' THEN 450000 ELSE 12000 END FROM inserted;
WITH host AS (
  SELECT id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-public-owner@ejemplo.invalid'
), fixtures(categoria_codigo, valores) AS (VALUES
  ('oficina','{"wifi":true,"mobiliario_incluido":true,"puestos_trabajo":6,"tipo_uso_oficina":"privada","escritorios":4,"climatizacion":["aire_acondicionado"]}'::jsonb),
  ('sala_multiproposito','{"wifi":true,"proyector":true,"disposiciones_admitidas":["auditorio","aula"],"sillas_disponibles":40,"mesas_disponibles":8,"pantalla":true,"videoconferencia":true}'::jsonb),
  ('bodega','{"carro_carga_disponible":true,"muelle_carga":false,"altura_util_m":3.5,"volumen_util_m3":120.25,"acceso_carga":"vehiculo_liviano","ancho_acceso_m":2.5,"altura_acceso_m":3.0,"tipo_almacenamiento":"seco"}'::jsonb),
  ('estacionamiento','{"techado":true,"carga_electrica":false,"largo_plaza_m":5.0,"ancho_plaza_m":2.5,"vehiculos_admitidos":["automovil","suv"],"altura_maxima_m":2.1,"tipo_acceso":"porton_automatico"}'::jsonb),
  ('local_flexible','{"acceso_nivel_calle":true,"vitrina":true,"frente_m":4.5,"probadores":2,"bodega_apoyo_m2":6.25,"climatizacion":["aire_acondicionado"]}'::jsonb),
  ('stand','{"montaje_incluido":true,"wifi":true,"ancho_stand_m":3.0,"fondo_stand_m":2.0,"lados_abiertos":2,"altura_maxima_m":2.5,"potencia_disponible_w":1500,"mesas_incluidas":1,"sillas_incluidas":2}'::jsonb),
  ('quincho','{"techado":true,"agua_potable":true,"tipo_parrilla":"carbon","asientos_disponibles":12,"parrillas_disponibles":1,"mesas_disponibles":2,"lavaplatos":true,"refrigerador":true}'::jsonb),
  ('parcela_eventos','{"salon_cubierto":true,"quincho_disponible":true,"superficie_exterior_util_m2":100.0,"superficie_cubierta_util_m2":45.5,"estacionamientos_disponibles":12,"cocina_disponible":true,"piscina_disponible":false,"acceso_bus":true}'::jsonb)
), inserted AS (
  SELECT e.id,e.categoria_codigo,
         CASE WHEN right(e.titulo,1) IN ('2','4') THEN fixtures.valores-'muelle_carga'
              WHEN right(e.titulo,1)='3' THEN jsonb_set(fixtures.valores,'{muelle_carga}','true'::jsonb)
              ELSE fixtures.valores END
  FROM public.espacio e JOIN fixtures USING(categoria_codigo)
  WHERE e.titulo LIKE 'MPV2-12 % fixture %'
)
INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores)
SELECT id,categoria_codigo,1,valores FROM inserted;
SQL
if [[ "${FE14_TEST_GREP:-}" == *GADM-02* || "${FE14_SEED_GADM04_RETIREMENT:-0}" == "1" ]]; then
  "${compose[@]}" exec -T database psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 \
    --set=run_id="$RUN_ID" --username espacigo_admin --dbname espacigo_local <<'SQL' >/dev/null
BEGIN;
SET CONSTRAINTS ALL DEFERRED;
SELECT id AS space_id,propietario_id AS host_id FROM public.espacio WHERE titulo='Retiro con obligación FE-14' \gset
SELECT id AS renter_id FROM public.usuario WHERE correo_normalizado='fe14-'||:'run_id'||'-renter@ejemplo.invalid' \gset
SELECT gen_random_uuid() AS booking_id,gen_random_uuid() AS quote_id,gen_random_uuid() AS occupancy_id \gset
SELECT now()+interval '48 hours' AS booking_start \gset
SELECT :'booking_start'::timestamptz+interval '1 hour' AS booking_end \gset
INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo)
VALUES(:'occupancy_id',:'space_id',:'booking_id',tstzrange(:'booking_start',:'booking_end','[)'),'reserva',true);
INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,categoria_codigo,perfil_version,perfil_valores_snapshot,creada_en,vence_en)
VALUES(:'quote_id',:'space_id',:'host_id',:'renter_id',1,'hora',12000,'CLP',1,12000,:'booking_start',:'booking_end','America/Santiago','Reserva sintética de FE-14','oficina',1,'{}'::jsonb,now(),now()+interval '15 minutes');
INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,condiciones_snapshot,pago_vence_en,creada_en,actualizada_en)
VALUES(:'booking_id',:'quote_id',:'space_id',:'host_id',:'renter_id','fe14-gadm02-active-obligation',decode(repeat('00',32),'hex'),:'occupancy_id','pagada',12000,1,12000,'hora','CLP',:'booking_start',:'booking_end','America/Santiago','Reserva sintética de FE-14',now()+interval '15 minutes',now(),now());
INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada)
VALUES(:'space_id',:'host_id',:'renter_id',true);
COMMIT;
SQL
fi

if [[ "${FE14_SEED_GADM04_RETIREMENT:-0}" == "1" ]]; then
  RETIREMENT_FIXTURE_FAILED="$(${compose[@]} exec -T database psql --no-psqlrc --quiet --tuples-only --username espacigo_admin --dbname espacigo_local --command "WITH fixtures AS (
    SELECT
      EXISTS(SELECT 1 FROM public.espacio e JOIN public.usuario owner ON owner.id=e.propietario_id
        WHERE e.titulo='Retiro permitido FE-14' AND e.estado='activa' AND owner.correo_normalizado='fe14-${RUN_ID}-host@ejemplo.invalid'
        AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local r WHERE r.espacio_id=e.id)
        AND NOT EXISTS(SELECT 1 FROM public.espacio_eliminacion_historial_local h WHERE h.espacio_id=e.id)) AS allowed_case,
      EXISTS(SELECT 1 FROM public.espacio e JOIN public.usuario owner ON owner.id=e.propietario_id
        JOIN public.reserva_ensayo_local r ON r.espacio_id=e.id AND r.anfitrion_id=e.propietario_id
        JOIN public.usuario renter ON renter.id=r.arrendatario_id
        JOIN public.ocupacion o ON o.id=r.ocupacion_id AND o.reserva_id=r.id AND o.espacio_id=e.id AND o.activo AND o.tipo='reserva'
        JOIN public.reserva_ensayo_local_fixture fx ON fx.espacio_id=e.id AND fx.anfitrion_id=e.propietario_id AND fx.arrendatario_id=r.arrendatario_id AND fx.habilitada
        WHERE e.titulo='Retiro con obligación FE-14' AND e.estado='activa' AND r.estado='pagada'
        AND owner.correo_normalizado='fe14-${RUN_ID}-host@ejemplo.invalid'
        AND renter.correo_normalizado='fe14-${RUN_ID}-renter@ejemplo.invalid') AS blocked_case
  ) SELECT CASE WHEN allowed_case AND blocked_case THEN '' ELSE 'retirement_fixture_invalid' END FROM fixtures")"
  [[ -z "${RETIREMENT_FIXTURE_FAILED//[[:space:]]/}" ]] || { printf 'GADM-04 retirement fixture failed its linked-obligation/allowed-case SQL assertions\n' >&2; exit 1; }
  printf 'PASS GADM-04 fixture SQL: protected active publication has linked paid reservation/participants/occupancy; allowed publication remains active with no obligations\n'
fi


if [[ "${FE14_RUN_GADM02_INTEGRATION:-0}" == "1" ]]; then
  admin_password="$(<"$LOCAL_SECRETS_DIR/db_admin_password")"
  printf 'Building Go/PostGIS GADM-02 integration binary for isolated execution\n'
  CGO_ENABLED=0 go -C "$ROOT_DIR" test -c -o "$STATE_DIR/adminglobal.test" ./internal/adminglobal
  mkdir -m 0700 "$STATE_DIR/gadm02-local-testing"
  docker run --rm --network "container:${PROJECT}-database-1" \
    --volume "$ROOT_DIR:$ROOT_DIR:ro" \
    --volume "$STATE_DIR/gadm02-local-testing:$ROOT_DIR/.local/testing" \
    --volume "$(go env GOMODCACHE):/opt/gomod:ro" \
    --volume "$STATE_DIR/adminglobal.test:/tmp/adminglobal.test:ro" \
    --workdir "$ROOT_DIR" \
    --env GOMODCACHE=/opt/gomod \
    --env "TEST_DATABASE_URL=postgres://espacigo_admin:${admin_password}@127.0.0.1:5432/espacigo_local?sslmode=disable" \
    golang:1.27.1-alpine3.24 /tmp/adminglobal.test -test.run '^TestGlobalBootstrapDirectoryAndLastMasterConcurrency$' -test.count=1 -test.v
  unset admin_password
  printf 'PASS GADM-02/PostGIS integration tests ran in the disposable FE-14 data network\n'
fi

if [[ "${FE14_RUN_PUBLIC_CATALOG_ORDERING_INTEGRATION:-0}" == "1" ]]; then
  admin_password="$(<"$LOCAL_SECRETS_DIR/db_admin_password")"
  printf 'Building PostgreSQL public catalog ordering integration binary\n'
  CGO_ENABLED=0 go -C "$ROOT_DIR" test -c -o "$STATE_DIR/publiccatalog-ordering.test" ./internal/adapters/postgres/booking
  printf 'Running PostgreSQL integration inside the disposable data network\n'
  docker run --rm --network "${PROJECT}_data" \
    --volume "$ROOT_DIR:/workspace:ro" \
    --volume "$STATE_DIR/publiccatalog-ordering.test:/tmp/publiccatalog-ordering.test:ro" \
    --workdir /workspace/internal/adapters/postgres/booking \
    --env "TEST_DATABASE_URL=postgres://espacigo_admin:${admin_password}@database:5432/espacigo_local?sslmode=disable" \
    golang:1.27.1-alpine3.24 /tmp/publiccatalog-ordering.test -test.run '^TestLocalPublicCatalogQueryPlanAndSitemapPaging$' -test.count=1 -test.v
  unset admin_password
fi
printf 'PASS disposable FE-14 database contains more than 48 eligible synthetic public listings with varied categories, rates, and attributes\n'

if [[ "$LOCAL_CLOSE02_CLOCK" == "1" ]]; then
  FE14_CLOCK_FIXTURE_SPACE_ID="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only \
    --username "$LOCAL_DATABASE_ADMIN_USER" --dbname "$LOCAL_DATABASE_NAME" \
    --command "SELECT id::text FROM public.espacio WHERE titulo='Espacio sintético público FE-14' AND estado='activa' AND propietario_id=(SELECT id FROM public.usuario WHERE correo_normalizado='fe14-${RUN_ID}-host@ejemplo.invalid')")"
  FE14_CLOCK_FIXTURE_SPACE_ID="${FE14_CLOCK_FIXTURE_SPACE_ID//[[:space:]]/}"
  [[ "$FE14_CLOCK_FIXTURE_SPACE_ID" =~ ^[0-9a-f-]{36}$ ]] || { printf 'LOCAL-CLOSE-02 host-owned public fixture verification failed\n' >&2; exit 1; }
  export FE14_CLOCK_FIXTURE_SPACE_ID
  printf 'PASS disposable database exposes an eligible public lifecycle fixture owned by the authenticated host\n'
fi

bash "$ROOT_DIR/scripts/local-docker-stack.sh" run-e2e >"$STATE_DIR/e2e-build.log" 2>&1
if [[ -n "${FE14_TEST_GREP:-}" ]]; then
  printf 'Running focused connected FE-14 journey: %s\n' "$FE14_TEST_GREP"
  "${compose[@]}" --profile e2e run --rm angular-e2e npx playwright test --config=playwright.config.cjs --grep "$FE14_TEST_GREP" >"$STATE_DIR/e2e.log" 2>&1 || { print_sanitized_e2e_log >&2; exit 1; }
  cat "$STATE_DIR/e2e.log"
elif [[ "${FE14_PUBLIC_ONLY:-0}" == "1" ]]; then
  printf 'Running focused anonymous public catalog journey\n'
  if [[ "$LOCAL_CLOSE02_CLOCK" == "1" ]]; then
    PUBLIC_TEST_GREP="${FE14_PUBLIC_TEST_GREP:-LOCAL-CLOSE-02 serves a signed reservation through its financial close}"
    "${compose[@]}" --profile e2e run --rm angular-e2e npx playwright test --config=playwright.config.cjs --grep "$PUBLIC_TEST_GREP" >"$STATE_DIR/e2e.log" 2>&1 &
    E2E_PID=$!
    wait_for_clock_file() {
      local target="$1" deadline=$((SECONDS + 900))
      while (( SECONDS < deadline )); do
        [[ -f "$target" ]] && return 0
        kill -0 "$E2E_PID" >/dev/null 2>&1 || { print_sanitized_e2e_log >&2; return 1; }
        sleep .2
      done
      printf 'Timed out coordinating the LOCAL-CLOSE-02 business clock\n' >&2
      print_sanitized_e2e_log >&2
      return 1
    }
    wait_for_clock_file "$STATE_DIR/artifacts/local-close-02-clock-started"
    clock_deadline=$((SECONDS + 900))
    while kill -0 "$E2E_PID" >/dev/null 2>&1 && (( SECONDS < clock_deadline )); do
      for request_file in "$STATE_DIR"/artifacts/clock-request-*.json; do
        [[ -f "$request_file" ]] || continue
        phase="$(basename "$request_file" .json | sed 's/^clock-request-//')"
        [[ "$phase" =~ ^[a-z0-9-]{1,32}$ ]] || { printf 'Invalid LOCAL-CLOSE-02 clock phase\n' >&2; exit 1; }
        done_file="$STATE_DIR/artifacts/clock-done-${phase}"
        [[ -e "$done_file" ]] && continue
        clock_value="$(python3 - "$request_file" <<'PY'
import json, pathlib, re, sys
payload=json.loads(pathlib.Path(sys.argv[1]).read_text(encoding='ascii'))
phase=payload.get('phase','')
value=payload.get('now','')
if not re.fullmatch(r'[a-z0-9-]{1,32}',phase) or not re.fullmatch(r'\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d+)?(?:Z|[+-]\d\d:\d\d)',value):
    raise SystemExit('invalid LOCAL-CLOSE-02 clock payload')
if pathlib.Path(sys.argv[1]).stem != 'clock-request-'+phase:
    raise SystemExit('clock phase does not match its request filename')
print(value)
PY
)"
        clock_value="$(LOCAL_CLOSE02_CLOCK_KIND="${LOCAL_CLOSE02_CLOCK_KIND:-run}" python3 "$ROOT_DIR/scripts/local-close-02-business-clock" "$RUN_ID" set "$clock_value" | python3 -c 'import json,sys; print(json.loads(sys.stdin.read())["now"])')"
        python3 - "$STATE_DIR/artifacts/clock-result-${phase}.json" "$clock_value" <<'PY'
import json, pathlib, sys
pathlib.Path(sys.argv[1]).write_text(json.dumps({'now':sys.argv[2]},separators=(',',':'))+'\n',encoding='ascii')
PY
        touch "$done_file"
      done
      sleep .2
    done
    (( SECONDS < clock_deadline )) || { printf 'Timed out during LOCAL-CLOSE-02 browser journey\n' >&2; exit 1; }
    set +e
    wait "$E2E_PID"
    E2E_STATUS=$?
    set -e
    E2E_PID=""
    if (( E2E_STATUS != 0 )); then print_sanitized_e2e_log >&2; exit "$E2E_STATUS"; fi
    cat "$STATE_DIR/e2e.log"
  else
    PUBLIC_TEST_GREP="${FE14_PUBLIC_TEST_GREP:-MPV2-07 serves public catalog|MPV2-10 orders real Go results|MPV2-13 serves real 48-item keyset pages|MPV2-13 renders a fixed four-column desktop catalog and a one-column mobile list|MPV2-04 host declares|Removing applied filter chips|Mobile filter dialog|Contextual registration and verification|Contextual login returns}"
    "${compose[@]}" --profile e2e run --rm angular-e2e npx playwright test --config=playwright.config.cjs --grep "$PUBLIC_TEST_GREP" >"$STATE_DIR/e2e.log" 2>&1 || { print_sanitized_e2e_log >&2; exit 1; }
    cat "$STATE_DIR/e2e.log"
  fi
  "${compose[@]}" stop backend >/dev/null
  outage_status="$(curl --silent --show-error --max-time 10 --output "$STATE_DIR/ssr-outage.html" --write-out '%{http_code}' "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/explorar")"
  [[ "$outage_status" == "503" ]] || { printf 'Expected public SSR to return 503 during Go outage; received %s\n' "$outage_status" >&2; exit 1; }
  sitemap_outage_status="$(curl --silent --show-error --max-time 10 --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/sitemap.xml")"
  [[ "$sitemap_outage_status" == "503" ]] || { printf 'Expected sitemap to return 503 during Go outage; received %s\n' "$sitemap_outage_status" >&2; exit 1; }
  "${compose[@]}" start backend >/dev/null
  curl --silent --show-error --fail --retry 30 --retry-connrefused --retry-delay 1 --max-time 45 \
    "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null
  restored_status="$(curl --silent --show-error --max-time 10 --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/explorar")"
  [[ "$restored_status" == "200" ]] || { printf 'Expected public SSR to recover with Go; received %s\n' "$restored_status" >&2; exit 1; }
  printf 'PASS public SSR and sitemap return 503 during Go outage and recover after Go restoration\n'
else
printf 'Waiting for same-tab outage and restore test\n'
"${compose[@]}" --profile e2e run --rm angular-e2e >"$STATE_DIR/e2e.log" 2>&1 &
E2E_PID=$!
wait_for_file() {
  local target="$1" deadline=$((SECONDS + 180))
  while (( SECONDS < deadline )); do
    [[ -f "$target" ]] && return 0
    kill -0 "$E2E_PID" >/dev/null 2>&1 || { print_sanitized_e2e_log >&2; return 1; }
    sleep .25
  done
  printf 'Timed out waiting for disposable E2E coordination\n' >&2
  print_sanitized_e2e_log >&2
  return 1
}
wait_for_file "$STATE_DIR/artifacts/outage-session-ready"
bash "$ROOT_DIR/scripts/local-docker-stack.sh" stop-backend >/dev/null 2>&1
if curl --silent --fail --max-time 2 "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null; then
  printf 'Go readiness remained available after stopping the disposable backend\n' >&2
  exit 1
fi
touch "$STATE_DIR/artifacts/outage-stopped"
wait_for_file "$STATE_DIR/artifacts/outage-verified"
bash "$ROOT_DIR/scripts/local-docker-stack.sh" up-frontend >/dev/null 2>&1
curl --silent --show-error --fail --retry 30 --retry-connrefused --retry-delay 1 --max-time 45 \
  "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null
touch "$STATE_DIR/artifacts/outage-restored"
set +e
wait "$E2E_PID"
E2E_STATUS=$?
set -e
E2E_PID=""
if (( E2E_STATUS != 0 )); then
  print_sanitized_e2e_log >&2
  exit "$E2E_STATUS"
fi
cat "$STATE_DIR/e2e.log"
fi

if [[ "${FE14_TEST_GREP:-}" == *GADM-02* ]]; then
  GADM_FAILED="$(${compose[@]} exec -T database psql --no-psqlrc --quiet --tuples-only --username espacigo_admin --dbname espacigo_local --command "WITH checks(name,passed) AS (VALUES
    ('account-profile',EXISTS(SELECT 1 FROM public.perfil_usuario p JOIN public.usuario u ON u.id=p.usuario_id WHERE u.correo_normalizado='fe14-${RUN_ID}-renter@ejemplo.invalid' AND p.nombre_visible='Arrendatario Actualizado' AND p.telefono_normalizado='987654321')),
    ('marketplace-role',EXISTS(SELECT 1 FROM public.rol_usuario r JOIN public.usuario u ON u.id=r.usuario_id WHERE u.correo_normalizado='fe14-${RUN_ID}-renter@ejemplo.invalid' AND r.rol='arrendador')),
    ('account-unblocked',NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b JOIN public.usuario u ON u.id=b.cuenta_id WHERE u.correo_normalizado='fe14-${RUN_ID}-renter@ejemplo.invalid')),
    ('publication-attributes',EXISTS(SELECT 1 FROM public.espacio e JOIN public.espacio_caracteristicas c ON c.espacio_id=e.id WHERE e.titulo='Espacio sintético público FE-14 editado' AND c.valores->>'wifi'='false')),
    ('gallery-visibility',EXISTS(SELECT 1 FROM public.espacio e JOIN public.espacio_galeria_sintetica_local g ON g.espacio_id=e.id WHERE e.titulo='Espacio sintético público FE-14 editado' AND g.estado='activa' AND g.publicada_en_catalogo)),
    ('schedule',EXISTS(SELECT 1 FROM public.espacio e JOIN public.espacio_horario_semanal h ON h.espacio_id=e.id WHERE e.titulo='Horario FE14' AND h.activo AND EXISTS(SELECT 1 FROM public.espacio_horario_semanal_tramo t WHERE t.espacio_id=e.id AND t.dia_iso=1 AND t.apertura_minuto=540 AND t.cierre_minuto=720))),
    ('retire-allowed',EXISTS(SELECT 1 FROM public.espacio e JOIN public.espacio_eliminacion_historial_local h ON h.espacio_id=e.id JOIN public.usuario u ON u.id=h.actor_id WHERE e.titulo='Publicación retirada' AND e.estado='eliminada' AND u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid')),
    ('retire-blocked',EXISTS(SELECT 1 FROM public.espacio e JOIN public.reserva_ensayo_local r ON r.espacio_id=e.id WHERE e.titulo='Retiro con obligación FE-14' AND e.estado='activa' AND r.estado='pagada')),
    ('audit-session-revocation',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.accounts.sessions.revoke')),
    ('audit-publication-update',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.publications.update')),
    ('audit-gallery-visibility',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.publications.gallery.visibility')),
    ('audit-schedule',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.publications.schedule')),
    ('audit-retirement',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.publications.retire'))
  ) SELECT COALESCE(string_agg(name,','),'') FROM checks WHERE NOT passed")"
  [[ -z "${GADM_FAILED//[[:space:]]/}" ]] || { printf 'GADM-02 database persistence/audit assertions failed: %s\n' "${GADM_FAILED//[[:space:]]/}" >&2; exit 1; }
  printf 'PASS GADM-02 database state and administrative audit assertions match the served browser mutations\n'
fi

if [[ "${FE14_TEST_GREP:-}" == *GADM-03* ]]; then
  GADM03_FAILED="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only --username espacigo_admin --dbname espacigo_local --command "WITH checks(name,passed) AS (VALUES
    ('kpi-read-audited',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.kpi.read' AND a.resultado='exito')),
    ('account-read-target-audited',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario actor ON actor.id=a.actor_id JOIN public.usuario target ON target.id=a.cuenta_objetivo_id WHERE actor.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND target.correo_normalizado='fe14-${RUN_ID}-renter@ejemplo.invalid' AND a.accion='admin.global.accounts.read' AND a.resultado='exito')),
    ('audit-read-audited',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.audit.read' AND a.resultado='exito')),
    ('audit-export-audited',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.accion='admin.global.audit.export' AND a.resultado='exito'))
    ,('missing-account-404-audited',EXISTS(SELECT 1 FROM public.evento_auditoria_local a JOIN public.usuario u ON u.id=a.actor_id WHERE u.correo_normalizado='fe14-${RUN_ID}-globaladmin@ejemplo.invalid' AND a.recurso_id='00000000-0000-4000-8000-000000000099' AND a.accion='admin.global.accounts.read' AND a.resultado='rechazo' AND a.motivo_codigo='cuenta_no_encontrada' AND a.cuenta_objetivo_id IS NULL))
  ) SELECT COALESCE(string_agg(name,','),'') FROM checks WHERE NOT passed")"
  [[ -z "${GADM03_FAILED//[[:space:]]/}" ]] || { printf 'GADM-03 KPI/audit persistence assertions failed: %s\n' "${GADM03_FAILED//[[:space:]]/}" >&2; exit 1; }
  printf 'PASS GADM-03 KPI, actor/target scope, read and export audits match real API/browser requests\n'
fi


python3 - "$STATE_DIR/accounts.json" "${compose[*]}" <<'PY'
import json
import subprocess
import sys

accounts = json.load(open(sys.argv[1], encoding='utf-8'))
logs = subprocess.check_output(sys.argv[2].split() + ['logs', '--no-color', 'backend'], stderr=subprocess.DEVNULL, text=True)
for account in accounts.values():
    if account['email'] in logs or account['password'] in logs:
        raise SystemExit('synthetic credentials appeared in backend logs')
print('PASS backend logs contain none of the synthetic account emails or passwords')
PY

mkdir -p "$ROOT_DIR/planning/frontend/evidencia" "$ROOT_DIR/planning/marketplace_publico_v2/evidencia/mpv2_03"
if [[ -n "${FE14_EVIDENCE_OUTPUT_DIR:-}" ]]; then mkdir -p "$FE14_EVIDENCE_OUTPUT_DIR"; fi
for image in "$STATE_DIR"/artifacts/fe14_*.jpg; do
  [[ -f "$image" ]] || continue
  if [[ -n "$FE14_EVIDENCE_PREFIX" && "$(basename "$image")" == "$FE14_EVIDENCE_PREFIX"* ]]; then
    evidence_name="$(basename "$image")"
    if [[ "${FE14_RETAIN_STACK:-0}" == "1" ]]; then evidence_name="${evidence_name%.jpg}_${RUN_ID}.jpg"; fi
    destination="$FE14_EVIDENCE_OUTPUT_DIR/$evidence_name"
    cp -- "$image" "$destination"
    chmod 0644 "$destination"
    continue
  fi
  if [[ -n "$FE14_EVIDENCE_PREFIX" ]]; then continue; fi
  destination="$ROOT_DIR/planning/frontend/evidencia/$(basename "$image")"
  if [[ -e "$destination" ]]; then
    extension="${destination##*.}"
    destination="${destination%.*}_${RUN_ID}.${extension}"
  fi
  cp -- "$image" "$destination"
  chmod 0644 "$destination"
done
for measurements in "$STATE_DIR"/artifacts/fe14_*.json; do
  [[ -f "$measurements" ]] || continue
  if [[ -n "$FE14_EVIDENCE_PREFIX" && "$(basename "$measurements")" == "$FE14_EVIDENCE_PREFIX"* ]]; then
    evidence_name="$(basename "$measurements")"
    if [[ "${FE14_RETAIN_STACK:-0}" == "1" ]]; then evidence_name="${evidence_name%.json}_${RUN_ID}.json"; fi
    destination="$FE14_EVIDENCE_OUTPUT_DIR/$evidence_name"
    cp -- "$measurements" "$destination"
    chmod 0644 "$destination"
    continue
  fi
  if [[ -n "$FE14_EVIDENCE_PREFIX" ]]; then continue; fi
  destination="$ROOT_DIR/planning/frontend/evidencia/$(basename "$measurements")"
  if [[ -e "$destination" ]]; then
    extension="${destination##*.}"
    destination="${destination%.*}_${RUN_ID}.${extension}"
  fi
  cp -- "$measurements" "$destination"
  chmod 0644 "$destination"
done
if [[ "${FE14_TEST_GREP:-}" == *GADM-03* && -f "$STATE_DIR/artifacts/gadm03-account-query-plan.txt" ]]; then
  cp -- "$STATE_DIR/artifacts/gadm03-account-query-plan.txt" "$FE14_EVIDENCE_OUTPUT_DIR/gadm03-account-query-plan.txt"
  chmod 0644 "$FE14_EVIDENCE_OUTPUT_DIR/gadm03-account-query-plan.txt"
fi
if [[ "${FE14_RETAIN_STACK:-0}" == "1" ]]; then
  printf 'PASS review screenshots and viewport measurements copied; synthetic demo accounts and required stack secrets remain private in the retained state directory\n'
else
  printf 'PASS authenticated screenshots and viewport measurements copied; temporary accounts and private state are removed on exit\n'
fi
