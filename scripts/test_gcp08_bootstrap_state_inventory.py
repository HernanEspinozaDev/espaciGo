import io
import os
import stat
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace

from gcp08_bootstrap_state_inventory import RESOURCE_TYPES, diagnose, summarize_state_addresses


class BootstrapStateInventoryTests(unittest.TestCase):
    def test_summary_counts_only_allowlisted_types_and_never_returns_addresses(self):
        marker = 'user:private@example.test'
        counts = summarize_state_addresses([
            "google_cloud_run_v2_service.api[0]",
            'google_cloud_run_v2_service.web["private"]',
            'google_cloud_run_v2_service_iam_member.private_iap_invoker[0]',
            "data.google_sql_database_instance.v67_target[0]",
            f'google_iap_web_cloud_run_service_iam_member.private_evaluators["{marker}"]',
            "google_secret_manager_secret.runtime[\"database-password\"]",
            42,
        ])
        self.assertEqual(counts, {
            "run_services": 2,
            "run_service_iam": 1,
            "iap_service_iam": 1,
            "service_account_iam": 0,
            "sql_instance": 1,
            "sql_database": 0,
        })
        self.assertEqual(tuple(counts), tuple(label for label, _ in RESOURCE_TYPES))
        self.assertNotIn(marker, str(counts))

    def test_state_list_output_is_private_and_only_sanitized_counts_are_printed(self):
        marker = 'user:private@example.test'

        def runner(command, **kwargs):
            self.assertEqual(command, ["terraform", "state", "list"])
            kwargs["stdout"].write(
                'google_cloud_run_v2_service.web["private"]\n'
                f'google_iap_web_cloud_run_service_iam_member.private["{marker}"]\n'
            )
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            private = Path(root) / "private"
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(diagnose(root, private, runner=runner))
            log = private / "state-list.log"
            self.assertEqual(stat.S_IMODE(private.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE(log.stat().st_mode), 0o600)
        self.assertIn("run_services=1", output.getvalue())
        self.assertIn("iap_service_iam=1", output.getvalue())
        self.assertNotIn(marker, output.getvalue())

    def test_state_list_failure_prints_no_provider_or_backend_output(self):
        marker = "private-bucket-and-error-details"

        def runner(_command, **kwargs):
            kwargs["stdout"].write(marker)
            return SimpleNamespace(returncode=1)

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(diagnose(root, root, runner=runner))
        self.assertEqual(output.getvalue().strip(),
                         "GCP08_STATE_INVENTORY stage=bootstrap category=unavailable")
        self.assertNotIn(marker, output.getvalue())


if __name__ == "__main__":
    unittest.main()
