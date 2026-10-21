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
        for index, key, value in ((0, "project", "wrong-project"), (0, "location", "wrong-region"),
                                  (0, "name", "other-service"),
                                  (7, "project", "wrong-project"),
                                  (7, "secret_id", "other-secret")):
            changed = self.summary()
            changed["iam_changes"][index][key] = value
            self.assertFalse(valid(changed, "b" * 64))
        changed = self.summary()
        changed["iam_changes"][0]["condition"] = {"title": "unexpected"}
        self.assertFalse(valid(changed, "b" * 64))
        serialized = self.summary()
        self.assertIn("secret_id", serialized["iam_changes"][-1])
        self.assertNotIn("secret_payload", str(serialized))
        self.assertFalse(valid(self.summary(), "f" * 64))


if __name__ == "__main__":
    unittest.main()
