#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_W7_B1_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
CONTAINER="espacigo-gcp04-w7-b1-${RUN_ID}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-w7-b1.XXXXXX")"
chmod 0700 "$TMP_DIR"
cleanup(){ docker rm -f "$CONTAINER" >/dev/null 2>&1 || true; rm -rf -- "$TMP_DIR"; }
trap cleanup EXIT

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}' 2>/dev/null || true)"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL 18/PostGIS image unavailable; refusing pull or substitute.' >&2; exit 1; }
[[ -z "$(docker ps -aq --filter "name=^/${CONTAINER}$")" ]] || { echo 'Disposable B1 container name collision.' >&2; exit 1; }

docker run --rm -d --name "$CONTAINER" --network none \
  --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=1g \
  -e POSTGRES_HOST_AUTH_METHOD=trust "$IMAGE" postgres -c 'listen_addresses=' >/dev/null
ready=0
for _ in $(seq 1 40); do
  if docker exec "$CONTAINER" pg_isready -U postgres >/dev/null 2>&1; then ready=1; break; fi
  sleep 1
done
[[ "$ready" == 1 ]] || { echo 'Isolated PostgreSQL 18 fixture did not become ready.' >&2; exit 1; }

docker exec -i "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres <<'SQL'
CREATE DATABASE espacigo;
CREATE ROLE cloudsqlsuperuser NOLOGIN;
CREATE ROLE espacigo_gcp04_w7_bootstrap LOGIN CREATEROLE CREATEDB INHERIT;
GRANT cloudsqlsuperuser TO espacigo_gcp04_w7_bootstrap WITH ADMIN FALSE, INHERIT TRUE, SET FALSE;
SQL
docker exec -i "$CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d espacigo <<'SQL'
CREATE TABLE public.schema_migrations(version text PRIMARY KEY, dirty boolean NOT NULL DEFAULT false);
GRANT CONNECT ON DATABASE espacigo TO espacigo_gcp04_w7_bootstrap WITH GRANT OPTION;
GRANT USAGE ON SCHEMA public TO espacigo_gcp04_w7_bootstrap WITH GRANT OPTION;
GRANT SELECT ON TABLE public.schema_migrations TO espacigo_gcp04_w7_bootstrap WITH GRANT OPTION;
SQL
docker cp "$ROOT_DIR/db/query/gcp04_w7_b1_actor_probe.sql" "$CONTAINER:/tmp/gcp04_w7_b1_actor_probe.sql" >/dev/null
docker exec "$CONTAINER" psql -X -q -A -t -v ON_ERROR_STOP=1 -U espacigo_gcp04_w7_bootstrap -d espacigo -f /tmp/gcp04_w7_b1_actor_probe.sql > "$TMP_DIR/report.json"
python3 - "$TMP_DIR/report.json" <<'PY'
import json, sys
from pathlib import Path
r=json.loads(Path(sys.argv[1]).read_text())
assert r['format']=='espacigo.gcp04.w7.b1-actor-probe.v1'
assert r['target']['session_user']=='espacigo_gcp04_w7_bootstrap'
assert r['target']['current_user']=='espacigo_gcp04_w7_bootstrap'
assert r['target']['transaction_read_only'] is True
assert r['actor']['superuser'] is False
assert r['actor']['createdb'] is True and r['actor']['createrole'] is True and r['actor']['inherit'] is True
assert any(m['granted_role']=='cloudsqlsuperuser' and m['member_role']=='espacigo_gcp04_w7_bootstrap' for m in r['memberships'])
assert r['capabilities']['database_connect_grant_option'] is True
assert r['capabilities']['public_schema_usage_grant_option'] is True
assert r['capabilities']['ledger_select_grant_option'] is True
assert any(a['grantee']=='PUBLIC' and a['privilege_type']=='TEMPORARY' for a in r['public_acl'])
assert r['scope']=={'business_rows_read':False,'password_or_secret_read':False,'mutations':False}
PY

docker rm -f "$CONTAINER" >/dev/null
trap - EXIT
rm -rf -- "$TMP_DIR"
echo 'PASS: pinned PostgreSQL 18 B1 query reports actor attributes, membership, owner/grant-option matrix and PUBLIC ACL in READ ONLY; network none, tmpfs only.'
