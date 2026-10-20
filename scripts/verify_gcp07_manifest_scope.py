#!/usr/bin/env python3
"""Do not roll out a release whose source contains database migrations."""
import json
import os
import sys


def rollout_allowed(manifest):
    return (manifest.get("schema") == 1
            and isinstance(manifest.get("contains_migrations"), bool)
            and manifest["contains_migrations"] is False)


def main(path):
    allowed = False
    try:
        with open(path, encoding="utf-8") as stream:
            allowed = rollout_allowed(json.load(stream))
    except (OSError, ValueError, TypeError):
        allowed = False
    output = os.environ.get("GITHUB_OUTPUT")
    if output:
        with open(output, "a", encoding="utf-8") as stream:
            stream.write(f"migration_free={'true' if allowed else 'false'}\n")
    print("release scope is eligible" if allowed else "rollout skipped: release contains migrations or has invalid scope")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
