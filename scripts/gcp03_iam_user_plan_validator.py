#!/usr/bin/env python3
"""Validate and safely summarize staged GCP-03 IAM SQL-user Terraform plans."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any


TARGETS = (
    "google_sql_user.restore_iam[0]",
    "google_sql_user.verify_iam[0]",
)


class PlanValidationError(ValueError):
    """Raised when a closeout plan contains an unapproved action shape."""


def _without_deletion_policy(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: item for key, item in value.items() if key != "deletion_policy"}
    if isinstance(value, list):
        return [_without_deletion_policy(item) for item in value]
    return value


def validate_plan(plan: dict[str, Any], phase: str) -> dict[str, Any]:
    """Validate a stage0 policy update or stage1 deletion plan.

    Stage 1 is accepted only when both resources have before.deletion_policy=DELETE.
    The function returns a non-sensitive summary and never copies resource values.
    """
    if phase not in {"stage0", "stage1"}:
        raise PlanValidationError("phase must be stage0 or stage1")
    if plan.get("applyable") is not True or plan.get("complete") is not False:
        raise PlanValidationError("expected an applyable, intentionally targeted partial plan")

    target_changes: dict[str, dict[str, Any]] = {}
    for change in plan.get("resource_changes", []):
        address = change.get("address")
        actions = change.get("change", {}).get("actions", [])
        if actions == ["no-op"]:
            continue
        if address not in TARGETS:
            raise PlanValidationError(f"unexpected mutating resource: {address}")
        if address in target_changes:
            raise PlanValidationError(f"duplicate resource change: {address}")
        target_changes[address] = change

    if set(target_changes) != set(TARGETS):
        raise PlanValidationError("plan must contain both and only the two target resource changes")

    expected_action = ["update"] if phase == "stage0" else ["delete"]
    summaries = []
    for address in TARGETS:
        change = target_changes[address].get("change", {})
        if change.get("actions") != expected_action:
            raise PlanValidationError(f"{address} must have action {expected_action}")
        before = change.get("before")
        if not isinstance(before, dict):
            raise PlanValidationError(f"{address} is missing a prior resource value")
        before_policy = before.get("deletion_policy")

        if phase == "stage0":
            after = change.get("after")
            if before_policy != "PREVENT" or not isinstance(after, dict):
                raise PlanValidationError(f"{address} must update deletion_policy PREVENT to DELETE")
            if after.get("deletion_policy") != "DELETE":
                raise PlanValidationError(f"{address} after.deletion_policy must be DELETE")
            if _without_deletion_policy(before) != _without_deletion_policy(after):
                raise PlanValidationError(f"{address} stage0 may change deletion_policy only")
            after_policy = after.get("deletion_policy")
        else:
            if before_policy != "DELETE":
                raise PlanValidationError(
                    f"refusing delete for {address}: before.deletion_policy must be DELETE"
                )
            after_policy = None

        summaries.append(
            {"address": address, "actions": expected_action, "before_deletion_policy": before_policy,
             "after_deletion_policy": after_policy}
        )

    return {
        "phase": phase,
        "applyable": True,
        "complete": False,
        "summary": {"add": 0, "change": 2 if phase == "stage0" else 0,
                    "delete": 2 if phase == "stage1" else 0},
        "resource_changes": summaries,
    }


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--phase", choices=("stage0", "stage1"), required=True)
    parser.add_argument("plan_json", type=Path)
    args = parser.parse_args()

    try:
        with args.plan_json.open(encoding="utf-8") as plan_file:
            plan = json.load(plan_file)
        result = validate_plan(plan, args.phase)
    except (OSError, json.JSONDecodeError, PlanValidationError) as error:
        print(f"REJECTED: {error}", file=sys.stderr)
        return 1

    print(json.dumps(result, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
