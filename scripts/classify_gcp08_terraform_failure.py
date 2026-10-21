#!/usr/bin/env python3
"""Emit only a safe, allowlisted classification for private Terraform diagnostics."""
import argparse
import re
import sys


RULES = (
    ("variables", re.compile(
        r"no value for required variable|invalid value for input variable|"
        r"undeclared input variable|invalid type for variable|"
        r"required variable.*not set|failed to parse.*tfvars", re.IGNORECASE)),
    ("iam", re.compile(
        r"iam_permission_denied|permission denied|does not have .*permission|"
        r"permission\s+[\"'][^\n]{1,120}[\"']\s+denied|"
        r"insufficient permissions|access denied|\b403\b|forbidden", re.IGNORECASE)),
    ("backend", re.compile(
        r"error acquiring the state lock|failed to .*state|backend .*error|"
        r"bucket .*not found|storage\.buckets\.|state lock|"
        r"no workspace named|workspace .*not found", re.IGNORECASE)),
    ("provider", re.compile(
        r"error configuring provider|failed to instantiate provider|"
        r"could not load plugin|plugin\.6|error retrieving credentials|"
        r"could not find default credentials|unauthenticated|oauth", re.IGNORECASE)),
)
STAGES = {"init", "workspace", "plan", "inspect", "policy"}
DENIED_PERMISSION = re.compile(
    r"permission\s+[\"']((?:run|iam\.serviceAccounts|storage\.(?:objects|managedFolders)|"
    r"resourcemanager\.projects|serviceusage\.services)\.[A-Za-z][A-Za-z0-9.]*)[\"']\s+denied",
    re.IGNORECASE,
)


def classify(stage, diagnostic):
    if stage not in STAGES:
        raise ValueError("invalid stage")
    if stage == "policy":
        return "guard"
    for category, pattern in RULES:
        if pattern.search(diagnostic):
            return category
    return "unclassified"


def denied_permission(diagnostic):
    match = DENIED_PERMISSION.search(diagnostic)
    return match.group(1) if match else None


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--stage", choices=sorted(STAGES), required=True)
    parser.add_argument("diagnostic_file")
    args = parser.parse_args()
    try:
        with open(args.diagnostic_file, "r", encoding="utf-8", errors="replace") as stream:
            diagnostic = stream.read(2_000_000)
        category = classify(args.stage, diagnostic)
    except OSError:
        category = "unclassified"
    # Never echo diagnostic text, even when it contains values or credentials.
    detail = f" permission={denied_permission(diagnostic)}" if category == "iam" and denied_permission(diagnostic) else ""
    print(f"GCP08_TERRAFORM_FAILURE stage={args.stage} category={category}{detail}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
