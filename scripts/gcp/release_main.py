#!/usr/bin/env python3
"""One main release: source quality -> lock -> images -> backup/SQL -> rollout.

No database password enters the builder. Native configuration is preserved.
A failed/uncertain release retains its owned lock and immutable audit records.
"""
from datetime import datetime,timezone,timedelta
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
import time

from verify_web_backend import Cloud,GuardError,PROJECT,REGION,BUCKET,REGISTRY,SA,UUID,HEX64,require,active_api
from rollout_web import TRIGGER,LOCK,WATERMARK,SERVICES,verify_private,ready_revision,promote_and_verify,execute as web_release
from migration_release_manifest import validate as validate_manifest,TARGET
from verify_migration_inventory import validate as validate_inventory
from schema_release_gate import MODULE as SCHEMA
from backup_readiness import check as backup_check
from release_jobs import request,provision,execute_job,remove_job

ALL_SERVICES=('espacigo-test-api',*SERVICES)


def source_inventory(root):
    result={}
    for directory,dirs,files in os.walk(root,followlinks=False):
        dirs[:]=[d for d in dirs if d not in {'.git','.local','node_modules','.next','__pycache__','.terraform'}]
        for f in files:
            p=Path(directory)/f;name=p.relative_to(root).as_posix()
            if name.endswith('.md') or name.startswith('planning/') and name!='planning/openapi.yaml':continue
            require(not p.is_symlink(),'source symlinks are not accepted by release classification')
            result[name]=hashlib.sha256(p.read_bytes()).hexdigest()
    require('go.mod' in result and 'cloudbuild.yaml' in result,'incomplete source inventory')
    return result


def classify(current,prior):
    previous=prior.get('source_inventory')
    if not isinstance(previous,dict):return 'full'
    changed={p for p in set(current)|set(previous) if current.get(p)!=previous.get(p)}
    if not changed:return 'docs'
    if all(p.startswith('frontend/') for p in changed):return 'web'
    return 'full'


def parent_identity(parent,sha,bid):
    require(re.fullmatch('[a-f0-9]{40}',sha) is not None and re.fullmatch(UUID,bid) is not None,'full source SHA/build UUID required')
    require(parent.get('id')==bid and parent.get('buildTriggerId')==TRIGGER and parent.get('serviceAccount')==SA and parent.get('status')=='WORKING','not the active canonical main trigger build')
    subs=parent.get('substitutions',{})
    require(subs.get('COMMIT_SHA')==sha and subs.get('BRANCH_NAME')=='main' and subs.get('REPO_FULL_NAME')=='HernanEspinozaDev/espaciGo' and subs.get('_DEPLOY_SCOPE')=='full','parent source/branch/repository/scope differs')
    return int(datetime.fromisoformat(parent['createTime'].replace('Z','+00:00')).timestamp())


def assert_lock(cloud,generation,text):
    require(str(cloud.json('storage','objects','describe',LOCK)['generation'])==generation and cloud.run('storage','cat',LOCK+'#'+generation)==text,'release lock ownership changed')


def write_once(cloud,work,uri,data):
    path=work/'transfer.json';text=json.dumps(data,sort_keys=True,separators=(',',':'))+'\n';path.write_text(text)
    cloud.run('storage','cp',path,uri,'--if-generation-match=0')
    generation=str(cloud.json('storage','objects','describe',uri)['generation'])
    require(cloud.run('storage','cat',uri+'#'+generation)==text,'immutable release artifact readback differs')
    return {'uri':uri,'generation':generation,'sha256':hashlib.sha256(text.encode()).hexdigest()}


def acquire_lock(cloud,path,text,bid,timeout=1800):
    deadline=time.monotonic()+timeout
    while True:
        try:
            cloud.run('storage','cp',path,LOCK,'--if-generation-match=0')
            generation=str(cloud.json('storage','objects','describe',LOCK)['generation'])
            assert_lock(cloud,generation,text)
            return generation
        except GuardError as error:
            if not any(t in str(error).lower() for t in ['412','precondition','conditionnotmet']):raise
            owner=json.loads(cloud.run('storage','cat',LOCK))
            require(isinstance(owner,dict) and re.fullmatch(UUID,owner.get('build_id','')) is not None and owner['build_id']!=bid,'lock owner requires diagnosis, never delete blindly')
            prior=cloud.json('builds','describe',owner['build_id'])
            require(prior.get('status') in ['PENDING','QUEUED','WORKING'],'prior release ended with retained lock; inspect its recorded outcome')
            require(time.monotonic()<deadline,'waiting for release lock timed out')
            time.sleep(10)


def retain_backups(cloud,current,now=None):
    now=now or datetime.now(timezone.utc)
    receipts=[]
    for uri in cloud.run('storage','ls',f'gs://{BUCKET}/releases/*/completed.json').splitlines():
        match=re.fullmatch(re.escape(f'gs://{BUCKET}/releases/')+'('+UUID+')/completed.json',uri.strip())
        require(match is not None,'unexpected release receipt resource')
        gen=str(cloud.json('storage','objects','describe',uri)['generation'])
        report=json.loads(cloud.run('storage','cat',uri+'#'+gen))
        require(report.get('format')=='espacigo.main-release.v1' and report.get('build_id')==match[1],'backup receipt identity differs')
        b=report.get('backup',{})
        if b.get('pending_sql') is True and b.get('backup_type')=='ON_DEMAND':
            require(b.get('instance')==TARGET['instance'] and re.fullmatch('[1-9][0-9]*',str(b.get('backup_id',''))) is not None,'backup receipt target differs')
            receipts.append((b,match[1]))
    receipts.sort(key=lambda r:r[0]['backup_start'],reverse=True)
    deleted=[]
    protected={str(current['backup_id'])}
    # Always retain seven newest successful owned release backups, plus seven
    # days. Failed-release/untracked/historical backups are never candidates.
    for b,bid in receipts[7:]:
        start=datetime.fromisoformat(b['backup_start'].replace('Z','+00:00'))
        if now-start<timedelta(days=7) or str(b['backup_id']) in protected:continue
        native=cloud.json('sql','backups','describe',str(b['backup_id']),'--instance='+TARGET['instance'])
        expected=f"https://sqladmin.googleapis.com/sql/v1beta4/projects/{PROJECT}/instances/{TARGET['instance']}/backupRuns/{b['backup_id']}"
        require(native.get('selfLink')==expected and native.get('instance')==TARGET['instance'] and native.get('description')=='espacigo-release-'+bid and native.get('type')=='ON_DEMAND' and native.get('status')=='SUCCESSFUL','refusing cleanup of an unowned backup')
        cloud.run('sql','backups','delete',str(b['backup_id']),'--instance='+TARGET['instance']);deleted.append(str(b['backup_id']))
    return {'policy':'seven newest successful owned backups and at least seven days','deleted_ids':deleted}


def image_build(cloud,root,sha,config,repositories):
    submitted=cloud.json('builds','submit',root,f'--config={root/config}',f'--ignore-file={root/".gcloudignore"}',f'--gcs-source-staging-dir=gs://{BUCKET}/source',f'--service-account={SA}',f'--substitutions=_COMMIT_SHA={sha}','--async')
    bid=submitted.get('id','');require(re.fullmatch(UUID,bid) is not None,'image build ID missing')
    print('Image build '+bid,flush=True);deadline=time.monotonic()+1800
    while True:
        b=cloud.json('builds','describe',bid)
        if b.get('status') not in ['WORKING','PENDING','QUEUED']:break
        require(time.monotonic()<deadline,'image build timeout; inspect child and retained lock')
        time.sleep(10)
    require(b.get('status')=='SUCCESS' and b.get('serviceAccount')==SA and b.get('substitutions',{}).get('_COMMIT_SHA')==sha,'image build failed or source identity differs')
    outputs=b.get('results',{}).get('images',[])
    require(len(outputs)==len(repositories),'unexpected image outputs')
    images={}
    for repo in repositories:
        match=[r for r in outputs if r.get('name')==f'{REGISTRY}/{repo}:sha-{sha}'];require(len(match)==1,'exact image build output missing')
        digest=match[0].get('digest','');require(re.fullmatch('sha256:'+HEX64,digest) is not None,'image digest missing')
        image=f'{REGISTRY}/{repo}@{digest}';p=cloud.json('artifacts','docker','images','describe',image,'--show-provenance')
        invocation=f'https://cloudbuild.googleapis.com/v1/projects/{PROJECT}/locations/{REGION}/builds/{bid}'
        statements=[x.get('build',{}).get('inTotoSlsaProvenanceV1',{}) for x in p.get('provenance_summary',{}).get('provenance',[])]
        require(p.get('image_summary',{}).get('digest')==digest and any(s.get('predicateType')=='https://slsa.dev/provenance/v1' and s.get('predicate',{}).get('runDetails',{}).get('metadata',{}).get('invocationId')==invocation and any(t.get('digest',{}).get('sha256')==digest[7:] for t in s.get('subject',[])) for s in statements),'matching immutable image provenance missing')
        images[repo]=image
    return images,bid


def migration_policy(root,preflight):
    p=json.loads((root/'db/release-policy.json').read_text())
    require(p.get('format')=='espacigo.initial-migration-policy.v1' and p.get('baseline')==83,'migration policy baseline differs')
    for m in preflight['pending']:
        require(p.get('automatic_migrations',{}).get(m['name'])=={'checksum':m['checksum'],'strategy':'expand'},'pending migration lacks exact reviewed expand policy: '+m['name'])


def backup(cloud,pending,locked_at,bid,record):
    instance=cloud.json('sql','instances','describe',TARGET['instance'])
    if pending:
        op=cloud.json('sql','backups','create','--instance='+TARGET['instance'],'--description=espacigo-release-'+bid,'--async')
        record('backup-intent',op);operation=op['name'];deadline=time.monotonic()+1800
        while True:
            o=cloud.json('sql','operations','describe',operation)
            if o.get('status')=='DONE':break
            require(time.monotonic()<deadline,'backup outcome uncertain; do not apply SQL')
            time.sleep(10)
        record('backup-operation',o);require(not o.get('error'),'on-demand backup failed')
        backups=cloud.json('sql','backups','list','--instance='+TARGET['instance'])
        selected=[b for b in backups if b.get('description')=='espacigo-release-'+bid and b.get('type')=='ON_DEMAND' and b.get('status')=='SUCCESSFUL']
        require(len(selected)==1,'exact successful release backup missing')
        chosen=selected[0]
    else:
        backups=cloud.json('sql','backups','list','--instance='+TARGET['instance'],'--limit=20')
        successful=[b for b in backups if b.get('status')=='SUCCESSFUL'];require(bool(successful),'no successful backup')
        chosen=max(successful,key=lambda b:b.get('startTime',''))
    native=cloud.json('sql','backups','describe',str(chosen['id']),'--instance='+TARGET['instance'])
    snapshot={'captured_at':datetime.now(timezone.utc).isoformat(),'instance':instance,'backup':native,'release_lock_acquired_at':locked_at}
    receipt=backup_check(snapshot,pending);record('backup',{'snapshot':snapshot,'verified':receipt})
    return receipt


def semantic_template(service):
    template=service['spec']['template'];spec=json.loads(json.dumps(template['spec']))
    for c in spec.get('containers',[]):c.pop('image',None)
    annotations={k:v for k,v in template.get('metadata',{}).get('annotations',{}).items() if k not in ['run.googleapis.com/client-name','run.googleapis.com/client-version','run.googleapis.com/operation-id']}
    return {'spec':spec,'annotations':annotations}


def snapshot_services(cloud):
    result={}
    for service in ALL_SERVICES:
        d=cloud.json('run','services','describe',service);t=d['spec']['template'];c=t['spec']['containers']
        require(len(c)==1,'one application container required')
        expected_sa=('espacigo-test-api' if service==ALL_SERVICES[0] else 'espacigo-test-public-web' if service==SERVICES[0] else 'espacigo-test-private-web')+'@'+PROJECT+'.iam.gserviceaccount.com'
        require(t['spec'].get('serviceAccountName')==expected_sa,'application runtime identity differs')
        port=8080 if service==ALL_SERVICES[0] else 4000;path='/health/ready' if port==8080 else '/healthz'
        require(c[0].get('startupProbe',{}).get('httpGet')=={'path':path,'port':port},'application startup readiness contract differs')
        traffic=[x for x in d.get('status',{}).get('traffic',[]) if x.get('percent',0)]
        require(len(traffic)==1 and traffic[0].get('percent')==100,'single baseline revision at 100% required')
        result[service]={'metadata':d,'revision':traffic[0]['revisionName'],'template':semantic_template(d)}
    verify_private(cloud)
    return result


def rollout(cloud,images,sha,bid,baseline,record):
    revisions={};promoted=[]
    try:
        for service in ALL_SERVICES:
            image=images['api'] if service==ALL_SERVICES[0] else images['web']
            revision=service+'-b'+bid.replace('-','')[:24]
            cloud.run('run','deploy',service,'--image='+image,'--no-traffic','--revision-suffix=b'+bid.replace('-','')[:24],'--update-labels=source-commit='+sha+',cloud-build-id='+bid,timeout=420)
            ready_revision(cloud.json('run','revisions','describe',revision),image,sha,bid)
            current=cloud.json('run','services','describe',service)
            require(semantic_template(current)==baseline[service]['template'],'service configuration changed during image rollout')
            revisions[service]=revision;record('ready-'+service,{'revision':revision,'image':image})
        verify_private(cloud)
        for service in ALL_SERVICES:
            promoted.append(service);promote_and_verify(cloud,service,revisions[service])
        verify_private(cloud)
        return revisions
    except Exception as original:
        record('application-failure',{'promotion_attempts':promoted,'error':str(original),'previous_revisions':{k:v['revision'] for k,v in baseline.items()}})
        # Only expand changes are admitted. Never roll back SQL. Revert traffic
        # to the recorded compatible revisions if promotion partially occurred.
        rollback_errors={}
        for service in reversed(promoted):
            try:promote_and_verify(cloud,service,baseline[service]['revision'])
            except Exception as error:rollback_errors[service]=str(error)
        record('application-rollback',{'attempted':promoted,'errors':rollback_errors})
        raise


def execute(cloud,root,sha,bid):
    epoch=parent_identity(cloud.json('builds','describe',bid),sha,bid)
    inventory=source_inventory(root)
    prior=json.loads(cloud.run('storage','cat',WATERMARK));scope=classify(inventory,prior)
    if scope=='docs':
        print('DOCS ONLY: no lock, Job, SQL or rollout',flush=True);return {'scope':'docs','source_sha':sha}
    if scope=='web':return web_release(cloud,root,sha,bid,source_inventory=inventory,allow_full_parent=True)
    work=root/'.local'/'release'/bid;work.mkdir(parents=True,mode=0o700);os.chmod(work,0o700)
    lock_text=json.dumps({'source_sha':sha,'build_id':bid},sort_keys=True)+'\n';lock_file=work/'lock.json';lock_file.write_text(lock_text)
    # A busy/failed release fails closed. No orphan lock is removed automatically.
    generation=acquire_lock(cloud,lock_file,lock_text,bid)
    locked_at=datetime.now(timezone.utc).isoformat()
    records={}
    def record(name,data):
        assert_lock(cloud,generation,lock_text)
        records[name]=write_once(cloud,work,f'gs://{BUCKET}/releases/{bid}/{name}.json',data)
        return records[name]
    try:
        prior_generation=str(cloud.json('storage','objects','describe',WATERMARK)['generation'])
        prior=json.loads(cloud.run('storage','cat',WATERMARK+'#'+prior_generation))
        require(type(prior.get('created_epoch')) is int and epoch>prior['created_epoch'],'stale release older than successful watermark')
        baseline=snapshot_services(cloud);record('baseline',baseline)
        outputs,app_build=image_build(cloud,root,sha,'cloudbuild-images.yaml',['espacigo-api','espacigo-web'])
        migrator,migration_build=image_build(cloud,root,sha,'cloudbuild-migrator-image.yaml',['espacigo-db-migrator'])
        entries=SCHEMA.migration_inventory(root/'db/migrations');fingerprint=hashlib.sha256(''.join(f"{m['checksum']}  {m['name']}\n" for m in entries).encode()).hexdigest()
        mi={'format':'espacigo.migrations.inventory.v1','inventory_sha256':fingerprint,'migrations':entries};validate_inventory(mi)
        manifest=validate_manifest({'format':'espacigo.database-release.v1','source_sha':sha,'target':TARGET.copy(),'images':{'api':outputs['espacigo-api'],'web':outputs['espacigo-web'],'migrator':migrator['espacigo-db-migrator']},'migration_inventory':mi});record('manifest',manifest)
        plan_request=request(manifest,bid,'plan');plan_job=provision(cloud,root,plan_request,record)
        plan=execute_job(cloud,plan_job,plan_request,record);record('preflight',plan)
        migration_policy(root,plan)
        receipt=backup(cloud,bool(plan['pending']),locked_at,bid,record)
        apply_request=request(manifest,bid,'apply',plan);apply_job=provision(cloud,root,apply_request,record)
        result=execute_job(cloud,apply_job,apply_request,record);record('postcheck',result)
        assert_lock(cloud,generation,lock_text)
        revisions=rollout(cloud,manifest['images'],sha,bid,baseline,record)
        report={'format':'espacigo.main-release.v1','source_sha':sha,'build_id':bid,'created_epoch':epoch,'source_inventory':inventory,'images':manifest['images'],'image_build_id':app_build,'migration_image_build_id':migration_build,'migration_execution':records['migration-verified'],'backup':receipt,'revisions':revisions,'schema_migrations':len(entries),'evidence':records.copy(),'product_qa':'reserved for user; not inferred from technical deployment'}
        record('completed',report)
        try:retention=retain_backups(cloud,receipt)
        except Exception as error:retention={'status':'deferred','reason':str(error)}
        report['backup_retention']=retention
        record('backup-retention',retention)
        record('success',report)
        remove_job(cloud,plan_job,plan_request);remove_job(cloud,apply_job,apply_request)
        assert_lock(cloud,generation,lock_text)
        path=work/'watermark.json';text=json.dumps(report,sort_keys=True)+'\n';path.write_text(text)
        cloud.run('storage','cp',path,WATERMARK,'--if-generation-match='+prior_generation)
        updated=str(cloud.json('storage','objects','describe',WATERMARK)['generation']);require(cloud.run('storage','cat',WATERMARK+'#'+updated)==text,'watermark readback differs')
        assert_lock(cloud,generation,lock_text);cloud.run('storage','rm',LOCK,'--if-generation-match='+generation)
        print('MAIN RELEASE VERIFIED '+json.dumps({'sha':sha,'build_id':bid,'revisions':revisions,'schema':len(entries)}),flush=True)
        return report
    except Exception as error:
        # Never relaunch/cancel a migration or delete its lock on uncertainty.
        try:record('failure',{'error':str(error),'build_id':bid,'source_sha':sha,'lock_generation':generation,'records':records.copy()})
        except Exception:pass
        raise


def main():
    p=argparse.ArgumentParser();p.add_argument('--execute',action='store_true');p.add_argument('--sha',required=True);p.add_argument('--build-id',required=True);a=p.parse_args()
    if not a.execute:p.error('explicit Cloud Build execution required')
    try:execute(Cloud(),Path(__file__).resolve().parents[2],a.sha,a.build_id)
    except (ValueError,OSError,KeyError,TypeError) as e:p.exit(1,'Main release failed; inspect recorded execution and owned lock: '+str(e)+'\n')


if __name__=='__main__':main()
