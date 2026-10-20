import copy
import unittest
from unittest.mock import patch

from verify_gcp07_environment import event_gate_ready, is_ready, published_release_exists
from verify_gcp07_plan import ALLOWED, IMAGE_PATH, PolicyError, validate
from classify_gcp07_release_changes import classify, migrations_pending
from verify_gcp07_manifest_scope import rollout_allowed


def environment():
    return {
        "protection_rules": [],
        "deployment_branch_policy": {"protected_branches": False, "custom_branch_policies": True},
    }


def resource(address, image):
    base = {"name": "service", "template": [{"containers": [{"image": "old"}]}]}
    after = copy.deepcopy(base)
    after["template"][0]["containers"][0]["image"] = image
    return {"address": address, "change": {"actions": ["update"], "before": base, "after": after}}


class Gcp07dPolicyTests(unittest.TestCase):
    def test_environment_allows_continuous_rollout_without_reviewer_gate(self):
        self.assertTrue(is_ready(environment(), {"branch_policies": [{"name": "main"}]}))
        self.assertFalse(is_ready(environment(), {"branch_policies": [{"name": "main"}, {"name": "release/*"}]}))
        reviewer_gate = environment()
        reviewer_gate["protection_rules"] = [{"type": "required_reviewers"}]
        self.assertFalse(is_ready(reviewer_gate, {"branch_policies": [{"name": "main"}]}))

    def test_automatic_trigger_requires_successful_publish_and_artifact(self):
        run = {"status": "completed", "conclusion": "success", "event": "push",
               "head_branch": "main", "path": ".github/workflows/application-release.yml",
               "head_repository": {"full_name": "HernanEspinozaDev/espaciGo"}}
        jobs = {"jobs": [{"name": "publish digest-pinned images and attestations", "conclusion": "success"}]}
        artifact = {"artifacts": [{"name": "gcp07d-release-42", "expired": False}]}
        with patch("verify_gcp07_environment.request_json", side_effect=[run, jobs, artifact]):
            self.assertTrue(published_release_exists("HernanEspinozaDev/espaciGo", "42", "token"))
        artifact["artifacts"][0]["expired"] = True
        with patch("verify_gcp07_environment.request_json", side_effect=[run, jobs, artifact]):
            self.assertFalse(published_release_exists("HernanEspinozaDev/espaciGo", "42", "token"))

    def test_release_push_checks_environment_without_rollout_flags(self):
        self.assertTrue(event_gate_ready("push", "", False, False))
        self.assertTrue(event_gate_ready("workflow_dispatch", "manual-publish", True, False))
        self.assertFalse(event_gate_ready("workflow_dispatch", "manual-publish", False, False))
        self.assertFalse(event_gate_ready("workflow_run", "auto", True, False))
        self.assertTrue(event_gate_ready("workflow_dispatch", "preview", True, False))
        self.assertFalse(event_gate_ready("workflow_dispatch", "apply", True, True,
                                          "", "42", "f" * 64))

    def test_migration_only_and_mixed_changes_are_marked_and_blocked(self):
        migration_only = classify(["db/migrations/V65__example.sql"])
        self.assertTrue(migration_only["api"])
        self.assertTrue(migration_only["migrations"])
        self.assertFalse(migration_only["web"])
        manifest = {"schema": 1, "contains_migrations": migration_only["migrations"], "source_sha": "b" * 40, "publish_only": False}
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, []))

        mixed = classify(["internal/api/handler.go", "frontend/src/app.ts", "db/migrations/V65__example.sql"])
        self.assertTrue(mixed["api"])
        self.assertTrue(mixed["web"])
        self.assertTrue(mixed["migrations"])
        manifest = {"schema": 1, "contains_migrations": mixed["migrations"], "source_sha": "b" * 40, "publish_only": False}
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, []))

        app_only = classify(["internal/api/handler.go", "frontend/src/app.ts"])
        manifest = {"schema": 1, "contains_migrations": app_only["migrations"], "source_sha": "b" * 40, "publish_only": False}
        self.assertTrue(rollout_allowed(manifest, "a" * 40, True, []))
        self.assertFalse(rollout_allowed(manifest, "", False, []))
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, ["db/migrations/V66__example.sql"]))
        self.assertFalse(rollout_allowed(manifest, "c" * 40, False, []))
        self.assertFalse(rollout_allowed({"schema": 1, "contains_migrations": False}, "a" * 40, True, []))
        self.assertFalse(rollout_allowed({**manifest, "publish_only": True}, "a" * 40, True, []))

    def test_migration_baseline_fails_closed_and_detects_pending_paths(self):
        self.assertTrue(migrations_pending("", "b" * 40))
        with patch("classify_gcp07_release_changes.subprocess.run", return_value=type("Result", (), {"returncode": 0})()):
            with patch("classify_gcp07_release_changes.subprocess.check_output", return_value="db/migrations/V65_example.sql\n"):
                self.assertTrue(migrations_pending("a" * 40, "b" * 40))
            with patch("classify_gcp07_release_changes.subprocess.check_output", return_value=""):
                self.assertFalse(migrations_pending("a" * 40, "b" * 40))

    def test_accepts_single_allowlisted_digest_update(self):
        image = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        address = next(key for key, value in ALLOWED.items() if value[0] == "api")
        result = validate({"applyable": True, "resource_changes": [resource(address, image)]}, {"api": image}, "b" * 40, "123")
        self.assertEqual(len(result["fingerprint"]), 64)

    def test_accepts_empty_plan_as_noop(self):
        result = validate({"applyable": True, "resource_changes": []}, {}, "b" * 40, "123")
        self.assertEqual(result["changes"], [])

    def test_rejects_create_wrong_image_and_non_image_change(self):
        image = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        address = next(key for key, value in ALLOWED.items() if value[0] == "api")
        bad = resource(address, image)
        bad["change"]["actions"] = ["delete", "create"]
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": [bad]}, {"api": image}, "b" * 40, "123")
        bad = resource(address, image)
        bad["change"]["after"]["name"] = "other"
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": [bad]}, {"api": image}, "b" * 40, "123")
        bad = resource(address, "example.invalid/image:latest")
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": [bad]}, {"api": bad["change"]["after"]["template"][0]["containers"][0]["image"]}, "b" * 40, "123")


if __name__ == "__main__":
    unittest.main()
