import copy
from datetime import datetime,timezone
import unittest
from backup_readiness import check

NOW=datetime(2026,11,7,1,0,tzinfo=timezone.utc)


def snapshot():
    return {'captured_at':NOW.isoformat(),'release_lock_acquired_at':'2026-11-07T00:40:00Z','instance':{'project':'espacigo-testprod','name':'espacigo-testprod-db','connectionName':'espacigo-testprod:southamerica-west1:espacigo-testprod-db','region':'southamerica-west1','state':'RUNNABLE','databaseVersion':'POSTGRES_18','settings':{'backupConfiguration':{'enabled':True,'pointInTimeRecoveryEnabled':False,'backupRetentionSettings':{'retentionUnit':'COUNT','retainedBackups':7}}}},'backup':{'id':'123456','instance':'espacigo-testprod-db','status':'SUCCESSFUL','type':'AUTOMATED','startTime':'2026-11-07T00:45:00Z','endTime':'2026-11-07T00:46:00Z','selfLink':'https://sqladmin.googleapis.com/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-testprod-db/backupRuns/123456'}}


class BackupReadinessTests(unittest.TestCase):
    def test_initial_noop_policy_accepts_recent_success_without_claiming_pitr(self):
        result=check(snapshot(),now=NOW)
        self.assertFalse(result['pitr_enabled']);self.assertEqual(result['retained_copies'],7)

    def test_pending_sql_requires_successful_ondemand_backup_after_lock(self):
        s=snapshot()
        with self.assertRaises(ValueError):check(s,True,NOW)
        s['backup']['type']='ON_DEMAND'
        self.assertTrue(check(s,True,NOW)['pending_sql'])
        s['release_lock_acquired_at']='2026-11-07T00:50:00Z'
        with self.assertRaises(ValueError):check(s,True,NOW)

    def test_wrong_instance_project_status_and_disabled_backup_fail_closed(self):
        for change in [lambda s:s['instance'].update(project='other'),lambda s:s['backup'].update(instance='other'),lambda s:s['backup'].update(selfLink='https://elsewhere/backup'),lambda s:s['backup'].update(status='RUNNING'),lambda s:s['backup'].update(error={'message':'failure'}),lambda s:s['instance']['settings']['backupConfiguration'].update(enabled=False),lambda s:s['instance']['settings']['backupConfiguration']['backupRetentionSettings'].update(retainedBackups=1)]:
            s=snapshot();change(s)
            with self.assertRaises(ValueError):check(s,now=NOW)

    def test_stale_capture_old_backup_and_invalid_chronology_fail_closed(self):
        for change in [lambda s:s.update(captured_at='2026-11-07T00:40:00Z'),lambda s:s['backup'].update(startTime='2026-11-05T00:00:00Z'),lambda s:s['backup'].update(startTime='2026-11-07T00:59:00Z'),lambda s:s['backup'].update(endTime='2026-11-07T02:00:00Z')]:
            s=snapshot();change(s)
            with self.assertRaises(ValueError):check(s,now=NOW)


if __name__=='__main__':unittest.main()
