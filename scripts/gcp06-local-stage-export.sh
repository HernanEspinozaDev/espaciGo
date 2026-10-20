#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SOURCE_CONTAINER="${GCP06_SOURCE_CONTAINER:-espacigo-database-1}"
SOURCE_DB="${GCP06_SOURCE_DATABASE:-espacigo_local}"
SOURCE_USER="${GCP06_SOURCE_DATABASE_USER:-espacigo_admin}"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_REPO_DIGEST='postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'

if [[ $# -ne 1 ]]; then
  echo 'usage: scripts/gcp06-local-stage-export.sh .local/testing/gcp06-stage-<run-id>' >&2
  exit 2
fi
OUTPUT_DIR="$1"
case "$OUTPUT_DIR" in
  .local/testing/gcp06-stage-*) ;;
  *) echo 'output must be a run-scoped directory under .local/testing/gcp06-stage-*' >&2; exit 2 ;;
esac
[[ "$OUTPUT_DIR" =~ ^\.local/testing/gcp06-stage-[a-z0-9][a-z0-9-]{5,30}$ ]] || { echo 'unsafe output path' >&2; exit 2; }
[[ ! -e "$ROOT_DIR/$OUTPUT_DIR" ]] || { echo 'refusing to overwrite an existing private stage directory' >&2; exit 1; }
[[ "$SOURCE_CONTAINER" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]{0,63}$ && "$SOURCE_DB" == 'espacigo_local' && "$SOURCE_USER" =~ ^[A-Za-z_][A-Za-z0-9_]{0,62}$ ]] || {
  echo 'source must be the configured local database container, database espacigo_local, and a simple role name' >&2; exit 2;
}

RUN_ID="$(basename "$OUTPUT_DIR" | sed 's/^gcp06-stage-//')"
[[ "$RUN_ID" =~ ^[a-z0-9][a-z0-9-]{5,30}$ ]] || { echo 'unsafe GCP-06 run id' >&2; exit 2; }
STAGE_DB="espacigo_gcp06_stage_${RUN_ID}"
CONTAINER="espacigo-gcp06-stage-${RUN_ID}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp06-export.XXXXXX")"
chmod 0700 "$TMP_DIR"
mkdir -m 0700 -p "$ROOT_DIR/.local/testing"
mkdir -m 0700 "$ROOT_DIR/$OUTPUT_DIR"
chmod 0700 "$ROOT_DIR/$OUTPUT_DIR"

cleanup() {
  local status=$?
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
  if (( status != 0 )); then
    rm -f -- "$ROOT_DIR/$OUTPUT_DIR/data.sql" "$ROOT_DIR/$OUTPUT_DIR/data.sql.tmp" \
      "$ROOT_DIR/$OUTPUT_DIR/data.sql.dump.tmp" "$ROOT_DIR/$OUTPUT_DIR/manifest.json" \
      "$ROOT_DIR/$OUTPUT_DIR/manifest.json.tmp"
    rmdir -- "$ROOT_DIR/$OUTPUT_DIR" 2>/dev/null || true
    echo 'GCP-06 staging stopped; no source database changes were made.' >&2
  fi
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}' 2>/dev/null || true)"
IMAGE_REPO_DIGESTS="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{json .RepoDigests}}' 2>/dev/null || true)"
if [[ "$IMAGE_ID" != "$EXPECTED_IMAGE_ID" && "$IMAGE_REPO_DIGESTS" != *"$EXPECTED_IMAGE_REPO_DIGEST"* ]]; then
  echo "Pinned PostgreSQL 18 image is not cached or its repo digest differs (id=$IMAGE_ID; repoDigests=$IMAGE_REPO_DIGESTS)." >&2
  exit 1
fi
[[ -z "$(docker ps -aq --filter "name=^/${CONTAINER}$")" ]] || { echo 'staging container name already exists.' >&2; exit 1; }
docker inspect "$SOURCE_CONTAINER" >/dev/null 2>&1 || { echo 'configured local source container is unavailable.' >&2; exit 1; }

source_identity="$(docker exec "$SOURCE_CONTAINER" psql -X -qAt -U "$SOURCE_USER" -d "$SOURCE_DB" \
  -c "SET default_transaction_read_only=on; SELECT current_database() || ':' || current_setting('server_version_num')::int / 10000;" 2>/dev/null || true)"
[[ "$source_identity" == 'espacigo_local:18' ]] || { echo 'source identity/version preflight failed.' >&2; exit 1; }
source_counts="$(docker exec "$SOURCE_CONTAINER" psql -X -qAt -U "$SOURCE_USER" -d "$SOURCE_DB" \
  -c "SET default_transaction_read_only=on; SELECT json_build_object('migration_count',(SELECT count(*) FROM public.schema_migrations),'migration_max',(SELECT max(version) FROM public.schema_migrations),'users',(SELECT count(*) FROM public.usuario),'spaces',(SELECT count(*) FROM public.espacio),'bookings',(SELECT count(*) FROM public.reserva_ensayo_local))::text;" 2>/dev/null || true)"
python3 - "$source_counts" <<'PY'
import json,sys
try:
    report=json.loads(sys.argv[1])
except (ValueError,IndexError):
    raise SystemExit('source schema/count preflight failed')
if report.get('migration_count') != 61 or report.get('migration_max') != 61:
    raise SystemExit('source must have exactly V1-V61')
if any(type(report.get(key)) is not int for key in ('users','spaces','bookings')):
    raise SystemExit('source row-count preflight was incomplete')
PY

# Compare only catalog relation identities before opening the data stream. The
# query emits schema/name/kind (never rows or column values); the report keeps
# only counts and digests so unexpected source identifiers cannot escape.
relation_inventory() {
  docker exec "$1" psql -X -qAt -U "$2" -d "$3" -c \
    "BEGIN READ ONLY; SELECT n.nspname || '.' || c.relname || ':' || c.relkind::text FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','S','f') ORDER BY 1; COMMIT;" 2>/dev/null
}

for version in $(seq -w 1 64); do
  matches=("$ROOT_DIR"/db/migrations/V0000"$version"*.sql)
  [[ -f "${matches[0]}" ]] || { echo 'migration files are incomplete.' >&2; exit 1; }
  cp -- "${matches[0]}" "$TMP_DIR/"
done
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=local go build -trimpath -o "$TMP_DIR/dbmigrate" ./cmd/dbmigrate

docker run -d --rm --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=1536m \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 240); do
  ready="$(docker exec "$CONTAINER" psql -X -At -U postgres -d postgres \
    -c 'SELECT current_setting('"'"'listen_addresses'"'"')' 2>/dev/null || true)"
  [[ "$ready" == '*' ]] && break
  sleep 0.25
done
[[ "$ready" == '*' ]] || { echo 'isolated PostgreSQL final server did not become ready.' >&2; exit 1; }
docker exec "$CONTAINER" createdb -U postgres "$STAGE_DB" >/dev/null
docker cp "$TMP_DIR/dbmigrate" "$CONTAINER:/tmp/dbmigrate" >/dev/null
docker exec "$CONTAINER" mkdir -p /tmp/migrations-v61 /tmp/migrations-v64
docker cp "$TMP_DIR/V000001"* "$CONTAINER:/tmp/migrations-v61/" >/dev/null
for version in $(seq -w 2 61); do docker cp "$TMP_DIR/V0000$version"* "$CONTAINER:/tmp/migrations-v61/" >/dev/null; done
docker cp "$TMP_DIR/." "$CONTAINER:/tmp/migrations-v64/" >/dev/null

docker exec -e DATABASE_URL="postgres://postgres@localhost:5432/$STAGE_DB?sslmode=disable" \
  "$CONTAINER" /tmp/dbmigrate -dir /tmp/migrations-v61 >/dev/null

source_inventory="$(relation_inventory "$SOURCE_CONTAINER" "$SOURCE_USER" "$SOURCE_DB" || true)"
stage_inventory="$(relation_inventory "$CONTAINER" postgres "$STAGE_DB" || true)"
python3 - "$source_inventory" "$stage_inventory" <<'PY'
import hashlib,sys
source,stage=(set(x for x in arg.splitlines() if x) for arg in sys.argv[1:])
if not source or not stage:
    raise SystemExit('catalog relation inventory unavailable; no source rows were streamed')
missing=stage-source
extra=source-stage
def digest(items):
    return hashlib.sha256(('\n'.join(sorted(items))+'\n').encode()).hexdigest()
if missing or extra:
    print(f"catalog relation inventory mismatch (source_count={len(source)} stage_count={len(stage)} source_only={len(extra)} stage_only={len(missing)} source_sha256={digest(source)} stage_sha256={digest(stage)}); identifiers and row data suppressed",file=sys.stderr)
    raise SystemExit(1)
print(f"catalog relation inventory matched (relations={len(source)} sha256={digest(source)})")
PY

# The raw snapshot streams directly from the unchanged source container to the
# disposable staging container. No unredacted dump is written to host storage.
SOURCE_DUMP_STDERR="$TMP_DIR/source-dump.stderr"
STAGE_RESTORE_STDERR="$TMP_DIR/stage-restore.stderr"
classify_pg_error() {
  python3 - "$1" <<'PY'
import re,sys
try:
    lines=open(sys.argv[1],encoding='utf-8',errors='replace').read().splitlines()
except OSError:
    lines=[]
line=next((x for x in lines if re.search(r'\b(error|fatal|panic)\b',x,re.I)), '')
if not line:
    print('no_error_line')
else:
    low=line.lower()
    if 'permission denied' in low or 'must be superuser' in low: code='permission_denied'
    elif 'duplicate key' in low or 'unique constraint' in low or '23505' in low: code='unique_violation'
    elif 'foreign key' in low or '23503' in low: code='foreign_key_violation'
    elif 'check constraint' in low or '23514' in low: code='check_violation'
    elif 'does not exist' in low: code='missing_object'
    elif 'invalid input syntax' in low or 'cannot cast' in low or '22p02' in low: code='data_type_mismatch'
    elif 'connection' in low or 'server closed' in low: code='connection_failure'
    elif 'broken pipe' in low or 'unexpected end of file' in low: code='stream_interrupted'
    else: code='unclassified_error'
    print(code)
PY
}
set +e
docker exec "$SOURCE_CONTAINER" pg_dump --data-only --format=custom --no-owner --no-privileges \
  --no-comments --serializable-deferrable \
  --exclude-table-data=public.schema_migrations \
  --exclude-table-data=public.spatial_ref_sys \
  --exclude-table-data=public.atributo_filtro_publico \
  --exclude-table-data=public.categoria_espacio \
  --exclude-table-data=public.categoria_perfil_atributos \
  --exclude-table-data=public.catalogo_ubicacion_publica \
  --exclude-table-data=public.comuna_publica \
  --exclude-table-data=public.region_publica \
  --exclude-table-data=public.reserva_ensayo_local_ubicacion_sintetica \
  --exclude-table-data=public.resena_autoria_marca_local \
  --exclude-table-data=public.version_terminos \
  -U "$SOURCE_USER" -d "$SOURCE_DB" 2>"$SOURCE_DUMP_STDERR" |
  docker exec -i "$CONTAINER" pg_restore --data-only --disable-triggers --no-owner --no-privileges \
    -U postgres -d "$STAGE_DB" 2>"$STAGE_RESTORE_STDERR"
pipeline_status=("${PIPESTATUS[@]}")
set -e
if (( pipeline_status[0] != 0 || pipeline_status[1] != 0 )); then
  source_error='not_applicable'
  stage_error='not_applicable'
  if (( pipeline_status[0] != 0 )); then
    source_error="$(classify_pg_error "$SOURCE_DUMP_STDERR")"
  fi
  if (( pipeline_status[1] != 0 )); then
    stage_error="$(classify_pg_error "$STAGE_RESTORE_STDERR")"
  fi
  printf 'local logical snapshot failed (pg_dump_exit=%s pg_dump_error=%s; pg_restore_exit=%s pg_restore_error=%s); raw diagnostics were discarded with the ephemeral container.\n' \
    "${pipeline_status[0]}" "$source_error" "${pipeline_status[1]}" "$stage_error" >&2
  exit 1
fi

docker exec -i "$CONTAINER" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d "$STAGE_DB" \
  -f /dev/stdin < "$ROOT_DIR/scripts/gcp06_stage_sanitize.sql" >/dev/null
docker exec -e DATABASE_URL="postgres://postgres@localhost:5432/$STAGE_DB?sslmode=disable" \
  "$CONTAINER" /tmp/dbmigrate -dir /tmp/migrations-v64 >/dev/null

stage_report="$(docker exec "$CONTAINER" psql -X -qAt -U postgres -d "$STAGE_DB" -c \
  "SELECT json_build_object('migration_count',(SELECT count(*) FROM schema_migrations),'migration_max',(SELECT max(version) FROM schema_migrations),'users',(SELECT count(*) FROM usuario),'spaces',(SELECT count(*) FROM espacio),'active_spaces',(SELECT count(*) FROM espacio WHERE estado='activa'),'catalog_candidates',(SELECT count(*) FROM espacio e JOIN categoria_espacio c ON c.codigo=e.categoria_codigo AND c.activa JOIN LATERAL (SELECT 1 FROM tarifa_espacio t0 WHERE t0.espacio_id=e.id ORDER BY t0.version DESC LIMIT 1) t ON true JOIN usuario u ON u.id=e.propietario_id AND u.estado='activo' WHERE e.estado='activa' AND NOT EXISTS(SELECT 1 FROM bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id) AND EXISTS(SELECT 1 FROM elegibilidad_verificacion_local el JOIN verificacion v ON v.id=el.verificacion_id AND v.usuario_id=el.usuario_id AND v.tipo=el.tipo AND v.estado='aprobada' WHERE el.usuario_id=u.id AND el.tipo='kyc' AND el.estado='elegible') AND NOT EXISTS(SELECT 1 FROM reserva_ensayo_local_fixture fx WHERE fx.espacio_id=e.id AND fx.anfitrion_id=e.propietario_id AND fx.habilitada)),'synthetic_kyc',(SELECT count(*) FROM verificacion WHERE clave_idempotencia LIKE 'gcp06-synthetic-kyc-%'),'bookings',(SELECT count(*) FROM reserva_ensayo_local),'active_users',(SELECT count(*) FROM usuario WHERE estado='activo'),'admins',(SELECT count(*) FROM rol_usuario WHERE rol='administrador'),'global_admins',(SELECT count(*) FROM administrador_global),'disabled_hashes',(SELECT count(*) FROM usuario WHERE hash_clave='!gcp06-disabled!' AND estado='activo'))::text" 2>/dev/null)"
python3 - "$stage_report" <<'PY'
import json,sys
r=json.loads(sys.argv[1])
if r.get('migration_count') != 64 or r.get('migration_max') != 64:
    raise SystemExit('staged schema did not reach canonical V1-V64')
if r.get('admins') != 0 or r.get('global_admins') != 0:
    raise SystemExit('global/administrator identities remain in staged dataset')
if r.get('users') != r.get('disabled_hashes'):
    raise SystemExit('staged account verifiers were not fully disabled')
if r.get('active_users') != r.get('users'):
    raise SystemExit('staged synthetic account state was not preserved for public publication policy')
if r.get('active_spaces', 0) > 0 and r.get('catalog_candidates', 0) == 0:
    raise SystemExit('staged active listings do not pass the public catalog eligibility query')
PY

set +e
docker exec "$CONTAINER" pg_dump --format=plain --no-owner --no-privileges --no-comments \
  -U postgres -d "$STAGE_DB" 2>/dev/null > "$ROOT_DIR/$OUTPUT_DIR/data.sql.dump.tmp"
dump_status=$?
set -e
(( dump_status == 0 )) || { echo 'sanitized V64 data export failed.' >&2; exit 1; }
{
  cat <<'SQL'
BEGIN;
DO $gcp06_target_guard$
BEGIN
    IF current_database() !~ '^espacigo_gcp06_candidate_[a-z0-9-]{6,30}$' THEN
        RAISE EXCEPTION 'GCP-06 data can only be imported into a named isolated candidate database';
    END IF;
END
$gcp06_target_guard$;
SQL
  # Cloud SQL SQL import consumes SQL, not psql shell commands. PostgreSQL 18
  # pg_dump adds these two client-side guards; discard only those exact lines.
  sed '/^\\restrict /d; /^\\unrestrict /d' "$ROOT_DIR/$OUTPUT_DIR/data.sql.dump.tmp"
  printf '\nCOMMIT;\n'
} > "$ROOT_DIR/$OUTPUT_DIR/data.sql.tmp"
chmod 0600 "$ROOT_DIR/$OUTPUT_DIR/data.sql.tmp"
rm -f -- "$ROOT_DIR/$OUTPUT_DIR/data.sql.dump.tmp"
mv -- "$ROOT_DIR/$OUTPUT_DIR/data.sql.tmp" "$ROOT_DIR/$OUTPUT_DIR/data.sql"
python3 - "$ROOT_DIR/$OUTPUT_DIR/manifest.json.tmp" "$source_counts" "$stage_report" "$ROOT_DIR/$OUTPUT_DIR/data.sql" <<'PY'
import hashlib,json,os,sys
path,source_raw,stage_raw,dump=sys.argv[1:]
record={"format":"espacigo.gcp06.local-stage.v1","source":json.loads(source_raw),"staged":json.loads(stage_raw),
        "dump_bytes":os.path.getsize(dump),"dump_sha256":hashlib.sha256(open(dump,'rb').read()).hexdigest(),
        "identity_policy":"synthetic dataset namespace only; account identifiers pseudonymized; password verifiers disabled; active account state preserved; administrator/global-bootstrap rows excluded",
        "schema_target":"portable PostgreSQL 18 V1-V64 schema, seeds, ledger, and sanitized data dump; target extensions must be compatible",
        "catalog_policy":"synthetic KYC fixture marker only; destination must be an isolated allowlisted candidate"}
with open(path,'x',encoding='utf-8') as f:
    json.dump(record,f,sort_keys=True,indent=2); f.write('\n')
os.chmod(path,0o600)
PY
mv -- "$ROOT_DIR/$OUTPUT_DIR/manifest.json.tmp" "$ROOT_DIR/$OUTPUT_DIR/manifest.json"
echo "GCP-06 sanitized V64 staging export ready: $OUTPUT_DIR/data.sql"
