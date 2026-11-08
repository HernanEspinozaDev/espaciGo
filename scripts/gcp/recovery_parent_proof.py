"""Pure proof protocol for binding a recovery parent to historical evidence."""
from __future__ import annotations

import hashlib
import json
import re
from datetime import datetime, timezone

from verify_web_backend import PROJECT, REGION, BUCKET, UUID, HEX64
from rollout_web import LOCK

PINNED_BUILD_ID = '2dc002a9-5831-4717-8064-6432b98d3e01'
PINNED_SOURCE_SHA = 'bb10309129adad9d7204b9b54bb2259186a34242'
PINNED_GENERATION = '1794036269394395'
HEX = HEX64
FACT_KEYS = frozenset({'build_id','source_sha','lock_uri','lock_generation','lock_text','job_name','job_uid','execution_name','execution_uid','execution_state','failed_count','running_count','succeeded_count','list_count','next_page','intent_uri','intent_generation','intent_sha256','failed_execution'})
PROOF_KEYS = frozenset({'version','parent_build_id','parent_source_sha','parent_source_uri','parent_archive_sha256','observed_at','failed_release'})


class ProofError(ValueError):
    """Proof does not satisfy the pinned protocol."""


def _require(ok, message):
    if not ok:
        raise ProofError(message)


def canonical_proof_sha256(proof):
    try:
        raw = json.dumps(proof, sort_keys=True, separators=(',', ':'), ensure_ascii=True, allow_nan=False).encode()
    except (TypeError, ValueError) as error:
        raise ProofError('proof is not canonical JSON data') from error
    return hashlib.sha256(raw).hexdigest()


def make_failed_release_proof(observed_facts, parent_bid, source_sha, source_uri, archive_sha):
    _require(isinstance(observed_facts, dict), 'observed historical facts must be an object')
    _require('observed_at' in observed_facts, 'observation timestamp missing')
    return {'version': 'espacigo.failed-release-proof.v2', 'parent_build_id': parent_bid,
            'parent_source_sha': source_sha, 'parent_source_uri': source_uri,
            'parent_archive_sha256': archive_sha, 'observed_at': observed_facts['observed_at'],
            'failed_release': {k: v for k, v in observed_facts.items() if k != 'observed_at'}}


def validate_request_proof(proof, parent_bid, parent_sha, source_uri, archive_sha, now=None):
    _require(isinstance(proof, dict) and set(proof) == PROOF_KEYS, 'proof fields differ')
    _require(proof.get('version') == 'espacigo.failed-release-proof.v2', 'proof version differs')
    _require(re.fullmatch(UUID, parent_bid or '') is not None and proof['parent_build_id'] == parent_bid, 'parent build identity differs')
    _require(re.fullmatch(r'[a-f0-9]{40}', parent_sha or '') is not None and proof['parent_source_sha'] == parent_sha, 'parent source SHA differs')
    _require(isinstance(source_uri, str) and re.fullmatch(r'gs://' + re.escape(BUCKET) + r'/[^#]+#[1-9][0-9]*', source_uri) is not None and proof['parent_source_uri'] == source_uri, 'parent source URI differs')
    _require(re.fullmatch(HEX, archive_sha or '') is not None and proof['parent_archive_sha256'] == archive_sha, 'parent archive digest differs')
    try:
        observed = datetime.fromisoformat(proof['observed_at'].replace('Z', '+00:00'))
        _require(observed.tzinfo is not None and observed.utcoffset() is not None, 'observation time needs timezone')
        current = now if now is not None else datetime.now(timezone.utc)
        if isinstance(current, (int, float)):
            current = datetime.fromtimestamp(current, timezone.utc)
        _require(current.tzinfo is not None and current.utcoffset() is not None, 'now needs timezone')
        age = (current - observed).total_seconds()
    except (AttributeError, TypeError, ValueError, OverflowError) as error:
        if isinstance(error, ProofError):
            raise
        raise ProofError('invalid observation timestamp') from error
    _require(0 <= age <= 600, 'proof observation future or stale')
    f = proof['failed_release']
    _require(isinstance(f, dict) and set(f) == FACT_KEYS, 'historical fact fields differ')
    bid = PINNED_BUILD_ID
    _require(f['build_id'] == bid and f['source_sha'] == PINNED_SOURCE_SHA, 'pinned historical build/source differs')
    _require(f['lock_uri'] == LOCK and str(f['lock_generation']) == PINNED_GENERATION, 'pinned lock identity differs')
    try:
        lock = json.loads(f['lock_text'])
    except (TypeError, json.JSONDecodeError) as error:
        raise ProofError('lock text is not JSON') from error
    _require(isinstance(lock, dict) and lock == {'build_id': bid, 'source_sha': PINNED_SOURCE_SHA}, 'lock owner differs')
    job = 'espacigo-db-r-' + bid.replace('-', '')
    _require(f['job_name'] == job and re.fullmatch(UUID, f['job_uid'] or '') is not None, 'historical Job identity differs')
    expected_execution = f'projects/{PROJECT}/locations/{REGION}/jobs/{job}/executions/{job}-v62zb'
    _require(f['execution_name'] == expected_execution and re.fullmatch(UUID, f['execution_uid'] or '') is not None and f['failed_execution'] == job + '-v62zb', 'historical execution identity differs')
    _require(f['execution_state'] == 'CONDITION_FAILED', 'historical execution was not failed')
    _require(all(type(f[k]) is int for k in ('failed_count','running_count','succeeded_count','list_count')), 'historical counts must be strict integers')
    _require((f['failed_count'], f['running_count'], f['succeeded_count'], f['list_count']) == (1,0,0,1) and f['next_page'] == '', 'historical execution counts/list differ')
    _require(f['intent_uri'] == f'gs://{BUCKET}/releases/{bid}/job-intent-apply.json', 'historical apply intent URI differs')
    _require(re.fullmatch(r'[1-9][0-9]*', str(f['intent_generation'])) is not None and re.fullmatch(HEX, f['intent_sha256'] or '') is not None, 'historical intent generation/digest invalid')
    return proof
