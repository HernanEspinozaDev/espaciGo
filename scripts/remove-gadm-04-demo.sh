#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${1:-}"
[[ "$RUN_ID" =~ ^[0-9a-f]{12}$ ]] || { printf 'usage: remove-gadm-04-demo.sh 12-hex-run-id\n' >&2; exit 2; }
PROJECT="espacigo_gadm04_demo_${RUN_ID}"
VOLUME="${PROJECT}_pgdata"
STATE_DIR="$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID"
[[ -f "$STATE_DIR/runtime.env" && -f "$STATE_DIR/demo-record.txt" && "$(realpath -m "$STATE_DIR")" == "$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID" ]] || { printf 'matching retained GADM-04 state not found\n' >&2; exit 2; }
printf 'Removing only GADM-04 project %s, exact volume %s, and private state %s\n' "$PROJECT" "$VOLUME" "$STATE_DIR"
mapfile -t ids < <(docker ps --all --quiet --filter "label=com.docker.compose.project=$PROJECT")
if (( ${#ids[@]} )); then docker rm --force "${ids[@]}" >/dev/null; fi
mapfile -t networks < <(docker network ls --quiet --filter "label=com.docker.compose.project=$PROJECT")
if (( ${#networks[@]} )); then docker network rm "${networks[@]}" >/dev/null; fi
if docker volume ls --format '{{.Name}}' | grep -Fxq "$VOLUME"; then docker volume rm "$VOLUME" >/dev/null; fi
rm -rf -- "$STATE_DIR"
printf 'Removed GADM-04 demo %s\n' "$RUN_ID"
