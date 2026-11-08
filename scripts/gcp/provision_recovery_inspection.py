#!/usr/bin/env python3
"""Dedicated recovery Job creator. No execution, override, secret payload or DDL."""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys

from recovery_inspection import (Cloud, BUCKET, PROVISIONER, UUID, HEX, RecoveryError,
                                 require, parent_identity, verify_archive, source_uri,
                                 validate_request, create_args, assert_lock)


def execute(cloud, root, bid, parent_bid, digest, request_uri, generation, request_sha):
    # All externally supplied identifiers are rejected before any cloud access.
    require(re.fullmatch(UUID, bid) and re.fullmatch(UUID, parent_bid) and re.fullmatch(HEX, digest) and re.fullmatch(HEX, request_sha) and re.fullmatch('[1-9][0-9]*', generation), 'invalid provision identity')
    require(request_uri == f'gs://{BUCKET}/recovery-inspections/{parent_bid}/job-request.json', 'request target differs')
    text = cloud.run('storage', 'cat', request_uri + '#' + generation)
    require(hashlib.sha256(text.encode()).hexdigest() == request_sha, 'immutable request SHA256 differs')
    req = json.loads(text); validate_request(req)
    require(req['build_id'] == parent_bid and req['source_archive_sha256'] == digest, 'request parent/source differs')
    parent = cloud.json('builds', 'describe', parent_bid)
    uri = parent_identity(parent, req['source_sha'], parent_bid, digest)
    require(req['source_uri'] == uri, 'request source generation differs from parent')
    me = cloud.json('builds', 'describe', bid)
    require(me.get('id') == bid and me.get('status') == 'WORKING' and me.get('serviceAccount') == PROVISIONER and not me.get('buildTriggerId'), 'child identity differs')
    subs = me.get('substitutions', {})
    expected = {'_PARENT_BUILD_ID': parent_bid, '_SOURCE_ARCHIVE_SHA256': digest,
                '_REQUEST_URI': request_uri, '_REQUEST_GENERATION': generation, '_REQUEST_SHA256': request_sha}
    require(all(subs.get(k) == v for k, v in expected.items()), 'child request substitutions differ')
    verify_archive(cloud, source_uri(me), digest, root)
    # Historical metadata is observed by the existing parent authority. The
    # immutable request binds that observation; this identity reads only the
    # exact request and lock, and can create but never execute the inspector.
    lock = req['failed_release_proof']['failed_release']
    args = create_args(req)
    assert_lock(cloud, lock['lock_generation'], lock['lock_text'])
    cloud.run(*(a for a in args[1:] if not a.startswith(('--project=', '--region='))))
    # A changed release lock never triggers cleanup or execution of this Job.
    assert_lock(cloud, lock['lock_generation'], lock['lock_text'])


def main(argv=None):
    p = argparse.ArgumentParser()
    for field in ['build-id', 'parent-build-id', 'source-archive-sha256', 'request-uri', 'request-generation', 'request-sha256']:
        p.add_argument('--' + field, required=True)
    a = p.parse_args(sys.argv[1:] if argv is None else argv)
    try:
        execute(Cloud(), Path('/workspace'), a.build_id, a.parent_build_id, a.source_archive_sha256, a.request_uri, a.request_generation, a.request_sha256)
    except (ValueError, OSError, KeyError, TypeError) as error:
        print('recovery provision refused: ' + str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    raise SystemExit(main())
