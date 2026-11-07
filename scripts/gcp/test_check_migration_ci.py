import unittest
from check_migration_ci import REQUIRED, verify

PACKAGE = 'github.com/HernanEspinozaDev/espaciGo/internal/migrator'


def successful_events():
    return [dict(Package=PACKAGE, Action='pass', Test=t) for t in REQUIRED] + [dict(Package=PACKAGE, Action='pass')]


class MigrationEvidenceTests(unittest.TestCase):
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
