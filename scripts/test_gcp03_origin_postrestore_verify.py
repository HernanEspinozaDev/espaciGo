import unittest
from unittest.mock import patch
import sys

from gcp03_origin_postrestore_verify import (
    EXPECTED_EXTENSIONS, READ_ONLY_SQL, VerificationError, current_migration_history,
    current_migration_inventory_sha256,
    read_gcs_generation, shutdown_proxy, table_names_from_toc, verify_report,
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

    def test_exact_generation_reader_checks_bucket_object_size_and_hash(self):
        import hashlib
        import sys
        import types
        payload = b"reviewed bytes"
        class Blob:
            generation = 17
            size = len(payload)
            def reload(self, *, if_generation_match):
                self.assert_generation = if_generation_match
            def download_as_bytes(self, *, if_generation_match, checksum):
                self.assert_download = (if_generation_match, checksum)
                return payload
        blob = Blob()
        class Bucket:
            def blob(self, name, *, generation):
                self.assert_args = (name, generation)
                return blob
        class Client:
            def bucket(self, name):
                self.assert_bucket = name
                return Bucket()
        cloud = types.ModuleType("google.cloud")
        cloud.storage = types.SimpleNamespace(Client=Client)
        google = types.ModuleType("google")
        google.cloud = cloud
        with patch.dict(sys.modules, {"google": google, "google.cloud": cloud}):
            actual = read_gcs_generation(bucket_name="espacigo-gcp03-origin-g03o-1017",
                object_name="runs/g03o-1017/toc.list", generation=17,
                expected_bytes=len(payload), expected_sha256=hashlib.sha256(payload).hexdigest())
        self.assertEqual(actual, payload)
        self.assertEqual(blob.assert_generation, 17)
        self.assertEqual(blob.assert_download, (17, "crc32c"))
        with self.assertRaises(VerificationError):
            read_gcs_generation(bucket_name="other", object_name="runs/g03o-1017/toc.list",
                generation=17, expected_bytes=len(payload), expected_sha256=hashlib.sha256(payload).hexdigest())

    def test_proxy_shutdown_is_bounded_and_sanitized(self):
        class Response:
            status = 204
            def read(self): pass
        class Connection:
            def __init__(self, host, port, timeout):
                self.args = (host, port, timeout)
            def request(self, method, path):
                self.requested = (method, path)
            def getresponse(self): return Response()
            def close(self): pass
        with patch("gcp03_origin_postrestore_verify.http.client.HTTPConnection", Connection):
            self.assertTrue(shutdown_proxy())
        self.assertEqual(Connection("127.0.0.1", 9091, 3).args, ("127.0.0.1", 9091, 3))

    def test_entrypoint_shuts_down_proxy_after_success_and_verification_failure(self):
        import gcp03_origin_postrestore_verify as module
        argv = ["verify", "--artifact-bucket", "espacigo-gcp03-origin-g03o-1017",
            "--toc-object", "runs/g03o-1017/toc.list", "--toc-generation", "1",
            "--toc-bytes", "1", "--expected-toc-sha256", "a" * 64,
            "--normalization-object", "runs/g03o-1017/normalization.sql",
            "--normalization-generation", "2", "--normalization-bytes", "1",
            "--expected-normalization-sha256", "b" * 64,
            "--expected-migration-sha256", "c" * 64, "--expected-run-id", "g03o-1017",
            "--expected-source-seal-sha256", "d" * 64, "--expected-receipt-sha256", "e" * 64,
            "--expected-manifest-sha256", "f" * 64, "--database-user", "observer@project.iam"]
        with patch.object(sys, "argv", argv), patch.object(module, "read_gcs_generation", side_effect=[b"t", b"n"]), \
             patch.object(module, "wait_for_proxy"), patch.object(module, "run_verifier", return_value={"status": "verified"}), \
             patch.object(module, "shutdown_proxy", return_value=True) as shutdown:
            module.main()
            shutdown.assert_called_once_with()
        with patch.object(sys, "argv", argv), patch.object(module, "read_gcs_generation", side_effect=VerificationError("failed")), \
             patch.object(module, "shutdown_proxy", return_value=True) as shutdown:
            with self.assertRaises(SystemExit):
                module.main()
            shutdown.assert_called_once_with()


if __name__ == "__main__":
    unittest.main()
