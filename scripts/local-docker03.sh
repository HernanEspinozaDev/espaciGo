#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER03_PROJECT="${LOCAL_DOCKER03_PROJECT_NAME:-espacigo_docker03_20261011}"
DOCKER03_STATE="$(python3 -c 'import pathlib,sys; print(pathlib.Path(sys.argv[1]).expanduser().resolve(strict=False))' "${LOCAL_DOCKER03_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/espacigo/docker03-20261011}")"
DOCKER03_VOLUME="${LOCAL_DOCKER03_PGDATA_VOLUME:-espacigo_docker03_pgdata_20261011}"
DOCKER03_RESTORE_VOLUME="${LOCAL_DOCKER03_RESTORE_VOLUME:-espacigo_docker03_restore_pgdata_20261011}"
DOCKER03_TEST_VOLUME="${LOCAL_DOCKER03_TEST_PGDATA_VOLUME:-espacigo_docker03_test_pgdata_20261011}"
DOCKER03_RESTORE_DATABASE="${LOCAL_DOCKER03_RESTORE_DATABASE_NAME:-espacigo_local_restored}"
DOCKER03_API_PORT="${LOCAL_DOCKER03_API_PORT:-18280}"
DOCKER03_ANGULAR_PORT="${LOCAL_DOCKER03_ANGULAR_PORT:-18284}"
DOCKER03_MAILPIT_PORT="${LOCAL_DOCKER03_MAILPIT_PORT:-18225}"
DOCKER03_DATABASE_PORT="${LOCAL_DOCKER03_DATABASE_PORT:-18132}"
DOCKER03_RESTORE_API_PORT="${LOCAL_DOCKER03_RESTORE_API_PORT:-18282}"
DOCKER03_TLS_PROBE_PORT="${LOCAL_DOCKER03_TLS_PROBE_PORT:-18283}"

if [[ "${LOCAL_DOCKER03_RUNTIME_USER:-espacigo_runtime}" != espacigo_runtime ]]; then
  echo 'DOCKER-03 requires the fixed application role espacigo_runtime' >&2
  exit 2
fi
if [[ ! "$DOCKER03_RESTORE_DATABASE" =~ ^[a-z][a-z0-9_]{2,62}$ ]]; then
  echo 'DOCKER-03 restore database name must be a lowercase SQL identifier' >&2
  exit 2
fi

validate() {
python3 - "$ROOT_DIR" "$DOCKER03_STATE" "$DOCKER03_PROJECT" "$DOCKER03_VOLUME" "$DOCKER03_RESTORE_VOLUME" "$DOCKER03_TEST_VOLUME" "$DOCKER03_API_PORT" "$DOCKER03_MAILPIT_PORT" "$DOCKER03_DATABASE_PORT" "$DOCKER03_RESTORE_API_PORT" "$DOCKER03_TLS_PROBE_PORT" "$DOCKER03_ANGULAR_PORT" <<'PY'
import os, pathlib, pwd, sys
root, state = pathlib.Path(sys.argv[1]).resolve(), pathlib.Path(sys.argv[2]).resolve(strict=False)
project, volume, restore_volume, test_volume = sys.argv[3:7]
ports = [int(value) for value in sys.argv[7:13]]
if not all(3 <= len(v) <= 63 and v[0].islower() and all(c.islower() or c.isdigit() or c in "_-" for c in v) for v in (project, volume, restore_volume, test_volume)):
    raise SystemExit("DOCKER-03 project and volume names must be lowercase Compose identifiers")
if not project.startswith("espacigo_docker03_") or not volume.startswith("espacigo_docker03_") or not restore_volume.startswith("espacigo_docker03_") or not test_volume.startswith("espacigo_docker03_"):
    raise SystemExit("DOCKER-03 project and volumes must use the dedicated espacigo_docker03_ prefix")
if project == "espacigo_docker03" or volume == "espacigo_docker03_pgdata" or restore_volume == "espacigo_docker03_restore_pgdata" or test_volume == "espacigo_docker03_test_pgdata":
    raise SystemExit("DOCKER-03 names must include a unique run suffix")
if len({volume, restore_volume, test_volume}) != 3:
    raise SystemExit("DOCKER-03 source, restore, and test volume names must be distinct")
if len(set(ports)) != len(ports) or any(p < 1024 or p > 65535 for p in ports) or any(p in {8080,8025,4200,18080,18025,18420} for p in ports):
    raise SystemExit("DOCKER-03 ports must be unique and differ from development and DOCKER-02")
home = pathlib.Path(pwd.getpwuid(os.getuid()).pw_dir)
data_home = pathlib.Path(os.environ.get("XDG_DATA_HOME", home / ".local/share"))
state_home = pathlib.Path(os.environ.get("XDG_STATE_HOME", home / ".local/state"))
protected = [root / ".local/secrets", root / ".local/database-tls", root / ".local/docker-isolated/docker02-20261011", data_home / "espacigo/m03-evidence", state_home / "espacigo/privacy-replay"]
if state == root or root in state.parents:
    raise SystemExit("DOCKER-03 private state must not be inside the repository")
for candidate in [state, state / "secrets", state / "private-evidence", state / "restored-private-evidence", state / "privacy-replay", state / "database-tls", state / "database-ca", state / "backups"]:
    for protected_path in protected:
        candidate_real, protected_real = candidate.resolve(strict=False), protected_path.resolve(strict=False)
        if candidate_real == protected_real or candidate_real in protected_real.parents or protected_real in candidate_real.parents:
            raise SystemExit("DOCKER-03 state overlaps development or DOCKER-02 data")
PY
}

export LOCAL_DOCKER_PROJECT_NAME="$DOCKER03_PROJECT"
export LOCAL_IMAGE_PREFIX="$DOCKER03_PROJECT"
export LOCAL_API_PORT="$DOCKER03_API_PORT"
export LOCAL_ANGULAR_PORT="$DOCKER03_ANGULAR_PORT"
export LOCAL_MAILPIT_PORT="$DOCKER03_MAILPIT_PORT"
export LOCAL_PGDATA_VOLUME="$DOCKER03_VOLUME"
export LOCAL_DOCKER03_RESTORE_VOLUME="$DOCKER03_RESTORE_VOLUME"
export LOCAL_DOCKER03_TEST_PGDATA_VOLUME="$DOCKER03_TEST_VOLUME"
export LOCAL_DOCKER03_TEST_NO_RUNTIME_INIT_FILE="$DOCKER03_STATE/test-no-runtime-init.sh"
export LOCAL_DOCKER03_RESTORE_DATABASE_NAME="$DOCKER03_RESTORE_DATABASE"
export LOCAL_DOCKER03_DATABASE_PORT="$DOCKER03_DATABASE_PORT"
export LOCAL_DOCKER03_RESTORE_API_PORT="$DOCKER03_RESTORE_API_PORT"
export LOCAL_DOCKER03_TLS_PROBE_PORT="$DOCKER03_TLS_PROBE_PORT"
export LOCAL_DOCKER_STATE_DIR="$DOCKER03_STATE"
export LOCAL_SECRETS_DIR="$DOCKER03_STATE/secrets"
export LOCAL_M03_EVIDENCE_DIR="$DOCKER03_STATE/private-evidence"
export LOCAL_PRIVACY_REPLAY_DIR="$DOCKER03_STATE/privacy-replay"
export LOCAL_DATABASE_TLS_DIR="$DOCKER03_STATE/database-ca"
export LOCAL_DOCKER03_POSTGRES_TLS_DIR="$DOCKER03_STATE/database-tls"
export LOCAL_DATABASE_NAME="${LOCAL_DATABASE_NAME:-espacigo_local}"
export LOCAL_DATABASE_HOST="${LOCAL_DATABASE_HOST:-database}"
export LOCAL_DATABASE_PORT="${LOCAL_DATABASE_PORT:-5432}"
export LOCAL_DATABASE_ADMIN_USER="${LOCAL_DATABASE_ADMIN_USER:-espacigo_admin}"
export LOCAL_DATABASE_RUNTIME_USER=espacigo_runtime
export DATABASE_SSLMODE=verify-full
export DATABASE_SSLROOTCERT=/run/database-tls/root.pem
export DATABASE_SSLCERT=
export DATABASE_SSLKEY=
export LOCAL_UID="$(id -u)" LOCAL_GID="$(id -g)"

compose() {
  docker compose --project-directory "$ROOT_DIR" -f "$ROOT_DIR/compose.yaml" -f "$ROOT_DIR/compose.docker03.yaml" -p "$DOCKER03_PROJECT" "$@"
}

private_dir() {
  [[ ! -L "$1" ]] || { echo "DOCKER-03 private directory cannot be a symlink: $1" >&2; exit 2; }
  umask 077; mkdir -p "$1"; chmod 0700 "$1"
  [[ "$(stat -c %u "$1")" == "$(id -u)" ]] || { echo "DOCKER-03 private directory must be owned by current UID: $1" >&2; exit 2; }
}
secret() {
  local path="$LOCAL_SECRETS_DIR/$1"
  [[ ! -L "$path" ]] || { echo "DOCKER-03 secret cannot be a symlink: $path" >&2; exit 2; }
  if [[ ! -s "$path" ]]; then
    python3 - "$path" <<'PY'
import os,secrets,sys
fd=os.open(sys.argv[1],os.O_CREAT|os.O_EXCL|os.O_WRONLY,0o600)
with os.fdopen(fd,"w") as f: f.write(secrets.token_hex(32)+"\n")
PY
  fi
  chmod 0600 "$path"
  [[ "$(stat -c %a "$path")" == 600 ]]
}
ensure() {
  validate
  for directory in "$DOCKER03_STATE" "$LOCAL_SECRETS_DIR" "$LOCAL_M03_EVIDENCE_DIR" "$LOCAL_PRIVACY_REPLAY_DIR" "$LOCAL_DATABASE_TLS_DIR" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR" "$DOCKER03_STATE/backups"; do private_dir "$directory"; done
  if [[ ! -e "$LOCAL_DOCKER03_TEST_NO_RUNTIME_INIT_FILE" ]]; then
    (umask 077; : > "$LOCAL_DOCKER03_TEST_NO_RUNTIME_INIT_FILE")
  fi
  chmod 0644 "$LOCAL_DOCKER03_TEST_NO_RUNTIME_INIT_FILE"
  for name in db_admin_password runtime_password local_payment_webhook_secret local_contract_encryption_key; do secret "$name"; done
  if [[ ! -s "$LOCAL_DATABASE_TLS_DIR/root.pem" && -s "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root.pem" && -s "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.key" && -s "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.crt" && -s "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-root.pem" ]]; then
    cp "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root.pem" "$LOCAL_DATABASE_TLS_DIR/root.pem"
    cp "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-root.pem" "$LOCAL_DATABASE_TLS_DIR/untrusted-root.pem"
  elif [[ ! -s "$LOCAL_DATABASE_TLS_DIR/root.pem" ]]; then
    local cert_tmp="$LOCAL_DOCKER03_POSTGRES_TLS_DIR/ca.tmp.$$" ext_tmp="$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.ext.tmp.$$"
    trap 'rm -f -- "$cert_tmp" "$ext_tmp"' RETURN
    openssl req -x509 -newkey rsa:3072 -nodes -keyout "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root-key.pem" -out "$cert_tmp" -days 3650 -subj "/CN=EspaciGo DOCKER-03 local CA" >/dev/null 2>&1
    cat > "$ext_tmp" <<'EOF'
subjectAltName=DNS:database,DNS:restore-database,DNS:localhost,IP:127.0.0.1
extendedKeyUsage=serverAuth
keyUsage=digitalSignature,keyEncipherment
EOF
    openssl req -new -newkey rsa:3072 -nodes -keyout "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.key" -out "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.csr" -subj "/CN=database" >/dev/null 2>&1
    openssl x509 -req -in "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.csr" -CA "$cert_tmp" -CAkey "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root-key.pem" -CAcreateserial -out "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.crt" -days 825 -extfile "$ext_tmp" >/dev/null 2>&1
    cp "$cert_tmp" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root.pem"
    cp "$cert_tmp" "$LOCAL_DATABASE_TLS_DIR/root.pem"
    openssl req -x509 -newkey rsa:2048 -nodes -keyout "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-key.pem" -out "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-root.pem" -days 3650 -subj "/CN=EspaciGo DOCKER-03 untrusted CA" >/dev/null 2>&1
    cp "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-root.pem" "$LOCAL_DATABASE_TLS_DIR/untrusted-root.pem"
    rm -f -- "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.csr" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root-key.pem" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-key.pem" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR"/*.srl "$cert_tmp" "$ext_tmp"
    cat > "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/pg_hba.conf" <<'EOF'
local all all trust
hostnossl all all all reject
hostssl all all all scram-sha-256
EOF
  fi
  chmod 0600 "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.key" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/untrusted-root.pem" 2>/dev/null || true
  chmod 0644 "$LOCAL_DATABASE_TLS_DIR/root.pem" "$LOCAL_DATABASE_TLS_DIR/untrusted-root.pem" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/root.pem" "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/server.crt"
  chmod 0600 "$LOCAL_DOCKER03_POSTGRES_TLS_DIR/pg_hba.conf"
  chmod 0755 "$LOCAL_DATABASE_TLS_DIR"
  export DATABASE_SSLMODE=verify-full DATABASE_SSLROOTCERT=/run/database-tls/root.pem
}

usage() {
  cat <<'EOF'
Usage: scripts/local-docker03.sh {config|up|status|restart-backend|dump|restore-db|replay|start-restored-backend}

The helper owns DOCKER-03 project, ports, volumes, credentials, TLS CA, backups,
private evidence and replay registry. It never removes a volume or secret.
`restore-db` refuses a non-empty target; `replay` must finish before the
restored API can be started. Set RESTORE_ID and ADMIN_ACCOUNT_ID for replay.
EOF
}

command="${1:-}"; [[ $# -eq 0 ]] || shift
case "$command" in
  config) ensure; compose config --quiet; echo "DOCKER-03 config valid: $DOCKER03_PROJECT" ;;
  up) ensure; compose up --build --wait database restore-database mailpit migrate backend angular-frontend ;;
  status) ensure; compose ps ;;
  restart-backend) ensure; compose restart backend; compose ps backend ;;
  dump)
    ensure
    target="$DOCKER03_STATE/backups/source.dump"
    [[ ! -e "$target" ]] || { echo 'backup already exists; choose a fresh DOCKER-03 state rather than overwrite it' >&2; exit 2; }
    umask 077
    compose exec -T database pg_dump --format=custom --no-owner --no-privileges --username "$LOCAL_DATABASE_ADMIN_USER" --dbname "$LOCAL_DATABASE_NAME" > "$target"
    chmod 0600 "$target"
    compose exec -T backend /api local-privacy-replay-export >/dev/null
    chmod 0600 "$LOCAL_PRIVACY_REPLAY_DIR/completed-suppressions-v1.json"
    rm -f -- "$LOCAL_PRIVACY_REPLAY_DIR/replay-applied.sha256"
    mkdir -p "$DOCKER03_STATE/restored-private-evidence"
    chmod 0700 "$DOCKER03_STATE/restored-private-evidence"
    cp -a "$LOCAL_M03_EVIDENCE_DIR/." "$DOCKER03_STATE/restored-private-evidence/"
    echo "DOCKER-03 source dump, suppression registry and private files captured under $DOCKER03_STATE (0600/0700)." ;;
  restore-db)
    ensure
    [[ -s "$DOCKER03_STATE/backups/source.dump" ]] || { echo 'source dump is missing; run dump first' >&2; exit 2; }
    if compose exec -T restore-database psql -X -v ON_ERROR_STOP=1 --username "$LOCAL_DATABASE_ADMIN_USER" --dbname postgres -Atqc "SELECT 1 FROM pg_database WHERE datname='$DOCKER03_RESTORE_DATABASE'" | grep -q 1; then
      echo 'restore target database already exists; refusing to overwrite it' >&2; exit 2
    fi
    compose up -d --wait restore-database >/dev/null
    compose exec -T restore-database createdb --username "$LOCAL_DATABASE_ADMIN_USER" "$DOCKER03_RESTORE_DATABASE"
    compose exec -T restore-database pg_restore --exit-on-error --no-owner --no-privileges --username "$LOCAL_DATABASE_ADMIN_USER" --dbname "$DOCKER03_RESTORE_DATABASE" < "$DOCKER03_STATE/backups/source.dump"
    if ! compose exec -T restore-database psql -X -v ON_ERROR_STOP=1 --username "$LOCAL_DATABASE_ADMIN_USER" --dbname postgres -Atqc "SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime'" | grep -q 1; then
      echo 'restore PostgreSQL is missing the fixed espacigo_runtime role; refusing to run migrations' >&2
      exit 1
    fi
    compose exec -T restore-database psql -X -v ON_ERROR_STOP=1 --username "$LOCAL_DATABASE_ADMIN_USER" --dbname postgres -c "GRANT CONNECT ON DATABASE \"$DOCKER03_RESTORE_DATABASE\" TO espacigo_runtime" >/dev/null
    export LOCAL_DATABASE_HOST=restore-database
    export LOCAL_DATABASE_NAME="$DOCKER03_RESTORE_DATABASE"
    compose run --rm --no-deps migrate
    rm -f -- "$LOCAL_PRIVACY_REPLAY_DIR/replay-applied.sha256"
    echo 'DOCKER-03 restore completed into the separate restore volume; API remains stopped.' ;;
  replay)
    ensure
    [[ -n "${RESTORE_ID:-}" && -n "${ADMIN_ACCOUNT_ID:-}" ]] || { echo 'RESTORE_ID and ADMIN_ACCOUNT_ID are required' >&2; exit 2; }
    [[ -s "$LOCAL_PRIVACY_REPLAY_DIR/completed-suppressions-v1.json" ]] || { echo 'external suppression registry is missing' >&2; exit 2; }
    export LOCAL_DATABASE_HOST=restore-database
    export LOCAL_DATABASE_NAME="$DOCKER03_RESTORE_DATABASE"
    export LOCAL_M03_EVIDENCE_DIR="$DOCKER03_STATE/restored-private-evidence"
    compose run --rm --no-deps -T backend local-privacy-replay-apply "$RESTORE_ID" "$ADMIN_ACCOUNT_ID" < "$LOCAL_PRIVACY_REPLAY_DIR/completed-suppressions-v1.json"
    sha256sum "$LOCAL_PRIVACY_REPLAY_DIR/completed-suppressions-v1.json" | awk '{print $1}' > "$LOCAL_PRIVACY_REPLAY_DIR/replay-applied.sha256"
    chmod 0600 "$LOCAL_PRIVACY_REPLAY_DIR/replay-applied.sha256"
    echo 'External suppressions replayed on restored DB before API exposure.' ;;
  start-restored-backend)
    ensure
    [[ -s "$LOCAL_PRIVACY_REPLAY_DIR/completed-suppressions-v1.json" ]] || { echo 'external suppression registry is missing; refusing API exposure' >&2; exit 2; }
    expected="$(sha256sum "$LOCAL_PRIVACY_REPLAY_DIR/completed-suppressions-v1.json" | awk '{print $1}')"
    [[ -s "$LOCAL_PRIVACY_REPLAY_DIR/replay-applied.sha256" && "$(cat "$LOCAL_PRIVACY_REPLAY_DIR/replay-applied.sha256")" == "$expected" ]] || { echo 'restored registry replay is not confirmed; refusing API exposure' >&2; exit 2; }
    export LOCAL_M03_EVIDENCE_DIR="$DOCKER03_STATE/restored-private-evidence"
    compose up -d --no-deps restore-backend
    ;;
  *) usage >&2; exit 2 ;;
esac
