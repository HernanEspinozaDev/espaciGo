#!/usr/bin/env python3
"""One-shot, hash-bound Cloud SQL importer for the sealed GCP-03 origin.

Cloud calls are limited to reading exact GCS object generations. Database changes
are delegated to fixed pg_restore/psql commands over a separately configured
Cloud SQL Auth Proxy sidecar. This program never starts an application service.
"""

from __future__ import annotations

import hashlib
import hmac
import io
import json
import os
from pathlib import Path
import re
import secrets
import socket
import stat
import subprocess
import sys
import threading
import time
from dataclasses import dataclass
from typing import BinaryIO, Callable, Protocol

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

import gcp03_origin_import as transport
from gcp03_origin_private_files import copy_private_files

SOURCE_CAPTURE_RUN_ID = "gcp03-merge-review-20261017"
TARGET_RUN_ID = "g03o-1017"
SOURCE_SEALED_SHA256 = "80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262"
TARGET_DATABASE = "espacigo_scratch"
TARGET_BUCKET = "espacigo-gcp03-origin-g03o-1017"
PRIVATE_FILES_BUCKET = "espacigo-gcp03-private-g03o-1017"
EXPECTED_TARGET = "espacigo-testprod:southamerica-west1:espacigo-gcp03-origin-g03o-1017-pg/espacigo_scratch"
FORMAT = "espacigo.gcp03.origin-cloud-import.v1"
RECEIPT_FORMAT = "espacigo.gcp03.origin-upload-receipt.v1"
ARTIFACTS = {"database.dump", "private-files.tar", "contract-encryption-key", "origin-metadata.json"}
SHA_RE = re.compile(r"^[0-9a-f]{64}$")
GENERATION_RE = re.compile(r"^[1-9][0-9]*$")
MAX_SMALL_OBJECT_BYTES = 4 * 1024 * 1024
DISALLOWED_TOC = (" EXTENSION - ", " COMMENT - EXTENSION ", " TABLE DATA public spatial_ref_sys ")
DISALLOWED_NORMALIZATION_SQL = re.compile(
    r"\b(PASSWORD|SUPERUSER|CREATEDB|CREATEROLE|REPLICATION|BYPASSRLS|DROP|CREATE\s+(TABLE|DATABASE|EXTENSION))\b",
    re.IGNORECASE,
)


class ImportStopped(RuntimeError):
    def __init__(self, code: str, stage: str, *, mutation_started: bool = False):
        super().__init__(code)
        self.code = code
        self.stage = stage
        self.mutation_started = mutation_started


@dataclass(frozen=True)
class ObjectRef:
    name: str
    generation: int
    sha256: str
    size_bytes: int


@dataclass(frozen=True)
class Download:
    generation: int
    stream: BinaryIO


class ObjectStore(Protocol):
    def open(self, bucket: str, ref: ObjectRef) -> Download: ...


class DatabaseRunner(Protocol):
    def preflight(self) -> dict: ...
    def list_toc(self, feed_archive: Callable[[BinaryIO], dict]) -> list[str]: ...
    def restore(self, feed_archive: Callable[[BinaryIO], dict], allowlist: bytes) -> None: ...
    def normalize_owners_and_acls(self, sql: bytes) -> None: ...
    def sanitize(self) -> dict: ...


class HashingReader:
    def __init__(self, stream: BinaryIO):
        self.stream = stream
        self.digest = hashlib.sha256()
        self.size = 0

    def read(self, size: int = -1) -> bytes:
        data = self.stream.read(size)
        self.digest.update(data)
        self.size += len(data)
        return data


def parse_ref(value: dict) -> ObjectRef:
    if not isinstance(value, dict):
        raise ImportStopped("OBJECT_REF_INVALID", "receipt")
    name, generation, checksum, size = value.get("object_name"), value.get("generation"), value.get("sha256"), value.get("bytes")
    if not isinstance(name, str) or not name or name.startswith("/") or ".." in name.split("/") or \
            not isinstance(generation, (str, int)) or not GENERATION_RE.fullmatch(str(generation)) or \
            not isinstance(checksum, str) or not SHA_RE.fullmatch(checksum) or \
            type(size) is not int or size <= 0 or "\x00" in name or "\\" in name:
        raise ImportStopped("OBJECT_REF_INVALID", "receipt")
    return ObjectRef(name, int(generation), checksum, size)


def read_exact(store: ObjectStore, bucket: str, ref: ObjectRef, limit: int = MAX_SMALL_OBJECT_BYTES) -> bytes:
    if ref.size_bytes > limit:
        raise ImportStopped("SMALL_OBJECT_TOO_LARGE", "object_read")
    download = store.open(bucket, ref)
    try:
        if download.generation != ref.generation:
            raise ImportStopped("OBJECT_GENERATION_MISMATCH", "object_read")
        data = download.stream.read(limit + 1)
    finally:
        download.stream.close()
    if len(data) != ref.size_bytes or hashlib.sha256(data).hexdigest() != ref.sha256:
        raise ImportStopped("OBJECT_HASH_OR_SIZE_MISMATCH", "object_read")
    return data


def unwrap_dek(k_run: bytes, item: dict, artifact: str) -> bytes:
    nonce = bytes.fromhex(item["wrap_nonce_hex"])
    wrapped = bytes.fromhex(item["wrapped_dek_hex"])
    aad = f"{transport.FORMAT}:{TARGET_RUN_ID}:{artifact}:dek".encode()
    try:
        return AESGCM(k_run).decrypt(nonce, wrapped, aad)
    except Exception as exc:
        raise ImportStopped("DEK_AUTHENTICATION_FAILED", "key_unwrap") from exc


def decrypt_object(store: ObjectStore, bucket: str, ref: ObjectRef, manifest_item: dict,
                   k_run: bytes, artifact: str, sink: BinaryIO) -> dict:
    download = store.open(bucket, ref)
    if download.generation != ref.generation:
        download.stream.close()
        raise ImportStopped("OBJECT_GENERATION_MISMATCH", "artifact_read")
    source = HashingReader(download.stream)
    try:
        dek = unwrap_dek(k_run, manifest_item, artifact)
        stats = transport.decrypt_chunks(source, sink, dek, TARGET_RUN_ID, artifact)
    except Exception as exc:
        raise ImportStopped("ARTIFACT_AUTHENTICATION_FAILED", "artifact_decrypt") from exc
    finally:
        download.stream.close()
    if source.digest.hexdigest() != ref.sha256 or source.size != ref.size_bytes or \
            source.digest.hexdigest() != manifest_item.get("transport_cipher_sha256") or \
            source.size != manifest_item.get("transport_cipher_bytes") or \
            stats["plain_sha256"] != manifest_item.get("plain_sha256") or \
            stats["plain_bytes"] != manifest_item.get("plain_bytes") or \
            stats["chunks"] != manifest_item.get("chunk_count"):
        raise ImportStopped("ARTIFACT_HASH_OR_SIZE_MISMATCH", "artifact_verify")
    return stats


@dataclass(frozen=True)
class ImportConfig:
    bucket: str
    private_files_bucket: str
    receipt: ObjectRef
    expected_receipt_sha256: str
    expected_manifest_sha256: str
    expected_bootstrap_sha256: str
    expected_observer_sha256: str
    expected_normalization_sha256: str
    confirmation: str
    transport_key_file: Path


def load_transport_key(path: Path) -> bytes:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) not in {0o400, 0o444, 0o600}:
        raise ImportStopped("TRANSPORT_KEY_FILE_INVALID", "key_load")
    raw = path.read_bytes().strip()
    if not re.fullmatch(rb"[0-9a-f]{64}", raw):
        raise ImportStopped("TRANSPORT_KEY_FORMAT_INVALID", "key_load")
    return bytes.fromhex(raw.decode("ascii"))


def validate_manifest_and_receipt(receipt: dict, manifest: dict, config: ImportConfig) -> tuple[dict[str, ObjectRef], ObjectRef, ObjectRef]:
    if receipt.get("format") != RECEIPT_FORMAT or \
            receipt.get("source_capture_run_id") != SOURCE_CAPTURE_RUN_ID or \
            receipt.get("target_run_id") != TARGET_RUN_ID or \
            receipt.get("source_sealed_sha256") != SOURCE_SEALED_SHA256 or \
            receipt.get("bootstrap_report_sha256") != config.expected_bootstrap_sha256 or \
            receipt.get("observer_report_sha256") != config.expected_observer_sha256 or \
            receipt.get("normalization_sql_sha256") != config.expected_normalization_sha256:
        raise ImportStopped("RECEIPT_IDENTITY_OR_EVIDENCE_MISMATCH", "receipt")
    if manifest.get("format") != transport.FORMAT or \
            manifest.get("source_capture_run_id") != SOURCE_CAPTURE_RUN_ID or \
            manifest.get("target_run_id") != TARGET_RUN_ID or \
            manifest.get("source_sealed_sha256") != SOURCE_SEALED_SHA256:
        raise ImportStopped("MANIFEST_IDENTITY_MISMATCH", "manifest")
    contract = manifest.get("import_contract") or {}
    if contract.get("format") != "espacigo.gcp03.origin-import-contract.v1" or \
            contract.get("target_database") != TARGET_DATABASE or contract.get("requires_empty_target") is not True or \
            contract.get("restore_attempts") != 1 or contract.get("automatic_retries") != 0 or \
            contract.get("application_services_and_workers_may_start") is not False or \
            contract.get("sanitize_copy_before_verification") is not True or \
            contract.get("source_mutation_allowed") is not False or contract.get("cloud_upload_performed") is not False:
        raise ImportStopped("IMPORT_CONTRACT_INVALID", "manifest")
    object_items = receipt.get("artifacts")
    if not isinstance(object_items, dict) or set(object_items) != ARTIFACTS or \
            not isinstance(manifest.get("artifacts"), dict) or set(manifest["artifacts"]) != ARTIFACTS:
        raise ImportStopped("ARTIFACT_ALLOWLIST_MISMATCH", "receipt")
    refs = {name: parse_ref(value) for name, value in object_items.items()}
    for name, ref in refs.items():
        item = manifest["artifacts"][name]
        if ref.name != f"runs/{TARGET_RUN_ID}/" + Path(item.get("object_name", "")).name or \
                ref.sha256 != item.get("transport_cipher_sha256") or ref.size_bytes != item.get("transport_cipher_bytes"):
            raise ImportStopped("ARTIFACT_RECEIPT_MISMATCH", "receipt")
    allowlist_ref = parse_ref(receipt.get("toc_allowlist"))
    normalization_ref = parse_ref(receipt.get("normalization_sql"))
    if normalization_ref.sha256 != config.expected_normalization_sha256 or not normalization_ref.name.startswith(f"runs/{TARGET_RUN_ID}/"):
        raise ImportStopped("NORMALIZATION_PLAN_HASH_MISMATCH", "receipt")
    if not allowlist_ref.name.startswith(f"runs/{TARGET_RUN_ID}/"):
        raise ImportStopped("TOC_ALLOWLIST_OBJECT_PATH_INVALID", "receipt")
    return refs, allowlist_ref, normalization_ref


def validate_toc(available_text: str, allowlist_data: bytes) -> list[str]:
    try:
        available_lines = [line.strip() for line in available_text.splitlines() if line.strip() and not line.lstrip().startswith(";")]
        selected_lines = [line.strip() for line in allowlist_data.decode("utf-8").splitlines() if line.strip() and not line.lstrip().startswith(";")]
    except UnicodeDecodeError as exc:
        raise ImportStopped("TOC_ENCODING_INVALID", "toc_validate") from exc
    if not selected_lines or len(selected_lines) != len(set(selected_lines)) or not set(selected_lines).issubset(set(available_lines)):
        raise ImportStopped("TOC_ALLOWLIST_NOT_SUBSET", "toc_validate")
    if any(any(fragment in line for fragment in DISALLOWED_TOC) for line in selected_lines):
        raise ImportStopped("TOC_ALLOWLIST_INCLUDES_EXTENSION_OBJECT", "toc_validate")
    return selected_lines


def validate_normalization_sql(sql: bytes, expected_sha256: str) -> None:
    if hashlib.sha256(sql).hexdigest() != expected_sha256 or not sql or b"\x00" in sql:
        raise ImportStopped("NORMALIZATION_SQL_HASH_OR_FORMAT_INVALID", "normalization_validate")
    try:
        decoded = sql.decode("utf-8", "strict")
    except UnicodeDecodeError as exc:
        raise ImportStopped("NORMALIZATION_SQL_ENCODING_INVALID", "normalization_validate") from exc
    if DISALLOWED_NORMALIZATION_SQL.search(decoded) or "\\" in decoded or "--" in decoded or "/*" in decoded or "*/" in decoded:
        raise ImportStopped("NORMALIZATION_SQL_CONTAINS_FORBIDDEN_OPERATION", "normalization_validate")
    ident = r"(?:[A-Za-z_][A-Za-z0-9_$]*)(?:\.(?:[A-Za-z_][A-Za-z0-9_$]*))*"
    type_name = r"(?:[A-Za-z_][A-Za-z0-9_$]*)(?:\.(?:[A-Za-z_][A-Za-z0-9_$]*))*(?:\s+(?:varying|precision|with|without|time|zone|character|double|timestamp|bit))*(?:\s*\([A-Za-z_0-9$ ,\.]+\))?(?:\[\])*"
    function_ident = rf"{ident}\s*\(\s*(?:{type_name}(?:\s*,\s*{type_name})*)?\s*\)"
    role = r"(?:espacigo_runtime|espacigo_gadm_bootstrap|espacigo_gcp03_observer|PUBLIC)"
    privileges = r"(?:ALL(?:\s+PRIVILEGES)?|SELECT|INSERT|UPDATE|DELETE|TRUNCATE|REFERENCES|TRIGGER|USAGE|CREATE|EXECUTE)(?:\s*,\s*(?:ALL(?:\s+PRIVILEGES)?|SELECT|INSERT|UPDATE|DELETE|TRUNCATE|REFERENCES|TRIGGER|USAGE|CREATE|EXECUTE))*"
    grant_object = rf"(?:ALL\s+(?:TABLES|SEQUENCES|FUNCTIONS)\s+IN\s+SCHEMA\s+{ident}|(?:TABLE|SEQUENCE|SCHEMA)\s+{ident}|(?:FUNCTION|PROCEDURE)\s+{function_ident})"
    object_owner = re.compile(rf"ALTER\s+(?:TABLE|SEQUENCE|TYPE|SCHEMA)\s+{ident}\s+OWNER\s+TO\s+{role}|ALTER\s+(?:FUNCTION|PROCEDURE)\s+{function_ident}\s+OWNER\s+TO\s+{role}", re.IGNORECASE)
    grants = re.compile(rf"(?:GRANT\s+{privileges}\s+ON\s+{grant_object}\s+TO\s+{role}|REVOKE\s+{privileges}\s+ON\s+{grant_object}\s+FROM\s+{role}|ALTER\s+DEFAULT\s+PRIVILEGES\s+FOR\s+ROLE\s+{ident}\s+IN\s+SCHEMA\s+{ident}\s+(?:GRANT\s+{privileges}\s+ON\s+(?:TABLES|SEQUENCES|FUNCTIONS)\s+TO\s+{role}|REVOKE\s+{privileges}\s+ON\s+(?:TABLES|SEQUENCES|FUNCTIONS)\s+FROM\s+{role}))", re.IGNORECASE)
    statements = [part.strip() for part in decoded.split(";") if part.strip()]
    if not statements or any(not (object_owner.fullmatch(stmt) or grants.fullmatch(stmt)) for stmt in statements):
        raise ImportStopped("NORMALIZATION_SQL_OUTSIDE_ALLOWLIST", "normalization_validate")


class OriginImportExecutor:
    def __init__(self, config: ImportConfig, store: ObjectStore, database: DatabaseRunner, private_file_store=None):
        self.config, self.store, self.database = config, store, database
        self.private_file_store = private_file_store

    def run(self) -> dict:
        stage = "confirmation"
        mutation_started = False
        try:
            if self.config.confirmation != EXPECTED_TARGET or self.config.bucket != TARGET_BUCKET:
                raise ImportStopped("TARGET_CONFIRMATION_MISMATCH", stage)
            if self.config.private_files_bucket != PRIVATE_FILES_BUCKET:
                raise ImportStopped("PRIVATE_FILES_BUCKET_MISMATCH", stage)
            if not all(SHA_RE.fullmatch(v) for v in (self.config.expected_receipt_sha256,
                    self.config.expected_manifest_sha256,self.config.expected_bootstrap_sha256,
                    self.config.expected_observer_sha256,self.config.expected_normalization_sha256)):
                raise ImportStopped("EXPECTED_EVIDENCE_HASH_INVALID", stage)
            k_run = load_transport_key(self.config.transport_key_file)
            stage = "receipt_read"
            receipt_bytes = read_exact(self.store, self.config.bucket, self.config.receipt)
            if hashlib.sha256(receipt_bytes).hexdigest() != self.config.expected_receipt_sha256:
                raise ImportStopped("RECEIPT_HASH_MISMATCH", stage)
            receipt = json.loads(receipt_bytes)
            manifest_ref = parse_ref(receipt.get("transport_manifest"))
            stage = "manifest_read"
            manifest_bytes = read_exact(self.store, self.config.bucket, manifest_ref)
            if hashlib.sha256(manifest_bytes).hexdigest() != self.config.expected_manifest_sha256:
                raise ImportStopped("MANIFEST_HASH_MISMATCH", stage)
            manifest = json.loads(manifest_bytes)
            run_prefix = f"runs/{TARGET_RUN_ID}/"
            if not manifest_ref.name.startswith(run_prefix) or not self.config.receipt.name.startswith(run_prefix):
                raise ImportStopped("RECEIPT_OR_MANIFEST_PATH_INVALID", "receipt")
            if manifest.get("migration_inventory_sha256") != current_migration_inventory_sha256():
                raise ImportStopped("MIGRATION_INVENTORY_MISMATCH", "manifest")
            refs, allowlist_ref, normalization_ref = validate_manifest_and_receipt(receipt, manifest, self.config)
            salt = bytes.fromhex(manifest.get("salt_hex", ""))
            if len(salt) != 32:
                raise ImportStopped("TRANSPORT_SALT_INVALID", "manifest")
            # Secret Manager stores K_run itself; K_recovery never enters GCP.
            k_wrap = k_run
            stage = "offline_artifact_authentication"
            artifact_reports = {}
            for artifact in sorted(ARTIFACTS):
                sink = _DiscardSink()
                stats = decrypt_object(self.store, self.config.bucket, refs[artifact], manifest["artifacts"][artifact], k_wrap, artifact, sink)
                artifact_reports[artifact] = {"plain_bytes": stats["plain_bytes"], "chunks": stats["chunks"]}
            stage = "toc_read"
            allowlist = read_exact(self.store, self.config.bucket, allowlist_ref)
            normalization_sql = read_exact(self.store, self.config.bucket, normalization_ref)
            validate_normalization_sql(normalization_sql, self.config.expected_normalization_sha256)
            stage = "database_readiness"
            preflight = self.database.preflight()
            if preflight.get("database") != TARGET_DATABASE or preflight.get("empty") is not True or \
                    preflight.get("roles_ready") is not True or preflight.get("extensions_ready") is not True or \
                    preflight.get("application_schemas_empty") is not True or preflight.get("postgres_version") is not True or \
                    preflight.get("admin_role") is not True:
                raise ImportStopped("TARGET_NOT_PRISTINE", stage)
            stage = "archive_toc_validation"
            available_toc = "\n".join(self.database.list_toc(lambda sink: decrypt_object(
                self.store, self.config.bucket, refs["database.dump"], manifest["artifacts"]["database.dump"], k_wrap, "database.dump", sink))
            )
            selected_toc = validate_toc(available_toc, allowlist)
            stage = "database_restore"
            mutation_started = True
            self.database.restore(lambda sink: decrypt_object(
                self.store, self.config.bucket, refs["database.dump"], manifest["artifacts"]["database.dump"], k_wrap, "database.dump", sink), allowlist)
            stage = "owner_acl_normalization"
            self.database.normalize_owners_and_acls(normalization_sql)
            stage = "copy_sanitization"
            sanitation = self.database.sanitize()
            required = ("valid_sessions", "usable_action_tokens", "pending_notices", "identity_event_leases")
            if any(sanitation.get(name) != 0 for name in required) or sanitation.get("runtime_role_login") is not False:
                raise ImportStopped("SANITATION_POSTCONDITIONS_FAILED", stage, mutation_started=True)
            stage = "private_files_copy"
            private_archive = io.BytesIO()
            private_stats = decrypt_object(self.store, self.config.bucket, refs["private-files.tar"],
                manifest["artifacts"]["private-files.tar"], k_wrap, "private-files.tar", private_archive)
            private_store = self.private_file_store or GCSPrivateFileStore(self.config.private_files_bucket)
            private_files = copy_private_files(private_archive.getvalue(), run_id=TARGET_RUN_ID, store=private_store)
            return {"format": FORMAT, "status": "imported_and_sanitized", "source_capture_run_id": SOURCE_CAPTURE_RUN_ID,
                    "target_run_id": TARGET_RUN_ID, "source_sealed_sha256": SOURCE_SEALED_SHA256,
                    "manifest_sha256": self.config.expected_manifest_sha256,
                    "receipt_sha256": self.config.expected_receipt_sha256,
                    "artifacts_authenticated": len(artifact_reports), "allowlist_entries": len(selected_toc),
                    "normalization_sql_sha256": self.config.expected_normalization_sha256,
                    "sanitized": True, "private_files_copied": len(private_files),
                    "private_files_plain_bytes": private_stats["plain_bytes"], "services_or_workers_started": False}
        except ImportStopped as exc:
            return {"format": FORMAT, "status": "stopped", "stage": exc.stage or stage,
                    "error_code": exc.code, "transaction_outcome": "uncertain" if exc.mutation_started or mutation_started else "not_started",
                    "retry": False, "source_capture_run_id": SOURCE_CAPTURE_RUN_ID, "target_run_id": TARGET_RUN_ID}
        except Exception:
            return {"format": FORMAT, "status": "stopped", "stage": stage,
                    "error_code": "UNEXPECTED_EXECUTOR_FAILURE", "transaction_outcome": "uncertain" if mutation_started else "not_started",
                    "retry": False, "source_capture_run_id": SOURCE_CAPTURE_RUN_ID, "target_run_id": TARGET_RUN_ID}


class _DiscardSink:
    def write(self, value: bytes) -> int:
        return len(value)


class GCSObjectStore:
    def __init__(self):
        from google.cloud import storage
        self.client = storage.Client()

    def open(self, bucket: str, ref: ObjectRef) -> Download:
        blob = self.client.bucket(bucket).blob(ref.name, generation=ref.generation)
        try:
            blob.reload(if_generation_match=ref.generation)
            if int(blob.generation) != ref.generation or int(blob.size) != ref.size_bytes:
                raise ImportStopped("OBJECT_GENERATION_OR_SIZE_MISMATCH", "gcs_open")
            return Download(ref.generation, blob.open("rb"))
        except ImportStopped:
            raise
        except Exception as exc:
            raise ImportStopped("GCS_OBJECT_READ_FAILED", "gcs_open") from exc


class GCSPrivateFileStore:
    """Create-only object writer for the dedicated, non-serving private-file bucket."""
    def __init__(self, bucket: str):
        if bucket != PRIVATE_FILES_BUCKET:
            raise ImportStopped("PRIVATE_FILES_BUCKET_MISMATCH", "private_files_copy", mutation_started=True)
        from google.cloud import storage
        self.client = storage.Client()
        self.bucket = bucket

    def create(self, object_name: str, data: bytes, sha256: str) -> None:
        prefix = f"runs/{TARGET_RUN_ID}/"
        if not object_name.startswith(prefix) or ".." in object_name.split("/") or \
                hashlib.sha256(data).hexdigest() != sha256:
            raise ImportStopped("PRIVATE_FILE_OBJECT_INVALID", "private_files_copy", mutation_started=True)
        blob = self.client.bucket(self.bucket).blob(object_name)
        try:
            blob.metadata = {"sha256": sha256}
            blob.upload_from_file(io.BytesIO(data), rewind=True, size=len(data),
                content_type="application/octet-stream", if_generation_match=0, checksum="crc32c")
            if not blob.generation or int(blob.size) != len(data):
                raise ImportStopped("PRIVATE_FILE_UPLOAD_VERIFY_FAILED", "private_files_copy", mutation_started=True)
        except ImportStopped:
            raise
        except Exception as exc:
            raise ImportStopped("PRIVATE_FILE_UPLOAD_FAILED", "private_files_copy", mutation_started=True) from exc


def _psql_env(passfile: Path) -> dict[str, str]:
    env = os.environ.copy()
    env["PGPASSFILE"] = str(passfile)
    env["PGAPPNAME"] = "gcp03-origin-import-g03o-1017"
    return env


class PsqlPgRestoreRunner:
    """Fixed PostgreSQL 18 client runner; target access is via loopback proxy."""
    def __init__(self, password_file: Path, *, host: str = "127.0.0.1", port: int = 5432,
                 pg_restore: str = "pg_restore", psql: str = "psql"):
        if password_file.is_symlink() or not password_file.is_file() or stat.S_IMODE(password_file.stat().st_mode) not in {0o400,0o444,0o600}:
            raise ImportStopped("DATABASE_PASSWORD_FILE_INVALID", "database_setup")
        password = password_file.read_text().strip()
        if not password or any(c in password for c in "\r\n\x00"):
            raise ImportStopped("DATABASE_PASSWORD_FORMAT_INVALID", "database_setup")
        try:
            self.pgpass = Path("/dev/shm") / ("g03-pgpass-" + secrets.token_hex(12))
            fd = os.open(self.pgpass, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            escaped = password.replace("\\", "\\\\").replace(":", "\\:")
            with os.fdopen(fd, "w") as stream:
                pass_host = "localhost" if host.startswith("/") else host
                stream.write(f"{pass_host}:{port}:{TARGET_DATABASE}:postgres:{escaped}\n")
                stream.flush(); os.fsync(stream.fileno())
        except Exception as exc:
            raise ImportStopped("DATABASE_AUTH_PREPARATION_FAILED", "database_setup") from exc
        self.pg_restore, self.psql = pg_restore, psql
        self.host, self.port = host, port
        self.last_list_pipe_closed_early = False

    def close(self) -> None:
        try: self.pgpass.unlink()
        except OSError: pass

    def _psql(self, sql: str) -> str:
        proc = subprocess.run([self.psql,"-X","-qAt","--set=ON_ERROR_STOP=1","--host="+self.host,"--port="+str(self.port),"--username=postgres","--dbname="+TARGET_DATABASE,"-c",sql],
                              env=_psql_env(self.pgpass),stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,check=False)
        if proc.returncode != 0:
            raise ImportStopped("PSQL_COMMAND_FAILED", "database_query")
        return proc.stdout.decode("utf-8", "strict").strip()

    def preflight(self) -> dict:
        raw = self._psql(PREFLIGHT_SQL)
        try: data = json.loads(raw)
        except json.JSONDecodeError as exc: raise ImportStopped("TARGET_PREFLIGHT_INVALID", "database_preflight") from exc
        return data

    def _pipe_archive(self, args: list[str], feed_archive: Callable[[BinaryIO], dict], *, capture: bool) -> str:
        proc = subprocess.Popen(args,stdin=subprocess.PIPE,stdout=subprocess.PIPE if capture else subprocess.DEVNULL,stderr=subprocess.DEVNULL,env=_psql_env(self.pgpass))
        assert proc.stdin is not None
        output: list[bytes] = []
        reader = None
        if capture:
            assert proc.stdout is not None
            reader = threading.Thread(target=lambda: output.append(proc.stdout.read()),daemon=True);reader.start()
        try:
            sink = _ArchivePipeSink(proc.stdin, tolerate_early_close=capture)
            feed_archive(sink)
            sink.close()
            if capture:
                self.last_list_pipe_closed_early = sink.discard
            rc = proc.wait(timeout=3600)
            if reader:
                reader.join(timeout=60)
                if reader.is_alive():
                    proc.kill()
                    proc.wait()
                    raise ImportStopped("PG_RESTORE_LIST_OUTPUT_TIMEOUT", "archive_toc")
        except BaseException:
            if proc.poll() is None:
                proc.kill()
            proc.wait()
            if reader:
                reader.join(timeout=60)
            raise
        finally:
            for stream in (proc.stdin, proc.stdout):
                if stream is not None and not stream.closed:
                    try:
                        stream.close()
                    except BrokenPipeError:
                        pass
        if rc != 0:
            raise ImportStopped("PG_RESTORE_COMMAND_FAILED", "archive_toc" if capture else "database_restore", mutation_started=not capture)
        return output[0].decode("utf-8","strict") if output else ""

    def list_toc(self, feed_archive: Callable[[BinaryIO], dict]) -> list[str]:
        self.last_list_pipe_closed_early = False
        text=self._pipe_archive([self.pg_restore,"--list"],feed_archive,capture=True)
        return [line.rstrip() for line in text.splitlines()]

    def restore(self, feed_archive: Callable[[BinaryIO], dict], allowlist: bytes) -> None:
        path=Path("/dev/shm")/("g03-toc-"+secrets.token_hex(12))
        fd=os.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
        try:
            with os.fdopen(fd,"wb") as stream: stream.write(allowlist);stream.flush();os.fsync(stream.fileno())
            self._pipe_archive([self.pg_restore,"--exit-on-error","--single-transaction","--no-owner","--no-acl",
                "--username=postgres","--host="+self.host,"--port="+str(self.port),"--dbname="+TARGET_DATABASE,
                "--use-list="+str(path)],feed_archive,capture=False)
        finally:
            try:path.unlink()
            except OSError:pass

    def normalize_owners_and_acls(self, sql: bytes) -> None:
        self._run_sql(b"BEGIN;\n" + sql + b"\nCOMMIT;\n", "OWNER_ACL_NORMALIZATION_FAILED")

    def sanitize(self) -> dict:
        sql_path=Path(__file__).resolve().with_name("gcp03_origin_quarantine_cloud.sql")
        if not sql_path.is_file() or sql_path.is_symlink():
            raise ImportStopped("QUARANTINE_SQL_MISSING","copy_sanitization",mutation_started=True)
        try:return json.loads(self._run_sql(sql_path.read_bytes(),"SANITATION_SQL_FAILED"))
        except (json.JSONDecodeError,ImportStopped) as exc: raise ImportStopped("SANITATION_REPORT_INVALID","copy_sanitization",mutation_started=True) from exc

    def _run_sql(self, sql: bytes, error_code: str) -> str:
        proc=subprocess.run([self.psql,"-X","-qAt","--set=ON_ERROR_STOP=1","--host="+self.host,"--port="+str(self.port),"--username=postgres","--dbname="+TARGET_DATABASE],
            input=sql,env=_psql_env(self.pgpass),stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,check=False)
        if proc.returncode != 0: raise ImportStopped(error_code,"database_mutation",mutation_started=True)
        return proc.stdout.decode("utf-8","strict").strip()


class _ArchivePipeSink:
    """Keep authenticating/listing the full archive if pg_restore --list exits early."""
    def __init__(self, stream: BinaryIO, *, tolerate_early_close: bool):
        self.stream = stream
        self.tolerate_early_close = tolerate_early_close
        self.discard = False

    def write(self, data: bytes) -> int:
        if self.discard:
            return len(data)
        try:
            return self.stream.write(data)
        except BrokenPipeError:
            if not self.tolerate_early_close:
                raise
            self.discard = True
            try:
                self.stream.close()
            except BrokenPipeError:
                pass
            return len(data)

    def close(self) -> None:
        if self.discard:
            return
        try:
            self.stream.close()
        except BrokenPipeError:
            if not self.tolerate_early_close:
                raise
            self.discard = True


PREFLIGHT_SQL = """
SELECT json_build_object(
 'database',current_database(),
 'postgres_version',(current_setting('server_version_num')::int>=180000 AND current_setting('server_version_num')::int<190000),
 'admin_role',(current_user='postgres'),
 'empty',
   (SELECT count(*)=0 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND c.relkind IN ('r','p','v','m','S','f')
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass))
   AND (SELECT count(*)=0 FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass))
   AND (SELECT count(*)=0 FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace
    WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema' AND t.typtype IN ('b','c','d','e','m','r')
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_type'::regclass AND d.objid=t.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),
 'roles_ready',(SELECT count(*)=3 FROM pg_roles WHERE rolname IN ('espacigo_runtime','espacigo_gadm_bootstrap','espacigo_gcp03_observer') AND NOT rolcanlogin AND NOT rolsuper AND NOT rolcreatedb AND NOT rolcreaterole AND NOT rolreplication AND NOT rolbypassrls AND NOT rolinherit),
 'extensions_ready',((SELECT count(*)=7 FROM pg_extension) AND (SELECT count(*)=7 FROM pg_extension WHERE (extname,extversion) IN (('btree_gist','1.8'),('fuzzystrmatch','1.2'),('plpgsql','1.0'),('postgis','3.6.4'),('postgis_tiger_geocoder','3.6.4'),('postgis_topology','3.6.4'),('unaccent','1.1')))),
 'application_schemas_empty',(WITH schema_inventory AS (
   SELECT n.oid,n.nspname,
     EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_namespace'::regclass AND d.objid=n.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass) AS directly_extension_owned,
     (EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_depend d ON d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass WHERE c.relnamespace=n.oid AND c.relkind IN ('r','p','v','m','S','f'))
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_depend d ON d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass WHERE p.pronamespace=n.oid)) AS has_extension_objects,
     (EXISTS (SELECT 1 FROM pg_catalog.pg_class c WHERE c.relnamespace=n.oid AND c.relkind IN ('r','p','v','m','S','f') AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_proc p WHERE p.pronamespace=n.oid AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_catalog.pg_extension'::regclass))) AS has_application_objects
   FROM pg_catalog.pg_namespace n WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname NOT IN ('public','information_schema')
 ) SELECT count(*) FILTER (WHERE NOT directly_extension_owned AND NOT (has_extension_objects AND NOT has_application_objects))=0 FROM schema_inventory)
)::text;
"""


def current_migration_inventory_sha256() -> str:
    migrations_dir = Path(__file__).resolve().parents[1] / "db" / "migrations"
    items = []
    for version in range(1, 62):
        matches = list(migrations_dir.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1 or matches[0].is_symlink() or not matches[0].is_file():
            raise ImportStopped("MIGRATION_INVENTORY_INVALID", "manifest")
        path = matches[0]
        items.append((version, path.name, hashlib.sha256(path.read_bytes()).hexdigest()))
    return hashlib.sha256("".join(f"{v}:{name}:{digest}\n" for v, name, digest in items).encode()).hexdigest()


def config_from_env(env: dict[str,str]) -> ImportConfig:
    confirmation=env.get("GCP03_ORIGIN_IMPORT_CONFIRM_TARGET","")
    key_file=Path(env.get("GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE",""))
    try:
        receipt=ObjectRef(env["GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT"],int(env["GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION"]),env["GCP03_ORIGIN_IMPORT_RECEIPT_SHA256"],int(env["GCP03_ORIGIN_IMPORT_RECEIPT_BYTES"]))
        return ImportConfig(env["GCP03_ORIGIN_IMPORT_BUCKET"], env["GCP03_ORIGIN_PRIVATE_FILES_BUCKET"], receipt,env["GCP03_ORIGIN_IMPORT_RECEIPT_SHA256"],
            env["GCP03_ORIGIN_IMPORT_MANIFEST_SHA256"],env["GCP03_ORIGIN_IMPORT_BOOTSTRAP_SHA256"],
            env["GCP03_ORIGIN_IMPORT_OBSERVER_SHA256"],env["GCP03_ORIGIN_IMPORT_NORMALIZATION_SHA256"],confirmation,key_file)
    except (KeyError,ValueError) as exc: raise ImportStopped("IMPORT_ENVIRONMENT_INVALID","config") from exc


def main() -> None:
    config=None; runner=None
    try:
        config=config_from_env(dict(os.environ))
        store=GCSObjectStore()
        wait_for_proxy()
        runner=PsqlPgRestoreRunner(Path("/secrets/admin/password"))
        report=OriginImportExecutor(config,store,runner).run()
        print(json.dumps(report,sort_keys=True))
        if report.get("status")!="imported_and_sanitized": raise SystemExit(1)
    except ImportStopped as exc:
        print(json.dumps({"format":FORMAT,"status":"stopped","stage":exc.stage,"error_code":exc.code,
            "transaction_outcome":"uncertain" if exc.mutation_started else "not_started","retry":False},sort_keys=True))
        raise SystemExit(1)
    finally:
        if runner: runner.close()


def wait_for_proxy(*, host: str = "127.0.0.1", port: int = 5432, timeout_seconds: float = 90,
                   connect=socket.create_connection, clock=time.monotonic, sleep=time.sleep) -> None:
    deadline = clock() + timeout_seconds
    while True:
        try:
            with connect((host, port), timeout=1):
                return
        except OSError:
            if clock() >= deadline:
                raise ImportStopped("CLOUD_SQL_PROXY_NOT_READY", "proxy_startup")
            sleep(min(0.5, max(0, deadline - clock())))


if __name__=="__main__":
    main()
