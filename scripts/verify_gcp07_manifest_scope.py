#!/usr/bin/env python3
"""Require an applied schema baseline and no pending migrations before rollout."""
import json
import os
import re
import subprocess
import sys

SHA = re.compile(r"^[0-9a-f]{40}$")


def rollout_allowed(manifest, baseline_sha, baseline_is_ancestor, pending_migrations):
    if not isinstance(manifest, dict):
        return False
    source_sha = manifest.get("source_sha")
    return (manifest.get("schema") == 1
            and isinstance(manifest.get("contains_migrations"), bool)
            and manifest["contains_migrations"] is False
            and isinstance(source_sha, str) and SHA.fullmatch(source_sha) is not None
            and isinstance(baseline_sha, str) and SHA.fullmatch(baseline_sha) is not None
            and baseline_is_ancestor and not pending_migrations)


def pending_migrations(baseline_sha, source_sha):
    if not SHA.fullmatch(baseline_sha or "") or not SHA.fullmatch(source_sha or ""):
        return None
    ancestor = subprocess.run(
        ["git", "merge-base", "--is-ancestor", baseline_sha, source_sha],
        check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
    )
    if ancestor.returncode != 0:
        return None
    changed = subprocess.run(
        ["git", "diff", "--name-only", f"{baseline_sha}..{source_sha}", "--", "db/migrations"],
        check=False, capture_output=True, text=True,
    )
    if changed.returncode != 0:
        return None
    return [line for line in changed.stdout.splitlines() if line]


def main(path):
    allowed = False
    try:
        with open(path, encoding="utf-8") as stream:
            manifest = json.load(stream)
        baseline = os.environ.get("SCHEMA_BASELINE_SHA", "")
        source_sha = manifest.get("source_sha") if isinstance(manifest, dict) else ""
        pending = pending_migrations(baseline, source_sha)
        allowed = rollout_allowed(manifest, baseline, pending is not None, pending or [])
    except (OSError, ValueError, TypeError):
        allowed = False
    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        with open(output, "a", encoding="utf-8") as stream:
            stream.write(f"migration_free={'true' if allowed else 'false'}\n")
    print("release scope is eligible" if allowed else "rollout skipped: schema baseline is absent/unverified or migrations remain unapplied")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
