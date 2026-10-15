import importlib.util
import hashlib
import socket
import tempfile
import unittest
from datetime import datetime, timedelta, timezone
from pathlib import Path
from unittest.mock import patch


SCRIPT = Path(__file__).with_name("dataset-import.py")
SPEC = importlib.util.spec_from_file_location("dataset_import", SCRIPT)
loader = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(loader)


class FakeAPI:
    calls = []
    account = None
    fail_registration_once = False
    token_counter = 0
    active_tokens = {"operator-session-that-predates-importer"}
    galleries = {}
    files = {}
    serial = 0

    def __init__(self, bases, token=""):
        self.bases = bases
        self.token = token

    def request(self, method, path, body=None, key="", expect=(200,)):
        type(self).calls.append((method, path, body, self.token))
        if path == "/api/v1/auth/login":
            type(self).token_counter += 1
            token = f"importer-token-{type(self).token_counter}"
            type(self).active_tokens.add(token)
            return {"access_token": token, "roles": ["administrador_global"],
                    "capabilities": ["legacy.administration"]}
        if path == "/api/v1/auth/logout":
            type(self).active_tokens.discard(self.token)
            return {}
        if path.startswith("/api/v1/admin/global/accounts?"):
            return {"data": {"items": [type(self).account] if type(self).account else []}}
        if path == "/api/v1/auth/register":
            if type(self).fail_registration_once:
                type(self).fail_registration_once = False
                type(self).account = {
                    "account_id": "account-created-by-import", "email": body["email"],
                    "created_at": (datetime.now(timezone.utc) + timedelta(seconds=1)).isoformat(), "roles": [],
                }
                raise loader.SafeHTTPError(path, 0, "connection_failed")
            raise loader.SafeHTTPError(path, 409, "email_registered")
        if path == "/api/v1/profile":
            return {"display_name": "Synthetic User", "phone": None}
        if path.endswith("/gallery") and method == "GET":
            space = path.split("/")[-2]
            return {"items": list(type(self).galleries.get(space, []))}
        if path.endswith("/gallery") and method == "POST":
            space = path.split("/")[-2]
            type(self).serial += 1
            photo_id = f"new-photo-{type(self).serial}"
            blob = b"\x89PNG\r\n\x1a\nfixture"
            item = {"id": photo_id, "fixture_code": "synthetic-png-v1", "mime_type": "image/png",
                    "sha256": hashlib.sha256(blob).hexdigest(), "size_bytes": len(blob),
                    "public_in_catalog": False}
            type(self).galleries.setdefault(space, []).append(item)
            type(self).files[photo_id] = blob
            return {"photo": item, "reused": False}
        if path.endswith("/cover") and method == "PUT":
            return {"photo_id": body["photo_id"], "selected": True}
        if path.endswith("/publication") and method == "PUT":
            parts = path.split("/")
            space, photo_id = parts[-4], parts[-2]
            item = next(row for row in type(self).galleries[space] if row["id"] == photo_id)
            item["public_in_catalog"] = body["public_in_catalog"]
            return item
        if "/gallery/" in path and method == "DELETE":
            parts = path.split("/")
            space, photo_id = parts[-3], parts[-1]
            type(self).galleries[space] = [row for row in type(self).galleries.get(space, []) if row["id"] != photo_id]
            type(self).files.pop(photo_id, None)
            return {"removed": True}
        raise AssertionError(f"unexpected API request: {method} {path}")

    def request_bytes(self, path, token=""):
        photo_id = path.rsplit("/", 2)[-2] if path.endswith("/content") else path.rsplit("/", 1)[-1]
        blob = type(self).files.get(photo_id)
        if blob is None:
            return 404, {"Content-Type": "application/json"}, b""
        return 200, {"Content-Type": "image/png"}, blob


class DatasetImporterTests(unittest.TestCase):
    def setUp(self):
        FakeAPI.calls = []
        FakeAPI.account = None
        FakeAPI.fail_registration_once = False
        FakeAPI.token_counter = 0
        FakeAPI.active_tokens = {"operator-session-that-predates-importer"}
        FakeAPI.galleries = {}
        FakeAPI.files = {}
        FakeAPI.serial = 0
        self.network_block = patch.object(
            socket, "create_connection",
            side_effect=AssertionError("network access is disabled in dataset importer tests"),
        )
        self.network_block.start()
        self.socket_connect_block = patch.object(
            socket.socket, "connect",
            side_effect=AssertionError("network access is disabled in dataset importer tests"),
        )
        self.socket_connect_block.start()
        self.api_patch = patch.object(loader, "API", FakeAPI)
        self.api_patch.start()
        self.temp = tempfile.TemporaryDirectory()
        self.importer = loader.Importer(["http://127.0.0.1:8080"], "http://127.0.0.1:8025",
                                        {"owner_test_email": "operator@example.invalid", "test_password": "secret"},
                                        {"users": {}, "pending_users": {}, "verifications": {}, "listings": {}},
                                        "espacigo-1000-v1")
        self.importer._state_path = Path(self.temp.name) / "state.json"
        self.importer.terms_ids = ["terms-version"]

    def tearDown(self):
        try:
            self.assertEqual(self.importer.cleanup_sessions(), 0)
            self.assertEqual(FakeAPI.active_tokens, {"operator-session-that-predates-importer"})
        finally:
            self.temp.cleanup()
            self.api_patch.stop()
            self.socket_connect_block.stop()
            self.network_block.stop()

    def test_preexisting_same_email_collision_is_rejected_before_any_mutation(self):
        FakeAPI.account = {"account_id": "unrelated-account", "email": "u0001@dataset.invalid",
                           "created_at": "2026-10-14T12:00:00+00:00", "display_name": "Existing profile",
                           "password": "secret", "roles": ["administrador"]}
        with self.assertRaisesRegex(RuntimeError, "already belongs"):
            self.importer.register_or_recover(
                {"user_ref": "u0001", "roles": ["arrendatario"]}, {"display_name": "Synthetic User"})
        self.assertFalse(any(path == "/api/v1/auth/register" or path.startswith("/api/v1/profile") or "/roles/" in path or "legacy-administrator" in path
                             for method, path, _, _ in FakeAPI.calls))
        self.assertEqual(FakeAPI.account["roles"], ["administrador"])
        self.assertEqual(FakeAPI.account["display_name"], "Existing profile")
        self.assertEqual(FakeAPI.account["password"], "secret")

    def test_post_commit_before_checkpoint_recovers_from_durable_absent_baseline(self):
        FakeAPI.fail_registration_once = True
        user = {"user_ref": "u0001", "roles": ["arrendatario"]}
        profile = {"display_name": "Synthetic User"}
        with self.assertRaises(loader.SafeHTTPError):
            self.importer.register_or_recover(user, profile)
        self.assertNotIn("u0001", self.importer.state["users"])
        self.assertTrue(self.importer.state["registration_attempts"]["u0001"]["baseline_absent"])
        self.assertEqual(self.importer.register_or_recover(user, profile), "account-created-by-import")
        self.assertIn("u0001", self.importer.state["users"])
        self.assertEqual(FakeAPI.account["roles"], [])

    def test_success_and_injected_failure_cleanup_only_importer_sessions(self):
        self.importer.email = "operator@example.invalid"
        self.importer.password = "secret"
        self.importer.admin()
        first = self.importer.admin_token
        self.importer.admin()
        self.assertEqual(self.importer.admin_token, first)
        self.assertEqual(self.importer.cleanup_sessions(), 0)
        self.assertEqual(FakeAPI.active_tokens, {"operator-session-that-predates-importer"})
        try:
            self.importer.login("user@example.invalid", "secret")
            raise RuntimeError("injected import failure")
        except RuntimeError as error:
            self.assertEqual(str(error), "injected import failure")
        finally:
            self.assertEqual(self.importer.cleanup_sessions(), 0)
        self.assertEqual(FakeAPI.active_tokens, {"operator-session-that-predates-importer"})

    def test_missing_fixture_file_is_replaced_idempotently_and_selected_as_public_cover(self):
        blob = b"\x89PNG\r\n\x1a\nmissing"
        old = {"id": "old-photo", "fixture_code": "synthetic-png-v1", "mime_type": "image/png",
               "sha256": hashlib.sha256(blob).hexdigest(), "size_bytes": len(blob), "public_in_catalog": True}
        FakeAPI.galleries["space-1"] = [old]
        self.importer.state["listings"]["listing-1"] = {"id": "space-1", "photo_id": "old-photo"}
        owner = loader.API(["http://127.0.0.1:8080"], "owner-session")

        replacement = self.importer.ensure_photo_content(owner, "space-1", "listing-1", "old-photo", [old], public=True)
        self.assertEqual(replacement, "new-photo-1")
        self.assertEqual(self.importer.state["listings"]["listing-1"]["photo_id"], replacement)
        self.assertEqual([row["id"] for row in FakeAPI.galleries["space-1"]], [replacement])
        self.assertTrue(FakeAPI.galleries["space-1"][0]["public_in_catalog"])
        self.assertTrue(any(path == "/api/v1/spaces/space-1/cover" and body == {"photo_id": replacement}
                            for method, path, body, _ in FakeAPI.calls))

        second = self.importer.ensure_photo_content(owner, "space-1", "listing-1", replacement,
                                                   FakeAPI.galleries["space-1"], public=True)
        self.assertEqual(second, replacement)
        self.assertEqual(sum(method == "POST" and path.endswith("/gallery") for method, path, _, _ in FakeAPI.calls), 1)

    def test_shared_api_port_is_allowed_only_for_explicit_repair_mode(self):
        with self.assertRaisesRegex(RuntimeError, "shared persistent API"):
            loader.local_url("http://127.0.0.1:8080", pilot=False)
        self.assertEqual(loader.local_url("http://127.0.0.1:8080", pilot=False, allow_shared_api=True),
                         "http://127.0.0.1:8080")

    def test_private_listing_replacement_stays_unpublished_without_cover(self):
        blob = b"\x89PNG\r\n\x1a\nmissing"
        old = {"id": "old-private-photo", "fixture_code": "synthetic-png-v1", "mime_type": "image/png",
               "sha256": hashlib.sha256(blob).hexdigest(), "size_bytes": len(blob), "public_in_catalog": False}
        FakeAPI.galleries["space-private"] = [old]
        self.importer.state["listings"]["listing-private"] = {"id": "space-private", "photo_id": old["id"]}
        owner = loader.API(["http://127.0.0.1:8080"], "owner-session")

        replacement = self.importer.ensure_photo_content(owner, "space-private", "listing-private", old["id"],
                                                         [old], public=False)
        self.assertEqual(replacement, "new-photo-1")
        self.assertFalse(FakeAPI.galleries["space-private"][0]["public_in_catalog"])
        self.assertFalse(any(path.endswith("/cover") for _, path, _, _ in FakeAPI.calls))
        self.assertFalse(any(path.startswith("/api/v1/public/") for _, path, _, _ in FakeAPI.calls))


if __name__ == "__main__":
    unittest.main()
