import unittest
from migration_job_plan import REGISTRY, proposal

IMAGE=REGISTRY+'/espacigo-db-migrator@sha256:'+'a'*64
SHA='b'*40


class MigrationJobProposalTests(unittest.TestCase):
    def test_only_plans_a_fixed_private_target_with_pinned_secret(self):
        report=proposal(IMAGE,SHA,'1')
        self.assertFalse(report['executed'])
        command=report['proposed_job_create_argv']
        self.assertIn('--args=-dir,/app/db/migrations,-plan-only',command)
        self.assertIn('--max-retries=0',command)
        self.assertIn('--parallelism=1',command)
        self.assertIn('--vpc-egress=private-ranges-only',command)
        self.assertFalse(any(x.startswith('--execute') for x in command))
        self.assertFalse(any('postgres-admin-password' in x for x in command))

    def test_rejects_mutable_images_other_projects_invalid_sha_and_secret(self):
        for image,sha,version in [
            (REGISTRY+'/espacigo-db-migrator:latest',SHA,'1'),
            (IMAGE.replace('espacigo-testprod','another-project'),SHA,'1'),
            (IMAGE,SHA[:7],'1'),(IMAGE,SHA,'latest'),(IMAGE,SHA,'0'),
        ]:
            with self.assertRaises(ValueError):
                proposal(image,sha,version)


if __name__=='__main__':
    unittest.main()
