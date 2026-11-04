import importlib.util
import tempfile
import unittest
from unittest.mock import patch
import hashlib
from pathlib import Path

SCRIPT = Path(__file__).with_name("verify_f5_restore_schema.py")
spec = importlib.util.spec_from_file_location("checker", SCRIPT)
checker = importlib.util.module_from_spec(spec)
spec.loader.exec_module(checker)

V82 = "b53fe7f6f03af4d9081e4a378053ad0780d8b532962326488108201eb098ed96"
V83 = "405c2f02f8a29e962421a44cbbd07e6e7fae4aa4696483e9d45ab209e82e9333"
FUNC = "CREATE FUNCTION public.document_verification_space_owner_changed() RETURNS trigger\n LANGUAGE plpgsql\nAS $$\nBEGIN\n RETURN NEW;\nEND;\n$$;\n"
BEFORE = "-- pg_dump header\nCREATE TABLE public.t (id integer);\nALTER TABLE public.t OWNER TO app_owner;\n" + FUNC
AFTER = BEFORE.replace("RETURN NEW;", "RETURN OLD;") + checker.GRANT + "\n"
V82_SOURCE = "CREATE OR REPLACE FUNCTION public.document_verification_space_owner_changed() RETURNS trigger LANGUAGE plpgsql AS $function$\nBEGIN\n RETURN NEW;\nEND;\n$function$;"
V83_SOURCE = V82_SOURCE.replace("RETURN NEW;", "RETURN OLD;")


class CheckerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.root = Path(self.tmp.name)
        self.before = self.root / "before.sql"
        self.after = self.root / "after.sql"
        self.migrations = self.root / "migrations"
        self.migrations.mkdir()
        self.before.write_text(BEFORE)
        self.after.write_text(AFTER)
        v82_hash = hashlib.sha256(V82_SOURCE.encode()).hexdigest()
        v83_hash = hashlib.sha256(V83_SOURCE.encode()).hexdigest()
        self.patches = [patch.object(checker, "V82_SHA", v82_hash), patch.object(checker, "V83_SHA", v83_hash)]
        for item in self.patches:
            item.start()
            self.addCleanup(item.stop)
        (self.migrations / "V82__approved.sql").write_text(V82_SOURCE)
        (self.migrations / "V83__change.sql").write_text(V83_SOURCE)

    def tearDown(self):
        self.tmp.cleanup()

    def check(self, before=None, after=None):
        return checker.verify(self.before if before is None else before,
                              self.after if after is None else after, self.migrations)

    def test_accepts_only_pinned_function_body_and_one_grant_line_removal(self):
        result = self.check()
        self.assertEqual(result, (hashlib.sha256(V82_SOURCE.encode()).hexdigest(), hashlib.sha256(V83_SOURCE.encode()).hexdigest()))

    def test_rejects_acl_delta(self):
        self.after.write_text(AFTER + "GRANT SELECT ON public.t TO PUBLIC;\n")
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_rejects_owner_delta(self):
        self.after.write_text(AFTER.replace("OWNER TO app_owner", "OWNER TO other_owner"))
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_rejects_duplicate_target_grant(self):
        self.before.write_text(BEFORE + "GRANT UPDATE(id) ON TABLE public.document_verification_reviewer_grant TO espacigo_runtime;\n")
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_rejects_function_body_tampering(self):
        self.after.write_text(AFTER.replace("RETURN OLD;", "RETURN NULL;"))
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_rejects_signature_or_search_path_change(self):
        self.after.write_text(AFTER.replace("RETURNS trigger", "RETURNS text"))
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_preserves_other_function_string_and_dollar_body_content(self):
        self.before.write_text(BEFORE + "CREATE FUNCTION public.other() RETURNS text AS $$ SELECT '-- fake comment'; $$ LANGUAGE sql;\n")
        self.after.write_text(AFTER + "CREATE FUNCTION public.other() RETURNS text AS $$ SELECT '-- changed comment'; $$ LANGUAGE sql;\n")
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_requires_migration_bytes_to_match_pinned_hashes(self):
        (self.migrations / "V82__approved.sql").write_text(V82_SOURCE + "\n")
        with self.assertRaises(checker.VerificationError):
            self.check()


    def test_preserves_dollar_body_comment_lines_and_blank_lines(self):
        other = "CREATE FUNCTION public.other() RETURNS text AS $body$\n-- preserved\n\nSELECT 'value';\n$body$ LANGUAGE sql;\n"
        self.before.write_text(BEFORE + other)
        self.after.write_text(AFTER + other.replace('-- preserved', '-- altered'))
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_preserves_multiline_string_content(self):
        other = "COMMENT ON TABLE public.t IS 'line one\n-- literal data\n\nline two';\n"
        self.before.write_text(BEFORE + other)
        self.after.write_text(AFTER + other.replace('-- literal data', '-- altered data'))
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_preserves_function_settings(self):
        self.after.write_text(AFTER.replace(' LANGUAGE plpgsql', " LANGUAGE plpgsql\n SET search_path TO public"))
        with self.assertRaises(checker.VerificationError):
            self.check()

    def test_accepts_dump_metadata_changes_only(self):
        self.before.write_text('\\restrict old\n' + BEFORE + '\\unrestrict old\n')
        self.after.write_text('\\restrict new\n-- dump header differs\n' + AFTER + '\\unrestrict new\n')
        self.check()


if __name__ == "__main__":
    unittest.main()
