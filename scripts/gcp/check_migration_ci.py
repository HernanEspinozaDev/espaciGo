#!/usr/bin/env python3
"""Reject successful-looking migration runs with missing or skipped DB tests."""
import argparse
import json
from pathlib import Path

REQUIRED = frozenset({
    'TestRunnerSchemaEvidenceReadonlyBlocksWritingSelect',
    'TestRunnerReleaseCatalogFailureDoesNotReportSuccessAfterDDL',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/catalog-read',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/catalog-read',
    'TestRunnerReleasePinsInventoryAndReviewedPrefix',
    'TestRunnerReleaseFailureDoesNotProduceSuccessOrReplayCommittedPrefix',
    'TestRunnerReleaseSerializesCompetingReviewedPrefixes',
    'TestRunnerAppliesFromEmptyAndRepeatIsNoop',
    'TestRunnerRefusesToInitializeLedgerBesideExistingApplicationObjects',
    'TestPlanOnlyDoesNotInitializeLedgerOrExecutePendingMigrations',
    'TestRunnerAppliesDDLUnderRestrictedSetRoleMembership',
    'TestMigratorSerializesIndependentProcesses',
    'TestRunnerStoresExactMigrationAndRunnerChecksums',
    'TestRunnerRejectsChecksumDriftBeforeDDL',
    'TestRunnerRollsBackMigrationAndHistoryTogether',
    'TestRunnerRecoversAfterFailureBetweenMigrationCommits',
    'TestRunnerSerializesConcurrentExecutions',
    'TestRunnerRejectsVersionGapBeforeDDL',
    'TestCurrentSchemaMigrationFromEmptyAndV75',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/api-schema-contract',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/api-schema-contract',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/api-schema-contract/public-gallery-sync',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/api-schema-contract/public-gallery-sync',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/release-noop',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/release-noop',
    'TestRunnerV84AuthorityRecoveryPostgres',
    'TestRunnerV84AuthorityRecoveryPostgres/failure_cleanup',
    'TestRunnerV84AuthorityRecoveryPostgres/apply_and_finalize',
})


def verify(events):
    passed = set()
    package_passed = False
    package = 'github.com/HernanEspinozaDev/espaciGo/internal/migrator'
    for event in events:
        if event.get('Package') != package:
            continue
        action, test = event.get('Action'), event.get('Test')
        if action in {'skip', 'fail'}:
            raise ValueError('required database suite contains a skip or failure')
        if action == 'pass':
            if test:
                passed.add(test)
            else:
                package_passed = True
    if not package_passed or REQUIRED - passed:
        raise ValueError('required database tests were not all executed successfully')
    return len(REQUIRED)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('events', type=Path)
    args = parser.parse_args()
    try:
        count = verify(json.loads(line) for line in args.events.read_text().splitlines() if line)
    except (ValueError, OSError) as error:
        parser.exit(1, f'migration CI evidence rejected: {error}\n')
    print(f'PASS: {count} required database test results, none missing or skipped')


if __name__ == '__main__':
    main()
