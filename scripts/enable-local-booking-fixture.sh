#!/usr/bin/env bash
set -euo pipefail
if [[ $# -ne 2 ]]; then
  echo "Uso: $0 <correo-anfitrion-verificado> <correo-arrendatario-verificado>" >&2
  exit 2
fi
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)"
export LOCAL_M03_EVIDENCE_DIR="${XDG_DATA_HOME:-$(getent passwd "$(id -u)" | cut -d: -f6)/.local/share}/espacigo/m03-evidence"
"$ROOT_DIR/scripts/dev-env.sh" config
docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" run --rm --no-deps \
  --entrypoint /api -e LOCAL_BOOKING_TRIAL=1 migrate local-booking-fixture "$1" "$2"
