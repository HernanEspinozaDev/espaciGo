#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-docker01.XXXXXX")"
API_IMAGE="espacigo-docker01-api-test:$$"
MOCK_IMAGE="espacigo-docker01-mock-test:$$"
API_LOG="$TEST_ROOT/api-build.log"
MOCK_LOG="$TEST_ROOT/mock-build.log"
cleanup() {
  docker image rm -f "$API_IMAGE" "$MOCK_IMAGE" >/dev/null 2>&1 || true
  python3 - "$TEST_ROOT" <<'PY'
import pathlib
import shutil
import sys

path = pathlib.Path(sys.argv[1]).resolve()
if path.parent == pathlib.Path("/tmp") and path.name.startswith("espacigo-docker01."):
    shutil.rmtree(path)
PY
}
trap cleanup EXIT INT TERM

command -v docker >/dev/null
command -v python3 >/dev/null
docker info >/dev/null

isolated_json="$TEST_ROOT/isolated-compose.json"
ISOLATED_STATE_DIR="$TEST_ROOT/isolated"
LOCAL_DOCKER_STATE_DIR="$ISOLATED_STATE_DIR" \
LOCAL_DOCKER_PROJECT_NAME="espacigo_docker01_test_$$" \
LOCAL_DOCKER_PGDATA_VOLUME="espacigo_docker01_test_pgdata_$$" \
LOCAL_DOCKER_API_PORT=18080 LOCAL_DOCKER_MOCK_PORT=18081 LOCAL_DOCKER_MAILPIT_PORT=18025 \
DATABASE_SSLMODE=verify-full DATABASE_SSLROOTCERT=/run/database-tls/root.pem \
DATABASE_SSLCERT=/run/database-tls/client.pem DATABASE_SSLKEY=/run/database-tls/client.key \
  "$ROOT_DIR/scripts/local-docker-stack.sh" config > "$isolated_json"

if LOCAL_DOCKER_STATE_DIR="$TEST_ROOT/invalid" LOCAL_DOCKER_API_PORT=08080 \
  "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated configuration accepted a numerically colliding development port' >&2
  exit 1
fi
if LOCAL_DOCKER_STATE_DIR="$TEST_ROOT/invalid" LOCAL_DOCKER_API_PORT=18080 \
  LOCAL_DOCKER_MOCK_PORT=18080 \
  "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated configuration accepted duplicate numeric ports' >&2
  exit 1
fi

# Render the ordinary development configuration without starting services or
# creating its directories; this confirms its project/volume/ports stay stable.
dev_json="$TEST_ROOT/dev-compose.json"
LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)" \
LOCAL_COMPOSE_PROJECT_NAME=espacigo LOCAL_PGDATA_VOLUME=espacigo_pgdata \
LOCAL_SECRETS_DIR="$ROOT_DIR/.local/secrets" \
LOCAL_M03_EVIDENCE_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/espacigo/m03-evidence" \
LOCAL_PRIVACY_REPLAY_DIR="${XDG_STATE_HOME:-$HOME/.local/state}/espacigo/privacy-replay" \
LOCAL_DATABASE_TLS_DIR="$TEST_ROOT/dev-tls" \
  docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" config --format json > "$dev_json"

python3 - "$isolated_json" "$dev_json" "$ISOLATED_STATE_DIR" <<'PY'
import json
import pathlib
import sys

isolated = json.loads(pathlib.Path(sys.argv[1]).read_text())
dev = json.loads(pathlib.Path(sys.argv[2]).read_text())
isolated_root = str(pathlib.Path(sys.argv[3]).resolve())

def check(condition, message):
    if not condition:
        raise SystemExit("FAIL: " + message)

check(isolated.get("name", "").startswith("espacigo_docker01_test_"), "isolated Compose project name")
check(dev.get("name") == "espacigo", "development Compose project name remains espacigo")
check(isolated["volumes"]["pgdata"]["name"].startswith("espacigo_docker01_test_pgdata_"),
      "isolated PostgreSQL volume name")
check(dev["volumes"]["pgdata"]["name"] == "espacigo_pgdata",
      "development volume name remains espacigo_pgdata")

def host_port(service):
    ports = service["ports"]
    check(len(ports) == 1, "one loopback port is published")
    return str(ports[0]["published"])

check(host_port(isolated["services"]["backend"]) == "18080", "isolated API port")
check(host_port(isolated["services"]["mock-frontend"]) == "18081", "isolated mock port")
check(host_port(isolated["services"]["mailpit"]) == "18025", "isolated Mailpit port")
check(host_port(dev["services"]["backend"]) == "8080", "development API port remains 8080")
check(host_port(dev["services"]["mock-frontend"]) == "8081", "development mock port remains 8081")
check(host_port(dev["services"]["mailpit"]) == "8025", "development Mailpit port remains 8025")

backend_env = isolated["services"]["backend"]["environment"]
migrate_env = isolated["services"]["migrate"]["environment"]
check(backend_env["DATABASE_USER"] == "espacigo_runtime", "Backend uses the runtime DB role")
check(migrate_env["DATABASE_USER"] == "espacigo_admin", "migrator uses the administrative DB role")
for env in (backend_env, migrate_env):
    check(env["DATABASE_SSLMODE"] == "verify-full", "TLS mode reaches both Backend and migrator")
    check(env["DATABASE_SSLROOTCERT"] == "/run/database-tls/root.pem", "TLS root certificate path")
    check(env["DATABASE_SSLCERT"] == "/run/database-tls/client.pem", "TLS client certificate path")
    check(env["DATABASE_SSLKEY"] == "/run/database-tls/client.key", "TLS client key path")
check("DATABASE_USER" not in isolated["services"]["mock-frontend"]["environment"],
      "mock has no database user")
check(not isolated["services"]["mock-frontend"].get("secrets"), "mock receives no secrets")
check(set(isolated["services"]["mock-frontend"]["networks"]) == {"api"},
      "mock connects only to the API network")
check("data" not in isolated["services"]["mock-frontend"]["networks"],
      "mock has no database network route")
for name in ("db_admin_password", "runtime_password", "local_payment_webhook_secret", "local_contract_encryption_key"):
    secret_path = pathlib.Path(isolated["secrets"][name]["file"]).resolve()
    check(str(secret_path).startswith(isolated_root + "/secrets/"), "isolated secret path " + name)
    check(secret_path.stat().st_mode & 0o077 == 0, "isolated secret mode " + name)
for service in ("backend", "migrate"):
    mounts = isolated["services"][service]["volumes"]
    tls = next((mount for mount in mounts if mount["target"] == "/run/database-tls"), None)
    check(tls is not None and str(pathlib.Path(tls["source"]).resolve()).startswith(isolated_root + "/database-tls"),
          "private TLS material path for " + service)
print("PASS: isolated Compose project, ports, volume, secrets, TLS/private directories, and runtime/migrator roles")
print("PASS: ordinary development project, volume, and ports remain unchanged")
print("PASS: mock has API-only network access and no database credentials/secrets")
PY

docker build --progress=plain --target api -t "$API_IMAGE" -f "$ROOT_DIR/Dockerfile" "$ROOT_DIR" >"$API_LOG" 2>&1
if grep -Eq '\[(mock-assets-build|mock-build) ' "$API_LOG"; then
  cat "$API_LOG" >&2
  echo 'FAIL: API target executed a mock build stage' >&2
  exit 1
fi
docker build --progress=plain --target mock -t "$MOCK_IMAGE" -f "$ROOT_DIR/Dockerfile" "$ROOT_DIR" >"$MOCK_LOG" 2>&1
if ! grep -q '\[mock-assets-build ' "$MOCK_LOG" || ! grep -q '\[mock-build ' "$MOCK_LOG"; then
  cat "$MOCK_LOG" >&2
  echo 'FAIL: mock target did not compile TypeScript and its Go server' >&2
  exit 1
fi
echo 'PASS: Docker API target builds without invoking Node/TypeScript mock stages'
echo 'PASS: Docker mock target independently compiles TypeScript and the Go mock server'
echo 'PASS: DOCKER-01 focused checks complete; no services or persistent volumes were started'
