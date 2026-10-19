#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
container_name="espacigo-gcp04-w8-pg18-$$"
cleanup() {
  docker rm -f "$container_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker run --rm -d --name "$container_name" --network none \
  -e POSTGRES_HOST_AUTH_METHOD=trust postgres:18-alpine >/dev/null
ready=0
for _ in $(seq 1 40); do
  if docker exec "$container_name" pg_isready -U postgres >/dev/null 2>&1; then
    ready=1
    break
  fi
  sleep 1
done
if [[ "$ready" != 1 ]]; then
  echo "isolated PostgreSQL 18 fixture did not become ready" >&2
  exit 1
fi

docker exec -i "$container_name" psql -X -v ON_ERROR_STOP=1 -U postgres -d postgres <<'SQL'
CREATE DATABASE espacigo;
CREATE ROLE espacigo_gcp04_w8_catalog_probe LOGIN INHERIT;
GRANT pg_read_all_settings TO espacigo_gcp04_w8_catalog_probe;
SQL

report="$(docker exec -i "$container_name" psql -X -qAt -v ON_ERROR_STOP=1 \
  -U espacigo_gcp04_w8_catalog_probe -d espacigo < "$repo_root/db/query/gcp04_w8_catalog_probe.sql")"
printf '%s' "$report" | python3 -c '
import json, sys
r=json.load(sys.stdin)
assert r["format"] == "espacigo.gcp04.w8.catalog-probe.v1"
assert r["target"]["database"] == "espacigo"
assert r["target"]["login"] == "espacigo_gcp04_w8_catalog_probe"
assert r["target"]["server_version_num"] >= 180000
assert r["target"]["transaction_read_only"] is True
assert r["scope"] == {"business_rows_read": False, "configuration_catalog_read": False, "statistics_read": False, "password_or_secret_read": False, "mutations": False}
roles={role["name"] for role in r["roles"]}
assert "pg_read_all_settings" in roles
assert any(m["granted_role"] == "pg_read_all_settings" and m["member_role"] == "espacigo_gcp04_w8_catalog_probe" for m in r["memberships"])
print("W8 PostgreSQL 18 isolated catalog probe passed")
'
