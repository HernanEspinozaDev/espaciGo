#!/usr/bin/env python3
"""Unit tests for the GCP-03 sealed-origin transport envelope."""

from __future__ import annotations

import hashlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import gcp03_origin_import as importer


class OriginImportTests(unittest.TestCase):
    def test_chunk_envelope_round_trip_and_tamper_detection(self):
        target_run_id, artifact, key = "g03o-1017", "database.dump", bytes(range(32))
        source = (b"origin-payload" * 90000) + b"tail"
        encrypted = io.BytesIO()
        stats = importer.encrypt_chunks(io.BytesIO(source), encrypted, key, target_run_id, artifact)
        self.assertEqual(stats["plain_sha256"], hashlib.sha256(source).hexdigest())
        self.assertEqual(stats["chunks"], 2)
        restored = io.BytesIO()
        decrypted = importer.decrypt_chunks(io.BytesIO(encrypted.getvalue()), restored, key, target_run_id, artifact)
        self.assertEqual(restored.getvalue(), source)
        self.assertEqual(decrypted, stats)
        corrupted = bytearray(encrypted.getvalue())
        corrupted[30] ^= 0x40
        with self.assertRaises(importer.ImportErrorSafe):
            importer.decrypt_chunks(io.BytesIO(corrupted), io.BytesIO(), key, target_run_id, artifact)

    def test_seal_hash_binds_expected_snapshot_and_allowlist(self):
        with tempfile.TemporaryDirectory() as temp:
            snapshot = Path(temp) / "g03o-1017"
            snapshot.mkdir()
            artifacts = {}
            for name in importer.ARTIFACTS:
                payload = ("cipher-" + name).encode()
                (snapshot / name).write_bytes(payload)
                os.chmod(snapshot / name, 0o600)
                artifacts[name] = {"cipher_sha256": hashlib.sha256(payload).hexdigest(), "cipher_bytes": len(payload)}
            seal = {"format": importer.SOURCE_SEAL_FORMAT, "run_id": snapshot.name,
                    "v1_v61_preserved": True, "custody_key_in_artifact": False,
                    "database_volume_in_artifact": False, "source_secret_files_included": False,
                    "contract_encryption_key_separately_encrypted": True, "encrypted_artifacts": artifacts}
            raw = json.dumps(seal, sort_keys=True, separators=(",", ":")).encode()
            seal_sha = hashlib.sha256(raw).hexdigest()
            (snapshot / "SEALED.json").write_bytes(raw)
            os.chmod(snapshot / "SEALED.json", 0o600)
            (snapshot / "SEALED.sha256").write_text(f"{seal_sha}  SEALED.json\n")
            os.chmod(snapshot / "SEALED.sha256", 0o600)
            verified, _ = importer.verify_seal(snapshot, snapshot.name, seal_sha)
            self.assertEqual(verified["run_id"], "g03o-1017")
            with self.assertRaises(importer.ImportErrorSafe):
                importer.verify_seal(snapshot, snapshot.name, "0" * 64)

    def test_hkdf_context_is_run_bound(self):
        master, salt = bytes(range(32)), bytes(reversed(range(32)))
        self.assertNotEqual(importer.transport_key(master, salt, "g03o-1017"),
                            importer.transport_key(master, salt, "g03o-1018"))

    def test_authenticated_metadata_must_match_current_v1_v61_bytes(self):
        migrations_dir = Path(importer.__file__).resolve().parents[1] / "db" / "migrations"
        history = []
        for version in range(1, 62):
            path, = migrations_dir.glob(f"V{version:06d}__*.sql")
            history.append({"name": path.name, "checksum": hashlib.sha256(path.read_bytes()).hexdigest()})
        metadata = {"database_report": {"database": "espacigo", "migration_match_v1_v61": True,
                                         "source_credentials_included": False, "role_passwords_included": False,
                                         "migration_history": history}}

        class Process:
            def wait(self, timeout=None):
                return 0

            def kill(self):
                pass

        snapshot = Path("/tmp/gcp03-import-test-snapshot")
        with patch.object(importer, "gpg_decrypt", return_value=(Process(), io.BytesIO(json.dumps(metadata).encode()))):
            self.assertEqual(len(importer.verify_source_metadata(snapshot, bytes(32), "g03o-1017")), 64)
        history[32]["checksum"] = "0" * 64
        with patch.object(importer, "gpg_decrypt", return_value=(Process(), io.BytesIO(json.dumps(metadata).encode()))):
            with self.assertRaises(importer.ImportErrorSafe):
                importer.verify_source_metadata(snapshot, bytes(32), "g03o-1017")

    @unittest.skipUnless(shutil.which("gpg"), "GnuPG is required for the streaming staging integration test")
    def test_synthetic_gpg_snapshot_stages_and_recovers_without_plaintext_files(self):
        repo_root = Path(importer.__file__).resolve().parents[1]
        testing = repo_root / ".local" / "testing"
        master = bytes(range(32))
        source_capture_run_id = "gcp03-merge-review-20261017"
        target_run_id = "g03o-1017"
        with tempfile.TemporaryDirectory(prefix="gcp03-origin-import-test-", dir=testing) as root_text:
            root = Path(root_text)
            os.chmod(root, 0o700)
            snapshot = root / source_capture_run_id
            snapshot.mkdir(mode=0o700)
            key_file = root / "recovery.key"
            key_file.write_text(master.hex() + "\n")
            os.chmod(key_file, 0o600)
            migrations_dir = repo_root / "db" / "migrations"
            history = []
            for version in range(1, 62):
                path, = migrations_dir.glob(f"V{version:06d}__*.sql")
                history.append({"name": path.name, "checksum": hashlib.sha256(path.read_bytes()).hexdigest()})
            metadata = {"database_report": {"database": "espacigo", "migration_match_v1_v61": True,
                                             "source_credentials_included": False, "role_passwords_included": False,
                                             "migration_history": history}}
            plaintexts = {
                "database.dump.gpg": b"synthetic custom archive bytes\0" * 40000,
                "private-files.tar.gpg": b"synthetic private tar bytes\0" * 9000,
                "contract-encryption-key.gpg": b"synthetic contract key bytes",
                "origin-metadata.json.gpg": json.dumps(metadata).encode(),
            }
            labels = {"database.dump.gpg": "database-dump", "private-files.tar.gpg": "private-files",
                      "contract-encryption-key.gpg": "contract-key", "origin-metadata.json.gpg": "metadata"}
            descriptors = {}
            for name, plaintext in plaintexts.items():
                path = snapshot / name
                clear_path = root / (name + ".clear")
                clear_path.write_bytes(plaintext)
                os.chmod(clear_path, 0o600)
                encrypted = subprocess.run([
                    "gpg", "--batch", "--pinentry-mode", "loopback", "--passphrase-fd", "0",
                    "--symmetric", "--force-aead", "--cipher-algo", "AES256", "--output", str(path), str(clear_path),
                ], input=importer.derive_source_passphrase(master, source_capture_run_id, labels[name]) + b"\n",
                   check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                self.assertEqual(encrypted.returncode, 0)
                clear_path.unlink()
                os.chmod(path, 0o600)
                digest, size = importer.sha256_file(path)
                descriptors[name] = {"cipher_sha256": digest, "cipher_bytes": size}
            seal = {"format": importer.SOURCE_SEAL_FORMAT, "run_id": source_capture_run_id,
                    "v1_v61_preserved": True, "custody_key_in_artifact": False,
                    "database_volume_in_artifact": False, "source_secret_files_included": False,
                    "contract_encryption_key_separately_encrypted": True, "encrypted_artifacts": descriptors}
            seal_bytes = json.dumps(seal, sort_keys=True, separators=(",", ":")).encode()
            seal_sha = hashlib.sha256(seal_bytes).hexdigest()
            (snapshot / "SEALED.json").write_bytes(seal_bytes)
            (snapshot / "SEALED.sha256").write_text(f"{seal_sha}  SEALED.json\n")
            os.chmod(snapshot / "SEALED.json", 0o600)
            os.chmod(snapshot / "SEALED.sha256", 0o600)
            staged = root / "staged"
            result = importer.stage(snapshot, key_file, source_capture_run_id, target_run_id, seal_sha, staged)
            recovered = importer.recover_verify(staged, key_file, source_capture_run_id, target_run_id, result["transport_manifest_sha256"])
            self.assertEqual(result["status"], "staged_ciphertext_only")
            self.assertEqual(result["source_capture_run_id"], source_capture_run_id)
            self.assertEqual(result["target_run_id"], target_run_id)
            self.assertEqual(recovered["verified_artifacts"], 4)
            self.assertFalse(recovered["plaintext_written"])
            self.assertFalse(any(path.name.endswith(".clear") for path in root.iterdir()))
            manifest = json.loads((staged / "transport-manifest.json").read_bytes())
            self.assertEqual(manifest["source_capture_run_id"], source_capture_run_id)
            self.assertEqual(manifest["target_run_id"], target_run_id)
            self.assertEqual(manifest["artifacts"]["database.dump"]["object_name"], f"runs/{target_run_id}/database.dump.gcm")
            with self.assertRaises(importer.ImportErrorSafe):
                importer.recover_verify(staged, key_file, source_capture_run_id, target_run_id, "0" * 64)
            manifest_path = staged / "transport-manifest.json"
            manifest["import_contract"]["application_services_and_workers_may_start"] = True
            manifest_path.write_text(json.dumps(manifest, sort_keys=True, indent=2) + "\n")
            os.chmod(manifest_path, 0o600)
            mutated_sha = importer.sha256_file(manifest_path)[0]
            with self.assertRaises(importer.ImportErrorSafe):
                importer.recover_verify(staged, key_file, source_capture_run_id, target_run_id, mutated_sha)


if __name__ == "__main__":
    unittest.main()
