#!/usr/bin/env python3
"""Render a pinned applying Job or verify its captured result; never invokes cloud."""
import argparse
from datetime import datetime,timedelta,timezone
import json
from pathlib import Path
import re
from migration_job_plan import PROJECT,REGION,SECRET,SERVICE_ACCOUNT,proposal as plan_proposal
from migration_release_manifest import TARGET,check_plan,validate

UUID=r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}'


def contract(manifest,preflight):
    checked=check_plan(manifest,preflight)
    if checked['applied']<83:raise ValueError('release cannot bootstrap or repair a ledger older than V83')
    return {'source_sha':manifest['source_sha'],'inventory_sha256':manifest['migration_inventory']['inventory_sha256'],'expected_applied':checked['applied'],'database':TARGET['database']}


def proposal(manifest,preflight,build_id,secret_version):
    validate(manifest)
    if not isinstance(build_id,str) or not re.fullmatch(UUID,build_id):raise ValueError('exact parent Cloud Build UUID required')
    pinned=contract(manifest,preflight)
    base=plan_proposal(manifest['images']['migrator'],manifest['source_sha'],secret_version)
    job='espacigo-db-release-'+build_id
    argv=base['proposed_job_create_argv'].copy();argv[4]=job
    args=['-dir','/app/db/migrations','-release','-release-source-sha',pinned['source_sha'],'-release-inventory-sha256',pinned['inventory_sha256'],'-release-expected-applied',str(pinned['expected_applied'])]
    argv=[('--args='+','.join(args) if x.startswith('--args=') else '--labels=source-commit='+manifest['source_sha']+',cloud-build-id='+build_id+',purpose=flujo-migration-release' if x.startswith('--labels=') else x) for x in argv]
    return {'format':'espacigo.migration-application-proposal.v1','executed':False,'job':job,'contract':pinned,'create_argv':argv,'args':args,'execute_argv':['gcloud','run','jobs','execute',job,f'--project={PROJECT}',f'--region={REGION}','--wait','--format=json'],'prerequisites':['Release-wide generation-owned lock held until postcheck and application rollout','Same-SHA quality and API/web/migrator build provenance verified','Successful recent backup and recovery policy checked before application','Review DDL authority and backwards compatibility for every pending migration','Job name absent, dedicated provisioner separate from no-override execution identity','After submission retain execution ID; do not retry/cancel blindly if parent fails'],'limits':['Not wired into the active trigger','No IAM grants or resources created by this renderer','Structured execution evidence supplements existing IMG-01 catalog evidence; does not replace it']}


def parse_time(value):
    try:t=datetime.fromisoformat(value.replace('Z','+00:00'))
    except (AttributeError,ValueError):raise ValueError('execution timestamps are invalid') from None
    if t.tzinfo is None:raise ValueError('execution timestamp needs timezone')
    return t


def verify_execution(manifest,preflight,build_id,secret_version,job_uid,execution_name,execution,logs,now=None):
    expected=proposal(manifest,preflight,build_id,secret_version)
    if not re.fullmatch(UUID,job_uid) or not re.fullmatch(re.escape(expected['job'])+r'-[a-z0-9]{5}',execution_name):raise ValueError('created Job UID and exact execution name required')
    if execution.get('apiVersion')!='run.googleapis.com/v1' or execution.get('kind')!='Execution':raise ValueError('unsupported execution metadata')
    meta=execution.get('metadata',{});labels=meta.get('labels',{});annotations=meta.get('annotations',{})
    if meta.get('name')!=execution_name or meta.get('namespace')!='447373903153' or labels.get('run.googleapis.com/job')!=expected['job'] or labels.get('run.googleapis.com/jobUid')!=job_uid or labels.get('source-commit')!=manifest['source_sha'] or labels.get('cloud-build-id')!=build_id or labels.get('cloud.googleapis.com/location')!=REGION:
        raise ValueError('execution belongs to another Job, region, project, build or source')
    if not any(o.get('kind')=='Job' and o.get('name')==expected['job'] and o.get('uid')==job_uid and o.get('controller') is True for o in meta.get('ownerReferences',[])):raise ValueError('execution is not owned by the created Job')
    try:networks=json.loads(annotations.get('run.googleapis.com/network-interfaces','null'))
    except (ValueError,TypeError):raise ValueError('invalid private network metadata') from None
    if networks!=[{'network':'espacigo-testprod','subnetwork':'espacigo-testprod-api-serverless'}] or annotations.get('run.googleapis.com/vpc-access-egress')!='private-ranges-only':raise ValueError('execution private network differs')
    spec=execution.get('spec',{});task=spec.get('template',{}).get('spec',{})
    if any(type(spec.get(k)) is not int or spec[k]!=1 for k in ['taskCount','parallelism']) or type(task.get('maxRetries')) is not int or task['maxRetries']!=0 or str(task.get('timeoutSeconds'))!='600' or task.get('serviceAccountName')!=SERVICE_ACCOUNT:raise ValueError('execution task or identity controls differ')
    containers=task.get('containers',[])
    if len(containers)!=1:raise ValueError('exactly one migration container required')
    c=containers[0]
    if c.get('image')!=manifest['images']['migrator'] or c.get('command')!=['/app/dbmigrate'] or c.get('args')!=expected['args']:raise ValueError('execution image, command or pinned arguments differ')
    if c.get('resources',{}).get('limits')!={'cpu':'1','memory':'512Mi'}:raise ValueError('execution resource limits differ')
    env_flag=next(a for a in expected['create_argv'] if a.startswith('--set-env-vars='))
    desired=dict(item.split('=',1) for item in env_flag.removeprefix('--set-env-vars=^|^').split('|'))
    if c.get('env') is None or len(c['env'])!=len(desired) or any(set(e)!={'name','value'} for e in c['env']) or {e['name']:e['value'] for e in c['env']}!=desired:raise ValueError('execution connection environment differs')
    mounts=c.get('volumeMounts',[]);volumes=task.get('volumes',[])
    if len(mounts)!=1 or mounts[0].get('mountPath')!='/var/secrets/migration' or len(volumes)!=1 or volumes[0].get('name')!=mounts[0].get('name') or volumes[0].get('secret')!={'secretName':SECRET,'items':[{'key':secret_version,'path':'password'}]}:raise ValueError('execution secret mount differs')
    status=execution.get('status',{})
    completed=[c for c in status.get('conditions',[]) if c.get('type')=='Completed']
    if len(completed)!=1 or str(completed[0].get('status'))!='True' or type(status.get('succeededCount')) is not int or status['succeededCount']!=1 or any(type(status.get(k,0)) is not int or status.get(k,0)!=0 for k in ['failedCount','runningCount','cancelledCount','retriedCount']):raise ValueError('execution is incomplete, retried or unsuccessful')
    now=now or datetime.now(timezone.utc);start=parse_time(meta.get('creationTimestamp'));end=parse_time(status.get('completionTime'))
    if not start<=end<=now+timedelta(seconds=120) or now-start>timedelta(minutes=30) or end-start>timedelta(minutes=20):raise ValueError('execution evidence is stale or has invalid timing')
    reports=[]
    for entry in logs:
        report=entry.get('jsonPayload')
        if not isinstance(report,dict) or report.get('format')!='espacigo.migration-execution.v1':continue
        resource=entry.get('resource',{})
        if resource.get('type')!='cloud_run_job' or resource.get('labels')!={'project_id':PROJECT,'location':REGION,'job_name':expected['job']} or entry.get('labels',{}).get('run.googleapis.com/execution_name')!=execution_name or entry.get('logName')!=f'projects/{PROJECT}/logs/run.googleapis.com%2Fstdout':raise ValueError('release report comes from a different execution/log stream')
        if not start<=parse_time(entry.get('timestamp'))<=end+timedelta(seconds=120):raise ValueError('release log timestamp differs from execution')
        reports.append(report)
    if len(reports)!=1:raise ValueError('exactly one structured success report required; wait for logs without rerunning SQL')
    report=reports[0]
    if set(report)!={'format','contract','before','after','applied'} or report['contract']!=expected['contract'] or report['before']!=preflight:raise ValueError('release report differs from reviewed preflight/contract')
    check_plan(manifest,report['after'],True)
    if report['applied']!=[m['version'] for m in preflight['pending']]:raise ValueError('release applied set differs from reviewed pending suffix')
    return {'format':'espacigo.verified-migration-execution.v1','source_sha':manifest['source_sha'],'build_id':build_id,'job':expected['job'],'job_uid':job_uid,'execution':execution_name,'image':manifest['images']['migrator'],'completed_at':status['completionTime'],'inventory_sha256':expected['contract']['inventory_sha256'],'applied':report['applied'],'pending':0}


def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('action',choices=['propose','verify'])
    for name in ['manifest','preflight']:p.add_argument('--'+name,type=Path,required=True)
    for name in ['build-id','secret-version']:p.add_argument('--'+name,required=True)
    for name in ['execution','logs']:p.add_argument('--'+name,type=Path)
    p.add_argument('--execution-name');p.add_argument('--job-uid')
    a=p.parse_args()
    try:
        m=json.loads(a.manifest.read_text());before=json.loads(a.preflight.read_text())
        if a.action=='propose':result=proposal(m,before,a.build_id,a.secret_version)
        else:
            if any(x is None for x in [a.execution,a.logs,a.execution_name,a.job_uid]):raise ValueError('verification requires execution/log files, exact name and created Job UID')
            result=verify_execution(m,before,a.build_id,a.secret_version,a.job_uid,a.execution_name,json.loads(a.execution.read_text()),json.loads(a.logs.read_text()))
    except (ValueError,OSError,TypeError,KeyError) as error:p.exit(1,'migration execution rejected: '+str(error)+'\n')
    print(json.dumps(result,indent=2))


if __name__=='__main__':main()
