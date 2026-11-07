#!/usr/bin/env python3
"""Dedicated Cloud Build Job provisioner; never executes a Job or reads SQL."""
import argparse
import hashlib
import re
import json
from pathlib import Path
from release_jobs import create_args
from migration_release_manifest import validate
from rollout_web import TRIGGER
from verify_web_backend import Cloud,SA,require,GuardError


def execute(cloud,r):
    require(set(r)=={'format','manifest','build_id','mode','preflight'} and r['format']=='espacigo.job-request.v1','unsupported provisioner request')
    validate(r['manifest'])
    parent=cloud.json('builds','describe',r['build_id'])
    require(parent.get('buildTriggerId')==TRIGGER and parent.get('serviceAccount')==SA and parent.get('status')=='WORKING','provisioner requires the live canonical parent build')
    require(parent.get('substitutions',{}).get('COMMIT_SHA')==r['manifest']['source_sha'],'provisioner parent SHA differs')
    args=create_args(r)
    # Create only. Existing/replaced Jobs are never updated or executed here.
    cloud.run(*(a for a in args[1:] if not a.startswith(('--project=','--region='))))


def main():
    p=argparse.ArgumentParser()
    for name in ['request-uri','request-generation','request-sha256']:p.add_argument('--'+name,required=True)
    a=p.parse_args()
    try:
        require(re.fullmatch(r'gs://espacigo-testprod-cloud-build-447373903153/releases/[a-f0-9-]{36}/job-request-(plan|apply)\.json',a.request_uri) is not None and re.fullmatch('[1-9][0-9]*',a.request_generation) is not None,'request resource differs')
        text=Cloud().run('storage','cat',a.request_uri+'#'+a.request_generation)
        require(hashlib.sha256(text.encode()).hexdigest()==a.request_sha256,'immutable request digest differs')
        execute(Cloud(),json.loads(text))
    except (ValueError,KeyError,TypeError,OSError) as e:p.exit(1,'Job provisioner rejected: '+str(e)+'\n')


if __name__=='__main__':main()
