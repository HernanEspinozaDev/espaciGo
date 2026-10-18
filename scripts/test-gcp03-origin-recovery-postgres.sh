#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_OUTPUT="$ROOT_DIR/.local/testing/g03o-1017-recovery-20261018/fixture-e2e"
CONTAINER="espacigo-g03o-recovery-pg18-20261018"
IMAGE="postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677"
SOCKET_DIR="${G03O_RECOVERY_TEST_SOCKET:-}"

[[ -n "$SOCKET_DIR" && -d "$SOCKET_DIR" ]] || { echo "set G03O_RECOVERY_TEST_SOCKET to the fixture Unix-socket path" >&2; exit 2; }
docker inspect "$CONTAINER" >/dev/null 2>&1 || { echo "dedicated PostgreSQL 18 fixture container is required" >&2; exit 2; }
[[ "$(docker inspect "$CONTAINER" --format '{{.State.Status}}')" == "running" ]] || { echo "fixture must remain running; the harness never starts or removes it" >&2; exit 2; }
[[ "$(docker inspect "$CONTAINER" --format '{{.Config.Image}}')" == "$IMAGE" ]] || { echo "fixture image digest differs from the reviewed PG18 image" >&2; exit 2; }
[[ "$(docker inspect "$CONTAINER" --format '{{.HostConfig.NetworkMode}}')" == "none" ]] || { echo "fixture must have network mode none" >&2; exit 2; }
mkdir -p "$TEST_OUTPUT"
chmod 0700 "$TEST_OUTPUT"

cd "$ROOT_DIR"
env -u DATABASE_URL G03O_RECOVERY_TEST_SOCKET="$SOCKET_DIR" G03O_RECOVERY_TEST_OUTPUT="$TEST_OUTPUT" PYTHONPATH="$ROOT_DIR/scripts" \
  python3 -m unittest scripts.test_gcp03_origin_recovery_postgres -v
