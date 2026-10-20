#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${G03O_SOURCE_ACCESS_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
[[ "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]] || { echo 'Unsafe run id.' >&2; exit 2; }
PROJECT="espacigo-g03o-source-access-${RUN_ID,,}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-g03o-source-access.XXXXXX")"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
chmod 0700 "$TMP_DIR"

compose() { docker compose --project-directory "$ROOT_DIR" --project-name "$PROJECT" -f "$COMPOSE_FILE" "$@"; }
cleanup() {
  local status=$?
  if [[ "$status" -ne 0 ]]; then compose logs postgres >&2 || true; fi
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  python3 - "$TMP_DIR" <<'PY'
import shutil, sys
shutil.rmtree(sys.argv[1], ignore_errors=True)
PY
  return "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}')"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL 18/PostGIS image unavailable; refusing pull or substitute.' >&2; exit 1; }
[[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]] || { echo 'Disposable Compose project already exists.' >&2; exit 1; }

python3 - "$COMPOSE_FILE" "$IMAGE" "$CONTAINER" "$RUN_ID" <<'PY'
import json, sys
from pathlib import Path
compose_file, image, container, run_id = sys.argv[1:]
Path(compose_file).write_text(json.dumps({"services":{"postgres":{
    "image":image,"container_name":container,"network_mode":"none",
    "command":["postgres","-c","listen_addresses=*","-c","max_connections=40"],
    "tmpfs":["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
    "environment":{"POSTGRES_HOST_AUTH_METHOD":"trust"},
    "labels":{"com.espacigo.scope":"g03o-source-access-preflight-test","com.espacigo.run":run_id}
}}}),encoding="utf-8")
PY
compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$CONTAINER" <<'PY'
import json, sys
from pathlib import Path
s=json.loads(Path(sys.argv[1]).read_text())["services"]["postgres"]
assert s["image"]==sys.argv[2] and s["container_name"]==sys.argv[3]
assert s["network_mode"]=="none" and not s.get("ports")
assert any(x.startswith("/var/lib/postgresql:") for x in s["tmpfs"])
print("Disposable G03O source-report fixture resolved: pinned image, network none, tmpfs data.")
PY

compose up -d postgres >/dev/null
ready=""
pid1=""
for _ in $(seq 1 240); do
  ready="$(compose exec -T postgres psql -X -At -U postgres -d postgres -c "SELECT current_setting('listen_addresses')" 2>/dev/null || true)"
  pid1="$(compose exec -T postgres cat /proc/1/comm 2>/dev/null || true)"
  [[ "$ready" == '*' && "$pid1" == 'postgres' ]] && break
  sleep 0.25
done
[[ "$ready" == '*' && "$pid1" == 'postgres' ]] || { echo 'Final PostgreSQL 18 server did not become ready.' >&2; exit 1; }
psql_local() { compose exec -T postgres psql -X -v ON_ERROR_STOP=1 "$@"; }
psql_local -U postgres -d postgres <<'SQL' >/dev/null
CREATE ROLE g03o_report_login LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE DATABASE espacigo_scratch;
SQL
psql_local -U postgres -d postgres -c 'GRANT CONNECT ON DATABASE espacigo_scratch TO g03o_report_login' >/dev/null
psql_local -U postgres -d espacigo_scratch <<'SQL' >/dev/null
CREATE TABLE public.usuario(id integer);
CREATE TABLE public.espacio(id integer);
CREATE TABLE public.reserva(id integer);
CREATE TABLE public.schema_migrations(version integer,name text,checksum text,runner_checksum text,applied_at timestamptz);
CREATE TABLE public.atributo_filtro_publico(id integer);
CREATE TABLE public.categoria_espacio(id integer);
CREATE TABLE public.categoria_perfil_atributos(id integer);
CREATE TABLE public.catalogo_ubicacion_publica(id integer);
CREATE TABLE public.comuna_publica(id integer);
CREATE TABLE public.region_publica(id integer);
CREATE TABLE public.version_terminos(id integer);
CREATE TABLE public.foto_perfil_sintetica_local(id integer);
CREATE TABLE public.espacio_galeria_sintetica_local(id integer);
CREATE TABLE public.verificacion_evidencia_sintetica(id integer);
CREATE TABLE public.espacio_galeria_archivo_candidato_local(id integer);
CREATE TABLE public.operacion_arriendo_archivo_candidato_local(id integer);
CREATE TABLE public.verificacion_archivo_retiro_pendiente_local(id integer);
CREATE TABLE public.baja_archivo_pendiente_local(id integer);
CREATE TABLE public.operacion_arriendo_evidencia_ensayo_local(id integer);
INSERT INTO public.usuario VALUES (1);
REVOKE USAGE ON SCHEMA public FROM PUBLIC;
SQL

set +e
python3 - "$ROOT_DIR" <<'PY' | compose exec -T postgres psql -X -qAt --set=ON_ERROR_STOP=1 --set=VERBOSITY=sqlstate -U g03o_report_login -d espacigo_scratch -f - > "$TMP_DIR/blocked.stdout" 2> "$TMP_DIR/blocked.stderr"
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import source_report_script
sys.stdout.write(source_report_script())
PY
blocked_status=$?
set -e
[[ "$blocked_status" == 3 ]] || { echo "Expected the read-only access guard to stop before count_usuario; exit=$blocked_status" >&2; exit 1; }
if rg -q '__G03O_V64_REPORT_COMPONENT__count_usuario' "$TMP_DIR/blocked.stdout"; then
  echo 'The count query ran despite the failed read-access precheck.' >&2; exit 1
fi
python3 - "$ROOT_DIR" "$TMP_DIR/blocked.stdout" "$TMP_DIR/blocked.stderr" <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import failed_source_report_component, failed_source_table_access
stdout, stderr = Path(sys.argv[2]).read_bytes(), Path(sys.argv[3]).read_bytes()
assert failed_source_report_component(stdout) == "source_table_access"
assert failed_source_table_access(stdout) == {
    "source_table_present": True,
    "source_table_select_allowed": False,
    "source_public_schema_usage_allowed": False,
    "source_schema_owner_role_present": False,
    "source_schema_owner_member": False,
    "source_schema_owner_set_allowed": False,
    "source_table_owner_role_present": True,
    "source_table_owner_is_current_user": False,
    "source_current_user_can_set_table_owner": False,
}
print('Missing SELECT/USAGE and absent owner role are reported as booleans before count.')
PY

psql_local -U postgres -d espacigo_scratch <<'SQL' >/dev/null
CREATE ROLE espacigo_schema_owner NOLOGIN;
ALTER TABLE public.usuario OWNER TO espacigo_schema_owner;
GRANT espacigo_schema_owner TO g03o_report_login WITH ADMIN FALSE, INHERIT FALSE, SET FALSE;
SQL
set +e
python3 - "$ROOT_DIR" <<'PY' | compose exec -T postgres psql -X -qAt --set=ON_ERROR_STOP=1 --set=VERBOSITY=sqlstate -U g03o_report_login -d espacigo_scratch -f - > "$TMP_DIR/member.stdout" 2> "$TMP_DIR/member.stderr"
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import source_report_script
sys.stdout.write(source_report_script())
PY
member_status=$?
set -e
[[ "$member_status" == 3 ]] || { echo "Expected missing access to stop before count after membership setup; exit=$member_status" >&2; exit 1; }
python3 - "$ROOT_DIR" "$TMP_DIR/member.stdout" <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import failed_source_table_access
assert failed_source_table_access(Path(sys.argv[2]).read_bytes()) == {
    "source_table_present": True,
    "source_table_select_allowed": False,
    "source_public_schema_usage_allowed": False,
    "source_schema_owner_role_present": True,
    "source_schema_owner_member": True,
    "source_schema_owner_set_allowed": False,
    "source_table_owner_role_present": True,
    "source_table_owner_is_current_user": False,
    "source_current_user_can_set_table_owner": False,
}
print('PostgreSQL 18 distinguishes owner role membership from SET ROLE permission.')
PY

psql_local -U postgres -d espacigo_scratch <<'SQL' >/dev/null
GRANT espacigo_schema_owner TO g03o_report_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT USAGE ON SCHEMA public TO PUBLIC;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO g03o_report_login;
SQL
python3 - "$ROOT_DIR" <<'PY' | compose exec -T postgres psql -X -qAt --set=ON_ERROR_STOP=1 --set=VERBOSITY=sqlstate -U g03o_report_login -d espacigo_scratch -f - > "$TMP_DIR/allowed.stdout" 2> "$TMP_DIR/allowed.stderr"
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import source_report_script
sys.stdout.write(source_report_script())
PY
python3 - "$ROOT_DIR" "$TMP_DIR/allowed.stdout" <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import parse_source_report_output
report = parse_source_report_output(Path(sys.argv[2]).read_bytes())
assert report['source_table_access'] == {
    'usuario_present': True,
    'usuario_select_allowed': True,
    'public_schema_usage_allowed': True,
    'schema_owner_role_present': True,
    'schema_owner_member': True,
    'schema_owner_set_allowed': True,
    'usuario_owner_role_present': True,
    'usuario_owner_is_current_user': False,
    'current_user_can_set_usuario_owner': True,
}
assert report['counts']['usuario'] == 1 and report['report_complete'] is True
print('With fixture-only schema usage, SET ROLE membership, and SELECT, the report passes and completes.')
PY

psql_local -U postgres -d espacigo_scratch <<'SQL' >/dev/null
GRANT CREATE ON SCHEMA public TO g03o_report_login;
ALTER TABLE public.usuario OWNER TO g03o_report_login;
SQL
python3 - "$ROOT_DIR" <<'PY' | compose exec -T postgres psql -X -qAt --set=ON_ERROR_STOP=1 --set=VERBOSITY=sqlstate -U g03o_report_login -d espacigo_scratch -f - > "$TMP_DIR/owner.stdout" 2> "$TMP_DIR/owner.stderr"
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import source_report_script
sys.stdout.write(source_report_script())
PY
python3 - "$ROOT_DIR" "$TMP_DIR/owner.stdout" <<'PY'
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import parse_source_report_output
report = parse_source_report_output(Path(sys.argv[2]).read_bytes())
access = report['source_table_access']
assert access['usuario_owner_role_present'] is True
assert access['usuario_owner_is_current_user'] is True
assert access['current_user_can_set_usuario_owner'] is True
print('When current_user owns usuario, only the expected owner booleans are exposed.')
PY

run_owner_discovery() {
  local login="$1" output="$2"
  python3 - "$ROOT_DIR" <<'PY' | compose exec -T postgres psql -X -qAt --set=ON_ERROR_STOP=1 --set=VERBOSITY=sqlstate -U "$login" -d espacigo_scratch -f - > "$output"
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import source_owner_discovery_sql
sys.stdout.write(source_owner_discovery_sql() + ";\n")
PY
}

psql_local -U postgres -d espacigo_scratch -c 'ALTER TABLE public.usuario OWNER TO postgres' >/dev/null
run_owner_discovery postgres "$TMP_DIR/owner-unique.json"
python3 - "$TMP_DIR/owner-unique.json" <<'PY'
import json, sys
from pathlib import Path
report=json.loads(Path(sys.argv[1]).read_text())
assert report['complete'] is True
assert report['owner_count'] == 1
assert report['set_allowed'] is True
assert report['access_complete'] is True
assert report['row_security_complete'] is True
print('Real owner inventory passes for one complete accessible owner.')
PY

psql_local -U postgres -d espacigo_scratch -c 'ALTER TABLE public.usuario OWNER TO g03o_report_login' >/dev/null
run_owner_discovery postgres "$TMP_DIR/owner-mixed.json"
python3 - "$TMP_DIR/owner-mixed.json" <<'PY'
import json, sys
from pathlib import Path
report=json.loads(Path(sys.argv[1]).read_text())
assert report['complete'] is True
assert report['owner_count'] > 1
print('Real owner inventory detects mixed owners and blocks the source cut.')
PY

python3 - "$ROOT_DIR" <<'PY' | compose exec -T postgres psql -X -qAt --set=ON_ERROR_STOP=1 -U postgres -d espacigo_scratch >/dev/null
import sys
from pathlib import Path
sys.path.insert(0, str(Path(sys.argv[1]) / "scripts"))
from gcp03_origin_v64_transfer import SOURCE_REPORT_TABLES, SOURCE_REFERENCE_TABLES
print("DO $$ DECLARE name text; BEGIN FOR name IN SELECT tablename FROM pg_tables WHERE schemaname='public' LOOP EXECUTE format('ALTER TABLE public.%I OWNER TO espacigo_schema_owner',name); END LOOP; END $$;")
PY
psql_local -U postgres -d espacigo_scratch -c 'GRANT espacigo_schema_owner TO g03o_report_login WITH ADMIN FALSE, INHERIT FALSE, SET FALSE' >/dev/null
run_owner_discovery g03o_report_login "$TMP_DIR/owner-denied-set.json"
python3 - "$TMP_DIR/owner-denied-set.json" <<'PY'
import json, sys
from pathlib import Path
report=json.loads(Path(sys.argv[1]).read_text())
assert report['complete'] is True
assert report['owner_count'] == 1
assert report['set_allowed'] is False
assert report['access_complete'] is True
assert report['row_security_complete'] is True
print('Real owner inventory denies continuation when SET ROLE is unavailable.')
PY

psql_local -U postgres -d espacigo_scratch -c 'GRANT espacigo_schema_owner TO g03o_report_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE' >/dev/null
psql_local -U postgres -d espacigo_scratch -c 'ALTER TABLE public.usuario ENABLE ROW LEVEL SECURITY; ALTER TABLE public.usuario FORCE ROW LEVEL SECURITY' >/dev/null
run_owner_discovery g03o_report_login "$TMP_DIR/owner-forced-rls.json"
python3 - "$TMP_DIR/owner-forced-rls.json" <<'PY'
import json, sys
from pathlib import Path
report=json.loads(Path(sys.argv[1]).read_text())
assert report['complete'] is True
assert report['owner_count'] == 1
assert report['set_allowed'] is True
assert report['access_complete'] is True
assert report['row_security_complete'] is False
print('Real owner inventory detects forced RLS that would make the dump incomplete.')
PY

compose down --volumes --remove-orphans >/dev/null
[[ -z "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]]
[[ -z "$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT")" ]]
echo 'G03O source access precheck passed in denied, MEMBER/SET, allowed, and current-owner local cases.'
