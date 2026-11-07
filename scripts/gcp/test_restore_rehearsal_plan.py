import copy
import unittest
from migration_job_plan import PROJECT,REGISTRY
from restore_rehearsal_plan import JOB,LABELS,SOURCE,TARGET,assert_owned_target,proposal

IMAGE=REGISTRY+'/espacigo-db-migrator@sha256:'+'a'*64


class RestoreRehearsalTests(unittest.TestCase):
    def test_commands_target_only_the_new_private_recovery_instance(self):
        r=proposal('1793940180000',IMAGE,'b'*40,'1')
        self.assertFalse(r['executed'])
        self.assertNotEqual(SOURCE,TARGET)
        self.assertIn('--restore-instance='+TARGET,r['restore_argv'])
        self.assertIn('--backup-instance='+SOURCE,r['restore_argv'])
        self.assertIn('--no-assign-ip',r['create_argv'])
        self.assertIn('--no-backup',r['create_argv'])
        self.assertEqual(r['plan_job_create_argv'][4],JOB)
        self.assertIn('--args=-dir,/app/db/migrations,-plan-only',r['plan_job_create_argv'])
        self.assertNotIn(SOURCE,r['cleanup_instance_argv'])
        env=next(x for x in r['plan_job_create_argv'] if x.startswith('--set-env-vars='))
        self.assertIn('CLOUD_SQL_INSTANCE_CONNECTION_NAME='+PROJECT+':southamerica-west1:'+TARGET,env)
        self.assertNotIn('CLOUD_SQL_INSTANCE_CONNECTION_NAME='+PROJECT+':southamerica-west1:'+SOURCE,env)

    def test_cleanup_and_restore_guard_rejects_existing_source_or_unowned_public_target(self):
        fixture={'name':TARGET,'project':PROJECT,'settings':{'userLabels':LABELS.copy(),'ipConfiguration':{'ipv4Enabled':False,'privateNetwork':'projects/espacigo-testprod/global/networks/espacigo-testprod'}}}
        assert_owned_target(fixture)
        for mutate in [lambda m:m.update(name=SOURCE),lambda m:m.update(project='other'),lambda m:m['settings']['userLabels'].update(owner='another-task'),lambda m:m['settings']['ipConfiguration'].update(ipv4Enabled=True)]:
            m=copy.deepcopy(fixture);mutate(m)
            with self.assertRaises(ValueError):assert_owned_target(m)
        with self.assertRaises(ValueError):proposal('latest',IMAGE,'b'*40,'1')


if __name__=='__main__':unittest.main()
