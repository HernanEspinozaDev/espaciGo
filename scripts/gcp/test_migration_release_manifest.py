import copy
import unittest
from migration_release_manifest import FORMAT,IMAGES,REGISTRY,TARGET,check_plan,validate
from test_verify_migration_inventory import report


def manifest():
    return {'format':FORMAT,'source_sha':'a'*40,'target':TARGET.copy(),'images':{name:REGISTRY+'/'+repository+'@sha256:'+'b'*64 for name,repository in IMAGES.items()},'migration_inventory':report()}


def plan(applied=0):
    rows=report()['migrations']
    return {'database':TARGET['database'],'server_version':'18.6','read_only':True,'history_exists':True,'applied':rows[:applied],'pending':rows[applied:]}


class ReleaseManifestTests(unittest.TestCase):
    def test_preflight_allows_exact_pending_suffix_and_postcheck_requires_complete_inventory(self):
        self.assertEqual(check_plan(manifest(),plan())['pending'],1)
        self.assertEqual(check_plan(manifest(),plan(1),True)['pending'],0)
        with self.assertRaises(ValueError):check_plan(manifest(),plan(),True)

    def test_wrong_target_engine_non_readonly_and_unknown_history_are_rejected(self):
        for key,value in [('database','other'),('server_version','17.9'),('read_only',False),('history_exists',False)]:
            fixture=plan();fixture[key]=value
            with self.assertRaises(ValueError):check_plan(manifest(),fixture)
        for rows in [[],[dict(report()['migrations'][0],checksum='0'*64)]]:
            fixture=plan();fixture['pending']=rows
            with self.assertRaises(ValueError):check_plan(manifest(),fixture)
        fixture=plan(1);fixture['applied']*=2
        with self.assertRaises(ValueError):check_plan(manifest(),fixture)

    def test_mutable_image_wrong_sha_and_altered_manifest_cannot_authorize_a_release(self):
        for mutate in [lambda m:m.update(source_sha='main'),lambda m:m['target'].update(instance='other'),lambda m:m['images'].update(api=REGISTRY+'/espacigo-api:latest')]:
            fixture=copy.deepcopy(manifest());mutate(fixture)
            with self.assertRaises(ValueError):validate(fixture)


if __name__=='__main__':unittest.main()
