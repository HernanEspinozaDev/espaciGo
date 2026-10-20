#!/usr/bin/env python3
"""Compare private sanitized GCP-06 V64 manifest aggregates with fixed SQL output."""

from __future__ import annotations

import argparse
import hashlib
import json
import stat
from pathlib import Path
from typing import Any


RUN_ID = "20261020-gcp06a"
DATABASE = "espacigo_gcp06_candidate_20261020-gcp06a"
INSTANCE = "espacigo-gcp06-20261020-gcp06a-pg"
REGION = "southamerica-west1"
CONNECTION_NAME = f"espacigo-testprod:{REGION}:{INSTANCE}"
QUERY_ID = "GCP06-ROWS-001"
MANIFEST_FORMAT = "espacigo.gcp06.local-stage.v1"
IMPORTED_DUMP_BYTES = 16_620_453
IMPORTED_DUMP_SHA256 = "bb35a8709ae74d51e73b69d0dd75d7b4b761822508353dab9dddcd631cec9c11"

STAGED_FIELDS = (
    "migration_count",
    "migration_max",
    "users",
    "spaces",
    "active_spaces",
    "catalog_candidates",
    "synthetic_kyc",
    "bookings",
    "active_users",
    "admins",
    "global_admins",
    "disabled_hashes",
)
ACTUAL_FIELDS = {
    "migration_count": "migration_count",
    "migration_max": "migration_max",
    "users": "users",
    "spaces": "spaces",
    "active_spaces": "active_spaces",
    "catalog_candidates": "catalog_candidates",
    "synthetic_kyc": "synthetic_kyc",
    "bookings": "bookings",
    "active_users": "active_users",
    "admins": "admins",
    "global_admins": "global_admins",
    "disabled_hashes": "disabled_hashes",
}


class VerificationError(Exception):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


def _read_private_json(path: Path, max_bytes: int) -> tuple[bytes, Any]:
    try:
        parent_info = path.parent.lstat()
        info = path.lstat()
    except OSError as exc:
        raise VerificationError("input_unavailable") from exc
    if stat.S_ISLNK(parent_info.st_mode) or not stat.S_ISDIR(parent_info.st_mode) or stat.S_IMODE(parent_info.st_mode) != 0o700:
        raise VerificationError("input_parent_permissions")
    if stat.S_ISLNK(info.st_mode) or not stat.S_ISREG(info.st_mode):
        raise VerificationError("input_not_regular")
    if stat.S_IMODE(info.st_mode) != 0o600 or info.st_size < 1 or info.st_size > max_bytes:
        raise VerificationError("input_permissions_or_size")
    try:
        raw = path.read_bytes()
        return raw, json.loads(raw)
    except (OSError, UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise VerificationError("input_invalid_json") from exc


def verify(manifest_path: Path, actual_path: Path, target_evidence_path: Path) -> dict[str, Any]:
    manifest_raw, manifest = _read_private_json(manifest_path, 1_000_000)
    _, actual = _read_private_json(actual_path, 100_000)
    target_evidence_raw, target_evidence = _read_private_json(target_evidence_path, 100_000)
    manifest_sha = hashlib.sha256(manifest_raw).hexdigest()

    if not isinstance(manifest, dict) or manifest.get("format") != MANIFEST_FORMAT:
        raise VerificationError("manifest_format_mismatch")
    source = manifest.get("source")
    staged = manifest.get("staged")
    if not isinstance(source, dict) or not isinstance(staged, dict):
        raise VerificationError("manifest_profile_missing")
    if source.get("migration_count") != 61 or source.get("migration_max") != 61:
        raise VerificationError("manifest_source_ledger_mismatch")
    if manifest.get("dump_bytes") != IMPORTED_DUMP_BYTES or manifest.get("dump_sha256") != IMPORTED_DUMP_SHA256:
        raise VerificationError("manifest_import_artifact_mismatch")
    if any(type(staged.get(field)) is not int for field in STAGED_FIELDS):
        raise VerificationError("manifest_aggregate_profile_invalid")
    if staged["migration_count"] != 64 or staged["migration_max"] != 64:
        raise VerificationError("manifest_v64_baseline_mismatch")

    if not isinstance(actual, dict) or set(actual) != {
        "query_id", "run_id", "database", "server_major", "session_user", "current_user", "observed_at", "ledger", "staged"
    }:
        raise VerificationError("query_output_shape_invalid")
    if actual.get("query_id") != QUERY_ID or actual.get("run_id") != RUN_ID:
        raise VerificationError("query_identity_mismatch")
    if actual.get("database") != DATABASE or actual.get("server_major") != 18:
        raise VerificationError("query_target_mismatch")
    session_user = actual.get("session_user")
    current_user = actual.get("current_user")
    observed_at = actual.get("observed_at")
    if not isinstance(session_user, str) or not session_user or current_user != session_user:
        raise VerificationError("query_session_identity_invalid")
    if not isinstance(observed_at, str) or not observed_at.endswith("Z"):
        raise VerificationError("query_observation_time_invalid")

    if not isinstance(target_evidence, dict) or set(target_evidence) != {
        "name", "region", "connectionName", "databaseVersion", "state"
    }:
        raise VerificationError("target_evidence_shape_invalid")
    expected_evidence = {
        "name": INSTANCE,
        "region": REGION,
        "connectionName": CONNECTION_NAME,
        "databaseVersion": "POSTGRES_18",
        "state": "RUNNABLE",
    }
    if any(target_evidence.get(key) != value for key, value in expected_evidence.items()):
        raise VerificationError("target_evidence_mismatch")
    ledger = actual.get("ledger")
    if not isinstance(ledger, dict) or set(ledger) != {
        "rows", "min_version", "max_version", "distinct_versions", "duplicate_rows", "contiguous_v1_v64"
    }:
        raise VerificationError("ledger_result_shape_invalid")
    if ledger != {
        "rows": 64,
        "min_version": 1,
        "max_version": 64,
        "distinct_versions": 64,
        "duplicate_rows": 0,
        "contiguous_v1_v64": True,
    }:
        raise VerificationError("candidate_ledger_mismatch")
    actual_staged = actual.get("staged")
    if not isinstance(actual_staged, dict) or set(actual_staged) != set(STAGED_FIELDS):
        raise VerificationError("aggregate_result_shape_invalid")
    if any(type(actual_staged.get(field)) is not int for field in STAGED_FIELDS):
        raise VerificationError("aggregate_result_type_invalid")
    if any(actual_staged[actual_key] != staged[manifest_key] for manifest_key, actual_key in ACTUAL_FIELDS.items()):
        raise VerificationError("candidate_aggregate_mismatch")

    return {
        "status": "PASS",
        "run_id": RUN_ID,
        "instance": INSTANCE,
        "database": DATABASE,
        "query_id": QUERY_ID,
        "manifest_sha256": manifest_sha,
        "target_evidence_sha256": hashlib.sha256(target_evidence_raw).hexdigest(),
        "imported_dump_sha256": IMPORTED_DUMP_SHA256,
        "session_user": session_user,
        "observed_at": observed_at,
        "checks": ["manifest_format_and_private_mode", "target_evidence_and_session_binding", "imported_dump_identity", "ledger_v1_v64", "sanitized_aggregates"],
    }


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, required=True)
    parser.add_argument("--actual", type=Path, required=True)
    parser.add_argument("--target-evidence", type=Path, required=True)
    args = parser.parse_args(argv)
    try:
        report = verify(args.manifest, args.actual, args.target_evidence)
    except VerificationError as exc:
        report = {
            "status": "STOP",
            "run_id": RUN_ID,
            "query_id": QUERY_ID,
            "reason_code": exc.code,
        }
        print(json.dumps(report, sort_keys=True))
        return 1
    print(json.dumps(report, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
