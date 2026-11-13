#!/usr/bin/env python3
"""Offline release manifest and schema-plan checks; does not access cloud or SQL."""
import argparse
import json
from pathlib import Path
import re
from verify_migration_inventory import validate as validate_inventory

FORMAT='espacigo.database-release.v1'
TARGET={'project':'espacigo-testprod','region':'southamerica-west1','instance':'espacigo-testprod-db','database':'espacigo_dms_stage_20261020'}
REGISTRY='southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod'
IMAGES={'api':'espacigo-api','web':'espacigo-web','migrator':'espacigo-db-migrator'}
PLAN_KEYS={'database','server_version','read_only','history_exists','applied','pending'}
PLAN_IDENTITY_KEYS={'session_user','current_user'}
PLAN_SESSION_USER='espacigo_migrator_login'
PLAN_CURRENT_USER='espacigo_schema_owner'


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


def is_plan_report_shape(report):
    """Recognize legacy/new plan payloads so the log reader passes them to validation."""
    if not isinstance(report,dict):
        return False
    keys=set(report)
    return PLAN_KEYS.issubset(keys)


def validate_plan_contract(plan):
    if not isinstance(plan,dict) or set(plan) not in (PLAN_KEYS,PLAN_KEYS|PLAN_IDENTITY_KEYS):
        raise ValueError('invalid migration plan contract')
    if PLAN_IDENTITY_KEYS.issubset(plan) and (plan['session_user']!=PLAN_SESSION_USER or plan['current_user']!=PLAN_CURRENT_USER):
        raise ValueError('plan database identities differ from the reviewed migrator contract')


def check_plan(manifest,plan,postcheck=False):
    validate(manifest)
    validate_plan_contract(plan)
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
