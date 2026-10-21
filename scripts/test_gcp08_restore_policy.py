import unittest

from verify_gcp08_restore_plan import PolicyError, expected_targets, validate_binding


class Gcp08RestorePolicyTests(unittest.TestCase):
    def test_targets_include_only_the_v67_secret_binding_as_a_possible_new_address(self):
        inputs = {"iap_evaluator_members": ["user:reviewer@example.test"]}
        targets = expected_targets(inputs)
        self.assertIn("google_secret_manager_secret_iam_member.api_v67_secret_access[0]", targets)
        self.assertNotIn("google_secret_manager_secret_iam_member.api_secret_access[\"database-password-v67\"]", targets)

    def test_runtime_secret_access_binding_is_exact(self):
        address = "google_secret_manager_secret_iam_member.api_v67_secret_access[0]"
        binding = {
            "project": "espacigo-testprod",
            "secret_id": "espacigo-test-database-password-v67",
            "role": "roles/secretmanager.secretAccessor",
            "member": "serviceAccount:espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com",
        }
        summary = validate_binding(address, binding, {})
        self.assertEqual(summary["resource"], "espacigo-test-database-password-v67")
        self.assertEqual(summary["role"], "roles/secretmanager.secretAccessor")

        for key, value in (("project", "other-project"),
                           ("secret_id", "espacigo-test-database-password-gcp06"),
                           ("role", "roles/secretmanager.admin"),
                           ("member", "allUsers")):
            changed = {**binding, key: value}
            with self.subTest(key=key), self.assertRaises(PolicyError):
                validate_binding(address, changed, {})


if __name__ == "__main__":
    unittest.main()
