#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="$(python3 -c 'import secrets; print(secrets.token_hex(6))')"
PROJECT="espacigo_fe14_${RUN_ID}"
STATE_DIR="${TMPDIR:-/tmp}/espacigo-fe14-${RUN_ID}"
BASE_PORT="${FE14_PORT_BASE:-18780}"
if [[ ! "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] || (( BASE_PORT < 1024 || BASE_PORT > 65000 )); then
  printf 'FE14_PORT_BASE must be an unprivileged TCP port from 1024 to 65000\n' >&2
  exit 2
fi

export LOCAL_DOCKER_PROJECT_NAME="$PROJECT"
export LOCAL_DOCKER_IMAGE_PREFIX="$PROJECT"
export LOCAL_DOCKER_STATE_DIR="$STATE_DIR"
export LOCAL_DOCKER_PGDATA_VOLUME="${PROJECT}_pgdata"
export LOCAL_DOCKER_API_PORT="$BASE_PORT"
export LOCAL_DOCKER_MOCK_PORT="$((BASE_PORT + 1))"
export LOCAL_DOCKER_MAILPIT_PORT="$((BASE_PORT + 2))"
export LOCAL_DOCKER_ANGULAR_PORT="$((BASE_PORT + 3))"
export LOCAL_COMPOSE_PROJECT_NAME="$PROJECT"
export LOCAL_IMAGE_PREFIX="$PROJECT"
export LOCAL_API_PORT="$LOCAL_DOCKER_API_PORT"
export LOCAL_MOCK_PORT="$LOCAL_DOCKER_MOCK_PORT"
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
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
export LOCAL_SECRETS_DIR="$STATE_DIR/secrets"
export LOCAL_M03_EVIDENCE_DIR="$STATE_DIR/private-evidence"
export LOCAL_PRIVACY_REPLAY_DIR="$STATE_DIR/privacy-replay"
export LOCAL_DATABASE_TLS_DIR="$STATE_DIR/database-tls"
umask 077
mkdir -p "$STATE_DIR/artifacts"
chmod 0700 "$STATE_DIR" "$STATE_DIR/artifacts"
export LOCAL_DOCKER_COMPOSE_OVERRIDE="$STATE_DIR/network-pools.yaml"

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

compose=(docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" -f "$ROOT_DIR/compose.frontend.yaml" -f "$ROOT_DIR/compose.frontend-e2e.yaml" -f "$LOCAL_DOCKER_COMPOSE_OVERRIDE" -p "$PROJECT")
E2E_PID=""
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
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" down-frontend >/dev/null 2>&1 || true
  local -a ids=()
  mapfile -t ids < <(docker ps --all --quiet --filter "label=com.docker.compose.project=${PROJECT}")
  if (( ${#ids[@]} > 0 )); then docker rm --force "${ids[@]}" >/dev/null 2>&1 || true; fi
  mapfile -t ids < <(docker network ls --quiet --filter "label=com.docker.compose.project=${PROJECT}")
  if (( ${#ids[@]} > 0 )); then docker network rm "${ids[@]}" >/dev/null 2>&1 || true; fi
  docker volume rm "$LOCAL_DOCKER_PGDATA_VOLUME" >/dev/null 2>&1 || true
  if docker image inspect "${LOCAL_IMAGE_PREFIX}-angular-e2e:dev" >/dev/null 2>&1; then
    docker run --rm --user 0 -v "$STATE_DIR:/run/fe14" --entrypoint /bin/rm \
      "${LOCAL_IMAGE_PREFIX}-angular-e2e:dev" -rf /run/fe14/test-results >/dev/null 2>&1 || true
  fi
  rm -rf -- "$STATE_DIR"
  return "$status"
}
trap cleanup EXIT

printf 'FE-14 disposable project: %s\n' "$PROJECT"
printf 'API/Angular/Mailpit ports: %s/%s/%s\n' "$LOCAL_DOCKER_API_PORT" "$LOCAL_DOCKER_ANGULAR_PORT" "$LOCAL_DOCKER_MAILPIT_PORT"
printf 'New database volume: %s; no persistent volume is referenced\n' "$LOCAL_DOCKER_PGDATA_VOLUME"
bash "$ROOT_DIR/scripts/local-docker-stack.sh" up-frontend >"$STATE_DIR/stack-build.log" 2>&1
curl --silent --show-error --fail --max-time 5 "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null
curl --silent --show-error --fail --max-time 5 "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/healthz" >/dev/null
printf 'PASS Angular and Go readiness are HTTP 200 before authenticated E2E\n'

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
SQL
ROLE_COUNT="$("${compose[@]}" exec -T database psql --no-psqlrc --quiet --tuples-only \
  --username espacigo_admin --dbname espacigo_local \
  --command "SELECT count(*) FROM public.usuario u JOIN public.rol_usuario r ON r.usuario_id=u.id WHERE u.correo_normalizado LIKE 'fe14-${RUN_ID}-%' AND r.rol IN ('arrendatario','arrendador','administrador')")"
[[ "${ROLE_COUNT//[[:space:]]/}" == "6" ]] || { printf 'Synthetic role grant verification failed\n' >&2; exit 1; }
printf 'PASS Go verified four synthetic accounts; only this disposable database received host/admin roles\n'

bash "$ROOT_DIR/scripts/local-docker-stack.sh" run-e2e >"$STATE_DIR/e2e-build.log" 2>&1
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

mkdir -p "$ROOT_DIR/planning/frontend/evidencia"
for image in "$STATE_DIR"/artifacts/fe14_*.jpg; do
  [[ -f "$image" ]] || continue
  cp -- "$image" "$ROOT_DIR/planning/frontend/evidencia/$(basename "$image")"
  chmod 0644 "$ROOT_DIR/planning/frontend/evidencia/$(basename "$image")"
done
for measurements in "$STATE_DIR"/artifacts/fe14_*.json; do
  [[ -f "$measurements" ]] || continue
  cp -- "$measurements" "$ROOT_DIR/planning/frontend/evidencia/$(basename "$measurements")"
  chmod 0644 "$ROOT_DIR/planning/frontend/evidencia/$(basename "$measurements")"
done
printf 'PASS authenticated screenshots and viewport measurements copied; temporary accounts and private state are removed on exit\n'
