#!/usr/bin/env python3
"""Static contract checks for the isolated wizard empty-ledger preparation."""
from pathlib import Path
import re
import unittest

ROOT = Path(__file__).resolve().parents[1]
SQL = ROOT / "db/bootstrap/wizard-improvements/002_empty_ledger.sql"
RUNNER = ROOT / "internal/migrator/runner.go"


def compact(text: str) -> str:
    return re.sub(r"\s+", " ", re.sub(r"--[^\n]*", " ", text)).strip().lower()


class EmptyLedgerContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.sql = SQL.read_text()
        cls.normalized = compact(cls.sql)
        cls.runner = RUNNER.read_text()

    def test_exact_runner_table_contract(self):
        match = re.search(r"const createHistory = `(.*?)`", self.runner, re.S)
        self.assertIsNotNone(match)
        runner_ddl = compact(match.group(1))
        created = re.search(r"create table public\.schema_migrations\s*\((.*?)\);", self.normalized)
        self.assertIsNotNone(created)
        self.assertEqual("create table public.schema_migrations (" + created.group(1) + ")", runner_ddl.replace("if not exists ", ""))
        self.assertIn("create table public.schema_migrations (", self.normalized)
        self.assertNotIn("create table if not exists", self.normalized)

    def test_pinned_identity_and_atomic_lockstep_contract(self):
        for fragment in ("current_database() <> 'espacigo_wizard_20261106'", "session_user <> 'postgres'", "current_user <> 'postgres'", "current_setting('server_version_num')::integer <> 180006", "begin;", "commit;", "pg_advisory_xact_lock"):
            self.assertIn(fragment, self.normalized)
        lock_class = re.search(r"advisoryLockClass\s+int32\s*=\s*(0x[0-9a-fA-F]+)", self.runner)
        lock_object = re.search(r"advisoryLockObject\s+int32\s*=\s*(0x[0-9a-fA-F]+)", self.runner)
        if lock_class is None or lock_object is None:
            self.fail("runner advisory lock constants are missing")
        expected_lock = (int(lock_class.group(1), 16), int(lock_object.group(1), 16))
        sql_lock = re.search(r"pg_advisory_xact_lock\s*\(\s*(\d+)\s*,\s*(\d+)\s*\)", self.normalized)
        if sql_lock is None:
            self.fail("SQL advisory lock call is missing")
        self.assertEqual(expected_lock, tuple(map(int, sql_lock.groups())))

    def test_fail_closed_for_existing_ledger_objects_and_acl_baseline(self):
        for fragment in ("to_regclass('public.schema_migrations') is not null", "application objects already exist", "application types already exist", "unexpected default acls", "unexpected public schema acl grantee", "ledger already exists"):
            self.assertIn(fragment, self.normalized)
        self.assertNotIn("create table if not exists", self.normalized)

    def test_minimal_acl_and_postcondition_readback(self):
        for fragment in ("revoke all on table public.schema_migrations from public", "grant select, insert on table public.schema_migrations to espacigo_schema_owner", "a.grantor <> owner_oid", "a.is_grantable", "where c.oid=ledger_oid and (a.grantee=0", "from public.schema_migrations", "a.grantee <> owner_oid", "acldefault('r',owner_oid)"):
            self.assertIn(fragment, self.normalized)
        self.assertIn("array['espacigo_schema_owner:insert:false','espacigo_schema_owner:select:false']", self.normalized)
        self.assertIn("count(*) from pg_catalog.pg_attribute", self.normalized)
        self.assertIn("a.grantee=0 and a.privilege_type='usage' and not a.is_grantable", self.normalized)
        self.assertIn("count(*) from pg_catalog.pg_roles", self.normalized)
        self.assertIn("m.set_option", self.normalized)
        expected_roles = "'espacigo_schema_owner','espacigo_runtime','espacigo_gadm_bootstrap','espacigo_privacy_cleanup_worker','espacigo_gcp06_test_access','espacigo_migrator_login','espacigo_api_login'"
        self.assertIn(f"where (g.rolname in ({expected_roles}) or u.rolname in ({expected_roles}))", self.normalized)

    def test_transaction_mode_is_not_double_wrapped_instructions(self):
        self.assertIn("execute with psql -x -v on_error_stop=1 -f; this file owns its transaction", self.sql.lower())
        self.assertEqual(1, len(re.findall(r"\bbegin\s*;", self.normalized)))
        self.assertEqual(1, len(re.findall(r"\bcommit\s*;", self.normalized)))


if __name__ == "__main__":
    unittest.main()
