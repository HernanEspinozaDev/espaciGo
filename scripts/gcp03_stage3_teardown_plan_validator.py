#!/usr/bin/env python3
"""Validate the reviewed-only GCP-03 Stage3 run-scoped teardown plan."""
from __future__ import annotations
import argparse
import json
from collections import Counter
from pathlib import Path
from typing import Any

EXPECTED_ADDRESSES = {
    'google_cloud_run_v2_job.admin_phase["bootstrap"]',
    'google_cloud_run_v2_job.admin_phase["retire"]',
    "google_cloud_run_v2_job.diagnose[0]",
    "google_cloud_run_v2_job.restore[0]",
    "google_cloud_run_v2_job.retirement_diagnose[0]",
    "google_cloud_run_v2_job.verify[0]",
    "google_cloud_run_v2_job_iam_member.bootstrap_invoker[0]",
    "google_cloud_run_v2_job_iam_member.diagnostic_invoker[0]",
    "google_cloud_run_v2_job_iam_member.restore_invoker[0]",
    "google_cloud_run_v2_job_iam_member.retire_invoker[0]",
    "google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker[0]",
    "google_cloud_run_v2_job_iam_member.verify_invoker[0]",
    "google_project_iam_member.admin_sql_client[0]",
    "google_project_iam_member.sql_client[0]",
    "google_project_iam_member.sql_instance_user[0]",
    "google_project_iam_member.verify_sql_client[0]",
    "google_project_iam_member.verify_sql_instance_user[0]",
    "google_secret_manager_secret.postgres_password[0]",
    "google_secret_manager_secret_iam_member.admin_password_reader[0]",
    "google_service_account.admin[0]",
    "google_service_account.restore[0]",
    "google_service_account.verify[0]",
    "google_service_account_iam_member.bootstrap_deployer_act_as[0]",
    "google_service_account_iam_member.restore_operator_act_as[0]",
    "google_service_account_iam_member.verify_operator_act_as[0]",
    "google_sql_database.restore[0]",
    "google_sql_database_instance.restore[0]",
    "google_storage_bucket.artifacts[0]",
    "google_storage_bucket_iam_member.artifact_reader[0]",
    "google_storage_bucket_iam_member.diagnostic_artifact_reader[0]",
}
EXPECTED_VARIABLES = {
    "project_id": "espacigo-testprod", "run_id": "g03r-1016",
    "enable_foundation": False, "confirm_isolated_restore": False,
    "enable_bootstrap_job": False, "enable_restore_jobs": False,
    "confirm_bootstrap_complete": False, "enable_diagnostic_job": False,
    "enable_retirement_diagnostic_job": False, "enable_restore_iam_users": False,
    "confirm_read_only_empty_baseline": False, "diagnostic_evidence_sha256": "",
    "restore_executor_image": "",
    "allow_restore_iam_user_destroy": True, "allow_restore_destroy": True,
    "confirm_destroy_run_id": "g03r-1016",
}
REQUIRED_GRAPH_EDGES = (
    ("google_sql_database_instance.restore[0]", "google_sql_database.restore[0]"),
    ("google_sql_database_instance.restore[0]", "google_cloud_run_v2_job.diagnose[0]"),
    ("google_sql_database_instance.restore[0]", "google_cloud_run_v2_job.restore[0]"),
    ("google_sql_database_instance.restore[0]", "google_cloud_run_v2_job.retirement_diagnose[0]"),
    ("google_sql_database_instance.restore[0]", "google_cloud_run_v2_job.verify[0]"),
    ("google_sql_database_instance.restore[0]", r'google_cloud_run_v2_job.admin_phase[\"bootstrap\"]'),
    ("google_sql_database_instance.restore[0]", r'google_cloud_run_v2_job.admin_phase[\"retire\"]'),
    ("google_secret_manager_secret.postgres_password[0]", "google_secret_manager_secret_iam_member.admin_password_reader[0]"),
    ("google_secret_manager_secret.postgres_password[0]", "google_cloud_run_v2_job.retirement_diagnose[0]"),
    ("google_secret_manager_secret.postgres_password[0]", r'google_cloud_run_v2_job.admin_phase[\"bootstrap\"]'),
    ("google_secret_manager_secret.postgres_password[0]", r'google_cloud_run_v2_job.admin_phase[\"retire\"]'),
    ("google_service_account.admin[0]", "google_project_iam_member.admin_sql_client[0]"),
    ("google_service_account.admin[0]", "google_secret_manager_secret_iam_member.admin_password_reader[0]"),
    ("google_service_account.admin[0]", "google_service_account_iam_member.bootstrap_deployer_act_as[0]"),
    ("google_service_account.restore[0]", "google_project_iam_member.sql_client[0]"),
    ("google_service_account.restore[0]", "google_project_iam_member.sql_instance_user[0]"),
    ("google_service_account.restore[0]", "google_service_account_iam_member.restore_operator_act_as[0]"),
    ("google_service_account.restore[0]", "google_storage_bucket_iam_member.artifact_reader[0]"),
    ("google_service_account.verify[0]", "google_project_iam_member.verify_sql_client[0]"),
    ("google_service_account.verify[0]", "google_project_iam_member.verify_sql_instance_user[0]"),
    ("google_service_account.verify[0]", "google_service_account_iam_member.verify_operator_act_as[0]"),
    ("google_service_account.verify[0]", "google_storage_bucket_iam_member.diagnostic_artifact_reader[0]"),
    ("google_service_account_iam_member.bootstrap_deployer_act_as[0]", "google_cloud_run_v2_job.retirement_diagnose[0]"),
    ("google_service_account_iam_member.bootstrap_deployer_act_as[0]", r'google_cloud_run_v2_job.admin_phase[\"bootstrap\"]'),
    ("google_service_account_iam_member.bootstrap_deployer_act_as[0]", r'google_cloud_run_v2_job.admin_phase[\"retire\"]'),
    ("google_service_account_iam_member.restore_operator_act_as[0]", "google_cloud_run_v2_job.restore[0]"),
    ("google_service_account_iam_member.verify_operator_act_as[0]", "google_cloud_run_v2_job.diagnose[0]"),
    ("google_service_account_iam_member.verify_operator_act_as[0]", "google_cloud_run_v2_job.verify[0]"),
    ("google_cloud_run_v2_job.diagnose[0]", "google_cloud_run_v2_job_iam_member.diagnostic_invoker[0]"),
    ("google_cloud_run_v2_job.restore[0]", "google_cloud_run_v2_job_iam_member.restore_invoker[0]"),
    ("google_cloud_run_v2_job.retirement_diagnose[0]", "google_cloud_run_v2_job_iam_member.retirement_diagnostic_invoker[0]"),
    ("google_cloud_run_v2_job.verify[0]", "google_cloud_run_v2_job_iam_member.verify_invoker[0]"),
    (r'google_cloud_run_v2_job.admin_phase[\"bootstrap\"]', "google_cloud_run_v2_job_iam_member.bootstrap_invoker[0]"),
    (r'google_cloud_run_v2_job.admin_phase[\"retire\"]', "google_cloud_run_v2_job_iam_member.retire_invoker[0]"),
    ("google_storage_bucket.artifacts[0]", "google_storage_bucket_iam_member.artifact_reader[0]"),
    ("google_storage_bucket.artifacts[0]", "google_storage_bucket_iam_member.diagnostic_artifact_reader[0]"),
    ("google_storage_bucket.artifacts[0]", "google_cloud_run_v2_job.restore[0]"),
)


class PlanValidationError(ValueError):
    pass

def _module_resources(module: dict[str, Any] | None) -> list[dict[str, Any]]:
    if not module:
        return []
    result = list(module.get("resources", []))
    for child in module.get("child_modules", []):
        result.extend(_module_resources(child))
    return result

def validate_plan(plan: dict[str, Any], graph_text: str | None = None) -> dict[str, Any]:
    if plan.get("complete") is not True or plan.get("applyable") is not True:
        raise PlanValidationError("Plan must be complete and applyable.")
    if plan.get("resource_drift"):
        raise PlanValidationError("Plan reports resource drift.")
    changes = plan.get("resource_changes", [])
    addresses = [item.get("address") for item in changes]
    if len(addresses) != len(set(addresses)) or set(addresses) != EXPECTED_ADDRESSES:
        missing = sorted(EXPECTED_ADDRESSES - set(addresses))
        unexpected = sorted(set(addresses) - EXPECTED_ADDRESSES)
        raise PlanValidationError(f"Resource inventory mismatch; missing={missing}; unexpected={unexpected}.")
    for item in changes:
        if item.get("mode") != "managed" or item.get("change", {}).get("actions") != ["delete"]:
            raise PlanValidationError(f"Only managed deletes are allowed; invalid change at {item.get('address')}.")
    prior_resources = _module_resources(plan.get("prior_state", {}).get("values", {}).get("root_module"))
    prior_addresses = {item.get("address") for item in prior_resources}
    if prior_addresses != EXPECTED_ADDRESSES:
        raise PlanValidationError("Prior-state inventory differs from the exact run-scoped inventory.")
    if any(item.get("type") == "google_sql_user" for item in prior_resources):
        raise PlanValidationError("Prior state unexpectedly contains a Cloud SQL user resource.")
    variables = plan.get("variables", {})
    for name, expected in EXPECTED_VARIABLES.items():
        if variables.get(name, {}).get("value") != expected:
            raise PlanValidationError(f"Variable {name} does not have its exact Stage3 value.")
    if graph_text is not None:
        for source, dependency in REQUIRED_GRAPH_EDGES:
            edge = f'"[root] {source} (destroy)" -> "[root] {dependency} (destroy)"'
            if edge not in graph_text:
                raise PlanValidationError(f"Destroy graph missing required edge: {source} -> {dependency}.")
    counts = Counter(item["type"] for item in changes)
    return {
        "schema": "espacigo.gcp03.stage3-teardown-plan-validation.v1",
        "complete": True, "applyable": True, "applied": False,
        "summary": {"add": 0, "change": 0, "delete": len(changes)},
        "resource_type_counts": dict(sorted(counts.items())),
        "resource_addresses": sorted(addresses),
        "graph_edges_checked": len(REQUIRED_GRAPH_EDGES) if graph_text is not None else 0,
    }

def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("plan_json", type=Path)
    parser.add_argument("--graph", type=Path, help="Terraform graph -plan DOT for the same saved plan")
    args = parser.parse_args()
    try:
        plan = json.loads(args.plan_json.read_text())
        graph = args.graph.read_text() if args.graph else None
        print(json.dumps(validate_plan(plan, graph), indent=2, sort_keys=True))
    except (OSError, json.JSONDecodeError, PlanValidationError) as exc:
        parser.exit(1, f"REJECTED: {exc}\n")
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
