"""Local-only, reviewed recovery runner for an already-restored database copy.

There is deliberately no archive restore method and no Cloud client in this
module. It normalizes exact reviewed SQL, verifies catalog state, sanitizes,
then create-only copies a fully validated private-files archive.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import subprocess
import sys

from gcp03_origin_private_files import (
    PrivateFile, PrivateFileStore, copy_validated_private_files,
    validate_private_file_references, validate_private_files_tar,
)
from gcp03_origin_recovery_plan import (RecoveryStop, build_normalization_recovery_plan,
    execute_reviewed_recovery)
from gcp03_origin_recovery_preflight import (
    CatalogReader, DIAGNOSTIC_SHA256, EXPECTED_CURRENT,
    NORMALIZATION_SHA256, validate_historical_diagnostic,
)

RUN_ID = "g03o-1017"
REFS_SQL = """SELECT COALESCE(json_agg(file_id::text ORDER BY file_id), '[]'::json)::text FROM (
 SELECT DISTINCT file_id FROM (
 SELECT archivo_id AS file_id FROM public.foto_perfil_sintetica_local WHERE archivo_id IS NOT NULL
 UNION ALL SELECT archivo_id FROM public.espacio_galeria_sintetica_local WHERE archivo_id IS NOT NULL
 UNION ALL SELECT id FROM public.verificacion_evidencia_sintetica
 UNION ALL SELECT archivo_id FROM public.espacio_galeria_archivo_candidato_local
 UNION ALL SELECT archivo_id FROM public.operacion_arriendo_archivo_candidato_local
 UNION ALL SELECT archivo_id FROM public.verificacion_archivo_retiro_pendiente_local
 ) refs) unique_refs"""
POSTCHECK_SQL = """SELECT json_build_object(
 'relations_owned_by_runtime', NOT EXISTS (
   SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f','i','I')
    AND c.relowner<>to_regrole('espacigo_runtime')
    AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)
    AND NOT EXISTS(SELECT 1 FROM pg_index ix JOIN pg_depend d ON d.classid='pg_class'::regclass AND d.objid=ix.indrelid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass WHERE ix.indexrelid=c.oid)
    AND NOT EXISTS(SELECT 1 FROM pg_index ix JOIN pg_constraint con ON con.conindid=ix.indexrelid JOIN pg_depend d ON d.classid='pg_constraint'::regclass AND d.objid=con.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass WHERE ix.indexrelid=c.oid)),
 'routines_owned_by_runtime', NOT EXISTS (SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND p.proowner<>to_regrole('espacigo_runtime') AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),
 'text_search_owned_by_runtime', NOT EXISTS(SELECT 1 FROM pg_ts_dict d JOIN pg_namespace n ON n.oid=d.dictnamespace WHERE n.nspname='public' AND d.dictowner<>to_regrole('espacigo_runtime') AND NOT EXISTS(SELECT 1 FROM pg_depend dep WHERE dep.classid='pg_ts_dict'::regclass AND dep.objid=d.oid AND dep.deptype='e' AND dep.refclassid='pg_extension'::regclass)) AND NOT EXISTS(SELECT 1 FROM pg_ts_config c JOIN pg_namespace n ON n.oid=c.cfgnamespace WHERE n.nspname='public' AND c.cfgowner<>to_regrole('espacigo_runtime') AND NOT EXISTS(SELECT 1 FROM pg_depend dep WHERE dep.classid='pg_ts_config'::regclass AND dep.objid=c.oid AND dep.deptype='e' AND dep.refclassid='pg_extension'::regclass)),
 'sequences_parent_owner_consistent', COALESCE((SELECT count(*)={expected_sequence_dependency_count} AND bool_and(s.relowner=p.relowner) FROM pg_depend d JOIN pg_class s ON s.oid=d.objid JOIN pg_class p ON p.oid=d.refobjid JOIN pg_namespace n ON n.oid=s.relnamespace WHERE d.classid='pg_class'::regclass AND d.refclassid='pg_class'::regclass AND s.relkind='S' AND n.nspname='public' AND d.deptype IN ('a','i')), false),
 'observer_write_privileges_absent', (SELECT count(*) FILTER(WHERE has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'INSERT') OR has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'UPDATE') OR has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'REFERENCES'))=0 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped WHERE n.nspname='public' AND c.relkind IN ('r','p')),
 'runtime_public_create_absent', NOT has_schema_privilege('espacigo_runtime','public','CREATE'))::text"""


def postcheck_sql(expected_sequence_dependency_count: int) -> str:
    if type(expected_sequence_dependency_count) is not int or expected_sequence_dependency_count < 0:
        raise ValueError("owned sequence dependency count must be a non-negative integer")
    return POSTCHECK_SQL.format(expected_sequence_dependency_count=expected_sequence_dependency_count)


class LocalPsqlRecoveryDatabase:
    def __init__(self, *, socket: str, passfile: Path, expected_sequence_dependency_count: int = EXPECTED_CURRENT["owned_sequence_dependency_count"]):
        if not re.fullmatch(r"/[-A-Za-z0-9_./]+", socket):
            raise ValueError("local database socket path is invalid")
        if passfile.is_symlink() or not passfile.is_file() or stat.S_IMODE(passfile.stat().st_mode) != 0o600:
            raise ValueError("local database credential file must be regular mode 0600")
        self.socket, self.passfile = socket, passfile
        self.expected_sequence_dependency_count = expected_sequence_dependency_count

    def _psql(self, sql: bytes | str, *, read_only=False) -> str:
        text = sql.decode("utf-8", "strict") if isinstance(sql, bytes) else sql
        prefix = "BEGIN TRANSACTION READ ONLY;\n" if read_only else ""
        suffix = "\n;\nCOMMIT;" if read_only else ""
        env = {**os.environ, "PGPASSFILE": str(self.passfile)}
        proc = subprocess.run(["psql", "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--host=" + self.socket,
             "--port=5432", "--username=postgres", "--dbname=espacigo_scratch"], input=(prefix+text+suffix).encode(),
             env=env, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False)
        if proc.returncode:
            raise RuntimeError("fixed database command failed")
        return proc.stdout.decode("utf-8", "strict").strip()

    def normalize_owners_and_acls(self, sql: bytes) -> None:
        self._psql(b"BEGIN;\n" + sql + b"\nCOMMIT;")

    def verify_normalized(self) -> dict:
        return json.loads(self._psql(postcheck_sql(self.expected_sequence_dependency_count), read_only=True))

    def sanitize(self) -> dict:
        path = Path(__file__).with_name("gcp03_origin_quarantine_cloud.sql")
        if path.is_symlink() or not path.is_file():
            raise RuntimeError("fixed sanitation SQL is unavailable")
        return json.loads(self._psql(path.read_bytes()))

    def private_file_refs(self) -> set[str]:
        value = json.loads(self._psql("SET LOCAL ROLE espacigo_runtime;\n" + REFS_SQL, read_only=True))
        if not isinstance(value, list) or any(not isinstance(item, str) for item in value):
            raise RuntimeError("private reference report is malformed")
        return set(value)


class LocalCreateOnlyStore:
    def __init__(self, root: Path):
        if root.is_symlink():
            raise ValueError("private output root cannot be a symlink")
        root.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.root = root.resolve(strict=True)

    def create(self, object_name: str, data: bytes, sha256: str) -> None:
        path = PurePosixPath(object_name)
        if path.is_absolute() or any(part in {"", ".", ".."} for part in path.parts):
            raise ValueError("private object name is invalid")
        target = self.root.joinpath(*path.parts)
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        fd = os.open(target, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "wb") as stream:
            stream.write(data)
        if hashlib.sha256(target.read_bytes()).hexdigest() != sha256:
            raise RuntimeError("private copy hash verification failed")


def _read_private(path: Path) -> bytes:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise ValueError("recovery input must be a regular mode 0600 file")
    return path.read_bytes()


def _current_bound_plan(reader, normalization, diagnostic, archive):
    diag = validate_historical_diagnostic(diagnostic, DIAGNOSTIC_SHA256)
    current = reader.identity_and_inventory()
    if current != EXPECTED_CURRENT or hashlib.sha256(normalization).hexdigest() != NORMALIZATION_SHA256:
        raise ValueError("target copy or reviewed artifacts differ from the approved baseline")
    facts, parents = reader.statement_facts(normalization)
    binding = {"historical_diagnostic_sha256": DIAGNOSTIC_SHA256,
        "table_inventory_sha256": current["table_inventory_sha256"],
        "migration_history_sha256": current["migration_history_sha256"],
        "migration_history_sha256_v2": current["migration_history_sha256_v2"],
        "database": current["database"], "login": current["login"],
        "server_version_major": current["server_version_major"],
        "owned_sequence_dependency_count": current["owned_sequence_dependency_count"],
        "historical_owner_sequence_count": diag["owned_sequence_parent_owner_counts"]["dependencies"],
        "private_files_tar_sha256": hashlib.sha256(archive).hexdigest()}
    return build_normalization_recovery_plan(normalization, NORMALIZATION_SHA256, facts,
        owned_sequence_parent_statement=parents, binding=binding)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--socket", required=True)
    parser.add_argument("--passfile", type=Path, required=True)
    parser.add_argument("--normalization", type=Path, required=True)
    parser.add_argument("--diagnostic", type=Path, required=True)
    parser.add_argument("--private-files-tar", type=Path, required=True)
    parser.add_argument("--private-output", type=Path, required=True)
    parser.add_argument("--approved-plan-sha256")
    parser.add_argument("--plan-only", action="store_true")
    args = parser.parse_args()
    normalization = _read_private(args.normalization)
    diagnostic = _read_private(args.diagnostic)
    archive = _read_private(args.private_files_tar)
    validate_historical_diagnostic(diagnostic, DIAGNOSTIC_SHA256)
    if hashlib.sha256(normalization).hexdigest() != NORMALIZATION_SHA256:
        raise SystemExit("reviewed normalization SQL hash mismatch")
    files = validate_private_files_tar(archive, run_id=RUN_ID)
    if args.private_output.is_symlink():
        raise SystemExit("private output directory cannot be a symlink")
    args.private_output.mkdir(mode=0o700, parents=True, exist_ok=True)
    reader = CatalogReader(host=args.socket, port=5432, user="postgres", database="espacigo_scratch", passfile=args.passfile)
    if args.plan_only:
        plan = _current_bound_plan(reader, normalization, diagnostic, archive)
        output = {**plan.summary, "format": "espacigo.g03o.recovery-preflight.v1"}
        fd = os.open(args.private_output / "recovery-plan.json", os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as stream:
            json.dump(output, stream, sort_keys=True, indent=2); stream.write("\n")
        print(json.dumps({"status": "plan_ready", "plan_sha256": plan.summary["plan_sha256"], "execution_performed": False, "report_complete": True}))
        return
    if not args.approved_plan_sha256 or not re.fullmatch(r"[0-9a-f]{64}", args.approved_plan_sha256):
        raise SystemExit("apply requires the separately reviewed plan SHA-256")
    # Recompute the entire plan from current catalog facts and bind the exact
    # private archive before allowing the first database mutation.
    plan = _current_bound_plan(reader, normalization, diagnostic, archive)
    if plan.summary["plan_sha256"] != args.approved_plan_sha256:
        raise SystemExit("current plan differs from approved plan; no mutation started")
    database = LocalPsqlRecoveryDatabase(socket=args.socket, passfile=args.passfile)
    report = execute_reviewed_recovery(plan=plan, approved_plan_sha256=args.approved_plan_sha256,
        database=database, private_files=files,
        private_file_store=LocalCreateOnlyStore(args.private_output), run_id=RUN_ID)
    print(json.dumps(report, sort_keys=True))
    if report.get("report_complete") is not True or report.get("phase_success") is not True:
        raise SystemExit(1)


if __name__ == "__main__":
    main()
