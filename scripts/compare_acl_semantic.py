#!/usr/bin/env python3
"""Compare two PostgreSQL semantic ACL snapshots without learning a baseline."""
from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

ACL_COLUMNS = [
    "pg_attribute.attacl", "pg_class.relacl", "pg_database.datacl",
    "pg_default_acl.defaclacl", "pg_foreign_data_wrapper.fdwacl",
    "pg_foreign_server.srvacl", "pg_init_privs.initprivs", "pg_language.lanacl",
    "pg_largeobject_metadata.lomacl", "pg_namespace.nspacl", "pg_parameter_acl.paracl",
    "pg_proc.proacl", "pg_tablespace.spcacl", "pg_type.typacl",
]
OWNER_COLUMNS = [
    "pg_class.relowner", "pg_collation.collowner", "pg_conversion.conowner",
    "pg_database.datdba", "pg_event_trigger.evtowner", "pg_extension.extowner",
    "pg_foreign_data_wrapper.fdwowner", "pg_foreign_server.srvowner",
    "pg_language.lanowner", "pg_largeobject_metadata.lomowner", "pg_namespace.nspowner",
    "pg_opclass.opcowner", "pg_operator.oprowner", "pg_opfamily.opfowner",
    "pg_proc.proowner", "pg_publication.pubowner", "pg_statistic_ext.stxowner",
    "pg_subscription.subowner", "pg_tablespace.spcowner", "pg_ts_config.cfgowner",
    "pg_ts_dict.dictowner", "pg_type.typowner",
]

class SnapshotError(ValueError):
    pass


def _canonical(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: _canonical(value[key]) for key in sorted(value)}
    if isinstance(value, list):
        items = [_canonical(item) for item in value]
        return sorted(items, key=lambda item: json.dumps(item, sort_keys=True, separators=(",", ":"), ensure_ascii=False))
    if isinstance(value, float):
        raise SnapshotError("floating point values are not valid in semantic snapshots")
    return value


def _validate(snapshot: Any, label: str) -> dict[str, Any]:
    if not isinstance(snapshot, dict):
        raise SnapshotError(f"{label}: snapshot root must be an object")
    required = {"database", "server_version_num", "acl_inventory", "owner_inventory", "owners", "acls", "initial_privileges", "default_acls", "acl_states", "default_acl_objects"}
    if set(snapshot) != required:
        raise SnapshotError(f"{label}: fields differ from the required snapshot contract")
    if not isinstance(snapshot["database"], str) or snapshot["database"] != "espacigo_f5_gate_20261101":
        raise SnapshotError(f"{label}: wrong database")
    if type(snapshot["server_version_num"]) is not int or snapshot["server_version_num"] != 180006:
        raise SnapshotError(f"{label}: PostgreSQL server version must be 180006")
    if not isinstance(snapshot["acl_inventory"], list) or any(not isinstance(x, str) for x in snapshot["acl_inventory"]) or len(snapshot["acl_inventory"]) != len(set(snapshot["acl_inventory"])) or sorted(snapshot["acl_inventory"]) != ACL_COLUMNS:
        raise SnapshotError(f"{label}: aclitem[] catalog inventory mismatch")
    if not isinstance(snapshot["owner_inventory"], list) or any(not isinstance(x, str) for x in snapshot["owner_inventory"]) or len(snapshot["owner_inventory"]) != len(set(snapshot["owner_inventory"])) or sorted(snapshot["owner_inventory"]) != OWNER_COLUMNS:
        raise SnapshotError(f"{label}: owner catalog inventory mismatch")
    for field in ("owners", "acls", "initial_privileges", "default_acls", "acl_states", "default_acl_objects"):
        if not isinstance(snapshot[field], list):
            raise SnapshotError(f"{label}: {field} must be an array")
    # OIDs are prohibited structurally: every row is an allowlisted semantic record,
    # with object identities supplied by pg_identify_object, never catalog identifiers.
    for field, keys in {
        "owners": {"class", "identity", "owner"},
        "acls": {"class", "identity", "column", "grantee", "grantor", "privilege", "grantable", "source"},
        "initial_privileges": {"class", "identity", "column", "grantee", "grantor", "privilege", "grantable", "source"},
        "default_acls": {"role", "schema", "object_type", "grantee", "grantor", "privilege", "grantable"},
        "acl_states": {"class", "identity", "column", "source", "state"},
        "default_acl_objects": {"role", "schema", "object_type", "state"},
    }.items():
        for row in snapshot[field]:
            if not isinstance(row, dict) or set(row) != keys:
                raise SnapshotError(f"{label}: malformed {field} semantic row")
            if any("oid" in key.lower() for key in row):
                raise SnapshotError(f"{label}: OID-bearing fields are forbidden")
            if field in ("acls", "initial_privileges", "default_acls"):
                for key in ("grantee", "grantor", "privilege"):
                    if not isinstance(row[key], str) or not row[key]:
                        raise SnapshotError(f"{label}: {key} must be a non-empty string")
                if not isinstance(row["grantable"], bool):
                    raise SnapshotError(f"{label}: grantable must be boolean")
            if field == "acls" or field == "initial_privileges":
                if row["source"] not in ("effective", "initial", "initial:e", "initial:i") or not isinstance(row["class"], str) or not isinstance(row["identity"], str) or not isinstance(row["column"], str):
                    raise SnapshotError(f"{label}: invalid ACL identity/source")
            if field == "default_acls" and (row["object_type"] not in ("r", "S", "f", "F", "T", "n", "L", "p") or any(not isinstance(row[k], str) for k in ("role", "schema"))):
                raise SnapshotError(f"{label}: invalid default ACL row")
            if field == "owners" and any(not isinstance(row[k], str) or not row[k] for k in ("class", "identity", "owner")):
                raise SnapshotError(f"{label}: invalid owner row")
            if field == "acl_states" and (row["state"] not in ("null", "empty", "explicit") or any(not isinstance(row[k], str) for k in ("class", "identity", "column", "source"))):
                raise SnapshotError(f"{label}: invalid ACL state")
            if field == "default_acl_objects" and (row["state"] not in ("empty", "explicit") or any(not isinstance(row[k], str) for k in ("role", "schema", "object_type"))):
                raise SnapshotError(f"{label}: invalid default ACL object")
    return snapshot


def compare_snapshots(reference: Any, target: Any) -> None:
    left = _canonical(_validate(reference, "reference"))
    right = _canonical(_validate(target, "target"))
    if left != right:
        raise SnapshotError("semantic ACL snapshots differ")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reference", type=Path)
    parser.add_argument("target", type=Path)
    args = parser.parse_args(argv)
    try:
        compare_snapshots(json.loads(args.reference.read_text()), json.loads(args.target.read_text()))
    except (OSError, json.JSONDecodeError, SnapshotError) as exc:
        print(f"ACL comparison failed: {exc}", file=sys.stderr)
        return 1
    print("semantic ACL snapshots match")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
