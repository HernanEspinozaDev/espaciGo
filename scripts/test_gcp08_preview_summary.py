import unittest

from verify_gcp07_plan import BOOTSTRAP_IAM, initial_iam_summary
from verify_gcp08_preview_summary import TARGETS, valid


class Gcp08PreviewSummaryTests(unittest.TestCase):
    def summary(self):
        return {
            "mode": "bootstrap",
            "source_sha": "a" * 40,
            "release_run_id": "123",
            "fingerprint": "b" * 64,
            "changes": [
                {"address": address, "action": "create",
                 "image": "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
                          + image + "@sha256:" + ("c" if image == "espacigo-api" else "d") * 64}
                for address, image in TARGETS.items()
            ],
            "iam_changes": [initial_iam_summary(address) for address in BOOTSTRAP_IAM],
        }

    def test_accepts_only_complete_exact_initial_service_targets(self):
        self.assertTrue(valid(self.summary(), "b" * 64))
        summary = self.summary()
        evaluator = next(item for item in summary["iam_changes"] if item["address"].startswith("google_iap_web_cloud_run_service_iam_member"))
        evaluator["action"] = "no-op"
        self.assertTrue(valid(summary, "b" * 64))

    def test_rejects_wrong_mode_action_target_image_and_fingerprint(self):
        summary = self.summary()
        summary["mode"] = "update"
        self.assertFalse(valid(summary, "b" * 64))
        summary = self.summary()
        summary["changes"][0]["action"] = "update"
        self.assertFalse(valid(summary, "b" * 64))
        summary = self.summary()
        summary["changes"].pop()
        self.assertFalse(valid(summary, "b" * 64))
        summary = self.summary()
        summary["changes"][0]["image"] = "example.invalid/api:latest"
        self.assertFalse(valid(summary, "b" * 64))
        summary = self.summary()
        summary["iam_changes"].pop()
        self.assertFalse(valid(summary, "b" * 64))
        summary = self.summary()
        summary["iam_changes"][0]["member"] = "allUsers"
        self.assertFalse(valid(summary, "b" * 64))
        for address, key, value in (
                ('google_cloud_run_v2_service_iam_member.api_invoker["public"]', "project", "wrong-project"),
                ('google_cloud_run_v2_service_iam_member.api_invoker["public"]', "location", "wrong-region"),
                ('google_cloud_run_v2_service_iam_member.api_invoker["public"]', "name", "other-service"),
                ("google_secret_manager_secret_iam_member.v67_api_secret_access[0]", "project", "wrong-project"),
                ("google_secret_manager_secret_iam_member.v67_api_secret_access[0]", "secret_id", "other-secret"),
                ('google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]',
                 "cloud_run_service_name", "projects/elsewhere/iap_web/other")):
            changed = self.summary()
            row = next(item for item in changed["iam_changes"] if item["address"] == address)
            row[key] = value
            self.assertFalse(valid(changed, "b" * 64))
        changed = self.summary()
        changed["iam_changes"][0]["condition"] = {"title": "unexpected"}
        self.assertFalse(valid(changed, "b" * 64))
        serialized = self.summary()
        secret = next(item for item in serialized["iam_changes"] if item["address"].startswith("google_secret_manager"))
        self.assertIn("secret_id", secret)
        self.assertNotIn("secret_payload", str(serialized))
        self.assertFalse(valid(self.summary(), "f" * 64))


if __name__ == "__main__":
    unittest.main()
