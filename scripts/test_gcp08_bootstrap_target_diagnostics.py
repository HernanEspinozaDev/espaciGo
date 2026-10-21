import io
import re
import stat
import subprocess
import tempfile
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from types import SimpleNamespace

from gcp08_bootstrap_target_diagnostics import TARGETS, diagnose


ROOT = Path(__file__).resolve().parents[1]


class BootstrapTargetDiagnosticsTests(unittest.TestCase):
    def test_fixed_diagnostic_targets_match_the_bootstrap_plan_scope(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        start = workflow.index('if [[ "$POLICY_MODE" == bootstrap ]]')
        end = workflow.index("          fi", start)
        scope = workflow[start:end]
        planned = {
            option for option in re.findall(r"-target=([^'\s]+)", scope)
        }
        diagnostic = {target for target, _ in TARGETS}
        self.assertEqual(diagnostic, planned)
        self.assertEqual(len(TARGETS), 12)
        self.assertEqual(len({safe for _, safe in TARGETS}), 12)
        self.assertNotIn("pasteleriahijitos@gmail.com", " ".join(safe for _, safe in TARGETS))

        addresses = [target for target, _ in TARGETS]
        self.assertIn('google_cloud_run_v2_service.web["private"]', addresses)
        self.assertIn(
            'google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]',
            addresses,
        )
        self.assertNotEqual(
            addresses.index('google_cloud_run_v2_service.web["private"]'),
            addresses.index('google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]'),
        )

    def test_runs_one_private_plan_per_fixed_target_and_emits_sanitized_failure(self):
        marker = "private-project-resource-and-caller"
        observed = []

        def runner(command, **kwargs):
            observed.append((command, kwargs))
            if len(observed) == 2:
                kwargs["stdout"].write(
                    "Error: Error when reading Cloud Run service\n"
                    "googleapi: Error 403: Permission 'run.services.get' denied on "
                    f"{marker}, forbidden\n"
                    '  with google_cloud_run_v2_service.web["public"],\n'
                )
                return SimpleNamespace(returncode=1)
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            private = Path(root) / "private"
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(diagnose(root, Path(root) / "vars.json", private, runner=runner))

            self.assertEqual(stat.S_IMODE(private.stat().st_mode), 0o700)
            self.assertEqual(len(list(private.glob("target-*.log"))), len(TARGETS))
            self.assertTrue(all(stat.S_IMODE(path.stat().st_mode) == 0o600 for path in private.glob("*.log")))

        self.assertEqual(len(observed), len(TARGETS))
        for (command, kwargs), (target, _) in zip(observed, TARGETS, strict=True):
            self.assertEqual(command[:3], ["terraform", "plan", "-input=false"])
            self.assertIn(f"-target={target}", command)
            self.assertEqual(sum(arg.startswith("-target=") for arg in command), 1)
            self.assertNotIn("apply", command)
            self.assertFalse(any(key.startswith("TF_LOG") for key in kwargs["env"]))
            self.assertEqual(kwargs["stderr"], subprocess.STDOUT)

        lines = output.getvalue().splitlines()
        self.assertEqual(len(lines), len(TARGETS))
        self.assertIn(
            "GCP08_TERRAFORM_TARGET stage=bootstrap category=iam "
            "plan_target=google_cloud_run_v2_service.web[public] status=403 "
            "reason=permission_denied permission=run.services.get service=run.googleapis.com "
            "resource_type=run.googleapis.com/Service "
            "terraform_address=cloud_run_v2_service.web.public operation=get",
            lines[1],
        )
        self.assertNotIn(marker, output.getvalue())
        self.assertNotIn("Hernan", output.getvalue())

    def test_terraform_target_failures_do_not_stop_following_targets(self):
        observed = []

        def fail_all(_command, **kwargs):
            observed.append(kwargs)
            kwargs["stdout"].write("opaque private failure")
            return SimpleNamespace(returncode=1)

        with tempfile.TemporaryDirectory() as root:
            output = io.StringIO()
            with redirect_stdout(output):
                self.assertFalse(diagnose(root, "vars.json", root, runner=fail_all))
        self.assertEqual(len(observed), len(TARGETS))
        self.assertEqual(output.getvalue().count("category=unclassified"), len(TARGETS))


if __name__ == "__main__":
    unittest.main()
