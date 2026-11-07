#!/usr/bin/env python3
"""Operator-only temporary IAM lease. Never executes a Job or connects to SQL."""
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import urllib.parse
import urllib.request
import uuid

import recovery_inspection as r
import recovery_iam_policy as policy


class NotFound(ValueError):
    pass


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def require(ok, message):
    if not ok: raise ValueError(message)


def encoded(value):
    return (json.dumps(value, sort_keys=True, separators=(',', ':')) + '\n').encode()


def digest(value):
    return hashlib.sha256(encoded(value)).hexdigest()


def save(directory, name, value):
    require(not directory.is_symlink() and directory.is_dir(), 'unsafe lease directory')
    path = directory / name
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'wb') as stream:
        stream.write(encoded(value)); stream.flush(); os.fsync(stream.fileno())
    return path


def load(directory, name):
    require(not directory.is_symlink() and directory.is_dir(), 'unsafe lease directory')
    fd = os.open(directory / name, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, 'r') as stream:
        metadata = os.fstat(stream.fileno())
        require(stat.S_ISREG(metadata.st_mode) and stat.S_IMODE(metadata.st_mode) == 0o600
                and metadata.st_uid == os.getuid() and metadata.st_nlink == 1, 'unsafe lease file')
        return json.load(stream)


class IAM:
    """Fixed SDK operations; credentials remain with the existing gcloud account."""
    def __init__(self, account): self.account = account
    def _json(self, *args):
        response = subprocess.run(['gcloud', *map(str, args), '--project=' + r.PROJECT,
                                   '--account=' + self.account, '--quiet', '--format=json'],
                                  text=True, capture_output=True, timeout=120)
        if response.returncode:
            if 'NOT_FOUND' in response.stderr: raise NotFound('IAM target absent')
            raise ValueError('IAM SDK operation failed; inspect private lease and read back before retry')
        return json.loads(response.stdout or '{}')
    def policy_get(self): return self._json('projects', 'get-iam-policy', r.PROJECT)
    def policy_set(self, path): return self._json('projects', 'set-iam-policy', r.PROJECT, path)
    def role_get(self, role): return self._json('iam', 'roles', 'describe', role.rsplit('/', 1)[1])
    def role_create(self, role, path): return self._json('iam', 'roles', 'create', role.rsplit('/', 1)[1], '--file=' + str(path))
    def role_delete(self, role, etag):
        # The installed SDK delete command has no etag flag. REST supports a
        # guarded DELETE: /iam/docs/reference/rest/v1/projects.roles/delete.
        require(policy.ROLE.fullmatch(role) and isinstance(etag, str) and etag, 'invalid owned role deletion identity')
        try:
            response = subprocess.run(['gcloud', 'auth', 'print-access-token', '--account=' + self.account, '--quiet'],
                                      capture_output=True, text=True, timeout=30)
            require(response.returncode == 0 and response.stdout.strip(), 'operator authentication unavailable')
            url = 'https://iam.googleapis.com/v1/' + role + '?' + urllib.parse.urlencode({'etag': etag})
            request = urllib.request.Request(url, headers={'Authorization': 'Bearer ' + response.stdout.strip()}, method='DELETE')
            with urllib.request.build_opener(NoRedirect).open(request, timeout=30) as result:
                body = result.read(1048577)
            require(len(body) <= 1048576, 'role deletion response oversized')
            return json.loads(body)
        except Exception:
            raise ValueError('owned role deletion uncertain; read back before retry') from None


def verify_role(value, role):
    require(value.get('name') == role and set(value.get('includedPermissions', [])) == policy.PERMISSIONS
            and value.get('stage') == 'GA' and not value.get('deleted', False)
            and isinstance(value.get('etag'), str) and value['etag'], 'temporary role readback differs')


def verify_owned_role(current, created):
    verify_role(created, created['name']); verify_role(current, created['name'])
    require(all(current.get(k) == created.get(k) for k in ('name', 'etag', 'title', 'description', 'stage')),
            'own role revision changed; refuse deleting an unowned revision')


def write_policy(iam, path, expected):
    acknowledgement = iam.policy_set(path)
    require(policy.semantic_policy(acknowledgement) == policy.semantic_policy(expected), 'policy write acknowledgement differs')
    observed = iam.policy_get()
    require(policy.semantic_policy(observed) == policy.semantic_policy(expected), 'project policy readback differs')
    return observed


def grant(cloud, iam, root, lease, parent_bid, sha, now=None):
    require(re.fullmatch(r.UUID, parent_bid) and re.fullmatch('[a-f0-9]{40}', sha), 'invalid authorized source/parent')
    require(not root.is_symlink() and root.is_dir(), 'unsafe operator root')
    try: parts = lease.relative_to(root).parts
    except ValueError as error: raise ValueError('lease must be under operator root') from error
    require(parts and '..' not in parts, 'unsafe lease path')
    directory = root
    for part in parts[:-1]:
        directory /= part
        require(not directory.is_symlink(), 'lease parent symlink rejected')
        directory.mkdir(exist_ok=True, mode=0o700)
    require(not lease.exists() and not lease.is_symlink(), 'lease already exists or is a symlink')
    lease.mkdir(mode=0o700)
    uri = f'gs://{r.BUCKET}/recovery-inspections/{parent_bid}/job-request.json'
    metadata = cloud.json('storage', 'objects', 'describe', uri)
    generation = str(metadata.get('generation', ''))
    require(re.fullmatch('[1-9][0-9]*', generation), 'invalid exact request generation')
    text = cloud.run('storage', 'cat', uri + '#' + generation)
    req = json.loads(text); r.validate_request(req)
    require(req['source_sha'] == sha and req['build_id'] == parent_bid, 'request source/parent differs')
    parent = cloud.json('builds', 'describe', parent_bid)
    source = r.parent_identity(parent, sha, parent_bid, req['source_archive_sha256'])
    require(source == req['source_uri'], 'request archive differs from active parent')
    lock = r.inspect_failed_release(cloud)
    current = now or datetime.now(timezone.utc)
    expiry = (current + timedelta(hours=1)).isoformat().replace('+00:00', 'Z')
    role = f'projects/{r.PROJECT}/roles/espacigoRecoveryRead20261107{uuid.uuid4().hex[:8]}'
    binding = policy.build_binding(parent_bid, role, expiry, now=current)
    baseline = iam.policy_get()
    policy.semantic_policy(baseline)  # Reject malformed/legacy conditions before role creation.
    require(not any('_withcond_' in b.get('role', '') for b in baseline.get('bindings', [])), 'policy conditions were not retrieved intact')
    try: iam.role_get(role)
    except NotFound: pass
    else: raise ValueError('refusing pre-existing custom role')
    request_receipt = {'uri': uri, 'generation': generation, 'sha256': hashlib.sha256(text.encode()).hexdigest()}
    state = {'parent_build_id': parent_bid, 'source_sha': sha, 'grant_time': current.isoformat(),
             'expires_at': expiry, 'role': role, 'binding': binding, 'request': request_receipt}
    save(lease, 'before.json', baseline); save(lease, 'state.json', state)
    definition = {'title': 'Temporary recovery inspection reader', 'description': 'Task-owned exact-resource read lease; no SQL, secrets or Job execution',
                  'stage': 'GA', 'includedPermissions': sorted(policy.PERMISSIONS)}
    handoff_started = False
    try:
        role_path = save(lease, 'role-definition.json', definition)
        created = iam.role_create(role, role_path); verify_role(created, role)
        save(lease, 'role-created.json', created)
        role_readback = iam.role_get(role); verify_owned_role(role_readback, created)
        fresh = iam.policy_get()
        require(policy.semantic_policy(fresh) == policy.semantic_policy(baseline), 'project policy changed before grant; no stale overwrite')
        delta = policy.grant_policy(fresh, binding, now=current if now is not None else datetime.now(timezone.utc))
        r.assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
        observed = write_policy(iam, save(lease, 'grant-request.json', delta), delta)
        save(lease, 'grant-readback.json', observed)
        role_readback = iam.role_get(role); verify_owned_role(role_readback, created)
        r.assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
        readiness = {'format': 'espacigo.recovery-iam-ready.v1', 'parent_build_id': parent_bid,
                     'source_sha': sha, 'request': request_receipt, 'principal': policy.MEMBER,
                     'binding': binding, 'expires_at': expiry, 'policy_readback_etag': observed['etag'],
                     'policy_readback_sha256': digest(observed), 'custom_role_readback_sha256': digest(role_readback)}
        save(lease, 'ready-publish-intent.json', readiness)
        # Once publication starts the parent may consume readiness. A local
        # bookkeeping/network failure must not silently withdraw that handoff.
        handoff_started = True
        receipt = r.record_evidence(cloud, root, parent_bid, 'iam-ready', readiness)
        return {**state, 'ready_receipt': receipt}
    except Exception:
        # A failed/uncertain write is never retried. Readback-first withdrawal
        # removes only the exact task-owned binding, even if setIamPolicy timed out.
        if handoff_started:
            save(lease, 'handoff-uncertain.json', {'inspect_parent_and_ready_object_before_withdrawal': True,
                                                'automatic_withdrawal': False})
        else:
            try: revoke(cloud, iam, root, lease)
            except Exception as cleanup_error:
                save(lease, 'cleanup-blocked.json', {'error_type': type(cleanup_error).__name__, 'manual_readback_required': True})
        raise


def revoke(cloud, iam, root, lease):
    state = load(lease, 'state.json'); baseline = load(lease, 'before.json')
    binding = policy.build_binding(state['parent_build_id'], state['role'], state['expires_at'],
                                   now=datetime.fromisoformat(state['grant_time']))
    require(binding == state['binding'], 'lease binding altered; refuse unrelated IAM mutation')
    current = iam.policy_get()
    attempt = uuid.uuid4().hex[:8]
    own = [b for b in current.get('bindings', []) if b.get('role') == state['role']]
    if own:
        require(own == [binding], 'own role binding drifted; do not remove other bindings')
        updated = policy.revoke_policy(current, binding)
        write_policy(iam, save(lease, 'revoke-request-' + attempt + '.json', updated), updated)
    observed = iam.policy_get()
    require(not any(b.get('role') == state['role'] for b in observed.get('bindings', [])), 'temporary binding still present')
    role_deleted, role_absent = False, False
    try:
        role = iam.role_get(state['role'])
        require(role.get('name') == state['role'], 'role identity differs during withdrawal')
        if not role.get('deleted', False):
            require((lease / 'role-created.json').is_file(), 'own role creation not proven; leave unowned role unchanged')
            created = load(lease, 'role-created.json')
            verify_owned_role(role, created)
            iam.role_delete(state['role'], role['etag'])
        require(iam.role_get(state['role']).get('deleted') is True, 'own role deletion not confirmed')
        role_deleted = True
    except NotFound:
        role_absent = True
    result = {'parent_build_id': state['parent_build_id'], 'role': state['role'], 'binding_absent': True,
              'role_deleted': role_deleted, 'role_absent': role_absent,
              'baseline_preserved': policy.semantic_policy(observed) == policy.semantic_policy(baseline),
              'policy_readback_etag': observed['etag'], 'policy_readback_sha256': digest(observed),
              'limits': 'Confirms own binding/role withdrawal, not absence of unrelated inherited authority'}
    result['concurrent_policy_changed'] = not result['baseline_preserved']
    save(lease, 'revoke-readback-' + attempt + '.json', result)
    r.record_evidence(cloud, root, state['parent_build_id'], 'iam-revoked-' + attempt, result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('mode', choices=['grant', 'revoke'])
    parser.add_argument('--account', required=True)
    parser.add_argument('--parent-build-id', required=True)
    parser.add_argument('--sha')
    args = parser.parse_args()
    require(re.fullmatch(r.UUID, args.parent_build_id), 'invalid parent ID')
    root = Path(__file__).resolve().parents[2]
    lease = root / '.local' / 'recovery-iam' / args.parent_build_id
    cloud, iam = r.Cloud(account=args.account), IAM(args.account)
    if args.mode == 'grant':
        require(args.sha is not None, 'reviewed source SHA required')
        result = grant(cloud, iam, root, lease, args.parent_build_id, args.sha)
    else:
        require(load(lease, 'state.json')['parent_build_id'] == args.parent_build_id, 'lease parent differs from selected operation')
        result = revoke(cloud, iam, root, lease)
    print(json.dumps({'lease': str(lease), 'result': result}, sort_keys=True))


if __name__ == '__main__':
    main()
