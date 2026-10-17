#!/usr/bin/env python3
"""Unit tests for the staged GCP-03 IAM-user plan validator."""

from __future__ import annotations

import unittest

from gcp03_iam_user_plan_validator import PlanValidationError, TARGETS, validate_plan


def plan_for(phase: str, deletion_policy: str = "PREVENT") -> dict:
    changes = []
    for address in TARGETS:
        before = {"name": address, "deletion_policy": deletion_policy, "database_roles": ["role"]}
        if phase == "stage0":
            after = {**before, "deletion_policy": "DELETE"}
            actions = ["update"]
        else:
            after = None
            actions = ["delete"]
        changes.append({"address": address, "change": {"actions": actions, "before": before, "after": after}})
    return {"applyable": True, "complete": False, "resource_changes": changes}


class Gcp03IamUserPlanValidatorTest(unittest.TestCase):
    def test_accepts_exact_stage0_policy_updates(self) -> None:
        summary = validate_plan(plan_for("stage0"), "stage0")
        self.assertEqual(summary["summary"], {"add": 0, "change": 2, "delete": 0})

    def test_rejects_stage0_changes_other_than_deletion_policy(self) -> None:
        plan = plan_for("stage0")
        plan["resource_changes"][0]["change"]["after"]["database_roles"] = ["different"]
        with self.assertRaises(PlanValidationError):
            validate_plan(plan, "stage0")

    def test_accepts_stage1_only_after_delete_policy_is_applied(self) -> None:
        summary = validate_plan(plan_for("stage1", "DELETE"), "stage1")
        self.assertEqual(summary["summary"], {"add": 0, "change": 0, "delete": 2})

    def test_rejects_delete_when_before_policy_is_prevent(self) -> None:
        with self.assertRaisesRegex(PlanValidationError, "before.deletion_policy must be DELETE"):
            validate_plan(plan_for("stage1", "PREVENT"), "stage1")

    def test_rejects_mutation_of_any_other_resource(self) -> None:
        plan = plan_for("stage0")
        plan["resource_changes"].append(
            {"address": "google_sql_database_instance.restore[0]", "change": {"actions": ["update"]}}
        )
        with self.assertRaisesRegex(PlanValidationError, "unexpected mutating resource"):
            validate_plan(plan, "stage0")


if __name__ == "__main__":
    unittest.main()
