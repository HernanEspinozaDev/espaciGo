#!/usr/bin/env python3
"""Independently read back and hash-check every ciphertext upload by generation."""
from __future__ import annotations

import argparse
import hashlib
import json
from pathlib import Path
import re
import stat
import sys

BUCKET = "espacigo-gcp03-origin-g03o-1017"
RUN_ID = "g03o-1017"
ARTIFACTS = {"database.dump", "private-files.tar", "contract-encryption-key", "origin-metadata.json"}
SHA_RE = re.compile(r"^[0-9a-f]{64}$")


class VerifyError(ValueError):
    pass


def verify_upload(receipt_path: Path, expected_receipt_sha256: str, client) -> dict:
    if receipt_path.is_symlink() or not receipt_path.is_file() or stat.S_IMODE(receipt_path.stat().st_mode) != 0o600:
        raise VerifyError("local receipt type or permissions are invalid")
    raw = receipt_path.read_bytes()
    if not SHA_RE.fullmatch(expected_receipt_sha256) or hashlib.sha256(raw).hexdigest() != expected_receipt_sha256:
        raise VerifyError("receipt does not match reviewed hash")
    try:
        receipt = json.loads(raw)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise VerifyError("receipt JSON is invalid") from exc
    if receipt.get("format") != "espacigo.gcp03.origin-upload-receipt.v1" or \
       receipt.get("source_capture_run_id") != "gcp03-merge-review-20261017" or receipt.get("target_run_id") != RUN_ID:
        raise VerifyError("receipt identity is invalid")
    refs = receipt.get("artifacts")
    if not isinstance(refs, dict) or set(refs) != ARTIFACTS:
        raise VerifyError("receipt artifact allowlist is invalid")
    objects = [(f"runs/{RUN_ID}/{name}.gcm", ref) for name, ref in refs.items()]
    objects += [(f"runs/{RUN_ID}/transport-manifest.json", receipt.get("transport_manifest")),
                (f"runs/{RUN_ID}/toc.list", receipt.get("toc_allowlist")),
                (f"runs/{RUN_ID}/normalization.sql", receipt.get("normalization_sql"))]
    checked = 0
    for expected_name, ref in objects:
        if not isinstance(ref, dict):
            raise VerifyError("receipt object reference is invalid")
        name, generation, digest, size = ref.get("object_name"), ref.get("generation"), ref.get("sha256"), ref.get("bytes")
        if not isinstance(name, str) or name != expected_name or ".." in name.split("/") or \
           not isinstance(generation, str) or not generation.isdigit() or not SHA_RE.fullmatch(str(digest)) or type(size) is not int or size < 0:
            raise VerifyError("receipt object reference fields are invalid")
        blob = client.bucket(BUCKET).blob(name, generation=int(generation))
        try:
            blob.reload(if_generation_match=int(generation))
            content = blob.download_as_bytes(if_generation_match=int(generation))
        except Exception as exc:
            raise VerifyError("object generation readback failed") from exc
        if int(blob.generation) != int(generation) or int(blob.size) != size or len(content) != size or hashlib.sha256(content).hexdigest() != digest:
            raise VerifyError("object readback hash, size, or generation mismatch")
        checked += 1
    return {"status": "ciphertext_upload_verified", "receipt_sha256": expected_receipt_sha256,
            "verified_object_count": checked, "plaintext_written": False}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--receipt", type=Path, required=True)
    parser.add_argument("--expected-receipt-sha256", required=True)
    args = parser.parse_args()
    try:
        from google.cloud import storage
        print(json.dumps(verify_upload(args.receipt, args.expected_receipt_sha256, storage.Client()), sort_keys=True))
    except (VerifyError, OSError, ValueError) as exc:
        print(json.dumps({"status": "failed", "reason": str(exc)}, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)


if __name__ == "__main__":
    main()
