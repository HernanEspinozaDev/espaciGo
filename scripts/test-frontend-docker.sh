#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
STAMP="$(date -u +%Y%m%d%H%M%S)_$$"
PROJECT="espacigo_fe15_${STAMP}"
STATE_DIR="${TMPDIR:-/tmp}/espacigo-fe15-${STAMP}"
BASE_PORT="${FE15_PORT_BASE:-18680}"
if [[ ! "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] || (( BASE_PORT < 1024 || BASE_PORT > 65000 )); then
  printf 'FE15_PORT_BASE must be an unprivileged TCP port from 1024 to 65000\n' >&2
  exit 2
fi

export LOCAL_DOCKER_PROJECT_NAME="$PROJECT"
export LOCAL_DOCKER_IMAGE_PREFIX="$PROJECT"
export LOCAL_DOCKER_STATE_DIR="$STATE_DIR"
export LOCAL_DOCKER_PGDATA_VOLUME="${PROJECT}_pgdata"
export LOCAL_DOCKER_API_PORT="$BASE_PORT"
export LOCAL_DOCKER_MAILPIT_PORT="$((BASE_PORT + 1))"
export LOCAL_DOCKER_ANGULAR_PORT="$((BASE_PORT + 2))"
umask 077
mkdir -p "$STATE_DIR"
export LOCAL_DOCKER_COMPOSE_OVERRIDE="$STATE_DIR/network-pools.yaml"
mapfile -t FE15_SUBNETS < <(python3 - <<'PY'
import ipaddress
import json
import subprocess

identifiers = subprocess.check_output(['docker', 'network', 'ls', '-q'], text=True).split()
used = []
if identifiers:
    networks = json.loads(subprocess.check_output(['docker', 'network', 'inspect', *identifiers], text=True))
    for network in networks:
        for config in (network.get('IPAM') or {}).get('Config') or []:
            if config.get('Subnet'):
                try:
                    used.append(ipaddress.ip_network(config['Subnet'], strict=False))
                except ValueError:
                    pass
for second_octet in range(240, 255):
    candidates = [ipaddress.ip_network(f'10.{second_octet}.{third_octet}.0/24') for third_octet in range(3)]
    if all(not candidate.overlaps(existing) for candidate in candidates for existing in used):
        print(*candidates, sep='\n')
        break
else:
    raise SystemExit('No free FE-15 subnet triplet found in 10.240-254.0.0/24')
PY
)
if (( ${#FE15_SUBNETS[@]} != 3 )); then
  printf 'Could not select three free Docker subnets\n' >&2
  exit 1
fi
cat > "$LOCAL_DOCKER_COMPOSE_OVERRIDE" <<EOF
networks:
  api:
    ipam:
      config:
        - subnet: ${FE15_SUBNETS[0]}
  mail:
    ipam:
      config:
        - subnet: ${FE15_SUBNETS[1]}
  data:
    ipam:
      config:
        - subnet: ${FE15_SUBNETS[2]}
EOF

cleanup() {
  local result=$?
  if [[ "${FE15_KEEP_STACK:-0}" == "1" ]]; then
    printf 'Keeping isolated FE-15 stack %s and private state %s\n' "$LOCAL_DOCKER_PROJECT_NAME" "$LOCAL_DOCKER_STATE_DIR"
    return "$result"
  fi
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" down-frontend >/dev/null 2>&1 || true
  local -a containers=() networks=()
  mapfile -t containers < <(docker ps --all --quiet --filter "label=com.docker.compose.project=${PROJECT}")
  if (( ${#containers[@]} > 0 )); then docker rm --force "${containers[@]}" >/dev/null 2>&1 || true; fi
  mapfile -t networks < <(docker network ls --quiet --filter "label=com.docker.compose.project=${PROJECT}")
  if (( ${#networks[@]} > 0 )); then docker network rm "${networks[@]}" >/dev/null 2>&1 || true; fi
  docker volume rm "$LOCAL_DOCKER_PGDATA_VOLUME" >/dev/null 2>&1 || true
  rm -rf -- "$LOCAL_DOCKER_STATE_DIR"
  return "$result"
}
trap cleanup EXIT

printf 'FE-15 disposable project: %s\n' "$LOCAL_DOCKER_PROJECT_NAME"
printf 'Fresh database volume: %s\n' "$LOCAL_DOCKER_PGDATA_VOLUME"
printf 'Published API/Angular ports: %s/%s\n' "$LOCAL_DOCKER_API_PORT" "$LOCAL_DOCKER_ANGULAR_PORT"
printf 'Isolated API/mail/data subnets: %s, %s, %s\n' "${FE15_SUBNETS[0]}" "${FE15_SUBNETS[1]}" "${FE15_SUBNETS[2]}"

bash "$ROOT_DIR/scripts/local-docker-stack.sh" up-frontend
ANGULAR_ID="$(docker ps --quiet --filter "label=com.docker.compose.project=${PROJECT}" --filter label=com.docker.compose.service=angular-frontend)"
if [[ -z "$ANGULAR_ID" ]]; then
  printf 'Angular service container was not started\n' >&2
  exit 1
fi
ANGULAR_ENV="$(docker inspect --format '{{range .Config.Env}}{{println .}}{{end}}' "$ANGULAR_ID")"
if grep -Eq '^(DATABASE_|.*PASSWORD=|.*SECRET=)' <<<"$ANGULAR_ENV"; then
  printf 'Angular runtime unexpectedly received a database credential\n' >&2
  exit 1
fi
ANGULAR_NETWORKS="$(docker inspect --format '{{range $name, $network := .NetworkSettings.Networks}}{{printf "%s" $name}}{{end}}' "$ANGULAR_ID")"
if [[ "$ANGULAR_NETWORKS" != "${PROJECT}_api" ]]; then
  printf 'Angular networks were [%s], expected only %s_api\n' "$ANGULAR_NETWORKS" "$PROJECT" >&2
  exit 1
fi
compose_config="$(COMPOSE_PROFILES=verify bash "$ROOT_DIR/scripts/local-docker-stack.sh" config-frontend)"
python3 -c 'import json,sys; services=json.load(sys.stdin)["services"]; assert "angular-frontend" in services; assert "mock-frontend" not in services; assert "angular-runner" in services' <<<"$compose_config"
printf 'PASS Angular is the default service, has no DB credentials, and uses only the API network\n'
curl --silent --show-error --fail --max-time 5 \
  "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/" >/dev/null
printf 'PASS Angular SSR shell is reachable on the published host URL\n'
bash "$ROOT_DIR/scripts/local-docker-stack.sh" verify-frontend

printf 'Stopping only the isolated backend to verify outage behavior\n'
bash "$ROOT_DIR/scripts/local-docker-stack.sh" stop-backend >/dev/null
if curl --silent --show-error --fail --max-time 5 \
  "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null; then
  printf 'Go readiness remained available after stopping the disposable backend\n' >&2
  exit 1
fi
curl --silent --show-error --fail --max-time 5 \
  "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/healthz" >/dev/null
curl --silent --show-error --fail --max-time 5 \
  "http://127.0.0.1:${LOCAL_DOCKER_ANGULAR_PORT}/" >/dev/null
printf 'PASS Angular process stays healthy while Go is stopped; Go readiness is unavailable\n'

bash "$ROOT_DIR/scripts/local-docker-stack.sh" up-frontend
bash "$ROOT_DIR/scripts/local-docker-stack.sh" verify-frontend
curl --silent --show-error --fail --max-time 5 \
  "http://127.0.0.1:${LOCAL_DOCKER_API_PORT}/health/ready" >/dev/null
printf 'PASS Go readiness restored after restarting the isolated stack\n'
