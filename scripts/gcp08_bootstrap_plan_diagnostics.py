#!/usr/bin/env python3
"""Run private, read-only Terraform plans for the fixed bootstrap target set."""
import argparse
import os
import subprocess
import sys
import tempfile
from pathlib import Path

from classify_gcp08_terraform_failure import classify, safe_details


BOOTSTRAP_TARGETS = (
    ("cloud_run_v2_service.api", "google_cloud_run_v2_service.api[0]"),
    ("cloud_run_v2_service.web.public", 'google_cloud_run_v2_service.web["public"]'),
    ("cloud_run_v2_service.web.private", 'google_cloud_run_v2_service.web["private"]'),
    ("cloud_run_iam.api_invoker.public", 'google_cloud_run_v2_service_iam_member.api_invoker["public"]'),
    ("cloud_run_iam.api_invoker.private", 'google_cloud_run_v2_service_iam_member.api_invoker["private"]'),
    ("cloud_run_iam.deployer_updater.api", 'google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]'),
    ("cloud_run_iam.deployer_updater.public", 'google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]'),
    ("cloud_run_iam.deployer_updater.private", 'google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]'),
    ("cloud_run_iam.public_web_invoker", "google_cloud_run_v2_service_iam_member.public_web_invoker[0]"),
    ("cloud_run_iam.private_iap_invoker", "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]"),
    ("secret_manager_iam.v67_api_secret_access", "google_secret_manager_secret_iam_member.v67_api_secret_access[0]"),
    ("iap_iam.private_evaluators", 'google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]'),
)
TARGET_BY_ENUM = dict(BOOTSTRAP_TARGETS)


def _emit(target, category, diagnostic=""):
    fields = [f"target={target}", f"category={category}"]
    if category == "iam":
        fields.extend(safe_details(diagnostic))
    print("GCP08_TERRAFORM_TARGET stage=plan " + " ".join(fields), flush=True)


def run_target(target, var_file, private_root, working_directory, runner=subprocess.run, timeout=120):
    """Run one validated target, keeping all Terraform output in a private temp dir."""
    if target not in TARGET_BY_ENUM:
        raise ValueError("invalid target")
    address = TARGET_BY_ENUM[target]
    os.umask(0o077)
    with tempfile.TemporaryDirectory(prefix="target-", dir=private_root) as private_dir:
        log_path = Path(private_dir) / "plan.log"
        plan_path = Path(private_dir) / "target.tfplan"
        command = [
            "terraform", "plan", "-input=false", "-lock-timeout=30s",
            f"-var-file={var_file}", f"-target={address}", f"-out={plan_path}", "-no-color",
        ]
        try:
            with log_path.open("w", encoding="utf-8") as log:
                result = runner(command, cwd=working_directory, stdout=log, stderr=subprocess.STDOUT,
                                timeout=timeout, check=False)
            if result.returncode == 0:
                _emit(target, "success")
                return True
            diagnostic = log_path.read_text(encoding="utf-8", errors="replace")[:2_000_000]
            category = classify("plan", diagnostic)
            _emit(target, category, diagnostic)
            return False
        except (OSError, subprocess.TimeoutExpired):
            _emit(target, "unclassified")
            return False


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--var-file", required=True)
    parser.add_argument("--private-dir", required=True)
    parser.add_argument("--working-directory", default=os.getcwd())
    args = parser.parse_args()
    private_root = Path(args.private_dir)
    private_root.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_root, 0o700)
    for target, _ in BOOTSTRAP_TARGETS:
        run_target(target, args.var_file, private_root, args.working_directory)
    return 0


if __name__ == "__main__":
    sys.exit(main())
