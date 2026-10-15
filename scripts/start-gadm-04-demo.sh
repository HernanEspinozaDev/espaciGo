#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${1:-}"
[[ "$RUN_ID" =~ ^[0-9a-f]{12}$ ]] || { printf 'usage: start-gadm-04-demo.sh 12-hex-run-id\n' >&2; exit 2; }
PROJECT="espacigo_gadm04_demo_${RUN_ID}"
STATE_DIR="$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID"
[[ -f "$STATE_DIR/runtime.env" && -f "$STATE_DIR/demo-record.txt" && "$(realpath -m "$STATE_DIR")" == "$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID" ]] || { printf 'matching retained GADM-04 state not found\n' >&2; exit 2; }
BASE_PORT="$(awk -F= '$1=="ports" {split($2,p,"-"); print p[1]}' "$STATE_DIR/demo-record.txt")"
[[ "$BASE_PORT" =~ ^[0-9]{4,5}$ ]] && (( BASE_PORT >= 1024 && BASE_PORT <= 64997 )) || { printf 'retained GADM-04 port record is invalid\n' >&2; exit 2; }
VOLUME="${PROJECT}_pgdata"
docker volume ls --format '{{.Name}}' | grep -Fxq "$VOLUME" || { printf 'dedicated GADM-04 data volume is missing\n' >&2; exit 2; }
mapfile -t ids < <(docker ps --all --quiet --filter "label=com.docker.compose.project=$PROJECT")
(( ${#ids[@]} >= 4 )) || { printf 'dedicated GADM-04 Compose services are incomplete\n' >&2; exit 2; }
mapfile -t names < <(docker inspect --format '{{.Name}}' "${ids[@]}" | sed 's#^/##')
for name in "${names[@]}"; do
  [[ "$name" == "$PROJECT-"* ]] || { printf 'unexpected container under GADM-04 project label\n' >&2; exit 2; }
done
for service in database backend angular-frontend mailpit; do
  printf '%s\n' "${names[@]}" | grep -Fxq "$PROJECT-$service-1" || { printf 'required GADM-04 service is missing: %s\n' "$service" >&2; exit 2; }
done
docker start "${ids[@]}" >/dev/null
ready=0
for _ in $(seq 1 90); do
  if curl --silent --fail --max-time 2 "http://127.0.0.1:$((BASE_PORT + 2))/healthz" >/dev/null \
    && curl --silent --fail --max-time 2 "http://127.0.0.1:${BASE_PORT}/health/ready" >/dev/null; then
    ready=1; break
  fi
  sleep 1
done
(( ready == 1 )) || { printf 'GADM-04 services restarted but did not become ready; inspect only project %s\n' "$PROJECT" >&2; exit 1; }
printf 'GADM-04 demo ready: http://127.0.0.1:%s/ (health: http://127.0.0.1:%s/healthz); project=%s\n' "$((BASE_PORT + 2))" "$((BASE_PORT + 2))" "$PROJECT"
