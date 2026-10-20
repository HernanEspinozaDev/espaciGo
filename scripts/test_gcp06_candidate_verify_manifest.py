#!/usr/bin/env python3
"""Synthetic tests for the private GCP-06 manifest comparator."""

from __future__ import annotations

import json
import hashlib
import tempfile
import unittest
from pathlib import Path

import gcp06_candidate_verify_manifest as verifier


class CandidateManifestVerificationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.root.chmod(0o700)
        self.manifest_path = self.root / "manifest.json"
        self.actual_path = self.root / "actual.json"
        self.target_evidence_path = self.root / "target-evidence.json"
        self.manifest = {
            "format": verifier.MANIFEST_FORMAT,
            "source": {"migration_count": 61, "migration_max": 61},
            "staged": {field: self._expected_value(field) for field in verifier.STAGED_FIELDS},
            "dump_bytes": verifier.IMPORTED_DUMP_BYTES,
            "dump_sha256": verifier.IMPORTED_DUMP_SHA256,
        }
        self.actual = {
            "query_id": verifier.QUERY_ID,
            "run_id": verifier.RUN_ID,
            "database": verifier.DATABASE,
            "server_major": 18,
            "session_user": "gcp06_readonly_test",
            "current_user": "gcp06_readonly_test",
            "observed_at": "2026-10-20T12:34:56.123456Z",
            "ledger": {
                "rows": 64,
                "min_version": 1,
                "max_version": 64,
                "distinct_versions": 64,
                "duplicate_rows": 0,
                "contiguous_v1_v64": True,
            },
            "staged": self.manifest["staged"].copy(),
        }
        self._write(self.manifest_path, self.manifest)
        self._write_actual_and_evidence()

    def tearDown(self) -> None:
        self.tmp.cleanup()

    @staticmethod
    def _expected_value(field: str) -> int:
        return {
            "migration_count": 64,
            "migration_max": 64,
            "users": 10,
            "spaces": 3,
            "active_spaces": 2,
            "catalog_candidates": 1,
            "synthetic_kyc": 1,
            "bookings": 4,
            "active_users": 10,
            "admins": 0,
            "global_admins": 0,
            "disabled_hashes": 10,
        }[field]

    @staticmethod
    def _write(path: Path, value: dict) -> None:
        path.write_text(json.dumps(value, sort_keys=True), encoding="utf-8")
        path.chmod(0o600)

    def _write_actual_and_evidence(self) -> None:
        self._write(self.actual_path, self.actual)
        self._write(self.target_evidence_path, {
            "name": verifier.INSTANCE,
            "region": verifier.REGION,
            "connectionName": verifier.CONNECTION_NAME,
            "databaseVersion": "POSTGRES_18",
            "state": "RUNNABLE",
        })

    def test_matches_synthetic_v64_manifest_and_returns_only_sanitized_summary(self) -> None:
        report = verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)
        self.assertEqual(report["status"], "PASS")
        self.assertEqual(report["manifest_sha256"], hashlib.sha256(self.manifest_path.read_bytes()).hexdigest())
        self.assertNotIn("users", report)
        self.assertNotIn("email", json.dumps(report).lower())

    def test_rejects_non_contiguous_or_unexpected_ledger(self) -> None:
        self.actual["ledger"]["min_version"] = 2
        self._write_actual_and_evidence()
        with self.assertRaisesRegex(verifier.VerificationError, "candidate_ledger_mismatch"):
            verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)

    def test_rejects_aggregate_drift(self) -> None:
        self.actual["staged"]["spaces"] += 1
        self._write_actual_and_evidence()
        with self.assertRaisesRegex(verifier.VerificationError, "candidate_aggregate_mismatch"):
            verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)

    def test_rejects_private_input_with_wrong_permissions(self) -> None:
        self.manifest_path.chmod(0o644)
        with self.assertRaisesRegex(verifier.VerificationError, "input_permissions_or_size"):
            verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)

    def test_rejects_private_directory_with_wrong_permissions(self) -> None:
        self.root.chmod(0o755)
        with self.assertRaisesRegex(verifier.VerificationError, "input_parent_permissions"):
            verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)

    def test_rejects_unexpected_personal_fields(self) -> None:
        self.actual["email"] = "should-not-exist"
        self._write_actual_and_evidence()
        with self.assertRaisesRegex(verifier.VerificationError, "query_output_shape_invalid"):
            verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)

    def test_rejects_target_evidence_bound_to_other_instance(self) -> None:
        evidence = json.loads(self.target_evidence_path.read_text(encoding="utf-8"))
        evidence["name"] = "other-instance"
        self._write(self.target_evidence_path, evidence)
        with self.assertRaisesRegex(verifier.VerificationError, "target_evidence_mismatch"):
            verifier.verify(self.manifest_path, self.actual_path, self.target_evidence_path)


if __name__ == "__main__":
    unittest.main()
