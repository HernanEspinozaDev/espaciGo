#!/usr/bin/env bash
set -Eeuo pipefail

readonly EXPECTED_BRANCH='codex/marketplace/fix/kyc-06-save-identity-first'
readonly EXPECTED_HEAD='274f37e483a078890e92382a159fa49fd23c2fe8'
readonly DATABASE='espacigo_dms_stage_20261020'
readonly DATABASE_OID='16388'
readonly PORT='5432'
readonly SOCKET_DIR='/var/run/postgresql'
readonly MIGRATOR='espacigo_migrator_login'
readonly SET_ROLE='espacigo_schema_owner'
readonly PGPASS='/home/nandev/.local/share/espacigo-native-secrets/pgpass'
readonly BACKUP_DIR='/home/nandev/.local/share/espacigo-native-backups'
readonly PRE_V82_BACKUP="$BACKUP_DIR/espacigo_dms_stage_20261020-post-v81-pre-v82-20261108T064927Z.dump"
readonly PRE_V82_BACKUP_SHA='99fba9d7f2b186102cb727b760089847012f368300640eb21f6fd69f7a13cbb4'
readonly EXPECTED_INVENTORY_SHA256='fba8f6351e14622dbe8d65ae8c594d3ff73bf63a96976bbd7dd7a52417f8bc23'
readonly OPERATOR_USER='nandev'
readonly OPERATOR_HOME='/home/nandev'
readonly PRIVATE_SCRATCH='/home/nandev/.hermes/cache/scratch'
readonly NANDEV_PATH='/usr/local/go/bin:/usr/local/bin:/usr/bin:/bin'

if [[ $# -gt 1 || ( $# -eq 1 && "$1" != '--preflight-only' ) ]]; then
    printf 'usage: %s [--preflight-only]\n' "$0" >&2
    exit 2
fi
readonly PREFLIGHT_ONLY="${1:-}"

[[ "$EUID" == 0 && "${SUDO_USER:-}" == "$OPERATOR_USER" ]] || { echo 'run this supervisor directly as sudo from the nandev session' >&2; exit 2; }
readonly NANDEV_UID="$(id -u "$OPERATOR_USER")"
readonly NANDEV_GID="$(id -g "$OPERATOR_USER")"
repo_root='/home/nandev/Documentos/projects/espaciGo-main-sync'
[[ "$(runuser -u "$OPERATOR_USER" -- git -C "$repo_root" rev-parse --show-toplevel)" == "$repo_root" ]] || { echo 'refusing: unexpected task worktree' >&2; exit 2; }
cd "$repo_root"
[[ "$(runuser -u "$OPERATOR_USER" -- git -C "$repo_root" branch --show-current)" == "$EXPECTED_BRANCH" ]] || { echo 'refusing: unexpected task branch' >&2; exit 2; }
[[ "$(runuser -u "$OPERATOR_USER" -- git -C "$repo_root" rev-parse HEAD)" == "$EXPECTED_HEAD" ]] || { echo 'refusing: unexpected HEAD' >&2; exit 2; }
[[ -d "$SOCKET_DIR" ]] || { echo 'refusing: expected native PostgreSQL socket directory is absent' >&2; exit 2; }

[[ -d "$PRIVATE_SCRATCH" && "$(stat -c '%a' "$PRIVATE_SCRATCH")" == '700' && "$(stat -c '%u' "$PRIVATE_SCRATCH")" == "$NANDEV_UID" ]] || { echo 'refusing: private Hermes scratch must be owned by nandev with mode 0700' >&2; exit 2; }
[[ "$(stat -c '%a' "$(dirname "$PGPASS")")" == '700' && "$(stat -c '%u' "$(dirname "$PGPASS")")" == "$NANDEV_UID" ]] || { echo 'refusing: migrator passfile directory must be private mode 0700' >&2; exit 2; }
[[ -f "$PGPASS" && "$(stat -c '%a' "$PGPASS")" == '600' && "$(stat -c '%u' "$PGPASS")" == "$NANDEV_UID" ]] || { echo 'refusing: migrator passfile must be a nandev-owned regular file mode 0600' >&2; exit 2; }
[[ "$(stat -c '%a' "$BACKUP_DIR")" == '700' && "$(stat -c '%u' "$BACKUP_DIR")" == "$NANDEV_UID" ]] || { echo 'refusing: native backup directory must be private mode 0700' >&2; exit 2; }

unset PGPASSWORD PGOPTIONS PGSERVICE PGSERVICEFILE PGHOST PGHOSTADDR PGPORT PGDATABASE PGUSER PGTARGETSESSIONATTRS
unset DATABASE_PASSWORD_FILE CLOUD_SQL_INSTANCE_CONNECTION_NAME CLOUD_SQL_PRIVATE_IP
readonly DATABASE_URL="postgresql://$MIGRATOR@127.0.0.1:$PORT/$DATABASE?sslmode=disable"

workdir=$(runuser -u "$OPERATOR_USER" -- mktemp -d "$PRIVATE_SCRATCH/espacigo-native-v82-v85.XXXXXX")
child_pid=''
cleanup_v83=0
cleanup_v85=0
cleanup_failed=0

admin_psql() {
    runuser -u postgres -- env HOME=/var/lib/postgresql PATH=/usr/bin:/bin psql -X -v ON_ERROR_STOP=1 -h "$SOCKET_DIR" -p "$PORT" -U postgres -d "$DATABASE" "$@"
}

run_nandev() {
    runuser -u "$OPERATOR_USER" -- env HOME="$OPERATOR_HOME" USER="$OPERATOR_USER" LOGNAME="$OPERATOR_USER" PATH="$NANDEV_PATH" "$@"
}

run_migrator_command() {
    run_nandev env -u PGPASSWORD -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE -u PGHOST -u PGHOSTADDR -u PGPORT -u PGDATABASE -u PGUSER \
        PGPASSFILE="$PGPASS" DATABASE_URL="$DATABASE_URL" DATABASE_CONNECTION_MODE=direct DATABASE_SET_ROLE="$SET_ROLE" DBMIGRATE_TIMEOUT=10m "$@"
}

cleanup() {
    local status=$?
    trap - EXIT INT TERM
    if [[ -n "$child_pid" ]] && kill -0 "$child_pid" 2>/dev/null; then
        kill -TERM -- "-$child_pid" 2>/dev/null || true
        for _ in {1..15}; do
            kill -0 "$child_pid" 2>/dev/null || break
            sleep 1
        done
        if kill -0 "$child_pid" 2>/dev/null; then
            kill -KILL -- "-$child_pid" 2>/dev/null || true
        fi
        wait "$child_pid" 2>/dev/null || true
        child_pid=''
    fi
    if (( cleanup_v85 )); then
        echo 'cleanup: closing native V84/V85 temporary authority'
        if ! admin_psql < "$workdir/finalize_native_v85.sql"; then
            cleanup_failed=1
            echo "cleanup failed; keep private evidence at $workdir" >&2
        else
            cleanup_v85=0
        fi
    fi
    if (( cleanup_v83 )); then
        echo 'cleanup: closing native V82/V83 temporary authority'
        if ! admin_psql < "$workdir/finalize_native_v83.sql"; then
            cleanup_failed=1
            echo "cleanup failed; keep private evidence at $workdir" >&2
        else
            cleanup_v83=0
        fi
    fi
    if (( cleanup_failed )); then
        status=1
    fi
    if (( status == 0 && cleanup_failed == 0 )); then
        rm -rf -- "$workdir"
    else
        echo "native migration operation stopped; private diagnostics retained at $workdir" >&2
    fi
    exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

render_template() {
    local source="$1" destination="$2"
    run_nandev python3 - "$source" "$destination" "$DATABASE" "$DATABASE_OID" <<'PY'
from pathlib import Path
import os, sys
source, destination, database, oid = map(str, sys.argv[1:])
text = Path(source).read_text()
if text.count('__NATIVE_TARGET_DATABASE__') != 1 or text.count('__NATIVE_TARGET_OID__') != 1:
    raise SystemExit(f'{source}: target pins must each occur exactly once')
text = text.replace('__NATIVE_TARGET_DATABASE__', "'" + database.replace("'", "''") + "'", 1)
text = text.replace('__NATIVE_TARGET_OID__', oid, 1)
path = Path(destination)
path.write_text(text)
os.chmod(path, 0o600)
PY
}

verify_plan() {
    local plan_file="$1" expected_applied="$2" expected_pending="$3"
    python3 - "$plan_file" "$DATABASE" "$expected_applied" "$expected_pending" <<'PY'
import json, sys
path, expected_db, applied_count, pending_csv = sys.argv[1:]
plan = json.load(open(path, encoding='utf-8'))
applied_count = int(applied_count)
pending = [] if pending_csv == '-' else [int(v) for v in pending_csv.split(',')]
if plan.get('database') != expected_db or not plan.get('read_only') or not plan.get('history_exists'):
    raise SystemExit('plan target/read-only/history gate failed')
if not str(plan.get('server_version', '')).startswith('18.6'):
    raise SystemExit('unexpected PostgreSQL server version')
if [m.get('version') for m in plan.get('applied', [])] != list(range(1, applied_count + 1)):
    raise SystemExit('applied ledger is not the exact contiguous expected prefix')
if [m.get('version') for m in plan.get('pending', [])] != pending:
    raise SystemExit('pending migration suffix differs from the exact expected versions')
expected_checksums = {
    81: '7b9700b546532854ab7569654d05039c1df832a6a80154ae053d13b5cea43967',
    82: 'b53fe7f6f03af4d9081e4a378053ad0780d8b532962326488108201eb098ed96',
    83: '405c2f02f8a29e962421a44cbbd07e6e7fae4aa4696483e9d45ab209e82e9333',
    84: 'd8cc68f3ea73c6e351a31bd8f88fd0606e8217c028777d5ebca7b406bb852b8f',
    85: '7b78094558e2d9bde7804919b01c54d34caf2ac31ac34fb1b86a63999aeeaf51',
}
for migration in plan.get('applied', []) + plan.get('pending', []):
    version = migration.get('version')
    if version in expected_checksums and migration.get('checksum') != expected_checksums[version]:
        raise SystemExit(f'V{version} source/history checksum differs from the reviewed pin')
if applied_count in (81,83,84):
    authority = plan.get('ddl_authority') or {}
    if authority.get('session_user') != 'espacigo_migrator_login' or authority.get('current_user') != 'espacigo_schema_owner':
        raise SystemExit('dedicated migration identity gate failed')
    catalog = authority.get('catalog') or {}
    if catalog.get('schemas') != ['pg_catalog','public']:
        raise SystemExit('V84/V85 effective migration search path is not constrained to pg_catalog/public')
    if not authority.get('public_usage') or authority.get('public_create') or authority.get('usuario_id_references'):
        raise SystemExit('V82/V83 baseline authority is not the reviewed closed state')
    if not authority.get('ledger_select') or not authority.get('ledger_insert'):
        raise SystemExit('schema_migrations owner contract is incomplete')
print(f'PASS: read-only plan V1..V{applied_count}, pending {pending}')
PY
}

run_migrator() {
    local directory="$1" status
    setsid runuser -u "$OPERATOR_USER" -- env HOME="$OPERATOR_HOME" USER="$OPERATOR_USER" LOGNAME="$OPERATOR_USER" PATH="$NANDEV_PATH" \
        PGPASSFILE="$PGPASS" DATABASE_URL="$DATABASE_URL" DATABASE_CONNECTION_MODE=direct DATABASE_SET_ROLE="$SET_ROLE" DBMIGRATE_TIMEOUT=10m \
        "$workdir/dbmigrate" -dir "$directory" &
    child_pid=$!
    if wait "$child_pid"; then status=0; else status=$?; fi
    child_pid=''
    return "$status"
}

# Pin the native peer-admin target before any grant.
admin_fingerprint=$(admin_psql -At -F '|' -c "SELECT current_database(),d.oid,current_setting('server_version_num'),session_user,current_user,inet_client_addr()::text FROM pg_catalog.pg_database d WHERE d.datname=current_database()")
[[ "$admin_fingerprint" == "$DATABASE|$DATABASE_OID|180006|postgres|postgres|" ]] || { echo 'refusing: native peer-admin target fingerprint differs' >&2; exit 2; }

render_template db/repair/prepare_native_v82_v83.sql.tmpl "$workdir/prepare_native_v82_v83.sql"
render_template db/repair/finalize_native_v83.sql.tmpl "$workdir/finalize_native_v83.sql"
render_template db/repair/prepare_native_v84_v85.sql.tmpl "$workdir/prepare_native_v84_v85.sql"
render_template db/repair/finalize_native_v85.sql.tmpl "$workdir/finalize_native_v85.sql"

run_nandev env GOTOOLCHAIN=local TMPDIR="$PRIVATE_SCRATCH" go build -trimpath -o "$workdir/dbmigrate" ./cmd/dbmigrate
chmod 700 "$workdir/dbmigrate"

run_nandev "$workdir/dbmigrate" -inventory-only > "$workdir/source-inventory.json"
python3 - "$workdir/source-inventory.json" planning/gcp/migraciones_inventory_v001_v085_20261107.json planning/gcp/migraciones_source_manifest_v001_v084_20261107.tsv "$EXPECTED_BRANCH" "$EXPECTED_HEAD" "$EXPECTED_INVENTORY_SHA256" <<'PY'
import json, sys
from pathlib import Path
actual_path, pinned_path, tsv_path, branch, head, expected_root = sys.argv[1:]
actual = json.loads(Path(actual_path).read_text())
pinned = json.loads(Path(pinned_path).read_text())
if actual != pinned or actual.get('inventory_sha256') != expected_root:
    raise SystemExit('complete V1..V85 inventory differs from the pinned reviewed manifest')
lines = Path(tsv_path).read_text().splitlines()
if f'# Branch: {branch}' not in lines or f'# HEAD: {head}' not in lines:
    raise SystemExit('V1..V84 source manifest is not pinned to the expected branch and HEAD')
rows = [line.split('\t') for line in lines if line and not line.startswith('#')]
migrations = actual.get('migrations', [])
if len(rows) != 84 or len(migrations) != 85:
    raise SystemExit('source manifest or complete inventory has an unexpected count')
for index, (row, migration) in enumerate(zip(rows, migrations[:84]), start=1):
    if len(row) != 3 or row != [f'{index:03d}', migration['name'], migration['checksum']] or migration['version'] != index:
        raise SystemExit(f'V{index:03d} differs from the independently reviewed V1..V84 manifest')
print(f'PASS: complete source inventory V1..V85 pinned at sha256={expected_root}; V1..V84 manifest reconciled')
PY

run_nandev python3 - "$PWD/db/migrations" "$workdir" <<'PY'
from pathlib import Path
import hashlib, os, re, shutil, sys
source, workdir = map(Path, sys.argv[1:])
files = sorted(source.glob('V*.sql'))
versions = []
for path in files:
    match = re.fullmatch(r'V(\d{6})__[^/]+\.sql', path.name)
    if not match:
        raise SystemExit(f'unexpected migration filename: {path.name}')
    versions.append(int(match.group(1)))
if versions != list(range(1, 86)):
    raise SystemExit('source migration inventory is not exactly V1..V85')
for count in (83,84):
    destination = workdir / f'migrations-v{count}'
    destination.mkdir(mode=0o700)
    for path in files[:count]:
        target = destination / path.name
        shutil.copyfile(path, target)
        os.chmod(target, 0o600)
        if hashlib.sha256(path.read_bytes()).digest() != hashlib.sha256(target.read_bytes()).digest():
            raise SystemExit(f'staged source differs byte-for-byte: {path.name}')
    if len(list(destination.glob('V*.sql'))) != count:
        raise SystemExit(f'V1..V{count} private source snapshot count mismatch')
print('PASS: staged source V1..V83/V84 byte-for-byte from the reviewed V1..V85 inventory')
PY

run_migrator_command "$workdir/dbmigrate" -plan-only -dir db/migrations > "$workdir/plan-before.json"
initial_applied=$(python3 - "$workdir/plan-before.json" <<'PY'
import json, sys
print(len(json.load(open(sys.argv[1], encoding='utf-8')).get('applied', [])))
PY
)
case "$initial_applied" in
    81) verify_plan "$workdir/plan-before.json" 81 '82,83,84,85' ;;
    83) verify_plan "$workdir/plan-before.json" 83 '84,85' ;;
    84) verify_plan "$workdir/plan-before.json" 84 '85' ;;
    *) echo "refusing: native ledger prefix $initial_applied is not an approved V82-V85 resume boundary" >&2; exit 2 ;;
esac
runtime_oid_before=$(run_migrator_command psql -X -At -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -c "SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'")
[[ "$runtime_oid_before" =~ ^[0-9]+$ ]] || { echo 'refusing: runtime role OID unavailable' >&2; exit 2; }

if [[ "$PREFLIGHT_ONLY" == '--preflight-only' ]]; then
    echo "preflight-only complete at V$initial_applied; no backup, grant, DDL or migration was performed"
    exit 0
fi

create_verified_backup() {
    local tag="$1" timestamp backup_path backup_tmp backup_hash backup_size
    timestamp=$(date -u +%Y%m%dT%H%M%SZ)
    backup_path="$BACKUP_DIR/espacigo_dms_stage_20261020-${tag}-${timestamp}.dump"
    backup_tmp="$workdir/${tag}.dump"
    [[ ! -e "$backup_path" ]] || { echo "refusing: backup path already exists: $backup_path" >&2; return 2; }
    umask 077
    runuser -u postgres -- env HOME=/var/lib/postgresql PATH=/usr/bin:/bin pg_dump -w -Fc -h "$SOCKET_DIR" -p "$PORT" -U postgres -d "$DATABASE" > "$backup_tmp"
    pg_restore --list "$backup_tmp" >/dev/null
    pg_restore --exit-on-error --file=/dev/null "$backup_tmp"
    chown "$NANDEV_UID:$NANDEV_GID" "$backup_tmp"
    chmod 600 "$backup_tmp"
    mv -- "$backup_tmp" "$backup_path"
    run_nandev pg_restore --list "$backup_path" >/dev/null
    run_nandev pg_restore --exit-on-error --file=/dev/null "$backup_path"
    read -r backup_hash _ < <(sha256sum "$backup_path")
    backup_size=$(stat -c '%s' "$backup_path")
    [[ "$(stat -c '%a %u' "$backup_path")" == "600 $NANDEV_UID" ]] || { echo 'backup mode/owner readback failed' >&2; return 1; }
    echo "PASS: validated $tag backup size=$backup_size mode=600 sha256=$backup_hash path=$backup_path"
}

if [[ "$initial_applied" == 81 ]]; then
    [[ -f "$PRE_V82_BACKUP" && "$(stat -c '%a' "$PRE_V82_BACKUP")" == '600' && "$(stat -c '%u' "$PRE_V82_BACKUP")" == "$NANDEV_UID" ]] || { echo 'refusing: validated post-V81/pre-V82 backup is absent or has unsafe permissions' >&2; exit 2; }
    read -r pre_backup_hash _ < <(sha256sum "$PRE_V82_BACKUP")
    [[ "$pre_backup_hash" == "$PRE_V82_BACKUP_SHA" ]] || { echo 'refusing: post-V81/pre-V82 backup checksum differs from the reviewed receipt' >&2; exit 2; }
    run_nandev pg_restore --list "$PRE_V82_BACKUP" >/dev/null
    run_nandev pg_restore --exit-on-error --file=/dev/null "$PRE_V82_BACKUP"
    echo "PASS: validated pre-V82 backup sha256=$pre_backup_hash"

    cleanup_v83=1
    admin_psql < "$workdir/prepare_native_v82_v83.sql"
    run_migrator "$workdir/migrations-v83"
    admin_psql < "$workdir/finalize_native_v83.sql"
    cleanup_v83=0
    run_migrator_command "$workdir/dbmigrate" -plan-only -dir db/migrations > "$workdir/plan-after-v83.json"
    verify_plan "$workdir/plan-after-v83.json" 83 '84,85'
fi

if [[ "$initial_applied" != 84 ]]; then
    create_verified_backup 'post-v83-pre-v84'
    cleanup_v85=1
    admin_psql < "$workdir/prepare_native_v84_v85.sql"
    run_migrator "$workdir/migrations-v84"
    admin_psql < "$workdir/finalize_native_v85.sql"
    cleanup_v85=0
    run_migrator_command "$workdir/dbmigrate" -plan-only -dir db/migrations > "$workdir/plan-after-v84.json"
    verify_plan "$workdir/plan-after-v84.json" 84 '85'
fi

create_verified_backup 'post-v84-pre-v85'
cleanup_v85=1
admin_psql < "$workdir/prepare_native_v84_v85.sql"
run_migrator db/migrations
admin_psql < "$workdir/finalize_native_v85.sql"
cleanup_v85=0
run_migrator_command "$workdir/dbmigrate" -plan-only -dir db/migrations > "$workdir/plan-after-v85.json"
verify_plan "$workdir/plan-after-v85.json" 85 '-'

runtime_oid_after=$(run_migrator_command psql -X -At -v ON_ERROR_STOP=1 -d "$DATABASE_URL" -c "SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'")
[[ "$runtime_oid_after" == "$runtime_oid_before" ]] || { echo 'postcheck failed: espacigo_runtime OID changed' >&2; exit 1; }

run_migrator_command psql -X -v ON_ERROR_STOP=1 -d "$DATABASE_URL" <<'SQL'
BEGIN READ ONLY;
SET ROLE espacigo_schema_owner;
DO $postcheck$
DECLARE
    target_oid oid;
    count_rows bigint;
    min_version bigint;
    max_version bigint;
    schema_owner_oid oid;
    runtime_oid oid;
    postgres_oid oid;
    public_owner oid;
    reviewer_owner oid;
    function_owner oid;
    table_owner oid;
    membership_count integer;
    public_acl_count integer;
BEGIN
    SELECT oid INTO target_oid FROM pg_catalog.pg_database WHERE datname=current_database();
    SELECT count(*),min(version),max(version) INTO count_rows,min_version,max_version FROM public.schema_migrations;
    SELECT oid INTO schema_owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO runtime_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit;
    SELECT oid INTO postgres_oid FROM pg_catalog.pg_roles WHERE rolname='postgres' AND rolsuper;
    SELECT nspowner INTO public_owner FROM pg_catalog.pg_namespace WHERE nspname='public';
    SELECT relowner INTO reviewer_owner FROM pg_catalog.pg_class WHERE oid='public.document_verification_reviewer_grant'::regclass;
    SELECT proowner INTO function_owner FROM pg_catalog.pg_proc WHERE oid='public.document_verification_space_owner_changed()'::regprocedure;
    SELECT relowner INTO table_owner FROM pg_catalog.pg_class WHERE oid='public.identity_declaration_revision'::regclass;
    SELECT count(*) INTO membership_count FROM pg_catalog.pg_auth_members
     WHERE roleid IN (SELECT oid FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_api_login','espacigo_migrator_login','espacigo_schema_owner','espacigo_document_verification_fn','espacigo_runtime'))
        OR member IN (SELECT oid FROM pg_catalog.pg_roles WHERE rolname IN ('espacigo_api_login','espacigo_migrator_login','espacigo_schema_owner','espacigo_document_verification_fn','espacigo_runtime'));
    SELECT count(*) INTO public_acl_count FROM pg_catalog.pg_class c
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r',c.relowner))) acl
     WHERE c.oid IN ('public.identity_declaration_revision'::regclass,'public.identity_provider_attempt'::regclass,'public.identity_provider_event'::regclass)
       AND acl.grantee=0;
    IF current_database()<>'espacigo_dms_stage_20261020' OR target_oid<>16388 OR current_setting('server_version_num')::integer<>180006
       OR session_user<>'espacigo_migrator_login' OR current_user<>'espacigo_schema_owner'
       OR count_rows<>85 OR min_version<>1 OR max_version<>85
       OR schema_owner_oid IS NULL OR runtime_oid IS NULL OR postgres_oid IS NULL OR public_owner IS NULL
       OR reviewer_owner<>postgres_oid OR function_owner<>postgres_oid OR table_owner<>schema_owner_oid
       OR membership_count<>3 OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members WHERE roleid=schema_owner_oid AND member=postgres_oid)
       OR pg_catalog.has_schema_privilege(schema_owner_oid,'public','CREATE')
       OR pg_catalog.has_column_privilege(schema_owner_oid,'public.usuario','id','REFERENCES')
       OR public_acl_count<>0 THEN
        RAISE EXCEPTION 'native V85 target, ledger, role, owner, ACL or membership postcheck failed';
    END IF;
    IF EXISTS (SELECT 1 FROM public.identity_verification_attempts)
       OR EXISTS (SELECT 1 FROM public.didit_webhook_inbox)
       OR EXISTS (SELECT 1 FROM public.identity_declaration_revision)
       OR EXISTS (SELECT 1 FROM public.identity_provider_attempt)
       OR EXISTS (SELECT 1 FROM public.identity_provider_event) THEN
        RAISE EXCEPTION 'native V85 postcheck found unexpected identity-verification rows';
    END IF;
    IF pg_catalog.has_table_privilege(runtime_oid,'public.identity_verification_attempts','SELECT')
       OR pg_catalog.has_table_privilege(runtime_oid,'public.didit_webhook_inbox','SELECT')
       OR NOT pg_catalog.has_column_privilege(runtime_oid,'public.identity_declaration_revision','ciphertext','SELECT')
       OR NOT pg_catalog.has_column_privilege(runtime_oid,'public.identity_provider_attempt','status','UPDATE')
       OR NOT pg_catalog.has_column_privilege(runtime_oid,'public.identity_provider_event','processing_status','UPDATE')
       OR NOT pg_catalog.has_column_privilege(runtime_oid,'public.document_verification_reviewer_grant','id','UPDATE') THEN
        RAISE EXCEPTION 'native V85 runtime privilege postcheck failed';
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_catalog.pg_attribute a
        CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) acl
        WHERE a.attrelid='public.document_verification_reviewer_grant'::regclass AND a.attname='id'
          AND acl.grantee=runtime_oid AND acl.grantor=postgres_oid AND acl.privilege_type='UPDATE' AND NOT acl.is_grantable
    ) THEN
        RAISE EXCEPTION 'native V83 runtime UPDATE(id) is not directly granted by postgres';
    END IF;
END;
$postcheck$;
ROLLBACK;
SQL

echo 'PASS: native PostgreSQL ledger V1..V85, temporary ACL revocations, role OID, empty identity tables, and runtime column grants verified'
