#!/usr/bin/env bash
set -Eeuo pipefail

# Local-only demonstration of an already-migrated PostgreSQL baseline.
# Creates two disposable, loopback-only containers with no mounted volumes.
# It never reads or connects to Cloud SQL.

if [[ "${RUN_LOCAL_BASELINE_DEMO:-}" != "YES" ]]; then
  echo 'Set RUN_LOCAL_BASELINE_DEMO=YES to create and destroy two local disposable PostgreSQL containers.' >&2
  exit 2
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"
IMAGE="postgis/postgis:18-3.6"
EXPECTED_IMAGE_ID="sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677"
IMAGE_ID="$(docker image inspect "$IMAGE" --format '{{.Id}}')"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned local PostGIS image ID mismatch.' >&2; exit 1; }

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-$$"
PREFIX="espaciGo-gcp03-baseline-${RUN_ID}"
NETWORK="${PREFIX}-net"
SOURCE="${PREFIX}-source"
TARGET="${PREFIX}-target"
EVIDENCE_DIR="$ROOT_DIR/.local/planning/gcp/gcp03-v60-baseline-demo-${RUN_ID}"
umask 077
mkdir -p "$EVIDENCE_DIR"
chmod 700 "$EVIDENCE_DIR"

cleanup_best_effort() {
  docker rm -fv "$SOURCE" "$TARGET" >/dev/null 2>&1 || true
  docker network rm "$NETWORK" >/dev/null 2>&1 || true
}
cleanup_verified() {
  docker rm -fv "$SOURCE" "$TARGET" >/dev/null
  docker network rm "$NETWORK" >/dev/null
  if docker container inspect "$SOURCE" "$TARGET" >/dev/null 2>&1; then
    echo 'Disposable baseline containers remain after cleanup.' >&2
    return 1
  fi
  if docker network inspect "$NETWORK" >/dev/null 2>&1; then
    echo 'Disposable baseline network remains after cleanup.' >&2
    return 1
  fi
}
trap cleanup_best_effort EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

docker network create "$NETWORK" >/dev/null
for container in "$SOURCE" "$TARGET"; do
  docker run -d --name "$container" --network "$NETWORK" \
    -p 127.0.0.1::5432 \
    --tmpfs /var/lib/postgresql:rw,noexec,nosuid,size=1g \
    -e POSTGRES_HOST_AUTH_METHOD=trust \
    "$IMAGE" >/dev/null
  docker inspect "$container" | jq -e \
    '.[0].HostConfig.Tmpfs | has("/var/lib/postgresql")' \
    >/dev/null
  docker inspect "$container" | jq -e '.[0].Mounts | all(.Type != "volume")' >/dev/null
  for _ in $(seq 1 60); do
    if docker exec "$container" psql -U postgres -d postgres -Atqc 'SELECT 1' >/dev/null 2>&1; then
      sleep 1
      if docker exec "$container" psql -U postgres -d postgres -Atqc 'SELECT 1' >/dev/null 2>&1; then break; fi
    fi
    sleep 1
  done
  docker exec "$container" psql -U postgres -d postgres -Atqc 'SELECT 1' >/dev/null
done

port_for() { docker port "$1" 5432/tcp | sed 's/.*://'; }
SOURCE_PORT="$(port_for "$SOURCE")"
TARGET_PORT="$(port_for "$TARGET")"
SOURCE_URL="postgres://postgres@127.0.0.1:${SOURCE_PORT}/espacigo_scratch?sslmode=disable"
TARGET_ADMIN_URL="postgres://postgres@127.0.0.1:${TARGET_PORT}/postgres?sslmode=disable"
TARGET_DB_URL="postgres://postgres@127.0.0.1:${TARGET_PORT}/espacigo_baseline?sslmode=disable"

for _ in $(seq 1 30); do
  if docker exec "$SOURCE" createdb -U postgres espacigo_scratch >/dev/null 2>&1; then break; fi
  sleep 1
done
docker exec "$SOURCE" psql -U postgres -d espacigo_scratch -Atqc 'SELECT 1' >/dev/null
DATABASE_URL="$SOURCE_URL" go run ./cmd/dbmigrate -dir "$ROOT_DIR/db/migrations" \
  >"$EVIDENCE_DIR/source-migrations.log" 2>&1

psql "$SOURCE_URL" -X -v ON_ERROR_STOP=1 -AtF $'\t' \
  -c 'SELECT version,name,checksum,runner_checksum,applied_at FROM public.schema_migrations ORDER BY version' \
  >"$EVIDENCE_DIR/source-history.tsv"
psql "$SOURCE_URL" -X -v ON_ERROR_STOP=1 -AtF $'\t' \
  -c "SELECT extname,extversion FROM pg_extension WHERE extname IN ('postgis','btree_gist','unaccent') ORDER BY extname" \
  >"$EVIDENCE_DIR/source-extensions.tsv"

pg_dump --format=custom --no-owner --no-acl "$SOURCE_URL" \
  --file "$EVIDENCE_DIR/espacigo-baseline.dump"
sha256sum "$EVIDENCE_DIR/espacigo-baseline.dump" >"$EVIDENCE_DIR/dump.sha256"
pg_restore --list "$EVIDENCE_DIR/espacigo-baseline.dump" >"$EVIDENCE_DIR/dump.toc"
awk '
  /^;/ { print; next }
  / (EXTENSION|COMMENT - EXTENSION) (- )?(btree_gist|postgis|unaccent)( |$)/ { print > omitted; next }
  / TABLE DATA public spatial_ref_sys / { print > omitted; next }
  { print }
' omitted="$EVIDENCE_DIR/omitted-extension-toc.txt" \
  "$EVIDENCE_DIR/dump.toc" >"$EVIDENCE_DIR/restore.toc"
[[ "$(wc -l <"$EVIDENCE_DIR/omitted-extension-toc.txt")" -eq 7 ]]

# Cluster roles and comments are separate bootstrap inputs: database-only
# pg_dump intentionally does not carry them. Create least-privilege roles and
# the destination owner before restore; don't apply migration files to target.
psql "$TARGET_ADMIN_URL" -X -v ON_ERROR_STOP=1 <<'SQL'
CREATE ROLE espacigo_owner NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_runtime NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_gadm_bootstrap NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
COMMENT ON ROLE espacigo_gadm_bootstrap IS
  'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.';
CREATE DATABASE espacigo_baseline OWNER espacigo_owner;
SQL

# Extension installation is a separately governed bootstrap phase, not part
# of the application-role restore. Cloud SQL extension availability/ownership
# must still be validated independently before this proposal can be accepted.
for extension in postgis btree_gist unaccent; do
  psql "${TARGET_ADMIN_URL/postgres?sslmode=disable/espacigo_baseline?sslmode=disable}" \
    -X -v ON_ERROR_STOP=1 -c "CREATE EXTENSION $extension" >/dev/null
done

pg_restore --exit-on-error --no-owner --no-acl --role=espacigo_owner \
  --use-list "$EVIDENCE_DIR/restore.toc" \
  --dbname "$TARGET_DB_URL" "$EVIDENCE_DIR/espacigo-baseline.dump" \
  >"$EVIDENCE_DIR/restore.log" 2>&1

psql "$TARGET_DB_URL" -X -v ON_ERROR_STOP=1 -AtF $'\t' \
  -c 'SELECT version,name,checksum,runner_checksum,applied_at FROM public.schema_migrations ORDER BY version' \
  >"$EVIDENCE_DIR/target-history.tsv"
psql "$TARGET_DB_URL" -X -v ON_ERROR_STOP=1 -AtF $'\t' \
  -c "SELECT extname,extversion FROM pg_extension WHERE extname IN ('postgis','btree_gist','unaccent') ORDER BY extname" \
  >"$EVIDENCE_DIR/target-extensions.tsv"
diff -u "$EVIDENCE_DIR/source-history.tsv" "$EVIDENCE_DIR/target-history.tsv" \
  >"$EVIDENCE_DIR/history.diff"
diff -u "$EVIDENCE_DIR/source-extensions.tsv" "$EVIDENCE_DIR/target-extensions.tsv" \
  >"$EVIDENCE_DIR/extensions.diff"

[[ "$(wc -l <"$EVIDENCE_DIR/source-history.tsv")" -eq 61 ]]
psql "$TARGET_DB_URL" -X -v ON_ERROR_STOP=1 -At \
  -c "SELECT count(*) FROM pg_catalog.pg_auth_members WHERE roleid='espacigo_gadm_bootstrap'::regrole OR member='espacigo_gadm_bootstrap'::regrole" \
  >"$EVIDENCE_DIR/target-gadm-membership-count.txt"
[[ "$(cat "$EVIDENCE_DIR/target-gadm-membership-count.txt")" == "0" ]]
psql "$TARGET_DB_URL" -X -v ON_ERROR_STOP=1 -At \
  -c "SELECT shobj_description('espacigo_gadm_bootstrap'::regrole,'pg_authid')" \
  >"$EVIDENCE_DIR/target-gadm-comment.txt"
grep -Fxq 'NOLOGIN GADM local bootstrap group; temporary isolated login membership only.' \
  "$EVIDENCE_DIR/target-gadm-comment.txt"
psql "$TARGET_DB_URL" -X -v ON_ERROR_STOP=1 -At \
  -c "SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f') AND c.relname NOT IN ('spatial_ref_sys','geometry_columns','geography_columns') AND pg_get_userbyid(c.relowner) <> 'espacigo_owner'" \
  >"$EVIDENCE_DIR/target-public-objects-not-owner-count.txt"
[[ "$(cat "$EVIDENCE_DIR/target-public-objects-not-owner-count.txt")" == "0" ]]

cat >"$EVIDENCE_DIR/manifest.txt" <<EOF
scope=local disposable PostgreSQL 18/PostGIS baseline dump and restore demonstration
source_git_commit=$(git -C "$ROOT_DIR" rev-parse HEAD)
migration_files=$(python3 - "$ROOT_DIR/db/migrations" <<'PY'
from pathlib import Path
import sys

directory = Path(sys.argv[1])
print(sum(1 for version in range(1, 62) if len(list(directory.glob(f"V{version:06d}__*.sql"))) == 1))
PY
)
history_rows=$(wc -l <"$EVIDENCE_DIR/source-history.tsv")
image_id=$IMAGE_ID
source_container=$SOURCE
target_container=$TARGET
docker_network=$NETWORK
source_migrator_ran=yes
target_migrator_ran=no
target_v60_replayed=no
database_dump_flags=--format=custom --no-owner --no-acl
restore_flags=--exit-on-error --no-owner --no-acl --role=espacigo_owner (connected as local postgres administrator)
toc_exclusions=extension creation/comments for btree_gist,postgis,unaccent and extension-managed spatial_ref_sys data
history_comparison=identical version,name,checksum,runner_checksum,applied_at
extension_comparison=identical for postgis,btree_gist,unaccent
target_gadm_role_membership_count=0
target_gadm_role_comment=provisioned separately from database dump
target_public_objects_owner=espacigo_owner
cloud_sql_access=none
cloud_sql_sql=none
cloud_sql_data_read=none
cleanup=explicit Docker API inspection confirmed both exact containers and isolated network absent before PASS
limitations=local-only demonstration; no Cloud SQL managed-role compatibility, production data, or acceptance of a changed empty-database criterion
EOF
chmod 600 "$EVIDENCE_DIR"/*
cleanup_verified
trap - EXIT
printf 'PASS: migrated baseline restored locally; history and selected extensions match.\nEvidence: %s\n' "$EVIDENCE_DIR"
