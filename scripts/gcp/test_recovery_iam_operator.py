"""Synthetic operator tests: no IAM calls and no credentials."""
import copy
import io
from datetime import datetime, timezone
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import recovery_iam_operator as operator
import recovery_inspection as recovery
from test_recovery_parent_proof import request_fixture
from test_recovery_inspection import SHA, BID, DIGEST, IMAGE, parent

NOW = datetime(2026, 11, 7, 22, 0, tzinfo=timezone.utc)


class FakeIAM:
    def __init__(self):
        self.policy = {'version': 3, 'etag': 'initial', 'bindings': [{'role': 'roles/viewer', 'members': ['user:synthetic@example.test']}]}
        self.role = None; self.calls = []
    def policy_get(self): return copy.deepcopy(self.policy)
    def policy_set(self, path):
        requested = json.loads(Path(path).read_text()); self.calls.append('policy-set')
        assert requested['etag'] == self.policy['etag']
        requested['etag'] += '-new'; self.policy = requested
        return copy.deepcopy(self.policy)
    def role_get(self, role):
        if self.role is None: raise operator.NotFound('synthetic absent')
        return copy.deepcopy(self.role)
    def role_create(self, role, path):
        assert self.role is None
        self.calls.append('role-create'); self.role = json.loads(Path(path).read_text())
        self.role.update(name=role, etag='role-etag')
        return copy.deepcopy(self.role)
    def role_delete(self, role, etag):
        assert self.role is not None
        assert self.role['etag'] == etag
        self.calls.append('role-delete'); self.role['deleted'] = True


class FakeCloud:
    def __init__(self):
        self.req = request_fixture(SHA, BID, DIGEST, IMAGE, f'gs://{recovery.BUCKET}/source/recovery.tgz#17')
    def json(self, *args):
        if args[:2] == ('builds', 'describe'): return parent()
        if args[:3] == ('storage', 'objects', 'describe'): return {'generation': '17'}
        raise AssertionError(args)
    def run(self, *args):
        if args[:2] == ('storage', 'cat'): return json.dumps(self.req)
        raise AssertionError(args)


class OperatorTests(unittest.TestCase):
    def test_role_delete_uses_exact_rest_target_and_etag(self):
        role = 'projects/espacigo-testprod/roles/espacigoRecoveryRead20261107a1b2c3d4'
        with patch.object(operator.subprocess, 'run') as sdk, \
                patch('urllib.request.build_opener') as opener:
            sdk.return_value.returncode = 0; sdk.return_value.stdout = 'synthetic-bearer\n'
            opener.return_value.open.return_value = io.BytesIO(b'{"deleted":true}')
            operator.IAM('operator@example.test').role_delete(role, 'own-etag+/=')
            request = opener.return_value.open.call_args.args[0]
            self.assertEqual(request.get_method(), 'DELETE')
            self.assertTrue(request.full_url.startswith('https://iam.googleapis.com/v1/' + role + '?etag='))
            self.assertEqual(request.get_header('Authorization'), 'Bearer synthetic-bearer')

    def test_uncertain_ready_handoff_is_not_auto_revoked(self):
        cloud, iam = FakeCloud(), FakeIAM()
        lock = {'lock_generation': recovery.PINNED_GENERATION, 'lock_text': '{}'}
        with tempfile.TemporaryDirectory() as root, patch.object(operator.r, 'inspect_failed_release', return_value=lock), \
                patch.object(operator.r, 'assert_lock'), patch.object(operator.r, 'record_evidence', side_effect=TimeoutError('publication uncertain')):
            lease = Path(root) / 'lease'
            with self.assertRaises(TimeoutError):
                operator.grant(cloud, iam, Path(root), lease, BID, SHA, now=NOW)
            self.assertTrue((lease / 'ready-publish-intent.json').exists())
            self.assertTrue((lease / 'handoff-uncertain.json').exists())
        self.assertEqual(iam.calls, ['role-create', 'policy-set'])

    def test_revoke_preserves_concurrent_bindings_and_reports_drift(self):
        cloud, iam = FakeCloud(), FakeIAM()
        lock = {'lock_generation': recovery.PINNED_GENERATION, 'lock_text': '{}'}
        with tempfile.TemporaryDirectory() as root, patch.object(operator.r, 'inspect_failed_release', return_value=lock), \
                patch.object(operator.r, 'assert_lock'), patch.object(operator.r, 'record_evidence'):
            lease = Path(root) / 'lease'
            operator.grant(cloud, iam, Path(root), lease, BID, SHA, now=NOW)
            concurrent = {'role': 'roles/logging.viewer', 'members': ['user:other-synthetic@example.test']}
            iam.policy['bindings'].append(concurrent); iam.policy['etag'] = 'concurrent'
            result = operator.revoke(cloud, iam, Path(root), lease)
            self.assertFalse(result['baseline_preserved'])
            self.assertTrue(result['binding_absent'])
            self.assertIn(concurrent, iam.policy['bindings'])

    def test_repeated_revoke_emits_fresh_readback_bound_evidence(self):
        cloud, iam = FakeCloud(), FakeIAM()
        lock = {'lock_generation': recovery.PINNED_GENERATION, 'lock_text': '{}'}
        with tempfile.TemporaryDirectory() as root, patch.object(operator.r, 'inspect_failed_release', return_value=lock), \
                patch.object(operator.r, 'assert_lock'), patch.object(operator.r, 'record_evidence') as evidence:
            lease = Path(root) / 'lease'
            operator.grant(cloud, iam, Path(root), lease, BID, SHA, now=NOW)
            operator.revoke(cloud, iam, Path(root), lease)
            evidence.reset_mock()
            result = operator.revoke(cloud, iam, Path(root), lease)
            self.assertTrue(result['binding_absent'])
            evidence.assert_called_once()
            self.assertTrue(evidence.call_args.args[3].startswith('iam-revoked-'))

    def test_create_collision_does_not_delete_an_unowned_role(self):
        cloud, iam = FakeCloud(), FakeIAM()
        def collision(role, path):
            # A different creator wins between absence readback and CREATE.
            iam.role = {'name': role, 'etag': 'other-creator', 'stage': 'GA',
                        'includedPermissions': sorted(operator.policy.PERMISSIONS)}
            raise ValueError('synthetic ALREADY_EXISTS')
        iam.role_create = collision
        lock = {'lock_generation': recovery.PINNED_GENERATION, 'lock_text': '{}'}
        with tempfile.TemporaryDirectory() as root, patch.object(operator.r, 'inspect_failed_release', return_value=lock), \
                patch.object(operator.r, 'assert_lock'), patch.object(operator.r, 'record_evidence'):
            with self.assertRaisesRegex(ValueError, 'ALREADY_EXISTS'):
                operator.grant(cloud, iam, Path(root), Path(root) / 'lease', BID, SHA, now=NOW)
        self.assertNotIn('role-delete', iam.calls)
        assert iam.role is not None
        self.assertFalse(iam.role.get('deleted', False))

    def test_grant_readback_receipt_and_revoke_preserve_baseline(self):
        cloud, iam = FakeCloud(), FakeIAM()
        baseline = copy.deepcopy(iam.policy)
        lock = {'lock_generation': recovery.PINNED_GENERATION, 'lock_text': '{}'}
        with tempfile.TemporaryDirectory() as root, patch.object(operator.r, 'inspect_failed_release', return_value=lock), \
                patch.object(operator.r, 'assert_lock'), patch.object(operator.r, 'record_evidence', return_value={'generation': '21'}) as evidence:
            lease = Path(root) / 'own-lease'
            result = operator.grant(cloud, iam, Path(root), lease, BID, SHA, now=NOW)
            self.assertEqual(iam.calls, ['role-create', 'policy-set'])
            self.assertEqual(evidence.call_args.args[3], 'iam-ready')
            self.assertEqual(evidence.call_args.args[4]['binding'], result['binding'])
            assert iam.role is not None
            self.assertEqual(set(iam.role['includedPermissions']), {'storage.objects.get'})
            revoked = operator.revoke(cloud, iam, Path(root), lease)
            self.assertTrue(revoked['binding_absent'])
            self.assertTrue(revoked['role_deleted'])
            self.assertTrue(revoked['baseline_preserved'])
            self.assertEqual(operator.policy.semantic_policy(iam.policy), operator.policy.semantic_policy(baseline))
            self.assertTrue(evidence.call_args.args[3].startswith('iam-revoked-'))
    def test_readiness_is_not_published_without_matching_policy_readback(self):
        cloud, iam = FakeCloud(), FakeIAM()
        original_set = iam.policy_set
        def corrupt(path):
            acknowledgement = original_set(path)
            iam.policy['bindings'] = [b for b in iam.policy['bindings'] if not b['role'].startswith('projects/')]
            return acknowledgement
        iam.policy_set = corrupt
        lock = {'lock_generation': recovery.PINNED_GENERATION, 'lock_text': '{}'}
        with tempfile.TemporaryDirectory() as root, patch.object(operator.r, 'inspect_failed_release', return_value=lock), \
                patch.object(operator.r, 'assert_lock'), patch.object(operator.r, 'record_evidence') as evidence:
            with self.assertRaisesRegex(ValueError, 'policy readback'):
                operator.grant(cloud, iam, Path(root), Path(root) / 'lease', BID, SHA, now=NOW)
            self.assertFalse(any(call.args[3] == 'iam-ready' for call in evidence.call_args_list))
    def test_wrong_source_and_symlink_lease_fail_before_iam_mutation(self):
        cloud, iam = FakeCloud(), FakeIAM()
        with tempfile.TemporaryDirectory() as root:
            base = Path(root)
            (base / 'link').symlink_to(base / 'other')
            with self.assertRaises(ValueError):
                operator.grant(cloud, iam, base, base / 'link', BID, SHA, now=NOW)
            with self.assertRaises(ValueError):
                operator.grant(cloud, iam, base, base / 'lease', BID, '0' * 40, now=NOW)
        self.assertEqual(iam.calls, [])


if __name__ == '__main__': unittest.main()
