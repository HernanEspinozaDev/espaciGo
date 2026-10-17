#!/usr/bin/env python3
"""Independently verify a sanitized GCP-03 target using a read-only SQL transaction."""
from __future__ import annotations

import argparse
import hashlib
import http.client
import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import sys
import time

FORMAT = "espacigo.gcp03.origin-postrestore-verification.v1"
TARGET_DATABASE = "espacigo_scratch"
TARGET_RUN_ID = "g03o-1017"
SHA_RE = re.compile(r"^[0-9a-f]{64}$")
EXPECTED_EXTENSIONS = {
    "btree_gist": "1.8", "fuzzystrmatch": "1.2", "plpgsql": "1.0",
    "postgis": "3.6.4", "postgis_tiger_geocoder": "3.6.4",
    "postgis_topology": "3.6.4", "unaccent": "1.1",
}


class VerificationError(ValueError):
    pass


READ_ONLY_SQL = r"""
BEGIN TRANSACTION READ ONLY;
SET ROLE espacigo_gcp03_observer;
SELECT json_build_object(
  'database', current_database(),
  'server_version_num', current_setting('server_version_num')::integer,
  'current_user', current_user,
  'current_user_superuser', (SELECT rolsuper FROM pg_roles WHERE rolname=current_user),
  'observer_role_active', pg_has_role(session_user,'espacigo_gcp03_observer','member'),
  'extensions', (SELECT COALESCE(json_object_agg(extname,extversion ORDER BY extname),'{}'::json)
                 FROM pg_extension),
  'public_tables', (SELECT COALESCE(json_agg(c.relname::text ORDER BY c.relname),'[]'::json)
                    FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
                    WHERE n.nspname='public' AND c.relkind IN ('r','p')
                      AND NOT EXISTS (SELECT 1 FROM pg_depend d
                        WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e'
                          AND d.refclassid='pg_extension'::regclass)),
  'migration_history', (SELECT COALESCE(json_agg(json_build_object(
      'version',version,'name',name,'checksum',checksum) ORDER BY version),'[]'::json)
      FROM public.schema_migrations),
  'application_owners_valid', NOT EXISTS (
    SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f','i','I')
      AND c.relowner <> to_regrole('espacigo_runtime')
      AND NOT EXISTS (SELECT 1 FROM pg_depend d
        WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e'
          AND d.refclassid='pg_extension'::regclass)
      AND NOT EXISTS (
        SELECT 1 FROM pg_index ix
        JOIN pg_depend table_extension_member
          ON table_extension_member.classid='pg_class'::regclass
         AND table_extension_member.objid=ix.indrelid
         AND table_extension_member.deptype='e'
         AND table_extension_member.refclassid='pg_extension'::regclass
        WHERE ix.indexrelid=c.oid)
      AND NOT EXISTS (
        SELECT 1 FROM pg_index ix
        JOIN pg_constraint con ON con.conindid=ix.indexrelid
        JOIN pg_depend constraint_extension_member
          ON constraint_extension_member.classid='pg_constraint'::regclass
         AND constraint_extension_member.objid=con.oid
         AND constraint_extension_member.deptype='e'
         AND constraint_extension_member.refclassid='pg_extension'::regclass
        WHERE ix.indexrelid=c.oid))
    AND NOT EXISTS (
      SELECT 1 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
      WHERE n.nspname='public' AND p.proowner <> to_regrole('espacigo_runtime')
        AND NOT EXISTS (SELECT 1 FROM pg_depend d
          WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e'
            AND d.refclassid='pg_extension'::regclass)),
  'valid_sessions', (SELECT count(*) FROM public.sesion
    WHERE revocada_en IS NULL AND expira_en>transaction_timestamp()),
  'usable_action_tokens', (SELECT count(*) FROM public.token_accion
    WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en>transaction_timestamp()),
  'pending_notices', (SELECT count(*) FROM public.aviso_local
    WHERE estado IN ('pendiente','procesando')),
  'identity_event_leases', (SELECT count(*) FROM public.outbox_evento_local
    WHERE lease_hasta IS NOT NULL AND lease_hasta>transaction_timestamp()),
  'runtime_role_login', (SELECT rolcanlogin FROM pg_roles WHERE rolname='espacigo_runtime')
)::text;
COMMIT;
"""


def _read_bound(path: Path, expected_hash: str) -> bytes:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise VerificationError("verification input type or permissions are invalid")
    data = path.read_bytes()
    if not SHA_RE.fullmatch(expected_hash) or hashlib.sha256(data).hexdigest() != expected_hash:
        raise VerificationError("verification input hash mismatch")
    return data


def read_gcs_generation(*, bucket_name: str, object_name: str, generation: int,
                        expected_bytes: int, expected_sha256: str) -> bytes:
    """Read one hash-bound review artifact at one exact object generation."""
    if (bucket_name != f"espacigo-gcp03-origin-{TARGET_RUN_ID}" or
            object_name not in {f"runs/{TARGET_RUN_ID}/toc.list",
                                f"runs/{TARGET_RUN_ID}/normalization.sql"} or
            type(generation) is not int or generation <= 0 or
            type(expected_bytes) is not int or not 0 < expected_bytes <= 1_048_576 or
            not SHA_RE.fullmatch(expected_sha256)):
        raise VerificationError("review artifact identity or bounds are invalid")
    try:
        from google.cloud import storage
        client = storage.Client()
        blob = client.bucket(bucket_name).blob(object_name, generation=generation)
        blob.reload(if_generation_match=generation)
        if int(blob.generation) != generation or int(blob.size) != expected_bytes:
            raise VerificationError("review artifact generation or size mismatch")
        data = blob.download_as_bytes(if_generation_match=generation, checksum="crc32c")
    except VerificationError:
        raise
    except Exception as exc:
        raise VerificationError("review artifact exact-generation read failed") from exc
    if len(data) != expected_bytes or hashlib.sha256(data).hexdigest() != expected_sha256:
        raise VerificationError("review artifact byte count or hash mismatch")
    return data


def wait_for_proxy(*, host: str = "127.0.0.1", port: int = 9801,
                   timeout_seconds: float = 90) -> None:
    deadline = time.monotonic() + timeout_seconds
    while time.monotonic() < deadline:
        try:
            connection = http.client.HTTPConnection(host, port, timeout=2)
            connection.request("GET", "/startup")
            response = connection.getresponse()
            response.read()
            connection.close()
            if response.status == 200:
                return
        except OSError:
            pass
        time.sleep(1)
    raise VerificationError("Cloud SQL Proxy startup timed out")


def shutdown_proxy(*, host: str = "127.0.0.1", port: int = 9091) -> bool:
    """Ask the Cloud Run proxy sidecar to exit; bounded and best effort."""
    try:
        connection = http.client.HTTPConnection(host, port, timeout=3)
        connection.request("POST", "/quitquitquit")
        response = connection.getresponse()
        response.read()
        connection.close()
        return response.status in (200, 202, 204)
    except OSError:
        return False


def current_migration_inventory_sha256() -> str:
    migrations_dir = Path(__file__).resolve().parents[1] / "db" / "migrations"
    records = []
    for version in range(1, 62):
        matches = list(migrations_dir.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1 or matches[0].is_symlink() or not matches[0].is_file():
            raise VerificationError("migration inventory is invalid")
        path = matches[0]
        records.append((version, path.name, hashlib.sha256(path.read_bytes()).hexdigest()))
    body = "".join(f"{version}:{name}:{digest}\n" for version, name, digest in records)
    return hashlib.sha256(body.encode()).hexdigest()


def current_migration_history() -> list[dict[str, object]]:
    migrations_dir = Path(__file__).resolve().parents[1] / "db" / "migrations"
    history = []
    for version in range(1, 62):
        matches = list(migrations_dir.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1 or matches[0].is_symlink() or not matches[0].is_file():
            raise VerificationError("migration inventory is invalid")
        path = matches[0]
        history.append({"version": version, "name": path.name,
                        "checksum": hashlib.sha256(path.read_bytes()).hexdigest()})
    return history


def table_names_from_toc(allowlist: bytes) -> set[str]:
    try:
        lines = allowlist.decode("utf-8", "strict").splitlines()
    except UnicodeDecodeError as exc:
        raise VerificationError("TOC allowlist encoding is invalid") from exc
    tables = set()
    seen_lines = set()
    for line in lines:
        stripped = line.strip()
        if not stripped or stripped.startswith(";"):
            continue
        if ";" not in stripped or stripped in seen_lines:
            raise VerificationError("TOC allowlist entry is malformed or duplicated")
        seen_lines.add(stripped)
        _, body = stripped.split(";", 1)
        try:
            fields = shlex.split(body.strip())
        except ValueError as exc:
            raise VerificationError("TOC allowlist entry is malformed") from exc
        if len(fields) < 3 or not fields[0].isdigit() or not fields[1].isdigit():
            raise VerificationError("TOC allowlist entry is malformed")
        for index, field in enumerate(fields):
            if field == "TABLE" and index + 2 < len(fields) and fields[index + 1] == "public":
                tables.add(fields[index + 2])
                break
    if not tables:
        raise VerificationError("TOC allowlist contains no public tables")
    return tables


def verify_report(report: dict, expected_tables: set[str], *, expected_run_id: str,
                  expected_source_seal_sha256: str, expected_receipt_sha256: str,
                  expected_manifest_sha256: str, expected_toc_sha256: str,
                  expected_normalization_sha256: str, expected_migration_sha256: str,
                  expected_migration_history: list[dict[str, object]] | None = None) -> dict:
    if expected_run_id != TARGET_RUN_ID or any(not SHA_RE.fullmatch(value) for value in (
            expected_source_seal_sha256, expected_receipt_sha256, expected_manifest_sha256,
            expected_toc_sha256, expected_normalization_sha256, expected_migration_sha256)):
        raise VerificationError("reviewed identity or hash is invalid")
    migration_history = expected_migration_history or current_migration_history()
    migration_body = "".join(
        f"{item['version']}:{item['name']}:{item['checksum']}\n" for item in migration_history)
    if hashlib.sha256(migration_body.encode()).hexdigest() != expected_migration_sha256:
        raise VerificationError("migration history does not match its reviewed inventory hash")
    if not isinstance(report, dict):
        raise VerificationError("post-restore report shape is invalid")
    try:
        table_values = report["public_tables"]
        if not isinstance(table_values, list) or any(not isinstance(item, str) for item in table_values):
            raise VerificationError("post-restore table inventory is invalid")
        actual_tables = set(table_values)
        if len(actual_tables) != len(table_values):
            raise VerificationError("post-restore table inventory is duplicated")
        version = int(report["server_version_num"])
    except VerificationError:
        raise
    except (KeyError, TypeError, ValueError) as exc:
        raise VerificationError("post-restore report shape is invalid") from exc
    checks = {
        "database": report.get("database") == TARGET_DATABASE,
        "postgres_version": 180000 <= version < 190000,
        "database_role_is_not_superuser": report.get("current_user_superuser") is False,
        "observer_role_active": report.get("observer_role_active") is True,
        "extensions": report.get("extensions") == EXPECTED_EXTENSIONS,
        "application_tables_match_toc": actual_tables == expected_tables,
        "application_object_owners": report.get("application_owners_valid") is True,
        "sanitized": all(type(report.get(key)) is int and report[key] == 0 for key in (
            "valid_sessions", "usable_action_tokens", "pending_notices", "identity_event_leases")),
        "runtime_role_disabled": report.get("runtime_role_login") is False,
        "migration_history_matches": report.get("migration_history") == migration_history,
    }
    if not all(checks.values()):
        raise VerificationError("post-restore database checks failed")
    return {
        "format": FORMAT, "status": "verified", "target_run_id": expected_run_id,
        "source_sealed_sha256": expected_source_seal_sha256,
        "receipt_sha256": expected_receipt_sha256, "manifest_sha256": expected_manifest_sha256,
        "toc_sha256": expected_toc_sha256, "normalization_sha256": expected_normalization_sha256,
        "migration_inventory_sha256": expected_migration_sha256,
        "public_table_count": len(actual_tables),
        "public_table_inventory_sha256": hashlib.sha256(
            ("\n".join(sorted(actual_tables)) + "\n").encode()).hexdigest(),
        "checks": checks, "database_rows_emitted": False,
    }


def run_verifier(*, toc_path: Path | None = None, toc_bytes: bytes | None = None,
                 expected_toc_sha256: str,
                 normalization_path: Path | None = None, normalization_bytes: bytes | None = None,
                 expected_normalization_sha256: str,
                 expected_migration_sha256: str, expected_run_id: str,
                 expected_source_seal_sha256: str, expected_receipt_sha256: str,
                 expected_manifest_sha256: str, psql: str = "psql",
                 host: str = "127.0.0.1", port: int = 5432,
                 database_user: str = "espacigo_gcp03_observer") -> dict:
    if (toc_bytes is None) == (toc_path is None) or (normalization_bytes is None) == (normalization_path is None):
        raise VerificationError("provide exactly one source for each review artifact")
    if toc_bytes is None:
        toc_bytes = _read_bound(toc_path, expected_toc_sha256)
    elif hashlib.sha256(toc_bytes).hexdigest() != expected_toc_sha256:
        raise VerificationError("TOC allowlist hash mismatch")
    if normalization_bytes is None:
        _read_bound(normalization_path, expected_normalization_sha256)
    elif hashlib.sha256(normalization_bytes).hexdigest() != expected_normalization_sha256:
        raise VerificationError("normalization SQL hash mismatch")
    actual_migration_sha = current_migration_inventory_sha256()
    if actual_migration_sha != expected_migration_sha256:
        raise VerificationError("migration inventory hash mismatch")
    expected_tables = table_names_from_toc(toc_bytes)
    env = os.environ.copy()
    env["PGAPPNAME"] = "gcp03-origin-postrestore-verifier"
    result = subprocess.run([psql, "-X", "-qAt", "--set=ON_ERROR_STOP=1",
        "--host=" + host, "--port=" + str(port), "--username=" + database_user,
        "--dbname=" + TARGET_DATABASE, "-c", READ_ONLY_SQL],
        env=env, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False, timeout=120)
    if result.returncode != 0:
        raise VerificationError("read-only post-restore database query failed")
    try:
        report = json.loads(result.stdout.decode("utf-8", "strict"))
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise VerificationError("post-restore database report is invalid") from exc
    return verify_report(report, expected_tables, expected_run_id=expected_run_id,
        expected_source_seal_sha256=expected_source_seal_sha256,
        expected_receipt_sha256=expected_receipt_sha256,
        expected_manifest_sha256=expected_manifest_sha256,
        expected_toc_sha256=expected_toc_sha256,
        expected_normalization_sha256=expected_normalization_sha256,
        expected_migration_sha256=expected_migration_sha256,
        expected_migration_history=current_migration_history())


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--artifact-bucket", required=True)
    parser.add_argument("--toc-object", required=True)
    parser.add_argument("--toc-generation", type=int, required=True)
    parser.add_argument("--toc-bytes", type=int, required=True)
    parser.add_argument("--expected-toc-sha256", required=True)
    parser.add_argument("--normalization-object", required=True)
    parser.add_argument("--normalization-generation", type=int, required=True)
    parser.add_argument("--normalization-bytes", type=int, required=True)
    parser.add_argument("--expected-normalization-sha256", required=True)
    parser.add_argument("--expected-migration-sha256", required=True)
    parser.add_argument("--expected-run-id", required=True)
    parser.add_argument("--expected-source-seal-sha256", required=True)
    parser.add_argument("--expected-receipt-sha256", required=True)
    parser.add_argument("--expected-manifest-sha256", required=True)
    parser.add_argument("--database-user", required=True,
                        help="Cloud SQL IAM login granted membership in espacigo_gcp03_observer")
    args = parser.parse_args()
    result = None
    try:
        toc_bytes = read_gcs_generation(bucket_name=args.artifact_bucket,
            object_name=args.toc_object, generation=args.toc_generation,
            expected_bytes=args.toc_bytes, expected_sha256=args.expected_toc_sha256)
        normalization_bytes = read_gcs_generation(bucket_name=args.artifact_bucket,
            object_name=args.normalization_object, generation=args.normalization_generation,
            expected_bytes=args.normalization_bytes, expected_sha256=args.expected_normalization_sha256)
        wait_for_proxy()
        result = run_verifier(toc_bytes=toc_bytes,
            expected_toc_sha256=args.expected_toc_sha256,
            normalization_bytes=normalization_bytes,
            expected_normalization_sha256=args.expected_normalization_sha256,
            expected_migration_sha256=args.expected_migration_sha256,
            expected_run_id=args.expected_run_id,
            expected_source_seal_sha256=args.expected_source_seal_sha256,
            expected_receipt_sha256=args.expected_receipt_sha256,
            expected_manifest_sha256=args.expected_manifest_sha256,
            database_user=args.database_user)
        print(json.dumps(result, sort_keys=True))
    except (VerificationError, OSError, ValueError, subprocess.SubprocessError) as exc:
        print(json.dumps({"format": FORMAT, "status": "failed", "reason": str(exc)}, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)
    finally:
        shutdown_proxy()


if __name__ == "__main__":
    main()
