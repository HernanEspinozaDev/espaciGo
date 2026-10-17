#!/usr/bin/env python3
"""Offline checks for the one-shot K_run version helper."""
from __future__ import annotations

import base64
import hashlib
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import gcp03_origin_import as transport
import gcp03_origin_k_run as k_run


class Response:
    def __init__(self, status_code: int, body: dict):
        self.status_code = status_code
        self._body = body

    def json(self):
        return self._body


class FakeSession:
    def __init__(self, versions=None):
        self.versions = versions or []
        self.posts = []

    def get(self, url, **kwargs):
        self.get_call = (url, kwargs)
        return Response(200, {"versions": self.versions})

    def post(self, url, **kwargs):
        self.posts.append((url, kwargs))
        return Response(200, {"name": f"projects/{k_run.PROJECT}/secrets/{k_run.SECRET_ID}/versions/9"})


class OriginKRunTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.stage = root / "stage"
        self.stage.mkdir(mode=0o700)
        salt = bytes(range(32))
        self.manifest = {
            "format": transport.FORMAT,
            "source_capture_run_id": k_run.SOURCE_RUN_ID,
            "target_run_id": k_run.TARGET_RUN_ID,
            "source_sealed_sha256": k_run.SOURCE_SEAL_SHA256,
            "salt_hex": salt.hex(),
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
        self.salt = salt

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

    def test_derives_run_bound_key_and_adds_one_version_without_printing_payload(self):
        fake = FakeSession()
        result = self.call_helper(fake)
        self.assertEqual(result["version"], "9")
        self.assertEqual(len(fake.posts), 1)
        self.assertEqual(fake.posts[0][0],
                         f"{k_run.API}/projects/{k_run.PROJECT}/secrets/{k_run.SECRET_ID}:addVersion")
        payload = fake.posts[0][1]["json"]["payload"]["data"]
        actual = base64.b64decode(payload)
        expected = transport.transport_key(bytes.fromhex("11" * 32), self.salt,
                                           k_run.TARGET_RUN_ID)
        self.assertEqual(actual, expected)
        self.assertEqual(fake.get_call[1]["params"], {"pageSize": 2})
        self.assertNotIn(payload, json.dumps(result))

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


if __name__ == "__main__":
    unittest.main()
