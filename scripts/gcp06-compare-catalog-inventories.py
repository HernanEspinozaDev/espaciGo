#!/usr/bin/env python3
"""Compare private pg_catalog inventory files without printing identifiers."""

import hashlib
import json
import sys
from collections import Counter
from pathlib import Path


def load(path: str) -> set[tuple[str, str]]:
    records: set[tuple[str, str]] = set()
    for line in Path(path).read_text(encoding="utf-8").splitlines():
        value = json.loads(line)
        if not isinstance(value, list) or len(value) != 2 or not all(isinstance(part, str) for part in value):
            raise ValueError("invalid catalog inventory record")
        records.add((value[0], value[1]))
    return records


def digest(items: set[tuple[str, str]]) -> str:
    canonical = "\n".join(f"{kind}\0{identity}" for kind, identity in sorted(items)) + "\n"
    return hashlib.sha256(canonical.encode()).hexdigest()


def main() -> int:
    if len(sys.argv) != 3:
        print("usage: gcp06-compare-catalog-inventories.py SOURCE STAGE", file=sys.stderr)
        return 2
    try:
        source, stage = load(sys.argv[1]), load(sys.argv[2])
    except (OSError, ValueError, json.JSONDecodeError):
        print("catalog inventory unavailable; no source rows were streamed", file=sys.stderr)
        return 2

    source_only, stage_only = source - stage, stage - source
    source_counts = Counter(kind for kind, _ in source_only)
    stage_counts = Counter(kind for kind, _ in stage_only)
    report = {
        "source_objects": len(source),
        "stage_objects": len(stage),
        "source_only": len(source_only),
        "stage_only": len(stage_only),
        "source_only_by_class": dict(sorted(source_counts.items())),
        "stage_only_by_class": dict(sorted(stage_counts.items())),
        "source_sha256": digest(source),
        "stage_sha256": digest(stage),
    }
    print(json.dumps(report, sort_keys=True))
    return 1 if source_only or stage_only else 0


if __name__ == "__main__":
    raise SystemExit(main())
