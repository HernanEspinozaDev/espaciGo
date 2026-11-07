#!/usr/bin/env python3
"""Fail-closed preflight and isolated empty PostgreSQL trial creation."""
from __future__ import annotations

import argparse
import hashlib
import json
import os
import secrets
import shutil
import socket
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Callable

ROOT = Path(__file__).resolve().parents[1]
PROJECT = "espacigo_wizard_20261106"
DB = PROJECT
CONTAINER = f"{PROJECT}-db"
VOLUME = f"{PROJECT}-pgdata"
NETWORK = f"{PROJECT}-internal"
PORTS = (15436, 18086, 4206, 18087, 11036)
IMAGE = "postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677"
POLICY = ROOT / ".local/testing/test-account-policy.json"
PRIVATE_DIR = ROOT / ".local/testing/wizard-improvements"
TRIAL_DIR = PRIVATE_DIR / "postgres"
ADMIN_FILE = TRIAL_DIR / "admin-password"
PASSFILE = TRIAL_DIR / "pgpass"
RECEIPT = TRIAL_DIR / "create-empty-receipt.json"
BACKUP = TRIAL_DIR / f"{PROJECT}-empty.dump"

Runner = Callable[[list[str], dict[str, str] | None], subprocess.CompletedProcess[str]]


def run(argv: list[str], env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    return subprocess.run(argv, cwd=ROOT, text=True, capture_output=True, check=False, env=env, timeout=30)


def ports_free() -> list[int]:
    busy = []
    for port in PORTS:
        with socket.socket() as sock:
            try:
                sock.bind(("127.0.0.1", port))
            except OSError:
                busy.append(port)
    return busy


def safe_call(runner: Runner, argv: list[str], env: dict[str, str] | None = None) -> subprocess.CompletedProcess[str]:
    # Never relay Docker/libpq output: it may contain environment or credential data.
    return runner(argv, env)


def docker_exists(docker: str, kind: str, name: str, runner: Runner) -> bool:
    if kind == "container":
        argv = [docker, "container", "ls", "-a", "--filter", f"name={name}", "--format", "{{.Names}}"]
    elif kind == "volume":
        argv = [docker, "volume", "ls", "--filter", f"name={name}", "--format", "{{.Name}}"]
    elif kind == "network":
        argv = [docker, "network", "ls", "--filter", f"name={name}", "--format", "{{.Name}}"]
    else:
        raise ValueError(f"unsupported Docker resource kind: {kind}")
    result = safe_call(runner, argv, None)
    if result.returncode != 0:
        raise RuntimeError(f"cannot establish {kind} absence")
    names = result.stdout.splitlines()
    if any(not entry or any(not (ch.isalnum() or ch in "_.-") for ch in entry) for entry in names):
        raise RuntimeError(f"cannot establish {kind} absence")
    return name in names


def write_private(path: Path, data: bytes) -> None:
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(data)
        stream.flush()
        os.fsync(stream.fileno())


def write_failure_receipt(private_dir: Path, stage: str, status: str, error: Exception) -> None:
    if private_dir.is_symlink() or not private_dir.is_dir():
        raise RuntimeError("private receipt directory is absent or unsafe")
    metadata = private_dir.stat()
    if metadata.st_uid != os.getuid() or metadata.st_mode & 0o077:
        raise RuntimeError("private receipt directory must be user-owned and mode 0700")
    receipt_path = private_dir / "create-empty-failure.json"
    if receipt_path.is_symlink():
        raise RuntimeError("failure receipt path is unsafe")
    receipt = {"state": "needs-manual-reconciliation", "target": PROJECT, "stage": stage,
               "status": status, "container": CONTAINER, "volume": VOLUME, "network": NETWORK,
               "container_label": f"com.espacigo.trial={PROJECT}", "cleanup_performed": False,
               "error_class": type(error).__name__}
    write_private(receipt_path, (json.dumps(receipt, sort_keys=True, indent=2) + "\n").encode())
    fd = os.open(private_dir, os.O_RDONLY | getattr(os, "O_DIRECTORY", 0) | getattr(os, "O_NOFOLLOW", 0))
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def require_ok(runner: Runner, argv: list[str], env: dict[str, str] | None = None) -> None:
    if safe_call(runner, argv, env).returncode:
        raise RuntimeError("a required local command failed; output withheld")


def create_empty(runner: Runner = run, sleep: Callable[[float], None] = time.sleep) -> None:
    docker = shutil.which("docker")
    if not docker:
        raise RuntimeError("docker CLI unavailable")
    if not POLICY.is_file():
        raise RuntimeError("private account policy is absent")
    busy = ports_free()
    if busy:
        raise RuntimeError("required loopback ports are occupied")
    if not PRIVATE_DIR.is_dir() or PRIVATE_DIR.is_symlink():
        raise RuntimeError("existing private testing directory is absent or unsafe")
    if PRIVATE_DIR.stat().st_uid != os.getuid() or PRIVATE_DIR.stat().st_mode & 0o077:
        raise RuntimeError("existing private testing directory must be user-owned and mode 0700")
    if TRIAL_DIR.exists() or TRIAL_DIR.is_symlink():
        raise FileExistsError(TRIAL_DIR)
    require_ok(runner, [docker, "info", "--format", "{{.ServerVersion}}"])
    for kind, name in (("container", CONTAINER), ("volume", VOLUME), ("network", NETWORK)):
        if docker_exists(docker, kind, name, runner):
            raise RuntimeError(f"exclusive target already exists: {kind}")
    image = safe_call(runner, [docker, "image", "inspect", IMAGE, "--format", "{{index .RepoDigests 0}}"])
    if image.returncode or image.stdout.strip() != IMAGE:
        raise RuntimeError("pinned Docker image is absent or does not match")

    dockerfile = "/run/secrets/admin"
    stage, status = "trial-directory", "intent"
    try:
        TRIAL_DIR.mkdir(mode=0o700, exist_ok=False)
        os.chmod(TRIAL_DIR, 0o700)
        password = secrets.token_urlsafe(48).encode("ascii")
        stage = "first-credential"
        write_private(ADMIN_FILE, password + b"\n")
        stage = "second-credential"
        write_private(PASSFILE, f"127.0.0.1:15436:{DB}:postgres:{password.decode('ascii')}\n".encode())
        del password
        stage, status = "volume-create", "intent"
        require_ok(runner, [docker, "volume", "create", "--label", f"com.espacigo.trial={PROJECT}", VOLUME])
        status = "command-confirmed"
        stage, status = "network-create", "intent"
        require_ok(runner, [docker, "network", "create", "--internal", "--label", f"com.espacigo.trial={PROJECT}", NETWORK])
        status = "command-confirmed"
        stage, status = "container-create", "intent"
        require_ok(runner, [docker, "create", "--name", CONTAINER,
            "--label", f"com.espacigo.trial={PROJECT}", "--network", NETWORK,
            "--publish", f"127.0.0.1:15436:5432", "--env", "POSTGRES_PASSWORD_FILE=/run/secrets/admin",
            "--env", f"POSTGRES_DB={DB}", "--mount", f"type=volume,src={VOLUME},dst=/var/lib/postgresql",
            "--mount", f"type=bind,src={ADMIN_FILE},dst={dockerfile},readonly", IMAGE])
        status = "command-confirmed"
        stage, status = "container-start", "intent"
        require_ok(runner, [docker, "start", CONTAINER])
        status = "command-confirmed"
        env = {key: os.environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "TMPDIR") if key in os.environ}
        env["PGPASSFILE"] = str(PASSFILE)
        stage, status = "readiness-readback", "attempted-needs-readback"
        ready = False
        for _ in range(60):
            probe = safe_call(runner, ["psql", "-X", "-h", "127.0.0.1", "-p", "15436", "-U", "postgres", "-d", DB, "-w", "-At", "-c", "SELECT current_database(),session_user;"], env)
            if probe.returncode == 0 and probe.stdout.strip() == f"{DB}|postgres":
                ready = True
                break
            sleep(1)
        if not ready:
            raise RuntimeError("PostgreSQL did not become ready with expected identity")
        stage, status = "empty-database-readback", "attempted-needs-readback"
        check = safe_call(runner, ["psql", "-X", "-h", "127.0.0.1", "-p", "15436", "-U", "postgres", "-d", DB, "-w", "-At", "-c", "SELECT current_database() || '|' || session_user || '|' || COALESCE(to_regclass('public.schema_migrations')::text,'') || '|' || COALESCE(to_regclass('public.usuario')::text,'') || '|' || COALESCE(to_regclass('public.espacio')::text,'') || '|' || count(*) FILTER (WHERE d.objid IS NULL) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_depend d ON d.classid='pg_class'::regclass AND d.objid=c.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e' WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f');"], env)
        if check.returncode or check.stdout.strip() != f"{DB}|postgres||||0":
            raise RuntimeError("fresh database readback found application relations or unexpected public objects")
        stage, status = "backup-create", "intent"
        require_ok(runner, ["pg_dump", "-Fc", "-h", "127.0.0.1", "-p", "15436", "-U", "postgres", "-d", DB, "-f", str(BACKUP)], env)
        os.chmod(BACKUP, 0o600)
        stage, status = "backup-verify", "attempted-needs-readback"
        require_ok(runner, ["pg_restore", "--list", str(BACKUP)])
        require_ok(runner, ["pg_restore", "--file=/dev/null", str(BACKUP)])
        digest = hashlib.sha256(BACKUP.read_bytes()).hexdigest()
        receipt = {"target": PROJECT, "container": CONTAINER, "volume": VOLUME, "network": NETWORK,
                   "image": IMAGE, "bind": "127.0.0.1:15436:5432", "database": DB,
                   "readback": "current_database/session_user verified; public application relations absent",
                   "backup": BACKUP.name, "backup_sha256": digest,
                   "created_at_utc": datetime.now(timezone.utc).isoformat()}
        write_private(RECEIPT, (json.dumps(receipt, sort_keys=True, indent=2) + "\n").encode())
    except Exception as exc:
        try:
            write_failure_receipt(PRIVATE_DIR, stage, status, exc)
        except Exception:
            pass
        raise


def resume_readback(runner: Runner = run) -> None:
    """Read back and back up only the already-owned isolated trial."""
    docker = shutil.which("docker")
    if not docker:
        raise RuntimeError("docker CLI unavailable")
    if PRIVATE_DIR.is_symlink() or TRIAL_DIR.is_symlink() or not PRIVATE_DIR.is_dir() or not TRIAL_DIR.is_dir():
        raise RuntimeError("existing private trial directories are absent or unsafe")
    for directory in (PRIVATE_DIR, TRIAL_DIR):
        if directory.stat().st_uid != os.getuid() or directory.stat().st_mode & 0o077:
            raise RuntimeError("private trial directories must be user-owned and mode 0700")
    for path in (ADMIN_FILE, PASSFILE):
        if path.is_symlink() or not path.is_file() or path.stat().st_uid != os.getuid() or path.stat().st_mode & 0o077:
            raise RuntimeError("existing private credential files are absent or unsafe")
    if BACKUP.exists() or BACKUP.is_symlink() or RECEIPT.exists() or RECEIPT.is_symlink():
        raise FileExistsError("refusing to overwrite existing backup or receipt")
    result = safe_call(runner, [docker, "inspect", CONTAINER, "--format", "{{json .}}"])
    if result.returncode:
        raise RuntimeError("existing isolated container inspection failed")
    try:
        item = json.loads(result.stdout)
        labels = item["Config"]["Labels"] or {}
        mounts = item["Mounts"]
        networks = item["NetworkSettings"]["Networks"]
        if (item["Name"] != "/" + CONTAINER or labels.get("com.espacigo.trial") != PROJECT
            or item["Config"]["Image"] != IMAGE or not item["State"]["Running"]
            or not any(m.get("Type") == "volume" and m.get("Name") == VOLUME and m.get("Destination") == "/var/lib/postgresql" for m in mounts)
            or NETWORK not in networks or not networks[NETWORK].get("IPAddress")):
            raise ValueError
    except (KeyError, TypeError, ValueError, json.JSONDecodeError):
        raise RuntimeError("existing container, image, volume, network, labels, or running state did not validate") from None
    env = {key: os.environ[key] for key in ("PATH", "HOME", "LANG", "LC_ALL", "TMPDIR") if key in os.environ}
    env["PGPASSFILE"] = str(PASSFILE)
    psql = ["psql", "-X", "-h", "127.0.0.1", "-p", "15436", "-U", "postgres", "-d", DB, "-w", "-At"]
    ready = safe_call(runner, psql + ["-c", "SELECT current_database(),session_user;"], env)
    if ready.returncode or ready.stdout.strip() != f"{DB}|postgres":
        raise RuntimeError("readback failed; confirm the loopback forward is running")
    check_sql = "SELECT current_database() || '|' || session_user || '|' || COALESCE(to_regclass('public.schema_migrations')::text,'') || '|' || COALESCE(to_regclass('public.usuario')::text,'') || '|' || COALESCE(to_regclass('public.espacio')::text,'') || '|' || count(*) FILTER (WHERE d.objid IS NULL) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace LEFT JOIN pg_depend d ON d.classid='pg_class'::regclass AND d.objid=c.oid AND d.refclassid='pg_extension'::regclass AND d.deptype='e' WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f');"
    check = safe_call(runner, psql + ["-c", check_sql], env)
    if check.returncode or check.stdout.strip() != f"{DB}|postgres||||0":
        raise RuntimeError("readback found application relations or unexpected public objects; no backup created")
    require_ok(runner, ["pg_dump", "-Fc", "-h", "127.0.0.1", "-p", "15436", "-U", "postgres", "-d", DB, "-f", str(BACKUP)], env)
    os.chmod(BACKUP, 0o600)
    require_ok(runner, ["pg_restore", "--list", str(BACKUP)])
    require_ok(runner, ["pg_restore", "--file=/dev/null", str(BACKUP)])
    receipt = {"target": PROJECT, "container": CONTAINER, "volume": VOLUME, "network": NETWORK, "image": IMAGE,
               "bind": "127.0.0.1:15436:5432", "database": DB,
               "readback": "current_database/session_user verified; public application relations absent",
               "backup": BACKUP.name, "backup_sha256": hashlib.sha256(BACKUP.read_bytes()).hexdigest(),
               "created_at_utc": datetime.now(timezone.utc).isoformat()}
    write_private(RECEIPT, (json.dumps(receipt, sort_keys=True, indent=2) + "\n").encode())


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=("preflight", "create-empty", "resume-readback"), nargs="?", default="preflight")
    parser.add_argument("--confirm", default="", help="must exactly match the isolated trial target")
    args = parser.parse_args(argv)
    if args.action == "resume-readback":
        if args.confirm != PROJECT:
            print(f"blocked: resume-readback requires --confirm {PROJECT}", file=sys.stderr)
            return 2
        try:
            resume_readback()
        except Exception as exc:
            print(f"blocked: {exc}", file=sys.stderr)
            return 2
        print(f"readback and backup validated for existing isolated trial {PROJECT}")
        return 0
    if args.action == "create-empty":
        if args.confirm != PROJECT:
            print(f"blocked: create-empty requires --confirm {PROJECT}", file=sys.stderr)
            return 2
        try:
            create_empty()
        except Exception as exc:
            print(f"blocked: {exc}", file=sys.stderr)
            return 2
        print(f"created isolated empty trial {PROJECT}; private receipt and validated backup recorded")
        return 0

    checks: dict[str, object] = {"target": PROJECT, "database": DB, "ports": list(PORTS), "image": IMAGE}
    busy = ports_free()
    checks["loopback_ports_free"] = not busy
    checks["busy_ports"] = busy
    docker = shutil.which("docker")
    checks["docker_available"] = bool(docker)
    if docker:
        result = run([docker, "info", "--format", "{{.ServerVersion}}"])
        checks["docker_daemon_available"] = result.returncode == 0
        if result.returncode == 0:
            checks["docker_server_version"] = result.stdout.strip()
        inspect = run([docker, "image", "inspect", IMAGE, "--format", "{{index .RepoDigests 0}}"])
        checks["pinned_image_present"] = inspect.returncode == 0 and inspect.stdout.strip() == IMAGE
    else:
        checks["docker_daemon_available"] = False
        checks["pinned_image_present"] = False
    checks["private_account_policy_present"] = POLICY.is_file()
    checks["create_empty_status"] = "requires exact confirmation; no application roles, DDL or migrations are performed"
    print(json.dumps(checks, sort_keys=True, indent=2))
    return 0 if not busy else 1


if __name__ == "__main__":
    raise SystemExit(main())
