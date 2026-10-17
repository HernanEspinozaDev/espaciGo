#!/usr/bin/env python3
"""Validate a complete plan that lowers only GCP-03 run resource protections."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


EXPECTED: dict[str, dict[str, tuple[Any, Any]]] = {
    "google_sql_database_instance.restore[0]": {
        "deletion_protection": (True, False),
        "deletion_policy": ("PREVENT", "DELETE"),
        "settings.0.deletion_protection_enabled": (True, False),
    },
    "google_sql_database.restore[0]": {
        "deletion_policy": ("PREVENT", "DELETE"),
    },
    "google_secret_manager_secret.postgres_password[0]": {
        "deletion_protection": (True, False),
        "deletion_policy": ("PREVENT", "DELETE"),
    },
    'google_cloud_run_v2_job.admin_phase["bootstrap"]': {
        "deletion_protection": (True, False),
    },
    'google_cloud_run_v2_job.admin_phase["retire"]': {
        "deletion_protection": (True, False),
    },
    "google_cloud_run_v2_job.diagnose[0]": {
        "deletion_protection": (True, False),
    },
    "google_cloud_run_v2_job.restore[0]": {
        "deletion_protection": (True, False),
    },
    "google_cloud_run_v2_job.retirement_diagnose[0]": {
        "deletion_protection": (True, False),
    },
    "google_cloud_run_v2_job.verify[0]": {
        "deletion_protection": (True, False),
    },
}


class PlanValidationError(ValueError):
    """Raised when a run-protection plan exceeds the approved boundary."""


_MISSING = object()


def _leaf_values(value: Any, prefix: str = "") -> dict[str, Any]:
    if isinstance(value, dict):
        result: dict[str, Any] = {}
        for key, child in value.items():
            path = f"{prefix}.{key}" if prefix else key
            result.update(_leaf_values(child, path))
        return result
    if isinstance(value, list):
        result = {}
        for index, child in enumerate(value):
            path = f"{prefix}.{index}" if prefix else str(index)
            result.update(_leaf_values(child, path))
        return result
    return {prefix: value}


def _changed_values(change: dict[str, Any]) -> dict[str, tuple[Any, Any]]:
    before = _leaf_values(change.get("before"))
    after = _leaf_values(change.get("after"))
    return {
        path: (before.get(path, _MISSING), after.get(path, _MISSING))
        for path in sorted(set(before) | set(after))
        if before.get(path, _MISSING) != after.get(path, _MISSING)
    }


def validate_plan(plan: dict[str, Any]) -> dict[str, Any]:
    if plan.get("complete") is not True or plan.get("applyable") is not True:
        raise PlanValidationError("expected a complete and applyable full plan")

    variables = plan.get("variables", {})
    required_variables = {
        "project_id": "espacigo-testprod",
        "run_id": "g03r-1016",
        "enable_restore_iam_users": False,
        "allow_restore_iam_user_destroy": True,
        "allow_restore_destroy": True,
        "confirm_destroy_run_id": "g03r-1016",
        "enable_restore_jobs": True,
    }
    for name, expected_value in required_variables.items():
        actual = variables.get(name, {}).get("value")
        if actual != expected_value:
            raise PlanValidationError(f"unexpected value for required variable {name}")

    changes_by_address: dict[str, dict[str, Any]] = {}
    for resource in plan.get("resource_changes", []):
        address = resource.get("address")
        actions = resource.get("change", {}).get("actions", [])
        if actions == ["no-op"]:
            continue
        if address not in EXPECTED:
            raise PlanValidationError(f"unexpected mutating resource: {address}")
        if address in changes_by_address:
            raise PlanValidationError(f"duplicate resource change: {address}")
        changes_by_address[address] = resource

    if set(changes_by_address) != set(EXPECTED):
        missing = sorted(set(EXPECTED) - set(changes_by_address))
        raise PlanValidationError(f"expected exactly the nine remaining protected resources; missing {missing}")

    summaries = []
    for address, expected_diffs in EXPECTED.items():
        change = changes_by_address[address].get("change", {})
        if change.get("actions") != ["update"]:
            raise PlanValidationError(f"{address} must be updated in place")
        actual_diffs = _changed_values(change)
        if actual_diffs != expected_diffs:
            raise PlanValidationError(f"{address} changes differ from the approved protection-only fields")
        summaries.append({"address": address, "actions": ["update"], "changed_attributes": sorted(expected_diffs)})

    return {
        "schema": "espacigo.gcp03.run-protection-plan-validation.v1",
        "complete": True,
        "applyable": True,
        "applied": False,
        "summary": {"add": 0, "change": len(EXPECTED), "delete": 0},
        "resource_changes": summaries,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("plan_json", type=Path)
    args = parser.parse_args()
    try:
        with args.plan_json.open(encoding="utf-8") as plan_file:
            summary = validate_plan(json.load(plan_file))
    except (OSError, json.JSONDecodeError, PlanValidationError) as error:
        print(f"REJECTED: {error}", file=sys.stderr)
        return 1
    print(json.dumps(summary, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
