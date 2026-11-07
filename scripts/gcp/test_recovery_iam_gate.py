"""Synthetic tests of the operator-IAM readback barrier, not live IAM proof."""
import copy
from datetime import datetime, timedelta, timezone
import hashlib
import json
import unittest

from test_recovery_inspection import SHA, BID, DIGEST, IMAGE
import recovery_inspection as recovery
import recovery_iam_gate as gate

NOW = datetime(2026, 11, 7, 22, 0, tzinfo=timezone.utc)
REQUEST = {'uri': f'gs://{recovery.BUCKET}/recovery-inspections/{BID}/job-request.json', 'generation': '17', 'sha256': DIGEST}


def ready():
    return {'format': 'espacigo.recovery-iam-ready.v1', 'parent_build_id': BID,
            'source_sha': SHA, 'request': dict(REQUEST),
            'principal': f'serviceAccount:espacigo-release-job-builder@{recovery.PROJECT}.iam.gserviceaccount.com',
            'binding': {'synthetic_fixture': True},
            'expires_at': (NOW + timedelta(hours=1)).isoformat(),
            'policy_readback_etag': 'observed-etag', 'policy_readback_sha256': DIGEST,
            'custom_role_readback_sha256': DIGEST}


class Cloud:
    def __init__(self, value, missing=0):
        self.value = value; self.missing = missing; self.calls = []
    def json(self, *args):
        self.calls.append(args)
        if self.missing:
            self.missing -= 1
            raise ValueError('synthetic metadata unavailable')
        return {'generation': '23'}
    def run(self, *args):
        self.calls.append(args)
        return json.dumps(self.value)


class RecoveryIAMGateTests(unittest.TestCase):
    def setUp(self):
        self.req = recovery.make_request(SHA, BID, DIGEST, IMAGE,
                                        f'gs://{recovery.BUCKET}/source/recovery.tgz#17')
        self.ticks = 0; self.locks = 0; self.bindings = []
    def clock(self): return self.ticks
    def sleep(self, seconds): self.ticks += seconds
    def check_lock(self): self.locks += 1
    def verify_binding(self, binding, value):
        self.bindings.append(binding)
        if binding != {'synthetic_fixture': True}:
            raise ValueError('synthetic wrong binding')
    def invoke(self, cloud, **kwargs):
        return gate.wait_for_iam_ready(cloud, self.req, REQUEST,
                                      self.verify_binding, self.check_lock,
                                      timeout=2, interval=1, clock=self.clock,
                                      sleep=self.sleep, now=lambda: NOW, **kwargs)
    def test_waits_for_exact_parent_receipt_and_rechecks_lock(self):
        c = Cloud(ready(), missing=1)
        result = self.invoke(c)
        uri = f'gs://{recovery.BUCKET}/recovery-inspections/{BID}/iam-ready.json'
        self.assertEqual(c.calls, [('storage', 'objects', 'describe', uri),
                                  ('storage', 'objects', 'describe', uri),
                                  ('storage', 'cat', uri + '#23')])
        self.assertEqual(self.locks, 2)
        self.assertEqual(result['receipt']['uri'], uri)
        self.assertEqual(result['receipt']['generation'], '23')
        self.assertEqual(result['receipt']['sha256'], hashlib.sha256(json.dumps(ready()).encode()).hexdigest())
        self.assertEqual(self.bindings, [{'synthetic_fixture': True}])
    def test_missing_receipt_times_out_without_submit_or_execution(self):
        c = Cloud(ready(), missing=10)
        with self.assertRaisesRegex(ValueError, 'timed out'):
            self.invoke(c)
        self.assertTrue(all(call[:3] == ('storage', 'objects', 'describe') for call in c.calls))
        self.assertGreaterEqual(self.locks, 2)
    def test_malformed_stale_or_other_parent_receipt_fails_closed(self):
        mutations = [lambda x: x.update(parent_build_id='other'),
                     lambda x: x.update(source_sha='0' * 40),
                     lambda x: x['request'].update(generation='18'),
                     lambda x: x.update(principal='serviceAccount:other'),
                     lambda x: x.update(expires_at=(NOW - timedelta(seconds=1)).isoformat()),
                     lambda x: x.update(expires_at=(NOW + timedelta(hours=3)).isoformat()),
                     lambda x: x.update(binding={'broad': True}),
                     lambda x: x.update(policy_readback_sha256='bad'),
                     lambda x: x.update(extra='unsupported')]
        for mutate in mutations:
            with self.subTest(mutate=mutate):
                value = ready(); mutate(value)
                with self.assertRaises(ValueError): self.invoke(Cloud(value))
    def test_changed_lock_aborts_before_reading_receipt(self):
        c = Cloud(ready())
        def fail(): raise ValueError('lock changed')
        self.check_lock = fail
        with self.assertRaisesRegex(ValueError, 'lock changed'): self.invoke(c)
        self.assertEqual(c.calls, [])


if __name__ == '__main__': unittest.main()
