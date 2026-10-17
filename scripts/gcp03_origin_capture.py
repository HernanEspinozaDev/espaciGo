#!/usr/bin/env python3
"""Gated local GCP-03 origin capture and isolated-copy tooling.

Preflight is read-only. Capture requires a reviewed, run-specific release file;
it never stops PostgreSQL or accesses Google Cloud. See the accompanying plan.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import ipaddress
import json
import os
import re
import secrets
import shutil
import signal
import stat
import subprocess
import sys
import tarfile
import tempfile
import threading
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from datetime import datetime, timezone
from io import BytesIO
from pathlib import Path, PurePosixPath
from typing import Any, BinaryIO, Protocol


FORMAT = "espacigo.gcp03.local-origin-capture.v1"
RELEASE_FORMAT = "espacigo.gcp03.local-origin-capture-release.v1"
RUN_ID_RE = re.compile(r"^[a-z0-9][a-z0-9-]{5,63}$")
IMAGE_DIGEST_REF_RE = re.compile(r"^[^@\s]+@sha256:[0-9a-f]{64}$")
DEFAULT_DB = "espacigo-database-1"
DEFAULT_API = "espacigo-backend-1"
STOP_TIMEOUT_SECONDS = 35
START_TIMEOUT_SECONDS = 60
QUIESCE_TIMEOUT_SECONDS = 180
PG_DUMP_TIMEOUT_SECONDS = 3600
TREE_TIMEOUT_SECONDS = 1800
READINESS_TIMEOUT_SECONDS = 300
DEFAULT_TIMEOUT_PROFILE = {"stop": 35, "start": 60, "readiness": 300, "quiesce": 180, "pg_dump": 900, "private_files": 600, "contract_key": 60, "metadata_sql": 120, "metadata_encrypt": 300, "margin": 300}
TIMEOUT_PROFILE_BOUNDS = {"stop": (10, 35), "start": (10, 60), "readiness": (30, 300), "quiesce": (30, 180), "pg_dump": (60, 3600), "private_files": (60, 1800), "contract_key": (10, 60), "metadata_sql": (30, 120), "metadata_encrypt": (30, 300), "margin": (60, 300)}
MIN_FREE_SPACE_BYTES = 128 * 1024 * 1024
_STAGE = "arguments"


class CaptureError(RuntimeError):
    pass


class CaptureInterrupted(CaptureError):
    pass


def utc_now() -> str:
    return datetime.now(timezone.utc).isoformat()


def strict_json(data: str | bytes) -> Any:
    def unique(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
        out: dict[str, Any] = {}
        for key, value in pairs:
            if key in out:
                raise CaptureError(f"duplicate JSON key: {key}")
            out[key] = value
        return out

    return json.loads(data, object_pairs_hook=unique)


def canonical_json(value: Any) -> bytes:
    return (json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":")) + "\n").encode()


def sha256_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def sha256_file(path: Path) -> tuple[str, int]:
    digest = hashlib.sha256()
    size = 0
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
            size += len(chunk)
    return digest.hexdigest(), size


def normalize_ip_address(value: Any) -> str | None:
    if not isinstance(value, str) or not value:
        return None
    try:
        address = ipaddress.ip_address(value)
    except ValueError:
        return None
    if isinstance(address, ipaddress.IPv6Address) and address.ipv4_mapped:
        return str(address.ipv4_mapped)
    return str(address)


def fsync_file(path: Path) -> None:
    fd = os.open(path, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def mode_exact(path: Path, expected: int = 0o600) -> None:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != expected:
        raise CaptureError("private file mode or file type is invalid")


def mode_one_of(path: Path, expected: set[int]) -> None:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) not in expected:
        raise CaptureError("sealed file mode or file type is invalid")


def private_root(path: Path) -> Path:
    path = path.expanduser()
    if not path.is_absolute():
        raise CaptureError("private storage root must be an absolute path")
    current = Path(path.anchor)
    for part in path.parts[1:]:
        current = current / part
        if current.exists() and current.is_symlink():
            raise CaptureError("private storage root cannot contain symlinks")
    path.mkdir(parents=True, mode=0o700, exist_ok=True)
    resolved = path.resolve(strict=True)
    if resolved != path.absolute() or not resolved.is_dir():
        raise CaptureError("private storage root is invalid")
    os.chmod(resolved, 0o700)
    if stat.S_IMODE(resolved.stat().st_mode) != 0o700:
        raise CaptureError("private storage root mode must be 0700")
    return resolved


def safe_private_file(path: Path) -> None:
    if path.is_symlink() or not path.is_file():
        raise CaptureError("private source file is absent or is not a regular file")
    if stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise CaptureError("private source file mode must be 0600")


def write_new_private(path: Path, data: bytes) -> None:
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    if hasattr(os, "O_NOFOLLOW"):
        flags |= os.O_NOFOLLOW
    fd = os.open(path, flags, 0o600)
    try:
        with os.fdopen(fd, "wb", closefd=False) as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
    finally:
        os.close(fd)
    os.chmod(path, 0o600)


def write_private_replace(path: Path, data: bytes) -> None:
    temp = path.with_name(f".{path.name}.{os.getpid()}.tmp")
    write_new_private(temp, data)
    os.replace(temp, path)
    os.chmod(path, 0o600)
    fd = os.open(path.parent, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def local_testing_path(repo: Path, requested: str) -> Path:
    rel = Path(requested)
    if rel.is_absolute() or rel.parts[:2] != (".local", "testing") or ".." in rel.parts:
        raise CaptureError("private output path must be under .local/testing")
    current = repo
    for part in rel.parts:
        current = current / part
        if current.exists() and current.is_symlink():
            raise CaptureError("private output path contains a symlink")
    return current


def ensure_new_directory(path: Path) -> Path:
    path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    os.chmod(path.parent, 0o700)
    path.mkdir(mode=0o700, exist_ok=False)
    os.chmod(path, 0o700)
    return path


def run_quiet(command: list[str], *, timeout: int = 30, input_bytes: bytes | None = None) -> bytes:
    try:
        result = subprocess.run(command, input=input_bytes, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=timeout, check=False)
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise CaptureError("a required local command failed or timed out") from exc
    if result.returncode != 0:
        raise CaptureError("a required local command returned an error")
    return result.stdout


def docker_inspect(identifier: str) -> dict[str, Any]:
    result = strict_json(run_quiet(["docker", "inspect", identifier], timeout=20))
    if not isinstance(result, list) or len(result) != 1:
        raise CaptureError("Docker container identity was ambiguous")
    return result[0]


def container_env(info: dict[str, Any]) -> dict[str, str]:
    env: dict[str, str] = {}
    for entry in info.get("Config", {}).get("Env", []):
        key, sep, value = entry.partition("=")
        if sep:
            env[key] = value
    return env


def container_descriptor(info: dict[str, Any], repo: Path) -> dict[str, Any]:
    config = info.get("Config", {})
    labels = config.get("Labels") or {}
    state = info.get("State") or {}
    network_map = info.get("NetworkSettings", {}).get("Networks", {}) or {}
    networks = []
    for name, network in sorted(network_map.items()):
        networks.append({
            "name": name,
            "id": network.get("NetworkID", ""),
            "ip_address": network.get("IPAddress", ""),
            "global_ipv6_address": network.get("GlobalIPv6Address", ""),
            "aliases": sorted(network.get("Aliases") or []),
        })
    mounts = []
    for mount in sorted(info.get("Mounts", []), key=lambda item: item.get("Destination", "")):
        mounts.append({
            "type": mount.get("Type", ""),
            "name": mount.get("Name", ""),
            "source": mount.get("Source", ""),
            "destination": mount.get("Destination", ""),
            "read_write": bool(mount.get("RW", False)),
        })
    published_ports = {
        port: bindings
        for port, bindings in (info.get("NetworkSettings", {}).get("Ports") or {}).items()
        if isinstance(bindings, list) and bindings
    }
    return {
        "id": info.get("Id", ""),
        "state": state.get("Status", "unknown"),
        "running": bool(state.get("Running", False)),
        "health": state.get("Health", {}).get("Status", "none"),
        "compose_project": labels.get("com.docker.compose.project", ""),
        "compose_service": labels.get("com.docker.compose.service", ""),
        "compose_working_dir": labels.get("com.docker.compose.project.working_dir", ""),
        "restart_policy": (info.get("HostConfig", {}).get("RestartPolicy") or {}).get("Name", "no"),
        "configured_image": config.get("Image", ""),
        "image_id": info.get("Image", ""),
        "entrypoint_sha256": sha256_bytes(canonical_json(config.get("Entrypoint") or [])),
        "entrypoint_count": len(config.get("Entrypoint") or []),
        "runtime_path": info.get("Path", ""),
        "runtime_args_sha256": sha256_bytes(canonical_json(info.get("Args") or [])),
        "runtime_args_count": len(info.get("Args") or []),
        "networks": networks,
        "mounts": mounts,
        "published_ports": published_ports,
    }


def inspect_image_reference(reference: str, *, missing_ok: bool = False) -> dict[str, Any]:
    """Return local image provenance, distinguishing an absent image from a Docker failure."""
    try:
        result = subprocess.run(
            ["docker", "image", "inspect", reference],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            timeout=20,
            check=False,
        )
    except (OSError, subprocess.TimeoutExpired) as exc:
        raise CaptureError("Docker image provenance inspection failed or timed out") from exc
    if result.returncode != 0:
        # Docker's daemon error is used only to classify a missing local image;
        # never propagate its text, which may contain host-specific details.
        if missing_ok and re.search(rb"(?:Error response from daemon: )?No such image(?::|$)", result.stderr):
            return {"status": "missing_locally", "image_id": reference, "repo_digests": [], "reason_code": "image_not_found"}
        raise CaptureError("Docker image provenance inspection failed")
    value = strict_json(result.stdout)
    if not isinstance(value, list) or len(value) != 1 or not isinstance(value[0], dict):
        raise CaptureError("Docker image provenance response was ambiguous")
    image = value[0]
    return {
        "status": "available",
        "image_id": image.get("Id", ""),
        "repo_digests": sorted(image.get("RepoDigests") or []),
        "reason_code": "",
    }


def validate_restore_image_reference(reference: str) -> dict[str, Any]:
    if not isinstance(reference, str) or not IMAGE_DIGEST_REF_RE.fullmatch(reference):
        raise CaptureError("restore image must be selected by an immutable sha256 digest reference")
    inspected = inspect_image_reference(reference)
    if reference not in inspected["repo_digests"]:
        raise CaptureError("selected restore image digest is not present in the local image provenance")
    if not re.fullmatch(r"sha256:[0-9a-f]{64}", inspected["image_id"]):
        raise CaptureError("selected restore image has no canonical local image ID")
    return {"image_ref": reference, "image_id": inspected["image_id"], "repo_digests": inspected["repo_digests"]}


def database_runtime_binary_version(container_id: str) -> str:
    raw = run_quiet(["docker", "exec", container_id, "postgres", "--version"], timeout=20).decode("utf-8", "strict").strip()
    if not re.fullmatch(r"postgres \(PostgreSQL\) [0-9]+(?:\.[0-9]+)*(?: \([A-Za-z0-9 ._+-]+\))?", raw):
        raise CaptureError("database runtime binary returned an unrecognized version string")
    return raw


def migration_files_from_repo(repo: Path) -> dict[int, dict[str, str]]:
    found: dict[int, dict[str, str]] = {}
    for path in sorted((repo / "db" / "migrations").glob("V[0-9][0-9][0-9][0-9][0-9][0-9]__*.sql")):
        match = re.fullmatch(r"V([0-9]{6})__(.+)\.sql", path.name)
        if not match:
            raise CaptureError("local migration file name is invalid")
        found[int(match.group(1))] = {"name": path.name, "sha256": sha256_file(path)[0]}
    return found


def migration_files_from_image(container_id: str) -> dict[int, dict[str, str]]:
    tar_bytes = run_quiet(["docker", "cp", f"{container_id}:/migrations", "-"], timeout=60)
    found: dict[int, dict[str, str]] = {}
    try:
        with tarfile.open(fileobj=BytesIO(tar_bytes), mode="r:*") as archive:
            for member in archive.getmembers():
                if member.isdir():
                    continue
                name = PurePosixPath(member.name).name
                match = re.fullmatch(r"V([0-9]{6})__(.+)\.sql", name)
                if not match or not member.isfile():
                    raise CaptureError("active image migration tree contains an unsupported entry")
                stream = archive.extractfile(member)
                if stream is None:
                    raise CaptureError("active image migration could not be read")
                digest = hashlib.sha256()
                while chunk := stream.read(1024 * 1024):
                    digest.update(chunk)
                found[int(match.group(1))] = {"name": name, "sha256": digest.hexdigest()}
    except (tarfile.TarError, OSError) as exc:
        raise CaptureError("active image migration tree could not be inspected") from exc
    return found


SQL_CAPTURE = r"""
BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SELECT jsonb_build_object(
  'database',current_database(),
  'server_version_num',current_setting('server_version_num'),
  'encoding',current_setting('server_encoding'),
  'database_size_bytes',pg_database_size(current_database()),
  'extensions',(SELECT coalesce(jsonb_agg(jsonb_build_object('name',extname,'version',extversion) ORDER BY extname),'[]'::jsonb) FROM pg_extension),
  'migration_history',(SELECT coalesce(jsonb_agg(jsonb_build_object('version',version,'name',name,'checksum',checksum) ORDER BY version),'[]'::jsonb) FROM public.schema_migrations),
  'roles',(SELECT coalesce(jsonb_agg(jsonb_build_object('name',rolname,'login',rolcanlogin,'superuser',rolsuper,'create_db',rolcreatedb,'create_role',rolcreaterole,'replication',rolreplication,'bypass_rls',rolbypassrls,'inherit',rolinherit,'connection_limit',rolconnlimit,'valid_until',rolvaliduntil,'config',coalesce((SELECT jsonb_agg(setting ORDER BY setting) FROM unnest(rolconfig) setting WHERE setting !~* '(password|secret|token)'), '[]'::jsonb)) ORDER BY rolname),'[]'::jsonb) FROM pg_roles),
  'memberships',(SELECT coalesce(jsonb_agg(jsonb_build_object('member',member.rolname,'granted_role',granted.rolname,'admin_option',m.admin_option,'inherit_option',m.inherit_option,'set_option',m.set_option) ORDER BY member.rolname,granted.rolname),'[]'::jsonb) FROM pg_auth_members m JOIN pg_roles member ON member.oid=m.member JOIN pg_roles granted ON granted.oid=m.roleid),
  'database_owner_acl',(SELECT jsonb_build_object('owner',owner.rolname,'acl',d.datacl::text,'settings',(SELECT coalesce(jsonb_agg(setting ORDER BY setting),'[]'::jsonb) FROM pg_db_role_setting setting_row CROSS JOIN LATERAL unnest(setting_row.setconfig) setting WHERE setting_row.setdatabase=d.oid AND setting_row.setrole=0)) FROM pg_database d JOIN pg_roles owner ON owner.oid=d.datdba WHERE d.datname=current_database()),
  'schema_owners_acls',(SELECT coalesce(jsonb_agg(jsonb_build_object('schema',n.nspname,'owner',owner.rolname,'acl',n.nspacl::text) ORDER BY n.nspname),'[]'::jsonb) FROM pg_namespace n JOIN pg_roles owner ON owner.oid=n.nspowner WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'),
  'relation_owners_acls',(SELECT coalesce(jsonb_agg(jsonb_build_object('schema',n.nspname,'name',c.relname,'kind',c.relkind::text,'owner',owner.rolname,'acl',c.relacl::text) ORDER BY n.nspname,c.relname),'[]'::jsonb) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_roles owner ON owner.oid=c.relowner WHERE c.relkind IN ('r','p','v','m','S','f') AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'),
  'routine_owners_acls',(SELECT coalesce(jsonb_agg(jsonb_build_object('schema',n.nspname,'name',p.proname,'identity_args',pg_get_function_identity_arguments(p.oid),'owner',owner.rolname,'security_definer',p.prosecdef,'acl',p.proacl::text) ORDER BY n.nspname,p.proname,pg_get_function_identity_arguments(p.oid)),'[]'::jsonb) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_roles owner ON owner.oid=p.proowner WHERE n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'),
  'default_acls',(SELECT coalesce(jsonb_agg(jsonb_build_object('owner',owner.rolname,'schema',n.nspname,'object_type',d.defaclobjtype::text,'acl',d.defaclacl::text) ORDER BY owner.rolname,n.nspname,d.defaclobjtype),'[]'::jsonb) FROM pg_default_acl d JOIN pg_roles owner ON owner.oid=d.defaclrole LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace),
  'clients',(SELECT coalesce(jsonb_agg(jsonb_build_object('address',client_addr::text,'database',datname,'user',usename,'application',application_name,'state',coalesce(state,'unknown')) ORDER BY client_addr,datname,usename,application_name),'[]'::jsonb) FROM pg_stat_activity WHERE backend_type='client backend' AND datname=current_database() AND pid<>pg_backend_pid()),
  'leases',(SELECT jsonb_build_object('outbox',coalesce((SELECT count(*) FROM public.outbox_evento_local WHERE lease_hasta>transaction_timestamp()),0),'notices',coalesce((SELECT count(*) FROM public.aviso_local WHERE estado='procesando' AND lease_hasta>transaction_timestamp()),0))),
  'columns',(SELECT jsonb_build_object('uuid',count(*) FILTER(WHERE t.typname='uuid'),'int64',count(*) FILTER(WHERE t.typname='int8')) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_type t ON t.oid=a.atttypid WHERE a.attnum>0 AND NOT a.attisdropped AND c.relkind IN ('r','p','v','m') AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'),
  'security_definer_count',(SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE p.prosecdef AND n.nspname NOT LIKE 'pg_%' AND n.nspname<>'information_schema'),
  'notice_states',(SELECT coalesce(jsonb_object_agg(estado,c),'{}'::jsonb) FROM (SELECT estado,count(*) c FROM public.aviso_local GROUP BY estado) s),
  'sessions',(SELECT jsonb_build_object('total',count(*),'valid',count(*) FILTER(WHERE revocada_en IS NULL AND expira_en>transaction_timestamp())) FROM public.sesion),
  'tokens',(SELECT jsonb_build_object('total',count(*),'usable',count(*) FILTER(WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en>transaction_timestamp())) FROM public.token_accion),
  'core_counts',jsonb_build_array(
    jsonb_build_object('table','usuario','rows',(SELECT count(*) FROM public.usuario)),
    jsonb_build_object('table','rol_usuario','rows',(SELECT count(*) FROM public.rol_usuario)),
    jsonb_build_object('table','espacio','rows',(SELECT count(*) FROM public.espacio)),
    jsonb_build_object('table','reserva_ensayo_local','rows',(SELECT count(*) FROM public.reserva_ensayo_local)),
    jsonb_build_object('table','ocupacion','rows',(SELECT count(*) FROM public.ocupacion)),
    jsonb_build_object('table','sesion','rows',(SELECT count(*) FROM public.sesion)),
    jsonb_build_object('table','token_accion','rows',(SELECT count(*) FROM public.token_accion)),
    jsonb_build_object('table','outbox_evento_local','rows',(SELECT count(*) FROM public.outbox_evento_local)),
    jsonb_build_object('table','aviso_local','rows',(SELECT count(*) FROM public.aviso_local)),
    jsonb_build_object('table','reserva_pago_ensayo_operacion','rows',(SELECT count(*) FROM public.reserva_pago_ensayo_operacion)),
    jsonb_build_object('table','reserva_pago_evento_ensayo','rows',(SELECT count(*) FROM public.reserva_pago_evento_ensayo)),
    jsonb_build_object('table','contrato_ensayo_local','rows',(SELECT count(*) FROM public.contrato_ensayo_local)),
    jsonb_build_object('table','documento_privado_sintetico_local','rows',(SELECT count(*) FROM public.documento_privado_sintetico_local)),
    jsonb_build_object('table','verificacion','rows',(SELECT count(*) FROM public.verificacion))
  )
)::text;
COMMIT;
"""


def _database_query(container_id: str, password_file: str, sql: str, timeout: int = 120) -> dict[str, Any]:
    if not password_file.startswith("/run/secrets/"):
        raise CaptureError("database auth file is outside the mounted secret directory")
    shell = (
        'password="$(cat "$POSTGRES_PASSWORD_FILE")"; '
        'export PGPASSWORD="$password" PGAPPNAME=gcp03-origin-capture; unset password; '
        'exec psql -h 127.0.0.1 -XqAt -v ON_ERROR_STOP=1 '
        '--username="$POSTGRES_USER" --dbname="$POSTGRES_DB" -f -'
    )
    output = run_quiet(["docker", "exec", "-i", container_id, "sh", "-eu", "-c", shell], timeout=timeout, input_bytes=sql.encode())
    lines = [line for line in output.decode().splitlines() if line.strip()]
    if len(lines) != 1:
        raise CaptureError("read-only database inventory returned an unexpected result")
    parsed = strict_json(lines[0])
    if not isinstance(parsed, dict):
        raise CaptureError("read-only database inventory is not a JSON object")
    return parsed


def _mount_source(descriptor: dict[str, Any], destination: str, *, mount_type: str | None = None) -> Path:
    found = [m for m in descriptor["mounts"] if m["destination"] == destination]
    if len(found) != 1 or (mount_type and found[0]["type"] != mount_type):
        raise CaptureError("required source mount is missing or ambiguous")
    path = Path(found[0]["source"])
    if not path.is_absolute() or path.is_symlink():
        raise CaptureError("source mount path is not an absolute real path")
    return path


def inventory_tree(root: Path, *, exact_private_modes: bool = True) -> list[dict[str, Any]]:
    if root.is_symlink() or not root.is_dir():
        raise CaptureError("source artifact root is absent or is a symlink")
    if exact_private_modes and stat.S_IMODE(root.stat().st_mode) != 0o700:
        raise CaptureError("source artifact directory mode must be 0700")
    result: list[dict[str, Any]] = []
    for path in sorted(root.rglob("*")):
        info = path.lstat()
        if stat.S_ISLNK(info.st_mode) or not (stat.S_ISDIR(info.st_mode) or stat.S_ISREG(info.st_mode)):
            raise CaptureError("source file tree contains a symlink or unsupported file type")
        if stat.S_ISDIR(info.st_mode):
            if exact_private_modes and stat.S_IMODE(info.st_mode) != 0o700:
                raise CaptureError("source directory mode must be 0700")
            continue
        mode = stat.S_IMODE(info.st_mode)
        if exact_private_modes and mode != 0o600:
            raise CaptureError("source file mode must be 0600")
        digest, size = sha256_file(path)
        result.append({"relative_path": path.relative_to(root).as_posix(), "mode_octal": f"{mode:04o}", "bytes": size, "sha256": digest})
    return result


def require_safe_relative(name: str) -> PurePosixPath:
    path = PurePosixPath(name)
    if path.is_absolute() or not path.parts or any(part in ("", ".", "..") for part in path.parts):
        raise CaptureError("archive path escapes its declared source root")
    return path


def source_tree_tar(source_roots: dict[str, Path], destination: BinaryIO) -> list[dict[str, Any]]:
    manifest: list[dict[str, Any]] = []
    roots: list[tuple[str, Path, list[dict[str, Any]]]] = []
    for label, root in sorted(source_roots.items()):
        if not re.fullmatch(r"[a-z][a-z0-9_-]{1,40}", label):
            raise CaptureError("archive root label is invalid")
        entries = inventory_tree(root)
        roots.append((label, root, entries))
        manifest.extend({"root": label, **item} for item in entries)
    with tarfile.open(fileobj=destination, mode="w|") as archive:
        added_directories: set[str] = set()
        for label, root, entries in roots:
            root_info = tarfile.TarInfo(label)
            root_info.type = tarfile.DIRTYPE
            root_info.mode = 0o700
            root_info.mtime = 0
            archive.addfile(root_info)
            added_directories.add(label)
            expected = {item["relative_path"]: item for item in entries}
            for relative in sorted(expected):
                posix = require_safe_relative(relative)
                source = root.joinpath(*posix.parts)
                resolved_root = root.resolve(strict=True)
                resolved_source = source.resolve(strict=True)
                if not resolved_source.is_relative_to(resolved_root):
                    raise CaptureError("source file path escaped its declared root")
                info = source.lstat()
                if not stat.S_ISREG(info.st_mode) or stat.S_ISLNK(info.st_mode):
                    raise CaptureError("source tree changed or contains a symlink during capture")
                parent = PurePosixPath(label, *posix.parts[:-1])
                for index in range(1, len(parent.parts) + 1):
                    parent_name = PurePosixPath(*parent.parts[:index]).as_posix()
                    if parent_name in added_directories:
                        continue
                    directory = tarfile.TarInfo(parent_name)
                    directory.type = tarfile.DIRTYPE
                    directory.mode = 0o700
                    directory.mtime = 0
                    archive.addfile(directory)
                    added_directories.add(parent_name)
                file_info = tarfile.TarInfo(PurePosixPath(label, *posix.parts).as_posix())
                file_info.size = info.st_size
                file_info.mode = stat.S_IMODE(info.st_mode)
                file_info.mtime = 0
                file_info.uid = file_info.gid = 0
                file_info.uname = file_info.gname = ""
                flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
                fd = os.open(source, flags)
                class HashingReader:
                    def __init__(self, stream: BinaryIO):
                        self.stream = stream
                        self.digest = hashlib.sha256()
                        self.size = 0
                    def read(self, count: int = -1) -> bytes:
                        chunk = self.stream.read(count)
                        self.digest.update(chunk)
                        self.size += len(chunk)
                        return chunk
                try:
                    with os.fdopen(fd, "rb") as raw_stream:
                        hashing_reader = HashingReader(raw_stream)
                        archive.addfile(file_info, hashing_reader)
                finally:
                    try:
                        os.close(fd)
                    except OSError:
                        pass
                source_record = expected[relative]
                if hashing_reader.size != source_record["bytes"] or hashing_reader.digest.hexdigest() != source_record["sha256"]:
                    raise CaptureError("source file changed while it was being encrypted")
        meta = canonical_json({"format": f"{FORMAT}.files", "files": manifest})
        info = tarfile.TarInfo("capture-file-manifest.json")
        info.size = len(meta)
        info.mode = 0o600
        info.mtime = 0
        info.uid = info.gid = 0
        archive.addfile(info, BytesIO(meta))
    return manifest


def derive_passphrase(master: bytes, run_id: str, label: str) -> bytes:
    return hmac.new(master, f"{FORMAT}:{run_id}:{label}".encode(), hashlib.sha256).hexdigest().encode()


def gpg_encrypt_producer(producer: Any, output: Path, passphrase: bytes, timeout: int) -> dict[str, Any]:
    if output.exists() or output.is_symlink():
        raise CaptureError("encrypted artifact destination already exists")
    read_fd, write_fd = os.pipe()
    command = ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--symmetric", "--force-aead", "--cipher-algo", "AES256", "--output", str(output)]
    previous_umask = os.umask(0o077)
    try:
        process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
    except OSError as exc:
        os.close(read_fd)
        os.close(write_fd)
        raise CaptureError("GnuPG with authenticated encryption is unavailable") from exc
    finally:
        os.umask(previous_umask)
    os.close(read_fd)
    timer_fired = threading.Event()
    def expire() -> None:
        timer_fired.set()
        process.kill()
    timer = threading.Timer(timeout, expire)
    timer.start()
    try:
        os.write(write_fd, passphrase + b"\n")
        os.close(write_fd)
        write_fd = -1
        assert process.stdin is not None
        plain_meta = producer(process.stdin)
        process.stdin.close()
        rc = process.wait(timeout=timeout + 5)
        if timer_fired.is_set() or rc != 0:
            raise CaptureError("authenticated encryption failed or exceeded its deadline")
    except BaseException:
        if process.stdin is not None and not process.stdin.closed:
            process.stdin.close()
        try:
            process.kill()
        except OSError:
            pass
        process.wait()
        raise
    finally:
        timer.cancel()
        if write_fd >= 0:
            os.close(write_fd)
    os.chmod(output, 0o600)
    fsync_file(output)
    cipher_hash, cipher_size = sha256_file(output)
    return plain_meta | {"cipher_sha256": cipher_hash, "cipher_bytes": cipher_size, "encryption": "GnuPG AES-256 AEAD; per-artifact HMAC-derived passphrase"}


def gpg_encrypt_stream(source: BinaryIO, output: Path, passphrase: bytes, timeout: int) -> dict[str, Any]:
    if output.exists() or output.is_symlink():
        raise CaptureError("encrypted artifact destination already exists")
    read_fd, write_fd = os.pipe()
    command = ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--symmetric", "--force-aead", "--cipher-algo", "AES256", "--output", str(output)]
    previous_umask = os.umask(0o077)
    try:
        process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
    except OSError as exc:
        os.close(read_fd)
        os.close(write_fd)
        raise CaptureError("GnuPG with authenticated encryption is unavailable") from exc
    finally:
        os.umask(previous_umask)
    os.close(read_fd)
    timer_fired = threading.Event()

    def expire() -> None:
        timer_fired.set()
        process.kill()

    timer = threading.Timer(timeout, expire)
    timer.start()
    try:
        os.write(write_fd, passphrase + b"\n")
        os.close(write_fd)
        write_fd = -1
        assert process.stdin is not None
        digest = hashlib.sha256()
        plain_bytes = 0
        while chunk := source.read(1024 * 1024):
            digest.update(chunk)
            plain_bytes += len(chunk)
            process.stdin.write(chunk)
        process.stdin.close()
        rc = process.wait(timeout=timeout + 5)
        if timer_fired.is_set() or rc != 0:
            raise CaptureError("authenticated encryption failed or exceeded its deadline")
    except BaseException:
        if process.stdin is not None and not process.stdin.closed:
            process.stdin.close()
        try:
            process.kill()
        except OSError:
            pass
        process.wait()
        raise
    finally:
        timer.cancel()
        if write_fd >= 0:
            os.close(write_fd)
    os.chmod(output, 0o600)
    fsync_file(output)
    cipher_hash, cipher_size = sha256_file(output)
    return {"cipher_sha256": cipher_hash, "cipher_bytes": cipher_size, "plain_sha256": digest.hexdigest(), "plain_bytes": plain_bytes, "encryption": "GnuPG AES-256 AEAD; per-artifact HMAC-derived passphrase"}


def gpg_decrypt_file(source: Path, output: Path, passphrase: bytes, timeout: int = 3600) -> None:
    if not source.is_file() or source.is_symlink() or output.exists() or output.is_symlink():
        raise CaptureError("encrypted input is invalid or recovery output already exists")
    temp_output = output.with_name(f".{output.name}.{os.getpid()}.partial")
    if temp_output.exists() or temp_output.is_symlink():
        raise CaptureError("recovery temporary destination already exists")
    read_fd, write_fd = os.pipe()
    command = ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--decrypt", "--output", str(temp_output), str(source)]
    previous_umask = os.umask(0o077)
    try:
        process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
    except OSError as exc:
        os.close(read_fd)
        os.close(write_fd)
        raise CaptureError("GnuPG recovery command is unavailable") from exc
    finally:
        os.umask(previous_umask)
    os.close(read_fd)
    try:
        os.write(write_fd, passphrase + b"\n")
        os.close(write_fd)
        write_fd = -1
        rc = process.wait(timeout=timeout)
        if rc != 0:
            raise CaptureError("authenticated decryption failed")
        os.chmod(temp_output, 0o600)
        fsync_file(temp_output)
        os.link(temp_output, output, follow_symlinks=False)
        temp_output.unlink()
    except BaseException:
        try:
            process.kill()
        except OSError:
            pass
        process.wait()
        if temp_output.exists() and not temp_output.is_symlink():
            temp_output.unlink()
        raise
    finally:
        if write_fd >= 0:
            os.close(write_fd)
    os.chmod(output, 0o600)


def gpg_symmetric_decrypt_stream(source: Path, destination: BinaryIO, passphrase: bytes, timeout: int = 3600) -> dict[str, Any]:
    if not source.is_file() or source.is_symlink():
        raise CaptureError("encrypted artifact is absent or unsafe")
    read_fd, write_fd = os.pipe()
    command = ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--decrypt", str(source)]
    process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
    os.close(read_fd)
    try:
        os.write(write_fd, passphrase + b"\n")
        os.close(write_fd)
        write_fd = -1
        assert process.stdout is not None
        digest, size = hashlib.sha256(), 0
        while chunk := process.stdout.read(1024 * 1024):
            destination.write(chunk)
            digest.update(chunk)
            size += len(chunk)
        rc = process.wait(timeout=timeout)
        if rc != 0:
            raise CaptureError("authenticated decryption failed")
        return {"sha256": digest.hexdigest(), "bytes": size}
    except BaseException:
        process.kill()
        process.wait()
        raise
    finally:
        if write_fd >= 0:
            os.close(write_fd)


def create_recovery_key(path: Path) -> bytes:
    path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    os.chmod(path.parent, 0o700)
    if path.exists() or path.is_symlink():
        raise CaptureError("run recovery key already exists")
    key = secrets.token_bytes(32)
    write_new_private(path, key.hex().encode() + b"\n")
    return key


def load_recovery_key(path: Path) -> bytes:
    safe_private_file(path)
    raw = path.read_bytes().strip()
    if not re.fullmatch(rb"[0-9a-f]{64}", raw):
        raise CaptureError("recovery key file format is invalid")
    return bytes.fromhex(raw.decode())


class SnapshotPort(Protocol):
    def preflight(self) -> dict[str, Any]: ...
    def validate_restore_target(self, restore_target: dict[str, Any]) -> None: ...
    def stop_container(self, container_id: str, timeout: int) -> None: ...
    def start_container(self, container_id: str, timeout: int) -> None: ...
    def stop_event_status(self, container_id: str, since: str) -> str: ...
    def container_state(self, container_id: str) -> str: ...
    def wait_quiescent(self, timeout: int) -> dict[str, Any]: ...
    def wait_ready(self, container_id: str, timeout: int, expected_health: str) -> None: ...
    def stream_database_dump(self, destination: BinaryIO, timeout: int) -> dict[str, Any]: ...
    def metadata_report(self, timeout: int = 120) -> dict[str, Any]: ...
    def source_roots(self) -> dict[str, Path]: ...
    def source_contract_key(self) -> BinaryIO: ...


@dataclass
class CaptureLayout:
    snapshot_root: Path
    custody_root: Path
    journal_root: Path


def release_validate(release: dict[str, Any], run_id: str, head_sha: str) -> list[str]:
    if release.get("format") != RELEASE_FORMAT or release.get("status") != "approved":
        raise CaptureError("capture requires an approved run-specific release record")
    if release.get("run_id") != run_id or not RUN_ID_RE.fullmatch(run_id):
        raise CaptureError("release run-id does not match explicit confirmation")
    if release.get("reviewed_head_sha") != head_sha or not re.fullmatch(r"[0-9a-f]{40}", head_sha):
        raise CaptureError("release must approve the exact currently checked out reviewed HEAD")
    if release.get("reviewed_by") != "HernanEspinozaDev":
        raise CaptureError("release record lacks the required independent reviewer")
    if release.get("source_change_window_authorized") is not True:
        raise CaptureError("release record does not authorize this exact source change window")
    if release.get("all_ingress_and_writer_sources_enumerated") is not True:
        raise CaptureError("release record must attest complete ingress and writer enumeration")
    if release.get("exclusive_docker_control") is not True:
        raise CaptureError("release record must attest exclusive Docker control during quiesce and recovery")
    if release.get("external_ingress_and_writer_sources") != []:
        raise CaptureError("capture cannot quiesce external writers or ingress; release must enumerate none")
    try:
        window_start = datetime.fromisoformat(release["window_start_utc"].replace("Z", "+00:00"))
        window_end = datetime.fromisoformat(release["window_end_utc"].replace("Z", "+00:00"))
    except (KeyError, AttributeError, TypeError, ValueError) as exc:
        raise CaptureError("release must specify an exact UTC maintenance window") from exc
    if window_start.tzinfo is None or window_end.tzinfo is None or window_start.utcoffset() != timezone.utc.utcoffset(None) or window_end.utcoffset() != timezone.utc.utcoffset(None) or window_end <= window_start or (window_end - window_start).total_seconds() > 4 * 60 * 60:
        raise CaptureError("release maintenance window is invalid or exceeds four hours")
    now = datetime.now(timezone.utc)
    if not window_start <= now <= window_end:
        raise CaptureError("current time is outside the reviewed maintenance window")
    source = release.get("source") or {}
    if not source.get("database_container_id") or not source.get("expected_source_fingerprint"):
        raise CaptureError("release source identity is incomplete")
    restore_target = release.get("restore_target") or {}
    if (
        not IMAGE_DIGEST_REF_RE.fullmatch(str(restore_target.get("image_ref", "")))
        or not re.fullmatch(r"sha256:[0-9a-f]{64}", str(restore_target.get("image_id", "")))
        or not isinstance(restore_target.get("repo_digests"), list)
        or restore_target.get("image_ref", "") not in restore_target.get("repo_digests", [])
    ):
        raise CaptureError("release must pin a separately selected local restore image by digest")
    order = release.get("quiesce_order")
    if not isinstance(order, list) or not order or any(not isinstance(item, str) for item in order):
        raise CaptureError("release stop sequence is empty or malformed")
    if source["database_container_id"] in order or len(set(order)) != len(order):
        raise CaptureError("release may not stop PostgreSQL and stop IDs must be unique")
    validate_timeout_profile(release.get("timeout_profile"))
    return order


def resolve_stop_intents(journal: dict[str, Any], port: SnapshotPort, health_map: dict[str, Any], *, timeout_profile: dict[str, int] | None = None) -> list[str]:
    """Resume only services whose stop was owned by this run; prove readiness for every intent."""
    profile = timeout_profile or DEFAULT_TIMEOUT_PROFILE
    owned = set(journal.get("stopped_by_this_run", []))
    resolved = set(journal.get("resolved_stop_intents", []))
    unresolved: list[str] = []
    for container_id in reversed(journal.get("stop_intent_ids", [])):
        if container_id in resolved:
            continue
        try:
            event_status = port.stop_event_status(container_id, journal["started_at_utc"])
            state = port.container_state(container_id)
        except BaseException:
            unresolved.append(container_id)
            continue
        owned_stop = container_id in owned or event_status == "confirmed"
        try:
            if state == "running":
                port.wait_ready(container_id, profile["readiness"], health_map.get(container_id, "none"))
            elif owned_stop:
                port.start_container(container_id, profile["start"])
                port.wait_ready(container_id, profile["readiness"], health_map.get(container_id, "none"))
                started = set(journal.get("restored_by_this_run", []))
                started.add(container_id)
                journal["restored_by_this_run"] = [item for item in journal.get("quiesce_order", []) if item in started]
            else:
                unresolved.append(container_id)
                continue
            resolved.add(container_id)
            journal["resolved_stop_intents"] = [item for item in journal.get("stop_intent_ids", []) if item in resolved]
            journal["unresolved_stop_intents"] = [item for item in journal.get("stop_intent_ids", []) if item not in resolved]
        except BaseException:
            unresolved.append(container_id)
    journal["resolved_stop_intents"] = [item for item in journal.get("stop_intent_ids", []) if item in resolved]
    journal["unresolved_stop_intents"] = [item for item in journal.get("stop_intent_ids", []) if item not in resolved]
    return unresolved


def validate_timeout_profile(profile: Any) -> dict[str, int]:
    if not isinstance(profile, dict) or set(profile) != set(TIMEOUT_PROFILE_BOUNDS):
        raise CaptureError("release must contain the complete bounded timeout profile")
    validated: dict[str, int] = {}
    for name, (minimum, maximum) in TIMEOUT_PROFILE_BOUNDS.items():
        value = profile.get(name)
        if isinstance(value, bool) or not isinstance(value, int) or not minimum <= value <= maximum:
            raise CaptureError(f"release timeout profile value is outside reviewed bounds: {name}")
        validated[name] = value
    return validated


def capture_budget_seconds(candidate_count: int, profile: dict[str, int] | None = None) -> int:
    profile = validate_timeout_profile(profile or DEFAULT_TIMEOUT_PROFILE)
    fixed_stages = sum(profile[name] for name in ("quiesce", "pg_dump", "private_files", "contract_key", "metadata_sql", "metadata_encrypt", "margin"))
    per_service = profile["stop"] + 15 + profile["start"] + profile["readiness"]
    return fixed_stages + candidate_count * per_service


class CaptureCoordinator:
    def __init__(self, port: SnapshotPort, layout: CaptureLayout):
        self.port = port
        self.layout = layout

    def capture(self, release: dict[str, Any], run_id: str, head_sha: str, *, free_bytes: int | None = None) -> dict[str, Any]:
        order = release_validate(release, run_id, head_sha)
        timeout_profile = validate_timeout_profile(release["timeout_profile"])
        self.layout.snapshot_root = private_root(self.layout.snapshot_root)
        self.layout.custody_root = private_root(self.layout.custody_root)
        self.layout.journal_root = private_root(self.layout.journal_root)
        final = self.layout.snapshot_root / run_id
        partial = self.layout.snapshot_root / f"{run_id}.partial"
        journal_path = self.layout.journal_root / f"{run_id}.journal.json"
        if final.exists() or final.is_symlink() or partial.exists() or partial.is_symlink() or journal_path.exists() or journal_path.is_symlink():
            raise CaptureError("snapshot run-id already has output; snapshots are never overwritten")

        self.port.validate_restore_target(release["restore_target"])
        preflight = self.port.preflight()
        if preflight.get("source_fingerprint") != release["source"]["expected_source_fingerprint"]:
            raise CaptureError("source identity/configuration changed after the release candidate was reviewed")
        source_ids = {item["id"] for item in preflight.get("quiesce_candidates", []) if item.get("running")}
        if set(order) != source_ids:
            raise CaptureError("release container list is incomplete or contains an unknown writer/ingress")
        db_id = preflight.get("database_container_id")
        if not db_id or db_id == "" or db_id in order or release["source"].get("database_container_id") != db_id:
            raise CaptureError("database container must remain running throughout capture")
        db_volume_ids = set(preflight.get("database_volume_ids", []))
        if not db_volume_ids or not db_volume_ids.issubset(set(preflight.get("mounted_volume_ids", []))):
            raise CaptureError("persistent database volume identity could not be verified")
        window_end = datetime.fromisoformat(release["window_end_utc"].replace("Z", "+00:00"))
        if (window_end - datetime.now(timezone.utc)).total_seconds() < capture_budget_seconds(len(order), timeout_profile):
            raise CaptureError("remaining maintenance window is shorter than the bounded capture and resume envelope")

        estimate = int(preflight.get("estimated_private_bytes", 0))
        required_space = max(MIN_FREE_SPACE_BYTES, estimate * 2 + MIN_FREE_SPACE_BYTES)
        if free_bytes is None:
            free_bytes = shutil.disk_usage(self.layout.snapshot_root).free
        if free_bytes < required_space:
            raise CaptureError("private destination lacks conservative free-space reserve")

        partial.mkdir(mode=0o700, exist_ok=False)
        os.chmod(partial, 0o700)
        custody_dir = self.layout.custody_root / run_id
        custody_dir.mkdir(mode=0o700, exist_ok=False)
        os.chmod(custody_dir, 0o700)
        recovery_path = custody_dir / "recovery.key"
        recovery_key = create_recovery_key(recovery_path)
        started_at = utc_now()
        journal: dict[str, Any] = {
            "format": f"{FORMAT}.quiesce-journal",
            "run_id": run_id,
            "started_at_utc": started_at,
            "database_container_id": db_id,
            "database_volume_ids": sorted(db_volume_ids),
            "partial_snapshot": partial.name,
            "quiesce_order": order,
            "timeout_profile": timeout_profile,
            "initial_health": preflight.get("container_health", {}),
            "stop_intent_ids": [],
            "stopped_by_this_run": [],
            "restored_by_this_run": [],
            "resolved_stop_intents": [],
            "unresolved_stop_intents": [],
            "status": "prepared",
        }
        write_new_private(journal_path, canonical_json(journal))
        stage = "quiesce"
        primary_error: BaseException | None = None
        stop_confirmed: list[str] = []
        artifact_meta: dict[str, Any] = {}
        files_manifest: list[dict[str, Any]] = []
        try:
            journal["status"] = "quiescing"
            write_private_replace(journal_path, canonical_json(journal))
            for container_id in order:
                journal["stop_intent_ids"].append(container_id)
                write_private_replace(journal_path, canonical_json(journal))
                self.port.stop_container(container_id, timeout_profile["stop"])
                stop_confirmed.append(container_id)
                journal["stopped_by_this_run"].append(container_id)
                write_private_replace(journal_path, canonical_json(journal))

            stage = "quiesce evidence"
            quiet = self.port.wait_quiescent(timeout_profile["quiesce"])
            if quiet.get("unknown_clients") or int(quiet.get("active_writers", -1)) != 0 or int(quiet.get("active_leases", -1)) != 0:
                raise CaptureError("source connections or leases are not fully quiescent")
            journal["quiesce_evidence"] = quiet
            journal["status"] = "quiescent"
            write_private_replace(journal_path, canonical_json(journal))

            stage = "database dump"
            db_temp = partial / "database.dump.gpg"
            artifact_meta["database.dump.gpg"] = gpg_encrypt_producer(
                lambda encrypted_stream: self.port.stream_database_dump(encrypted_stream, timeout_profile["pg_dump"]),
                db_temp,
                derive_passphrase(recovery_key, run_id, "database-dump"),
                timeout_profile["pg_dump"],
            )

            stage = "copy private file trees"
            files_path = partial / "private-files.tar.gpg"
            tree_meta: dict[str, Any] = {}
            def tar_producer(encrypted_stream: BinaryIO) -> dict[str, Any]:
                nonlocal files_manifest
                files_manifest = source_tree_tar(self.port.source_roots(), encrypted_stream)
                expected = [
                    {"root": root_label, **entry}
                    for root_label, inventory_key in (("private-evidence", "evidence_files"), ("privacy-replay", "privacy_replay_files"))
                    for entry in preflight.get("source_projection", {}).get(inventory_key, [])
                ]
                sort_key = lambda item: (item["root"], item["relative_path"])
                if canonical_json(sorted(files_manifest, key=sort_key)) != canonical_json(sorted(expected, key=sort_key)):
                    raise CaptureError("private files changed between reviewed preflight and quiesced capture")
                tree_meta.update({"file_count": len(files_manifest), "source_tree_sha256": sha256_bytes(canonical_json(files_manifest))})
                return tree_meta
            encrypted_tree = gpg_encrypt_producer(tar_producer, files_path, derive_passphrase(recovery_key, run_id, "private-files"), timeout_profile["private_files"])
            artifact_meta["private-files.tar.gpg"] = encrypted_tree

            stage = "contract key custody encryption"
            key_path = partial / "contract-encryption-key.gpg"
            with self.port.source_contract_key() as source_key:
                artifact_meta["contract-encryption-key.gpg"] = gpg_encrypt_stream(source_key, key_path, derive_passphrase(recovery_key, run_id, "contract-key"), timeout_profile["contract_key"])

            stage = "private metadata encryption"
            metadata = self.port.metadata_report(timeout_profile["metadata_sql"])
            if metadata.get("source_credentials_included") or "role_passwords" in metadata:
                raise CaptureError("metadata report contains forbidden credential data")
            metadata_path = partial / "origin-metadata.json.gpg"
            stream = BytesIO(canonical_json({"preflight": preflight, "database_report": metadata, "file_inventory": files_manifest, "release": release}))
            artifact_meta["origin-metadata.json.gpg"] = gpg_encrypt_stream(stream, metadata_path, derive_passphrase(recovery_key, run_id, "metadata"), timeout_profile["metadata_encrypt"])

            stage = "integrity sealing"
            seal = {
                "format": f"{FORMAT}.sealed",
                "run_id": run_id,
                "created_at_utc": utc_now(),
                "reviewed_head_sha": head_sha,
                "source_fingerprint": preflight["source_fingerprint"],
                "encrypted_artifacts": artifact_meta,
                "custody_key_in_artifact": False,
                "database_volume_in_artifact": False,
                "v1_v61_preserved": bool(metadata.get("migration_match_v1_v61")),
                "source_secret_files_included": False,
                "contract_encryption_key_separately_encrypted": True,
            }
            if not seal["v1_v61_preserved"]:
                raise CaptureError("migration V1-V61 checksum contract failed")
            write_new_private(partial / "SEALED.json", canonical_json(seal))
            write_new_private(partial / "SEALED.sha256", (sha256_file(partial / "SEALED.json")[0] + "  SEALED.json\n").encode())
            journal["status"] = "captured_pending_resume"
            journal["artifact_seal_sha256"] = sha256_file(partial / "SEALED.json")[0]
            write_private_replace(journal_path, canonical_json(journal))
        except BaseException as exc:
            primary_error = exc
            journal["status"] = "capture_failed_pending_resume"
            journal["failure_stage"] = stage
            journal["failure_type"] = type(exc).__name__
            write_private_replace(journal_path, canonical_json(journal))
        finally:
            stage = "resume source services"
            for container_id in journal["stop_intent_ids"]:
                if container_id not in journal["stopped_by_this_run"] and self.port.stop_event_status(container_id, started_at) == "confirmed":
                    journal["stopped_by_this_run"].append(container_id)
                    stop_confirmed.append(container_id)
            unresolved = resolve_stop_intents(journal, self.port, preflight.get("container_health", {}), timeout_profile=timeout_profile)
            write_private_replace(journal_path, canonical_json(journal))
            if unresolved:
                journal["status"] = "resume_incomplete"
                journal["resume_unresolved_ids"] = unresolved
                write_private_replace(journal_path, canonical_json(journal))
                if primary_error is None:
                    primary_error = CaptureError("source service state or readiness is unresolved; inspect the private recovery journal")

        if primary_error is not None:
            journal["status"] = "failed_source_resumed" if not journal.get("unresolved_stop_intents") else "resume_incomplete"
            write_private_replace(journal_path, canonical_json(journal))
            if isinstance(primary_error, CaptureError):
                raise primary_error
            if isinstance(primary_error, (KeyboardInterrupt, SystemExit)):
                raise CaptureInterrupted("capture interrupted; source resume was attempted") from None
            raise CaptureError(f"capture stopped safely during {stage}; source resume was attempted") from None
        if journal.get("unresolved_stop_intents"):
            raise CaptureError("source recovery is incomplete; do not retry capture")

        os.rename(partial, final)
        make_snapshot_read_only(final)
        fd = os.open(self.layout.snapshot_root, os.O_RDONLY)
        try:
            os.fsync(fd)
        finally:
            os.close(fd)
        journal["status"] = "sealed_and_source_ready"
        journal["snapshot_path"] = final.name
        write_private_replace(journal_path, canonical_json(journal))
        return {"run_id": run_id, "snapshot_sha256": sha256_file(final / "SEALED.json")[0], "artifact_count": len(artifact_meta), "source_services_restored": len(stop_confirmed)}

class DockerSource:
    def __init__(self, database_container: str, api_container: str, repo: Path):
        self.database_ref = database_container
        self.api_ref = api_container
        self.repo = repo
        self._current: dict[str, Any] | None = None

    def _inspect_all_running(self) -> list[dict[str, Any]]:
        ids = run_quiet(["docker", "ps", "--all", "--no-trunc", "-q"], timeout=20).decode().split()
        return [docker_inspect(identifier) for identifier in ids]

    def _all_data(self) -> tuple[dict[str, Any], dict[str, Any], list[dict[str, Any]]]:
        db = docker_inspect(self.database_ref)
        api = docker_inspect(self.api_ref)
        db_project = (db.get("Config", {}).get("Labels") or {}).get("com.docker.compose.project", "")
        api_project = (api.get("Config", {}).get("Labels") or {}).get("com.docker.compose.project", "")
        if not db_project or not api_project:
            raise CaptureError("database or backend has no Compose project identity")
        db_networks = {n.get("NetworkID", "") for n in (db.get("NetworkSettings", {}).get("Networks") or {}).values()}
        api_networks = {n.get("NetworkID", "") for n in (api.get("NetworkSettings", {}).get("Networks") or {}).values()}
        source_networks = db_networks | api_networks
        candidates = []
        for info in self._inspect_all_running():
            labels = info.get("Config", {}).get("Labels") or {}
            networks = info.get("NetworkSettings", {}).get("Networks") or {}
            ids = {n.get("NetworkID", "") for n in networks.values()}
            if info.get("Id") == db.get("Id"):
                continue
            if labels.get("com.docker.compose.project") in {db_project, api_project} or (source_networks & ids):
                candidates.append(info)
        if api.get("Id") == db.get("Id"):
            raise CaptureError("database and backend resolve to the same container")
        if api.get("Id") not in {c.get("Id") for c in candidates}:
            raise CaptureError("backend was not discovered as a source client candidate")
        return db, api, candidates

    def preflight(self) -> dict[str, Any]:
        db, api, candidates = self._all_data()
        for info, service in ((db, "database"), (api, "backend")):
            if not info.get("State", {}).get("Running"):
                raise CaptureError(f"expected source {service} container is not running")
            health = info.get("State", {}).get("Health", {}).get("Status", "none")
            if health not in ("healthy", "none"):
                raise CaptureError(f"expected source {service} container is not healthy")
        for info in candidates:
            if not info.get("State", {}).get("Running"):
                policy = (info.get("HostConfig", {}).get("RestartPolicy") or {}).get("Name", "no")
                if policy in {"always", "unless-stopped", "on-failure"}:
                    raise CaptureError("a stopped source candidate has an automatic restart policy")

        db_desc = container_descriptor(db, self.repo)
        api_desc = container_descriptor(api, self.repo)
        db_desc["runtime_binary_version"] = database_runtime_binary_version(db_desc["id"])
        if db_desc["published_ports"]:
            raise CaptureError("source PostgreSQL publishes a host port; capture cannot prove exclusive quiesce")
        db_desc["image_provenance"] = inspect_image_reference(db_desc["image_id"], missing_ok=True)
        api_desc["image_provenance"] = inspect_image_reference(api_desc["image_id"], missing_ok=True)
        expected_entrypoint = sha256_bytes(canonical_json(["/api"]))
        if api_desc["runtime_path"] != "/api" or api_desc["entrypoint_sha256"] != expected_entrypoint:
            raise CaptureError("backend runtime image entrypoint is not the reviewed direct API binary")

        db_env, api_env = container_env(db), container_env(api)
        db_name = db_env.get("POSTGRES_DB", "")
        db_user = db_env.get("POSTGRES_USER", "")
        db_password_file = db_env.get("POSTGRES_PASSWORD_FILE", "")
        if not db_name or not db_user or not db_password_file.startswith("/run/secrets/"):
            raise CaptureError("database container identity or mounted auth path is incomplete")
        mode = api_env.get("DATABASE_CONNECTION_MODE", "direct").strip() or "direct"
        if mode != "direct":
            raise CaptureError("origin preflight only accepts a direct local PostgreSQL connection")
        host = api_env.get("DATABASE_HOST", "").strip()
        port = api_env.get("DATABASE_PORT", "5432").strip()
        api_db_name = api_env.get("DATABASE_NAME", "")
        api_user = api_env.get("DATABASE_USER", "")
        if not host or not port.isdigit() or not api_db_name or not api_user:
            raise CaptureError("backend effective database configuration is incomplete")
        if api_db_name != db_name or port != "5432":
            raise CaptureError("backend configured database name/port does not match the source container")

        db_networks = db.get("NetworkSettings", {}).get("Networks") or {}
        api_networks = api.get("NetworkSettings", {}).get("Networks") or {}
        matched_network = None
        for name, db_network in db_networks.items():
            api_network = api_networks.get(name)
            if api_network is None or db_network.get("NetworkID") != api_network.get("NetworkID"):
                continue
            aliases = set(db_network.get("Aliases") or []) | {db_network.get("IPAddress", "")}
            if host in aliases:
                matched_network = name
                break
        if matched_network is None:
            raise CaptureError("backend database host does not resolve to the inspected source container on a shared network")

        image_migrations = migration_files_from_image(api_desc["id"])
        if not image_migrations:
            raise CaptureError("active backend image contains no migration files")
        db_mounts = [m for m in db_desc["mounts"] if m["destination"] == "/var/lib/postgresql" and m["type"] == "volume"]
        if len(db_mounts) != 1 or not db_mounts[0]["name"]:
            raise CaptureError("persistent PostgreSQL volume identity is missing or ambiguous")
        evidence_root = _mount_source(api_desc, "/var/lib/espacigo/private-evidence", mount_type="bind")
        replay_root = _mount_source(api_desc, "/var/lib/espacigo/privacy-replay", mount_type="bind")
        key_path = _mount_source(api_desc, "/run/secrets/local_contract_encryption_key", mount_type="bind")
        runtime_password = api_env.get("DATABASE_PASSWORD_FILE", "")
        if not runtime_password.startswith("/run/secrets/"):
            raise CaptureError("runtime password path is not a mounted secret path")
        # Confirm the exact secret mount without reading its content.
        _mount_source(api_desc, runtime_password, mount_type="bind")
        if not key_path.is_file() or key_path.is_symlink() or stat.S_IMODE(key_path.stat().st_mode) != 0o600:
            raise CaptureError("contract encryption key is not a private regular source file")
        key_stat = key_path.stat()
        if key_stat.st_size not in range(64, 67):
            raise CaptureError("contract encryption key file size is invalid")
        evidence_inventory = inventory_tree(evidence_root)
        replay_inventory = inventory_tree(replay_root)

        report = _database_query(db_desc["id"], db_password_file, SQL_CAPTURE)
        if report.get("database") != db_name:
            raise CaptureError("read-only SQL database identity does not match container configuration")
        db_history = {int(row["version"]): {"name": row["name"], "sha256": row["checksum"]} for row in report.get("migration_history", [])}
        image_matches = set(db_history) == set(image_migrations) and all(db_history[v] == image_migrations[v] for v in db_history)
        if not image_matches or len(db_history) != 61 or sorted(db_history) != list(range(1, 62)):
            raise CaptureError("database migration history does not match migrations embedded in active image V1-V61")

        api_ips = {normalize_ip_address(n.get("IPAddress", "")) for n in api_networks.values()}
        api_ips.discard(None)
        db_client_matches = [client for client in report.get("clients", []) if normalize_ip_address(client.get("address")) in api_ips and client.get("database") == db_name and client.get("user") == api_user]
        if not db_client_matches:
            raise CaptureError("no live database session proves the backend runtime connection target")
        known_ips = {normalize_ip_address(n.get("IPAddress", "")) for item in candidates for n in (item.get("NetworkSettings", {}).get("Networks") or {}).values()}
        known_ips.discard(None)
        known_ips |= {"127.0.0.1", "::1"}
        unknown_clients = [client for client in report.get("clients", []) if normalize_ip_address(client.get("address")) not in known_ips and client.get("application") != "pg_isready"]
        if unknown_clients:
            raise CaptureError("database has a client outside the enumerated Docker source candidates")

        candidate_descriptors = [container_descriptor(info, self.repo) for info in candidates]
        for desc in candidate_descriptors:
            desc["image_provenance"] = inspect_image_reference(desc["image_id"], missing_ok=True)
        container_health = {desc["id"]: desc["health"] for desc in candidate_descriptors}
        db_volume_id = db_mounts[0]["name"]
        source_projection = {
            "database_container": db_desc,
            "backend_container": api_desc,
            "candidate_containers": candidate_descriptors,
            "configured_database_role": api_user,
            "configured_database_name": db_name,
            "database_admin_role": db_user,
            "effective_connection": {"mode": mode, "host_alias_matches_database": host, "port": port, "database": api_db_name, "role_matches_live_session": api_user, "matched_network": matched_network, "active_session_count": len(db_client_matches)},
            "image_migration_fingerprint": sha256_bytes(canonical_json(image_migrations)),
            "database_migration_fingerprint": sha256_bytes(canonical_json(db_history)),
            "database_extensions": report["extensions"],
            "evidence_files": evidence_inventory,
            "privacy_replay_files": replay_inventory,
            "database_volume_name": db_volume_id,
            "contract_key_file_stat": {"size_bytes": key_stat.st_size, "mode_octal": f"{stat.S_IMODE(key_stat.st_mode):04o}", "mtime_ns": key_stat.st_mtime_ns, "inode": key_stat.st_ino},
            "database_and_backend_compose_roots_match": db_desc["compose_working_dir"] == api_desc["compose_working_dir"],
        }
        source_fingerprint = sha256_bytes(canonical_json(source_projection))
        self._current = {
            "database": db,
            "api": api,
            "candidates": candidates,
            "database_descriptor": db_desc,
            "api_descriptor": api_desc,
            "candidate_descriptors": candidate_descriptors,
            "database_report": report,
            "image_migrations": image_migrations,
            "source_roots": {"private-evidence": evidence_root, "privacy-replay": replay_root},
            "contract_key": key_path,
            "database_password_file": db_password_file,
            "runtime_password_container_path": runtime_password,
            "host_target": host,
            "source_projection": source_projection,
        }
        self._current["source_fingerprint"] = source_fingerprint
        self._current["database_container_id"] = db_desc["id"]
        self._current["database_volume_ids"] = [db_volume_id]
        self._current["mounted_volume_ids"] = [m["name"] for m in db_desc["mounts"] if m["type"] == "volume" and m["name"]]
        self._current["estimated_private_bytes"] = int(report["database_size_bytes"]) + sum(x["bytes"] for x in evidence_inventory + replay_inventory)
        self._current["container_health"] = container_health
        self._current["quiesce_candidates"] = [{"id": desc["id"], "running": desc["running"]} for desc in candidate_descriptors]
        self._current["public_summary"] = {
            "postgres_version": report["server_version_num"],
            "database_runtime_binary_version": db_desc["runtime_binary_version"],
            "database_bytes": report["database_size_bytes"],
            "migration_count": len(db_history),
            "migration_image_match": image_matches,
            "candidate_count": len(candidate_descriptors),
            "source_fingerprint": source_fingerprint,
            "compose_roots_equal": source_projection["database_and_backend_compose_roots_match"],
            "source_image_provenance_complete": all(
                desc.get("image_provenance", {}).get("status") == "available"
                for desc in [db_desc, api_desc, *candidate_descriptors]
            ),
            "connection_target_correlated": True,
            "database_volume_captured": bool(db_volume_id),
        }
        # Never serialize Docker inspect Config.Env or mounted secret paths in
        # the release/snapshot metadata. The connection projection is selected
        # from non-secret variables only.
        return {key: self._current[key] for key in (
            "source_fingerprint", "database_container_id", "database_volume_ids", "mounted_volume_ids",
            "estimated_private_bytes", "container_health", "quiesce_candidates", "database_descriptor",
            "api_descriptor", "candidate_descriptors", "database_report", "image_migrations",
            "source_projection", "public_summary",
        )}

    def validate_restore_target(self, restore_target: dict[str, Any]) -> None:
        selected = validate_restore_image_reference(restore_target.get("image_ref", ""))
        if selected != restore_target:
            raise CaptureError("selected restore image digest or local image identity changed")

    def stop_container(self, container_id: str, timeout: int) -> None:
        run_quiet(["docker", "stop", "--time", str(timeout), container_id], timeout=timeout + 15)

    def start_container(self, container_id: str, timeout: int) -> None:
        info = docker_inspect(container_id)
        if info.get("State", {}).get("Running"):
            return
        run_quiet(["docker", "start", container_id], timeout=timeout)

    def stop_event_status(self, container_id: str, since: str) -> str:
        try:
            result = run_quiet(["docker", "events", "--since", since, "--until", utc_now(), "--filter", f"container={container_id}", "--filter", "event=stop", "--format", "{{json .}}"], timeout=20)
            for line in result.decode().splitlines():
                event = strict_json(line)
                actor = event.get("Actor") or {}
                event_id = event.get("id", "")
                actor_id = actor.get("ID", "")
                if (event.get("Action") == "stop" or event.get("status") == "stop") and (event_id == container_id or actor_id == container_id):
                    return "confirmed"
        except CaptureError:
            return "unknown"
        return "absent"

    def container_state(self, container_id: str) -> str:
        try:
            info = docker_inspect(container_id)
            if info.get("Id") != container_id:
                return "unknown"
            state = info.get("State") or {}
            if state.get("Running") is True:
                return "running"
            if state.get("Status") in {"exited", "dead"}:
                return "stopped"
        except CaptureError:
            return "unknown"
        return "unknown"

    def wait_quiescent(self, timeout: int) -> dict[str, Any]:
        deadline = time.monotonic() + timeout
        last: dict[str, Any] = {}
        while time.monotonic() < deadline:
            last = _database_query(self._current["database_descriptor"]["id"], self._current["database_password_file"], SQL_CAPTURE)
            clients = [c for c in last.get("clients", []) if c.get("application") != "pg_isready"]
            leases = last.get("leases", {})
            if not clients and not leases.get("outbox") and not leases.get("notices"):
                return {"active_writers": 0, "active_leases": 0, "unknown_clients": [], "observed_client_count": len(clients), "leases": leases, "observed_at_utc": utc_now()}
            time.sleep(1)
        unknown = [{"address": c.get("address"), "database": c.get("database"), "application": c.get("application")} for c in last.get("clients", []) if c.get("application") != "pg_isready"]
        return {"active_writers": len(unknown), "active_leases": int(last.get("leases", {}).get("outbox", 0)) + int(last.get("leases", {}).get("notices", 0)), "unknown_clients": unknown, "observed_at_utc": utc_now()}

    def wait_ready(self, container_id: str, timeout: int, expected_health: str) -> None:
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            info = docker_inspect(container_id)
            state = info.get("State", {})
            health = state.get("Health", {}).get("Status", "none")
            if not state.get("Running"):
                time.sleep(2)
                continue
            if expected_health not in ("none", ""):
                if health == "healthy":
                    return
            else:
                service = (info.get("Config", {}).get("Labels") or {}).get("com.docker.compose.service", "")
                if service == "backend":
                    try:
                        run_quiet(["docker", "exec", container_id, "/api", "check"], timeout=15)
                        return
                    except CaptureError:
                        pass
                elif service == "mailpit":
                    published = (info.get("NetworkSettings", {}).get("Ports") or {}).get("8025/tcp") or []
                    if len(published) == 1 and published[0].get("HostPort", "").isdigit():
                        port = published[0]["HostPort"]
                        try:
                            with urllib.request.urlopen(f"http://127.0.0.1:{port}/api/v1/info", timeout=5) as response:
                                if response.status == 200:
                                    response.read(1024 * 1024)
                                    return
                        except (OSError, urllib.error.URLError, TimeoutError):
                            pass
                else:
                    raise CaptureError("source service has no healthcheck or reviewed readiness probe")
            time.sleep(2)
        raise CaptureError("source service did not return to its original ready state")

    def stream_database_dump(self, destination: BinaryIO, timeout: int) -> dict[str, Any]:
        current = self._current
        db_id = current["database_descriptor"]["id"]
        shell = (
            'password="$(cat "$POSTGRES_PASSWORD_FILE")"; '
            'export PGPASSWORD="$password" PGAPPNAME=gcp03-origin-capture; unset password; '
            'exec pg_dump --no-password --format=custom --host=127.0.0.1 '
            '--username="$POSTGRES_USER" --dbname="$POSTGRES_DB" --create'
        )
        process = subprocess.Popen(["docker", "exec", "-i", db_id, "sh", "-eu", "-c", shell], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        timer_fired = threading.Event()
        def expire() -> None:
            timer_fired.set()
            process.kill()
        timer = threading.Timer(timeout, expire)
        timer.start()
        digest, size = hashlib.sha256(), 0
        try:
            assert process.stdout is not None
            while chunk := process.stdout.read(1024 * 1024):
                digest.update(chunk)
                size += len(chunk)
                destination.write(chunk)
            rc = process.wait(timeout=15)
            if timer_fired.is_set() or rc != 0 or size == 0:
                raise CaptureError("PostgreSQL custom dump failed or exceeded its deadline")
        except BaseException:
            process.kill()
            process.wait()
            raise
        finally:
            timer.cancel()
        return {"plain_sha256": digest.hexdigest(), "plain_bytes": size, "format": "PostgreSQL custom archive", "global_role_password_hashes_exported": False, "source_database_stopped": False}

    def metadata_report(self, timeout: int = 120) -> dict[str, Any]:
        report = _database_query(self._current["database_descriptor"]["id"], self._current["database_password_file"], SQL_CAPTURE, timeout=timeout)
        assert_preflight_catalog_stable(self._current["database_report"], report)
        history = {int(row["version"]): {"name": row["name"], "sha256": row["checksum"]} for row in report.get("migration_history", [])}
        if history != {v: {"name": info["name"], "sha256": info["sha256"]} for v, info in self._current["image_migrations"].items()}:
            raise CaptureError("V1-V61 migration checksums changed or no longer match the active API image")
        if report.get("leases", {}).get("outbox") or report.get("leases", {}).get("notices"):
            raise CaptureError("lease found after source quiesce")
        report["migration_match_v1_v61"] = True
        report["role_passwords_included"] = False
        report["source_credentials_included"] = False
        report["database_and_backend_compose_roots_match"] = self._current["source_projection"]["database_and_backend_compose_roots_match"]
        return report

    def source_roots(self) -> dict[str, Path]:
        return dict(self._current["source_roots"])

    def source_contract_key(self) -> BinaryIO:
        path = self._current["contract_key"]
        safe_private_file(path)
        flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0)
        fd = os.open(path, flags)
        with os.fdopen(fd, "rb") as stream:
            before = os.fstat(stream.fileno())
            expected = self._current["source_projection"]["contract_key_file_stat"]
            actual = {"size_bytes": before.st_size, "mode_octal": f"{stat.S_IMODE(before.st_mode):04o}", "mtime_ns": before.st_mtime_ns, "inode": before.st_ino}
            if actual != expected:
                raise CaptureError("contract encryption key changed after reviewed preflight")
            key_text = stream.read(67)
            after = os.fstat(stream.fileno())
        if before.st_size != after.st_size or before.st_mtime_ns != after.st_mtime_ns or len(key_text) != before.st_size or not re.fullmatch(rb"[0-9a-fA-F]{64}\s*", key_text):
            raise CaptureError("contract encryption key format or stability check failed")
        return BytesIO(key_text)


def read_release(path: Path) -> dict[str, Any]:
    safe_private_file(path)
    try:
        value = strict_json(path.read_bytes())
    except (UnicodeDecodeError, json.JSONDecodeError) as exc:
        raise CaptureError("release record is not valid JSON") from exc
    if not isinstance(value, dict):
        raise CaptureError("release record must be a JSON object")
    return value


def build_release_candidate(preflight: dict[str, Any], run_id: str, reviewed_head_sha: str, restore_target: dict[str, Any]) -> dict[str, Any]:
    database_id = preflight["database_container_id"]
    order = [item["id"] for item in preflight["quiesce_candidates"] if item["running"]]
    # Ingress containers with published host ports are stopped first; the rest
    # follow. Resume always uses the exact reverse sequence.
    descriptors = {item["id"]: item for item in preflight["candidate_descriptors"]}
    order.sort(key=lambda ident: (0 if any(v for bindings in (descriptors[ident].get("published_ports") or {}).values() if isinstance(bindings, list) for v in bindings) else 1, ident))
    if database_id in order:
        raise CaptureError("candidate list unexpectedly contains the database container")
    return {
        "format": RELEASE_FORMAT,
        "status": "not_approved",
        "run_id": run_id,
        "reviewed_head_sha": reviewed_head_sha,
        "reviewed_by": "",
        "source_change_window_authorized": False,
        "all_ingress_and_writer_sources_enumerated": False,
        "exclusive_docker_control": False,
        "external_ingress_and_writer_sources": None,
        "window_start_utc": "",
        "window_end_utc": "",
        "source": {"database_container_id": database_id, "expected_source_fingerprint": preflight["source_fingerprint"]},
        "restore_target": restore_target,
        "timeout_profile": DEFAULT_TIMEOUT_PROFILE.copy(),
        "quiesce_order": order,
        "created_at_utc": utc_now(),
        "required_review": "After this capture implementation is reviewed and merged, record a specific capture window, verify every candidate and external ingress source, then create a separate mode-0600 approved release record bound to the reviewed HEAD and this run-id.",
    }


def recover_journal(path: Path, port: SnapshotPort, snapshot_root: Path | None = None) -> dict[str, Any]:
    safe_private_file(path)
    journal = strict_json(path.read_bytes())
    if journal.get("format") != f"{FORMAT}.quiesce-journal":
        raise CaptureError("recovery journal format is unsupported")
    run_id = journal.get("run_id")
    order = journal.get("quiesce_order")
    database_id = journal.get("database_container_id")
    stop_intents = journal.get("stop_intent_ids")
    stopped_ids = journal.get("stopped_by_this_run")
    restored_ids = journal.get("restored_by_this_run")
    if (
        not isinstance(run_id, str) or not RUN_ID_RE.fullmatch(run_id)
        or not isinstance(database_id, str) or not database_id
        or not isinstance(order, list) or not order or any(not isinstance(item, str) or not item for item in order)
        or database_id in order or len(order) != len(set(order))
        or not isinstance(stop_intents, list) or not isinstance(stopped_ids, list) or not isinstance(restored_ids, list)
        or any(item not in order for item in stop_intents + stopped_ids + restored_ids)
        or len(stop_intents) != len(set(stop_intents)) or len(stopped_ids) != len(set(stopped_ids))
        or len(restored_ids) != len(set(restored_ids))
    ):
        raise CaptureError("recovery journal container scope or run identity is invalid")
    if snapshot_root is not None:
        if snapshot_root.is_symlink() or not snapshot_root.is_dir():
            raise CaptureError("recovery snapshot root is not a private real directory")
        partial_name = journal.get("partial_snapshot", f"{run_id}.partial")
        if partial_name != f"{run_id}.partial":
            raise CaptureError("recovery journal snapshot path is invalid")
    health_map = journal.get("initial_health", {})
    timeout_profile = validate_timeout_profile(journal.get("timeout_profile", DEFAULT_TIMEOUT_PROFILE))
    unresolved = resolve_stop_intents(journal, port, health_map, timeout_profile=timeout_profile)
    errors = len(unresolved)
    journal["status"] = "recovered" if errors == 0 else "resume_incomplete"
    journal["resume_unresolved_ids"] = unresolved
    journal["recovered_at_utc"] = utc_now()
    if errors == 0 and snapshot_root is not None and journal.get("artifact_seal_sha256"):
        partial = snapshot_root / journal.get("partial_snapshot", f"{run_id}.partial")
        final = snapshot_root / run_id
        candidate = partial if partial.exists() else final
        if candidate.exists() and not candidate.is_symlink():
            seal = candidate / "SEALED.json"
            if seal.is_file() and sha256_file(seal)[0] == journal["artifact_seal_sha256"]:
                verified = check_seal_files(candidate, run_id, allow_partial=candidate == partial)
                if verified:
                    if stat.S_IMODE(candidate.stat().st_mode) == 0o700:
                        make_snapshot_read_only(candidate)
                    if candidate == partial:
                        if final.exists() or final.is_symlink():
                            raise CaptureError("snapshot final destination already exists during recovery")
                        os.rename(partial, final)
                    journal["status"] = "recovered_and_sealed"
                    journal["snapshot_path"] = final.name
    write_private_replace(path, canonical_json(journal))
    if errors:
        raise CaptureError("one or more services remain unrestored; inspect private recovery journal")
    return {"run_id": journal["run_id"], "restored_services": len(journal.get("restored_by_this_run", [])), "status": journal["status"]}


def check_seal_files(snapshot: Path, expected_run_id: str, *, allow_partial: bool = False) -> dict[str, Any]:
    if snapshot.is_symlink() or not snapshot.is_dir() or stat.S_IMODE(snapshot.stat().st_mode) not in {0o500, 0o700}:
        raise CaptureError("snapshot directory is absent or not private")
    seal_path = snapshot / "SEALED.json"
    checksum_path = snapshot / "SEALED.sha256"
    mode_one_of(seal_path, {0o400, 0o600})
    mode_one_of(checksum_path, {0o400, 0o600})
    checksum_text = checksum_path.read_text(encoding="ascii")
    match = re.fullmatch(r"([0-9a-f]{64})  SEALED\.json\n?", checksum_text)
    if not match:
        raise CaptureError("snapshot seal checksum record is malformed")
    checksum = match.group(1)
    if sha256_file(seal_path)[0] != checksum:
        raise CaptureError("snapshot seal checksum mismatch")
    seal = strict_json(seal_path.read_bytes())
    if seal.get("format") != f"{FORMAT}.sealed" or seal.get("custody_key_in_artifact") is not False:
        raise CaptureError("snapshot seal format is invalid")
    if not isinstance(seal.get("run_id"), str) or not RUN_ID_RE.fullmatch(seal["run_id"]):
        raise CaptureError("snapshot run identity is invalid")
    expected_names = {"database.dump.gpg", "private-files.tar.gpg", "contract-encryption-key.gpg", "origin-metadata.json.gpg"}
    if not isinstance(seal.get("encrypted_artifacts"), dict) or set(seal["encrypted_artifacts"]) != expected_names:
        raise CaptureError("snapshot encrypted artifact manifest is incomplete or contains unknown paths")
    expected_dir = f"{expected_run_id}.partial" if allow_partial else expected_run_id
    if seal["run_id"] != expected_run_id or snapshot.name != expected_dir:
        raise CaptureError("snapshot directory does not match the sealed run identity")
    if {entry.name for entry in snapshot.iterdir()} != expected_names | {"SEALED.json", "SEALED.sha256"}:
        raise CaptureError("snapshot contains missing or unmanifested files")
    for name, artifact in seal.get("encrypted_artifacts", {}).items():
        if name not in expected_names or not isinstance(artifact, dict):
            raise CaptureError("snapshot artifact path or descriptor is invalid")
        path = snapshot / name
        mode_one_of(path, {0o400, 0o600})
        actual_hash, actual_bytes = sha256_file(path)
        expected_hash = artifact.get("cipher_sha256", artifact.get("encrypted_file_sha256"))
        expected_size = artifact.get("cipher_bytes")
        if not isinstance(expected_hash, str) or not re.fullmatch(r"[0-9a-f]{64}", expected_hash) or actual_hash != expected_hash:
            raise CaptureError("encrypted artifact checksum mismatch")
        if expected_size is not None and (not isinstance(expected_size, int) or expected_size != actual_bytes):
            raise CaptureError("encrypted artifact checksum mismatch")
    return seal


def make_snapshot_read_only(snapshot: Path) -> None:
    for entry in snapshot.iterdir():
        if entry.is_symlink() or not entry.is_file() or stat.S_IMODE(entry.stat().st_mode) not in {0o400, 0o600}:
            raise CaptureError("snapshot contains an unexpected file before sealing")
        fsync_file(entry)
        os.chmod(entry, 0o400)
        fsync_file(entry)
    os.chmod(snapshot, 0o500)
    fd = os.open(snapshot, os.O_RDONLY)
    try:
        os.fsync(fd)
    finally:
        os.close(fd)


def check_encrypted_snapshot(snapshot: Path, custody_key: bytes) -> dict[str, Any]:
    seal = check_seal_files(snapshot, snapshot.name)
    # Authenticate at least the metadata before any database container is started.
    metadata_path = snapshot / "origin-metadata.json.gpg"
    with tempfile.TemporaryDirectory(prefix="gcp03-verify-") as tempdir:
        temp = Path(tempdir)
        os.chmod(temp, 0o700)
        output = temp / "metadata.json"
        gpg_decrypt_file(metadata_path, output, derive_passphrase(custody_key, seal["run_id"], "metadata"), 120)
        metadata = strict_json(output.read_bytes())
        if not metadata.get("database_report", {}).get("migration_match_v1_v61"):
            raise CaptureError("snapshot migration integrity evidence is absent")
    return seal


def _quote_ident(value: str) -> str:
    if not isinstance(value, str) or "\x00" in value:
        raise CaptureError("role catalog contains an invalid identifier")
    return '"' + value.replace('"', '""') + '"'


def _quote_literal(value: str) -> str:
    if not isinstance(value, str) or "\x00" in value:
        raise CaptureError("role catalog contains an invalid setting")
    return "'" + value.replace("'", "''") + "'"


def role_reconciliation_sql(role_report: dict[str, Any], existing_role_names: set[str]) -> bytes:
    statements: list[str] = []
    role_names = {row["name"] for row in role_report.get("roles", [])}
    for role in sorted(role_report.get("roles", []), key=lambda item: item["name"]):
        name = role["name"]
        if name.startswith("pg_") or name == "pg_database_owner":
            continue
        if name == "postgres" and (not role.get("superuser") or not role.get("login")):
            raise CaptureError("source postgres role cannot safely serve as the isolated restore administrator")
        ident = _quote_ident(name)
        flags = [
            "SUPERUSER" if role["superuser"] else "NOSUPERUSER",
            "CREATEDB" if role["create_db"] else "NOCREATEDB",
            "CREATEROLE" if role["create_role"] else "NOCREATEROLE",
            "LOGIN" if role["login"] else "NOLOGIN",
            "INHERIT" if role["inherit"] else "NOINHERIT",
            "REPLICATION" if role["replication"] else "NOREPLICATION",
            "BYPASSRLS" if role["bypass_rls"] else "NOBYPASSRLS",
            f"CONNECTION LIMIT {int(role['connection_limit'])}",
        ]
        valid_until = role.get("valid_until")
        if isinstance(valid_until, str):
            flags.append("VALID UNTIL " + _quote_literal(valid_until))
        operation = "ALTER ROLE" if name in existing_role_names else "CREATE ROLE"
        statements.append(f"{operation} {ident} WITH {' '.join(flags)};")
        for setting in role.get("config") or []:
            if not isinstance(setting, str) or "=" not in setting or re.search(r"(?i)(password|secret|token)", setting):
                raise CaptureError("role settings contain an invalid or sensitive field")
            key, value = setting.split("=", 1)
            if not re.fullmatch(r"[a-zA-Z_][a-zA-Z0-9_.]*", key):
                raise CaptureError("role setting name is invalid")
            statements.append(f"ALTER ROLE {ident} SET {_quote_ident(key)} TO {_quote_literal(value)};")
    for grant in sorted(role_report.get("memberships", []), key=lambda item: (item["member"], item["granted_role"])):
        if grant["member"] not in role_names | existing_role_names or grant["granted_role"] not in role_names | existing_role_names:
            raise CaptureError("role membership references a role missing from the manifest")
        statements.append(
            f"GRANT {_quote_ident(grant['granted_role'])} TO {_quote_ident(grant['member'])} "
            f"WITH ADMIN {'TRUE' if grant['admin_option'] else 'FALSE'}, "
            f"INHERIT {'TRUE' if grant['inherit_option'] else 'FALSE'}, "
            f"SET {'TRUE' if grant['set_option'] else 'FALSE'};"
        )
    return ("\n".join(statements) + "\n").encode()


def verify_private_tar(snapshot: Path, master: bytes, seal: dict[str, Any], expected_files: list[dict[str, Any]]) -> dict[str, Any]:
    if not isinstance(expected_files, list):
        raise CaptureError("private file inventory is malformed")
    expected_by_name: dict[str, dict[str, Any]] = {}
    for item in expected_files:
        if not isinstance(item, dict) or item.get("root") not in {"private-evidence", "privacy-replay"}:
            raise CaptureError("private file inventory contains an unsupported root")
        relative = item.get("relative_path")
        if not isinstance(relative, str):
            raise CaptureError("private file inventory path is malformed")
        safe = require_safe_relative(relative)
        name = PurePosixPath(item["root"], *safe.parts).as_posix()
        if name in expected_by_name or not isinstance(item.get("bytes"), int) or item["bytes"] < 0 or not re.fullmatch(r"[0-9a-f]{64}", str(item.get("sha256", ""))) or item.get("mode_octal") != "0600":
            raise CaptureError("private file inventory entry is invalid")
        expected_by_name[name] = item
    source = snapshot / "private-files.tar.gpg"
    artifact = seal["encrypted_artifacts"]["private-files.tar.gpg"]
    if sha256_file(source)[0] != artifact["cipher_sha256"]:
        raise CaptureError("private file archive ciphertext hash mismatch")
    read_fd, write_fd = os.pipe()
    command = ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--decrypt", str(source)]
    process = subprocess.Popen(command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
    os.close(read_fd)
    timer_fired = threading.Event()
    def expire() -> None:
        timer_fired.set()
        try:
            process.kill()
        except OSError:
            pass
    timer = threading.Timer(TREE_TIMEOUT_SECONDS, expire)
    timer.start()
    try:
        os.write(write_fd, derive_passphrase(master, seal["run_id"], "private-files") + b"\n")
        os.close(write_fd)
        write_fd = -1
        actual: list[dict[str, Any]] = []
        archive_manifest: dict[str, Any] | None = None
        seen: set[str] = set()
        assert process.stdout is not None
        with tarfile.open(fileobj=process.stdout, mode="r|*") as archive:
            for member in archive:
                name = member.name
                path = require_safe_relative(name)
                if name in seen:
                    raise CaptureError("private archive contains a duplicate path")
                seen.add(name)
                if member.issym() or member.islnk() or member.isdev() or member.isfifo():
                    raise CaptureError("private archive contains a link or special file")
                if member.isdir():
                    if stat.S_IMODE(member.mode) != 0o700:
                        raise CaptureError("private archive directory mode is not 0700")
                    if path.parts[0] not in {"private-evidence", "privacy-replay"}:
                        raise CaptureError("private archive directory escaped its declared roots")
                    continue
                if not member.isfile():
                    raise CaptureError("private archive has an unsupported entry type")
                if name == "capture-file-manifest.json":
                    if member.size > 64 * 1024 * 1024 or stat.S_IMODE(member.mode) != 0o600:
                        raise CaptureError("private archive manifest size or mode is invalid")
                    stream = archive.extractfile(member)
                    if stream is None:
                        raise CaptureError("private archive manifest cannot be read")
                    archive_manifest = strict_json(stream.read())
                    continue
                if path.parts[0] not in {"private-evidence", "privacy-replay"} or stat.S_IMODE(member.mode) != 0o600:
                    raise CaptureError("private file path or mode is invalid")
                expected_record = expected_by_name.get(name)
                if expected_record is None or member.size != expected_record["bytes"]:
                    raise CaptureError("private archive contains an unmanifested or oversized file")
                stream = archive.extractfile(member)
                if stream is None:
                    raise CaptureError("private archive file cannot be read")
                digest, size = hashlib.sha256(), 0
                while chunk := stream.read(1024 * 1024):
                    digest.update(chunk)
                    size += len(chunk)
                root = path.parts[0]
                relative = PurePosixPath(*path.parts[1:]).as_posix()
                record = {"root": root, "relative_path": relative, "mode_octal": "0600", "bytes": size, "sha256": digest.hexdigest()}
                if canonical_json(record) != canonical_json(expected_record):
                    raise CaptureError("private archive file does not match the reviewed manifest")
                actual.append(record)
        rc = process.wait(timeout=TREE_TIMEOUT_SECONDS + 5)
        if timer_fired.is_set() or rc != 0:
            raise CaptureError("private file archive authentication failed or exceeded its deadline")
        if archive_manifest is None or archive_manifest.get("format") != f"{FORMAT}.files":
            raise CaptureError("private archive manifest is absent or invalid")
        if canonical_json(archive_manifest.get("files")) != canonical_json(expected_files):
            raise CaptureError("private archive manifest differs from encrypted origin metadata")
        if canonical_json(sorted(actual, key=lambda item: (item["root"], item["relative_path"]))) != canonical_json(sorted(expected_files, key=lambda item: (item["root"], item["relative_path"]))):
            raise CaptureError("private file archive content hash verification failed")
        return {"verified_file_count": len(actual), "manifest_sha256": sha256_bytes(canonical_json(expected_files))}
    except BaseException:
        process.kill()
        process.wait()
        raise
    finally:
        timer.cancel()
        if process.stdout is not None and not process.stdout.closed:
            process.stdout.close()
        if write_fd >= 0:
            os.close(write_fd)


def _docker_exec_sql(container_id: str, database: str, sql: bytes, *, timeout: int = 600) -> bytes:
    return run_quiet(["docker", "exec", "--user", "postgres", "-i", container_id, "psql", "-XqAt", "-v", "ON_ERROR_STOP=1", "--username=postgres", f"--dbname={database}", "-f", "-"], timeout=timeout, input_bytes=sql)


def _docker_query(container_id: str, database: str, sql: str, *, timeout: int = 120) -> dict[str, Any]:
    output = _docker_exec_sql(container_id, database, sql.encode(), timeout=timeout)
    lines = [line for line in output.decode().splitlines() if line.strip()]
    if len(lines) != 1:
        raise CaptureError("isolated restore query returned an unexpected result")
    value = strict_json(lines[0])
    if not isinstance(value, dict):
        raise CaptureError("isolated restore query did not return an object")
    return value


def compare_restored_database(source: dict[str, Any], restored: dict[str, Any]) -> dict[str, Any]:
    keys = ("database", "server_version_num", "encoding", "extensions", "migration_history", "columns", "security_definer_count", "notice_states", "core_counts", "roles", "memberships", "database_owner_acl", "schema_owners_acls", "relation_owners_acls", "routine_owners_acls", "default_acls")
    source_roles = source.get("roles") or []
    allowed_extra_roles = {"postgres"} - {role["name"] for role in source_roles}
    normalized_source = dict(source)
    normalized_restored = dict(restored)
    normalized_source["roles"] = source_roles
    normalized_restored["roles"] = [r for r in (restored.get("roles") or []) if r["name"] not in allowed_extra_roles]
    mismatches = [key for key in keys if canonical_json(normalized_source.get(key)) != canonical_json(normalized_restored.get(key))]
    for key in ("sessions", "tokens"):
        if (source.get(key) or {}).get("total") != (restored.get(key) or {}).get("total"):
            mismatches.append(f"{key}.total")
    compared = list(keys) + ["sessions.total", "tokens.total"]
    return {"matches": not mismatches, "mismatched_fields": mismatches, "compared_fields": compared}


def assert_preflight_catalog_stable(before: dict[str, Any], after: dict[str, Any]) -> None:
    keys = (
        "database", "server_version_num", "encoding", "extensions", "migration_history", "roles", "memberships",
        "database_owner_acl", "schema_owners_acls", "relation_owners_acls", "routine_owners_acls", "default_acls",
        "columns", "security_definer_count", "notice_states", "core_counts",
    )
    changed = [key for key in keys if canonical_json(before.get(key)) != canonical_json(after.get(key))]
    for key in ("sessions", "tokens"):
        before_total = (before.get(key) or {}).get("total")
        after_total = (after.get(key) or {}).get("total")
        if before_total != after_total:
            changed.append(f"{key}.total")
    if changed:
        raise CaptureError("source catalog or aggregate data changed after read-only preflight")


def _write_synthetic_password(path: Path) -> None:
    # The versioned database entrypoint accepts only lowercase hex credentials
    # for its isolated runtime role initialization.
    password = secrets.token_hex(48).encode() + b"\n"
    write_new_private(path, password)


def wait_local_pg(container_id: str, timeout: int = READINESS_TIMEOUT_SECONDS) -> None:
    deadline = time.monotonic() + timeout
    stable_checks = 0
    while time.monotonic() < deadline:
        try:
            run_quiet(["docker", "exec", "--user", "postgres", container_id, "pg_isready", "--username=postgres", "--dbname=restore_bootstrap"], timeout=10)
            run_quiet(["docker", "exec", "--user", "postgres", container_id, "psql", "-XqAt", "--username=postgres", "--dbname=restore_bootstrap", "-c", "SELECT 1"], timeout=10)
            stable_checks += 1
            if stable_checks >= 5:
                return
        except CaptureError:
            stable_checks = 0
            time.sleep(2)
        else:
            time.sleep(1)
    raise CaptureError("isolated PostgreSQL restore container did not become ready")


def _restore_database_dump(snapshot: Path, seal: dict[str, Any], master: bytes, container_id: str, timeout: int = PG_DUMP_TIMEOUT_SECONDS) -> dict[str, Any]:
    source = snapshot / "database.dump.gpg"
    artifact = seal["encrypted_artifacts"]["database.dump.gpg"]
    if sha256_file(source)[0] != artifact["cipher_sha256"]:
        raise CaptureError("database dump ciphertext hash mismatch")
    read_fd, write_fd = os.pipe()
    decrypt_command = ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--decrypt", str(source)]
    decrypt = subprocess.Popen(decrypt_command, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, pass_fds=(read_fd,))
    os.close(read_fd)
    restore = subprocess.Popen(["docker", "exec", "--user", "postgres", "-i", container_id, "pg_restore", "--exit-on-error", "--create", "--username=postgres", "--dbname=postgres"], stdin=decrypt.stdout, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    assert decrypt.stdout is not None
    decrypt.stdout.close()
    try:
        os.write(write_fd, derive_passphrase(master, seal["run_id"], "database-dump") + b"\n")
        os.close(write_fd)
        write_fd = -1
        restore_rc = restore.wait(timeout=timeout)
        decrypt_rc = decrypt.wait(timeout=30)
        if restore_rc != 0 or decrypt_rc != 0:
            raise CaptureError("authenticated database restore failed in the isolated copy")
        return {"restore_process": "success", "gpg_authentication": "success"}
    except BaseException:
        for process in (restore, decrypt):
            try:
                process.kill()
            except OSError:
                pass
            process.wait()
        raise
    finally:
        if write_fd >= 0:
            os.close(write_fd)


def restore_validate_copy(repo: Path, snapshot: Path, recovery_key_path: Path, run_id: str, restore_image_ref: str) -> dict[str, Any]:
    if not RUN_ID_RE.fullmatch(run_id):
        raise CaptureError("restore run-id format is invalid")
    mode_exact(recovery_key_path)
    master = load_recovery_key(recovery_key_path)
    seal = check_encrypted_snapshot(snapshot, master)
    if seal.get("run_id") != run_id:
        raise CaptureError("snapshot run-id does not match the restore confirmation")
    metadata_dir = Path(tempfile.mkdtemp(prefix="gcp03-restore-meta-"))
    os.chmod(metadata_dir, 0o700)
    try:
        metadata_path = metadata_dir / "origin-metadata.json"
        gpg_decrypt_file(snapshot / "origin-metadata.json.gpg", metadata_path, derive_passphrase(master, run_id, "metadata"), 300)
        metadata = strict_json(metadata_path.read_bytes())
        report = metadata.get("database_report") or {}
        preflight = metadata.get("preflight") or {}
        release = metadata.get("release") or {}
        restore_target = release.get("restore_target") or {}
        if restore_image_ref != restore_target.get("image_ref"):
            raise CaptureError("restore image does not match the separately reviewed digest in the snapshot")
        selected_restore_image = validate_restore_image_reference(restore_image_ref)
        if selected_restore_image != restore_target:
            raise CaptureError("selected restore image provenance differs from the reviewed snapshot metadata")
        expected_image = selected_restore_image["image_id"]
        admin_role = (preflight.get("source_projection") or {}).get("database_admin_role")
        if not expected_image or not admin_role:
            raise CaptureError("snapshot lacks its pinned restore image or database-role metadata")
        verify_private_tar(snapshot, master, seal, metadata.get("file_inventory") or [])
        restore_root = private_root(repo / ".local/testing/gcp03-origin-restores")
        result_dir = ensure_new_directory(restore_root / run_id)
        container_name = "g03copy-" + run_id[:40]
        try:
            run_quiet(["docker", "inspect", container_name], timeout=10)
        except CaptureError:
            pass
        else:
            raise CaptureError("isolated restore container name already exists")
        with tempfile.TemporaryDirectory(prefix="gcp03-copy-auth-") as auth_dir_text:
            auth_dir = Path(auth_dir_text)
            os.chmod(auth_dir, 0o700)
            admin_secret = auth_dir / "db-admin"
            runtime_secret = auth_dir / "runtime"
            _write_synthetic_password(admin_secret)
            _write_synthetic_password(runtime_secret)
            tempfs_bytes = max(512 * 1024 * 1024, int(seal["encrypted_artifacts"]["database.dump.gpg"]["plain_bytes"]) * 3 + MIN_FREE_SPACE_BYTES)
            run_output = run_quiet([
                "docker", "run", "--detach", "--name", container_name, "--network", "none", "--read-only", "--restart", "no",
                "--tmpfs", f"/var/lib/postgresql:rw,nosuid,nodev,noexec,size={tempfs_bytes}",
                "--tmpfs", "/tmp:rw,nosuid,nodev,size=268435456",
                "--tmpfs", "/run:rw,nosuid,nodev,size=67108864",
                "--mount", f"type=bind,source={admin_secret},target=/run/secrets/db_admin_password,readonly",
                "--mount", f"type=bind,source={runtime_secret},target=/run/secrets/runtime_password,readonly",
                "--env", "POSTGRES_DB=restore_bootstrap", "--env", "POSTGRES_USER=postgres",
                "--env", "POSTGRES_PASSWORD_FILE=/run/secrets/db_admin_password", restore_image_ref,
            ], timeout=30).decode().strip()
            container_id = run_output
            try:
                info = docker_inspect(container_id)
                if info.get("Image") != expected_image or info.get("HostConfig", {}).get("NetworkMode") != "none":
                    raise CaptureError("restore container image or network isolation differs from the sealed snapshot")
                if info.get("HostConfig", {}).get("PortBindings") or any(m.get("Type") == "volume" for m in info.get("Mounts", [])):
                    raise CaptureError("isolated restore unexpectedly publishes ports or mounts a Docker volume")
                wait_local_pg(container_id)
                target_runtime = _docker_query(container_id, "restore_bootstrap", """
SELECT json_build_object(
  'server_version_num', current_setting('server_version_num')::integer,
  'extension_versions', coalesce((
    SELECT json_object_agg(name, versions)
    FROM (
      SELECT name, json_agg(version ORDER BY version) AS versions
      FROM pg_available_extension_versions
      GROUP BY name
    ) AS available
  ), '{}'::json)
)::text;
""")
                source_version = int(report.get("server_version_num", 0))
                target_version = int(target_runtime.get("server_version_num", 0))
                if source_version // 10000 != target_version // 10000:
                    raise CaptureError("selected restore image PostgreSQL major version is incompatible")
                extension_versions = target_runtime.get("extension_versions") or {}
                for extension in report.get("extensions") or []:
                    if extension.get("version") not in extension_versions.get(extension.get("name"), []):
                        raise CaptureError("selected restore image lacks an exact source extension version")
                roles = report.get("roles") or []
                existing = set(run_quiet(["docker", "exec", "--user", "postgres", container_id, "psql", "-XqAt", "--username=postgres", "--dbname=restore_bootstrap", "-c", "SELECT rolname FROM pg_roles ORDER BY rolname"], timeout=30).decode().splitlines())
                role_sql = role_reconciliation_sql(report, existing)
                if role_sql.strip():
                    _docker_exec_sql(container_id, "restore_bootstrap", role_sql)
                _restore_database_dump(snapshot, seal, master, container_id)
                db_name = report["database"]
                restored = _docker_query(container_id, db_name, SQL_CAPTURE)
                comparison = compare_restored_database(report, restored)
                if not comparison["matches"]:
                    raise CaptureError("restored schema/data/ACL comparison failed: " + ",".join(comparison["mismatched_fields"]))
                quarantine_sql = (repo / "scripts/gcp03_origin_quarantine_copy.sql").read_bytes()
                quarantine_output = _docker_exec_sql(container_id, db_name, quarantine_sql, timeout=120)
                quarantine_lines = [line for line in quarantine_output.decode().splitlines() if line.strip()]
                if len(quarantine_lines) != 1:
                    raise CaptureError("copy quarantine returned an unexpected proof result")
                quarantine = strict_json(quarantine_lines[0])
                if quarantine.get("valid_sessions") != 0 or quarantine.get("usable_action_tokens") != 0 or quarantine.get("pending_notices") != 0 or quarantine.get("identity_event_leases") != 0 or quarantine.get("runtime_role_login") is not False:
                    raise CaptureError("restored copy quarantine postconditions failed")
                if not isinstance(quarantine.get("quarantined_notices"), int):
                    raise CaptureError("copy notice-quarantine proof is invalid")
                validation = {
                    "format": f"{FORMAT}.restore-validation",
                    "run_id": run_id,
                    "source_database_mutated": False,
                    "copy_container_network": "none",
                    "database_storage": "tmpfs; no Docker volume",
                    "restore_image_ref": restore_image_ref,
                    "restore_image_id": selected_restore_image["image_id"],
                    "restore_runtime_compatible": True,
                    "application_or_workers_started": False,
                    "mail_webhooks_payment_providers_enabled": False,
                    "database_and_acl_comparison": comparison,
                    "private_file_count": len(metadata.get("file_inventory") or []),
                    "private_file_manifest_sha256": sha256_bytes(canonical_json(metadata.get("file_inventory") or [])),
                    "quarantine": quarantine,
                    "validated_at_utc": utc_now(),
                }
                write_new_private(result_dir / "restore-validation-private.json", canonical_json(validation))
                return {"run_id": run_id, "status": "isolated_copy_restored_and_quarantined", "core_checks": len(comparison["compared_fields"]), "quarantined_notices": quarantine["quarantined_notices"]}
            finally:
                try:
                    run_quiet(["docker", "rm", "--force", container_id], timeout=30)
                except CaptureError:
                    try:
                        docker_inspect(container_id)
                    except CaptureError:
                        pass
                    else:
                        raise CaptureError("isolated restore container cleanup could not be confirmed")
    finally:
        for path in metadata_dir.iterdir():
            if path.is_file() and not path.is_symlink():
                path.unlink()
        metadata_dir.rmdir()


def main(argv: list[str] | None = None) -> int:
    global _STAGE
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    pre = sub.add_parser("preflight", help="read-only correlation and release candidate")
    pre.add_argument("--database-container", default=DEFAULT_DB)
    pre.add_argument("--api-container", default=DEFAULT_API)
    pre.add_argument("--private-output-dir", required=True)
    candidate = sub.add_parser("release-candidate", help="write a private, unapproved run record")
    candidate.add_argument("--database-container", default=DEFAULT_DB)
    candidate.add_argument("--api-container", default=DEFAULT_API)
    candidate.add_argument("--run-id", required=True)
    candidate.add_argument("--private-output-dir", required=True)
    candidate.add_argument("--restore-image-ref", required=True, help="locally available repository@sha256 digest for isolated restore copy")
    capture = sub.add_parser("capture", help="capture only after reviewed release-specific approval")
    capture.add_argument("--release-file", required=True)
    capture.add_argument("--run-id", required=True)
    capture.add_argument("--confirm-capture-run-id", required=True)
    capture.add_argument("--database-container", default=DEFAULT_DB)
    capture.add_argument("--api-container", default=DEFAULT_API)
    recover = sub.add_parser("recover", help="resume only containers recorded as stopped by this run")
    recover.add_argument("--database-container", default=DEFAULT_DB)
    recover.add_argument("--api-container", default=DEFAULT_API)
    recover.add_argument("--journal", required=True)
    decrypt = sub.add_parser("recover-key", help="decrypt the separately encrypted contract key into a new private file")
    decrypt.add_argument("--snapshot", required=True)
    decrypt.add_argument("--recovery-key", required=True)
    decrypt.add_argument("--output", required=True)
    restore = sub.add_parser("restore-copy", help="restore and quarantine only an isolated local copy")
    restore.add_argument("--snapshot", required=True)
    restore.add_argument("--recovery-key", required=True)
    restore.add_argument("--run-id", required=True)
    restore.add_argument("--restore-image-ref", required=True, help="exact repository@sha256 digest reviewed in snapshot metadata")
    args = parser.parse_args(argv)
    repo = Path(__file__).resolve().parent.parent
    os.umask(0o077)
    try:
        if args.command in {"preflight", "release-candidate"}:
            _STAGE = "read-only runtime correlation"
            restore_target = validate_restore_image_reference(args.restore_image_ref) if args.command == "release-candidate" else None
            port = DockerSource(args.database_container, args.api_container, repo)
            report = port.preflight()
            _STAGE = "private preflight evidence write"
            private_dir = local_testing_path(repo, args.private_output_dir)
            ensure_new_directory(private_dir)
            private = {k: v for k, v in report.items() if k not in {"database", "api", "candidates", "source_roots", "contract_key"}}
            private["database_container"] = report["database_descriptor"]
            private["backend_container"] = report["api_descriptor"]
            private["candidate_containers"] = report["candidate_descriptors"]
            private["effective_connection"] = report["source_projection"]["effective_connection"]
            private["database_report"] = report["database_report"]
            private_path = private_dir / "preflight-private.json"
            write_new_private(private_path, canonical_json(private))
            if args.command == "release-candidate":
                _STAGE = "unapproved release candidate write"
                if not RUN_ID_RE.fullmatch(args.run_id):
                    raise CaptureError("run-id format is invalid")
                head = run_quiet(["git", "rev-parse", "HEAD"], timeout=10).decode().strip()
                candidate_doc = build_release_candidate(report, args.run_id, head, restore_target)
                write_new_private(private_dir / "release-candidate.json", canonical_json(candidate_doc))
                print(json.dumps({"status": "unapproved_candidate_written", "run_id": args.run_id, "candidate_count": len(report["candidate_descriptors"]), "source_fingerprint": report["source_fingerprint"], "private_output": str(private_dir.relative_to(repo))}, sort_keys=True))
            else:
                print(json.dumps(report["public_summary"], sort_keys=True))
            return 0
        if args.command == "capture":
            _STAGE = "release authorization validation"
            if args.run_id != args.confirm_capture_run_id:
                raise CaptureError("explicit run-id confirmation does not match")
            release_path = local_testing_path(repo, args.release_file)
            release = read_release(release_path)
            head = run_quiet(["git", "rev-parse", "HEAD"], timeout=10).decode().strip()
            snapshots = private_root(repo / ".local/testing/gcp03-origin-snapshots")
            custody = private_root(Path.home() / ".local/share/espacigo/gcp03-origin-custody")
            journals = private_root(repo / ".local/testing/gcp03-origin-journals")
            port = DockerSource(args.database_container, args.api_container, repo)
            def interrupt_handler(signum: int, frame: Any) -> None:
                raise CaptureInterrupted(f"capture interrupted by signal {signum}")
            old_int = signal.signal(signal.SIGINT, interrupt_handler)
            old_term = signal.signal(signal.SIGTERM, interrupt_handler)
            try:
                result = CaptureCoordinator(port, CaptureLayout(snapshots, custody, journals)).capture(release, args.run_id, head)
            finally:
                signal.signal(signal.SIGINT, old_int)
                signal.signal(signal.SIGTERM, old_term)
            _STAGE = "capture completed"
            print(json.dumps({"status": "sealed", **result}, sort_keys=True))
            return 0
        if args.command == "recover":
            journal_path = local_testing_path(repo, args.journal)
            port = DockerSource(args.database_container, args.api_container, repo)
            snapshot_root = private_root(repo / ".local/testing/gcp03-origin-snapshots")
            print(json.dumps(recover_journal(journal_path, port, snapshot_root), sort_keys=True))
            return 0
        if args.command in {"recover-key", "restore-copy"}:
            _STAGE = "restore-copy validation" if args.command == "restore-copy" else "recover-key validation"
            snapshot = local_testing_path(repo, args.snapshot)
            recovery_root = (Path.home() / ".local/share/espacigo/gcp03-origin-custody").resolve()
            recovery_key_path = Path(args.recovery_key).expanduser().absolute()
            if not recovery_key_path.resolve(strict=True).is_relative_to(recovery_root):
                raise CaptureError("recovery key must remain outside the snapshot under local private custody")
            mode_exact(recovery_key_path)
            if args.command == "restore-copy":
                if not RUN_ID_RE.fullmatch(args.run_id):
                    raise CaptureError("restore run-id format is invalid")
                result = restore_validate_copy(repo, snapshot, recovery_key_path, args.run_id, args.restore_image_ref)
                print(json.dumps(result, sort_keys=True))
                return 0
            output = local_testing_path(repo, args.output)
            output.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
            os.chmod(output.parent, 0o700)
            key = load_recovery_key(recovery_key_path)
            seal = check_encrypted_snapshot(snapshot, key)
            encrypted = snapshot / "contract-encryption-key.gpg"
            artifact = seal["encrypted_artifacts"]["contract-encryption-key.gpg"]
            if sha256_file(encrypted)[0] != artifact["cipher_sha256"]:
                raise CaptureError("contract key ciphertext checksum mismatch")
            gpg_decrypt_file(encrypted, output, derive_passphrase(key, seal["run_id"], "contract-key"), 60)
            print(json.dumps({"status": "recovered_to_private_file", "run_id": seal["run_id"], "output_mode_octal": "0600"}, sort_keys=True))
            return 0
    except (CaptureError, OSError, ValueError, KeyError, subprocess.SubprocessError) as exc:
        print(f"GCP-03 local capture stopped safely at {_STAGE}: {type(exc).__name__}; sensitive command output was suppressed.", file=sys.stderr)
        return 2
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
