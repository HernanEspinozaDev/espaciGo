#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
LOCAL_DOCKER_PROJECT_NAME="${LOCAL_DOCKER_PROJECT_NAME:-espacigo_docker01}"
LOCAL_DOCKER_STATE_DIR="${LOCAL_DOCKER_STATE_DIR:-$ROOT_DIR/.local/docker-isolated}"
LOCAL_DOCKER_API_PORT="${LOCAL_DOCKER_API_PORT:-18080}"
LOCAL_DOCKER_MOCK_PORT="${LOCAL_DOCKER_MOCK_PORT:-18081}"
LOCAL_DOCKER_MAILPIT_PORT="${LOCAL_DOCKER_MAILPIT_PORT:-18025}"
LOCAL_DOCKER_PGDATA_VOLUME="${LOCAL_DOCKER_PGDATA_VOLUME:-espacigo_docker01_pgdata}"
LOCAL_DOCKER_STATE_DIR="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).expanduser().resolve(strict=False))' "$LOCAL_DOCKER_STATE_DIR")"

if [[ "${LOCAL_DATABASE_RUNTIME_USER:-espacigo_runtime}" != "espacigo_runtime" ]]; then
  printf 'isolated managed stack requires runtime user espacigo_runtime\n' >&2
  exit 2
fi
LOCAL_DATABASE_RUNTIME_USER="espacigo_runtime"

export LOCAL_COMPOSE_PROJECT_NAME="$LOCAL_DOCKER_PROJECT_NAME"
export LOCAL_API_PORT="$LOCAL_DOCKER_API_PORT"
export LOCAL_MOCK_PORT="$LOCAL_DOCKER_MOCK_PORT"
export LOCAL_MAILPIT_PORT="$LOCAL_DOCKER_MAILPIT_PORT"
export LOCAL_PGDATA_VOLUME="$LOCAL_DOCKER_PGDATA_VOLUME"
export LOCAL_SECRETS_DIR="$LOCAL_DOCKER_STATE_DIR/secrets"
export LOCAL_M03_EVIDENCE_DIR="$LOCAL_DOCKER_STATE_DIR/private-evidence"
export LOCAL_PRIVACY_REPLAY_DIR="$LOCAL_DOCKER_STATE_DIR/privacy-replay"
export LOCAL_DATABASE_TLS_DIR="$LOCAL_DOCKER_STATE_DIR/database-tls"
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
export LOCAL_DATABASE_NAME="${LOCAL_DATABASE_NAME:-espacigo_local}"
export LOCAL_DATABASE_HOST="${LOCAL_DATABASE_HOST:-database}"
export LOCAL_DATABASE_PORT="${LOCAL_DATABASE_PORT:-5432}"
export LOCAL_DATABASE_ADMIN_USER="${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}"
export LOCAL_DATABASE_RUNTIME_USER="${LOCAL_DATABASE_RUNTIME_USER:-espacigo_runtime}"
export DATABASE_SSLMODE="${DATABASE_SSLMODE:-disable}"
export DATABASE_SSLROOTCERT="${DATABASE_SSLROOTCERT:-}"
export DATABASE_SSLCERT="${DATABASE_SSLCERT:-}"
export DATABASE_SSLKEY="${DATABASE_SSLKEY:-}"

usage() {
  cat <<'EOF'
Usage: scripts/local-docker-stack.sh {config|build-backend|build-mock|up|down|status}

This stack uses a dedicated Compose project, volume, ports, secrets and private
directories. `down` retains its named PostgreSQL volume. There is intentionally
no command here that deletes the volume or its secrets.

Configuration overrides: LOCAL_DOCKER_PROJECT_NAME, LOCAL_DOCKER_STATE_DIR,
LOCAL_DOCKER_API_PORT, LOCAL_DOCKER_MOCK_PORT, LOCAL_DOCKER_MAILPIT_PORT,
LOCAL_DOCKER_PGDATA_VOLUME, DATABASE_HOST/PORT/NAME/USER through their
LOCAL_DATABASE_* equivalents, and DATABASE_SSLMODE/SSLROOTCERT/SSLCERT/SSLKEY.
The runtime role is fixed to espacigo_runtime; state paths are resolved before
creation and rejected if they overlap local development private directories.
TLS file values are container paths under /run/database-tls; place local files
in the isolated database-tls directory. Never put credentials in URLs.
EOF
}

validate_name() {
  local label="$1" value="$2"
  if [[ ! "$value" =~ ^[a-z][a-z0-9_-]{2,62}$ ]]; then
    printf '%s must be 3-63 lowercase letters, digits, _ or - and start with a letter\n' "$label" >&2
    exit 2
  fi
}

validate_port() {
  local label="$1" value="$2"
  if [[ ! "$value" =~ ^[0-9]+$ || ${#value} -gt 5 ]]; then
    printf '%s must be an unprivileged TCP port from 1024 to 65535\n' "$label" >&2
    exit 2
  fi
  local numeric_port=$((10#$value))
  if (( numeric_port < 1024 || numeric_port > 65535 )); then
    printf '%s must be an unprivileged TCP port from 1024 to 65535\n' "$label" >&2
    exit 2
  fi
  if (( numeric_port == 8080 || numeric_port == 8081 || numeric_port == 8025 )); then
    printf '%s must not reuse a port from the development stack\n' "$label" >&2
    exit 2
  fi
}

validate_isolated_paths() {
  python3 - "$ROOT_DIR" "$LOCAL_DOCKER_STATE_DIR" <<'PY'
import os
import pathlib
import pwd
import sys

root = pathlib.Path(sys.argv[1]).resolve()
state = pathlib.Path(sys.argv[2]).expanduser().resolve(strict=False)
isolated = [state / name for name in (
    "secrets", "private-evidence", "privacy-replay", "database-tls",
)]
home = pathlib.Path(pwd.getpwuid(os.getuid()).pw_dir)
data_home = pathlib.Path(os.environ.get("XDG_DATA_HOME", home / ".local/share"))
state_home = pathlib.Path(os.environ.get("XDG_STATE_HOME", home / ".local/state"))
development = [
    root / ".local/secrets",
    root / ".local/database-tls",
    data_home / "espacigo/m03-evidence",
    state_home / "espacigo/privacy-replay",
]

def overlaps(left, right):
    return left == right or left in right.parents or right in left.parents

for candidate in isolated:
    real_candidate = candidate.resolve(strict=False)
    for protected in development:
        real_protected = protected.resolve(strict=False)
        if overlaps(real_candidate, real_protected):
            print("isolated state path resolves into or overlaps a development data path", file=sys.stderr)
            raise SystemExit(2)
PY
}

validate_name LOCAL_DOCKER_PROJECT_NAME "$LOCAL_DOCKER_PROJECT_NAME"
validate_name LOCAL_DOCKER_PGDATA_VOLUME "$LOCAL_DOCKER_PGDATA_VOLUME"
validate_port LOCAL_DOCKER_API_PORT "$LOCAL_DOCKER_API_PORT"
validate_port LOCAL_DOCKER_MOCK_PORT "$LOCAL_DOCKER_MOCK_PORT"
validate_port LOCAL_DOCKER_MAILPIT_PORT "$LOCAL_DOCKER_MAILPIT_PORT"
if [[ "$LOCAL_DOCKER_PROJECT_NAME" == "espacigo" || "$LOCAL_DOCKER_PGDATA_VOLUME" == "espacigo_pgdata" ]]; then
  printf 'isolated project and volume must differ from the development stack\n' >&2
  exit 2
fi
if (( 10#$LOCAL_DOCKER_API_PORT == 10#$LOCAL_DOCKER_MOCK_PORT || 10#$LOCAL_DOCKER_API_PORT == 10#$LOCAL_DOCKER_MAILPIT_PORT || 10#$LOCAL_DOCKER_MOCK_PORT == 10#$LOCAL_DOCKER_MAILPIT_PORT )); then
  printf 'isolated host ports must be distinct\n' >&2
  exit 2
fi

compose() {
  docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" -p "$LOCAL_DOCKER_PROJECT_NAME" "$@"
}

ensure_private_dir() {
  local path="$1"
  if [[ -L "$path" ]]; then
    printf 'isolated private path cannot be a symlink: %s\n' "$path" >&2
    exit 1
  fi
  umask 077
  mkdir -p "$path"
  chmod 0700 "$path"
  if [[ "$(stat -c '%u' "$path")" != "$LOCAL_UID" ]]; then
    printf 'isolated private path must belong to UID %s: %s\n' "$LOCAL_UID" "$path" >&2
    exit 1
  fi
}

ensure_secret() {
  local name="$1" path="$LOCAL_SECRETS_DIR/$1"
  if [[ ! -s "$path" ]]; then
    python3 - "$path" <<'PY'
import os
import secrets
import sys

path = sys.argv[1]
fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
with os.fdopen(fd, "w", encoding="ascii") as stream:
    stream.write(secrets.token_hex(32) + "\n")
PY
  fi
  chmod 0600 "$path"
  python3 - "$path" <<'PY'
import pathlib
import re
import sys

if re.fullmatch(r"[0-9a-f]{64}", pathlib.Path(sys.argv[1]).read_text(encoding="ascii").strip()) is None:
    raise SystemExit("isolated local credential has invalid format")
PY
}

prepare() {
  ensure_private_dir "$LOCAL_DOCKER_STATE_DIR"
  ensure_private_dir "$LOCAL_SECRETS_DIR"
  ensure_private_dir "$LOCAL_M03_EVIDENCE_DIR"
  ensure_private_dir "$LOCAL_PRIVACY_REPLAY_DIR"
  ensure_private_dir "$LOCAL_DATABASE_TLS_DIR"
  ensure_secret db_admin_password
  ensure_secret runtime_password
  ensure_secret local_payment_webhook_secret
  ensure_secret local_contract_encryption_key
}

command="${1:-}"
case "$command" in
  config)
    validate_isolated_paths
    prepare
    compose config --format json
    ;;
  build-backend)
    validate_isolated_paths
    prepare
    compose build backend migrate
    ;;
  build-mock)
    validate_isolated_paths
    prepare
    compose build mock-frontend
    ;;
  up)
    validate_isolated_paths
    prepare
    compose up --build --wait
    ;;
  down)
    validate_isolated_paths
    compose down
    ;;
  status)
    validate_isolated_paths
    compose ps
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
