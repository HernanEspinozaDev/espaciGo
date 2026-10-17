#!/usr/bin/env python3
"""Prepare a locally encrypted, recoverable transport envelope for a sealed origin.

This tool has no Google Cloud integration. It verifies the exact SEALED.json hash,
decrypts each source artifact as a stream, and immediately re-encrypts fixed-size
chunks under per-artifact AES-256-GCM keys. Plaintext is never written to disk.
"""

from __future__ import annotations

import argparse
import hashlib
import hmac
import json
import os
from pathlib import Path
import re
import secrets
import stat
import struct
import subprocess
import sys
from typing import BinaryIO

from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives.kdf.hkdf import HKDF
from cryptography.hazmat.primitives import hashes

FORMAT = "espacigo.gcp03.origin-transport.v1"
SOURCE_SEAL_FORMAT = "espacigo.gcp03.local-origin-capture.v1.sealed"
RUN_RE = re.compile(r"^[a-z0-9][a-z0-9-]{5,63}$")
SHA_RE = re.compile(r"^[0-9a-f]{64}$")
ARTIFACTS = {
    "database.dump.gpg": "database.dump.gcm",
    "private-files.tar.gpg": "private-files.tar.gcm",
    "contract-encryption-key.gpg": "contract-encryption-key.gcm",
    "origin-metadata.json.gpg": "origin-metadata.json.gcm",
}
CHUNK_BYTES = 1024 * 1024
MAGIC = b"EG03GCM1"
INFO_PREFIX = b"espacigo/gcp03/origin/"


class ImportErrorSafe(RuntimeError):
    pass


def sha256_file(path: Path) -> tuple[str, int]:
    digest, size = hashlib.sha256(), 0
    with path.open("rb") as stream:
        while chunk := stream.read(1024 * 1024):
            digest.update(chunk)
            size += len(chunk)
    return digest.hexdigest(), size


def private_file(path: Path) -> bytes:
    if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
        raise ImportErrorSafe("recovery key must be a regular file with mode 0600")
    value = path.read_bytes().strip()
    if not re.fullmatch(rb"[0-9a-f]{64}", value):
        raise ImportErrorSafe("recovery key format is invalid")
    return bytes.fromhex(value.decode("ascii"))


def derive_source_passphrase(master: bytes, source_capture_run_id: str, label: str) -> bytes:
    msg = f"espacigo.gcp03.local-origin-capture.v1:{source_capture_run_id}:{label}".encode()
    return hmac.new(master, msg, hashlib.sha256).hexdigest().encode("ascii")


def transport_key(master: bytes, salt: bytes, target_run_id: str) -> bytes:
    return HKDF(algorithm=hashes.SHA256(), length=32, salt=salt,
                info=INFO_PREFIX + target_run_id.encode() + b"/transport/v1").derive(master)


def verify_seal(snapshot: Path, source_capture_run_id: str, expected_sha: str) -> tuple[dict, bytes]:
    if not RUN_RE.fullmatch(source_capture_run_id) or not SHA_RE.fullmatch(expected_sha):
        raise ImportErrorSafe("run ID or expected SEALED.json SHA-256 is invalid")
    if snapshot.is_symlink() or not snapshot.is_dir() or snapshot.name != source_capture_run_id:
        raise ImportErrorSafe("snapshot directory must be the real directory named by source capture run ID")
    seal_path = snapshot / "SEALED.json"
    checksum_path = snapshot / "SEALED.sha256"
    if seal_path.is_symlink() or checksum_path.is_symlink():
        raise ImportErrorSafe("seal files cannot be symlinks")
    if stat.S_IMODE(seal_path.stat().st_mode) not in {0o400, 0o600} or stat.S_IMODE(checksum_path.stat().st_mode) not in {0o400, 0o600}:
        raise ImportErrorSafe("seal file permissions are invalid")
    seal_bytes = seal_path.read_bytes()
    actual_sha = hashlib.sha256(seal_bytes).hexdigest()
    if actual_sha != expected_sha:
        raise ImportErrorSafe("SEALED.json does not match the reviewed hash")
    expected_line = f"{actual_sha}  SEALED.json\n".encode("ascii")
    if checksum_path.read_bytes() != expected_line:
        raise ImportErrorSafe("SEALED.sha256 does not bind the exact seal")
    try:
        seal = json.loads(seal_bytes)
    except (json.JSONDecodeError, UnicodeDecodeError) as exc:
        raise ImportErrorSafe("SEALED.json is invalid") from exc
    names = set(seal.get("encrypted_artifacts", {}))
    if seal.get("format") != SOURCE_SEAL_FORMAT or seal.get("run_id") != source_capture_run_id or \
            seal.get("v1_v61_preserved") is not True or seal.get("custody_key_in_artifact") is not False or \
            seal.get("database_volume_in_artifact") is not False or seal.get("source_secret_files_included") is not False or \
            seal.get("contract_encryption_key_separately_encrypted") is not True or names != set(ARTIFACTS):
        raise ImportErrorSafe("seal identity, V1-V61 claim, or artifact allowlist is invalid")
    if {p.name for p in snapshot.iterdir()} != names | {"SEALED.json", "SEALED.sha256"}:
        raise ImportErrorSafe("snapshot has missing or unmanifested files")
    for name, descriptor in seal["encrypted_artifacts"].items():
        path = snapshot / name
        if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) not in {0o400, 0o600}:
            raise ImportErrorSafe("sealed artifact type or permissions are invalid")
        digest, size = sha256_file(path)
        if digest != descriptor.get("cipher_sha256", descriptor.get("encrypted_file_sha256")):
            raise ImportErrorSafe("sealed artifact hash mismatch")
        if descriptor.get("cipher_bytes", size) != size:
            raise ImportErrorSafe("sealed artifact length mismatch")
    return seal, seal_bytes


def encrypt_chunks(source: BinaryIO, destination: BinaryIO, key: bytes, target_run_id: str, artifact: str) -> dict:
    aead = AESGCM(key)
    digest = hashlib.sha256()
    plain_bytes = chunks = 0
    destination.write(MAGIC)
    while data := source.read(CHUNK_BYTES):
        nonce = secrets.token_bytes(12)
        aad = f"{FORMAT}:{target_run_id}:{artifact}:{chunks}".encode()
        encrypted = aead.encrypt(nonce, data, aad)
        destination.write(struct.pack(">I", len(data)))
        destination.write(nonce)
        destination.write(encrypted)
        digest.update(data)
        plain_bytes += len(data)
        chunks += 1
    destination.write(struct.pack(">I", 0))
    return {"plain_sha256": digest.hexdigest(), "plain_bytes": plain_bytes, "chunks": chunks}


def decrypt_chunks(source: BinaryIO, destination: BinaryIO, key: bytes, target_run_id: str, artifact: str) -> dict:
    if source.read(len(MAGIC)) != MAGIC:
        raise ImportErrorSafe("transport artifact header is invalid")
    aead, digest, plain_bytes, index = AESGCM(key), hashlib.sha256(), 0, 0
    while True:
        raw_length = source.read(4)
        if len(raw_length) != 4:
            raise ImportErrorSafe("transport artifact is truncated")
        length = struct.unpack(">I", raw_length)[0]
        if length == 0:
            if source.read(1):
                raise ImportErrorSafe("transport artifact has trailing bytes")
            break
        if length > CHUNK_BYTES:
            raise ImportErrorSafe("transport chunk exceeds the fixed bound")
        nonce, encrypted = source.read(12), source.read(length + 16)
        if len(nonce) != 12 or len(encrypted) != length + 16:
            raise ImportErrorSafe("transport artifact is truncated")
        aad = f"{FORMAT}:{target_run_id}:{artifact}:{index}".encode()
        try:
            data = aead.decrypt(nonce, encrypted, aad)
        except Exception as exc:
            raise ImportErrorSafe("transport chunk authentication failed") from exc
        destination.write(data)
        digest.update(data)
        plain_bytes += len(data)
        index += 1
    return {"plain_sha256": digest.hexdigest(), "plain_bytes": plain_bytes, "chunks": index}


def gpg_decrypt(source: Path, passphrase: bytes):
    read_fd, write_fd = os.pipe()
    process = subprocess.Popen(
        ["gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", str(read_fd), "--decrypt", str(source)],
        stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, pass_fds=(read_fd,),
    )
    os.close(read_fd)
    try:
        os.write(write_fd, passphrase + b"\n")
    finally:
        os.close(write_fd)
    assert process.stdout is not None
    return process, process.stdout


def verify_source_metadata(snapshot: Path, master: bytes, source_capture_run_id: str) -> str:
    process, output = gpg_decrypt(snapshot / "origin-metadata.json.gpg", derive_source_passphrase(master, source_capture_run_id, "metadata"))
    try:
        metadata = json.load(output)
        rc = process.wait(timeout=180)
    except BaseException:
        process.kill()
        process.wait()
        raise
    finally:
        output.close()
    if rc != 0:
        raise ImportErrorSafe("sealed source metadata authentication failed")
    report = metadata.get("database_report") or {}
    preflight = metadata.get("preflight") or {}
    projection = preflight.get("source_projection") or {}
    release = metadata.get("release") or {}
    source_database = projection.get("configured_database_name")
    try:
        source_database_bytes = len(source_database.encode("utf-8")) if isinstance(source_database, str) else 0
    except UnicodeEncodeError:
        raise ImportErrorSafe("sealed source metadata does not prove the V1-V61 and credential-exclusion contract") from None
    if not isinstance(source_database, str) or not source_database or "\x00" in source_database or source_database_bytes > 63 or \
            report.get("database") != source_database or release.get("run_id") != source_capture_run_id or \
            report.get("migration_match_v1_v61") is not True or \
            report.get("source_credentials_included") is not False or report.get("role_passwords_included") is not False:
        raise ImportErrorSafe("sealed source metadata does not prove the V1-V61 and credential-exclusion contract")
    history = report.get("migration_history")
    if not isinstance(history, list) or len(history) != 61:
        raise ImportErrorSafe("sealed source migration history is not exactly V1-V61")
    expected = {}
    migrations_dir = Path(__file__).resolve().parents[1] / "db" / "migrations"
    for version in range(1, 62):
        matches = list(migrations_dir.glob(f"V{version:06d}__*.sql"))
        if len(matches) != 1 or matches[0].is_symlink():
            raise ImportErrorSafe("working tree must contain exactly one regular V1-V61 migration file per version")
        name = matches[0].name
        expected[version] = (name, sha256_file(matches[0])[0])
    for version, entry in enumerate(history, start=1):
        try:
            found = (entry["name"], entry["checksum"])
        except (KeyError, TypeError) as exc:
            raise ImportErrorSafe("sealed migration history entry is malformed") from exc
        if found != expected[version]:
            raise ImportErrorSafe("current V1-V61 migration bytes differ from the sealed source history")
    return hashlib.sha256("".join(f"{v}:{n}:{h}\n" for v, (n, h) in expected.items()).encode()).hexdigest()


def private_output_dir(path: Path) -> Path:
    path = path.expanduser()
    if not path.is_absolute():
        raise ImportErrorSafe("output must be an absolute new path under .local/testing")
    current = Path(path.anchor)
    for part in path.parts[1:]:
        current /= part
        if current.exists() and current.is_symlink():
            raise ImportErrorSafe("output path cannot contain symlinks")
    resolved = path.resolve()
    testing = (Path(__file__).resolve().parents[1] / ".local" / "testing").resolve()
    if not resolved.is_relative_to(testing) or resolved.exists():
        raise ImportErrorSafe("output must be a new directory under this repository's .local/testing")
    resolved.mkdir(parents=True, mode=0o700)
    os.chmod(resolved, 0o700)
    return resolved


def stage(snapshot: Path, key_path: Path, source_capture_run_id: str, target_run_id: str, expected_sha: str, output: Path) -> dict:
    master = private_file(key_path)
    seal, _ = verify_seal(snapshot, source_capture_run_id, expected_sha)
    if not RUN_RE.fullmatch(target_run_id):
        raise ImportErrorSafe("target run ID is invalid")
    migration_inventory_sha = verify_source_metadata(snapshot, master, source_capture_run_id)
    out = private_output_dir(output)
    salt = secrets.token_bytes(32)
    kek = transport_key(master, salt, target_run_id)
    manifest = {"format": FORMAT, "source_capture_run_id": source_capture_run_id,
                "target_run_id": target_run_id, "source_sealed_sha256": expected_sha,
                "kdf": "HKDF-SHA-256", "salt_hex": salt.hex(), "artifacts": {},
                "migration_inventory_sha256": migration_inventory_sha,
                "import_contract": {
                    "format": "espacigo.gcp03.origin-import-contract.v1",
                    "status": "staged_ciphertext_only",
                    "source_capture_run_id": source_capture_run_id,
                    "target_run_id": target_run_id,
                    "target_database": "espacigo_scratch",
                    "requires_empty_target": True,
                    "requires_reviewed_source_seal_sha256": expected_sha,
                    "requires_immutable_restore_image_digest": True,
                    "requires_hash_bound_observer_and_bootstrap_evidence": True,
                    "restore_attempts": 1,
                    "automatic_retries": 0,
                    "application_services_and_workers_may_start": False,
                    "sanitize_copy_before_verification": True,
                    "source_mutation_allowed": False,
                    "cloud_upload_performed": False,
                }}
    try:
        for source_name, output_name in ARTIFACTS.items():
            dek, wrap_nonce = secrets.token_bytes(32), secrets.token_bytes(12)
            artifact_key = source_name.removesuffix(".gpg")
            aad = f"{FORMAT}:{target_run_id}:{artifact_key}:dek".encode()
            wrapped = AESGCM(kek).encrypt(wrap_nonce, dek, aad)
            target = out / output_name
            source_path = snapshot / source_name
            source_digest, source_size = sha256_file(source_path)
            proc, plain_stream = gpg_decrypt(source_path, derive_source_passphrase(master, source_capture_run_id, {
                "database.dump.gpg": "database-dump", "private-files.tar.gpg": "private-files",
                "contract-encryption-key.gpg": "contract-key", "origin-metadata.json.gpg": "metadata",
            }[source_name]))
            try:
                with target.open("xb") as encrypted_file:
                    os.chmod(target, 0o600)
                    stats = encrypt_chunks(plain_stream, encrypted_file, dek, target_run_id, artifact_key)
                    encrypted_file.flush()
                    os.fsync(encrypted_file.fileno())
                if proc.wait(timeout=3600) != 0:
                    raise ImportErrorSafe("source authenticated decryption failed")
            except BaseException:
                proc.kill()
                proc.wait()
                raise
            finally:
                plain_stream.close()
            cipher_digest, cipher_size = sha256_file(target)
            manifest["artifacts"][artifact_key] = {
                "source_name": source_name, "object_name": f"runs/{target_run_id}/{output_name}",
                "source_cipher_sha256": source_digest, "source_cipher_bytes": source_size,
                "transport_cipher_sha256": cipher_digest, "transport_cipher_bytes": cipher_size,
                "plain_sha256": stats["plain_sha256"], "plain_bytes": stats["plain_bytes"],
                "chunk_count": stats["chunks"], "chunk_size": CHUNK_BYTES,
                "wrap_nonce_hex": wrap_nonce.hex(), "wrapped_dek_hex": wrapped.hex(),
            }
        manifest_path = out / "transport-manifest.json"
        with manifest_path.open("xb") as stream:
            os.chmod(manifest_path, 0o600)
            stream.write((json.dumps(manifest, sort_keys=True, indent=2) + "\n").encode())
            stream.flush()
            os.fsync(stream.fileno())
        return {"source_capture_run_id": source_capture_run_id, "target_run_id": target_run_id,
                "source_sealed_sha256": expected_sha,
                "transport_manifest_sha256": sha256_file(manifest_path)[0],
                "artifact_count": len(manifest["artifacts"]), "status": "staged_ciphertext_only"}
    except BaseException:
        # Preserve partial ciphertext for diagnosis; never delete the source seal.
        raise


def recover_verify(stage_dir: Path, key_path: Path, source_capture_run_id: str, target_run_id: str, expected_manifest_sha: str) -> dict:
    master = private_file(key_path)
    if not RUN_RE.fullmatch(source_capture_run_id) or not RUN_RE.fullmatch(target_run_id):
        raise ImportErrorSafe("source capture or target run ID is invalid")
    manifest_path = stage_dir / "transport-manifest.json"
    if stage_dir.is_symlink() or not stage_dir.is_dir() or stat.S_IMODE(stage_dir.stat().st_mode) != 0o700 or \
            manifest_path.is_symlink() or not manifest_path.is_file() or stat.S_IMODE(manifest_path.stat().st_mode) != 0o600:
        raise ImportErrorSafe("transport directory or manifest permissions are invalid")
    if not SHA_RE.fullmatch(expected_manifest_sha) or sha256_file(manifest_path)[0] != expected_manifest_sha:
        raise ImportErrorSafe("transport manifest does not match the reviewed hash")
    if {entry.name for entry in stage_dir.iterdir()} != set(ARTIFACTS.values()) | {"transport-manifest.json"}:
        raise ImportErrorSafe("transport directory contains missing or unmanifested paths")
    manifest = json.loads(manifest_path.read_bytes())
    if manifest.get("format") != FORMAT or manifest.get("source_capture_run_id") != source_capture_run_id or \
            manifest.get("target_run_id") != target_run_id or \
            not SHA_RE.fullmatch(str(manifest.get("source_sealed_sha256", ""))) or \
            set(manifest.get("artifacts", {})) != {n.removesuffix(".gpg") for n in ARTIFACTS}:
        raise ImportErrorSafe("transport manifest identity or allowlist is invalid")
    contract = manifest.get("import_contract") or {}
    if contract.get("format") != "espacigo.gcp03.origin-import-contract.v1" or \
            contract.get("status") != "staged_ciphertext_only" or contract.get("target_database") != "espacigo_scratch" or \
            contract.get("source_capture_run_id") != source_capture_run_id or contract.get("target_run_id") != target_run_id or \
            contract.get("requires_empty_target") is not True or contract.get("restore_attempts") != 1 or \
            contract.get("automatic_retries") != 0 or contract.get("application_services_and_workers_may_start") is not False or \
            contract.get("sanitize_copy_before_verification") is not True or contract.get("source_mutation_allowed") is not False or \
            contract.get("cloud_upload_performed") is not False or contract.get("requires_reviewed_source_seal_sha256") != manifest.get("source_sealed_sha256"):
        raise ImportErrorSafe("transport import contract is invalid")
    salt = bytes.fromhex(manifest["salt_hex"])
    if len(salt) != 32:
        raise ImportErrorSafe("transport salt is invalid")
    kek = transport_key(master, salt, target_run_id)
    verified = {}
    for artifact, entry in manifest["artifacts"].items():
        source_name = entry["source_name"]
        if source_name not in ARTIFACTS or entry.get("object_name") != f"runs/{target_run_id}/{ARTIFACTS[source_name]}":
            raise ImportErrorSafe("transport object path is invalid")
        path = stage_dir / ARTIFACTS[source_name]
        if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
            raise ImportErrorSafe("transport artifact is absent or has unsafe permissions")
        cipher_hash, cipher_size = sha256_file(path)
        if cipher_hash != entry.get("transport_cipher_sha256") or cipher_size != entry.get("transport_cipher_bytes"):
            raise ImportErrorSafe("transport ciphertext hash mismatch")
        nonce = bytes.fromhex(entry["wrap_nonce_hex"])
        dek = AESGCM(kek).decrypt(nonce, bytes.fromhex(entry["wrapped_dek_hex"]), f"{FORMAT}:{target_run_id}:{artifact}:dek".encode())
        with path.open("rb") as stream:
            stats = decrypt_chunks(stream, _HashSink(), dek, target_run_id, artifact)
        if stats["plain_sha256"] != entry.get("plain_sha256") or stats["plain_bytes"] != entry.get("plain_bytes") or stats["chunks"] != entry.get("chunk_count"):
            raise ImportErrorSafe("transport plaintext hash or length mismatch")
        verified[artifact] = stats
    return {"source_capture_run_id": source_capture_run_id, "target_run_id": target_run_id,
            "source_sealed_sha256": manifest["source_sealed_sha256"],
            "verified_artifacts": len(verified), "plaintext_written": False}


class _HashSink:
    def write(self, value: bytes) -> int:
        return len(value)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="command", required=True)
    p_stage = sub.add_parser("stage", help="verify the seal and create a local encrypted transport envelope")
    p_stage.add_argument("--snapshot", type=Path, required=True)
    p_stage.add_argument("--recovery-key", type=Path, required=True)
    p_stage.add_argument("--source-capture-run-id", required=True)
    p_stage.add_argument("--target-run-id", required=True)
    p_stage.add_argument("--expected-seal-sha256", required=True)
    p_stage.add_argument("--output", type=Path, required=True)
    p_verify = sub.add_parser("verify-recovery", help="locally authenticate every staged artifact without writing plaintext")
    p_verify.add_argument("--stage-dir", type=Path, required=True)
    p_verify.add_argument("--recovery-key", type=Path, required=True)
    p_verify.add_argument("--source-capture-run-id", required=True)
    p_verify.add_argument("--target-run-id", required=True)
    p_verify.add_argument("--expected-manifest-sha256", required=True)
    args = parser.parse_args()
    try:
        result = stage(args.snapshot, args.recovery_key, args.source_capture_run_id, args.target_run_id, args.expected_seal_sha256, args.output) if args.command == "stage" else recover_verify(args.stage_dir, args.recovery_key, args.source_capture_run_id, args.target_run_id, args.expected_manifest_sha256)
        print(json.dumps(result, sort_keys=True))
    except (ImportErrorSafe, OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
        print(json.dumps({"status": "failed", "reason": str(exc)}, sort_keys=True), file=sys.stderr)
        raise SystemExit(2)


if __name__ == "__main__":
    main()
