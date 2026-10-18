#!/usr/bin/env python3
"""Separate hash-bound GCP-03 post-restore recovery entrypoint (no restore path)."""
from __future__ import annotations
import hashlib, io, json, os, re, secrets, stat, subprocess, sys
from pathlib import Path
import gcp03_origin_import as transport
from gcp03_origin_cloud_import_executor import (
    EXPECTED_TARGET, ImportConfig, ImportStopped, ObjectRef, GCSObjectStore,
    GCSPrivateFileStore, TARGET_BUCKET, PRIVATE_FILES_BUCKET, TARGET_RUN_ID,
    parse_ref, read_exact, load_transport_key, decrypt_object,
    validate_manifest_and_receipt, validate_normalization_sql, wait_for_proxy,
    shutdown_proxy,
)
from gcp03_origin_private_files import validate_private_files_tar
from gcp03_origin_recovery_plan import (build_normalization_recovery_plan, execute_reviewed_recovery,
    build_private_file_continuation_plan, execute_private_file_continuation, REQUIRED_SANITATION_COUNTS)
from gcp03_origin_recovery_preflight import (CatalogReader, DIAGNOSTIC_SHA256, EXPECTED_CURRENT,
    CANONICAL_MIGRATION_HISTORY_SHA256_V2, migration_history_hashes)
from gcp03_origin_recovery_runner import REFS_SQL, postcheck_sql
from gcp03_origin_private_file_refs import PrivateFileReferenceQueryError

PLAN_FORMAT = "espacigo.g03o.recovery-plan.v1"
APPLY_FORMAT = "espacigo.g03o.recovery-execution.v1"
U2_MANIFEST_SHA256 = "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30"
U2_NORMALIZATION_SHA256 = "2ca53ad3e92e078c30375c691ca6259197be54f49c4682750ed73dd41d6820b2"
U2_RECEIPT_SHA256 = "42f817e89382af2df3e0f452b594311936e91839fd92cc251f133bd4d867377c"
U2_RECEIPT_GENERATION = 1792273080133875
U2_RECEIPT_BYTES = 1892
F4_IMPORT_FAILURE_SHA256 = "bd4054e1a49eb082467f3c88bba5ef27d3aaec972115962e86e6ab87cb04f349"


def config(env):
    try:
        ref = parse_ref({"object_name": env["GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT"],
            "generation": env["GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION"],
            "sha256": env["GCP03_ORIGIN_IMPORT_RECEIPT_SHA256"],
            "bytes": int(env["GCP03_ORIGIN_IMPORT_RECEIPT_BYTES"])})
        cfg = ImportConfig(env["GCP03_ORIGIN_IMPORT_BUCKET"], env["GCP03_ORIGIN_PRIVATE_FILES_BUCKET"], ref,
            env["GCP03_ORIGIN_IMPORT_RECEIPT_SHA256"], env["GCP03_ORIGIN_IMPORT_MANIFEST_SHA256"],
            env["GCP03_ORIGIN_IMPORT_BOOTSTRAP_SHA256"], env["GCP03_ORIGIN_IMPORT_OBSERVER_SHA256"],
            env["GCP03_ORIGIN_IMPORT_NORMALIZATION_SHA256"], EXPECTED_TARGET,
            Path(env["GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE"]))
        mode = env["GCP03_ORIGIN_RECOVERY_MODE"]
        diag_hash = env["GCP03_ORIGIN_RECOVERY_DIAGNOSTIC_SHA256"]
        import_failure_hash = env["GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256"]
        confirmation = env["GCP03_ORIGIN_IMPORT_CONFIRM_TARGET"]
        instance = env["CLOUD_SQL_INSTANCE_CONNECTION_NAME"]
        private_ip = env["CLOUD_SQL_PRIVATE_IP"]
        password_file = env["DATABASE_PASSWORD_FILE"]
    except (KeyError, ValueError) as exc:
        raise ImportStopped("RECOVERY_CONFIGURATION_INVALID", "configuration") from exc
    expected_instance = EXPECTED_TARGET.rsplit("/", 1)[0]
    hashes = (cfg.expected_receipt_sha256, cfg.expected_manifest_sha256,
              cfg.expected_bootstrap_sha256, cfg.expected_observer_sha256,
              cfg.expected_normalization_sha256, diag_hash)
    if cfg.bucket != TARGET_BUCKET or cfg.private_files_bucket != PRIVATE_FILES_BUCKET or \
       mode not in {"plan", "apply", "continuation-plan", "continuation-copy"} or diag_hash != DIAGNOSTIC_SHA256 or import_failure_hash != F4_IMPORT_FAILURE_SHA256 or \
       any(not re.fullmatch(r"[0-9a-f]{64}", value) for value in hashes) or \
       cfg.expected_manifest_sha256 != U2_MANIFEST_SHA256 or \
       cfg.expected_normalization_sha256 != U2_NORMALIZATION_SHA256 or \
       cfg.expected_receipt_sha256 != U2_RECEIPT_SHA256 or \
       cfg.receipt.generation != U2_RECEIPT_GENERATION or \
       cfg.receipt.size_bytes != U2_RECEIPT_BYTES or \
       confirmation != EXPECTED_TARGET or instance != expected_instance or private_ip != "true" or \
       password_file != "/secrets/admin/password" or \
       str(cfg.transport_key_file) != "/secrets/transport/k_run":
        raise ImportStopped("RECOVERY_CONFIGURATION_INVALID", "configuration")
    if cfg.receipt.name != f"runs/{TARGET_RUN_ID}/receipt.json":
        raise ImportStopped("RECOVERY_CONFIGURATION_INVALID", "configuration")
    return cfg, ref, mode


def authenticated_inputs(cfg, store):
    receipt_data = read_exact(store, cfg.bucket, cfg.receipt)
    if hashlib.sha256(receipt_data).hexdigest() != cfg.expected_receipt_sha256:
        raise ImportStopped("RECEIPT_HASH_MISMATCH", "receipt")
    receipt = json.loads(receipt_data.decode("utf-8", "strict"))
    manifest_ref = parse_ref(receipt.get("transport_manifest"))
    manifest_data = read_exact(store, cfg.bucket, manifest_ref)
    if hashlib.sha256(manifest_data).hexdigest() != cfg.expected_manifest_sha256:
        raise ImportStopped("MANIFEST_HASH_MISMATCH", "manifest")
    manifest = json.loads(manifest_data.decode("utf-8", "strict"))
    refs, _, norm_ref = validate_manifest_and_receipt(receipt, manifest, cfg)
    normalization = read_exact(store, cfg.bucket, norm_ref)
    validate_normalization_sql(normalization, cfg.expected_normalization_sha256)
    private_ref = refs["private-files.tar"]
    key = load_transport_key(cfg.transport_key_file)
    sink = io.BytesIO()
    archive_stats = decrypt_object(store, cfg.bucket, private_ref,
        manifest["artifacts"]["private-files.tar"], key, "private-files.tar", sink)
    archive = sink.getvalue()
    files = validate_private_files_tar(archive, run_id=TARGET_RUN_ID)
    return receipt, manifest, norm_ref, private_ref, archive_stats, files, normalization


def safe_current_drift(expected: dict, observed: dict) -> dict:
    """Return field names and digests only; never expose catalog values."""
    def digest(present, value):
        encoded = json.dumps({"present": present, "value": value}, sort_keys=True,
                             separators=(",", ":"), ensure_ascii=True).encode("ascii")
        return hashlib.sha256(encoded).hexdigest()

    fields = []
    for key in sorted(set(expected) | set(observed)):
        expected_present = key in expected
        observed_present = key in observed
        if expected_present == observed_present and expected.get(key) == observed.get(key):
            continue
        fields.append({"key": key,
            "expected_sha256": digest(expected_present, expected.get(key)),
            "observed_sha256": digest(observed_present, observed.get(key))})
    return {"format": "espacigo.g03o.current-drift.v1", "fields": fields}


def make_plan(reader, normalization, archive_stats, private_ref, cfg, import_failure_sha256):
    current = reader.identity_and_inventory()
    if current != EXPECTED_CURRENT:
        raise ImportStopped("CURRENT_STATE_OR_ARTIFACT_DRIFT", "current_preflight",
                            safe_details={"drift": safe_current_drift(EXPECTED_CURRENT, current)})
    facts, parents = reader.statement_facts(normalization)
    binding = {
        "diagnostic_report_sha256": DIAGNOSTIC_SHA256,
        "import_receipt_sha256": cfg.expected_receipt_sha256,
        "import_manifest_sha256": cfg.expected_manifest_sha256,
        "normalization_sql_sha256": cfg.expected_normalization_sha256,
        "normalization_generation": reader.normalization_generation,
        "private_files_ciphertext_sha256": private_ref.sha256,
        "private_files_generation": private_ref.generation,
        "private_files_plain_sha256": archive_stats["plain_sha256"],
        "table_inventory_sha256": current["table_inventory_sha256"],
        "migration_history_sha256": current["migration_history_sha256"],
        "migration_history_sha256_v2": current["migration_history_sha256_v2"],
        "database": current["database"], "login": current["login"],
        "server_version_major": current["server_version_major"],
        "owned_sequence_dependency_count": current["owned_sequence_dependency_count"],
        "failed_import_report_sha256": import_failure_sha256,
    }
    return build_normalization_recovery_plan(normalization, cfg.expected_normalization_sha256,
        facts, owned_sequence_parent_statement=parents, binding=binding)


class CloudRecoveryDatabase:
    """psql-only adapter; deliberately has no restore/pg_restore method."""
    def __init__(self, password_path):
        if password_path.is_symlink() or not password_path.is_file() or stat.S_IMODE(password_path.stat().st_mode) not in {0o400, 0o444, 0o600}:
            raise ImportStopped("DATABASE_PASSWORD_FILE_INVALID", "database_setup")
        password = password_path.read_text().strip()
        if not password or any(c in password for c in "\r\n\x00"):
            raise ImportStopped("DATABASE_PASSWORD_FORMAT_INVALID", "database_setup")
        self.passfile = Path("/dev/shm") / ("g03-recovery-" + secrets.token_hex(12))
        fd = os.open(self.passfile, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        escaped = password.replace("\\", "\\\\").replace(":", "\\:")
        with os.fdopen(fd, "w") as f:
            f.write(f"127.0.0.1:5432:espacigo_scratch:postgres:{escaped}\n")

    def close(self):
        try: self.passfile.unlink()
        except OSError: pass

    def _psql(self, sql, read_only=False):
        text = sql.decode("utf-8", "strict") if isinstance(sql, bytes) else sql
        env = os.environ.copy(); env["PGPASSFILE"] = str(self.passfile); env["PGAPPNAME"] = "g03o-1017-recovery"
        prefix = "BEGIN TRANSACTION READ ONLY;\n" if read_only else ""
        suffix = "\n;\nCOMMIT;" if read_only else ""
        proc = subprocess.run(["psql", "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--set=VERBOSITY=verbose", "--host=127.0.0.1",
            "--port=5432", "--username=postgres", "--dbname=espacigo_scratch"],
            input=(prefix + text + suffix).encode(), env=env, stdout=subprocess.PIPE,
            stderr=subprocess.PIPE, check=False)
        if proc.returncode:
            match = re.search(rb"\b(?:SQLSTATE|ERROR):?\s*([0-9A-Z]{5})\b", proc.stderr)
            if "verificacion_evidencia_sintetica" in text or "archivo_id" in text:
                raise PrivateFileReferenceQueryError(match.group(1).decode() if match else None)
            raise RuntimeError("fixed database operation failed")
        return proc.stdout.decode("utf-8", "strict").strip()

    def normalize_owners_and_acls(self, sql): self._psql(b"BEGIN;\n" + sql + b"\nCOMMIT;")
    def verify_normalized(self):
        sql = postcheck_sql(EXPECTED_CURRENT["owned_sequence_dependency_count"])
        return json.loads(self._psql(sql, read_only=True))
    def sanitize(self):
        path = Path(__file__).with_name("gcp03_origin_quarantine_cloud.sql")
        if path.is_symlink() or not path.is_file(): raise RuntimeError("fixed sanitation SQL unavailable")
        return json.loads(self._psql(path.read_bytes()))
    def private_file_refs(self):
        sql = "SET LOCAL ROLE espacigo_runtime;\n" + REFS_SQL
        try:
            value = json.loads(self._psql(sql, read_only=True))
        except (json.JSONDecodeError, PrivateFileReferenceQueryError) as exc:
            if isinstance(exc, PrivateFileReferenceQueryError):
                raise
            raise PrivateFileReferenceQueryError() from exc
        if not isinstance(value, list) or any(not isinstance(item, str) for item in value):
            raise PrivateFileReferenceQueryError()
        return set(value)

    def continuation_sanitation_counts(self):
        sql = """SET LOCAL ROLE espacigo_runtime;
SELECT jsonb_build_object(
 'valid_sessions',(SELECT count(*) FROM public.sesion WHERE revocada_en IS NULL AND expira_en>transaction_timestamp()),
 'usable_action_tokens',(SELECT count(*) FROM public.token_accion WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en>transaction_timestamp()),
 'pending_notices',(SELECT count(*) FROM public.aviso_local WHERE estado IN ('pendiente','procesando')),
 'identity_event_leases',(SELECT count(*) FROM public.outbox_evento_local WHERE lease_hasta IS NOT NULL AND lease_hasta>transaction_timestamp()),
 'runtime_role_login',(SELECT rolcanlogin FROM pg_roles WHERE rolname='espacigo_runtime'))::text"""
        return json.loads(self._psql(sql, read_only=True))


def run_private_file_continuation(*, mode, database, reader, files, store=None,
                                  approved_plan_sha256="", run_id=TARGET_RUN_ID):
    """Execute either continuation route; the plan route is strictly read-only."""
    if mode not in {"continuation-plan", "continuation-copy"}:
        raise ImportStopped("CONTINUATION_MODE_INVALID", "continuation_preflight")
    identity = json.loads(database._psql(
        "SELECT json_build_object('session_user',session_user,'current_user',current_user)::text", read_only=True))
    if identity != {"session_user": "postgres", "current_user": "postgres"}:
        raise ImportStopped("CONTINUATION_DATABASE_IDENTITY_FAILED", "continuation_preflight")
    migration_rows = json.loads(reader.query(
        "SET LOCAL ROLE espacigo_runtime;\n"
        "SELECT COALESCE(json_agg(json_build_array(version::text,checksum::text) ORDER BY version), '[]'::json) "
        "FROM public.schema_migrations"))
    if not isinstance(migration_rows, list) or len(migration_rows) != 61:
        raise ImportStopped("CONTINUATION_MIGRATION_HISTORY_FAILED", "continuation_preflight")
    _, migration_history_sha256 = migration_history_hashes(migration_rows)
    if migration_history_sha256 != CANONICAL_MIGRATION_HISTORY_SHA256_V2:
        raise ImportStopped("CONTINUATION_MIGRATION_CHECKSUM_DRIFT", "continuation_preflight")
    role_checks = json.loads(database._psql("""SET LOCAL ROLE espacigo_runtime;
SELECT json_build_object(
 'session_user_is_postgres',session_user='postgres',
 'current_user_is_runtime',current_user='espacigo_runtime',
 'runtime_no_login',(SELECT NOT rolcanlogin AND NOT rolinherit FROM pg_roles WHERE rolname='espacigo_runtime'),
 'postgres_runtime_set_only',(SELECT count(*)=1 AND bool_and(m.set_option AND NOT m.inherit_option AND NOT m.admin_option)
   FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles g ON g.oid=m.member
   WHERE r.rolname='espacigo_runtime' AND g.rolname='postgres'))::text""", read_only=True))
    if not isinstance(role_checks, dict) or any(value is not True for value in role_checks.values()):
        raise ImportStopped("CONTINUATION_ROLE_POSTCHECK_FAILED", "continuation_preflight")
    owner_checks = database.verify_normalized()
    sanitation_counts = database.continuation_sanitation_counts()
    references = database.private_file_refs()
    plan = build_private_file_continuation_plan(references=references, private_files=files,
        run_id=run_id, normalized_checks=owner_checks, sanitation_counts=sanitation_counts,
        migration_history_sha256=migration_history_sha256, role_checks=role_checks)
    if mode == "continuation-plan":
        return {**plan, "phase": mode, "status": "plan_ready", "phase_success": True,
                "report_complete": True, "retry": False}
    if store is None:
        raise ImportStopped("CONTINUATION_COPY_STORE_MISSING", "continuation_preflight")
    return execute_private_file_continuation(plan=plan, approved_plan_sha256=approved_plan_sha256,
        references=references, private_files=files, normalized_checks=owner_checks,
        sanitation_counts=sanitation_counts, run_id=run_id, store=store,
        migration_history_sha256=migration_history_sha256, role_checks=role_checks) | {"phase": mode}


def run(env):
    cfg, receipt_ref, mode = config(env)
    phase = "recover-plan" if mode == "plan" else "recover" if mode == "apply" else mode
    db = None
    try:
        store = GCSObjectStore()
        wait_for_proxy()
        db = CloudRecoveryDatabase(Path(env["DATABASE_PASSWORD_FILE"]))
        receipt, manifest, norm_ref, private_ref, archive_stats, files, normalization = authenticated_inputs(cfg, store)
        reader = CatalogReader(host="127.0.0.1", port=5432, user="postgres", database="espacigo_scratch")
        reader.passfile = db.passfile
        reader.receipt_sha256 = cfg.expected_receipt_sha256
        reader.manifest_sha256 = cfg.expected_manifest_sha256
        reader.normalization_sha256 = cfg.expected_normalization_sha256
        reader.normalization_generation = norm_ref.generation
        if mode in {"continuation-plan", "continuation-copy"}:
            return run_private_file_continuation(mode=mode, database=db, reader=reader,
                files=files, store=GCSPrivateFileStore(cfg.private_files_bucket) if mode == "continuation-copy" else None,
                approved_plan_sha256=env.get("GCP03_ORIGIN_RECOVERY_APPROVED_PLAN_SHA256", ""))
        plan = make_plan(reader, normalization, archive_stats, private_ref, cfg,
            env["GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256"])
        if mode == "plan":
            return {"format": PLAN_FORMAT, "phase": phase, "run_id": TARGET_RUN_ID,
                "status": "plan_ready", "phase_success": True, "report_complete": True,
                "plan_sha256": plan.summary["plan_sha256"],
                "catalog_facts_sha256": plan.summary["catalog_facts_sha256"],
                "normalization_sql_sha256": cfg.expected_normalization_sha256,
                "pending_statement_count": plan.summary["pending_statement_count"],
                "satisfied_statement_count": plan.summary["satisfied_statement_count"],
                "execution_performed": False, "retry": False, "review_plan": plan.summary}
        approved = env.get("GCP03_ORIGIN_RECOVERY_APPROVED_PLAN_SHA256", "")
        if not re.fullmatch(r"[0-9a-f]{64}", approved) or approved != plan.summary["plan_sha256"]:
            return {"format": APPLY_FORMAT, "phase": phase, "run_id": TARGET_RUN_ID,
                "status": "stopped", "phase_success": False, "report_complete": False,
                "stage": "plan_hash_check", "error_code": "CURRENT_PLAN_DIFFERS_FROM_APPROVED_PLAN",
                "transaction_outcome": "not_started", "retry": False}
        result = execute_reviewed_recovery(plan=plan, approved_plan_sha256=approved,
            database=db, private_files=files, private_file_store=GCSPrivateFileStore(cfg.private_files_bucket),
            run_id=TARGET_RUN_ID)
        result["phase"] = phase
        result["catalog_facts_sha256"] = plan.summary["catalog_facts_sha256"]
        result["private_files_ciphertext_sha256"] = plan.summary["binding"]["private_files_ciphertext_sha256"]
        return result
    except ImportStopped as exc:
        is_plan = mode in {"plan", "continuation-plan"}
        result = {"format": "espacigo.g03o.private-file-continuation.v1" if mode.startswith("continuation-") else PLAN_FORMAT if is_plan else APPLY_FORMAT, "phase": phase,
            "run_id": TARGET_RUN_ID, "status": "stopped", "phase_success": False,
            "report_complete": False, "stage": exc.stage, "error_code": exc.code,
            "transaction_outcome": "uncertain" if exc.mutation_started else "not_started", "retry": False}
        result.update(exc.safe_details)
        return result
    except Exception:
        is_plan = mode in {"plan", "continuation-plan"}
        return {"format": "espacigo.g03o.private-file-continuation.v1" if mode.startswith("continuation-") else PLAN_FORMAT if is_plan else APPLY_FORMAT, "phase": phase,
            "run_id": TARGET_RUN_ID, "status": "stopped", "phase_success": False,
            "report_complete": False, "stage": "unexpected", "error_code": "RECOVERY_EXECUTOR_FAILED",
            "transaction_outcome": "not_started", "retry": False}
    finally:
        if db: db.close()
        shutdown_proxy()


def main():
    try:
        report = run(dict(os.environ))
    except ImportStopped as exc:
        mode = os.environ.get("GCP03_ORIGIN_RECOVERY_MODE")
        is_plan = mode in {"plan", "continuation-plan"}
        continuation = mode in {"continuation-plan", "continuation-copy"}
        report = {"format": "espacigo.g03o.private-file-continuation.v1" if continuation else PLAN_FORMAT if is_plan else APPLY_FORMAT,
            "phase": mode if continuation else "recover-plan" if is_plan else "recover", "run_id": TARGET_RUN_ID,
            "status": "stopped", "phase_success": False, "report_complete": False,
            "stage": exc.stage, "error_code": exc.code, "transaction_outcome": "not_started", "retry": False}
    print(json.dumps(report, sort_keys=True))
    if report.get("phase_success") is not True or report.get("report_complete") is not True:
        raise SystemExit(1)


if __name__ == "__main__": main()
