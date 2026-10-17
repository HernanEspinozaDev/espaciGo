#!/usr/bin/env python3
"""Disposable-fixture tests for gated local GCP-03 capture and recovery."""

from __future__ import annotations

import hashlib
import io
import json
import os
import stat
import subprocess
import shutil
import sys
import tempfile
import uuid
import unittest
from unittest.mock import patch
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
        self.states = {"fixture-api-container": "running", "fixture-ingress-container": "running"}
        self.events_unknown = False
        self.state_unknown = False
        self.fail_stop_after_stop = False
        self.fail_dump = False
        self.fail_quiesce = False
        self.inject_symlink_after_stop = False
        self.fail_start_once = False
        self.failed_start = False
        self.validated_restore_targets: list[dict[str, object]] = []
        def provenance(name: str) -> dict[str, object]:
            return {"status": "available", "image_id": "sha256:" + hashlib.sha256(name.encode()).hexdigest(), "repo_digests": ["fixture/" + name + "@sha256:" + hashlib.sha256((name + "-manifest").encode()).hexdigest()]}
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
            "candidate_descriptors": [
                {"id": "fixture-api-container", "published_ports": {}, "image_provenance": provenance("api")},
                {"id": "fixture-ingress-container", "published_ports": {"80/tcp": [{"HostPort": "8080"}]}, "image_provenance": provenance("ingress")},
            ],
            "source_projection": {
                "database_container": {"id": "fixture-database-container", "configured_image": "fixture/database:dev", "image_provenance": provenance("database")},
                "backend_container": {"id": "fixture-api-container", "image_provenance": provenance("api")},
                "evidence_files": self.evidence_manifest,
                "privacy_replay_files": self.replay_manifest,
            },
        }

    def preflight(self) -> dict:
        return self.preflight_report

    def validate_restore_target(self, restore_target: dict) -> None:
        self.validated_restore_targets.append(restore_target)
        return None

    def stop_container(self, container_id: str, timeout: int) -> None:
        self.stopped.append(container_id)
        self.events.add(container_id)
        self.states[container_id] = "stopped"
        if self.inject_symlink_after_stop and container_id == "fixture-api-container":
            (self.evidence / "escape-link").symlink_to(self.root / "outside.bin")
        if self.fail_stop_after_stop:
            self.fail_stop_after_stop = False
            raise capture.CaptureError("fixture stop acknowledgement lost")

    def start_container(self, container_id: str, timeout: int) -> None:
        if self.fail_start_once and not self.failed_start:
            self.failed_start = True
            raise capture.CaptureError("fixture one-time service restart failure")
        self.started.append(container_id)
        self.states[container_id] = "running"

    def stop_event_status(self, container_id: str, since: str) -> str:
        return "unknown" if self.events_unknown else "confirmed" if container_id in self.events else "absent"

    def container_state(self, container_id: str) -> str:
        return "unknown" if self.state_unknown else self.states.get(container_id, "unknown")

    def wait_quiescent(self, timeout: int) -> dict:
        if self.fail_quiesce:
            raise capture.CaptureInterrupted("fixture interruption")
        return {"active_writers": 0, "active_leases": 0, "unknown_clients": [], "fixture_only": True}

    def wait_ready(self, container_id: str, timeout: int, expected_health: str) -> None:
        if self.states.get(container_id) != "running":
            raise AssertionError("service was not ready after recovery")

    def stream_database_dump(self, destination: BinaryIO, timeout: int) -> dict:
        if self.fail_dump:
            raise capture.CaptureError("fixture dump failure")
        destination.write(FIXTURE_DUMP)
        return {"plain_sha256": hashlib.sha256(FIXTURE_DUMP).hexdigest(), "plain_bytes": len(FIXTURE_DUMP), "format": "fixture custom archive", "global_role_password_hashes_exported": False}

    def metadata_report(self, timeout: int = 120) -> dict:
        return {"migration_match_v1_v61": True, "source_credentials_included": False, "roles": [], "fixture_only": True}

    def source_roots(self) -> dict[str, Path]:
        return {"private-evidence": self.evidence, "privacy-replay": self.replay}

    def source_contract_key(self) -> BinaryIO:
        return io.BytesIO(FIXTURE_KEY)


class LocalPostgresFixtureSource(FakeSource):
    """Uses only a disposable network-none local PostgreSQL container as dump input."""
    def __init__(self, root: Path, container_id: str, image_id: str, repo: Path):
        source_root = root / "local-postgres-source"
        source_root.mkdir(mode=0o700)
        super().__init__(source_root)
        self.container_id = container_id
        self.repo = repo
        self.preflight_report.update({
            "database_container_id": container_id,
            "database_volume_ids": ["fixture-tmpfs-only"],
            "mounted_volume_ids": ["fixture-tmpfs-only"],
            "database_descriptor": {"id": container_id, "image_id": image_id, "image_provenance": capture.inspect_image_reference(image_id, missing_ok=True)},
            "source_projection": self.preflight_report["source_projection"] | {"database_admin_role": "postgres"},
        })

    def stream_database_dump(self, destination: BinaryIO, timeout: int) -> dict:
        process = subprocess.Popen(["docker", "exec", "--user", "postgres", self.container_id, "pg_dump", "--no-password", "--format=custom", "--host=127.0.0.1", "--username=postgres", "--dbname=gcp03_restore_fixture", "--create"], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
        digest = hashlib.sha256()
        size = 0
        assert process.stdout is not None
        while chunk := process.stdout.read(1024 * 1024):
            destination.write(chunk)
            digest.update(chunk)
            size += len(chunk)
        process.stdout.close()
        if process.wait(timeout=timeout) != 0 or not size:
            raise capture.CaptureError("fixture-only PostgreSQL dump failed")
        return {"plain_sha256": digest.hexdigest(), "plain_bytes": size, "format": "PostgreSQL custom archive", "global_role_password_hashes_exported": False}

    def metadata_report(self, timeout: int = 120) -> dict:
        output = subprocess.run(["docker", "exec", "-i", "--user", "postgres", self.container_id, "psql", "-XqAt", "-v", "ON_ERROR_STOP=1", "--username=postgres", "--dbname=gcp03_restore_fixture", "-f", "-"], input=capture.SQL_CAPTURE.encode(), capture_output=True, timeout=timeout, check=False)
        if output.returncode != 0:
            raise capture.CaptureError("fixture-only PostgreSQL inventory failed")
        report = capture.strict_json(output.stdout.strip())
        report["migration_match_v1_v61"] = True
        report["source_credentials_included"] = False
        return report


def approved_release(source: FakeSource, run_id: str) -> dict:
    now = capture.datetime.now(capture.timezone.utc)
    restore_ref = "fixture.database@sha256:" + "d" * 64
    _, image_provenance_complete, image_provenance_gaps = capture.source_image_provenance_review(source.preflight_report)
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
        "source": {
            "database_container_id": source.preflight_report["database_container_id"],
            "expected_source_fingerprint": source.preflight_report["source_fingerprint"],
            "image_provenance_complete": image_provenance_complete,
            "image_provenance_gaps": image_provenance_gaps,
            "image_provenance_gap_accepted": not image_provenance_complete,
        },
        "restore_target": {"image_ref": restore_ref, "image_id": "sha256:" + "e" * 64, "repo_digests": [restore_ref]},
        "timeout_profile": capture.DEFAULT_TIMEOUT_PROFILE.copy(),
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

    def test_missing_source_image_is_recorded_and_restore_digest_is_separate(self) -> None:
        missing_id = "sha256:" + "1" * 64
        missing = subprocess.CompletedProcess([], 1, b"", b"Error response from daemon: No such image: " + missing_id.encode())
        target_ref = "local/postgis@sha256:" + "2" * 64
        selected = subprocess.CompletedProcess([], 0, json.dumps([{"Id": "sha256:" + "3" * 64, "RepoDigests": [target_ref]}]).encode(), b"")
        with patch.object(capture.subprocess, "run", side_effect=[missing, selected]):
            provenance = capture.inspect_image_reference(missing_id, missing_ok=True)
            target = capture.validate_restore_image_reference(target_ref)
        self.assertEqual(provenance["status"], "missing_locally")
        self.assertEqual(provenance["reason_code"], "image_not_found")
        self.assertEqual(target["image_ref"], target_ref)
        self.assertNotEqual(target["image_id"], missing_id)

    def test_non_missing_docker_image_error_is_not_downgraded_to_missing(self) -> None:
        failure = subprocess.CompletedProcess([], 1, b"", b"Cannot connect to the Docker daemon")
        with patch.object(capture.subprocess, "run", return_value=failure):
            with self.assertRaisesRegex(capture.CaptureError, "provenance inspection failed"):
                capture.inspect_image_reference("sha256:" + "1" * 64, missing_ok=True)

    def test_container_image_provenance_records_configured_tag_digest_mismatch(self) -> None:
        image_id = "sha256:" + "1" * 64
        tag_id = "sha256:" + "2" * 64
        missing = subprocess.CompletedProcess([], 1, b"", b"Error response from daemon: No such image: " + image_id.encode())
        tag = subprocess.CompletedProcess([], 0, json.dumps([{"Id": tag_id, "RepoDigests": ["fixture/database@" + tag_id]}]).encode(), b"")
        with patch.object(capture.subprocess, "run", side_effect=[missing, tag]):
            provenance = capture.container_image_provenance({"image_id": image_id, "configured_image": "fixture/database:dev"})
        self.assertEqual(provenance["status"], "missing_locally")
        self.assertFalse(provenance["configured_reference_matches_runtime_image"])
        self.assertEqual(provenance["configured_reference_provenance"]["image_id"], tag_id)

    def test_ipv4_mapped_postgres_client_address_correlates_to_docker_ipv4(self) -> None:
        self.assertEqual(capture.normalize_ip_address("::ffff:192.0.2.17"), "192.0.2.17")
        self.assertEqual(capture.normalize_ip_address("::ffff:192.0.2.17/128"), "192.0.2.17")
        self.assertEqual(capture.normalize_ip_address("192.0.2.17"), "192.0.2.17")
        self.assertEqual(capture.normalize_ip_address("172.19.0.3/32"), "172.19.0.3")
        self.assertIsNone(capture.normalize_ip_address("not-an-address"))

    def test_masked_statistics_are_reported_as_insufficient_observability(self) -> None:
        clients = [{"address": None, "database": "espacigo_local", "user": "espacigo_runtime", "application": "api"}]
        with self.assertRaisesRegex(capture.CaptureError, "insufficient PostgreSQL statistics visibility"):
            capture.correlate_backend_sessions(
                clients, {"172.19.0.3"}, "espacigo_local", "espacigo_runtime",
                {"all_client_session_details_authorized": False},
            )

    def test_visible_but_unaddressed_session_is_not_classified_as_foreign(self) -> None:
        clients = [{"address": None, "database": "espacigo_local", "user": "espacigo_runtime", "application": "api"}]
        observability = {"all_client_session_details_authorized": True}
        with self.assertRaisesRegex(capture.CaptureError, "transport cannot be correlated"):
            capture.correlate_backend_sessions(clients, {"172.19.0.3"}, "espacigo_local", "espacigo_runtime", observability)
        with self.assertRaisesRegex(capture.CaptureError, "transport cannot be classified"):
            capture.validate_client_inventory(clients, {"172.19.0.3"}, observability)

    def test_visible_runtime_client_at_unknown_address_is_distinct_from_masked_stats(self) -> None:
        clients = [{"address": "172.19.0.99/32", "database": "espacigo_local", "user": "espacigo_runtime", "application": "api"}]
        observability = {"all_client_session_details_authorized": True}
        with self.assertRaisesRegex(capture.CaptureError, "none originate from the inspected backend IP"):
            capture.correlate_backend_sessions(clients, {"172.19.0.3"}, "espacigo_local", "espacigo_runtime", observability)
        with self.assertRaisesRegex(capture.CaptureError, "outside the enumerated source candidates"):
            capture.validate_client_inventory(clients, {"172.19.0.2", "172.19.0.3"}, observability)

    def test_backend_session_correlation_accepts_postgresql_inet_prefix(self) -> None:
        clients = [{"address": "172.19.0.3/32", "database": "espacigo_local", "user": "espacigo_runtime", "application": "api"}]
        matches = capture.correlate_backend_sessions(
            clients, {"172.19.0.3"}, "espacigo_local", "espacigo_runtime",
            {"all_client_session_details_authorized": True},
        )
        self.assertEqual(len(matches), 1)

    def test_database_runtime_binary_version_is_taken_from_running_container(self) -> None:
        with patch.object(capture, "run_quiet", return_value=b"postgres (PostgreSQL) 18.6 (Debian 18.6-1.pgdg13+2)\n"):
            version = capture.database_runtime_binary_version("fixture-db-container")
        self.assertIn("PostgreSQL) 18.6", version)
        with patch.object(capture, "run_quiet", return_value=b"unexpected command output\n"):
            with self.assertRaisesRegex(capture.CaptureError, "unrecognized version"):
                capture.database_runtime_binary_version("fixture-db-container")

    def test_release_candidate_does_not_claim_unknown_external_ingress_is_absent(self) -> None:
        candidate = capture.build_release_candidate(
            self.source.preflight_report,
            self.run_id,
            FIXTURE_HEAD,
            approved_release(self.source, self.run_id)["restore_target"],
            restore_target_digest_verified=True,
        )
        self.assertFalse(candidate["all_ingress_and_writer_sources_enumerated"])
        self.assertFalse(candidate["exclusive_docker_control"])
        self.assertIsNone(candidate["external_ingress_and_writer_sources"])
        self.assertTrue(candidate["restore_target"]["image_ref"].endswith("d" * 64))
        self.assertEqual(candidate["capture_budget_seconds"], capture.capture_budget_seconds(2))

    def test_unapproved_candidate_preserves_missing_source_image_and_tag_mismatch(self) -> None:
        db = self.source.preflight_report["source_projection"]["database_container"]
        original = db["image_provenance"]
        db["image_provenance"] = {
            "status": "missing_locally",
            "reason_code": "image_not_found",
            "image_id": "sha256:" + "1" * 64,
            "repo_digests": [],
            "configured_reference_provenance": {"status": "available", "image_id": "sha256:" + "2" * 64, "repo_digests": ["fixture/database@sha256:" + "2" * 64]},
            "configured_reference_matches_runtime_image": False,
        }
        candidate = capture.build_release_candidate(
            self.source.preflight_report,
            self.run_id,
            FIXTURE_HEAD,
            approved_release(self.source, self.run_id)["restore_target"],
            restore_target_digest_verified=True,
        )
        self.assertEqual(candidate["status"], "not_approved")
        self.assertFalse(candidate["source_image_provenance_complete"])
        self.assertFalse(candidate["source"]["image_provenance_gap_accepted"])
        self.assertEqual(candidate["source"]["container_descriptors"][0]["configured_image"], "fixture/database:dev")
        self.assertIn("runtime_image_not_available_locally", candidate["source_image_provenance_gaps"][0]["reason_codes"])
        self.assertIn("configured_reference_resolves_to_a_different_image_id", candidate["source_image_provenance_gaps"][0]["reason_codes"])
        self.assertFalse(candidate["source_change_window_authorized"])
        self.assertFalse(candidate["all_ingress_and_writer_sources_enumerated"])
        self.assertFalse(candidate["exclusive_docker_control"])
        self.assertIsNone(candidate["external_ingress_and_writer_sources"])
        self.assertTrue(candidate["restore_target_review"]["local_digest_reference_validated"])

    def test_unapproved_candidate_preserves_locally_available_image_without_repo_digests(self) -> None:
        db = self.source.preflight_report["source_projection"]["database_container"]
        original = db["image_provenance"]
        db["image_provenance"] = {"status": "available", "image_id": original["image_id"], "repo_digests": []}
        try:
            candidate = capture.build_release_candidate(
                self.source.preflight_report,
                self.run_id,
                FIXTURE_HEAD,
                approved_release(self.source, self.run_id)["restore_target"],
                restore_target_digest_verified=True,
            )
        finally:
            db["image_provenance"] = original
        self.assertFalse(candidate["source_image_provenance_complete"])
        self.assertIn("runtime_image_has_no_verified_repo_digest", candidate["source_image_provenance_gaps"][0]["reason_codes"])

    def test_release_requires_explicit_acknowledgement_of_source_image_gap(self) -> None:
        release = approved_release(self.source, self.run_id)
        release["source"]["image_provenance_complete"] = False
        release["source"]["image_provenance_gaps"] = [{"reason_codes": ["runtime_image_not_available_locally"]}]
        with self.assertRaisesRegex(capture.CaptureError, "explicitly review and acknowledge"):
            capture.release_validate(release, self.run_id, FIXTURE_HEAD)

    def test_capture_rejects_unapproved_candidate_before_validating_or_stopping_services(self) -> None:
        candidate = capture.build_release_candidate(
            self.source.preflight_report,
            self.run_id,
            FIXTURE_HEAD,
            approved_release(self.source, self.run_id)["restore_target"],
            restore_target_digest_verified=True,
        )
        with self.assertRaisesRegex(capture.CaptureError, "requires an approved"):
            capture.CaptureCoordinator(self.source, self.layout).capture(candidate, self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.validated_restore_targets, [])
        self.assertEqual(self.source.stopped, [])
        self.assertFalse((self.layout.snapshot_root / self.run_id).exists())

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

    def test_role_reconciliation_preserves_unbounded_valid_until_as_null(self) -> None:
        role = {"name": "fixture_runtime", "superuser": False, "create_db": False, "create_role": False,
                "login": True, "inherit": False, "replication": False, "bypass_rls": False,
                "connection_limit": -1, "valid_until": None, "config": []}
        sql = capture.role_reconciliation_sql({"roles": [role], "memberships": []}, set())
        self.assertNotIn(b"VALID UNTIL", sql)
        role["valid_until"] = "2030-01-01T00:00:00+00:00"
        sql = capture.role_reconciliation_sql({"roles": [role], "memberships": []}, set())
        self.assertIn(b"VALID UNTIL", sql)

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

    def test_timeout_profile_is_bounded_and_computes_explicit_window_envelope(self) -> None:
        profile = capture.DEFAULT_TIMEOUT_PROFILE.copy()
        self.assertEqual(capture.capture_budget_seconds(2, profile), 2460 + 2 * 410)
        profile["pg_dump"] = 59
        with self.assertRaises(capture.CaptureError):
            capture.validate_timeout_profile(profile)
        profile = capture.DEFAULT_TIMEOUT_PROFILE.copy()
        profile["pg_dump"] = 3601
        with self.assertRaises(capture.CaptureError):
            capture.validate_timeout_profile(profile)
        profile = capture.DEFAULT_TIMEOUT_PROFILE.copy()
        profile["margin"] = True
        with self.assertRaises(capture.CaptureError):
            capture.validate_timeout_profile(profile)

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
        self.source.states["fixture-api-container"] = "stopped"
        result = capture.recover_journal(journal, self.source)
        self.assertEqual(result["status"], "recovered")
        self.assertEqual(self.source.started, ["fixture-api-container"])
        self.assertNotIn("fixture-database-container", self.source.started)

    def test_stop_ack_loss_uses_event_and_restores_service(self) -> None:
        self.source.fail_stop_after_stop = True
        with self.assertRaises(capture.CaptureError):
            capture.CaptureCoordinator(self.source, self.layout).capture(approved_release(self.source, self.run_id), self.run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        self.assertEqual(self.source.started, ["fixture-ingress-container"])
        record = capture.strict_json((self.layout.journal_root / f"{self.run_id}.journal.json").read_bytes())
        self.assertEqual(record["unresolved_stop_intents"], [])
        self.assertEqual(record["status"], "failed_source_resumed")

    def test_recover_key_cli_accepts_read_only_seal_and_rejects_replay_or_bad_ciphertext(self) -> None:
        repo = Path(capture.__file__).resolve().parent.parent
        run_id = f"gcp03-cli-{uuid.uuid4().hex[:12]}"
        snapshot_root = repo / ".local/testing/gcp03-origin-snapshots"
        custody_root = Path.home() / ".local/share/espacigo/gcp03-origin-custody"
        snapshot_root.mkdir(parents=True, mode=0o700, exist_ok=True)
        custody_root.mkdir(parents=True, mode=0o700, exist_ok=True)
        os.chmod(snapshot_root, 0o700)
        os.chmod(custody_root, 0o700)
        layout = capture.CaptureLayout(snapshot_root, custody_root, repo / ".local/testing/gcp03-origin-journals")
        capture.CaptureCoordinator(self.source, layout).capture(approved_release(self.source, run_id), run_id, FIXTURE_HEAD, free_bytes=1024 * 1024 * 1024)
        snapshot = snapshot_root / run_id
        key = custody_root / run_id / "recovery.key"
        output_dir = repo / ".local/testing" / f"{run_id}-output"
        output_dir.mkdir(mode=0o700)
        output = output_dir / "contract-key"
        cli = [sys.executable, str(repo / "scripts/gcp03_origin_capture.py"), "recover-key", "--snapshot", str(snapshot.relative_to(repo)), "--recovery-key", str(key), "--output", str(output.relative_to(repo))]
        try:
            completed = subprocess.run(cli, cwd=repo, capture_output=True, text=True, check=False)
            self.assertEqual(completed.returncode, 0, completed.stderr)
            self.assertEqual(output.read_bytes(), FIXTURE_KEY)
            self.assertEqual(stat.S_IMODE(output.stat().st_mode), 0o600)
            replay = subprocess.run(cli, cwd=repo, capture_output=True, text=True, check=False)
            self.assertEqual(replay.returncode, 2)
            self.assertEqual(output.read_bytes(), FIXTURE_KEY)

            os.chmod(snapshot, 0o700)
            artifact_path = snapshot / "contract-encryption-key.gpg"
            os.chmod(artifact_path, 0o600)
            data = bytearray(artifact_path.read_bytes())
            data[-1] ^= 1
            artifact_path.write_bytes(data)
            seal_path = snapshot / "SEALED.json"
            os.chmod(seal_path, 0o600)
            seal = capture.strict_json(seal_path.read_bytes())
            descriptor = seal["encrypted_artifacts"][artifact_path.name]
            descriptor["cipher_sha256"] = capture.sha256_file(artifact_path)[0]
            descriptor["cipher_bytes"] = artifact_path.stat().st_size
            seal_path.write_bytes(capture.canonical_json(seal))
            checksum = snapshot / "SEALED.sha256"
            os.chmod(checksum, 0o600)
            checksum.write_text(capture.sha256_file(seal_path)[0] + "  SEALED.json\n", encoding="ascii")
            os.chmod(seal_path, 0o400)
            os.chmod(checksum, 0o400)
            os.chmod(snapshot, 0o500)
            corrupt_output = output_dir / "corrupt-key"
            bad_cli = [sys.executable, str(repo / "scripts/gcp03_origin_capture.py"), "recover-key", "--snapshot", str(snapshot.relative_to(repo)), "--recovery-key", str(key), "--output", str(corrupt_output.relative_to(repo))]
            bad = subprocess.run(bad_cli, cwd=repo, capture_output=True, text=True, check=False)
            self.assertEqual(bad.returncode, 2)
            self.assertFalse(corrupt_output.exists())
        finally:
            shutil.rmtree(snapshot, ignore_errors=True)
            shutil.rmtree(custody_root / run_id, ignore_errors=True)
            shutil.rmtree(output_dir, ignore_errors=True)

    @unittest.skipUnless(shutil.which("docker"), "Docker CLI is unavailable")
    def test_restore_copy_cli_end_to_end_with_disposable_postgres_fixture(self) -> None:
        repo = Path(capture.__file__).resolve().parent.parent
        image_ref = "espacigo-local-database:dev"
        image_result = subprocess.run(["docker", "image", "inspect", image_ref, "--format", "{{.Id}}"], capture_output=True, text=True, check=False)
        if image_result.returncode != 0:
            self.skipTest("local PostgreSQL fixture image is not cached")
        image_id = image_result.stdout.strip()
        digest_result = subprocess.run(["docker", "image", "inspect", image_ref, "--format", "{{index .RepoDigests 0}}"], capture_output=True, text=True, check=False)
        if digest_result.returncode != 0 or not digest_result.stdout.strip():
            self.skipTest("local PostgreSQL fixture image has no immutable repository digest")
        restore_image_ref = digest_result.stdout.strip()
        restore_target = capture.validate_restore_image_reference(restore_image_ref)
        run_id = f"gcp03-restore-{uuid.uuid4().hex[:12]}"
        container_name = f"g03fixture-{uuid.uuid4().hex[:10]}"
        snapshot_root = repo / ".local/testing/gcp03-origin-snapshots"
        custody_root = Path.home() / ".local/share/espacigo/gcp03-origin-custody"
        result_root = repo / ".local/testing/gcp03-origin-restores"
        snapshot_root.mkdir(parents=True, mode=0o700, exist_ok=True)
        custody_root.mkdir(parents=True, mode=0o700, exist_ok=True)
        os.chmod(snapshot_root, 0o700)
        os.chmod(custody_root, 0o700)
        container_id = ""
        auth_dir = Path(tempfile.mkdtemp(prefix="gcp03-source-fixture-auth-"))
        os.chmod(auth_dir, 0o700)
        password_path = auth_dir / "admin-password"
        runtime_path = auth_dir / "runtime-password"
        private_file(password_path, b"fixture-only-postgres-password\n")
        private_file(runtime_path, b"a" * 96 + b"\n")
        try:
            started = subprocess.run([
                "docker", "run", "--detach", "--name", container_name, "--network", "none", "--read-only", "--restart", "no",
                "--tmpfs", "/var/lib/postgresql:rw,nosuid,nodev,noexec,size=1073741824", "--tmpfs", "/tmp:rw,nosuid,nodev,size=268435456", "--tmpfs", "/run:rw,nosuid,nodev,size=67108864",
                "--mount", f"type=bind,source={password_path},target=/run/secrets/db_admin_password,readonly",
                "--mount", f"type=bind,source={runtime_path},target=/run/secrets/runtime_password,readonly",
                "--env", "POSTGRES_DB=restore_bootstrap", "--env", "POSTGRES_USER=postgres", "--env", "POSTGRES_PASSWORD_FILE=/run/secrets/db_admin_password", image_id,
            ], capture_output=True, text=True, check=False)
            self.assertEqual(started.returncode, 0, started.stderr)
            container_id = started.stdout.strip()
            capture.wait_local_pg(container_id, 240)
            ddl = """
CREATE DATABASE gcp03_restore_fixture;
"""
            capture._docker_exec_sql(container_id, "restore_bootstrap", ddl.encode())
            migration_rows = []
            for version, detail in capture.migration_files_from_repo(repo).items():
                name = detail["name"].replace("'", "''")
                checksum = detail["sha256"]
                migration_rows.append(f"({version},'{name}','{checksum}','{'0' * 64}')")
            schema = """
CREATE TABLE public.schema_migrations(version bigint PRIMARY KEY,name text NOT NULL UNIQUE,checksum text NOT NULL,runner_checksum text NOT NULL,applied_at timestamptz NOT NULL DEFAULT now());
CREATE TABLE public.usuario(id uuid);
CREATE TABLE public.rol_usuario(id uuid);
CREATE TABLE public.espacio(id uuid);
CREATE TABLE public.reserva_ensayo_local(id uuid);
CREATE TABLE public.ocupacion(id bigint);
CREATE TABLE public.sesion(id uuid, revocada_en timestamptz, expira_en timestamptz, creada_en timestamptz);
CREATE TABLE public.token_accion(id uuid, consumido_en timestamptz, invalidado_en timestamptz, expira_en timestamptz, creado_en timestamptz);
CREATE TABLE public.outbox_evento_local(id uuid, entregada_en timestamptz, cancelada_en timestamptz, fallo_terminal_en timestamptz, lease_hasta timestamptz);
CREATE TABLE public.aviso_local(id uuid, ciclo integer, estado text, cancelada_en timestamptz, creada_en timestamptz, retirar_en timestamptz, lease_hasta timestamptz, codigo_error text);
CREATE TABLE public.reserva_pago_ensayo_operacion(id uuid);
CREATE TABLE public.reserva_pago_evento_ensayo(id uuid);
CREATE TABLE public.contrato_ensayo_local(id uuid);
CREATE TABLE public.documento_privado_sintetico_local(id uuid);
CREATE TABLE public.verificacion(id uuid);
CREATE TABLE public.aviso_local_ciclo(aviso_id uuid,ciclo integer,estado text,finalizada_en timestamptz,iniciada_en timestamptz,codigo_resultado text);
INSERT INTO public.schema_migrations(version,name,checksum,runner_checksum) VALUES
""" + ",\n".join(migration_rows) + ";\n" + """
INSERT INTO public.sesion VALUES ('00000000-0000-0000-0000-000000000001',NULL,now()+interval '1 day',now());
INSERT INTO public.token_accion VALUES ('00000000-0000-0000-0000-000000000002',NULL,NULL,now()+interval '1 day',now());
INSERT INTO public.aviso_local VALUES ('00000000-0000-0000-0000-000000000003',1,'pendiente',NULL,now(),NULL,NULL,NULL);
INSERT INTO public.aviso_local_ciclo VALUES ('00000000-0000-0000-0000-000000000003',1,'pendiente',NULL,now(),NULL);
INSERT INTO public.outbox_evento_local VALUES ('00000000-0000-0000-0000-000000000004',NULL,NULL,NULL,now()+interval '1 minute');
"""
            schema_result = subprocess.run(["docker", "exec", "--user", "postgres", "-i", container_id, "psql", "-XqAt", "-v", "ON_ERROR_STOP=1", "--username=postgres", "--dbname=gcp03_restore_fixture", "-f", "-"], input=schema.encode(), capture_output=True, timeout=120, check=False)
            self.assertEqual(schema_result.returncode, 0, schema_result.stderr.decode(errors="replace"))
            local_source = LocalPostgresFixtureSource(self.root, container_id, image_id, repo)
            local_source.preflight_report["source_projection"]["database_admin_role"] = "postgres"
            layout = capture.CaptureLayout(snapshot_root, custody_root, repo / ".local/testing/gcp03-origin-journals")
            release = approved_release(local_source, run_id)
            release["restore_target"] = restore_target
            capture.CaptureCoordinator(local_source, layout).capture(release, run_id, FIXTURE_HEAD, free_bytes=2 * 1024 * 1024 * 1024)
            key = custody_root / run_id / "recovery.key"
            # Exercise the implementation once in-process first so fixture failures
            # retain a traceback; the CLI pass below verifies the public command path.
            diagnostic = capture.restore_validate_copy(repo, snapshot_root / run_id, key, run_id, restore_image_ref)
            self.assertEqual(diagnostic["status"], "isolated_copy_restored_and_quarantined")
            shutil.rmtree(result_root / run_id, ignore_errors=True)
            command = [sys.executable, str(repo / "scripts/gcp03_origin_capture.py"), "restore-copy", "--snapshot", f".local/testing/gcp03-origin-snapshots/{run_id}", "--recovery-key", str(key), "--run-id", run_id, "--restore-image-ref", restore_image_ref]
            result = subprocess.run(command, cwd=repo, capture_output=True, text=True, timeout=900, check=False)
            self.assertEqual(result.returncode, 0, result.stderr)
            summary = json.loads(result.stdout)
            self.assertEqual(summary["status"], "isolated_copy_restored_and_quarantined")
            self.assertEqual(summary["quarantined_notices"], 1)
            evidence = result_root / run_id / "restore-validation-private.json"
            validation = capture.strict_json(evidence.read_bytes())
            self.assertTrue(validation["database_and_acl_comparison"]["matches"])
            self.assertFalse(validation["source_database_mutated"])
            self.assertEqual(validation["copy_container_network"], "none")
            self.assertTrue(validation["restore_runtime_compatible"])
        finally:
            subprocess.run(["docker", "rm", "--force", container_name], capture_output=True, check=False)
            shutil.rmtree(snapshot_root / run_id, ignore_errors=True)
            shutil.rmtree(custody_root / run_id, ignore_errors=True)
            shutil.rmtree(result_root / run_id, ignore_errors=True)
            shutil.rmtree(auth_dir, ignore_errors=True)

    def test_unknown_event_and_stopped_state_remains_incomplete_without_start(self) -> None:
        journal = self.layout.journal_root / f"{self.run_id}.journal.json"
        self.layout.journal_root.mkdir(mode=0o700)
        record = {"format": f"{capture.FORMAT}.quiesce-journal", "run_id": self.run_id,
                  "started_at_utc": capture.utc_now(), "database_container_id": "fixture-database-container",
                  "database_volume_ids": ["fixture-protected-volume"], "quiesce_order": ["fixture-api-container"],
                  "stop_intent_ids": ["fixture-api-container"], "stopped_by_this_run": [], "restored_by_this_run": [],
                  "initial_health": {}, "status": "quiescing"}
        private_file(journal, capture.canonical_json(record))
        self.source.states["fixture-api-container"] = "stopped"
        self.source.events_unknown = True
        with self.assertRaises(capture.CaptureError):
            capture.recover_journal(journal, self.source)
        self.assertEqual(self.source.started, [])
        saved = capture.strict_json(journal.read_bytes())
        self.assertEqual(saved["status"], "resume_incomplete")
        self.assertEqual(saved["unresolved_stop_intents"], ["fixture-api-container"])

    def test_absent_stop_event_and_stopped_unowned_service_is_not_claimed_recovered(self) -> None:
        journal = self.layout.journal_root / f"{self.run_id}.journal.json"
        self.layout.journal_root.mkdir(mode=0o700)
        record = {"format": f"{capture.FORMAT}.quiesce-journal", "run_id": self.run_id,
                  "started_at_utc": capture.utc_now(), "database_container_id": "fixture-database-container",
                  "database_volume_ids": ["fixture-protected-volume"], "quiesce_order": ["fixture-api-container"],
                  "stop_intent_ids": ["fixture-api-container"], "stopped_by_this_run": [], "restored_by_this_run": [],
                  "initial_health": {}, "status": "quiescing"}
        private_file(journal, capture.canonical_json(record))
        self.source.states["fixture-api-container"] = "stopped"
        with self.assertRaises(capture.CaptureError):
            capture.recover_journal(journal, self.source)
        self.assertEqual(self.source.started, [])
        self.assertEqual(capture.strict_json(journal.read_bytes())["status"], "resume_incomplete")

    def test_running_service_with_unknown_event_is_checked_without_restart(self) -> None:
        journal = self.layout.journal_root / f"{self.run_id}.journal.json"
        self.layout.journal_root.mkdir(mode=0o700)
        record = {"format": f"{capture.FORMAT}.quiesce-journal", "run_id": self.run_id,
                  "started_at_utc": capture.utc_now(), "database_container_id": "fixture-database-container",
                  "database_volume_ids": ["fixture-protected-volume"], "quiesce_order": ["fixture-api-container"],
                  "stop_intent_ids": ["fixture-api-container"], "stopped_by_this_run": [], "restored_by_this_run": [],
                  "initial_health": {}, "status": "quiescing"}
        private_file(journal, capture.canonical_json(record))
        self.source.events_unknown = True
        result = capture.recover_journal(journal, self.source)
        self.assertEqual(result["status"], "recovered")
        self.assertEqual(self.source.started, [])

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
