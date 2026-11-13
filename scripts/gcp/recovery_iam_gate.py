"""Wait for an operator's exact-object IAM readback before provisioning.

This is an observation barrier, not a grant/retry mechanism. An absent receipt
can be polled; a present invalid receipt terminates the parent without a child
submission. Only the operator may publish the create-only readiness receipt.
"""
from datetime import datetime, timedelta, timezone
import hashlib
import json
import re
import time

from verify_web_backend import PROJECT, BUCKET, UUID, HEX64


class IAMGateError(ValueError):
    pass


def require(ok, message):
    if not ok:
        raise IAMGateError(message)


def ready_uri(parent_bid):
    require(isinstance(parent_bid, str) and re.fullmatch(UUID, parent_bid), 'invalid IAM readiness parent ID')
    return f'gs://{BUCKET}/recovery-inspections/{parent_bid}/iam-ready.json'


def wait_for_iam_ready(cloud, request, receipt, verify_binding, check_lock,
                       timeout=600, interval=5, clock=None, sleep=None, now=None):
    clock = clock or time.monotonic
    sleep = sleep or time.sleep
    now = now or (lambda: datetime.now(timezone.utc))
    require(0 < timeout <= 600 and 0 < interval <= 10, 'invalid IAM barrier bounds')
    uri = ready_uri(request['build_id'])
    require(receipt.get('uri') == f'gs://{BUCKET}/recovery-inspections/{request["build_id"]}/job-request.json'
            and set(receipt) == {'uri', 'generation', 'sha256'}
            and re.fullmatch('[1-9][0-9]*', receipt.get('generation', ''))
            and re.fullmatch(HEX64, receipt.get('sha256', '')), 'invalid immutable request receipt')
    deadline = clock() + timeout
    while True:
        check_lock()
        try:
            metadata = cloud.json('storage', 'objects', 'describe', uri)
        except (ValueError, OSError):
            # Read failures never authorize a grant, submit or execute. There is
            # no retry of a mutation; expiry/error leaves evidence and lock.
            require(clock() < deadline, 'IAM readiness timed out; preserve parent evidence')
            sleep(interval)
            continue
        generation = str(metadata.get('generation', ''))
        require(re.fullmatch('[1-9][0-9]*', generation), 'invalid IAM readiness generation')
        text = cloud.run('storage', 'cat', uri + '#' + generation)
        require(isinstance(text, str) and len(text.encode()) <= 65536, 'IAM readiness receipt oversized')
        try:
            value = json.loads(text)
        except (TypeError, ValueError) as error:
            raise IAMGateError('malformed IAM readiness receipt') from error
        required = {'format', 'parent_build_id', 'source_sha', 'request', 'principal', 'binding',
                    'expires_at', 'policy_readback_etag', 'policy_readback_sha256', 'custom_role_readback_sha256'}
        require(isinstance(value, dict) and set(value) == required, 'IAM readiness fields differ')
        require(value['format'] == 'espacigo.recovery-iam-ready.v1'
                and value['parent_build_id'] == request['build_id']
                and value['source_sha'] == request['source_sha']
                and value['request'] == receipt, 'IAM readiness parent/source/request differs')
        require(value['principal'] == f'serviceAccount:espacigo-release-job-builder@{PROJECT}.iam.gserviceaccount.com',
                'IAM readiness principal differs')
        require(isinstance(value['policy_readback_etag'], str) and value['policy_readback_etag']
                and all(isinstance(value[k], str) and re.fullmatch(HEX64, value[k])
                        for k in ('policy_readback_sha256', 'custom_role_readback_sha256')),
                'IAM readback evidence missing')
        try:
            expiry = datetime.fromisoformat(value['expires_at'].replace('Z', '+00:00'))
        except (AttributeError, TypeError, ValueError) as error:
            raise IAMGateError('invalid IAM readiness expiry') from error
        current = now()
        require(expiry.tzinfo is not None and current < expiry <= current + timedelta(hours=2),
                'IAM readiness expired or exceeds lease bound')
        # Production passes the source-derived exact binding comparison here;
        # the gate does not accept a declaration of broad privileges instead.
        verify_binding(value['binding'], value)
        return {'receipt': {'uri': uri, 'generation': generation,
                            'sha256': hashlib.sha256(text.encode()).hexdigest()},
                'operator_readback': value}
