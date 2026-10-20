#!/usr/bin/env python3
"""Compare only sanitized plan summaries before the exact ephemeral plan is applied."""
import json
import re
import sys


def main(current_path, approved_path, fingerprint, release_run_id, source_sha):
    try:
        with open(current_path, encoding="utf-8") as stream:
            current = json.load(stream)
        with open(approved_path, encoding="utf-8") as stream:
            approved = json.load(stream)
        if (not re.fullmatch(r"[0-9a-f]{64}", fingerprint)
                or current != approved or current.get("fingerprint") != fingerprint
                or current.get("release_run_id") != str(release_run_id)
                or current.get("source_sha") != source_sha):
            raise ValueError()
    except Exception:
        print("current allowlisted Terraform changes do not match the approved preview", file=sys.stderr)
        return 1
    print("approved preview fingerprint matched")
    return 0


if __name__ == "__main__":
    sys.exit(main(*sys.argv[1:]))
