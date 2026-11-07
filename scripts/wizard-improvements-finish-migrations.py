#!/usr/bin/env python3
"""Apply the reviewed V82/V83 set under a temporary schema CREATE grant."""
from __future__ import annotations
import argparse, hashlib, json, os, re, signal, stat, subprocess, sys
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
TARGET='espacigo_wizard_20261106'
PRIVATE=ROOT/'.local/testing/wizard-improvements'
ADMIN=PRIVATE/'postgres/pgpass'
MIGRATOR=PRIVATE/'espacigo_migrator_login.pgpass'
MANIFEST=PRIVATE/'reviewed-full-83-manifest.json'
BOOT=ROOT/'db/bootstrap/wizard-improvements'
MIGRATIONS=ROOT/'db/migrations'
HEX=re.compile(r'^[0-9a-f]{64}$')

def safe_file(path, mode):
    if path.is_symlink() or not path.is_file(): raise ValueError('required private file missing or symlinked')
    s=path.stat()
    if s.st_uid!=os.getuid() or stat.S_IMODE(s.st_mode)!=mode: raise ValueError('private file ownership/mode mismatch')

def safe_dirs():
    for p in (ROOT/'.local', ROOT/'.local/testing', PRIVATE, PRIVATE/'postgres'):
        if p.is_symlink() or not p.is_dir(): raise ValueError('private directory missing or symlinked')
        s=p.stat()
        if s.st_uid!=os.getuid() or stat.S_IMODE(s.st_mode)!=0o700: raise ValueError('private directory ownership/mode mismatch')

def clean_env(passfile, database_url=None):
    env={k:os.environ[k] for k in ('PATH','HOME','TMPDIR') if k in os.environ}
    env['PGPASSFILE']=str(passfile)
    if database_url is not None:
        env['DATABASE_URL']=database_url
        env['DATABASE_SET_ROLE']='espacigo_schema_owner'
        env['DATABASE_CONNECTION_MODE']='direct'
    return env

def run(argv, env, timeout):
    proc=subprocess.Popen(argv,cwd=ROOT,env=env,text=True,stdout=subprocess.PIPE,stderr=subprocess.PIPE,start_new_session=True)
    try: out,err=proc.communicate(timeout=timeout); return subprocess.CompletedProcess(argv,proc.returncode,out,err)
    except (subprocess.TimeoutExpired, KeyboardInterrupt):
        try: os.killpg(proc.pid,signal.SIGTERM)
        except ProcessLookupError: pass
        try: proc.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            try: os.killpg(proc.pid,signal.SIGKILL)
            except ProcessLookupError: pass
            proc.communicate()
        raise

def assert_plan(runner,binary,env,phase,expected):
    r=runner([binary,'-plan-only','-dir',str(MIGRATIONS)],env,120)
    if r.returncode: raise RuntimeError(f'{phase} read-only plan failed (exit {r.returncode})')
    try: d=json.loads(r.stdout)
    except Exception: raise RuntimeError(f'{phase} plan was not valid JSON') from None
    applied=d.get('applied'); pending=d.get('pending')
    if not isinstance(applied,list): raise RuntimeError(f'{phase} plan has invalid applied ledger')
    if d.get('database')!=TARGET or d.get('read_only') is not True or d.get('history_exists') is not True: raise RuntimeError(f'{phase} plan target/read-only/history is not exact')
    versions=[x.get('version') for x in applied if isinstance(x,dict)]
    if versions!=list(range(1,len(applied)+1)): raise RuntimeError(f'{phase} applied ledger is not contiguous')
    def matches(rows, entries):
        return all(isinstance(row,dict) and row.get('version')==entry['version'] and row.get('name')==entry['file'] and row.get('checksum')==entry['sha256'] for row,entry in zip(rows,entries)) and len(rows)==len(entries)
    if phase=='before':
        if len(applied)!=81 or not matches(applied,expected[:81]) or not isinstance(pending,list) or not matches(pending,expected[81:]): raise RuntimeError('preflight ledger/pending inventory is not exact V1..V81 with V82/V83 pending')
    elif len(applied)!=83 or not matches(applied,expected) or pending not in (None,[]): raise RuntimeError('postflight ledger is not exact V1..V83 or still has pending migrations')

def main(argv=None, runner=run):
    p=argparse.ArgumentParser(description=__doc__); p.add_argument('--confirm',required=True); p.add_argument('--migrator-binary',required=True); p.add_argument('--migrator-sha256',required=True); a=p.parse_args(argv)
    attempted=False; main_error=None; cleanup_error=None; old_handlers={}
    try:
        if a.confirm!=TARGET: raise ValueError('confirmation must exactly match the authorized target')
        if not HEX.fullmatch(a.migrator_sha256): raise ValueError('migrator SHA-256 must be 64 lowercase hex characters')
        b=Path(a.migrator_binary)
        if not b.is_absolute() or b.is_symlink() or not b.is_file() or b.stat().st_uid!=os.getuid() or not os.access(b,os.X_OK): raise ValueError('migrator must be an owned absolute executable regular file')
        if hashlib.sha256(b.read_bytes()).hexdigest()!=a.migrator_sha256: raise ValueError('migrator binary digest mismatch')
        safe_dirs()
        for f in (ADMIN,MIGRATOR,MANIFEST): safe_file(f,0o600)
        manifest=json.loads(MANIFEST.read_text())
        files=sorted(MIGRATIONS.glob('V*.sql'))
        expected=manifest
        if not isinstance(expected,list) or len(expected)!=83 or len(files)!=83: raise ValueError('review manifest/source inventory must contain exactly V1..V83')
        for i,(entry,path) in enumerate(zip(expected,files),1):
            if entry!={'version':i,'file':path.name,'sha256':hashlib.sha256(path.read_bytes()).hexdigest()} or path.name[:7]!=f'V{i:06d}': raise ValueError('reviewed migration inventory mismatch')
        adminenv=clean_env(ADMIN)
        dburl='postgres://espacigo_migrator_login@127.0.0.1:15436/espacigo_wizard_20261106?sslmode=disable'
        migenv=clean_env(MIGRATOR,dburl)
        # Plan through migrator's read-only mode; output remains in memory only.
        def call(cmd,env,timeout): return runner(cmd,env,timeout)
        # Dependency-injected runner supports mocked end-to-end tests.
        assert_plan(call,str(b),migenv,'before',expected)
        def interrupt_handler(_signum,_frame): raise KeyboardInterrupt()
        for sig in (signal.SIGINT,signal.SIGTERM): old_handlers[sig]=signal.signal(sig,interrupt_handler)
        attempted=True
        grant=call(['psql','-X','-v','ON_ERROR_STOP=1','-h','127.0.0.1','-p','15436','-U','postgres','-d',TARGET,'-w','-f',str(BOOT/'005_allow_v82_v83_create.sql')],adminenv,30)
        if grant.returncode: raise RuntimeError(f'temporary permission grant failed (exit {grant.returncode})')
        apply=call([str(b),'-dir',str(MIGRATIONS)],migenv,120)
        if apply.returncode: raise RuntimeError(f'migrator failed (exit {apply.returncode})')
        assert_plan(call,str(b),migenv,'after',expected)
    except (Exception, KeyboardInterrupt, SystemExit) as exc:
        main_error=f'{type(exc).__name__}: {exc}' if isinstance(exc,Exception) else 'interrupted'
    finally:
        if attempted:
            try:
                signal.signal(signal.SIGINT,signal.SIG_IGN); signal.signal(signal.SIGTERM,signal.SIG_IGN)
                safe_dirs(); safe_file(ADMIN,0o600)
                r=runner(['psql','-X','-v','ON_ERROR_STOP=1','-h','127.0.0.1','-p','15436','-U','postgres','-d',TARGET,'-w','-f',str(BOOT/'006_revoke_v82_v83_create.sql')],clean_env(ADMIN),30)
                if r.returncode: cleanup_error=f'revocation failed (exit {r.returncode})'
            except BaseException: cleanup_error='revocation failed (exception/timeout)'
        for sig,handler in locals().get('old_handlers',{}).items():
            try: signal.signal(sig,handler)
            except BaseException: pass
    if main_error or cleanup_error:
        print('; '.join(x for x in (main_error,cleanup_error) if x),file=sys.stderr); return 1
    print('V82/V83 applied; read-only plan has no pending migrations; temporary CREATE revoked')
    return 0


if __name__=='__main__': raise SystemExit(main())
