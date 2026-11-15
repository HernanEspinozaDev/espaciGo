"""Exact GET cleanup verification without project-wide list authority."""
import unittest
from unittest.mock import patch
from types import SimpleNamespace
import recovery_inspection as r

NAME = 'espacigo-db-i-8367075d88d7492b8049c7f006c48120'
ABSENT = f'ERROR: (gcloud.run.jobs.describe) Cannot find job [{NAME}].'


class CleanupGetTests(unittest.TestCase):
    def test_only_exact_sdk_not_found_proves_absence(self):
        with patch.object(r.subprocess, 'run', return_value=SimpleNamespace(returncode=1, stderr=ABSENT, stdout='')) as sdk:
            r.assert_inspection_job_absent(r.Cloud(), NAME)
        command = sdk.call_args.args[0]
        self.assertEqual(command[:5], ['gcloud', 'run', 'jobs', 'describe', NAME])
        self.assertNotIn('list', command)
    def test_denied_timeout_other_target_and_existing_job_do_not_prove_absence(self):
        for error in ('Permission run.jobs.get denied', 'connection timeout', ABSENT.replace(NAME, NAME + '-other')):
            with self.subTest(error=error), patch.object(r.subprocess, 'run', return_value=SimpleNamespace(returncode=1, stderr=error, stdout='')):
                with self.assertRaises(ValueError): r.assert_inspection_job_absent(r.Cloud(), NAME)
        with patch.object(r.subprocess, 'run', return_value=SimpleNamespace(returncode=0, stderr='', stdout='{}')):
            with self.assertRaisesRegex(ValueError, 'still present'):
                r.assert_inspection_job_absent(r.Cloud(), NAME)
    def test_noninspection_target_rejected_before_sdk(self):
        with patch.object(r.subprocess, 'run') as sdk:
            with self.assertRaises(ValueError): r.assert_inspection_job_absent(r.Cloud(), 'espacigo-db-r-' + 'a' * 32)
            sdk.assert_not_called()


if __name__ == '__main__': unittest.main()
