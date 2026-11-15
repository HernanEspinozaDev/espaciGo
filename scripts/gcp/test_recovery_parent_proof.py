import copy
import json
import unittest
from datetime import datetime, timedelta, timezone

import recovery_parent_proof as proof

PARENT = '7c4f59df-fc71-4c8e-a4e4-68fa8e123456'
SHA = 'a' * 40
URI = f'gs://{proof.BUCKET}/recovery/source.tar.gz#12345'
ARCHIVE = 'b' * 64
NOW = datetime(2026, 11, 7, 12, 0, tzinfo=timezone.utc)


def _facts(now):
    bid = proof.PINNED_BUILD_ID
    job = 'espacigo-db-r-' + bid.replace('-', '')
    execution = job + '-v62zb'
    intent_uri = f'gs://{proof.BUCKET}/releases/{bid}/job-intent-apply.json'
    lock_text = json.dumps({'build_id': bid, 'source_sha': proof.PINNED_SOURCE_SHA}, sort_keys=True)
    return {'observed_at': now.isoformat(), 'build_id': bid,
            'source_sha': proof.PINNED_SOURCE_SHA, 'lock_uri': proof.LOCK,
            'lock_generation': proof.PINNED_GENERATION, 'lock_text': lock_text,
            'job_name': job, 'job_uid': 'a12e34bc-5678-4abc-8def-1234567890ab',
            'execution_name': f'projects/{proof.PROJECT}/locations/{proof.REGION}/jobs/{job}/executions/{execution}',
            'execution_uid': 'b12e34bc-5678-4abc-8def-1234567890ab',
            'execution_state': 'CONDITION_FAILED', 'failed_count': 1,
            'running_count': 0, 'succeeded_count': 0, 'list_count': 1,
            'next_page': '', 'intent_uri': intent_uri, 'intent_generation': '9876',
            'intent_sha256': 'c' * 64, 'failed_execution': execution}


def proof_fixture(parent_bid=None, sha=SHA, uri=URI, digest=ARCHIVE, now=NOW):
    return proof.make_failed_release_proof(_facts(now), parent_bid or PARENT, sha, uri, digest)


def request_fixture(sha, bid, digest, image, uri):
    """Synthetic request, never a production observation."""
    import recovery_inspection as recovery
    value = proof_fixture(bid, sha, uri, digest, datetime.now(timezone.utc))
    return recovery.make_request(sha, bid, digest, image, uri, value)


class ProofProtocolTests(unittest.TestCase):
    def test_make_proof_keeps_historical_and_parent_namespaces_separate(self):
        facts = _facts(NOW); original = copy.deepcopy(facts)
        made = proof.make_failed_release_proof(facts, PARENT, SHA, URI, ARCHIVE)
        self.assertEqual(facts, original)
        self.assertEqual(set(made), proof.PROOF_KEYS)
        self.assertEqual(made['failed_release']['build_id'], proof.PINNED_BUILD_ID)
        self.assertEqual(made['parent_build_id'], PARENT)

    def test_valid_proof_and_exact_600_second_boundary(self):
        p = proof_fixture(now=NOW - timedelta(seconds=600))
        proof.validate_request_proof(p, PARENT, SHA, URI, ARCHIVE, NOW)

    def test_rejects_missing_and_extra_proof_fields(self):
        p = proof_fixture(); del p['failed_release']['job_uid']
        with self.assertRaises(proof.ProofError): proof.validate_request_proof(p, PARENT, SHA, URI, ARCHIVE, NOW)
        p = proof_fixture(); p['unexpected'] = 1
        with self.assertRaises(proof.ProofError): proof.validate_request_proof(p, PARENT, SHA, URI, ARCHIVE, NOW)

    def test_rejects_bad_historical_identity_uuid_lock_counts_and_boolean(self):
        mutations = [lambda f: f.update(build_id='0'*36), lambda f: f.update(source_sha='d'*40), lambda f: f.update(job_uid='not-uuid'), lambda f: f.update(lock_text='{}'), lambda f: f.update(failed_count=True), lambda f: f.update(succeeded_count=1), lambda f: f.update(intent_sha256='z'*64)]
        for mutate in mutations:
            p = proof_fixture(); mutate(p['failed_release'])
            with self.subTest(mutate=mutate), self.assertRaises(proof.ProofError): proof.validate_request_proof(p, PARENT, SHA, URI, ARCHIVE, NOW)

    def test_rejects_future_stale_and_naive_times(self):
        for stamp in ((NOW + timedelta(seconds=1)).isoformat(), (NOW-timedelta(seconds=601)).isoformat(), '2026-11-07T12:00:00'):
            p = proof_fixture(); p['observed_at'] = stamp
            with self.subTest(stamp=stamp), self.assertRaises(proof.ProofError): proof.validate_request_proof(p, PARENT, SHA, URI, ARCHIVE, NOW)

    def test_binds_parent_source_uri_generation_and_archive(self):
        for args in ((PARENT, 'd'*40, URI, ARCHIVE), (PARENT, SHA, URI+'9', ARCHIVE), (PARENT, SHA, URI, 'd'*64), ('7c4f59df-fc71-4c8e-a4e4-68fa8e999999', SHA, URI, ARCHIVE)):
            with self.subTest(args=args), self.assertRaises(proof.ProofError): proof.validate_request_proof(proof_fixture(), *args, now=NOW)

    def test_canonical_digest_binds_complete_proof(self):
        p = proof_fixture(); digest = proof.canonical_proof_sha256(p)
        q = copy.deepcopy(p); q['failed_release']['execution_uid'] = 'c12e34bc-5678-4abc-8def-1234567890ab'
        self.assertNotEqual(digest, proof.canonical_proof_sha256(q)); self.assertEqual(len(digest), 64)


if __name__ == '__main__': unittest.main()
