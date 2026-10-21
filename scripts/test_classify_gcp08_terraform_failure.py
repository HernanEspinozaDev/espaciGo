import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

from classify_gcp08_terraform_failure import classify


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
        self.assertEqual(classify("init", "could not find default credentials"), "provider")
        self.assertEqual(classify("init", "failed to acquire the state lock"), "backend")

    def test_policy_rejection_is_guard_and_unmatched_errors_stay_unclassified(self):
        self.assertEqual(classify("policy", "sensitive image and state data"), "guard")
        self.assertEqual(classify("plan", "opaque diagnostic with secret-looking values"), "unclassified")


if __name__ == "__main__":
    unittest.main()
