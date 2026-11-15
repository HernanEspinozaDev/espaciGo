"""Offline contract tests for V84 job recovery validation."""
import hashlib
import copy
import json
import unittest
from pathlib import Path
from unittest.mock import Mock, patch

import v84_recovery_jobs as jobs


class RecoveryJobTests(unittest.TestCase):
    def test_inventory_fingerprint_uses_checksum_then_filename(self):
        rows = [{'version': 1, 'name': 'V1__one.sql', 'checksum': 'abc'}]
        with patch.object(jobs.inspection, 'local_migrations', return_value=rows):
            got_rows, digest = jobs._inventory(Path('/ignored'))
        self.assertEqual(got_rows, rows)
        self.assertEqual(digest, hashlib.sha256(b'abc  V1__one.sql\n').hexdigest())

    def test_verify_rejects_unrecognized_container_fields(self):
        desired = self.desired()
        task = jobs._expected_task(desired)
        jobs._verify_task(task, desired)
        task['containers'][0]['mystery'] = 'x'
        with self.assertRaisesRegex(ValueError, 'complete effective task'):
            jobs._verify_task(task, desired)

    def desired(self):
        return jobs.operator.expected_job_spec('admin-inspect',
                jobs.operator.REGISTRY + '/espacigo-db-migrator@sha256:' + 'a' * 64,
                'b' * 40, '7d4cdc80-1b21-4d2b-8dbc-401a59829dc6',
                '11111111-2222-4333-8444-555555555555')

    def test_task_rejects_latest_secret_retries_and_identity_change(self):
        desired = self.desired()
        for change in ('latest', 'retry', 'identity'):
            task = jobs._expected_task(desired)
            if change == 'latest':
                task['volumes'][0]['secret']['items'][0]['key'] = 'latest'
            elif change == 'retry':
                task['maxRetries'] = 1
            else:
                task['serviceAccountName'] = jobs.operator.MIGRATOR_SA
            with self.subTest(change=change), self.assertRaises(ValueError):
                jobs._verify_task(task, desired)

    def test_only_reviewed_empty_environment_value_may_be_omitted(self):
        desired = self.desired()
        task = jobs._expected_task(desired)
        empty = next(row for row in task['containers'][0]['env'] if row['name'] == 'DATABASE_SET_ROLE')
        del empty['value']
        jobs._verify_task(task, desired)
        nonempty = next(row for row in task['containers'][0]['env'] if row['name'] == 'CLOUD_SQL_INSTANCE_CONNECTION_NAME')
        del nonempty['value']
        with self.assertRaises(ValueError):
            jobs._verify_task(task, desired)

    def test_execution_uses_task_spec_not_job_wrapper(self):
        desired = self.desired()
        valid = {'taskCount': 1, 'parallelism': 1, 'template': {'spec': jobs._expected_task(desired)}}
        jobs._verify_execution_spec(valid, desired)
        with self.assertRaises(ValueError):
            jobs._verify_execution_spec({'template': {'metadata': {}, 'spec': valid}}, desired)

    def test_job_level_labels_precede_named_container(self):
        argv = jobs._create_argv(self.desired())
        label_index = next(i for i, value in enumerate(argv) if value.startswith('--labels='))
        self.assertLess(label_index, argv.index('--container=v84'))

    def test_authority_report_requires_exact_identity_and_closed_finalize(self):
        valid = {'format': 'espacigo.v84-authority.v1', 'mode': 'finalize',
                 'database': jobs.operator.DB, 'session_user': 'postgres', 'current_user': 'postgres',
                 'applied_count': 84, 'baseline_sha256': 'a' * 64, 'authority_closed': True}
        self.assertEqual(jobs._validate_report(valid, 'admin-finalize', 'b' * 40, 'c' * 64), valid)
        for field, value in (('session_user', 'espacigo_migrator_login'), ('authority_closed', False),
                             ('applied_count', True), ('unknown', 'extra')):
            report = dict(valid, **{field: value})
            with self.subTest(field=field), self.assertRaises(ValueError):
                jobs._validate_report(report, 'admin-finalize', 'b' * 40, 'c' * 64)

    def test_exact_missing_job_message_is_required(self):
        cloud = Mock()
        cloud.json.side_effect = ValueError('permission denied')
        with self.assertRaises(ValueError): jobs._assert_job_absent(cloud, 'job')
        cloud.json.side_effect = ValueError('gcloud run jobs failed: ERROR: (gcloud.run.jobs.describe) Cannot find job [job].')
        jobs._assert_job_absent(cloud, 'job')

    def test_initial_admin_inspection_requires_closed_v83(self):
        report = {'format': 'espacigo.v84-authority.v1', 'mode': 'inspect', 'database': jobs.operator.DB,
                  'session_user': 'postgres', 'current_user': 'postgres', 'applied_count': 83,
                  'baseline_sha256': 'a' * 64, 'authority_closed': True}
        jobs._validate_report(report, 'admin-inspect', 'b' * 40, 'c' * 64)
        for field, value in (('applied_count', 84), ('authority_closed', False)):
            with self.subTest(field=field), self.assertRaisesRegex(ValueError, 'initial inspection boundary'):
                jobs._validate_report(dict(report, **{field: value}), 'admin-inspect', 'b' * 40, 'c' * 64)

    def test_report_release_contract_requires_exact_shape(self):
        report = {'format':'espacigo.migration-execution.v2','contract':{'source_sha':'a'*40,'inventory_sha256':'b'*64,'database':'espacigo_dms_stage_20261020','expected_applied':83},'applied':[84]}
        with self.assertRaises(ValueError): jobs._validate_report(report, 'migrate', 'a'*40, 'b'*64)


if __name__ == '__main__':
    unittest.main()
