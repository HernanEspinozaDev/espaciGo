#!/usr/bin/env python3
"""Verify a reviewed, version-pinned V67 runtime preflight record."""
import hashlib
import json
import re

TARGET = {
    "project": "espacigo-testprod",
    "instance": "espacigo-testprod-db",
    "database": "espacigo_dms_stage_20261020",
    "encoding": "UTF8",
    "collation": "es_ES.utf8",
}
SECRET_ID = "espacigo-test-database-password-v67"


def valid(record, expected_sha, password_version):
    if (not re.fullmatch(r"[0-9a-f]{64}", expected_sha or "")
            or not re.fullmatch(r"[1-9][0-9]*", password_version or "")
            or not isinstance(record, dict)
            or record.get("schema") != 1
            or record.get("status") != "verified"
            or record.get("target") != TARGET):
        return False
    checks = record.get("checks")
    if not isinstance(checks, dict) or checks != {
        "v67_schema_verified": True,
        "runtime_login_and_grants_verified": True,
        "runtime_connectivity_probe_passed": True,
        "password_secret_enabled": True,
    }:
        return False
    secret = record.get("password_secret")
    if not isinstance(secret, dict) or secret != {
        "id": SECRET_ID,
        "version": password_version,
        "state": "ENABLED",
    }:
        return False
    review = record.get("review")
    if (not isinstance(review, dict)
            or set(review) != {"reviewer", "pr", "commit", "reviewed_at"}
            or review.get("reviewer") != "HernanEspinozaDev"
            or type(review.get("pr")) is not int or review["pr"] < 1
            or not re.fullmatch(r"[0-9a-f]{40}", review.get("commit", ""))
            or not re.fullmatch(r"20[0-9]{2}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z", review.get("reviewed_at", ""))):
        return False
    refs = record.get("evidence_refs")
    if not isinstance(refs, list):
        return False
    pr_ref = f"https://github.com/HernanEspinozaDev/espaciGo/pull/{review['pr']}"
    run_refs = [ref for ref in refs if isinstance(ref, str)
                and re.fullmatch(r"https://github\.com/HernanEspinozaDev/espaciGo/actions/runs/[1-9][0-9]*", ref)]
    return (len(refs) == 4 and len(set(refs)) == 4
            and pr_ref in refs and len(run_refs) == 3
            and hashlib.sha256(json.dumps(record, sort_keys=True, separators=(",", ":")).encode()).hexdigest() == expected_sha)


def load_valid(path, expected_sha, password_version):
    try:
        with open(path, encoding="utf-8") as stream:
            record = json.load(stream)
        return valid(record, expected_sha, password_version)
    except (OSError, ValueError, TypeError):
        return False
