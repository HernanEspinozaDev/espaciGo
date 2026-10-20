from __future__ import annotations

import os
from pathlib import Path
import json
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

from gcp03_origin_v64_transfer import (
    GCSFiles, PsqlDatabase, SOURCE_REPORT_MARKER, TransferStopped,
    source_report_components, source_report_script,
)


class Blob:
    name = "g03o-1017/11111111-1111-4111-8111-111111111111.blob"
    generation = 7
    size = 12
    crc32c = "c2VudGlubA=="
    metadata = {"sha256": "a" * 64}

    def reload(self, *, if_generation_match: int) -> None:
        if if_generation_match != self.generation:
            raise AssertionError("destination generation was not pinned")


class Bucket:
    def __init__(self) -> None:
        self.lookups: list[str] = []
        self.blob = Blob()

    def get_blob(self, name: str) -> Blob | None:
        self.lookups.append(name)
        return self.blob if name == self.blob.name else None

    def list_blobs(self, **kwargs):
        raise AssertionError("destination inventory must not list the bucket")


class ExistingDestinationBlob:
    def __init__(self) -> None:
        self.generation = 42
        self.size = 12
        self.crc32c = "c2VudGlubA=="
        self.persisted_metadata = {"sha256": "b" * 64, "transfer_id": "prior-invocation"}
        self.requested_metadata: dict[str, str] = {}
        self.delete_calls: list[int] = []

    @property
    def metadata(self) -> dict[str, str]:
        return self.persisted_metadata

    @metadata.setter
    def metadata(self, value: dict[str, str]) -> None:
        self.requested_metadata = value

    def rewrite(self, *args, **kwargs):
        raise RuntimeError("precondition failed: generation already exists")

    def delete(self, *, if_generation_match: int) -> None:
        self.delete_calls.append(if_generation_match)


class SourceBucket:
    def blob(self, name: str, *, generation: int):
        return object()


class ExistingDestinationBucket:
    def __init__(self) -> None:
        self.existing = ExistingDestinationBlob()

    def blob(self, name: str) -> ExistingDestinationBlob:
        return self.existing


class DestinationInventoryTests(unittest.TestCase):
    def test_reads_only_exact_allowlisted_destination_names(self) -> None:
        files = object.__new__(GCSFiles)
        files.target = Bucket()
        names = {
            "g03o-1017/11111111-1111-4111-8111-111111111111.blob",
            "g03o-1017/privacy-replay/privacy-replay-v1.json",
        }

        found = files.destination_inventory(names)

        self.assertEqual(files.target.lookups, sorted(names))
        self.assertEqual(list(found), ["g03o-1017/11111111-1111-4111-8111-111111111111.blob"])
        self.assertEqual(found[next(iter(found))]["generation"], 7)

        files.source = Bucket()
        with self.assertRaises(TransferStopped):
            files.inventory({"11111111-1111-4111-8111-111111111111"})
        self.assertEqual(files.source.lookups, [])

    def test_create_only_conflict_never_deletes_preexisting_generation(self) -> None:
        files = object.__new__(GCSFiles)
        files.source = SourceBucket()
        files.target = ExistingDestinationBucket()
        source = {
            "source_name": "runs/g03o-1017/11111111-1111-4111-8111-111111111111.blob",
            "generation": 7,
            "size": 12,
            "sha256": "a" * 64,
            "crc32c": "c2VudGlubA==",
        }

        with self.assertRaises(TransferStopped) as error:
            files.copy_create_only(source, "g03o-1017/11111111-1111-4111-8111-111111111111.blob")

        self.assertEqual(error.exception.code, "PRIVATE_FILE_COPY_FAILED")
        self.assertEqual(files.target.existing.generation, 42)
        self.assertEqual(files.target.existing.persisted_metadata["transfer_id"], "prior-invocation")
        self.assertEqual(files.target.existing.delete_calls, [])


class DatabasePreflightDiagnosticTests(unittest.TestCase):
    def test_source_dump_uses_raw_discovered_role_argument(self):
        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            db = PsqlDatabase(database="espacigo_scratch", user="postgres", password_file=password_path, port=5432)
            owner = SimpleNamespace(returncode=0, stdout=b'{"complete":true,"owner_count":1,"owner_oid":1,"owner_name":"fixture_owner","set_allowed":true,"access_complete":true,"row_security_complete":true}', stderr=b"")
            dumped = SimpleNamespace(returncode=0, stdout=b"", stderr=b"")
            try:
                destination = Path(directory) / "rows.sql"
                with patch("gcp03_origin_v64_transfer.subprocess.run", side_effect=[owner, dumped]) as run:
                    db.dump_plain_data(destination)
                args = run.call_args_list[1].args[0]
                self.assertIn("--role=fixture_owner", args)
                self.assertNotIn('--role="fixture_owner"', args)
            finally:
                db.close()

    def test_owner_discovery_fails_closed_for_mixed_owner_without_exposing_names(self):
        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            db = PsqlDatabase(database="espacigo_scratch", user="postgres", password_file=password_path, port=5432)
            response = b'{"complete":true,"owner_count":2,"owner_oid":1,"owner_name":"sensitive_role","set_allowed":true,"access_complete":true,"row_security_complete":true}'
            try:
                with patch("gcp03_origin_v64_transfer.subprocess.run", return_value=SimpleNamespace(returncode=0, stdout=response, stderr=b"")):
                    with self.assertRaises(TransferStopped) as error:
                        db.discover_source_owner()
                self.assertEqual(error.exception.code, "SOURCE_OWNER_PREFLIGHT_FAILED")
                self.assertFalse(error.exception.safe_details["source_required_owner_unique"])
                self.assertNotIn("sensitive_role", repr(error.exception.safe_details))
            finally:
                db.close()

    def test_owner_discovery_fails_closed_when_set_role_is_denied(self):
        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            db = PsqlDatabase(database="espacigo_scratch", user="postgres", password_file=password_path, port=5432)
            response = b'{"complete":true,"owner_count":1,"owner_oid":1,"owner_name":"sensitive_role","set_allowed":false,"access_complete":true,"row_security_complete":true}'
            try:
                with patch("gcp03_origin_v64_transfer.subprocess.run", return_value=SimpleNamespace(returncode=0, stdout=response, stderr=b"")):
                    with self.assertRaises(TransferStopped) as error:
                        db.discover_source_owner()
                self.assertFalse(error.exception.safe_details["source_required_owner_set_allowed"])
                self.assertNotIn("sensitive_role", repr(error.exception.safe_details))
            finally:
                db.close()

    def run_report_failure(self, database: str, stderr: bytes, stdout: bytes = b"", returncode: int = 2):
        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            source = database == "espacigo_scratch"
            db = PsqlDatabase(database=database, user="postgres" if source else "espacigo_migrator_login",
                password_file=password_path, port=5432 if source else 5433)
            try:
                failure = SimpleNamespace(returncode=returncode, stdout=stdout, stderr=stderr)
                owner = SimpleNamespace(returncode=0, stdout=b'{"complete":true,"owner_count":1,"owner_oid":1,"owner_name":"fixture_owner","set_allowed":true,"access_complete":true,"row_security_complete":true}', stderr=b"")
                with patch("gcp03_origin_v64_transfer.subprocess.run",
                    side_effect=[owner, failure] if source else [failure]) as run:
                    with self.assertRaises(TransferStopped) as error:
                        db.report()
                command = run.call_args.args[0]
                self.assertIn("--set=VERBOSITY=sqlstate", command)
                self.assertEqual(run.call_args.kwargs["stderr"], subprocess.PIPE)
                return error.exception
            finally:
                db.close()

    def test_source_report_failure_exposes_only_endpoint_and_category(self):
        marker = f"{SOURCE_REPORT_MARKER}database_identity\n".encode()
        error = self.run_report_failure("espacigo_scratch", b"ERROR:  42501\n", marker)

        self.assertEqual(error.code, "DATABASE_PREFLIGHT_QUERY_FAILED")
        self.assertEqual(error.stage, "preflight")
        self.assertEqual(error.safe_details, {
            "database_endpoint": "source",
            "report_component": "database_identity",
            "sqlstate_category": "insufficient_privilege",
        })
        self.assertNotIn("42501", repr(error.safe_details))
        self.assertNotIn("synthetic-password", repr(error.safe_details))

    def test_target_report_failure_exposes_endpoint_and_auth_category(self):
        error = self.run_report_failure("espacigo", b"ERROR:  28P01\n")

        self.assertEqual(error.safe_details, {
            "database_endpoint": "target",
            "report_component": "target_report",
            "sqlstate_category": "invalid_password",
        })

    def test_failure_without_sqlstate_keeps_endpoint_only(self):
        error = self.run_report_failure("espacigo", b"psql: connection reset\n")

        self.assertEqual(error.safe_details, {
            "database_endpoint": "target",
            "report_component": "target_report",
        })

    def test_source_report_parses_sqlstate_with_psql_stdin_location_prefix(self):
        marker = f"{SOURCE_REPORT_MARKER}count_usuario\n".encode()
        error = self.run_report_failure(
            "espacigo_scratch", b"psql:<stdin>:5: ERROR:  42501\n", marker)

        self.assertEqual(error.safe_details, {
            "database_endpoint": "source",
            "report_component": "count_usuario",
            "sqlstate_category": "insufficient_privilege",
        })
        self.assertNotIn("42501", repr(error.safe_details))

    def test_sqlstate_safe_reason_allowlist_covers_known_query_failures(self):
        cases = [
            (b"ERROR:  42501\n", "insufficient_privilege"),
            (b"psql:<stdin>:6: ERROR:  42P01\n", "undefined_table"),
            (b"psql:<stdin>:7: ERROR:  42703\n", "undefined_column"),
            (b"psql:<stdin>:8: ERROR:  42601\n", "syntax_error"),
            (b"ERROR:  42P99\n", "query_or_privilege"),
            (b"ERROR:  28P01\n", "invalid_password"),
            (b"ERROR:  99001\n", "other"),
        ]
        from gcp03_origin_v64_transfer import sqlstate_category
        for stderr, expected in cases:
            with self.subTest(stderr=stderr):
                reason = sqlstate_category(stderr)
                self.assertEqual(reason, expected)
                self.assertNotIn(stderr.strip().split()[-1].decode(), reason)

    def test_source_report_marks_failing_component_in_one_read_only_snapshot(self):
        stdout = []
        for name, _ in source_report_components():
            if name == "database_identity":
                value = {"database": "espacigo_scratch", "postgres_major": 18}
            elif name == "source_table_access":
                value = {"source_table_access": {
                    "usuario_present": True, "usuario_select_allowed": True,
                    "public_schema_usage_allowed": True, "schema_owner_role_present": True,
                    "schema_owner_member": True, "schema_owner_set_allowed": False,
                    "usuario_owner_role_present": True, "usuario_owner_is_current_user": False,
                    "current_user_can_set_usuario_owner": False,
                }}
            elif name.startswith("count_"):
                value = {"counts": {name.removeprefix("count_"): 1}}
            elif name == "migration_history":
                value = {"migration_history": []}
            elif name.startswith("seed_"):
                value = {"seed_fingerprints": {name.removeprefix("seed_"): "a" * 32}}
            elif name == "external_sessions":
                value = {"external_sessions": 0}
            else:
                value = {"report_complete": True}
            stdout.extend([f"{SOURCE_REPORT_MARKER}{name}", json.dumps(value)])
        completed = SimpleNamespace(returncode=0, stdout=("\n".join(stdout) + "\n").encode(), stderr=b"")
        owner = SimpleNamespace(returncode=0, stdout=b'{"complete":true,"owner_count":1,"owner_oid":1,"owner_name":"fixture_owner","set_allowed":true,"access_complete":true,"row_security_complete":true}', stderr=b"")

        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            db = PsqlDatabase(database="espacigo_scratch", user="postgres",
                password_file=password_path, port=5432)
            try:
                with patch("gcp03_origin_v64_transfer.subprocess.run", side_effect=[owner, completed]) as run:
                    report = db.report()
                script = run.call_args_list[1].kwargs["input"].decode()
                self.assertTrue(script.startswith('SET ROLE "fixture_owner";'))
                self.assertIn("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;", script)
                self.assertTrue(script.rstrip().endswith("COMMIT;"))
                self.assertIn(f"{SOURCE_REPORT_MARKER}count_usuario", script)
                self.assertEqual(report["counts"], {"usuario": 1, "espacio": 1, "reserva": 1})
                self.assertEqual(report["source_table_access"], {
                    "usuario_present": True, "usuario_select_allowed": True,
                    "public_schema_usage_allowed": True, "schema_owner_role_present": True,
                    "schema_owner_member": True, "schema_owner_set_allowed": False,
                    "usuario_owner_role_present": True, "usuario_owner_is_current_user": False,
                    "current_user_can_set_usuario_owner": False,
                })
                self.assertEqual(report["external_sessions"], 0)
                self.assertTrue(report["report_complete"])
            finally:
                db.close()

    def test_source_report_failure_identifies_later_component_without_echoing_error(self):
        stdout = (f"{SOURCE_REPORT_MARKER}database_identity\n"
            '{"database":"espacigo_scratch","postgres_major":18}\n'
            f"{SOURCE_REPORT_MARKER}count_usuario\n").encode()
        error = self.run_report_failure("espacigo_scratch", b"ERROR:  42501\nprivate SQL and row data", stdout)

        self.assertEqual(error.safe_details, {
            "database_endpoint": "source",
            "report_component": "count_usuario",
            "sqlstate_category": "insufficient_privilege",
        })
        self.assertNotIn("private SQL", repr(error.safe_details))

    def test_source_table_access_precheck_blocks_count_and_reports_only_booleans(self):
        script = source_report_script()
        self.assertIn("pg_catalog.pg_class", script)
        self.assertIn("has_table_privilege(current_user,usuario_oid,'SELECT')", script)
        self.assertIn("has_schema_privilege(current_user,'public','USAGE')", script)
        self.assertIn("pg_has_role(current_user,schema_owner_oid,'MEMBER')", script)
        self.assertIn("pg_has_role(current_user,schema_owner_oid,'SET')", script)
        self.assertIn("usuario_owner_oid", script)
        self.assertIn("usuario_owner_oid=current_user_oid", script)
        self.assertIn("pg_has_role(current_user,usuario_owner_oid,'SET')", script)
        self.assertLess(script.index(f"{SOURCE_REPORT_MARKER}source_table_access"),
            script.index(f"{SOURCE_REPORT_MARKER}count_usuario"))
        self.assertLess(script.index("RAISE EXCEPTION 'source access precheck failed'"),
            script.index(f"{SOURCE_REPORT_MARKER}count_usuario"))
        self.assertIn("NOT has_schema_privilege(current_user,'public','USAGE')", script)

        stdout = (f"{SOURCE_REPORT_MARKER}source_table_access\n"
            '{"source_table_access":{"usuario_present":true,"usuario_select_allowed":false,'
            '"public_schema_usage_allowed":true,"schema_owner_role_present":false,'
            '"schema_owner_member":false,"schema_owner_set_allowed":false,'
            '"usuario_owner_role_present":true,"usuario_owner_is_current_user":false,'
            '"current_user_can_set_usuario_owner":false}}\n').encode()
        error = self.run_report_failure("espacigo_scratch", b"", stdout, returncode=3)

        self.assertEqual(error.code, "DATABASE_PREFLIGHT_QUERY_FAILED")
        self.assertEqual(error.safe_details, {
            "database_endpoint": "source",
            "report_component": "source_table_access",
            "source_table_present": True,
            "source_table_select_allowed": False,
            "source_public_schema_usage_allowed": True,
            "source_schema_owner_role_present": False,
            "source_schema_owner_member": False,
            "source_schema_owner_set_allowed": False,
            "source_table_owner_role_present": True,
            "source_table_owner_is_current_user": False,
            "source_current_user_can_set_table_owner": False,
        })
        self.assertNotIn("usuario", repr(error.safe_details))
        self.assertNotIn("SELECT", repr(error.safe_details))


if __name__ == "__main__":
    unittest.main()
