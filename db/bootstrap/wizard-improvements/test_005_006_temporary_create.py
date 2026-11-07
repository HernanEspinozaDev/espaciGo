from pathlib import Path
import hashlib, re, unittest

BASE = Path(__file__).parent
ROOT = BASE.parents[2]

class TemporaryCreateSqlTests(unittest.TestCase):
    def test_v82_v83_manifest_matches_all_81_sources(self):
        sql = (BASE / "005_allow_v82_v83_create.sql").read_text()
        entries = re.findall(r"\((\d+), '(V\d{6}__[^']+\.sql)', '([0-9a-f]{64})'\)", sql)
        self.assertEqual(len(entries), 81)
        for version, name, digest in entries:
            path = ROOT / "db/migrations" / name
            self.assertTrue(path.is_file(), name)
            self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), digest, name)
            self.assertEqual(int(version), int(name[1:7]))
        self.assertEqual([int(e[0]) for e in entries], list(range(1, 82)))

    def test_grant_is_narrow_and_transactional_with_readback(self):
        sql = (BASE / "005_allow_v82_v83_create.sql").read_text()
        self.assertIn("BEGIN;", sql); self.assertIn("COMMIT;", sql)
        self.assertIn("pg_advisory_xact_lock(1163087943, 1296648018)", sql)
        self.assertIn("GRANT CREATE ON SCHEMA public TO espacigo_schema_owner", sql)
        self.assertNotRegex(sql, r"(?i)GRANT\s+CREATE\s+ON\s+SCHEMA\s+public\s+TO\s+(PUBLIC|espacigo_runtime|espacigo_api_login)")
        self.assertIn("espacigo_wizard_20261106", sql)
        self.assertIn("180006", sql)
        self.assertIn("expected exact source-matching V1..V81 ledger", sql)
        self.assertIn("has_schema_privilege(so,'public','CREATE')", sql)
        self.assertIn("public.schema_migrations'::regclass AND relowner='postgres'::regrole", sql)
        self.assertIn("n.nspname='public' AND p.proname='document_verification_space_owner_changed'", sql)
        v81_source = (ROOT / "db/migrations/V000081__document_verification_runtime_scope_helpers.sql").read_text()
        expected_functions = re.findall(r"CREATE FUNCTION (document_verification_private\.document_verification_[a-z_]+)\(([^)]*)\)", v81_source)
        expected_signatures = {
            "document_verification_private.document_verification_lock_active_actor(uuid)",
            "document_verification_private.document_verification_lock_space_owner(uuid,uuid)",
            "document_verification_private.document_verification_space_owned_by(uuid,uuid)",
            "document_verification_private.document_verification_lock_space_row(uuid)",
        }
        self.assertEqual(len(expected_functions), 4)
        self.assertEqual({name + "(" + ",".join(re.findall(r"\b(uuid)\b", args)) + ")" for name, args in expected_functions}, expected_signatures)
        self.assertIn("n.nspname='document_verification_private' AND n.nspowner=helper", sql)
        for signature in expected_signatures:
            self.assertIn("'" + signature + "'", sql)
        self.assertIn("to_regprocedure(expected.signature)", sql)
        self.assertIn("p.proowner<>helper", sql)
        self.assertNotIn("private_schema", sql)
        self.assertNotIn("document_verification_private.document_verification_space_owner_changed", sql)
        self.assertRegex(sql, r"pg_auth_members WHERE roleid IN \(so,helper,runtime_role\) OR member IN \(so,mig,helper,api,runtime_role\)")
        self.assertNotIn("count(*) FROM pg_auth_members)", sql)

    def test_cleanup_revokes_without_later_ledger_diagnostics(self):
        sql = (BASE / "006_revoke_v82_v83_create.sql").read_text()
        self.assertIn("BEGIN;", sql); self.assertIn("COMMIT;", sql)
        self.assertIn("pg_advisory_xact_lock(1163087943, 1296648018)", sql)
        revoke = sql.index("REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner")
        self.assertEqual(sql.count("REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner"), 1)
        self.assertNotIn("ledger", sql[revoke:])
        self.assertIn("has_schema_privilege('espacigo_schema_owner','public','CREATE')", sql)
        self.assertNotRegex(sql, r"(?i)\b(INSERT|UPDATE|DELETE|TRUNCATE)\s+INTO?\s+public\.schema_migrations")
        self.assertIn("V82/V83 cleanup target or administrator", sql)

if __name__ == "__main__":
    unittest.main()
