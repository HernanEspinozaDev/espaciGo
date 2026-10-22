#!/usr/bin/env bash
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-img01b-test-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-img01b-test.XXXXXX")"
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
docker exec "$NAME" psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c \
  'CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT' >/dev/null
export DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR"
mkdir "$TMP_DIR/v67"
cp db/migrations/V*.sql "$TMP_DIR/v67/"
rm "$TMP_DIR/v67/V000068__img_01b_media_schema.sql"
go run ./cmd/dbmigrate -dir "$TMP_DIR/v67"

docker exec -i "$NAME" psql -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
VALUES ('10000000-0000-4000-8000-000000000001','fixture@example.invalid','fixture@example.invalid','x','activo');
INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion)
VALUES ('20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','oficina','Fixture','Descripción sintética suficientemente extensa para cumplir el mínimo de cien caracteres en el registro.',10,2,'Reglas','dia',6000,'Dirección de prueba');
INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
VALUES
 ('30000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','60000000-0000-4000-8000-000000000001','synthetic-png-v1','image/png',repeat('a',64),1,'activa','2026-01-01Z','fixture-key-001'),
 ('30000000-0000-4000-8000-000000000002','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','60000000-0000-4000-8000-000000000002','synthetic-png-v1','image/png',repeat('b',64),1,'activa','2026-01-02Z','fixture-key-002');
SQL
go run ./cmd/dbmigrate -dir db/migrations
go run ./cmd/dbmigrate -dir db/migrations

docker exec -i "$NAME" psql -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
DO $$
DECLARE n integer;
BEGIN
  IF (SELECT max(version) FROM public.schema_migrations) <> 68 THEN RAISE EXCEPTION 'expected V68'; END IF;
  SELECT count(*) INTO n FROM public.gallery_item_order WHERE espacio_id='20000000-0000-4000-8000-000000000001';
  IF n <> 2 THEN RAISE EXCEPTION 'expected two backfilled gallery rows'; END IF;
  IF (SELECT synthetic_photo_id FROM public.gallery_item_order WHERE espacio_id='20000000-0000-4000-8000-000000000001' AND position=1) <> '30000000-0000-4000-8000-000000000001' THEN RAISE EXCEPTION 'backfill order is not deterministic'; END IF;
  IF NOT has_table_privilege('espacigo_runtime','public.media_asset','INSERT') OR has_table_privilege('espacigo_runtime','public.espacio','UPDATE') THEN RAISE EXCEPTION 'runtime grants are missing or overbroad'; END IF;
  IF (SELECT count(*) FROM public.espacio_galeria_sintetica_local WHERE estado='activa') <> 2 THEN RAISE EXCEPTION 'historical rows were changed'; END IF;
END $$;

-- Nine common slots (two active photos and seven durable reservations) leave
-- exactly one slot for a real or synthetic write competing concurrently.
INSERT INTO public.espacio_galeria_archivo_candidato_local(archivo_id,propietario_id,espacio_id,estado,creada_en)
SELECT ('40000000-0000-4000-8000-' || lpad(to_hex(i),12,'0'))::uuid,
       '10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','reservado',now()
FROM generate_series(1,7) i;
INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256)
VALUES
 ('50000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','espacio','failed','fixture-real-001',repeat('c',64)),
 ('50000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','espacio','failed','fixture-real-002',repeat('d',64));
SQL

set +e
docker exec -i "$NAME" psql -U postgres -d postgres -v ON_ERROR_STOP=1 >"$TMP_DIR/synthetic-race.log" 2>&1 <<'SQL' &
BEGIN;
INSERT INTO public.espacio_galeria_archivo_candidato_local(archivo_id,propietario_id,espacio_id,estado,creada_en)
VALUES ('40000000-0000-4000-8000-000000000008','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','reservado',now());
INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia)
VALUES ('30000000-0000-4000-8000-000000000003','20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','60000000-0000-4000-8000-000000000003','synthetic-png-v1','image/png',repeat('e',64),1,'activa',now(),'fixture-key-003');
DELETE FROM public.espacio_galeria_archivo_candidato_local WHERE archivo_id='40000000-0000-4000-8000-000000000008';
COMMIT;
SQL
synthetic_pid=$!
docker exec "$NAME" psql -U postgres -d postgres -v ON_ERROR_STOP=1 -c \
  "SET ROLE espacigo_runtime; INSERT INTO public.media_asset(id,propietario_id,espacio_id,proposito,estado,idempotency_key,input_sha256) VALUES ('50000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000001','20000000-0000-4000-8000-000000000001','espacio','pending','fixture-race-real',repeat('f',64))" \
  >"$TMP_DIR/real-race.log" 2>&1 &
real_pid=$!
wait "$synthetic_pid"; synthetic_status=$?
wait "$real_pid"; real_status=$?
set -e
if [[ "$synthetic_status" -eq "$real_status" ]]; then
  cat "$TMP_DIR/synthetic-race.log" "$TMP_DIR/real-race.log" >&2
  echo 'expected exactly one concurrent synthetic/real slot reservation to succeed' >&2
  exit 1
fi

docker exec -i "$NAME" psql -U postgres -d postgres -v ON_ERROR_STOP=1 <<'SQL'
DO $$
BEGIN
  BEGIN
    UPDATE public.media_asset SET estado='pending' WHERE id='50000000-0000-4000-8000-000000000001';
    RAISE EXCEPTION 'failed-to-pending transition bypassed the full gallery limit';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
  BEGIN
    UPDATE public.media_asset SET estado='ready' WHERE id='50000000-0000-4000-8000-000000000002';
    RAISE EXCEPTION 'failed-to-ready transition bypassed the full gallery limit';
  EXCEPTION WHEN check_violation THEN NULL;
  END;
END $$;
SQL

docker exec "$NAME" createdb -U postgres fresh_install
export DATABASE_URL="postgres://postgres@localhost/fresh_install?host=$SOCKET_DIR"
go run ./cmd/dbmigrate -dir db/migrations
go run ./cmd/dbmigrate -dir db/migrations
docker exec -i "$NAME" psql -U postgres -d fresh_install -v ON_ERROR_STOP=1 -c \
  "DO \$\$ BEGIN IF (SELECT max(version) FROM public.schema_migrations) <> 68 THEN RAISE EXCEPTION 'fresh install did not reach V68'; END IF; IF NOT has_table_privilege('espacigo_runtime','public.media_asset','INSERT') THEN RAISE EXCEPTION 'runtime grant missing on fresh install'; END IF; END \$\$"
echo 'PASS: fresh V1-V68 install, V67 to V68 upgrade, synthetic backfill, concurrent synthetic/real quota, failed-asset reactivation guards, scoped runtime ACL, and migration no-op reapply.'
