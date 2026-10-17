#!/usr/bin/env python3
"""Validate public GCP-03 aggregate evidence and reject duplicate JSON keys."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path
from typing import Any


EVIDENCE = (
    Path(__file__).resolve().parent.parent
    / "planning/gcp/evidence/gcp03-origin-snapshot-prep-20261016/aggregate-inventory.json"
)
FORBIDDEN_KEY_PARTS = (
    "email",
    "password",
    "secret_value",
    "key_payload",
    "relative_path",
    "source_path",
    "acl_text",
    "role_names",
    "credential_value",
)


def unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def walk(value: Any):
    if isinstance(value, dict):
        for key, child in value.items():
            yield key, child
            yield from walk(child)
    elif isinstance(value, list):
        for child in value:
            yield from walk(child)


def validate(document: dict[str, Any]) -> None:
    try:
        json.loads('{"outer":{"duplicate":1,"duplicate":2}}', object_pairs_hook=unique_object)
    except ValueError as exc:
        assert "duplicate JSON key" in str(exc)
    else:
        raise AssertionError("nested duplicate JSON keys were accepted")

    assert document["format"] == "espacigo.gcp03.origin-snapshot-prep.aggregate.v1"
    source = document["source"]
    assert source["database_repeatable_read_read_only_observation"] is True
    assert source["database_and_files_observed_atomically"] is False
    assert source["dump_or_copy_created"] is False
    assert source["cloud_touched"] is False
    assert source["backend_worktree_head_proves_running_image_code"] is False
    assert source["database_connection_correlated_with_backend_runtime"] is False

    schema = document["schema"]
    assert schema["migration_count"] == 61
    assert schema["migration_files_match_database_history_v1_v61"] is True
    for field, value in schema.items():
        if field.endswith("_count"):
            assert type(value) is int and value >= 0

    rows = document["selected_exact_row_counts"]
    assert rows and all(type(count) is int and count >= 0 for count in rows.values())
    assert rows["aviso_local"] == document["copy_safety_aggregates"]["pending_notices"] + document["copy_safety_aggregates"]["terminal_failure_notices"] + document["copy_safety_aggregates"]["delivered_notices"]
    assert document["copy_safety_aggregates"]["pending_notices"] > 0

    forbidden = [key for key, _ in walk(document) if any(part in key.lower() for part in FORBIDDEN_KEY_PARTS)]
    assert not forbidden, f"sensitive field names found: {forbidden}"
    encoded = json.dumps(document, ensure_ascii=False)
    assert not re.search(r"[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}", encoded, re.IGNORECASE)


def main() -> int:
    try:
        document = json.loads(EVIDENCE.read_text(encoding="utf-8"), object_pairs_hook=unique_object)
        if not isinstance(document, dict):
            raise ValueError("aggregate root must be a JSON object")
        validate(document)
    except (OSError, json.JSONDecodeError, ValueError, AssertionError, KeyError) as exc:
        print(f"aggregate evidence validation failed: {exc}", file=sys.stderr)
        return 1
    print("aggregate JSON duplicate-key, contract and sensitive-field checks: PASS")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
