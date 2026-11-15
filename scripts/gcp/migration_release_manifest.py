#!/usr/bin/env python3
"""Offline release manifest and schema-plan checks; does not access cloud or SQL."""
import argparse
import json
from pathlib import Path
import re
from verify_migration_inventory import validate as validate_inventory
from v84_authority import validate_catalog as validate_v84_catalog
from identity_tail_authority import IDENTITY_TAIL_DDL_MIGRATIONS,validate_authority as validate_identity_tail_authority

FORMAT='espacigo.database-release.v1'
TARGET={'project':'espacigo-testprod','region':'southamerica-west1','instance':'espacigo-testprod-db','database':'espacigo_dms_stage_20261020'}
REGISTRY='southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod'
IMAGES={'api':'espacigo-api','web':'espacigo-web','migrator':'espacigo-db-migrator'}
V84_MIGRATION_NAME='V000084__didit_identity_verification_schema.sql'
IDENTITY_DDL_MIGRATIONS=frozenset((V84_MIGRATION_NAME,'V000085__identity_encrypted_declarations_and_minimal_events.sql','V000086__identity_declaration_retention_purge.sql','V000087__identity_declaration_retention_worker.sql','V000088__revoke_runtime_identity_purge_execute.sql'))
V84_AUTHORITY_FIELDS={'session_user','current_user','current_schema','session_role','schema_owner_role','public_schema_owner','public_usage','public_create','usuario_owner','usuario_id_references','ledger_owner','ledger_select','ledger_insert','memberships','catalog'}
V84_AUTHORITY_PRIVILEGES=('public_usage','public_create','usuario_id_references','ledger_select','ledger_insert')
V84_ROLE_FIELDS={'can_login','inherit','superuser','create_role','create_db','replication','bypass_rls'}
V84_ROLE_ELEVATIONS=('inherit','superuser','create_role','create_db','replication','bypass_rls')


def validate(manifest):
    if not isinstance(manifest,dict) or set(manifest)!={'format','source_sha','target','images','migration_inventory'} or manifest['format']!=FORMAT:
        raise ValueError('unsupported release manifest')
    if not isinstance(manifest['source_sha'],str) or not re.fullmatch('[a-f0-9]{40}',manifest['source_sha']):
        raise ValueError('full source SHA required')
    if manifest['target']!=TARGET:
        raise ValueError('release target differs from the reviewed development destination')
    images=manifest['images']
    if not isinstance(images,dict) or set(images)!=set(IMAGES):
        raise ValueError('API, web and migrator image references are required')
    for name,repository in IMAGES.items():
        if not isinstance(images[name],str) or not re.fullmatch(re.escape(REGISTRY+'/'+repository)+r'@sha256:[a-f0-9]{64}',images[name]):
            raise ValueError('all release images must use reviewed repositories and immutable digests')
    validate_inventory(manifest['migration_inventory'])
    return manifest


def check_plan(manifest,plan,postcheck=False):
    validate(manifest)
    base_fields={'database','server_version','read_only','history_exists','applied','pending'}
    pending=plan.get('pending') if isinstance(plan,dict) else None
    needs_v84_authority=isinstance(pending,list) and any(isinstance(row,dict) and row.get('name') in IDENTITY_DDL_MIGRATIONS for row in pending)
    needs_identity_tail_authority=isinstance(pending,list) and any(isinstance(row,dict) and row.get('name') in IDENTITY_TAIL_DDL_MIGRATIONS for row in pending)
    expected_fields=base_fields|({'ddl_authority'} if needs_v84_authority else set())|({'identity_ddl_authority'} if needs_identity_tail_authority else set())
    if not isinstance(plan,dict) or set(plan)!=expected_fields:
        raise ValueError('invalid migration plan contract')
    if plan['database']!=TARGET['database'] or plan['read_only'] is not True or plan['history_exists'] is not True:
        raise ValueError('plan must identify the existing target and a read-only ledger')
    if not isinstance(plan['server_version'],str) or not re.fullmatch(r'18(?:\.\d+)?(?:\s.*)?',plan['server_version']):
        raise ValueError('PostgreSQL 18 is required')
    if not isinstance(plan['applied'],list) or not isinstance(plan['pending'],list):
        raise ValueError('plan migration arrays are missing')
    expected=manifest['migration_inventory']['migrations']
    count=len(plan['applied'])
    if count>len(expected) or plan['applied']!=expected[:count] or plan['pending']!=expected[count:]:
        raise ValueError('plan has a gap, changed checksum/name, unknown migration or different source inventory')
    if needs_v84_authority:
        authority=plan['ddl_authority']
        if not isinstance(authority,dict) or set(authority)!=V84_AUTHORITY_FIELDS:
            raise ValueError('V84 pending migration lacks the exact DDL authority report')
        if authority['session_user']!='espacigo_migrator_login' or authority['current_user']!='espacigo_schema_owner' or authority['current_schema']!='public':
            raise ValueError('V84 preflight database identity or target schema differs')
        for field,can_login in (('session_role',True),('schema_owner_role',False)):
            role=authority[field]
            if not isinstance(role,dict) or set(role)!=V84_ROLE_FIELDS or any(type(value) is not bool for value in role.values()):
                raise ValueError('V84 preflight role attributes are incomplete: '+field)
            if role['can_login'] is not can_login or any(role[name] is not False for name in V84_ROLE_ELEVATIONS):
                raise ValueError('V84 preflight role has unsafe privilege attributes: '+field)
        for field in ('public_schema_owner','usuario_owner','ledger_owner'):
            if not isinstance(authority[field],str) or not authority[field]:
                raise ValueError('V84 preflight object owner is missing')
        for field in V84_AUTHORITY_PRIVILEGES:
            if type(authority[field]) is not bool or authority[field] is not True:
                raise ValueError('V84 preflight lacks effective DDL or ledger privilege: '+field)
        validate_v84_catalog(authority)
    if needs_identity_tail_authority:
        names=[row.get('name') for row in (pending if isinstance(pending,list) else []) if isinstance(row,dict) and row.get('name') in IDENTITY_TAIL_DDL_MIGRATIONS]
        validate_identity_tail_authority(plan['identity_ddl_authority'],names)
    if postcheck and count!=len(expected):
        raise ValueError('postcheck still has pending migrations')
    return {'applied':count,'pending':len(expected)-count,'source_sha':manifest['source_sha']}


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    sub=parser.add_subparsers(dest='action',required=True)
    create=sub.add_parser('create')
    create.add_argument('--source-sha',required=True)
    create.add_argument('--inventory',required=True,type=Path)
    for name in IMAGES:create.add_argument('--'+name+'-image',required=True)
    check=sub.add_parser('check-plan')
    check.add_argument('manifest',type=Path)
    check.add_argument('plan',type=Path)
    check.add_argument('--postcheck',action='store_true')
    args=parser.parse_args()
    try:
        if args.action=='create':
            result=validate({'format':FORMAT,'source_sha':args.source_sha,'target':TARGET.copy(),'images':{name:getattr(args,name+'_image') for name in IMAGES},'migration_inventory':json.loads(args.inventory.read_text())})
        else:
            result=check_plan(json.loads(args.manifest.read_text()),json.loads(args.plan.read_text()),args.postcheck)
    except (ValueError,OSError,TypeError) as error:
        parser.exit(1,f'release manifest rejected: {error}\n')
    print(json.dumps(result,indent=2))


if __name__=='__main__':main()
