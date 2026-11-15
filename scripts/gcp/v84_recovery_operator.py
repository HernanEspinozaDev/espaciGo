#!/usr/bin/env python3
"""Explicitly authorized V84-only recovery, with immutable phase receipts.

Normal application rollout remains in release_main.py. This exceptional
operator neither accepts arbitrary SQL nor changes a migration's bytes.
"""
from __future__ import annotations
import argparse
from datetime import datetime, timedelta, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import sys
import time
import uuid

import recovery_inspection as inspection
from verify_web_backend import PROJECT, REGION, Cloud as BaseCloud
from migration_job_plan import REGISTRY, SECRET as MIGRATION_SECRET, SERVICE_ACCOUNT as MIGRATOR_SA
from backup_readiness import check as check_backup

ADMIN_SA = f'espacigo-test-migrator@{PROJECT}.iam.gserviceaccount.com'
ADMIN_SECRET = 'espacigo-testprod-cloudsql-postgres-admin-password'
ADMIN_SECRET_VERSION = '1'
DB = 'espacigo_dms_stage_20261020'
INSTANCE = f'{PROJECT}:{REGION}:espacigo-testprod-db'
IMAGE_RE = re.compile(re.escape(REGISTRY) + r'/espacigo-db-migrator@sha256:[a-f0-9]{64}')
SHA_RE = re.compile(r'[a-f0-9]{40}')
HEX64 = re.compile(r'[a-f0-9]{64}')
UUID_RE = re.compile(inspection.UUID)
LOCK_GENERATION = '1794600804441535'

LOCK_OWNER = '2dc002a9-5831-4717-8064-6432b98d3e01'
LOCK_SOURCE = 'bb10309129adad9d7204b9b54bb2259186a34242'
INVENTORY84 = '3921ff747e4a7e7f49de443de7333dad03535f0687078b93b8a3cac6042a209a'
MODES = ('admin-inspect', 'admin-prepare', 'admin-finalize', 'migrate', 'inspect', 'catalog')


def require(ok: bool, message: str) -> None:
    if not ok:
        raise ValueError(message)


def parse_args(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('command', choices=('lease-grant', 'lease-revoke', 'job-run', 'release-lock'))
    p.add_argument('--account', required=True)
    p.add_argument('--run-id', required=True)
    p.add_argument('--sha40', required=True)
    p.add_argument('--image', required=True)
    p.add_argument('--image-build-id', required=True)
    p.add_argument('--mode', choices=MODES)
    p.add_argument('--baseline')
    p.add_argument('--backup-receipt', type=Path)
    p.add_argument('--backup-snapshot', type=Path)
    a = p.parse_args(argv)
    require(UUID_RE.fullmatch(a.run_id) is not None, '--run-id must be a UUID')
    require(SHA_RE.fullmatch(a.sha40) is not None, '--sha40 must be 40 lowercase hex characters')
    require(IMAGE_RE.fullmatch(a.image) is not None, '--image must be the project migrator image pinned by sha256 digest')
    require(UUID_RE.fullmatch(a.image_build_id) is not None, '--image-build-id must be a UUID')
    if a.command == 'job-run':
        require(a.mode in MODES, 'job-run requires --mode')
        require((a.mode == 'admin-finalize') == (a.baseline is not None), '--baseline is required only for admin-finalize')
        if a.baseline is not None:
            require(HEX64.fullmatch(a.baseline) is not None, '--baseline must be a SHA-256 hex digest')
        if a.mode in ('migrate', 'admin-prepare'):
            require(a.backup_receipt is not None and a.backup_snapshot is not None,
                    'prepare and migrate require --backup-receipt and a fresh --backup-snapshot')
    else:
        require(a.mode is None and a.baseline is None and a.backup_receipt is None and a.backup_snapshot is None,
                '--mode/--baseline/backup options are valid only with job-run')
    return a


def receipt_dir(run_id: str, root: Path | None = None) -> Path:
    require(UUID_RE.fullmatch(run_id) is not None, 'invalid receipt run UUID')
    root = (root or Path.cwd()).resolve()
    base = root / '.local' / 'v84-recovery'
    for path in (root / '.local', base, base / run_id):
        require(not path.is_symlink(), 'receipt path cannot contain symlinks')
        path.mkdir(mode=0o700, exist_ok=True)
        require(path.is_dir() and stat.S_IMODE(path.stat().st_mode) == 0o700,
                'receipt directories must have mode 0700')
    return base / run_id


def write_receipt(directory: Path, name: str, value) -> Path:
    require(re.fullmatch(r'[a-z][a-z0-9-]*', name) is not None, 'invalid receipt name')
    path = directory / (name + '.json')
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as f:
        json.dump(value, f, sort_keys=True, separators=(',', ':'))
        f.write('\n'); f.flush(); os.fsync(f.fileno())
    return path


def read_receipt(path: Path):
    require(not path.is_symlink(), 'receipt cannot be a symlink')
    fd = os.open(path, os.O_RDONLY | os.O_NOFOLLOW)
    with os.fdopen(fd, encoding='utf-8') as f:
        s = os.fstat(f.fileno())
        require(stat.S_ISREG(s.st_mode) and stat.S_IMODE(s.st_mode) == 0o600 and
                s.st_uid == os.getuid() and s.st_nlink == 1, 'unsafe receipt file')
        return json.load(f)


def computed_job_name(mode: str, run_id: str) -> str:
    require(mode in MODES and UUID_RE.fullmatch(run_id) is not None, 'invalid job identity')
    name = 'espacigo-v84-' + mode.replace('-', '') + '-' + run_id.replace('-', '')[:16]
    require(len(name) <= 63, 'computed Job name exceeds Cloud Run limit')
    return name


def expected_job_spec(mode, image, sha40, run_id, build_id, baseline=None):
    require(mode in MODES and IMAGE_RE.fullmatch(image) is not None
            and SHA_RE.fullmatch(sha40) is not None and UUID_RE.fullmatch(build_id) is not None,
            'invalid pinned Job inputs')
    admin = mode.startswith('admin-')
    mount = '/var/secrets/v84-admin' if admin else '/var/secrets/migration'
    user = 'postgres' if admin else 'espacigo_migrator_login'
    args = ['-dir', '/app/db/migrations']
    if admin:
        args += ['-v84-authority-mode', mode.removeprefix('admin-'), '-confirm-v84-authority', DB]
        if mode == 'admin-finalize':
            require(isinstance(baseline, str) and HEX64.fullmatch(baseline) is not None,
                    'finalize requires the observed baseline')
            args += ['-v84-authority-baseline', baseline]
        else:
            require(baseline is None, 'baseline is valid only for finalize')
    elif mode == 'migrate':
        args += ['-release', '-release-source-sha', sha40, '-release-inventory-sha256', INVENTORY84,
                 '-release-expected-applied', '83']
    else:
        args += ['-recovery-inspection-only' if mode == 'inspect' else '-schema-evidence-only']
    return {'name': computed_job_name(mode, run_id), 'image': image,
            'service_account': ADMIN_SA if admin else MIGRATOR_SA,
            'secret': ADMIN_SECRET if admin else MIGRATION_SECRET, 'secret_version': '1', 'mount': mount,
            'command': ['/app/dbmigrate'], 'args': args,
            'env': {'DATABASE_CONNECTION_MODE': 'cloudsql', 'CLOUD_SQL_INSTANCE_CONNECTION_NAME': INSTANCE,
                    'CLOUD_SQL_PRIVATE_IP': 'true', 'DATABASE_URL': f'postgres://{user}@localhost/{DB}?sslmode=disable',
                    'DATABASE_SET_ROLE': '' if admin else 'espacigo_schema_owner',
                    'DATABASE_PASSWORD_FILE': mount + '/password', 'DBMIGRATE_TIMEOUT': '8m'},
            'tasks': 1, 'parallelism': 1, 'max_retries': 0, 'timeout_seconds': 600,
            'network': PROJECT, 'subnetwork': 'espacigo-testprod-api-serverless',
            'vpc_egress': 'private-ranges-only', 'source_sha': sha40, 'build_id': build_id}



def validate_backup_receipt(receipt_path: Path, snapshot_path: Path):
    for path in (receipt_path, snapshot_path):
        require(not path.is_symlink() and path.is_file(), 'backup evidence must be regular non-symlink files')
    receipt = json.loads(receipt_path.read_text(encoding='utf-8'))
    snapshot = json.loads(snapshot_path.read_text(encoding='utf-8'))
    # The operation receipt is evidence of the already-submitted ON_DEMAND
    # backup; readiness must be re-evaluated from the separately refreshed
    # native snapshot, never from a stale readiness summary.
    op = receipt.get('operation', receipt)
    require(op.get('name') == 'b8ac3bf5-b6ac-4242-8b70-eabc0000004b' or
            op.get('description') == 'espacigo-v84-recovery-' + snapshot_path.parent.name,
            'backup operation receipt is not the authorized V84 recovery backup')
    result = check_backup(snapshot, True)
    backup = snapshot.get('backup', {})
    require(backup.get('type') == 'ON_DEMAND' and
            backup.get('description') == 'espacigo-v84-recovery-' + snapshot_path.parent.name,
            'fresh backup snapshot does not identify the authorized recovery backup')
    require(result.get('pending_sql') is True, 'backup readiness must explicitly report pending_sql=true')
    return result


def verify_image_build(cloud, digest_image: str, build_id: str, sha40: str):
    """Read-only evidence check; never builds or substitutes a different image."""
    build = cloud.json('builds', 'describe', build_id)
    require(build.get('id') == build_id and build.get('status') == 'SUCCESS' and
            build.get('serviceAccount') == inspection.DEPLOYER and
            build.get('substitutions', {}).get('_COMMIT_SHA') == sha40,
            'image build must be successful, source-pinned and deployed by the authorized builder')
    proof = cloud.json('artifacts', 'docker', 'images', 'describe', digest_image, '--show-provenance')
    digest = digest_image.rsplit('@', 1)[1]
    invocation = f'https://cloudbuild.googleapis.com/v1/projects/{PROJECT}/locations/{REGION}/builds/{build_id}'
    statements = [x.get('build', {}).get('inTotoSlsaProvenanceV1', {})
                  for x in proof.get('provenance_summary', {}).get('provenance', [])]
    require(proof.get('image_summary', {}).get('digest') == digest and any(
        s.get('predicateType') == 'https://slsa.dev/provenance/v1' and
        s.get('predicate', {}).get('runDetails', {}).get('metadata', {}).get('invocationId') == invocation and
        any(x.get('digest', {}).get('sha256') == digest.removeprefix('sha256:') for x in s.get('subject', []))
        for s in statements), 'image digest/SLSA provenance does not match requested build')
    return build


class Cloud(BaseCloud):
    """CLI transport for fixed recovery resources, not an arbitrary shell."""
    def run(self, *args, timeout=180):
        a: tuple[str, ...] = tuple(map(str, args))
        if len(a) < 2:
            raise ValueError('incomplete recovery command')
        allowed = a[:2] == ('builds', 'describe') or a[:4] == ('artifacts', 'docker', 'images', 'describe')
        if a[:2] in (('secrets', 'get-iam-policy'), ('secrets', 'set-iam-policy')):
            allowed = len(a) >= 3 and a[2] == ADMIN_SECRET
        if len(a) >= 3 and a[:2] == ('run', 'jobs'):
            if a[2] == 'executions':
                allowed = len(a) >= 5 and a[3] == 'describe' and a[4].startswith('espacigo-v84-')
            else:
                allowed = len(a) >= 4 and a[2] in ('create', 'describe', 'execute', 'delete') and a[3].startswith('espacigo-v84-')
        if a[:2] == ('logging', 'read'):
            allowed = len(a) >= 3 and 'cloud_run_job' in a[2] and 'espacigo-v84-' in a[2]
        if a[:2] == ('storage', 'rm'):
            allowed = a == ('storage', 'rm', inspection.LOCK, '--if-generation-match=' + LOCK_GENERATION)
        if a[:3] == ('storage', 'objects', 'describe'):
            allowed = len(a) >= 4 and a[3] == inspection.LOCK
        require(allowed, 'command is outside V84 recovery resources')
        flags = ['--project=' + PROJECT, '--quiet']
        if a[0] in ('builds', 'run'):
            flags.append('--region=' + REGION)
        if self.account:
            flags.append('--account=' + self.account)
        command = (['gcloud', *a[:4], *flags, *a[4:]] if a[:3] == ('run', 'jobs', 'create')
                   else ['gcloud', *a, *flags])
        result = subprocess.run(command, text=True, capture_output=True, timeout=timeout)
        require(result.returncode == 0, f'gcloud {a[0]} {a[1]} failed: ' + result.stderr.strip())
        return result.stdout


def read_phase(directory, mode):
    result = read_receipt(directory / ('phase-' + mode + '.json'))
    require(result.get('mode') == mode and isinstance(result.get('report'), dict), 'invalid phase receipt')
    return result['report']


def bind_run(directory, a):
    identity = {'run_id': a.run_id, 'source_sha': a.sha40, 'image': a.image,
                'image_build_id': a.image_build_id, 'database': DB, 'instance': INSTANCE}
    path = directory / 'run-identity.json'
    if path.exists():
        require(read_receipt(path) == identity, 'recovery identity differs from its create-only receipt')
    else:
        write_receipt(directory, 'run-identity', identity)


def check_quality(sha):
    from quality_gate import fetch_json, fetch_runs, evaluate, verify_main_ref
    verify_main_ref(fetch_json('/git/ref/heads/main'), sha)
    return evaluate(fetch_runs(sha), sha, require_database=True)


def verify_source(directory, a, build):
    receipt = read_receipt(directory / 'source-receipt.json')
    require(receipt.get('source_sha') == a.sha40 and receipt.get('image_build_id') == a.image_build_id
            and isinstance(receipt.get('archive_sha256'), str)
            and HEX64.fullmatch(receipt['archive_sha256']) is not None, 'source archive receipt differs')
    expected = {'source_sha': a.sha40, 'image_build_id': a.image_build_id, 'image': a.image,
                'source_uri': inspection.source_uri(build), 'archive_sha256': receipt['archive_sha256']}
    path = directory / 'source-verified.json'
    if path.exists():
        require(read_receipt(path) == expected, 'immutable build source differs from verified source')
    else:
        inspection.verify_archive(inspection.Cloud(account=a.account), expected['source_uri'],
                                  expected['archive_sha256'], Path(__file__).resolve().parents[2])
        write_receipt(directory, 'source-verified', expected)


def verify_failed_inventory(inventory):
    """Recovery has new code, but may not change the failed release's SQL."""
    digest = hashlib.sha256()
    for entry in inventory:
        result = subprocess.run(['git', '-C', str(Path(__file__).resolve().parents[2]),
                                 'show', LOCK_SOURCE + ':db/migrations/' + entry['name']],
                                capture_output=True, timeout=30)
        require(result.returncode == 0, 'cannot establish immutable failed-source migration bytes')
        checksum = hashlib.sha256(result.stdout).hexdigest()
        require(checksum == entry['checksum'], 'recovery SQL differs from the failed source')
        digest.update((checksum + '  ' + entry['name'] + '\n').encode())
    require(digest.hexdigest() == INVENTORY84, 'failed source is not the reviewed V84 inventory')
    return digest.hexdigest()


def release_lock(cloud, directory, a):
    from schema_release_gate import validate_catalog
    from migration_release_manifest import validate_plan_contract
    applied = read_phase(directory, 'migrate')
    prepared = read_phase(directory, 'admin-prepare')
    finalized = read_phase(directory, 'admin-finalize')
    observed = read_phase(directory, 'inspect')
    catalog = read_phase(directory, 'catalog')
    inventory = inspection.local_migrations(Path.cwd())
    require(len(inventory) == 84 and hashlib.sha256(''.join(
            x['checksum'] + '  ' + x['name'] + '\n' for x in inventory).encode()).hexdigest() == INVENTORY84,
            'local migration inventory differs from the reviewed recovery source')
    require(applied.get('format') == 'espacigo.migration-execution.v2' and applied.get('applied') == [84]
            and applied.get('contract') == {'source_sha': a.sha40, 'inventory_sha256': INVENTORY84,
                                           'database': DB, 'expected_applied': 83},
            'only the exact single-version recovery may release the lock')
    for key, prefix in (('before', 83), ('after', 84)):
        plan = applied.get(key)
        validate_plan_contract(plan)
        require(plan.get('database') == DB and plan.get('read_only') is True
                and plan.get('history_exists') is True and plan.get('session_user') == 'espacigo_migrator_login'
                and plan.get('current_user') == 'espacigo_schema_owner'
                and plan.get('applied') == inventory[:prefix] and plan.get('pending') == inventory[prefix:],
                'migration result does not prove its exact ledger boundary')
    baseline = prepared.get('baseline_sha256')
    require(isinstance(baseline, str) and HEX64.fullmatch(baseline) is not None
            and finalized.get('baseline_sha256') == baseline
            and finalized.get('authority_closed') is True and finalized.get('applied_count') == 84,
            'temporary SQL authority has not been independently finalized')
    inspection.validate_report(observed, inventory)
    require(len(observed['applied']) == 84 and observed['pending'] == []
            and observed['authority']['effective_privileges']['schema_public_create'] is False
            and observed['authority']['effective_privileges']['usuario_references_id'] is False,
            'fresh limited-identity postcheck must show V84 with both grants absent')
    validate_catalog(catalog)
    # Access must be withdrawn before the recovery lock can be removed.
    revoked = read_receipt(directory / 'lease-revoked.json')
    require(revoked.get('revoked') is True, 'Secret Manager lease is not verified withdrawn')
    historical = inspection.Cloud(account=a.account)
    proof = inspection.inspect_failed_release(historical)
    require(proof.get('source_sha') == LOCK_SOURCE and proof.get('build_id') == LOCK_OWNER
            and proof.get('lock_generation') == LOCK_GENERATION, 'failed-release proof identity differs')
    failed_inventory = verify_failed_inventory(inventory)
    write_receipt(directory, 'lock-release-intent', {'failed_release': proof,
                  'source_sha': a.sha40, 'run_id': a.run_id, 'generation': LOCK_GENERATION,
                  'failed_source_sha': LOCK_SOURCE, 'shared_inventory_sha256': failed_inventory})
    inspection.assert_lock(historical, LOCK_GENERATION, proof['lock_text'])
    cloud.run('storage', 'rm', inspection.LOCK, '--if-generation-match=' + LOCK_GENERATION)
    try:
        cloud.json('storage', 'objects', 'describe', inspection.LOCK)
    except ValueError as error:
        text = str(error)
        require(inspection.LOCK in text and re.search(r'not found:\s*404\b|\bHTTPError 404\b|\b404 Not Found\b', text) is not None,
                'lock absence is not established by this response')
    else:
        raise ValueError('a live lock remains; never delete a replacement generation')
    receipt = {'released': True, 'generation': LOCK_GENERATION, 'source_sha': a.sha40,
               'verified_at': datetime.now(timezone.utc).isoformat()}
    write_receipt(directory, 'lock-released', receipt)
    return receipt


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__)
    try:
        a = parse_args(argv)
        d = receipt_dir(a.run_id)
        bind_run(d, a)
        cloud = Cloud(account=a.account)
        # Withdrawal remains available if main/quality changes after prepare.
        if a.command == 'lease-revoke':
            from v84_recovery_iam import revoke
            result = revoke(cloud, d, a.run_id)
            require(result.get('verified') is True, 'lease withdrawal is not verified')
            write_receipt(d, 'lease-revoked', dict(result, revoked=True))
        else:
            if not (a.command == 'job-run' and a.mode == 'admin-finalize'):
                check_quality(a.sha40)
            build = verify_image_build(cloud, a.image, a.image_build_id, a.sha40)
            verify_source(d, a, build)
            if a.command == 'lease-grant':
                from v84_recovery_iam import grant
                result = grant(cloud, d, a.run_id)
                require(result.get('verified') is True, 'lease grant is not verified')
                write_receipt(d, 'lease-granted', result)
            elif a.command == 'release-lock':
                result = release_lock(cloud, d, a)
            else:
                from v84_recovery_jobs import run_job
                if a.mode in ('admin-prepare', 'migrate'):
                    validate_backup_receipt(a.backup_receipt, a.backup_snapshot)
                if a.mode == 'admin-prepare':
                    before = read_phase(d, 'admin-inspect')
                    require(before.get('authority_closed') is True and before.get('applied_count') == 83,
                            'prepare requires the successful closed-authority V83 inspection')
                if a.mode == 'migrate':
                    require(read_phase(d, 'admin-prepare').get('applied_count') == 83, 'migration requires prepared V83')
                if a.mode == 'admin-finalize':
                    expected = read_phase(d, 'admin-inspect').get('baseline_sha256')
                    require(a.baseline == expected, 'finalizer must use the original observed baseline')
                write_receipt(d, 'phase-' + a.mode + '-intent', {'source_sha': a.sha40, 'mode': a.mode,
                              'created_at': datetime.now(timezone.utc).isoformat()})
                result = run_job(cloud, d, a.run_id, a.sha40, a.image, a.image_build_id, a.mode, a.baseline)
                write_receipt(d, 'phase-' + a.mode, result)
        print(json.dumps(result, sort_keys=True))
    except (ValueError, OSError, KeyError, TypeError) as e:
        p.exit(2, f'v84 recovery refused: {e}\n')


if __name__ == '__main__':
    main()
