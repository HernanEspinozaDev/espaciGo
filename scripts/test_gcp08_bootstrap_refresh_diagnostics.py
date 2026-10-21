import io
import os
import stat
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace

from gcp08_bootstrap_refresh_diagnostics import diagnose
from gcp08_bootstrap_target_diagnostics import TARGETS


class BootstrapRefreshDiagnosticsTests(unittest.TestCase):
    def test_runs_one_refresh_free_targeted_plan_and_keeps_raw_output_private(self):
        seen = []

        def runner(command, **kwargs):
            seen.append((command, kwargs))
            kwargs["stdout"].write("No changes. Your infrastructure matches the configuration.")
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            private = Path(root) / "private"
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertTrue(diagnose(root, Path(root) / "vars.json", private, runner=runner))
            log_path = private / "refresh-disabled.log"
            self.assertEqual(stat.S_IMODE(private.stat().st_mode), 0o700)
            self.assertEqual(stat.S_IMODE(log_path.stat().st_mode), 0o600)
            self.assertNotIn("No changes", output.getvalue())

        self.assertEqual(len(seen), 1)
        command, kwargs = seen[0]
        self.assertEqual(command[:5], ["terraform", "plan", "-input=false", "-lock-timeout=5m", "-refresh=false"])
        self.assertEqual(sum(arg.startswith("-target=") for arg in command), len(TARGETS))
        self.assertEqual(kwargs["stderr"], subprocess.STDOUT)
        self.assertFalse(any(key.startswith("TF_LOG") for key in kwargs["env"]))
        self.assertIn("category=success refresh=disabled", output.getvalue())

    def test_failure_is_sanitized_and_does_not_fail_the_outer_workflow(self):
        marker = "private-resource-value"

        def runner(_command, **kwargs):
            kwargs["stdout"].write(
                f"Error 403: Permission 'run.services.get' denied on {marker}"
            )
            return SimpleNamespace(returncode=1)

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(diagnose(root, "vars.json", root, runner=runner))
        self.assertIn("category=iam refresh=disabled", output.getvalue())
        self.assertIn("permission=run.services.get", output.getvalue())
        self.assertNotIn(marker, output.getvalue())


if __name__ == "__main__":
    unittest.main()
