from __future__ import annotations

import hashlib
import unittest

from gcp03_origin_recovery_plan import (
    REQUIRED_OWNER_POSTCHECKS,
    REQUIRED_SANITATION_COUNTS,
    NormalizationRecoveryPlan,
    RecoveryStop,
    RecoveryPlanError,
    build_normalization_recovery_plan,
    execute_reviewed_recovery,
    statement_id,
)
from gcp03_origin_private_files import PrivateFile


NORMALIZATION = b"""GRANT CREATE ON SCHEMA public TO espacigo_runtime;
ALTER TABLE public.parent_probe OWNER TO espacigo_runtime;
ALTER SEQUENCE public.standalone_probe_seq OWNER TO espacigo_runtime;
ALTER SEQUENCE public.parent_probe_id_seq OWNER TO espacigo_runtime;
GRANT SELECT (state) ON TABLE public.notice_probe TO espacigo_gcp03_observer;
REVOKE CREATE ON SCHEMA public FROM espacigo_runtime;
"""


def facts_for(*, table_owner="postgres", standalone_owner="postgres", acl="missing", create_grant="missing", create_revoke="satisfied"):
    statements = [x.strip() for x in NORMALIZATION.decode().split(";") if x.strip()]
    values = (create_grant, table_owner, standalone_owner, standalone_owner, acl, create_revoke)
    return {statement_id(sql): value for sql, value in zip(statements, values)}


class RecoveryPlanTests(unittest.TestCase):
    def test_initial_restored_state_applies_only_pending_statements(self):
        facts = facts_for(table_owner="postgres", standalone_owner="postgres", acl="missing")
        plan = build_normalization_recovery_plan(NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts)
        self.assertEqual(plan.summary["statement_count"], 6)
        self.assertEqual(plan.summary["pending_statement_count"], 6)
        self.assertEqual(plan.summary["satisfied_statement_count"], 0)
        self.assertEqual(plan.summary["execution_performed"], False)
        self.assertEqual(plan.summary["contains_sql_text"], False)
        self.assertNotIn(b"ALTER TABLE", str(plan.summary).encode())
        self.assertIn(b"ALTER TABLE public.parent_probe OWNER TO espacigo_runtime", plan.pending_sql)
        self.assertIn(b"ALTER SEQUENCE public.standalone_probe_seq OWNER TO espacigo_runtime", plan.pending_sql)

    def test_owned_sequence_statement_is_skipped_when_parent_transfer_cascades(self):
        ids = [statement_id(x.strip()) for x in NORMALIZATION.decode().split(";") if x.strip()]
        facts = facts_for()
        # The parent probe sequence is owned by parent_probe; the other sequence is standalone.
        plan = build_normalization_recovery_plan(
            NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts,
            owned_sequence_parent_statement={ids[3]: ids[1]},
        )
        self.assertIn(b"ALTER TABLE public.parent_probe OWNER", plan.pending_sql)
        self.assertIn(b"ALTER SEQUENCE public.standalone_probe_seq OWNER", plan.pending_sql)
        self.assertNotIn(b"ALTER SEQUENCE public.parent_probe_id_seq OWNER", plan.pending_sql)

    def test_owned_sequence_owner_drift_blocks_plan(self):
        ids = [statement_id(x.strip()) for x in NORMALIZATION.decode().split(";") if x.strip()]
        facts = facts_for()
        facts[ids[1]] = "espacigo_runtime"
        with self.assertRaisesRegex(RecoveryPlanError, "inconsistent"):
            build_normalization_recovery_plan(
                NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts,
                owned_sequence_parent_statement={ids[3]: ids[1]},
            )

    def test_both_mixed_owned_sequence_owner_states_block_plan(self):
        ids = [statement_id(x.strip()) for x in NORMALIZATION.decode().split(";") if x.strip()]
        for parent, sequence in (("postgres", "espacigo_runtime"),
                                 ("espacigo_runtime", "postgres")):
            with self.subTest(parent=parent, sequence=sequence):
                facts = facts_for(table_owner=parent)
                facts[ids[3]] = sequence
                with self.assertRaisesRegex(RecoveryPlanError, "inconsistent"):
                    build_normalization_recovery_plan(
                        NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts,
                        owned_sequence_parent_statement={ids[3]: ids[1]},
                    )

    def test_already_normalized_state_skips_owner_transfers(self):
        facts = facts_for(table_owner="espacigo_runtime", standalone_owner="espacigo_runtime", acl="satisfied")
        plan = build_normalization_recovery_plan(NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts)
        self.assertEqual(plan.summary["pending_statement_count"], 0)
        self.assertNotIn(b"ALTER TABLE", plan.pending_sql)
        self.assertNotIn(b"ALTER SEQUENCE", plan.pending_sql)

    def test_unexpected_owner_blocks_plan_before_any_execution_sql_exists(self):
        facts = facts_for(table_owner="unexpected_owner")
        with self.assertRaisesRegex(RecoveryPlanError, "unexpected or contradictory"):
            build_normalization_recovery_plan(NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts)

    def test_missing_or_surplus_catalog_facts_fail_closed(self):
        facts = facts_for()
        missing = dict(facts)
        missing.pop(next(iter(missing)))
        with self.assertRaisesRegex(RecoveryPlanError, "does not match"):
            build_normalization_recovery_plan(NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), missing)
        surplus = {**facts, "0" * 64: "postgres"}
        with self.assertRaisesRegex(RecoveryPlanError, "does not match"):
            build_normalization_recovery_plan(NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), surplus)

    def test_hash_mismatch_and_contradictory_acl_block_plan(self):
        with self.assertRaisesRegex(RecoveryPlanError, "hash mismatch"):
            build_normalization_recovery_plan(NORMALIZATION, "0" * 64, facts_for())
        with self.assertRaisesRegex(RecoveryPlanError, "unexpected or contradictory"):
            build_normalization_recovery_plan(
                NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(),
                facts_for(acl="contradictory"))

    def test_plan_hash_is_bound_to_catalog_state_and_input(self):
        source_hash = hashlib.sha256(NORMALIZATION).hexdigest()
        first = build_normalization_recovery_plan(NORMALIZATION, source_hash, facts_for(table_owner="postgres"))
        second = build_normalization_recovery_plan(NORMALIZATION, source_hash, facts_for(table_owner="espacigo_runtime"))
        self.assertNotEqual(first.summary["plan_sha256"], second.summary["plan_sha256"])
        self.assertEqual(first.summary["normalization_sql_sha256"], source_hash)


class FakeDatabase:
    def __init__(self, *, owner_ok=True, sanitation_ok=True, refs=None, failure=None):
        self.calls = []
        self.owner_ok = owner_ok
        self.sanitation_ok = sanitation_ok
        self.refs = refs if refs is not None else set()
        self.failure = failure

    def normalize_owners_and_acls(self, sql):
        self.calls.append("normalize")
        if self.failure == "normalize_rollback":
            raise RecoveryStop("OWNER_ACL_FAILED", "owner_acl_normalization", "rolled_back")
        if self.failure == "normalize_uncertain":
            raise RuntimeError("raw database error")

    def verify_normalized(self):
        self.calls.append("owner_check")
        return {key: self.owner_ok for key in REQUIRED_OWNER_POSTCHECKS}

    def sanitize(self):
        self.calls.append("sanitize")
        if self.failure == "sanitize":
            raise RuntimeError("raw sanitation error")
        return {**{key: 0 for key in REQUIRED_SANITATION_COUNTS},
                "runtime_role_login": False if self.sanitation_ok else True}

    def private_file_refs(self):
        self.calls.append("refs")
        return self.refs


class FakeStore:
    def __init__(self, fail=False):
        self.calls = []
        self.fail = fail

    def create(self, name, data, sha256):
        self.calls.append(name)
        if self.fail:
            raise RuntimeError("raw store error")


class RecoveryExecutionTests(unittest.TestCase):
    def setUp(self):
        facts = facts_for()
        self.plan = build_normalization_recovery_plan(
            NORMALIZATION, hashlib.sha256(NORMALIZATION).hexdigest(), facts)
        self.approved = self.plan.summary["plan_sha256"]
        self.files = []

    def run_flow(self, db=None, store=None, approved=None):
        db = db or FakeDatabase()
        store = store or FakeStore()
        report = execute_reviewed_recovery(
            plan=self.plan, approved_plan_sha256=approved or self.approved,
            database=db, private_files=self.files, private_file_store=store,
            run_id="g03o-1017")
        return report, db, store

    def test_plan_hash_mismatch_stops_before_any_mutation(self):
        report, db, store = self.run_flow(approved="0" * 64)
        self.assertEqual(report["error_code"], "APPROVED_PLAN_HASH_MISMATCH")
        self.assertFalse(report["report_complete"])
        self.assertEqual(db.calls, [])
        self.assertEqual(store.calls, [])

    def test_mutated_pending_sql_or_review_summary_stops_before_mutation(self):
        original_sql = self.plan.pending_sql
        object.__setattr__(self.plan, "pending_sql", original_sql.replace(b"parent_probe", b"other_probe", 1))
        report, db, store = self.run_flow()
        self.assertEqual(report["error_code"], "APPROVED_PLAN_HASH_MISMATCH")
        self.assertEqual(db.calls, [])
        self.assertEqual(store.calls, [])
        object.__setattr__(self.plan, "pending_sql", original_sql)
        self.plan.summary["catalog_facts_sha256"] = "f" * 64
        report, db, store = self.run_flow()
        self.assertEqual(report["error_code"], "APPROVED_PLAN_HASH_MISMATCH")
        self.assertEqual(db.calls, [])

    def test_order_is_normalize_postcheck_sanitize_refs_then_copy(self):
        db = FakeDatabase()
        report, db, store = self.run_flow(db=db)
        self.assertTrue(report["phase_success"])
        self.assertTrue(report["report_complete"])
        self.assertEqual(db.calls, ["normalize", "owner_check", "sanitize", "refs"])
        self.assertEqual(store.calls, [])

    def test_failed_or_uncertain_normalization_never_advances(self):
        for failure, expected in (("normalize_rollback", "rolled_back"),
                                  ("normalize_uncertain", "uncertain")):
            with self.subTest(failure=failure):
                report, db, store = self.run_flow(db=FakeDatabase(failure=failure))
                self.assertEqual(report["transaction_outcome"], expected)
                self.assertFalse(report["retry"])
                self.assertEqual(db.calls, ["normalize"])
                self.assertEqual(store.calls, [])

    def test_owner_and_sanitation_postchecks_gate_later_phases(self):
        report, db, store = self.run_flow(db=FakeDatabase(owner_ok=False))
        self.assertEqual(report["error_code"], "OWNER_ACL_POSTCHECK_FAILED")
        self.assertEqual(db.calls, ["normalize", "owner_check"])
        self.assertEqual(store.calls, [])
        report, db, store = self.run_flow(db=FakeDatabase(sanitation_ok=False))
        self.assertEqual(report["error_code"], "SANITATION_POSTCONDITIONS_FAILED")
        self.assertEqual(db.calls, ["normalize", "owner_check", "sanitize"])
        self.assertEqual(store.calls, [])

    def test_sanitizer_and_copy_failures_are_not_retried(self):
        report, db, store = self.run_flow(db=FakeDatabase(failure="sanitize"))
        self.assertEqual(report["error_code"], "SANITATION_FAILED")
        self.assertFalse(report["retry"])
        self.assertEqual(store.calls, [])
        payload = b"opaque private bytes"
        import uuid
        file_id = str(uuid.uuid4())
        self.files = [PrivateFile(f"runs/g03o-1017/{file_id}.blob", payload,
                                  hashlib.sha256(payload).hexdigest())]
        report, db, store = self.run_flow(
            db=FakeDatabase(refs={file_id}), store=FakeStore(fail=True))
        self.assertEqual(report["error_code"], "PRIVATE_FILE_COPY_FAILED")
        self.assertFalse(report["retry"])
        self.assertEqual(db.calls, ["normalize", "owner_check", "sanitize", "refs"])
        self.assertEqual(len(store.calls), 1)

    def test_private_reference_mismatch_prevents_copy(self):
        payload = b"opaque private bytes"
        import uuid
        self.files = [PrivateFile(f"runs/g03o-1017/{uuid.uuid4()}.blob", payload,
                                  hashlib.sha256(payload).hexdigest())]
        report, db, store = self.run_flow(db=FakeDatabase(refs=set()))
        self.assertEqual(report["error_code"], "PRIVATE_FILE_REFERENCE_MISMATCH")
        self.assertEqual(store.calls, [])


if __name__ == "__main__":
    unittest.main()
