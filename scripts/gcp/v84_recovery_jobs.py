#!/usr/bin/env python3
"""Create, execute, validate and remove exactly one V84 Cloud Run Job."""
from __future__ import annotations
import hashlib
import copy
import json
import re
import time
from datetime import datetime
from pathlib import Path

import recovery_inspection as inspection
import v84_recovery_operator as operator
from verify_web_backend import PROJECT, REGION


def _need(ok, message):
    if not ok:
        raise ValueError(message)


def _record(directory, name, value):
    return operator.write_receipt(directory, name, value)


def _inventory(directory):
    rows = inspection.local_migrations(Path(__file__).resolve().parents[2])
    raw = ''.join(x['checksum'] + '  ' + x['name'] + '\n' for x in rows).encode()
    return rows, hashlib.sha256(raw).hexdigest()


def _missing_job(error, name):
    return str(error) == f'gcloud run jobs failed: ERROR: (gcloud.run.jobs.describe) Cannot find job [{name}].'


def _assert_job_absent(cloud, name):
    try:
        cloud.json('run', 'jobs', 'describe', name)
    except ValueError as error:
        _need(_missing_job(error, name), 'Job lookup did not return the exact not-found response')
        return
    raise ValueError('Job name already exists; refusing reuse')


def _desired(mode, image, sha, run_id, build_id, baseline):
    return operator.expected_job_spec(mode, image, sha, run_id, build_id, baseline)


def _create_argv(d):
    # Keep Cloud Run's job-level options before the named container group.
    env = '^|^' + '|'.join(f'{k}={v}' for k, v in d['env'].items())
    return ['run', 'jobs', 'create', d['name'],
            '--tasks=1', '--parallelism=1', '--max-retries=0', '--task-timeout=600s',
            f"--network={d['network']}", f"--subnet={d['subnetwork']}", f"--vpc-egress={d['vpc_egress']}",
            f"--service-account={d['service_account']}",
            f"--labels=source-commit={d['source_sha']},cloud-build-id={d['build_id']},purpose=v84-recovery",
            '--container=v84', f"--image={d['image']}",
            '--cpu=1', '--memory=512Mi', '--command=/app/dbmigrate', '--args=' + ','.join(d['args']),
            '--set-env-vars=' + env, f"--set-secrets={d['mount']}/password={d['secret']}:{d['secret_version']}"]


def _expected_task(d):
    return {'containers': [{'name': 'v84', 'image': d['image'], 'command': d['command'], 'args': d['args'],
                            'env': [{'name': k, 'value': v} for k, v in sorted(d['env'].items())],
                            'resources': {'limits': {'cpu': '1', 'memory': '512Mi'}},
                            'volumeMounts': [{'name': 'secret', 'mountPath': d['mount']}]}],
            'volumes': [{'name': 'secret', 'secret': {'secretName': d['secret'],
                        'items': [{'key': '1', 'path': 'password'}]}}],
            'serviceAccountName': d['service_account'], 'timeoutSeconds': '600', 'maxRetries': 0}


def _verify_task(task, d):
    actual = copy.deepcopy(task)
    _need(isinstance(actual, dict), 'task must be an object')
    if 'sandboxes' in actual:
        _need(actual.pop('sandboxes') == {}, 'unexpected sandbox controls')
    containers, volumes = actual.get('containers', []), actual.get('volumes', [])
    _need(len(containers) == len(volumes) == 1, 'one container and secret volume required')
    name = volumes[0].get('name')
    _need(isinstance(name, str) and bool(name)
          and containers[0].get('volumeMounts') == [{'name': name, 'mountPath': d['mount']}],
          'exact secret volume mount required')
    volumes[0]['name'] = 'secret'
    containers[0]['volumeMounts'][0]['name'] = 'secret'
    actual['timeoutSeconds'] = str(actual.get('timeoutSeconds'))
    containers[0]['env'] = sorted(containers[0].get('env', []), key=lambda row: row.get('name', ''))
    _need(actual == _expected_task(d), 'complete effective task specification differs')


def _verify_execution_spec(spec, d):
    _need(isinstance(spec, dict) and set(spec) == {'taskCount', 'parallelism', 'template'}
          and spec['taskCount'] == 1 and spec['parallelism'] == 1
          and isinstance(spec['template'], dict) and set(spec['template']) == {'spec'},
          'execution task controls differ')
    _verify_task(spec['template']['spec'], d)


def _verify(job, d):
    """Verify all security-relevant fields, rejecting unknown task/container fields."""
    m = job.get('metadata', {})
    _need(m.get('name') == d['name'] and m.get('namespace') == '447373903153' and re.fullmatch(operator.UUID_RE, m.get('uid','')),
          'Job name/namespace/UID differs')
    t = job.get('spec', {}).get('template', {})
    labels = t.get('metadata', {}).get('labels', {})
    _need(labels.get('source-commit') == d['source_sha'] and labels.get('cloud-build-id') == d['build_id'] and labels.get('purpose') == 'v84-recovery', 'Job provenance labels differ')
    anns = t.get('metadata', {}).get('annotations', {})
    _need(json.loads(anns.get('run.googleapis.com/network-interfaces','null')) == [{'network':d['network'],'subnetwork':d['subnetwork']}] and anns.get('run.googleapis.com/vpc-access-egress') == d['vpc_egress'], 'Job network differs')
    _verify_execution_spec(t.get('spec'), d)
    return m['uid']


def _validate_report(report, mode, sha, invhash):
    _need(isinstance(report,dict), 'stdout report is not an object')
    if mode.startswith('admin-'):
        expected_mode = mode.removeprefix('admin-')
        _need(report.get('format')=='espacigo.v84-authority.v1' and report.get('mode')==expected_mode, 'V84 authority report format/mode differs')
        _need(set(report) == {'format', 'mode', 'database', 'session_user', 'current_user', 'applied_count',
                             'baseline_sha256', 'authority_closed'}, 'authority report shape differs')
        _need(report.get('database') == operator.DB and report.get('session_user') == 'postgres'
              and report.get('current_user') == 'postgres', 'authority report target/session differs')
        _need(type(report.get('applied_count')) is int and report['applied_count'] in (83, 84)
              and isinstance(report.get('baseline_sha256'), str)
              and operator.HEX64.fullmatch(report['baseline_sha256']) is not None
              and type(report.get('authority_closed')) is bool, 'authority report facts invalid')
        if mode == 'admin-prepare':
            _need(report['applied_count'] == 83 and report['authority_closed'] is False, 'prepare boundary differs')
        if mode == 'admin-inspect':
            _need(report['applied_count'] == 83 and report['authority_closed'] is True, 'initial inspection boundary differs')
        if mode == 'admin-finalize':
            _need(report['authority_closed'] is True, 'finalize did not close temporary authority')
        return report
    if mode == 'inspect':
        rows = inspection.local_migrations(Path(__file__).resolve().parents[2])
        _need(report.get('format') == 'espacigo.recovery-inspection.v1', 'inspection report format differs')
        inspection.validate_report(report, rows)
    elif mode == 'catalog':
        from schema_release_gate import validate_catalog
        _need(report.get('format') == 'espacigo.img01.schema-evidence.v1', 'catalog report format differs')
        validate_catalog(report)
    elif mode == 'migrate':
        from migration_release_manifest import validate_plan_contract
        _need(set(report) == {'format', 'contract', 'before', 'after', 'applied', 'schema_evidence'}, 'release report shape differs')
        _need(report.get('format') == 'espacigo.migration-execution.v2', 'migration report format differs')
        _need(report.get('contract') == {'source_sha':sha, 'inventory_sha256':invhash, 'database':'espacigo_dms_stage_20261020', 'expected_applied':83} and report.get('applied') == [84], 'migration execution contract differs')
        inventory = inspection.local_migrations(Path(__file__).resolve().parents[2])
        _need(len(inventory) == 84, 'migration inventory length differs')
        for field, count in (('before',83),('after',84)):
            plan = report.get(field); validate_plan_contract(plan)
            _need(plan.get('database') == operator.DB and plan.get('read_only') is True
                  and plan.get('history_exists') is True and plan.get('session_user') == 'espacigo_migrator_login'
                  and plan.get('current_user') == 'espacigo_schema_owner'
                  and plan.get('applied') == inventory[:count] and plan.get('pending') == inventory[count:],
                  f'{field} plan identity or inventory differs')
        from schema_release_gate import validate_catalog
        validate_catalog(report['schema_evidence'])
    else:
        raise ValueError('unsupported report mode')
    return report


def _timestamp(s):
    _need(isinstance(s,str),'execution timestamp missing')
    t=datetime.fromisoformat(s.replace('Z','+00:00'))
    _need(t.tzinfo is not None,'execution timestamp must include timezone')
    return t


def run_job(cloud, directory, run_id, sha, image, image_build_id, mode, baseline=None) -> dict:
    """Run once only; all uncertain/failed outcomes retain the Job and receipts."""
    root=Path(__file__).resolve().parents[2]
    _need(mode in operator.MODES and re.fullmatch(operator.UUID_RE,run_id) and re.fullmatch(operator.SHA_RE,sha) and re.fullmatch(operator.UUID_RE,image_build_id), 'invalid V84 run identity/mode')
    d=_desired(mode,image,sha,run_id,image_build_id,baseline)
    rows, invhash=_inventory(root)
    _need(len(rows)==84 and invhash==operator.INVENTORY84, 'source inventory is not the reviewed V84 set')
    _need(rows[-1]['version']==84,'V84 migration missing')
    supplied = Path(directory)
    _need(not supplied.is_symlink(), 'receipt directory cannot be a symlink')
    directory = supplied.resolve()
    _need(directory == operator.receipt_dir(run_id, root).resolve(), 'supplied receipt directory differs from this run')
    operator.verify_image_build(cloud,image,image_build_id,sha)
    intention={'format':'espacigo.v84-job-intent.v1','mode':mode,'run_id':run_id,'sha':sha,'image':image,'image_build_id':image_build_id,'job':d['name'],'created_at':datetime.now().astimezone().isoformat()}
    _record(directory,'job-intent-'+mode,intention)
    # Check exact name before create; never adopt/reuse a pre-existing Job.
    try:
        cloud.json('run','jobs','describe',d['name'])
    except ValueError as error:
        _need(_missing_job(error, d['name']), 'Job lookup did not return exact not-found response')
    else:
        raise ValueError('Job name already exists; refusing reuse')
    cloud.run(*_create_argv(d))
    job=cloud.json('run','jobs','describe',d['name'])
    uid=_verify(job,d)
    _record(directory,'job-created-'+mode,{'job':d['name'],'uid':uid,'desired':d})
    # Intent is durable before the single execute request. Never retry this call.
    _record(directory,'execution-intent-'+mode,{'job':d['name'],'uid':uid,'mode':mode})
    started=cloud.json('run','jobs','execute',d['name'],'--async')
    execution=started.get('metadata',{}).get('name','')
    _need(re.fullmatch(re.escape(d['name'])+r'-[a-z0-9]{5}',execution),'execution name missing; preserve resources and do not retry')
    _record(directory,'execution-start-'+mode,started)
    deadline=time.monotonic()+720
    while True:
        e=cloud.json('run','jobs','executions','describe',execution)
        conditions=[x for x in e.get('status',{}).get('conditions',[]) if x.get('type')=='Completed']
        if len(conditions)==1 and conditions[0].get('status') in ('True','False'): break
        _need(time.monotonic()<deadline,'execution outcome uncertain; preserve Job and do not retry')
        time.sleep(5)
    _record(directory,'execution-finished-'+mode,e)
    metadata = e.get('metadata', {})
    _need(metadata.get('name') == execution and operator.UUID_RE.fullmatch(metadata.get('uid', '')) is not None
          and metadata.get('namespace') == '447373903153'
          and metadata.get('labels', {}).get('run.googleapis.com/jobUid') == uid, 'execution does not belong to created Job')
    status=e.get('status',{})
    _need(conditions[0].get('status')=='True' and status.get('succeededCount')==1 and all(status.get(k,0)==0 for k in ('failedCount','runningCount','retriedCount','cancelledCount')), 'Job execution failed or retried')
    _verify_execution_spec(e.get('spec'), d)
    query=f'resource.type="cloud_run_job" AND resource.labels.job_name="{d["name"]}" AND resource.labels.location="{REGION}" AND labels."run.googleapis.com/execution_name"="{execution}" AND logName="projects/{PROJECT}/logs/run.googleapis.com%2Fstdout"'
    deadline = time.monotonic() + 90
    while True:
        logs = cloud.json('logging', 'read', query, '--limit=100')
        _need(isinstance(logs, list) and len(logs) < 100, 'stdout log selection is truncated or invalid')
        candidates = [row for row in logs if isinstance(row.get('jsonPayload'), dict)]
        if candidates:
            break
        _need(time.monotonic() < deadline, 'report absent after log observation; do not replay execution')
        time.sleep(5)
    _need(len(candidates)==1,'expected exactly one recognized report in the execution stdout; do not replay')
    entry=candidates[0]
    _need(entry.get('resource', {}).get('type') == 'cloud_run_job' and entry.get('resource',{}).get('labels')=={'project_id':PROJECT,'location':REGION,'job_name':d['name']} and entry.get('labels',{}).get('run.googleapis.com/execution_name')==execution and entry.get('logName')==f'projects/{PROJECT}/logs/run.googleapis.com%2Fstdout','report log resource/execution differs')
    _need(_timestamp(status.get('startTime')) <= _timestamp(entry.get('timestamp')) <= _timestamp(status.get('completionTime')), 'report timestamp outside execution')
    report=_validate_report(entry['jsonPayload'],mode,sha,invhash)
    _record(directory,'report-'+mode,report)
    # Cleanup is limited to this exact name and UID; persist intention first.
    current=cloud.json('run','jobs','describe',d['name'])
    _need(_verify(current,d)==uid,'own Job was replaced; preserve it')
    _record(directory,'cleanup-intent-'+mode,{'job':d['name'],'uid':uid,'action':'delete-own-job'})
    cloud.run('run','jobs','delete',d['name'])
    _assert_job_absent(cloud, d['name'])
    _record(directory,'cleanup-verified-'+mode,{'job':d['name'],'uid':uid,'absent':True})
    return {'mode':mode,'report':report,'job':d['name'],'job_uid':uid,'execution':execution,'cleanup_verified':True}
