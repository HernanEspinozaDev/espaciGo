#!/usr/bin/env python3
"""Run private one-target Terraform plans and emit only safe diagnostics."""
import argparse
import os
import subprocess
from pathlib import Path

from classify_gcp08_terraform_failure import classify, safe_details


MAX_LOG_BYTES = 2_000_000
TARGETS = (
    ("google_cloud_run_v2_service.api[0]", "google_cloud_run_v2_service.api[0]"),
    ('google_cloud_run_v2_service.web["public"]', "google_cloud_run_v2_service.web[public]"),
    ('google_cloud_run_v2_service.web["private"]', "google_cloud_run_v2_service.web[private]"),
    ('google_cloud_run_v2_service_iam_member.api_invoker["public"]',
     "google_cloud_run_v2_service_iam_member.api_invoker[public]"),
    ('google_cloud_run_v2_service_iam_member.api_invoker["private"]',
     "google_cloud_run_v2_service_iam_member.api_invoker[private]"),
    ('google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]',
     "google_cloud_run_v2_service_iam_member.deployer_service_updater[api]"),
    ('google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]',
     "google_cloud_run_v2_service_iam_member.deployer_service_updater[public]"),
    ('google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]',
     "google_cloud_run_v2_service_iam_member.deployer_service_updater[private]"),
    ("google_cloud_run_v2_service_iam_member.public_web_invoker[0]",
     "google_cloud_run_v2_service_iam_member.public_web_invoker[0]"),
    ("google_cloud_run_v2_service_iam_member.private_iap_invoker[0]",
     "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]"),
    ("google_secret_manager_secret_iam_member.v67_api_secret_access[0]",
     "google_secret_manager_secret_iam_member.v67_api_secret_access[0]"),
    ('google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]',
     "google_iap_web_cloud_run_service_iam_member.private_evaluators[<key>]"),
)
# Each plan targets one address, but Terraform also refreshes that address's dependency closure.
SAFE_DETAIL_KEYS = {
    "status", "reason", "permission", "service", "resource_type", "address", "operation",
    "service_account",
}


def _read_log(path):
    try:
        return path.read_text(encoding="utf-8", errors="replace")[:MAX_LOG_BYTES]
    except OSError:
        return ""


def _run_target(target_dir, var_file, private_dir, index, target, safe_address,
                runner, timeout):
    log_path = private_dir / f"target-{index:02d}.log"
    descriptor = os.open(log_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    env = os.environ.copy()
    # Provider tracing can expose request/response values; diagnostics never enable it.
    for key in tuple(env):
        if key.startswith("TF_LOG"):
            del env[key]

    command = [
        "terraform", "plan", "-input=false", "-lock-timeout=5m",
        f"-var-file={var_file}", f"-target={target}", "-no-color",
    ]
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as log:
            result = runner(
                command, cwd=str(target_dir), stdout=log, stderr=subprocess.STDOUT,
                env=env, timeout=timeout, check=False,
            )
        diagnostic = _read_log(log_path)
        category = "success" if result.returncode == 0 else classify("plan", diagnostic)
    except (OSError, subprocess.TimeoutExpired):
        diagnostic = _read_log(log_path)
        category = classify("plan", diagnostic)
        if category == "unclassified":
            category = "other"

    fields = [f"plan_target={safe_address}"]
    if category == "iam":
        for item in safe_details(diagnostic):
            key, separator, value = item.partition("=")
            if not separator or key not in SAFE_DETAIL_KEYS:
                continue
            fields.append(f"terraform_address={value}" if key == "address" else item)
    print(
        f"GCP08_TERRAFORM_TARGET stage=bootstrap category={category} "
        f"{' '.join(fields)}",
        flush=True,
    )
    return category == "success"


def diagnose(target_dir, var_file, private_dir, runner=subprocess.run, timeout=120):
    """Run sequential one-address plans; never apply, print provider logs, or set TF_LOG."""
    target_dir = Path(target_dir)
    var_file = Path(var_file)
    private_dir = Path(private_dir)
    private_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_dir, 0o700)
    results = []
    for index, (target, safe_address) in enumerate(TARGETS, 1):
        results.append(_run_target(
            target_dir, var_file, private_dir, index, target, safe_address,
            runner, timeout,
        ))
    return all(results)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--terraform-dir", required=True)
    parser.add_argument("--var-file", required=True)
    parser.add_argument("--private-dir", required=True)
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()
    diagnose(args.terraform_dir, args.var_file, args.private_dir, timeout=args.timeout)
    # Target-plan failures are diagnostic evidence; the enclosing full plan already failed.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
