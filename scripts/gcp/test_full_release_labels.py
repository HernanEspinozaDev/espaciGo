#!/usr/bin/env python3
"""Contract and executable guard tests verifying Cloud Build full release deploy labels."""
import json
import os
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

try:
    import yaml
except ImportError:
    import sys
    sys.exit('PyYAML required')

ROOT = Path(__file__).resolve().parents[2]
FULL_DEPLOY_STEPS = (
    ('deploy-api-by-digest', '${_API_SERVICE}'),
    ('deploy-public-web-by-digest', '${_PUBLIC_SERVICE}'),
    ('deploy-private-web-by-digest', '${_PRIVATE_SERVICE}'),
)


class FullReleaseLabelsContractTest(unittest.TestCase):
    def setUp(self):
        cb_path = ROOT / 'cloudbuild.yaml'
        self.assertTrue(cb_path.is_file(), 'cloudbuild.yaml not found')
        self.config = yaml.safe_load(cb_path.read_text(encoding='utf-8'))
        self.steps_by_id = {s.get('id'): s for s in self.config.get('steps', []) if 'id' in s}

    def test_full_deploy_steps_include_traceability_labels(self):
        expected_label_flag = '--labels="source-commit=$COMMIT_SHA,cloud-build-id=$BUILD_ID"'

        for step_id, _ in FULL_DEPLOY_STEPS:
            with self.subTest(step=step_id):
                self.assertIn(step_id, self.steps_by_id, f'step {step_id} missing in cloudbuild.yaml')
                step = self.steps_by_id[step_id]
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

    def test_full_deploy_steps_deploy_no_traffic_with_build_revision_suffix(self):
        for step_id, _ in FULL_DEPLOY_STEPS:
            with self.subTest(step=step_id):
                self.assertIn(step_id, self.steps_by_id, f'step {step_id} missing in cloudbuild.yaml')
                step = self.steps_by_id[step_id]
                script = '\n'.join(step.get('args', []))
                self.assertIn(
                    '--no-traffic',
                    script,
                    f'step {step_id} missing --no-traffic flag',
                )
                self.assertIn(
                    '--revision-suffix="b$BUILD_ID"',
                    script,
                    f'step {step_id} missing deterministic revision-suffix flag',
                )

    def test_full_deploy_steps_verify_exact_revision_before_traffic_promotion(self):
        for step_id, service_var in FULL_DEPLOY_STEPS:
            with self.subTest(step=step_id):
                self.assertIn(step_id, self.steps_by_id, f'step {step_id} missing in cloudbuild.yaml')
                step = self.steps_by_id[step_id]
                script = '\n'.join(step.get('args', []))
                expected_revision_var = f'revision="{service_var}-b$BUILD_ID"'
                self.assertIn(
                    expected_revision_var,
                    script,
                    f'step {step_id} missing revision definition {expected_revision_var}',
                )
                self.assertIn(
                    'gcloud run revisions describe "$$revision"',
                    script,
                    f'step {step_id} missing exact revision describe verification',
                )
                self.assertIn(
                    '--format=json',
                    script,
                    f'step {step_id} missing --format=json verification',
                )
                self.assertIn(
                    'gcloud info --format=\'value(basic.python_location)\'',
                    script,
                    f'step {step_id} must resolve the SDK interpreter',
                )
                self.assertIn('"$$revision_python" -I -c', script)
                self.assertIn(
                    'gcloud run services update-traffic',
                    script,
                    f'step {step_id} missing explicit services update-traffic call',
                )
                self.assertIn(
                    '--to-revisions="$${revision}=100"',
                    script,
                    f'step {step_id} missing exact revision traffic promotion',
                )
                self.assertNotIn(
                    '--to-latest',
                    script,
                    f'step {step_id} must not use --to-latest',
                )


class FullReleaseGuardExecutableTest(unittest.TestCase):
    def setUp(self):
        cb_path = ROOT / 'cloudbuild.yaml'
        self.assertTrue(cb_path.is_file(), 'cloudbuild.yaml not found')
        self.config = yaml.safe_load(cb_path.read_text(encoding='utf-8'))
        self.steps_by_id = {s.get('id'): s for s in self.config.get('steps', []) if 'id' in s}

        scratch_base = os.environ.get('TMPDIR') or tempfile.gettempdir()
        self.temp_dir = Path(tempfile.mkdtemp(prefix='test_guard_', dir=scratch_base))
        self.workspace_dir = self.temp_dir / 'workspace'
        self.workspace_dir.mkdir(parents=True, exist_ok=True)
        self.fake_bin = self.temp_dir / 'bin'
        self.fake_bin.mkdir(parents=True, exist_ok=True)
        self.mock_json_file = self.temp_dir / 'mock_revision.json'
        self.traffic_log = self.temp_dir / 'traffic_promotions.log'

        fake_gcloud_path = self.fake_bin / 'gcloud'
        fake_gcloud_code = '#!' + sys.executable + '\n' + r'''
import sys, os

mode = os.environ.get("FAKE_GCLOUD_MODE", "normal")
traffic_log = os.environ.get("FAKE_GCLOUD_TRAFFIC_LOG")
mock_json_file = os.environ.get("FAKE_GCLOUD_JSON_FILE")

args = sys.argv[1:]

if args and args[0] == 'info':
    print(sys.executable)
    sys.exit(0)

if "revisions" in args and "describe" in args:
    if mode == "CLIexiterror":
        sys.stderr.write("ERROR: (gcloud.run.revisions.describe) Simulated CLI failure\n")
        sys.exit(1)

    format_arg = None
    for a in args:
        if a.startswith("--format="):
            format_arg = a.split("=", 1)[1].strip("'\"")

    if format_arg != "json":
        sys.stderr.write("ERROR: (gcloud.run.revisions.describe) Transform function expected\n")
        sys.exit(1)

    if mode == "JSONilegible":
        sys.stdout.write("{not valid json: broken")
        sys.exit(0)

    if mock_json_file and os.path.exists(mock_json_file):
        with open(mock_json_file, "r", encoding="utf-8") as f:
            sys.stdout.write(f.read())
        sys.exit(0)
    else:
        sys.stderr.write("Missing mock revision json file\n")
        sys.exit(1)

if "services" in args and "update-traffic" in args:
    if traffic_log:
        with open(traffic_log, "a", encoding="utf-8") as f:
            f.write(" ".join(args) + "\n")
    sys.exit(0)

sys.exit(0)
'''
        fake_gcloud_path.write_text(fake_gcloud_code, encoding='utf-8')
        fake_gcloud_path.chmod(fake_gcloud_path.stat().st_mode | stat.S_IEXEC | stat.S_IXGRP | stat.S_IXOTH)
        bash_path = shutil.which('bash')
        self.assertIsNotNone(bash_path)
        assert bash_path is not None
        (self.fake_bin / 'bash').symlink_to(bash_path)

    def tearDown(self):
        shutil.rmtree(self.temp_dir, ignore_errors=True)

    def _run_guard_block(self, step_id, service_var, mode='normal', json_data=None, isolated_path=False):
        step = self.steps_by_id[step_id]
        script = '\n'.join(step.get('args', []))

        lines = script.splitlines()
        start_idx = None
        end_idx = None
        for i, line in enumerate(lines):
            if 'revision="' in line and '-b$BUILD_ID"' in line:
                start_idx = i
            if start_idx is not None and 'gcloud run services update-traffic' in line:
                end_idx = i
                break
        if start_idx is None or end_idx is None:
            self.fail(f'Could not find guard block in step {step_id}')

        raw_block = '\n'.join(lines[start_idx:end_idx + 1])

        commit_sha = 'a' * 40
        build_id = 'build123'
        resolved_service = service_var.replace('${_API_SERVICE}', 'espacigo-test-api')\
                                      .replace('${_PUBLIC_SERVICE}', 'espacigo-test-public')\
                                      .replace('${_PRIVATE_SERVICE}', 'espacigo-test-private')
        expected_image = f'southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/{resolved_service}@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'

        block = raw_block
        block = block.replace('${_PROJECT}', 'espacigo-testprod')
        block = block.replace('${_REGION}', 'southamerica-west1')
        block = block.replace('${_API_SERVICE}', 'espacigo-test-api')
        block = block.replace('${_PUBLIC_SERVICE}', 'espacigo-test-public')
        block = block.replace('${_PRIVATE_SERVICE}', 'espacigo-test-private')
        block = block.replace('$BUILD_ID', build_id)
        block = block.replace('$COMMIT_SHA', commit_sha)
        block = block.replace('$$', '$')
        block = block.replace('/workspace', str(self.workspace_dir))

        full_script = f"set -eu\nimage=\"{expected_image}\"\n{block}\n"

        if json_data is not None:
            with open(self.mock_json_file, 'w', encoding='utf-8') as f:
                json.dump(json_data, f)

        if self.traffic_log.exists():
            self.traffic_log.unlink()

        env = os.environ.copy()
        env['PATH'] = f"{self.fake_bin}:{env.get('PATH', '')}"
        if isolated_path:
            env['PATH'] = str(self.fake_bin)
        env['FAKE_GCLOUD_MODE'] = mode
        env['FAKE_GCLOUD_JSON_FILE'] = str(self.mock_json_file)
        env['FAKE_GCLOUD_TRAFFIC_LOG'] = str(self.traffic_log)

        return subprocess.run(
            ['bash', '-ceu', full_script],
            env=env,
            capture_output=True,
            text=True,
            cwd=str(self.workspace_dir),
        )

    def _make_valid_json(self, step_id, service_var):
        commit_sha = 'a' * 40
        build_id = 'build123'
        resolved_service = service_var.replace('${_API_SERVICE}', 'espacigo-test-api')\
                                      .replace('${_PUBLIC_SERVICE}', 'espacigo-test-public')\
                                      .replace('${_PRIVATE_SERVICE}', 'espacigo-test-private')
        image = f'southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/{resolved_service}@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
        revision = f'{resolved_service}-b{build_id}'
        return {
            'apiVersion': 'serving.knative.dev/v1',
            'kind': 'Revision',
            'metadata': {
                'name': revision,
                'labels': {
                    'source-commit': commit_sha,
                    'cloud-build-id': build_id,
                },
            },
            'status': {
                'imageDigest': image,
                'conditions': [
                    {'type': 'Ready', 'status': 'True'},
                    {'type': 'ContainerReady', 'status': 'True'},
                ],
            },
        }

    def test_happy_path_promotes_exactly_100_traffic(self):
        for step_id, service_var in FULL_DEPLOY_STEPS:
            with self.subTest(step=step_id):
                data = self._make_valid_json(step_id, service_var)
                res = self._run_guard_block(step_id, service_var, mode='normal', json_data=data)
                self.assertEqual(res.returncode, 0, f'Guard block failed unexpectedly: {res.stderr}')
                self.assertTrue(self.traffic_log.exists(), f'Traffic log not created for step {step_id}')
                log_content = self.traffic_log.read_text(encoding='utf-8')
                resolved_service = service_var.replace('${_API_SERVICE}', 'espacigo-test-api')\
                                              .replace('${_PUBLIC_SERVICE}', 'espacigo-test-public')\
                                              .replace('${_PRIVATE_SERVICE}', 'espacigo-test-private')
                expected_flag = f'--to-revisions={resolved_service}-bbuild123=100'
                self.assertIn(expected_flag, log_content, f'Step {step_id} did not promote exactly 100 traffic')

    def test_guard_uses_sdk_python_when_python3_is_not_on_path(self):
        for step_id, service_var in FULL_DEPLOY_STEPS:
            with self.subTest(step=step_id):
                data = self._make_valid_json(step_id, service_var)
                result = self._run_guard_block(step_id, service_var, json_data=data, isolated_path=True)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertTrue(self.traffic_log.exists())

    def test_drift_and_failures_block_traffic_promotion(self):
        step_id, service_var = FULL_DEPLOY_STEPS[0]
        test_matrix = [
            ('mismatchdigest', lambda d: d['status'].update({'imageDigest': 'wrong-image@sha256:999'}), 'normal'),
            ('ReadyFalse', lambda d: d['status'].update({'conditions': [{'type': 'Ready', 'status': 'False'}]}), 'normal'),
            ('ReadyUnknown', lambda d: d['status'].update({'conditions': [{'type': 'Ready', 'status': 'Unknown'}]}), 'normal'),
            ('missingReady', lambda d: d['status'].update({'conditions': [{'type': 'ContainerReady', 'status': 'True'}]}), 'normal'),
            ('mismatchSHA', lambda d: d['metadata']['labels'].update({'source-commit': 'wrong-sha'}), 'normal'),
            ('mismatchBuild', lambda d: d['metadata']['labels'].update({'cloud-build-id': 'wrong-build'}), 'normal'),
            ('JSONilegible', lambda d: None, 'JSONilegible'),
            ('CLIexiterror', lambda d: None, 'CLIexiterror'),
        ]

        for case_name, mutate_fn, mode in test_matrix:
            with self.subTest(case=case_name):
                data = self._make_valid_json(step_id, service_var)
                mutate_fn(data)
                res = self._run_guard_block(step_id, service_var, mode=mode, json_data=data)
                self.assertNotEqual(res.returncode, 0, f'Case {case_name} should have failed, but returned 0')
                if self.traffic_log.exists():
                    self.assertEqual(self.traffic_log.read_text().strip(), '', f'Case {case_name} promoted traffic despite failure')


if __name__ == '__main__':
    unittest.main()
