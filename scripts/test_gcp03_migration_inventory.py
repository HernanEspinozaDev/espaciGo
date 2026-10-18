from pathlib import Path
import tempfile
import unittest

from gcp03_migration_inventory import (
    HistoricalMigrationInventoryError,
    historical_v1_v61_migrations,
)


class HistoricalMigrationInventoryTests(unittest.TestCase):
    def test_selects_fixed_v1_through_v61_and_ignores_v62(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for version in range(1, 63):
                (root / f"V{version:06d}__fixture.sql").write_text("SELECT 1;\n")

            files = historical_v1_v61_migrations(root)

            self.assertEqual(len(files), 61)
            self.assertEqual(files[0].name, "V000001__fixture.sql")
            self.assertEqual(files[-1].name, "V000061__fixture.sql")
            self.assertNotIn("V000062__fixture.sql", {path.name for path in files})

    def test_rejects_missing_or_duplicate_historical_versions(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for version in range(1, 62):
                (root / f"V{version:06d}__fixture.sql").write_text("SELECT 1;\n")
            (root / "V000061__duplicate.sql").write_text("SELECT 1;\n")

            with self.assertRaises(HistoricalMigrationInventoryError):
                historical_v1_v61_migrations(root)


if __name__ == "__main__":
    unittest.main()
