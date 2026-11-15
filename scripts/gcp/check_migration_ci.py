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
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/release-v91-v92',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/release-v91-v92',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/release-v91-v92',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/release-v92-v93',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/release-v92-v93',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/release-v92-v93',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/release-noop',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/release-noop',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/catalog-read',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/api-schema-contract',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/release-noop',
    'TestCurrentSchemaMigrationFromEmptyAndV75/empty/release-v83-v84',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v75/release-v83-v84',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/release-v83-v84',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/native-role-transition-contract',
    'TestCurrentSchemaMigrationFromEmptyAndV75/upgrade-v76/v77-duplicate-predecessor-blocks',
}) | frozenset(
    f'TestCurrentSchemaMigrationFromEmptyAndV75/{baseline}/{case}'
    for baseline in ('empty','upgrade-v75','upgrade-v76')
    for case in ('authority-wire',
                 'v67-inmuebles-seed',
                 'release-v84-v85',
                 'release-v84-v85/legacy-data-blocks-without-deleting',
                 'release-v85-v86',
                 'release-v86-v87',
                 'release-v87-v88',
                 'release-v88-v89',
                 'release-v89-v90',
                 'release-v90-v91',
                 'release-v91-v92',
                 'release-v92-v93',
                 'release-v93-v94',
                 'release-v94-v95',
                 'release-v94-v95/foreign-case-owner-blocks-v95',
                 'release-v94-v95/v95-identity-case-binding',
                 'release-v95-v96',
                 'release-v95-v96/foreign-event-owner-blocks-v96',
                 'release-v95-v96/v96-identity-review-audit',
                 'release-v93-v94/v94-worker-only-bounded-retirement',
                 'v85-identity-schema',
                 'release-v86-v87/v87-identity-retention-worker',
                 'v88-runtime-execute-revoked',
                 'v85-identity-schema/short_ciphertext',
                 'v85-identity-schema/cross-account_declaration',
                 'v85-identity-schema/duplicate_active_intention',
                 'v85-identity-schema/duplicate_event',
                 'v85-identity-schema/wrong_event_namespace',
                 'v85-identity-schema/immutable_declaration',
                 'v85-identity-schema/immutable_attempt_owner',
                 'v85-identity-schema/partial_encrypted_decision',
                 'v85-identity-schema/missing_processed_time',
                 'v85-identity-schema/invalid_retention',
                 'authority-negative/extra-outgoing-membership',
                 'authority-negative/extra-incoming-membership',
                 'authority-negative/helper-escalation',
                 'authority-negative/unexpected-direct-acl',
                 'authority-negative/grant-option',
                 'authority-negative/public-data-exposure',
                 'authority-negative/unsafe-table-default')
)

V19_REQUIRED = frozenset({
    'TestCurrentSchemaMigrationV19WeeklyHours',
    'TestCurrentSchemaMigrationV19WeeklyHours/fixture-required',
    'TestCurrentSchemaMigrationV19WeeklyHours/interval-day-check',
    'TestCurrentSchemaMigrationV19WeeklyHours/interval-positive-duration-check',
    'TestCurrentSchemaMigrationV19WeeklyHours/interval-overlap-exclusion',
    'TestCurrentSchemaMigrationV19WeeklyHours/disabled-fixture-rejected',
    'TestCurrentSchemaMigrationV19WeeklyHours/latest-tariff-must-be-hourly',
    'TestCurrentSchemaMigrationV19WeeklyHours/valid-fixture-enables-hourly-schedule',
    'TestCurrentSchemaMigrationV19WeeklyHours/zone-change-blocked',
    'TestCurrentSchemaMigrationV19WeeklyHours/space-mode-change-blocked',
    'TestCurrentSchemaMigrationV19WeeklyHours/tariff-mode-change-blocked',
})
V76_REQUIRED = frozenset({
    'TestGalleryCapacityTransitionV68ToV76',
    'TestGalleryCapacityTransitionV68ToV76/v68-backfill-ten-ordered',
    'TestGalleryCapacityTransitionV68ToV76/v68-eleventh-slot-rejected-at-10',
    'TestGalleryCapacityTransitionV68ToV76/v76-upgrade-preserves-legacy-and-runtime-grant',
    'TestGalleryCapacityTransitionV68ToV76/v76-concurrent-29-to-30-allows-one-writer',
    'TestGalleryCapacityTransitionV68ToV76/v76-thirtieth-order-position-accepted',
    'TestGalleryCapacityTransitionV68ToV76/v76-thirty-first-slot-rejected',
})
AUTHORITY_WINDOW_REQUIRED = frozenset({
    'TestNativeKYCAuthorityWindowPostgres',
    'TestNativeKYCAuthorityWindowPostgres/v95-wrong-target-no-ddl',
    'TestNativeKYCAuthorityWindowPostgres/v95-prepare-finalize-retry',
    'TestNativeKYCAuthorityWindowPostgres/v96-wrong-target-no-ddl',
    'TestNativeKYCAuthorityWindowPostgres/v96-prepare-finalize-retry',
})
FULL_REQUIRED = REQUIRED | V19_REQUIRED


def verify(events, scope='full'):
    required = {'full': FULL_REQUIRED, 'v19': V19_REQUIRED, 'v76': V76_REQUIRED,
                'authority-window': AUTHORITY_WINDOW_REQUIRED}.get(scope)
    if required is None:
        raise ValueError(f'unknown migration evidence scope: {scope}')
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
    missing = sorted(required - passed)
    if not package_passed or missing:
        details = ', '.join(missing) if missing else 'package-level pass missing'
        raise ValueError(f'required database tests were not all executed successfully: {details}')
    return len(required)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--scope', choices=('full', 'v19', 'v76', 'authority-window'), default='full')
    parser.add_argument('events', type=Path)
    args = parser.parse_args()
    try:
        count = verify((json.loads(line) for line in args.events.read_text().splitlines() if line), args.scope)
    except (ValueError, OSError) as error:
        parser.exit(1, f'migration CI evidence rejected: {error}\n')
    print(f'PASS: {count} required database test results, none missing or skipped')


if __name__ == '__main__':
    main()
