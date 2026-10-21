from pathlib import Path
import re
import unittest


ROOT = Path(__file__).resolve().parents[1]
BOOTSTRAP_ACTOR_ID = "176093371"


def bootstrap_actor_is_allowed(actor_id: str) -> bool:
    terraform = (ROOT / "infra/terraform/environments/testprod/gcp08-bootstrap/main.tf").read_text(encoding="utf-8")
    variables = (ROOT / "infra/terraform/environments/testprod/gcp08-bootstrap/variables.tf").read_text(encoding="utf-8")
    condition = re.search(r'attribute_condition\s*=\s*"([^"]+)"', terraform)
    pinned_actor = re.search(r'variable "github_bootstrap_actor_id" \{.*?default\s*=\s*"([0-9]+)"', variables, re.S)
    if condition is None or pinned_actor is None or "assertion.actor_id == '${var.github_bootstrap_actor_id}'" not in condition.group(1):
        return False
    return actor_id == pinned_actor.group(1)


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

    def test_wif_allows_only_the_pinned_bootstrap_actor(self):
        terraform = (ROOT / "infra/terraform/environments/testprod/gcp08-bootstrap/main.tf").read_text(encoding="utf-8")
        self.assertIn('"attribute.actor_id"            = "assertion.actor_id"', terraform)
        self.assertTrue(bootstrap_actor_is_allowed(BOOTSTRAP_ACTOR_ID))
        self.assertFalse(bootstrap_actor_is_allowed("152030325"))
        self.assertFalse(bootstrap_actor_is_allowed(""))

    def test_initial_restore_job_has_actor_guard_before_bootstrap_auth(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        rollout_job_match = re.search(r"(?ms)^  rollout:\n(.*?)(?=^  [a-z][a-z_-]*:\n|\Z)", workflow)
        self.assertIsNotNone(rollout_job_match)
        rollout_job = rollout_job_match.group(1)
        guard = "github.actor_id == '176093371'"
        bootstrap_auth = workflow.index("      - name: Authenticate bootstrap federated identity")
        self.assertIn(guard, rollout_job)
        self.assertLess(workflow.index(guard), bootstrap_auth)

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
