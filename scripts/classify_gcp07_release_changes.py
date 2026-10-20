#!/usr/bin/env python3
"""Classify app paths without exposing the changed path list in workflow logs."""
import os
import subprocess
import sys


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


def main():
    before = os.environ.get("BEFORE_SHA", "")
    current = os.environ.get("CURRENT_SHA", "")
    output = os.environ.get("GITHUB_OUTPUT")
    initial = bool(before) and set(before) == {"0"}
    if not output or not current:
        print("application change classification unavailable", file=sys.stderr)
        return 1
    paths = []
    if not initial:
        if not before:
            print("application change classification unavailable", file=sys.stderr)
            return 1
        try:
            paths = subprocess.check_output(["git", "diff", "--name-only", before, current], text=True).splitlines()
        except subprocess.CalledProcessError:
            print("application change classification unavailable", file=sys.stderr)
            return 1
    result = classify(paths, initial=initial)
    with open(output, "a", encoding="utf-8") as stream:
        for name, changed in result.items():
            stream.write(f"{name}={'true' if changed else 'false'}\n")
    return 0


if __name__ == "__main__":
    sys.exit(main())
