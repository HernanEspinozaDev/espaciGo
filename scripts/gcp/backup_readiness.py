#!/usr/bin/env python3
"""Check a fresh native Cloud SQL backup snapshot; does not call cloud or SQL."""
import argparse
from datetime import datetime,timedelta,timezone
import json
from pathlib import Path
import re

PROJECT='espacigo-testprod'
INSTANCE='espacigo-testprod-db'
CONNECTION=PROJECT+':southamerica-west1:'+INSTANCE


def timestamp(value):
    try:t=datetime.fromisoformat(value.replace('Z','+00:00'))
    except (AttributeError,ValueError):raise ValueError('invalid backup timestamp') from None
    if t.tzinfo is None:raise ValueError('backup timestamp must include timezone')
    return t


def check(snapshot,pending=False,now=None):
    now=now or datetime.now(timezone.utc)
    captured=timestamp(snapshot.get('captured_at'))
    if not -120<=(now-captured).total_seconds()<=300:raise ValueError('backup metadata must have been captured within five minutes')
    instance=snapshot.get('instance',{})
    if instance.get('project')!=PROJECT or instance.get('name')!=INSTANCE or instance.get('connectionName')!=CONNECTION or instance.get('region')!='southamerica-west1' or instance.get('state')!='RUNNABLE' or instance.get('databaseVersion')!='POSTGRES_18':raise ValueError('backup instance identity differs from development target')
    config=instance.get('settings',{}).get('backupConfiguration',{})
    retention=config.get('backupRetentionSettings',{})
    if config.get('enabled') is not True or retention.get('retentionUnit')!='COUNT' or type(retention.get('retainedBackups')) is not int or retention['retainedBackups']<7:raise ValueError('automatic backups must retain at least seven copies')
    if type(config.get('pointInTimeRecoveryEnabled')) is not bool:raise ValueError('PITR state must be explicitly observed')
    backup=snapshot.get('backup',{})
    bid=str(backup.get('id',''))
    if not re.fullmatch('[1-9][0-9]*',bid) or backup.get('instance')!=INSTANCE or backup.get('status')!='SUCCESSFUL' or backup.get('type') not in ['AUTOMATED','ON_DEMAND'] or backup.get('error') or backup.get('instanceDeletionTime'):raise ValueError('successful backup of the current instance required')
    expected=f'https://sqladmin.googleapis.com/sql/v1beta4/projects/{PROJECT}/instances/{INSTANCE}/backupRuns/{bid}'
    if backup.get('selfLink')!=expected:raise ValueError('backup resource belongs to another project or instance')
    start=timestamp(backup.get('startTime'));end=timestamp(backup.get('endTime'))
    if not start<=end<=captured+timedelta(seconds=120) or not -120<=(now-start).total_seconds()<=24*3600:raise ValueError('backup must have completed and started within the last 24 hours')
    if pending:
        lock=timestamp(snapshot.get('release_lock_acquired_at'))
        if not lock<=start or lock>captured or backup['type']!='ON_DEMAND':raise ValueError('pending SQL requires an on-demand backup started after acquiring the release lock')
    return {'format':'espacigo.backup-readiness.v1','checked_at':now.isoformat(),'captured_at':snapshot['captured_at'],'project':PROJECT,'instance':INSTANCE,'backup_id':bid,'backup_start':backup['startTime'],'backup_end':backup['endTime'],'backup_type':backup['type'],'retained_copies':retention['retainedBackups'],'pitr_enabled':config['pointInTimeRecoveryEnabled'],'pending_sql':pending,'policy':'initial-development-24h-seven-copies; pending SQL requires locked on-demand backup','limits':['Metadata validation does not prove capture authenticity or business RPO/RTO','Native backup restore rehearsal is recorded separately; PITR recovery is not inferred']}


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('snapshot',type=Path);p.add_argument('--pending-sql',action='store_true');a=p.parse_args()
    try:r=check(json.loads(a.snapshot.read_text()),a.pending_sql)
    except (ValueError,TypeError,OSError) as error:p.exit(1,'backup gate rejected: '+str(error)+'\n')
    print(json.dumps(r,indent=2))


if __name__=='__main__':main()
