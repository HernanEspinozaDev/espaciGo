#!/usr/bin/env python3
"""Read public Actions evidence for the exact main SHA; never deploys or writes."""
import argparse
from datetime import datetime,timezone
import json
import re
import ssl
import time
import urllib.error
import urllib.parse
import urllib.request

REPOSITORY='HernanEspinozaDev/espaciGo'
REPOSITORY_ID=1361448198
BASE_PATHS=frozenset({'.github/workflows/application-quality.yml','.github/workflows/frontend-quality.yml'})
DATABASE_PATH='.github/workflows/database-quality.yml'


class PendingQuality(Exception):
    pass


def evaluate(runs,sha,require_database=True,now=None):
    if not isinstance(sha,str) or not re.fullmatch('[a-f0-9]{40}',sha):
        raise ValueError('full source SHA required')
    now=now or datetime.now(timezone.utc)
    paths=BASE_PATHS|({DATABASE_PATH} if require_database else set())
    selected={}
    for run in runs:
        if not isinstance(run,dict):
            raise ValueError('quality evidence contains an invalid run')
        path=run.get('path')
        if not isinstance(path,str) or path not in paths or run.get('head_sha')!=sha or run.get('head_branch')!='main' or run.get('event')!='push':
            continue
        repo=run.get('repository') or {}
        head_repo=run.get('head_repository') or {}
        if any(r.get('id')!=REPOSITORY_ID or r.get('full_name')!=REPOSITORY for r in [repo,head_repo]):
            raise ValueError('quality evidence originates from a different repository')
        if type(run.get('id')) is not int or type(run.get('run_attempt')) is not int or run['id']<1 or run['run_attempt']<1:
            raise ValueError('quality run identity/attempt is missing')
        rank=(run['id'],run['run_attempt'])
        prior=selected.get(path)
        if prior is None or rank>(prior['id'],prior['run_attempt']):selected[path]=run
    pending=[]
    evidence=[]
    for path in sorted(paths):
        run=selected.get(path)
        if run is None or run.get('status')!='completed':
            pending.append(path)
            continue
        if run.get('conclusion')!='success':
            raise ValueError('required quality workflow did not succeed: '+path)
        try:updated=datetime.fromisoformat(run['updated_at'].replace('Z','+00:00'))
        except (ValueError,KeyError,AttributeError):raise ValueError('quality run timestamp is invalid') from None
        if updated.tzinfo is None or not -120 <= (now-updated).total_seconds() <= 24*3600:
            raise ValueError('quality evidence is stale or from the future')
        evidence.append({'workflow_path':path,'run_id':run['id'],'attempt':run['run_attempt'],'updated_at':run['updated_at']})
    if pending:raise PendingQuality('waiting for required quality workflows: '+', '.join(pending))
    return {'format':'espacigo.release-quality.v1','source_sha':sha,'branch':'main','repository':REPOSITORY,'verified_at':now.isoformat(),'runs':evidence}


def fetch_json(path):
    req=urllib.request.Request('https://api.github.com/repos/'+REPOSITORY+path,headers={'Accept':'application/vnd.github+json','X-GitHub-Api-Version':'2022-11-28','User-Agent':'espacigo-release-quality'})
    with urllib.request.urlopen(req,timeout=20) as response:return json.load(response)


def fetch_runs(sha):
    query=urllib.parse.urlencode({'head_sha':sha,'branch':'main','event':'push','per_page':100})
    data=fetch_json('/actions/runs?'+query)
    if not isinstance(data,dict) or not isinstance(data.get('workflow_runs'),list) or type(data.get('total_count')) is not int or not len(data['workflow_runs'])<=data['total_count']<=100:
        raise ValueError('quality API response is incomplete or requires pagination')
    return data['workflow_runs']


def verify_main_ref(ref,sha):
    if not isinstance(ref,dict) or ref.get('ref')!='refs/heads/main' or ref.get('object',{}).get('type')!='commit' or ref.get('object',{}).get('sha')!=sha:
        raise ValueError('candidate no longer matches the current remote main head')


def retryable_transport(error):
    if isinstance(error,urllib.error.HTTPError):return error.code in {408,429,500,502,503,504}
    if isinstance(error,urllib.error.URLError):return not isinstance(error.reason,ssl.SSLCertVerificationError)
    return isinstance(error,TimeoutError)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--sha',required=True)
    parser.add_argument('--scope',choices=['web','full'],default='full')
    parser.add_argument('--wait-seconds',type=int,default=0)
    args=parser.parse_args()
    if not re.fullmatch('[a-f0-9]{40}',args.sha) or not 0<=args.wait_seconds<=1200:
        parser.error('full SHA and a wait deadline between zero and 1200 seconds required')
    deadline=time.monotonic()+args.wait_seconds
    while True:
        try:
            result=evaluate(fetch_runs(args.sha),args.sha,args.scope=='full')
            verify_main_ref(fetch_json('/git/ref/heads/main'),args.sha)
            result['main_head_sha']=args.sha
            print(json.dumps(result,sort_keys=True))
            return
        except PendingQuality as error:
            if time.monotonic()>=deadline:parser.exit(3,str(error)+'\n')
            # One request per 45s limits anonymous public-API rate consumption.
            time.sleep(min(45,max(0,deadline-time.monotonic())))
        except (ValueError,OSError,urllib.error.URLError) as error:
            if retryable_transport(error) and time.monotonic()<deadline:
                print('Temporary quality API transport failure; retrying within the existing deadline: '+str(error),flush=True)
                time.sleep(min(30,max(0,deadline-time.monotonic())))
                continue
            parser.exit(1,'quality gate rejected: '+str(error)+'\n')


if __name__=='__main__':main()
