import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

import gcp03_origin_cloud_uploader as uploader


class CreateOnlyBlob:
    def __init__(self, name, objects):
        self.name, self.objects = name, objects
        self.generation = None
        self.size = None

    def upload_from_string(self, data, *, content_type, if_generation_match, retry):
        if if_generation_match != 0 or retry is not None or self.name in self.objects:
            raise AssertionError("upload was not create-only")
        self.objects[self.name] = bytes(data)
        self.generation = len(self.objects)
        self.size = len(data)

    def __getattr__(self, name):
        raise AssertionError(f"object read/metadata call is forbidden: {name}")


class CreateOnlyBucket:
    def __init__(self):
        self.objects = {}

    def blob(self, name):
        return CreateOnlyBlob(name, self.objects)

    def __getattr__(self, name):
        raise AssertionError(f"bucket read/list/delete call is forbidden: {name}")


class CreateOnlyClient:
    def __init__(self):
        self.store = CreateOnlyBucket()
        self.bucket_calls = 0

    def bucket(self, name):
        self.bucket_calls += 1
        if name != uploader.BUCKET:
            raise AssertionError("uploader selected an unexpected bucket")
        return self.store

    def __getattr__(self, name):
        raise AssertionError(f"client read/list/delete call is forbidden: {name}")


def _write_private(path: Path, data: bytes):
    path.write_bytes(data)
    path.chmod(0o600)
    return hashlib.sha256(data).hexdigest()


class CloudUploaderTests(unittest.TestCase):
    def _inputs(self, root: Path):
        stage = root / "stage"
        stage.mkdir(mode=0o700)
        master = bytes(range(32))
        key = root / "recovery-key"
        key.write_text(master.hex())
        key.chmod(0o600)
        salt = b"s" * 32
        kek = uploader.transport.transport_key(master, salt, uploader.RUN_ID)
        artifacts = {}
        for i, (source_name, output_name) in enumerate(uploader.transport.ARTIFACTS.items()):
            artifact_key = source_name.removesuffix(".gpg")
            plaintext = f"synthetic authenticated input {i}".encode()
            dek, wrap_nonce = bytes([i + 1]) * 32, bytes([i + 1]) * 12
            wrapped = AESGCM(kek).encrypt(wrap_nonce, dek,
                f"{uploader.transport.FORMAT}:{uploader.RUN_ID}:{artifact_key}:dek".encode())
            encrypted = io.BytesIO()
            chunk_stats = uploader.transport.encrypt_chunks(
                io.BytesIO(plaintext), encrypted, dek, uploader.RUN_ID, artifact_key)
            payload = encrypted.getvalue()
            cipher_hash = _write_private(stage / output_name, payload)
            artifacts[artifact_key] = {
                "source_name": source_name,
                "object_name": f"runs/{uploader.RUN_ID}/{output_name}",
                "source_cipher_sha256": "b" * 64, "source_cipher_bytes": 1,
                "transport_cipher_sha256": cipher_hash, "transport_cipher_bytes": len(payload),
                "plain_sha256": chunk_stats["plain_sha256"], "plain_bytes": chunk_stats["plain_bytes"],
                "chunk_count": chunk_stats["chunks"], "chunk_size": 1024 * 1024,
                "wrap_nonce_hex": wrap_nonce.hex(), "wrapped_dek_hex": wrapped.hex(),
            }
        seal_sha = "a" * 64
        manifest = {"format": uploader.transport.FORMAT,
            "source_capture_run_id": uploader.SOURCE_RUN_ID, "target_run_id": uploader.RUN_ID,
            "source_sealed_sha256": seal_sha, "kdf": "HKDF-SHA-256", "salt_hex": salt.hex(),
            "artifacts": artifacts, "migration_inventory_sha256": "c" * 64,
            "import_contract": {"format": "espacigo.gcp03.origin-import-contract.v1",
                "status": "staged_ciphertext_only", "source_capture_run_id": uploader.SOURCE_RUN_ID,
                "target_run_id": uploader.RUN_ID, "target_database": "espacigo_scratch",
                "requires_empty_target": True, "requires_reviewed_source_seal_sha256": seal_sha,
                "requires_immutable_restore_image_digest": True,
                "requires_hash_bound_observer_and_bootstrap_evidence": True,
                "restore_attempts": 1, "automatic_retries": 0,
                "application_services_and_workers_may_start": False,
                "sanitize_copy_before_verification": True, "source_mutation_allowed": False,
                "cloud_upload_performed": False}}
        manifest_hash = _write_private(stage / "transport-manifest.json",
            json.dumps(manifest, sort_keys=True, separators=(",", ":")).encode())
        normal_hash = _write_private(root / "normalization.sql", b"reviewed normalization SQL")
        toc_hash = _write_private(root / "toc.list", b"reviewed TOC allowlist")
        bootstrap_hash = _write_private(root / "bootstrap.json", b"bootstrap evidence")
        observer_hash = _write_private(root / "observer.json", b"observer evidence")
        return {"stage_dir": stage, "recovery_key": key,
            "source_run_id": uploader.SOURCE_RUN_ID, "target_run_id": uploader.RUN_ID,
            "expected_seal_sha256": seal_sha, "expected_manifest_sha256": manifest_hash,
            "normalization_sql": root / "normalization.sql", "expected_normalization_sha256": normal_hash,
            "toc_allowlist": root / "toc.list", "expected_toc_sha256": toc_hash,
            "bootstrap_report": root / "bootstrap.json", "expected_bootstrap_sha256": bootstrap_hash,
            "observer_report": root / "observer.json", "expected_observer_sha256": observer_hash,
            "receipt_path": root / "receipt.json"}

    def test_upload_is_create_only_uses_write_metadata_and_publishes_receipt_last(self):
        with tempfile.TemporaryDirectory() as tmp:
            args = self._inputs(Path(tmp))
            client = CreateOnlyClient()
            result = uploader.upload_package(**args, client=client)
            expected = {f"runs/{uploader.RUN_ID}/{name}" for name in uploader.ARTIFACTS}
            expected |= {f"runs/{uploader.RUN_ID}/normalization.sql", f"runs/{uploader.RUN_ID}/toc.list",
                         f"runs/{uploader.RUN_ID}/transport-manifest.json", f"runs/{uploader.RUN_ID}/receipt.json"}
            self.assertEqual(set(client.store.objects), expected)
            self.assertEqual(list(client.store.objects)[-1], f"runs/{uploader.RUN_ID}/receipt.json")
            self.assertEqual(list(client.store.objects)[-2], f"runs/{uploader.RUN_ID}/transport-manifest.json")
            receipt = json.loads(client.store.objects[f"runs/{uploader.RUN_ID}/receipt.json"])
            self.assertEqual(set(receipt["artifacts"]), {name.removesuffix(".gcm") for name in uploader.ARTIFACTS})
            self.assertEqual(result["status"], "uploaded_ciphertext_only")
            self.assertEqual(result["artifact_count"], 4)

    def test_invalid_local_artifact_stops_before_any_create_request(self):
        with tempfile.TemporaryDirectory() as tmp:
            args = self._inputs(Path(tmp))
            client = CreateOnlyClient()
            original_verify = uploader.transport.recover_verify
            def verify_then_tamper(*verify_args):
                result = original_verify(*verify_args)
                (args["stage_dir"] / uploader.ARTIFACTS[-1]).write_bytes(b"tampered after verification")
                return result
            with mock.patch.object(uploader.transport, "recover_verify", side_effect=verify_then_tamper):
                with self.assertRaises(uploader.UploadError):
                    uploader.upload_package(**args, client=client)
            self.assertEqual(client.store.objects, {})
            self.assertEqual(client.bucket_calls, 0)


if __name__ == "__main__":
    unittest.main()
