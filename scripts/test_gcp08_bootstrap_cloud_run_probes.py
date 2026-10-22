import io
import json
import os
import stat
import subprocess
import tempfile
import unittest
import urllib.error
import urllib.parse
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock, patch

from gcp08_bootstrap_cloud_run_probes import (
    PROBES, SAFE_CATEGORIES, _write_private_response, build_gcloud_command,
    build_rest_url, classify_rest_response, probe_configuration_matches,
    read_probe_configuration, run_probe, _sqladmin_rest_url,
    safe_auth_context, safe_provider_credentials_context, safe_rest_fields,
)
import gcp08_bootstrap_cloud_run_probes as cloud_run_probes


ROOT = Path(__file__).resolve().parents[1]


class BootstrapCloudRunProbeTests(unittest.TestCase):
    def test_provider_credential_source_uses_documented_override_precedence_safely(self):
        marker = "credential-secret-marker"
        context = safe_provider_credentials_context({
            "GOOGLE_APPLICATION_CREDENTIALS": "/private/wif.json",
            "GOOGLE_CREDENTIALS": marker,
            "GOOGLE_CLOUD_KEYFILE_JSON": "second-secret-marker",
            "GCLOUD_KEYFILE_JSON": "third-secret-marker",
        })
        self.assertEqual(context["provider_source"], "GOOGLE_CREDENTIALS")
        self.assertEqual(context["override_conflict"], "yes")
        self.assertEqual(context["google_credentials"], "present")
        self.assertEqual(context["google_cloud_keyfile_json"], "present")
        self.assertEqual(context["gcloud_keyfile_json"], "present")
        rendered = " ".join(context.values())
        for secret in (marker, "second-secret-marker", "third-secret-marker", "/private/wif.json"):
            self.assertNotIn(secret, rendered)

    def test_provider_credential_source_allows_adc_and_matching_override(self):
        adc = "/private/wif.json"
        context = safe_provider_credentials_context({"GOOGLE_APPLICATION_CREDENTIALS": adc})
        self.assertEqual(context["provider_source"], "GOOGLE_APPLICATION_CREDENTIALS")
        self.assertEqual(context["override_conflict"], "no")

        same_file = safe_provider_credentials_context({
            "GOOGLE_APPLICATION_CREDENTIALS": adc,
            "GCLOUD_KEYFILE_JSON": adc,
        })
        self.assertEqual(same_file["provider_source"], "GCLOUD_KEYFILE_JSON")
        self.assertEqual(same_file["override_matches_adc"], "yes")
        self.assertEqual(same_file["override_conflict"], "no")

    def test_provider_credential_check_fails_closed_without_leaking_override(self):
        marker = "private-credential-value-marker"
        output = io.StringIO()
        with patch.dict(os.environ, {
            "GOOGLE_APPLICATION_CREDENTIALS": "/private/wif.json",
            "GOOGLE_CREDENTIALS": marker,
        }, clear=True), patch("sys.argv", ["probe", "--check-provider-credentials"]), redirect_stdout(output):
            result = cloud_run_probes.main()
        self.assertEqual(result, 2)
        self.assertIn("provider_source=GOOGLE_CREDENTIALS", output.getvalue())
        self.assertIn("override_conflict=yes", output.getvalue())
        self.assertNotIn(marker, output.getvalue())
        self.assertNotIn("/private/wif.json", output.getvalue())

    def test_auth_context_reports_only_shared_credential_file_and_impersonation_flags(self):
        private_path = "/runner/private/wif-credentials.json"
        private_identity = "service-account@example.invalid"
        context = safe_auth_context({
            "GOOGLE_APPLICATION_CREDENTIALS": private_path,
            "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE": private_path,
            "GOOGLE_IMPERSONATE_SERVICE_ACCOUNT": "",
            "CLOUDSDK_AUTH_IMPERSONATE_SERVICE_ACCOUNT": private_identity,
        })
        self.assertEqual(context, {
            "terraform_adc": "present",
            "gcloud_credential_override": "present",
            "shared_credential_file": "yes",
            "terraform_impersonation_env": "absent",
            "gcloud_impersonation_env": "present",
        })
        self.assertNotIn(private_path, " ".join(context.values()))
        self.assertNotIn(private_identity, " ".join(context.values()))

        mismatched = safe_auth_context({
            "GOOGLE_APPLICATION_CREDENTIALS": "/runner/private/terraform.json",
            "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE": "/runner/private/gcloud.json",
        })
        self.assertEqual(mismatched["shared_credential_file"], "no")

    def test_dependency_permission_details_are_allowlisted_without_resource_values(self):
        marker = "private-resource-name"
        for permission, domain in (
            ("compute.networks.get", "compute.googleapis.com"),
            ("compute.subnetworks.get", "compute.googleapis.com"),
            ("serviceusage.services.get", "serviceusage.googleapis.com"),
            ("iam.serviceAccounts.getIamPolicy", "iam.googleapis.com"),
            ("secretmanager.secrets.get", "secretmanager.googleapis.com"),
            ("secretmanager.secrets.getIamPolicy", "secretmanager.googleapis.com"),
            ("secretmanager.secrets.setIamPolicy", "secretmanager.googleapis.com"),
        ):
            with self.subTest(permission=permission):
                payload = {"error": {"code": 403, "details": [{
                    "domain": domain,
                    "metadata": {"permission": permission, "resource": marker},
                }]}}
                self.assertEqual(safe_rest_fields(403, payload), [
                    "status=403", f"permission={permission}", f"service={domain}",
                ])

    def test_new_dependency_reads_dispatch_through_fixed_rest_gets(self):
        expected = {
            "compute_network.testprod.get": (
                "https://compute.googleapis.com/compute/v1/projects/espacigo-testprod/global/networks/espacigo-testprod"
            ),
            "compute_subnetwork.serverless.get": (
                "https://compute.googleapis.com/compute/v1/projects/espacigo-testprod/regions/southamerica-west1/subnetworks/espacigo-testprod-serverless"
            ),
            "compute_subnetwork.api_serverless.get": (
                "https://compute.googleapis.com/compute/v1/projects/espacigo-testprod/regions/southamerica-west1/subnetworks/espacigo-testprod-api-serverless"
            ),
            "serviceusage.run_api.get": (
                "https://serviceusage.googleapis.com/v1/projects/espacigo-testprod/services/run.googleapis.com"
            ),
        }

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

        for target, url in expected.items():
            with self.subTest(target=target):
                observed = {}

                def token_runner(command, **_kwargs):
                    observed["token_command"] = command
                    return SimpleNamespace(returncode=0, stdout="private-token", stderr="")

                class FakeOpener:
                    def open(self, request, timeout):
                        observed["url"] = request.full_url
                        observed["method"] = request.get_method()
                        return FakeResponse()

                with tempfile.TemporaryDirectory() as root:
                    output = io.StringIO()
                    with redirect_stdout(output):
                        self.assertTrue(run_probe(
                            target, root, runner=token_runner,
                            opener_factory=lambda *_: FakeOpener(),
                        ))
                self.assertEqual(observed["token_command"], ["gcloud", "auth", "print-access-token"])
                self.assertEqual(observed["url"], url)
                self.assertEqual(observed["method"], "GET")
                self.assertNotIn("private-token", output.getvalue())
                self.assertIn(f"target={target} category=success", output.getvalue())

    def test_v67_secret_metadata_and_iam_policy_probes_are_exact_read_only_gets(self):
        expected = {
            "secret.v67.metadata.get": (
                "https://secretmanager.googleapis.com/v1/projects/espacigo-testprod/secrets/"
                "espacigo-test-database-password-v67"
            ),
            "secret.v67.iam_policy.get": (
                "https://secretmanager.googleapis.com/v1/projects/espacigo-testprod/secrets/"
                "espacigo-test-database-password-v67:getIamPolicy?options.requestedPolicyVersion=3"
            ),
        }

        class FakeResponse:
            read_called = False

            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                self.read_called = True
                return b'{"secretData":"must-not-be-read"}'

        for target, url in expected.items():
            with self.subTest(target=target):
                observed = {}
                response = FakeResponse()

                def token_runner(command, **_kwargs):
                    observed["token_command"] = command
                    return SimpleNamespace(returncode=0, stdout="private-token", stderr="")

                class FakeOpener:
                    def open(self, request, timeout):
                        observed["url"] = request.full_url
                        observed["method"] = request.get_method()
                        return response

                with tempfile.TemporaryDirectory() as root:
                    output = io.StringIO()
                    with redirect_stdout(output):
                        self.assertTrue(run_probe(
                            target, root, runner=token_runner,
                            opener_factory=lambda *_: FakeOpener(),
                        ))

                self.assertEqual(observed["url"], url)
                self.assertEqual(observed["method"], "GET")
                self.assertFalse(response.read_called)
                self.assertNotIn("secretData", output.getvalue())
                self.assertNotIn("private-token", output.getvalue())
                self.assertIn("category=success", output.getvalue())

    def test_v67_secret_test_iam_checks_only_three_permissions_on_the_exact_secret(self):
        marker = "unapproved-secret-permission"
        observed = {}

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return json.dumps({"permissions": [
                    "secretmanager.secrets.get",
                    "secretmanager.secrets.getIamPolicy",
                    marker,
                ]}).encode()

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["body"] = json.loads(request.data)
                observed["authorization"] = request.get_header("Authorization")
                return FakeResponse()

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "secret.v67.test_iam_permissions", root, runner=token_runner,
                    opener_factory=lambda *_: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        self.assertEqual(observed["url"],
                         "https://secretmanager.googleapis.com/v1/projects/espacigo-testprod/secrets/"
                         "espacigo-test-database-password-v67:testIamPermissions")
        self.assertEqual(observed["method"], "POST")
        self.assertEqual(set(observed["body"]["permissions"]), {
            "secretmanager.secrets.get", "secretmanager.secrets.getIamPolicy",
            "secretmanager.secrets.setIamPolicy",
        })
        self.assertEqual(observed["authorization"], "Bearer token-hidden")
        emitted = output.getvalue()
        self.assertIn("resource=secret category=success", emitted)
        self.assertIn("permission=secretmanager.secrets.get result=granted", emitted)
        self.assertIn("permission=secretmanager.secrets.getIamPolicy result=granted", emitted)
        self.assertIn("permission=secretmanager.secrets.setIamPolicy result=not_granted", emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("token-hidden", emitted)

    def test_v67_secret_403_keeps_only_exact_permission_and_service(self):
        for target, permission in (
            ("secret.v67.metadata.get", "secretmanager.secrets.get"),
            ("secret.v67.iam_policy.get", "secretmanager.secrets.getIamPolicy"),
        ):
            with self.subTest(target=target):
                marker = "private-secret-resource-and-message"
                payload = {"error": {"code": 403, "status": "PERMISSION_DENIED", "details": [{
                    "reason": "IAM_PERMISSION_DENIED",
                    "domain": "secretmanager.googleapis.com",
                    "metadata": {"permission": permission, "resource": marker},
                }]}}

                def token_runner(_command, **_kwargs):
                    return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

                class FakeOpener:
                    def open(self, request, timeout):
                        return urllib.error.HTTPError(
                            request.full_url, 403, "Forbidden", {},
                            io.BytesIO(json.dumps(payload).encode()),
                        )

                with tempfile.TemporaryDirectory() as root:
                    output = io.StringIO()
                    with redirect_stdout(output):
                        self.assertFalse(run_probe(
                            target, root, runner=token_runner,
                            opener_factory=lambda *_: FakeOpener(),
                        ))

                emitted = output.getvalue()
                self.assertIn(f"permission={permission}", emitted)
                self.assertIn("service=secretmanager.googleapis.com", emitted)
                self.assertIn("category=permission_denied status=403", emitted)
                self.assertNotIn(marker, emitted)
                self.assertNotIn("token-hidden", emitted)

    def test_fixed_allowlist_uses_rest_describe_and_read_only_iam_commands(self):
        self.assertEqual(len(PROBES), 42)
        self.assertEqual(len({target for target, _, _ in PROBES}), 42)
        self.assertEqual({operation for _, operation, _ in PROBES}, {
            "describe", "get-iam-policy", "list", "iap-get-iam-policy", "iap-test-iam",
            "project-get", "project-test-iam",
            "sql-instance-get", "run-test-iam", "run-parent-test-iam", "sa-test-iam",
            "compute-network-get", "compute-subnetwork-get", "serviceusage-service-get",
            "secretmanager-secret-get", "secretmanager-secret-get-iam-policy",
            "secretmanager-secret-test-iam", "sa-metadata-get", "sa-get-iam-policy",
            "sa-test-get-iam-policy", "sa-test-get-metadata", "run-get-iam-policy-v2",
        })
        self.assertEqual({service for _, operation, service in PROBES
                          if service is not None and operation not in {
                              "iap-get-iam-policy", "iap-test-iam", "project-get",
                              "project-test-iam", "sql-instance-get", "run-test-iam",
                              "run-parent-test-iam", "sa-test-iam",
                              "compute-network-get", "compute-subnetwork-get", "serviceusage-service-get",
                              "secretmanager-secret-get", "secretmanager-secret-get-iam-policy",
                              "secretmanager-secret-test-iam", "sa-metadata-get",
                              "sa-get-iam-policy", "sa-test-get-iam-policy", "sa-test-get-metadata",
                              "run-get-iam-policy-v2",
                          }}, {
            "espacigo-test-api", "espacigo-test-public", "espacigo-test-private",
        })
        for target, operation, service in PROBES:
            if operation == "describe":
                self.assertEqual(
                    build_rest_url(target),
                    f"https://run.googleapis.com/v2/projects/espacigo-testprod/locations/"
                    f"southamerica-west1/services/{service}",
                )
            elif operation == "list":
                self.assertEqual(
                    build_rest_url(target),
                    "https://run.googleapis.com/v2/projects/espacigo-testprod/locations/"
                    "southamerica-west1/services",
                )
                with self.assertRaises(ValueError):
                    build_gcloud_command(target)
            elif operation == "get-iam-policy":
                command = build_gcloud_command(target)
                self.assertEqual(command[:4], ["gcloud", "run", "services", "get-iam-policy"])
                self.assertEqual(command[4], service)
                self.assertIn("--project=espacigo-testprod", command)
                self.assertIn("--region=southamerica-west1", command)
                self.assertIn("--format=none", command)
                self.assertNotIn("list", command)
                self.assertNotIn("set-iam-policy", command)
                self.assertNotIn("--log-http", command)
            elif operation == "run-get-iam-policy-v2":
                self.assertEqual(
                    build_rest_url(target),
                    f"https://run.googleapis.com/v2/projects/espacigo-testprod/locations/"
                    f"southamerica-west1/services/espacigo-test-{service}:getIamPolicy"
                    "?options.requestedPolicyVersion=3",
                )
            elif operation in {
                "iap-get-iam-policy", "iap-test-iam", "project-get", "project-test-iam",
                "sql-instance-get", "run-test-iam",
                "run-parent-test-iam", "sa-test-iam",
                "sa-metadata-get", "sa-get-iam-policy", "sa-test-get-iam-policy",
                "sa-test-get-metadata",
            }:
                self.assertIn(target, {
                    "cloud_run_service.private.iap_get_iam_policy",
                    "cloud_run_service.private.iap_test_iam_permissions",
                    "project.metadata.get", "project.test_iam_permissions",
                    "cloud_sql_instance.v67.get",
                    "cloud_run_service.api.test_iam_permissions",
                    "cloud_run_service.public.test_iam_permissions",
                    "cloud_run_service.private.test_iam_permissions",
                    "cloud_run_create_parent.project.test_iam_permissions",
                    "runtime_service_account.api.test_act_as",
                    "runtime_service_account.public.test_act_as",
                    "runtime_service_account.private.test_act_as",
                "secret.v67.metadata.get", "secret.v67.iam_policy.get",
                "secret.v67.test_iam_permissions",
                "service_account.deployer.metadata.get", "service_account.deployer.iam_policy.get",
                "service_account.deployer.test_get_iam_policy",
                "service_account.deployer.test_get_metadata",
                "service_account.api.metadata.get", "service_account.api.iam_policy.get",
                "service_account.api.test_get_iam_policy",
                "service_account.public.metadata.get", "service_account.public.iam_policy.get",
                "service_account.public.test_get_iam_policy",
                "service_account.private.metadata.get", "service_account.private.iam_policy.get",
                "service_account.private.test_get_iam_policy",
                })
            elif operation == "compute-network-get":
                self.assertEqual(build_rest_url(target),
                                 "https://compute.googleapis.com/compute/v1/projects/espacigo-testprod/global/networks/espacigo-testprod")
            elif operation == "compute-subnetwork-get":
                self.assertIn("/regions/southamerica-west1/subnetworks/espacigo-testprod-", build_rest_url(target))
            elif operation == "serviceusage-service-get":
                self.assertEqual(build_rest_url(target),
                                 "https://serviceusage.googleapis.com/v1/projects/espacigo-testprod/services/run.googleapis.com")
            elif operation == "secretmanager-secret-get":
                self.assertEqual(build_rest_url(target),
                                 "https://secretmanager.googleapis.com/v1/projects/espacigo-testprod/secrets/espacigo-test-database-password-v67")
            elif operation == "secretmanager-secret-get-iam-policy":
                self.assertEqual(build_rest_url(target),
                                 "https://secretmanager.googleapis.com/v1/projects/espacigo-testprod/secrets/espacigo-test-database-password-v67:getIamPolicy?options.requestedPolicyVersion=3")
            elif operation == "secretmanager-secret-test-iam":
                self.assertEqual(build_rest_url(target),
                                 "https://secretmanager.googleapis.com/v1/projects/espacigo-testprod/secrets/espacigo-test-database-password-v67:testIamPermissions")

    def test_cloudsql_probe_urls_use_only_validated_v67_identifiers(self):
        self.assertEqual(
            _sqladmin_rest_url("sql-instance-get", "espacigo-testprod-db"),
            "https://sqladmin.googleapis.com/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-testprod-db",
        )
        for invalid in ("../instance", "instance/name", "private project"):
            with self.assertRaises(ValueError):
                _sqladmin_rest_url("sql-instance-get", invalid)

    def test_probe_configuration_uses_pinned_iap_service_default_and_reads_v67_identifiers(self):
        with tempfile.TemporaryDirectory() as root:
            vars_path = Path(root) / "vars.json"
            vars_path.write_text(json.dumps({
                "project_id": "espacigo-testprod",
                "project_number": 447373903153,
                "api_v67_database_instance_name": "espacigo-testprod-db",
            }))
            config = read_probe_configuration(vars_path)
        self.assertEqual(config, {
            "project_id": "espacigo-testprod",
            "project_number": "447373903153",
            "region": "southamerica-west1",
            "api_service_name": "espacigo-test-api",
            "public_service_name": "espacigo-test-public",
            "private_service_name": "espacigo-test-private",
            "sql_instance_name": "espacigo-testprod-db",
        })

    def test_protected_probe_target_matches_emit_only_aliases_and_booleans(self):
        config = {
            "project_id": "espacigo-testprod", "project_number": "447373903153",
            "region": "southamerica-west1", "api_service_name": "espacigo-test-api",
            "public_service_name": "espacigo-test-public", "private_service_name": "espacigo-test-private",
        }
        matches = probe_configuration_matches(config)
        self.assertTrue(all(matches.values()))
        output = io.StringIO()
        with redirect_stdout(output):
            self.assertTrue(cloud_run_probes._emit_configuration_matches(config))
        self.assertEqual(output.getvalue().splitlines(), [
            f"GCP08_PROBE_CONFIG alias={alias} result=match" for alias in matches
        ])
        for sensitive_value in ("espacigo-testprod", "447373903153", "southamerica-west1"):
            self.assertNotIn(sensitive_value, output.getvalue())

    def test_mismatched_protected_target_stops_before_any_cloud_probe(self):
        marker = "private-project-name"
        with tempfile.TemporaryDirectory() as root:
            vars_path = Path(root) / "vars.json"
            vars_path.write_text(json.dumps({"project_id": marker, "project_number": 447373903153}))
            output = io.StringIO()
            with redirect_stdout(output), patch.object(
                cloud_run_probes.sys, "argv",
                ["gcp08_bootstrap_cloud_run_probes.py", "--validate-configuration", "--var-file", str(vars_path)],
            ), patch.object(cloud_run_probes, "run_probe") as probe:
                self.assertEqual(cloud_run_probes.main(), 2)
                probe.assert_not_called()
        self.assertIn("alias=project_id result=mismatch", output.getvalue())
        self.assertNotIn(marker, output.getvalue())
        self.assertNotIn("status=", output.getvalue())

    def test_main_passes_only_probe_supported_configuration_to_run_probe(self):
        protected_config = {
            "project_id": "espacigo-testprod",
            "project_number": 447373903153,
            "region": "southamerica-west1",
            "api_service_name": "espacigo-test-api",
            "public_service_name": "espacigo-test-public",
            "private_service_name": "espacigo-test-private",
            "api_v67_database_instance_name": "espacigo-testprod-db",
        }
        with tempfile.TemporaryDirectory() as root:
            vars_path = Path(root) / "vars.json"
            vars_path.write_text(json.dumps(protected_config), encoding="utf-8")
            calls = []

            def strict_run_probe(target, private_root, project_number=None,
                                 private_service_name=None, sql_instance_name=None):
                calls.append((target, Path(private_root), project_number,
                              private_service_name, sql_instance_name))
                return True

            argv = [
                "gcp08_bootstrap_cloud_run_probes.py", "--private-dir", root,
                "--var-file", str(vars_path),
            ]
            with redirect_stdout(io.StringIO()), patch.object(
                cloud_run_probes.sys, "argv", argv
            ), patch.object(
                cloud_run_probes, "PROBES",
                (("cloud_run_service.api.get_iam_policy_v2", "run-get-iam-policy-v2", "api"),),
            ), patch.object(
                cloud_run_probes, "safe_auth_context", return_value={}
            ), patch.object(
                cloud_run_probes, "run_probe", side_effect=strict_run_probe
            ):
                self.assertEqual(cloud_run_probes.main(), 0)

        self.assertEqual(calls, [(
            "cloud_run_service.api.get_iam_policy_v2", Path(root),
            "447373903153", "espacigo-test-private", "espacigo-testprod-db",
        )])

    def test_services_list_rest_probe_emits_only_permission_metadata(self):
        token = "private-short-lived-token"
        marker = "private-service-name-and-response-body"
        payload = {
            "error": {
                "code": 403,
                "status": "PERMISSION_DENIED",
                "details": [{
                    "reason": "IAM_PERMISSION_DENIED",
                    "domain": "run.googleapis.com",
                    "metadata": {
                        "permission": "run.services.list",
                        "resource": f"projects/private-project/services/{marker}",
                    },
                }],
            },
        }
        observed = {}

        def token_runner(command, **_kwargs):
            observed["token_command"] = command
            return SimpleNamespace(returncode=0, stdout=token, stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["authorization"] = request.get_header("Authorization")
                return urllib.error.HTTPError(
                    request.full_url, 403, "Forbidden", {},
                    io.BytesIO(json.dumps(payload).encode()),
                )

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(
                    "cloud_run_services.list", root, runner=token_runner,
                    opener_factory=lambda *_handlers: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        self.assertEqual(observed["url"],
                         "https://run.googleapis.com/v2/projects/espacigo-testprod/locations/"
                         "southamerica-west1/services")
        self.assertEqual(observed["method"], "GET")
        self.assertEqual(observed["authorization"], f"Bearer {token}")
        self.assertEqual(observed["token_command"], ["gcloud", "auth", "print-access-token"])
        emitted = output.getvalue().strip()
        self.assertEqual(
            emitted,
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_services.list "
            "category=permission_denied status=403 permission=run.services.list "
            "service=run.googleapis.com",
        )
        self.assertNotIn(token, emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("private-project", emitted)

    def test_services_list_success_does_not_read_or_emit_service_names(self):
        marker = "private-service-name-list"
        response = None

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                self.read_called = True
                return json.dumps({"services": [{"name": marker}]}).encode()

        class FakeOpener:
            def open(self, _request, timeout):
                nonlocal response
                response = FakeResponse()
                response.read_called = False
                return response

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "cloud_run_services.list", root, runner=token_runner,
                    opener_factory=lambda *_handlers: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        self.assertFalse(response.read_called)
        self.assertNotIn(marker, output.getvalue())
        self.assertEqual(
            output.getvalue().strip(),
            "GCP08_CLOUD_RUN_PROBE stage=bootstrap target=cloud_run_services.list category=success",
        )

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

    def test_iap_cloud_run_policy_probe_uses_post_and_emits_only_allowlisted_fields(self):
        marker = "private-resource-and-message"
        payload = {"error": {"code": 403, "status": "PERMISSION_DENIED", "details": [{
            "reason": "IAM_PERMISSION_DENIED", "domain": "iap.googleapis.com",
            "metadata": {"permission": "iap.webServices.getIamPolicy", "resource": marker},
        }]}}
        observed = {}

        def token_runner(command, **_kwargs):
            observed["token_command"] = command
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["body"] = request.data
                observed["authorization"] = request.get_header("Authorization")
                return urllib.error.HTTPError(
                    request.full_url, 403, "Forbidden", {},
                    io.BytesIO(json.dumps(payload).encode()),
                )

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(
                    "cloud_run_service.private.iap_get_iam_policy", root,
                    runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                    project_number="447373903153", private_service_name="espacigo-test-private",
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        self.assertEqual(observed["method"], "POST")
        self.assertEqual(observed["body"], b"{}")
        self.assertEqual(observed["url"], "https://iap.googleapis.com/v1/projects/447373903153/"
                         "iap_web/cloud_run-southamerica-west1/services/espacigo-test-private:getIamPolicy")
        self.assertEqual(observed["authorization"], "Bearer token-hidden")
        emitted = output.getvalue().strip()
        self.assertEqual(emitted, "GCP08_CLOUD_RUN_PROBE stage=bootstrap "
                         "target=cloud_run_service.private.iap_get_iam_policy "
                         "category=permission_denied status=403 permission="
                         "iap.webServices.getIamPolicy service=iap.googleapis.com")
        self.assertNotIn(marker, emitted)
        self.assertNotIn("token-hidden", emitted)

    def test_iap_probe_rejects_unvalidated_resource_parameters_before_auth(self):
        runner = Mock()
        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(
                    "cloud_run_service.private.iap_get_iam_policy", root, runner=runner,
                    project_number="project-number", private_service_name="private/service",
                ))
            self.assertEqual(list(Path(root).iterdir()), [])
        runner.assert_not_called()
        self.assertIn("category=other", output.getvalue())

    def test_cloudsql_instance_probe_reports_only_allowlisted_permission_and_service(self):
        payload = {"error": {"code": 403, "details": [{
            "reason": "IAM_PERMISSION_DENIED", "domain": "sqladmin.googleapis.com",
            "metadata": {"permission": "cloudsql.instances.get", "resource": "private-resource"},
        }]}}

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                return urllib.error.HTTPError(
                    request.full_url, 403, "Forbidden", {},
                    io.BytesIO(json.dumps(payload).encode()),
                )

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_probe(
                    "cloud_sql_instance.v67.get", root, runner=token_runner,
                    opener_factory=lambda *_: FakeOpener(),
                    sql_instance_name="espacigo-testprod-db",
                ))
        self.assertIn("permission=cloudsql.instances.get", output.getvalue())
        self.assertIn("service=sqladmin.googleapis.com", output.getvalue())
        self.assertNotIn("private-resource", output.getvalue())
        self.assertNotIn("token-hidden", output.getvalue())

    def test_cloud_run_test_iam_permissions_reports_only_allowlisted_grants(self):
        observed = {}
        marker = "private-extra-permission"

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return json.dumps({"permissions": ["run.services.create", "run.services.update", marker]}).encode()

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["body"] = json.loads(request.data)
                return FakeResponse()

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "cloud_run_service.private.test_iam_permissions", root,
                    runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        self.assertEqual(observed["method"], "POST")
        self.assertEqual(observed["url"], "https://run.googleapis.com/v2/projects/espacigo-testprod/"
                         "locations/southamerica-west1/services/espacigo-test-private:testIamPermissions")
        self.assertEqual(set(observed["body"]["permissions"]), {
            "run.services.get", "run.services.getIamPolicy", "run.services.setIamPolicy",
            "run.services.update",
        })
        emitted = output.getvalue()
        self.assertIn("permission=run.services.update result=granted", emitted)
        self.assertIn("permission=run.services.get result=not_granted", emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("token-hidden", emitted)

    def test_cloud_run_create_parent_permission_uses_project_level_read_only_test(self):
        observed = {}
        marker = "private-extra-permission"

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return json.dumps({"permissions": [
                    "run.services.create", "run.operations.get",
                    "serviceusage.services.get", marker,
                ]}).encode()

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["body"] = json.loads(request.data)
                return FakeResponse()

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "cloud_run_create_parent.project.test_iam_permissions", root,
                    runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                ))
            self.assertEqual(list(Path(root).iterdir()), [])

        self.assertEqual(observed["method"], "POST")
        self.assertEqual(observed["url"],
                         "https://cloudresourcemanager.googleapis.com/v3/projects/"
                         "espacigo-testprod:testIamPermissions")
        self.assertEqual(set(observed["body"]["permissions"]), {
            "run.services.create", "run.operations.get", "serviceusage.services.get",
        })
        emitted = output.getvalue()
        self.assertIn("method=projects.testIamPermissions resource=project", emitted)
        self.assertIn("permission=run.services.create result=granted", emitted)
        self.assertIn("permission=run.operations.get result=granted", emitted)
        self.assertIn("permission=serviceusage.services.get result=granted", emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("token-hidden", emitted)

    def test_runtime_service_account_actas_test_uses_exact_account_and_safe_result(self):
        observed = {}

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return b'{"permissions":["iam.serviceAccounts.actAs","iam.serviceAccounts.get"]}'

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["body"] = json.loads(request.data)
                return FakeResponse()

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "runtime_service_account.private.test_act_as", root,
                    runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                ))
        self.assertEqual(observed["url"], "https://iam.googleapis.com/v1/projects/espacigo-testprod/"
                         "serviceAccounts/espacigo-test-private-web%40espacigo-testprod.iam.gserviceaccount.com:testIamPermissions")
        self.assertEqual(observed["method"], "POST")
        self.assertEqual(observed["body"], {
            "permissions": ["iam.serviceAccounts.actAs", "iam.serviceAccounts.get"]
        })
        self.assertIn("permission=iam.serviceAccounts.actAs result=granted", output.getvalue())
        self.assertIn("permission=iam.serviceAccounts.get result=granted", output.getvalue())

    def test_deployer_test_iam_permissions_checks_only_service_account_get(self):
        marker = "unapproved-permission-marker"
        observed = {}

        def token_runner(command, **_kwargs):
            observed["token_command"] = command
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return json.dumps({"permissions": ["iam.serviceAccounts.get", marker]}).encode()

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["body"] = json.loads(request.data)
                return FakeResponse()

        target = "service_account.deployer.test_get_metadata"
        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    target, root, runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                ))

        self.assertEqual(observed["token_command"], ["gcloud", "auth", "print-access-token"])
        self.assertEqual(observed["url"], "https://iam.googleapis.com/v1/projects/espacigo-testprod/"
                         "serviceAccounts/espacigo-test-deployer%40espacigo-testprod.iam.gserviceaccount.com:testIamPermissions")
        self.assertEqual(observed["method"], "POST")
        self.assertEqual(observed["body"], {"permissions": ["iam.serviceAccounts.get"]})
        emitted = output.getvalue()
        self.assertIn("target=service_account.deployer.test_get_metadata", emitted)
        self.assertIn("permission=iam.serviceAccounts.get result=granted", emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("token-hidden", emitted)
        self.assertNotIn("espacigo-test-deployer@", emitted)

    def test_service_account_metadata_and_policy_probes_are_fixed_read_only_requests(self):
        accounts = {
            "deployer": "espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com",
            "api": "espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com",
            "public": "espacigo-test-public-web@espacigo-testprod.iam.gserviceaccount.com",
            "private": "espacigo-test-private-web@espacigo-testprod.iam.gserviceaccount.com",
        }

        class FakeResponse:
            def __init__(self):
                self.read_called = False

            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                self.read_called = True
                return b'{"email":"private-email","policy":"private-policy"}'

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        for label, email in accounts.items():
            for suffix, method, body in (
                ("metadata.get", "GET", None),
                ("iam_policy.get", "POST", b""),
            ):
                with self.subTest(account=label, probe=suffix):
                    observed = {}
                    response = FakeResponse()

                    class FakeOpener:
                        def open(self, request, timeout):
                            observed["url"] = request.full_url
                            observed["method"] = request.get_method()
                            observed["body"] = request.data
                            observed["authorization"] = request.get_header("Authorization")
                            return response

                    target = f"service_account.{label}.{suffix}"
                    with tempfile.TemporaryDirectory() as root:
                        output = io.StringIO()
                        with redirect_stdout(output):
                            self.assertTrue(run_probe(
                                target, root, runner=token_runner,
                                opener_factory=lambda *_: FakeOpener(),
                            ))

                    encoded_email = urllib.parse.quote(email, safe="")
                    expected_url = (
                        "https://iam.googleapis.com/v1/projects/espacigo-testprod/serviceAccounts/"
                        f"{encoded_email}"
                    )
                    if suffix == "iam_policy.get":
                        expected_url += ":getIamPolicy?options.requestedPolicyVersion=3"
                    self.assertEqual(observed["url"], expected_url)
                    self.assertEqual(observed["method"], method)
                    self.assertEqual(observed["body"], body)
                    self.assertEqual(observed["authorization"], "Bearer token-hidden")
                    self.assertFalse(response.read_called)
                    emitted = output.getvalue()
                    self.assertIn(f"target={target} category=success", emitted)
                    self.assertNotIn(email, emitted)
                    self.assertNotIn("private-email", emitted)
                    self.assertNotIn("private-policy", emitted)
                    self.assertNotIn("token-hidden", emitted)

    def test_service_account_test_iam_checks_only_get_iam_policy_for_each_account(self):
        accounts = {
            "deployer": "espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com",
            "api": "espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com",
            "public": "espacigo-test-public-web@espacigo-testprod.iam.gserviceaccount.com",
            "private": "espacigo-test-private-web@espacigo-testprod.iam.gserviceaccount.com",
        }

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return (
                    b'{"permissions":["iam.serviceAccounts.getIamPolicy",'
                    b'"iam.serviceAccounts.update","private-permission"]}'
                )

        for label, email in accounts.items():
            with self.subTest(account=label):
                observed = {}

                class FakeOpener:
                    def open(self, request, timeout):
                        observed["url"] = request.full_url
                        observed["method"] = request.get_method()
                        observed["body"] = json.loads(request.data)
                        observed["authorization"] = request.get_header("Authorization")
                        return FakeResponse()

                target = f"service_account.{label}.test_get_iam_policy"
                with tempfile.TemporaryDirectory() as root:
                    output = io.StringIO()
                    with redirect_stdout(output):
                        self.assertTrue(run_probe(
                            target, root, runner=token_runner,
                            opener_factory=lambda *_: FakeOpener(),
                        ))
                    self.assertEqual(list(Path(root).iterdir()), [])

                encoded_email = urllib.parse.quote(email, safe="")
                self.assertEqual(
                    observed["url"],
                    "https://iam.googleapis.com/v1/projects/espacigo-testprod/serviceAccounts/"
                    f"{encoded_email}:testIamPermissions",
                )
                self.assertEqual(observed["method"], "POST")
                self.assertEqual(observed["body"], {
                    "permissions": ["iam.serviceAccounts.getIamPolicy"],
                })
                self.assertEqual(observed["authorization"], "Bearer token-hidden")
                emitted = output.getvalue()
                self.assertIn(f"target={target}", emitted)
                self.assertIn("resource=service_account category=success", emitted)
                self.assertIn("permission=iam.serviceAccounts.getIamPolicy result=granted", emitted)
                self.assertNotIn("iam.serviceAccounts.update", emitted)
                self.assertNotIn("private-permission", emitted)
                self.assertNotIn(email, emitted)
                self.assertNotIn("token-hidden", emitted)

    def test_service_account_policy_403_output_exposes_only_permission_service_and_status(self):
        permission = "iam.serviceAccounts.getIamPolicy"
        private_marker = "private-account-and-policy-data"
        payload = {"error": {"code": 403, "status": "PERMISSION_DENIED", "details": [{
            "reason": "IAM_PERMISSION_DENIED",
            "domain": "iam.googleapis.com",
            "metadata": {"permission": permission, "resource": private_marker},
        }]}}

        def token_runner(_command, **_kwargs):
            return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                return urllib.error.HTTPError(
                    request.full_url, 403, "Forbidden", {},
                    io.BytesIO(json.dumps(payload).encode()),
                )

        for label in ("deployer", "api", "public", "private"):
            with self.subTest(account=label):
                target = f"service_account.{label}.iam_policy.get"
                with tempfile.TemporaryDirectory() as root:
                    output = io.StringIO()
                    with redirect_stdout(output):
                        self.assertFalse(run_probe(
                            target, root, runner=token_runner,
                            opener_factory=lambda *_: FakeOpener(),
                        ))
                emitted = output.getvalue()
                self.assertIn(f"permission={permission}", emitted)
                self.assertIn("service=iam.googleapis.com", emitted)
                self.assertIn("category=permission_denied status=403", emitted)
                self.assertNotIn(private_marker, emitted)
                self.assertNotIn("token-hidden", emitted)

    def test_missing_iap_cloud_run_service_is_reported_as_not_found(self):
        payload = {"error": {"code": 404, "status": "NOT_FOUND"}}

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
                    "cloud_run_service.private.iap_get_iam_policy", root,
                    runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                    project_number="447373903153", private_service_name="espacigo-test-private",
                ))
        self.assertIn("category=not_found status=404", output.getvalue())
        self.assertNotIn("permission=", output.getvalue())

    def test_errorinfo_permission_uses_exact_cloud_run_allowlist(self):
        def fields(permission):
            payload = {"error": {"code": 403, "details": [{
                "reason": "IAM_PERMISSION_DENIED",
                "domain": "run.googleapis.com",
                "metadata": {"permission": permission, "resource": "private-resource"},
            }]}}
            with tempfile.TemporaryDirectory() as root:
                output = io.StringIO()
                body = json.dumps(payload).encode()

                def token_runner(_command, **_kwargs):
                    return SimpleNamespace(returncode=0, stdout="token-hidden", stderr="")

                class FakeOpener:
                    def open(self, request, timeout):
                        return urllib.error.HTTPError(
                            request.full_url, 403, "Forbidden", {}, io.BytesIO(body),
                        )

                with redirect_stdout(output):
                    self.assertFalse(run_probe(
                        "cloud_run_service.api.describe", root, runner=token_runner,
                        opener_factory=lambda *_handlers: FakeOpener(),
                    ))
            return output.getvalue().strip()

        allowed = fields("run.services.setIamPolicy")
        self.assertIn("permission=run.services.setIamPolicy", allowed)
        self.assertNotIn("private-resource", allowed)
        unknown = fields("run.services.delete")
        self.assertIn("category=permission_denied status=403", unknown)
        self.assertNotIn("permission=", unknown)
        self.assertNotIn("run.services.delete", unknown)

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
            "GCP08_CLOUD_RUN_IAM_COMPARE stage=bootstrap alias=public "
            "api=v1 status=success category=success",
        )

    def test_cloud_run_v2_iam_policy_probe_uses_provider_endpoint_and_emits_only_alias_status(self):
        marker = "private-policy-body-and-resource"
        observed = {}

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, *_args):
                return None

            def getcode(self):
                return 200

            def read(self, _limit):
                return marker.encode()

        def token_runner(command, **_kwargs):
            self.assertEqual(command, ["gcloud", "auth", "print-access-token"])
            return SimpleNamespace(returncode=0, stdout="private-token", stderr="")

        class FakeOpener:
            def open(self, request, timeout):
                observed["url"] = request.full_url
                observed["method"] = request.get_method()
                observed["authorization"] = request.get_header("Authorization")
                return FakeResponse()

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_probe(
                    "cloud_run_service.public.get_iam_policy_v2", root,
                    runner=token_runner, opener_factory=lambda *_: FakeOpener(),
                ))
        self.assertEqual(
            observed["url"],
            "https://run.googleapis.com/v2/projects/espacigo-testprod/locations/"
            "southamerica-west1/services/espacigo-test-public:getIamPolicy"
            "?options.requestedPolicyVersion=3",
        )
        self.assertEqual(observed["method"], "GET")
        self.assertEqual(observed["authorization"], "Bearer private-token")
        self.assertEqual(output.getvalue().strip(),
                         "GCP08_CLOUD_RUN_IAM_COMPARE stage=bootstrap alias=public "
                         "api=v2 status=200 category=success")
        self.assertNotIn(marker, output.getvalue())
        self.assertNotIn("private-token", output.getvalue())
        self.assertNotIn("espacigo-test-public", output.getvalue())

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
        private_dir = workflow.index('--private-dir "$RUNNER_TEMP/gcp07d"')
        start = workflow.rfind("python3 \"$GITHUB_WORKSPACE/scripts/gcp08_bootstrap_cloud_run_probes.py\"", 0, private_dir)
        block_start = workflow.rfind("if [[ \"$POLICY_MODE\" == bootstrap ]]", 0, start)
        self.assertGreater(block_start, workflow.index("terraform plan -input=false"))
        self.assertIn("inputs.operation == 'initial-restore'", workflow[setup:rollout])
        self.assertIn("gcp08_bootstrap_refresh_diagnostics.py", workflow[block_start:start])
        self.assertIn('--var-file "$GCP07D_TFVARS"', workflow[start:start + 250])
        self.assertNotRegex(workflow[start:start + 300], r"gcloud\s+run\s+services\s+list|--log-http")

    def test_workflow_validates_protected_names_before_authentication(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        validation = workflow.index("Validate protected bootstrap probe targets")
        auth = workflow.index("Authenticate bootstrap federated identity")
        block = workflow[validation:auth]
        self.assertLess(validation, auth)
        self.assertIn("--validate-configuration", block)
        self.assertIn('--var-file "$GCP07D_TFVARS"', block)
        self.assertIn(
            'run: python3 "$GITHUB_WORKSPACE/scripts/gcp08_bootstrap_cloud_run_probes.py" '
            '--validate-configuration --var-file "$GCP07D_TFVARS"',
            block,
        )


if __name__ == "__main__":
    unittest.main()
