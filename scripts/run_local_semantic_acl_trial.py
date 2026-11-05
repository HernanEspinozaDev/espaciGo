#!/usr/bin/env python3
"""Exercise the ACL gate on the authorized local reference and synthetic target.

No credentials, TCP fallback, ACL repairs or migrations are applied. Only the
schema-only reference receives negative transactional DDL, always rolled back.
Copied public test artifacts are removed from each exact container afterwards.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[1]
IMAGE = "sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677"
TARGETS = (
    ("reference", "espacigo-acl-reference-schema-20261104"),
    ("target", "espacigo-f5-gate-db-20261101"),
)


def run(argv, **kwargs):
    return subprocess.run(argv, check=True, cwd=ROOT, **kwargs)


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--authorized-local-synthetic-only", action="store_true", required=True)
    parser.parse_args()
    scratch = Path(os.environ.get("TMPDIR", ""))
    if not scratch.is_absolute() or not scratch.is_dir():
        raise SystemExit("Explicit existing private scratch TMPDIR is required")
    binary = scratch / ("semantic-acl-" + uuid.uuid4().hex + ".test")
    ledger_sources = {p.name: sha(p) for p in (ROOT / "db/migrations").glob("V*.sql")}
    if len(ledger_sources) != 83:
        raise SystemExit("Expected exactly the 83 preserved migration sources")
    env = dict(os.environ, CGO_ENABLED="0")
    try:
        run(["go", "test", "./internal/dbbootstrap"], env=env)
        run(["go", "test", "-c", "-o", str(binary), "./internal/dbbootstrap"], env=env)
        digest = sha(binary)
        for mode, container in TARGETS:
            info = json.loads(subprocess.check_output(["docker", "inspect", container], text=True))[0]
            if not info["State"]["Running"] or info["Image"] != IMAGE:
                raise SystemExit("Selected local instance image/running identity differs")
            if mode == "reference" and (
                info["HostConfig"]["NetworkMode"] != "none"
                or info["HostConfig"].get("PortBindings")
            ):
                raise SystemExit("Schema-only reference must remain unexposed/network-none")
            remote = "/var/tmp/espacigo-semantic-acl-" + uuid.uuid4().hex
            run(["docker", "exec", container, "mkdir", "-p", remote + "/db"], capture_output=True)
            try:
                run(["docker", "cp", str(binary), container + ":" + remote + "/acl.test"], capture_output=True)
                actual = subprocess.check_output(["docker", "exec", container, "sha256sum", remote + "/acl.test"], text=True).split()[0]
                if actual != digest:
                    raise SystemExit("Copied test executable checksum differs")
                if mode == "target":
                    run(["docker", "cp", str(ROOT / "db/migrations"), container + ":" + remote + "/db/migrations"], capture_output=True)
                print("LOCAL_ACL_INSTANCE", mode, flush=True)
                run(["docker", "exec", "-u", "postgres", "-e", "ESPACIGO_LOCAL_SEMANTIC_ACL_TEST=" + mode,
                     "-e", "ESPACIGO_LOCAL_ACL_MIGRATIONS=" + remote + "/db/migrations", container,
                     remote + "/acl.test", "-test.run", "^TestLocalSemanticACLPostgresGateAndRollbackNegatives$", "-test.v", "-test.timeout", "90s"])
            finally:
                # Exact fresh artifact directory, never data/storage or a pattern.
                run(["docker", "exec", container, "rm", "-rf", "--", remote], capture_output=True)
        if ledger_sources != {p.name: sha(p) for p in (ROOT / "db/migrations").glob("V*.sql")}:
            raise SystemExit("Migration source bytes changed during trial")
        print("LOCAL_SEMANTIC_REFERENCE_NEGATIVES_AND_REAL_TARGET_GATE_PASS", flush=True)
    finally:
        binary.unlink(missing_ok=True)


if __name__ == "__main__":
    main()
