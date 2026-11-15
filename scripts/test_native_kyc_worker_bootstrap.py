#!/usr/bin/env python3
"""Offline contract tests for the native KYC worker LOGIN template."""
import base64
import hashlib
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TEMPLATE = ROOT / "db/repair/bootstrap_native_kyc_worker_login.sql.tmpl"
PIN = "395d771da8da79a6926feeeb43d9b355ea47778d311dffc1dd420aa63549eb4b"


def render(template: str, ledger: list[tuple[int, str, str]], verifier: str, finish: str = "ROLLBACK;") -> str:
    """Validate parent-supplied reviewed inputs, then render without DB access."""
    match = re.fullmatch(r"SCRAM-SHA-256\$4096:([A-Za-z0-9+/]+={0,2})\$([A-Za-z0-9+/]+={0,2}):([A-Za-z0-9+/]+={0,2})", verifier)
    if match is None:
        raise ValueError("verifier must have generated SCRAM-SHA-256 shape")
    try:
        lengths = tuple(len(base64.b64decode(v, validate=True)) for v in match.groups())
    except ValueError as exc:
        raise ValueError("invalid SCRAM base64") from exc
    if lengths != (16, 32, 32):
        raise ValueError("invalid SCRAM component lengths")
    if len(ledger) != 94 or [r[0] for r in ledger] != list(range(1, 95)):
        raise ValueError("ledger must be exactly contiguous V1..V94")
    if any(not re.fullmatch(r"[0-9a-f]{64}", c) for _, _, c in ledger):
        raise ValueError("invalid ledger checksum")
    encoded = "".join(f"{v} {name} {checksum}\n" for v, name, checksum in ledger).encode()
    if hashlib.sha256(encoded).hexdigest() != PIN:
        raise ValueError("ledger does not match reviewed fingerprint")
    if finish not in {"COMMIT;", "ROLLBACK;"}:
        raise ValueError("finish must be exactly COMMIT; or ROLLBACK;")
    if template.count("__SCRAM_VERIFIER__") != 1 or template.count("__FINISH__") != 1:
        raise ValueError("template placeholders are not exact")
    rendered = template.replace("__SCRAM_VERIFIER__", verifier).replace("__FINISH__", finish)
    if "__" in rendered:
        raise ValueError("unexpanded placeholder")
    return rendered


class BootstrapContractTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.sql = TEMPLATE.read_text()
        cls.verifier = "SCRAM-SHA-256$4096:" + base64.b64encode(b"s"*16).decode() + "$" + base64.b64encode(b"k"*32).decode() + ":" + base64.b64encode(b"v"*32).decode()
        cls.ledger = sorted((int(f.name[1:].split("__", 1)[0]), f.name, hashlib.sha256(f.read_bytes()).hexdigest()) for f in (ROOT / "db/migrations").glob("V*.sql"))

    def test_renderer_rejects_bad_ledger_and_credentials(self):
        synthetic = [(i, f"V{i:06d}__synthetic.sql", "a" * 64) for i in range(1, 95)]
        verifier = self.verifier
        for ledger in (synthetic[:-1], list(reversed(synthetic))):
            with self.assertRaises(ValueError):
                render(self.sql, ledger, verifier)
        with self.assertRaisesRegex(ValueError, "fingerprint"):
            render(self.sql, synthetic, verifier)
        with self.assertRaisesRegex(ValueError, "SCRAM"):
            render(self.sql, synthetic, "not-a-verifier")
        with self.assertRaises(ValueError):
            render(self.sql, self.ledger, verifier, "COMMIT; SELECT 1;")

    def test_successful_render_is_rollback_by_default(self):
        sql = render(self.sql, self.ledger, self.verifier)
        self.assertTrue(sql.rstrip().endswith("ROLLBACK;"))
        self.assertNotIn("__", sql)
        self.assertEqual(sql.count(self.verifier), 1)
        self.assertTrue(render(self.sql, self.ledger, self.verifier, "COMMIT;").rstrip().endswith("COMMIT;"))

    def test_bad_components_and_duplicate_placeholders_are_rejected(self):
        for bad in (self.verifier.replace("$4096:", "$1:"), self.verifier + "'", "SCRAM-SHA-256$4096:AAAA$AAAA:AAAA"):
            with self.assertRaises(ValueError):
                render(self.sql, self.ledger, bad)
        with self.assertRaises(ValueError):
            render(self.sql + "__FINISH__", self.ledger, self.verifier)

    def test_ledger_uses_real_newline_and_dependency_guards(self):
        self.assertIn("format(E'%s %s %s\\n'", self.sql)
        self.assertIn("pg_catalog.pg_shdepend", self.sql)
        self.assertIn("objid=16388", self.sql)
        self.assertIn("roleid=login OR member=login", self.sql)

    def test_template_scope_and_guardrails(self):
        for required in (r"\set ON_ERROR_STOP on", r"\set ECHO none", "lock_timeout='3s'", "statement_timeout='15s'", "pg_advisory_xact_lock", "current_database()", "16388", "180006", "session_user<>'postgres'", "inet_server_addr() IS NOT NULL", PIN, "CREATE ROLE espacigo_privacy_worker_login", "PASSWORD '__SCRAM_VERIFIER__'", "CONNECTION LIMIT 2", "GRANT espacigo_privacy_cleanup_worker", "WITH ADMIN FALSE, INHERIT FALSE, SET TRUE", "__FINISH__"):
            self.assertIn(required, self.sql)
        for forbidden in ("ALTER ROLE", "DROP ROLE", "UPDATE public.schema_migrations", "INSERT INTO public.schema_migrations", "GRANT ALL", "Cloud SQL", "espacigo_api_login"):
            self.assertNotIn(forbidden, self.sql)
        self.assertEqual(self.sql.count("CREATE ROLE"), 1)
        self.assertEqual(self.sql.count("GRANT "), 1)
        self.assertIn("IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_worker_login')", self.sql)
        self.assertIn("FROM pg_catalog.pg_auth_members WHERE roleid=worker OR member=worker", self.sql)


if __name__ == "__main__":
    unittest.main()
