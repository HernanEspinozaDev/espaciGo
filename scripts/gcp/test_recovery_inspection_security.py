import copy
import hashlib
import io
import json
import tarfile
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import recovery_inspection as r
from test_recovery_parent_proof import request_fixture
import provision_recovery_inspection as provision
from test_recovery_inspection import SHA, BID, DIGEST, IMAGE, parent, report


class RecoverySecurityTests(unittest.TestCase):
    def test_provision_waits_for_exact_iam_receipt_before_child(self):
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        job = r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')
        events = []
        c = type('C', (), {'json': lambda self, *a: job})()
        lock = dict(req['failed_release_proof']['failed_release'], observed_at=req['failed_release_proof']['observed_at'])
        def record(name, value):
            events.append(name)
            return {'uri': f'gs://{r.BUCKET}/recovery-inspections/{BID}/{name}.json', 'generation': '17', 'sha256': DIGEST}
        with patch.object(r, 'inspect_failed_release', return_value=lock), \
                patch.object(r, 'wait_for_iam_ready', create=True, side_effect=lambda *a, **k: events.append('iam-gate')) as gate, \
                patch.object(r, 'submit_child', side_effect=lambda *a: events.append('submit')):
            r.provision_job(c, Path('/workspace'), req, record)
        gate.assert_called_once()
        self.assertLess(events.index('job-request'), events.index('iam-gate'))
        self.assertLess(events.index('iam-gate'), events.index('submit'))

    def test_execution_task_configuration_and_report_timestamp_fail_closed(self):
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        job = r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')
        execution = r.job_name(BID) + '-abcde'
        finished = {'metadata': {'name': execution, 'namespace': r.PROJECT_NUMBER,
                                'labels': {'run.googleapis.com/jobUid': job['metadata']['uid']}},
                    'spec': {'taskCount': 1, 'parallelism': 1, 'template': {'spec': job['spec']['template']['spec']['template']['spec']}},
                    'status': {'conditions': [{'type': 'Completed', 'status': 'True'}], 'succeededCount': 1,
                               'startTime': '2026-11-07T20:00:00Z', 'completionTime': '2026-11-07T20:01:00Z'}}
        entry = {'timestamp': '2026-11-07T20:00:30Z', 'jsonPayload': report(),
                 'logName': f'projects/{r.PROJECT}/logs/run.googleapis.com%2Fstdout',
                 'resource': {'type': 'cloud_run_job', 'labels': {'project_id': r.PROJECT, 'location': r.REGION, 'job_name': r.job_name(BID)}},
                 'labels': {'run.googleapis.com/execution_name': execution}}
        class Cloud:
            def __init__(self, end, log): self.end = end; self.log = log; self.executions = 0
            def json(self, *args):
                if args[:3] == ('run', 'jobs', 'execute'):
                    self.executions += 1; return {'metadata': {'name': execution}}
                if args[:4] == ('run', 'jobs', 'executions', 'describe'): return self.end
                if args[:2] == ('logging', 'read'): return [self.log]
                raise AssertionError(args)
        r.inspect_job(Cloud(finished, entry), job, req, lambda *a: None, [])
        variants = [lambda e, l: e['spec'].update(taskCount=2),
                    lambda e, l: e['spec'].update(extraRuntimeField=True),
                    lambda e, l: e['spec']['template']['spec']['containers'][0].update(workingDir='/other'),
                    lambda e, l: l.update(timestamp='2026-11-07T19:00:00Z'),
                    lambda e, l: l.update(logName='other'),
                    lambda e, l: l['resource'].update(type='cloud_run_revision')]
        for mutate in variants:
            e, l = copy.deepcopy(finished), copy.deepcopy(entry); mutate(e, l)
            c = Cloud(e, l)
            with self.assertRaises(ValueError): r.inspect_job(c, job, req, lambda *a: None, [])
            self.assertEqual(c.executions, 1)

    def test_command_scope_rejects_mutated_create_and_child_build(self):
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        create = tuple(a for a in r.create_args(req)[1:] if not a.startswith(('--project=', '--region=')))
        build = ('builds', 'submit', '/private/source.tar.gz', '--config=/workspace/cloudbuild-migrator-image.yaml',
                 '--gcs-source-staging-dir=gs://' + r.BUCKET + '/source', '--service-account=' + r.DEPLOYER,
                 '--substitutions=_COMMIT_SHA=' + SHA, '--async', '--format=json')
        bad = [create + ('--set-env-vars=DATABASE_SET_ROLE=postgres',),
               create + ('--execute-now',), build + ('--substitutions=_APPLY=true',),
               tuple('--config=/elsewhere/cloudbuild-migrator-image.yaml' if x.startswith('--config=') else x for x in build),
               tuple('--substitutions=_COMMIT_SHA=' + SHA + ',_APPLY=true' if x.startswith('--substitutions=') else x for x in build)]
        with patch.object(r.subprocess, 'run') as run:
            run.return_value.returncode = 0; run.return_value.stdout = '{}'
            for args in bad:
                with self.subTest(args=args), self.assertRaises(ValueError): r.Cloud().run(*args)
            run.assert_not_called()
            run.return_value.returncode = 0; run.return_value.stdout = '{}'
            r.Cloud().run(*create); r.Cloud().run(*build)
            self.assertEqual(run.call_count, 2)
            command = run.call_args_list[0].args[0]
            for flag in ('--region=' + r.REGION, '--service-account=' + r.DB_SA, '--tasks=1', '--network=espacigo-testprod'):
                self.assertLess(command.index(flag), command.index('--container=inspection'))
            for flag in ('--image=' + IMAGE, '--cpu=1', '--memory=512Mi'):
                self.assertGreater(command.index(flag), command.index('--container=inspection'))

    def test_build_config_and_parent_runtime_step_are_identical(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        d = yaml.safe_load((root / 'cloudbuild-db-recovery-inspect.yaml').read_text())
        step = d['steps'][0]
        for key, value in {'${_RECOVERY_INSPECTION_ONLY}': 'true', '${_COMMIT_SHA}': SHA, '${BUILD_ID}': BID, '${_SOURCE_ARCHIVE_SHA256}': DIGEST}.items():
            step['args'] = [a.replace(key, value) for a in step['args']]
        step['args'] = [a.replace('$$', '$') for a in step['args']]
        self.assertEqual(step, r.parent_step(SHA, BID, DIGEST))

    def test_evidence_create_only_generation_readback_and_mismatch_rejected(self):
        class Cloud:
            def __init__(self, corrupt=False): self.payload = ''; self.calls = []; self.corrupt = corrupt
            def run(self, *a):
                self.calls.append(tuple(map(str, a)))
                if a[:2] == ('storage', 'cp'):
                    self.payload = Path(a[2]).read_text(); return ''
                if a[:2] == ('storage', 'cat'):
                    return self.payload + ('bad' if self.corrupt else '')
                raise AssertionError(a)
            def json(self, *a): return {'generation': '23'}
        with tempfile.TemporaryDirectory() as d:
            c = Cloud(); out = r.record_evidence(c, Path(d), BID, 'inspection-report', {'read_only': True})
            self.assertEqual(out['sha256'], hashlib.sha256(c.payload.encode()).hexdigest())
            self.assertIn('--if-generation-match=0', c.calls[0])
            self.assertTrue(c.calls[1][2].endswith('#23'))
            with self.assertRaises(FileExistsError):
                r.record_evidence(c, Path(d), BID, 'inspection-report', {'different': True})
            with self.assertRaisesRegex(ValueError, 'readback'):
                r.record_evidence(Cloud(True), Path(d), BID, 'corrupt-report', {})

    def test_failed_applying_release_is_inspected_without_unlock_or_reexecute(self):
        uid = '22222222-3333-4444-8555-666666666666'
        name = 'espacigo-db-r-' + r.PINNED_BUILD_ID.replace('-', '')
        owner = json.dumps({'build_id': r.PINNED_BUILD_ID, 'source_sha': r.PINNED_SOURCE_SHA})
        class Cloud:
            extra = False
            def __init__(self): self.calls = []
            def run(self, *a):
                self.calls.append(a)
                if a[2] == r.LOCK + '#' + r.PINNED_GENERATION: return owner
                return json.dumps({'job': name, 'uid': uid, 'request': {'mode': 'apply'}})
            def json(self, *a):
                self.calls.append(a)
                if a[:3] == ('storage', 'objects', 'describe'):
                    return {'generation': r.PINNED_GENERATION if a[3] == r.LOCK else '4'}
                if a[:2] == ('builds', 'describe'):
                    return {'id': r.PINNED_BUILD_ID, 'status': 'FAILURE', 'buildTriggerId': r.TRIGGER, 'serviceAccount': r.DEPLOYER,
                            'substitutions': {'COMMIT_SHA': r.PINNED_SOURCE_SHA}}
                if a[:3] == ('run', 'jobs', 'describe'): return {'metadata': {'uid': uid}}
                raise AssertionError(a)
            def historical_list(self):
                value = self.historical_get()
                return {'executions': [value] + ([{'name': 'extra'}] if self.extra else [])}
            def historical_get(self):
                return {'name': f'projects/{r.PROJECT}/locations/{r.REGION}/jobs/{name}/executions/{name}-v62zb',
                        'job': name, 'uid': '33333333-4444-4555-8666-777777777777', 'failedCount': 1,
                        'conditions': [{'type': 'Completed', 'state': 'CONDITION_FAILED'}]}
        c = Cloud(); r.inspect_failed_release(c)
        self.assertFalse(any('rm' in a or 'execute' in a or 'delete' in a for a in c.calls))
        c = Cloud(); c.extra = True
        with self.assertRaisesRegex(ValueError, 'additional applying'): r.inspect_failed_release(c)

    def test_provisioner_rejects_bad_uri_before_cloud_and_apply_before_create(self):
        class Cloud:
            calls = []
            def run(self, *a): self.calls.append(a); return json.dumps({'format': 'espacigo.recovery-job-request.v1', 'mode': 'apply'})
        c = Cloud()
        with self.assertRaises(ValueError):
            provision.execute(c, Path('/workspace'), BID, BID, DIGEST, 'gs://wrong/request.json', '1', DIGEST)
        self.assertEqual(c.calls, [])
        text = json.dumps({'format': 'espacigo.recovery-job-request.v1', 'mode': 'apply'})
        with self.assertRaisesRegex(ValueError, 'only inspection'):
            provision.execute(c, Path('/workspace'), BID, BID, DIGEST,
                              f'gs://{r.BUCKET}/recovery-inspections/{BID}/job-request.json', '1', hashlib.sha256(text.encode()).hexdigest())
        self.assertEqual(len(c.calls), 1)

    def test_provisioner_creates_only_exact_readonly_request_after_source_and_lock(self):
        me = '77777777-8888-4999-8aaa-bbbbbbbbbbbb'
        req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{r.BUCKET}/source/recovery.tgz#17')
        text = json.dumps(req); checksum = hashlib.sha256(text.encode()).hexdigest()
        uri = f'gs://{r.BUCKET}/recovery-inspections/{BID}/job-request.json'
        class Cloud:
            def __init__(self): self.calls = []
            def run(self, *a):
                self.calls.append(a)
                if a[:2] == ('storage', 'cat'): return text
                if a[:3] == ('run', 'jobs', 'create'): return ''
                raise AssertionError(a)
            def json(self, *a):
                self.calls.append(a)
                if a == ('builds', 'describe', BID): return parent()
                if a == ('builds', 'describe', me):
                    p = parent(); p.update(id=me, serviceAccount=r.PROVISIONER)
                    p['substitutions'] = {'_PARENT_BUILD_ID': BID, '_SOURCE_ARCHIVE_SHA256': DIGEST, '_REQUEST_URI': uri,
                                          '_REQUEST_GENERATION': '1', '_REQUEST_SHA256': checksum}
                    return p
                raise AssertionError(a)
        c = Cloud()
        lock_value = req['failed_release_proof']['failed_release']
        with patch.object(provision, 'verify_archive') as verify, \
                patch.object(provision, 'assert_lock', create=True) as check:
            provision.execute(c, Path('/workspace'), me, BID, DIGEST, uri, '1', checksum)
        verify.assert_called_once()
        check.assert_called_with(c, lock_value['lock_generation'], lock_value['lock_text'])
        self.assertEqual(check.call_count, 2)
        writes = [a for a in c.calls if a[:3] == ('run', 'jobs', 'create')]
        self.assertEqual(len(writes), 1)
        self.assertIn('--args=-dir,/app/db/migrations,-recovery-inspection-only', writes[0])

    def test_image_provenance_must_match_the_actual_child_build(self):
        child = '77777777-8888-4999-8aaa-bbbbbbbbbbbb'
        digest = 'sha256:' + 'c' * 64
        built = {'id': child, 'substitutions': {'_COMMIT_SHA': SHA}, 'results': {'images': [{'name': r.REGISTRY + '/espacigo-db-migrator:sha-' + SHA, 'digest': digest}]}}
        class Cloud:
            def __init__(self): self.bad = False
            def json(self, *a):
                invocation = f'https://cloudbuild.googleapis.com/v1/projects/{r.PROJECT}/locations/{r.REGION}/builds/{child}'
                return {'image_summary': {'digest': digest}, 'provenance_summary': {'provenance': [{'build': {'inTotoSlsaProvenanceV1': {
                    'predicateType': 'https://slsa.dev/provenance/v1', 'subject': [{'digest': {'sha256': digest[7:]}}],
                    'predicate': {'runDetails': {'metadata': {'invocationId': 'wrong' if self.bad else invocation}}}}}}]}}
        c = Cloud()
        with patch.object(r, 'submit_child', return_value=built):
            self.assertEqual(r.build_image(c, Path('/workspace'), 'gs://source', DIGEST, SHA, lambda *a: None), IMAGE)
            c.bad = True
            with self.assertRaisesRegex(ValueError, 'provenance'): r.build_image(c, Path('/workspace'), 'gs://source', DIGEST, SHA, lambda *a: None)


if __name__ == '__main__': unittest.main()
