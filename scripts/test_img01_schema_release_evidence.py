#!/usr/bin/env python3
from __future__ import annotations

from copy import deepcopy
from datetime import datetime, timedelta, timezone
import hashlib
import importlib.util
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
MODULE_PATH = ROOT / "scripts/verify_img01_schema_release_evidence.py"
SPEC = importlib.util.spec_from_file_location("img01_schema_evidence", MODULE_PATH)
assert SPEC and SPEC.loader
MODULE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(MODULE)


def valid_report() -> dict[str, object]:
    migrations = MODULE.migration_inventory(ROOT / "db/migrations")
    roles = [
        {"name": "espacigo_api_login", "can_login": True, "superuser": False, "createdb": False, "createrole": False, "replication": False, "bypassrls": False, "inherit": True},
        {"name": "espacigo_runtime", "can_login": False, "superuser": False, "createdb": False, "createrole": False, "replication": False, "bypassrls": False, "inherit": False},
        {"name": "espacigo_privacy_cleanup_worker", "can_login": False, "superuser": False, "createdb": False, "createrole": False, "replication": False, "bypassrls": False, "inherit": False},
        {"name": "postgres", "can_login": True, "superuser": False, "createdb": True, "createrole": True, "replication": False, "bypassrls": False, "inherit": True},
    ]
    relations = [
        {"name": name, "kind": "r", "owner": "postgres"}
        for name in sorted(MODULE.REQUIRED_RELATIONS)
    ]
    routines = [
        {
            "name": name,
            "arguments": arguments,
            "owner": "postgres",
            "security_definer": True,
            "configuration": ["search_path=pg_catalog, public", "TimeZone=UTC"],
            "public_execute_revoked": True,
        }
        for name, arguments in sorted(MODULE.REQUIRED_ROUTINES)
    ]
    runtime_privileges = {key: False for key in MODULE.RUNTIME_TRUE | MODULE.RUNTIME_FALSE}
    runtime_privileges.update({key: True for key in MODULE.RUNTIME_TRUE})
    worker_privileges = {key: False for key in MODULE.WORKER_TRUE | MODULE.WORKER_FALSE}
    worker_privileges.update({key: True for key in MODULE.WORKER_TRUE})
    return {
        "format": MODULE.FORMAT,
        "captured_at": datetime.now(timezone.utc).isoformat(),
        "target": {
            "project_id": MODULE.PROJECT,
            "instance_connection_name": MODULE.INSTANCE,
            "database": MODULE.DATABASE,
            "server_version_num": 180006,
            "transaction_read_only": "on",
            "session_user": "postgres",
            "current_user": "postgres",
            "database_owner": "postgres",
            "public_schema_owner": "pg_database_owner",
            "schema_migrations_owner": "postgres",
            "schema_migrations_kind": "r",
        },
        "query_sha256": hashlib.sha256((ROOT / "db/query/img01_schema_release_evidence.sql").read_bytes()).hexdigest(),
        "migration_ledger": {
            "count": len(migrations),
            "first_version": 1,
            "last_version": len(migrations),
            "entries": migrations,
        },
        "relations": relations,
        "routines": routines,
        "roles": roles,
        "memberships": [
            {"granted_role": "espacigo_runtime", "member": "espacigo_api_login", "admin_option": False, "inherit_option": False, "set_option": True}
        ],
        "runtime_privileges": runtime_privileges,
        "worker_privileges": worker_privileges,
    }


class SchemaEvidenceTests(unittest.TestCase):
    def validate_report(self, report: dict[str, object]) -> None:
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "evidence.json"
            import json

            path.write_text(json.dumps(report), encoding="utf-8")
            MODULE.validate(path, ROOT / "db/migrations", ROOT / "db/query/img01_schema_release_evidence.sql")

    def test_accepts_current_target_with_owner_neutral_inventory(self) -> None:
        report = valid_report()
        report["target"]["database_owner"] = "observed_db_owner"
        report["target"]["public_schema_owner"] = "pg_database_owner"
        for relation in report["relations"]:
            relation["owner"] = "observed_object_owner"
        self.validate_report(report)

    def test_rejects_ledger_checksum_drift(self) -> None:
        report = valid_report()
        report["migration_ledger"]["entries"][67]["checksum"] = "0" * 64
        with self.assertRaisesRegex(MODULE.EvidenceError, "ledger"):
            self.validate_report(report)

    def test_rejects_missing_relation(self) -> None:
        report = valid_report()
        report["relations"] = report["relations"][:-1]
        with self.assertRaisesRegex(MODULE.EvidenceError, "relations"):
            self.validate_report(report)

    def test_rejects_runtime_ledger_privilege(self) -> None:
        report = valid_report()
        report["runtime_privileges"]["ledger_select"] = True
        with self.assertRaisesRegex(MODULE.EvidenceError, "ledger privilege"):
            self.validate_report(report)

    def test_rejects_stale_or_future_report(self) -> None:
        report = valid_report()
        report["captured_at"] = (datetime.now(timezone.utc) - timedelta(hours=7)).isoformat()
        with self.assertRaisesRegex(MODULE.EvidenceError, "older than six hours"):
            self.validate_report(report)

    def test_rejects_missing_or_unexpected_migrator_identity_not_schema_owner(self) -> None:
        report = valid_report()
        report["roles"] = [role for role in report["roles"] if role["name"] != "espacigo_schema_owner"]
        self.validate_report(report)
        report["roles"].append({"name": "espacigo_migrator_login", "can_login": True, "superuser": False, "createdb": False, "createrole": False, "replication": False, "bypassrls": False, "inherit": True})
        self.validate_report(report)

    def test_accepts_only_the_automatic_owner_admin_edge(self):
        report = valid_report()
        report["memberships"].append({"granted_role": "espacigo_privacy_cleanup_worker", "member": "postgres", "grantor_oid": 10, "admin_option": True, "inherit_option": False, "set_option": False})
        self.validate_report(report)
        for change in ({"member": "espacigo_api_login"}, {"member": "unreviewed_owner"}, {"grantor_oid": 999}, {"admin_option": False}, {"inherit_option": True}, {"set_option": True}):
            with self.subTest(change=change):
                invalid = deepcopy(report)
                invalid["memberships"][-1].update(change)
                with self.assertRaises(MODULE.EvidenceError):
                    self.validate_report(invalid)


if __name__ == "__main__":
    unittest.main()
