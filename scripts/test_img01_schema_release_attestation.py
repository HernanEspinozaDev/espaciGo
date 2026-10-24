from __future__ import annotations

from datetime import datetime, timedelta, timezone
import importlib.util
import json
from pathlib import Path
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0, str(ROOT / "scripts"))
MODULE_PATH = ROOT / "scripts/resolve_img01_schema_release_attestation.py"
SPEC = importlib.util.spec_from_file_location("img01_schema_release_attestation", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)
MANIFEST_PATH = ROOT / "db/schema-evidence/img01-release-attestation.json"


def manifest() -> dict[str, object]:
    return json.loads(MANIFEST_PATH.read_text(encoding="utf-8"))


class SchemaReleaseAttestationTests(unittest.TestCase):
    def validate(self, value: dict[str, object], **kwargs: object) -> tuple[str, str, str, str]:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "manifest.json"
            path.write_text(json.dumps(value), encoding="utf-8")
            return MODULE.validate(
                path,
                ROOT / "db/migrations",
                ROOT / "db/query/img01_schema_release_evidence.sql",
                "a" * 40,
                **kwargs,
            )

    def test_checked_in_attestation_resolves_without_manual_trigger_inputs(self) -> None:
        uri, generation, digest, captured_at = self.validate(manifest())
        self.assertEqual(uri, manifest()["evidence_uri"])
        self.assertEqual(generation, manifest()["evidence_generation"])
        self.assertEqual(digest, manifest()["evidence_sha256"])
        self.assertEqual(captured_at, manifest()["captured_at"])

    def test_accepts_old_attestation_when_schema_fingerprint_matches(self) -> None:
        value = manifest()
        value["captured_at"] = (datetime.now(timezone.utc) - timedelta(days=30)).isoformat()
        self.validate(value)

    def test_rejects_missing_or_unexpected_fields(self) -> None:
        value = manifest()
        value.pop("evidence_uri")
        with self.assertRaisesRegex(MODULE.AttestationError, "fields differ"):
            self.validate(value)

    def test_rejects_missing_manifest_file(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaisesRegex(MODULE.AttestationError, "missing or invalid JSON"):
                MODULE.validate(
                    Path(directory) / "missing.json",
                    ROOT / "db/migrations",
                    ROOT / "db/query/img01_schema_release_evidence.sql",
                    "a" * 40,
                )
        value = manifest()
        value["unreviewed"] = True
        with self.assertRaisesRegex(MODULE.AttestationError, "fields differ"):
            self.validate(value)

    def test_rejects_bad_uri_generation_or_digest(self) -> None:
        for key, bad in (
            ("evidence_uri", "gs://somewhere/report.json"),
            ("evidence_generation", "0"),
            ("evidence_generation", "1; rm -rf /"),
            ("evidence_sha256", "F" * 64),
        ):
            with self.subTest(key=key, bad=bad):
                value = manifest()
                value[key] = bad
                with self.assertRaises(MODULE.AttestationError):
                    self.validate(value)

    def test_rejects_wrong_target_or_query(self) -> None:
        value = manifest()
        value["target"]["database"] = "wrong_database"
        with self.assertRaisesRegex(MODULE.AttestationError, "target"):
            self.validate(value)
        value = manifest()
        value["query_sha256"] = "0" * 64
        with self.assertRaisesRegex(MODULE.AttestationError, "query hash"):
            self.validate(value)

    def test_rejects_migration_inventory_fingerprint_drift(self) -> None:
        value = manifest()
        value["migration_fingerprint_sha256"] = "0" * 64
        with self.assertRaisesRegex(MODULE.AttestationError, "fingerprint"):
            self.validate(value)

    def test_rejects_malformed_capture_time_and_unapproved_operator(self) -> None:
        value = manifest()
        value["captured_at"] = "not-a-date"
        with self.assertRaises(MODULE.AttestationError):
            self.validate(value)
        value = manifest()
        value["captured_by"] = "unknown"
        with self.assertRaisesRegex(MODULE.AttestationError, "operator"):
            self.validate(value)

    def test_rejects_attestation_timestamp_too_far_in_future(self) -> None:
        value = manifest()
        value["captured_at"] = (datetime.now(timezone.utc) + timedelta(minutes=3)).isoformat()
        with self.assertRaisesRegex(MODULE.AttestationError, "too far in the future"):
            self.validate(value)

    def test_rejects_invalid_migration_source_pr_or_commit(self) -> None:
        value = manifest()
        value["migration_source_pr"] = "536"
        with self.assertRaisesRegex(MODULE.AttestationError, "PR must be a positive integer"):
            self.validate(value)
        value = manifest()
        value["migration_source_commit"] = "bad-commit"
        with self.assertRaisesRegex(MODULE.AttestationError, "full lowercase commit SHA"):
            self.validate(value)

    def test_query_or_migration_changes_require_a_new_attestation(self) -> None:
        value = manifest()
        with tempfile.TemporaryDirectory() as directory:
            query = Path(directory) / "query.sql"
            query.write_text((ROOT / "db/query/img01_schema_release_evidence.sql").read_text() + "\n-- changed\n")
            manifest_path = Path(directory) / "manifest.json"
            manifest_path.write_text(json.dumps(value))
            with self.assertRaisesRegex(MODULE.AttestationError, "query hash"):
                MODULE.validate(manifest_path, ROOT / "db/migrations", query, "a" * 40)

            migrations = Path(directory) / "migrations"
            migrations.mkdir()
            for source in (ROOT / "db/migrations").glob("*.sql"):
                (migrations / source.name).write_bytes(source.read_bytes())
            changed = migrations / "V000075__auth_evaluation_consent_documents.sql"
            changed.write_bytes(changed.read_bytes() + b"\n-- changed\n")
            with self.assertRaisesRegex(MODULE.AttestationError, "fingerprint"):
                MODULE.validate(manifest_path, migrations, ROOT / "db/query/img01_schema_release_evidence.sql", "a" * 40)


if __name__ == "__main__":
    unittest.main()
