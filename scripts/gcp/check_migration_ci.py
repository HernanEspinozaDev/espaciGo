#!/usr/bin/env python3
"""Reject successful-looking migration runs with missing or skipped DB tests."""
import argparse
import json
from pathlib import Path

V97_AUTHORITY_ROOTS = (
    'TestRentalCombinedFixtureOptIn/v97-upgrade',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/v97-upgrade',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/v97-upgrade',
)
V97_AUTHORITY_CASES = (
    'failure_cleanup', 'migration_failure_cleanup', 'rejects_public_table_grant',
    'rejects_public_column_grant', 'rejects_other_table_grantee',
    'rejects_runtime_membership_drift', 'rejects_runtime_grant_option',
)

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
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/api-schema-contract/commercial-quotes',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/api-schema-contract/commercial-quotes',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/commercial-quote-suppression',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/commercial-quote-suppression',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/commercial-quote-persistence',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/commercial-quote-persistence',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/commercial-quote-persistence/retention-purge-publication-lock',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/commercial-quote-persistence/retention-purge-publication-lock',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/release-noop',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/release-noop',
    'TestRunnerV84AuthorityRecoveryPostgres',
    'TestRunnerV84AuthorityRecoveryPostgres/failure_cleanup',
    'TestRunnerV84AuthorityRecoveryPostgres/apply_and_finalize',
    'TestRentalCombinedFixtureOptIn',
    'TestRentalCombinedFixtureOptIn/v97-without-authority',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/retention-purge-publication-lock',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/admin-local-account-block',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/authentication-account-lock',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/authentication-account-lock/email',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/authentication-account-lock/verification-id',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/authentication-account-lock/action-token-id',
    'TestRentalCombinedFixtureOptIn/commercial-quote-persistence/authentication-account-lock/session-hash',
}) | frozenset(V97_AUTHORITY_ROOTS) | frozenset(
    root + '/' + case for root in V97_AUTHORITY_ROOTS for case in V97_AUTHORITY_CASES
) | frozenset(
    root.removesuffix('/v97-upgrade') + f'/authority-wire-v{version}'
    for root in V97_AUTHORITY_ROOTS for version in (84, 85, 86, 87, 88, 95, 96)
)


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
    if not package_passed:
        raise ValueError('required database package did not complete successfully')
    missing = REQUIRED - passed
    if missing:
        raise ValueError('required database tests were not all executed successfully: ' + ', '.join(sorted(missing)))
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
