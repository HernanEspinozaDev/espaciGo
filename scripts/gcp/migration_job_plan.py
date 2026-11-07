#!/usr/bin/env python3
"""Render a reviewable, read-only migration Job proposal; never runs gcloud."""
import argparse
import json
import re

PROJECT = 'espacigo-testprod'
REGION = 'southamerica-west1'
REGISTRY = f'{REGION}-docker.pkg.dev/{PROJECT}/{PROJECT}'
JOB = 'espacigo-db-plan'
SERVICE_ACCOUNT = f'espacigo-db-migrator@{PROJECT}.iam.gserviceaccount.com'
SECRET = 'espacigo-testprod-cloudsql-migrator-password'


def proposal(image, sha, secret_version):
    if not re.fullmatch(re.escape(REGISTRY) + r'/espacigo-db-migrator@sha256:[a-f0-9]{64}', image):
        raise ValueError('the migrator image must use the exact project/repository and an immutable digest')
    if not re.fullmatch('[a-f0-9]{40}', sha):
        raise ValueError('a full reviewed source SHA is required')
    if not re.fullmatch('[1-9][0-9]*', secret_version):
        raise ValueError('the secret must use a numeric enabled version, never latest')
    config = {
        'DATABASE_CONNECTION_MODE': 'cloudsql',
        'CLOUD_SQL_INSTANCE_CONNECTION_NAME': f'{PROJECT}:{REGION}:espacigo-testprod-db',
        'CLOUD_SQL_PRIVATE_IP': 'true',
        'DATABASE_URL': 'postgres://espacigo_migrator_login@localhost/espacigo_dms_stage_20261020?sslmode=disable',
        'DATABASE_SET_ROLE': 'espacigo_schema_owner',
        'DATABASE_PASSWORD_FILE': '/var/secrets/migration/password',
        'DBMIGRATE_TIMEOUT': '8m',
    }
    command = ['gcloud', 'run', 'jobs', 'create', JOB,
        f'--project={PROJECT}', f'--region={REGION}', f'--image={image}',
        f'--service-account={SERVICE_ACCOUNT}', '--tasks=1', '--parallelism=1',
        '--max-retries=0', '--task-timeout=600s', '--cpu=1', '--memory=512Mi',
        '--network=espacigo-testprod', '--subnet=espacigo-testprod-api-serverless',
        '--vpc-egress=private-ranges-only', '--command=/app/dbmigrate',
        '--args=-dir,/app/db/migrations,-plan-only',
        '--set-env-vars=^|^' + '|'.join(f'{k}={v}' for k,v in config.items()),
        f'--set-secrets=/var/secrets/migration/password={SECRET}:{secret_version}',
        f'--labels=source-commit={sha},purpose=flujo-migration-preflight']
    return {
        'format': 'espacigo.migration-job-proposal.v1',
        'executed': False,
        'mode': 'read-only-plan',
        'source_sha': sha,
        'prerequisites': [
            'Reviewed image digest/provenance matches source SHA and inventory',
            'Dedicated service account and least-privilege grants provisioned and verified',
            'Secret version enabled and runtime mount authorized; no payload read by builder',
            'No existing Job with this name; existing resources must be reviewed, not replaced',
        ],
        'proposed_job_create_argv': command,
        'proposed_execute_argv': ['gcloud','run','jobs','execute',JOB,f'--project={PROJECT}',f'--region={REGION}','--wait'],
        'iam_proposal': {
            'new_service_account': SERVICE_ACCOUNT,
            'runtime_project_role': 'roles/cloudsql.client',
            'runtime_sql_condition': "resource.name == 'projects/espacigo-testprod/instances/espacigo-testprod-db' && resource.type == 'sqladmin.googleapis.com/Instance'",
            'secret_accessor_only_on': f'projects/{PROJECT}/secrets/{SECRET}',
            'job_execute_permission': 'run.jobs.run',
            'execution_observer_permissions': ['run.jobs.get','run.executions.get','run.executions.list'],
            'excluded_from_routine_caller': ['run.jobs.runWithOverrides','secretmanager.versions.access','run.jobs.update'],
        },
        'limits': [
            'A proposal is not a deployment or proof of IAM/provenance/SQL authority',
            'This first Job only plans; an applying Job and release orchestration are still separate gates',
            'Cloud SQL Client is not PostgreSQL DDL authority and may have project-wide effective scope',
            'No SQL instance creation, backup-policy changes, trigger updates, or bastion deletion',
        ],
    }


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--image', required=True)
    parser.add_argument('--source-sha', required=True)
    parser.add_argument('--secret-version', required=True)
    args=parser.parse_args()
    try:
        result=proposal(args.image,args.source_sha,args.secret_version)
    except ValueError as error:
        parser.exit(2,f'proposal rejected: {error}\n')
    print(json.dumps(result,indent=2))


if __name__=='__main__':
    main()
