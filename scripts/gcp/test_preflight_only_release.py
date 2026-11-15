"""Offline orchestration tests; FakeCloud never contacts Google Cloud."""
import json
import unittest
from unittest.mock import DEFAULT, patch

import test_main_release as base
from provision_release_job import execute as provision_execute
from release_jobs import request
from rollout_web import LOCK, WATERMARK
from verify_web_backend import GuardError

release = base.release


class PreflightOnlyTests(unittest.TestCase):
    def setUp(self):
        fixture = base.MainReleaseTests()
        fixture.setUp()
        self.addCleanup(fixture.doCleanups)
        self.cloud, self.root, self.events = fixture.cloud, fixture.root, fixture.events
        self.patches = fixture.patches

    def enable(self):
        self.cloud.parent['substitutions']['_DB_PREFLIGHT_ONLY'] = 'true'

    def test_preflight_parent_cannot_apply_when_cli_mode_is_missing(self):
        self.enable()
        with self.patches(), self.assertRaises(GuardError):
            release.execute(self.cloud, self.root, base.SHA, base.BUILD)
        self.assertFalse(any(call[:2] == ('storage', 'cp') for call in self.cloud.calls))
        self.assertNotIn('job-apply', self.events)

    def test_preflight_stops_before_policy_backup_apply_rollout_and_retention(self):
        self.enable()
        old_watermark = self.cloud.objects[WATERMARK]
        with self.patches(), patch.multiple(release,
                migration_policy=DEFAULT, rollout=DEFAULT,
                retain_backups=DEFAULT) as forbidden:
            report = release.execute(self.cloud, self.root, base.SHA, base.BUILD, preflight_only=True)
        self.assertEqual(report['mode'], 'preflight-only')
        for field in ('sql_applied', 'backup_created', 'rollout'):
            self.assertIs(report[field], False)
        for mocked in forbidden.values():
            mocked.assert_not_called()
        self.assertIn('job-plan', self.events)
        for event in ('backup', 'provision-apply', 'job-apply'):
            self.assertNotIn(event, self.events)
        self.assertNotIn(LOCK, self.cloud.objects)
        self.assertEqual(self.cloud.objects[WATERMARK], old_watermark)
        self.assertTrue(any('/preflight-only-completed.json' in uri for uri in self.cloud.objects))

    def test_explicit_preflight_does_not_shortcut_as_docs(self):
        self.enable()
        inventory = release.source_inventory(self.root)
        self.cloud.objects[WATERMARK] = ('1', json.dumps({'source_inventory': inventory, 'created_epoch': 0}))
        with self.patches():
            report = release.execute(self.cloud, self.root, base.SHA, base.BUILD, preflight_only=True)
        self.assertEqual(report['mode'], 'preflight-only')
        self.assertIn('job-plan', self.events)

    def test_failed_plan_preserves_lock_and_failure_receipt_without_applying(self):
        self.enable()
        old_watermark = self.cloud.objects[WATERMARK]
        with self.patches(), patch.object(release, 'execute_job', side_effect=GuardError('plan rejected')), self.assertRaises(GuardError):
            release.execute(self.cloud, self.root, base.SHA, base.BUILD, preflight_only=True)
        self.assertIn(LOCK, self.cloud.objects)
        self.assertTrue(any('/failure.json' in uri for uri in self.cloud.objects))
        self.assertEqual(self.cloud.objects[WATERMARK], old_watermark)
        self.assertNotIn('backup', self.events)
        self.assertNotIn('provision-apply', self.events)

    def test_cleanup_failure_preserves_lock_and_failure_receipt_without_applying(self):
        self.enable()
        old_watermark = self.cloud.objects[WATERMARK]
        with self.patches(), patch.object(release, 'remove_job', side_effect=GuardError('cleanup readback denied')), self.assertRaisesRegex(GuardError, 'cleanup readback denied'):
            release.execute(self.cloud, self.root, base.SHA, base.BUILD, preflight_only=True)
        self.assertIn(LOCK, self.cloud.objects)
        self.assertTrue(any('/failure.json' in uri for uri in self.cloud.objects))
        self.assertEqual(self.cloud.objects[WATERMARK], old_watermark)
        self.assertNotIn('backup', self.events)
        self.assertNotIn('provision-apply', self.events)
        self.assertNotIn('job-apply', self.events)

    def test_parent_and_cli_modes_must_match_exactly(self):
        for value in (None, 'false', True, 'TRUE'):
            with self.subTest(value=value):
                if value is None:
                    self.cloud.parent['substitutions'].pop('_DB_PREFLIGHT_ONLY', None)
                else:
                    self.cloud.parent['substitutions']['_DB_PREFLIGHT_ONLY'] = value
                with self.patches(), self.assertRaises(GuardError):
                    release.execute(self.cloud, self.root, base.SHA, base.BUILD, preflight_only=True)
        self.enable()
        with self.patches(), self.assertRaises(GuardError):
            release.execute(self.cloud, self.root, base.SHA, base.BUILD, preflight_only='true')  # pyright: ignore[reportArgumentType] -- intentional malformed input

    def test_provisioner_cannot_create_apply_job_for_preflight_parent(self):
        manifest, plan, *_ = base.fixture()
        candidate = request(manifest, base.BUILD, 'apply', plan)
        self.enable()
        with self.assertRaisesRegex(GuardError, 'preflight'):
            provision_execute(self.cloud, candidate)
        self.assertFalse(any(call[:3] == ('run', 'jobs', 'create') for call in self.cloud.calls))
