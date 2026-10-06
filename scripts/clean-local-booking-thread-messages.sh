#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
reservation_id="${1:-}"
if [[ ! "$reservation_id" =~ ^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$ ]]; then
  printf 'Uso: %s <reservation-uuid>\n' "$0" >&2
  exit 2
fi
printf 'Esto elimina solo mensajes de ensayo del hilo %s en la base local persistente.\n' "$reservation_id"
read -r -p 'Confirma escribiendo borrar-hilo: ' answer
if [[ "$answer" != "borrar-hilo" ]]; then
  printf 'Sin cambios.\n'
  exit 1
fi

export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
export LOCAL_M03_EVIDENCE_DIR="${XDG_DATA_HOME:-$(getent passwd "$(id -u)" | cut -d: -f6)/.local/share}/espacigo/m03-evidence"
docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" exec -T database \
  psql -X -v ON_ERROR_STOP=1 -U espacigo_admin -d espacigo_local \
  --set=reservation_id="$reservation_id" \
  -c "WITH deleted AS (DELETE FROM public.mensaje_reserva_ensayo WHERE reserva_id=:'reservation_id'::uuid RETURNING 1) SELECT count(*) AS mensajes_eliminados FROM deleted"
