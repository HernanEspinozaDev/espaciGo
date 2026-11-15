#!/usr/bin/env python3
"""Validate real PostgreSQL -> Go PlanResult JSON with the release validator."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys

from migration_release_manifest import FORMAT, IMAGES, REGISTRY, TARGET, check_plan

ROOT = Path(__file__).resolve().parents[2]


def source_inventory():
    env = os.environ.copy()
    for name in ('DATABASE_URL', 'DATABASE_SET_ROLE', 'PGPASSWORD', 'PGPASSFILE', 'PGOPTIONS', 'PGSERVICE', 'PGSERVICEFILE', 'TEST_DATABASE_URL'):
        env.pop(name, None)
    completed = subprocess.run(
        ['go', 'run', './cmd/dbmigrate', '-inventory-only', '-dir', 'db/migrations'],
        cwd=ROOT, env=env, check=True, capture_output=True, text=True, timeout=120,
    )
    return json.loads(completed.stdout)


def main(directory):
    inventory = source_inventory()
    for version in (95, 96):
        path = directory / f'identity-tail-v{version}-plan.json'
        plan = json.loads(path.read_text())
        # The disposable cluster is intentionally not the Cloud SQL release
        # target; adapt only this offline fixture's database identity.
        TARGET['database'] = plan['database']
        rows = inventory['migrations'][:version]
        sliced_inventory = {
            'format': inventory['format'],
            'migrations': rows,
            'inventory_sha256': hashlib.sha256(
                ''.join(f"{row['checksum']}  {row['name']}\n" for row in rows).encode()
            ).hexdigest(),
        }
        release = {
            'format': FORMAT, 'source_sha': 'a' * 40, 'target': TARGET.copy(),
            'images': {name: REGISTRY + '/' + repository + '@sha256:' + 'b' * 64 for name, repository in IMAGES.items()},
            'migration_inventory': sliced_inventory,
        }
        try:
            result = check_plan(release, plan)
        except ValueError as error:
            expected = rows
            actual = plan.get('applied', []) + plan.get('pending', [])
            first = next((index for index, pair in enumerate(zip(expected, actual)) if pair[0] != pair[1]), None)
            print(f'V{version} wire diagnostic: expected={len(expected)} actual={len(actual)} first_difference={first}')
            if first is not None:
                print(f'V{version} expected_row={expected[first]!r} actual_row={actual[first]!r}')
            raise ValueError(f'V{version} Go plan rejected: {error}') from error
        if result['pending'] != 1 or plan['pending'][0]['version'] != version:
            raise ValueError(f'captured V{version} plan has the wrong pending suffix')
        print(f'PASS: PostgreSQL/Go JSON V{version} authority accepted by migration_release_manifest.check_plan')


if __name__ == '__main__':
    if len(sys.argv) != 2:
        raise SystemExit('usage: test_identity_tail_plan_wire.py REPORT_DIR')
    main(Path(sys.argv[1]))
