#!/usr/bin/env python3
"""Count selected Terraform state resource types without printing addresses or values."""
import argparse
import os
import re
import subprocess
from pathlib import Path


MAX_LOG_BYTES = 2_000_000
RESOURCE_TYPES = (
    ("run_services", "google_cloud_run_v2_service"),
    ("run_service_iam", "google_cloud_run_v2_service_iam_member"),
    ("iap_service_iam", "google_iap_web_cloud_run_service_iam_member"),
    ("service_account_iam", "google_service_account_iam_member"),
    ("sql_instance", "google_sql_database_instance"),
    ("sql_database", "google_sql_database"),
)
TYPE_PATTERN = re.compile(
    r"(?:^|\.)(" + "|".join(re.escape(resource_type) for _, resource_type in RESOURCE_TYPES) + r")(?=\.)"
)
STATE_ADDRESS_ALIASES = {
    'google_sql_database_instance.postgres[0]': "legacy_postgres_managed",
    'data.google_sql_database_instance.v67_target[0]': "v67_instance_data",
    'data.google_sql_database_instance.gcp06_candidate[0]': "candidate_instance_data",
    'data.google_sql_database.v67_target[0]': "v67_database_data",
    'google_sql_database.application[0]': "legacy_database_managed",
}


def summarize_state_addresses(addresses):
    counts = {label: 0 for label, _ in RESOURCE_TYPES}
    counts.update({label: 0 for label in STATE_ADDRESS_ALIASES.values()})
    for address in addresses:
        if not isinstance(address, str):
            continue
        alias = STATE_ADDRESS_ALIASES.get(address)
        if alias:
            counts[alias] += 1
        match = TYPE_PATTERN.search(address)
        if match:
            resource_type = match.group(1)
            label = next(label for label, candidate in RESOURCE_TYPES if candidate == resource_type)
            counts[label] += 1
    return counts


def _read_private_log(path):
    try:
        return path.read_text(encoding="utf-8", errors="replace")[:MAX_LOG_BYTES]
    except OSError:
        return ""


def diagnose(terraform_dir, private_dir, runner=subprocess.run, timeout=120):
    terraform_dir = Path(terraform_dir)
    private_dir = Path(private_dir)
    private_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_dir, 0o700)
    log_path = private_dir / "state-list.log"
    descriptor = os.open(log_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    env = os.environ.copy()
    for key in tuple(env):
        if key.startswith("TF_LOG"):
            del env[key]
    try:
        with os.fdopen(descriptor, "w", encoding="utf-8") as log:
            result = runner(
                ["terraform", "state", "list"], cwd=str(terraform_dir), stdout=log,
                stderr=subprocess.STDOUT, env=env, timeout=timeout, check=False,
            )
    except (OSError, subprocess.TimeoutExpired):
        print("GCP08_STATE_INVENTORY stage=bootstrap category=unavailable", flush=True)
        return False

    if result.returncode != 0:
        print("GCP08_STATE_INVENTORY stage=bootstrap category=unavailable", flush=True)
        return False
    counts = summarize_state_addresses(_read_private_log(log_path).splitlines())
    fields = " ".join(f"{label}={value}" for label, value in counts.items())
    print(
        f"GCP08_STATE_INVENTORY stage=bootstrap category=success "
        f"backend=gcs workspace=default policy_mode=bootstrap {fields}",
        flush=True,
    )
    return True


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--terraform-dir", required=True)
    parser.add_argument("--private-dir", required=True)
    parser.add_argument("--timeout", type=int, default=120)
    args = parser.parse_args()
    diagnose(args.terraform_dir, args.private_dir, timeout=args.timeout)
    # Inventory failure is supplemental evidence and does not replace the failed plan.
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
