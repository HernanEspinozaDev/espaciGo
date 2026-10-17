#!/usr/bin/env python3
"""Disposable-fixture tests for gated local GCP-03 capture and recovery."""

from __future__ import annotations

import hashlib
import io
import json
import os
import stat
import tempfile
import unittest
from pathlib import Path
from typing import BinaryIO
from datetime import timedelta

import gcp03_origin_capture as capture


FIXTURE_KEY = b"f" * 64 + b"\n"
FIXTURE_DUMP = b"fixture-only-custom-database-archive"
FIXTURE_HEAD = "a" * 40


def private_file(path: Path, data: bytes) -> None:
    path.parent.mkdir(parents=True, mode=0o700, exist_ok=True)
    os.chmod(path.parent, 0o700)
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as stream:
        stream.write(data)


class FakeSource:
    def __init__(self, root: Path):
        self.root = root
        self.evidence = root / "source-evidence"
        self.replay = root / "source-replay"
        self.evidence.mkdir(mode=0o700)
        self.replay.mkdir(mode=0o700)
        self.evidence_file = self.evidence / "fixture.bin"
        private_file(self.evidence_file, b"fixture private evidence bytes")
        self.replay_file = self.replay / "replay.json"
        private_file(self.replay_file, b'{"fixture":true}\n')
        self.evidence_manifest = capture.inventory_tree(self.evidence)
        self.replay_manifest = capture.inventory_tree(self.replay)
        self.stopped: list[str] = []
        self.started: list[str] = []
        self.events: set[str] = set()
        self.fail_dump = False
        self.fail_quiesce = False
        self.inject_symlink_after_stop = False
        self.fail_start_once = False
        self.failed_start = False
        self.preflight_report = {
            "source_fingerprint": "b" * 64,
            "database_container_id": "fixture-database-container",
            "database_volume_ids": ["fixture-protected-volume"],
            "mounted_volume_ids": ["fixture-protected-volume"],
            "estimated_private_bytes": len(FIXTURE_DUMP) + len(FIXTURE_KEY) + 64,
            "container_health": {"fixture-api-container": "healthy", "fixture-ingress-container": "healthy"},
            "quiesce_candidates": [
                {"id": "fixture-api-container", "running": True},
                {"id": "fixture-ingress-container", "running": True},
            ],
            "source_projection": {"evidence_files": self.evidence_manifest, "privacy_replay_files": self.replay_manifest},
        }

    def preflight(self) -> dict:
        return self.preflight_report

    def stop_container(self, container_id: str, timeout: int) -> None:
        self.stopped.append(container_id)
        self.events.add(container_id)
        if self.inject_symlink_after_stop and container_id == "fixture-api-container":
            (self.evidence / "escape-link").symlink_to(self.root / "outside.bin")

    def start_container(self, container_id: str, timeout: int) -> None:
        if self.fail_start_once and not self.failed_start:
            self.failed_start = True
            raise capture.CaptureError("fixture one-time service restart failure")
        self.started.append(container_id)

    def stop_event_confirmed(self, container_id: str, since: str) -> bool:
        return container_id in self.events

    def wait_quiescent(self, timeout: int) -> dict:
        if self.fail_quiesce:
            raise capture.CaptureInterrupted("fixture interruption")
        return {"active_writers": 0, "active_leases": 0, "unknown_clients": [], "fixture_only": True}

    def wait_ready(self, container_id: str, timeout: int, expected_health: str) -> None:
        if container_id not in self.started:
            raise AssertionError("only a service started by the capture may be readied")

    def stream_database_dump(self, destination: BinaryIO, timeout: int) -> dict:
        if self.fail_dump:
            raise capture.CaptureError("fixture dump failure")
        destination.write(FIXTURE_DUMP)
        return {"plain_sha256": hashlib.sha256(FIXTURE_DUMP).hexdigest(), "plain_bytes": len(FIXTURE_DUMP), "format": "fixture custom archive", "global_role_password_hashes_exported": False}

    def metadata_report(self) -> dict:
        return {"migration_match_v1_v61": True, "source_credentials_included": False, "roles": [], "fixture_only": True}

    def source_roots(self) -> dict[str, Path]:
        return {"private-evidence": self.evidence, "privacy-replay": self.replay}

    def source_contract_key(self) -> BinaryIO:
        return io.BytesIO(FIXTURE_KEY)


def approved_release(source: FakeSource, run_id: str) -> dict:
    now = capture.datetime.now(capture.timezone.utc)
    return {
        "format": capture.RELEASE_FORMAT,
        "status": "approved",
        "run_id": run_id,
        "reviewed_head_sha": FIXTURE_HEAD,
        "reviewed_by": "HernanEspinozaDev",
        "source_change_window_authorized": True,
        "all_ingress_and_writer_sources_enumerated": True,
        "exclusive_docker_control": True,
        "external_ingress_and_writer_sources": [],
        "window_start_utc": (now - timedelta(minutes=1)).isoformat(),
        "window_end_utc": (now + timedelta(hours=2)).isoformat(),
        "source": {"database_container_id": "fixture-database-container", "expected_source_fingerprint": source.preflight_report["source_fingerprint"]},
        "quiesce_order": ["fixture-ingress-container", "fixture-api-container"],
    }


class CaptureFixtureTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp = tempfile.TemporaryDirectory(prefix="gcp03-origin-fixture-")
        self.root = Path(self.temp.name)
        os.chmod(self.root, 0o700)
        self.source = FakeSource(self.root)
        self.layout = capture.CaptureLayout(self.root / "snapshots", self.root / "custody", self.root / "journals")
        self.run_id = "gcp03-fixture-001"

    def tearDown(self) -> None:
        self.temp.cleanup()

    def test_capture_encrypts_each_payload_and_keeps_recovery_key_outside_snapshot(self) -> None:
        result = capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        snapshot = self.layout.snapshot_root / self.run_id
        seal = capture.check_encrypted_snapshot(snapshot, capture.load_recovery_key(self.layout.custody_root / self.run_id / "recovery.key"))
        self.assertEqual(result["status"] if "status" in result else "sealed", "sealed")
        self.assertEqual(result["source_services_restored"], 2)
        self.assertEqual(self.source.started, ["fixture-api-container", "fixture-ingress-container"])
        self.assertFalse((snapshot / "recovery.key").exists())
        for name in seal["encrypted_artifacts"]:
            raw = (snapshot / name).read_bytes()
            self.assertNotIn(FIXTURE_DUMP, raw)
            self.assertNotIn(FIXTURE_KEY, raw)
            self.assertEqual(stat.S_IMODE((snapshot / name).stat().st_mode), 0o400)
        key = capture.load_recovery_key(self.layout.custody_root / self.run_id / "recovery.key")
        with tempfile.TemporaryDirectory(prefix="gcp03-key-recovery-fixture-") as output_dir:
            os.chmod(output_dir, 0o700)
            recovered = Path(output_dir) / "recovered-key"
            capture.gpg_decrypt_file(snapshot / "contract-encryption-key.gpg", recovered, capture.derive_passphrase(key, self.run_id, "contract-key"), 30)
            self.assertEqual(recovered.read_bytes(), FIXTURE_KEY)
            self.assertEqual(stat.S_IMODE(recovered.stat().st_mode), 0o600)

    def test_dump_failure_resumes_only_services_stopped_by_run(self) -> None:
        self.source.fail_dump = True
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(set(self.source.started), set(self.source.stopped))
        self.assertNotIn("fixture-database-container", self.source.started)
        self.assertFalse((self.layout.snapshot_root / self.run_id).exists())

    def test_failure_during_private_tree_copy_resumes_source(self) -> None:
        external = self.root / "outside.bin"
        private_file(external, b"fixture outside")
        self.source.inject_symlink_after_stop = True
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(set(self.source.started), set(self.source.stopped))
        self.assertNotIn("fixture-database-container", self.source.started)

    def test_interruption_during_quiesce_uses_finally_to_resume(self) -> None:
        self.source.fail_quiesce = True
        with self.assertRaises(capture.CaptureInterrupted):
            capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(set(self.source.started), set(self.source.stopped))

    def test_unknown_or_database_container_in_stop_list_aborts_before_quiesce(self) -> None:
        release = approved_release(self.source, self.run_id)
        release["quiesce_order"] = ["fixture-api-container"]
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(release, self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.stopped, [])
        release["quiesce_order"] = ["fixture-database-container", "fixture-api-container", "fixture-ingress-container"]
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(release, self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.stopped, [])

    def test_builtin_database_owner_role_is_not_recreated(self) -> None:
        report = {"roles": [{"name": "pg_database_owner", "superuser": False, "create_db": False, "create_role": False, "login": False, "inherit": True, "replication": False, "bypass_rls": False, "connection_limit": -1, "valid_until": None, "config": []}], "memberships": []}
        sql = capture.role_reconciliation_sql(report, {"postgres", "pg_database_owner"})
        self.assertNotIn(b"pg_database_owner", sql)

    def test_preflight_consistency_ignores_expiration_drift_but_rejects_catalog_change(self) -> None:
        before = {"sessions": {"total": 1, "valid": 1}, "tokens": {"total": 2, "usable": 2}, "roles": [{"name": "runtime"}]}
        after = {"sessions": {"total": 1, "valid": 0}, "tokens": {"total": 2, "usable": 1}, "roles": [{"name": "runtime"}]}
        capture.assert_preflight_catalog_stable(before, after)
        after["roles"] = [{"name": "runtime", "login": False}]
        with self.assertRaises(capture.CaptureError):
            capture.assert_preflight_catalog_stable(before, after)

    def test_contract_key_stream_checks_private_file_shape_and_preflight_stat(self) -> None:
        key_path = self.root / "contract-key"
        private_file(key_path, FIXTURE_KEY)
        key_stat = key_path.stat()
        port = object.__new__(capture.DockerSource)
        port._current = {
            "contract_key": key_path,
            "source_projection": {"contract_key_file_stat": {"size_bytes": key_stat.st_size, "mode_octal": f"{stat.S_IMODE(key_stat.st_mode):04o}", "mtime_ns": key_stat.st_mtime_ns, "inode": key_stat.st_ino}},
        }
        with port.source_contract_key() as stream:
            self.assertEqual(stream.read(), FIXTURE_KEY)
        key_path.write_bytes(b"not-a-hex-contract-key")
        with self.assertRaises(capture.CaptureError):
            port.source_contract_key()

    def test_restore_comparison_uses_session_token_totals_not_expiration_clock(self) -> None:
        before = {"sessions": {"total": 3, "valid": 2}, "tokens": {"total": 4, "usable": 3}}
        after = {"sessions": {"total": 3, "valid": 0}, "tokens": {"total": 4, "usable": 0}}
        result = capture.compare_restored_database(before, after)
        self.assertTrue(result["matches"])
        after["tokens"]["total"] = 5
        result = capture.compare_restored_database(before, after)
        self.assertFalse(result["matches"])
        self.assertIn("tokens.total", result["mismatched_fields"])

    def test_release_rejects_external_writer_and_missing_exclusive_control(self) -> None:
        release = approved_release(self.source, self.run_id)
        release["external_ingress_and_writer_sources"] = ["external-scheduler"]
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(release, self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.stopped, [])
        release = approved_release(self.source, self.run_id)
        release["exclusive_docker_control"] = False
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(release, self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.stopped, [])

    def test_short_remaining_window_aborts_before_quiesce(self) -> None:
        release = approved_release(self.source, self.run_id)
        release["window_end_utc"] = (capture.datetime.now(capture.timezone.utc) + timedelta(seconds=30)).isoformat()
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(release, self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.stopped, [])

    def test_release_rejects_non_utc_window(self) -> None:
        release = approved_release(self.source, self.run_id)
        local_zone = capture.timezone(timedelta(hours=-3))
        now = capture.datetime.now(capture.timezone.utc).astimezone(local_zone)
        release["window_start_utc"] = (now - timedelta(minutes=1)).isoformat()
        release["window_end_utc"] = (now + timedelta(hours=2)).isoformat()
        with self.assertRaises(capture.CaptureError):
            capture.release_validate(release, self.run_id, FIXTURE_HEAD)

    def test_path_escape_and_symlink_archive_entries_are_rejected(self) -> None:
        with self.assertRaises(capture.CaptureError):
            capture.require_safe_relative("../outside")
        outside = self.root / "outside"
        private_file(outside, b"fixture")
        (self.source.evidence / "escape").symlink_to(outside)
        with self.assertRaises(capture.CaptureError):
            capture.inventory_tree(self.source.evidence, exact_private_modes=False)

    def test_corrupt_artifact_is_rejected_before_decryption(self) -> None:
        capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        snapshot = self.layout.snapshot_root / self.run_id
        artifact = snapshot / "database.dump.gpg"
        artifact.chmod(0o600)
        raw = bytearray(artifact.read_bytes())
        raw[-1] ^= 1
        artifact.write_bytes(raw)
        key = capture.load_recovery_key(self.layout.custody_root / self.run_id / "recovery.key")
        with self.assertRaises(capture.CaptureError):
            capture.check_encrypted_snapshot(snapshot, key)

    def test_recovery_refuses_a_database_container_in_modified_journal(self) -> None:
        journal = self.layout.journal_root / f"{self.run_id}.journal.json"
        self.layout.journal_root.mkdir(mode=0o700)
        record = {
            "format": f"{capture.FORMAT}.quiesce-journal",
            "run_id": self.run_id,
            "started_at_utc": capture.utc_now(),
            "database_container_id": "fixture-database-container",
            "database_volume_ids": ["fixture-protected-volume"],
            "quiesce_order": ["fixture-database-container"],
            "stop_intent_ids": ["fixture-database-container"],
            "stopped_by_this_run": ["fixture-database-container"],
            "restored_by_this_run": [],
            "initial_health": {},
            "status": "quiescing",
        }
        private_file(journal, capture.canonical_json(record))
        with self.assertRaises(capture.CaptureError):
            capture.recover_journal(journal, self.source)
        self.assertEqual(self.source.started, [])

    def test_recovery_resumes_confirmed_interrupted_stop_and_never_starts_database(self) -> None:
        journal_root = self.layout.journal_root
        journal_root.mkdir(mode=0o700)
        journal = journal_root / f"{self.run_id}.journal.json"
        write = {
            "format": f"{capture.FORMAT}.quiesce-journal",
            "run_id": self.run_id,
            "started_at_utc": capture.utc_now(),
            "database_container_id": "fixture-database-container",
            "database_volume_ids": ["fixture-protected-volume"],
            "quiesce_order": ["fixture-api-container", "fixture-ingress-container"],
            "stop_intent_ids": ["fixture-api-container"],
            "stopped_by_this_run": [],
            "restored_by_this_run": [],
            "initial_health": {"fixture-api-container": "healthy"},
            "status": "quiescing",
        }
        private_file(journal, capture.canonical_json(write))
        self.source.events.add("fixture-api-container")
        result = capture.recover_journal(journal, self.source)
        self.assertEqual(result["status"], "recovered")
        self.assertEqual(self.source.started, ["fixture-api-container"])
        self.assertNotIn("fixture-database-container", self.source.started)

    def test_recovery_verifies_and_finalizes_sealed_partial_after_resume(self) -> None:
        self.source.fail_start_once = True
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        journal = self.layout.journal_root / f"{self.run_id}.journal.json"
        result = capture.recover_journal(journal, self.source, self.layout.snapshot_root)
        final = self.layout.snapshot_root / self.run_id
        self.assertEqual(result["status"], "recovered_and_sealed")
        self.assertTrue(final.is_dir())
        self.assertEqual(stat.S_IMODE(final.stat().st_mode), 0o500)
        self.assertNotIn("fixture-database-container", self.source.started)


if __name__ == "__main__":
    unittest.main(verbosity=2)
