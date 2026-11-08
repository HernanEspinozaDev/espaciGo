"""Synthetic boundary tests; never authenticates or opens a network."""
import copy
import hashlib
import json
import unittest

import recovery_inspection as r


class HistoricalCloud:
    def __init__(self):
        self.lock = json.dumps({'build_id': r.PINNED_BUILD_ID, 'source_sha': r.PINNED_SOURCE_SHA})
        self.job = 'espacigo-db-r-' + r.PINNED_BUILD_ID.replace('-', '')
        self.uid = '22222222-3333-4444-8555-666666666666'
        self.execution = {'name': f'projects/{r.PROJECT}/locations/{r.REGION}/jobs/{self.job}/executions/{self.job}-v62zb',
                          'job': self.job, 'uid': '33333333-4444-4555-8666-777777777777',
                          'conditions': [{'type': 'Completed', 'state': 'CONDITION_FAILED'}],
                          'failedCount': 1}
        self.intent = json.dumps({'job': self.job, 'uid': self.uid, 'request': {'mode': 'apply'}})
        self.intent_reads = 0
    def json(self, *args):
        if args == ('storage', 'objects', 'describe', r.LOCK):
            return {'generation': r.PINNED_GENERATION}
        if args[:3] == ('storage', 'objects', 'describe'):
            return {'generation': '19'}
        if args == ('builds', 'describe', r.PINNED_BUILD_ID):
            return {'id': r.PINNED_BUILD_ID, 'status': 'FAILURE', 'buildTriggerId': r.TRIGGER,
                    'serviceAccount': r.DEPLOYER, 'substitutions': {'COMMIT_SHA': r.PINNED_SOURCE_SHA}}
        if args == ('run', 'jobs', 'describe', self.job):
            return {'metadata': {'uid': self.uid}}
        raise AssertionError(args)
    def run(self, *args):
        if args[:2] == ('storage', 'cat'):
            if args[2].startswith(r.LOCK + '#'): return self.lock
            self.intent_reads += 1
            return self.intent
        raise AssertionError(args)
    def historical_list(self): return {'executions': [copy.deepcopy(self.execution)]}
    def historical_get(self): return copy.deepcopy(self.execution)


class ObservedFactsTests(unittest.TestCase):
    def test_observation_hashes_the_same_single_intent_read_and_records_all_counters(self):
        cloud = HistoricalCloud()
        facts = r.inspect_failed_release(cloud)
        self.assertEqual(cloud.intent_reads, 1)
        self.assertEqual(facts['intent_sha256'], hashlib.sha256(cloud.intent.encode()).hexdigest())
        self.assertEqual(facts['lock_uri'], r.LOCK)
        self.assertEqual(facts['job_uid'], cloud.uid)
        self.assertEqual(facts['execution_uid'], cloud.execution['uid'])
        self.assertEqual(facts['succeeded_count'], 0)
        proof = r.make_failed_release_proof(facts, '11111111-2222-4333-8444-555555555555',
                                            'a' * 40, f'gs://{r.BUCKET}/source/synthetic.tgz#17', 'b' * 64)
        r.validate_failed_release_proof(proof)
        self.assertEqual(proof['failed_release']['build_id'], r.PINNED_BUILD_ID)
        self.assertEqual(proof['parent_source_sha'], 'a' * 40)
    def test_historical_counters_are_strict_and_cannot_hide_running_or_success(self):
        for field, value in [('failedCount', True), ('runningCount', False), ('runningCount', 1), ('succeededCount', 1)]:
            cloud = HistoricalCloud(); cloud.execution[field] = value
            with self.subTest(field=field, value=value), self.assertRaisesRegex(ValueError, 'terminal failure'):
                r.inspect_failed_release(cloud)


if __name__ == '__main__': unittest.main()
