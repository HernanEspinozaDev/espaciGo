#!/usr/bin/env python3
import unittest

from check_local_qa_results import evaluate


class RequiredIntegrationGateTests(unittest.TestCase):
    def setUp(self):
        self.required = {("repo/internal/db", "TestMigration"), ("repo/internal/mail", "TestDelivery")}

    def test_required_integrations_must_pass(self):
        events = [
            {"Package": "repo/internal/db", "Test": "TestMigration", "Action": "run"},
            {"Package": "repo/internal/db", "Test": "TestMigration", "Action": "pass"},
            {"Package": "repo/internal/mail", "Test": "TestDelivery", "Action": "pass"},
        ]
        missing, skipped, failed, _ = evaluate(events, self.required)
        self.assertEqual((missing, skipped, failed), ([], [], []))

    def test_required_skip_fails_the_gate(self):
        events = [
            {"Package": "repo/internal/db", "Test": "TestMigration", "Action": "pass"},
            {"Package": "repo/internal/mail", "Test": "TestDelivery", "Action": "skip"},
        ]
        missing, skipped, failed, _ = evaluate(events, self.required)
        self.assertEqual(missing, [])
        self.assertEqual(skipped, [("repo/internal/mail", "TestDelivery")])
        self.assertEqual(failed, [])

    def test_missing_required_integration_fails_the_gate(self):
        events = [{"Package": "repo/internal/db", "Test": "TestMigration", "Action": "pass"}]
        missing, skipped, failed, _ = evaluate(events, self.required)
        self.assertEqual(missing, [("repo/internal/mail", "TestDelivery")])
        self.assertEqual(skipped, [])

    def test_optional_skip_is_reported_without_weakening_required_gate(self):
        optional = {"Package": "repo/internal/optional", "Test": "TestOptional", "Action": "skip"}
        events = [
            {"Package": "repo/internal/db", "Test": "TestMigration", "Action": "pass"},
            {"Package": "repo/internal/mail", "Test": "TestDelivery", "Action": "pass"},
            optional,
        ]
        missing, skipped, failed, skipped_all = evaluate(events, self.required)
        self.assertEqual((missing, skipped, failed), ([], [], []))
        self.assertEqual(skipped_all, {("repo/internal/optional", "TestOptional")})


if __name__ == "__main__":
    unittest.main()
