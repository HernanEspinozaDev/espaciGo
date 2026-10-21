#!/usr/bin/env python3
"""Require every GCP-08 restore target to remain registered in canonical state."""
import hashlib
import os
import sys


def main(targets_path, state_path):
    try:
        targets = {line.strip() for line in open(targets_path, encoding="utf-8") if line.strip()}
        state = {line.strip() for line in open(state_path, encoding="utf-8") if line.strip()}
        if not targets or not targets.issubset(state):
            raise ValueError()
        digest = hashlib.sha256("\n".join(sorted(targets)).encode()).hexdigest()
        output = os.environ.get("GITHUB_OUTPUT")
        if output:
            with open(output, "a", encoding="utf-8") as stream:
                stream.write(f"target_count={len(targets)}\n")
                stream.write(f"target_scope_sha256={digest}\n")
    except Exception:
        print("restore targets are not all present in canonical Terraform state", file=sys.stderr)
        return 1
    print(f"canonical state contains all {len(targets)} exact restore target addresses")
    return 0


if __name__ == "__main__":
    if len(sys.argv) != 3:
        raise SystemExit("usage: verify_gcp08_state_scope.py TARGETS STATE_LIST")
    raise SystemExit(main(sys.argv[1], sys.argv[2]))
