import unittest
from contextlib import ExitStack
from types import SimpleNamespace
from unittest.mock import patch

from gcp03_origin_cloud_import_executor import ImportStopped
import gcp03_origin_cloud_recovery as cloud
from gcp03_origin_cloud_recovery import config
from gcp03_origin_cloud_import_executor import EXPECTED_TARGET
from gcp03_origin_recovery_preflight import DIAGNOSTIC_SHA256


class CloudRecoveryConfigTests(unittest.TestCase):
    def env(self, **overrides):
        values = {
            "GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT": "runs/g03o-1017/receipt.json",
            "GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION": str(cloud.U2_RECEIPT_GENERATION),
            "GCP03_ORIGIN_IMPORT_RECEIPT_SHA256": cloud.U2_RECEIPT_SHA256,
            "GCP03_ORIGIN_IMPORT_RECEIPT_BYTES": str(cloud.U2_RECEIPT_BYTES),
            "GCP03_ORIGIN_IMPORT_BUCKET": "espacigo-gcp03-origin-g03o-1017",
            "GCP03_ORIGIN_PRIVATE_FILES_BUCKET": "espacigo-gcp03-private-g03o-1017",
            "GCP03_ORIGIN_IMPORT_MANIFEST_SHA256": cloud.U2_MANIFEST_SHA256,
            "GCP03_ORIGIN_IMPORT_BOOTSTRAP_SHA256": "c" * 64,
            "GCP03_ORIGIN_IMPORT_OBSERVER_SHA256": "d" * 64,
            "GCP03_ORIGIN_IMPORT_NORMALIZATION_SHA256": cloud.U2_NORMALIZATION_SHA256,
            "GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE": "/secrets/transport/k_run",
            "GCP03_ORIGIN_RECOVERY_MODE": "plan",
            "GCP03_ORIGIN_RECOVERY_DIAGNOSTIC_SHA256": DIAGNOSTIC_SHA256,
            "GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256": cloud.F4_IMPORT_FAILURE_SHA256,
            "GCP03_ORIGIN_IMPORT_CONFIRM_TARGET": EXPECTED_TARGET,
            "CLOUD_SQL_INSTANCE_CONNECTION_NAME": EXPECTED_TARGET.rsplit("/", 1)[0],
            "CLOUD_SQL_PRIVATE_IP": "true",
            "DATABASE_PASSWORD_FILE": "/secrets/admin/password",
        }
        values.update(overrides)
        return values

    def test_exact_existing_receipt_path_is_accepted(self):
        cfg, ref, mode = config(self.env())
        self.assertEqual(ref.name, "runs/g03o-1017/receipt.json")
        self.assertEqual(mode, "plan")
        self.assertEqual(cfg.receipt.generation, cloud.U2_RECEIPT_GENERATION)

    def test_other_receipt_path_is_rejected(self):
        with self.assertRaises(ImportStopped):
            config(self.env(GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT="runs/other/receipt.json"))

    def test_wrong_cloud_sql_target_or_network_is_rejected(self):
        for overrides in (
            {"GCP03_ORIGIN_IMPORT_CONFIRM_TARGET": "other-project:region:instance/db"},
            {"CLOUD_SQL_INSTANCE_CONNECTION_NAME": "other-project:region:instance"},
            {"CLOUD_SQL_PRIVATE_IP": "false"},
        ):
            with self.subTest(overrides=overrides), self.assertRaises(ImportStopped):
                config(self.env(**overrides))

    def test_secret_paths_and_hash_formats_are_fixed(self):
        for overrides in (
            {"DATABASE_PASSWORD_FILE": "/tmp/password"},
            {"GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE": "/tmp/k_run"},
            {"GCP03_ORIGIN_IMPORT_MANIFEST_SHA256": "bad"},
            {"GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION": "123"},
        ):
            with self.subTest(overrides=overrides), self.assertRaises(ImportStopped):
                config(self.env(**overrides))

    def test_failed_import_evidence_is_required_and_exact(self):
        missing = self.env()
        del missing["GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256"]
        with self.assertRaises(ImportStopped):
            config(missing)
        with self.assertRaises(ImportStopped):
            config(self.env(GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256="f" * 64))


class CloudRecoveryRunTests(unittest.TestCase):
    def setUp(self):
        self.events = []
        self.cfg = SimpleNamespace(
            expected_receipt_sha256="a" * 64,
            expected_manifest_sha256="b" * 64,
            expected_normalization_sha256="c" * 64,
            private_files_bucket="espacigo-gcp03-private-g03o-1017",
        )
        self.plan = SimpleNamespace(summary={
            "plan_sha256": "1" * 64, "catalog_facts_sha256": "2" * 64,
            "pending_statement_count": 3, "satisfied_statement_count": 4,
            "binding": {"private_files_ciphertext_sha256": "3" * 64},
        })

    def test_plan_mode_is_read_only_and_closes_proxy(self):
        stack = ExitStack()
        with stack:
            store_patch = stack.enter_context(patch.object(cloud, "GCSPrivateFileStore", side_effect=AssertionError("plan created a writer")))
            execute_patch = stack.enter_context(patch.object(cloud, "execute_reviewed_recovery", side_effect=AssertionError("plan executed recovery")))
            stack.enter_context(patch.object(cloud, "config", return_value=(self.cfg, object(), "plan")))
            stack.enter_context(patch.object(cloud, "GCSObjectStore", return_value=object()))
            stack.enter_context(patch.object(cloud, "wait_for_proxy", side_effect=lambda: self.events.append("proxy-ready")))
            stack.enter_context(patch.object(cloud, "CloudRecoveryDatabase", side_effect=lambda path: SimpleNamespace(passfile="/tmp/test-passfile", close=lambda: self.events.append("db-close"))))
            stack.enter_context(patch.object(cloud, "authenticated_inputs", side_effect=lambda *a: (self.events.append("inputs-authenticated") or (None, None, SimpleNamespace(generation=7), object(), {"plain_sha256": "4" * 64}, [], b"sql"))))
            stack.enter_context(patch.object(cloud, "CatalogReader", side_effect=lambda **kw: SimpleNamespace()))
            stack.enter_context(patch.object(cloud, "make_plan", return_value=self.plan))
            stack.enter_context(patch.object(cloud, "shutdown_proxy", side_effect=lambda: self.events.append("proxy-shutdown") or True))
            result = cloud.run({"GCP03_ORIGIN_RECOVERY_MODE": "plan",
                                "GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256": cloud.F4_IMPORT_FAILURE_SHA256,
                                "DATABASE_PASSWORD_FILE": "/secrets/admin/password"})
        self.assertTrue(result["phase_success"], result)
        self.assertTrue(result["report_complete"])
        self.assertFalse(result["execution_performed"])
        self.assertEqual(self.events, ["proxy-ready", "inputs-authenticated", "db-close", "proxy-shutdown"])

    def test_apply_plan_drift_stops_before_mutation_and_closes_proxy(self):
        with patch.object(cloud, "config", return_value=(self.cfg, object(), "apply")), \
             patch.object(cloud, "GCSObjectStore", return_value=object()), \
             patch.object(cloud, "wait_for_proxy"), \
             patch.object(cloud, "CloudRecoveryDatabase", side_effect=lambda path: SimpleNamespace(passfile="/tmp/test-passfile", close=lambda: self.events.append("db-close"))), \
             patch.object(cloud, "authenticated_inputs", side_effect=lambda *a: (None, None, SimpleNamespace(generation=7), object(), {"plain_sha256": "4" * 64}, [], b"sql")), \
             patch.object(cloud, "CatalogReader", side_effect=lambda **kw: SimpleNamespace()), \
             patch.object(cloud, "make_plan", return_value=self.plan), \
             patch.object(cloud, "execute_reviewed_recovery") as execute, \
             patch.object(cloud, "shutdown_proxy", side_effect=lambda: self.events.append("proxy-shutdown") or True):
            result = cloud.run({"GCP03_ORIGIN_RECOVERY_MODE": "apply",
                                "GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256": cloud.F4_IMPORT_FAILURE_SHA256,
                                "GCP03_ORIGIN_RECOVERY_APPROVED_PLAN_SHA256": "9" * 64,
                                "DATABASE_PASSWORD_FILE": "/secrets/admin/password"})
        self.assertEqual(result["error_code"], "CURRENT_PLAN_DIFFERS_FROM_APPROVED_PLAN")
        self.assertEqual(result["transaction_outcome"], "not_started")
        execute.assert_not_called()
        self.assertEqual(self.events, ["db-close", "proxy-shutdown"])

    def test_receipt_hash_mismatch_stops_before_reading_manifest(self):
        cfg = SimpleNamespace(receipt=object(), bucket="bucket", expected_receipt_sha256="a" * 64)
        store = object()
        with patch.object(cloud, "read_exact", return_value=b"wrong") as read:
            with self.assertRaises(ImportStopped):
                cloud.authenticated_inputs(cfg, store)
        read.assert_called_once_with(store, "bucket", cfg.receipt)

    def test_authentication_failure_precedes_catalog_and_recovery_operations(self):
        with patch.object(cloud, "config", return_value=(self.cfg, object(), "apply")), \
             patch.object(cloud, "GCSObjectStore", return_value=object()), \
             patch.object(cloud, "wait_for_proxy"), \
             patch.object(cloud, "CloudRecoveryDatabase", side_effect=lambda path: SimpleNamespace(passfile="/tmp/test-passfile", close=lambda: self.events.append("db-close"))), \
             patch.object(cloud, "authenticated_inputs", side_effect=ImportStopped("RECEIPT_HASH_MISMATCH", "receipt")), \
             patch.object(cloud, "CatalogReader") as reader, \
             patch.object(cloud, "execute_reviewed_recovery") as execute, \
             patch.object(cloud, "shutdown_proxy", side_effect=lambda: self.events.append("proxy-shutdown") or True):
            result = cloud.run({"GCP03_ORIGIN_RECOVERY_MODE": "apply",
                                "GCP03_ORIGIN_RECOVERY_IMPORT_FAILURE_SHA256": cloud.F4_IMPORT_FAILURE_SHA256,
                                "DATABASE_PASSWORD_FILE": "/secrets/admin/password"})
        self.assertEqual(result["error_code"], "RECEIPT_HASH_MISMATCH")
        self.assertEqual(result["transaction_outcome"], "not_started")
        reader.assert_not_called()
        execute.assert_not_called()
        self.assertEqual(self.events, ["db-close", "proxy-shutdown"])

if __name__ == "__main__":
    unittest.main()
