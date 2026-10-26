#!/usr/bin/env python3
"""Cloud Build-only web rollout; default CLI behavior is read-only verification.

The original full rollout keeps its live schema gate in cloudbuild.yaml.
This executor never deploys API, reads SQL/secrets, or changes IAM/network.
Failed releases intentionally retain the shared deployment lock for diagnosis.
"""
from __future__ import annotations
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
import time

from verify_web_backend import (API, BUCKET, Cloud, GuardError, HEX64, PROJECT,
                                REGION, REGISTRY, SA, UUID, active_api, require,
                                verify_backend)
TRIGGER = '548b8eff-e489-4307-a77f-455b56aa4e57'
SERVICES = ('espacigo-test-public', 'espacigo-test-private')
LOCK = f'gs://{BUCKET}/cloudrun-main.lock'
WATERMARK = f'gs://{BUCKET}/last-successful-main.json'

def deploy_args(service, image, sha, build_id, api_url):
    require(service in SERVICES, 'web executor refuses non-web service')
    require(re.fullmatch(re.escape(REGISTRY + '/espacigo-web@sha256:') + HEX64, image) is not None, 'web image digest is not canonical')
    require(re.fullmatch(r'[a-f0-9]{40}', sha) is not None and re.fullmatch(UUID, build_id) is not None, 'full source SHA and build UUID required')
    require(re.fullmatch(r'https://espacigo-test-api-[a-z0-9-]+\.a\.run\.app', api_url) is not None, 'unexpected API service URL')
    mode = 'public' if service == SERVICES[0] else 'private'
    host = 'espacigo.testingpage.store' if mode == 'public' else 'espacigo-demo.testingpage.store'
    env = f'API_BACKEND_BASE_URL={api_url}/api/v1,API_INTERNAL_BASE_URL={api_url}/api/v1,API_SERVICE_AUDIENCE={api_url},FRONTEND_ENTRY_MODE={mode},FRONTEND_PUBLIC_ORIGIN=https://{host}'
    return ['run', 'deploy', service, f'--image={image}', '--port=4000', '--no-traffic',
            f'--revision-suffix=b{build_id.replace("-", "")[:24]}',
            f'--update-labels=source-commit={sha},cloud-build-id={build_id}',
            f'--update-env-vars={env}']

def parent_identity(build, sha, build_id):
    require(build.get('id') == build_id and build.get('buildTriggerId') == TRIGGER and build.get('serviceAccount') == SA, 'not the canonical trigger build/deployment identity')
    subs = build.get('substitutions', {})
    require(subs.get('COMMIT_SHA') == sha and subs.get('_DEPLOY_SCOPE') == 'web', 'parent SHA/scope does not match requested web release')
    require(subs.get('REPO_FULL_NAME') == 'HernanEspinozaDev/espaciGo', 'unexpected source repository')
    require(build.get('status') == 'WORKING', 'parent build is not actively executing')
    return int(datetime.fromisoformat(build['createTime'].replace('Z', '+00:00')).timestamp())

def verify_private(cloud):
    service = cloud.json('run', 'services', 'describe', SERVICES[1])
    require(service.get('metadata', {}).get('annotations', {}).get('run.googleapis.com/iap-enabled') == 'true', 'private IAP must already be enabled')
    policy = cloud.json('run', 'services', 'get-iam-policy', SERVICES[1])
    members = [m for b in policy.get('bindings', []) if b.get('role') == 'roles/run.invoker' for m in b.get('members', [])]
    require('allUsers' not in members and 'allAuthenticatedUsers' not in members, 'private invoker policy is unexpectedly public')
    require('serviceAccount:service-447373903153@gcp-sa-iap.iam.gserviceaccount.com' in members, 'private IAP service agent binding is missing')

def ready_revision(revision, image, sha, build_id):
    require(revision.get('status', {}).get('imageDigest') == image, 'ready revision digest mismatch')
    require(any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in revision.get('status', {}).get('conditions', [])), 'new revision is not Ready')
    labels = revision.get('metadata', {}).get('labels', {})
    require(labels.get('source-commit') == sha and labels.get('cloud-build-id') == build_id, 'new revision source/build labels mismatch')

def promote_and_verify(cloud, service, revision):
    cloud.run('run', 'services', 'update-traffic', service, f'--to-revisions={revision}=100', timeout=240)
    target = cloud.json('run', 'services', 'describe', service)
    traffic = [t for t in target.get('status', {}).get('traffic', []) if t.get('percent', 0)]
    require(len(traffic) == 1 and traffic[0].get('revisionName') == revision and traffic[0].get('percent') == 100, 'traffic promotion does not match exact verified revision')

def web_image(cloud, root, sha):
    submitted = json.loads(cloud.run('builds', 'submit', root / 'frontend',
        f'--config={root / "cloudbuild-web-images.yaml"}',
        f'--ignore-file={root / ".gcloudignore"}',
        f'--gcs-source-staging-dir=gs://{BUCKET}/source', f'--service-account={SA}',
        f'--substitutions=_COMMIT_SHA={sha}', '--async', '--format=json', timeout=180))
    child_id = submitted.get('id', '')
    require(re.fullmatch(UUID, child_id) is not None, 'child image build ID missing')
    print(f'Web image build: {child_id}', flush=True)
    deadline = time.monotonic() + 1200
    while True:
        child = cloud.json('builds', 'describe', child_id)
        status = child.get('status')
        if status not in ('QUEUED', 'WORKING', 'PENDING'):
            break
        require(time.monotonic() < deadline, 'child build deadline exceeded; inspect child build and retained lock')
        time.sleep(10)
    require(status == 'SUCCESS' and child.get('serviceAccount') == SA and child.get('substitutions', {}).get('_COMMIT_SHA') == sha, f'web image build did not succeed for exact source SHA: {child_id} {status}')
    images = child.get('results', {}).get('images', [])
    require(len(images) == 1 and images[0].get('name') == f'{REGISTRY}/espacigo-web:sha-{sha}', 'child must publish exactly the web image for source SHA')
    digest = images[0].get('digest', '')
    require(re.fullmatch('sha256:' + HEX64, digest) is not None, 'child image digest missing')
    image = f'{REGISTRY}/espacigo-web@{digest}'
    provenance = cloud.json('artifacts', 'docker', 'images', 'describe', image, '--show-provenance')
    require(provenance.get('image_summary', {}).get('digest') == digest, 'artifact digest differs from child result')
    invocation = f'https://cloudbuild.googleapis.com/v1/projects/{PROJECT}/locations/{REGION}/builds/{child_id}'
    statements = [p.get('build', {}).get('inTotoSlsaProvenanceV1', {}) for p in provenance.get('provenance_summary', {}).get('provenance', [])]
    require(any(s.get('predicateType') == 'https://slsa.dev/provenance/v1' and s.get('predicate', {}).get('runDetails', {}).get('metadata', {}).get('invocationId') == invocation and any(t.get('digest', {}).get('sha256') == digest.removeprefix('sha256:') for t in s.get('subject', [])) for s in statements), 'matching child SLSA v1 web provenance missing')
    return image, child_id

def execute(cloud, root, sha, build_id):
    require(re.fullmatch(r'[a-f0-9]{40}', sha) is not None and re.fullmatch(UUID, build_id) is not None, 'full source SHA/build ID required')
    parent = cloud.json('builds', 'describe', build_id)
    epoch = parent_identity(parent, sha, build_id)
    api = verify_backend(cloud, root)  # failure here does not create a lock
    verify_private(cloud)
    with tempfile.TemporaryDirectory(prefix='espacigo-web-release-', dir=os.environ.get('TMPDIR') or str(root)) as temp:
        path = Path(temp) / 'lock.json'
        lock_text = json.dumps({'source_sha': sha, 'build_id': build_id}) + '\n'
        path.write_text(lock_text)
        cloud.run('storage', 'cp', path, LOCK, '--if-generation-match=0')
        lock_meta = cloud.json('storage', 'objects', 'describe', LOCK)
        lock_generation = str(lock_meta['generation'])
        require(cloud.run('storage', 'cat', f'{LOCK}#{lock_generation}') == lock_text, 'deployment lock ownership verification failed')
        # Intentionally no automatic failure cleanup: partial releases need operator review.
        print(f'Deployment lock acquired: build={build_id} generation={lock_generation}', flush=True)
        prior_meta = cloud.json('storage', 'objects', 'describe', WATERMARK)
        prior_generation = str(prior_meta['generation'])
        prior = json.loads(cloud.run('storage', 'cat', f'{WATERMARK}#{prior_generation}'))
        require(isinstance(prior.get('created_epoch'), int) and epoch > prior['created_epoch'], 'refusing rollout older than successful main watermark')
        require(active_api(cloud)['image'] == api['image'], 'API changed after lock acquisition')
        image, child = web_image(cloud, root, sha)
        revisions = {}
        for service in SERVICES:
            existing = cloud.json('run', 'services', 'describe', service)
            mode = 'public' if service == SERVICES[0] else 'private'
            expected_sa = f'espacigo-test-{mode}-web@{PROJECT}.iam.gserviceaccount.com'
            require(existing.get('spec', {}).get('template', {}).get('spec', {}).get('serviceAccountName') == expected_sa, 'web service runtime identity differs from canonical inventory')
            require(active_api(cloud)['image'] == api['image'], 'API changed before web deploy')
            cloud.run(*deploy_args(service, image, sha, build_id, api['url']), timeout=300)
            revision = f'{service}-b{build_id.replace("-", "")[:24]}'
            metadata = cloud.json('run', 'revisions', 'describe', revision)
            ready_revision(metadata, image, sha, build_id)
            revisions[service] = revision
            print(f'Ready without traffic: {revision} image={image}', flush=True)
        verify_private(cloud)
        for service, revision in revisions.items():
            require(active_api(cloud)['image'] == api['image'], 'API changed before traffic promotion')
            promote_and_verify(cloud, service, revision)
            print(f'Verified traffic: service={service} revision={revision} percent=100', flush=True)
        verify_private(cloud)
        require(active_api(cloud)['image'] == api['image'], 'API changed during web release')
        report = {'source_sha': sha, 'created_epoch': epoch, 'build_id': build_id, 'image_build_id': child, 'web_image': image, 'api_image_unchanged': api['image'], 'revisions': revisions}
        path.write_text(json.dumps(report, sort_keys=True) + '\n')
        cloud.run('storage', 'cp', path, WATERMARK, f'--if-generation-match={prior_generation}')
        require(json.loads(cloud.run('storage', 'cat', WATERMARK)) == report, 'successful release watermark readback mismatch')
        cloud.run('storage', 'rm', LOCK, f'--if-generation-match={lock_generation}')
        print('WEB RELEASE VERIFIED ' + json.dumps(report, sort_keys=True), flush=True)
        return report

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--execute', action='store_true')
    parser.add_argument('--sha', default='')
    parser.add_argument('--build-id', default='')
    parser.add_argument('--account', default=None)
    args = parser.parse_args()
    root = Path(__file__).resolve().parents[2]
    cloud = Cloud(args.account)
    try:
        if args.execute:
            execute(cloud, root, args.sha, args.build_id)
        else:
            verify_backend(cloud, root)
        return 0
    except (GuardError, OSError, ValueError, subprocess.TimeoutExpired) as exc:
        print(f'Web rollout refused/failed: {exc}. Inspect build and any retained lock; never delete it blindly.', file=sys.stderr)
        return 1

if __name__ == '__main__':
    raise SystemExit(main())
