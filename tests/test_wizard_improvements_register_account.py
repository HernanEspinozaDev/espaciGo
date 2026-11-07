import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

SCRIPT = Path(__file__).resolve().parents[1] / "scripts/wizard-improvements-register-account.py"
spec = importlib.util.spec_from_file_location("register_account", SCRIPT)
assert spec is not None and spec.loader is not None
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


TERMS = {"items": [
    {"type": "terminos", "id": "00000000-0000-4000-8000-000000000001", "document": {"type": "terminos", "sections": [{"title": "Synthetic terms", "body": "Test fixture only."}]}},
    {"type": "privacidad", "id": "00000000-0000-4000-8000-000000000002", "document": {"type": "privacidad", "sections": [{"title": "Synthetic privacy", "body": "Test fixture only."}]}},
]}


def make_session(post):
    return Mock(post=post, get=Mock(return_value=Mock(status_code=200, json=lambda: TERMS)))


class PolicyTests(unittest.TestCase):
    def policy(self):
        return {"target": {"project": "espacigo_wizard_20261106", "database": "espacigo_wizard_20261106", "host": "127.0.0.1", "port": 15436}, "authorization": {"maximum_new_accounts": 2, "synthetic_email_domain": "example.invalid", "account_inventory": [], "registration": "normal registration only", "verification": "normal verification only"}, "private_storage": ".local/testing/accounts"}

    def test_exact_target_and_authorization_allow(self):
        policy = self.policy()
        self.assertTrue(module.policy_allows(policy))
        policy["target"]["database"] = "databaseespacigo_wizard_20261106"
        self.assertFalse(module.policy_allows(policy))

    def test_prefixed_policy_storage_resolves_exact_private_path(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            policy = self.policy()
            policy["private_storage"] = ".local/testing/accounts"
            resolved = module._private_dir(root, policy["private_storage"])
            self.assertEqual(resolved, root / policy["private_storage"])
            self.assertEqual(resolved, root / ".local/testing/accounts")

    def test_202_is_accepted_and_receipt_is_updated(self):
        with tempfile.TemporaryDirectory() as temp:
            session = make_session(Mock(return_value=Mock(status_code=202, json=lambda: {"token_id": "synthetic-token"})))
            receipt = module.register_account(self.policy(), repo_root=Path(temp), session=session, confirm="espacigo_wizard_20261106")
            data = json.loads(receipt.read_text())
            self.assertEqual(data["status"], "awaiting_verification")

    def test_english_wire_types_are_rejected_before_post(self):
        with tempfile.TemporaryDirectory() as temp:
            english_terms = {"items": [
                {**TERMS["items"][0], "type": "terms", "document": {**TERMS["items"][0]["document"], "type": "terms"}},
                {**TERMS["items"][1], "type": "privacy", "document": {**TERMS["items"][1]["document"], "type": "privacy"}},
            ]}
            session = make_session(Mock())
            session.get.return_value = Mock(status_code=200, json=lambda: english_terms)
            with self.assertRaisesRegex(RuntimeError, "invalid terms response"):
                module.register_account(self.policy(), repo_root=Path(temp), session=session, confirm="espacigo_wizard_20261106")
            session.post.assert_not_called()

    def test_private_pending_receipt_is_durable_before_registration_post(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            storage = root / ".local/testing/accounts"
            fsync_seen = []
            def inspect_pending(url, *, json, timeout):
                receipt = storage / "wizard-improvements-pending.json"
                self.assertTrue(receipt.exists())
                self.assertEqual(receipt.stat().st_mode & 0o777, 0o600)
                self.assertEqual(storage.stat().st_mode & 0o777, 0o700)
                pending = __import__("json").loads(receipt.read_text())
                self.assertEqual(pending["status"], "pending")
                self.assertTrue(pending["password"])
                self.assertEqual(json["password"], pending["password"])
                self.assertEqual(json["email"], pending["email"])
                self.assertEqual(json["terms_version_ids"], [TERMS["items"][0]["id"], TERMS["items"][1]["id"]])
                self.assertTrue(fsync_seen, "pending receipt must be fsynced before POST")
                return Mock(status_code=202, json=lambda: {"token_id": "synthetic-token"})
            session = make_session(Mock(side_effect=inspect_pending))
            real_fsync = os.fsync
            def record_fsync(fd):
                fsync_seen.append(fd)
                return real_fsync(fd)
            with patch.object(module.os, "fsync", side_effect=record_fsync) as fsync:
                module.register_account(self.policy(), repo_root=root, session=session, confirm="espacigo_wizard_20261106")
            self.assertGreaterEqual(fsync.call_count, 1)
            self.assertEqual(session.post.call_count, 1)

    def test_urllib_response_body_is_read_inside_context_manager(self):
        class Response:
            status = 200
            closed = False
            def __enter__(self):
                return self
            def __exit__(self, *_):
                self.closed = True
            def read(self):
                if self.closed:
                    raise AssertionError("response read after context manager closed")
                return b'{"items": []}'
        response = Response()
        with patch.object(module.urllib.request, "urlopen", return_value=response):
            result = module._request(None, "GET", "http://127.0.0.1:18086/api/v1/auth/terms")
        self.assertEqual(result.status_code, 200)
        self.assertEqual(result.json(), {"items": []})
        self.assertTrue(response.closed)

    def test_pending_receipt_prevents_second_register(self):
        with tempfile.TemporaryDirectory() as temp:
            post = Mock(return_value=Mock(status_code=201, json=lambda: {"token_id": "t"}))
            session = make_session(post)
            result = module.register_account(self.policy(), repo_root=Path(temp), session=session, confirm="espacigo_wizard_20261106")
            self.assertTrue(result.exists())
            self.assertEqual(result.stat().st_mode & 0o777, 0o600)
            with self.assertRaises(module.PendingReceiptError):
                module.register_account(self.policy(), repo_root=Path(temp), session=session, confirm="espacigo_wizard_20261106")
            self.assertEqual(post.call_count, 1)


if __name__ == "__main__":
    unittest.main()
