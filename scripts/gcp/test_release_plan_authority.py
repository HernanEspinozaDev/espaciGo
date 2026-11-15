"""Offline regression: the real Job log reader must not drop V84 authority."""
import copy
import unittest
from unittest.mock import patch

from release_jobs import execute_job, request
from test_migration_execution import BUILD, UID
from test_migration_release_manifest import v84_release_fixture, v95_release_fixture
from migration_execution import check_execution_authority


class PlanCloud:
    def __init__(self, report):
        self.report = report
        self.calls = []
        self.name = 'espacigo-db-p-' + BUILD.replace('-', '')
        self.execution = self.name + '-abc12'
        self.task = {'synthetic': 'same-task'}

    def json(self, *args):
        self.calls.append(args)
        if args[:3] == ('run', 'jobs', 'execute'):
            return {'metadata': {'name': self.execution}}
        if args[:4] == ('run', 'jobs', 'executions', 'describe'):
            return {
                'metadata': {'namespace': '447373903153', 'labels': {'run.googleapis.com/jobUid': UID}},
                'spec': {'template': {'spec': self.task}},
                'status': {'conditions': [{'type': 'Completed', 'status': 'True'}], 'succeededCount': 1},
            }
        if args[:2] == ('logging', 'read'):
            return [{
                'jsonPayload': self.report,
                'resource': {'labels': {'project_id': 'espacigo-testprod', 'location': 'southamerica-west1', 'job_name': self.name}},
                'labels': {'run.googleapis.com/execution_name': self.execution},
            }]
        raise AssertionError(('unexpected operation', args))

    def job(self):
        return {'metadata': {'name': self.name, 'uid': UID}, 'spec': {'template': {'spec': {'template': {'spec': self.task}}}}}


class ReleasePlanAuthorityTests(unittest.TestCase):
    def test_apply_time_authority_is_required_and_matches_preflight(self):
        _, preflight = v84_release_fixture()
        with self.assertRaises(ValueError):
            check_execution_authority(preflight, {})
        report = {'ddl_authority': copy.deepcopy(preflight['ddl_authority'])}
        check_execution_authority(preflight, report)
        report['ddl_authority']['catalog']['database_create'] = True
        with self.assertRaises(ValueError):
            check_execution_authority(preflight, report)

    def execute(self, report):
        manifest, _ = v84_release_fixture()
        cloud = PlanCloud(report)
        records = []
        # No sleeps/cloud access: a discarded report fails immediately.
        with patch('release_jobs.time.sleep', side_effect=AssertionError('V84 plan report was discarded')):
            result = execute_job(cloud, cloud.job(), request(manifest, BUILD, 'plan'), lambda *args: records.append(args))
        self.assertEqual(sum(args[:2] == ('logging', 'read') for args in cloud.calls), 1)
        self.assertFalse(any('backups' in args or 'deploy' in args for args in cloud.calls))
        return result

    def test_real_job_reader_preserves_v84_authority(self):
        _, report = v84_release_fixture()
        self.assertEqual(self.execute(report), report)

    def test_real_job_reader_rejects_invalid_authority(self):
        _, report = v84_release_fixture()
        report = copy.deepcopy(report)
        report['ddl_authority']['public_create'] = False
        with self.assertRaisesRegex(ValueError, 'privilege'):
            self.execute(report)

    def execute_tail(self, report):
        manifest, _ = v95_release_fixture()
        cloud = PlanCloud(report)
        with patch('release_jobs.time.sleep', side_effect=AssertionError('V95/V96 plan report was discarded')):
            result = execute_job(cloud, cloud.job(), request(manifest, BUILD, 'plan'), lambda *args: None)
        self.assertEqual(sum(args[:2] == ('logging', 'read') for args in cloud.calls), 1)
        return result

    def test_real_job_reader_preserves_v95_v96_identity_tail_authority(self):
        _, report = v95_release_fixture()
        self.assertEqual(self.execute_tail(report), report)

    def test_real_job_reader_surfaces_malformed_plan_to_closed_validation(self):
        _, report = v95_release_fixture()
        report = copy.deepcopy(report)
        report['unexpected'] = True
        with self.assertRaisesRegex(ValueError, 'invalid migration plan contract'):
            self.execute_tail(report)


if __name__ == '__main__':
    unittest.main()
