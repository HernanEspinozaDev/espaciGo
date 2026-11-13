"""Pinned ephemeral migration Jobs; provisioned by a separate build identity."""
import base64
from datetime import datetime,timezone
import json
from pathlib import Path
import re
import time
from migration_job_plan import proposal as plan_proposal,SERVICE_ACCOUNT
from migration_execution import proposal as apply_proposal,verify_execution
from migration_release_manifest import check_plan,is_plan_report_shape
from verify_web_backend import PROJECT,REGION,SA,UUID,require

PROVISIONER=f'projects/{PROJECT}/serviceAccounts/espacigo-release-job-builder@{PROJECT}.iam.gserviceaccount.com'


def request(manifest,build_id,mode,preflight=None):
    require(mode in ['plan','apply'],'unsupported Job mode')
    require(re.fullmatch(UUID,build_id) is not None,'parent UUID required')
    r={'format':'espacigo.job-request.v1','manifest':manifest,'build_id':build_id,'mode':mode,'preflight':preflight}
    return r


def create_args(r):
    m=r['manifest'];bid=r['build_id']
    if r['mode']=='apply':return apply_proposal(m,r['preflight'],bid,'1')['create_argv']
    require(r['mode']=='plan','unsupported Job mode')
    args=plan_proposal(m['images']['migrator'],m['source_sha'],'1')['proposed_job_create_argv']
    args[4]='espacigo-db-p-'+bid.replace('-','')
    return [('--labels=source-commit='+m['source_sha']+',cloud-build-id='+bid+',purpose=flujo-release-plan' if x.startswith('--labels=') else x) for x in args]


def provision(cloud,root,r,record):
    args=create_args(r);name=args[4]
    receipt=record('job-request-'+r['mode'],r)
    build=cloud.json('builds','submit',root,f'--config={root/"cloudbuild-job-provision.yaml"}',f'--ignore-file={root/".gcloudignore"}',f'--gcs-source-staging-dir=gs://espacigo-testprod-cloud-build-447373903153/source',f'--service-account={PROVISIONER}',f"--substitutions=_REQUEST_URI={receipt['uri']},_REQUEST_GENERATION={receipt['generation']},_REQUEST_SHA256={receipt['sha256']}", '--async')
    child=build['id'];require(re.fullmatch(UUID,child) is not None,'provisioning build UUID missing')
    deadline=time.monotonic()+600
    while True:
        b=cloud.json('builds','describe',child)
        if b.get('status') not in ['PENDING','QUEUED','WORKING']:break
        require(time.monotonic()<deadline,'Job provisioning timeout; inspect child build before retrying')
        time.sleep(5)
    require(b.get('status')=='SUCCESS' and b.get('serviceAccount')==PROVISIONER,'separate Job provisioner failed')
    job=cloud.json('run','jobs','describe',name)
    verify_job(job,r)
    return job


def verify_job(job,r):
    args=create_args(r);name=args[4];m=r['manifest'];meta=job.get('metadata',{})
    require(meta.get('name')==name and meta.get('namespace')=='447373903153' and re.fullmatch(UUID,meta.get('uid','')) is not None,'Job identity differs')
    template=job.get('spec',{}).get('template',{});spec=template.get('spec',{});task=spec.get('template',{}).get('spec',{})
    labels=template.get('metadata',{}).get('labels',{});ann=template.get('metadata',{}).get('annotations',{})
    require(labels.get('source-commit')==m['source_sha'] and labels.get('cloud-build-id')==r['build_id'],'Job source/build differs')
    require(json.loads(ann.get('run.googleapis.com/network-interfaces','null'))==[{'network':'espacigo-testprod','subnetwork':'espacigo-testprod-api-serverless'}] and ann.get('run.googleapis.com/vpc-access-egress')=='private-ranges-only','Job private network differs')
    require(spec.get('taskCount')==1 and spec.get('parallelism')==1 and task.get('maxRetries')==0 and str(task.get('timeoutSeconds'))=='600' and task.get('serviceAccountName')==SERVICE_ACCOUNT,'Job task controls differ')
    containers=task.get('containers',[]);require(len(containers)==1,'one Job container required');c=containers[0]
    desired=next(a for a in args if a.startswith('--args=')).removeprefix('--args=').split(',')
    require(c.get('image')==m['images']['migrator'] and c.get('command')==['/app/dbmigrate'] and c.get('args')==desired and c.get('resources',{}).get('limits')=={'cpu':'1','memory':'512Mi'},'Job image, args or resources differ')
    expected=dict(x.split('=',1) for x in next(a for a in args if a.startswith('--set-env-vars=')).removeprefix('--set-env-vars=^|^').split('|'))
    env=c.get('env',[]);require(len(env)==len(expected) and all(set(v)=={'name','value'} for v in env) and {v['name']:v['value'] for v in env}==expected,'Job environment differs')
    volumes=task.get('volumes',[]);mounts=c.get('volumeMounts',[])
    require(len(volumes)==1 and volumes[0].get('secret')=={'secretName':'espacigo-testprod-cloudsql-migrator-password','items':[{'key':'1','path':'password'}]} and mounts==[{'name':volumes[0]['name'],'mountPath':'/var/secrets/migration'}],'Job secret version/mount differs')


def execute_job(cloud,job,r,record):
    name=job['metadata']['name'];uid=job['metadata']['uid']
    # Record submission intent before the request. No retry of this call.
    record('job-intent-'+r['mode'],{'job':name,'uid':uid,'request':r})
    started=cloud.json('run','jobs','execute',name,'--async')
    execution=started.get('metadata',{}).get('name','')
    require(re.fullmatch(re.escape(name)+r'-[a-z0-9]{5}',execution) is not None,'exact execution ID missing; inspect Job, do not relaunch')
    record('job-start-'+r['mode'],started)
    deadline=time.monotonic()+660
    while True:
        e=cloud.json('run','jobs','executions','describe',execution)
        conditions=[c for c in e.get('status',{}).get('conditions',[]) if c.get('type')=='Completed']
        if conditions and conditions[0].get('status') in ['True','False']:break
        require(time.monotonic()<deadline,'Job outcome uncertain; inspect execution before retrying')
        time.sleep(5)
    record('job-finished-'+r['mode'],e)
    require(e.get('metadata',{}).get('labels',{}).get('run.googleapis.com/jobUid')==uid and e['metadata'].get('namespace')=='447373903153','execution ownership differs')
    require(e.get('spec',{}).get('template',{}).get('spec')==job['spec']['template']['spec']['template']['spec'],'effective execution config differs')
    status=e.get('status',{});require(len(conditions)==1 and conditions[0]['status']=='True' and status.get('succeededCount')==1 and all(status.get(k,0)==0 for k in ['failedCount','runningCount','retriedCount','cancelledCount']),'Job did not finish exactly once successfully')
    query=f'resource.type="cloud_run_job" AND resource.labels.job_name="{name}" AND resource.labels.location="{REGION}" AND labels."run.googleapis.com/execution_name"="{execution}" AND logName="projects/{PROJECT}/logs/run.googleapis.com%2Fstdout"'
    wanted='espacigo.migration-execution.v2' if r['mode']=='apply' else None
    deadline=time.monotonic()+90
    while True:
        logs=cloud.json('logging','read',query,'--limit=100')
        reports=[x for x in logs if isinstance(x.get('jsonPayload'),dict) and (x['jsonPayload'].get('format')==wanted if wanted else is_plan_report_shape(x['jsonPayload']))]
        if reports:break
        require(time.monotonic()<deadline,'execution succeeded but log is missing; do not replay SQL')
        time.sleep(5)
    require(len(reports)==1,'exactly one Job report required')
    for entry in reports:
        require(entry.get('resource',{}).get('labels')=={'project_id':PROJECT,'location':REGION,'job_name':name} and entry.get('labels',{}).get('run.googleapis.com/execution_name')==execution,'Job log identity differs')
    record('job-logs-'+r['mode'],logs)
    report=reports[0]['jsonPayload']
    if r['mode']=='apply':
        verified=verify_execution(r['manifest'],r['preflight'],r['build_id'],'1',uid,execution,e,logs)
        record('migration-verified',verified)
    else:check_plan(r['manifest'],report)
    return report


def remove_job(cloud,job,r):
    current=cloud.json('run','jobs','describe',job['metadata']['name'])
    require(current['metadata']['uid']==job['metadata']['uid'],'refusing cleanup of replaced Job')
    verify_job(current,r)
    cloud.run('run','jobs','delete',job['metadata']['name'])
