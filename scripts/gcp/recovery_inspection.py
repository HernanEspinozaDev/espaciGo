#!/usr/bin/env python3
"""Private READ ONLY recovery inspection; never releases the failed release lock."""
from __future__ import annotations
import argparse
from datetime import datetime
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import tarfile
import tempfile
import time

from migration_job_plan import proposal as plan_proposal
from verify_web_backend import PROJECT, REGION, BUCKET, SA as DEPLOYER, UUID, HEX64
from rollout_web import LOCK, TRIGGER
from recovery_observation_v2 import HistoricalExecutionReader, ObservationError
from recovery_parent_proof import (make_failed_release_proof as _make_parent_proof, validate_request_proof, canonical_proof_sha256)
from recovery_iam_gate import wait_for_iam_ready
from recovery_iam_policy import build_binding

PROJECT_NUMBER = '447373903153'
PROVISIONER = f'projects/{PROJECT}/serviceAccounts/espacigo-release-job-builder@{PROJECT}.iam.gserviceaccount.com'
REGISTRY = f'{REGION}-docker.pkg.dev/{PROJECT}/{PROJECT}'
DB_SA = f'espacigo-db-migrator@{PROJECT}.iam.gserviceaccount.com'
SDK_IMAGE = 'gcr.io/google.com/cloudsdktool/google-cloud-cli:stable@sha256:1855ee6031f03bb7d4b251cfd0e5b88a18fd9a2552a742cf86d269810832c80b'
PINNED_BUILD_ID = '2dc002a9-5831-4717-8064-6432b98d3e01'
PINNED_SOURCE_SHA = 'bb10309129adad9d7204b9b54bb2259186a34242'
PINNED_GENERATION = '1794036269394395'
HEX = HEX64


class RecoveryError(ValueError):
    pass


def require(ok, message):
    if not ok:
        raise RecoveryError(message)


def job_name(bid):
    require(re.fullmatch(UUID, bid) is not None, 'invalid inspection build ID')
    return 'espacigo-db-i-' + bid.replace('-', '')


def parent_step(sha, bid, digest):
    script = (f'[[ "true" == "true" ]]\n'
              f'export PYTHONDONTWRITEBYTECODE=1\n'
              '[[ -s /usr/lib/google-cloud-sdk/lib/third_party/certifi/cacert.pem ]]\n'
              'export SSL_CERT_FILE=/usr/lib/google-cloud-sdk/lib/third_party/certifi/cacert.pem\n'
              "inspection_python=$(gcloud info --format='value(basic.python_location)')\n"
              '[[ "$inspection_python" == /* && -x "$inspection_python" ]]\n'
              f'"$inspection_python" scripts/gcp/recovery_inspection.py --sha="{sha}" --build-id="{bid}" --source-archive-sha256="{digest}"\n')
    return {'id': 'read-only-recovery-inspection', 'name': SDK_IMAGE, 'entrypoint': 'bash', 'args': ['-ceu', script]}


class Cloud:
    """No generic mutation forwarding: only new inspection Jobs/builds/evidence."""
    def __init__(self, account=None):
        self.account = account

    def run(self, *args, timeout=180):
        a = tuple(map(str, args)); ok = False
        if a[:2] == ('storage', 'cat'):
            ok = len(a) >= 3 and a[2].startswith(f'gs://{BUCKET}/')
        elif a[:3] == ('storage', 'objects', 'describe'):
            ok = len(a) >= 4 and a[3].startswith(f'gs://{BUCKET}/')
        elif a[:2] == ('storage', 'cp') and len(a) >= 4:
            src, dst = a[2:4]
            ok = (src.startswith(f'gs://{BUCKET}/') and re.search(r'#[1-9][0-9]*$', src) and dst.startswith('/')) or (
                src.startswith('/') and dst.startswith(f'gs://{BUCKET}/recovery-inspections/') and '--if-generation-match=0' in a)
        elif a[:2] == ('builds', 'describe'):
            ok = len(a) >= 3 and re.fullmatch(UUID, a[2])
        elif a[:2] == ('builds', 'submit'):
            validate_child_command(a)
            ok = True
        elif a[:3] == ('run', 'jobs', 'describe'):
            ok = len(a) >= 4 and re.fullmatch(r'espacigo-db-[ir]-[a-f0-9]{32}', a[3])
        elif a[:3] == ('run', 'jobs', 'list'):
            ok = len(a) >= 4 and re.fullmatch(r'--filter=metadata.name=espacigo-db-i-[a-f0-9]{32}', a[3])
        elif a[:3] == ('run', 'jobs', 'execute'):
            ok = len(a) == 6 and re.fullmatch(r'espacigo-db-i-[a-f0-9]{32}', a[3]) and a[4:] == ('--async', '--format=json')
        elif a[:3] == ('run', 'jobs', 'delete'):
            ok = len(a) == 4 and re.fullmatch(r'espacigo-db-i-[a-f0-9]{32}', a[3])
        elif a[:3] == ('run', 'jobs', 'create'):
            validate_create_command(a)
            ok = True
        elif a[:3] == ('run', 'jobs', 'executions'):
            ok = len(a) >= 5 and a[3] in ('describe', 'list')
        elif a[:4] == ('artifacts', 'docker', 'images', 'describe'):
            ok = len(a) >= 5 and a[4].startswith(REGISTRY + '/espacigo-db-migrator@sha256:')
        elif a[:2] == ('logging', 'read'):
            ok = True
        require(bool(ok), 'cloud command is outside read-only recovery scope')
        flags = ['--project=' + PROJECT, '--quiet']
        if a[0] in ('builds', 'run'):
            flags.append('--region=' + REGION)
        if self.account:
            flags.append('--account=' + self.account)
        # gcloud requires Job-level options before a named container group.
        cmd = (['gcloud', *a[:4], *flags, *a[4:]] if a[:3] == ('run', 'jobs', 'create')
               else ['gcloud', *a, *flags])
        p = subprocess.run(cmd, text=True, capture_output=True, timeout=timeout)
        require(p.returncode == 0, f'gcloud {a[0]} {a[1]} failed: ' + p.stderr.strip())
        return p.stdout

    def json(self, *args):
        return json.loads(self.run(*args, '--format=json'))

    def historical_list(self):
        try:
            return HistoricalExecutionReader(self.account).list()
        except ObservationError as error:
            raise RecoveryError(str(error)) from None

    def historical_get(self):
        try:
            return HistoricalExecutionReader(self.account).get()
        except ObservationError as error:
            raise RecoveryError(str(error)) from None


def validate_child_command(a):
    require(len(a) == 9 and Path(a[2]).is_absolute() and '..' not in Path(a[2]).parts and a[2].endswith('.tar.gz'), 'child source/argv differs')
    configs = {'--config=/workspace/cloudbuild-migrator-image.yaml': DEPLOYER,
               '--config=/workspace/cloudbuild-db-recovery-provision.yaml': PROVISIONER}
    require(a[3] in configs and a[4] == '--gcs-source-staging-dir=gs://' + BUCKET + '/source' and a[5] == '--service-account=' + configs[a[3]] and a[7:] == ('--async', '--format=json'), 'child config/service account/flags differ')
    require(a[6].startswith('--substitutions='), 'child substitutions missing')
    entries = a[6].removeprefix('--substitutions=').split(',')
    require(all('=' in x for x in entries), 'invalid child substitutions')
    subs = dict(x.split('=', 1) for x in entries)
    require(len(subs) == len(entries), 'duplicate child substitutions')
    if configs[a[3]] == DEPLOYER:
        require(set(subs) == {'_COMMIT_SHA'} and re.fullmatch('[a-f0-9]{40}', subs['_COMMIT_SHA']), 'only image source SHA substitution allowed')
    else:
        require(set(subs) == {'_PARENT_BUILD_ID', '_SOURCE_ARCHIVE_SHA256', '_REQUEST_URI', '_REQUEST_GENERATION', '_REQUEST_SHA256'}, 'only recovery provision substitutions allowed')
        require(re.fullmatch(UUID, subs['_PARENT_BUILD_ID']) and re.fullmatch(HEX, subs['_SOURCE_ARCHIVE_SHA256']) and re.fullmatch(HEX, subs['_REQUEST_SHA256']) and re.fullmatch('[1-9][0-9]*', subs['_REQUEST_GENERATION']), 'invalid provision substitution identity')
        require(subs['_REQUEST_URI'] == f'gs://{BUCKET}/recovery-inspections/{subs["_PARENT_BUILD_ID"]}/job-request.json', 'provision request URI differs')


def validate_create_command(a):
    images = [x.removeprefix('--image=') for x in a if x.startswith('--image=')]
    labels = [x.removeprefix('--labels=') for x in a if x.startswith('--labels=')]
    require(len(images) == len(labels) == 1, 'inspection image/labels missing or duplicated')
    match = re.fullmatch(r'source-commit=([a-f0-9]{40}),cloud-build-id=(' + UUID + r'),purpose=readonly-recovery-inspection', labels[0])
    if match is None:
        raise RecoveryError('inspection labels differ')
    expected = tuple(x for x in inspection_create_args(images[0], match[1], match[2])[1:] if not x.startswith(('--project=', '--region=')))
    require(a == expected, 'inspection create argv differs from exact contract')


def parse_invocation(argv):
    require(not any(x.split('=', 1)[0] in {'--execute', '--apply', '--mode', '--unlock', '--unlock-preapply', '--backup', '--rollout', '--deploy'} for x in argv), 'inspection accepts no mutation/release flags')
    p = argparse.ArgumentParser()
    p.add_argument('--sha', required=True); p.add_argument('--build-id', required=True)
    p.add_argument('--source-archive-sha256', required=True)
    a = p.parse_args(argv)
    require(re.fullmatch('[a-f0-9]{40}', a.sha) and re.fullmatch(UUID, a.build_id) and re.fullmatch(HEX, a.source_archive_sha256), 'source SHA/build UUID/archive SHA256 required')
    return a


def source_uri(build):
    s = build.get('sourceProvenance', {}).get('resolvedStorageSource', {})
    require(s.get('bucket') == BUCKET and re.fullmatch('[1-9][0-9]*', str(s.get('generation', ''))) and isinstance(s.get('object'), str) and s['object'] and '#' not in s['object'], 'generation-pinned source in approved bucket required')
    return f"gs://{BUCKET}/{s['object']}#{s['generation']}"


def parent_identity(parent, sha, bid, archive_sha):
    require(parent.get('id') == bid and parent.get('status') == 'WORKING' and parent.get('serviceAccount') == DEPLOYER, 'active dedicated parent identity differs')
    require(not parent.get('buildTriggerId'), 'recovery parent must be manual; normal main trigger is forbidden')
    s = parent.get('substitutions', {})
    require(s.get('_RECOVERY_INSPECTION_ONLY') == 'true' and s.get('_COMMIT_SHA') == sha and s.get('_SOURCE_ARCHIVE_SHA256') == archive_sha, 'dedicated parent substitutions differ')
    steps = parent.get('steps', [])
    want = parent_step(sha, bid, archive_sha)
    require(len(steps) == 1 and all(steps[0].get(k) == v for k, v in want.items()), 'dedicated parent step/config differs')
    return source_uri(parent)


def archive_inventory(path):
    out = {}; total = 0
    with tarfile.open(path, 'r:gz') as tar:
        for m in tar:
            raw = m.name.removeprefix('./'); p = PurePosixPath(raw)
            require(not p.is_absolute() and '..' not in p.parts and '\\' not in raw and not m.issym() and not m.islnk(), 'unsafe source archive member')
            if m.isdir():
                continue
            total += m.size
            require(m.isfile() and raw not in out and 0 <= m.size <= 32 * 1024 * 1024 and total <= 256 * 1024 * 1024, 'nonregular/duplicate/oversized source member')
            f = tar.extractfile(m); require(f is not None, 'unreadable source member')
            out[raw] = hashlib.sha256(f.read()).hexdigest()
    return out


def workspace_inventory(root):
    out = {}; excluded = {'.git', '.local', '__pycache__', 'node_modules', '.next', '.terraform'}
    for directory, dirs, files in os.walk(root, followlinks=False):
        require(all(not (Path(directory) / x).is_symlink() for x in dirs), 'workspace directory symlink rejected')
        dirs[:] = [d for d in dirs if d not in excluded]
        for name in files:
            p = Path(directory) / name
            require(not p.is_symlink(), 'workspace file symlink rejected')
            out[p.relative_to(root).as_posix()] = hashlib.sha256(p.read_bytes()).hexdigest()
    return out


def download_archive(cloud, uri, digest, path):
    cloud.run('storage', 'cp', uri, path)
    require(hashlib.sha256(path.read_bytes()).hexdigest() == digest, 'reviewed source archive SHA256 differs')


def verify_archive(cloud, uri, digest, root):
    with tempfile.TemporaryDirectory(prefix='recovery-source-', dir=os.environ.get('TMPDIR')) as tmp:
        p = Path(tmp) / 'source.tar.gz'; download_archive(cloud, uri, digest, p)
        inventory = archive_inventory(p)
        require(inventory == workspace_inventory(root), 'generation-pinned archive/workspace differs')
        return inventory


def assert_lock(cloud, generation, text):
    m = cloud.json('storage', 'objects', 'describe', LOCK)
    require(str(m.get('generation')) == generation and cloud.run('storage', 'cat', LOCK + '#' + generation) == text, 'failed release lock changed; preserve resources')


def inspect_failed_release(cloud):
    m = cloud.json('storage', 'objects', 'describe', LOCK)
    require(str(m.get('generation')) == PINNED_GENERATION, 'pinned failed lock generation differs')
    text = cloud.run('storage', 'cat', LOCK + '#' + PINNED_GENERATION); owner = json.loads(text)
    require(owner == {'build_id': PINNED_BUILD_ID, 'source_sha': PINNED_SOURCE_SHA}, 'pinned failed lock owner differs')
    b = cloud.json('builds', 'describe', PINNED_BUILD_ID)
    require(b.get('id') == PINNED_BUILD_ID and b.get('status') in ('FAILURE', 'CANCELLED', 'TIMEOUT', 'INTERNAL_ERROR', 'EXPIRED') and b.get('buildTriggerId') == TRIGGER and b.get('serviceAccount') == DEPLOYER and b.get('substitutions', {}).get('COMMIT_SHA') == PINNED_SOURCE_SHA, 'failed canonical release identity differs')
    job = 'espacigo-db-r-' + PINNED_BUILD_ID.replace('-', '')
    j = cloud.json('run', 'jobs', 'describe', job); uid = j.get('metadata', {}).get('uid')
    require(re.fullmatch(UUID, uid or '') is not None, 'failed Job UID missing')
    intent_uri = f'gs://{BUCKET}/releases/{PINNED_BUILD_ID}/job-intent-apply.json'
    intent_meta = cloud.json('storage', 'objects', 'describe', intent_uri)
    intent_text = cloud.run('storage', 'cat', intent_uri + '#' + str(intent_meta['generation']))
    intent = json.loads(intent_text)
    require(intent.get('job') == job and intent.get('uid') == uid and intent.get('request', {}).get('mode') == 'apply', 'recorded applying intent differs')
    listed = cloud.historical_list()
    es = listed.get('executions', [])
    name = f'projects/{PROJECT}/locations/{REGION}/jobs/{job}/executions/{job}-v62zb'
    require(not listed.get('nextPageToken') and len(es) == 1 and es[0].get('name') == name and es[0].get('job') == job,
            'unexpected additional applying executions; inspect before continuing')
    e = cloud.historical_get()
    c = [x for x in e.get('conditions', []) if x.get('type') == 'Completed']
    require(e.get('name') == name and e.get('job') == job and re.fullmatch(UUID, e.get('uid', ''))
            and e.get('uid') == es[0].get('uid') and len(c) == 1 and c[0].get('state') == 'CONDITION_FAILED'
            and type(e.get('failedCount')) is int and e['failedCount'] == 1
            and type(e.get('runningCount', 0)) is int and e.get('runningCount', 0) == 0
            and type(e.get('succeededCount', 0)) is int and e.get('succeededCount', 0) == 0,
            'applying execution not expected terminal failure')
    assert_lock(cloud, PINNED_GENERATION, text)
    return {'observed_at': datetime.now().astimezone().isoformat(),
            'build_id': b['id'], 'source_sha': PINNED_SOURCE_SHA,
            'lock_uri': LOCK, 'lock_generation': str(m['generation']), 'lock_text': text,
            'job_name': job, 'job_uid': uid, 'execution_name': e['name'], 'execution_uid': e['uid'],
            'execution_state': c[0]['state'], 'failed_count': e['failedCount'],
            'running_count': e.get('runningCount', 0), 'succeeded_count': e.get('succeededCount', 0),
            'list_count': len(es), 'next_page': listed.get('nextPageToken', ''),
            'intent_uri': intent_uri, 'intent_generation': str(intent_meta['generation']),
            'intent_sha256': hashlib.sha256(intent_text.encode()).hexdigest(),
            'failed_execution': job + '-v62zb'}


def make_failed_release_proof(observation, parent_bid, source_sha, source_uri_value, archive_sha):
    return _make_parent_proof(observation, parent_bid, source_sha, source_uri_value, archive_sha)

def validate_failed_release_proof(proof, now=None):
    return validate_request_proof(proof, proof.get('parent_build_id') if isinstance(proof, dict) else None,
                                  proof.get('parent_source_sha') if isinstance(proof, dict) else None,
                                  proof.get('parent_source_uri') if isinstance(proof, dict) else None,
                                  proof.get('parent_archive_sha256') if isinstance(proof, dict) else None, now)

def compare_failed_release_proof(expected, actual):
    validate_failed_release_proof(expected); validate_failed_release_proof(actual)
    require(all(expected[k] == actual[k] for k in expected if k != 'observed_at'), 'failed-release proof changed')

def record_evidence(cloud, root, bid, name, value):
    require(re.fullmatch('[a-z][a-z0-9-]*', name) and re.fullmatch(UUID, bid), 'invalid evidence identity')
    work = root
    require(not root.is_symlink(), 'evidence root symlink rejected')
    for part in ('.local', 'recovery', bid):
        work = work / part
        require(not work.is_symlink(), 'evidence directory symlink rejected')
        work.mkdir(exist_ok=True, mode=0o700)
        require(work.is_dir(), 'evidence parent is not a directory')
    path = work / (name + '.json'); text = json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n'
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w') as output:
        output.write(text); output.flush(); os.fsync(output.fileno())
    uri = f'gs://{BUCKET}/recovery-inspections/{bid}/{name}.json'
    cloud.run('storage', 'cp', path.resolve(), uri, '--if-generation-match=0')
    gen = str(cloud.json('storage', 'objects', 'describe', uri)['generation'])
    require(cloud.run('storage', 'cat', uri + '#' + gen) == text, 'evidence generation readback differs')
    return {'uri': uri, 'generation': gen, 'sha256': hashlib.sha256(text.encode()).hexdigest()}


def wait_build(cloud, bid, sa):
    require(re.fullmatch(UUID, bid), 'child build ID missing'); deadline = time.monotonic() + 1800
    while True:
        b = cloud.json('builds', 'describe', bid)
        if b.get('status') not in ('PENDING', 'QUEUED', 'WORKING'):
            break
        require(time.monotonic() < deadline, 'child build timeout; preserve evidence, do not resubmit')
        time.sleep(5)
    require(b.get('status') == 'SUCCESS' and b.get('serviceAccount') == sa, 'child build failed or identity differs')
    return b


def submit_child(cloud, root, uri, digest, config, sa, substitutions, record):
    with tempfile.TemporaryDirectory(prefix='recovery-child-', dir=os.environ.get('TMPDIR')) as tmp:
        archive = Path(tmp) / 'source.tar.gz'; download_archive(cloud, uri, digest, archive)
        record('child-intent-' + ('image' if sa == DEPLOYER else 'provision'), {'source_uri': uri, 'source_archive_sha256': digest, 'config': config, 'service_account': sa})
        b = cloud.json('builds', 'submit', archive, '--config=' + str(root / config), '--gcs-source-staging-dir=gs://' + BUCKET + '/source', '--service-account=' + sa, '--substitutions=' + substitutions, '--async')
    record('child-start-' + ('image' if sa == DEPLOYER else 'provision'), {'build_id': b.get('id')})
    result = wait_build(cloud, b.get('id', ''), sa)
    verify_archive(cloud, source_uri(result), digest, root)
    return result


def build_image(cloud, root, uri, digest, sha, record):
    b = submit_child(cloud, root, uri, digest, 'cloudbuild-migrator-image.yaml', DEPLOYER, '_COMMIT_SHA=' + sha, record)
    require(b.get('substitutions', {}).get('_COMMIT_SHA') == sha, 'image source SHA differs')
    images = b.get('results', {}).get('images', []); expected = REGISTRY + '/espacigo-db-migrator:sha-' + sha
    require(len(images) == 1 and images[0].get('name') == expected and re.fullmatch('sha256:' + HEX, images[0].get('digest', '')), 'image outputs differ')
    image = REGISTRY + '/espacigo-db-migrator@' + images[0]['digest']
    proof = cloud.json('artifacts', 'docker', 'images', 'describe', image, '--show-provenance')
    statements = [x.get('build', {}).get('inTotoSlsaProvenanceV1', {}) for x in proof.get('provenance_summary', {}).get('provenance', [])]
    invocation = f'https://cloudbuild.googleapis.com/v1/projects/{PROJECT}/locations/{REGION}/builds/{b["id"]}'
    require(proof.get('image_summary', {}).get('digest') == images[0]['digest'] and any(s.get('predicateType') == 'https://slsa.dev/provenance/v1' and s.get('predicate', {}).get('runDetails', {}).get('metadata', {}).get('invocationId') == invocation and any(x.get('digest', {}).get('sha256') == images[0]['digest'][7:] for x in s.get('subject', [])) for s in statements), 'immutable image provenance differs')
    record('image-verified', {'image': image, 'build_id': b['id'], 'source_archive_sha256': digest})
    return image


def make_request(sha, bid, digest, image, uri, failed_release_proof):
    validate_request_proof(failed_release_proof, bid, sha, uri, digest)
    req = {'format': 'espacigo.recovery-job-request.v2', 'mode': 'inspection-only', 'source_sha': sha, 'build_id': bid, 'source_archive_sha256': digest, 'source_uri': uri, 'image': image, 'failed_release_proof': failed_release_proof, 'failed_release_proof_sha256': canonical_proof_sha256(failed_release_proof)}
    validate_request(req)
    return req

def validate_request(req):
    require(isinstance(req, dict) and set(req) == {'format', 'mode', 'source_sha', 'build_id', 'source_archive_sha256', 'source_uri', 'image', 'failed_release_proof', 'failed_release_proof_sha256'} and req['format'] == 'espacigo.recovery-job-request.v2' and req['mode'] == 'inspection-only', 'only inspection request v2 supported')
    require(re.fullmatch('[a-f0-9]{40}', req['source_sha']) and re.fullmatch(UUID, req['build_id']) and re.fullmatch(HEX, req['source_archive_sha256']), 'invalid request identity')
    require(re.fullmatch(re.escape('gs://' + BUCKET + '/') + r'[^#]+#[1-9][0-9]*', req['source_uri']), 'invalid pinned request source URI')
    plan_proposal(req['image'], req['source_sha'], '1')
    validate_request_proof(req['failed_release_proof'], req['build_id'], req['source_sha'], req['source_uri'], req['source_archive_sha256'])
    require(req['failed_release_proof_sha256'] == canonical_proof_sha256(req['failed_release_proof']), 'proof digest differs')

def create_args(req):
    validate_request(req)
    return inspection_create_args(req['image'], req['source_sha'], req['build_id'])


def inspection_create_args(image, sha, bid):
    args = plan_proposal(image, sha, '1')['proposed_job_create_argv']
    args[4] = job_name(bid)
    args = [('--args=-dir,/app/db/migrations,-recovery-inspection-only' if x.startswith('--args=') else '--labels=source-commit=' + sha + ',cloud-build-id=' + bid + ',purpose=readonly-recovery-inspection' if x.startswith('--labels=') else x) for x in args]
    container_flags = ('--image=', '--cpu=', '--memory=', '--command=', '--args=', '--set-env-vars=', '--set-secrets=')
    return args[:5] + [x for x in args[5:] if not x.startswith(container_flags)] + ['--container=inspection'] + [x for x in args[5:] if x.startswith(container_flags)]


def desired_job(req, uid=''):
    """Expected semantic configuration, also used by production readback checks."""
    args = create_args(req)
    env = dict(x.split('=', 1) for x in next(a for a in args if a.startswith('--set-env-vars=')).removeprefix('--set-env-vars=^|^').split('|'))
    task = {'containers': [{'name': 'inspection', 'image': req['image'], 'command': ['/app/dbmigrate'], 'args': ['-dir', '/app/db/migrations', '-recovery-inspection-only'], 'env': [{'name': k, 'value': v} for k, v in env.items()], 'resources': {'limits': {'cpu': '1', 'memory': '512Mi'}}, 'volumeMounts': [{'name': 'secret', 'mountPath': '/var/secrets/migration'}]}], 'volumes': [{'name': 'secret', 'secret': {'secretName': 'espacigo-testprod-cloudsql-migrator-password', 'items': [{'key': '1', 'path': 'password'}]}}], 'serviceAccountName': DB_SA, 'timeoutSeconds': '600', 'maxRetries': 0}
    return {'metadata': {'name': job_name(req['build_id']), 'namespace': PROJECT_NUMBER, 'uid': uid}, 'spec': {'template': {'metadata': {'labels': {'source-commit': req['source_sha'], 'cloud-build-id': req['build_id'], 'purpose': 'readonly-recovery-inspection'}, 'annotations': {'run.googleapis.com/network-interfaces': json.dumps([{'network': PROJECT, 'subnetwork': 'espacigo-testprod-api-serverless'}]), 'run.googleapis.com/vpc-access-egress': 'private-ranges-only'}}, 'spec': {'taskCount': 1, 'parallelism': 1, 'template': {'spec': task}}}}}


def verify_job(job, req):
    want = desired_job(req); m = job.get('metadata', {})
    require(all(m.get(k) == want['metadata'][k] for k in ('name', 'namespace')) and re.fullmatch(UUID, m.get('uid', '')), 'Job identity differs')
    t = job.get('spec', {}).get('template', {}); a = t.get('metadata', {}).get('annotations', {}); labels = t.get('metadata', {}).get('labels', {})
    require(all(labels.get(k) == v for k, v in want['spec']['template']['metadata']['labels'].items()), 'Job source/build/purpose differs')
    require(json.loads(a.get('run.googleapis.com/network-interfaces', 'null')) == [{'network': PROJECT, 'subnetwork': 'espacigo-testprod-api-serverless'}] and a.get('run.googleapis.com/vpc-access-egress') == 'private-ranges-only', 'Job private network differs')
    spec = t.get('spec', {})
    require(spec.get('taskCount') == 1 and spec.get('parallelism') == 1, 'Job task controls differ')
    require(set(spec) == {'taskCount', 'parallelism', 'template'} and set(spec['template']) == {'spec'}, 'unexpected Job execution fields')
    verify_task(spec['template']['spec'], req)


def verify_task(task, req):
    """Compare the complete task, normalizing only known server defaults."""
    actual = json.loads(json.dumps(task))
    expected = desired_job(req)['spec']['template']['spec']['template']['spec']
    if 'sandboxes' in actual:
        require(actual.pop('sandboxes') == {}, 'unexpected sandbox controls')
    cs = actual.get('containers', []); vs = actual.get('volumes', [])
    require(len(cs) == 1 and len(vs) == 1, 'one Job container and volume required')
    volume = vs[0].get('name')
    require(isinstance(volume, str) and volume and cs[0].get('volumeMounts') == [{'name': volume, 'mountPath': '/var/secrets/migration'}], 'Job secret mount differs')
    vs[0]['name'] = 'secret'; cs[0]['volumeMounts'][0]['name'] = 'secret'
    actual['timeoutSeconds'] = str(actual.get('timeoutSeconds'))
    cs[0]['env'] = sorted(cs[0].get('env', []), key=lambda x: x.get('name', ''))
    expected['containers'][0]['env'] = sorted(expected['containers'][0]['env'], key=lambda x: x['name'])
    require(actual == expected, 'effective inspection task configuration differs')


def provision_job(cloud, root, req, record):
    validate_request(req)
    receipt = record('job-request', req)
    lock = req['failed_release_proof']['failed_release']
    def verify_binding(binding, value):
        expected = build_binding(req['build_id'], binding.get('role'), value['expires_at'])
        require(binding == expected, 'IAM readiness binding differs from exact authorized resources')
    proof = wait_for_iam_ready(cloud, req, receipt, verify_binding,
                              lambda: assert_lock(cloud, lock['lock_generation'], lock['lock_text']))
    record('iam-ready-verified', proof)
    sub = ','.join(['_PARENT_BUILD_ID=' + req['build_id'], '_SOURCE_ARCHIVE_SHA256=' + req['source_archive_sha256'], '_REQUEST_URI=' + receipt['uri'], '_REQUEST_GENERATION=' + receipt['generation'], '_REQUEST_SHA256=' + receipt['sha256']])
    before = make_failed_release_proof(inspect_failed_release(cloud), req['build_id'],
                                       req['source_sha'], req['source_uri'], req['source_archive_sha256'])
    compare_failed_release_proof(req['failed_release_proof'], before)
    record('parent-pre-provision-proof', before)
    submit_child(cloud, root, req['source_uri'], req['source_archive_sha256'], 'cloudbuild-db-recovery-provision.yaml', PROVISIONER, sub, record)
    after = make_failed_release_proof(inspect_failed_release(cloud), req['build_id'],
                                      req['source_sha'], req['source_uri'], req['source_archive_sha256'])
    compare_failed_release_proof(req['failed_release_proof'], after)
    record('parent-post-provision-proof', after)
    job = cloud.json('run', 'jobs', 'describe', job_name(req['build_id'])); verify_job(job, req)
    return job


def local_migrations(root):
    result = []
    for p in sorted((root / 'db' / 'migrations').glob('V*.sql')):
        v = re.fullmatch(r'V([0-9]+)__.+\.sql', p.name); require(v is not None, 'invalid source migration filename')
        result.append({'version': int(v[1]), 'name': p.name, 'checksum': hashlib.sha256(p.read_bytes()).hexdigest()})
    require([x['version'] for x in result] == list(range(1, len(result) + 1)), 'source inventory not contiguous')
    return result


def validate_report(report, inventory):
    require(isinstance(report, dict) and report.get('format') == 'espacigo.recovery-inspection.v1' and report.get('read_only') is True, 'not read-only recovery report')
    require(report.get('database') == 'espacigo_dms_stage_20261020' and report.get('session_user') == 'espacigo_migrator_login' and report.get('current_user') == 'espacigo_schema_owner', 'inspection SQL identities differ')
    require(isinstance(report.get('server_version'), str) and isinstance(report.get('authority'), dict), 'report authority/version types differ')
    require(report.get('transaction_isolation') == 'repeatable read', 'report transaction is not repeatable read')
    a = report['authority']
    require(set(a) == {'public_schema_owner', 'usuario_owner', 'roles', 'memberships', 'effective_privileges', 'acl_metadata'}, 'report authority facts missing')
    require(isinstance(a['public_schema_owner'], str) and a['public_schema_owner'] and isinstance(a['usuario_owner'], str), 'report object owner types differ')
    names = {'espacigo_migrator_login', 'espacigo_schema_owner', 'espacigo_runtime', 'espacigo_document_verification_fn'}
    flags = {'exists', 'superuser', 'create_role', 'create_db', 'replication', 'bypass_rls', 'login', 'inherit'}
    require(isinstance(a['roles'], list) and len(a['roles']) == len(names), 'report role facts missing')
    require(all(isinstance(x, dict) and set(x) == {'name'} | flags and all(type(x[k]) is bool for k in flags) for x in a['roles']), 'report role fact types differ')
    require({x['name'] for x in a['roles']} == names, 'report role identities differ')
    require(isinstance(a['memberships'], list) and all(isinstance(x, list) and len(x) == 5 and all(isinstance(y, str) for y in x[:2]) and all(type(y) is bool for y in x[2:]) for x in a['memberships']), 'report membership facts invalid')
    effective = {'session_public_usage', 'schema_owner_public_usage', 'schema_public_create', 'usuario_references_id', 'ledger_select', 'ledger_insert'}
    require(isinstance(a['effective_privileges'], dict) and set(a['effective_privileges']) == effective and all(type(x) is bool for x in a['effective_privileges'].values()), 'report effective privilege facts invalid')
    require(isinstance(a['acl_metadata'], list) and all(isinstance(x, dict) and all(isinstance(x.get(k), str) for k in ('class', 'schema', 'object', 'owner', 'acl_source', 'grantee', 'grantor', 'privilege')) and type(x.get('grantable')) is bool for x in a['acl_metadata']), 'report ACL facts invalid')
    applied, pending = report.get('applied'), report.get('pending')
    require(isinstance(applied, list) and isinstance(pending, list) and len(applied) <= len(inventory), 'report inventory types/length differ')
    for got, expected in zip(applied + pending, inventory):
        require(all(got.get(k) == expected[k] for k in ('version', 'name', 'checksum')), 'report ledger/source mismatch')
    require(len(applied) + len(pending) == len(inventory), 'report omits source versions')
    return report


def inspect_job(cloud, job, req, record, inventory):
    verify_job(job, req); name = job['metadata']['name']; uid = job['metadata']['uid']
    record('inspection-intent', {'job': name, 'uid': uid, 'request': req})
    start = cloud.json('run', 'jobs', 'execute', name, '--async')  # Deliberately no retry.
    execution = start.get('metadata', {}).get('name', '')
    require(re.fullmatch(re.escape(name) + '-[a-z0-9]{5}', execution), 'exact inspection execution ID missing; do not relaunch')
    record('inspection-start', start); deadline = time.monotonic() + 660
    while True:
        e = cloud.json('run', 'jobs', 'executions', 'describe', execution)
        conditions = [c for c in e.get('status', {}).get('conditions', []) if c.get('type') == 'Completed']
        if conditions and conditions[0].get('status') in ('True', 'False'):
            break
        require(time.monotonic() < deadline, 'inspection outcome uncertain; preserve Job/evidence')
        time.sleep(5)
    record('inspection-finished', e)
    require(e.get('metadata', {}).get('name') == execution and e.get('metadata', {}).get('namespace') == PROJECT_NUMBER and e.get('metadata', {}).get('labels', {}).get('run.googleapis.com/jobUid') == uid, 'effective inspection execution identity differs')
    spec = e.get('spec', {})
    require(set(spec) == {'taskCount', 'parallelism', 'template'} and spec['taskCount'] == spec['parallelism'] == 1 and set(spec['template']) == {'spec'}, 'effective execution task controls differ')
    verify_task(e.get('spec', {}).get('template', {}).get('spec', {}), req)
    s = e.get('status', {})
    require(len(conditions) == 1 and conditions[0].get('status') == 'True' and s.get('succeededCount') == 1 and all(s.get(k, 0) == 0 for k in ('failedCount', 'runningCount', 'retriedCount', 'cancelledCount')), 'inspection did not finish exactly once successfully')
    query = f'resource.type="cloud_run_job" AND resource.labels.job_name="{name}" AND resource.labels.location="{REGION}" AND labels."run.googleapis.com/execution_name"="{execution}" AND logName="projects/{PROJECT}/logs/run.googleapis.com%2Fstdout"'
    deadline = time.monotonic() + 90
    while True:
        logs = cloud.json('logging', 'read', query, '--limit=100')
        reports = [x for x in logs if isinstance(x.get('jsonPayload'), dict) and x['jsonPayload'].get('format') == 'espacigo.recovery-inspection.v1']
        if reports:
            break
        require(time.monotonic() < deadline, 'inspection report absent; do not replay')
        time.sleep(5)
    require(len(reports) == 1, 'exactly one inspection report required'); entry = reports[0]
    require(entry.get('resource', {}).get('type') == 'cloud_run_job' and entry.get('logName') == f'projects/{PROJECT}/logs/run.googleapis.com%2Fstdout', 'inspection log resource/stream differs')
    require(entry.get('resource', {}).get('labels') == {'project_id': PROJECT, 'location': REGION, 'job_name': name} and entry.get('labels', {}).get('run.googleapis.com/execution_name') == execution, 'inspection log identity differs')
    start_time, log_time, completion_time = (parse_timestamp(x) for x in (s.get('startTime'), entry.get('timestamp'), s.get('completionTime')))
    require(start_time <= log_time <= completion_time, 'inspection report timestamp is outside exact execution')
    result = validate_report(entry['jsonPayload'], inventory)
    record('inspection-report', result)
    return result


def parse_timestamp(value):
    require(isinstance(value, str), 'execution/log timestamp missing')
    try:
        result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    except ValueError as error:
        raise RecoveryError('invalid execution/log timestamp') from error
    require(result.tzinfo is not None, 'execution/log timestamp timezone missing')
    return result


def execute(cloud, root, sha, bid, archive_sha):
    require(re.fullmatch('[a-f0-9]{40}', sha) and re.fullmatch(UUID, bid) and re.fullmatch(HEX, archive_sha), 'invalid recovery identity')
    parent = cloud.json('builds', 'describe', bid); uri = parent_identity(parent, sha, bid, archive_sha)
    verify_archive(cloud, uri, archive_sha, root); lock = inspect_failed_release(cloud)
    receipts = {}
    def record(name, value):
        assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
        receipt = record_evidence(cloud, root, bid, name, value); receipts[name] = receipt
        return receipt
    record('preflight', {'source_sha': sha, 'source_archive_sha256': archive_sha, 'failed_release': lock})
    try:
        image = build_image(cloud, root, uri, archive_sha, sha, record)
        assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
        observation = inspect_failed_release(cloud)
        require(observation['lock_generation'] == lock['lock_generation'] and observation['lock_text'] == lock['lock_text'], 'failed release lock changed after image build')
        proof = make_failed_release_proof(observation, bid, sha, uri, archive_sha)
        req = make_request(sha, bid, archive_sha, image, uri, proof)
        job = provision_job(cloud, root, req, record)
        result = inspect_job(cloud, job, req, record, local_migrations(root))
        record('completed', {'read_only': True, 'source_sha': sha, 'report': result, 'evidence': receipts.copy(), 'release_lock_unchanged': True, 'sql_applied': False, 'rollout': False})
        current = cloud.json('run', 'jobs', 'describe', job['metadata']['name']); verify_job(current, req)
        require(current['metadata']['uid'] == job['metadata']['uid'], 'refuse cleanup of replaced inspection Job')
        record('cleanup-intent', {'job': job['metadata']['name'], 'uid': job['metadata']['uid'], 'action': 'delete-own-inspection-job'})
        assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
        cloud.run('run', 'jobs', 'delete', job['metadata']['name'])
        require(cloud.json('run', 'jobs', 'list', '--filter=metadata.name=' + job['metadata']['name']) == [], 'inspection Job absence not confirmed; do not re-delete')
        record('cleanup-verified', {'job': job['metadata']['name'], 'uid': job['metadata']['uid'], 'absent': True})
        assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
        return result
    except Exception as error:
        try:
            record('failure', {'error_type': type(error).__name__, 'message': str(error), 'evidence': receipts.copy(), 'release_lock_preserved': True})
        except Exception:
            pass
        raise


def main(argv=None):
    try:
        a = parse_invocation(sys.argv[1:] if argv is None else argv)
        execute(Cloud(), Path('/workspace'), a.sha, a.build_id, a.source_archive_sha256)
        print('READ ONLY RECOVERY INSPECTION VERIFIED; release lock retained')
        return 0
    except (ValueError, OSError, KeyError, TypeError, subprocess.TimeoutExpired) as error:
        print('read-only recovery inspection failed; preserve resources: ' + str(error), file=sys.stderr)
        return 1


if __name__ == '__main__':
    raise SystemExit(main())
