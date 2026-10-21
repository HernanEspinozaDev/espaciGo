import io
import json
import os
import stat
import subprocess
import tempfile
import unittest
import urllib.error
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock

from gcp08_bootstrap_cloud_run_probes import (
    PROBES, SAFE_CATEGORIES, _write_private_response, build_gcloud_command,
    build_rest_url, classify_rest_response, run_probe,
)


ROOT = Path(__file__).resolve().parents[1]


class BootstrapCloudRunProbeTests(unittest.TestCase):
    def test_fixed_allowlist_uses_rest_describe_and_read_only_iam_commands(self):
        self.assertEqual(len(PROBES), 6)
        self.assertEqual(len({target for target, _, _ in PROBES}), 6)
        self.assertEqual({operation for _, operation, _ in PROBES}, {"describe", "get-iam-policy"})
        self.assertEqual({service for _, _, service in PROBES}, {
            "espacigo-test-api", "espacigo-test-public", "espacigo-test-private",
        })
        for target, operation, service in PROBES:
            if operation == "describe":
                self.assertEqual(
                    build_rest_url(target),
                    f"https://run.googleapis.com/v2/projects/espacigo-testprod/locations/"
                    f"southamerica-west1/services/{service}",
                )
            else:
                command = build_gcloud_command(target)
                self.assertEqual(command[:4], ["gcloud", "run", "services", "get-iam-policy"])
                self.assertEqual(command[4], service)
                self.assertIn("--project=espacigo-testprod", command)
                self.assertIn("--region=southamerica-west1", command)
                self.assertIn("--format=none", command)
                self.assertNotIn("list", command)
                self.assertNotIn("set-iam-policy", command)
                self.assertNotIn("--log-http", command)

    def test_rest_403_errorinfo_emits_only_allowlisted_status_permission_and_service(self):
        token = "private-short-lived-token-value"
        marker = "private-resource-and-error-message"
        payload = {
            "error": {
                "code": 403,
                "status": "PERMISSION_DENIED",
                "message": marker,
                "details": [{
                    "reason": "IAM_PERMISSION_DENIED",
                    "domain": "run.googleapis.com",
                    "metadata": {
                        "permission": "run.services.get",
                        "resource": "projects/private-project/locations/private-region/services/private-service",
                    },
                }],
            },
        }
        observed = {}

        def token_runner(command, **kwargs):
            observed["token_command"] = command
            observed["token_env"] = kwargs["env"]
            return SimpleNamespace(returncode=0, stdout=token, stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["authorization"] = request.get_header("Authorization")
                observed["timeout"] = timeout
                body = json.dumps(payload).encode()
                return urllib.error.HTTPError(
                    request.full_url, 403, "Forbidden", {}, io.BytesIO(body),
                )

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(
                    "cloud_run_service.api.describe", root, runner=token_runner,
                    opener_factory=lambda *_handlers: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        emitted = output.getvalue().strip()
        self.assertEqual(
            emitted,
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_service.api.describe "
            "category=permission_denied status=403 permission=run.services.get service=run.googleapis.com",
        )
        self.assertNotIn(token, emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("private-project", emitted)
        self.assertEqual(observed["authorization"], f"Bearer {token}")
        self.assertEqual(observed["method"], "GET")
        self.assertEqual(observed["token_command"], ["gcloud", "auth", "print-access-token"])
        self.assertTrue(observed["token_env"]["CLOUDSDK_CONFIG"].startswith(root))

    def test_rest_404_error_json_emits_status_but_not_error_body(self):
        marker = "private-service-name-and-message"
        payload = {"error": {"code": 404, "status": "NOT_FOUND", "message": marker}}

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                return urllib.error.HTTPError(
                    request.full_url, 404, "Not Found", {},
                    io.BytesIO(json.dumps(payload).encode()),
                )

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(
                    "cloud_run_service.private.describe", root, runner=token_runner,
                    opener_factory=lambda *_handlers: FakeOpener(),
                ))
        self.assertEqual(
            output.getvalue().strip(),
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_service.private.describe "
            "category=not_found status=404",
        )
        self.assertNotIn(marker, output.getvalue())
        self.assertNotIn("token-hidden", output.getvalue())

    def test_success_response_body_is_private_and_never_echoed(self):
        marker = "private-service-config-body"
        payload = {"name": marker, "uri": "https://private.example"}
        response = None

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            status = 200
            read_called = False

            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return self.status

            def read(self, _limit):
                self.read_called = True
                return json.dumps(payload).encode()

        class FakeOpener:
            def open(self, _request, timeout):
                nonlocal response
                response = FakeResponse()
                return response

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "cloud_run_service.public.describe", root, runner=token_runner,
                    opener_factory=lambda *_handlers: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])
        self.assertEqual(
            output.getvalue().strip(),
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_service.public.describe category=success",
        )
        self.assertNotIn(marker, output.getvalue())
        self.assertFalse(response.read_called)

    def test_private_response_file_is_mode_0600(self):
        with tempfile.TemporaryDirectory() as root:
            path = Path(root) / "response.json"
            _write_private_response(path, b'{"error":{"code":403}}')
            self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
            self.assertEqual(json.loads(path.read_text()), {"error": {"code": 403}})

    def test_rest_categories_are_closed_and_auth_failure_never_exposes_token_error(self):
        self.assertEqual(SAFE_CATEGORIES, {
            "success", "not_found", "permission_denied", "auth_error", "other",
        })
        self.assertEqual(classify_rest_response(401, {"error": {"code": 401, "status": "UNAUTHENTICATED"}}), "auth_error")
        self.assertEqual(classify_rest_response(500, {"error": {"code": 500}}), "other")
        marker = "private-auth-error"

        def failed_token(_command, **_kwargs):
            return SimpleNamespace(returncode=1, stdout="", stderr=marker)

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe("cloud_run_service.api.describe", root, runner=failed_token))
            self.assertEqual(list(Path(root).iterdir()), [])
        self.assertIn("category=auth_error", output.getvalue())
        self.assertNotIn(marker, output.getvalue())

    def test_unknown_enum_fails_before_spawning_a_command(self):
        runner = Mock()
        with self.assertRaises(ValueError):
            run_probe("cloud_run_service.private.list", "/nonexistent", runner=runner)
        runner.assert_not_called()

    def test_iam_policy_gcloud_success_emits_only_enum_and_success(self):
        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
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

    def test_iam_policy_timeout_classifies_partial_private_diagnostic(self):
        def timeout(_command, **kwargs):
            kwargs["stdout"].write("gcloud: problem refreshing auth tokens: invalid_grant secret-marker")
            raise subprocess.TimeoutExpired("gcloud", 120)

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
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
