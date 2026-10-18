from __future__ import annotations

import hashlib
import json
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import gcp03_origin_cloud_recovery as cloud_recovery
from gcp03_origin_cloud_recovery import run_private_file_continuation
from gcp03_origin_private_files import PrivateFile
from gcp03_origin_private_file_refs import PrivateFileReferenceQueryError
from gcp03_origin_recovery_preflight import CANONICAL_MIGRATION_HISTORY_SHA256_V2


ROOT = Path(__file__).resolve().parents[1]


def canonical_migration_rows():
    rows = []
    for version in range(1, 62):
        path = next((ROOT / "db/migrations").glob(f"V{version:06d}__*.sql"))
        rows.append([str(version), hashlib.sha256(path.read_bytes()).hexdigest()])
    return rows


class FakeReader:
    def __init__(self, rows=None): self.rows = rows or canonical_migration_rows()
    def query(self, sql): return json.dumps(self.rows)


class FakeDatabase:
    def __init__(self, refs, query_failure=None):
        self.refs, self.query_failure, self.mutations, self.read_queries = refs, query_failure, [], []
        self.passfile = None
    def _psql(self, sql, read_only=False):
        self.read_queries.append((sql, read_only))
        if "session_user_is_postgres" in sql:
            return json.dumps({"session_user_is_postgres": True,
                "current_user_is_runtime": True, "runtime_no_login": True,
                "postgres_runtime_set_only": True})
        return json.dumps({"session_user": "postgres", "current_user": "postgres"})
    def verify_normalized(self):
        return {"relations_owned_by_runtime": True, "routines_owned_by_runtime": True,
            "text_search_owned_by_runtime": True, "sequences_parent_owner_consistent": True,
            "observer_write_privileges_absent": True, "runtime_public_create_absent": True}
    def continuation_sanitation_counts(self):
        return {"valid_sessions": 0, "usable_action_tokens": 0, "pending_notices": 0,
            "identity_event_leases": 0, "runtime_role_login": False}
    def private_file_refs(self):
        if self.query_failure: raise self.query_failure
        return self.refs
    def close(self): pass
    def normalize_owners_and_acls(self, *_): raise AssertionError("continuation must not normalize")
    def sanitize(self, *_): raise AssertionError("continuation must not sanitize")


def private_files():
    result = []
    for file_id, payload in (("00000000-0000-4000-8000-000000000001", b"first"),
                             ("00000000-0000-4000-8000-000000000002", b"second")):
        result.append(PrivateFile(f"runs/g03o-1017/{file_id}.blob", payload,
            hashlib.sha256(payload).hexdigest()))
    return result


class Store:
    def __init__(self, fail_at=None): self.created = []; self.fail_at = fail_at
    def create(self, object_name, data, sha256):
        if self.fail_at == len(self.created) + 1: raise OSError("synthetic create failure")
        self.created.append(object_name)


class CloudRecoveryContinuationTests(unittest.TestCase):
    def setUp(self):
        self.files = private_files()
        self.refs = {item.object_name.rsplit("/", 1)[-1][:-5] for item in self.files}
        self.db = FakeDatabase(self.refs)
        self.reader = FakeReader()
        self.assertEqual(CANONICAL_MIGRATION_HISTORY_SHA256_V2,
            "dd2a4f3cf716137549b584f35bec5900df78c3e02a86354b4bad7ef1a90515ee")

    def run_entrypoint(self, database):
        cfg = SimpleNamespace(expected_receipt_sha256="a" * 64,
            expected_manifest_sha256="b" * 64, expected_normalization_sha256="c" * 64,
            private_files_bucket="synthetic-private-bucket")
        norm_ref = SimpleNamespace(generation=1)
        auth_inputs = ({}, {}, norm_ref, {}, {}, self.files, b"synthetic-normalization")
        with patch.object(cloud_recovery, "config", return_value=(cfg, object(), "continuation-copy")), \
             patch.object(cloud_recovery, "GCSObjectStore", return_value=object()), \
             patch.object(cloud_recovery, "wait_for_proxy"), \
             patch.object(cloud_recovery, "CloudRecoveryDatabase", return_value=database), \
             patch.object(cloud_recovery, "authenticated_inputs", return_value=auth_inputs), \
             patch.object(cloud_recovery, "CatalogReader", return_value=self.reader), \
             patch.object(cloud_recovery, "GCSPrivateFileStore") as writer, \
             patch.object(cloud_recovery, "shutdown_proxy"):
            report = cloud_recovery.run({
                "DATABASE_PASSWORD_FILE": "/synthetic/password",
                "GCP03_ORIGIN_RECOVERY_APPROVED_PLAN_SHA256": "d" * 64,
            })
        return report, writer

    def test_plan_route_is_read_only(self):
        def forbidden_writer(): raise AssertionError("plan mode must not construct a writer")
        result = run_private_file_continuation(mode="continuation-plan", database=self.db,
            reader=self.reader, files=self.files, store_factory=forbidden_writer)
        self.assertEqual(result["status"], "plan_ready")
        self.assertFalse(result["execution_performed"])
        self.assertFalse(self.db.mutations)
        role_sql = next(sql for sql, _ in self.db.read_queries if "postgres_runtime_set_only" in sql)
        self.assertIn("count(*)=1 AND bool_and", role_sql)
        self.assertIn("NOT m.inherit_option", role_sql)

    def test_copy_route_uses_approved_plan_and_create_only_store(self):
        store = Store()
        plan = run_private_file_continuation(mode="continuation-plan", database=self.db,
            reader=self.reader, files=self.files)
        result = run_private_file_continuation(mode="continuation-copy", database=self.db,
            reader=self.reader, files=self.files, store_factory=lambda: store,
            approved_plan_sha256=plan["plan_sha256"])
        self.assertTrue(result["phase_success"])
        self.assertEqual(len(store.created), 2)
        self.assertFalse(self.db.mutations)

    def test_unapproved_copy_plan_never_constructs_writer(self):
        writer_calls = []
        result = run_private_file_continuation(mode="continuation-copy", database=self.db,
            reader=self.reader, files=self.files,
            store_factory=lambda: writer_calls.append("constructed"),
            approved_plan_sha256="0" * 64)
        self.assertEqual(result["error_code"], "CURRENT_PLAN_DIFFERS_FROM_APPROVED_PLAN")
        self.assertEqual(result["transaction_outcome"], "not_started")
        self.assertEqual(writer_calls, [])

    def test_61_rows_with_changed_checksum_are_rejected(self):
        rows = canonical_migration_rows()
        rows[20][1] = "0" * 64
        with self.assertRaisesRegex(Exception, "CONTINUATION_MIGRATION_CHECKSUM_DRIFT"):
            run_private_file_continuation(mode="continuation-plan", database=self.db,
                reader=FakeReader(rows), files=self.files)

    def test_60_migration_rows_are_rejected(self):
        writer_calls = []
        with self.assertRaisesRegex(Exception, "CONTINUATION_MIGRATION_HISTORY_FAILED"):
            run_private_file_continuation(mode="continuation-plan", database=self.db,
                reader=FakeReader(canonical_migration_rows()[:-1]), files=self.files,
                store_factory=lambda: writer_calls.append("constructed"))
        self.assertEqual(writer_calls, [])

    def test_second_create_failure_is_reported_uncertain_without_retry(self):
        store = Store(fail_at=2)
        plan = run_private_file_continuation(mode="continuation-plan", database=self.db,
            reader=self.reader, files=self.files)
        result = run_private_file_continuation(mode="continuation-copy", database=self.db,
            reader=self.reader, files=self.files, store_factory=lambda: store,
            approved_plan_sha256=plan["plan_sha256"])
        self.assertFalse(result["phase_success"])
        self.assertEqual(result["transaction_outcome"], "uncertain")
        self.assertEqual(result["private_file_copy_completed_count"], 1)
        self.assertFalse(result["retry"])
        self.assertEqual(len(store.created), 1)
        self.assertFalse(self.db.mutations)

    def test_private_reference_query_failure_preserves_safe_sqlstate_and_never_constructs_writer(self):
        writer_calls = []
        db = FakeDatabase(self.refs, PrivateFileReferenceQueryError("42703"))
        result = run_private_file_continuation(mode="continuation-copy", database=db,
            reader=self.reader, files=self.files,
            store_factory=lambda: writer_calls.append("constructed"))
        self.assertEqual(result["error_code"], "PRIVATE_FILE_REFERENCE_QUERY_FAILED")
        self.assertEqual(result["sqlstate"], "42703")
        self.assertEqual(result["stage"], "private_file_validation")
        self.assertEqual(result["transaction_outcome"], "not_started")
        self.assertFalse(result["retry"])
        self.assertEqual(writer_calls, [])
        self.assertFalse(self.db.mutations)

    def test_entrypoint_query_error_is_specific_and_does_not_construct_writer(self):
        db = FakeDatabase(self.refs, PrivateFileReferenceQueryError("42703"))
        report, writer = self.run_entrypoint(db)
        self.assertEqual(report["error_code"], "PRIVATE_FILE_REFERENCE_QUERY_FAILED")
        self.assertEqual(report["sqlstate"], "42703")
        self.assertEqual(report["stage"], "private_file_validation")
        self.assertFalse(report["retry"])
        writer.assert_not_called()

    def test_entrypoint_set_mismatch_has_only_counts_and_hashes_before_writer(self):
        db = FakeDatabase(set())
        report, writer = self.run_entrypoint(db)
        self.assertEqual(report["error_code"], "PRIVATE_FILE_REFERENCE_SET_MISMATCH")
        self.assertEqual(report["database_reference_count"], 0)
        self.assertEqual(report["private_evidence_file_count"], len(self.files))
        self.assertRegex(report["database_reference_set_sha256"], r"^[0-9a-f]{64}$")
        self.assertRegex(report["private_evidence_set_sha256"], r"^[0-9a-f]{64}$")
        safe_report = json.dumps(report)
        self.assertNotIn("00000000-0000-4000-8000-000000000001", safe_report)
        self.assertNotIn("00000000-0000-4000-8000-000000000002", safe_report)
        writer.assert_not_called()

    def test_private_reference_set_mismatch_returns_safe_counts_and_hashes_before_writer(self):
        writer_calls = []
        db = FakeDatabase(set())
        result = run_private_file_continuation(mode="continuation-copy", database=db,
            reader=self.reader, files=self.files,
            store_factory=lambda: writer_calls.append("constructed"))
        self.assertEqual(result["error_code"], "PRIVATE_FILE_REFERENCE_SET_MISMATCH")
        self.assertEqual(result["database_reference_count"], 0)
        self.assertEqual(result["private_evidence_file_count"], len(self.files))
        self.assertRegex(result["database_reference_set_sha256"], r"^[0-9a-f]{64}$")
        self.assertRegex(result["private_evidence_set_sha256"], r"^[0-9a-f]{64}$")
        self.assertNotIn("00000000-0000-4000-8000-000000000001", json.dumps(result))
        self.assertNotIn("00000000-0000-4000-8000-000000000002", json.dumps(result))
        self.assertEqual(writer_calls, [])
        self.assertFalse(db.mutations)


if __name__ == "__main__":
    unittest.main()
