import copy
from datetime import datetime,timezone
import json
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch
import release_main as release
from verify_web_backend import GuardError,SA,BUCKET,REGISTRY
from rollout_web import LOCK,WATERMARK,TRIGGER
from test_migration_execution import fixture,BUILD,UID
from release_jobs import request,verify_job,create_args
from provision_release_job import execute as provision_execute

ROOT=Path(__file__).resolve().parents[2]
SHA='a'*40


class FakeCloud:
    def __init__(self):
        self.calls=[];self.objects={WATERMARK:('1',json.dumps({'created_epoch':0}))};self.generation=1
        self.parent={'id':BUILD,'buildTriggerId':TRIGGER,'serviceAccount':SA,'status':'WORKING','createTime':'2026-11-07T00:00:00Z','substitutions':{'COMMIT_SHA':SHA,'BRANCH_NAME':'main','REPO_FULL_NAME':'HernanEspinozaDev/espaciGo','_DEPLOY_SCOPE':'full'}}
        self.services={};self.revisions={};self.fail_ready=False
        for i,s in enumerate(release.ALL_SERVICES):
            port=8080 if i==0 else 4000
            sa=['espacigo-test-api','espacigo-test-public-web','espacigo-test-private-web'][i]
            self.services[s]={'metadata':{'annotations':{'run.googleapis.com/iap-enabled':'true'}},'spec':{'template':{'metadata':{'annotations':{'autoscaling.knative.dev/maxScale':'10'}},'spec':{'serviceAccountName':sa+'@espacigo-testprod.iam.gserviceaccount.com','containers':[{'image':'old','env':[{'name':'KEEP','value':'preserved'}],'startupProbe':{'httpGet':{'path':'/health/ready' if i==0 else '/healthz','port':port}}}]}}},'status':{'traffic':[{'revisionName':s+'-old','percent':100}]}}
    def run(self,*args,**kwargs):
        self.calls.append(tuple(str(a) for a in args))
        if args[:2]==('storage','ls'):return '\n'.join(k for k in self.objects if k.endswith('/completed.json'))+'\n'
        if args[:2]==('storage','cat'):
            uri=str(args[2]);name,_,generation=uri.partition('#');stored=self.objects[name]
            if generation and generation!=stored[0]:raise GuardError('generation mismatch')
            return stored[1]
        if args[:2]==('storage','cp'):
            path,uri=Path(args[2]),str(args[3]);expected=str(args[4]).split('=',1)[1]
            actual=self.objects.get(uri,('0',''))[0]
            if actual!=expected:raise GuardError('precondition 412')
            self.generation+=1;self.objects[uri]=(str(self.generation),path.read_text());return ''
        if args[:2]==('storage','rm'):
            uri=str(args[2]);self.assert_generation(uri,args[3]);del self.objects[uri];return ''
        if args[:2]==('run','deploy'):
            service=args[2];image=next(a.split('=',1)[1] for a in args if a.startswith('--image='));suffix=next(a.split('=',1)[1] for a in args if a.startswith('--revision-suffix='));revision=service+'-'+suffix
            container=self.services[service]['spec']['template']['spec']['containers'][0]
            container['image']=image
            update_env=next((a.split('=',1)[1] for a in args if a.startswith('--update-env-vars=')), '')
            for entry in update_env.split(',') if update_env else []:
                name,value=entry.split('=',1);env=container.setdefault('env',[]);env[:]=[item for item in env if item.get('name')!=name];env.append({'name':name,'value':value})
            self.revisions[revision]={'metadata':{'labels':{'source-commit':SHA,'cloud-build-id':BUILD}},'spec':{'containers':[copy.deepcopy(container)]},'status':{'imageDigest':image,'conditions':[{'type':'Ready','status':'False' if self.fail_ready else 'True'}]}}
            return ''
        if args[:3]==('run','services','update-traffic'):
            revision=args[4].split('=',1)[1].rsplit('=',1)[0];self.services[args[3]]['status']['traffic']=[{'revisionName':revision,'percent':100}];return ''
        raise AssertionError(('unexpected cloud mutation',args))
    def assert_generation(self,uri,flag):
        if self.objects[uri][0]!=str(flag).split('=',1)[1]:raise GuardError('not owner')
    def json(self,*args):
        self.calls.append(tuple(map(str,args)))
        if args[:2]==('builds','describe'):
            result=copy.deepcopy(self.parent)
            if args[2]!=BUILD:result['status']='FAILURE'
            return result
        if args[:3]==('storage','objects','describe'):return {'generation':self.objects[str(args[3])][0]}
        if args[:3]==('run','services','describe'):return copy.deepcopy(self.services[args[3]])
        if args[:3]==('run','services','get-iam-policy'):return {'bindings':[{'role':'roles/run.invoker','members':['serviceAccount:service-447373903153@gcp-sa-iap.iam.gserviceaccount.com']}]}
        if args[:3]==('run','revisions','describe'):return copy.deepcopy(self.revisions[args[3]])
        raise AssertionError(('unexpected cloud read',args))


class MainReleaseTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup);self.root=Path(self.temp.name)
        for name in ['go.mod','cloudbuild.yaml','db/release-policy.json']:
            dest=self.root/name;dest.parent.mkdir(parents=True,exist_ok=True);shutil.copyfile(ROOT/name,dest)
        shutil.copytree(ROOT/'db/migrations',self.root/'db/migrations')
        self.cloud=FakeCloud();self.events=[]
    def patches(self,job_failure=False,backup_failure=False):
        def image(*args):
            self.events.append('image')
            return ({r:REGISTRY+'/'+r+'@sha256:'+'b'*64 for r in args[-1]},BUILD)
        def provision(c,r,req,record):self.events.append('provision-'+req['mode']);return {'metadata':{'name':req['mode']}}
        def job(c,j,r,record):
            self.events.append('job-'+r['mode'])
            if r['mode']=='apply':
                if job_failure:raise GuardError('migration failed')
                record('migration-verified',{'pending':0});return {'applied':[]}
            entries=release.SCHEMA.migration_inventory(self.root/'db/migrations')
            return {'database':'espacigo_dms_stage_20261020','server_version':'18.6','read_only':True,'history_exists':True,'applied':entries,'pending':[]}
        def backup(*args):
            self.events.append('backup')
            if backup_failure:raise GuardError('backup invalid')
            return {'backup_id':'123','pending_sql':False}
        return patch.multiple(release,image_build=image,provision=provision,execute_job=job,backup=backup,remove_job=lambda *a:self.events.append('remove'))
    def test_complete_release_waits_for_verified_job_then_preserves_config_and_releases_owned_lock(self):
        with self.patches():report=release.execute(self.cloud,self.root,SHA,BUILD)
        self.assertNotIn(LOCK,self.cloud.objects)
        self.assertLess(self.events.index('backup'),self.events.index('job-apply'))
        self.assertEqual(report['schema_migrations'],len(list((self.root/'db/migrations').glob('V*.sql'))))
        for call in self.cloud.calls:
            if call[:2]==('run','deploy'):
                self.assertIn('--no-traffic',call);self.assertIn('--update-labels=source-commit='+SHA+',cloud-build-id='+BUILD,call)
                self.assertFalse(any(x.startswith(('--max','--set-env-vars','--set-secrets','--service-account')) for x in call))
                if call[2]==release.ALL_SERVICES[0]:self.assertIn('--update-env-vars=CLOUD_STORAGE_MEDIA_BUCKET='+release.MEDIA_BUCKET,call)
                else:self.assertFalse(any(x.startswith('--update-env-vars=CLOUD_STORAGE_MEDIA_BUCKET=') for x in call))
        for service in release.ALL_SERVICES:
            self.assertEqual(self.cloud.services[service]['spec']['template']['metadata']['annotations']['autoscaling.knative.dev/maxScale'],'10')
        api_env={item['name']:item.get('value') for item in self.cloud.services[release.ALL_SERVICES[0]]['spec']['template']['spec']['containers'][0]['env']}
        self.assertEqual(api_env['CLOUD_STORAGE_MEDIA_BUCKET'],release.MEDIA_BUCKET)
        self.assertTrue(any('/success.json' in x for x in self.cloud.objects))
    def test_job_failure_or_backup_failure_never_deploys_and_retains_owned_lock(self):
        for job,backup in [(True,False),(False,True)]:
            self.cloud=FakeCloud()
            if (self.root/'.local').exists():shutil.rmtree(self.root/'.local')
            with self.patches(job,backup),self.assertRaises(GuardError):release.execute(self.cloud,self.root,SHA,BUILD)
            self.assertIn(LOCK,self.cloud.objects)
            self.assertFalse(any(c[:2]==('run','deploy') for c in self.cloud.calls))
            self.assertTrue(any('/failure.json' in x for x in self.cloud.objects))
    def test_busy_lock_is_never_deleted_or_replaced(self):
        self.cloud.objects[LOCK]=('9',json.dumps({'build_id':'87654321-1234-1234-1234-123456789abc'}))
        with self.patches(),self.assertRaises(GuardError):release.execute(self.cloud,self.root,SHA,BUILD)
        self.assertEqual(self.cloud.objects[LOCK][0],'9')
        self.assertFalse(any(c[:2]==('storage','rm') for c in self.cloud.calls))
    def test_not_ready_revision_cannot_receive_traffic(self):
        self.cloud.fail_ready=True
        with self.patches(),self.assertRaises(GuardError):release.execute(self.cloud,self.root,SHA,BUILD)
        self.assertFalse(any(c[:3]==('run','services','update-traffic') for c in self.cloud.calls))
        self.assertIn(LOCK,self.cloud.objects)
    def test_partial_promotion_failure_restores_previous_traffic_including_uncertain_attempt(self):
        original=release.promote_and_verify
        def promote(cloud,service,revision):
            if service==release.SERVICES[0] and not revision.endswith('-old'):
                cloud.run('run','services','update-traffic',service,'--to-revisions='+revision+'=100')
                raise GuardError('promotion readback failed after request')
            return original(cloud,service,revision)
        with self.patches(),patch.object(release,'promote_and_verify',promote),self.assertRaises(GuardError):release.execute(self.cloud,self.root,SHA,BUILD)
        for service in release.ALL_SERVICES:self.assertEqual(self.cloud.services[service]['status']['traffic'][0]['revisionName'],service+'-old')
        self.assertIn(LOCK,self.cloud.objects)

    def test_docs_have_no_lock_job_or_rollout_and_web_classification_is_explicit(self):
        inv=release.source_inventory(self.root)
        self.cloud.objects[WATERMARK]=('1',json.dumps({'source_inventory':inv,'created_epoch':0}))
        with self.patches():self.assertEqual(release.execute(self.cloud,self.root,SHA,BUILD)['scope'],'docs')
        self.assertFalse(any(c[:2]==('storage','cp') for c in self.cloud.calls))
        self.assertEqual(release.classify({**inv,'frontend/a.ts':'a'},{'source_inventory':inv}),'web')
        self.assertEqual(release.classify({**inv,'internal/x.go':'a'},{'source_inventory':inv}),'full')
    def test_parent_and_migration_policy_fail_closed(self):
        self.cloud.parent['buildTriggerId']='other'
        with self.assertRaises(GuardError):release.execute(self.cloud,self.root,SHA,BUILD)
        with self.assertRaises(GuardError):release.migration_policy(self.root,{'pending':[{'name':'V000084__unreviewed.sql','checksum':'c'*64}]})


class JobBoundaryTests(unittest.TestCase):
    def test_job_config_readback_rejects_overrides_before_execution(self):
        m,b,e,logs,name=fixture();r=request(m,BUILD,'apply',b)
        job={'metadata':{'name':create_args(r)[4],'namespace':'447373903153','uid':UID},'spec':{'template':{'metadata':{'labels':e['metadata']['labels'],'annotations':e['metadata']['annotations']},'spec':e['spec']}}}
        verify_job(job,r)
        job['spec']['template']['spec']['template']['spec']['containers'][0]['env'].append({'name':'PGOPTIONS','value':'unsafe'})
        with self.assertRaises(GuardError):verify_job(job,r)
    def test_provisioner_cannot_use_a_failed_or_foreign_parent(self):
        m,b,*_=fixture();r=request(m,BUILD,'apply',b);cloud=FakeCloud();cloud.parent['status']='FAILURE'
        with self.assertRaises(GuardError):provision_execute(cloud,r)
        self.assertFalse(any(c[:2]==('run','jobs') for c in cloud.calls))



class BackupRetentionTests(unittest.TestCase):
    def cloud(self,unowned=False):
        c=FakeCloud();backup_ids={}
        for n in range(9):
            bid=f'{n+1:08x}-1234-1234-1234-123456789abc'
            start=f'2026-10-{29-n:02d}T00:00:00Z'
            receipt={'format':'espacigo.main-release.v1','build_id':bid,'backup':{'pending_sql':True,'backup_type':'ON_DEMAND','instance':'espacigo-testprod-db','backup_id':str(100+n),'backup_start':start}}
            uri=f'gs://{BUCKET}/releases/{bid}/completed.json';c.objects[uri]=('1',json.dumps(receipt));backup_ids[str(100+n)]=bid
        original=c.json
        def read(*args):
            if args[:3]==('sql','backups','describe'):
                id=args[3]
                return {'selfLink':f'https://sqladmin.googleapis.com/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-testprod-db/backupRuns/{id}','instance':'espacigo-testprod-db','description':'foreign' if unowned else 'espacigo-release-'+backup_ids[id],'type':'ON_DEMAND','status':'SUCCESSFUL'}
            return original(*args)
        c.json=read;run=c.run
        def mutate(*args,**kwargs):
            if args[:3]==('sql','backups','delete'):c.calls.append(args);return ''
            return run(*args,**kwargs)
        c.run=mutate
        return c
    def test_only_owned_old_backups_outside_seven_newest_are_removed_and_current_is_protected(self):
        c=self.cloud();r=release.retain_backups(c,{'backup_id':'108'},datetime(2026,11,7,tzinfo=timezone.utc))
        self.assertEqual(r['deleted_ids'],['107'])
    def test_unowned_backup_is_never_deleted(self):
        c=self.cloud(True)
        with self.assertRaises(GuardError):release.retain_backups(c,{'backup_id':'108'},datetime(2026,11,7,tzinfo=timezone.utc))
        self.assertFalse(any(x[:3]==('sql','backups','delete') for x in c.calls))

if __name__=='__main__':unittest.main()
