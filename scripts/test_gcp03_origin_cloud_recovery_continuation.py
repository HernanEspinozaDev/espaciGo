from __future__ import annotations

import hashlib
import json
from pathlib import Path
import unittest

from gcp03_origin_cloud_recovery import run_private_file_continuation
from gcp03_origin_private_files import PrivateFile
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
    def __init__(self, refs): self.refs, self.mutations, self.read_queries = refs, [], []
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
    def private_file_refs(self): return self.refs
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

    def test_plan_route_is_read_only(self):
        result = run_private_file_continuation(mode="continuation-plan", database=self.db,
            reader=self.reader, files=self.files)
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
            reader=self.reader, files=self.files, store=store,
            approved_plan_sha256=plan["plan_sha256"])
        self.assertTrue(result["phase_success"])
        self.assertEqual(len(store.created), 2)
        self.assertFalse(self.db.mutations)

    def test_61_rows_with_changed_checksum_are_rejected(self):
        rows = canonical_migration_rows()
        rows[20][1] = "0" * 64
        with self.assertRaisesRegex(Exception, "CONTINUATION_MIGRATION_CHECKSUM_DRIFT"):
            run_private_file_continuation(mode="continuation-plan", database=self.db,
                reader=FakeReader(rows), files=self.files)

    def test_60_migration_rows_are_rejected(self):
        with self.assertRaisesRegex(Exception, "CONTINUATION_MIGRATION_HISTORY_FAILED"):
            run_private_file_continuation(mode="continuation-plan", database=self.db,
                reader=FakeReader(canonical_migration_rows()[:-1]), files=self.files)

    def test_second_create_failure_is_reported_uncertain_without_retry(self):
        store = Store(fail_at=2)
        plan = run_private_file_continuation(mode="continuation-plan", database=self.db,
            reader=self.reader, files=self.files)
        result = run_private_file_continuation(mode="continuation-copy", database=self.db,
            reader=self.reader, files=self.files, store=store,
            approved_plan_sha256=plan["plan_sha256"])
        self.assertFalse(result["phase_success"])
        self.assertEqual(result["transaction_outcome"], "uncertain")
        self.assertEqual(result["private_file_copy_completed_count"], 1)
        self.assertFalse(result["retry"])
        self.assertEqual(len(store.created), 1)
        self.assertFalse(self.db.mutations)


if __name__ == "__main__":
    unittest.main()
