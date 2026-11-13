"""Offline IAM policy fixtures: never contacts Google or reads credentials."""
import copy
import json
from pathlib import Path
import tempfile
import unittest

import v84_recovery_iam as iam

RUN = '7d4cdc80-1b21-4d2b-8dbc-401a59829dc6'


class PolicyFixture:
    def __init__(self, document=None):
        self.document = copy.deepcopy(document if document is not None else {'etag': 'synthetic-etag-1', 'version': 1})
        self.calls = []
        self.payloads = []

    def json(self, *args):
        self.calls.append(args)
        if args == ('secrets', 'get-iam-policy', iam.SECRET):
            return copy.deepcopy(self.document)
        if len(args) == 4 and args[:3] == ('secrets', 'set-iam-policy', iam.SECRET):
            path = Path(args[3])
            policy = json.loads(path.read_text())
            if policy.get('etag') != self.document.get('etag'):
                raise ValueError('synthetic etag conflict')
            self.payloads.append(path.name)
            policy['etag'] = 'synthetic-etag-' + str(len(self.payloads) + 1)
            self.document = policy
            return copy.deepcopy(policy)
        raise AssertionError('unexpected IAM fixture call: ' + str(args))


class V84RecoveryIAMTests(unittest.TestCase):
    def test_empty_policy_grant_and_revoke_with_distinct_payloads(self):
        cloud = PolicyFixture()
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            granted = iam.grant(cloud, directory, RUN)
            self.assertTrue(granted['verified'])
            binding = cloud.document['bindings'][0]
            self.assertEqual(binding['role'], iam.ROLE)
            self.assertEqual(binding['members'], [iam.MEMBER])
            self.assertTrue(iam._valid_own(binding, RUN))
            self.assertTrue(iam.revoke(cloud, directory, RUN)['verified'])
            self.assertEqual(cloud.document['bindings'], [])
            self.assertEqual(cloud.payloads, ['secret-iam-policy-grant.json', 'secret-iam-policy-revoke.json'])
            for path in directory.iterdir():
                self.assertEqual(path.stat().st_mode & 0o777, 0o600)
        self.assertTrue(all(call[:2] in (('secrets', 'get-iam-policy'), ('secrets', 'set-iam-policy')) for call in cloud.calls))

    def test_revoke_preserves_other_members_concurrent_bindings_and_audit(self):
        audit = [{'service': 'secretmanager.googleapis.com', 'auditLogConfigs': [{'logType': 'DATA_READ'}]}]
        cloud = PolicyFixture({'etag': 'synthetic-etag-1', 'version': 1, 'auditConfigs': audit})
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            iam.grant(cloud, directory, RUN)
            other = 'serviceAccount:synthetic-other@example.invalid'
            cloud.document['bindings'][0]['members'].append(other)
            unrelated = {'role': 'roles/viewer', 'members': ['user:synthetic@example.invalid']}
            cloud.document['bindings'].append(unrelated)
            result = iam.revoke(cloud, directory, RUN)
            self.assertTrue(result['verified'])
            self.assertEqual(cloud.document['bindings'][0]['members'], [other])
            self.assertIn(unrelated, cloud.document['bindings'])
            self.assertEqual(cloud.document['auditConfigs'], audit)

    def test_changed_run_condition_is_not_removed(self):
        cloud = PolicyFixture()
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            iam.grant(cloud, directory, RUN)
            cloud.document['bindings'][0]['condition']['expression'] = 'true'
            with self.assertRaisesRegex(ValueError, 'condition was changed'):
                iam.revoke(cloud, directory, RUN)
            self.assertEqual(len(cloud.payloads), 1)
            self.assertIn(iam.MEMBER, cloud.document['bindings'][0]['members'])

    def test_preexisting_access_and_missing_etag_rejected_without_write(self):
        for document in (
                {'etag': 'synthetic-etag', 'bindings': [{'role': iam.ROLE, 'members': [iam.MEMBER]}]},
                {'version': 1, 'bindings': []}):
            with self.subTest(document=document), tempfile.TemporaryDirectory() as temporary:
                cloud = PolicyFixture(document)
                with self.assertRaises(ValueError):
                    iam.grant(cloud, Path(temporary), RUN)
                self.assertEqual(cloud.payloads, [])

    def test_no_revoke_without_owned_private_intent(self):
        with tempfile.TemporaryDirectory() as temporary:
            cloud = PolicyFixture()
            with self.assertRaises(FileNotFoundError):
                iam.revoke(cloud, Path(temporary), RUN)
            self.assertEqual(cloud.calls, [])

    def test_semantic_ordering_normalizes_members_before_bindings(self):
        first = {'bindings': [{'role': 'roles/viewer', 'members': ['b', 'a']},
                              {'role': 'roles/viewer', 'members': ['c']}]}
        second = {'bindings': [{'role': 'roles/viewer', 'members': ['c']},
                               {'role': 'roles/viewer', 'members': ['a', 'b']}]}
        self.assertEqual(iam._semantic(first), iam._semantic(second))


if __name__ == '__main__':
    unittest.main()
