#!/usr/bin/env python3
"""Apply the versioned supplemental release roles; no SQL/trigger/instance writes.

Ownership: this script and infra/releases role files, independent of Terraform's
existing deployer role. Never edits that role or reads a secret payload.
"""
import argparse
import json
from pathlib import Path
import subprocess
from verify_web_backend import PROJECT,REGION,BUCKET,GuardError,require

ROOT=Path(__file__).resolve().parents[2]
DEPLOYER=f'espacigo-cloud-build-deployer@{PROJECT}.iam.gserviceaccount.com'
BUILDER=f'espacigo-release-job-builder@{PROJECT}.iam.gserviceaccount.com'
MIGRATOR=f'espacigo-db-migrator@{PROJECT}.iam.gserviceaccount.com'


def commands():
    member='serviceAccount:'+BUILDER
    return [
        ['projects','add-iam-policy-binding',PROJECT,'--member=serviceAccount:'+DEPLOYER,'--role=projects/'+PROJECT+'/roles/espacigoMainReleaseExecutor','--condition=None'],
        ['projects','add-iam-policy-binding',PROJECT,'--member='+member,'--role=projects/'+PROJECT+'/roles/espacigoReleaseJobProvisioner','--condition=None'],
        ['projects','add-iam-policy-binding',PROJECT,'--member='+member,'--role=roles/logging.logWriter','--condition=None'],
        ['iam','service-accounts','add-iam-policy-binding',BUILDER,'--member=serviceAccount:'+DEPLOYER,'--role=roles/iam.serviceAccountUser'],
        ['iam','service-accounts','add-iam-policy-binding',MIGRATOR,'--member='+member,'--role=roles/iam.serviceAccountUser'],
        ['storage','buckets','add-iam-policy-binding','gs://'+BUCKET,'--member='+member,'--role=roles/storage.objectViewer',"--condition=expression=resource.name.startsWith('projects/_/buckets/"+BUCKET+"/objects/source/') || resource.name.startsWith('projects/_/buckets/"+BUCKET+"/objects/releases/'),title=flujo_release_source_requests"],
        ['artifacts','repositories','add-iam-policy-binding','espacigo-testprod','--location='+REGION,'--member='+member,'--role=roles/artifactregistry.reader'],
    ]


def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--apply',action='store_true');p.add_argument('--account',required=True);a=p.parse_args()
    base=['--project='+PROJECT,'--account='+a.account,'--quiet','--format=json']
    def call(args,allow_absent=False):
        r=subprocess.run(['gcloud',*args,*base],capture_output=True,text=True,timeout=60)
        if r.returncode:
            if allow_absent and any(v in r.stderr for v in ['NOT_FOUND','does not exist','not found']):return None
            raise GuardError(r.stderr[-1000:])
        return json.loads(r.stdout) if r.stdout.strip() else {}
    plan={'builder':BUILDER,'commands':commands(),'role_files':['infra/releases/main-executor-role.json','infra/releases/job-provisioner-role.json']}
    if not a.apply:print(json.dumps(plan,indent=2));return
    try:
        account=call(['iam','service-accounts','describe',BUILDER],True)
        if account is None:call(['iam','service-accounts','create','espacigo-release-job-builder','--display-name=EspaciGo FLUJO release Job provisioner'])
        for rid,file in [('espacigoMainReleaseExecutor','main-executor-role.json'),('espacigoReleaseJobProvisioner','job-provisioner-role.json')]:
            path=ROOT/'infra/releases'/file;desired=json.loads(path.read_text())
            existing=call(['iam','roles','describe',rid],True)
            if existing is None:call(['iam','roles','create',rid,'--file='+str(path)])
            else:
                require(existing.get('title')==desired['title'] and set(existing.get('includedPermissions',[]))==set(desired['includedPermissions']),'existing supplemental role differs; review before changing')
        for cmd in commands():call(cmd)
        roles={rid:call(['iam','roles','describe',rid]) for rid in ['espacigoMainReleaseExecutor','espacigoReleaseJobProvisioner']}
        print(json.dumps({'applied':True,'builder':BUILDER,'roles_verified':{k:v['includedPermissions'] for k,v in roles.items()},'limits':'No SQL grants, secret payload access, Job updates/overrides or trigger writes'},indent=2))
    except (GuardError,OSError,ValueError) as error:p.exit(1,'release identity bootstrap failed: '+str(error)+'\n')


if __name__=='__main__':main()
