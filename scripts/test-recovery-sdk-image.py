#!/usr/bin/env python3
"""Exercise recovery bootstraps in the exact SDK image, offline and credential-free."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import uuid

import yaml

ROOT = Path(__file__).resolve().parents[1]
VALUES = {'_RECOVERY_INSPECTION_ONLY': 'true', '_COMMIT_SHA': 'a' * 40,
          '_SOURCE_ARCHIVE_SHA256': 'b' * 64, 'BUILD_ID': '11111111-2222-4333-8444-555555555555',
          '_PARENT_BUILD_ID': '11111111-2222-4333-8444-555555555555',
          '_REQUEST_GENERATION': '17', '_REQUEST_SHA256': 'c' * 64,
          '_REQUEST_URI': 'gs://espacigo-testprod-cloud-build-447373903153/recovery-inspections/11111111-2222-4333-8444-555555555555/job-request.json'}


def main():
    configs = ['cloudbuild-db-recovery-inspect.yaml', 'cloudbuild-db-recovery-provision.yaml']
    with tempfile.TemporaryDirectory(prefix='recovery-sdk-smoke-', dir=os.environ['TMPDIR']) as tmp:
        mount = Path(tmp)
        code = mount / 'scripts' / 'gcp'; code.mkdir(parents=True)
        for source in (ROOT / 'scripts' / 'gcp').glob('*.py'):
            shutil.copy2(source, code / source.name)
        for config in configs:
            step = yaml.safe_load((ROOT / config).read_text())['steps'][0]
            script = step['args'][1]
            for key, value in VALUES.items(): script = script.replace('${' + key + '}', value)
            script = script.replace('$$', '$')
            match = re.search(r'^\s*(?P<runner>python3|"\$[a-z_]+")\s+(?P<program>scripts/gcp/(?:recovery_inspection|provision_recovery_inspection)\.py)', script, re.M)
            assert match, 'SDK bootstrap entrypoint not identified'
            # Execute the actual YAML bootstrap, but replace the cloud-facing
            # program invocation with --help. No account or network is present.
            command = script[:match.start()] + match['runner'] + ' ' + match['program'] + ' --help\n'
            image = step['name']
            info = json.loads(subprocess.check_output(['docker', 'image', 'inspect', image], text=True))[0]
            args = ['docker', 'create', '--name', 'espacigo-recovery-sdk-' + uuid.uuid4().hex[:10],
                    '--network', 'none', '--read-only', '--cap-drop', 'ALL', '--security-opt', 'no-new-privileges',
                    '--user', f'{os.getuid()}:{os.getgid()}', '--tmpfs', '/tmp:rw,nosuid,size=64m',
                    '--mount', f'type=bind,src={mount},dst=/workspace,readonly', '--workdir', '/workspace',
                    '-e', 'HOME=/tmp', '-e', 'CLOUDSDK_CONFIG=/tmp/gcloud', '-e', 'PYTHONDONTWRITEBYTECODE=1',
                    '-e', 'CLOUDSDK_COMPONENT_MANAGER_DISABLE_UPDATE_CHECK=true', '--entrypoint', 'bash']
            for volume in info['Config'].get('Volumes') or {}:
                if volume != '/tmp': args.extend(['--tmpfs', volume + ':rw,nosuid,size=8m'])
            cid = subprocess.check_output(args + [image, '-ceu', command], text=True).strip()
            try:
                observed = json.loads(subprocess.check_output(['docker', 'inspect', cid], text=True))[0]
                assert observed['HostConfig']['NetworkMode'] == 'none'
                assert not observed['HostConfig'].get('PortBindings')
                assert not any(v['Type'] == 'volume' for v in observed.get('Mounts', []))
                execution = subprocess.run(['docker', 'start', '--attach', cid], capture_output=True, text=True, timeout=120)
                print(config + ':\n' + execution.stdout + execution.stderr)
                assert execution.returncode == 0, 'exact SDK image cannot execute recovery bootstrap'
            finally:
                subprocess.run(['docker', 'rm', '-f', cid], check=True, stdout=subprocess.DEVNULL)
                remaining = subprocess.check_output(['docker', 'ps', '-a', '--filter', 'id=' + cid, '--format', '{{.ID}}'], text=True)
                assert not remaining.strip(), 'own SDK container removal not verified'
    print('PASS: both actual SDK bootstraps ran offline without credentials; own containers absent')


if __name__ == '__main__': main()
