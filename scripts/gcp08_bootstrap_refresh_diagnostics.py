#!/usr/bin/env python3
"""Contrast the bootstrap plan with state refresh disabled; never apply or emit plan text."""
import argparse
import os
import subprocess
from pathlib import Path

from classify_gcp08_terraform_failure import classify, safe_details
from gcp08_bootstrap_target_diagnostics import TARGETS


MAX_LOG_BYTES = 2_000_000
SAFE_DETAIL_KEYS = {
    "status", "reason", "permission", "service", "resource_type", "address", "operation",
    "service_account",
}


def _read_log(path):
    try:
        return path.read_text(encoding="utf-8", errors="replace")[:MAX_LOG_BYTES]
    except OSError:
        return ""


def diagnose(terraform_dir, var_file, private_dir, runner=subprocess.run, timeout=120):
    """Run one refresh-free targeted plan, keeping all Terraform output private."""
    terraform_dir = Path(terraform_dir)
    private_dir = Path(private_dir)
    private_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_dir, 0o700)
    log_path = private_dir / "refresh-disabled.log"
    descriptor = os.open(log_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    env = os.environ.copy()
    for key in tuple(env):
        if key.startswith("TF_LOG"):
            del env[key]
    command = [
        "terraform", "plan", "-input=false", "-lock-timeout=5m", "-refresh=false",
        f"-var-file={var_file}", "-no-color",
    ]
    command.extend(f"-target={target}" for target, _ in TARGETS)
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as log:
            result = runner(
                command, cwd=str(terraform_dir), stdout=log, stderr=subprocess.STDOUT,
                env=env, timeout=timeout, check=False,
            )
        diagnostic = _read_log(log_path)
        category = "success" if result.returncode == 0 else classify("plan", diagnostic)
    except (OSError, subprocess.TimeoutExpired):
        diagnostic = _read_log(log_path)
        category = classify("plan", diagnostic)
        if category == "unclassified":
            category = "other"

    fields = ["refresh=disabled"]
    for item in safe_details(diagnostic):
        key, separator, value = item.partition("=")
        if separator and key in SAFE_DETAIL_KEYS:
            fields.append(f"terraform_address={value}" if key == "address" else item)
    print(
        f"GCP08_TERRAFORM_REFRESH_DIAGNOSTIC stage=bootstrap category={category} "
        f"{' '.join(fields)}",
        flush=True,
    )
    return category == "success"


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--terraform-dir", required=True)
    parser.add_argument("--var-file", required=True)
    parser.add_argument("--private-dir", required=True)
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()
    diagnose(args.terraform_dir, args.var_file, args.private_dir, timeout=args.timeout)
    # Diagnostic failures do not replace the original failed plan's exit status.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
