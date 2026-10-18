#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGE='postgis/postgis:18-3.6@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
EXPECTED_IMAGE_ID='sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677'
RUN_ID="${GCP04_REPLAY_RUN_ID:-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
if [[ ! "$RUN_ID" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]; then
  echo 'GCP04_REPLAY_RUN_ID must contain 1-64 safe filename characters.' >&2
  exit 2
fi
RUN_SLUG="${RUN_ID,,}"
RUN_SLUG="${RUN_SLUG//./-}"
PROJECT="espacigo-gcp04-replay-${RUN_SLUG}"
CONTAINER="${PROJECT}-postgres"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-gcp04-replay.XXXXXX")"
SOCKET_DIR="$TMP_DIR/socket"
COMPOSE_FILE="$TMP_DIR/compose.json"
RESOLVED_FILE="$TMP_DIR/resolved-compose.json"
mkdir -m 0777 "$SOCKET_DIR"
chmod 0700 "$TMP_DIR"

compose() {
  docker compose --project-directory "$ROOT_DIR" --project-name "$PROJECT" -f "$COMPOSE_FILE" "$@"
}
cleanup_best_effort() {
  compose down --volumes --remove-orphans >/dev/null 2>&1 || true
  rm -rf -- "$TMP_DIR"
}
cleanup_verified() {
  compose down --volumes --remove-orphans >/dev/null
  if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]]; then
    echo "Disposable Compose containers remain for project $PROJECT." >&2
    return 1
  fi
  if [[ -n "$(docker volume ls -q --filter "label=com.docker.compose.project=$PROJECT")" ]]; then
    echo "Disposable Compose volumes remain for project $PROJECT." >&2
    return 1
  fi
  rm -rf -- "$TMP_DIR"
}
trap cleanup_best_effort EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

IMAGE_ID="$(docker image inspect 'postgis/postgis:18-3.6' --format '{{.Id}}')"
[[ "$IMAGE_ID" == "$EXPECTED_IMAGE_ID" ]] || { echo 'Pinned PostgreSQL 18/PostGIS image ID mismatch; refusing to pull or substitute an image.' >&2; exit 1; }
if [[ -n "$(docker ps -aq --filter "label=com.docker.compose.project=$PROJECT")" ]]; then
  echo "Compose project already exists; refusing to reuse it: $PROJECT" >&2
  exit 1
fi

python3 - "$COMPOSE_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" "$RUN_ID" <<'PY'
import json
import sys
from pathlib import Path

compose_file, image, container, socket_dir, run_id = sys.argv[1:]
config = {
    "services": {
        "postgres": {
            "image": image,
            "container_name": container,
            "network_mode": "none",
            "tmpfs": ["/var/lib/postgresql:rw,noexec,nosuid,size=1g"],
            "volumes": [{
                "type": "bind",
                "source": socket_dir,
                "target": "/var/run/postgresql",
                "bind": {"create_host_path": False},
            }],
            "environment": {"POSTGRES_HOST_AUTH_METHOD": "trust"},
            "labels": {"com.espacigo.scope": "gcp04-replay-test", "com.espacigo.run": run_id},
        }
    }
}
Path(compose_file).write_text(json.dumps(config), encoding="utf-8")
PY

compose config --format json > "$RESOLVED_FILE"
python3 - "$RESOLVED_FILE" "$IMAGE" "$CONTAINER" "$SOCKET_DIR" <<'PY'
import json
import sys
from pathlib import Path

resolved = json.loads(Path(sys.argv[1]).read_text(encoding="utf-8"))
service = resolved["services"]["postgres"]
expected_image, expected_container, expected_socket = sys.argv[2:]
assert service["image"] == expected_image, service.get("image")
assert service["container_name"] == expected_container, service.get("container_name")
assert service["network_mode"] == "none", service.get("network_mode")
assert any(item.startswith("/var/lib/postgresql:") for item in service["tmpfs"]), service.get("tmpfs")
volumes = service["volumes"]
assert len(volumes) == 1 and volumes[0]["type"] == "bind", volumes
assert volumes[0]["source"] == expected_socket and volumes[0]["target"] == "/var/run/postgresql", volumes
assert not service.get("ports"), service.get("ports")
assert service["environment"]["POSTGRES_HOST_AUTH_METHOD"] == "trust"
print("Resolved GCP-04 test Compose: pinned PostgreSQL 18 image; no TCP ports/network; DB data in container tmpfs; one run-scoped Unix socket bind; no Docker volumes.")
PY

compose up -d postgres >/dev/null
CONTAINER_ID="$(compose ps -q postgres)"
[[ -n "$CONTAINER_ID" ]] || { echo 'Disposable PostgreSQL container did not start.' >&2; exit 1; }
docker inspect "$CONTAINER_ID" | python3 -c 'import json,sys; c=json.load(sys.stdin)[0]; mounts=c["Mounts"]; assert c["HostConfig"]["NetworkMode"] == "none"; assert "/var/lib/postgresql" in c["HostConfig"]["Tmpfs"]; assert all(m["Type"] == "bind" for m in mounts); assert not c["HostConfig"].get("PortBindings"); print("Runtime Compose container isolation verified.")'
for _ in $(seq 1 120); do
  if compose exec -T postgres pg_isready -U postgres -d postgres >/dev/null 2>&1 \
    && [[ "$(compose exec -T postgres psql -X -A -t -U postgres -d postgres -c 'SHOW listen_addresses' 2>/dev/null | tr -d '\r')" == '*' ]]; then
    break
  fi
  sleep 0.25
done
compose exec -T postgres pg_isready -U postgres -d postgres >/dev/null
[[ "$(compose exec -T postgres psql -X -A -t -U postgres -d postgres -c 'SHOW listen_addresses' | tr -d '\r')" == '*' ]] || {
  echo 'Disposable PostgreSQL did not reach the final server process after initialization.' >&2
  exit 1
}

cd "$ROOT_DIR"
set +e
timeout --signal=TERM --kill-after=10s 180s env -u DATABASE_URL \
  RUN_GCP04_REPLAY_INTEGRATION=YES \
  TEST_DATABASE_URL="postgres://postgres@localhost/postgres?host=$SOCKET_DIR" \
  go test -p 1 -count=1 -v \
  -run '^(TestGCP04ReplayTwoProcessesPostgres18|TestGCP04ReplayProcessHelper)$' \
  ./internal/adapters/postgres/identity
TEST_STATUS=$?
set -e
if [[ "$TEST_STATUS" -ne 0 ]]; then
  exit "$TEST_STATUS"
fi
cleanup_verified
trap - EXIT
