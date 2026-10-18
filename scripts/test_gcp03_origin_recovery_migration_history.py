from __future__ import annotations

import hashlib
from pathlib import Path
import re
import unittest

from gcp03_origin_recovery_plan import RecoveryPlanError
from gcp03_origin_recovery_preflight import (
    CANONICAL_MIGRATION_HISTORY_SHA256_V2,
    DIAGNOSTIC_MIGRATION_SHA256,
    migration_history_hashes,
)
from gcp03_migration_inventory import historical_v1_v61_migrations


ROOT = Path(__file__).resolve().parents[1]


class MigrationHistoryHashTests(unittest.TestCase):
    def test_versions_1_2_10_are_stable_across_input_order(self):
        rows = [("10", "c" * 64), ("2", "b" * 64), ("1", "a" * 64)]
        expected = (
            "c2af99dc3536948fd524d57eca7f5d2d38ff34519461983007531ce123aa2ca8",
            "b69202fa8709b699b00cd3cc07ada528cc3587b98d368405c5961545f64f682c",
        )
        self.assertEqual(migration_history_hashes(rows), expected)
        self.assertEqual(migration_history_hashes(list(reversed(rows))), expected)

    def test_all_v1_to_v61_source_checksums_match_both_versions(self):
        rows = []
        for path in historical_v1_v61_migrations(ROOT / "db/migrations"):
            match = re.match(r"V(\d+)__", path.name)
            if match:
                rows.append((str(int(match.group(1))), hashlib.sha256(path.read_bytes()).hexdigest()))
        self.assertEqual(len(rows), 61)
        self.assertEqual({int(version) for version, _ in rows}, set(range(1, 62)))
        legacy, canonical = migration_history_hashes(rows)
        self.assertEqual(legacy, DIAGNOSTIC_MIGRATION_SHA256)
        self.assertEqual(canonical, CANONICAL_MIGRATION_HISTORY_SHA256_V2)
        self.assertEqual(migration_history_hashes(list(reversed(rows))), (legacy, canonical))

    def test_rejects_non_numeric_or_duplicate_versions(self):
        with self.assertRaises(RecoveryPlanError):
            migration_history_hashes([("one", "a" * 64)])
        with self.assertRaises(RecoveryPlanError):
            migration_history_hashes([("1", "a" * 64), ("01", "b" * 64)])


if __name__ == "__main__":
    unittest.main()
