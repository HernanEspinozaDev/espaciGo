import io
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace

from gcp08_legacy_state_reconcile import (
    BUCKET, CONFIRM_REMOVE, STATE_OBJECT, TARGETS, _create_and_verify_backup,
    _object_metadata,
    _remove, analyze_state, state_fingerprint_without_targets, verify_identity,
    EXPECTED_ACTOR_ID, EXPECTED_OWNER_ID, EXPECTED_REF, EXPECTED_REPOSITORY,
    EXPECTED_REPOSITORY_ID, EXPECTED_WORKFLOW_REF,
)


def make_state(serial=12, include_targets=True, database_instance="espacigo-testprod-pg",
               include_extra_instance=False):
    resources = [
        {
            "mode": "data", "type": "google_sql_database_instance", "name": "gcp06_candidate",
            "instances": [{"index_key": 0, "attributes": {"name": "candidate-run", "project": "espacigo-testprod"}}],
        },
        {
            "mode": "managed", "type": "google_cloud_run_v2_service", "name": "api",
            "instances": [{"index_key": 0, "attributes": {"name": "api-service", "project": "espacigo-testprod"}}],
        },
    ]
    if include_targets:
        legacy_instances = [{"index_key": 0, "attributes": {
            "name": "espacigo-testprod-pg", "project": "espacigo-testprod", "id": "private-id-marker",
        }}]
        if include_extra_instance:
            legacy_instances.append({"index_key": 1, "attributes": {
                "name": "other-managed-instance", "project": "espacigo-testprod",
            }})
        resources.extend([
            {
                "mode": "managed", "type": "google_sql_database_instance", "name": "postgres",
                "instances": legacy_instances,
            },
            {
                "mode": "managed", "type": "google_sql_database", "name": "application",
                "instances": [{"index_key": 0, "attributes": {
                    "name": "espacigo", "instance": database_instance,
                    "project": "espacigo-testprod", "id": "private-database-id-marker",
                }}],
            },
        ])
    elif include_extra_instance:
        resources.append({
            "mode": "managed", "type": "google_sql_database_instance", "name": "postgres",
            "instances": [{"index_key": 1, "attributes": {
                "name": "other-managed-instance", "project": "espacigo-testprod",
            }}],
        })
    return {
        "version": 4, "terraform_version": "1.16.6", "serial": serial,
        "lineage": "11111111-2222-4333-8444-555555555555", "resources": resources,
    }


def state_resource_fingerprint(state):
    return state_fingerprint_without_targets(state)


class Gcp08LegacyStateReconcileTests(unittest.TestCase):
    def test_inspector_requires_exact_legacy_instance_and_database_binding(self):
        summary = analyze_state(make_state())
        self.assertEqual(summary["serial"], 12)
        self.assertTrue(summary["eligible"])
        self.assertTrue(summary["legacy_instance_matches"])
        self.assertTrue(summary["legacy_database_matches"])
        self.assertTrue(summary["database_bound_to_legacy_instance"])
        self.assertFalse(summary["database_is_v67"])

        for wrong in ("espacigo-testprod-db", "projects/espacigo-testprod/instances/espacigo-testprod-db"):
            with self.subTest(database_instance=wrong):
                summary = analyze_state(make_state(database_instance=wrong))
                self.assertFalse(summary["eligible"])
                self.assertFalse(summary["database_bound_to_legacy_instance"])
                self.assertTrue(summary["database_is_v67"])

    def test_identity_is_pinned_to_human_actor_main_workflow_and_environment(self):
        context = {
            "repository": EXPECTED_REPOSITORY,
            "repository_id": EXPECTED_REPOSITORY_ID,
            "owner_id": EXPECTED_OWNER_ID,
            "actor_id": EXPECTED_ACTOR_ID,
            "ref": EXPECTED_REF,
            "workflow_ref": EXPECTED_WORKFLOW_REF,
            "event_name": "workflow_dispatch",
            "environment": "testprod",
        }
        self.assertTrue(verify_identity(context))
        for key, value in (("actor_id", "0"), ("ref", "refs/heads/topic"),
                           ("workflow_ref", "other.yml@refs/heads/main"),
                           ("environment", "dev")):
            with self.subTest(field=key):
                invalid = dict(context, **{key: value})
                self.assertFalse(verify_identity(invalid))

    def test_backup_is_server_side_generation_pinned_and_create_only(self):
        source_name = STATE_OBJECT
        backup_name = "espacigo/testprod/gcp/manual-state-backups/gcp08-legacy-sql/123-1/default.tfstate"
        metadata = [
            {"name": source_name, "generation": "9001", "size": "100", "crc32c_hash": "abc", "md5_hash": "def"},
            {"name": backup_name, "generation": "9002", "size": "100", "crc32c_hash": "abc", "md5_hash": "def"},
        ]
        commands = []

        def runner(command, **kwargs):
            commands.append(command)
            if command[1:4] == ["storage", "objects", "describe"]:
                kwargs["stdout"].write(json.dumps(metadata.pop(0)).encode())
            else:
                kwargs["stdout"].write(b"copied privately")
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            private = Path(root) / "private"
            private.mkdir(mode=0o700)
            backup_object, _ = _create_and_verify_backup("123", "1", private, runner, 10)
            self.assertEqual(backup_object, backup_name)
            self.assertEqual(stat.S_IMODE(private.stat().st_mode), 0o700)
            for path in private.iterdir():
                self.assertEqual(stat.S_IMODE(path.stat().st_mode), 0o600)
        copy = commands[1]
        self.assertEqual(copy[:4], ["gcloud", "storage", "cp", f"gs://{BUCKET}/{source_name}#9001"])
        self.assertEqual(copy[4], f"gs://{BUCKET}/{backup_name}")
        self.assertIn("--if-generation-match=0", copy)
        self.assertFalse(any("terraform" in " ".join(command) for command in commands))

    def test_metadata_normalizes_actual_gcloud_snake_case_checksum_fields(self):
        metadata = {
            "name": STATE_OBJECT,
            "generation": 9001,
            "size": 100,
            "crc32c_hash": "crc-value",
            "md5_hash": "md5-value",
        }

        def runner(command, **kwargs):
            kwargs["stdout"].write(json.dumps(metadata).encode())
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            result = _object_metadata(f"gs://{BUCKET}/{STATE_OBJECT}", root, "metadata.json", runner, 10)
        self.assertEqual(result, {
            "name": STATE_OBJECT,
            "generation": "9001",
            "size": "100",
            "crc32c": "crc-value",
            "md5Hash": "md5-value",
        })

    def test_remove_executes_only_the_two_targets_with_lock_and_preserves_other_state(self):
        before = make_state(include_extra_instance=True)
        after = make_state(serial=13, include_targets=False, include_extra_instance=True)
        source_name = STATE_OBJECT
        backup_name = "espacigo/testprod/gcp/manual-state-backups/gcp08-legacy-sql/123-1/default.tfstate"
        object_metadata = [
            {"name": source_name, "generation": "9001", "size": "100", "crc32c_hash": "abc", "md5_hash": "def"},
            {"name": backup_name, "generation": "9002", "size": "100", "crc32c_hash": "abc", "md5_hash": "def"},
        ]
        states = [before, before, after]
        commands = []

        def runner(command, **kwargs):
            commands.append(command)
            if command == ["terraform", "workspace", "show"]:
                kwargs["stdout"].write(b"default\n")
            elif command == ["terraform", "state", "pull"]:
                kwargs["stdout"].write(json.dumps(states.pop(0)).encode())
            elif command[1:4] == ["storage", "objects", "describe"]:
                kwargs["stdout"].write(json.dumps(object_metadata.pop(0)).encode())
            else:
                kwargs["stdout"].write(b"private operation output")
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            private = Path(root) / "private"
            backup, summary = _remove(
                Path(root), private, 12, True, "123", "1", runner=runner, timeout=10,
            )
            self.assertEqual(backup, backup_name)
            self.assertEqual(summary["serial"], 13)
            self.assertEqual(stat.S_IMODE(private.stat().st_mode), 0o700)

        state_rm = next(command for command in commands if command[:3] == ["terraform", "state", "rm"])
        self.assertEqual(state_rm[3:5], ["-lock=true", "-lock-timeout=5m"])
        self.assertEqual(tuple(state_rm[5:]), TARGETS)
        self.assertTrue(any(command[0:4] == ["gcloud", "storage", "cp", f"gs://{BUCKET}/{source_name}#9001"]
                            for command in commands))
        self.assertFalse(any(command[0] == "terraform" and "apply" in command for command in commands))

        before_entries = __import__("gcp08_legacy_state_reconcile")._state_addresses(before)
        after_entries = __import__("gcp08_legacy_state_reconcile")._state_addresses(after)
        self.assertIn("google_sql_database_instance.postgres[1]", before_entries)
        self.assertIn("google_sql_database_instance.postgres[1]", after_entries)

    def test_refuses_missing_owner_404_attestation_before_backup_or_state_write(self):
        before = make_state()
        states = [before]
        commands = []

        def runner(command, **kwargs):
            commands.append(command)
            if command == ["terraform", "workspace", "show"]:
                kwargs["stdout"].write(b"default\n")
            elif command == ["terraform", "state", "pull"]:
                kwargs["stdout"].write(json.dumps(states.pop(0)).encode())
            return SimpleNamespace(returncode=0)

        with tempfile.TemporaryDirectory() as root:
            with self.assertRaisesRegex(Exception, "legacy_absence_not_attested"):
                _remove(Path(root), Path(root) / "private", 12, False, "123", "1",
                        runner=runner, timeout=10)
        self.assertFalse(any(command[:3] == ["terraform", "state", "rm"] for command in commands))
        self.assertFalse(any(command[:3] == ["gcloud", "storage", "cp"] for command in commands))

    def test_refuses_absent_state_bindings_and_state_fingerprint_detects_unrelated_change(self):
        summary = analyze_state(make_state(include_targets=False))
        self.assertFalse(summary["eligible"])
        base = make_state()
        changed = make_state()
        changed["resources"][1]["instances"][0]["attributes"]["name"] = "changed-service"
        self.assertNotEqual(state_resource_fingerprint(base), state_resource_fingerprint(changed))


if __name__ == "__main__":
    unittest.main()
