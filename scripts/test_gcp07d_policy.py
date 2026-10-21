import copy
import unittest
from unittest.mock import patch

from verify_gcp07_environment import (
    event_gate_ready,
    is_ready,
    published_release_exists,
    successful_preview_exists,
    v67_runtime_ready,
)
from verify_gcp07_plan import ALLOWED, IMAGE_PATH, PolicyError, validate
from classify_gcp07_release_changes import classify, migrations_pending
from verify_gcp07_manifest_scope import baseline_has_v67_migration, rollout_allowed


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


def initial_resource(address, image):
    name = {
        "google_cloud_run_v2_service.api[0]": "espacigo-test-api",
        'google_cloud_run_v2_service.web["public"]': "espacigo-test-public",
        'google_cloud_run_v2_service.web["private"]': "espacigo-test-private",
    }[address]
    after = {
        "name": name,
        "project": "espacigo-testprod",
        "location": "southamerica-west1",
        "ingress": "INGRESS_TRAFFIC_ALL",
        "deletion_protection": True,
        "template": [{"containers": [{"image": image, "env": []}], "volumes": []}],
    }
    if address == 'google_cloud_run_v2_service.web["public"]':
        after["iap_enabled"] = False
    if address == 'google_cloud_run_v2_service.web["private"]':
        after["iap_enabled"] = True
    if address == "google_cloud_run_v2_service.api[0]":
        after["template"][0]["containers"][0]["env"] = [
            {"name": "CLOUD_SQL_INSTANCE_CONNECTION_NAME", "value": "espacigo-testprod:southamerica-west1:espacigo-testprod-db"},
            {"name": "DATABASE_NAME", "value": "espacigo_dms_stage_20261020"},
            {"name": "DATABASE_USER", "value": "espacigo_api_login"},
            {"name": "DATABASE_SET_ROLE", "value": "espacigo_runtime"},
        ]
        after["template"][0]["volumes"] = [{"name": "database-secret", "secret": [{
            "secret": "espacigo-test-database-password-v67",
            "items": [{"path": "password", "version": "7"}],
        }]}]
    return {"address": address, "change": {"actions": ["create"], "before": None, "after": after}}


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

    def test_manual_apply_requires_a_successful_main_preview_artifact(self):
        run = {"status": "completed", "conclusion": "success", "event": "workflow_dispatch",
               "head_branch": "main", "path": ".github/workflows/application-rollout.yml",
               "head_repository": {"full_name": "HernanEspinozaDev/espaciGo"}}
        artifact = {"artifacts": [{"name": "gcp07d-preview-42", "expired": False}]}
        with patch("verify_gcp07_environment.request_json", side_effect=[run, artifact]):
            self.assertTrue(successful_preview_exists("HernanEspinozaDev/espaciGo", "42", "token"))
        artifact["artifacts"][0]["expired"] = True
        with patch("verify_gcp07_environment.request_json", side_effect=[run, artifact]):
            self.assertFalse(successful_preview_exists("HernanEspinozaDev/espaciGo", "42", "token"))

    def test_release_push_checks_environment_without_rollout_flags(self):
        self.assertTrue(event_gate_ready("push", "", False, False))
        self.assertTrue(event_gate_ready("workflow_dispatch", "manual-publish", True, False))
        self.assertFalse(event_gate_ready("workflow_dispatch", "manual-publish", False, False))
        self.assertFalse(event_gate_ready("workflow_run", "auto", True, False))
        self.assertTrue(event_gate_ready("workflow_dispatch", "preview", True, False))
        self.assertFalse(event_gate_ready("workflow_dispatch", "apply", True, True,
                                          "", "42", "f" * 64))
        self.assertTrue(event_gate_ready("workflow_dispatch", "bootstrap-preview", True, False))
        self.assertTrue(event_gate_ready("workflow_dispatch", "bootstrap-apply", True, True,
                                         "APPLY GCP-08 INITIAL RESTORE", "42", "f" * 64))
        self.assertFalse(event_gate_ready("workflow_dispatch", "bootstrap-apply", True, False,
                                          "APPLY GCP-08 INITIAL RESTORE", "42", "f" * 64))
        self.assertFalse(event_gate_ready("workflow_dispatch", "bootstrap-apply", True, True,
                                          "APPLY GCP-07D", "42", "f" * 64))
        self.assertTrue(v67_runtime_ready("7", "a" * 64))
        self.assertFalse(v67_runtime_ready("latest", "a" * 64))
        self.assertFalse(v67_runtime_ready("7", "0" * 64))

    def test_migration_only_and_mixed_changes_are_marked_and_blocked(self):
        migration_only = classify(["db/migrations/V65__example.sql"])
        self.assertTrue(migration_only["api"])
        self.assertTrue(migration_only["migrations"])
        self.assertFalse(migration_only["web"])
        manifest = {"schema": 1, "contains_migrations": migration_only["migrations"], "source_sha": "b" * 40, "publish_only": False}
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, [], True))

        mixed = classify(["internal/api/handler.go", "frontend/src/app.ts", "db/migrations/V65__example.sql"])
        self.assertTrue(mixed["api"])
        self.assertTrue(mixed["web"])
        self.assertTrue(mixed["migrations"])
        manifest = {"schema": 1, "contains_migrations": mixed["migrations"], "source_sha": "b" * 40, "publish_only": False}
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, [], True))

        app_only = classify(["internal/api/handler.go", "frontend/src/app.ts"])
        manifest = {"schema": 1, "contains_migrations": app_only["migrations"], "source_sha": "b" * 40, "publish_only": False}
        self.assertTrue(rollout_allowed(manifest, "a" * 40, True, [], True))
        self.assertFalse(rollout_allowed(manifest, "", False, [], False))
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, ["db/migrations/V66__example.sql"], True))
        self.assertFalse(rollout_allowed(manifest, "c" * 40, False, [], False))
        self.assertFalse(rollout_allowed({"schema": 1, "contains_migrations": False}, "a" * 40, True, [], True))
        self.assertFalse(rollout_allowed({**manifest, "publish_only": True}, "a" * 40, True, [], True))
        self.assertFalse(rollout_allowed(manifest, "a" * 40, True, [], False))

    def test_schema_baseline_must_contain_v67_migration(self):
        self.assertFalse(baseline_has_v67_migration(""))
        with patch("verify_gcp07_manifest_scope.subprocess.run", return_value=type("Result", (), {"returncode": 0})()) as run:
            self.assertTrue(baseline_has_v67_migration("a" * 40))
            self.assertIn("V000067__inmuebles_residential_rentals.sql", run.call_args.args[0][-1])
        with patch("verify_gcp07_manifest_scope.subprocess.run", return_value=type("Result", (), {"returncode": 1})()):
            self.assertFalse(baseline_has_v67_migration("a" * 40))

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

    def test_initial_restore_allows_only_the_exact_three_service_creates(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        plan = {"applyable": True, "resource_changes": [
            initial_resource(address, api if component == "api" else web)
            for address, (component, _) in ALLOWED.items()
        ]}
        result = validate(plan, {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")
        self.assertEqual({row["action"] for row in result["changes"]}, {"create"})
        self.assertEqual({row["address"] for row in result["changes"]}, set(ALLOWED))
        self.assertEqual(len(result["fingerprint"]), 64)
        with self.assertRaises(PolicyError):
            validate(plan, {"api": api, "web": web}, "c" * 40, "123", "update", "7")

    def test_initial_restore_rejects_v67_target_drift_partial_create_and_extra_resources(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = [initial_resource(address, api if component == "api" else web)
                     for address, (component, _) in ALLOWED.items()]
        expected = {"api": api, "web": web}
        plan = {"applyable": True, "resource_changes": copy.deepcopy(resources)}
        plan["resource_changes"][0]["change"]["after"]["template"][0]["containers"][0]["env"][0]["value"] = "old-db"
        with self.assertRaises(PolicyError):
            validate(plan, expected, "c" * 40, "123", "bootstrap", "7")
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": resources[:2]}, expected, "c" * 40, "123", "bootstrap", "7")
        extra = copy.deepcopy(resources)
        extra.append({"address": "google_project_iam_member.extra", "change": {"actions": ["create"]}})
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": extra}, expected, "c" * 40, "123", "bootstrap", "7")

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
