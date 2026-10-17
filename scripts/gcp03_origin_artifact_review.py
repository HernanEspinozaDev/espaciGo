#!/usr/bin/env python3
"""Generate a private, hash-bound TOC/owner plan from an existing GCP-03 envelope.

This local-only tool decrypts only origin-metadata.json and database.dump from
the reviewed transport package. Both plaintexts remain in process memory; the
dump is passed to ``pg_restore --list`` on stdin and is never restored/executed.
"""
from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
from pathlib import Path
import re
import shlex
import stat
import subprocess
import sys

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

import gcp03_origin_import as transport
from gcp03_origin_cloud_import_executor import ImportStopped, validate_normalization_sql, validate_toc
from gcp03_origin_postrestore_verify import table_names_from_toc

SOURCE_CAPTURE_RUN_ID = "gcp03-merge-review-20261017"
TARGET_RUN_ID = "g03o-1017"
SOURCE_SEALED_SHA256 = "80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262"
TRANSPORT_MANIFEST_SHA256 = "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30"
ARTIFACTS_TO_DECRYPT = {"database.dump", "origin-metadata.json"}
SHA_RE = re.compile(r"^[0-9a-f]{64}$")
SIMPLE_IDENT = re.compile(r"^[a-z_][a-z0-9_$]*$")

TYPE_WORDS = (
    "MATERIALIZED VIEW DATA", "TEXT SEARCH CONFIGURATION", "TEXT SEARCH DICTIONARY",
    "SEQUENCE OWNED BY", "PUBLICATION TABLES IN SCHEMA",
    "TEXT SEARCH CONFIGURATION", "TEXT SEARCH DICTIONARY", "TEXT SEARCH PARSER",
    "TEXT SEARCH TEMPLATE", "MATERIALIZED VIEW", "TABLE ATTACH", "INDEX ATTACH",
    "SEQUENCE SET", "TABLE DATA", "FK CONSTRAINT", "PUBLICATION TABLE",
    "FOREIGN TABLE", "EVENT TRIGGER", "DEFAULT ACL", "SEQUENCE", "AGGREGATE",
    "PROCEDURE", "FUNCTION", "CONSTRAINT", "TRIGGER", "POLICY", "DOMAIN",
    "INDEX", "RULE", "TYPE", "VIEW", "TABLE", "DEFAULT", "ACL", "COMMENT", "EXTENSION",
    "SCHEMA",
)
RESTORE_TYPES = {
    "AGGREGATE", "CONSTRAINT", "DOMAIN", "FK CONSTRAINT", "FOREIGN TABLE",
    "FUNCTION", "INDEX", "INDEX ATTACH", "MATERIALIZED VIEW", "MATERIALIZED VIEW DATA",
    "POLICY", "PROCEDURE", "RULE", "SEQUENCE", "SEQUENCE OWNED BY", "SEQUENCE SET",
    "TABLE", "TABLE ATTACH", "TABLE DATA", "TRIGGER", "TYPE", "VIEW", "DEFAULT",
    "TEXT SEARCH CONFIGURATION", "TEXT SEARCH DICTIONARY",
}
OWNER_RELATION_TYPES = {"TABLE", "SEQUENCE", "VIEW", "MATERIALIZED VIEW", "FOREIGN TABLE",
                        "TEXT SEARCH CONFIGURATION", "TEXT SEARCH DICTIONARY"}
OWNER_ROUTINE_TYPES = {"AGGREGATE", "FUNCTION", "PROCEDURE"}


class ArtifactReviewError(ValueError):
    pass


def sha256_file(path: Path) -> tuple[str, int]:
    digest, size = hashlib.sha256(), 0
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
            size += len(chunk)
    return digest.hexdigest(), size


def _private_key(path: Path) -> bytes:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise ArtifactReviewError("recovery key permissions are invalid")
    raw = path.read_bytes().strip()
    if not re.fullmatch(rb"[0-9a-f]{64}", raw):
        raise ArtifactReviewError("recovery key format is invalid")
    return bytes.fromhex(raw.decode("ascii"))


def _private_artifact(stage_dir: Path, manifest: dict, recovery_key: bytes, artifact: str) -> io.BytesIO:
    """Authenticate one explicitly allowlisted transport artifact into memory."""
    if artifact not in ARTIFACTS_TO_DECRYPT:
        raise ArtifactReviewError("artifact is outside the local review allowlist")
    item = manifest["artifacts"].get(artifact)
    if not isinstance(item, dict):
        raise ArtifactReviewError("review artifact descriptor is invalid")
    source_name = item.get("source_name")
    if source_name not in transport.ARTIFACTS or source_name.removesuffix(".gpg") != artifact:
        raise ArtifactReviewError("review artifact identity is invalid")
    filename = transport.ARTIFACTS[source_name]
    path = stage_dir / filename
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise ArtifactReviewError("review artifact permissions are invalid")
    cipher_sha, cipher_bytes = sha256_file(path)
    if cipher_sha != item.get("transport_cipher_sha256") or cipher_bytes != item.get("transport_cipher_bytes"):
        raise ArtifactReviewError("review artifact ciphertext binding failed")

    salt = bytes.fromhex(manifest["salt_hex"])
    kek = transport.transport_key(recovery_key, salt, TARGET_RUN_ID)
    dek = AESGCM(kek).decrypt(bytes.fromhex(item["wrap_nonce_hex"]),
        bytes.fromhex(item["wrapped_dek_hex"]),
        f"{transport.FORMAT}:{TARGET_RUN_ID}:{artifact}:dek".encode())
    clear = io.BytesIO()
    try:
        with path.open("rb") as source:
            stats = transport.decrypt_chunks(source, clear, dek, TARGET_RUN_ID, artifact)
            if source.tell() != cipher_bytes:
                raise ArtifactReviewError("review artifact ciphertext changed during read")
    except Exception:
        _wipe(clear)
        raise
    cipher_sha_after, cipher_bytes_after = sha256_file(path)
    if cipher_sha_after != cipher_sha or cipher_bytes_after != cipher_bytes:
        clear.close()
        raise ArtifactReviewError("review artifact ciphertext changed during verification")
    if stats["plain_sha256"] != item.get("plain_sha256") or \
       stats["plain_bytes"] != item.get("plain_bytes") or stats["chunks"] != item.get("chunk_count"):
        clear.close()
        raise ArtifactReviewError("review artifact plaintext binding failed")
    clear.seek(0)
    return clear


def _wipe(stream: io.BytesIO) -> None:
    try:
        view = stream.getbuffer()
        view[:] = b"\x00" * len(view)
        view.release()
    finally:
        stream.close()


def _migration_inventory(metadata: dict, repo_root: Path, expected_sha: str) -> str:
    report = metadata.get("database_report") or {}
    preflight = metadata.get("preflight") or {}
    source_projection = preflight.get("source_projection") or {}
    source_database = source_projection.get("configured_database_name")
    release = metadata.get("release") or {}
    history = report.get("migration_history")
    if not isinstance(source_database, str) or not source_database or "\x00" in source_database or \
       len(source_database.encode("utf-8")) > 63 or report.get("database") != source_database or \
       release.get("run_id") != SOURCE_CAPTURE_RUN_ID or report.get("migration_match_v1_v61") is not True or \
       report.get("source_credentials_included") is not False or report.get("role_passwords_included") is not False or \
       not isinstance(history, list) or len(history) != 61:
        raise ArtifactReviewError("sealed metadata does not prove the reviewed migration and credential-exclusion contract")
    expected = []
    migrations = repo_root / "db" / "migrations"
    for version in range(1, 62):
        matches = list(migrations.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1 or matches[0].is_symlink() or not matches[0].is_file():
            raise ArtifactReviewError("working-tree migration inventory is invalid")
        path = matches[0]
        expected.append((version, path.name, sha256_file(path)[0]))
    for version, entry in enumerate(history, start=1):
        try:
            found = (entry["name"], entry["checksum"])
        except (KeyError, TypeError) as exc:
            raise ArtifactReviewError("sealed migration history is malformed") from exc
        if found != (expected[version - 1][1], expected[version - 1][2]):
            raise ArtifactReviewError("working-tree migrations differ from the sealed history")
    digest = hashlib.sha256("".join(f"{v}:{n}:{h}\n" for v, n, h in expected).encode()).hexdigest()
    if digest != expected_sha:
        raise ArtifactReviewError("manifest migration inventory does not match the sealed history")
    return digest


def _parse_toc_entry(line: str) -> dict | None:
    stripped = line.strip()
    if not stripped or stripped.startswith(";") or ";" not in stripped:
        return None
    entry_id, body = stripped.split(";", 1)
    if not entry_id.strip().isdigit():
        raise ArtifactReviewError("archive TOC entry identifier is malformed")
    try:
        tokens = shlex.split(body.strip())
    except ValueError as exc:
        raise ArtifactReviewError("archive TOC entry encoding is malformed") from exc
    if len(tokens) < 3 or not tokens[0].isdigit() or not tokens[1].isdigit():
        raise ArtifactReviewError("archive TOC entry is malformed")
    remainder = tokens[2:]
    entry_type = next((kind for kind in TYPE_WORDS if remainder[:len(kind.split())] == kind.split()), None)
    if entry_type is None:
        raise ArtifactReviewError("archive TOC entry type is unsupported")
    words = len(entry_type.split())
    result = {"line": stripped, "entry_id": int(entry_id), "type": entry_type,
              "tokens": tokens, "schema": None, "name": None, "owner": None}
    if entry_type in {"ACL", "COMMENT", "EXTENSION", "SCHEMA"}:
        return result
    index = 2 + words
    if len(tokens) > index:
        result["schema"] = tokens[index]
    if entry_type in OWNER_ROUTINE_TYPES and len(tokens) > index + 2:
        result["name"] = " ".join(tokens[index + 1:-1])
    elif len(tokens) > index + 1:
        result["name"] = tokens[index + 1]
    if len(tokens) > index + 2:
        result["owner"] = tokens[-1]
    return result


def _quote_ident(value: str) -> str:
    if not value or "\x00" in value or ";" in value or "\\" in value:
        raise ArtifactReviewError("source object identifier is unsupported")
    if SIMPLE_IDENT.fullmatch(value):
        return value
    return '"' + value.replace('"', '""') + '"'


def _object_key(schema: str, name: str) -> tuple[str, str]:
    return schema, name


def build_review_plan(toc_text: str, metadata: dict, migration_sha: str) -> tuple[bytes, bytes, dict]:
    relation_rows = metadata.get("database_report", {}).get("relation_owners_acls")
    routine_rows = metadata.get("database_report", {}).get("routine_owners_acls")
    if not isinstance(relation_rows, list) or not isinstance(routine_rows, list):
        raise ArtifactReviewError("sealed ownership metadata is incomplete")
    relations = {_object_key(row.get("schema"), row.get("name")): row for row in relation_rows
                 if isinstance(row, dict) and row.get("schema") == "public"}
    routines = {}
    for row in routine_rows:
        if not isinstance(row, dict) or row.get("schema") != "public":
            continue
        routines[(row.get("name"), row.get("identity_args", ""))] = row

    parsed, selected, owner_map = [], [], []
    kind_counts: dict[str, int] = {}
    source_acl_digests = []
    for line in toc_text.splitlines():
        item = _parse_toc_entry(line)
        if item is None:
            continue
        kind = item["type"]
        kind_counts[kind] = kind_counts.get(kind, 0) + 1
        if kind in {"ACL", "COMMENT", "EXTENSION", "SCHEMA"}:
            continue
        if item["schema"] != "public":
            continue
        if kind not in RESTORE_TYPES:
            raise ArtifactReviewError("archive contains an unreviewed public TOC type: " + kind)
        if kind in {"TABLE", "TABLE DATA"} and item["name"] == "spatial_ref_sys":
            continue
        selected.append(item["line"])

        if kind in OWNER_RELATION_TYPES | {"TYPE", "DOMAIN"}:
            key = _object_key(item["schema"], item["name"])
            source = relations.get(key)
            # User-defined types do not appear in pg_class relation inventory;
            # their owner is bound by the custom-archive TOC record itself.
            if kind not in {"TYPE", "DOMAIN", "TEXT SEARCH DICTIONARY", "TEXT SEARCH CONFIGURATION"} and source is None and not item.get("owner"):
                raise ArtifactReviewError("archive relation is missing from sealed owner metadata")
            source_owner = source.get("owner") if source else item["owner"]
            source_acl = source.get("acl") if source else None
            if source_acl is not None:
                source_acl_digests.append(hashlib.sha256(str(source_acl).encode()).hexdigest())
            owner_map.append({"type": kind, "schema": item["schema"], "name": item["name"],
                "source_owner": source_owner, "target_owner": "espacigo_runtime",
                "source_acl_sha256": hashlib.sha256(str(source_acl).encode()).hexdigest() if source_acl is not None else None,
                "source_acl_replaced": True})
        elif kind in OWNER_ROUTINE_TYPES:
            toc_name = item["name"]
            signature_match = re.fullmatch(r"(.+?)\((.*)\)", toc_name)
            if not signature_match:
                raise ArtifactReviewError("archive routine signature is malformed")
            routine_name, toc_args = signature_match.group(1), signature_match.group(2)
            source = routines.get((routine_name, toc_args))
            if source is None:
                candidates = [row for (name, _), row in routines.items() if name == routine_name]
                if len(candidates) == 1:
                    source = candidates[0]
                elif not candidates:
                    # The authenticated TOC itself binds the owner's role and
                    # identity signature. Keep the ACL absent because the
                    # sealed inventory has no matching routine record.
                    source = {"owner": item["owner"], "identity_args": toc_args, "acl": None}
                else:
                    raise ArtifactReviewError("archive routine cannot be matched uniquely to sealed metadata")
            identity_args = source.get("identity_args", "")
            source_acl = source.get("acl")
            if source_acl is not None:
                source_acl_digests.append(hashlib.sha256(str(source_acl).encode()).hexdigest())
            owner_map.append({"type": kind, "schema": item["schema"], "name": routine_name,
                "identity_args": identity_args, "source_owner": source.get("owner"),
                "target_owner": "espacigo_runtime",
                "source_acl_sha256": hashlib.sha256(str(source_acl).encode()).hexdigest() if source_acl is not None else None,
                "source_acl_replaced": True})
        elif kind in {"INDEX", "INDEX ATTACH"}:
            owner_map.append({"type": kind, "schema": item["schema"], "name": item["name"],
                "source_owner": item["owner"], "target_owner": "espacigo_runtime",
                "owner_action": "index owner follows normalized parent relation",
                "source_acl_replaced": True})

    if not selected:
        raise ArtifactReviewError("archive contains no public application objects")
    allowlist = ("\n".join(selected) + "\n").encode()
    try:
        validated = validate_toc(toc_text, allowlist)
    except ImportStopped as exc:
        raise ArtifactReviewError("generated TOC failed executor validation") from exc
    if len(validated) != len(selected):
        raise ArtifactReviewError("generated TOC entry count changed during validation")

    statements = []
    for item in owner_map:
        schema_name = f"{_quote_ident(item['schema'])}.{_quote_ident(item['name'])}"
        if item["type"] in {"TABLE", "VIEW", "MATERIALIZED VIEW", "FOREIGN TABLE"}:
            statements.append(f"ALTER TABLE {schema_name} OWNER TO espacigo_runtime;")
        elif item["type"] == "SEQUENCE":
            statements.append(f"ALTER SEQUENCE {schema_name} OWNER TO espacigo_runtime;")
        elif item["type"] in {"TEXT SEARCH DICTIONARY", "TEXT SEARCH CONFIGURATION"}:
            statements.append(f"ALTER {item['type']} {schema_name} OWNER TO espacigo_runtime;")
        elif item["type"] in OWNER_ROUTINE_TYPES:
            args = item.get("identity_args", "")
            if any(token in args for token in (";", "--", "/*", "*/", "\\", "\x00")):
                raise ArtifactReviewError("routine identity signature is unsafe")
            command = item["type"]
            statements.append(f"ALTER {command} {schema_name}({args}) OWNER TO espacigo_runtime;")
        elif item["type"] in {"TYPE", "DOMAIN"}:
            command = "TYPE" if item["type"] == "TYPE" else "DOMAIN"
            statements.append(f"ALTER {command} {schema_name} OWNER TO espacigo_runtime;")
        elif item["type"] in {"INDEX", "INDEX ATTACH"}:
            continue
        else:
            raise ArtifactReviewError("archive owner object type requires manual review")

    # Observer grants are column-level and read-only. The query uses no other
    # application columns; catalog inventory remains available through PUBLIC.
    observer_grants = {
        "schema_migrations": ("version", "name", "checksum"),
        "sesion": ("revocada_en", "expira_en"),
        "token_accion": ("consumido_en", "invalidado_en", "expira_en"),
        "aviso_local": ("estado",),
        "outbox_evento_local": ("lease_hasta", "entregada_en", "cancelada_en", "fallo_terminal_en"),
    }
    for table, columns in observer_grants.items():
        if ("public", table) not in relations:
            raise ArtifactReviewError("required observer relation is missing from source metadata")
        rendered = ", ".join(_quote_ident(column) for column in columns)
        statements.append(f"GRANT SELECT ({rendered}) ON TABLE public.{_quote_ident(table)} TO espacigo_gcp03_observer;")
    normalization = ("\n".join(statements) + "\n").encode()
    try:
        validate_normalization_sql(normalization, hashlib.sha256(normalization).hexdigest())
    except ImportStopped as exc:
        rejected = []
        for statement in statements:
            payload = (statement + ";\n").encode()
            try:
                validate_normalization_sql(payload, hashlib.sha256(payload).hexdigest())
            except ImportStopped:
                rejected.append(" ".join(statement.split()[:2]))
        safe_kinds = ",".join(sorted(set(rejected))) or "multiple_statements"
        raise ArtifactReviewError("generated owner/ACL SQL failed executor validation: " + exc.code +
                                  "; rejected_statement_kinds=" + safe_kinds) from exc
    table_names = table_names_from_toc(allowlist)
    map_document = {
        "format": "espacigo.gcp03.origin-owner-acl-review.v1",
        "source_sealed_sha256": SOURCE_SEALED_SHA256,
        "transport_manifest_sha256": TRANSPORT_MANIFEST_SHA256,
        "migration_inventory_sha256": migration_sha,
        "toc_allowlist_sha256": hashlib.sha256(allowlist).hexdigest(),
        "normalization_sql_sha256": hashlib.sha256(normalization).hexdigest(),
        "public_table_count": len(table_names),
        "selected_toc_entry_count": len(selected),
        "toc_entry_type_counts": kind_counts,
        "owner_action_count": len(owner_map),
        "normalization_owner_statement_count": sum(item["type"] not in {"INDEX", "INDEX ATTACH"} for item in owner_map),
        "owner_actions": owner_map,
        "source_acl_digest_count": len(source_acl_digests),
        "source_acl_digest_sha256": hashlib.sha256("\n".join(sorted(source_acl_digests)).encode()).hexdigest(),
        "source_acl_policy": "source grants are not restored; normalization replaces them with exact read-only column grants for the observer; runtime owns application objects",
        "observer_column_grant_count": len(observer_grants),
        "observer_role": "espacigo_gcp03_observer",
        "observer_role_read_only": True,
    }
    return allowlist, normalization, map_document


def _read_transport_inputs(stage_dir: Path, recovery_path: Path, expected_manifest_sha: str) -> tuple[dict, bytes, bytes]:
    if stage_dir.is_symlink() or not stage_dir.is_dir() or stat.S_IMODE(stage_dir.stat().st_mode) != 0o700:
        raise ArtifactReviewError("transport directory permissions are invalid")
    manifest_path = stage_dir / "transport-manifest.json"
    if manifest_path.is_symlink() or not manifest_path.is_file() or stat.S_IMODE(manifest_path.stat().st_mode) != 0o600:
        raise ArtifactReviewError("transport manifest permissions are invalid")
    manifest_sha, _ = sha256_file(manifest_path)
    if expected_manifest_sha != TRANSPORT_MANIFEST_SHA256 or manifest_sha != expected_manifest_sha:
        raise ArtifactReviewError("transport manifest hash does not match the reviewed input")
    manifest = json.loads(manifest_path.read_bytes())
    if manifest.get("format") != transport.FORMAT or manifest.get("source_capture_run_id") != SOURCE_CAPTURE_RUN_ID or \
       manifest.get("target_run_id") != TARGET_RUN_ID or manifest.get("source_sealed_sha256") != SOURCE_SEALED_SHA256 or \
       set(manifest.get("artifacts", {})) != {name.removesuffix(".gpg") for name in transport.ARTIFACTS}:
        raise ArtifactReviewError("transport manifest identity or artifact allowlist is invalid")
    recovery_key = _private_key(recovery_path)
    metadata_stream = _private_artifact(stage_dir, manifest, recovery_key, "origin-metadata.json")
    try:
        metadata = json.loads(metadata_stream.getvalue())
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        _wipe(metadata_stream)
        raise ArtifactReviewError("sealed source metadata is invalid") from exc
    finally:
        if not metadata_stream.closed:
            _wipe(metadata_stream)
    migration_sha = _migration_inventory(metadata, Path(__file__).resolve().parents[1],
                                         manifest.get("migration_inventory_sha256", ""))
    return manifest, recovery_key, metadata, migration_sha


def _run_pg_restore_list(archive: bytes, pg_restore: str) -> str:
    try:
        result = subprocess.run([pg_restore, "--list"], input=archive, stdout=subprocess.PIPE,
                                stderr=subprocess.DEVNULL, check=False, timeout=120)
    except (OSError, subprocess.SubprocessError) as exc:
        raise ArtifactReviewError("pg_restore TOC listing failed") from exc
    if result.returncode != 0:
        raise ArtifactReviewError("pg_restore TOC listing rejected the authenticated archive")
    try:
        return result.stdout.decode("utf-8", "strict")
    except UnicodeDecodeError as exc:
        raise ArtifactReviewError("archive TOC encoding is invalid") from exc


def _write_private(path: Path, payload: bytes) -> None:
    if path.exists() or path.is_symlink():
        raise ArtifactReviewError("refusing to overwrite private review evidence")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(payload)
        stream.flush()
        os.fsync(stream.fileno())
    os.chmod(path, 0o600)


def generate(*, stage_dir: Path, recovery_key: Path, output_dir: Path,
             expected_manifest_sha: str = TRANSPORT_MANIFEST_SHA256,
             pg_restore: str = "pg_restore") -> dict:
    testing_root = Path(__file__).resolve().parents[1] / ".local" / "testing"
    output_dir = output_dir.expanduser().absolute()
    if output_dir.is_symlink() or output_dir.exists() or not output_dir.parent.resolve().is_relative_to(testing_root.resolve()):
        raise ArtifactReviewError("output must be a new directory under private .local/testing")
    manifest, recovery, metadata, migration_sha = _read_transport_inputs(stage_dir, recovery_key, expected_manifest_sha)
    archive_stream = _private_artifact(stage_dir, manifest, recovery, "database.dump")
    archive = bytearray()
    try:
        archive.extend(archive_stream.getbuffer())
        toc_text = _run_pg_restore_list(archive, pg_restore)
    finally:
        archive[:] = b"\x00" * len(archive)
        _wipe(archive_stream)
        recovery = b"\x00" * len(recovery)
    try:
        toc, normalization, owner_map = build_review_plan(toc_text, metadata, migration_sha)
    finally:
        metadata.clear()
        toc_text = ""
    final_manifest_sha, _ = sha256_file(stage_dir / "transport-manifest.json")
    if final_manifest_sha != expected_manifest_sha:
        raise ArtifactReviewError("transport manifest changed during review")
    output_dir.mkdir(parents=True, mode=0o700)
    os.chmod(output_dir, 0o700)
    toc_path = output_dir / "toc.allowlist"
    normalization_path = output_dir / "normalization.sql"
    map_path = output_dir / "owner-acl-map.json"
    _write_private(toc_path, toc)
    _write_private(normalization_path, normalization)
    map_bytes = (json.dumps(owner_map, sort_keys=True, indent=2) + "\n").encode()
    _write_private(map_path, map_bytes)
    # The summary is safe to print and contains no source object or ACL names.
    summary = {
        "format": "espacigo.gcp03.origin-artifact-review.v1",
        "status": "generated",
        "source_sealed_sha256": SOURCE_SEALED_SHA256,
        "transport_manifest_sha256": expected_manifest_sha,
        "migration_inventory_sha256": migration_sha,
        "toc_allowlist_sha256": hashlib.sha256(toc).hexdigest(),
        "normalization_sql_sha256": hashlib.sha256(normalization).hexdigest(),
        "owner_acl_map_sha256": hashlib.sha256(map_bytes).hexdigest(),
        "toc_allowlist_bytes": len(toc),
        "normalization_sql_bytes": len(normalization),
        "owner_acl_map_bytes": len(map_bytes),
        "public_table_count": owner_map["public_table_count"],
        "selected_toc_entry_count": owner_map["selected_toc_entry_count"],
        "owner_action_count": owner_map["owner_action_count"],
        "normalization_owner_statement_count": owner_map["normalization_owner_statement_count"],
        "source_acl_digest_count": owner_map["source_acl_digest_count"],
        "observer_column_grant_count": owner_map["observer_column_grant_count"],
        "plaintext_dump_written": False,
        "decrypted_artifact_allowlist": ["database.dump", "origin-metadata.json"],
    }
    _write_private(output_dir / "summary.json", (json.dumps(summary, sort_keys=True, indent=2) + "\n").encode())
    return summary


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stage-dir", type=Path, required=True)
    parser.add_argument("--recovery-key", type=Path, required=True)
    parser.add_argument("--output-dir", type=Path, required=True)
    parser.add_argument("--expected-manifest-sha256", default=TRANSPORT_MANIFEST_SHA256)
    parser.add_argument("--pg-restore", default="pg_restore")
    args = parser.parse_args()
    try:
        print(json.dumps(generate(stage_dir=args.stage_dir, recovery_key=args.recovery_key,
            output_dir=args.output_dir, expected_manifest_sha=args.expected_manifest_sha256,
            pg_restore=args.pg_restore), sort_keys=True))
    except (ArtifactReviewError, OSError, ValueError, KeyError, TypeError, ImportStopped) as exc:
        print(json.dumps({"format": "espacigo.gcp03.origin-artifact-review.v1",
            "status": "failed", "reason": str(exc)}, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)


if __name__ == "__main__":
    main()
