#!/usr/bin/env python3
"""Classify app paths without exposing the changed path list in workflow logs."""
import os
import re
import subprocess
import sys

SHA = re.compile(r"^[0-9a-f]{40}$")


def classify(paths, initial=False):
    api = initial
    web = initial
    migrations = initial
    for path in paths:
        if (path.endswith(".go") or path in {"go.mod", "go.sum", "go.work", "go.work.sum",
                                               "Dockerfile", ".dockerignore", "planning/openapi.yaml"}
                or path.startswith(("cmd/", "internal/", "db/migrations/"))):
            api = True
        if path.startswith("frontend/") or path == "planning/openapi.yaml":
            web = True
        if path.startswith("db/migrations/"):
            migrations = True
    return {"api": api, "web": web, "migrations": migrations}


def migrations_pending(baseline, current):
    if not SHA.fullmatch(baseline or "") or not SHA.fullmatch(current or ""):
        return True
    try:
        ancestor = subprocess.run(
            ["git", "merge-base", "--is-ancestor", baseline, current],
            check=False, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL,
        )
    except OSError:
        return True
    if ancestor.returncode != 0:
        return True
    try:
        paths = subprocess.check_output(
            ["git", "diff", "--name-only", f"{baseline}..{current}", "--", "db/migrations"],
            text=True,
        ).splitlines()
    except (OSError, subprocess.CalledProcessError):
        return True
    return any(paths)


def main():
    before = os.environ.get("BEFORE_SHA", "")
    current = os.environ.get("CURRENT_SHA", "")
    manual_publish_only = os.environ.get("MANUAL_PUBLISH_ONLY") == "true"
    output = os.environ.get("GITHUB_OUTPUT")
    initial = bool(before) and set(before) == {"0"}
    if not output or not current:
        print("application change classification unavailable", file=sys.stderr)
        return 1
    paths = []
    if not initial and not manual_publish_only:
        if not before:
            print("application change classification unavailable", file=sys.stderr)
            return 1
        try:
            paths = subprocess.check_output(["git", "diff", "--name-only", before, current], text=True).splitlines()
        except subprocess.CalledProcessError:
            print("application change classification unavailable", file=sys.stderr)
            return 1
    result = classify(paths, initial=initial)
    if manual_publish_only:
        result["api"] = True
        result["web"] = True
    result["migrations"] = result["migrations"] or migrations_pending(
        os.environ.get("SCHEMA_BASELINE_SHA", ""), current)
    with open(output, "a", encoding="utf-8") as stream:
        for name, changed in result.items():
            stream.write(f"{name}={'true' if changed else 'false'}\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
