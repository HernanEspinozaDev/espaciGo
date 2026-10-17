import base64
from pathlib import Path
import sys
import unittest
from unittest import mock

sys.path.insert(0, str(Path(__file__).resolve().parent))
import gcp03_origin_admin_secret as helper


def preflight_response(host, path, method, token, body=None):
    if host == "sqladmin.googleapis.com" and path.endswith("/instances/" + helper.INSTANCE):
        return {
            "name": helper.INSTANCE,
            "project": helper.PROJECT,
            "region": helper.REGION,
            "databaseVersion": "POSTGRES_18",
            "state": "RUNNABLE",
            "settings": {
                "ipConfiguration": {"ipv4Enabled": False},
                "userLabels": {"run_id": "g03o_1017"},
            },
        }
    if host == "sqladmin.googleapis.com" and path.endswith("/databases"):
        return {"items": [{"name": helper.DATABASE}]}
    if host == "secretmanager.googleapis.com" and path == f"/v1/{helper.SECRET_RESOURCE}":
        return {"name": helper.SECRET_RESOURCE}
    if host == "secretmanager.googleapis.com" and path.startswith(f"/v1/{helper.SECRET_RESOURCE}/versions?"):
        return {"versions": []}
    raise AssertionError((host, path, method))


class CredentialPreparationTests(unittest.TestCase):
    def test_resource_names_normalize_only_the_known_project_id_and_number(self):
        self.assertEqual(
            helper.canonical_resource_name(
                f"projects/{helper.PROJECT}/secrets/{helper.SECRET}", f"secrets/{helper.SECRET}"
            ),
            helper.SECRET_RESOURCE,
        )
        self.assertEqual(
            helper.canonical_resource_name(helper.SECRET_RESOURCE, f"secrets/{helper.SECRET}"),
            helper.SECRET_RESOURCE,
        )
        self.assertEqual(
            helper.canonical_version_name(f"projects/{helper.PROJECT}/secrets/{helper.SECRET}/versions/12"),
            f"{helper.SECRET_RESOURCE}/versions/12",
        )
        with self.assertRaises(helper.ApiFailure):
            helper.canonical_resource_name("projects/another-project/secrets/" + helper.SECRET, f"secrets/{helper.SECRET}")
        with self.assertRaises(helper.ApiFailure):
            helper.canonical_version_name(f"projects/447373903114/secrets/{helper.SECRET}/versions/12")

    @mock.patch.object(helper, "api_request")
    def test_validate_target_accepts_canonical_number_and_rejects_foreign_secret(self, request):
        def canonical(host, path, method, token, body=None):
            response = preflight_response(host, path, method, token, body)
            if host == "secretmanager.googleapis.com" and path == f"/v1/{helper.SECRET_RESOURCE}":
                response["name"] = f"projects/{helper.PROJECT_NUMBER}/secrets/{helper.SECRET}"
            return response

        request.side_effect = canonical
        helper.validate_target("access-token")
        self.assertTrue(request.call_args_list[-1].args[1].startswith(f"/v1/{helper.SECRET_RESOURCE}/versions?"))

        def foreign(host, path, method, token, body=None):
            response = preflight_response(host, path, method, token, body)
            if host == "secretmanager.googleapis.com" and path == f"/v1/{helper.SECRET_RESOURCE}":
                response["name"] = f"projects/447373903114/secrets/{helper.SECRET}"
            return response

        request.reset_mock(side_effect=True)
        request.side_effect = foreign
        with self.assertRaisesRegex(helper.ApiFailure, "outside the fixed project"):
            helper.validate_target("access-token")
        self.assertFalse(any(":addVersion" in call.args[1] for call in request.call_args_list))

    def test_api_request_sets_quota_project_without_echoing_error_body(self):
        connection = mock.Mock()
        response = mock.Mock(status=403)
        response.read.return_value = b'{"error":{"message":"sensitive detail"}}'
        connection.getresponse.return_value = response
        with mock.patch.object(helper.http.client, "HTTPSConnection", return_value=connection):
            with self.assertRaises(helper.ApiFailure) as raised:
                helper.api_request("secretmanager.googleapis.com", "/v1/x", "GET", "access-token")
        self.assertEqual(raised.exception.status, 403)
        self.assertNotIn("sensitive detail", str(raised.exception))
        self.assertEqual(connection.request.call_args.kwargs["headers"]["X-Goog-User-Project"], helper.PROJECT)

    @mock.patch.object(helper, "get_token", return_value="access-token")
    @mock.patch.object(helper.secrets, "token_urlsafe", return_value="generated-only-in-memory")
    @mock.patch.object(helper, "api_request")
    def test_success_adds_one_numeric_version_and_updates_only_postgres(self, request, _password, _token):
        def respond(host, path, method, token, body=None):
            if "/operations/op-1" in path:
                return {"status": "DONE"}
            if method == "GET":
                return preflight_response(host, path, method, token, body)
            if path == f"/v1/{helper.SECRET_RESOURCE}:addVersion":
                secret_bytes = base64.b64decode(body["payload"]["data"])
                self.assertEqual(secret_bytes.decode(), "generated-only-in-memory")
                return {"name": f"projects/{helper.PROJECT_NUMBER}/secrets/{helper.SECRET}/versions/1"}
            if host == "sqladmin.googleapis.com" and method == "PUT":
                self.assertEqual(body, {"name": "postgres", "password": "generated-only-in-memory"})
                self.assertIn(f"/instances/{helper.INSTANCE}/users?name=postgres", path)
                return {"name": "op-1"}
            raise AssertionError((host, path, method))

        request.side_effect = respond
        self.assertEqual(helper.execute(), f"{helper.SECRET_RESOURCE}/versions/1")
        self.assertEqual(sum(call.args[2] == "POST" for call in request.call_args_list), 1)
        self.assertEqual(sum(call.args[2] == "PUT" for call in request.call_args_list), 1)

    @mock.patch.object(helper, "get_token", return_value="access-token")
    @mock.patch.object(helper.secrets, "token_urlsafe", return_value="generated-only-in-memory")
    @mock.patch.object(helper, "api_request")
    def test_add_version_ambiguous_stops_before_sql_update_and_does_not_retry(self, request, _password, _token):
        def respond(host, path, method, token, body=None):
            if method == "GET":
                return preflight_response(host, path, method, token, body)
            if path == f"/v1/{helper.SECRET_RESOURCE}:addVersion":
                raise helper.ApiFailure("ambiguous", uncertain=True)
            raise AssertionError((host, path, method))

        request.side_effect = respond
        with self.assertRaisesRegex(helper.ApiFailure, "ambiguous"):
            helper.execute()
        self.assertEqual(sum(call.args[2] == "POST" for call in request.call_args_list), 1)
        self.assertEqual(sum(call.args[2] == "PUT" for call in request.call_args_list), 0)

    @mock.patch.object(helper, "get_token", return_value="access-token")
    @mock.patch.object(helper.secrets, "token_urlsafe", return_value="generated-only-in-memory")
    @mock.patch.object(helper, "api_request")
    def test_sql_update_ambiguous_reports_exact_version_without_retry(self, request, _password, _token):
        def respond(host, path, method, token, body=None):
            if method == "GET":
                return preflight_response(host, path, method, token, body)
            if path == f"/v1/{helper.SECRET_RESOURCE}:addVersion":
                return {"name": f"projects/{helper.PROJECT_NUMBER}/secrets/{helper.SECRET}/versions/7"}
            if method == "PUT":
                raise helper.ApiFailure("ambiguous", uncertain=True)
            raise AssertionError((host, path, method))

        request.side_effect = respond
        with self.assertRaisesRegex(helper.ApiFailure, "versions/7; do not rotate or rerun"):
            helper.execute()
        self.assertEqual(sum(call.args[2] == "POST" for call in request.call_args_list), 1)
        self.assertEqual(sum(call.args[2] == "PUT" for call in request.call_args_list), 1)

    @mock.patch.object(helper, "get_token", return_value="access-token")
    @mock.patch.object(helper.secrets, "token_urlsafe", return_value="generated-only-in-memory")
    @mock.patch.object(helper, "api_request")
    def test_poll_permission_error_after_put_acceptance_keeps_version_enabled(self, request, _password, _token):
        def respond(host, path, method, token, body=None):
            if "/operations/op-accepted" in path:
                raise helper.ApiFailure("read forbidden", status=403)
            if method == "GET":
                return preflight_response(host, path, method, token, body)
            if path == f"/v1/{helper.SECRET_RESOURCE}:addVersion":
                return {"name": f"projects/{helper.PROJECT_NUMBER}/secrets/{helper.SECRET}/versions/9"}
            if method == "PUT":
                return {"name": "op-accepted"}
            raise AssertionError((host, path, method))

        request.side_effect = respond
        with self.assertRaisesRegex(helper.ApiFailure, "versions/9; do not rotate or rerun"):
            helper.execute()
        self.assertEqual(sum(call.args[2] == "POST" for call in request.call_args_list), 1)
        self.assertEqual(sum(call.args[2] == "PUT" for call in request.call_args_list), 1)
        self.assertEqual(sum(":disable" in call.args[1] for call in request.call_args_list), 0)

    @mock.patch.object(helper, "get_token", return_value="access-token")
    @mock.patch.object(helper.secrets, "token_urlsafe", return_value="generated-only-in-memory")
    @mock.patch.object(helper, "api_request")
    def test_definite_sql_rejection_disables_only_the_new_exact_version(self, request, _password, _token):
        def respond(host, path, method, token, body=None):
            if method == "GET":
                return preflight_response(host, path, method, token, body)
            if path == f"/v1/{helper.SECRET_RESOURCE}:addVersion":
                return {"name": f"projects/{helper.PROJECT_NUMBER}/secrets/{helper.SECRET}/versions/10"}
            if method == "PUT":
                raise helper.ApiFailure("rejected", status=400)
            if path == f"/v1/{helper.SECRET_RESOURCE}/versions/10:disable":
                return {}
            raise AssertionError((host, path, method))

        request.side_effect = respond
        with self.assertRaisesRegex(helper.ApiFailure, "version was disabled"):
            helper.execute()
        self.assertEqual(sum(call.args[2] == "POST" for call in request.call_args_list), 2)
        self.assertEqual(sum(call.args[2] == "PUT" for call in request.call_args_list), 1)
        self.assertEqual(sum(":addVersion" in call.args[1] for call in request.call_args_list), 1)

    @mock.patch.object(helper, "get_token", return_value="access-token")
    @mock.patch.object(helper.secrets, "token_urlsafe", return_value="generated-only-in-memory")
    @mock.patch.object(helper.time, "sleep")
    @mock.patch.object(helper.time, "monotonic", side_effect=[0, 1, 601])
    @mock.patch.object(helper, "api_request")
    def test_pending_operation_timeout_does_not_rotate_or_retry(self, request, _clock, _sleep, _password, _token):
        def respond(host, path, method, token, body=None):
            if "/operations/op-pending" in path:
                return {"status": "PENDING"}
            if method == "GET" and "/operations/" not in path:
                return preflight_response(host, path, method, token, body)
            if path == f"/v1/{helper.SECRET_RESOURCE}:addVersion":
                return {"name": f"projects/{helper.PROJECT_NUMBER}/secrets/{helper.SECRET}/versions/8"}
            if method == "PUT":
                return {"name": "op-pending"}
            raise AssertionError((host, path, method))

        request.side_effect = respond
        with self.assertRaisesRegex(helper.ApiFailure, "versions/8; do not rotate or rerun"):
            helper.execute()
        self.assertEqual(sum(call.args[2] == "POST" for call in request.call_args_list), 1)
        self.assertEqual(sum(call.args[2] == "PUT" for call in request.call_args_list), 1)
        self.assertEqual(sum("/operations/op-pending" in call.args[1] for call in request.call_args_list), 1)


if __name__ == "__main__":
    unittest.main()
