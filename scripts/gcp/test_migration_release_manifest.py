import copy
import hashlib
import unittest
from migration_release_manifest import FORMAT,IMAGES,REGISTRY,TARGET,check_plan,validate
from test_verify_migration_inventory import report


def manifest():
    return {'format':FORMAT,'source_sha':'a'*40,'target':TARGET.copy(),'images':{name:REGISTRY+'/'+repository+'@sha256:'+'b'*64 for name,repository in IMAGES.items()},'migration_inventory':report()}


def plan(applied=0):
    rows=report()['migrations']
    return {'database':TARGET['database'],'server_version':'18.6','read_only':True,'history_exists':True,'applied':rows[:applied],'pending':rows[applied:]}


def v84_release_fixture():
    rows=[]
    digest=hashlib.sha256()
    for version in range(1,85):
        name='V000084__didit_identity_verification_schema.sql' if version==84 else f'V{version:06d}__probe.sql'
        checksum=f'{version:064x}'
        rows.append({'version':version,'name':name,'checksum':checksum})
        digest.update(f'{checksum}  {name}\n'.encode())
    fixture=copy.deepcopy(manifest())
    fixture['migration_inventory']={'format':'espacigo.migrations.inventory.v1','migrations':rows,'inventory_sha256':digest.hexdigest()}
    restricted={'inherit':False,'superuser':False,'create_role':False,'create_db':False,'replication':False,'bypass_rls':False}
    authority={'session_user':'espacigo_migrator_login','current_user':'espacigo_schema_owner','current_schema':'public','session_role':{'can_login':True,**restricted},'schema_owner_role':{'can_login':False,**restricted},'public_schema_owner':'pg_database_owner','public_usage':True,'public_create':True,'usuario_owner':'espacigo_schema_owner','usuario_id_references':True,'ledger_owner':'espacigo_schema_owner','ledger_select':True,'ledger_insert':True}
    authority['memberships']=[{'role':'espacigo_schema_owner','member':'espacigo_migrator_login','admin':False,'inherit':False,'set':True},{'role':'espacigo_document_verification_fn','member':'espacigo_schema_owner','admin':False,'inherit':False,'set':True}]
    authority['catalog']={'schemas':['pg_catalog','public'],'helper_role':{'can_login':False,**restricted},'database_owner':'postgres','database_create':False,'acl':[{'object':'public','column':'','owner':'pg_database_owner','grantor':'pg_database_owner','grantee':'PUBLIC','privilege':'USAGE','grantable':False}]}
    preflight={'database':TARGET['database'],'server_version':'18.6','read_only':True,'history_exists':True,'applied':rows[:83],'pending':rows[83:],'ddl_authority':authority}
    return fixture,preflight


def v95_release_fixture(applied=94):
    rows=[]
    digest=hashlib.sha256()
    tail_names={95:'V000095__identity_document_case_declaration_binding.sql',96:'V000096__identity_reviewer_observation_audit.sql'}
    for version in range(1,97):
        name=tail_names.get(version,f'V{version:06d}__probe.sql')
        checksum=f'{version:064x}'
        rows.append({'version':version,'name':name,'checksum':checksum})
        digest.update(f'{checksum}  {name}\n'.encode())
    release=copy.deepcopy(manifest())
    release['migration_inventory']={'format':'espacigo.migrations.inventory.v1','migrations':rows,'inventory_sha256':digest.hexdigest()}
    safe_role={'can_login':False,'inherit':False,'superuser':False,'create_role':False,'create_db':False,'replication':False,'bypass_rls':False}
    authority={
        'session_user':'espacigo_migrator_login','current_user':'espacigo_schema_owner','current_schema':'public','session_role':{'can_login':True,'inherit':False,'superuser':False,'create_role':False,'create_db':False,'replication':False,'bypass_rls':False},'public_create':True,
        'schema_owner_exists':True,'schema_owner_role':dict(safe_role),'runtime_exists':True,'runtime_role':dict(safe_role),
        'privacy_cleanup_worker_exists':True,'privacy_cleanup_worker_role':dict(safe_role),
        'case_owner':'espacigo_schema_owner','declaration_owner':'espacigo_schema_owner','event_owner':'espacigo_schema_owner','purge_owner':'espacigo_schema_owner',
        'purge_security_definer':True,'purge_search_path':['search_path=pg_catalog'],'purge_non_owner_acl_count':1,
        'purge_worker_execute':True,'purge_worker_grantor_owner':True,'purge_worker_grantable':False,'runtime_purge_execute':False,
        'runtime_case_select':True,'runtime_case_expiry_update':True,'runtime_event_insert':True,
    }
    preflight={'database':TARGET['database'],'server_version':'18.6','read_only':True,'history_exists':True,
        'applied':rows[:applied],'pending':rows[applied:],'identity_ddl_authority':authority}
    return release,preflight


class ReleaseManifestTests(unittest.TestCase):
    def test_v95_v96_plan_requires_and_accepts_the_identity_tail_authority_report(self):
        release,preflight=v95_release_fixture()
        self.assertEqual(check_plan(release,preflight)['pending'],2)
        v96_release,v96_preflight=v95_release_fixture(95)
        v96_preflight['identity_ddl_authority']['privacy_cleanup_worker_exists']=False
        self.assertEqual(check_plan(v96_release,v96_preflight)['pending'],1)

    def test_v95_v96_plan_rejects_partial_unknown_and_guard_mismatch_authority(self):
        release,preflight=v95_release_fixture()
        for mutate in [
            lambda p:p.pop('identity_ddl_authority'),
            lambda p:p['identity_ddl_authority'].pop('event_owner'),
            lambda p:p['identity_ddl_authority'].update(unexpected=True),
            lambda p:p['identity_ddl_authority'].update(public_create=1),
            lambda p:p['identity_ddl_authority']['session_role'].update(superuser=True),
            lambda p:p['identity_ddl_authority'].update(case_owner='postgres'),
            lambda p:p['identity_ddl_authority'].update(purge_non_owner_acl_count=True),
            lambda p:p['identity_ddl_authority'].update(runtime_purge_execute=True),
            lambda p:p['identity_ddl_authority'].update(runtime_event_insert=False),
        ]:
            fixture=copy.deepcopy(preflight);mutate(fixture)
            with self.assertRaises(ValueError):check_plan(release,fixture)

    def test_v85_requires_the_identity_ddl_authority_report_too(self):
        release,preflight=v84_release_fixture()
        rows=release['migration_inventory']['migrations']
        row={'version':85,'name':'V000085__identity_encrypted_declarations_and_minimal_events.sql','checksum':'c'*64}
        rows.append(row)
        release['migration_inventory']['inventory_sha256']=hashlib.sha256(''.join(f"{m['checksum']}  {m['name']}\n" for m in rows).encode()).hexdigest()
        preflight['applied']=rows[:84];preflight['pending']=rows[84:]
        self.assertEqual(check_plan(release,preflight)['pending'],1)
        preflight.pop('ddl_authority')
        with self.assertRaises(ValueError):check_plan(release,preflight)

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

    def test_v84_preflight_requires_exact_job_identity_and_effective_ddl_authority(self):
        release,preflight=v84_release_fixture()
        self.assertEqual(check_plan(release,preflight)['pending'],1)
        for key in ['public_usage','public_create','usuario_id_references','ledger_select','ledger_insert']:
            fixture=copy.deepcopy(preflight);fixture['ddl_authority'][key]=False
            with self.assertRaises(ValueError):check_plan(release,fixture)
        for key,value in [('session_user','postgres'),('current_user','postgres'),('current_schema','other')]:
            fixture=copy.deepcopy(preflight);fixture['ddl_authority'][key]=value
            with self.assertRaises(ValueError):check_plan(release,fixture)
        for role in ['session_role','schema_owner_role']:
            for key,value in preflight['ddl_authority'][role].items():
                fixture=copy.deepcopy(preflight);fixture['ddl_authority'][role][key]=not value
                with self.assertRaises(ValueError):check_plan(release,fixture)
            fixture=copy.deepcopy(preflight);fixture['ddl_authority'][role].pop('inherit')
            with self.assertRaises(ValueError):check_plan(release,fixture)
            fixture=copy.deepcopy(preflight);fixture['ddl_authority'][role]['inherit']='false'
            with self.assertRaises(ValueError):check_plan(release,fixture)
        for authority in [None,{**preflight['ddl_authority'],'unexpected':True}]:
            fixture=copy.deepcopy(preflight);fixture['ddl_authority']=authority
            with self.assertRaises(ValueError):check_plan(release,fixture)

    def test_mutable_image_wrong_sha_and_altered_manifest_cannot_authorize_a_release(self):
        for mutate in [lambda m:m.update(source_sha='main'),lambda m:m['target'].update(instance='other'),lambda m:m['images'].update(api=REGISTRY+'/espacigo-api:latest')]:
            fixture=copy.deepcopy(manifest());mutate(fixture)
            with self.assertRaises(ValueError):validate(fixture)


if __name__=='__main__':unittest.main()
