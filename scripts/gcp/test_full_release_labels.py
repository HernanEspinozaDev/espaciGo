#!/usr/bin/env python3
"""Contract tests verifying Cloud Build full release deploy labels."""
import unittest
from pathlib import Path
try:
    import yaml
except ImportError:
    import sys
    sys.exit('PyYAML required')

ROOT = Path(__file__).resolve().parents[2]
FULL_DEPLOY_STEPS = (
    'deploy-api-by-digest',
    'deploy-public-web-by-digest',
    'deploy-private-web-by-digest',
)

class FullReleaseLabelsContractTest(unittest.TestCase):
    def test_full_deploy_steps_include_traceability_labels(self):
        cb_path = ROOT / 'cloudbuild.yaml'
        self.assertTrue(cb_path.is_file(), 'cloudbuild.yaml not found')
        config = yaml.safe_load(cb_path.read_text(encoding='utf-8'))
        steps_by_id = {s.get('id'): s for s in config.get('steps', []) if 'id' in s}

        expected_label_flag = '--labels="source-commit=$COMMIT_SHA,cloud-build-id=$BUILD_ID"'

        for step_id in FULL_DEPLOY_STEPS:
            with self.subTest(step=step_id):
                self.assertIn(step_id, steps_by_id, f'step {step_id} missing in cloudbuild.yaml')
                step = steps_by_id[step_id]
                script = '\n'.join(step.get('args', []))
                self.assertIn(
                    'gcloud run deploy',
                    script,
                    f'step {step_id} does not execute gcloud run deploy',
                )
                self.assertIn(
                    expected_label_flag,
                    script,
                    f'step {step_id} missing {expected_label_flag} traceability flag',
                )

if __name__ == '__main__':
    unittest.main()
