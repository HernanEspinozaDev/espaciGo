from pathlib import Path
import unittest


ROOT = Path(__file__).resolve().parents[1]


class Gcp08BootstrapWorkflowTests(unittest.TestCase):
    def test_bootstrap_auth_is_direct_and_only_manual_initial_restore_uses_it(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        start = workflow.index("      - name: Authenticate bootstrap federated identity")
        end = workflow.index("      - name: Authenticate restricted application deployer", start)
        bootstrap_step = workflow[start:end]
        self.assertIn("if: github.event_name == 'workflow_dispatch' && inputs.operation == 'initial-restore'", bootstrap_step)
        self.assertIn("workloadIdentityPools/espacigo-gcp08-bootstrap/providers/espacigo-initial-restore", bootstrap_step)
        self.assertIn("create_credentials_file: true", bootstrap_step)
        self.assertNotIn("service_account:", bootstrap_step)
        self.assertNotIn("token_format:", bootstrap_step)

    def test_normal_and_automatic_rollouts_keep_the_restricted_deployer(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        start = workflow.index("      - name: Authenticate restricted application deployer")
        end = workflow.index("      - name:", start + 20)
        deployer_step = workflow[start:end]
        self.assertIn("if: github.event_name == 'workflow_run' || inputs.operation != 'initial-restore'", deployer_step)
        self.assertIn("service_account: espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com", deployer_step)
        self.assertIn("workloadIdentityPools/espacigo-testprod-github/providers/espacigo-main", deployer_step)

    def test_application_quality_runs_bootstrap_policy_tests_without_remote_backend(self):
        workflow = (ROOT / ".github/workflows/application-quality.yml").read_text(encoding="utf-8")
        self.assertIn("infra/terraform/environments/testprod/gcp08-bootstrap", workflow)
        self.assertIn("terraform init -backend=false -input=false -lockfile=readonly", workflow)
        self.assertIn("terraform test -no-color", workflow)


if __name__ == "__main__":
    unittest.main()
