#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
SECRETS_DIR="$ROOT_DIR/.local/secrets"
export LOCAL_UID="$(id -u)"
export LOCAL_GID="$(id -g)"
export LOCAL_M03_EVIDENCE_DIR="${XDG_DATA_HOME:-$(getent passwd "$(id -u)" | cut -d: -f6)/.local/share}/espacigo/m03-evidence"

compose() {
  docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" "$@"
}

ensure_secrets() {
  umask 077
  mkdir -p "$SECRETS_DIR"
  chmod 0700 "$SECRETS_DIR"
  for name in db_admin_password runtime_password local_payment_webhook_secret; do
    path="$SECRETS_DIR/$name"
    if [[ ! -s "$path" ]]; then
      rm -f -- "$path"
      python3 - "$path" <<'PY'
import os
import secrets
import sys

path = sys.argv[1]
try:
    fd = os.open(path, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
except FileExistsError:
    raise SystemExit(0)
with os.fdopen(fd, "w", encoding="ascii") as stream:
    stream.write(secrets.token_hex(32) + "\n")
PY
    fi
    chmod 0600 "$path"
    python3 - "$path" <<'PY'
import pathlib
import re
import sys

value = pathlib.Path(sys.argv[1]).read_text(encoding="ascii").strip()
if re.fullmatch(r"[0-9a-f]{64}", value) is None:
    raise SystemExit("local synthetic credential has invalid format")
PY
  done
}

ensure_evidence_dir() {
  umask 077
  mkdir -p "$LOCAL_M03_EVIDENCE_DIR"
  chmod 0700 "$LOCAL_M03_EVIDENCE_DIR"
  local owner
  owner="$(stat -c '%u' "$LOCAL_M03_EVIDENCE_DIR")"
  if [[ "$owner" != "$LOCAL_UID" ]]; then
    printf 'Private evidence directory must be owned by UID %s: %s\n' "$LOCAL_UID" "$LOCAL_M03_EVIDENCE_DIR" >&2
    exit 1
  fi
}

usage() {
  printf '%s\n' \
    'Usage: scripts/dev-env.sh {config|up|down|verify-isolation|verify-http|verify-m01|clean}' \
    '  config           Generate local secrets if missing and validate Compose' \
    '  up [options]      Build and start the stack, waiting for healthy services' \
    '  down [options]    Stop the stack (pass --volumes to remove the database)' \
    '  verify-isolation  Probe the dynamic PostgreSQL IP from mock and data networks' \
    '  verify-http       Check API health, mock assets, and HTTP/CORS behavior' \
    '  verify-m01        Exercise real registration, SMTP, verification and session' \
    '  clean             Stop, remove volumes/orphans, and delete generated secrets'
}

command="${1:-}"
if [[ $# -gt 0 ]]; then shift; fi
case "$command" in
  config)
    ensure_secrets
    ensure_evidence_dir
    compose config --quiet
    ;;
  up)
    ensure_secrets
    ensure_evidence_dir
    compose up --build --wait "$@"
    ;;
  down)
    compose down "$@"
    ;;
  verify-isolation)
    "$ROOT_DIR/scripts/verify-network-isolation.sh"
    ;;
  verify-http)
    python3 "$ROOT_DIR/scripts/verify-http.py"
    ;;
  verify-m01)
    "${M01_PYTHON:-python3}" "$ROOT_DIR/scripts/verify-m01.py" "$@"
    ;;
  clean)
    compose down --volumes --remove-orphans
    rm -f -- "$SECRETS_DIR/db_admin_password" "$SECRETS_DIR/runtime_password" "$SECRETS_DIR/local_payment_webhook_secret"
    rmdir "$SECRETS_DIR" 2>/dev/null || true
    rmdir "$ROOT_DIR/.local" 2>/dev/null || true
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
