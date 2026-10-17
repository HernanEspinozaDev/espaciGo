"""Validate and copy the sealed private-files tar without writing plaintext to disk.

This module intentionally stages files in memory until the complete tar and its
embedded capture manifest have been checked. Callers must provide an isolated,
private destination store; this module never serves files or changes database
references.
"""

from __future__ import annotations

import hashlib
import io
import json
import re
import tarfile
from dataclasses import dataclass
from pathlib import PurePosixPath
from typing import Protocol

ROOTS = {"private-evidence", "privacy-replay"}
MAX_ARCHIVE_BYTES = 64 * 1024 * 1024
MAX_FILE_COUNT = 20_000
SHA_RE = re.compile(r"^[0-9a-f]{64}$")


class PrivateFilesError(ValueError):
    pass


@dataclass(frozen=True)
class PrivateFile:
    object_name: str
    data: bytes
    sha256: str


class PrivateFileStore(Protocol):
    def create(self, object_name: str, data: bytes, sha256: str) -> None: ...


def _safe_name(name: str) -> PurePosixPath:
    path = PurePosixPath(name)
    if path.is_absolute() or not path.parts or any(part in ("", ".", "..") for part in path.parts) or "\\" in name or "\x00" in name:
        raise PrivateFilesError("unsafe archive member path")
    return path


def validate_private_files_tar(data: bytes, *, run_id: str) -> list[PrivateFile]:
    """Fully validate the authenticated archive before returning any file bytes."""
    if not data or len(data) > MAX_ARCHIVE_BYTES or not re.fullmatch(r"[a-z][a-z0-9-]{5,11}", run_id):
        raise PrivateFilesError("private archive size or run id is invalid")
    expected: dict[str, dict] | None = None
    seen: set[str] = set()
    directories: set[str] = set()
    files: list[PrivateFile] = []
    try:
        with tarfile.open(fileobj=io.BytesIO(data), mode="r:") as archive:
            for member in archive:
                path = _safe_name(member.name)
                if member.name in seen:
                    raise PrivateFilesError("duplicate archive member")
                seen.add(member.name)
                if member.name == "capture-file-manifest.json":
                    if expected is not None or not member.isfile() or member.mode != 0o600:
                        raise PrivateFilesError("capture manifest entry is invalid")
                    stream = archive.extractfile(member)
                    if stream is None:
                        raise PrivateFilesError("capture manifest is unreadable")
                    raw = stream.read(4 * 1024 * 1024 + 1)
                    if len(raw) > 4 * 1024 * 1024:
                        raise PrivateFilesError("capture manifest is too large")
                    parsed = json.loads(raw.decode("utf-8", "strict"))
                    if parsed.get("format") != "espacigo.gcp03.local-origin-capture.v1.files" or not isinstance(parsed.get("files"), list):
                        raise PrivateFilesError("capture manifest format is invalid")
                    expected = {}
                    for item in parsed["files"]:
                        if not isinstance(item, dict) or item.get("root") not in ROOTS:
                            raise PrivateFilesError("capture manifest root is invalid")
                        rel = _safe_name(item.get("relative_path", ""))
                        key = f"{item['root']}/{rel.as_posix()}"
                        if key in expected or type(item.get("bytes")) is not int or item["bytes"] < 0 or not SHA_RE.fullmatch(str(item.get("sha256", ""))) or item.get("mode_octal") != "0600":
                            raise PrivateFilesError("capture manifest file record is invalid")
                        expected[key] = item
                    if len(expected) > MAX_FILE_COUNT:
                        raise PrivateFilesError("capture manifest has too many files")
                    continue
                if expected is not None:
                    raise PrivateFilesError("capture manifest must be the final tar member")
                if len(seen) > MAX_FILE_COUNT * 2 + 10:
                    raise PrivateFilesError("private archive has too many members")
                if len(path.parts) == 1 and path.parts[0] in ROOTS:
                    if not member.isdir() or member.mode != 0o700:
                        raise PrivateFilesError("private root directory metadata is invalid")
                    directories.add(member.name)
                    continue
                if path.parts[0] not in ROOTS:
                    raise PrivateFilesError("archive member root is not allowed")
                if member.isdir():
                    if member.mode != 0o700:
                        raise PrivateFilesError("private directory mode is invalid")
                    directories.add(member.name)
                    continue
                if not member.isfile() or member.mode != 0o600 or member.size > MAX_ARCHIVE_BYTES:
                    raise PrivateFilesError("private file type, mode, or size is invalid")
                content = archive.extractfile(member)
                if content is None:
                    raise PrivateFilesError("private file is unreadable")
                payload = content.read(member.size + 1)
                digest = hashlib.sha256(payload).hexdigest()
                files.append(PrivateFile(f"runs/{run_id}/{member.name}", payload, digest))
                if len(files) > MAX_FILE_COUNT:
                    raise PrivateFilesError("private archive has too many files")
    except (tarfile.TarError, UnicodeError, json.JSONDecodeError, OSError) as exc:
        raise PrivateFilesError("private archive is malformed") from exc
    if expected is None:
        raise PrivateFilesError("capture manifest is missing")
    if directories.intersection(ROOTS) != ROOTS:
        raise PrivateFilesError("private root directories are incomplete")
    actual = {item.object_name.removeprefix(f"runs/{run_id}/"): item for item in files}
    if set(actual) != set(expected):
        raise PrivateFilesError("capture manifest file inventory does not match archive")
    for name, item in actual.items():
        record = expected[name]
        parts = PurePosixPath(name).parts
        if any(PurePosixPath(*parts[:index]).as_posix() not in directories for index in range(1, len(parts))):
            raise PrivateFilesError("private file parent directory is missing")
        if len(item.data) != record["bytes"] or item.sha256 != record["sha256"]:
            raise PrivateFilesError("private file hash or size does not match capture manifest")
    return files


def copy_private_files(data: bytes, *, run_id: str, store: PrivateFileStore) -> list[dict[str, object]]:
    """Validate the entire archive, then create-only copy files and return safe metadata."""
    files = validate_private_files_tar(data, run_id=run_id)
    reports = []
    for item in files:
        store.create(item.object_name, item.data, item.sha256)
        reports.append({"object_name": item.object_name, "bytes": len(item.data), "sha256": item.sha256})
    return reports
