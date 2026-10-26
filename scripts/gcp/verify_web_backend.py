#!/usr/bin/env python3
"""Read-only proof that a web release does not change the running API's sources.

Never trusts a tag or a claimed Git SHA as source identity. Reads the active
revision digest, SLSA v1 metadata, successful build and generation/hash-pinned
source archive. No SQL, secret payloads, archive extraction or cloud writes.
"""
from __future__ import annotations
import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import tarfile
import tempfile

PROJECT = 'espacigo-testprod'
REGION = 'southamerica-west1'
BUCKET = 'espacigo-testprod-cloud-build-447373903153'
SA = f'projects/{PROJECT}/serviceAccounts/espacigo-cloud-build-deployer@{PROJECT}.iam.gserviceaccount.com'
REGISTRY = f'{REGION}-docker.pkg.dev/{PROJECT}/{PROJECT}'
API = 'espacigo-test-api'
HEX64 = r'[a-f0-9]{64}'
UUID = r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}'

class GuardError(ValueError):
    pass

def require(ok, message):
    if not ok:
        raise GuardError(message)

class Cloud:
    def __init__(self, account=None):
        self.account = account

    def run(self, *args, timeout=120):
        cmd = ['gcloud', *map(str, args), f'--project={PROJECT}', '--quiet']
        if args[0] in ('run', 'builds'):
            cmd.append(f'--region={REGION}')
        if self.account:
            cmd.append(f'--account={self.account}')
        result = subprocess.run(cmd, text=True, capture_output=True, timeout=timeout)
        if result.returncode:
            raise GuardError(f'gcloud {args[0]} {args[1]} failed: {result.stderr.strip()}')
        return result.stdout

    def json(self, *args):
        return json.loads(self.run(*args, '--format=json'))

def relevant(name):
    return (name in {'Dockerfile', '.dockerignore', 'go.mod', 'go.sum', 'go.work', 'go.work.sum', 'planning/openapi.yaml'}
            or name.startswith(('cmd/', 'internal/', 'db/'))
            or (not name.startswith('frontend/') and name.endswith(('.go', '.mod', '.sum', '.work'))))

def archive_inventory(archive):
    inventory = {}
    total = 0
    with tarfile.open(archive, 'r:gz') as tar:
        for member in tar:
            raw = member.name
            while raw.startswith('./'):
                raw = raw[2:]
            path = PurePosixPath(raw)
            require(not path.is_absolute() and '..' not in path.parts and '\\' not in raw, 'unsafe source archive path')
            if not relevant(raw) or member.isdir():
                continue
            require(member.isfile() and not member.issym() and not member.islnk(), 'non-regular backend source in archive')
            require(raw not in inventory, 'duplicate backend source in archive')
            total += member.size
            require(0 <= member.size <= 32 * 1024 * 1024 and total <= 256 * 1024 * 1024, 'source archive exceeds backend size limit')
            stream = tar.extractfile(member)
            if stream is None:
                raise GuardError('unreadable source member')
            inventory[raw] = hashlib.sha256(stream.read()).hexdigest()
    require(bool(inventory) and 'go.mod' in inventory, 'empty or incomplete backend source archive')
    return inventory

def backend_inventory(root):
    result = {}
    for directory, dirs, files in os.walk(root, followlinks=False):
        dirs[:] = [d for d in dirs if d not in {'.git', '.local', '.next', 'node_modules', '__pycache__', 'frontend'}]
        for name in dirs + files:
            path = Path(directory) / name
            rel = path.relative_to(root).as_posix()
            if relevant(rel):
                require(not path.is_symlink(), 'backend source symlinks are not accepted')
                if path.is_file():
                    result[rel] = hashlib.sha256(path.read_bytes()).hexdigest()
    return result

def compare_backend(root, archive):
    expected = archive_inventory(archive)
    actual = backend_inventory(root)
    changed = [p for p in sorted(set(actual) | set(expected)) if actual.get(p) != expected.get(p)]
    require(not changed, 'Backend differs: full rollout with fresh schema evidence required. Changed paths: ' + ', '.join(changed[:15]))
    return len(expected)

def active_api(cloud):
    service = cloud.json('run', 'services', 'describe', API)
    traffic = [t for t in service.get('status', {}).get('traffic', []) if t.get('percent', 0)]
    require(len(traffic) == 1 and traffic[0].get('percent') == 100, 'API must have one explicitly identifiable revision at 100% traffic')
    revision = traffic[0].get('revisionName', '')
    require(revision.startswith(API + '-'), 'API active revision missing')
    metadata = cloud.json('run', 'revisions', 'describe', revision)
    require(any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in metadata.get('status', {}).get('conditions', [])), 'active API revision is not Ready')
    image = metadata.get('status', {}).get('imageDigest', '')
    require(re.fullmatch(re.escape(REGISTRY + '/espacigo-api@sha256:') + HEX64, image) is not None, 'API digest/registry is not canonical')
    return {'revision': revision, 'image': image, 'url': service['status']['url']}

def source_evidence(provenance, image):
    digest = image.rsplit('@', 1)[-1]
    require(provenance.get('image_summary', {}).get('digest') == digest, 'provenance digest differs from active API')
    candidates = []
    for occurrence in provenance.get('provenance_summary', {}).get('provenance', []):
        statement = occurrence.get('build', {}).get('inTotoSlsaProvenanceV1', {})
        if statement.get('predicateType') != 'https://slsa.dev/provenance/v1':
            continue
        subjects = statement.get('subject', [])
        if not any(s.get('digest', {}).get('sha256') == digest.removeprefix('sha256:') for s in subjects):
            continue
        predicate = statement.get('predicate', {})
        invocation = predicate.get('runDetails', {}).get('metadata', {}).get('invocationId', '')
        prefix = f'https://cloudbuild.googleapis.com/v1/projects/{PROJECT}/locations/{REGION}/builds/'
        require(invocation.startswith(prefix) and re.fullmatch(UUID, invocation[len(prefix):]) is not None, 'provenance is not from the regional trusted Cloud Build project')
        definition = predicate.get('buildDefinition', {})
        source = definition.get('externalParameters', {}).get('gcsPath', {})
        require(source.get('bucket') == BUCKET and str(source.get('object', '')).startswith('source/') and re.fullmatch(r'[1-9][0-9]*', str(source.get('generation', ''))) is not None, 'provenance source is not generation-pinned in approved bucket')
        uri = f"gs://{BUCKET}/{source['object']}#{source['generation']}"
        hashes = [d.get('digest', {}).get('sha256') for d in definition.get('resolvedDependencies', []) if d.get('uri') == uri]
        require(len(hashes) == 1 and re.fullmatch(HEX64, hashes[0] or '') is not None, 'provenance source hash missing or ambiguous')
        candidates.append({'build_id': invocation[len(prefix):], 'uri': uri, 'sha256': hashes[0], 'source': source})
    require(len(candidates) == 1, 'exactly one matching SLSA v1 source attestation is required')
    return candidates[0]

def verify_backend(cloud, root):
    api = active_api(cloud)
    evidence = source_evidence(cloud.json('artifacts', 'docker', 'images', 'describe', api['image'], '--show-provenance'), api['image'])
    build = cloud.json('builds', 'describe', evidence['build_id'])
    require(build.get('status') == 'SUCCESS' and build.get('serviceAccount') == SA, 'API source build was not successful under dedicated deployer')
    require(any(i.get('digest') == api['image'].rsplit('@', 1)[-1] and i.get('name', '').startswith(REGISTRY + '/espacigo-api:') for i in build.get('results', {}).get('images', [])), 'source build results do not contain active API digest')
    source = build.get('sourceProvenance', {}).get('resolvedStorageSource', {})
    require(source.get('bucket') == BUCKET and source.get('object') == evidence['source']['object'] and str(source.get('generation')) == str(evidence['source']['generation']), 'source build generation differs from provenance')
    with tempfile.TemporaryDirectory(prefix='espacigo-backend-proof-', dir=os.environ.get('TMPDIR') or str(root)) as temp:
        archive = Path(temp) / 'source.tgz'
        cloud.run('storage', 'cp', evidence['uri'], archive)
        require(hashlib.sha256(archive.read_bytes()).hexdigest() == evidence['sha256'], 'downloaded API source hash differs from provenance')
        count = compare_backend(root, archive)
    require(active_api(cloud) == api, 'API changed while checking source parity; retry with current API')
    api['files'] = count
    api['source_build'] = evidence['build_id']
    print(f"Backend parity verified: revision={api['revision']} files={count} digest={api['image'].rsplit('@', 1)[-1]}", flush=True)
    return api

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--account', default=None)
    parser.add_argument('--root', type=Path, default=Path(__file__).resolve().parents[2])
    args = parser.parse_args()
    try:
        print(json.dumps(verify_backend(Cloud(args.account), args.root.resolve()), sort_keys=True))
        return 0
    except (GuardError, OSError, subprocess.TimeoutExpired, ValueError, tarfile.TarError) as exc:
        print(f'Web release guard refused: {exc}', file=__import__('sys').stderr)
        return 1

if __name__ == '__main__':
    raise SystemExit(main())
