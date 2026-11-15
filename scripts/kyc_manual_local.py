#!/usr/bin/env python3
"""Fail-closed local KYC sandbox launcher. Services run in the foreground."""
import os
import pathlib
import re
import shlex
import shutil
import socket
import stat
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent
MANUAL_ROOT = ROOT / ".local/testing/kyc-manual"
DIDIT_ENV = pathlib.Path("/home/nandev/Documentos/projects/espaciGo-kyc-didit/.local/testing/didit_sandbox.env")
PUBLISHED_WORKFLOW_ID = "a4b973d4-4578-44fc-8580-7554e39821ce"
ALLOWED_DIDIT_KEYS = {"DIDIT_API_KEY", "DIDIT_WEBHOOK_SECRET", "DIDIT_BASE_URL", "DIDIT_WORKFLOW_ID"}
class LaunchError(Exception):
    pass

def fail(message):
    raise LaunchError(message)

def parse_didit_env(path):
    try:
        st = path.lstat()
        if not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode) != 0o600 or st.st_uid != os.getuid():
            fail("private Didit env file must be owned regular mode 0600")
        text = path.read_text(encoding="utf-8")
    except LaunchError:
        raise
    except Exception:
        fail("private Didit env file unavailable")
    result = {}
    for line in text.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        m = re.fullmatch(r"(?:export )?([A-Z_]+)=(.*)", line)
        if not m or m.group(1) not in ALLOWED_DIDIT_KEYS:
            fail("private Didit env contains invalid entry")
        key, raw = m.groups()
        if key in result:
            fail("private Didit env contains duplicate key")
        try:
            parts = shlex.split(raw, comments=False, posix=True)
        except ValueError:
            fail("private Didit env contains invalid value syntax")
        if len(parts) != 1 or not parts[0]:
            fail("private Didit env contains invalid value syntax")
        result[key] = parts[0]
    if not {"DIDIT_API_KEY", "DIDIT_WEBHOOK_SECRET", "DIDIT_WORKFLOW_ID"}.issubset(result):
        fail("private Didit env is incomplete")
    return result

def validate_private_file(path, private_root):
    try:
        rootst = private_root.lstat()
        st = path.lstat()
        if not stat.S_ISDIR(rootst.st_mode) or stat.S_IMODE(rootst.st_mode) != 0o700 or rootst.st_uid != os.getuid():
            fail("private directory must be owned mode 0700")
        if not stat.S_ISREG(st.st_mode) or stat.S_IMODE(st.st_mode) != 0o600 or st.st_uid != os.getuid():
            fail("private file must be owned regular mode 0600")
        return True
    except LaunchError:
        raise
    except Exception:
        fail("private file unavailable")

def clean_env():
    return {k: os.environ[k] for k in ("PATH", "HOME", "TMPDIR", "LANG") if k in os.environ}

def frontend_environment(node):
    return {**clean_env(), "NODE_ENV":"production", "HOSTNAME":"127.0.0.1", "PORT":"4201", "API_BACKEND_BASE_URL":"http://127.0.0.1:18090/api/v1", "NEXT_TELEMETRY_DISABLED":"1"}

def api_environment(secrets):
    env = clean_env()
    env.update({"API_RUNTIME_PROFILE":"local-kyc-didit-sandbox-manual", "LOCAL_AUTH_PROTOTYPE":"1", "LOCAL_VERIFICATION_IP_LIMIT":"100", "DATABASE_CONNECTION_MODE":"direct", "DATABASE_HOST":"127.0.0.1", "DATABASE_PORT":"5432", "DATABASE_NAME":"espacigo_dms_stage_20261020", "DATABASE_USER":"espacigo_api_login", "DATABASE_SET_ROLE":"espacigo_runtime", "PGPASSFILE":str(ROOT / ".local/testing/kyc-native/api.pgpass"), "HTTP_ADDR":"127.0.0.1:18090", "CORS_ALLOWED_ORIGINS":"https://127.0.0.1:4219", "FRONTEND_PUBLIC_ORIGIN":"https://127.0.0.1:4219", "IDENTITY_DECLARATION_KEYRING_FILE":str(MANUAL_ROOT / "declaration-keyring.json"), "IDENTITY_RUT_HMAC_KEYRING_FILE":str(MANUAL_ROOT / "rut-keyring.json"), "DIDIT_API_KEY":secrets["DIDIT_API_KEY"], "DIDIT_WEBHOOK_SECRET":secrets["DIDIT_WEBHOOK_SECRET"], "DIDIT_WORKFLOW_ID":PUBLISHED_WORKFLOW_ID, "DIDIT_APPLICATION_ID":"806fe3f4-4c9b-4567-bf6f-ce8c3e688fce", "DIDIT_ENVIRONMENT":"SANDBOX", "DIDIT_BASE_URL":"https://verification.didit.me", "DIDIT_REQUIRE_LIVE_FOR_VERIFIED_BADGE":"true", "DIDIT_WORKFLOW_VERSION":"1", "DIDIT_CALLBACK_URL":"https://127.0.0.1:4219/verificaciones/retorno", "LOCAL_SMTP_ADDR":"127.0.0.1:11025"})
    return env

def port_free(port):
    s = socket.socket()
    try:
        s.bind(("127.0.0.1", port))
        return True
    except OSError:
        return False
    finally:
        s.close()

def api_binary_path():
    return MANUAL_ROOT / "bin/api"

def validate_executable(path):
    try:
        st = path.lstat()
        if not stat.S_ISREG(st.st_mode) or st.st_uid != os.getuid() or not st.st_mode & 0o111:
            fail("required service binary is missing or not executable")
    except LaunchError:
        raise
    except Exception:
        fail("required service binary is missing or not executable")

def check_prerequisites(command="check", check_ports=True):
    if command in ("check", "run-api"):
        for p in (MANUAL_ROOT / "declaration-keyring.json", MANUAL_ROOT / "rut-keyring.json", MANUAL_ROOT / "localhost-cert.pem", MANUAL_ROOT / "localhost-key.pem"):
            validate_private_file(p, MANUAL_ROOT)
        parse_didit_env(DIDIT_ENV)
    if command in ("check", "run-frontend"):
        standalone = ROOT / "frontend/.next/standalone"
        if not (standalone / "server.js").is_file() or not (standalone / "public").is_dir() or not (standalone / ".next/static").is_dir():
            fail("frontend standalone artifacts/public/static are missing; parent must build and copy them")
    if command in ("check", "run-api"):
        validate_executable(api_binary_path())
    if command in ("check", "run-mailbox"):
        validate_executable(MANUAL_ROOT / "bin/mailbox")
    node = shutil.which("node")
    if command in ("check", "run-frontend", "run-proxy") and not node:
        fail("Node.js executable not found")
    if check_ports:
        for port in (18090, 11025, 11080, 4201, 4219):
            if not port_free(port):
                fail("required loopback port is occupied")
    return node

def exec_service(command):
    node = check_prerequisites(command, False)
    bind_ports = {"run-api":(18090,), "run-mailbox":(11025, 11080), "run-frontend":(4201,), "run-proxy":(4219,)}
    if any(not port_free(port) for port in bind_ports[command]):
        fail("service loopback port is occupied")
    secrets = parse_didit_env(DIDIT_ENV) if command == "run-api" else None
    if command == "run-api":
        validate_private_file(ROOT / ".local/testing/kyc-native/api.pgpass", ROOT / ".local/testing/kyc-native")
        os.chdir(ROOT)
        os.execve(str(api_binary_path()), ["api"], api_environment(secrets))
    if command == "run-mailbox":
        os.execve(str(MANUAL_ROOT / "bin/mailbox"), ["mailbox"], clean_env())
    if command == "run-frontend":
        os.chdir(ROOT / "frontend/.next/standalone")
        os.execve(node, [node, "server.js"], frontend_environment(node))
    if command == "run-proxy":
        os.execve(node, [node, str(ROOT / "scripts/kyc_manual_https.mjs")], clean_env())
    fail("unknown command")

def main(argv):
    if len(argv) != 1 or argv[0] not in {"check", "run-api", "run-mailbox", "run-frontend", "run-proxy"}:
        print("usage: kyc_manual_local.py check|run-api|run-mailbox|run-frontend|run-proxy", file=sys.stderr)
        return 2
    try:
        if argv[0] == "check":
            node = check_prerequisites("check")
            print("ready: artifacts, private-file metadata, executable and loopback ports verified")
            return 0
        exec_service(argv[0])
    except LaunchError as exc:
        print(f"not ready: {exc}", file=sys.stderr)
        return 1
    except OSError:
        print("not ready: service executable could not be started", file=sys.stderr)
        return 1
    return 0

if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
