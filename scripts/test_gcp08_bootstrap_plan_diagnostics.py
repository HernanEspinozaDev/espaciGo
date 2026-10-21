import re
import subprocess
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import Mock

from gcp08_bootstrap_plan_diagnostics import BOOTSTRAP_TARGETS, run_target


ROOT = Path(__file__).resolve().parents[1]


class BootstrapPlanDiagnosticsTests(unittest.TestCase):
    def test_fixed_targets_match_the_workflow_scope_in_order(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        start = workflow.index('if [[ "$POLICY_MODE" == bootstrap ]]')
        end = workflow.index("          fi", start)
        workflow_targets = re.findall(r"-target=([^'\s]+)", workflow[start:end])
        self.assertEqual([address for _, address in BOOTSTRAP_TARGETS], workflow_targets)
        self.assertEqual(len(BOOTSTRAP_TARGETS), 12)
        self.assertEqual(len({enum for enum, _ in BOOTSTRAP_TARGETS}), 12)

    def test_failed_plan_emits_only_fixed_enum_and_safe_classifier_fields(self):
        marker = "private-resource-and-request-987654"
        runner = Mock(return_value=SimpleNamespace(returncode=1))

        def execute(command, **kwargs):
            kwargs["stdout"].write(
                f"Error 403: Permission 'run.services.getIamPolicy' denied on {marker}\n"
                "with google_cloud_run_v2_service.api[0],\n"
            )
            return SimpleNamespace(returncode=1)

        runner.side_effect = execute
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as work:
            from contextlib import redirect_stdout
            from io import StringIO
            output = StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_target(BOOTSTRAP_TARGETS[0][0], "vars.tfvars", root, work, runner))
        emitted = output.getvalue()
        self.assertIn("target=cloud_run_v2_service.api", emitted)
        self.assertIn("category=iam", emitted)
        self.assertIn("permission=run.services.getIamPolicy", emitted)
        self.assertIn("address=cloud_run_v2_service.api", emitted)
        self.assertNotIn(marker, emitted)
        self.assertNotIn("denied on", emitted)
        command = runner.call_args.args[0]
        self.assertEqual(command[1], "plan")
        self.assertEqual(sum(arg.startswith("-target=") for arg in command), 1)
        self.assertTrue(any(arg.startswith("-out=") for arg in command))
        self.assertNotIn("apply", command)

    def test_success_emits_only_enum_and_success_category(self):
        runner = Mock(return_value=SimpleNamespace(returncode=0))
        from contextlib import redirect_stdout
        from io import StringIO
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as work:
            output = StringIO()
            with redirect_stdout(output):
                self.assertTrue(run_target(BOOTSTRAP_TARGETS[1][0], "vars.tfvars", root, work, runner))
        self.assertEqual(
            output.getvalue().strip(),
            "GCP08_TERRAFORM_TARGET stage=plan target=cloud_run_v2_service.web.public category=success",
        )

    def test_unknown_enum_is_rejected_before_execution(self):
        runner = Mock()
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as work:
            with self.assertRaises(ValueError):
                run_target("google_project_iam_member.anything", "vars.tfvars", root, work, runner)
        runner.assert_not_called()

    def test_timeout_is_safe_and_private_directory_is_cleaned(self):
        def timeout(*_args, **_kwargs):
            raise subprocess.TimeoutExpired("terraform", 120)

        from contextlib import redirect_stdout
        from io import StringIO
        with tempfile.TemporaryDirectory() as root, tempfile.TemporaryDirectory() as work:
            output = StringIO()
            with redirect_stdout(output):
                self.assertFalse(run_target(BOOTSTRAP_TARGETS[0][0], "vars.tfvars", root, work, timeout))
            self.assertEqual(list(Path(root).iterdir()), [])
        self.assertIn("category=unclassified", output.getvalue())


if __name__ == "__main__":
    unittest.main()
