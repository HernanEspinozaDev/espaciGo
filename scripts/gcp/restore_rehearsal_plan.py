#!/usr/bin/env python3
"""Render the fixed, isolated recovery rehearsal; never executes commands."""
import argparse
import copy
import json
import re
from migration_job_plan import PROJECT,REGION,SERVICE_ACCOUNT,proposal as job_proposal

SOURCE='espacigo-testprod-db'
TARGET='espacigo-flujo04-restore-20261106'
JOB='espacigo-db-restore-plan'
LABELS={'purpose':'flujo-restore-rehearsal','cut':'flujo-04','owner':'codex-flujo-inicial'}


def proposal(backup_id,image,sha,secret_version):
    if not re.fullmatch('[1-9][0-9]*',backup_id):
        raise ValueError('a verified numeric successful backup ID is required')
    source_job=job_proposal(image,sha,secret_version)
    job=copy.deepcopy(source_job['proposed_job_create_argv'])
    job[4]=JOB
    job=[x.replace(f'{PROJECT}:{REGION}:{SOURCE}',f'{PROJECT}:{REGION}:{TARGET}') for x in job]
    common=[f'--project={PROJECT}','--quiet','--format=json']
    # The installed stable SDK does not expose create-time labels; beta does.
    create=['gcloud','beta','sql','instances','create',TARGET,'--database-version=POSTGRES_18','--edition=ENTERPRISE',f'--region={REGION}','--tier=db-g1-small','--availability-type=ZONAL','--storage-type=SSD','--storage-size=10','--no-storage-auto-increase','--no-assign-ip','--network=projects/espacigo-testprod/global/networks/espacigo-testprod','--no-backup','--no-deletion-protection','--labels='+','.join(f'{k}={v}' for k,v in LABELS.items()),'--async',*common]
    restore=['gcloud','sql','backups','restore',backup_id,'--backup-instance='+SOURCE,'--restore-instance='+TARGET,'--async',*common]
    return {'format':'espacigo.restore-rehearsal-proposal.v1','executed':False,'source':SOURCE,'target':TARGET,'job':JOB,'labels':LABELS,'backup_id':backup_id,'create_argv':create,'restore_argv':restore,'plan_job_create_argv':job,'cleanup_job_argv':['gcloud','run','jobs','delete',JOB,f'--region={REGION}',*common],'cleanup_instance_argv':['gcloud','sql','instances','delete',TARGET,'--async',*common],'client_role':'roles/cloudsql.client','client_member':'serviceAccount:'+SERVICE_ACCOUNT,'client_condition_expression':f"resource.name == 'projects/{PROJECT}/instances/{TARGET}' && resource.type == 'sqladmin.googleapis.com/Instance'",'limits':['Target must be absent before creation; never restore over any pre-existing instance','Before restoring/deleting, read back name, project, private-only network and all ownership labels','Restores managed backup inside Cloud SQL; no production DSN, data, passwords or hashes exported locally','Job remains plan-only; runtime credentials stay mounted from the existing secret','Temporary IAM condition must have an expiry and be removed with Job/instance after verification','No source-instance updates, application traffic changes or bastion use']}


def assert_owned_target(metadata):
    if metadata.get('name')!=TARGET or metadata.get('project')!=PROJECT:
        raise ValueError('refusing a resource outside the exact rehearsal target')
    settings=metadata.get('settings',{})
    if any(settings.get('userLabels',{}).get(k)!=v for k,v in LABELS.items()):
        raise ValueError('rehearsal ownership labels do not match')
    ip=settings.get('ipConfiguration',{})
    if ip.get('ipv4Enabled') is not False or ip.get('privateNetwork')!='projects/espacigo-testprod/global/networks/espacigo-testprod':
        raise ValueError('rehearsal must remain on the verified private network only')


def main():
    p=argparse.ArgumentParser(description=__doc__)
    for name in ['backup-id','image','source-sha','secret-version']:p.add_argument('--'+name,required=True)
    args=p.parse_args()
    try:r=proposal(args.backup_id,args.image,args.source_sha,args.secret_version)
    except ValueError as error:p.exit(2,f'proposal rejected: {error}\n')
    print(json.dumps(r,indent=2))


if __name__=='__main__':main()
