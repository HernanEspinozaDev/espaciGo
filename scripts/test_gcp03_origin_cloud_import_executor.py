import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import unittest

from cryptography.hazmat.primitives.ciphers.aead import AESGCM

import gcp03_origin_import as transport
from gcp03_origin_cloud_import_executor import (
    ARTIFACTS, EXPECTED_TARGET, ImportConfig, ObjectRef, OriginImportExecutor,
    ImportStopped, current_migration_inventory_sha256, validate_normalization_sql, validate_toc,
)


class MemoryStore:
    def __init__(self, objects):
        self.objects = objects

    def open(self, bucket, ref):
        generation, payload = self.objects[ref.name]
        return type("Download", (), {"generation": generation, "stream": io.BytesIO(payload)})()


class FakeDatabase:
    def __init__(self, *, empty=True):
        self.empty = empty
        self.calls = []

    def preflight(self):
        self.calls.append("preflight")
        return {"database": "espacigo_scratch", "empty": self.empty, "roles_ready": True,
                "extensions_ready": True, "application_schemas_empty": True,
                "postgres_version": True, "admin_role": True}

    def list_toc(self, feed_archive):
        self.calls.append("toc")
        feed_archive(io.BytesIO())
        return ["1; 0 0 TABLE public sample postgres"]

    def restore(self, feed_archive, allowlist):
        self.calls.append("restore")
        self.asserted_allowlist = allowlist
        feed_archive(io.BytesIO())

    def normalize_owners_and_acls(self, sql):
        self.calls.append("normalize")

    def sanitize(self):
        self.calls.append("sanitize")
        return {"valid_sessions": 0, "usable_action_tokens": 0, "pending_notices": 0,
                "identity_event_leases": 0, "runtime_role_login": False}


class ExecutorTests(unittest.TestCase):
    def _fixture(self, *, empty=True):
        td = tempfile.TemporaryDirectory()
        self.addCleanup(td.cleanup)
        root = Path(td.name)
        k_run = bytes(range(32))
        key_file = root / "k_run"
        key_file.write_text(k_run.hex())
        key_file.chmod(0o600)
        objects = {}
        refs = {}
        manifest_items = {}
        for index, artifact in enumerate(sorted(ARTIFACTS)):
            plain = (b"synthetic artifact " + artifact.encode())
            dek = bytes([index + 1]) * 32
            wrap_nonce = bytes([index + 1]) * 12
            aad = f"{transport.FORMAT}:g03o-1017:{artifact}:dek".encode()
            wrapped = AESGCM(k_run).encrypt(wrap_nonce, dek, aad)
            ciphertext = io.BytesIO()
            stats = transport.encrypt_chunks(io.BytesIO(plain), ciphertext, dek, "g03o-1017", artifact)
            payload = ciphertext.getvalue()
            name = f"runs/g03o-1017/{artifact}.gcm"
            objects[name] = (7, payload)
            refs[artifact] = {"object_name": name, "generation": "7",
                              "sha256": hashlib.sha256(payload).hexdigest(), "bytes": len(payload)}
            manifest_items[artifact] = {"object_name": name,
                "transport_cipher_sha256": hashlib.sha256(payload).hexdigest(),
                "transport_cipher_bytes": len(payload), "plain_sha256": stats["plain_sha256"],
                "plain_bytes": stats["plain_bytes"], "chunk_count": stats["chunks"],
                "wrap_nonce_hex": wrap_nonce.hex(), "wrapped_dek_hex": wrapped.hex()}
        normalization = b"ALTER TABLE public.sample OWNER TO espacigo_runtime;\n"
        allowlist = b"1; 0 0 TABLE public sample postgres\n"
        for name, payload in (("normalization.sql", normalization), ("toc.list", allowlist)):
            objects[f"runs/g03o-1017/{name}"] = (7, payload)
        manifest = {"format": transport.FORMAT, "source_capture_run_id": "gcp03-merge-review-20261017",
            "target_run_id": "g03o-1017", "source_sealed_sha256": "80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262",
            "salt_hex": (b"s" * 32).hex(), "artifacts": manifest_items,
            "migration_inventory_sha256": current_migration_inventory_sha256(),
            "import_contract": {"format": "espacigo.gcp03.origin-import-contract.v1",
                "target_database": "espacigo_scratch", "requires_empty_target": True,
                "restore_attempts": 1, "automatic_retries": 0,
                "application_services_and_workers_may_start": False,
                "sanitize_copy_before_verification": True, "source_mutation_allowed": False,
                "cloud_upload_performed": False}}
        manifest_bytes = json.dumps(manifest).encode()
        objects["runs/g03o-1017/transport-manifest.json"] = (7, manifest_bytes)
        digest = lambda b: hashlib.sha256(b).hexdigest()
        evidence = {"format": "espacigo.gcp03.origin-upload-receipt.v1",
            "source_capture_run_id": "gcp03-merge-review-20261017", "target_run_id": "g03o-1017",
            "source_sealed_sha256": "80a4e1e0002cf12f8e750ddaac7ceb796726b57a36405aa2b0b8e7852e928262",
            "bootstrap_report_sha256": "1" * 64, "observer_report_sha256": "2" * 64,
            "normalization_sql_sha256": digest(normalization), "artifacts": refs,
            "transport_manifest": {"object_name": "runs/g03o-1017/transport-manifest.json", "generation": "7",
                "sha256": digest(manifest_bytes), "bytes": len(manifest_bytes)},
            "toc_allowlist": {"object_name": "runs/g03o-1017/toc.list", "generation": "7",
                "sha256": digest(allowlist), "bytes": len(allowlist)},
            "normalization_sql": {"object_name": "runs/g03o-1017/normalization.sql", "generation": "7",
                "sha256": digest(normalization), "bytes": len(normalization)}}
        receipt = json.dumps(evidence).encode()
        objects["runs/g03o-1017/receipt.json"] = (7, receipt)
        ref = ObjectRef("runs/g03o-1017/receipt.json", 7, digest(receipt), len(receipt))
        config = ImportConfig("espacigo-gcp03-origin-g03o-1017", ref, digest(receipt), digest(manifest_bytes), "1" * 64,
                              "2" * 64, digest(normalization), EXPECTED_TARGET, key_file)
        db = FakeDatabase(empty=empty)
        return config, MemoryStore(objects), db

    def test_one_shot_import_order_and_sanitation(self):
        config, store, db = self._fixture()
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["status"], "imported_and_sanitized")
        self.assertEqual(db.calls, ["preflight", "toc", "restore", "normalize", "sanitize"])
        self.assertFalse(result["services_or_workers_started"])

    def test_nonempty_target_stops_before_restore(self):
        config, store, db = self._fixture(empty=False)
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "TARGET_NOT_PRISTINE")
        self.assertEqual(db.calls, ["preflight"])
        self.assertEqual(result["transaction_outcome"], "not_started")

    def test_receipt_hash_failure_never_connects_to_database(self):
        config, store, db = self._fixture()
        config = ImportConfig(config.bucket, config.receipt, "0" * 64, config.expected_manifest_sha256,
            config.expected_bootstrap_sha256, config.expected_observer_sha256,
            config.expected_normalization_sha256, config.confirmation, config.transport_key_file)
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "RECEIPT_HASH_MISMATCH")
        self.assertEqual(db.calls, [])

    def test_generation_mismatch_stops_before_database(self):
        config, store, db = self._fixture()
        store.objects[config.receipt.name] = (8, store.objects[config.receipt.name][1])
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "OBJECT_GENERATION_MISMATCH")
        self.assertEqual(db.calls, [])

    def test_restore_and_sanitation_failure_is_uncertain_and_never_retryable(self):
        config, store, db = self._fixture()
        db.sanitize = lambda: {"valid_sessions": 1, "usable_action_tokens": 0,
            "pending_notices": 0, "identity_event_leases": 0, "runtime_role_login": False}
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "SANITATION_POSTCONDITIONS_FAILED")
        self.assertEqual(result["transaction_outcome"], "uncertain")
        self.assertFalse(result["retry"])

    def test_toc_must_be_subset_and_exclude_extension_entries(self):
        allowlist = b"1; 0 0 TABLE public sample postgres\n"
        self.assertEqual(len(validate_toc("1; 0 0 TABLE public sample postgres\n", allowlist)), 1)
        with self.assertRaises(ImportStopped):
            validate_toc("1; 0 0 TABLE public other postgres\n", allowlist)
        with self.assertRaises(ImportStopped):
            validate_toc("1; 0 0 EXTENSION - postgis postgres\n", b"1; 0 0 EXTENSION - postgis postgres\n")

    def test_normalization_sql_is_hash_bound_and_rejects_role_credentials(self):
        safe = b"ALTER TABLE public.sample OWNER TO espacigo_runtime;"
        validate_normalization_sql(safe, hashlib.sha256(safe).hexdigest())
        with self.assertRaises(ImportStopped):
            validate_normalization_sql(b"ALTER ROLE espacigo_runtime PASSWORD 'secret';", hashlib.sha256(b"ALTER ROLE espacigo_runtime PASSWORD 'secret';").hexdigest())


if __name__ == "__main__":
    unittest.main()
