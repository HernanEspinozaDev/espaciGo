import unittest
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


def successful_events():
    return [dict(Package=PACKAGE, Action='pass', Test=t) for t in REQUIRED] + [dict(Package=PACKAGE, Action='pass')]


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
        self.assertEqual(verify(successful_events()), len(REQUIRED))
        for events in [[], successful_events()[:-1], successful_events()[1:]]:
            with self.assertRaises(ValueError):
                verify(events)

    def test_rejects_skip_failure_and_wrong_package_even_after_pass(self):
        for action in ['skip', 'fail']:
            with self.assertRaises(ValueError):
                verify(successful_events() + [dict(Package=PACKAGE, Action=action, Test=next(iter(REQUIRED)))])
        with self.assertRaises(ValueError):
            verify([dict(event, Package='unrelated/package') for event in successful_events()])


if __name__ == '__main__':
    unittest.main()
