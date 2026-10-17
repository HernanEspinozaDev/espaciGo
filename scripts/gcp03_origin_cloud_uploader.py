#!/usr/bin/env python3
"""Upload a locally verified GCP-03 transport package as create-only ciphertext."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import sys

import gcp03_origin_import as transport

RUN_ID = "g03o-1017"
SOURCE_RUN_ID = "gcp03-merge-review-20261017"
BUCKET = "espacigo-gcp03-origin-g03o-1017"
ARTIFACTS = sorted(transport.ARTIFACTS.values())
SHA_RE = re.compile(r"^[0-9a-f]{64}$")


class UploadError(ValueError):
    pass


def _read_bound(path: Path, expected: str, *, secret: bool = False) -> tuple[bytes, str]:
    if path.is_symlink() or not path.is_file():
        raise UploadError("input file type is invalid")
    mode = stat.S_IMODE(path.stat().st_mode)
    if mode not in ({0o600, 0o400} if secret else {0o600}):
        raise UploadError("input file permissions are invalid")
    data = path.read_bytes()
    digest = hashlib.sha256(data).hexdigest()
    if not SHA_RE.fullmatch(expected) or digest != expected:
        raise UploadError("input hash does not match reviewed value")
    return data, digest


def _create_only(blob, object_name: str, data: bytes, *, content_type: str = "application/octet-stream"):
    """Create an object and use only metadata returned by that write response."""
    if blob.name != object_name:
        raise UploadError("storage client returned an unexpected object handle")
    blob.upload_from_string(data, content_type=content_type,
                            if_generation_match=0, retry=None)
    generation, size = blob.generation, blob.size
    if generation is None or size is None or int(size) != len(data):
        raise UploadError("create response omitted generation or returned an invalid size")
    return str(generation), int(size)


def upload_package(*, stage_dir: Path, recovery_key: Path, source_run_id: str,
                   target_run_id: str, expected_seal_sha256: str,
                   expected_manifest_sha256: str, normalization_sql: Path,
                   expected_normalization_sha256: str, toc_allowlist: Path,
                   expected_toc_sha256: str, bootstrap_report: Path,
                   expected_bootstrap_sha256: str, observer_report: Path,
                   expected_observer_sha256: str, receipt_path: Path, client) -> dict:
    """Verify local ciphertext, then create exact-prefix objects and receipt."""
    if source_run_id != SOURCE_RUN_ID or target_run_id != RUN_ID:
        raise UploadError("run identity does not match the reviewed import")
    verified = transport.recover_verify(stage_dir, recovery_key, source_run_id,
                                        target_run_id, expected_manifest_sha256)
    if verified["source_sealed_sha256"] != expected_seal_sha256:
        raise UploadError("source seal does not match reviewed value")
    manifest_bytes, manifest_hash = _read_bound(stage_dir / "transport-manifest.json", expected_manifest_sha256)
    manifest = json.loads(manifest_bytes)
    normal, normal_hash = _read_bound(normalization_sql, expected_normalization_sha256)
    toc, toc_hash = _read_bound(toc_allowlist, expected_toc_sha256)
    _, bootstrap_hash = _read_bound(bootstrap_report, expected_bootstrap_sha256)
    _, observer_hash = _read_bound(observer_report, expected_observer_sha256)

    # Read and bind every byte locally before the first create-only request. This
    # avoids leaving a partial run when a later local artifact fails validation.
    prepared_artifacts = []
    for name in ARTIFACTS:
        artifact_key = name.removesuffix(".gcm")
        entry = manifest.get("artifacts", {}).get(artifact_key)
        if not isinstance(entry, dict):
            raise UploadError("transport manifest artifact inventory is invalid")
        data, digest = _read_bound(stage_dir / name, entry.get("transport_cipher_sha256", ""))
        prepared_artifacts.append((name, data, digest))
    prepared_support = [
        ("runs/" + RUN_ID + "/normalization.sql", normal, normal_hash),
        ("runs/" + RUN_ID + "/toc.list", toc, toc_hash),
    ]
    prepared_manifest = ("runs/" + RUN_ID + "/transport-manifest.json", manifest_bytes, manifest_hash)
    receipt_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    if receipt_path.parent.is_symlink() or not receipt_path.parent.is_dir() or receipt_path.is_symlink() or receipt_path.exists():
        raise UploadError("local receipt destination is unsafe or already exists")

    bucket = client.bucket(BUCKET)
    refs = {}
    for name, data, digest in prepared_artifacts:
        object_name = f"runs/{RUN_ID}/{name}"
        blob = bucket.blob(object_name)
        generation, size = _create_only(blob, object_name, data)
        refs[name.removesuffix(".gcm")] = {"object_name": object_name, "generation": generation,
            "sha256": digest, "bytes": size}
    support = {}
    for label, (object_name, data, digest) in zip(("normalization_sql", "toc_allowlist"), prepared_support):
        blob = bucket.blob(object_name)
        generation, size = _create_only(blob, object_name, data)
        support[label] = {"object_name": object_name, "generation": generation,
                          "sha256": digest, "bytes": size}
    object_name, data, digest = prepared_manifest
    manifest_blob = bucket.blob(object_name)
    manifest_generation, manifest_size = _create_only(manifest_blob, object_name, data, content_type="application/json")
    manifest_ref = {"object_name": object_name, "generation": manifest_generation,
                    "sha256": digest, "bytes": manifest_size}
    receipt = {"format": "espacigo.gcp03.origin-upload-receipt.v1",
        "source_capture_run_id": source_run_id, "target_run_id": target_run_id,
        "source_sealed_sha256": expected_seal_sha256,
        "bootstrap_report_sha256": bootstrap_hash, "observer_report_sha256": observer_hash,
        "normalization_sql_sha256": normal_hash, "artifacts": refs,
        "transport_manifest": manifest_ref, "toc_allowlist": support["toc_allowlist"],
        "normalization_sql": support["normalization_sql"]}
    encoded = (json.dumps(receipt, sort_keys=True, separators=(",", ":")) + "\n").encode()
    receipt_name = f"runs/{RUN_ID}/receipt.json"
    blob = bucket.blob(receipt_name)
    receipt_generation, _ = _create_only(blob, receipt_name, encoded, content_type="application/json")
    fd = os.open(receipt_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0), 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(encoded)
        stream.flush()
        os.fsync(stream.fileno())
    return {"status": "uploaded_ciphertext_only", "receipt_sha256": hashlib.sha256(encoded).hexdigest(),
            "receipt_generation": receipt_generation, "artifact_count": len(refs)}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("stage-dir", "recovery-key", "normalization-sql", "toc-allowlist", "bootstrap-report", "observer-report", "receipt-path"):
        parser.add_argument("--" + name, type=Path, required=True)
    for name in ("source-run-id", "target-run-id", "expected-seal-sha256", "expected-manifest-sha256",
                 "expected-normalization-sha256", "expected-toc-sha256", "expected-bootstrap-sha256", "expected-observer-sha256"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    try:
        from google.cloud import storage
        result = upload_package(stage_dir=args.stage_dir, recovery_key=args.recovery_key,
            source_run_id=args.source_run_id, target_run_id=args.target_run_id,
            expected_seal_sha256=args.expected_seal_sha256, expected_manifest_sha256=args.expected_manifest_sha256,
            normalization_sql=args.normalization_sql, expected_normalization_sha256=args.expected_normalization_sha256,
            toc_allowlist=args.toc_allowlist, expected_toc_sha256=args.expected_toc_sha256,
            bootstrap_report=args.bootstrap_report, expected_bootstrap_sha256=args.expected_bootstrap_sha256,
            observer_report=args.observer_report, expected_observer_sha256=args.expected_observer_sha256,
            receipt_path=args.receipt_path, client=storage.Client())
        print(json.dumps(result, sort_keys=True))
    except (UploadError, transport.ImportErrorSafe, OSError, ValueError, KeyError) as exc:
        print(json.dumps({"status": "failed", "reason": str(exc)}, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)


if __name__ == "__main__":
    main()
