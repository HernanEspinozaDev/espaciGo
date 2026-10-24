#!/usr/bin/env python3
"""Validate an approved, read-only IMG-01 schema report against this checkout.

This checks report integrity and declared contents; it does not authenticate
the database connection or prove report provenance. A matching GCS generation
and digest identify an immutable operator-supplied attestation, not a signed
Cloud SQL observation.
"""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys

FORMAT = "espacigo.img01.schema-evidence.v1"
PROJECT = "espacigo-testprod"
INSTANCE = "espacigo-testprod:southamerica-west1:espacigo-testprod-db"
DATABASE = "espacigo_dms_stage_20261020"
ROLE_FLAGS = ("superuser", "createdb", "createrole", "replication", "bypassrls")
REQUIRED_RELATIONS = {
    "media_asset",
    "media_variant",
    "media_variant_intent",
    "space_media",
    "gallery_item_order",
    "rol_usuario_evento",
}
REQUIRED_ROUTINES = {
    ("claim_private_media_cleanup", "integer, timestamp with time zone"),
    ("complete_private_media_cleanup", "uuid, bigint, timestamp with time zone"),
    ("fail_private_media_cleanup", "uuid, bigint, timestamp with time zone"),
    ("finalize_pending_suppression_cleanup", "uuid, timestamp with time zone"),
}
RUNTIME_TRUE = {
    "database_connect",
    "public_usage",
    "space_media_select",
    "space_media_insert",
    "space_media_update",
    "space_media_delete",
    "role_event_insert",
}
RUNTIME_FALSE = {
    "database_create",
    "database_temporary",
    "public_create",
    "ledger_select",
    "ledger_insert",
    "ledger_update",
    "ledger_delete",
}
WORKER_TRUE = {
    "claim_execute",
    "complete_execute",
    "fail_execute",
    "finalize_execute",
    "media_asset_select",
    "media_variant_select",
    "media_intent_select",
}
WORKER_FALSE = {
    "media_asset_table_update",
    "media_variant_table_delete",
    "media_intent_table_delete",
}


class EvidenceError(ValueError):
    pass


def require(condition: bool, message: str) -> None:
    if not condition:
        raise EvidenceError(message)


def parse_time(value: object) -> datetime:
    require(isinstance(value, str), "captured_at must be an ISO-8601 string")
    try:
        parsed = datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError as exc:
        raise EvidenceError("captured_at is not valid ISO-8601") from exc
    require(parsed.tzinfo is not None, "captured_at must include a timezone")
    return parsed.astimezone(timezone.utc)


def migration_inventory(directory: Path) -> list[dict[str, object]]:
    pattern = re.compile(r"^V([0-9]{6})__[a-z0-9]+(?:_[a-z0-9]+)*\.sql$")
    files: list[dict[str, object]] = []
    for path in sorted(directory.glob("*.sql")):
        match = pattern.fullmatch(path.name)
        require(match is not None, f"invalid migration filename: {path.name}")
        files.append(
            {
                "version": int(match.group(1)),
                "name": path.name,
                "checksum": hashlib.sha256(path.read_bytes()).hexdigest(),
            }
        )
    require(bool(files), "migration inventory is empty")
    require([item["version"] for item in files] == list(range(1, len(files) + 1)), "local migration versions are not contiguous from V1")
    return files


def validate(
    path: Path,
    migrations_dir: Path,
    query_path: Path,
    now: datetime | None = None,
    expected_captured_at: str | None = None,
) -> dict[str, object]:
    try:
        evidence = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise EvidenceError("schema evidence is not readable JSON") from exc
    require(isinstance(evidence, dict), "schema evidence root must be an object")
    required_keys = {
        "format", "captured_at", "target", "query_sha256", "migration_ledger",
        "relations", "routines", "roles", "memberships", "runtime_privileges", "worker_privileges",
    }
    require(set(evidence) == required_keys, "schema evidence fields differ from the reviewed contract")
    require(evidence["format"] == FORMAT, "unsupported schema evidence format")

    captured_at = parse_time(evidence["captured_at"])
    if expected_captured_at is not None:
        require(
            evidence["captured_at"] == expected_captured_at,
            "report captured_at differs from the checked-in attestation",
        )
    current_time = (now or datetime.now(timezone.utc)).astimezone(timezone.utc)
    age_seconds = (current_time - captured_at).total_seconds()
    require(age_seconds >= -120, "schema evidence capture time is too far in the future")

    expected_query_sha = hashlib.sha256(query_path.read_bytes()).hexdigest()
    require(evidence["query_sha256"] == expected_query_sha, "evidence query hash does not match the reviewed SQL source")
    target = evidence["target"]
    require(isinstance(target, dict), "target must be an object")
    require(
        target.get("project_id") == PROJECT
        and target.get("instance_connection_name") == INSTANCE
        and target.get("database") == DATABASE,
        "schema evidence target differs from the reviewed Cloud SQL database",
    )
    require(str(target.get("server_version_num", "")).startswith("18"), "schema evidence is not from PostgreSQL 18")
    require(target.get("transaction_read_only") == "on", "schema evidence query did not run in a read-only transaction")
    for key in ("session_user", "current_user", "database_owner", "public_schema_owner", "schema_migrations_owner"):
        require(isinstance(target.get(key), str) and target[key], f"target {key} is missing")
    require(target.get("schema_migrations_kind") in {"r", "p"}, "schema_migrations must be a real table")
    require(target["current_user"] not in {"espacigo_runtime", "espacigo_api_login"}, "schema evidence was collected as the application role")

    expected_ledger = migration_inventory(migrations_dir)
    ledger = evidence["migration_ledger"]
    require(isinstance(ledger, dict), "migration_ledger must be an object")
    require(ledger.get("count") == len(expected_ledger), "database migration ledger count differs from this checkout")
    require(ledger.get("first_version") == 1 and ledger.get("last_version") == len(expected_ledger), "database migration ledger range is incomplete")
    actual_ledger = ledger.get("entries")
    require(actual_ledger == expected_ledger, "database migration ledger has a gap, rename, or checksum drift")

    relations = evidence["relations"]
    require(isinstance(relations, list), "relations must be an array")
    relation_map = {item.get("name"): item for item in relations if isinstance(item, dict)}
    require(set(relation_map) == REQUIRED_RELATIONS, "required IMG-01 relations are missing or unexpected")
    for name, relation in relation_map.items():
        require(relation.get("kind") in {"r", "p"}, f"required relation {name} is not a table")
        owner = relation.get("owner")
        require(isinstance(owner, str) and owner, f"required relation {name} has no owner")
        require(owner not in {"espacigo_api_login", "espacigo_runtime", "espacigo_privacy_cleanup_worker"}, f"application role owns {name}")

    routines = evidence["routines"]
    require(isinstance(routines, list), "routines must be an array")
    routine_map = {(item.get("name"), item.get("arguments")): item for item in routines if isinstance(item, dict)}
    require(set(routine_map) == REQUIRED_ROUTINES, "required privacy cleanup routines are missing or have changed signatures")
    for (name, _), routine in routine_map.items():
        owner = routine.get("owner")
        require(isinstance(owner, str) and owner, f"routine {name} has no owner")
        require(owner not in {"espacigo_api_login", "espacigo_runtime", "espacigo_privacy_cleanup_worker"}, f"application role owns routine {name}")
        require(routine.get("security_definer") is True, f"routine {name} must remain SECURITY DEFINER")
        require(routine.get("public_execute_revoked") is True, f"PUBLIC must not execute routine {name}")
        config = {str(item).replace(" ", "").lower() for item in routine.get("configuration", [])}
        require("search_path=pg_catalog,public" in config, f"routine {name} must pin search_path")
        require(any(item in {"timezone=utc", "timezone='utc'"} for item in config), f"routine {name} must pin timezone")

    roles = evidence["roles"]
    require(isinstance(roles, list), "roles must be an array")
    role_map = {item.get("name"): item for item in roles if isinstance(item, dict)}
    for name in ("espacigo_api_login", "espacigo_runtime", "espacigo_privacy_cleanup_worker"):
        require(name in role_map, f"required database role {name} is missing")
    api_login = role_map["espacigo_api_login"]
    runtime = role_map["espacigo_runtime"]
    worker = role_map["espacigo_privacy_cleanup_worker"]
    require(api_login.get("can_login") is True, "API database login must remain LOGIN")
    for role_name, role, no_login in (("API login", api_login, False), ("runtime", runtime, True), ("cleanup worker", worker, True)):
        require(role.get("can_login") is not no_login, f"{role_name} login attribute is unsafe")
        require(all(role.get(flag) is False for flag in ROLE_FLAGS), f"{role_name} has an elevated PostgreSQL attribute")
    require(runtime.get("inherit") is False and worker.get("inherit") is False, "NOLOGIN application groups must remain NOINHERIT")

    memberships = evidence["memberships"]
    require(isinstance(memberships, list), "memberships must be an array")
    api_edges = [item for item in memberships if isinstance(item, dict) and item.get("member") == "espacigo_api_login"]
    require(len(api_edges) == 1, "API login must have exactly one recorded membership")
    edge = api_edges[0]
    require(edge.get("granted_role") == "espacigo_runtime" and edge.get("set_option") is True and edge.get("admin_option") is False,
            "API login must have SET-only membership in the runtime role")
    require(not any(isinstance(item, dict) and item.get("member") in {"espacigo_runtime", "espacigo_privacy_cleanup_worker"} for item in memberships),
            "runtime and cleanup worker must not have role memberships")

    worker_edges = [item for item in memberships if isinstance(item, dict) and item.get("granted_role") == "espacigo_privacy_cleanup_worker"]
    require(len(worker_edges) <= 1, "cleanup worker has unexpected incoming memberships")
    for edge in worker_edges:
        require(edge.get("member") == "postgres" and edge.get("grantor_oid") == 10
                and edge.get("admin_option") is True and edge.get("inherit_option") is False
                and edge.get("set_option") is False,
                "cleanup worker may only have the bootstrap-granted creator ADMIN membership without SET/INHERIT")

    runtime_privileges = evidence["runtime_privileges"]
    worker_privileges = evidence["worker_privileges"]
    require(isinstance(runtime_privileges, dict) and isinstance(worker_privileges, dict), "privilege reports must be objects")
    require(all(runtime_privileges.get(key) is True for key in RUNTIME_TRUE), "runtime lacks a required application privilege")
    require(all(runtime_privileges.get(key) is False for key in RUNTIME_FALSE), "runtime has a forbidden schema/ledger privilege")
    require(all(worker_privileges.get(key) is True for key in WORKER_TRUE), "cleanup worker lacks a required restricted privilege")
    require(all(worker_privileges.get(key) is False for key in WORKER_FALSE), "cleanup worker has broad media table DML privileges")
    return evidence


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("evidence", type=Path)
    parser.add_argument("--migrations", type=Path, default=Path("db/migrations"))
    parser.add_argument("--query", type=Path, default=Path("db/query/img01_schema_release_evidence.sql"))
    parser.add_argument("--expected-captured-at")
    args = parser.parse_args()
    try:
        report = validate(
            args.evidence,
            args.migrations,
            args.query,
            expected_captured_at=args.expected_captured_at,
        )
    except EvidenceError as exc:
        print(f"schema evidence rejected: {exc}", file=sys.stderr)
        return 1
    print(
        "versioned schema release attestation validated: "
        f"database={report['target']['database']} "
        f"migrations={report['migration_ledger']['count']} "
        f"captured_at={report['captured_at']}"
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
