#!/usr/bin/env python3
"""Run fixed, read-only Cloud Run probes and emit only allowlisted diagnostics."""
import argparse
import os
import re
import subprocess
import sys
import tempfile
from pathlib import Path

from classify_gcp08_terraform_failure import classify, safe_details


PROJECT = "espacigo-testprod"
REGION = "southamerica-west1"
PROBES = (
    ("cloud_run_service.api.describe", "describe", "espacigo-test-api"),
    ("cloud_run_service.public.describe", "describe", "espacigo-test-public"),
    ("cloud_run_service.private.describe", "describe", "espacigo-test-private"),
    ("cloud_run_service.api.get_iam_policy", "get-iam-policy", "espacigo-test-api"),
    ("cloud_run_service.public.get_iam_policy", "get-iam-policy", "espacigo-test-public"),
    ("cloud_run_service.private.get_iam_policy", "get-iam-policy", "espacigo-test-private"),
)
PROBE_BY_ENUM = {target: (operation, service) for target, operation, service in PROBES}
NOT_FOUND = re.compile(r"\b404\b|\bNOT_FOUND\b|not found|does not exist", re.IGNORECASE)
AUTH_ERROR = re.compile(
    r"\b401\b|\bUNAUTHENTICATED\b|invalid_grant|invalid credentials|"
    r"could not find default credentials|unable to refresh|problem refreshing.*(?:auth|token)|"
    r"error retrieving credentials|failed to refresh|credential refresh|"
    r"expired (?:access )?token|oauth", re.IGNORECASE,
)
SAFE_CATEGORIES = {"success", "not_found", "permission_denied", "auth_error", "other"}
SAFE_DETAIL_KEYS = {"status", "permission", "service"}


def classify_probe(diagnostic):
    category = classify("plan", diagnostic)
    if category == "iam":
        return "permission_denied"
    details = safe_details(diagnostic)
    status = next((int(item.split("=", 1)[1]) for item in details if item.startswith("status=")), None)
    if status == 404 or NOT_FOUND.search(diagnostic):
        return "not_found"
    if status == 401 or AUTH_ERROR.search(diagnostic):
        return "auth_error"
    return "other"


def build_command(target):
    if target not in PROBE_BY_ENUM:
        raise ValueError("invalid target")
    operation, service = PROBE_BY_ENUM[target]
    return [
        "gcloud", "run", "services", operation, service,
        f"--region={REGION}", f"--project={PROJECT}", "--format=none", "--quiet",
    ]


def _emit(target, category, diagnostic=""):
    if category not in SAFE_CATEGORIES:
        category = "other"
    fields = [f"target={target}", f"category={category}"]
    if category != "success":
        fields.extend(
            item for item in safe_details(diagnostic)
            if item.split("=", 1)[0] in SAFE_DETAIL_KEYS
        )
    print("GCP08_CLOUD_RUN_PROBE stage=bootstrap " + " ".join(fields), flush=True)


def _read_diagnostic(log_path):
    try:
        return log_path.read_text(encoding="utf-8", errors="replace")[:2_000_000]
    except OSError:
        return ""


def run_probe(target, private_root, runner=subprocess.run, timeout=120):
    """Execute one fixed probe with private output files and no shell expansion."""
    command = build_command(target)
    with tempfile.TemporaryDirectory(prefix="probe-", dir=private_root) as temporary:
        private_dir = Path(temporary)
        os.chmod(private_dir, 0o700)
        log_path = private_dir / "probe.log"
        sdk_config = private_dir / "cloud-sdk"
        sdk_config.mkdir(mode=0o700)
        env = os.environ.copy()
        env["CLOUDSDK_CONFIG"] = str(sdk_config)
        env["CLOUDSDK_CORE_DISABLE_PROMPTS"] = "1"
        try:
            descriptor = os.open(log_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(descriptor, "w", encoding="utf-8") as log:
                result = runner(command, stdout=log, stderr=subprocess.STDOUT, env=env,
                                timeout=timeout, check=False)
        except (OSError, subprocess.TimeoutExpired):
            diagnostic = _read_diagnostic(log_path)
            _emit(target, classify_probe(diagnostic), diagnostic)
            return False
        if result.returncode == 0:
            _emit(target, "success")
            return True
        diagnostic = _read_diagnostic(log_path)
        _emit(target, classify_probe(diagnostic), diagnostic)
        return False


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--private-dir", required=True)
    args = parser.parse_args()
    private_root = Path(args.private_dir)
    private_root.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_root, 0o700)
    for target, _, _ in PROBES:
        run_probe(target, private_root)
    return 0


if __name__ == "__main__":
    sys.exit(main())
