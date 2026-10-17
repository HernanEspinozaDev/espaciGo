#!/usr/bin/env python3
"""Unit tests for GCP-03 run-protection plan validation."""

from __future__ import annotations

import copy
import unittest

from gcp03_run_protection_plan_validator import EXPECTED, PlanValidationError, validate_plan


def _path_set(value: dict, dotted_path: str, replacement: object) -> None:
    parts = dotted_path.split(".")
    target = value
    for index, part in enumerate(parts[:-1]):
        next_part = parts[index + 1]
        if isinstance(target, list):
            while len(target) <= int(part):
                target.append(None)
            if target[int(part)] is None:
                target[int(part)] = [] if next_part.isdigit() else {}
            target = target[int(part)]
        else:
            if part not in target:
                target[part] = [] if next_part.isdigit() else {}
            target = target[part]
    last = parts[-1]
    if isinstance(target, list):
        target[int(last)] = replacement
    else:
        target[last] = replacement


def valid_plan() -> dict:
    resources = []
    for address, diffs in EXPECTED.items():
        before: dict = {}
        for path, (old, _) in diffs.items():
            _path_set(before, path, old)
        after = copy.deepcopy(before)
        for path, (_, new) in diffs.items():
            _path_set(after, path, new)
        resources.append({"address": address, "change": {"actions": ["update"], "before": before, "after": after}})
    variables = {
        "project_id": "espacigo-testprod",
        "run_id": "g03r-1016",
        "enable_restore_iam_users": False,
        "allow_restore_iam_user_destroy": True,
        "allow_restore_destroy": True,
        "confirm_destroy_run_id": "g03r-1016",
        "enable_restore_jobs": True,
    }
    return {
        "complete": True,
        "applyable": True,
        "variables": {key: {"value": value} for key, value in variables.items()},
        "resource_changes": resources,
    }


class RunProtectionPlanValidatorTest(unittest.TestCase):
    def test_accepts_exact_complete_plan(self) -> None:
        summary = validate_plan(valid_plan())
        self.assertEqual(summary["summary"], {"add": 0, "change": 9, "delete": 0})

    def test_rejects_partial_target_plan(self) -> None:
        plan = valid_plan()
        plan["complete"] = False
        with self.assertRaises(PlanValidationError):
            validate_plan(plan)

    def test_rejects_resource_outside_run_protection_scope(self) -> None:
        plan = valid_plan()
        plan["resource_changes"].append({"address": "google_sql_user.restore_iam[0]", "change": {"actions": ["create"]}})
        with self.assertRaisesRegex(PlanValidationError, "unexpected mutating resource"):
            validate_plan(plan)

    def test_rejects_extra_attribute_change(self) -> None:
        plan = valid_plan()
        plan["resource_changes"][0]["change"]["after"]["name"] = "outside-scope"
        with self.assertRaises(PlanValidationError):
            validate_plan(plan)

    def test_rejects_new_null_attribute(self) -> None:
        plan = valid_plan()
        plan["resource_changes"][0]["change"]["after"]["unapproved_null"] = None
        with self.assertRaises(PlanValidationError):
            validate_plan(plan)

    def test_rejects_user_recreation_gate(self) -> None:
        plan = valid_plan()
        plan["variables"]["enable_restore_iam_users"]["value"] = True
        with self.assertRaises(PlanValidationError):
            validate_plan(plan)

    def test_rejects_any_delete(self) -> None:
        plan = valid_plan()
        plan["resource_changes"][0]["change"]["actions"] = ["delete"]
        with self.assertRaises(PlanValidationError):
            validate_plan(plan)


if __name__ == "__main__":
    unittest.main()
