"""Offline coverage; no credentials, SQL, browser or Cloud calls."""
import argparse
import json
import os
from pathlib import Path
import tempfile
import unittest
from unittest.mock import Mock, patch

import v84_recovery_operator as operator


class V84RecoveryOperatorTests(unittest.TestCase):
    def args(self, command='lease-grant', *extra):
        return [command, '--account', 'synthetic@example.invalid', '--run-id',
                '7d4cdc80-1b21-4d2b-8dbc-401a59829dc6', '--sha40', 'a' * 40,
                '--image', operator.REGISTRY + '/espacigo-db-migrator@sha256:' + 'b' * 64,
                '--image-build-id', '11111111-2222-4333-8444-555555555555', *extra]

    def test_finalize_requires_exact_baseline(self):
        with self.assertRaises(ValueError):
            operator.parse_args(self.args('job-run', '--mode', 'admin-finalize'))
        with self.assertRaises(ValueError):
            operator.parse_args(self.args('job-run', '--mode', 'admin-finalize', '--baseline', 'latest'))
        valid = operator.parse_args(self.args('job-run', '--mode', 'admin-finalize', '--baseline', 'c' * 64))
        self.assertEqual(valid.baseline, 'c' * 64)

    def test_apply_and_prepare_require_fresh_backup_inputs(self):
        for mode in ('admin-prepare', 'migrate'):
            with self.subTest(mode=mode), self.assertRaises(ValueError):
                operator.parse_args(self.args('job-run', '--mode', mode))

    def test_rejects_unpinned_image_and_identity(self):
        for flag, bad in (('--image', operator.REGISTRY + '/espacigo-db-migrator:latest'),
                          ('--sha40', 'main'), ('--run-id', '../../other'),
                          ('--image-build-id', 'latest')):
            argv = self.args()
            argv[argv.index(flag) + 1] = bad
            with self.subTest(flag=flag), self.assertRaises(ValueError):
                operator.parse_args(argv)

    def test_revoke_cannot_accept_job_options(self):
        with self.assertRaises(ValueError):
            operator.parse_args(self.args('lease-revoke', '--mode', 'migrate'))

    def test_receipts_are_private_create_only_and_identity_bound(self):
        with tempfile.TemporaryDirectory() as temporary:
            args = operator.parse_args(self.args())
            directory = operator.receipt_dir(args.run_id, Path(temporary))
            operator.bind_run(directory, args)
            path = directory / 'run-identity.json'
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            original = path.read_bytes()
            operator.bind_run(directory, args)
            args.sha40 = 'c' * 40
            with self.assertRaises(ValueError):
                operator.bind_run(directory, args)
            self.assertEqual(path.read_bytes(), original)
            with self.assertRaises(FileExistsError):
                operator.write_receipt(directory, 'run-identity', {})

    def test_symlink_receipt_rejected(self):
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            target = directory / 'target.json'
            target.write_text('{}')
            os.chmod(target, 0o600)
            link = directory / 'link.json'
            link.symlink_to(target)
            with self.assertRaises(ValueError):
                operator.read_receipt(link)

    def test_transport_rejects_unrelated_resources_before_cli(self):
        cloud = operator.Cloud(account='synthetic@example.invalid')
        forbidden = [(), ('secrets', 'versions', 'access', '1'),
                     ('secrets', 'set-iam-policy', 'another-secret', 'policy.json'),
                     ('projects', 'add-iam-policy-binding', operator.PROJECT),
                     ('storage', 'rm', operator.inspection.LOCK, '--if-generation-match=0'),
                     ('run', 'jobs', 'execute', 'another-job')]
        with patch.object(operator.subprocess, 'run') as cli:
            for command in forbidden:
                with self.subTest(command=command), self.assertRaises(ValueError):
                    cloud.run(*command)
            cli.assert_not_called()

    def test_named_container_global_flags_precede_container_scope(self):
        cloud = operator.Cloud(account='synthetic@example.invalid')
        response = argparse.Namespace(returncode=0, stdout='{}', stderr='')
        with patch.object(operator.subprocess, 'run', return_value=response) as cli:
            cloud.run('run', 'jobs', 'create', 'espacigo-v84-synthetic', '--container=dbmigrate')
            argv = cli.call_args.args[0]
            self.assertLess(argv.index('--project=' + operator.PROJECT), argv.index('--container=dbmigrate'))
            self.assertLess(argv.index('--account=synthetic@example.invalid'), argv.index('--container=dbmigrate'))

    def test_unlock_rejects_other_migration_result_before_cloud(self):
        args = operator.parse_args(self.args('release-lock'))
        with tempfile.TemporaryDirectory() as temporary:
            directory = Path(temporary)
            for mode, report in (
                    ('migrate', {'source_sha': args.sha40, 'applied': [84, 85],
                                 'applied_before': 83, 'pending_before': [84], 'pending_after': []}),
                    ('admin-prepare', {'baseline_sha256': 'c' * 64}),
                    ('admin-finalize', {'authority_closed': True, 'applied_count': 84,
                                        'baseline_sha256': 'c' * 64}),
                    ('inspect', {}), ('catalog', {})):
                operator.write_receipt(directory, 'phase-' + mode, {'mode': mode, 'report': report})
            cloud = Mock()
            with self.assertRaisesRegex(ValueError, 'exact single-version'):
                operator.release_lock(cloud, directory, args)
            self.assertEqual(cloud.mock_calls, [])

    def test_failed_source_sql_must_match_new_recovery_inventory(self):
        inventory = [{'name': 'V000084__synthetic.sql', 'checksum': 'b' * 64}]
        response = argparse.Namespace(returncode=0, stdout=b'different synthetic SQL')
        with patch.object(operator.subprocess, 'run', return_value=response) as git:
            with self.assertRaisesRegex(ValueError, 'SQL differs from the failed source'):
                operator.verify_failed_inventory(inventory)
            self.assertIn(operator.LOCK_SOURCE + ':db/migrations/V000084__synthetic.sql', git.call_args.args[0])


if __name__ == '__main__':
    unittest.main()
