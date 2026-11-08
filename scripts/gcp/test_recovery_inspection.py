import copy
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import recovery_inspection as r
from test_recovery_parent_proof import request_fixture

SHA = 'a' * 40
BID = '11111111-2222-4333-8444-555555555555'
DIGEST = 'b' * 64
IMAGE = r.REGISTRY + '/espacigo-db-migrator@sha256:' + 'c' * 64


def report():
    return {'format': 'espacigo.recovery-inspection.v1', 'read_only': True,
            'transaction_isolation': 'repeatable read',
            'database': 'espacigo_dms_stage_20261020', 'server_version': '18.6',
            'session_user': 'espacigo_migrator_login', 'current_user': 'espacigo_schema_owner',
            'applied': [], 'pending': [], 'authority': {
                'public_schema_owner': 'pg_database_owner', 'usuario_owner': 'postgres',
                'roles': [dict(name=name, exists=True, superuser=False, create_role=False,
                               create_db=False, replication=False, bypass_rls=False,
                               login=name == 'espacigo_migrator_login', inherit=False)
                          for name in ('espacigo_migrator_login', 'espacigo_schema_owner',
                                       'espacigo_runtime', 'espacigo_document_verification_fn')],
                'memberships': [['espacigo_schema_owner', 'espacigo_migrator_login', False, False, True]],
                'effective_privileges': dict(session_public_usage=True, schema_owner_public_usage=True,
                                            schema_public_create=False, usuario_references_id=False,
                                            ledger_select=True, ledger_insert=False),
                'acl_metadata': []}}


def parent():
    return {'id': BID, 'status': 'WORKING', 'serviceAccount': r.DEPLOYER,
            'steps': [r.parent_step(SHA, BID, DIGEST)],
            'substitutions': {'_RECOVERY_INSPECTION_ONLY': 'true', '_COMMIT_SHA': SHA,
                              '_SOURCE_ARCHIVE_SHA256': DIGEST},
            'sourceProvenance': {'resolvedStorageSource': {'bucket': r.BUCKET, 'object': 'source/recovery.tgz', 'generation': '17'}}}


class RecoveryInspectionTests(unittest.TestCase):
    def test_invocation_rejects_apply_and_bad_identity_before_cloud(self):
        good = ['--sha=' + SHA, '--build-id=' + BID, '--source-archive-sha256=' + DIGEST]
        self.assertEqual(r.parse_invocation(good).sha, SHA)
        for flag in ['--execute', '--apply', '--mode=apply', '--unlock', '--unlock-preapply', '--backup', '--deploy']:
            with self.subTest(flag=flag), self.assertRaises((ValueError, SystemExit)):
                r.parse_invocation(good + [flag])

    def test_parent_requires_exact_manual_readonly_step_and_source(self):
        p = parent()
        self.assertEqual(r.parent_identity(p, SHA, BID, DIGEST), f'gs://{r.BUCKET}/source/recovery.tgz#17')
        for mutate in [lambda x: x.update(status='SUCCESS'), lambda x: x.update(buildTriggerId='normal'),
                       lambda x: x.update(serviceAccount='wrong'), lambda x: x['substitutions'].update(_RECOVERY_INSPECTION_ONLY='false'),
                       lambda x: x['steps'].append({'id': 'apply'}), lambda x: x['steps'][0].update(args=['-ceu', 'apply SQL'])]:
            bad = copy.deepcopy(p); mutate(bad)
            with self.assertRaises(ValueError): r.parent_identity(bad, SHA, BID, DIGEST)

    def test_report_identity_and_inventory_fail_closed(self):
        r.validate_report(report(), [])
        for key, value in [('read_only', False), ('database', 'other'), ('current_user', 'espacigo_migrator_login')]:
            bad = report(); bad[key] = value
            with self.assertRaises(ValueError): r.validate_report(bad, [])
        bad = report(); bad['applied'] = [{'version': 2, 'name': 'wrong', 'checksum': DIGEST}]
        with self.assertRaises(ValueError): r.validate_report(bad, [])
        for key, value in [('transaction_isolation', 'read committed'), ('authority', {})]:
            bad = report(); bad[key] = value
            with self.assertRaises(ValueError): r.validate_report(bad, [])

    def test_job_is_inspection_only_and_effective_configuration_exact(self):
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        args = r.create_args(req)
        self.assertIn('--args=-dir,/app/db/migrations,-recovery-inspection-only', args)
        self.assertNotIn('-release', ','.join(args))
        job = r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')
        r.verify_job(job, req)
        bad = copy.deepcopy(job); bad['spec']['template']['spec']['template']['spec']['containers'][0]['args'] = ['-release']
        with self.assertRaises(ValueError): r.verify_job(bad, req)
        for mutate in [lambda t: t['containers'][0].update(name='other'),
                       lambda t: t['containers'][0].update(workingDir='/evil'),
                       lambda t: t.update(securityContext={'runAsUser': 0})]:
            bad = copy.deepcopy(job); mutate(bad['spec']['template']['spec']['template']['spec'])
            with self.assertRaises(ValueError): r.verify_job(bad, req)
        for key, value in [('mode', 'apply'), ('source_sha', 'bad'), ('image', 'untrusted')]:
            bad = dict(req); bad[key] = value
            with self.assertRaises(ValueError): r.create_args(bad)

    def test_cloud_command_allowlist_cannot_unlock_apply_or_touch_services(self):
        cloud = r.Cloud()
        for args in [('storage', 'rm', r.LOCK), ('sql', 'backups', 'create'), ('run', 'deploy', 'api'),
                     ('run', 'jobs', 'execute', 'espacigo-db-r-owned'), ('run', 'jobs', 'delete', 'espacigo-db-r-owned'),
                     ('run', 'jobs', 'update', 'anything'), ('builds', 'submit', '.')]:
            with self.subTest(args=args), self.assertRaises(ValueError): cloud.run(*args)

    def test_full_parent_records_intent_runs_once_and_preserves_lock(self):
        class Fake:
            def __init__(self): self.calls = []; self.saved = {}; self.executions = 0
            def json(self, *args):
                self.calls.append(args)
                if args == ('builds', 'describe', BID): return parent()
                if args[:3] == ('run', 'jobs', 'describe'):
                    return r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')
                if args[:3] == ('run', 'jobs', 'list'):
                    return []
                if args[:3] == ('run', 'jobs', 'execute'):
                    self.executions += 1
                    self.assert_intent()
                    return {'metadata': {'name': r.job_name(BID) + '-abcde'}}
                if args[:4] == ('run', 'jobs', 'executions', 'describe'):
                    j = r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')
                    return {'metadata': {'name': r.job_name(BID) + '-abcde', 'namespace': r.PROJECT_NUMBER,
                                         'labels': {'run.googleapis.com/jobUid': j['metadata']['uid']}},
                            'spec': {'taskCount': 1, 'parallelism': 1, 'template': {'spec': j['spec']['template']['spec']['template']['spec']}},
                            'status': {'conditions': [{'type': 'Completed', 'status': 'True'}], 'succeededCount': 1,
                                       'startTime': '2026-11-07T20:00:00Z', 'completionTime': '2026-11-07T20:01:00Z'}}
                if args[:2] == ('logging', 'read'):
                    return [{'timestamp': '2026-11-07T20:00:30Z', 'logName': f'projects/{r.PROJECT}/logs/run.googleapis.com%2Fstdout', 'jsonPayload': report(), 'resource': {'type': 'cloud_run_job', 'labels': {'project_id': r.PROJECT, 'location': r.REGION, 'job_name': r.job_name(BID)}},
                             'labels': {'run.googleapis.com/execution_name': r.job_name(BID) + '-abcde'}}]
                raise AssertionError(args)
            def assert_intent(self):
                assert 'inspection-intent' in self.saved
            def run(self, *args):
                self.calls.append(args)
                if args[:3] != ('run', 'jobs', 'delete'): raise AssertionError(args)
                assert 'cleanup-intent' in self.saved
                return ''
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        cloud = Fake()
        lock = dict(req['failed_release_proof']['failed_release'], observed_at=req['failed_release_proof']['observed_at'])
        def record(c, root, bid, name, value):
            cloud.saved[name] = copy.deepcopy(value)
            return {'uri': 'gs://evidence/' + name, 'generation': '1', 'sha256': DIGEST}
        with tempfile.TemporaryDirectory() as d, patch.object(r, 'verify_archive', return_value={}), \
                patch.object(r, 'inspect_failed_release', return_value=lock), patch.object(r, 'assert_lock') as check_lock, \
                patch.object(r, 'build_image', return_value=IMAGE), patch.object(r, 'provision_job', return_value=r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')), \
                patch.object(r, 'record_evidence', side_effect=record), patch.object(r, 'local_migrations', return_value=[]):
            result = r.execute(cloud, Path(d), SHA, BID, DIGEST)
        self.assertEqual(cloud.executions, 1)
        self.assertIn('completed', cloud.saved)
        self.assertIn('inspection-finished', cloud.saved)
        self.assertTrue(result['read_only'])
        self.assertGreaterEqual(check_lock.call_count, 3)
        self.assertFalse(any(a[:2] == ('storage', 'rm') for a in cloud.calls))
        self.assertFalse(any(a[0] == 'sql' or a[:2] == ('run', 'deploy') for a in cloud.calls))

    def test_uncertain_submission_is_never_retried_or_cleaned(self):
        c = type('C', (), {'json': lambda self, *a: (_ for _ in ()).throw(TimeoutError('uncertain'))})()
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        job = r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')
        calls = []
        with self.assertRaises(TimeoutError):
            r.inspect_job(c, job, req, lambda n, d: calls.append(n), [])
        self.assertEqual(calls, ['inspection-intent'])

    def test_source_archive_full_inventory_and_symlink_rejection(self):
        import tarfile, io
        with tempfile.TemporaryDirectory() as d:
            path = Path(d) / 'source.tgz'
            with tarfile.open(path, 'w:gz') as t:
                x = tarfile.TarInfo('cmd/code.go'); data = b'package main'; x.size = len(data); t.addfile(x, io.BytesIO(data))
            self.assertEqual(set(r.archive_inventory(path)), {'cmd/code.go'})
            with tarfile.open(path, 'w:gz') as t:
                x = tarfile.TarInfo('link'); x.type = tarfile.SYMTYPE; x.linkname = '/secret'; t.addfile(x)
            with self.assertRaises(ValueError): r.archive_inventory(path)


if __name__ == '__main__': unittest.main()
