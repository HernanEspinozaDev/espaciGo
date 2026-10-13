#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
TEST_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/espacigo-docker01.XXXXXX")"
API_IMAGE="espacigo-docker01-api-test:$$"
API_LOG="$TEST_ROOT/api-build.log"
cleanup() {
  docker image rm -f "$API_IMAGE" "espacigo-docker01-angular-test:$$" >/dev/null 2>&1 || true
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

FAKE_BIN="$TEST_ROOT/fake-bin"
mkdir -m 0700 "$FAKE_BIN"
cat > "$FAKE_BIN/docker" <<'SH'
#!/usr/bin/env bash
printf '%s\n' "$*" >> "$DOCKER_CALL_LOG"
exit 0
SH
chmod 0700 "$FAKE_BIN/docker"

snapshot_dev_private_state() {
  python3 - "$ROOT_DIR/.local" <<'PY'
import hashlib
import pathlib
import sys

root = pathlib.Path(sys.argv[1])
if not root.exists():
    raise SystemExit("development private directory is unavailable for collision check")
secrets = root / "secrets"
print(".", oct(root.stat().st_mode & 0o777), "directory")
for path in sorted(root.rglob("*")):
    if path.is_symlink():
        print(path.relative_to(root), "symlink", str(path.readlink()))
    elif path.is_file():
        stat = path.stat()
        details = hashlib.sha256(path.read_bytes()).hexdigest() if secrets in path.parents else f"{stat.st_size}:{stat.st_mtime_ns}"
        print(path.relative_to(root), oct(stat.st_mode & 0o777), details)
    elif path.is_dir():
        print(path.relative_to(root), oct(path.stat().st_mode & 0o777), "directory")
PY
}

private_before="$(snapshot_dev_private_state)"
docker_call_log="$TEST_ROOT/docker-calls.log"
ln -s "$ROOT_DIR/.local" "$TEST_ROOT/dev-state-alias"
if LOCAL_DOCKER_STATE_DIR="$TEST_ROOT/dev-state-alias" DOCKER_CALL_LOG="$docker_call_log" \
  PATH="$FAKE_BIN:$PATH" bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated state accepted the development private directory' >&2
  exit 1
fi
private_after="$(snapshot_dev_private_state)"
if [[ "$private_before" != "$private_after" || -e "$docker_call_log" ]]; then
  echo 'FAIL: path collision rejection modified development files or invoked Docker' >&2
  exit 1
fi

linked_evidence_state="$TEST_ROOT/isolated-with-dev-evidence-link"
mkdir -m 0700 "$linked_evidence_state"
dev_evidence_dir="$(python3 -c 'import os,pathlib,pwd; home=pathlib.Path(pwd.getpwuid(os.getuid()).pw_dir); print(pathlib.Path(os.environ.get("XDG_DATA_HOME", home / ".local/share")) / "espacigo/m03-evidence")')"
ln -s "$dev_evidence_dir" "$linked_evidence_state/private-evidence"
if LOCAL_DOCKER_STATE_DIR="$linked_evidence_state" DOCKER_CALL_LOG="$docker_call_log" \
  PATH="$FAKE_BIN:$PATH" bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated private-evidence path reused the development evidence directory' >&2
  exit 1
fi
if [[ -e "$linked_evidence_state/secrets" || -e "$docker_call_log" ]]; then
  echo 'FAIL: evidence-path rejection created isolated secrets or invoked Docker' >&2
  exit 1
fi

runtime_reject_state="$TEST_ROOT/runtime-rejected"
if LOCAL_DOCKER_STATE_DIR="$runtime_reject_state" LOCAL_DATABASE_RUNTIME_USER=espacigo_admin \
  DOCKER_CALL_LOG="$docker_call_log" PATH="$FAKE_BIN:$PATH" \
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated stack accepted a non-runtime application user' >&2
  exit 1
fi
if [[ -e "$runtime_reject_state" || -e "$docker_call_log" ]]; then
  echo 'FAIL: runtime-user rejection created files or invoked Docker' >&2
  exit 1
fi

image_prefix_reject_state="$TEST_ROOT/image-prefix-rejected"
if LOCAL_DOCKER_STATE_DIR="$image_prefix_reject_state" LOCAL_DOCKER_IMAGE_PREFIX=espacigo-local \
  DOCKER_CALL_LOG="$docker_call_log" PATH="$FAKE_BIN:$PATH" \
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated stack accepted the development image prefix' >&2
  exit 1
fi
if [[ -e "$image_prefix_reject_state" || -e "$docker_call_log" ]]; then
  echo 'FAIL: development image-prefix rejection created files or invoked Docker' >&2
  exit 1
fi

isolated_json="$TEST_ROOT/isolated-compose.json"
ISOLATED_STATE_DIR="$TEST_ROOT/isolated"
LOCAL_DOCKER_STATE_DIR="$ISOLATED_STATE_DIR" \
LOCAL_DOCKER_PROJECT_NAME="espacigo_docker01_test_$$" \
LOCAL_DOCKER_PGDATA_VOLUME="espacigo_docker01_test_pgdata_$$" \
LOCAL_DOCKER_API_PORT=18080 LOCAL_DOCKER_MAILPIT_PORT=18025 LOCAL_DOCKER_ANGULAR_PORT=18420 \
DATABASE_SSLMODE=verify-full DATABASE_SSLROOTCERT=/run/database-tls/root.pem \
DATABASE_SSLCERT=/run/database-tls/client.pem DATABASE_SSLKEY=/run/database-tls/client.key \
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" config > "$isolated_json"

if LOCAL_DOCKER_STATE_DIR="$TEST_ROOT/invalid" LOCAL_DOCKER_API_PORT=08080 \
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
  echo 'FAIL: isolated configuration accepted a numerically colliding development port' >&2
  exit 1
fi
if LOCAL_DOCKER_STATE_DIR="$TEST_ROOT/invalid" LOCAL_DOCKER_API_PORT=18080 \
  LOCAL_DOCKER_ANGULAR_PORT=18080 \
  bash "$ROOT_DIR/scripts/local-docker-stack.sh" config >/dev/null 2>&1; then
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
project_name = isolated["name"]
check(isolated["services"]["backend"]["image"] == project_name + "-api:dev",
      "isolated API image tag is project-specific")
check(isolated["services"]["migrate"]["image"] == project_name + "-api:dev",
      "isolated migrator image tag is project-specific")
check(isolated["services"]["angular-frontend"]["image"] == project_name + "-angular:dev",
      "isolated Angular image tag is project-specific")
check(isolated["services"]["database"]["image"] == project_name + "-database:dev",
      "isolated database image tag is project-specific")
check(dev["services"]["backend"]["image"] == "espacigo-local-api:dev",
      "development API image tag remains stable")
check(dev["services"]["angular-frontend"]["image"] == "espacigo-local-angular:dev",
      "development Angular image tag remains stable")
check(dev["services"]["database"]["image"] == "espacigo-local-database:dev",
      "development database image tag remains stable")

def host_port(service):
    ports = service["ports"]
    check(len(ports) == 1, "one loopback port is published")
    return str(ports[0]["published"])

check(host_port(isolated["services"]["backend"]) == "18080", "isolated API port")
check(host_port(isolated["services"]["angular-frontend"]) == "18420", "isolated Angular port")
check(host_port(isolated["services"]["mailpit"]) == "18025", "isolated Mailpit port")
check(host_port(dev["services"]["backend"]) == "8080", "development API port remains 8080")
check(host_port(dev["services"]["angular-frontend"]) == "4200", "development Angular port remains 4200")
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
check("DATABASE_USER" not in isolated["services"]["angular-frontend"]["environment"],
      "Angular has no database user")
check(not isolated["services"]["angular-frontend"].get("secrets"), "Angular receives no secrets")
check(set(isolated["services"]["angular-frontend"]["networks"]) == {"api"},
      "Angular connects only to the API network")
check("data" not in isolated["services"]["angular-frontend"]["networks"],
      "Angular has no database network route")
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
print("PASS: isolated image tags are project-specific and development tags remain unchanged")
print("PASS: ordinary development project, volume, and ports remain unchanged")
print("PASS: Angular has API-only network access and no database credentials/secrets")
PY

docker build --progress=plain --target api -t "$API_IMAGE" -f "$ROOT_DIR/Dockerfile" "$ROOT_DIR" >"$API_LOG" 2>&1
if grep -Eqi 'node|npm|frontend' "$API_LOG"; then
  cat "$API_LOG" >&2
  echo 'FAIL: Go API target executed frontend build work' >&2
  exit 1
fi
docker build --progress=plain -t "espacigo-docker01-angular-test:$$" -f "$ROOT_DIR/frontend/Dockerfile" "$ROOT_DIR/frontend" >"$TEST_ROOT/angular-build.log" 2>&1

echo 'PASS: Go API target builds without Node or frontend dependencies'
echo 'PASS: Angular Docker target builds independently as the default frontend'
echo 'PASS: DOCKER-01 focused checks complete; no services or persistent volumes were started'
