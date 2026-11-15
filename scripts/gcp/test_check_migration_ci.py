import unittest
from check_migration_ci import FULL_REQUIRED, V19_REQUIRED, V76_REQUIRED, verify
import check_migration_ci as migration_ci
import re
from check_migration_ci import REQUIRED, verify

PACKAGE = 'github.com/HernanEspinozaDev/espaciGo/internal/migrator'
AUTHORITY_CASES = {
    'failure_cleanup', 'migration_failure_cleanup', 'rejects_public_table_grant',
    'rejects_public_column_grant', 'rejects_other_table_grantee',
    'rejects_runtime_membership_drift', 'rejects_runtime_grant_option',
}
COMBINED = 'TestRentalCombinedFixtureOptIn'
V97_ROOTS = {COMBINED + '/v97-upgrade'} | {
    f'TestCurrentSchemaMigrationFromEmptyAndV75/{baseline}/v97-upgrade'
    for baseline in ('empty', 'upgrade-v75')
}
V97_AUTHORITY_EVENTS = V97_ROOTS | {
    root + '/' + case for root in V97_ROOTS for case in AUTHORITY_CASES
} | {
    COMBINED, COMBINED + '/v97-without-authority', COMBINED + '/commercial-quote-persistence',
    COMBINED + '/commercial-quote-persistence/retention-purge-publication-lock',
    COMBINED + '/commercial-quote-persistence/admin-local-account-block',
    COMBINED + '/commercial-quote-persistence/authentication-account-lock',
    COMBINED + '/commercial-quote-persistence/authentication-account-lock/email',
    COMBINED + '/commercial-quote-persistence/authentication-account-lock/verification-id',
    COMBINED + '/commercial-quote-persistence/authentication-account-lock/action-token-id',
    COMBINED + '/commercial-quote-persistence/authentication-account-lock/session-hash',
}
REAL_AUTHORITY_EVENTS = {
    root.removesuffix('/v97-upgrade') + f'/authority-wire-v{version}'
    for root in V97_ROOTS for version in (84, 85, 86, 87, 88, 95, 96)
}


def successful_events(scope='full'):
    required = {'full': FULL_REQUIRED, 'v19': V19_REQUIRED, 'v76': V76_REQUIRED}[scope]
    return [dict(Package=PACKAGE, Action='pass', Test=t) for t in required] + [dict(Package=PACKAGE, Action='pass')]


class MigrationEvidenceTests(unittest.TestCase):
    def test_requires_real_postgres_authority_wire_on_all_paths(self):
        self.assertTrue(REAL_AUTHORITY_EVENTS.issubset(REQUIRED))

    def test_real_authority_wire_cannot_be_omitted(self):
        for required in REAL_AUTHORITY_EVENTS:
            with self.subTest(required=required):
                events = [event for event in successful_events() if event.get('Test') != required]
                with self.assertRaisesRegex(ValueError, re.escape(required)):
                    verify(events)

    def test_requires_v97_lifecycle_on_all_three_real_schema_paths(self):
        self.assertTrue(V97_AUTHORITY_EVENTS.issubset(REQUIRED))
        self.assertFalse(any('/v85_authority' in event for event in REQUIRED))

    def test_each_v97_checkpoint_is_required_and_missing_name_is_reported(self):
        for required in V97_AUTHORITY_EVENTS:
            with self.subTest(required=required):
                events = [event for event in successful_events() if event.get('Test') != required]
                with self.assertRaisesRegex(ValueError, re.escape(required)):
                    verify(events)

    def test_requires_every_database_test_and_package_success(self):
        self.assertEqual(verify(successful_events()), len(FULL_REQUIRED))
        for events in [[], successful_events()[:-1], successful_events()[1:]]:
            with self.assertRaises(ValueError):
                verify(events)

    def test_rejects_skip_failure_and_wrong_package_even_after_pass(self):
        for action in ['skip', 'fail']:
            with self.assertRaises(ValueError):
                verify(successful_events() + [dict(Package=PACKAGE, Action=action, Test=next(iter(FULL_REQUIRED)))])
        with self.assertRaises(ValueError):
            verify([dict(event, Package='unrelated/package') for event in successful_events()])

    def test_full_scope_requires_v93_release_for_each_nested_baseline(self):
        events = successful_events()
        for baseline in ('empty', 'upgrade-v75', 'upgrade-v76'):
            case = f'TestCurrentSchemaMigrationKYCThroughV96/{baseline}/release-v92-v93'
            with self.subTest(baseline=baseline), self.assertRaises(ValueError):
                verify([event for event in events if event.get('Test') != case])

    def test_full_scope_summary_tracks_v93_inventory(self):
        self.assertEqual(verify(successful_events()), len(FULL_REQUIRED))
        self.assertTrue(all(f'TestCurrentSchemaMigrationKYCThroughV96/{b}/release-v92-v93' in FULL_REQUIRED for b in ('empty', 'upgrade-v75', 'upgrade-v76')))

    def test_full_scope_requires_v94_release_and_noop_for_each_baseline(self):
        events = successful_events()
        for baseline in ('empty', 'upgrade-v75', 'upgrade-v76'):
            for case in ('release-v93-v94', 'release-noop'):
                required = f'TestCurrentSchemaMigrationKYCThroughV96/{baseline}/{case}'
                with self.subTest(baseline=baseline, case=case), self.assertRaises(ValueError):
                    verify([event for event in events if event.get('Test') != required])

    def test_v19_scope_requires_only_the_v19_prefix_test(self):
        events = successful_events('v19')
        self.assertEqual(verify(events, 'v19'), len(V19_REQUIRED))
        with self.assertRaises(ValueError):
            verify(events[:-2], 'v19')

    def test_v76_scope_requires_each_transition_boundary_assertion(self):
        events = successful_events('v76')
        self.assertEqual(verify(events, 'v76'), len(V76_REQUIRED))
        with self.assertRaises(ValueError):
            verify([event for event in events if event.get('Test') != 'TestGalleryCapacityTransitionV68ToV76/v76-thirty-first-slot-rejected'], 'v76')

    def test_authority_window_scope_requires_v95_v96_postgres_boundaries(self):
        required = getattr(migration_ci, 'AUTHORITY_WINDOW_REQUIRED', frozenset())
        expected = {
            'TestNativeKYCAuthorityWindowPostgres',
            'TestNativeKYCAuthorityWindowPostgres/v95-wrong-target-no-ddl',
            'TestNativeKYCAuthorityWindowPostgres/v95-prepare-finalize-retry',
            'TestNativeKYCAuthorityWindowPostgres/v96-wrong-target-no-ddl',
            'TestNativeKYCAuthorityWindowPostgres/v96-prepare-finalize-retry',

        }
        self.assertTrue(expected <= required, f'missing required authority-window cases: {expected - required}')
        events = [dict(Package=PACKAGE, Action='pass', Test=name) for name in required]
        events.append(dict(Package=PACKAGE, Action='pass'))
        self.assertEqual(verify(events, 'authority-window'), len(required))
        with self.assertRaises(ValueError):
            verify([event for event in events if event.get('Test') != 'TestNativeKYCAuthorityWindowPostgres/v96-prepare-finalize-retry'], 'authority-window')


if __name__ == '__main__':
    unittest.main()
