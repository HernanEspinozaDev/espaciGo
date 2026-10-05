#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! "$1" =~ ^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$ ]]; then
  printf '%s\n' 'Usage: scripts/dev-grant-kyc-admin.sh synthetic-admin@example.invalid' >&2
  exit 2
fi
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
COMPOSE=(docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml")
email="$1"
"${COMPOSE[@]}" exec -T database psql --no-psqlrc --quiet --set=ON_ERROR_STOP=1 --set=target_email="$email" --username espacigo_admin --dbname espacigo_local <<'SQL'
INSERT INTO public.rol_usuario (usuario_id, rol)
SELECT id, 'administrador'
FROM public.usuario
WHERE correo_normalizado = lower(:'target_email')
ON CONFLICT (usuario_id, rol) DO NOTHING;
SELECT CASE WHEN EXISTS (
  SELECT 1 FROM public.usuario u JOIN public.rol_usuario r ON r.usuario_id=u.id
  WHERE u.correo_normalizado=lower(:'target_email') AND r.rol='administrador'
) THEN 'admin role confirmed' ELSE 'account not found; register it first' END;
SQL
