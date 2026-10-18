import unittest

from gcp03_origin_cloud_import_executor import ImportStopped
from gcp03_origin_cloud_recovery import config
from gcp03_origin_recovery_preflight import DIAGNOSTIC_SHA256


class CloudRecoveryConfigTests(unittest.TestCase):
    def env(self, **overrides):
        values = {
            "GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT": "runs/g03o-1017/receipt.json",
            "GCP03_ORIGIN_IMPORT_RECEIPT_GENERATION": "123",
            "GCP03_ORIGIN_IMPORT_RECEIPT_SHA256": "a" * 64,
            "GCP03_ORIGIN_IMPORT_RECEIPT_BYTES": "100",
            "GCP03_ORIGIN_IMPORT_BUCKET": "espacigo-gcp03-origin-g03o-1017",
            "GCP03_ORIGIN_PRIVATE_FILES_BUCKET": "espacigo-gcp03-private-g03o-1017",
            "GCP03_ORIGIN_IMPORT_MANIFEST_SHA256": "b" * 64,
            "GCP03_ORIGIN_IMPORT_BOOTSTRAP_SHA256": "c" * 64,
            "GCP03_ORIGIN_IMPORT_OBSERVER_SHA256": "d" * 64,
            "GCP03_ORIGIN_IMPORT_NORMALIZATION_SHA256": "e" * 64,
            "GCP03_ORIGIN_IMPORT_TRANSPORT_KEY_FILE": "/secrets/transport/k_run",
            "GCP03_ORIGIN_RECOVERY_MODE": "plan",
            "GCP03_ORIGIN_RECOVERY_DIAGNOSTIC_SHA256": DIAGNOSTIC_SHA256,
        }
        values.update(overrides)
        return values

    def test_exact_existing_receipt_path_is_accepted(self):
        cfg, ref, mode = config(self.env())
        self.assertEqual(ref.name, "runs/g03o-1017/receipt.json")
        self.assertEqual(mode, "plan")
        self.assertEqual(cfg.receipt.generation, 123)

    def test_other_receipt_path_is_rejected(self):
        with self.assertRaises(ImportStopped):
            config(self.env(GCP03_ORIGIN_IMPORT_RECEIPT_OBJECT="runs/other/receipt.json"))


if __name__ == "__main__":
    unittest.main()
