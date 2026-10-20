#!/usr/bin/env python3
"""Guarded one-shot transfer from the retained G03O copy to primary V64.

This entrypoint is deliberately separate from the V1-V61 scratch importer. It
does not create schemas, roles, grants, or migration history. All database
rows are loaded through one PostgreSQL transaction; create-only object copies
are generation-bound and are removed on a failed transaction. Cloud Run is
paused only around the mutation window; after a successful import it stays
paused until a reviewed canonical Terraform release activates the numeric
contract-key version and isolated object prefixes.
"""
from __future__ import annotations

import hashlib
import json
import base64
import os
from pathlib import Path
import re
import secrets
import stat
import subprocess
import tempfile
import time
import socket
from typing import Protocol

from gcp03_origin_cloud_import_executor import (
    GCSObjectStore, TARGET_RUN_ID, decrypt_object, load_transport_key,
    parse_ref, read_exact, validate_manifest_and_receipt,
)
from gcp03_origin_private_file_refs import PRIVATE_FILE_REFS_SQL
import gcp03_origin_cloud_import_executor as source_import
from gcp03_origin_private_files import UUID_RE

PROJECT = "espacigo-testprod"
REGION = "southamerica-west1"
SOURCE_INSTANCE = "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg"
SOURCE_DB = "espacigo_scratch"
TARGET_INSTANCE = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
TARGET_DB = "espacigo"
SOURCE_ARTIFACT_BUCKET = "espacigo-gcp03-origin-g03o-1017"
SOURCE_FILES_BUCKET = "espacigo-gcp03-private-g03o-1017"
TARGET_FILES_BUCKET = "espacigo-test-files-447373903153-gcp02"
DESTINATION_PREFIX = "g03o-1017"
API_SERVICE = "espacigo-test-api"
EXPECTED_CONFIRMATION = f"{TARGET_INSTANCE}/{TARGET_DB}:full-copy:g03o-1017"
TRANSFER_ID = f"g03o-1017-to-primary-v64-{secrets.token_hex(16)}"
FORMAT = "espacigo.g03o.primary-v64-transfer.v1"
EXPECTED_ROWS = {"usuario": 1045, "espacio": 1241, "reserva": 1228}
EXPECTED_FILE_COUNT = 3246
EXPECTED_FILE_BYTES = 808343
SEED_TABLES = (
    "atributo_filtro_publico", "categoria_espacio", "categoria_perfil_atributos",
    "catalogo_ubicacion_publica", "comuna_publica", "region_publica", "version_terminos",
)
OMIT_DATA_TABLES = (*[f"public.{name}" for name in SEED_TABLES],
    "public.schema_migrations", "public.spatial_ref_sys", "public.sesion",
    "public.token_accion", "public.outbox_evento_local", "public.outbox_evento_ciclo_local")
MIGRATIONS = Path(__file__).resolve().parents[1] / "db" / "migrations"
IDENT_RE = re.compile(r"^[a-z_][a-z0-9_]*$")
HEX64_RE = re.compile(r"^[0-9a-f]{64}$")
SQLSTATE_RE = re.compile(rb"^(?:ERROR|FATAL|PANIC):\s*([0-9A-Z]{5})\s*$", re.MULTILINE)
SQLSTATE_CATEGORIES = {
    "08": "connection",
    "22": "data",
    "23": "constraint",
    "25": "transaction_state",
    "28": "authentication",
    "3D": "database",
    "3F": "schema",
    "40": "transaction_rollback",
    "42": "query_or_privilege",
    "53": "resource",
    "55": "object_state",
    "57": "operator",
    "58": "system",
    "XX": "internal",
}
SOURCE_REPORT_MARKER = "__G03O_V64_REPORT_COMPONENT__"
SOURCE_REPORT_COMPONENTS = frozenset({
    "database_identity", "count_usuario", "count_espacio", "count_reserva",
    "migration_history", *(f"seed_{name}" for name in SEED_TABLES),
    "external_sessions", "report_complete",
})
REPORT_COMPONENTS = SOURCE_REPORT_COMPONENTS | {"target_report"}


def sqlstate_category(stderr: bytes) -> str | None:
    """Map psql's SQLSTATE-only diagnostic to a safe, low-detail category."""
    match = SQLSTATE_RE.search(stderr)
    if match is None:
        return None
    return SQLSTATE_CATEGORIES.get(match.group(1)[:2].decode("ascii"), "other")


class TransferStopped(RuntimeError):
    def __init__(self, code: str, stage: str, *, safe_details: dict | None = None):
        super().__init__(code)
        self.code, self.stage = code, stage
        self.safe_details = safe_details or {}


class Database(Protocol):
    def report(self) -> dict: ...
    def file_refs(self) -> set[str]: ...
    def dump_plain_data(self, destination: Path) -> None: ...
    def load_transaction(self, sql_path: Path) -> None: ...


class Files(Protocol):
    def inventory(self, references: set[str]) -> dict[str, dict]: ...
    def destination_inventory(self, names: set[str]) -> dict[str, dict]: ...
    def copy_create_only(self, source: dict, destination_name: str) -> int: ...
    def delete_generation(self, name: str, generation: int) -> None: ...


def migration_inventory(max_version: int) -> list[dict]:
    rows = []
    for version in range(1, max_version + 1):
        matches = list(MIGRATIONS.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1 or matches[0].is_symlink() or not matches[0].is_file():
            raise TransferStopped("MIGRATION_INVENTORY_INVALID", "preflight")
        path = matches[0]
        rows.append({"version": version, "name": path.name,
                     "checksum": hashlib.sha256(path.read_bytes()).hexdigest()})
    return rows


def validate_ledger(actual: object, expected_version: int) -> None:
    expected = migration_inventory(expected_version)
    if not isinstance(actual, list) or len(actual) != expected_version:
        raise TransferStopped("MIGRATION_LEDGER_INVALID", "preflight")
    normalized = []
    for row in actual:
        if not isinstance(row, dict) or type(row.get("version")) is not int:
            raise TransferStopped("MIGRATION_LEDGER_INVALID", "preflight")
        normalized.append({"version": row["version"], "name": row.get("name"), "checksum": row.get("checksum")})
    if normalized != expected:
        raise TransferStopped("MIGRATION_LEDGER_MISMATCH", "preflight")


def map_source_object(name: str) -> str:
    prefix = f"runs/{TARGET_RUN_ID}/"
    if not name.startswith(prefix) or ".." in name.split("/") or "\\" in name:
        raise TransferStopped("SOURCE_OBJECT_PATH_INVALID", "files_preflight")
    relative = name[len(prefix):]
    if relative.endswith(".blob") and UUID_RE.fullmatch(relative[:-5]):
        return f"{DESTINATION_PREFIX}/{relative[:-5].lower()}.blob"
    if relative == "privacy-replay/privacy-replay-v1.json":
        return f"{DESTINATION_PREFIX}/privacy-replay/privacy-replay-v1.json"
    raise TransferStopped("SOURCE_OBJECT_NOT_ALLOWLISTED", "files_preflight")


def validate_source_report(report: dict) -> None:
    if report.get("database") != SOURCE_DB or report.get("postgres_major") != 18 or report.get("report_complete") is not True:
        raise TransferStopped("SOURCE_IDENTITY_MISMATCH", "source_preflight")
    if report.get("counts") != EXPECTED_ROWS:
        raise TransferStopped("SOURCE_DATASET_COUNTS_MISMATCH", "source_preflight")
    validate_ledger(report.get("migration_history"), 61)
    if report.get("seed_fingerprints") is None:
        raise TransferStopped("SOURCE_SEED_REPORT_MISSING", "source_preflight")
    if report.get("external_sessions") != 0:
        raise TransferStopped("SOURCE_NOT_QUIESCENT", "source_preflight")


def validate_target_report(report: dict, source: dict, *, require_quiescent: bool = False) -> None:
    if report.get("database") != TARGET_DB or report.get("postgres_major") != 18 or report.get("report_complete") is not True:
        raise TransferStopped("TARGET_IDENTITY_MISMATCH", "target_preflight")
    validate_ledger(report.get("migration_history"), 64)
    if report.get("schema_migrations_unchanged") is not True:
        raise TransferStopped("TARGET_LEDGER_NOT_CANONICAL", "target_preflight")
    if report.get("nonseed_application_rows") != 0:
        raise TransferStopped("TARGET_NOT_EMPTY", "target_preflight")
    if report.get("seed_fingerprints") != source.get("seed_fingerprints"):
        raise TransferStopped("CANONICAL_SEEDS_DIFFER", "target_preflight")
    if report.get("transfer_role_ready") is not True:
        raise TransferStopped("TARGET_MIGRATOR_ROLE_NOT_READY", "target_preflight")
    if report.get("runtime_role_login") is not False:
        raise TransferStopped("TARGET_RUNTIME_ROLE_UNSAFE", "target_preflight")
    if require_quiescent and report.get("external_sessions") != 0:
        raise TransferStopped("TARGET_NOT_QUIESCENT", "target_preflight")


def validate_file_inventory(source: dict[str, dict], references: set[str]) -> dict[str, dict]:
    if len(source) != EXPECTED_FILE_COUNT or sum(int(item.get("size", -1)) for item in source.values()) != EXPECTED_FILE_BYTES:
        raise TransferStopped("SOURCE_FILE_INVENTORY_MISMATCH", "files_preflight")
    mapped = {}
    evidence_ids = set()
    replay_count = 0
    for name, item in source.items():
        destination = map_source_object(name)
        if destination in mapped or not isinstance(item, dict) or not HEX64_RE.fullmatch(str(item.get("sha256", ""))) or int(item.get("size", -1)) < 0:
            raise TransferStopped("SOURCE_FILE_METADATA_INVALID", "files_preflight")
        mapped[destination] = {**item, "source_name": name}
        if destination.endswith(".blob"):
            evidence_ids.add(destination[:-5])
        else:
            replay_count += 1
    if evidence_ids != {value.lower() for value in references} or replay_count != 1 or len(mapped) != EXPECTED_FILE_COUNT:
        raise TransferStopped("SOURCE_FILE_REFERENCES_MISMATCH", "files_preflight")
    return mapped


def validate_destination_files(expected: dict[str, dict], actual: dict[str, dict]) -> None:
    for name, found in actual.items():
        item = expected.get(name)
        if item is None or found.get("sha256") != item.get("sha256") or int(found.get("size", -1)) != int(item.get("size", -2)) or found.get("crc32c") != item.get("crc32c"):
            raise TransferStopped("DESTINATION_FILE_COLLISION", "files_preflight")


def build_transaction_sql(dump_path: Path, output_path: Path) -> None:
    sanitation_path = Path(__file__).resolve().with_name("gcp03_origin_v64_sanitize.sql")
    sanitation = sanitation_path.read_bytes()
    # V62 materializes the durable replay registry from the copied V1-V61 rows.
    derived = """
INSERT INTO public.registro_replay_baja_local
    (ejecucion_origen_id, solicitud_id, usuario_id, solicitada_en, completada_en)
SELECT x.id, x.solicitud_id, x.usuario_id, s.solicitada_en, x.completada_en
FROM public.ejecucion_baja_local x
JOIN public.solicitud_titular s ON s.id = x.solicitud_id
WHERE x.estado = 'completada' AND x.completada_en IS NOT NULL
ON CONFLICT (ejecucion_origen_id) DO NOTHING;
"""
    lock_public_tables = b"""
SET lock_timeout = '30s';
DO $$
DECLARE item record;
BEGIN
  FOR item IN
    SELECT n.nspname, c.relname
    FROM pg_class c
    JOIN pg_namespace n ON n.oid = c.relnamespace
    WHERE n.nspname = 'public' AND c.relkind IN ('r','p')
      AND c.relowner = (SELECT oid FROM pg_roles WHERE rolname='espacigo_schema_owner')
      AND NOT EXISTS (
        SELECT 1 FROM pg_depend d
        WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e'
      )
    ORDER BY n.nspname, c.relname
  LOOP
    EXECUTE format('LOCK TABLE %I.%I IN SHARE ROW EXCLUSIVE MODE', item.nspname, item.relname);
  END LOOP;
END $$;
"""
    with output_path.open("wb") as out:
        with dump_path.open("rb") as source:
            first = source.readline()
            if first.startswith(b"\\restrict "):
                out.write(first)
                out.write(b"SET ROLE espacigo_schema_owner;\n")
            else:
                out.write(b"SET ROLE espacigo_schema_owner;\n")
                out.write(first)
            out.write(lock_public_tables)
            while True:
                chunk = source.read(1024 * 1024)
                if not chunk:
                    break
                out.write(chunk)
        out.write(b"\n")
        out.write(derived.encode())
        out.write(b"\n")
        out.write(sanitation)
        out.write(b"\n")
        out.write(b"""
DO $$ BEGIN
  IF (SELECT count(*) FROM public.usuario) <> 1045
     OR (SELECT count(*) FROM public.espacio) <> 1241
     OR (SELECT count(*) FROM public.reserva) <> 1228
     OR EXISTS (SELECT 1 FROM public.ejecucion_baja_local x
       JOIN public.solicitud_titular s ON s.id=x.solicitud_id
       WHERE x.estado='completada' AND x.completada_en IS NOT NULL
         AND NOT EXISTS (SELECT 1 FROM public.registro_replay_baja_local r WHERE r.ejecucion_origen_id=x.id))
     OR (SELECT count(*) FROM public.sesion WHERE revocada_en IS NULL AND expira_en > transaction_timestamp()) <> 0
     OR (SELECT count(*) FROM public.token_accion WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en > transaction_timestamp()) <> 0
     OR (SELECT count(*) FROM public.aviso_local WHERE estado IN ('pendiente','procesando')) <> 0
     OR (SELECT count(*) FROM public.outbox_evento_local WHERE lease_hasta IS NOT NULL) <> 0
     OR (SELECT count(*) FROM public.schema_migrations) <> 64 THEN
    RAISE EXCEPTION 'V64 transfer in-transaction postcondition failed';
  END IF;
END $$;
""")
        out.flush()
        os.fsync(out.fileno())
    os.chmod(output_path, 0o600)


class V64Transfer:
    """Orchestrates the reviewed operation through narrow adapters."""
    def __init__(self, *, source_db: Database, target_db: Database, files: Files,
                 api, key_adopter, confirmation: str, execute: bool):
        self.source_db, self.target_db, self.files = source_db, target_db, files
        self.api, self.key_adopter, self.confirmation, self.execute = api, key_adopter, confirmation, execute

    def run(self) -> dict:
        if self.confirmation != EXPECTED_CONFIRMATION:
            raise TransferStopped("TARGET_CONFIRMATION_MISMATCH", "confirmation")
        source = self.source_db.report()
        validate_source_report(source)
        target = self.target_db.report()
        validate_target_report(target, source)
        original_target_ledger = target.get("migration_history")
        source_refs = self.source_db.file_refs()
        expected_files = validate_file_inventory(self.files.inventory(source_refs), source_refs)
        existing = self.files.destination_inventory(set(expected_files))
        validate_destination_files(expected_files, existing)

        if not self.execute:
            return {"format": FORMAT, "status": "preflight_passed", "source_database": SOURCE_DB,
                    "target_database": TARGET_DB, "counts": EXPECTED_ROWS,
                    "private_objects": len(expected_files), "existing_identical_objects": len(existing),
                    "cloud_mutations": False, "source_mutated": False}

        pause = self.api.pause_exact(API_SERVICE, PROJECT, REGION)
        created: list[tuple[str, int]] = []
        committed = False
        key_version = None
        try:
            # Recheck state after workers are stopped; fail closed on any drift.
            source = self.source_db.report()
            target = self.target_db.report()
            validate_source_report(source)
            validate_target_report(target, source, require_quiescent=True)
            for name, item in expected_files.items():
                if name in existing:
                    continue
                generation = self.files.copy_create_only(item, name)
                created.append((name, generation))
            key_version = self.key_adopter.adopt()
            with tempfile.TemporaryDirectory(prefix="g03o-v64-", dir="/dev/shm") as temp:
                os.chmod(temp, 0o700)
                dump = Path(temp) / "rows.sql"
                transaction = Path(temp) / "transaction.sql"
                self.source_db.dump_plain_data(dump)
                build_transaction_sql(dump, transaction)
                self.target_db.load_transaction(transaction)
            committed = True
            final = self.target_db.report()
            if final.get("counts") != EXPECTED_ROWS or final.get("schema_migrations_unchanged") is not True:
                raise TransferStopped("POST_IMPORT_VERIFICATION_FAILED", "postcheck")
            if final.get("external_sessions") != 0:
                raise TransferStopped("POST_IMPORT_TARGET_NOT_QUIESCENT", "postcheck")
            validate_ledger(final.get("migration_history"), 64)
            if final.get("migration_history") != original_target_ledger:
                raise TransferStopped("TARGET_LEDGER_CHANGED", "postcheck")
            if self.target_db.file_refs() != source_refs:
                raise TransferStopped("POST_IMPORT_FILE_REFERENCES_MISMATCH", "postcheck")
            if final.get("seed_fingerprints") != source.get("seed_fingerprints"):
                raise TransferStopped("POST_IMPORT_SEED_FINGERPRINT_MISMATCH", "postcheck")
            return {"format": FORMAT, "status": "transferred_api_paused", "source_database": SOURCE_DB,
                    "target_database": TARGET_DB, "source_connection": SOURCE_INSTANCE,
                    "target_connection": TARGET_INSTANCE, "copied_files": EXPECTED_FILE_COUNT,
                    "file_bytes": EXPECTED_FILE_BYTES, "counts": EXPECTED_ROWS,
                    "target_migration_history_preserved": True, "contract_key_version_added": key_version,
                    "api_resumed": False,
                    "api_action_required": "review and apply the canonical Cloud Run Terraform values for this numeric contract key version and g03o-1017 storage prefixes, then resume the API",
                    "source_mutated": False}
        except Exception as exc:
            code = exc.code if isinstance(exc, TransferStopped) else "TRANSFER_FAILED"
            stage = exc.stage if isinstance(exc, TransferStopped) else "mutation"
            details = dict(exc.safe_details) if isinstance(exc, TransferStopped) else {}
            if key_version:
                details["contract_key_version_added"] = key_version
                if not committed:
                    details["contract_key_version_action_required"] = "the added key version is unused after rollback; reconcile it before retrying"
            elif stage == "contract_key":
                details.setdefault("contract_key_version_outcome", "uncertain; inspect Secret Manager version metadata before retrying")
            if committed:
                details["database_transaction"] = "committed"
                details["api_action_required"] = "API remains paused; inspect committed database and postcheck before resuming"
                raise TransferStopped(code, stage, safe_details=details) from exc
            try:
                after = self.target_db.report()
            except Exception:
                details["database_transaction"] = "uncertain"
                details["api_action_required"] = "API remains paused; database state could not be proven"
                raise TransferStopped("DATABASE_ROLLBACK_UNCONFIRMED", "rollback", safe_details=details) from exc
            if after.get("database") != TARGET_DB or after.get("nonseed_application_rows") != 0:
                details["database_transaction"] = "uncertain"
                details["api_action_required"] = "API remains paused; inspect target before cleanup or resume"
                raise TransferStopped("DATABASE_ROLLBACK_UNCONFIRMED", "rollback", safe_details=details) from exc
            rollback_failed = False
            for name, generation in reversed(created):
                try:
                    self.files.delete_generation(name, generation)
                except Exception:
                    rollback_failed = True
            if rollback_failed:
                details["database_transaction"] = "rolled_back"
                details["api_action_required"] = "API remains paused; private object cleanup is incomplete"
                raise TransferStopped("FILE_ROLLBACK_INCOMPLETE", "rollback", safe_details=details) from exc
            try:
                self.api.resume_exact(API_SERVICE, PROJECT, REGION, {"scaling": pause["scaling"]})
            except Exception as resume_error:
                details["database_transaction"] = "rolled_back"
                details["api_action_required"] = "database rollback and private-file cleanup succeeded, but API resume was not confirmed; inspect the exact service"
                raise TransferStopped("API_RESUME_AFTER_ROLLBACK_FAILED", "api_resume", safe_details=details) from resume_error
            details["database_transaction"] = "rolled_back"
            raise TransferStopped(code, stage, safe_details=details) from exc


class PsqlDatabase:
    """Fixed endpoint PostgreSQL adapter. Passwords are read only into memory."""
    def __init__(self, *, database: str, user: str, password_file: Path, port: int,
                 pg_dump="pg_dump", psql="psql"):
        if database not in {SOURCE_DB, TARGET_DB} or not IDENT_RE.fullmatch(user) or port not in {5432, 5433}:
            raise TransferStopped("DATABASE_ENDPOINT_INVALID", "configuration")
        if (database, user, port) not in {(SOURCE_DB, "postgres", 5432), (TARGET_DB, "espacigo_migrator_login", 5433)}:
            raise TransferStopped("DATABASE_LOGIN_NOT_ALLOWLISTED", "configuration")
        if password_file.is_symlink() or not password_file.is_file() or stat.S_IMODE(password_file.stat().st_mode) not in {0o400, 0o444, 0o600}:
            raise TransferStopped("DATABASE_PASSWORD_FILE_INVALID", "configuration")
        self.database, self.user, self.port, self.pg_dump, self.psql = database, user, port, pg_dump, psql
        password = password_file.read_text().strip()
        if not password or any(c in password for c in "\r\n\x00"):
            raise TransferStopped("DATABASE_PASSWORD_FORMAT_INVALID", "configuration")
        self.passfile = Path("/dev/shm") / ("g03o-v64-pass-" + secrets.token_hex(10))
        fd = os.open(self.passfile, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(fd, "w") as stream:
            escaped = password.replace("\\", "\\\\").replace(":", "\\:")
            stream.write(f"127.0.0.1:{port}:{database}:{user}:{escaped}\n")
            stream.flush(); os.fsync(stream.fileno())
        self.env = {**os.environ, "PGPASSFILE": str(self.passfile), "PGAPPNAME": TRANSFER_ID}

    def close(self):
        try:
            self.passfile.unlink()
        except OSError:
            pass

    def _query(self, sql: str, *, diagnostic_step: str | None = None) -> str:
        query = sql if self.database == SOURCE_DB else "SET ROLE espacigo_schema_owner; " + sql
        proc = subprocess.run([self.psql, "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--set=VERBOSITY=sqlstate", "-h", "127.0.0.1",
            "-p", str(self.port), "-U", self.user, "-d", self.database, "-c", query],
            env=self.env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
        if proc.returncode:
            endpoint = "source" if self.database == SOURCE_DB else "target"
            safe_details = {"database_endpoint": endpoint}
            if diagnostic_step in REPORT_COMPONENTS:
                safe_details["report_component"] = diagnostic_step
            category = sqlstate_category(proc.stderr or b"")
            if category is not None:
                safe_details["sqlstate_category"] = category
            raise TransferStopped("DATABASE_PREFLIGHT_QUERY_FAILED", "preflight", safe_details=safe_details)
        return proc.stdout.decode("utf-8", "strict").strip()

    def report(self) -> dict:
        if self.database == SOURCE_DB:
            script = source_report_script()
            proc = subprocess.run([self.psql, "-X", "-qAt", "--set=ON_ERROR_STOP=1",
                "--set=VERBOSITY=sqlstate", "-h", "127.0.0.1", "-p", str(self.port),
                "-U", self.user, "-d", self.database, "-f", "-"], env=self.env,
                input=script.encode("utf-8"), stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
            if proc.returncode:
                safe_details = {"database_endpoint": "source"}
                component = failed_source_report_component(proc.stdout or b"")
                if component is not None:
                    safe_details["report_component"] = component
                category = sqlstate_category(proc.stderr or b"")
                if category is not None:
                    safe_details["sqlstate_category"] = category
                raise TransferStopped("DATABASE_PREFLIGHT_QUERY_FAILED", "preflight", safe_details=safe_details)
            return parse_source_report_output(proc.stdout)

        raw = self._query(report_sql(self.database), diagnostic_step="target_report")
        try:
            return json.loads(raw)
        except json.JSONDecodeError as exc:
            raise TransferStopped("DATABASE_REPORT_INVALID", "preflight") from exc

    def file_refs(self) -> set[str]:
        raw = self._query(PRIVATE_FILE_REFS_SQL)
        try:
            result = json.loads(raw)
            if not isinstance(result, list) or any(not isinstance(item, str) for item in result):
                raise ValueError
            return {item.lower() for item in result}
        except (json.JSONDecodeError, ValueError) as exc:
            raise TransferStopped("FILE_REFERENCE_REPORT_INVALID", "files_preflight") from exc

    def dump_plain_data(self, destination: Path) -> None:
        args = [self.pg_dump, "--data-only", "--format=plain", "--no-owner", "--no-privileges", "--no-comments", "--serializable-deferrable"]
        args.extend(f"--exclude-table-data={table}" for table in OMIT_DATA_TABLES)
        args.extend(["-h", "127.0.0.1", "-p", str(self.port), "-U", self.user, "-d", self.database])
        with destination.open("wb") as output:
            proc = subprocess.run(args, env=self.env, stdout=output, stderr=subprocess.DEVNULL, check=False)
            output.flush(); os.fsync(output.fileno())
        os.chmod(destination, 0o600)
        if proc.returncode:
            raise TransferStopped("SOURCE_DATA_DUMP_FAILED", "dump")

    def load_transaction(self, sql_path: Path) -> None:
        args = [self.psql, "-X", "-q", "--set=ON_ERROR_STOP=1", "--single-transaction", "-h", "127.0.0.1",
                "-p", str(self.port), "-U", self.user, "-d", self.database]
        with sql_path.open("rb") as source:
            proc = subprocess.run(args, env=self.env, stdin=source, stdout=subprocess.DEVNULL,
                                  stderr=subprocess.DEVNULL, check=False)
        if proc.returncode:
            raise TransferStopped("TARGET_DATA_TRANSACTION_FAILED", "database_transaction")


def source_report_components() -> list[tuple[str, str]]:
    components = [
        ("database_identity", "SELECT jsonb_build_object('database',current_database(),'postgres_major',current_setting('server_version_num')::int / 10000)::text"),
        ("count_usuario", "SELECT jsonb_build_object('counts',jsonb_build_object('usuario',(SELECT count(*) FROM public.usuario)))::text"),
        ("count_espacio", "SELECT jsonb_build_object('counts',jsonb_build_object('espacio',(SELECT count(*) FROM public.espacio)))::text"),
        ("count_reserva", "SELECT jsonb_build_object('counts',jsonb_build_object('reserva',(SELECT count(*) FROM public.reserva)))::text"),
        ("migration_history", "SELECT jsonb_build_object('migration_history',(SELECT jsonb_agg(jsonb_build_object('version',version,'name',name,'checksum',checksum,'runner_checksum',runner_checksum,'applied_at',applied_at) ORDER BY version) FROM public.schema_migrations))::text"),
    ]
    for table in SEED_TABLES:
        components.append((f"seed_{table}",
            "SELECT jsonb_build_object('seed_fingerprints',jsonb_build_object('" + table + "',"
            "md5(COALESCE(string_agg(to_jsonb(t)::text,E'\\n' ORDER BY to_jsonb(t)::text),''))))::text "
            f"FROM public.{table} AS t"))
    components.extend([
        ("external_sessions", "SELECT jsonb_build_object('external_sessions',(SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid()))::text"),
        ("report_complete", "SELECT jsonb_build_object('report_complete',true)::text"),
    ])
    return components


def source_report_script() -> str:
    return "BEGIN READ ONLY;\n" + "\n".join(
        f"\\echo {SOURCE_REPORT_MARKER}{name}\n{query};"
        for name, query in source_report_components()) + "\nCOMMIT;\n"


def failed_source_report_component(stdout: bytes) -> str | None:
    marker = (SOURCE_REPORT_MARKER + "").encode("ascii")
    component = None
    for line in stdout.splitlines():
        if line.startswith(marker):
            candidate = line[len(marker):].decode("ascii", "ignore")
            component = candidate if candidate in SOURCE_REPORT_COMPONENTS else None
    return component


def parse_source_report_output(stdout: bytes) -> dict:
    report: dict = {}
    active_component = None
    for line in stdout.decode("utf-8", "strict").splitlines():
        if line.startswith(SOURCE_REPORT_MARKER):
            active_component = line[len(SOURCE_REPORT_MARKER):]
            if active_component not in SOURCE_REPORT_COMPONENTS:
                raise TransferStopped("DATABASE_REPORT_INVALID", "preflight")
            continue
        if active_component is None:
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise TransferStopped("DATABASE_REPORT_INVALID", "preflight") from exc
        if not isinstance(value, dict):
            raise TransferStopped("DATABASE_REPORT_INVALID", "preflight")
        for key, nested in value.items():
            if key in {"counts", "seed_fingerprints"}:
                report.setdefault(key, {}).update(nested)
            else:
                report[key] = nested
        active_component = None
    if report.get("report_complete") is not True:
        raise TransferStopped("DATABASE_REPORT_INVALID", "preflight")
    return report


def report_sql(database: str) -> str:
    if database != TARGET_DB:
        raise ValueError("report_sql is reserved for the target endpoint")
    excluded = ",".join("'" + name.replace("'", "''") + "'" for name in (*SEED_TABLES, "schema_migrations", "spatial_ref_sys"))
    extra = f"""
      'nonseed_application_rows', (SELECT COALESCE(sum(row_count),0) FROM (
        SELECT ((xpath('/table/row/c/text()', query_to_xml(format('SELECT count(*) AS c FROM %I.%I',n.nspname,c.relname),false,true,'')))[1]::text)::bigint AS row_count
        FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public' AND c.relkind IN ('r','p') AND NOT c.relispartition AND c.relname NOT IN ({excluded})) counts),
      'schema_migrations_unchanged', (SELECT count(*)=64 FROM public.schema_migrations),
      'transfer_role_ready', (SELECT EXISTS(SELECT 1 FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles u ON u.oid=m.member WHERE r.rolname='espacigo_schema_owner' AND u.rolname=session_user AND m.set_option AND NOT m.admin_option AND NOT m.inherit_option)),
      'runtime_role_login', (SELECT rolcanlogin FROM pg_roles WHERE rolname='espacigo_runtime'),
      'external_sessions', (SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND pid<>pg_backend_pid()),
"""
    seed_union = " UNION ALL ".join(
        f"SELECT '{name}'::text AS table_name,to_jsonb(t)::text AS row_text FROM public.{name} AS t"
        for name in SEED_TABLES)
    return """SELECT jsonb_build_object(
      'database', current_database(),
      'postgres_major', current_setting('server_version_num')::int / 10000,
      'counts', jsonb_build_object('usuario',(SELECT count(*) FROM public.usuario),
        'espacio',(SELECT count(*) FROM public.espacio),'reserva',(SELECT count(*) FROM public.reserva)),
      'migration_history',(SELECT jsonb_agg(jsonb_build_object('version',version,'name',name,'checksum',checksum,'runner_checksum',runner_checksum,'applied_at',applied_at) ORDER BY version) FROM public.schema_migrations),
      'seed_fingerprints',(SELECT jsonb_object_agg(table_name, fingerprint) FROM (
        SELECT table_name, md5(COALESCE(string_agg(row_text, E'\\n' ORDER BY row_text),'')) AS fingerprint
        FROM (""" + seed_union + ") seed_rows GROUP BY table_name) seed_fingerprints),\n" + extra + """
      'report_complete', true
    )::text"""


class GCSFiles:
    def __init__(self):
        from google.cloud import storage
        self.client = storage.Client(project=PROJECT)
        self.source = self.client.bucket(SOURCE_FILES_BUCKET)
        self.target = self.client.bucket(TARGET_FILES_BUCKET)

    @staticmethod
    def _metadata(blob) -> dict:
        return {"generation": int(blob.generation), "size": int(blob.size),
                "sha256": (blob.metadata or {}).get("sha256", ""),
                "crc32c": blob.crc32c, "metadata": blob.metadata or {}}

    def inventory(self, references: set[str]) -> dict[str, dict]:
        if len(references) != EXPECTED_FILE_COUNT - 1 or any(not UUID_RE.fullmatch(value) for value in references):
            raise TransferStopped("SOURCE_FILE_REFERENCE_SET_INVALID", "files_preflight")
        names = {f"runs/{TARGET_RUN_ID}/{reference.lower()}.blob" for reference in references}
        names.add(f"runs/{TARGET_RUN_ID}/privacy-replay/privacy-replay-v1.json")
        found = {}
        for name in sorted(names):
            blob = self.source.get_blob(name)
            if blob is not None:
                blob.reload(if_generation_match=int(blob.generation))
                found[blob.name] = self._metadata(blob)
        return found

    def destination_inventory(self, names: set[str]) -> dict[str, dict]:
        found = {}
        # Check only the exact allowlisted destination names. Listing the bucket
        # would require broad object-list permission unrelated to the transfer.
        for name in sorted(names):
            blob = self.target.get_blob(name)
            if blob is not None:
                blob.reload(if_generation_match=int(blob.generation))
                found[blob.name] = self._metadata(blob)
        return found

    def copy_create_only(self, source: dict, destination_name: str) -> int:
        src = self.source.blob(source["source_name"], generation=source["generation"])
        dst = self.target.blob(destination_name)
        dst.metadata = {"sha256": source["sha256"], "transfer_id": TRANSFER_ID,
                        "source_generation": str(source["generation"])}
        token = None
        try:
            while True:
                token, _, _ = dst.rewrite(src, token=token, if_generation_match=0,
                    if_source_generation_match=source["generation"], timeout=120)
                if token is None:
                    break
            dst.reload()
        except Exception as exc:
            # A failed/ambiguous rewrite cannot prove which generation exists.
            # Preserve it for inspection and let the next preflight reconcile it.
            raise TransferStopped("PRIVATE_FILE_COPY_FAILED", "files_copy") from exc
        metadata = dst.metadata or {}
        ours = (metadata.get("transfer_id") == TRANSFER_ID and
                metadata.get("source_generation") == str(source["generation"]))
        if not ours:
            raise TransferStopped("PRIVATE_FILE_COPY_OWNERSHIP_UNCONFIRMED", "files_copy")
        if int(dst.size) != int(source["size"]) or metadata.get("sha256") != source["sha256"] or dst.crc32c != source.get("crc32c"):
            generation = int(dst.generation)
            try:
                dst.delete(if_generation_match=generation)
            except Exception:
                pass
            raise TransferStopped("PRIVATE_FILE_COPY_VERIFY_FAILED", "files_copy")
        return int(dst.generation)

    def delete_generation(self, name: str, generation: int) -> None:
        blob = self.target.blob(name, generation=generation)
        blob.delete(if_generation_match=generation)


class CloudRunAPI:
    def __init__(self):
        import google.auth
        from google.auth.transport.requests import AuthorizedSession
        credentials, _ = google.auth.default(scopes=["https://www.googleapis.com/auth/cloud-platform"])
        self.http = AuthorizedSession(credentials)

    def _url(self, service: str, project: str, region: str) -> str:
        return f"https://run.googleapis.com/v2/projects/{project}/locations/{region}/services/{service}"

    def _operation(self, name: str) -> None:
        url = "https://run.googleapis.com/v2/" + name
        for _ in range(90):
            response = self.http.get(url, timeout=10)
            if response.status_code != 200:
                raise TransferStopped("CLOUD_RUN_OPERATION_FAILED", "api_pause")
            data = response.json()
            if data.get("done"):
                if "error" in data:
                    raise TransferStopped("CLOUD_RUN_OPERATION_FAILED", "api_pause")
                return
            time.sleep(2)
        raise TransferStopped("CLOUD_RUN_OPERATION_TIMEOUT", "api_pause")

    def _get(self, service: str, project: str, region: str) -> dict:
        response = self.http.get(self._url(service, project, region), timeout=15)
        if response.status_code != 200:
            raise TransferStopped("API_SERVICE_LOOKUP_FAILED", "api_pause")
        data = response.json()
        if data.get("name", "").split("/")[-1] != API_SERVICE:
            raise TransferStopped("API_SERVICE_IDENTITY_MISMATCH", "api_pause")
        if any(item.get("tag") for item in data.get("traffic", [])):
            raise TransferStopped("API_SERVICE_HAS_TRAFFIC_TAGS", "api_pause")
        return data

    def pause_exact(self, service: str, project: str, region: str) -> dict:
        data = self._get(service, project, region)
        scaling = data.get("scaling", {})
        if scaling.get("scalingMode", "AUTOMATIC") != "AUTOMATIC" or scaling.get("manualInstanceCount") is not None:
            raise TransferStopped("API_SERVICE_SCALING_NOT_AUTOMATIC", "api_pause")
        prior = {"scaling": scaling}
        try:
            response = self.http.patch(self._url(service, project, region) + "?updateMask=scaling",
                json={"scaling": {"scalingMode": "MANUAL", "manualInstanceCount": 0}}, timeout=20)
            if response.status_code not in {200, 202}:
                raise TransferStopped("API_SERVICE_PAUSE_FAILED", "api_pause")
            body = response.json()
            if body.get("name", "").startswith("projects/"):
                self._operation(body["name"])
            paused = self._get(service, project, region).get("scaling", {})
            if paused.get("scalingMode") != "MANUAL" or paused.get("manualInstanceCount") != 0:
                raise TransferStopped("API_SERVICE_PAUSE_NOT_CONFIRMED", "api_pause")
        except Exception as exc:
            try:
                current = self._get(service, project, region).get("scaling", {})
                if current.get("scalingMode") == "MANUAL" and current.get("manualInstanceCount") == 0:
                    self.resume_exact(service, project, region, prior)
                elif (current.get("scalingMode", "AUTOMATIC") != scaling.get("scalingMode", "AUTOMATIC") or
                      current.get("manualInstanceCount") != scaling.get("manualInstanceCount")):
                    raise TransferStopped("API_SCALING_STATE_UNEXPECTED", "api_pause")
            except Exception as recovery_error:
                raise TransferStopped("API_PAUSE_RECOVERY_UNCONFIRMED", "api_pause",
                    safe_details={"api_action_required": "inspect the exact API service scaling state; it may remain paused"}) from recovery_error
            if isinstance(exc, TransferStopped):
                raise
            raise TransferStopped("API_SERVICE_PAUSE_FAILED", "api_pause") from exc
        return {"scaling": scaling}

    def resume_exact(self, service: str, project: str, region: str, prior: dict) -> None:
        response = self.http.patch(self._url(service, project, region) + "?updateMask=scaling",
            json=prior, timeout=20)
        if response.status_code not in {200, 202}:
            raise TransferStopped("API_SERVICE_RESUME_FAILED", "api_resume")
        body = response.json()
        if body.get("name", "").startswith("projects/"):
            self._operation(body["name"])
        actual = self._get(service, project, region).get("scaling", {})
        expected = prior.get("scaling", {})
        if (actual.get("scalingMode", "AUTOMATIC") != expected.get("scalingMode", "AUTOMATIC") or
                actual.get("manualInstanceCount") != expected.get("manualInstanceCount")):
            raise TransferStopped("API_SERVICE_RESUME_NOT_CONFIRMED", "api_resume")


class ContractKeyAdopter:
    def __init__(self, key_loader):
        self.key_loader = key_loader

    def adopt(self) -> str:
        key = bytearray(self.key_loader())
        try:
            if not re.fullmatch(rb"[0-9a-f]{64}", key):
                raise TransferStopped("CONTRACT_KEY_FORMAT_INVALID", "contract_key")
            import google.auth
            from google.auth.transport.requests import AuthorizedSession
            credentials, _ = google.auth.default(scopes=["https://www.googleapis.com/auth/cloud-platform"])
            session = AuthorizedSession(credentials)
            parent = f"projects/{PROJECT}/secrets/espacigo-test-contract-encryption-key"
            try:
                response = session.post(f"https://secretmanager.googleapis.com/v1/{parent}:addVersion",
                    json={"payload": {"data": base64.b64encode(bytes(key)).decode("ascii")}}, timeout=30)
            except Exception as exc:
                raise TransferStopped("CONTRACT_KEY_ADOPTION_FAILED", "contract_key",
                    safe_details={"contract_key_version_outcome": "uncertain; inspect Secret Manager version metadata before retrying"}) from exc
            if response.status_code != 200:
                raise TransferStopped("CONTRACT_KEY_ADOPTION_FAILED", "contract_key",
                    safe_details={"contract_key_version_outcome": "uncertain; inspect Secret Manager version metadata before retrying"})
            version_resource = response.json()
            name = version_resource.get("name", "")
            match = re.fullmatch(re.escape(parent) + r"/versions/([1-9][0-9]*)", name)
            if not match:
                raise TransferStopped("CONTRACT_KEY_VERSION_INVALID", "contract_key",
                    safe_details={"contract_key_version_outcome": "uncertain; inspect Secret Manager version metadata before retrying"})
            if version_resource.get("state") != "ENABLED":
                raise TransferStopped("CONTRACT_KEY_VERSION_NOT_ENABLED", "contract_key",
                    safe_details={"contract_key_version_added": match.group(1),
                                  "contract_key_version_state": str(version_resource.get("state", "unknown"))})
            return match.group(1)
        finally:
            for index in range(len(key)):
                key[index] = 0


def load_contract_key() -> bytes:
    env = dict(os.environ)
    cfg = source_import.config_from_env(env)
    if cfg.bucket != SOURCE_ARTIFACT_BUCKET or cfg.private_files_bucket != SOURCE_FILES_BUCKET or \
            cfg.confirmation != source_import.EXPECTED_TARGET or cfg.receipt.name != f"runs/{TARGET_RUN_ID}/receipt.json":
        raise TransferStopped("SOURCE_ARTIFACT_CONFIGURATION_INVALID", "contract_key")
    store = GCSObjectStore()
    receipt_data = read_exact(store, cfg.bucket, cfg.receipt)
    if hashlib.sha256(receipt_data).hexdigest() != cfg.expected_receipt_sha256:
        raise TransferStopped("SOURCE_RECEIPT_HASH_MISMATCH", "contract_key")
    receipt = json.loads(receipt_data)
    manifest_ref = parse_ref(receipt.get("transport_manifest"))
    manifest_data = read_exact(store, cfg.bucket, manifest_ref)
    if hashlib.sha256(manifest_data).hexdigest() != cfg.expected_manifest_sha256:
        raise TransferStopped("SOURCE_MANIFEST_HASH_MISMATCH", "contract_key")
    manifest = json.loads(manifest_data)
    refs, _, _ = validate_manifest_and_receipt(receipt, manifest, cfg)
    k_run = load_transport_key(Path(env["GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE"]))
    from io import BytesIO
    sink = BytesIO()
    decrypt_object(store, cfg.bucket, refs["contract-encryption-key"],
        manifest["artifacts"]["contract-encryption-key"], k_run, "contract-encryption-key", sink)
    key = sink.getvalue().strip()
    if not re.fullmatch(rb"[0-9a-f]{64}", key):
        raise TransferStopped("CONTRACT_KEY_FORMAT_INVALID", "contract_key")
    return key


def wait_for_proxy(port: int, timeout: float = 90) -> None:
    deadline = time.monotonic() + timeout
    while True:
        try:
            with socket.create_connection(("127.0.0.1", port), timeout=1):
                return
        except OSError:
            if time.monotonic() >= deadline:
                raise TransferStopped("CLOUD_SQL_PROXY_NOT_READY", "proxy_startup")
            time.sleep(0.5)


class NoKeyMutation:
    def adopt(self):
        raise TransferStopped("PRECHECK_ONLY", "configuration")


def main() -> int:
    source_db = target_db = None
    try:
        confirmation = os.environ.get("GCP03_V64_TRANSFER_CONFIRM_TARGET", "")
        if confirmation != EXPECTED_CONFIRMATION:
            raise TransferStopped("TARGET_CONFIRMATION_MISMATCH", "configuration")
        execute = os.environ.get("GCP03_V64_TRANSFER_EXECUTE") == "YES"
        if execute and not HEX64_RE.fullmatch(os.environ.get("GCP03_V64_TRANSFER_REVIEWED_RELEASE_SHA256", "")):
            raise TransferStopped("REVIEWED_RELEASE_HASH_REQUIRED", "configuration")
        wait_for_proxy(5432)
        wait_for_proxy(5433)
        source_db = PsqlDatabase(database=SOURCE_DB, user="postgres",
            password_file=Path(os.environ["GCP03_V64_SOURCE_PASSWORD_FILE"]), port=5432)
        target_db = PsqlDatabase(database=TARGET_DB, user="espacigo_migrator_login",
            password_file=Path(os.environ["GCP03_V64_TARGET_PASSWORD_FILE"]), port=5433)
        key_adopter = ContractKeyAdopter(load_contract_key) if execute else NoKeyMutation()
        transfer = V64Transfer(source_db=source_db, target_db=target_db, files=GCSFiles(),
            api=CloudRunAPI() if execute else None, key_adopter=key_adopter,
            confirmation=confirmation, execute=execute)
        report = transfer.run()
        print(json.dumps(report, sort_keys=True))
        return 0
    except TransferStopped as exc:
        print(json.dumps({"format": FORMAT, "status": "stopped", "stage": exc.stage,
            "error_code": exc.code, "transaction_outcome": exc.safe_details.get("database_transaction", "not_started"),
            "retry": False, **exc.safe_details}, sort_keys=True))
        return 1
    except Exception:
        print(json.dumps({"format": FORMAT, "status": "stopped", "stage": "unexpected",
            "error_code": "UNEXPECTED_TRANSFER_FAILURE", "transaction_outcome": "not_started", "retry": False}, sort_keys=True))
        return 1
    finally:
        for database in (source_db, target_db):
            if database is not None:
                database.close()


if __name__ == "__main__":
    raise SystemExit(main())
