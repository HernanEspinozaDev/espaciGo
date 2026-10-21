#!/usr/bin/env python3
"""Require an applied schema baseline and no pending migrations before rollout."""
import json
import os
import re
import subprocess
import sys

SHA = re.compile(r"^[0-9a-f]{40}$")
MIGRATION = re.compile(r"(?:^|/)V0*([0-9]+)__[^/]+\.sql$")


def rollout_allowed(manifest, baseline_sha, baseline_is_ancestor, pending_migrations, allow_publish_only=False):
    if not isinstance(manifest, dict):
        return False
    source_sha = manifest.get("source_sha")
    return (manifest.get("schema") == 1
            and isinstance(manifest.get("contains_migrations"), bool)
            and manifest["contains_migrations"] is False
            and (manifest.get("publish_only") is False or (allow_publish_only and manifest.get("publish_only") is True))
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


def baseline_has_v67(baseline_sha):
    if not SHA.fullmatch(baseline_sha or ""):
        return False
    result = subprocess.run(
        ["git", "ls-tree", "-r", "--name-only", baseline_sha, "--", "db/migrations"],
        check=False, capture_output=True, text=True,
    )
    if result.returncode != 0:
        return False
    versions = []
    for path in result.stdout.splitlines():
        match = MIGRATION.search(path)
        if match:
            versions.append(int(match.group(1)))
    return bool(versions) and max(versions) == 67 and 67 in versions


def main(path):
    allowed = False
    try:
        with open(path, encoding="utf-8") as stream:
            manifest = json.load(stream)
        baseline = os.environ.get("SCHEMA_BASELINE_SHA", "")
        source_sha = manifest.get("source_sha") if isinstance(manifest, dict) else ""
        pending = pending_migrations(baseline, source_sha)
        allow_publish_only = "--allow-publish-only" in sys.argv[2:]
        allowed = (baseline_has_v67(baseline)
                   and rollout_allowed(manifest, baseline, pending is not None, pending or [], allow_publish_only))
    except (OSError, ValueError, TypeError):
        allowed = False
    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        with open(output, "a", encoding="utf-8") as stream:
            stream.write(f"migration_free={'true' if allowed else 'false'}\n")
    print("release scope is eligible" if allowed else "rollout skipped: baseline must include V67 and the release must contain no unapplied migrations")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
