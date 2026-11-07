import json
from pathlib import Path
import unittest
from bootstrap_release_identity import commands,MIGRATOR,BUILDER

ROOT=Path(__file__).resolve().parents[2]


class ReleaseIdentityTests(unittest.TestCase):
    def test_executor_can_run_but_cannot_mutate_jobs_or_read_database_passwords(self):
        p=set(json.loads((ROOT/'infra/releases/main-executor-role.json').read_text())['includedPermissions'])
        self.assertIn('run.jobs.run',p)
        self.assertFalse(p&{'run.jobs.create','run.jobs.update','run.jobs.runWithOverrides','secretmanager.versions.access','cloudsql.instances.update','cloudsql.users.update','iam.serviceAccounts.getAccessToken'})
    def test_configurator_cannot_execute_or_update_jobs(self):
        p=set(json.loads((ROOT/'infra/releases/job-provisioner-role.json').read_text())['includedPermissions'])
        self.assertIn('run.jobs.create',p)
        self.assertFalse(p&{'run.jobs.run','run.jobs.update','run.jobs.runWithOverrides','secretmanager.versions.access'})
        targets=[c[3] for c in commands() if c[:3]==['iam','service-accounts','add-iam-policy-binding']]
        self.assertEqual(targets,[BUILDER,MIGRATOR])
    def test_source_reader_is_conditional_and_no_secret_policy_is_changed(self):
        all_args=[x for c in commands() for x in c]
        self.assertFalse(any('secretmanager' in x or 'secret-accessor' in x or 'TokenCreator' in x for x in all_args))
        self.assertTrue(any('/objects/source/' in x and '/objects/releases/' in x for x in all_args))


if __name__=='__main__':unittest.main()
