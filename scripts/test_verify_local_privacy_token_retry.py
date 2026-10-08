#!/usr/bin/env python3
"""Focused tests for stale Mailpit action-token recovery."""
from __future__ import annotations

import importlib.util
import pathlib
import unittest
from unittest import mock


SCRIPT = pathlib.Path(__file__).with_name("verify-local-privacy-dispute.py")
SPEC = importlib.util.spec_from_file_location("local_privacy_verifier", SCRIPT)
verifier = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(verifier)


def message(message_id: str, token_id: str) -> dict:
    return {
        "ID": message_id,
        "Text": f"Token ID: {token_id}\nToken: test-token-{token_id[-4:]}\n",
    }


class StaleTokenRetryTests(unittest.TestCase):
    def test_invalid_consumed_token_is_discarded_and_fresh_mail_is_consumed(self):
        stale = message("mail-stale", "00000000-0000-4000-8000-000000000001")
        fresh = message("mail-fresh", "00000000-0000-4000-8000-000000000002")
        state = {"actors": {"host": {}}}
        wait_exclusions = []
        api_responses = [
            (422, {"error": {"code": "invalid_token"}}),
            (204, None),
        ]

        def fake_wait(email, excluded, purpose):
            wait_exclusions.append(set(excluded))
            return stale if len(wait_exclusions) == 1 else fresh

        def request_fresh():
            self.assertIn("mail-stale", state["actors"]["host"]["discarded_token_mail_ids"])
            return 204, None

        with mock.patch.dict(verifier.os.environ, {}, clear=False):
            verifier.os.environ.pop("LOCAL_PRIV189_TEST_CONSUME_TOKEN_FIRST", None)
            with mock.patch.object(verifier, "wait_mail", side_effect=fake_wait), \
                    mock.patch.object(verifier, "request", side_effect=api_responses) as request_mock, \
                    mock.patch.object(verifier, "save_state") as save_mock:
                retry_count = verifier.consume_mail_token(
                    state, "host", "local@example.test", set(), "recupera tu clave", "recovery",
                    "/api/v1/auth/password/recovery/consume",
                    lambda token_id, token: {"token_id": token_id, "token": token,
                                             "new_password": "synthetic-test-password",
                                             "confirm_password": "synthetic-test-password"},
                    request_fresh,
                )

        self.assertEqual(len(request_mock.call_args_list), 2)
        self.assertEqual(retry_count, 1)
        self.assertIn("mail-stale", wait_exclusions[1])
        self.assertIn("mail-stale", state["actors"]["host"]["discarded_token_mail_ids"])
        self.assertNotIn("recovery", state["actors"]["host"].get("token_recovery", {}))
        self.assertGreaterEqual(save_mock.call_count, 3)

    def test_invalid_token_retries_are_bounded(self):
        state = {"actors": {"host": {}}}
        messages = [
            message(f"mail-{i}", f"00000000-0000-4000-8000-{i:012d}")
            for i in range(verifier.MAX_STALE_TOKEN_RETRIES + 1)
        ]
        waits = []
        resend_count = 0

        def fake_wait(email, excluded, purpose):
            waits.append(set(excluded))
            return messages[len(waits) - 1]

        def request_fresh():
            nonlocal resend_count
            resend_count += 1
            return 204, None

        with mock.patch.object(verifier, "wait_mail", side_effect=fake_wait), \
                mock.patch.object(verifier, "request", return_value=(422, {"error": {"code": "invalid_token"}})) as request_mock, \
                mock.patch.object(verifier, "save_state"):
            with self.assertRaisesRegex(RuntimeError, "bounded retries"):
                verifier.consume_mail_token(
                    state, "host", "local@example.test", set(), "verifica tu correo", "verification",
                    "/api/v1/auth/verification",
                    lambda token_id, token: {"token_id": token_id, "token": token},
                    request_fresh,
                )

        self.assertEqual(request_mock.call_count, verifier.MAX_STALE_TOKEN_RETRIES + 1)
        self.assertEqual(resend_count, verifier.MAX_STALE_TOKEN_RETRIES)
        self.assertEqual(len(waits), verifier.MAX_STALE_TOKEN_RETRIES + 1)
        for index in range(1, len(waits)):
            self.assertIn(messages[index - 1]["ID"], waits[index])


if __name__ == "__main__":
    unittest.main()
