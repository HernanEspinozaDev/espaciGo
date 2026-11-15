"""Offline own-Job cleanup; real spec validation, no SDK/network calls."""
import copy
import json
import unittest
from unittest.mock import Mock, call

import test_main_release as base
import release_jobs as jobs
from verify_web_backend import GuardError


class ReleaseJobCleanupTests(unittest.TestCase):
    def setUp(self):
        manifest, *_ = base.fixture()
        self.request = jobs.request(manifest, base.BUILD, 'plan')
        args = jobs.create_args(self.request)
        self.name = args[4]
        desired_args = next(a for a in args if a.startswith('--args=')).removeprefix('--args=').split(',')
        env = dict(item.split('=', 1) for item in next(a for a in args if a.startswith('--set-env-vars=')).removeprefix('--set-env-vars=^|^').split('|'))
        self.job = {
            'metadata': {'name': self.name, 'namespace': '447373903153', 'uid': '22222222-3333-4444-8555-666666666666'},
            'spec': {'template': {
                'metadata': {
                    'labels': {'source-commit': manifest['source_sha'], 'cloud-build-id': base.BUILD},
                    'annotations': {
                        'run.googleapis.com/network-interfaces': json.dumps([{'network': 'espacigo-testprod', 'subnetwork': 'espacigo-testprod-api-serverless'}]),
                        'run.googleapis.com/vpc-access-egress': 'private-ranges-only',
                    },
                },
                'spec': {'taskCount': 1, 'parallelism': 1, 'template': {'spec': {
                    'maxRetries': 0, 'timeoutSeconds': '600', 'serviceAccountName': jobs.SERVICE_ACCOUNT,
                    'containers': [{
                        'image': manifest['images']['migrator'], 'command': ['/app/dbmigrate'], 'args': desired_args,
                        'resources': {'limits': {'cpu': '1', 'memory': '512Mi'}},
                        'env': [{'name': key, 'value': value} for key, value in env.items()],
                        'volumeMounts': [{'name': 'migration', 'mountPath': '/var/secrets/migration'}],
                    }],
                    'volumes': [{'name': 'migration', 'secret': {
                        'secretName': 'espacigo-testprod-cloudsql-migrator-password',
                        'items': [{'key': '1', 'path': 'password'}],
                    }}],
                }}},
            }},
        }
        jobs.verify_job(self.job, self.request)
        self.cloud = Mock()

    def missing(self, name=None):
        return GuardError(f'gcloud run jobs failed: ERROR: (gcloud.run.jobs.describe) Cannot find job [{name or self.name}].')

    def test_cleanup_reads_exact_own_name_after_delete(self):
        self.cloud.json.side_effect = [self.job, self.missing()]
        jobs.remove_job(self.cloud, self.job, self.request)
        self.assertEqual(self.cloud.json.call_args_list, [
            call('run', 'jobs', 'describe', self.name),
            call('run', 'jobs', 'describe', self.name),
        ])
        self.cloud.run.assert_called_once_with('run', 'jobs', 'delete', self.name)

    def test_cleanup_propagates_inconclusive_reads_without_retry(self):
        errors = (
            GuardError('permission denied'), self.missing('different-job'),
            GuardError('404 not found'), TimeoutError('deadline exceeded'),
            OSError('transport unavailable'),
        )
        for error in errors:
            with self.subTest(error=repr(error)):
                cloud = Mock()
                cloud.json.side_effect = [self.job, error]
                with self.assertRaises(type(error)) as raised:
                    jobs.remove_job(cloud, self.job, self.request)
                self.assertIs(raised.exception, error)
                cloud.run.assert_called_once_with('run', 'jobs', 'delete', self.name)
                self.assertEqual(cloud.json.call_args_list, [
                    call('run', 'jobs', 'describe', self.name),
                    call('run', 'jobs', 'describe', self.name),
                ])

    def test_cleanup_refuses_surviving_or_replaced_name_without_retry(self):
        for uid in (self.job['metadata']['uid'], '33333333-4444-4555-8666-777777777777'):
            with self.subTest(uid=uid):
                remaining = copy.deepcopy(self.job)
                remaining['metadata']['uid'] = uid
                cloud = Mock()
                cloud.json.side_effect = [self.job, remaining]
                with self.assertRaisesRegex(GuardError, 'still present'):
                    jobs.remove_job(cloud, self.job, self.request)
                cloud.run.assert_called_once_with('run', 'jobs', 'delete', self.name)
                self.assertEqual(cloud.json.call_count, 2)

    def test_cleanup_refuses_replaced_job_before_delete(self):
        replacement = copy.deepcopy(self.job)
        replacement['metadata']['uid'] = '33333333-4444-4555-8666-777777777777'
        self.cloud.json.return_value = replacement
        with self.assertRaisesRegex(GuardError, 'refusing cleanup of replaced Job'):
            jobs.remove_job(self.cloud, self.job, self.request)
        self.cloud.run.assert_not_called()
        self.cloud.json.assert_called_once_with('run', 'jobs', 'describe', self.name)

    def test_cleanup_refuses_changed_spec_before_delete(self):
        changed = copy.deepcopy(self.job)
        changed['spec']['template']['spec']['template']['spec']['containers'][0]['args'] = ['-release']
        self.cloud.json.return_value = changed
        with self.assertRaisesRegex(GuardError, 'Job image, args or resources differ'):
            jobs.remove_job(self.cloud, self.job, self.request)
        self.cloud.run.assert_not_called()
        self.cloud.json.assert_called_once_with('run', 'jobs', 'describe', self.name)
