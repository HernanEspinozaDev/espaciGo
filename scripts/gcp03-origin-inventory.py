#!/usr/bin/env python3
"""Read-only inventory for preparing a private GCP-03 origin snapshot.

This tool does not create a dump, copy files/secrets, pause the source, run SQL
mutations, or call Google Cloud. It stores detailed catalog/file fingerprints
only under a new private .local/testing directory and prints aggregate output.
"""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import stat
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path
from typing import Any


SOURCE_PROJECT = "espacigo"
DB_CONTAINER = "espacigo-database-1"
API_CONTAINER = "espacigo-backend-1"
_STAGE = "argument validation"
SQL = r"""
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT jsonb_build_object(
  'database', current_database(),
  'server_version_num', current_setting('server_version_num'),
  'encoding', current_setting('server_encoding'),
  'timezone', current_setting('TimeZone'),
  'database_size_bytes', pg_database_size(current_database()),
  'extensions', (SELECT coalesce(jsonb_agg(jsonb_build_object('name', extname, 'version', extversion) ORDER BY extname), '[]'::jsonb) FROM pg_extension),
  'migration_history', (SELECT coalesce(jsonb_agg(jsonb_build_object('version', version, 'name', name, 'checksum', checksum) ORDER BY version), '[]'::jsonb) FROM public.schema_migrations),
  'roles', (SELECT coalesce(jsonb_agg(jsonb_build_object('name', rolname, 'login', rolcanlogin, 'superuser', rolsuper, 'create_db', rolcreatedb, 'create_role', rolcreaterole, 'replication', rolreplication, 'bypass_rls', rolbypassrls, 'inherit', rolinherit, 'connection_limit', rolconnlimit, 'valid_until_set', rolvaliduntil IS NOT NULL) ORDER BY rolname), '[]'::jsonb) FROM pg_roles),
  'memberships', (SELECT coalesce(jsonb_agg(jsonb_build_object('member', member.rolname, 'granted_role', granted.rolname, 'admin_option', m.admin_option, 'inherit_option', m.inherit_option, 'set_option', m.set_option) ORDER BY member.rolname, granted.rolname), '[]'::jsonb) FROM pg_auth_members m JOIN pg_roles member ON member.oid=m.member JOIN pg_roles granted ON granted.oid=m.roleid),
  'database_owner_acl', (SELECT jsonb_build_object('owner', owner.rolname, 'acl', d.datacl::text) FROM pg_database d JOIN pg_roles owner ON owner.oid=d.datdba WHERE d.datname=current_database()),
  'schema_owners_acls', (SELECT coalesce(jsonb_agg(jsonb_build_object('schema', n.nspname, 'owner', owner.rolname, 'acl', n.nspacl::text) ORDER BY n.nspname), '[]'::jsonb) FROM pg_namespace n JOIN pg_roles owner ON owner.oid=n.nspowner WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'),
  'relation_owners_acls', (SELECT coalesce(jsonb_agg(jsonb_build_object('schema', n.nspname, 'name', c.relname, 'kind', c.relkind::text, 'owner', owner.rolname, 'acl', c.relacl::text) ORDER BY n.nspname,c.relname), '[]'::jsonb) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_roles owner ON owner.oid=c.relowner WHERE c.relkind IN ('r','p','v','m','S','f') AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'),
  'routine_owners_acls', (SELECT coalesce(jsonb_agg(jsonb_build_object('schema', n.nspname, 'name', p.proname, 'identity_args', pg_get_function_identity_arguments(p.oid), 'owner', owner.rolname, 'security_definer', p.prosecdef, 'acl', p.proacl::text) ORDER BY n.nspname,p.proname,pg_get_function_identity_arguments(p.oid)), '[]'::jsonb) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_roles owner ON owner.oid=p.proowner WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'),
  'default_acls', (SELECT coalesce(jsonb_agg(jsonb_build_object('owner', owner.rolname, 'schema', n.nspname, 'object_type', d.defaclobjtype::text, 'acl', d.defaclacl::text) ORDER BY owner.rolname,n.nspname,d.defaclobjtype), '[]'::jsonb) FROM pg_default_acl d JOIN pg_roles owner ON owner.oid=d.defaclrole LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace),
  'relation_inventory', (SELECT coalesce(jsonb_agg(jsonb_build_object('schema', n.nspname, 'name', c.relname, 'kind', c.relkind::text, 'row_estimate', greatest(c.reltuples,0)::bigint, 'total_bytes', pg_total_relation_size(c.oid)) ORDER BY n.nspname,c.relname), '[]'::jsonb) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('r','p','v','m','S','f') AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'),
  'column_type_counts', (SELECT coalesce(jsonb_agg(jsonb_build_object('type', type_label, 'columns', column_count) ORDER BY type_label), '[]'::jsonb) FROM (SELECT CASE WHEN t.typname='uuid' THEN 'uuid' WHEN t.typname='int8' THEN 'int64' WHEN t.typname='int4' THEN 'int32' WHEN t.typname='numeric' THEN 'numeric' WHEN t.typname='jsonb' THEN 'jsonb' WHEN t.typname='bytea' THEN 'bytea' WHEN t.typname IN ('timestamptz','timestamp') THEN 'timestamp' ELSE 'other' END AS type_label, count(*) AS column_count FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_type t ON t.oid=a.atttypid WHERE a.attnum>0 AND NOT a.attisdropped AND c.relkind IN ('r','p','v','m') AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema' GROUP BY 1) types),
  'security_definer_routines', (SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prosecdef AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'),
  'user_trigger_counts', (SELECT jsonb_build_object('total',count(*),'enabled',count(*) FILTER (WHERE t.tgenabled<>'D')) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace WHERE NOT t.tgisinternal AND n.nspname NOT LIKE 'pg_%' AND n.nspname <> 'information_schema'),
  'activity_counts', (SELECT coalesce(jsonb_agg(jsonb_build_object('backend_type', backend_type, 'state', coalesce(state,'unknown'), 'connections', connection_count) ORDER BY backend_type,state), '[]'::jsonb) FROM (SELECT backend_type,state,count(*) AS connection_count FROM pg_stat_activity WHERE datname=current_database() GROUP BY backend_type,state) a),
  'exact_core_counts', jsonb_build_array(
    jsonb_build_object('table','usuario','rows',(SELECT count(*) FROM public.usuario)),
    jsonb_build_object('table','rol_usuario','rows',(SELECT count(*) FROM public.rol_usuario)),
    jsonb_build_object('table','espacio','rows',(SELECT count(*) FROM public.espacio)),
    jsonb_build_object('table','reserva_ensayo_local','rows',(SELECT count(*) FROM public.reserva_ensayo_local)),
    jsonb_build_object('table','ocupacion','rows',(SELECT count(*) FROM public.ocupacion)),
    jsonb_build_object('table','sesion','rows',(SELECT count(*) FROM public.sesion)),
    jsonb_build_object('table','token_accion','rows',(SELECT count(*) FROM public.token_accion)),
    jsonb_build_object('table','outbox_evento_local','rows',(SELECT count(*) FROM public.outbox_evento_local)),
    jsonb_build_object('table','aviso_local','rows',(SELECT count(*) FROM public.aviso_local)),
    jsonb_build_object('table','reserva_pago_ensayo_operacion','rows',(SELECT count(*) FROM public.reserva_pago_ensayo_operacion)),
    jsonb_build_object('table','reserva_pago_evento_ensayo','rows',(SELECT count(*) FROM public.reserva_pago_evento_ensayo)),
    jsonb_build_object('table','contrato_ensayo_local','rows',(SELECT count(*) FROM public.contrato_ensayo_local)),
    jsonb_build_object('table','documento_privado_sintetico_local','rows',(SELECT count(*) FROM public.documento_privado_sintetico_local)),
    jsonb_build_object('table','verificacion','rows',(SELECT count(*) FROM public.verificacion))
  ),
  'lifecycle_counts', jsonb_build_object(
    'sessions', (SELECT jsonb_build_object('total',count(*),'not_revoked',count(*) FILTER (WHERE revocada_en IS NULL),'not_revoked_unexpired',count(*) FILTER (WHERE revocada_en IS NULL AND expira_en>transaction_timestamp())) FROM public.sesion),
    'action_tokens', (SELECT jsonb_build_object('total',count(*),'pending_unconsumed_uninvalidated_unexpired',count(*) FILTER (WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en>transaction_timestamp()),'consumed',count(*) FILTER (WHERE consumido_en IS NOT NULL),'invalidated',count(*) FILTER (WHERE invalidado_en IS NOT NULL)) FROM public.token_accion),
    'identity_outbox', (SELECT jsonb_build_object('total',count(*),'undelivered',count(*) FILTER (WHERE entregada_en IS NULL),'leased',count(*) FILTER (WHERE lease_hasta IS NOT NULL AND lease_hasta>transaction_timestamp())) FROM public.outbox_evento_local),
    'notices_by_state', (SELECT coalesce(jsonb_object_agg(estado,row_count ORDER BY estado),'{}'::jsonb) FROM (SELECT estado,count(*) AS row_count FROM public.aviso_local GROUP BY estado) n),
    'payment_intents_by_state', (SELECT coalesce(jsonb_object_agg(estado,row_count ORDER BY estado),'{}'::jsonb) FROM (SELECT estado,count(*) AS row_count FROM public.reserva_pago_ensayo_operacion GROUP BY estado) p),
    'payment_event_applications_by_state', (SELECT coalesce(jsonb_object_agg(estado,row_count ORDER BY estado),'{}'::jsonb) FROM (SELECT estado,count(*) AS row_count FROM public.reserva_pago_evento_aplicacion_ensayo GROUP BY estado) p),
    'verification', (SELECT jsonb_build_object('total',count(*),'distinct_provider_count',count(DISTINCT proveedor_ref),'non_local_fixture_provider_count',count(*) FILTER (WHERE proveedor_ref<>'local-fixture-v1')) FROM public.verificacion)
  )
)::text;
COMMIT;
"""


def run(args: list[str], *, input_text: str | None = None) -> str:
    try:
        result = subprocess.run(args, input=input_text, text=True, capture_output=True, check=False)
    except OSError as exc:
        raise RuntimeError("required local inspection command is unavailable") from exc
    if result.returncode:
        # Do not echo subprocess stderr: Docker/DB diagnostics may contain local
        # paths or account identifiers. No credentials are ever printed.
        raise RuntimeError("read-only local inspection command failed")
    return result.stdout


def inspect_container(name: str) -> dict[str, Any]:
    data = json.loads(run(["docker", "inspect", name]))
    if len(data) != 1:
        raise RuntimeError("source container selection was ambiguous")
    return data[0]


def compose_env(container: dict[str, Any]) -> dict[str, str]:
    result: dict[str, str] = {}
    for entry in container.get("Config", {}).get("Env", []):
        key, sep, value = entry.partition("=")
        if sep:
            result[key] = value
    return result


def require_source(container: dict[str, Any], service: str) -> None:
    labels = container.get("Config", {}).get("Labels") or {}
    state = container.get("State") or {}
    if labels.get("com.docker.compose.project") != SOURCE_PROJECT:
        raise RuntimeError(f"{service} is not part of the expected local Compose project")
    if labels.get("com.docker.compose.service") != service or not state.get("Running"):
        raise RuntimeError(f"{service} is not the expected running source container")
    if state.get("Health", {}).get("Status") not in (None, "healthy"):
        raise RuntimeError(f"{service} source health is not healthy")


def mount_source(container: dict[str, Any], destination: str) -> Path:
    matches = [m for m in container.get("Mounts", []) if m.get("Destination") == destination]
    if len(matches) != 1 or matches[0].get("Type") != "bind":
        raise RuntimeError("expected source bind mount is absent or ambiguous")
    source = Path(matches[0]["Source"])
    if not source.is_absolute():
        raise RuntimeError("source mount path is not absolute")
    return source


def private_mode(path: Path, expected: int = 0o600) -> int:
    mode = stat.S_IMODE(path.lstat().st_mode)
    if mode != expected:
        raise RuntimeError("private source artifact has an unexpected mode")
    return mode


def sha256_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def inventory_tree(root: Path, label: str, *, require_private: bool = True) -> list[dict[str, Any]]:
    if root.is_symlink() or not root.is_dir():
        raise RuntimeError(f"{label} root must be a real directory")
    if require_private and stat.S_IMODE(root.stat().st_mode) & 0o077:
        raise RuntimeError(f"{label} root is not private")
    entries: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*")):
        info = path.lstat()
        if stat.S_ISLNK(info.st_mode) or not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)):
            raise RuntimeError(f"{label} tree contains a symlink or unsupported file type")
        if stat.S_ISDIR(info.st_mode):
            continue
        mode = stat.S_IMODE(info.st_mode)
        if require_private and mode & 0o077:
            raise RuntimeError(f"{label} contains a non-private file")
        entries.append({
            "relative_path": path.relative_to(root).as_posix(),
            "bytes": info.st_size,
            "mode_octal": f"{mode:04o}",
            "sha256": sha256_file(path),
        })
    return entries


def fingerprint(value: Any) -> str:
    canonical = json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")).encode()
    return hashlib.sha256(canonical).hexdigest()


def output_directory(root: Path, requested: str) -> Path:
    rel = Path(requested)
    if rel.is_absolute() or rel.parts[:2] != (".local", "testing") or ".." in rel.parts:
        raise RuntimeError("private output must remain under .local/testing")
    path = root / rel
    current = root
    for part in rel.parts:
        current = current / part
        if current.exists() and current.is_symlink():
            raise RuntimeError("private output path cannot include a symlink")
    path.mkdir(parents=True, mode=0o700, exist_ok=False)
    os.chmod(path, 0o700)
    return path


def main() -> int:
    global _STAGE
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--database-container", default=DB_CONTAINER)
    parser.add_argument("--api-container", default=API_CONTAINER)
    parser.add_argument("--private-output-dir", required=True, help="new path under .local/testing; never overwritten")
    args = parser.parse_args()

    root = Path(__file__).resolve().parent.parent
    os.umask(0o077)
    _STAGE = "private output directory validation"
    private_dir = output_directory(root, args.private_output_dir)

    _STAGE = "source container validation"
    db_container = inspect_container(args.database_container)
    api_container = inspect_container(args.api_container)
    require_source(db_container, "database")
    require_source(api_container, "backend")
    db_labels = db_container.get("Config", {}).get("Labels") or {}
    api_labels = api_container.get("Config", {}).get("Labels") or {}
    database_compose_root = db_labels.get("com.docker.compose.project.working_dir", "")
    api_compose_root = api_labels.get("com.docker.compose.project.working_dir", "")
    if not database_compose_root or not api_compose_root:
        raise RuntimeError("database or API container is missing its Compose source-root label")
    try:
        backend_source_head = run(["git", "-C", api_compose_root, "rev-parse", "HEAD"]).strip()
        backend_status = run(["git", "-C", api_compose_root, "status", "--porcelain=v1"])
    except RuntimeError as exc:
        raise RuntimeError("API Compose root is not a readable Git worktree") from exc

    env = compose_env(db_container)
    user = env.get("POSTGRES_USER", "")
    database = env.get("POSTGRES_DB", "")
    password_file = env.get("POSTGRES_PASSWORD_FILE", "")
    if not user or not database or not password_file.startswith("/run/secrets/"):
        raise RuntimeError("database container does not expose the expected mounted credential path")

    _STAGE = "read-only database catalog inventory"
    sql = SQL
    report_text = run([
        "docker", "exec", "-i", args.database_container, "sh", "-eu", "-c",
        'password="$(cat "$POSTGRES_PASSWORD_FILE")"; export PGPASSWORD="$password"; unset password; exec psql -h 127.0.0.1 -XqAt -v ON_ERROR_STOP=1 --username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -f -',
    ], input_text=sql)
    lines = [line for line in report_text.splitlines() if line.strip()]
    if len(lines) != 1:
        raise RuntimeError("database inventory query returned an unexpected result shape")
    db_report = json.loads(lines[0])

    _STAGE = "local migration fingerprint comparison"
    migrations: list[dict[str, Any]] = []
    migration_dir = root / "db" / "migrations"
    for path in sorted(migration_dir.glob("V[0-9][0-9][0-9][0-9][0-9][0-9]__*.sql")):
        data = path.read_bytes()
        match = re.fullmatch(r"V([0-9]{6})__(.+)\.sql", path.name)
        if not match:
            raise RuntimeError("migration filename did not match the repository convention")
        migrations.append({
            "version": int(match.group(1)),
            "name": path.name,
            "sha256": hashlib.sha256(data).hexdigest(),
        })
    history = db_report.get("migration_history", [])
    history_by_version = {int(row["version"]): row for row in history}
    local_by_version = {row["version"]: row for row in migrations}
    migration_checks = []
    for version in sorted(set(history_by_version) | set(local_by_version)):
        db_row, file_row = history_by_version.get(version), local_by_version.get(version)
        migration_checks.append({
            "version": version,
            "database_name": db_row.get("name") if db_row else None,
            "database_checksum": db_row.get("checksum") if db_row else None,
            "file_name": file_row.get("name") if file_row else None,
            "file_sha256": file_row.get("sha256") if file_row else None,
            "matches": bool(db_row and file_row and db_row.get("name") == file_row.get("name") and db_row.get("checksum") == file_row.get("sha256")),
        })

    _STAGE = "private file and key fingerprint inventory"
    evidence_root = mount_source(api_container, "/var/lib/espacigo/private-evidence")
    replay_root = mount_source(api_container, "/var/lib/espacigo/privacy-replay")
    tls_root = mount_source(api_container, "/run/database-tls")
    contract_key_path = mount_source(api_container, "/run/secrets/local_contract_encryption_key")
    webhook_path = mount_source(api_container, "/run/secrets/local_payment_webhook_secret")
    api_env = compose_env(api_container)
    replay_container_path = api_env.get("LOCAL_PRIVACY_REPLAY_FILE", "")
    if not replay_container_path.startswith("/var/lib/espacigo/privacy-replay/"):
        raise RuntimeError("privacy replay registry mount does not match the expected source")
    replay_path = replay_root / Path(replay_container_path).name

    evidence_entries = inventory_tree(evidence_root, "private evidence")
    replay_record: dict[str, Any]
    if replay_path.exists():
        if replay_path.is_symlink() or not replay_path.is_file():
            raise RuntimeError("privacy replay registry is not a regular file")
        replay_record = {"present": True, "bytes": replay_path.stat().st_size, "mode_octal": f"{private_mode(replay_path):04o}", "sha256": sha256_file(replay_path)}
    else:
        replay_record = {"present": False}

    if contract_key_path.is_symlink() or not contract_key_path.is_file():
        raise RuntimeError("contract encryption key mount is not a regular file")
    contract_key_record = {"present": True, "bytes": contract_key_path.stat().st_size, "mode_octal": f"{private_mode(contract_key_path):04o}", "sha256": sha256_file(contract_key_path), "content_read_for_sha256_only": True, "content_copied": False, "content_persisted": False}

    tls_entries = inventory_tree(tls_root, "database TLS root bundle", require_private=False)
    credential_metadata = []
    for label, path, read_for_auth in (("db_admin_password", mount_source(db_container, "/run/secrets/db_admin_password"), True), ("runtime_password", mount_source(api_container, "/run/secrets/runtime_password"), False), ("payment_webhook_secret", webhook_path, False)):
        if path.is_symlink() or not path.is_file():
            raise RuntimeError("a mounted source credential is not a regular file")
        credential_metadata.append({"kind": label, "present": True, "bytes": path.stat().st_size, "mode_octal": f"{private_mode(path):04o}", "read_in_memory_for_read_only_database_authentication": read_for_auth, "copied_or_persisted": False, "logged": False})

    migration_ok = bool(migrations) and len(migrations) == 61 and len(history) == 61 and all(row["matches"] for row in migration_checks) and [row["version"] for row in migrations] == list(range(1, 62))
    file_fingerprint = fingerprint({"evidence": evidence_entries, "privacy_replay": replay_record, "tls": tls_entries, "contract_key": contract_key_record})
    role_acl = {
        "roles": db_report.get("roles", []),
        "memberships": db_report.get("memberships", []),
        "database_owner_acl": db_report.get("database_owner_acl"),
        "schema_owners_acls": db_report.get("schema_owners_acls", []),
        "relation_owners_acls": db_report.get("relation_owners_acls", []),
        "routine_owners_acls": db_report.get("routine_owners_acls", []),
        "default_acls": db_report.get("default_acls", []),
    }
    role_acl_fingerprint = fingerprint(role_acl)
    migration_fingerprint = fingerprint(migration_checks)

    private_manifest = {
        "schema": "espacigo.gcp03.origin-snapshot-prep.private.v1",
        "observed_at_utc": datetime.now(timezone.utc).isoformat(),
        "git_head": run(["git", "rev-parse", "HEAD"]).strip(),
        "scope": {
            "compose_project": SOURCE_PROJECT,
            "database_container_image": db_container.get("Config", {}).get("Image"),
            "database_container_health": db_container.get("State", {}).get("Health", {}).get("Status"),
            "database_and_backend_compose_roots_match": os.path.realpath(database_compose_root) == os.path.realpath(api_compose_root),
            "backend_source_head_from_compose_worktree": backend_source_head,
            "backend_compose_worktree_has_local_changes": bool(backend_status.strip()),
            "database_volume_names": sorted(m.get("Name", "") for m in db_container.get("Mounts", []) if m.get("Type") == "volume"),
            "source_containers_were_running_during_observation": True,
            "tool_performed_no_source_mutations_or_service_stops": True,
            "private_artifacts_written_under_local_testing": True,
            "sql_mutations_performed_by_tool": False,
            "dump_created": False,
            "source_files_copied": False,
            "secret_values_written": False,
            "database_admin_credential_used_ephemerally": True,
            "source_credentials_persisted_or_logged": False,
            "google_cloud_calls": False,
            "database_observation_is_one_repeatable_read_read_only_transaction": True,
            "database_and_files_observed_at_one_atomic_point": False,
        },
        "database": db_report,
        "repository_migrations": migrations,
        "migration_comparison": {"count_matches_v1_v61": migration_ok, "rows": migration_checks, "aggregate_sha256": migration_fingerprint},
        "files": {
            "private_evidence": evidence_entries,
            "privacy_replay_registry": replay_record,
            "database_tls_files": tls_entries,
            "contract_encryption_key_fingerprint_only": contract_key_record,
            "source_credentials_metadata_only": credential_metadata,
            "aggregate_sha256": file_fingerprint,
        },
        "role_acl_fingerprint_sha256": role_acl_fingerprint,
    }
    out = private_dir / "origin-private-inventory.json"
    _STAGE = "private manifest write"
    fd = os.open(out, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(private_manifest, stream, ensure_ascii=False, sort_keys=True, indent=2)
        stream.write("\n")
    os.chmod(out, 0o600)

    exact_counts = {row["table"]: int(row["rows"]) for row in db_report["exact_core_counts"]}
    print(json.dumps({
        "schema": "espacigo.gcp03.origin-snapshot-prep.aggregate.v1",
        "observed_at_utc": private_manifest["observed_at_utc"],
        "git_head": private_manifest["git_head"],
        "database_and_backend_compose_roots_match": private_manifest["scope"]["database_and_backend_compose_roots_match"],
        "backend_source_head_from_compose_worktree": backend_source_head,
        "postgres_server_version_num": db_report["server_version_num"],
        "database_size_bytes": db_report["database_size_bytes"],
        "extensions": db_report["extensions"],
        "migration_history_rows": len(history),
        "migration_v1_v61_match_local_bytes": migration_ok,
        "public_relation_count": len(db_report["relation_inventory"]),
        "postgres_role_count": len(db_report["roles"]),
        "postgres_membership_count": len(db_report["memberships"]),
        "security_definer_routine_count": db_report["security_definer_routines"],
        "uuid_columns": next((int(row["columns"]) for row in db_report["column_type_counts"] if row["type"] == "uuid"), 0),
        "int64_columns": next((int(row["columns"]) for row in db_report["column_type_counts"] if row["type"] == "int64"), 0),
        "core_row_counts": exact_counts,
        "side_effect_state_counts": db_report["lifecycle_counts"],
        "private_evidence_file_count": len(evidence_entries),
        "file_inventory_sha256": file_fingerprint,
        "role_acl_fingerprint_sha256": role_acl_fingerprint,
        "private_manifest_written": True,
        "dump_or_copy_created": False,
        "database_admin_credential_used_ephemerally": True,
        "credential_values_copied_or_logged": False,
        "tool_performed_no_source_mutations_or_service_stops": True,
        "cloud_touched": False,
    }, sort_keys=True))
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (RuntimeError, OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
        detail = f": {exc}" if isinstance(exc, RuntimeError) else ""
        print(f"GCP-03 origin inventory stopped safely at {_STAGE} ({type(exc).__name__}){detail}; no subprocess output or source values were emitted.", file=sys.stderr)
        raise SystemExit(2)
