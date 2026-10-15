#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
RUN_ID="${1:-}"
[[ "$RUN_ID" =~ ^[0-9a-f]{12}$ ]] || { printf 'usage: stop-gadm-04-demo.sh 12-hex-run-id\n' >&2; exit 2; }
PROJECT="espacigo_gadm04_demo_${RUN_ID}"
STATE_DIR="$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID"
[[ -f "$STATE_DIR/runtime.env" && -f "$STATE_DIR/demo-record.txt" && "$(realpath -m "$STATE_DIR")" == "$ROOT_DIR/.local/testing/gadm-04-demo/$RUN_ID" ]] || { printf 'matching retained GADM-04 state not found\n' >&2; exit 2; }
mapfile -t ids < <(docker ps --all --quiet --filter "label=com.docker.compose.project=$PROJECT")
if (( ${#ids[@]} )); then docker stop "${ids[@]}" >/dev/null; fi
printf 'Stopped only GADM-04 project %s; data and private credentials remain at %s\n' "$PROJECT" "$STATE_DIR"
