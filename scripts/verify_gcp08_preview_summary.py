#!/usr/bin/env python3
"""Validate the allowlisted initial-restore preview artifact without exposing plan data."""
import json
import argparse
import re
import sys

SHA = re.compile(r"^[0-9a-f]{64}$")
TARGETS = {
    "google_cloud_run_v2_service.api[0]": "espacigo-api",
    'google_cloud_run_v2_service.web["public"]': "espacigo-web",
    'google_cloud_run_v2_service.web["private"]': "espacigo-web",
}
PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"


def valid(summary, fingerprint=None):
    if (not isinstance(summary, dict) or summary.get("mode") != "bootstrap"
            or not re.fullmatch(r"[0-9a-f]{40}", summary.get("source_sha", ""))
            or not str(summary.get("release_run_id", "")).isdigit()
            or not SHA.fullmatch(summary.get("fingerprint", ""))
            or (fingerprint is not None and summary.get("fingerprint") != fingerprint)):
        return False
    changes = summary.get("changes")
    if not isinstance(changes, list) or len(changes) != len(TARGETS):
        return False
    seen = set()
    for change in changes:
        if not isinstance(change, dict):
            return False
        address = change.get("address")
        if address not in TARGETS or address in seen or change.get("action") != "create":
            return False
        image = change.get("image", "")
        repository = TARGETS[address]
        if not re.fullmatch(re.escape(PREFIX + repository) + r"@sha256:[0-9a-f]{64}", image):
            return False
        seen.add(address)
    return seen == set(TARGETS)


def main(path, fingerprint):
    try:
        with open(path, encoding="utf-8") as stream:
            summary = json.load(stream)
        if not valid(summary, fingerprint):
            raise ValueError()
    except (OSError, ValueError, TypeError):
        print("approved initial-restore preview summary is invalid", file=sys.stderr)
        return 1
    print("approved initial-restore preview summary is valid")
    return 0


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("summary")
    parser.add_argument("--fingerprint", required=True)
    args = parser.parse_args()
    sys.exit(main(args.summary, args.fingerprint))
