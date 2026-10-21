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

    def test_bootstrap_cloud_sdk_is_available_only_after_direct_wif_auth(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        auth = workflow.index("- name: Authenticate bootstrap federated identity")
        deployer = workflow.index("- name: Authenticate restricted application deployer")
        setup = workflow.index("- name: Set up Cloud SDK for read-only bootstrap diagnostics")
        rollout = workflow.index("- name: Initialize canonical Terraform root and produce private plan")
        self.assertLess(auth, setup)
        self.assertLess(deployer, setup)
        self.assertLess(setup, rollout)
        setup_step = workflow[setup:rollout]
        self.assertIn("github.event_name == 'workflow_dispatch' && inputs.operation == 'initial-restore'", setup_step)
        self.assertIn("setup-gcloud@aa5489c8933f4cc7a4f7d45035b3b1440c9c10db # v3.0.1", setup_step)
        self.assertIn("project_id: espacigo-testprod", setup_step)

    def test_initial_restore_plan_is_targeted_and_skips_unrelated_refresh(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        start = workflow.index('if [[ "$POLICY_MODE" == bootstrap ]]')
        end = workflow.index("          fi", start)
        scope = workflow[start:end]
        expected_targets = {
            "google_cloud_run_v2_service.api[0]",
            'google_cloud_run_v2_service.web["public"]',
            'google_cloud_run_v2_service.web["private"]',
            'google_cloud_run_v2_service_iam_member.api_invoker["public"]',
            'google_cloud_run_v2_service_iam_member.api_invoker["private"]',
            'google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]',
            'google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]',
            'google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]',
            "google_cloud_run_v2_service_iam_member.public_web_invoker[0]",
            "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]",
            "google_secret_manager_secret_iam_member.v67_api_secret_access[0]",
            'google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]',
        }
        targets = set(re.findall(r"-target=([^'\s]+)", scope))
        self.assertNotIn("-refresh=false", scope)
        self.assertEqual(targets, expected_targets)
        self.assertIn('"${plan_scope[@]}"', workflow)

    def test_bootstrap_plan_failure_runs_per_target_diagnostics_before_fixed_api_probes(self):
        workflow = (ROOT / ".github/workflows/application-rollout.yml").read_text(encoding="utf-8")
        plan = workflow.index('terraform plan -input=false -lock-timeout=5m')
        bootstrap_branch = workflow.index('if [[ "$POLICY_MODE" == bootstrap ]]', plan)
        target_diagnostic = workflow.index("gcp08_bootstrap_target_diagnostics.py")
        fixed_probes = workflow.index("gcp08_bootstrap_cloud_run_probes.py")
        self.assertGreater(bootstrap_branch, plan)
        self.assertLess(target_diagnostic, fixed_probes)
        diagnostic_block = workflow[target_diagnostic:fixed_probes]
        self.assertIn("--terraform-dir", diagnostic_block)
        self.assertIn('--var-file "$GCP07D_TFVARS"', diagnostic_block)
        self.assertIn('"$RUNNER_TEMP/gcp07d/target-diagnostics"', diagnostic_block)
        self.assertNotIn("TF_LOG", diagnostic_block)
        self.assertNotIn("terraform apply", diagnostic_block)

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
