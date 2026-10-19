#!/usr/bin/env python3
"""Fail-closed offline verifier for a private GCP-04 canonical baseline bundle."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
import re
import stat
import subprocess
import sys
from pathlib import Path
from typing import Callable

FORMAT = "espacigo-gcp04-w4-baseline-package-v2"
LEGACY_FORMAT = "espacigo-gcp04-w4-baseline-package-v1"
FILES = {
    "baseline.dump",
    "baseline.manifest.json",
    "baseline.toc.original",
    "baseline.toc.restore",
    "baseline.toc.restore.skipped",
    "baseline.toc.bootstrap-acl",
    "baseline.toc.acl",
    "baseline.acl.raw.sql",
    "baseline.acl.restore.sql",
    "source.schema.sql",
    "source.structure.sql",
    "source.catalog.json",
}
SHA256 = re.compile(r"^[0-9a-f]{64}$")
ROLE_ALLOWLIST = {
    "espacigo_schema_owner",
    "espacigo_runtime",
    "espacigo_privacy_cleanup_worker",
    "espacigo_db_observer",
    "espacigo_gadm_bootstrap",
    "espacigo_api_login",
    "espacigo_privacy_worker_login",
    "espacigo_migrator_login",
    "espacigo_observer_login",
}
TABLE_PRIVILEGES = {"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"}
SEQUENCE_PRIVILEGES = {"SELECT", "USAGE", "UPDATE"}
COLUMN_PRIVILEGES = {"SELECT", "INSERT", "UPDATE", "REFERENCES"}
GADM_COMMENT = "NOLOGIN GADM local bootstrap group; temporary isolated login membership only."


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def canonical_json(value: object) -> bytes:
    return json.dumps(value, sort_keys=True, separators=(",", ":")).encode("utf-8")


def verify_gadm_bootstrap_manifest(manifest: dict, catalog: dict) -> None:
    gadm = next((role for role in catalog.get("roles", []) if role.get("name") == "espacigo_gadm_bootstrap"), None)
    bootstrap = manifest.get("gadm_bootstrap")
    if gadm is None or not isinstance(bootstrap, dict):
        raise ValueError("GADM bootstrap evidence is missing from manifest/catalog")
    if gadm.get("login") is not False or gadm.get("super") is not False or gadm.get("comment") != GADM_COMMENT:
        raise ValueError("GADM bootstrap catalog attributes/comment differ from the V60 contract")
    if bootstrap.get("helper") != "db/bootstrap/gcp04-runtime/004_gadm_global_role.sql":
        raise ValueError("GADM bootstrap helper identity differs from the reviewed helper")
    if bootstrap.get("actor_contract") != "LOGIN NOSUPERUSER NOCREATEDB CREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT":
        raise ValueError("GADM bootstrap actor contract differs from the tested actor")
    if bootstrap.get("exact_comment") != gadm["comment"]:
        raise ValueError("GADM manifest comment differs from the catalog and V60 contract")
    required_true = (
        "created_and_commented_by_controlled_actor",
        "zero_memberships_and_settings_after_restore",
        "executor_absent_after_commit",
        "failure_injection_rollback_clean",
        "reserved_executor_collision_rejected",
        "invalid_actor_and_setting_rejected",
    )
    if any(bootstrap.get(field) is not True for field in required_true):
        raise ValueError("GADM bootstrap manifest is missing a required passing assertion")


def read_lines(path: Path) -> list[str]:
    return path.read_text(encoding="utf-8").splitlines()


def toc_partition(lines: list[str], extension_tables: set[str]) -> tuple[list[str], list[str], list[str], list[str]]:
    kept: list[str] = []
    acl: list[str] = []
    skipped: list[str] = []
    bootstrap_acl: list[str] = []
    for line in lines:
        if line.lstrip().startswith(";"):
            kept.append(line)
            acl.append(line)
            continue
        extension_item = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+EXTENSION\s+-\s+", line))
        extension_comment = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+COMMENT\s+-\s+EXTENSION\s+", line))
        table_data = re.search(r"\bTABLE DATA\s+\S+\s+(\S+)", line)
        extension_data = bool(table_data and table_data.group(1) in extension_tables)
        acl_entry = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+(?:ACL|DEFAULT ACL)(?:\s+-|\s+)", line))
        schema_acl = bool(re.match(r"^\s*\d+;\s+\d+\s+\d+\s+ACL\s+\S+\s+SCHEMA\s+public\s+", line))
        if extension_item or extension_comment or extension_data:
            skipped.append(line)
        elif schema_acl:
            bootstrap_acl.append(line)
        elif acl_entry:
            acl.append(line)
        else:
            kept.append(line)
    return kept, acl, skipped, bootstrap_acl


def verify_owner_acl(manifest: dict) -> None:
    if manifest.get("owner_acl_effective_full_privileges_checked") is not True:
        raise ValueError("owner effective-privilege verification is missing")
    effective = manifest.get("owner_acl_effective_inventory")
    if not isinstance(effective, dict) or set(effective) != {"source", "target"}:
        raise ValueError("owner effective-privilege inventories are incomplete")
    normalized = {}
    for side in ("source", "target"):
        rows = effective[side]
        if not isinstance(rows, list):
            raise ValueError(f"{side} owner-privilege inventory is invalid")
        seen = set()
        for row in rows:
            if not isinstance(row, dict):
                raise ValueError(f"{side} owner-privilege row is invalid")
            key = (row.get("kind"), row.get("identity"))
            if key in seen:
                raise ValueError(f"duplicate {side} owner-privilege row {key}")
            seen.add(key)
            kind, identity = key
            if kind == "relation":
                object_kinds = {item["identity"]: item["kind"] for item in manifest["catalog"].get("public_object_kinds", [])}
                required = SEQUENCE_PRIVILEGES if object_kinds.get(identity) == "S" else TABLE_PRIVILEGES
            elif kind == "routine":
                required = {"EXECUTE"}
            elif kind == "type":
                required = {"USAGE"}
            elif kind == "schema" and identity == "public":
                required = {"USAGE", "CREATE"}
            elif kind == "database" and identity == "espacigo":
                required = {"CONNECT", "CREATE", "TEMP"}
            else:
                raise ValueError(f"unrecognized owner-privilege object {key}")
            if set(row.get("required_privileges", [])) != required:
                raise ValueError(f"incomplete owner privilege requirement for {side} {identity}")
            if row.get("missing_effective_privileges") != []:
                raise ValueError(f"owner has partial effective privileges on {side} {identity}")
        normalized[side] = [
            {key: value for key, value in row.items() if key != "owner"}
            for row in rows
        ]
    if normalized["source"] != normalized["target"]:
        raise ValueError("source and target effective owner privileges differ")
    if sha256(canonical_json(normalized["source"])) != manifest.get("owner_acl_effective_sha256_equal"):
        raise ValueError("owner effective-privilege hash mismatch")

    object_kinds = {item["identity"]: item["kind"] for item in manifest["catalog"].get("public_object_kinds", [])}
    explicit = manifest.get("explicit_owner_acl_representation")
    if not isinstance(explicit, dict) or set(explicit) != {"source", "target"}:
        raise ValueError("explicit owner ACL inventories are missing")
    key_fields = ("kind", "identity", "object_owner", "grantor", "grantee", "privilege", "grantable")
    owner_sets = {}
    for side in ("source", "target"):
        groups: dict[tuple[str, str, str], set[str]] = {}
        for entry in explicit[side]:
            if not isinstance(entry, dict) or entry.get("object_owner") != entry.get("grantor") or entry.get("grantee") != entry.get("object_owner"):
                raise ValueError(f"non-owner grant in explicit {side} owner ACL representation")
            group = (entry["kind"], entry["identity"], entry["object_owner"])
            groups.setdefault(group, set()).add(entry["privilege"])
        for (kind, identity, _owner), privileges in groups.items():
            if kind == "relation":
                required = SEQUENCE_PRIVILEGES if object_kinds.get(identity) == "S" else TABLE_PRIVILEGES
            elif kind == "column":
                required = COLUMN_PRIVILEGES
            elif kind == "routine":
                required = {"EXECUTE"}
            elif kind == "type":
                required = {"USAGE"}
            else:
                raise ValueError(f"unexpected explicit owner ACL kind {kind}")
            if privileges != required:
                raise ValueError(f"partial or unexpected explicit owner ACL for {side} {identity}: {sorted(privileges)}")
        owner_sets[side] = {
            tuple(entry.get(field) for field in key_fields)
            for entry in explicit[side]
        }
    diffs = {
        "source_only": [entry for entry in explicit["source"] if tuple(entry.get(field) for field in key_fields) not in owner_sets["target"]],
        "target_only": [entry for entry in explicit["target"] if tuple(entry.get(field) for field in key_fields) not in owner_sets["source"]],
    }
    if diffs != manifest.get("explicit_owner_acl_representation_differences"):
        raise ValueError("owner ACL representation differences do not match their explicit inventory")
    if sha256(canonical_json(diffs)) != manifest.get("explicit_owner_acl_representation_differences_sha256"):
        raise ValueError("owner ACL representation difference hash mismatch")
    if sha256(canonical_json(explicit)) != manifest.get("explicit_owner_acl_representation_sha256"):
        raise ValueError("explicit owner ACL inventory hash mismatch")


def verify_bundle(bundle_dir: Path, repo_root: Path, archive_lister: Callable[[Path], bytes] | None = None) -> dict:
    testing_root = (repo_root / ".local/testing").resolve()
    resolved = bundle_dir.resolve(strict=True)
    if resolved.parent != testing_root or not resolved.name.startswith("gcp04-w4-baseline-"):
        raise ValueError("bundle must be in a dedicated gcp04-w4-baseline-* directory under .local/testing")
    if bundle_dir.is_symlink() or not stat.S_ISDIR(resolved.stat().st_mode):
        raise ValueError("bundle path must be a real directory")
    if stat.S_IMODE(resolved.stat().st_mode) != 0o700 or resolved.stat().st_uid != os.getuid():
        raise ValueError("bundle directory must be owned by the current user with mode 0700")
    entries = list(resolved.iterdir())
    expected = FILES | {"SHA256SUMS"}
    if {entry.name for entry in entries} != expected:
        raise ValueError("bundle contains missing or unexpected files")
    for entry in entries:
        info = entry.lstat()
        if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600 or info.st_uid != os.getuid():
            raise ValueError(f"bundle file must be regular, current-user-owned, mode 0600: {entry.name}")

    manifest_path = resolved / "baseline.manifest.json"
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    manifest_format = manifest.get("format")
    if manifest_format not in {FORMAT, LEGACY_FORMAT} or manifest.get("role_definitions_and_memberships_in_dump") is not False:
        raise ValueError("unsupported baseline bundle format or archive role policy")
    body = {key: value for key, value in manifest.items() if key != "manifest_body_sha256"}
    if sha256(canonical_json(body)) != manifest.get("manifest_body_sha256"):
        raise ValueError("baseline manifest body hash mismatch")

    sums_lines = read_lines(resolved / "SHA256SUMS")
    listed = {}
    for line in sums_lines:
        match = re.fullmatch(r"([0-9a-f]{64})  ([A-Za-z0-9._-]+)", line)
        if not match:
            raise ValueError("malformed SHA256SUMS entry")
        listed[match.group(2)] = match.group(1)
    if set(listed) != FILES:
        raise ValueError("SHA256SUMS does not enumerate the exact bundle files")
    for name, digest in listed.items():
        if sha256((resolved / name).read_bytes()) != digest:
            raise ValueError(f"bundle file checksum mismatch: {name}")

    payload_digests = {
        "baseline.dump": "archive_sha256",
        "baseline.toc.original": "toc_original_sha256",
        "baseline.toc.restore": "toc_sha256",
        "baseline.toc.restore.skipped": "toc_skipped_sha256",
        "baseline.toc.bootstrap-acl": "toc_bootstrap_acl_sha256",
        "baseline.toc.acl": "toc_acl_sha256",
        "baseline.acl.raw.sql": "acl_raw_sql_sha256",
        "baseline.acl.restore.sql": "acl_restore_sql_sha256",
        "source.schema.sql": "source_schema_dump_sha256",
        "source.structure.sql": "source_structure_sha256",
        "source.catalog.json": "source_catalog_sha256",
    }
    for filename, field in payload_digests.items():
        if sha256((resolved / filename).read_bytes()) != manifest.get(field):
            raise ValueError(f"manifest payload hash mismatch: {filename}")

    migrations = manifest.get("migrations", {})
    files = migrations.get("canonical_files", [])
    ledger = migrations.get("source_ledger", [])
    if migrations.get("count") != 64 or migrations.get("first") != 1 or migrations.get("last") != 64 or migrations.get("source_target_checksums_equal") is not True:
        raise ValueError("manifest does not describe the canonical V1-V64 history")
    if len(files) != 64 or len(ledger) != 64:
        raise ValueError("manifest must enumerate all 64 canonical files and ledger rows")
    migration_root = repo_root / "db/migrations"
    for version, (file_entry, ledger_entry) in enumerate(zip(files, ledger, strict=True), start=1):
        expected_prefix = f"V{version:06d}__"
        if not file_entry["name"].startswith(expected_prefix) or ledger_entry.get("version") != version or ledger_entry.get("name") != file_entry["name"]:
            raise ValueError(f"migration inventory is not contiguous at V{version:06d}")
        if not SHA256.fullmatch(file_entry.get("sha256", "")) or not SHA256.fullmatch(ledger_entry.get("checksum", "")):
            raise ValueError(f"migration checksum is malformed at V{version:06d}")
        source_path = migration_root / file_entry["name"]
        if source_path.is_symlink() or not source_path.is_file():
            raise ValueError(f"canonical migration file is missing or symlinked: {file_entry['name']}")
        digest = sha256(source_path.read_bytes())
        if digest != file_entry["sha256"] or digest != ledger_entry["checksum"]:
            raise ValueError(f"canonical migration/ledger checksum mismatch at V{version:06d}")
    if sha256(canonical_json(ledger)) != manifest.get("migration_history_sha256_equal"):
        raise ValueError("canonical migration ledger hash mismatch")

    extension_lines = manifest.get("extension_config_tables")
    if not isinstance(extension_lines, list):
        raise ValueError("extension configuration table inventory is missing")
    extension_tables = {line.split(".", 1)[1] for line in extension_lines if "." in line}
    original = read_lines(resolved / "baseline.toc.original")
    if any(re.search(r"\b\d+;\s+\d+\s+\d+\s+(?:ROLE|USER)\s+", line, re.I) for line in original):
        raise ValueError("database archive TOC unexpectedly contains global role entries")
    kept, acl, skipped, bootstrap_acl = toc_partition(original, extension_tables)
    if kept != read_lines(resolved / "baseline.toc.restore"):
        raise ValueError("restore TOC is not the approved exact projection of original TOC")
    if acl != read_lines(resolved / "baseline.toc.acl"):
        raise ValueError("ACL TOC is not the exact original ACL projection")
    if skipped != read_lines(resolved / "baseline.toc.restore.skipped") or skipped != manifest.get("toc_entries_skipped"):
        raise ValueError("skipped TOC entries differ from their manifest inventory")
    if bootstrap_acl != read_lines(resolved / "baseline.toc.bootstrap-acl") or bootstrap_acl != manifest.get("toc_bootstrap_acl_entries"):
        raise ValueError("bootstrap ACL TOC entries differ from their explicit inventory")
    if any("SESSION AUTHORIZATION" in line.upper() for line in read_lines(resolved / "baseline.acl.restore.sql")):
        raise ValueError("restored ACL SQL still contains SESSION AUTHORIZATION")
    if "SET ROLE espacigo_schema_owner;" not in (resolved / "baseline.acl.restore.sql").read_text(encoding="utf-8"):
        raise ValueError("restored ACL SQL has no explicit schema-owner context")

    catalog_file = json.loads((resolved / "source.catalog.json").read_text(encoding="utf-8"))
    if sha256((resolved / "source.catalog.json").read_bytes()) != manifest.get("source_catalog_sha256"):
        raise ValueError("source catalog file hash mismatch")
    if manifest.get("catalog") != catalog_file:
        raise ValueError("source catalog does not match the canonical manifest")
    if manifest.get("catalog_manifest_sha256_equal") != sha256(canonical_json(catalog_file)):
        raise ValueError("catalog manifest hash mismatch")
    if manifest_format == FORMAT or "gadm_bootstrap" in manifest:
        verify_gadm_bootstrap_manifest(manifest, catalog_file)
    else:
        gadm = next((role for role in catalog_file.get("roles", []) if role.get("name") == "espacigo_gadm_bootstrap"), None)
        if gadm is None or gadm.get("login") is not False or gadm.get("super") is not False or gadm.get("comment") != GADM_COMMENT:
            raise ValueError("legacy GADM bootstrap role comment/attributes are not represented in the retained baseline")
    memberships = catalog_file.get("memberships") or []
    if any(edge.get("admin") or edge.get("granted") == "espacigo_gadm_bootstrap" or edge.get("member") == "espacigo_gadm_bootstrap" for edge in memberships):
        raise ValueError("baseline role membership contains ADMIN OPTION or a GADM membership")
    verify_owner_acl(manifest)

    lister = archive_lister or default_archive_lister
    if lister(resolved / "baseline.dump") != (resolved / "baseline.toc.original").read_bytes():
        raise ValueError("native pg_restore TOC differs from the retained original TOC")
    return {
        "format": FORMAT,
        "manifest_body_sha256": manifest["manifest_body_sha256"],
        "archive_sha256": manifest["archive_sha256"],
        "migration_history_sha256": manifest["migration_history_sha256_equal"],
        "table_count": len(manifest.get("tables", {})),
        "explicit_owner_acl_diff_entries": sum(len(rows) for rows in manifest["explicit_owner_acl_representation_differences"].values()),
        "validated": True,
    }


def default_archive_lister(path: Path) -> bytes:
    environment = os.environ.copy()
    environment["TZ"] = "UTC"
    result = subprocess.run(["pg_restore", "--list", str(path)], check=False, capture_output=True, env=environment)
    if result.returncode:
        raise ValueError("native pg_restore could not read the retained archive")
    return result.stdout


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("bundle_dir", type=Path)
    parser.add_argument("--repo-root", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    try:
        result = verify_bundle(args.bundle_dir, args.repo_root)
    except (OSError, ValueError, KeyError, TypeError, json.JSONDecodeError) as exc:
        print(f"baseline bundle rejected: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(result, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
