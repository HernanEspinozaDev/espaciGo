import copy
import hashlib
import unittest
from verify_migration_inventory import FORMAT, compare, validate


def report():
    checksum = hashlib.sha256(b'SELECT 1;\n').hexdigest()
    return {'format': FORMAT, 'migrations': [{'version': 1, 'name': 'V000001__probe.sql', 'checksum': checksum}], 'inventory_sha256': hashlib.sha256(f'{checksum}  V000001__probe.sql\n'.encode()).hexdigest()}


class MigrationInventoryTests(unittest.TestCase):
    def test_exact_inventory(self):
        self.assertEqual(compare(report(), report()), 1)

    def test_rejects_omission_tampering_rename_and_noninteger_version(self):
        for mutate in [lambda r: r.update(migrations=[]), lambda r: r['migrations'][0].update(checksum='0'*64), lambda r: r['migrations'][0].update(name='../V000001__probe.sql'), lambda r: r['migrations'][0].update(version=True), lambda r: r.update(extra='untrusted')]:
            fixture = report()
            mutate(fixture)
            with self.assertRaises(ValueError):
                validate(fixture)

    def test_rejects_consistently_rehashed_but_different_sql(self):
        changed = copy.deepcopy(report())
        changed['migrations'][0]['checksum'] = 'a'*64
        changed['inventory_sha256'] = hashlib.sha256(f"{'a'*64}  V000001__probe.sql\n".encode()).hexdigest()
        validate(changed)
        with self.assertRaises(ValueError):
            compare(report(), changed)


if __name__ == '__main__':
    unittest.main()
