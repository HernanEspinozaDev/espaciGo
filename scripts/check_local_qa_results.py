#!/usr/bin/env python3
"""Fail when a required local integration is missing, skipped, or failed."""
from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Iterable

Event = dict[str, object]
TestKey = tuple[str, str]


def evaluate(events: Iterable[Event], required: set[TestKey]) -> tuple[list[TestKey], list[TestKey], list[TestKey], set[TestKey]]:
    seen: set[TestKey] = set()
    skipped: set[TestKey] = set()
    failed: set[TestKey] = set()
    passed: set[TestKey] = set()
    for event in events:
        package, test, action = event.get("Package"), event.get("Test"), event.get("Action")
        if not isinstance(package, str) or not isinstance(test, str):
            continue
        key = package, test
        seen.add(key)
        if action == "skip":
            skipped.add(key)
        elif action == "fail":
            failed.add(key)
        elif action == "pass":
            passed.add(key)
    missing = sorted(required - seen)
    omitted = sorted(required & skipped)
    unsuccessful = sorted((required & failed) | (required - passed - skipped))
    return missing, omitted, unsuccessful, skipped


def read_manifest(path: Path) -> set[TestKey]:
    result: set[TestKey] = set()
    for line_number, raw in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "::" not in line:
            raise ValueError(f"{path}:{line_number}: expected package::TestName")
        package, test = line.split("::", 1)
        package = f"github.com/HernanEspinozaDev/espaciGo/{package}"
        if not package or not test.startswith("Test"):
            raise ValueError(f"{path}:{line_number}: invalid package or test name")
        result.add((package, test))
    if not result:
        raise ValueError(f"{path}: required integration manifest is empty")
    return result


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: check-local-qa-results.py <go-test-json-lines> <required-integrations-manifest>", file=sys.stderr)
        return 2
    result_path, manifest_path = map(Path, sys.argv[1:])
    events: list[Event] = []
    for number, line in enumerate(result_path.read_text(encoding="utf-8").splitlines(), 1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            print(f"invalid go test JSON at line {number}: {exc}", file=sys.stderr)
            return 2
        if isinstance(value, dict):
            events.append(value)
    try:
        required = read_manifest(manifest_path)
    except (OSError, ValueError) as exc:
        print(exc, file=sys.stderr)
        return 2
    missing, skipped_required, unsuccessful, skipped_all = evaluate(events, required)
    completed = sum(event.get("Action") == "pass" and isinstance(event.get("Test"), str) for event in events)
    print(f"Go tests reported passed events: {completed}; required integrations: {len(required)}; skipped tests: {len(skipped_all)}")
    if skipped_all:
        print("Skipped tests:")
        for package, test in sorted(skipped_all):
            print(f"  {package.removeprefix('github.com/HernanEspinozaDev/espaciGo/')}::{test}")
    if missing or skipped_required or unsuccessful:
        for label, values in (("missing required integrations", missing), ("skipped required integrations", skipped_required), ("required integrations without a passing result", unsuccessful)):
            if values:
                print(label + ":", file=sys.stderr)
                for package, test in values:
                    print(f"  {package.removeprefix('github.com/HernanEspinozaDev/espaciGo/')}::{test}", file=sys.stderr)
        return 1
    print("PASS: every required integration ran and passed")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
