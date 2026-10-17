import unittest

from gcp03_origin_postrestore_verify import (
    EXPECTED_EXTENSIONS, READ_ONLY_SQL, VerificationError, current_migration_history,
    current_migration_inventory_sha256,
    table_names_from_toc, verify_report,
)


class PostRestoreVerifyTests(unittest.TestCase):
    def _report(self):
        return {"database": "espacigo_scratch", "server_version_num": 180006,
            "current_user": "espacigo_gcp03_observer", "current_user_superuser": False,
            "observer_role_active": True,
            "extensions": EXPECTED_EXTENSIONS, "public_tables": ["sample", "sample_partition"],
            "migration_history": current_migration_history(),
            "application_owners_valid": True, "valid_sessions": 0,
            "usable_action_tokens": 0, "pending_notices": 0,
            "identity_event_leases": 0, "runtime_role_login": False}

    def _verify(self, report):
        digest = "a" * 64
        return verify_report(report, {"sample", "sample_partition"}, expected_run_id="g03o-1017",
            expected_source_seal_sha256=digest, expected_receipt_sha256=digest,
            expected_manifest_sha256=digest, expected_toc_sha256=digest,
            expected_normalization_sha256=digest, expected_migration_sha256=current_migration_inventory_sha256(),
            expected_migration_history=report["migration_history"])

    def test_reports_sanitized_read_only_database_inventory(self):
        result = self._verify(self._report())
        self.assertEqual(result["status"], "verified")
        self.assertEqual(result["public_table_count"], 2)
        self.assertFalse(result["database_rows_emitted"])
        self.assertIn("BEGIN TRANSACTION READ ONLY", READ_ONLY_SQL)
        self.assertNotIn("UPDATE ", READ_ONLY_SQL)

    def test_rejects_sanitation_mismatch_and_inventory_mismatch(self):
        report = self._report()
        report["pending_notices"] = 1
        with self.assertRaises(VerificationError):
            self._verify(report)
        report = self._report()
        report["public_tables"].append("unexpected")
        with self.assertRaises(VerificationError):
            self._verify(report)
        report = self._report()
        report["migration_history"] = []
        with self.assertRaises(VerificationError):
            self._verify(report)

    def test_toc_parser_collects_public_tables_and_ignores_table_data(self):
        toc = (b"; pg_dump TOC\n"
               b"1; 0 0 TABLE public sample postgres\n"
               b"2; 0 0 TABLE DATA public sample postgres\n"
               b"3; 0 0 TABLE public \"sample partition\" postgres\n")
        self.assertEqual(table_names_from_toc(toc), {"sample", "sample partition"})
        with self.assertRaises(VerificationError):
            table_names_from_toc(b"not a pg_restore TOC entry\n")


if __name__ == "__main__":
    unittest.main()
