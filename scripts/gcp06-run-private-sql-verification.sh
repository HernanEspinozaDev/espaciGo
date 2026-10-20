#!/usr/bin/env bash
set -Eeuo pipefail
umask 077

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
PROJECT='espacigo-testprod'
REGION='southamerica-west1'
INSTANCE='espacigo-gcp06-20261020-gcp06a-pg'
CONNECTION_NAME="${PROJECT}:${REGION}:${INSTANCE}"
DATABASE='espacigo_gcp06_candidate_20261020-gcp06a'
RUN_ID='20261020-gcp06a'

usage() {
  echo 'Usage: gcp06-run-private-sql-verification.sh --manifest PATH --sql-user LOGIN --output-dir PRIVATE_NEW_DIR' >&2
  exit 2
}

MANIFEST=''
SQL_USER=''
OUTPUT_DIR=''
while (($#)); do
  case "$1" in
    --manifest) (($# >= 2)) || usage; MANIFEST="$2"; shift 2 ;;
    --sql-user) (($# >= 2)) || usage; SQL_USER="$2"; shift 2 ;;
    --output-dir) (($# >= 2)) || usage; OUTPUT_DIR="$2"; shift 2 ;;
    *) usage ;;
  esac
done
[[ -n "$MANIFEST" && -n "$SQL_USER" && -n "$OUTPUT_DIR" ]] || usage
[[ -f "$MANIFEST" && ! -L "$MANIFEST" ]] || { echo 'Manifest file is unavailable.' >&2; exit 1; }
[[ "$(stat -c '%a' "$MANIFEST")" == 600 && "$(stat -c '%a' "$(dirname -- "$MANIFEST")")" == 700 ]] || {
  echo 'Manifest must be mode 0600 in a mode 0700 directory.' >&2; exit 1;
}
command -v gcloud >/dev/null && command -v cloud-sql-proxy >/dev/null && command -v psql >/dev/null && command -v openssl >/dev/null || {
  echo 'gcloud, cloud-sql-proxy, psql, and openssl are required.' >&2; exit 1;
}

mkdir -m 0700 -- "$OUTPUT_DIR" || { echo 'Output directory must be new.' >&2; exit 1; }
[[ "$(stat -c '%a' "$(dirname -- "$OUTPUT_DIR")")" == 700 ]] || { rmdir -- "$OUTPUT_DIR"; echo 'Output parent must be mode 0700.' >&2; exit 1; }
RESOURCE_JSON="$OUTPUT_DIR/resource-snapshot.json"
PROXY_LOG="$OUTPUT_DIR/.proxy.log"
PSQL_LOG="$OUTPUT_DIR/.psql.log"
GCLOUD_LOG="$OUTPUT_DIR/.gcloud.log"
ACTUAL="$OUTPUT_DIR/actual.json"
EVIDENCE="$OUTPUT_DIR/target-evidence.json"
REPORT="$OUTPUT_DIR/report.json"
PROXY_PID=''
cleanup() {
  local status=$?
  trap - EXIT
  if [[ -n "$PROXY_PID" ]]; then kill "$PROXY_PID" >/dev/null 2>&1 || true; wait "$PROXY_PID" >/dev/null 2>&1 || true; fi
  rm -f -- "$PROXY_LOG" "$PSQL_LOG" "$GCLOUD_LOG"
  exit "$status"
}
trap cleanup EXIT

# This runbook must only be used after separate approval of the exact Cloud read.
if ! gcloud sql instances describe "$INSTANCE" --project "$PROJECT" \
  --format='json(name,region,connectionName,databaseVersion,state)' > "$RESOURCE_JSON" 2> "$GCLOUD_LOG"; then
  rm -f -- "$RESOURCE_JSON"
  echo 'GCP06 target evidence acquisition failed; no query result is valid.' >&2
  exit 1
fi
chmod 0600 "$RESOURCE_JSON"
RESOURCE_OBSERVED_AT="$(date -u +%Y-%m-%dT%H:%M:%S.%6NZ)"
PORT="$(python3 - <<'PY'
import socket
s=socket.socket(); s.bind(('127.0.0.1',0)); print(s.getsockname()[1]); s.close()
PY
)"
NONCE="$(openssl rand -hex 32)"
cloud-sql-proxy --address 127.0.0.1 --port "$PORT" "$CONNECTION_NAME" > "$PROXY_LOG" 2>&1 &
PROXY_PID=$!
ready=0
for _ in $(seq 1 60); do
  kill -0 "$PROXY_PID" 2>/dev/null || break
  if (exec 3<>"/dev/tcp/127.0.0.1/$PORT") >/dev/null 2>&1; then ready=1; break; fi
  sleep 0.5
done
[[ "$ready" == 1 ]] || { echo 'Cloud SQL Auth Proxy did not become ready; stop and review privately.' >&2; exit 1; }

if ! PGSSLMODE=disable psql -h 127.0.0.1 -p "$PORT" -U "$SQL_USER" -d "$DATABASE" -W \
  -X -qAt -v ON_ERROR_STOP=1 -v "gcp06_challenge=$NONCE" \
  -f "$ROOT_DIR/planning/gcp/sql_candidates/gcp06_candidate_data_verify_v1_v64.sql" > "$ACTUAL" 2> "$PSQL_LOG"; then
  SQLSTATE="$(python3 - "$PSQL_LOG" <<'PY'
import re,sys
text=open(sys.argv[1],encoding='utf-8',errors='replace').read()
match=re.search(r"SQL state:\\s*([0-9A-Z]{5})",text)
print(match.group(1) if match else "UNKNOWN")
PY
)"
  rm -f -- "$ACTUAL"
  echo "GCP06-ROWS-001 STOP SQLSTATE=$SQLSTATE; no role/grant change or retry was attempted." >&2
  exit 1
fi
chmod 0600 "$ACTUAL"

python3 - "$RESOURCE_JSON" "$ACTUAL" "$EVIDENCE" "$CONNECTION_NAME" "$NONCE" "$RESOURCE_OBSERVED_AT" <<'PY'
import hashlib,json,os,sys
from pathlib import Path
resource_path,actual_path,evidence_path,connection_name,nonce,resource_observed_at=sys.argv[1:]
resource_raw=Path(resource_path).read_bytes()
resource=json.loads(resource_raw)
actual_raw=Path(actual_path).read_bytes()
actual=json.loads(actual_raw)
if actual.get('challenge_nonce') != nonce:
    raise SystemExit('SQL challenge mismatch; evidence not written')
evidence={
  'transport':'cloud_sql_auth_proxy',
  'proxy_connection_name':connection_name,
  'resource_snapshot_sha256':hashlib.sha256(resource_raw).hexdigest(),
  'resource_observed_at':resource_observed_at,
  'challenge_nonce':nonce,
  'query_output_sha256':hashlib.sha256(actual_raw).hexdigest(),
  'session_user':actual.get('session_user'),
  'observed_at':actual.get('observed_at')}
with open(evidence_path,'x',encoding='utf-8') as out:
    json.dump(evidence,out,sort_keys=True)
    out.write('\n')
os.chmod(evidence_path,0o600)
PY

if ! python3 "$ROOT_DIR/scripts/gcp06_candidate_verify_manifest.py" \
  --manifest "$MANIFEST" --actual "$ACTUAL" --target-evidence "$EVIDENCE" \
  --resource-snapshot "$RESOURCE_JSON" > "$REPORT"; then
  chmod 0600 "$REPORT"
  cat "$REPORT"
  exit 1
fi
chmod 0600 "$REPORT"
cat "$REPORT"
