import copy
from datetime import datetime,timezone
import hashlib
import unittest
from migration_execution import proposal,verify_execution,contract
from migration_release_manifest import FORMAT,IMAGES,REGISTRY,TARGET
from migration_job_plan import SECRET,SERVICE_ACCOUNT

BUILD='12345678-1234-1234-1234-123456789abc'
UID='abcdefab-1234-1234-1234-123456789abc'
NOW=datetime(2026,11,7,1,0,tzinfo=timezone.utc)


def fixture():
    entries=[{'version':i,'name':f'V{i:06d}__synthetic.sql','checksum':hashlib.sha256(str(i).encode()).hexdigest()} for i in range(1,85)]
    fingerprint=hashlib.sha256(''.join(f"{m['checksum']}  {m['name']}\n" for m in entries).encode()).hexdigest()
    manifest={'format':FORMAT,'source_sha':'a'*40,'target':TARGET.copy(),'images':{n:REGISTRY+'/'+r+'@sha256:'+'b'*64 for n,r in IMAGES.items()},'migration_inventory':{'format':'espacigo.migrations.inventory.v1','inventory_sha256':fingerprint,'migrations':entries}}
    before={'database':TARGET['database'],'server_version':'18.6','read_only':True,'history_exists':True,'applied':entries[:83],'pending':entries[83:]}
    p=proposal(manifest,before,BUILD,'1');job=p['job'];name=job+'-abc12'
    env_flag=next(x for x in p['create_argv'] if x.startswith('--set-env-vars='))
    env=[{'name':k,'value':v} for k,v in [item.split('=',1) for item in env_flag.removeprefix('--set-env-vars=^|^').split('|')]]
    execution={'apiVersion':'run.googleapis.com/v1','kind':'Execution','metadata':{'name':name,'namespace':'447373903153','creationTimestamp':'2026-11-07T00:59:00Z','labels':{'run.googleapis.com/job':job,'run.googleapis.com/jobUid':UID,'source-commit':manifest['source_sha'],'cloud-build-id':BUILD,'cloud.googleapis.com/location':'southamerica-west1'},'annotations':{'run.googleapis.com/network-interfaces':'[{"network":"espacigo-testprod","subnetwork":"espacigo-testprod-api-serverless"}]','run.googleapis.com/vpc-access-egress':'private-ranges-only'},'ownerReferences':[{'kind':'Job','name':job,'uid':UID,'controller':True}]},'spec':{'taskCount':1,'parallelism':1,'template':{'spec':{'maxRetries':0,'timeoutSeconds':'600','serviceAccountName':SERVICE_ACCOUNT,'containers':[{'image':manifest['images']['migrator'],'command':['/app/dbmigrate'],'args':p['args'],'env':env,'resources':{'limits':{'cpu':'1','memory':'512Mi'}},'volumeMounts':[{'name':'secret-mount','mountPath':'/var/secrets/migration'}]}],'volumes':[{'name':'secret-mount','secret':{'secretName':SECRET,'items':[{'key':'1','path':'password'}]}}]}}},'status':{'succeededCount':1,'conditions':[{'type':'Completed','status':'True'}],'completionTime':'2026-11-07T00:59:20Z'}}
    after=copy.deepcopy(before);after['applied']=entries;after['pending']=[]
    logs=[{'resource':{'type':'cloud_run_job','labels':{'project_id':'espacigo-testprod','location':'southamerica-west1','job_name':job}},'labels':{'run.googleapis.com/execution_name':name},'logName':'projects/espacigo-testprod/logs/run.googleapis.com%2Fstdout','timestamp':'2026-11-07T00:59:15Z','jsonPayload':{'format':'espacigo.migration-execution.v1','contract':contract(manifest,before),'before':before,'after':after,'applied':[84]}}]
    return manifest,before,execution,logs,name


def verify(m,b,e,logs,name):
    return verify_execution(m,b,BUILD,'1',UID,name,e,logs,NOW)


class MigrationExecutionTests(unittest.TestCase):
    def test_pinned_job_and_successful_execution_contract(self):
        m,b,e,logs,name=fixture();p=proposal(m,b,BUILD,'1')
        self.assertFalse(p['executed']);self.assertIn('--wait',p['execute_argv'])
        self.assertFalse(any('overrides' in x or x.startswith('--args') for x in p['execute_argv']))
        self.assertIn('-release',p['args'])
        self.assertEqual(verify(m,b,e,logs,name)['applied'],[84])

    def test_rejects_wrong_identity_image_args_environment_network_and_secret(self):
        changes=[lambda e:e['metadata'].update(namespace='other'),lambda e:e['metadata']['labels'].update({'cloud-build-id':'other'}),lambda e:e['metadata']['ownerReferences'][0].update(uid='other'),lambda e:e['metadata']['annotations'].update({'run.googleapis.com/vpc-access-egress':'all-traffic'}),lambda e:e['spec']['template']['spec'].update(serviceAccountName='runtime'),lambda e:e['spec']['template']['spec']['containers'][0]['resources']['limits'].update(cpu='8'),lambda e:e['spec']['template']['spec']['containers'][0].update(image='image:latest'),lambda e:e['spec']['template']['spec']['containers'][0].update(args=['-dir','/app/db/migrations']),lambda e:e['spec']['template']['spec']['containers'][0]['env'].append({'name':'PGOPTIONS','value':'unsafe'}),lambda e:e['spec']['template']['spec']['volumes'][0]['secret']['items'][0].update(key='latest')]
        for change in changes:
            m,b,e,logs,name=fixture();change(e)
            with self.assertRaises(ValueError):verify(m,b,e,logs,name)

    def test_rejects_incomplete_failed_retried_or_stale_execution(self):
        for change in [lambda e:e['status'].update(succeededCount=0),lambda e:e['status'].update(failedCount=1),lambda e:e['status'].update(retriedCount=1),lambda e:e['status']['conditions'][0].update(status='Unknown'),lambda e:e['metadata'].update(creationTimestamp='2026-11-06T00:00:00Z'),lambda e:e['spec'].update(taskCount=2)]:
            m,b,e,logs,name=fixture();change(e)
            with self.assertRaises(ValueError):verify(m,b,e,logs,name)

    def test_missing_duplicated_foreign_or_inconsistent_report_blocks_rollout(self):
        for change in [lambda logs:logs.clear(),lambda logs:logs.append(copy.deepcopy(logs[0])),lambda logs:logs[0]['labels'].update({'run.googleapis.com/execution_name':'other'}),lambda logs:logs[0].update(logName='projects/other/logs/stdout'),lambda logs:logs[0]['jsonPayload']['contract'].update(source_sha='c'*40),lambda logs:logs[0]['jsonPayload']['after'].update(pending=logs[0]['jsonPayload']['before']['pending']),lambda logs:logs[0]['jsonPayload'].update(applied=[])]:
            m,b,e,logs,name=fixture();change(logs)
            with self.assertRaises(ValueError):verify(m,b,e,logs,name)

    def test_old_baseline_and_mutable_release_are_rejected(self):
        m,b,_,_,_=fixture();b['applied']=b['applied'][:82];b['pending']=m['migration_inventory']['migrations'][82:]
        with self.assertRaises(ValueError):proposal(m,b,BUILD,'1')
        m,b,_,_,_=fixture();m['images']['migrator']='image:latest'
        with self.assertRaises(ValueError):proposal(m,b,BUILD,'1')


if __name__=='__main__':unittest.main()
