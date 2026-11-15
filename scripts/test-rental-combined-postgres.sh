#!/usr/bin/env bash
# Own networkless, disposable fixture only; never accepts an external DSN.
set -euo pipefail
ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
NAME="espacigo-rental-combined-ci-$$"
TMP_DIR="$(mktemp -d "${TMPDIR:?set TMPDIR to your scratch directory}/espacigo-rental-combined-ci.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
OWNED_CONTAINER_ID=''
OWNER_UID="$(id -u)"
OWNER_GID="$(id -g)"
mkdir -m 0777 "$SOCKET_DIR"
cleanup() {
  status=$?
  trap - EXIT INT TERM
  if [[ -n "$OWNED_CONTAINER_ID" ]]; then
    if [[ "$(docker inspect --format '{{.State.Running}}' "$OWNED_CONTAINER_ID")" == true ]]; then
      if ! docker exec --user 0 "$OWNED_CONTAINER_ID" chown "$OWNER_UID:$OWNER_GID" /var/run/postgresql; then
        echo 'failed to restore ownership of own socket directory' >&2
        exit 1
      fi
    fi
    if ! docker rm -f "$OWNED_CONTAINER_ID" >/dev/null; then
      echo 'failed to remove own combined-fixture container; preserving socket directory' >&2
      exit 1
    fi
    if ! python3 - "$OWNED_CONTAINER_ID" <<'PY'
import subprocess,sys
r=subprocess.run(['docker','inspect',sys.argv[1]],capture_output=True,text=True)
assert r.returncode!=0 and r.stderr.strip().lower()==('error: no such object: '+sys.argv[1]), 'owned container absence not verified'
print('PASS: owned combined-fixture container removed; exact-ID absence verified')
PY
    then exit 1; fi
  fi
  python3 -c 'import pathlib,shutil,sys; p=pathlib.Path(sys.argv[1]); shutil.rmtree(p); assert not p.exists(); print("PASS: own scratch directory removed and absence verified")' "$TMP_DIR"
  exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if docker inspect "$NAME" >/dev/null 2>&1; then
  echo 'refusing to reuse an existing combined-fixture container' >&2
  exit 1
fi
OWNED_CONTAINER_ID="$(docker create --name "$NAME" --network none \
  --tmpfs /var/lib/postgresql:rw,size=2g \
  --tmpfs /docker-entrypoint-initdb.d:rw,noexec,nosuid,size=1m \
  --mount "type=bind,src=$SOCKET_DIR,dst=/var/run/postgresql" \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE")"
python3 - "$TMP_DIR/container-receipt.json" "$OWNED_CONTAINER_ID" "$NAME" <<'PY'
import json,os,sys
fd=os.open(sys.argv[1],os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
with os.fdopen(fd,'w') as f:json.dump({'id':sys.argv[2],'name':sys.argv[3]},f)
PY
docker start "$OWNED_CONTAINER_ID" >/dev/null
ready=false
for _ in $(seq 1 160); do
  ready_count="$(docker logs "$NAME" 2>&1 | grep -cF 'database system is ready to accept connections' || true)"
  if [[ "${ready_count:-0}" -ge 2 ]] && [[ -S "$SOCKET_DIR/.s.PGSQL.5432" ]] && docker exec "$NAME" pg_isready -U postgres -d postgres >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 0.25
done
[[ "$ready" == true ]] || { docker logs --tail=30 "$NAME" >&2; exit 1; }
python3 - "$SOCKET_DIR/.espacigo-migration-ci" <<'PY'
import os,secrets,sys
fd=os.open(sys.argv[1],os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
with os.fdopen(fd,'w') as f:f.write(secrets.token_hex(32))
PY
python3 - "$OWNED_CONTAINER_ID" <<'PY'
import json,subprocess,sys
c=json.loads(subprocess.check_output(['docker','inspect',sys.argv[1]],text=True))[0]
assert c['HostConfig']['NetworkMode']=='none'
assert not c['HostConfig'].get('PortBindings')
assert not any(m['Type']=='volume' for m in c['Mounts'])
PY
env -u DATABASE_URL -u DATABASE_SET_ROLE -u PGPASSWORD -u PGPASSFILE -u PGOPTIONS -u PGSERVICE -u PGSERVICEFILE \
  TEST_MIGRATOR_CI_ISOLATED=1 \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR&sslmode=disable" \
  go test -race -json -count=1 -timeout=3m -run '^TestRentalCombinedFixtureOptIn$' ./internal/migrator > "$TMP_DIR/results.jsonl" 2>&1 || {
    python3 -c 'from pathlib import Path; import sys; print(Path(sys.argv[1]).read_text())' "$TMP_DIR/results.jsonl" >&2
    exit 1
  }
python3 - "$TMP_DIR/results.jsonl" <<'PY'
import json,sys
rows=[json.loads(line) for line in open(sys.argv[1]) if line.startswith('{')]
root='TestRentalCombinedFixtureOptIn'
required={root,root+'/v97-without-authority',root+'/v97-upgrade',root+'/commercial-quote-persistence',root+'/commercial-quote-persistence/retention-purge-publication-lock',root+'/commercial-quote-persistence/admin-local-account-block',root+'/commercial-quote-persistence/authentication-account-lock',root+'/contract-identity-reader'}
required.update(root+'/commercial-quote-persistence/authentication-account-lock/'+name for name in ('email','verification-id','action-token-id','session-hash'))
required.update(root+f'/authority-wire-v{version}' for version in (84,85,86,87,88,95,96))
required.update(root+'/v97-upgrade/'+name for name in (
 'failure_cleanup','migration_failure_cleanup','rejects_public_table_grant',
 'rejects_public_column_grant','rejects_other_table_grantee',
 'rejects_runtime_membership_drift','rejects_runtime_grant_option'))
passed={r.get('Test') for r in rows if r.get('Action')=='pass'}
assert required<=passed, 'missing required combined fixture results: '+repr(sorted(required-passed))
assert not any(r.get('Action') in ('fail','skip') for r in rows), 'failed or skipped combined fixture'
print(f'PASS: {len(required)} required checkpoints, no failed/skipped results; combined V1-V96 schema, V97 authority rejection/upgrade, quotes, retention-purge/publication serialization, admin-local block/account-lock and four authentication lookup/account-lock paths, plus identity ACL preservation; cleanup checked separately on exit.')
PY
