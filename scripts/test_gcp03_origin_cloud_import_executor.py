import hashlib
import io
import json
import os
from pathlib import Path
import tempfile
import tarfile
import unittest

from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from unittest import mock

import gcp03_origin_import as transport
import gcp03_origin_cloud_import_executor as executor_module
from gcp03_origin_cloud_import_executor import (
    ARTIFACTS, EXPECTED_TARGET, ImportConfig, ObjectRef, OriginImportExecutor,
    ImportStopped, current_migration_inventory_sha256, validate_normalization_sql, validate_toc,
    wait_for_proxy, shutdown_proxy,
)


PRIVATE_FILE_ID = "11111111-1111-4111-8111-111111111111"


def synthetic_private_tar(payload=b"synthetic private file"):
    manifest = json.dumps({"format": "espacigo.gcp03.local-origin-capture.v1.files", "files": [{
        "root": "private-evidence", "relative_path": f"{PRIVATE_FILE_ID}.png", "mode_octal": "0600",
        "bytes": len(payload), "sha256": hashlib.sha256(payload).hexdigest()}]},
        sort_keys=True, separators=(",", ":")).encode()
    output = io.BytesIO()
    with tarfile.open(fileobj=output, mode="w") as archive:
        root = tarfile.TarInfo("private-evidence")
        root.type, root.mode = tarfile.DIRTYPE, 0o700
        archive.addfile(root)
        other_root = tarfile.TarInfo("privacy-replay")
        other_root.type, other_root.mode = tarfile.DIRTYPE, 0o700
        archive.addfile(other_root)
        item = tarfile.TarInfo(f"private-evidence/{PRIVATE_FILE_ID}.png")
        item.size, item.mode = len(payload), 0o600
        archive.addfile(item, io.BytesIO(payload))
        meta = tarfile.TarInfo("capture-file-manifest.json")
        meta.size, meta.mode = len(manifest), 0o600
        archive.addfile(meta, io.BytesIO(manifest))
    return output.getvalue()


class MemoryPrivateStore:
    def __init__(self):
        self.objects = {}

    def create(self, name, data, sha256):
        if name in self.objects or hashlib.sha256(data).hexdigest() != sha256:
            raise AssertionError("unexpected private object write")
        self.objects[name] = data


class FailingPrivateStore:
    def create(self, name, data, sha256):
        raise OSError("synthetic object write failure")


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

    def private_file_refs(self):
        self.calls.append("private_file_refs")
        return {PRIVATE_FILE_ID}


class ExecutorTests(unittest.TestCase):
    def _fixture(self, *, empty=True, private_tar=None):
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
            plain = (private_tar if private_tar is not None else synthetic_private_tar()) if artifact == "private-files.tar" else (b"synthetic artifact " + artifact.encode())
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
        config = ImportConfig("espacigo-gcp03-origin-g03o-1017", "espacigo-gcp03-private-g03o-1017", ref, digest(receipt), digest(manifest_bytes), "1" * 64,
                              "2" * 64, digest(normalization), EXPECTED_TARGET, key_file)
        db = FakeDatabase(empty=empty)
        return config, MemoryStore(objects), db, MemoryPrivateStore()

    def test_one_shot_import_order_and_sanitation(self):
        config, store, db, private_store = self._fixture()
        result = OriginImportExecutor(config, store, db, private_store).run()
        self.assertEqual(result["status"], "imported_and_sanitized")
        self.assertEqual(result["private_files_copied"], 1)
        self.assertEqual(len(private_store.objects), 1)
        self.assertEqual(result["private_file_references"], 1)
        self.assertEqual(db.calls, ["preflight", "toc", "restore", "normalize", "sanitize", "private_file_refs"])

    def test_authenticated_but_malformed_private_archive_stops_before_database_mutation(self):
        config, store, db, private_store = self._fixture(private_tar=b"authenticated but not a tar archive")
        result = OriginImportExecutor(config, store, db, private_store).run()
        self.assertEqual(result["error_code"], "PRIVATE_FILES_ARCHIVE_INVALID")
        self.assertEqual(result["transaction_outcome"], "not_started")
        self.assertEqual(db.calls, [])
        self.assertEqual(private_store.objects, {})

    def test_database_file_reference_mismatch_stops_before_any_file_object_write(self):
        config, store, db, private_store = self._fixture()
        db.private_file_refs = lambda: set()
        result = OriginImportExecutor(config, store, db, private_store).run()
        self.assertEqual(result["error_code"], "PRIVATE_FILE_REFERENCE_MISMATCH")
        self.assertEqual(result["transaction_outcome"], "uncertain")
        self.assertFalse(result["retry"])
        self.assertEqual(private_store.objects, {})
        self.assertEqual(result["status"], "stopped")

    def test_nonempty_target_stops_before_restore(self):
        config, store, db, private_store = self._fixture(empty=False)
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "TARGET_NOT_PRISTINE")
        self.assertEqual(db.calls, ["preflight"])
        self.assertEqual(result["transaction_outcome"], "not_started")

    def test_receipt_hash_failure_never_connects_to_database(self):
        config, store, db, private_store = self._fixture()
        config = ImportConfig(config.bucket, config.private_files_bucket, config.receipt, "0" * 64, config.expected_manifest_sha256,
            config.expected_bootstrap_sha256, config.expected_observer_sha256,
            config.expected_normalization_sha256, config.confirmation, config.transport_key_file)
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "RECEIPT_HASH_MISMATCH")
        self.assertEqual(db.calls, [])

    def test_generation_mismatch_stops_before_database(self):
        config, store, db, private_store = self._fixture()
        store.objects[config.receipt.name] = (8, store.objects[config.receipt.name][1])
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "OBJECT_GENERATION_MISMATCH")
        self.assertEqual(db.calls, [])

    def test_restore_and_sanitation_failure_is_uncertain_and_never_retryable(self):
        config, store, db, private_store = self._fixture()
        db.sanitize = lambda: {"valid_sessions": 1, "usable_action_tokens": 0,
            "pending_notices": 0, "identity_event_leases": 0, "runtime_role_login": False}
        result = OriginImportExecutor(config, store, db).run()
        self.assertEqual(result["error_code"], "SANITATION_POSTCONDITIONS_FAILED")
        self.assertEqual(result["transaction_outcome"], "uncertain")
        self.assertFalse(result["retry"])

    def test_private_file_write_failure_after_import_is_uncertain_and_never_retryable(self):
        config, store, db, _ = self._fixture()
        result = OriginImportExecutor(config, store, db, FailingPrivateStore()).run()
        self.assertEqual(result["status"], "stopped")
        self.assertEqual(result["error_code"], "UNEXPECTED_EXECUTOR_FAILURE")
        self.assertEqual(result["transaction_outcome"], "uncertain")
        self.assertFalse(result["retry"])
        self.assertEqual(db.calls, ["preflight", "toc", "restore", "normalize", "sanitize", "private_file_refs"])

    def test_wait_for_proxy_succeeds_when_loopback_listener_is_ready(self):
        import socket
        with socket.socket() as server:
            server.bind(("127.0.0.1", 0))
            server.listen(1)
            wait_for_proxy(port=server.getsockname()[1], timeout_seconds=1)

    def test_shutdown_proxy_posts_quitquitquit_and_retries_within_bound(self):
        import urllib.error
        calls = []
        ticks = iter([0.0, 0.1, 0.2, 0.3])
        class Response:
            status = 200
            def __enter__(self): return self
            def __exit__(self, *args): pass
        def opener(request, timeout):
            calls.append((request.full_url, request.method, timeout))
            if len(calls) == 1:
                raise urllib.error.URLError("not ready")
            return Response()
        self.assertTrue(shutdown_proxy(open_url=opener, clock=lambda: next(ticks), sleep=lambda _: None))
        self.assertEqual(calls, [("http://127.0.0.1:9091/quitquitquit", "POST", 1)] * 2)

    def test_main_shuts_down_proxy_after_success_failure_and_preinitialization(self):
        class Runner:
            def __init__(self): self.closed = False
            def close(self): self.closed = True
        runner = Runner()
        with mock.patch.object(executor_module, "config_from_env", return_value=object()), \
             mock.patch.object(executor_module, "GCSObjectStore", return_value=object()), \
             mock.patch.object(executor_module, "PsqlPgRestoreRunner", return_value=runner), \
             mock.patch.object(executor_module, "OriginImportExecutor") as importer, \
             mock.patch.object(executor_module, "shutdown_proxy", return_value=True) as shutdown:
            importer.return_value.run.return_value = {"status": "imported_and_sanitized"}
            executor_module.main()
            self.assertTrue(runner.closed)
            shutdown.assert_called_once()

        for config_error in (False, True):
            runner = Runner()
            with mock.patch.object(executor_module, "config_from_env", side_effect=ImportStopped("bad config", "config") if config_error else None), \
                 mock.patch.object(executor_module, "GCSObjectStore", return_value=object()), \
                 mock.patch.object(executor_module, "PsqlPgRestoreRunner", return_value=runner), \
                 mock.patch.object(executor_module, "OriginImportExecutor") as importer, \
                 mock.patch.object(executor_module, "shutdown_proxy", return_value=True) as shutdown:
                importer.return_value.run.return_value = {"status": "stopped"}
                with self.assertRaises(SystemExit):
                    executor_module.main()
                shutdown.assert_called_once()
                self.assertEqual(runner.closed, not config_error)

    def test_toc_must_be_subset_and_exclude_extension_entries(self):
        allowlist = b"1; 0 0 TABLE public sample postgres\n"
        self.assertEqual(len(validate_toc("1; 0 0 TABLE public sample postgres\n", allowlist)), 1)
        with self.assertRaises(ImportStopped):
            validate_toc("1; 0 0 TABLE public other postgres\n", allowlist)
        with self.assertRaises(ImportStopped):
            validate_toc("1; 0 0 EXTENSION - postgis postgres\n", b"1; 0 0 EXTENSION - postgis postgres\n")

    def test_normalization_sql_is_hash_bound_and_rejects_role_credentials(self):
        safe = (b"ALTER TABLE public.sample OWNER TO espacigo_runtime;\n"
                b"ALTER FUNCTION public.sample(integer) OWNER TO espacigo_runtime;\n"
                b"GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE public.sample TO espacigo_runtime;\n"
                b"GRANT SELECT ON TABLE public.schema_migrations TO espacigo_gcp03_observer;\n"
                b"ALTER DEFAULT PRIVILEGES FOR ROLE source_role IN SCHEMA public GRANT SELECT ON TABLES TO espacigo_runtime;\n")
        validate_normalization_sql(safe, hashlib.sha256(safe).hexdigest())
        with self.assertRaises(ImportStopped):
            validate_normalization_sql(b"ALTER ROLE espacigo_runtime PASSWORD 'secret';", hashlib.sha256(b"ALTER ROLE espacigo_runtime PASSWORD 'secret';").hexdigest())
        dml = b"INSERT INTO public.sample VALUES (1, 'x');"
        with self.assertRaises(ImportStopped):
            validate_normalization_sql(dml, hashlib.sha256(dml).hexdigest())


if __name__ == "__main__":
    unittest.main()
