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
    source_report_components,
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
    def run_report_failure(self, database: str, stderr: bytes, stdout: bytes = b""):
        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            source = database == "espacigo_scratch"
            db = PsqlDatabase(database=database, user="postgres" if source else "espacigo_migrator_login",
                password_file=password_path, port=5432 if source else 5433)
            try:
                with patch("gcp03_origin_v64_transfer.subprocess.run",
                    return_value=SimpleNamespace(returncode=2, stdout=stdout, stderr=stderr)) as run:
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
            "sqlstate_category": "query_or_privilege",
        })
        self.assertNotIn("42501", repr(error.safe_details))
        self.assertNotIn("synthetic-password", repr(error.safe_details))

    def test_target_report_failure_exposes_endpoint_and_auth_category(self):
        error = self.run_report_failure("espacigo", b"ERROR:  28P01\n")

        self.assertEqual(error.safe_details, {
            "database_endpoint": "target",
            "report_component": "target_report",
            "sqlstate_category": "authentication",
        })

    def test_failure_without_sqlstate_keeps_endpoint_only(self):
        error = self.run_report_failure("espacigo", b"psql: connection reset\n")

        self.assertEqual(error.safe_details, {
            "database_endpoint": "target",
            "report_component": "target_report",
        })

    def test_source_report_marks_failing_component_in_one_read_only_snapshot(self):
        stdout = []
        for name, _ in source_report_components():
            if name == "database_identity":
                value = {"database": "espacigo_scratch", "postgres_major": 18}
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

        with tempfile.TemporaryDirectory() as directory:
            password_path = Path(directory) / "db-password"
            password_path.write_text("synthetic-password\n")
            os.chmod(password_path, 0o600)
            db = PsqlDatabase(database="espacigo_scratch", user="postgres",
                password_file=password_path, port=5432)
            try:
                with patch("gcp03_origin_v64_transfer.subprocess.run", return_value=completed) as run:
                    report = db.report()
                script = run.call_args.kwargs["input"].decode()
                self.assertTrue(script.startswith("BEGIN READ ONLY;"))
                self.assertTrue(script.rstrip().endswith("COMMIT;"))
                self.assertIn(f"{SOURCE_REPORT_MARKER}count_usuario", script)
                self.assertEqual(report["counts"], {"usuario": 1, "espacio": 1, "reserva": 1})
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
            "sqlstate_category": "query_or_privilege",
        })
        self.assertNotIn("private SQL", repr(error.safe_details))


if __name__ == "__main__":
    unittest.main()
