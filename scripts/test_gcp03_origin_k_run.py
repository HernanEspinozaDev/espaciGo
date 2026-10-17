#!/usr/bin/env python3
"""Offline contract checks for the one-shot K_run version helper."""
from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path
import stat
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import gcp03_origin_cloud_import_executor as importer
import gcp03_origin_import as transport
import gcp03_origin_k_run as k_run
import requests


class Response:
    def __init__(self, status_code: int, body: dict):
        self.status_code = status_code
        self._body = body

    def json(self):
        return self._body


class FakeSession:
    def __init__(self, versions=None, response=None, post_exception=None, after_get=None):
        self.versions = versions or []
        self.response = response
        self.post_exception = post_exception
        self.after_get = after_get
        self.posts = []

    def get(self, url, **kwargs):
        self.get_call = (url, kwargs)
        if self.after_get:
            self.after_get()
        return Response(200, {"versions": self.versions})

    def post(self, url, **kwargs):
        self.posts.append((url, kwargs))
        if self.post_exception:
            raise self.post_exception
        body = self.response or {
            "name": f"projects/{k_run.PROJECT_NUMBER}/secrets/{k_run.SECRET_ID}/versions/9"
        }
        return Response(200 if self.response is None else 200, body)


class FailingPostSession(FakeSession):
    def __init__(self, status_code):
        super().__init__()
        self.status_code = status_code

    def post(self, url, **kwargs):
        self.posts.append((url, kwargs))
        return Response(self.status_code, {"error": {"message": "sanitized test error"}})


class OriginKRunTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.stage = root / "stage"
        self.stage.mkdir(mode=0o700)
        self.salt = bytes(range(32))
        self.manifest = {
            "format": transport.FORMAT,
            "source_capture_run_id": k_run.SOURCE_RUN_ID,
            "target_run_id": k_run.TARGET_RUN_ID,
            "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256,
            "salt_hex": self.salt.hex(),
            "artifacts": {},
        }
        raw = json.dumps(self.manifest, sort_keys=True).encode()
        self.manifest_path = self.stage / "transport-manifest.json"
        self.manifest_path.write_bytes(raw)
        os.chmod(self.manifest_path, 0o600)
        self.manifest_sha = hashlib.sha256(raw).hexdigest()
        patcher = patch.object(k_run, "TRANSPORT_MANIFEST_SHA256", self.manifest_sha)
        patcher.start()
        self.addCleanup(patcher.stop)
        self.recovery_key = root / "recovery.key"
        self.recovery_key.write_text("11" * 32 + "\n")
        os.chmod(self.recovery_key, 0o600)
        self.expected_key = transport.transport_key(bytes.fromhex("11" * 32), self.salt,
                                                    k_run.TARGET_RUN_ID)

    def call_helper(self, fake):
        with patch.object(k_run.transport, "recover_verify", return_value={
            "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256, "verified_artifacts": 4,
        }) as verify:
            result = k_run.create_version(
                stage_dir=self.stage, recovery_key=self.recovery_key,
                expected_seal_sha256=k_run.SOURCE_SEAL_SHA256,
                expected_manifest_sha256=self.manifest_sha,
                access_token="test-token-never-logged", session=fake,
            )
        verify.assert_called_once_with(self.stage, self.recovery_key,
                                       k_run.SOURCE_RUN_ID, k_run.TARGET_RUN_ID,
                                       self.manifest_sha)
        return result

    def test_hex_payload_loads_as_exact_importer_key(self):
        fake = FakeSession()
        result = self.call_helper(fake)
        self.assertEqual(result["version"], "9")
        self.assertEqual(len(fake.posts), 1)
        self.assertEqual(fake.posts[0][0],
                         f"{k_run.API}/projects/{k_run.PROJECT_NUMBER}/secrets/{k_run.SECRET_ID}:addVersion")
        self.assertEqual(fake.get_call[0],
                         f"{k_run.API}/projects/{k_run.PROJECT_NUMBER}/secrets/{k_run.SECRET_ID}/versions")
        self.assertEqual(fake.posts[0][1]["headers"]["x-goog-user-project"], k_run.PROJECT)
        encoded_payload = fake.posts[0][1]["json"]["payload"]["data"]
        serialized_key = base64.b64decode(encoded_payload, validate=True)
        self.assertEqual(serialized_key, self.expected_key.hex().encode("ascii"))

        # Exercise the actual importer's on-disk contract with a private fixture.
        key_file = Path(self.temp.name) / "transport.key"
        fd = os.open(key_file, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
        with os.fdopen(fd, "wb") as stream:
            stream.write(serialized_key)
        self.assertEqual(stat.S_IMODE(key_file.stat().st_mode), 0o600)
        self.assertEqual(importer.load_transport_key(key_file), self.expected_key)
        self.assertNotIn(encoded_payload, json.dumps(result))

    def test_canonical_response_for_another_project_or_secret_is_rejected(self):
        invalid_names = (
            f"projects/999999999999/secrets/{k_run.SECRET_ID}/versions/9",
            f"projects/{k_run.PROJECT_NUMBER}/secrets/other-secret/versions/9",
        )
        for name in invalid_names:
            with self.subTest(name=name):
                fake = FakeSession(response={"name": name})
                with patch.object(k_run.transport, "recover_verify", return_value={
                    "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256, "verified_artifacts": 4,
                }):
                    with self.assertRaisesRegex(k_run.KeyRunError, "unexpected version resource"):
                        k_run.create_version(
                            stage_dir=self.stage, recovery_key=self.recovery_key,
                            expected_seal_sha256=k_run.SOURCE_SEAL_SHA256,
                            expected_manifest_sha256=self.manifest_sha,
                            access_token="test-token-never-logged", session=fake,
                        )
                self.assertEqual(len(fake.posts), 1)

    def test_timeout_or_server_error_is_uncertain_and_never_retried(self):
        cases = (
            (FakeSession(post_exception=requests.Timeout("simulated timeout")), requests.Timeout),
            (FailingPostSession(503), k_run.KeyRunError),
        )
        for fake, error in cases:
            with self.subTest(error=error.__name__):
                with patch.object(k_run.transport, "recover_verify", return_value={
                    "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256, "verified_artifacts": 4,
                }):
                    with self.assertRaises(error):
                        k_run.create_version(
                            stage_dir=self.stage, recovery_key=self.recovery_key,
                            expected_seal_sha256=k_run.SOURCE_SEAL_SHA256,
                            expected_manifest_sha256=self.manifest_sha,
                            access_token="test-token-never-logged", session=fake,
                        )
                self.assertEqual(len(fake.posts), 1)

    def test_existing_version_stops_before_add_version(self):
        fake = FakeSession(versions=[{"name": "existing-version"}])
        with patch.object(k_run.transport, "recover_verify", return_value={
            "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256, "verified_artifacts": 4,
        }):
            with self.assertRaisesRegex(k_run.KeyRunError, "already has a version"):
                k_run.create_version(
                    stage_dir=self.stage, recovery_key=self.recovery_key,
                    expected_seal_sha256=k_run.SOURCE_SEAL_SHA256,
                    expected_manifest_sha256=self.manifest_sha,
                    access_token="test-token-never-logged", session=fake,
                )
        self.assertEqual(fake.posts, [])

    def test_manifest_is_rehashed_immediately_before_post(self):
        def mutate_manifest():
            self.manifest_path.write_text("{}")

        fake = FakeSession(after_get=mutate_manifest)
        with patch.object(k_run.transport, "recover_verify", return_value={
            "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256, "verified_artifacts": 4,
        }):
            with self.assertRaisesRegex(k_run.KeyRunError, "changed before"):
                k_run.create_version(
                    stage_dir=self.stage, recovery_key=self.recovery_key,
                    expected_seal_sha256=k_run.SOURCE_SEAL_SHA256,
                    expected_manifest_sha256=self.manifest_sha,
                    access_token="test-token-never-logged", session=fake,
                )
        self.assertEqual(fake.posts, [])


if __name__ == "__main__":
    unittest.main()
