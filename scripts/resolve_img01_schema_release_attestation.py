#!/usr/bin/env python3
"""Validate the checked-in IMG-01 release attestation and print its GCS ref."""

from __future__ import annotations

import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sys

from verify_img01_schema_release_evidence import (
    DATABASE,
    EvidenceError,
    FORMAT as REPORT_FORMAT,
    INSTANCE,
    PROJECT,
    migration_inventory,
    parse_time,
)

ATTESTATION_FORMAT = "espacigo.img01.schema-release-attestation.v1"
BUCKET = "espacigo-testprod-cloud-build-447373903153"
SHA256 = re.compile(r"^[a-f0-9]{64}$")
COMMIT = re.compile(r"^[a-f0-9]{40}$")
GENERATION = re.compile(r"^[1-9][0-9]*$")


class AttestationError(ValueError):
    pass


def schema_fingerprint(migrations_dir: Path, query_path: Path) -> str:
    material = {
        "query_sha256": hashlib.sha256(query_path.read_bytes()).hexdigest(),
        "migration_ledger": migration_inventory(migrations_dir),
    }
    canonical = json.dumps(material, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(canonical).hexdigest()


def validate(
    manifest_path: Path,
    migrations_dir: Path,
    query_path: Path,
    source_commit: str,
    now: datetime | None = None,
) -> tuple[str, str, str, str]:
    try:
        manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise AttestationError("release attestation is missing or invalid JSON") from exc
    if not isinstance(manifest, dict):
        raise AttestationError("release attestation must be an object")
    required = {
        "format",
        "report_format",
        "evidence_uri",
        "evidence_generation",
        "evidence_sha256",
        "target",
        "query_sha256",
        "migration_fingerprint_sha256",
        "migration_source_commit",
        "migration_source_pr",
        "captured_at",
        "captured_by",
        "capture_method",
        "validation_build_id",
        "validation_source_commit",
        "approval_policy",
    }
    if set(manifest) != required:
        raise AttestationError("release attestation fields differ from the reviewed contract")
    if manifest["format"] != ATTESTATION_FORMAT or manifest["report_format"] != REPORT_FORMAT:
        raise AttestationError("unsupported release attestation or report format")

    digest = manifest["evidence_sha256"]
    if not isinstance(digest, str) or not SHA256.fullmatch(digest):
        raise AttestationError("evidence digest must be a lowercase SHA-256 digest")
    generation = manifest["evidence_generation"]
    if not isinstance(generation, str) or not GENERATION.fullmatch(generation):
        raise AttestationError("evidence generation must be a positive decimal string")
    uri = manifest["evidence_uri"]
    expected_uri = f"gs://{BUCKET}/img01-schema-evidence/{digest}.json"
    if uri != expected_uri:
        raise AttestationError("evidence URI must be content-addressed in the reviewed bucket")

    target = manifest["target"]
    if target != {
        "project_id": PROJECT,
        "instance_connection_name": INSTANCE,
        "database": DATABASE,
        "postgres_major_version": 18,
    }:
        raise AttestationError("attestation target differs from the reviewed Cloud SQL database")

    expected_query = hashlib.sha256(query_path.read_bytes()).hexdigest()
    if manifest["query_sha256"] != expected_query:
        raise AttestationError("attestation query hash does not match this checkout")
    expected_migrations = schema_fingerprint(migrations_dir, query_path)
    if manifest["migration_fingerprint_sha256"] != expected_migrations:
        raise AttestationError("attestation migration/query fingerprint does not match this checkout")

    for key in ("migration_source_commit", "validation_source_commit"):
        value = manifest[key]
        if not isinstance(value, str) or not COMMIT.fullmatch(value):
            raise AttestationError(f"{key} must be a full lowercase commit SHA")
    migration_pr = manifest["migration_source_pr"]
    if not isinstance(migration_pr, int) or migration_pr <= 0:
        raise AttestationError("migration source PR must be a positive integer")
    if not isinstance(source_commit, str) or not COMMIT.fullmatch(source_commit):
        raise AttestationError("deployment source commit must be a full lowercase commit SHA")

    try:
        captured_at = parse_time(manifest["captured_at"])
    except EvidenceError as exc:
        raise AttestationError(f"invalid attestation capture time: {exc}") from exc
    current_time = (now or datetime.now(timezone.utc)).astimezone(timezone.utc)
    if (current_time - captured_at).total_seconds() < -120:
        raise AttestationError("attestation capture time is too far in the future")
    if not isinstance(manifest["captured_by"], str) or not manifest["captured_by"]:
        raise AttestationError("capture operator is missing")
    if manifest["captured_by"] != "HernanEspinozaDev":
        raise AttestationError("capture operator differs from the reviewed operator")
    if manifest["capture_method"] != "scripts/capture_img01_schema_release_evidence.sh via local bastion forward; read-only transaction":
        raise AttestationError("capture method differs from the reviewed procedure")
    if manifest["approval_policy"] != {
        "type": "protected-pull-request-review",
        "required_reviewer": "HernanEspinozaDev",
    }:
        raise AttestationError("attestation approval policy differs from the reviewed process")
    build_id = manifest["validation_build_id"]
    if not isinstance(build_id, str) or not re.fullmatch(r"[a-f0-9-]{36}", build_id):
        raise AttestationError("validation build id is invalid")

    # A newer application commit is compatible only while the query and full
    # migration inventory remain identical; this does not observe live DB drift.
    captured_at_value = manifest["captured_at"]
    return uri, generation, digest, captured_at_value


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("manifest", type=Path)
    parser.add_argument("--source-commit", required=True)
    parser.add_argument("--migrations", type=Path, default=Path("db/migrations"))
    parser.add_argument("--query", type=Path, default=Path("db/query/img01_schema_release_evidence.sql"))
    args = parser.parse_args()
    try:
        uri, generation, digest, captured_at = validate(
            args.manifest, args.migrations, args.query, args.source_commit
        )
    except AttestationError as exc:
        print(f"schema release attestation rejected: {exc}", file=sys.stderr)
        return 1
    print(uri)
    print(generation)
    print(digest)
    print(captured_at)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
