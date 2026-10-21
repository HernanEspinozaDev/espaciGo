import os
import stat
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from io import StringIO
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock

from gcp08_bootstrap_cloud_run_probes import (
    PROBES, SAFE_CATEGORIES, build_command, classify_probe, run_probe,
)


ROOT = Path(__file__).resolve().parents[1]


class BootstrapCloudRunProbeTests(unittest.TestCase):
    def test_fixed_allowlist_is_exactly_six_get_only_probes_for_three_services(self):
        self.assertEqual(len(PROBES), 6)
        self.assertEqual(len({target for target, _, _ in PROBES}), 6)
        self.assertEqual({operation for _, operation, _ in PROBES}, {"describe", "get-iam-policy"})
        self.assertEqual({service for _, _, service in PROBES}, {
            "espacigo-test-api", "espacigo-test-public", "espacigo-test-private",
        })
        for target, operation, service in PROBES:
            command = build_command(target)
            self.assertEqual(command[:4], ["gcloud", "run", "services", operation])
            self.assertEqual(command[4], service)
            self.assertIn("--project=espacigo-testprod", command)
            self.assertIn("--region=southamerica-west1", command)
            self.assertIn("--format=none", command)
            self.assertNotIn("list", command)
            self.assertNotIn("set-iam-policy", command)
            self.assertNotIn("--log-http", command)

    def test_permission_denial_publishes_only_allowlisted_details(self):
        target = "cloud_run_service.api.describe"
        marker = "private-resource-name-and-config-value"
        diagnostic = (
            f"ERROR: HTTP status code 403 (gcloud.run.services.describe) PERMISSION_DENIED: "
            f"Permission 'run.services.get' "
            f"denied on {marker}"
        )
        observed = {}

        def fake_runner(command, **kwargs):
            observed["command"] = command
            observed["env"] = kwargs["env"]
            log = kwargs["stdout"]
            observed["log_mode"] = stat.S_IMODE(os.fstat(log.fileno()).st_mode)
            log.write(diagnostic)
            return SimpleNamespace(returncode=1)

        with tempfile.TemporaryDirectory() as root:
            output = StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(target, root, runner=fake_runner))
            self.assertEqual(list(Path(root).iterdir()), [])

        emitted = output.getvalue().strip()
        self.assertEqual(
            emitted,
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_service.api.describe "
            "category=permission_denied status=403 permission=run.services.get service=run.googleapis.com",
        )
        self.assertNotIn(marker, emitted)
        self.assertNotIn("PERMISSION_DENIED", emitted)
        self.assertNotIn("resource_type=", emitted)
        self.assertNotIn("operation=", emitted)
        self.assertEqual(observed["log_mode"], 0o600)
        self.assertTrue(observed["env"]["CLOUDSDK_CONFIG"].startswith(root))
        self.assertNotIn("--log-http", observed["command"])

    def test_not_found_is_distinct_and_does_not_expose_resource_text(self):
        marker = "hidden-service-name"
        diagnostic = f"ERROR: NOT_FOUND: service {marker} does not exist (HTTP 404)"
        self.assertEqual(classify_probe(diagnostic), "not_found")
        with tempfile.TemporaryDirectory() as root:
            output = StringIO()

            def fake_runner(_command, **kwargs):
                kwargs["stdout"].write(diagnostic)
                return SimpleNamespace(returncode=1)

            with redirect_stdout(output):
                self.assertFalse(run_probe("cloud_run_service.private.get_iam_policy", root, fake_runner))
        self.assertIn("category=not_found status=404", output.getvalue())
        self.assertNotIn(marker, output.getvalue())

    def test_structured_error_fields_are_extracted_and_categories_are_closed(self):
        diagnostic = (
            '{"error":{"code":403,"status":"PERMISSION_DENIED",'
            '"details":[{"permission":"run.services.get","service":"run.googleapis.com"}]}}'
        )
        self.assertEqual(SAFE_CATEGORIES, {
            "success", "not_found", "permission_denied", "auth_error", "other",
        })
        with tempfile.TemporaryDirectory() as root:
            output = StringIO()

            def fake_runner(_command, **kwargs):
                kwargs["stdout"].write(diagnostic)
                return SimpleNamespace(returncode=1)

            with redirect_stdout(output):
                self.assertFalse(run_probe("cloud_run_service.public.describe", root, fake_runner))
        self.assertEqual(
            output.getvalue().strip(),
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_service.public.describe "
            "category=permission_denied status=403 permission=run.services.get service=run.googleapis.com",
        )

    def test_auth_error_and_unknown_errors_use_fixed_categories_only(self):
        self.assertEqual(classify_probe("problem refreshing auth tokens: invalid_grant"), "auth_error")
        self.assertEqual(classify_probe("unexpected private error value"), "other")

    def test_success_emits_only_probe_enum_and_success(self):
        with tempfile.TemporaryDirectory() as root:
            output = StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "cloud_run_service.public.get_iam_policy", root,
                    runner=Mock(return_value=SimpleNamespace(returncode=0)),
                ))
        self.assertEqual(
            output.getvalue().strip(),
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap "
            "target=cloud_run_service.public.get_iam_policy category=success",
        )

    def test_unknown_enum_fails_before_spawning_a_command(self):
        runner = Mock()
        with self.assertRaises(ValueError):
            run_probe("cloud_run_service.private.list", "/nonexistent", runner=runner)
        runner.assert_not_called()

    def test_timeout_classifies_partial_private_diagnostic_and_cleans_temp_files(self):
        def timeout(*_args, **_kwargs):
            _kwargs["stdout"].write("gcloud: problem refreshing auth tokens: invalid_grant secret-marker")
            raise subprocess.TimeoutExpired("gcloud", 120)

        with tempfile.TemporaryDirectory() as root:
            output = StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe("cloud_run_service.api.get_iam_policy", root, timeout))
            self.assertEqual(list(Path(root).iterdir()), [])
        self.assertIn("category=auth_error", output.getvalue())
        self.assertNotIn("secret-marker", output.getvalue())

    def test_workflow_installs_sdk_after_bootstrap_auth_and_runs_only_on_plan_failure(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        auth = workflow.index("- name: Authenticate bootstrap federated identity")
        setup = workflow.index("- name: Set up Cloud SDK for read-only bootstrap diagnostics")
        rollout = workflow.index("- name: Initialize canonical Terraform root and produce private plan")
        self.assertLess(auth, setup)
        self.assertLess(setup, rollout)
        start = workflow.index("python3 \"$GITHUB_WORKSPACE/scripts/gcp08_bootstrap_cloud_run_probes.py\"")
        block_start = workflow.rfind("if [[ \"$POLICY_MODE\" == bootstrap ]]", 0, start)
        self.assertGreater(block_start, workflow.index("terraform plan -input=false"))
        self.assertIn("inputs.operation == 'initial-restore'", workflow[setup:rollout])
        self.assertNotRegex(workflow[start:start + 300], r"gcloud\s+run\s+services\s+list|--log-http")


if __name__ == "__main__":
    unittest.main()
