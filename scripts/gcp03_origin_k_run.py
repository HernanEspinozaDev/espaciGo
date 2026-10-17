#!/usr/bin/env python3
"""Create the exact K_run Secret Manager version from a verified local envelope."""
from __future__ import annotations

import argparse
import base64
import hashlib
import json
import re
import sys
from pathlib import Path

import requests

import gcp03_origin_import as transport

PROJECT = "espacigo-testprod"
SOURCE_RUN_ID = "gcp03-merge-review-20261017"
TARGET_RUN_ID = "g03o-1017"
SECRET_ID = "espacigo-gcp03-origin-g03o-1017-transport-key"
SOURCE_SEAL_SHA256 = "80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262"
TRANSPORT_MANIFEST_SHA256 = "c5fc1964175244e998742e34dd4c4376b2d822b27574416b61228b912641ec30"
API = "https://secretmanager.googleapis.com/v1"
SHA_RE = re.compile(r"^[0-9a-f]{64}$")


class KeyRunError(ValueError):
    pass


def create_version(*, stage_dir: Path, recovery_key: Path, expected_seal_sha256: str,
                   expected_manifest_sha256: str, access_token: str, session=requests) -> dict:
    """Verify all ciphertext locally, then perform one create-only-by-policy API call."""
    if not SHA_RE.fullmatch(expected_seal_sha256) or not SHA_RE.fullmatch(expected_manifest_sha256):
        raise KeyRunError("reviewed source and manifest hashes are required")
    if expected_seal_sha256 != SOURCE_SEAL_SHA256 or expected_manifest_sha256 != TRANSPORT_MANIFEST_SHA256:
        raise KeyRunError("source seal or transport manifest differs from the reviewed run inputs")
    if not access_token or "\n" in access_token or "\r" in access_token:
        raise KeyRunError("human access token input is invalid")

    verified = transport.recover_verify(stage_dir, recovery_key, SOURCE_RUN_ID,
                                        TARGET_RUN_ID, expected_manifest_sha256)
    if verified["source_sealed_sha256"] != expected_seal_sha256:
        raise KeyRunError("verified source seal does not match the reviewed hash")
    manifest_bytes = (stage_dir / "transport-manifest.json").read_bytes()
    manifest = json.loads(manifest_bytes)
    if manifest.get("source_capture_run_id") != SOURCE_RUN_ID or \
            manifest.get("target_run_id") != TARGET_RUN_ID or \
            manifest.get("source_sealed_sha256") != expected_seal_sha256:
        raise KeyRunError("transport manifest identity does not match the reviewed run")

    master = transport.private_file(recovery_key)
    run_key = transport.transport_key(master, bytes.fromhex(manifest["salt_hex"]), TARGET_RUN_ID)
    parent = f"projects/{PROJECT}/secrets/{SECRET_ID}"
    headers = {"Authorization": f"Bearer {access_token}", "Content-Type": "application/json",
               "x-goog-user-project": PROJECT}
    # Refuse an existing version. The operator must reconcile any prior attempt;
    # this helper never retries or creates a second key version.
    listed = session.get(f"{API}/{parent}/versions", headers=headers,
                         params={"pageSize": 2}, timeout=30)
    if listed.status_code != 200:
        raise KeyRunError("Secret Manager version preflight failed")
    if listed.json().get("versions"):
        raise KeyRunError("transport-key secret already has a version; reconcile before proceeding")

    payload = {"payload": {"data": base64.b64encode(run_key).decode("ascii")}}
    response = session.post(f"{API}/{parent}:addVersion", headers=headers,
                            json=payload, timeout=30)
    if response.status_code != 200:
        raise KeyRunError("Secret Manager addVersion did not return success; reconcile read-only")
    name = response.json().get("name", "")
    match = re.fullmatch(re.escape(parent) + r"/versions/([1-9][0-9]*)", name)
    if not match:
        raise KeyRunError("Secret Manager returned an unexpected version resource")
    return {"status": "created", "project": PROJECT, "secret_id": SECRET_ID,
            "version": match.group(1), "target_run_id": TARGET_RUN_ID,
            "source_sealed_sha256": expected_seal_sha256,
            "manifest_sha256": hashlib.sha256(manifest_bytes).hexdigest(),
            "verified_ciphertext_artifacts": verified["verified_artifacts"]}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--stage-dir", type=Path, required=True)
    parser.add_argument("--recovery-key", type=Path, required=True)
    parser.add_argument("--expected-seal-sha256", required=True)
    parser.add_argument("--expected-manifest-sha256", required=True)
    args = parser.parse_args()
    try:
        token = sys.stdin.readline().rstrip("\r\n")
        result = create_version(stage_dir=args.stage_dir, recovery_key=args.recovery_key,
                                expected_seal_sha256=args.expected_seal_sha256,
                                expected_manifest_sha256=args.expected_manifest_sha256,
                                access_token=token)
        print(json.dumps(result, sort_keys=True))
    except (KeyRunError, transport.ImportErrorSafe, OSError, ValueError, KeyError,
            json.JSONDecodeError, requests.RequestException) as exc:
        print(json.dumps({"status": "failed", "reason": str(exc)}, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)


if __name__ == "__main__":
    main()
