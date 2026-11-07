from pathlib import Path
import unittest
SQL=Path(__file__).with_name("004_prepare_v81.sql").read_text()
CHECKER=Path(__file__).with_name("003_pre_v81_contract.sql")
class PrepareV81Tests(unittest.TestCase):
 def test_separate_from_readonly_checker(self):
  self.assertIn("BEGIN TRANSACTION READ ONLY;",CHECKER.read_text()); self.assertIn("ROLLBACK;",CHECKER.read_text())
  self.assertTrue(SQL.rstrip().endswith("COMMIT;")); self.assertIn("advisory_xact_lock(1163087943, 1296648018)",SQL)
 def test_v80_and_absent_helper_guards(self):
  self.assertIn("expected exact V1..V80 source-matching ledger",SQL); self.assertIn("helper_absent_guard",SQL)
  self.assertIn("CREATE ROLE espacigo_document_verification_fn NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT",SQL)
  self.assertIn("CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn",SQL)
 def test_do_blocks_are_separate_and_postconditions_match_checker(self):
  import re
  tags=re.findall(r"(?im)^DO\s+\$([A-Za-z_][A-Za-z_0-9]*)\$", SQL)
  self.assertEqual(tags.count("helper_absent_guard"), 1)
  self.assertEqual(tags.count("v81_preconditions"), 1)
  self.assertNotRegex(SQL, r"(?i)DO\s+\$[A-Za-z_][A-Za-z_0-9]*\$\s*DO\b")
  for tag in tags:
   match=re.search(rf"(?is)^DO\s+\${tag}\$\s*(.*?)\s*\${tag}\$\s*;?", SQL, re.M)
   self.assertIsNotNone(match, f"unclosed DO block {tag}")
   self.assertRegex(match.group(1).lstrip(), r"(?i)^(?:BEGIN|DECLARE)\b")
  precondition=re.search(r"(?is)^DO\s+\$v81_preconditions\$\s*(.*?)\s*\$v81_preconditions\$\s*;?", SQL, re.M).group(1).strip()
  checker=CHECKER.read_text()
  checker_body=re.search(r"(?is)^DO\s+\$v81_preconditions\$\s*(.*?)\s*\$v81_preconditions\$\s*;?", checker, re.M).group(1).strip()
  self.assertEqual(precondition, checker_body)
  helper=SQL.index("DO $helper_absent_guard$")
  role=SQL.index("CREATE ROLE espacigo_document_verification_fn")
  end_helper=SQL.index("$helper_absent_guard$", helper+1)
  pre=SQL.index("DO $v81_preconditions$")
  self.assertLess(helper, end_helper)
  self.assertLess(end_helper, role)
  self.assertLess(role, pre)
 def test_scoped_acl(self):
  for x in ("GRANT USAGE ON SCHEMA public TO espacigo_document_verification_fn","GRANT espacigo_document_verification_fn TO espacigo_schema_owner WITH ADMIN FALSE, INHERIT FALSE, SET TRUE","REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner","GRANT SELECT (id, estado), UPDATE (id) ON TABLE public.usuario","GRANT SELECT (id, propietario_id), UPDATE (id) ON TABLE public.espacio"): self.assertIn(x,SQL)
  self.assertNotIn("GRANT ALL",SQL.upper()); self.assertNotIn("schema_migrations (",SQL)
if __name__=="__main__": unittest.main()
