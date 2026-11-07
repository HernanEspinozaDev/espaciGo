#!/usr/bin/env python3
"""Inspect a failed release; unlock only a proven pre-application failure."""
import argparse
import json
import re
from release_jobs import create_args,request
from rollout_web import LOCK,TRIGGER
from verify_web_backend import Cloud,BUCKET,SA,UUID,GuardError,require


def absent(cloud,args,expected):
    try:return cloud.json(*args)
    except GuardError as error:
        require(expected in str(error) and any(x in str(error).lower() for x in ['not found','404','cannot find']),'absence is not proven')
        return None


def recover(cloud,unlock=False):
    meta=cloud.json('storage','objects','describe',LOCK);gen=str(meta['generation']);text=cloud.run('storage','cat',LOCK+'#'+gen);owner=json.loads(text)
    bid=owner.get('build_id','');require(re.fullmatch(UUID,bid) is not None,'legacy/unknown lock requires separate review')
    b=cloud.json('builds','describe',bid)
    require(b.get('id')==bid and b.get('buildTriggerId')==TRIGGER and b.get('serviceAccount')==SA and b.get('status') in ['FAILURE','CANCELLED','TIMEOUT','INTERNAL_ERROR','EXPIRED'],'only a terminal failed canonical release can be recovered')
    intent=f'gs://{BUCKET}/releases/{bid}/job-intent-apply.json'
    require(absent(cloud,['storage','objects','describe',intent],intent) is None,'application may have started; inspect exact execution/ledger and do not unlock automatically')
    name='espacigo-db-r-'+bid.replace('-','')
    job=absent(cloud,['run','jobs','describe',name],name)
    if job is not None:require(cloud.json('run','jobs','executions','list','--job='+name)==[],'an applying execution exists; outcome review required')
    result={'build_id':bid,'generation':gen,'outcome':'proven before migration submission','unlocked':False}
    if unlock:
        require(str(cloud.json('storage','objects','describe',LOCK)['generation'])==gen and cloud.run('storage','cat',LOCK+'#'+gen)==text,'lock changed during recovery')
        cloud.run('storage','rm',LOCK,'--if-generation-match='+gen);result['unlocked']=True
    return result


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--unlock-preapply',action='store_true');p.add_argument('--account');a=p.parse_args()
    try:print(json.dumps(recover(Cloud(a.account),a.unlock_preapply),indent=2))
    except (ValueError,OSError,KeyError) as e:p.exit(1,'release recovery refused: '+str(e)+'\n')


if __name__=='__main__':main()
