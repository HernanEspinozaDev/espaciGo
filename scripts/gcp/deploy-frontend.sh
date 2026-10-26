#!/usr/bin/env bash
# deploy-frontend.sh — Safe Cloud Build trigger wrapper for EspaciGo Frontend.
#
# Target infrastructure:
#   Trigger: espacigo-main-rollout (548b8eff-e489-4307-a77f-455b56aa4e57)
#   Project: espacigo-testprod
#   Region:  southamerica-west1
#   Account: pasteleriahijitos@gmail.com
#   Repo:    HernanEspinozaDev/espaciGo (main branch)
#
# Usage:
#   ./scripts/gcp/deploy-frontend.sh              # READ-ONLY preflight and status inspection
#   ./scripts/gcp/deploy-frontend.sh --deploy     # Verify clean git & remote origin/main SHA and dispatch trigger
#   ./scripts/gcp/deploy-frontend.sh --dry-run    # Verify all gates including remote SHA without dispatching build
#   ./scripts/gcp/deploy-frontend.sh --help       # Display usage information

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v python3 >/dev/null 2>&1; then
  echo "[ERROR] python3 is required to execute the deployment wrapper." >&2
  exit 1
fi

exec python3 "${SCRIPT_DIR}/release.py" "$@"
