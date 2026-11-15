import copy
import unittest
from migration_release_manifest import check_plan
from test_migration_release_manifest import v84_release_fixture


class V84CatalogTests(unittest.TestCase):
    def test_default_public_temporary_database_acl_is_accepted_narrowly(self):
        release, base = v84_release_fixture()
        row = {'object':'database','column':'','owner':'postgres','grantee':'PUBLIC','grantor':'postgres','privilege':'TEMPORARY','grantable':False}
        base['ddl_authority']['catalog']['acl'].append(row)
        self.assertEqual(check_plan(release,base)['pending'],1)
        for change in [
            {'grantable':True},
            {'grantor':'unreviewed'},
        ]:
            bad = copy.deepcopy(base)
            bad['ddl_authority']['catalog']['acl'][-1].update(change)
            with self.assertRaises(ValueError):
                check_plan(release,bad)

    def test_catalog_negatives_fail_closed(self):
        release, base = v84_release_fixture()
        mutations = [
            lambda a: a.update(catalog=None),
            lambda a: a.update(memberships=[]),
            lambda a: a['memberships'].append(copy.deepcopy(a['memberships'][0])),
            lambda a: a['memberships'][0].update(admin=True),
            lambda a: a['memberships'][0].update(inherit=True),
            lambda a: a['memberships'][0].update(set=False),
            lambda a: a.update(usuario_owner='unreviewed'),
            lambda a: a.update(ledger_owner='unreviewed'),
            lambda a: a.update(public_schema_owner='unreviewed'),
            lambda a: a['catalog'].update(schemas=['public','pg_catalog']),
            lambda a: a['catalog'].update(database_create=True),
            lambda a: a['catalog'].update(database_owner=a['current_user']),
            lambda a: a['catalog'].update(acl=[]),
            lambda a: a['catalog']['acl'][0].update(privilege='CREATE'),
            lambda a: a['catalog']['acl'][0].update(grantable=True),
            lambda a: a['catalog']['acl'][0].update(object='default:public:r',privilege='SELECT'),
        ]
        for key in base['ddl_authority']['catalog']['helper_role']:
            mutations.append(lambda a, key=key: a['catalog']['helper_role'].update({key: True}))
        for index, mutate in enumerate(mutations):
            with self.subTest(index=index):
                bad = copy.deepcopy(base)
                mutate(bad['ddl_authority'])
                with self.assertRaises(ValueError):
                    check_plan(release,bad)


if __name__ == '__main__':
    unittest.main()
