#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/gcp06-private-verify.XXXXXX")"
chmod 0700 "$TMP_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_REPO_DIGEST='postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
CONTAINER="gcp06-private-verify-${RUN_ID,,}"
LOGIN='gcp06_inspect_012345abcdef'
CHALLENGE='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'

cleanup() {
  local status=$?
  if (( status != 0 )); then docker logs --tail 40 "$CONTAINER" >&2 2>/dev/null || true; fi
  docker rm -f "$CONTAINER" >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}' 2>/dev/null || true)"
IMAGE_REPO_DIGESTS="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{json .RepoDigests}}' 2>/dev/null || true)"
if [[ "$IMAGE_ID" != "$EXPECTED_IMAGE_ID" && "$IMAGE_REPO_DIGESTS" != *"$EXPECTED_IMAGE_REPO_DIGEST"* ]]; then
  echo 'Pinned PostgreSQL 18 image is unavailable or its digest differs.' >&2
  exit 1
fi
[[ -z "$(docker ps -aq --filter "name=^/${CONTAINER}$")" ]] || { echo 'Disposable container name already exists.' >&2; exit 1; }

docker run -d --rm --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=512m \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" >/dev/null
for _ in $(seq 1 240); do
  ready="$(docker exec "$CONTAINER" pg_isready -U postgres -d postgres 2>/dev/null || true)"
  [[ "$ready" == *"accepting connections"* ]] && break
  sleep 0.25
done
[[ "$ready" == *"accepting connections"* ]] || { echo 'Disposable PostgreSQL did not become ready.' >&2; exit 1; }

docker exec -i "$CONTAINER" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d postgres <<SQL >/dev/null
CREATE DATABASE "espacigo_gcp06_candidate_20261020-gcp06a";
CREATE ROLE $LOGIN LOGIN;
CREATE ROLE gcp06_inspect_abcdef012345 LOGIN;
SQL

docker exec -i "$CONTAINER" psql -X -q -v ON_ERROR_STOP=1 -U postgres -d espacigo_gcp06_candidate_20261020-gcp06a <<SQL >/dev/null
CREATE TABLE public.schema_migrations(version integer PRIMARY KEY);
INSERT INTO public.schema_migrations SELECT generate_series(1,64);
CREATE TABLE public.usuario(id uuid PRIMARY KEY, estado text NOT NULL, hash_clave text NOT NULL);
INSERT INTO public.usuario
SELECT ('10000000-0000-4000-8000-' || lpad(n::text,12,'0'))::uuid, 'activo', '!gcp06-disabled!'
FROM generate_series(1,10) n;
CREATE TABLE public.categoria_espacio(codigo text PRIMARY KEY, activa boolean NOT NULL);
INSERT INTO public.categoria_espacio VALUES ('oficina',true);
CREATE TABLE public.espacio(id uuid PRIMARY KEY, estado text NOT NULL, categoria_codigo text NOT NULL, propietario_id uuid NOT NULL);
INSERT INTO public.espacio VALUES
 ('20000000-0000-4000-8000-000000000001','activa','oficina','10000000-0000-4000-8000-000000000001'),
 ('20000000-0000-4000-8000-000000000002','activa','oficina','10000000-0000-4000-8000-000000000002'),
 ('20000000-0000-4000-8000-000000000003','borrador','oficina','10000000-0000-4000-8000-000000000003');
CREATE TABLE public.tarifa_espacio(espacio_id uuid NOT NULL, version integer NOT NULL);
INSERT INTO public.tarifa_espacio SELECT id, 1 FROM public.espacio;
CREATE TABLE public.bloqueo_cuenta_administrativo_local(cuenta_id uuid NOT NULL);
CREATE TABLE public.elegibilidad_verificacion_local(usuario_id uuid NOT NULL, tipo text NOT NULL, estado text NOT NULL, verificacion_id uuid NOT NULL);
CREATE TABLE public.verificacion(id uuid PRIMARY KEY, usuario_id uuid NOT NULL, tipo text NOT NULL, estado text NOT NULL, clave_idempotencia text NOT NULL);
INSERT INTO public.verificacion VALUES
 ('30000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000001','kyc','aprobada','gcp06-synthetic-kyc-fixture-1');
INSERT INTO public.elegibilidad_verificacion_local VALUES
 ('10000000-0000-4000-8000-000000000001','kyc','elegible','30000000-0000-4000-8000-000000000001');
CREATE TABLE public.reserva_ensayo_local(id uuid PRIMARY KEY);
INSERT INTO public.reserva_ensayo_local SELECT ('40000000-0000-4000-8000-' || lpad(n::text,12,'0'))::uuid FROM generate_series(1,4) n;
CREATE TABLE public.reserva_ensayo_local_fixture(espacio_id uuid NOT NULL, anfitrion_id uuid NOT NULL, habilitada boolean NOT NULL);
CREATE TABLE public.rol_usuario(usuario_id uuid NOT NULL, rol text NOT NULL);
CREATE TABLE public.administrador_global(usuario_id uuid PRIMARY KEY);
GRANT USAGE ON SCHEMA public TO ${LOGIN};
GRANT SELECT ON ALL TABLES IN SCHEMA public TO ${LOGIN};
SQL

umask 077
docker exec -i "$CONTAINER" psql -X -qAt -v ON_ERROR_STOP=1 -U "$LOGIN" \
  -v "gcp06_challenge=$CHALLENGE" \
  -d espacigo_gcp06_candidate_20261020-gcp06a \
  -f /dev/stdin < "$ROOT_DIR/planning/gcp/sql_candidates/gcp06_candidate_data_verify_v1_v64.sql" > "$TMP_DIR/actual.json"
chmod 0600 "$TMP_DIR/actual.json"

python3 - "$TMP_DIR/actual.json" "$TMP_DIR/target-evidence.json" "$TMP_DIR/resource-snapshot.json" <<'PY'
import hashlib,json,sys
from pathlib import Path
actual_path=Path(sys.argv[1]); actual=json.loads(actual_path.read_text(encoding='utf-8'))
resource={"name":"espacigo-gcp06-20261020-gcp06a-pg","region":"southamerica-west1",
          "connectionName":"espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg",
          "databaseVersion":"POSTGRES_18","state":"RUNNABLE"}
resource_path=Path(sys.argv[3]); resource_path.write_text(json.dumps(resource,sort_keys=True),encoding='utf-8'); resource_path.chmod(0o600)
evidence={"transport":"cloud_sql_auth_proxy",
          "proxy_connection_name":"espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg",
          "resource_snapshot_sha256":hashlib.sha256(resource_path.read_bytes()).hexdigest(),
          "challenge_nonce":actual["challenge_nonce"],
          "query_output_sha256":hashlib.sha256(actual_path.read_bytes()).hexdigest(),
          "session_user":actual["session_user"],"observed_at":actual["observed_at"]}
path=Path(sys.argv[2]); path.write_text(json.dumps(evidence,sort_keys=True),encoding='utf-8'); path.chmod(0o600)
PY

python3 - "$TMP_DIR/manifest.json" <<'PY'
import json,sys
from pathlib import Path
manifest={
  "format":"espacigo.gcp06.local-stage.v1",
  "source":{"migration_count":61,"migration_max":61},
  "staged":{
    "migration_count":64,"migration_max":64,"users":10,"spaces":3,"active_spaces":2,
    "catalog_candidates":1,"synthetic_kyc":1,"bookings":4,"active_users":10,
    "admins":0,"global_admins":0,"disabled_hashes":10
  },
  "dump_bytes":16620453,
  "dump_sha256":"bb35a8709ae74d51e73b69d0dd75d7b4b761822508353dab9dddcd631cec9c11"
}
path=Path(sys.argv[1]); path.write_text(json.dumps(manifest,sort_keys=True),encoding='utf-8'); path.chmod(0o600)
PY

if ! python3 "$ROOT_DIR/scripts/gcp06_candidate_verify_manifest.py" \
  --manifest "$TMP_DIR/manifest.json" --actual "$TMP_DIR/actual.json" \
  --target-evidence "$TMP_DIR/target-evidence.json" --resource-snapshot "$TMP_DIR/resource-snapshot.json" > "$TMP_DIR/report.json"; then
  cat "$TMP_DIR/report.json" >&2
  exit 1
fi
python3 - "$TMP_DIR/report.json" <<'PY'
import json,sys
report=json.load(open(sys.argv[1],encoding='utf-8'))
if report.get('status')!='PASS' or report.get('query_id')!='GCP06-ROWS-001' or 'manifest_sha256' not in report:
    raise SystemExit('synthetic manifest/query comparison did not pass')
print('PASS: fixed V1-V64 row aggregate query and private manifest comparator on disposable PostgreSQL 18')
PY

set +e
wrong_target_output="$(docker exec -i "$CONTAINER" psql -X -qAt -v ON_ERROR_STOP=1 -U "$LOGIN" -d postgres \
  -f /dev/stdin < "$ROOT_DIR/planning/gcp/sql_candidates/gcp06_candidate_data_verify_v1_v64.sql" 2>&1)"
wrong_target_status=$?
set -e
if [[ "$wrong_target_output" != *GCP06_STOP_TARGET* || "$wrong_target_output" == *'"query_id"'* ]]; then
  echo "Candidate SQL target guard result did not stop as expected (exit=$wrong_target_status, output=$wrong_target_output)." >&2
  exit 1
fi
echo 'PASS: fixed query rejects a non-candidate database before reading application tables'

set +e
denied_output="$(docker exec -i "$CONTAINER" psql -X -qAt -v ON_ERROR_STOP=1 -U gcp06_inspect_abcdef012345 \
  -d espacigo_gcp06_candidate_20261020-gcp06a \
  -f /dev/stdin < "$ROOT_DIR/planning/gcp/sql_candidates/gcp06_candidate_data_verify_v1_v64.sql" 2>&1)"
denied_status=$?
set -e
if (( denied_status == 0 )) || [[ "$denied_output" == *'"query_id"'* ]]; then
  echo 'Fixed query did not stop when the synthetic login lacked row SELECT.' >&2
  exit 1
fi
echo 'PASS: separate row-query phase stops when SELECT is denied; no grants or role changes are attempted'
