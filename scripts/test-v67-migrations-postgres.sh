#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-v67-test-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-v67-test.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  docker rm -f "$NAME" >/dev/null 2>&1 || true
  python3 -c 'import shutil,sys; shutil.rmtree(sys.argv[1],ignore_errors=True)' "$TMP_DIR"
}
trap cleanup EXIT INT TERM

docker run -d --name "$NAME" --network none \
  --tmpfs /var/lib/postgresql:rw,size=1g \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 120); do
  ready_count="$(docker logs "$NAME" 2>&1 | grep -c 'database system is ready to accept connections' || true)"
  if [[ "$ready_count" -ge 2 ]] && docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null 2>&1; then break; fi
  sleep 0.25
done
docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null
export DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR"
mkdir "$TMP_DIR/v66"
cp db/migrations/V*.sql "$TMP_DIR/v66/"
rm "$TMP_DIR/v66/V000067__inmuebles_residential_rentals.sql"

go run ./cmd/dbmigrate -dir "$TMP_DIR/v66"
go run ./cmd/dbmigrate -dir db/migrations
go run ./cmd/dbmigrate -dir db/migrations

assert_v67() {
  local database="$1"
  docker exec -i "$NAME" psql -U postgres -d "$database" -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
  IF (SELECT max(version) FROM public.schema_migrations) <> 67 THEN RAISE EXCEPTION 'expected V67 to be applied'; END IF;
  IF NOT EXISTS (SELECT 1 FROM public.categoria_espacio WHERE codigo = 'inmuebles' AND orden = 9) THEN RAISE EXCEPTION 'inmuebles category missing'; END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.categoria_perfil_atributos p,
      jsonb_array_elements(p.perfil->'attributes') a
    WHERE p.categoria_codigo = 'inmuebles' AND p.version = 1
      AND a->>'code' = 'tipo_inmueble' AND a->>'required' = 'true'
  ) THEN RAISE EXCEPTION 'required tipo_inmueble profile attribute missing'; END IF;
  IF NOT EXISTS (
    SELECT 1 FROM public.categoria_perfil_atributos p,
      jsonb_array_elements(p.perfil->'attributes') a
    WHERE p.categoria_codigo = 'inmuebles' AND p.version = 1
      AND a->>'code' = 'tipo_de_espacio' AND a->>'required' = 'true'
  ) THEN RAISE EXCEPTION 'required tipo_de_espacio profile attribute missing'; END IF;
END
$$;
SQL
}
assert_v67 postgres
docker exec "$NAME" createdb -U postgres fresh_install
export DATABASE_URL="postgres://postgres@localhost/fresh_install?host=$SOCKET_DIR"
go run ./cmd/dbmigrate -dir db/migrations
assert_v67 fresh_install
echo 'PASS: V1-V66 upgrade to V67, fresh V1-V67 install, and migration no-op reapply.'
