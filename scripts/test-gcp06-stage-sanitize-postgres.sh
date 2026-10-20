#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp06-sanitize.XXXXXX")"
chmod 0700 "$TMP_DIR"
trap 'rm -rf -- "$TMP_DIR"' EXIT
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_REPO_DIGEST='postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP06_SANITIZE_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo 'Unsafe run id.' >&2; exit 2; }
CONTAINER="espacigo-gcp06-sanitize-${RUN_ID,,}"
IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}' 2>/dev/null || true)"
IMAGE_REPO_DIGESTS="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{json .RepoDigests}}' 2>/dev/null || true)"
if [[ "$IMAGE_ID" != "$EXPECTED_IMAGE_ID" && "$IMAGE_REPO_DIGESTS" != *"$EXPECTED_IMAGE_REPO_DIGEST"* ]]; then
  echo "Pinned PostgreSQL 18 image is not cached or its repo digest differs (id=$IMAGE_ID; repoDigests=$IMAGE_REPO_DIGESTS)." >&2
  exit 1
fi
[[ -z "$(docker ps -aq --filter "name=^/${CONTAINER}$")" ]] || { echo 'Disposable container name already exists.' >&2; exit 1; }

mkdir "$TMP_DIR/migrations-v61" "$TMP_DIR/migrations-v64"
for version in $(seq -w 1 64); do
  matches=("$ROOT_DIR"/db/migrations/V0000"$version"*.sql)
  [[ -f "${matches[0]}" ]] || { echo 'Migration files are incomplete.' >&2; exit 1; }
  cp -- "${matches[0]}" "$TMP_DIR/migrations-v64/"
  if (( 10#$version <= 61 )); then cp -- "${matches[0]}" "$TMP_DIR/migrations-v61/"; fi
done
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -trimpath -o "$TMP_DIR/dbmigrate" ./cmd/dbmigrate

cleanup() {
  local status=$?
  if (( status != 0 )); then docker logs --tail 40 "$CONTAINER" >&2 2>/dev/null || true; fi
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker run -d --rm --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=1536m \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 240); do
  ready="$(docker exec "$CONTAINER" psql -X -At -U postgres -d postgres -c 'SELECT current_setting('"'"'listen_addresses'"'"')' 2>/dev/null || true)"
  [[ "$ready" == '*' ]] && break
  sleep 0.25
done
[[ "$ready" == '*' ]] || { echo 'Disposable PostgreSQL final server did not become ready.' >&2; exit 1; }
psql_local() { docker exec -i "$CONTAINER" psql -X -v ON_ERROR_STOP=1 "$@"; }
psql_local -U postgres -d postgres <<'SQL' >/dev/null
CREATE DATABASE espacigo_gcp06_stage_fixture;
CREATE DATABASE espacigo_gcp06_stage_v61;
SQL

psql_local -U postgres -d espacigo_gcp06_stage_fixture <<'SQL' >/dev/null
CREATE TABLE public.schema_migrations(version bigint PRIMARY KEY);
INSERT INTO public.schema_migrations SELECT generate_series(1,61);
CREATE TABLE public.usuario(
  id uuid PRIMARY KEY, correo_original text NOT NULL UNIQUE,
  correo_normalizado text NOT NULL UNIQUE, hash_clave text NOT NULL,
  estado text NOT NULL CHECK(estado IN ('correo_pendiente','activo','bloqueado','baja_solicitada','desidentificado')),
  baja_solicitada_en timestamptz, intentos_fallidos_consecutivos integer NOT NULL DEFAULT 0,
  bloqueado_hasta timestamptz, actualizado_en timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE public.perfil_usuario(
  usuario_id uuid PRIMARY KEY REFERENCES public.usuario(id) ON DELETE RESTRICT,
  nombre_visible text NOT NULL CHECK(nombre_visible ~ '^[[:alpha:] ]+$'),
  telefono_normalizado text, actualizado_en timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE public.rol_usuario(
  usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
  rol text NOT NULL CHECK(rol IN ('arrendatario','arrendador','administrador')),
  PRIMARY KEY(usuario_id,rol)
);
CREATE TABLE public.administrador_global(usuario_id uuid PRIMARY KEY REFERENCES public.usuario(id) ON DELETE RESTRICT);
CREATE TABLE public.administrador_global_bootstrap(usuario_id uuid PRIMARY KEY REFERENCES public.usuario(id) ON DELETE RESTRICT);
CREATE TABLE public.administrador_global_bootstrap_evento(id uuid PRIMARY KEY, usuario_objetivo_id uuid NOT NULL);
CREATE TABLE public.espacio(id uuid PRIMARY KEY, propietario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT);
CREATE TABLE public.reserva(id uuid PRIMARY KEY, arrendatario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT,
  anfitrion_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT);
CREATE TABLE public.reserva_detalle(id uuid PRIMARY KEY, reserva_id uuid NOT NULL REFERENCES public.reserva(id) ON DELETE RESTRICT);
CREATE TABLE public.sesion(id uuid PRIMARY KEY, usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT);
CREATE TABLE public.token_accion(id uuid PRIMARY KEY, usuario_id uuid NOT NULL REFERENCES public.usuario(id) ON DELETE RESTRICT);

INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES
 ('10000000-0000-4000-8000-000000000001','u0001@dataset.invalid','u0001@dataset.invalid','opaque-hash','activo'),
 ('10000000-0000-4000-8000-000000000002','u0002@dataset.invalid','u0002@dataset.invalid','opaque-hash','activo'),
 ('10000000-0000-4000-8000-000000000003','owner@local.invalid','owner@local.invalid','opaque-hash','activo');
INSERT INTO public.perfil_usuario(usuario_id,nombre_visible,telefono_normalizado) VALUES
 ('10000000-0000-4000-8000-000000000001','Nombre Sintetico','123456789'),
 ('10000000-0000-4000-8000-000000000002','Nombre Sintetico','123456789'),
 ('10000000-0000-4000-8000-000000000003','Nombre Sintetico','123456789');
INSERT INTO public.rol_usuario VALUES
 ('10000000-0000-4000-8000-000000000001','arrendador'),
 ('10000000-0000-4000-8000-000000000001','administrador'),
 ('10000000-0000-4000-8000-000000000002','arrendatario'),
 ('10000000-0000-4000-8000-000000000003','administrador');
INSERT INTO public.administrador_global VALUES('10000000-0000-4000-8000-000000000003');
INSERT INTO public.administrador_global_bootstrap VALUES('10000000-0000-4000-8000-000000000003');
INSERT INTO public.administrador_global_bootstrap_evento VALUES('20000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000003');
INSERT INTO public.espacio VALUES
 ('30000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001'),
 ('30000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003');
INSERT INTO public.reserva VALUES
 ('40000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000001'),
 ('40000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003');
INSERT INTO public.reserva_detalle VALUES
 ('50000000-0000-4000-8000-000000000001','40000000-0000-4000-8000-000000000001'),
 ('50000000-0000-4000-8000-000000000002','40000000-0000-4000-8000-000000000002');
INSERT INTO public.sesion VALUES('60000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001');
INSERT INTO public.token_accion VALUES('70000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001');
SQL

psql_local -U postgres -d espacigo_gcp06_stage_fixture -f /dev/stdin < "$ROOT_DIR/scripts/gcp06_stage_sanitize.sql" >/dev/null
result="$(psql_local -U postgres -d espacigo_gcp06_stage_fixture -qAt -c "SELECT json_build_object('users',(SELECT count(*) FROM usuario),'spaces',(SELECT count(*) FROM espacio),'reservations',(SELECT count(*) FROM reserva),'reservation_details',(SELECT count(*) FROM reserva_detalle),'sessions',(SELECT count(*) FROM sesion),'tokens',(SELECT count(*) FROM token_accion),'admins',(SELECT count(*) FROM rol_usuario WHERE rol='administrador'),'global_admins',(SELECT count(*) FROM administrador_global),'disabled_hashes',(SELECT count(*) FROM usuario WHERE hash_clave='!gcp06-disabled!' AND estado='activo'))::text")"
python3 - "$result" <<'PY'
import json, sys
actual=json.loads(sys.argv[1])
expected={"users":2,"spaces":1,"reservations":1,"reservation_details":1,"sessions":0,"tokens":0,"admins":0,"global_admins":0,"disabled_hashes":2}
if actual != expected:
    raise SystemExit("GCP-06 staging sanitizer fixture did not satisfy expected postconditions")
print("PASS: V61 fixture tainted-row closure, admin/session/token exclusion, and credential/profile sanitization")
PY

# Exercise the exact current V1-V61 schema with the production migrator.
docker cp "$TMP_DIR/dbmigrate" "$CONTAINER:/tmp/dbmigrate" >/dev/null
docker exec "$CONTAINER" mkdir -p /tmp/migrations-v61 /tmp/migrations-v64
docker cp "$TMP_DIR/migrations-v61/." "$CONTAINER:/tmp/migrations-v61/" >/dev/null
docker cp "$TMP_DIR/migrations-v64/." "$CONTAINER:/tmp/migrations-v64/" >/dev/null
docker exec -e DATABASE_URL='postgres://postgres@localhost:5432/espacigo_gcp06_stage_v61?sslmode=disable' \
  "$CONTAINER" /tmp/dbmigrate -dir /tmp/migrations-v61 >/dev/null
psql_local -U postgres -d espacigo_gcp06_stage_v61 <<'SQL' >/dev/null
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES
 ('10000000-0000-4000-8000-000000000011','u0011@dataset.invalid','u0011@dataset.invalid','opaque-hash','activo'),
 ('10000000-0000-4000-8000-000000000012','owner@local.invalid','owner@local.invalid','opaque-hash','activo');
INSERT INTO public.perfil_usuario(usuario_id,nombre_visible) VALUES
 ('10000000-0000-4000-8000-000000000011','Nombre Sintetico'),
 ('10000000-0000-4000-8000-000000000012','Nombre Sintetico');
INSERT INTO public.rol_usuario(usuario_id,rol) VALUES
 ('10000000-0000-4000-8000-000000000011','arrendador'),
 ('10000000-0000-4000-8000-000000000011','administrador');
INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo)
VALUES('10000000-0000-4000-8000-000000000012',now(),'bootstrap_propietario');
INSERT INTO public.administrador_global_bootstrap(usuario_id,creado_en)
VALUES('10000000-0000-4000-8000-000000000012',now());
INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
 reglas_uso,modalidad_tarifa,precio_base_clp,direccion)
VALUES
 ('30000000-0000-4000-8000-000000000011','10000000-0000-4000-8000-000000000011','oficina','Sala sintética',repeat('Descripción sintética. ',5),25,4,'Uso responsable','dia',25000,'Dirección sintética 1'),
 ('30000000-0000-4000-8000-000000000012','10000000-0000-4000-8000-000000000012','oficina','Sala sintética',repeat('Descripción sintética. ',5),25,4,'Uso responsable','dia',25000,'Dirección sintética 2');
INSERT INTO public.sesion(id,usuario_id,token_hash,creada_en,ultima_actividad_en,expira_en)
VALUES('60000000-0000-4000-8000-000000000011','10000000-0000-4000-8000-000000000011',repeat('a',64),now(),now(),now()+interval '1 hour');
INSERT INTO public.token_accion(id,usuario_id,proposito,token_hash,creado_en,expira_en)
VALUES('70000000-0000-4000-8000-000000000011','10000000-0000-4000-8000-000000000011','verificar_correo',repeat('b',64),now(),now()+interval '1 hour');
SQL
psql_local -U postgres -d espacigo_gcp06_stage_v61 -f /dev/stdin < "$ROOT_DIR/scripts/gcp06_stage_sanitize.sql" >/dev/null
docker exec -e DATABASE_URL='postgres://postgres@localhost:5432/espacigo_gcp06_stage_v61?sslmode=disable' \
  "$CONTAINER" /tmp/dbmigrate -dir /tmp/migrations-v64 >/dev/null
actual_schema_result="$(psql_local -U postgres -d espacigo_gcp06_stage_v61 -qAt -c "SELECT json_build_object('users',(SELECT count(*) FROM usuario),'spaces',(SELECT count(*) FROM espacio),'sessions',(SELECT count(*) FROM sesion),'tokens',(SELECT count(*) FROM token_accion),'admin_roles',(SELECT count(*) FROM rol_usuario WHERE rol='administrador'),'global_admins',(SELECT count(*) FROM administrador_global),'disabled_hashes',(SELECT count(*) FROM usuario WHERE hash_clave='!gcp06-disabled!' AND estado='activo'),'ledger',(SELECT count(*) FROM schema_migrations),'v62_table',to_regclass('public.registro_replay_baja_local') IS NOT NULL,'v63_claim',EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema='public' AND table_name='baja_archivo_pendiente_local' AND column_name='claim_generacion'))::text")"
python3 - "$actual_schema_result" <<'PY'
import json, sys
actual=json.loads(sys.argv[1])
expected={"users":1,"spaces":1,"sessions":0,"tokens":0,"admin_roles":0,"global_admins":0,"disabled_hashes":1,"ledger":64,"v62_table":True,"v63_claim":True}
if actual != expected:
    raise SystemExit(f"GCP-06 actual-schema result differed: {actual!r}")
print("PASS: actual V1-V61 schema sanitizer plus the V62-V64 migrations on synthetic rows")
PY

# Exercise the production local exporter end to end against a disposable local
# V61 source. The source and stage both remain isolated/no-network containers.
docker exec "$CONTAINER" createdb -U postgres espacigo_local >/dev/null
docker exec -e DATABASE_URL='postgres://postgres@localhost:5432/espacigo_local?sslmode=disable' \
  "$CONTAINER" /tmp/dbmigrate -dir /tmp/migrations-v61 >/dev/null
psql_local -U postgres -d espacigo_local <<'SQL' >/dev/null
INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES
 ('10000000-0000-4000-8000-000000000021','u0021@dataset.invalid','u0021@dataset.invalid','opaque-dataset-verifier','activo'),
 ('10000000-0000-4000-8000-000000000022','owner@local.invalid','owner@local.invalid','opaque-local-verifier','activo');
INSERT INTO public.perfil_usuario(usuario_id,nombre_visible) VALUES
 ('10000000-0000-4000-8000-000000000021','Nombre Sintetico'),
 ('10000000-0000-4000-8000-000000000022','Propietario Local');
INSERT INTO public.rol_usuario(usuario_id,rol) VALUES
 ('10000000-0000-4000-8000-000000000021','arrendador'),
 ('10000000-0000-4000-8000-000000000022','administrador');
INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,
 reglas_uso,modalidad_tarifa,precio_base_clp,direccion)
VALUES('30000000-0000-4000-8000-000000000021','10000000-0000-4000-8000-000000000021','oficina',
 'Sala sintética',repeat('Descripción sintética para prueba local. ',4),25,4,'Uso responsable','dia',25000,'Dirección sintética');
UPDATE public.espacio SET estado='activa' WHERE id='30000000-0000-4000-8000-000000000021';
INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp)
VALUES('30000000-0000-4000-8000-000000000021',1,'dia',25000);
SQL
EXPORT_ID="e2e-$(date -u +%Y%m%d%H%M%S)-$$"
EXPORT_DIR=".local/testing/gcp06-stage-$EXPORT_ID"
GCP06_SOURCE_CONTAINER="$CONTAINER" GCP06_SOURCE_DATABASE_USER=postgres \
  "$ROOT_DIR/scripts/gcp06-local-stage-export.sh" "$EXPORT_DIR" >/dev/null
python3 - "$ROOT_DIR/$EXPORT_DIR/manifest.json" "$ROOT_DIR/$EXPORT_DIR/data.sql" <<'PY'
import json,re,sys
manifest_path,dump_path=sys.argv[1:]
m=json.load(open(manifest_path,encoding='utf-8'))
assert m['format']=='espacigo.gcp06.local-stage.v1'
assert m['source']['migration_count']==61 and m['staged']['migration_count']==64
assert m['staged']['users']==2 and m['staged']['active_users']==2
assert m['staged']['spaces']==1 and m['staged']['admins']==0
assert m['staged']['synthetic_kyc']==1 and m['staged']['catalog_candidates']==1
dump=open(dump_path,encoding='utf-8').read()
assert 'owner@local.invalid' not in dump and 'opaque-local-verifier' not in dump
assert 'opaque-dataset-verifier' not in dump and '@transfer.invalid' in dump
assert '!gcp06-disabled!' in dump and 'Usuario Sintetico' in dump
assert 'gcp06-synthetic-kyc-' in dump and 'fixture:' in dump
assert "current_database() !~ '^espacigo_gcp06_candidate_[a-z0-9-]{6,30}$'" in dump
assert 'CREATE TABLE public.schema_migrations' in dump
assert 'COPY public.schema_migrations' in dump
assert 'CREATE TABLE public.categoria_espacio' in dump
assert 'COPY public.categoria_espacio' in dump
assert not re.search(r'^(CREATE ROLE|ALTER ROLE|GRANT|REVOKE)\b', dump, re.M)
assert 'OWNER TO' not in dump
assert not any(line.startswith('\\') and line != '\\.' for line in dump.splitlines())
print('PASS: full portable V64 SQL includes schema, static seeds, and ledger without role DDL or ACLs; only COPY terminators remain')
PY
docker exec "$CONTAINER" createdb -U postgres espacigo_gcp06_candidate_e2e-test >/dev/null
docker exec -i "$CONTAINER" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d espacigo_gcp06_candidate_e2e-test \
  < "$ROOT_DIR/$EXPORT_DIR/data.sql" >/dev/null
catalog_count="$(psql_local -U postgres -d espacigo_gcp06_candidate_e2e-test -qAt -c \
  "SELECT count(*) FROM public.espacio e JOIN public.categoria_espacio c ON c.codigo=e.categoria_codigo AND c.activa JOIN LATERAL (SELECT 1 FROM public.tarifa_espacio t0 WHERE t0.espacio_id=e.id ORDER BY t0.version DESC LIMIT 1) t ON true JOIN public.usuario u ON u.id=e.propietario_id AND u.estado='activo' WHERE e.estado='activa' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id) AND EXISTS(SELECT 1 FROM public.elegibilidad_verificacion_local el JOIN public.verificacion v ON v.id=el.verificacion_id AND v.usuario_id=el.usuario_id AND v.tipo=el.tipo AND v.estado='aprobada' WHERE el.usuario_id=u.id AND el.tipo='kyc' AND el.estado='elegible') AND NOT EXISTS(SELECT 1 FROM public.reserva_ensayo_local_fixture fx WHERE fx.espacio_id=e.id AND fx.anfitrion_id=e.propietario_id AND fx.habilitada)")"
[[ "$catalog_count" == 1 ]] || { echo 'Candidate database did not expose the expected synthetic catalog fixture.' >&2; exit 1; }
if docker exec -i "$CONTAINER" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d espacigo_local \
  < "$ROOT_DIR/$EXPORT_DIR/data.sql" >/dev/null 2>&1; then
  echo 'Import guard accepted a non-candidate database.' >&2
  exit 1
fi
source_rows_after_rejected_import="$(psql_local -U postgres -d espacigo_local -qAt -c 'SELECT count(*) FROM public.usuario')"
[[ "$source_rows_after_rejected_import" == 2 ]] || { echo 'Rejected import modified the synthetic source fixture.' >&2; exit 1; }
ledger_count="$(psql_local -U postgres -d espacigo_gcp06_candidate_e2e-test -qAt -c 'SELECT count(*) || chr(58) || max(version) FROM public.schema_migrations')"
[[ "$ledger_count" == '64:64' ]] || { echo 'Portable SQL dump did not restore ledger V1-V64.' >&2; exit 1; }
seed_count="$(psql_local -U postgres -d espacigo_gcp06_candidate_e2e-test -qAt -c 'SELECT count(*) FROM public.categoria_espacio')"
(( seed_count > 0 )) || { echo 'Portable SQL dump did not restore category seeds.' >&2; exit 1; }
echo 'PASS: full portable V64 SQL imported into an empty isolated database; schema, ledger 64:64, static category seeds, and one catalog fixture verified; wrong-database import rejected'
rm -f -- "$ROOT_DIR/$EXPORT_DIR/data.sql" "$ROOT_DIR/$EXPORT_DIR/manifest.json"
rmdir -- "$ROOT_DIR/$EXPORT_DIR"
