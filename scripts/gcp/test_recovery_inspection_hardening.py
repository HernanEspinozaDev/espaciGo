import copy
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import recovery_inspection as r
from test_recovery_parent_proof import request_fixture
from test_recovery_inspection import SHA, BID, DIGEST, IMAGE, parent, report


UID = '22222222-3333-4444-8555-666666666666'


def request():
    return request_fixture(SHA, BID, DIGEST, IMAGE,
                          f'gs://{r.BUCKET}/source/recovery.tgz#17')


def canonical_job():
    return r.desired_job(request(), uid=UID)


class RecoveryInspectionHardeningTests(unittest.TestCase):
    def test_verify_job_rejects_unapproved_container_and_runtime_fields(self):
        mutations = (
            lambda j: j['spec']['template']['spec']['template']['spec']['containers'][0].update(foo='bar'),
            lambda j: j['spec']['template']['spec']['template']['spec']['containers'][0].update(name='override'),
            lambda j: j['spec']['template']['spec']['template']['spec']['containers'].append(
                {'name': 'sidecar', 'image': IMAGE}),
            lambda j: j['spec']['template']['spec']['template']['spec'].update(runtimeClassName='other'),
            lambda j: j['spec']['template']['spec']['template']['spec']['containers'][0]['env'].append(
                {'name': 'EXTRA', 'value': 'x'}),
            lambda j: j['spec']['template']['spec']['template']['spec']['containers'][0]['volumeMounts'].append(
                {'name': 'extra', 'mountPath': '/tmp'}),
            lambda j: j['spec']['template']['spec']['template']['spec']['volumes'].append(
                {'name': 'extra', 'emptyDir': {}}),
        )
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                job = copy.deepcopy(canonical_job())
                mutate(job)
                with self.assertRaises(ValueError):
                    r.verify_job(job, request())

    def test_validate_report_requires_nonempty_authority_and_repeatable_isolation(self):
        # The shared fixture implements the complete production wire contract.
        valid = report()
        r.validate_report(valid, [])

        no_authority = copy.deepcopy(valid)
        no_authority['authority'] = {}
        missing_isolation = copy.deepcopy(valid)
        del missing_isolation['transaction_isolation']
        nonrepeatable = copy.deepcopy(valid)
        nonrepeatable['transaction_isolation'] = 'serializable'
        for bad in (no_authority, missing_isolation, nonrepeatable):
            with self.subTest(report=bad):
                with self.assertRaises(ValueError):
                    r.validate_report(bad, [])

    def test_record_evidence_refuses_existing_local_path_and_symlink_parent(self):
        class Cloud:
            def __init__(self): self.calls = []; self.payload = ''
            def run(self, *args):
                self.calls.append(args)
                if args[:2] == ('storage', 'cp'):
                    self.payload = Path(args[2]).read_text()
                if args[:2] == ('storage', 'cat'):
                    return self.payload
                return ''
            def json(self, *args): return {'generation': '1'}

        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            cloud = Cloud()
            r.record_evidence(cloud, root, BID, 'same-path', {'first': True})
            call_count = len(cloud.calls)
            with self.assertRaises((ValueError, OSError)):
                r.record_evidence(cloud, root, BID, 'same-path', {'second': True})
            self.assertEqual(len(cloud.calls), call_count, 'duplicate local evidence must fail before cloud access')

        with tempfile.TemporaryDirectory() as d, tempfile.TemporaryDirectory() as outside:
            root = Path(d)
            recovery = root / '.local' / 'recovery'
            recovery.parent.mkdir(parents=True)
            recovery.symlink_to(Path(outside), target_is_directory=True)
            cloud = Cloud()
            with self.assertRaises((ValueError, OSError)):
                r.record_evidence(cloud, root, BID, 'linked-parent', {'x': 1})
            self.assertEqual(cloud.calls, [], 'symlink parent must fail before cloud access')

    def test_cloud_rejects_mutating_create_args_and_unsafe_build_submit(self):
        req = request()
        allowed = [x for x in r.create_args(req)[1:] if not x.startswith(('--project=', '--region='))]
        forged = allowed + ['--set-env-vars=EVIL=1']
        with patch.object(r.subprocess, 'run') as run:
            with self.assertRaises(ValueError):
                r.Cloud().run(*forged)
            run.assert_not_called()

        submissions = (
            ('builds', 'submit', '/tmp/source.tar.gz', '--config=/elsewhere/cloudbuild-migrator-image.yaml',
             '--service-account=' + r.DEPLOYER, '--substitutions=_COMMIT_SHA=' + SHA, '--async'),
            ('builds', 'submit', '/tmp/source.tar.gz', '--config=/elsewhere/cloudbuild-migrator-image.yaml',
             '--service-account=' + r.DEPLOYER, '--substitutions=_COMMIT_SHA=' + SHA + ',_UNEXPECTED=x', '--async'),
        )
        for args in submissions:
            with self.subTest(args=args), self.assertRaises(ValueError):
                r.Cloud().run(*args)

    def test_cloud_positive_allowlisted_commands_reach_subprocess(self):
        with patch.object(r.subprocess, 'run') as run:
            run.return_value.returncode = 0
            run.return_value.stdout = '{}'
            run.return_value.stderr = ''
            r.Cloud().run(*(x for x in r.create_args(request())[1:] if not x.startswith(('--project=', '--region='))))
            r.Cloud().run('builds', 'submit', '/tmp/source.tar.gz',
                          '--config=/workspace/cloudbuild-migrator-image.yaml',
                          '--gcs-source-staging-dir=gs://' + r.BUCKET + '/source',
                          '--service-account=' + r.DEPLOYER,
                          '--substitutions=_COMMIT_SHA=' + SHA, '--async', '--format=json')
            self.assertEqual(run.call_count, 2)

    def test_execute_records_cleanup_intent_before_job_delete(self):
        # Lightweight full-parent ordering check, with all cloud-facing behavior fake.
        class FakeCloud:
            def __init__(self): self.events = []
            def json(self, *args):
                self.events.append(('json', args))
                if args == ('builds', 'describe', BID):
                    return parent()
                if args[:3] == ('run', 'jobs', 'describe'):
                    if any(event[0] == 'run' and event[1][:3] == ('run', 'jobs', 'delete') for event in self.events):
                        raise r.RecoveryError(f'gcloud run jobs failed: ERROR: (gcloud.run.jobs.describe) Cannot find job [{args[3]}].')
                    return canonical_job()
                if args[:3] == ('run', 'jobs', 'list'):
                    return []
                raise AssertionError(args)
            def run(self, *args):
                self.events.append(('run', args))
                return ''

        fake = FakeCloud()
        records = []
        def record(cloud, root, bid, name, value):
            records.append(name)
            fake.events.append(('record', name))
            return {'uri': 'gs://evidence/' + name, 'generation': '1', 'sha256': DIGEST}

        with tempfile.TemporaryDirectory() as d, \
             patch.object(r, 'verify_archive', return_value={}), \
             patch.object(r, 'inspect_failed_release', return_value=dict(request()['failed_release_proof']['failed_release'],
                 observed_at=request()['failed_release_proof']['observed_at'])), \
             patch.object(r, 'assert_lock'), \
             patch.object(r, 'build_image', return_value=IMAGE), \
             patch.object(r, 'provision_job', return_value=canonical_job()), \
             patch.object(r, 'inspect_job', return_value={'read_only': True}), \
             patch.object(r, 'local_migrations', return_value=[]), \
             patch.object(r, 'record_evidence', side_effect=record):
            r.execute(fake, Path(d), SHA, BID, DIGEST)
        delete_index = next(i for i, event in enumerate(fake.events)
                            if event[0] == 'run' and event[1][:3] == ('run', 'jobs', 'delete'))
        self.assertIn('cleanup-intent', records)
        intent_index = fake.events.index(('record', 'cleanup-intent'))
        self.assertLess(intent_index, delete_index)


if __name__ == '__main__':
    unittest.main()
