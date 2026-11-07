from pathlib import Path
import unittest

SQL = Path(__file__).with_name("003_pre_v81_contract.sql").read_text()
V81 = Path(__file__).parents[2] / "migrations" / "V000081__document_verification_runtime_scope_helpers.sql"


class PreV81ContractTests(unittest.TestCase):
    def test_target_and_exact_v80_fence(self):
        self.assertIn("current_database() <> 'espacigo_wizard_20261106'", SQL)
        self.assertIn("current_setting('server_version_num')::integer <> 180006", SQL)
        self.assertIn("n <> 80 OR lo <> 1 OR hi <> 80", SQL)
        self.assertIn("V000080__document_verification_space_ownership_supersession.sql", SQL)
        self.assertIn("fc2c3eacf10bc99d6ad4c66a7a5aa2c80a0592b45eeee6e938400c95f613c401", SQL)

    def test_provisions_only_v81_helper_and_scoped_column_acl(self):
        self.assertIn("CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER", SQL)
        self.assertIn("CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn", SQL)
        self.assertIn("GRANT SELECT (id, estado), UPDATE (id)", SQL)
        self.assertIn("GRANT SELECT (id, propietario_id), UPDATE (id)", SQL)
        self.assertIn("WITH ADMIN FALSE, INHERIT FALSE, SET TRUE", SQL)
        self.assertNotIn("GRANT ALL", SQL.upper())
        self.assertNotIn("CREATE FUNCTION", SQL.upper())

    def test_no_migration_runner_execution_or_v81_function_execution_grants(self):
        self.assertNotIn("db/migrations", SQL)
        self.assertNotIn("EXECUTE ON FUNCTION", SQL.upper())
        self.assertNotIn("schema_migrations (", SQL)
        self.assertIn("GRANT SELECT, INSERT ON TABLE public.schema_migrations", SQL)

    def test_contract_is_aligned_to_immutable_v81_predicates(self):
        source = V81.read_text()
        for required in ("document_verification_private", "has_schema_privilege(helper_oid, 'public', 'USAGE')",
                         "acl.grantor=c.relowner", "public.schema_migrations",
                         "NOT m.inherit_option AND NOT m.admin_option"):
            self.assertIn(required, source)
        self.assertIn("schema_migrations", SQL)


if __name__ == "__main__":
    unittest.main()
