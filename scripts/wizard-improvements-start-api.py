#!/usr/bin/env python3
"""Fail-closed preflight/launcher for the isolated worker-free wizard API."""
from __future__ import annotations

import argparse
import hashlib
import os
from pathlib import Path
import re
import socket
import stat
import sys
from typing import NoReturn

ROOT = Path(__file__).resolve().parents[1]
TRIAL = ROOT / ".local/testing/wizard-improvements"
API_PORT = 18086
EVIDENCE = TRIAL / "evidence"
PASSFILE = TRIAL / "espacigo_api_login.pgpass"
KEY_FILE = TRIAL / "public-catalog-cursor-key"


def fail(message: str) -> NoReturn:
    raise SystemExit(f"BLOCKED: {message}")


def check_private_dir(path: Path, create: bool = False) -> None:
    if create and not path.exists():
        path.mkdir(mode=0o700, parents=True)
    try:
        info = path.lstat()
    except OSError:
        fail(f"required private directory unavailable: {path}")
    if not stat.S_ISDIR(info.st_mode) or stat.S_ISLNK(info.st_mode):
        fail(f"directory must be a real directory: {path}")
    if info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o700:
        fail(f"directory must be owned by this user and mode 0700: {path}")


def check_passfile(path: Path) -> None:
    try:
        info = path.lstat()
    except OSError:
        fail("API runtime passfile is missing")
    if not stat.S_ISREG(info.st_mode) or stat.S_IMODE(info.st_mode) != 0o600 or info.st_uid != os.geteuid():
        fail("API runtime passfile must be owned regular file mode 0600")


def check_key_file(path: Path) -> None:
    try:
        info = path.lstat()
    except OSError:
        fail("public catalog cursor key is missing")
    if not stat.S_ISREG(info.st_mode) or info.st_uid != os.geteuid() or stat.S_IMODE(info.st_mode) != 0o600:
        fail("public catalog cursor key must be owned regular file mode 0600")
    try:
        key = path.read_text(encoding="ascii")
    except (OSError, UnicodeError):
        fail("public catalog cursor key must be 64 lowercase hexadecimal characters")
    if not re.fullmatch(r"[0-9a-f]{64}", key):
        fail("public catalog cursor key must be 64 lowercase hexadecimal characters")


def port_free(port: int) -> None:
    with socket.socket() as sock:
        try:
            sock.bind(("127.0.0.1", port))
        except OSError:
            fail(f"required loopback port {port} is occupied")


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--binary", type=Path, help="prebuilt API binary (absolute path required)")
    parser.add_argument("--sha256", help="expected SHA-256 of that binary")
    parser.add_argument("--launch", action="store_true", help="exec API; default is preflight only")
    parser.add_argument("--public-catalog", action="store_true", help="explicitly enable the isolated public-catalog trial")
    args = parser.parse_args()

    check_private_dir(TRIAL)
    check_passfile(PASSFILE)
    if args.public_catalog:
        check_key_file(KEY_FILE)
    port_free(API_PORT)
    if not args.binary or not args.binary.is_absolute() or not args.sha256:
        fail("provide --binary ABSOLUTE_PATH and --sha256 from the reviewed build")
    if not re.fullmatch(r"[0-9a-f]{64}", args.sha256):
        fail("--sha256 must be 64 lowercase hexadecimal characters")
    binary = args.binary
    try:
        info = binary.lstat()
    except OSError:
        fail("API binary is unavailable")
    if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode) or info.st_uid != os.geteuid() or not os.access(binary, os.X_OK):
        fail("API binary must be an executable regular file")
    digest = hashlib.sha256(binary.read_bytes()).hexdigest()
    if digest != args.sha256:
        fail("API binary SHA-256 does not match the supplied reviewed digest")

    check_private_dir(EVIDENCE, create=True)

    print(f"PASS: API preflight; binary sha256={digest}; loopback port {API_PORT} free")
    if not args.launch:
        print("No service started. Re-run with --launch only when explicitly requested.")
        return

    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": str(Path.home()),
        "TMPDIR": os.environ.get("TMPDIR") or str(Path.home() / ".hermes/cache/scratch"),
        "API_RUNTIME_PROFILE": "local-space-wizard",
        "LOCAL_AUTH_PROTOTYPE": "1",
        "LOCAL_VERIFICATION_IP_LIMIT": "20",
        "CORS_ALLOWED_ORIGINS": "https://127.0.0.1:4206",
        "FRONTEND_PUBLIC_ORIGIN": "https://127.0.0.1:4206",
        "HTTP_ADDR": "127.0.0.1:18086",
        "DATABASE_CONNECTION_MODE": "direct",
        "DATABASE_HOST": "127.0.0.1",
        "DATABASE_PORT": "15436",
        "DATABASE_NAME": "espacigo_wizard_20261106",
        "DATABASE_USER": "espacigo_api_login",
        "DATABASE_SET_ROLE": "espacigo_runtime",
        "DATABASE_SSLMODE": "disable",
        "PGPASSFILE": str(PASSFILE),
        "M03_EVIDENCE_DIR": str(EVIDENCE),
        "BLOB_STORE_BACKEND": "filesystem",
        "LOCAL_PRIVACY_REPLAY_BACKEND": "filesystem",
        "LOCAL_SMTP_ADDR": "127.0.0.1:11036",
    }
    if args.public_catalog:
        env["LOCAL_PUBLIC_CATALOG_TRIAL"] = "1"
        env["LOCAL_PUBLIC_CATALOG_CURSOR_KEY_FILE"] = str(KEY_FILE.resolve())
    os.execve(str(binary), [str(binary)], env)


if __name__ == "__main__":
    main()
