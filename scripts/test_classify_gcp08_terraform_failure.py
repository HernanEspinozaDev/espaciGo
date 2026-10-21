import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from classify_gcp08_terraform_failure import (
    classify, denied_permission, safe_details, terraform_address, terraform_operation,
)


SCRIPT = Path(__file__).with_name("classify_gcp08_terraform_failure.py")


class ClassifyGcp08TerraformFailureTests(unittest.TestCase):
    def test_classifies_variable_errors_without_emitting_diagnostic_values(self):
        secret_marker = "must-never-appear-in-output"
        diagnostic = f"Error: No value for required variable; supplied value={secret_marker}"
        self.assertEqual(classify("plan", diagnostic), "variables")
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False) as stream:
            stream.write(diagnostic)
            path = stream.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--stage", "plan", path],
                check=True, capture_output=True, text=True,
            )
        finally:
            os.unlink(path)
        self.assertEqual(result.stdout.strip(), "GCP08_TERRAFORM_FAILURE stage=plan category=variables")
        self.assertNotIn(secret_marker, result.stdout + result.stderr)
        self.assertNotIn("required variable", result.stdout + result.stderr)

    def test_distinguishes_iam_provider_and_backend_errors(self):
        self.assertEqual(classify("plan", "googleapi: Error 403: permission denied"), "iam")
        self.assertEqual(classify("plan", "PERMISSION_DENIED: Permission 'run.services.get' denied"), "iam")
        self.assertEqual(classify("init", "could not find default credentials"), "provider")
        self.assertEqual(classify("init", "failed to acquire the state lock"), "backend")

    def test_emits_only_a_recognized_denied_permission_name(self):
        diagnostic = "Error 403: Permission 'run.services.getIamPolicy' denied on private-resource secret-value"
        self.assertEqual(denied_permission(diagnostic), "run.services.getIamPolicy")

    def test_emits_permissions_from_each_approved_gcp_namespace(self):
        permissions = (
            "run.services.getIamPolicy",
            "compute.networks.get",
            "cloudsql.instances.get",
            "secretmanager.secrets.get",
            "iap.webServices.setIamPolicy",
            "artifactregistry.repositories.downloadArtifacts",
            "iam.roles.get",
            "iam.serviceAccounts.actAs",
            "storage.objects.get",
            "storage.managedFolders.get",
            "resourcemanager.projects.get",
            "serviceusage.services.use",
        )
        for permission in permissions:
            with self.subTest(permission=permission):
                self.assertEqual(
                    denied_permission(f"Permission '{permission}' denied on private-resource secret-value"),
                    permission,
                )

    def test_rejects_permissions_outside_allowlisted_namespaces_or_invalid_names(self):
        diagnostics = (
            "Permission 'token-looking-secret-value' denied",
            "Permission 'madeup.services.get' denied",
            "Permission 'storage.buckets.get' denied",
            "Permission 'iam.groups.get' denied",
            "Permission 'resourcemanager.organizations.get' denied",
            "Permission 'compute.invalid permission' denied",
            "Permission 'compute.instances.get;secret=value' denied",
            "Permission 'Compute.instances.get' denied",
        )
        for diagnostic in diagnostics:
            with self.subTest(diagnostic=diagnostic):
                self.assertIsNone(denied_permission(diagnostic))

    def test_cli_reports_permission_name_without_resource_or_value(self):
        sensitive_marker = "private-resource-and-value-must-stay-private"
        diagnostic = f"Permission 'run.services.getIamPolicy' denied on {sensitive_marker}"
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False) as stream:
            stream.write(diagnostic)
            path = stream.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--stage", "plan", path],
                check=True, capture_output=True, text=True,
            )
        finally:
            os.unlink(path)
        self.assertEqual(
            result.stdout.strip(),
            "GCP08_TERRAFORM_FAILURE stage=plan category=iam permission=run.services.getIamPolicy "
            "service=run.googleapis.com resource_type=run.googleapis.com/Service operation=getIamPolicy",
        )
        self.assertNotIn(sensitive_marker, result.stdout + result.stderr)

    def test_cli_reports_new_gcp_namespace_without_diagnostic_context(self):
        sensitive_marker = "resource-name-and-secret-value"
        diagnostic = f"Permission 'secretmanager.secrets.get' denied on {sensitive_marker}"
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False) as stream:
            stream.write(diagnostic)
            path = stream.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--stage", "plan", path],
                check=True, capture_output=True, text=True,
            )
        finally:
            os.unlink(path)
        self.assertEqual(
            result.stdout.strip(),
            "GCP08_TERRAFORM_FAILURE stage=plan category=iam permission=secretmanager.secrets.get "
            "service=secretmanager.googleapis.com "
            "resource_type=secretmanager.googleapis.com/Secret operation=get",
        )
        self.assertNotIn(sensitive_marker, result.stdout + result.stderr)

    def test_googleapi_permission_error_emits_only_safe_structured_fields(self):
        sensitive_values = ("projects/secret-project", "db-prod-private", "user@example.test")
        diagnostic = (
            "googleapi: Error 403: Permission 'cloudsql.instances.get' denied on resource "
            "'projects/secret-project/instances/db-prod-private' (caller user@example.test), forbidden"
        )
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False) as stream:
            stream.write(diagnostic)
            path = stream.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--stage", "plan", path],
                check=True, capture_output=True, text=True,
            )
        finally:
            os.unlink(path)
        self.assertEqual(
            result.stdout.strip(),
            "GCP08_TERRAFORM_FAILURE stage=plan category=iam status=403 reason=permission_denied "
            "permission=cloudsql.instances.get service=sqladmin.googleapis.com "
            "resource_type=sqladmin.googleapis.com/Instance operation=get",
        )
        for value in sensitive_values:
            self.assertNotIn(value, result.stdout + result.stderr)

    def test_provider_status_variants_are_bounded_to_predefined_codes(self):
        from classify_gcp08_terraform_failure import safe_details

        self.assertEqual(safe_details("Error 404: not found"), ["status=404", "reason=not_found"])
        self.assertEqual(safe_details("HTTP 429: quota"), ["status=429", "reason=quota_exceeded"])
        self.assertEqual(safe_details("status code 418: private details"), [])
        self.assertEqual(safe_details("opaque 403 value"), [])
        self.assertEqual(safe_details("Error 403: sqladmin.googleapis.com.evil/private"), [
            "status=403", "reason=permission_denied",
        ])

    def test_service_disabled_provider_error_never_echoes_message_or_resource(self):
        diagnostic = (
            "googleapi: Error 403: sqladmin.googleapis.com API is disabled for "
            "projects/secret-project, serviceDisabled"
        )
        from classify_gcp08_terraform_failure import safe_details

        self.assertEqual(
            safe_details(diagnostic),
            ["status=403", "reason=service_disabled", "service=sqladmin.googleapis.com"],
        )
        self.assertNotIn("secret-project", " ".join(safe_details(diagnostic)))

    def test_google_rpc_errorinfo_permission_and_terraform_address_are_allowlisted(self):
        sensitive_values = ("project-secret-marker", "service-secret-marker", "request-secret-marker")
        diagnostic = (
            'googleapi: Error 403: denied {"error":{"code":403,"details":[{"@type":'
            '"type.googleapis.com/google.rpc.ErrorInfo","reason":"IAM_PERMISSION_DENIED",'
            '"domain":"iam.googleapis.com","metadata":{"permission":"run.services.getIamPolicy",'
            '"resource":"projects/project-secret-marker/services/service-secret-marker",'
            '"request":"request-secret-marker"}}]}}\n'
            "  with google_cloud_run_v2_service.api[0],\n  on main.tf line 687"
        )
        details = safe_details(diagnostic)
        self.assertEqual(details, [
            "status=403", "reason=permission_denied", "permission=run.services.getIamPolicy",
            "service=run.googleapis.com", "resource_type=run.googleapis.com/Service",
            "address=cloud_run_v2_service.api", "operation=getIamPolicy",
        ])
        for value in sensitive_values:
            self.assertNotIn(value, " ".join(details))

    def test_proto_errorinfo_format_and_api_methods_map_to_fixed_operations(self):
        diagnostic = (
            'googleapi: Error 403: ErrorInfo{Reason: IAM_PERMISSION_DENIED, '
            'Domain: iam.googleapis.com, Metadata: map[permission:run.services.create '
            'resource:projects/private-marker/services/private-marker]}\n'
            '  with google_cloud_run_v2_service.web["private"],'
        )
        self.assertEqual(denied_permission(diagnostic), "run.services.create")
        self.assertEqual(terraform_address(diagnostic), "cloud_run_v2_service.web.private")
        self.assertEqual(terraform_operation(diagnostic, denied_permission(diagnostic)), "create")
        self.assertEqual(safe_details(diagnostic), [
            "status=403", "reason=permission_denied", "permission=run.services.create",
            "service=run.googleapis.com", "resource_type=run.googleapis.com/Service",
            "address=cloud_run_v2_service.web.private", "operation=create",
        ])

    def test_cli_emits_enum_address_and_operation_without_provider_payload(self):
        markers = ("private-project-marker", "private-service-marker", "secret-request-marker")
        diagnostic = (
            'googleapi: Error 403: denied {"error":{"details":[{"@type":'
            '"type.googleapis.com/google.rpc.ErrorInfo","metadata":{'
            '"permission":"run.services.update","resource":"projects/private-project-marker/'
            'services/private-service-marker","request":"secret-request-marker"}}]}}\n'
            '  with google_cloud_run_v2_service.web["private"],'
        )
        with tempfile.NamedTemporaryFile("w", encoding="utf-8", delete=False) as stream:
            stream.write(diagnostic)
            path = stream.name
        try:
            result = subprocess.run(
                [sys.executable, str(SCRIPT), "--stage", "plan", path],
                check=True, capture_output=True, text=True,
            )
        finally:
            os.unlink(path)
        self.assertEqual(
            result.stdout.strip(),
            "GCP08_TERRAFORM_FAILURE stage=plan category=iam status=403 reason=permission_denied "
            "permission=run.services.update service=run.googleapis.com "
            "resource_type=run.googleapis.com/Service address=cloud_run_v2_service.web.private "
            "operation=update",
        )
        for marker in markers:
            self.assertNotIn(marker, result.stdout + result.stderr)

    def test_iam_addresses_and_operations_use_enums_without_key_values(self):
        diagnostic = (
            "Error: Error when reading IAP binding\n"
            '  with google_iap_web_cloud_run_service_iam_member.private_evaluators'
            '["user:private-marker@example.test"],\n  on main.tf line 1350'
        )
        self.assertEqual(terraform_address(diagnostic), "iap_iam.private_evaluators")
        self.assertEqual(terraform_operation(diagnostic), "read")
        details = safe_details(diagnostic)
        self.assertEqual(details, [
            "address=iap_iam.private_evaluators", "operation=read",
        ])
        self.assertNotIn("private-marker", " ".join(details))

    def test_structured_provider_operation_field_is_canonicalized(self):
        diagnostic = (
            'googleapi: Error 403: {"error":{"code":403,"operation":"GetIamPolicy",'
            '"resource":"projects/private-marker/services/private-marker"}}\n'
            '  with google_cloud_run_v2_service.api[0],'
        )
        self.assertEqual(terraform_operation(diagnostic), "getIamPolicy")
        self.assertEqual(safe_details(diagnostic), [
            "status=403", "reason=permission_denied", "address=cloud_run_v2_service.api",
            "operation=getIamPolicy",
        ])

    def test_structured_permission_and_address_extractors_fail_closed(self):
        malformed = (
            'Error 403 {"metadata":{"permission":"not-a-gcp-permission"}}',
            'Error 403 {"metadata":{"permission":"run.services.get;secret=x"}}',
            'Error 403 {"metadata":{"permission":"run.services.get"}} '
            'Error 403 {"metadata":{"permission":"compute.instances.get"}}',
        )
        for diagnostic in malformed:
            with self.subTest(diagnostic=diagnostic):
                self.assertIsNone(denied_permission(diagnostic))
        # Addresses in shell command options are not provider error context.
        self.assertIsNone(terraform_address(
            '-target=google_cloud_run_v2_service.api[0]'
        ))
        self.assertEqual(terraform_operation("Error when updating Cloud Run Service"), "update")

    def test_policy_rejection_is_guard_and_unmatched_errors_stay_unclassified(self):
        self.assertEqual(classify("policy", "sensitive image and state data"), "guard")
        self.assertEqual(classify("plan", "opaque diagnostic with secret-looking values"), "unclassified")


if __name__ == "__main__":
    unittest.main()
