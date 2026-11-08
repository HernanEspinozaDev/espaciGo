import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch
from datetime import datetime, timezone
import sys
sys.path.insert(0, str(Path(__file__).resolve().parent))

import recovery_inspection as r
import provision_recovery_inspection as child
from recovery_parent_proof import make_failed_release_proof

PARENT='11111111-2222-4333-8444-555555555555'
SHA='a'*40
DIGEST='b'*64
URI=f'gs://{r.BUCKET}/source/recovery.tgz#17'
IMAGE=r.REGISTRY+'/espacigo-db-migrator@sha256:'+'c'*64


def facts():
    bid=r.PINNED_BUILD_ID; job='espacigo-db-r-'+bid.replace('-','')
    return {'observed_at':datetime.now(timezone.utc).isoformat(),'build_id':bid,'source_sha':r.PINNED_SOURCE_SHA,
      'lock_uri':r.LOCK,'lock_generation':r.PINNED_GENERATION,'lock_text':json.dumps({'build_id':bid,'source_sha':r.PINNED_SOURCE_SHA}),
      'job_name':job,'job_uid':'22222222-3333-4444-8555-666666666666',
      'execution_name':f'projects/{r.PROJECT}/locations/{r.REGION}/jobs/{job}/executions/{job}-v62zb','execution_uid':'33333333-4444-4555-8666-777777777777',
      'execution_state':'CONDITION_FAILED','failed_count':1,'running_count':0,'succeeded_count':0,'list_count':1,'next_page':'',
      'intent_uri':f'gs://{r.BUCKET}/releases/{bid}/job-intent-apply.json','intent_generation':'9','intent_sha256':'d'*64,'failed_execution':job+'-v62zb'}


def request():
    proof=make_failed_release_proof(facts(),PARENT,SHA,URI,DIGEST)
    return r.make_request(SHA,PARENT,DIGEST,IMAGE,URI,proof)


class ChildCloud:
    def __init__(self,req): self.req=req; self.calls=[]
    def run(self,*args):
        self.calls.append(args)
        if args[:2]==('storage','cat') and 'job-request.json' in args[2]: return json.dumps(self.req,sort_keys=True,separators=(',',':'))
        if args[:2]==('storage','cat'): return self.req['failed_release_proof']['failed_release']['lock_text']
        if args[:3]==('run','jobs','create'): return ''
        if args[:3]==('storage','objects','describe'): return {'generation':'9'}
        if args[:2]==('builds','describe'):
            return {}
        raise AssertionError(args)
    def json(self,*args):
        self.calls.append(args)
        if args[:2]==('builds','describe') and args[2]==PARENT:
            return {'id':PARENT,'status':'WORKING','serviceAccount':r.DEPLOYER,'substitutions':{'_RECOVERY_INSPECTION_ONLY':'true','_COMMIT_SHA':SHA,'_SOURCE_ARCHIVE_SHA256':DIGEST},'steps':[r.parent_step(SHA,PARENT,DIGEST)],'sourceProvenance':{'resolvedStorageSource':{'bucket':r.BUCKET,'object':'source/recovery.tgz','generation':'17'}}}
        if args[:2]==('builds','describe'): return {'id':args[2],'status':'WORKING','serviceAccount':r.PROVISIONER,'substitutions':{'_PARENT_BUILD_ID':PARENT,'_SOURCE_ARCHIVE_SHA256':DIGEST,'_REQUEST_URI':f'gs://{r.BUCKET}/recovery-inspections/{PARENT}/job-request.json','_REQUEST_GENERATION':'19','_REQUEST_SHA256':hashlib.sha256(json.dumps(self.req,sort_keys=True,separators=(',',':')).encode()).hexdigest()},'sourceProvenance':{'resolvedStorageSource':{'bucket':r.BUCKET,'object':'source/recovery.tgz','generation':'17'}}}
        if args == ('storage', 'objects', 'describe', r.LOCK):
            return {'generation': r.PINNED_GENERATION}
        raise AssertionError('child attempted unauthorized historical read: ' + str(args))


class Boundary(unittest.TestCase):
    def test_parent_checks_full_proof_before_and_after_child(self):
        req = request(); events = []
        cloud = type('Cloud', (), {'json': lambda self, *args: r.desired_job(req, uid='22222222-3333-4444-8555-666666666666')})()
        observed = dict(req['failed_release_proof']['failed_release'], observed_at=req['failed_release_proof']['observed_at'])
        def observe(*args):
            events.append('observe'); return copy.deepcopy(observed)
        def record(name, value):
            events.append(name)
            return {'uri': f'gs://{r.BUCKET}/recovery-inspections/{PARENT}/{name}.json', 'generation': '19', 'sha256': DIGEST}
        with patch.object(r, 'inspect_failed_release', side_effect=observe), \
                patch.object(r, 'wait_for_iam_ready', side_effect=lambda *a: events.append('ready')), \
                patch.object(r, 'submit_child', side_effect=lambda *a: events.append('submit')):
            r.provision_job(cloud, Path('/workspace'), req, record)
        submit = events.index('submit')
        self.assertEqual(events.count('submit'), 1)
        self.assertIn('observe', events[:submit])
        self.assertIn('observe', events[submit + 1:])
        self.assertLess(events.index('ready'), submit)

    def test_parent_postcheck_change_blocks_inspection_execution(self):
        changes = {key: (value + 1 if type(value) is int else value + 'x')
                   for key, value in facts().items() if key != 'observed_at'}
        changes.update(job_uid='44444444-5555-4666-8777-888888888888',
                       execution_uid='55555555-6666-4777-8888-999999999999',
                       intent_generation='10', intent_sha256='e' * 64)
        for field, changed in changes.items():
            initial = facts(); after = copy.deepcopy(initial); after[field] = changed
            req = request(); cloud = ChildCloud(req)
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory, \
                    patch.object(r, 'verify_archive'), patch.object(r, 'assert_lock'), \
                    patch.object(r, 'record_evidence', return_value={'uri': f'gs://{r.BUCKET}/recovery-inspections/{PARENT}/job-request.json', 'generation': '19', 'sha256': DIGEST}), \
                    patch.object(r, 'inspect_failed_release', side_effect=[initial, initial, initial, after]), \
                    patch.object(r, 'build_image', return_value=IMAGE), \
                    patch.object(r, 'wait_for_iam_ready', return_value={}), \
                    patch.object(r, 'submit_child') as submit, patch.object(r, 'inspect_job') as inspect:
                with self.assertRaises(ValueError):
                    r.execute(cloud, Path(directory), SHA, PARENT, DIGEST)
                submit.assert_called_once()
                inspect.assert_not_called()

    def test_child_rejects_expired_or_altered_proof_before_create(self):
        from datetime import timedelta
        for kind in ('stale', 'future', 'digest', 'parent', 'legacy', 'missing'):
            req = request()
            if kind in ('stale', 'future'):
                offset = -601 if kind == 'stale' else 60
                req['failed_release_proof']['observed_at'] = (datetime.now(timezone.utc) + timedelta(seconds=offset)).isoformat()
                req['failed_release_proof_sha256'] = r.canonical_proof_sha256(req['failed_release_proof'])
            elif kind == 'digest': req['failed_release_proof_sha256'] = '0' * 64
            elif kind == 'parent': req['failed_release_proof']['parent_build_id'] = '44444444-5555-4666-8777-888888888888'
            elif kind == 'legacy': req['format'] = 'espacigo.recovery-job-request.v1'
            else: del req['failed_release_proof']
            cloud = ChildCloud(req)
            text = json.dumps(req, sort_keys=True, separators=(',', ':'))
            with self.subTest(kind=kind), self.assertRaises(ValueError), patch.object(child, 'verify_archive'):
                child.execute(cloud, Path('/workspace'), '44444444-5555-4666-8777-888888888888', PARENT, DIGEST,
                              f'gs://{r.BUCKET}/recovery-inspections/{PARENT}/job-request.json', '19', hashlib.sha256(text.encode()).hexdigest())
            self.assertFalse(any(a[:3] == ('run', 'jobs', 'create') for a in cloud.calls))

    def test_valid_child_uses_embedded_proof_without_historical_reads_and_creates_once(self):
        req=request(); c=ChildCloud(req); bid='44444444-5555-4666-8777-888888888888'
        uri=f'gs://{r.BUCKET}/recovery-inspections/{PARENT}/job-request.json'; text=json.dumps(req,sort_keys=True,separators=(',',':'))
        with tempfile.TemporaryDirectory() as d, patch.object(child,'verify_archive'):
            child.execute(c,Path(d),bid,PARENT,DIGEST,uri,'19',hashlib.sha256(text.encode()).hexdigest())
        self.assertEqual(sum(1 for x in c.calls if x[:3]==('run','jobs','create')),1)
        self.assertFalse(any('historical' in str(x) or x[:2]==('run','jobs') and len(x)>2 and x[2] in ('executions','describe','list') for x in c.calls))

if __name__=='__main__': unittest.main()
