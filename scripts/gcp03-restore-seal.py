#!/usr/bin/env python3
"""Seal a reviewed local synthetic custom dump and pg_restore TOC list."""

from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

ROOT = Path(__file__).resolve().parents[1]
TESTING = (ROOT / ".local" / "testing").resolve()
MIGRATIONS = ROOT / "db" / "migrations"


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def inside(path: Path, parent: Path) -> bool:
    try:
        path.relative_to(parent)
        return True
    except ValueError:
        return False


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--run-id", required=True)
    parser.add_argument("--evidence-dir", required=True, help="existing run directory under .local/testing")
    parser.add_argument("--allowlist", required=True, help="manually reviewed pg_restore --use-list file")
    args = parser.parse_args()
    if not re.fullmatch(r"[a-z][a-z0-9-]{5,11}", args.run_id):
        raise SystemExit("run ID must be 6-12 lowercase alphanumeric/hyphen characters")
    run_dir = Path(args.evidence_dir).resolve()
    if not inside(run_dir, TESTING) or not run_dir.is_dir():
        raise SystemExit("evidence directory must be an existing .local/testing run directory")
    dump = run_dir / "synthetic-v1-v61.dump"
    fixture_manifest = run_dir / "fixture-manifest.json"
    restore_evidence = run_dir / "restore-evidence.json"
    allowlist = Path(args.allowlist).resolve()
    if not inside(allowlist, run_dir) or not dump.is_file() or not fixture_manifest.is_file() or not restore_evidence.is_file() or not allowlist.is_file():
        raise SystemExit("dump, fixture manifest, restore evidence, and reviewed allowlist must exist inside the run directory")
    manifest_path = run_dir / "cloud-restore-manifest.json"
    toc_path = run_dir / "dump.toc"
    canonical_allowlist = run_dir / "allowlist.toc"
    if manifest_path.exists() or toc_path.exists() or (canonical_allowlist.exists() and allowlist != canonical_allowlist):
        raise SystemExit("refusing to overwrite a previously sealed run")

    fixture_bytes = fixture_manifest.read_bytes()
    fixture = json.loads(fixture_bytes)
    if fixture.get("format") != "espacigo-gcp03-restore-fixture-v1":
        raise SystemExit("fixture manifest format mismatch")
    evidence = json.loads(restore_evidence.read_bytes())
    dump_bytes = dump.read_bytes()
    if evidence.get("format") != "espacigo-gcp03-synthetic-restore-evidence-v1" or \
            "locally generated synthetic data" not in evidence.get("source", "") or \
            evidence.get("fixture_sha256") != fixture.get("fixture_sql", {}).get("sha256") or \
            evidence.get("dump_bytes") != len(dump_bytes) or evidence.get("dump_sha256") != sha256(dump_bytes) or \
            evidence.get("migration_inventory_sha256") != fixture.get("migrations", {}).get("inventory_sha256") or \
            not evidence.get("checks") or not all(evidence["checks"].values()):
        raise SystemExit("restore evidence does not prove the exact synthetic fixture and dump passed locally")
    migrations = fixture.get("migrations", {}).get("files", [])
    if len(migrations) != 61:
        raise SystemExit("fixture manifest must contain exactly V1-V61")
    inventory = bytearray()
    expected = [f"V{i:06d}__" for i in range(1, 62)]
    for entry, prefix in zip(migrations, expected, strict=True):
        name = entry.get("file", "")
        if Path(name).name != name or not re.fullmatch(r"V[0-9]{6}__[A-Za-z0-9_-]+\.sql", name) or not name.startswith(prefix):
            raise SystemExit("fixture migration order is not V1-V61")
        migration_path = MIGRATIONS / name
        if migration_path.is_symlink():
            raise SystemExit("migration inventory cannot include symlinks")
        content = migration_path.read_bytes()
        digest = sha256(content)
        if digest != entry.get("sha256"):
            raise SystemExit("working-tree migration bytes differ from fixture manifest")
        inventory.extend(f"{name} {digest}\n".encode())
    inventory_sha = sha256(bytes(inventory))
    if fixture["migrations"].get("inventory_sha256") != inventory_sha:
        raise SystemExit("fixture migration inventory hash is invalid")

    toc = subprocess.run(["pg_restore", "--list", str(dump)], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout
    allowed = allowlist.read_bytes()
    available_entries = {line.strip() for line in toc.decode("utf-8").splitlines() if line.strip() and not line.lstrip().startswith(";")}
    selected_entries = [line.strip() for line in allowed.decode("utf-8").splitlines() if line.strip() and not line.lstrip().startswith(";")]
    if not selected_entries or len(selected_entries) != len(set(selected_entries)) or not set(selected_entries).issubset(available_entries):
        raise SystemExit("reviewed allowlist must be non-empty, unique, and a subset of the exact dump TOC")
    if allowlist != canonical_allowlist:
        with canonical_allowlist.open("xb") as f:
            f.write(allowed)
        os.chmod(canonical_allowlist, 0o600)
    toc_path.write_bytes(toc)
    os.chmod(toc_path, 0o600)
    manifest = {
        "format": "espacigo-gcp03-cloud-restore-v1",
        "run_id": args.run_id,
        "database": "espacigo_scratch",
        "dump_object": f"runs/{args.run_id}/synthetic-v1-v61.dump",
        "dump_bytes": len(dump_bytes),
        "dump_sha256": sha256(dump_bytes),
        "toc_object": f"runs/{args.run_id}/dump.toc",
        "toc_sha256": sha256(toc),
        "allowlist_object": f"runs/{args.run_id}/allowlist.toc",
        "allowlist_sha256": sha256(allowed),
        "migration_count": 61,
        "migration_inventory_sha256": inventory_sha,
    }
    with manifest_path.open("x", encoding="utf-8") as f:
        json.dump(manifest, f, indent=2, sort_keys=True)
        f.write("\n")
    os.chmod(manifest_path, 0o600)
    print(json.dumps({"run_id": args.run_id, "manifest": str(manifest_path), "manifest_sha256": sha256(manifest_path.read_bytes()), "dump_bytes": len(dump_bytes), "dump_sha256": manifest["dump_sha256"], "toc_sha256": manifest["toc_sha256"], "allowlist_sha256": manifest["allowlist_sha256"], "migration_inventory_sha256": inventory_sha}, sort_keys=True))


if __name__ == "__main__":
    main()
