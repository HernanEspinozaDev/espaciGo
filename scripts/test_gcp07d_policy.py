import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from verify_gcp07_environment import (
    event_gate_ready,
    is_ready,
    published_release_exists,
    successful_preview_exists,
    v67_runtime_ready,
)
from verify_gcp07_plan import ALLOWED, BOOTSTRAP_IAM, IMAGE_PATH, PolicyError, initial_iam_scope, initial_service_matches, validate, web_api_references_are_exact
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
    api = address.endswith("api[0]")
    private = address.endswith('["private"]')
    after = {
        "name": name, "project": "espacigo-testprod", "location": "southamerica-west1",
        "ingress": "INGRESS_TRAFFIC_ALL", "deletion_protection": True,
        "iap_enabled": private, "annotations": {}, "labels": {},
        "template": [{
            "service_account": f"espacigo-test-{'api' if api else ('private-web' if private else 'public-web')}@espacigo-testprod.iam.gserviceaccount.com",
            "max_instance_request_concurrency": 80 if api else 40,
            "scaling": [{"min_instance_count": 1 if api else 0, "max_instance_count": 1}],
            "vpc_access": [{"egress": "PRIVATE_RANGES_ONLY", "network_interfaces": [{"network": "espacigo-testprod", "subnetwork": "espacigo-testprod-serverless"}]}],
            "containers": [{"image": image, "env": [], "resources": [{"limits": {"cpu": "1", "memory": "1Gi"}}]}],
            "volumes": [],
        }],
    }
    container = after["template"][0]["containers"][0]
    after_unknown = {}
    if api:
        values = {
            "APP_PROFILE": "cloud-test", "LOCAL_VERIFICATION_IP_LIMIT": "20", "DATABASE_CONNECTION_MODE": "cloudsql",
            "CLOUD_SQL_INSTANCE_CONNECTION_NAME": "espacigo-testprod:southamerica-west1:espacigo-testprod-db",
            "DATABASE_NAME": "espacigo_dms_stage_20261020", "DATABASE_PASSWORD_FILE": "/var/secrets/database/password",
            "LOCAL_CONTRACT_ENCRYPTION_KEY_FILE": "/var/secrets/contract/key", "LOCAL_PAYMENT_WEBHOOK_SECRET_FILE": "/var/secrets/payment/webhook",
            "DATABASE_USER": "espacigo_api_login", "CORS_ALLOWED_ORIGINS": "https://espacigo.testingpage.store,https://espacigo-demo.testingpage.store",
            "DATABASE_SET_ROLE": "espacigo_runtime", "FRONTEND_PUBLIC_ORIGIN": "https://espacigo.testingpage.store",
            "DATABASE_POOL_MAX_CONNS": "4", "CLOUD_SQL_PRIVATE_IP": "true", "LOCAL_BOOKING_TRIAL": "1",
            "M03_EVIDENCE_DIR": "/var/lib/espacigo/evidence",
            "CLOUD_STORAGE_BUCKET": "espacigo-test-files-testprod", "CLOUD_STORAGE_OBJECT_PREFIX": "", "BLOB_STORE_BACKEND": "gcs",
            "LOCAL_PRIVACY_REPLAY_BACKEND": "gcs", "LOCAL_PRIVACY_REPLAY_BUCKET": "espacigo-test-files-testprod", "LOCAL_PRIVACY_REPLAY_PREFIX": "",
        }
        container["env"] = [{"name": k, "value": v} for k, v in values.items()]
        container["volume_mounts"] = [
            {"name": "database-secret", "mount_path": "/var/secrets/database"},
            {"name": "contract-secret", "mount_path": "/var/secrets/contract"},
            {"name": "payment-secret", "mount_path": "/var/secrets/payment"},
        ]
        container["resources"][0]["cpu_idle"] = False
        after["template"][0]["volumes"] = [
            {"name": "database-secret", "secret": [{"secret": "espacigo-test-database-password-v67", "items": [{"path": "password", "version": "7"}]}]},
            {"name": "contract-secret", "secret": [{"secret": "espacigo-test-contract-encryption-key", "items": [{"path": "key", "version": "1"}]}]},
            {"name": "payment-secret", "secret": [{"secret": "espacigo-test-payment-webhook-secret", "items": [{"path": "webhook", "version": "latest"}]}]},
        ]
    else:
        mode = "private" if private else "public"
        values = {
            "FRONTEND_ENTRY_MODE": mode, "NG_ALLOWED_HOSTS": "espacigo-demo.testingpage.store" if mode == "private" else "espacigo.testingpage.store",
            "API_INTERNAL_BASE_URL": "https://api.run.app/api/v1", "API_PUBLIC_BASE_URL": "/api/v1",
            "API_SERVICE_AUDIENCE": "https://api.run.app", "FRONTEND_PUBLIC_ORIGIN": "https://espacigo-demo.testingpage.store" if mode == "private" else "https://espacigo.testingpage.store",
            "PRIVATE_DEMO_ORIGIN": "https://espacigo-demo.testingpage.store",
            "IAP_JWT_AUDIENCE": "/projects/123/locations/southamerica-west1/services/espacigo-test-private" if private else "",
        }
        container["env"] = [{"name": k, "value": v} for k, v in values.items()]
        for item in container["env"]:
            if item["name"] in {"API_INTERNAL_BASE_URL", "API_SERVICE_AUDIENCE"}:
                item["value"] = None
        after_unknown = {"template": [{"containers": [{"env": [
            {"value": True} if item["name"] in {"API_INTERNAL_BASE_URL", "API_SERVICE_AUDIENCE"} else {}
            for item in container["env"]
        ]}]}]}
        container["resources"][0]["cpu_idle"] = True
    change = {"actions": ["create"], "before": None, "after": after}
    if after_unknown:
        change["after_unknown"] = after_unknown
    return {"address": address, "change": change}


def bootstrap_iam_resource(address, expected):
    name, role, member = expected
    after = {"role": role, "member": member, **initial_iam_scope(address, name)}
    if address.startswith("google_iap_web_cloud_run_service_iam_member."):
        after["condition"] = []
    return {"address": address, "change": {"actions": ["create"], "before": None, "after": after}}


@unittest.skipUnless(
    (Path(__file__).resolve().parents[1] / ".github/workflows/application-rollout.yml").exists(),
    "legacy Actions-to-Terraform rollout policy is superseded by Cloud Build",
)
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
               "head_sha": "a" * 40,
               "head_repository": {"full_name": "HernanEspinozaDev/espaciGo"}}
        jobs = {"jobs": [{"name": "publish digest-pinned images and attestations", "conclusion": "success"}]}
        artifact = {"artifacts": [{"id": 42, "name": "gcp07d-release-42", "expired": False}]}
        manifest = {"schema": 1, "repository": "HernanEspinozaDev/espaciGo", "run_id": "42",
                    "source_sha": "a" * 40, "contains_migrations": False, "publish_only": False,
                    "images": {"api": "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/espacigo-api@sha256:" + "a" * 64}}
        with patch("verify_gcp07_environment.request_json", side_effect=[run, jobs, artifact]), \
                patch("verify_gcp07_environment.download_release_manifest", return_value=manifest):
            self.assertTrue(published_release_exists("HernanEspinozaDev/espaciGo", "42", "token"))
        artifact["artifacts"][0]["expired"] = True
        with patch("verify_gcp07_environment.request_json", side_effect=[run, jobs, artifact]), \
                patch("verify_gcp07_environment.download_release_manifest") as download:
            self.assertFalse(published_release_exists("HernanEspinozaDev/espaciGo", "42", "token"))
            download.assert_not_called()

    def test_manual_publish_release_requires_exact_main_manifest_and_two_images(self):
        repo = "HernanEspinozaDev/espaciGo"
        run = {"status": "completed", "conclusion": "success", "event": "workflow_dispatch",
               "head_branch": "main", "path": ".github/workflows/application-release.yml",
               "head_sha": "a" * 40, "head_repository": {"full_name": repo}}
        jobs = {"jobs": [{"name": "publish digest-pinned images and attestations", "conclusion": "success"}]}
        artifact = {"artifacts": [{"id": 42, "name": "gcp07d-release-42", "expired": False}]}
        prefix = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
        manifest = {"schema": 1, "repository": repo, "run_id": "42", "source_sha": "a" * 40,
                    "contains_migrations": False, "publish_only": True,
                    "images": {"api": prefix + "espacigo-api@sha256:" + "a" * 64,
                               "web": prefix + "espacigo-web@sha256:" + "b" * 64}}

        def check(candidate_run=run, candidate_jobs=jobs, candidate_artifact=artifact,
                  candidate_manifest=manifest, allow_manual_publish=True):
            with patch("verify_gcp07_environment.request_json",
                       side_effect=[candidate_run, candidate_jobs, candidate_artifact]), \
                    patch("verify_gcp07_environment.download_release_manifest", return_value=candidate_manifest):
                return published_release_exists(repo, "42", "token", allow_manual_publish=allow_manual_publish)

        self.assertTrue(check())
        self.assertFalse(check(allow_manual_publish=False))
        bad_manifests = (
            {**manifest, "publish_only": False},
            {**manifest, "contains_migrations": True},
            {**manifest, "source_sha": "c" * 40},
            {**manifest, "repository": "attacker/repo"},
            {**manifest, "images": {"api": manifest["images"]["api"]}},
            {**manifest, "images": {**manifest["images"], "web": "example.invalid/web:latest"}},
        )
        for bad_manifest in bad_manifests:
            self.assertFalse(check(candidate_manifest=bad_manifest))
        self.assertFalse(check(candidate_run={**run, "head_branch": "feature"}))
        self.assertFalse(check(candidate_run={**run, "head_repository": {"full_name": "attacker/repo"}}))
        self.assertFalse(check(candidate_jobs={"jobs": [{"name": "publish digest-pinned images and attestations",
                                                          "conclusion": "failure"}]}))
        self.assertFalse(check(candidate_artifact={"artifacts": [{"id": 42, "name": "other-artifact",
                                                                    "expired": False}]}))
        self.assertFalse(check(candidate_artifact={"artifacts": [{"id": 42, "name": "gcp07d-release-42",
                                                                    "expired": True}]}))

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
        self.assertFalse(v67_runtime_ready("7", "a" * 64, "planning/gcp/gcp08_v67_runtime_preflight.json", "HernanEspinozaDev/espaciGo", "token"))
        self.assertFalse(v67_runtime_ready("latest", "a" * 64, "planning/gcp/gcp08_v67_runtime_preflight.json", "HernanEspinozaDev/espaciGo", "token"))
        self.assertFalse(v67_runtime_ready("7", "0" * 64, "planning/gcp/gcp08_v67_runtime_preflight.json", "HernanEspinozaDev/espaciGo", "token"))

    def test_runtime_gate_requires_exact_reviewed_evidence_hash(self):
        from verify_gcp08_runtime_preflight import TARGET, valid
        record = {
            "schema": 1,
            "status": "verified",
            "target": TARGET,
            "checks": {
                "v67_schema_verified": True,
                "runtime_login_and_grants_verified": True,
                "runtime_connectivity_probe_passed": True,
                "password_secret_enabled": True,
            },
            "password_secret": {"id": "espacigo-test-database-password-v67", "version": "7", "state": "ENABLED"},
            "review": {"reviewer": "HernanEspinozaDev", "pr": 999, "commit": "a" * 40, "reviewed_at": "2026-10-21T12:00:00Z"},
            "evidence_refs": ["https://github.com/HernanEspinozaDev/espaciGo/pull/999"] + [f"https://github.com/HernanEspinozaDev/espaciGo/actions/runs/{i}" for i in range(100, 103)],
        }
        digest = hashlib.sha256(json.dumps(record, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        self.assertTrue(valid(record, digest, "7"))
        self.assertFalse(valid(record, "b" * 64, "7"))
        for field, value in (("v67_schema_verified", False),
                             ("runtime_login_and_grants_verified", False),
                             ("runtime_connectivity_probe_passed", False),
                             ("password_secret_enabled", False)):
            changed = copy.deepcopy(record)
            changed["checks"][field] = value
            changed_digest = hashlib.sha256(json.dumps(changed, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
            self.assertFalse(valid(changed, changed_digest, "7"))
        changed = copy.deepcopy(record)
        changed["review"]["reviewer"] = "someone-else"
        digest = hashlib.sha256(json.dumps(changed, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
        self.assertFalse(valid(changed, digest, "7"))

    def test_runtime_gate_verifies_the_merged_review_on_the_evidence_commit(self):
        from verify_gcp08_runtime_preflight import TARGET
        record = {
            "schema": 1, "status": "verified", "target": TARGET,
            "checks": {"v67_schema_verified": True, "runtime_login_and_grants_verified": True,
                       "runtime_connectivity_probe_passed": True, "password_secret_enabled": True},
            "password_secret": {"id": "espacigo-test-database-password-v67", "version": "7", "state": "ENABLED"},
            "review": {"reviewer": "HernanEspinozaDev", "pr": 999, "commit": "a" * 40, "reviewed_at": "2026-10-21T12:00:00Z"},
            "evidence_refs": ["https://github.com/HernanEspinozaDev/espaciGo/pull/999"] + [f"https://github.com/HernanEspinozaDev/espaciGo/actions/runs/{i}" for i in range(100, 103)],
        }
        with tempfile.NamedTemporaryFile(mode="w", encoding="utf-8") as stream:
            json.dump(record, stream)
            stream.flush()
            digest = hashlib.sha256(json.dumps(record, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
            pull = {"merged": True, "base": {"ref": "main"}, "head": {"sha": "a" * 40}}
            approval = {"user": {"login": "HernanEspinozaDev"}, "state": "APPROVED", "commit_id": "a" * 40}
            run = {"status": "completed", "conclusion": "success", "head_sha": "a" * 40,
                   "repository": {"full_name": "HernanEspinozaDev/espaciGo"}}
            with patch("verify_gcp07_environment.request_json", side_effect=[pull, [approval], run, run, run]):
                self.assertTrue(v67_runtime_ready("7", digest, stream.name, "HernanEspinozaDev/espaciGo", "token"))
            with patch("verify_gcp07_environment.request_json", side_effect=[pull, [{**approval, "commit_id": "b" * 40}]]):
                self.assertFalse(v67_runtime_ready("7", digest, stream.name, "HernanEspinozaDev/espaciGo", "token"))
            failed_run = {**run, "conclusion": "failure"}
            with patch("verify_gcp07_environment.request_json", side_effect=[pull, [approval], failed_run]):
                self.assertFalse(v67_runtime_ready("7", digest, stream.name, "HernanEspinozaDev/espaciGo", "token"))

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
        prefix = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
        manual = {"schema": 1, "contains_migrations": False, "source_sha": "b" * 40,
                  "publish_only": True,
                  "images": {"api": prefix + "espacigo-api@sha256:" + "a" * 64,
                             "web": prefix + "espacigo-web@sha256:" + "b" * 64}}
        self.assertTrue(rollout_allowed(manual, "a" * 40, True, [], True))
        self.assertFalse(rollout_allowed({**manual, "contains_migrations": True}, "a" * 40, True, [], True))
        self.assertFalse(rollout_allowed({**manual, "images": {"api": manual["images"]["api"]}}, "a" * 40, True, [], True))
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

    def test_initial_restore_allows_exact_services_and_minimum_iam_creates(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        plan = {"applyable": True, "resource_changes": [
            initial_resource(address, api if component == "api" else web)
            for address, (component, _) in ALLOWED.items()
        ] + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()]}
        result = validate(plan, {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")
        self.assertEqual({row["action"] for row in result["changes"]}, {"create"})
        self.assertEqual({row["address"] for row in result["changes"]}, set(ALLOWED))
        self.assertEqual({row["address"] for row in result["iam_changes"]}, set(BOOTSTRAP_IAM))
        self.assertEqual(len(result["fingerprint"]), 64)
        with self.assertRaises(PolicyError):
            validate(plan, {"api": api, "web": web}, "c" * 40, "123", "update", "7")

    def test_initial_restore_accepts_exact_existing_private_iap_binding_as_noop(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = ([initial_resource(address, api if component == "api" else web)
                      for address, (component, _) in ALLOWED.items()]
                     + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()])
        evaluator_address = 'google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]'
        evaluator = next(row for row in resources if row["address"] == evaluator_address)
        evaluator["change"]["actions"] = ["no-op"]
        evaluator["change"]["before"] = copy.deepcopy(evaluator["change"]["after"])
        result = validate({"applyable": True, "resource_changes": resources},
                          {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")
        row = next(item for item in result["iam_changes"] if item["address"] == evaluator_address)
        self.assertEqual(row["action"], "no-op")
        evaluator["change"]["before"]["member"] = "user:other@example.test"
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": resources},
                     {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")

    def test_initial_restore_rejects_v67_target_drift_partial_create_and_extra_resources(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = ([initial_resource(address, api if component == "api" else web)
                      for address, (component, _) in ALLOWED.items()]
                     + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()])
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
        missing_iam = copy.deepcopy(resources[:-1])
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": missing_iam}, expected, "c" * 40, "123", "bootstrap", "7")

    def test_initial_restore_never_allows_stale_sql_state_actions(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = ([initial_resource(address, api if component == "api" else web)
                      for address, (component, _) in ALLOWED.items()]
                     + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()])
        resources.append({"address": "google_sql_database_instance.postgres[0]",
                          "change": {"actions": ["delete"], "before": {"name": "espacigo-testprod-pg"},
                                     "after": None}})
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": resources},
                     {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")

    def test_initial_restore_accepts_unchanged_private_services_access_only(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = ([initial_resource(address, api if component == "api" else web)
                      for address, (component, _) in ALLOWED.items()]
                     + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()])
        psa = [
            {"address": "google_compute_global_address.private_services[0]",
             "change": {"actions": ["no-op"], "before": {"name": "espacigo-testprod-private-services"},
                        "after": {"name": "espacigo-testprod-private-services"}}},
            {"address": "google_service_networking_connection.private_services[0]",
             "change": {"actions": ["no-op"], "before": {"service": "servicenetworking.googleapis.com"},
                        "after": {"service": "servicenetworking.googleapis.com"}}},
        ]
        result = validate({"applyable": True, "resource_changes": resources + psa},
                          {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")
        self.assertEqual(len(result["changes"]), len(ALLOWED))
        for item in psa:
            changed = copy.deepcopy(resources + psa)
            target = next(row for row in changed if row["address"] == item["address"])
            target["change"]["actions"] = ["update"]
            with self.assertRaises(PolicyError):
                validate({"applyable": True, "resource_changes": changed},
                         {"api": api, "web": web}, "c" * 40, "123", "bootstrap", "7")

    def test_initial_iam_rejects_conditions_and_unallowlisted_attributes(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = ([initial_resource(address, api if component == "api" else web)
                      for address, (component, _) in ALLOWED.items()]
                     + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()])
        expected = {"api": api, "web": web}
        for index, mutation in ((3, lambda after: after.update(condition={"title": "unexpected"})),
                                (4, lambda after: after.update(condition=[{"expression": "true"}])),
                                (3, lambda after: after.update(unexpected_attribute="extra")),
                                (4, lambda after: after.update(unexpected_attribute="extra"))):
            changed = copy.deepcopy(resources)
            mutation(changed[index]["change"]["after"])
            with self.assertRaises(PolicyError):
                validate({"applyable": True, "resource_changes": changed}, expected,
                         "c" * 40, "123", "bootstrap", "7")
        changed = copy.deepcopy(resources)
        changed[3]["change"]["after"]["after_unknown"] = {"condition": {"expression": True}}
        with self.assertRaises(PolicyError):
            validate({"applyable": True, "resource_changes": changed}, expected,
                     "c" * 40, "123", "bootstrap", "7")

    def test_initial_iam_requires_exact_binding_scope(self):
        api = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        web = IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64
        resources = ([initial_resource(address, api if component == "api" else web)
                      for address, (component, _) in ALLOWED.items()]
                     + [bootstrap_iam_resource(address, expected) for address, expected in BOOTSTRAP_IAM.items()])
        expected = {"api": api, "web": web}
        for index, key, value in ((3, "project", "wrong-project"), (3, "location", "wrong-region"),
                                  (3, "name", "other-service")):
            changed = copy.deepcopy(resources)
            changed[index]["change"]["after"][key] = value
            with self.assertRaises(PolicyError):
                validate({"applyable": True, "resource_changes": changed}, expected,
                         "c" * 40, "123", "bootstrap", "7")
        scope_mutations = (
            ("google_secret_manager_secret_iam_member.v67_api_secret_access[0]", "project", "wrong-project"),
            ("google_secret_manager_secret_iam_member.v67_api_secret_access[0]", "secret_id", "other-secret"),
            ('google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]',
             "cloud_run_service_name", "projects/elsewhere/iap_web/other"),
            ('google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]',
             "member", "user:other@example.test"),
        )
        for address, key, value in scope_mutations:
            changed = copy.deepcopy(resources)
            target = next(row for row in changed if row["address"] == address)
            target["change"]["after"][key] = value
            with self.assertRaises(PolicyError):
                validate({"applyable": True, "resource_changes": changed}, expected,
                         "c" * 40, "123", "bootstrap", "7")

    def test_initial_service_projection_rejects_out_of_scope_fields(self):
        image = IMAGE_PATH + "espacigo-api@sha256:" + "a" * 64
        pristine = initial_resource("google_cloud_run_v2_service.api[0]", image)["change"]
        self.assertTrue(initial_service_matches("google_cloud_run_v2_service.api[0]", pristine, "7"))
        computed_id = copy.deepcopy(pristine)
        computed_id["after_unknown"] = {"id": True}
        self.assertTrue(initial_service_matches("google_cloud_run_v2_service.api[0]", computed_id, "7"))
        mutations = []
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["service_account"] = "attacker@example.test"; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["containers"][0]["env"].append({"name": "EXTRA", "value": "x"}); mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["containers"][0]["env"][0]["value"] = "production"; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["volumes"][0]["secret"][0]["items"][0]["version"] = "latest"; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["vpc_access"][0]["egress"] = "ALL_TRAFFIC"; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["scaling"][0]["max_instance_count"] = 5; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["containers"][0]["command"] = ["sh"]; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"][0:0] = []; changed["after"]["template"][0]["containers"].append({"image": "sidecar"}); mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["containers"][0]["startup_probe"] = [{"http_get": [{"path": "/"}]}]; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["template"][0]["surprise"] = "new field"; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after"]["deletion_protection"] = False; mutations.append(changed)
        changed = copy.deepcopy(pristine); changed["after_unknown"] = {"template": [{"containers": [{"env": [{"value": True}]}]}]}; mutations.append(changed)
        for changed in mutations:
            self.assertFalse(initial_service_matches("google_cloud_run_v2_service.api[0]", changed, "7"))

        web_address = 'google_cloud_run_v2_service.web["public"]'
        web = initial_resource(web_address, IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64)["change"]
        web["after"]["template"][0]["containers"][0]["env"][2]["value"] = None
        web["after"]["template"][0]["containers"][0]["env"][4]["value"] = None
        web["after_unknown"] = {"template": [{"containers": [{"env": [{}, {}, {"value": True}, {}, {"value": True}]}]}]}
        self.assertTrue(initial_service_matches(web_address, web, "7"))
        web["after_unknown"]["template"][0]["containers"][0]["env"][1] = {"value": True}
        self.assertFalse(initial_service_matches(web_address, web, "7"))

    def test_web_api_url_and_audience_must_reference_the_new_api_service(self):
        web_address = 'google_cloud_run_v2_service.web["public"]'
        web = initial_resource(web_address, IMAGE_PATH + "espacigo-web@sha256:" + "b" * 64)["change"]
        self.assertTrue(web_api_references_are_exact())
        for name, external in (("API_INTERNAL_BASE_URL", "https://attacker.example/api/v1"),
                               ("API_SERVICE_AUDIENCE", "https://attacker.example")):
            changed = copy.deepcopy(web)
            env = changed["after"]["template"][0]["containers"][0]["env"]
            next(item for item in env if item["name"] == name)["value"] = external
            changed.pop("after_unknown", None)
            self.assertFalse(initial_service_matches(web_address, changed, "7"))

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
