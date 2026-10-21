import io
import json
import unittest
import zipfile
from unittest.mock import patch

from gcp07_release_policy import release_manifest_matches
from verify_gcp07_environment import download_release_manifest


REPO = "HernanEspinozaDev/espaciGo"
PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
RUN = {
    "status": "completed",
    "conclusion": "success",
    "event": "workflow_dispatch",
    "head_branch": "main",
    "path": ".github/workflows/application-release.yml",
    "head_sha": "a" * 40,
    "head_repository": {"full_name": REPO},
}
MANIFEST = {
    "schema": 1,
    "repository": REPO,
    "run_id": "39569841235",
    "source_sha": "a" * 40,
    "contains_migrations": False,
    "publish_only": True,
    "images": {
        "api": PREFIX + "espacigo-api@sha256:" + "b" * 64,
        "web": PREFIX + "espacigo-web@sha256:" + "c" * 64,
    },
}


class Gcp07ReleasePolicyTests(unittest.TestCase):
    def test_release_artifact_loader_reads_only_the_manifest(self):
        def archive_with(*files):
            stream = io.BytesIO()
            with zipfile.ZipFile(stream, "w") as bundle:
                for name, value in files:
                    bundle.writestr(name, value)
            return stream.getvalue()

        class Opener:
            def __init__(self, archive):
                self.archive = archive

            def open(self, request, timeout):
                return io.BytesIO(self.archive)

        archive = archive_with(("manifest.json", json.dumps(MANIFEST)))
        with patch("verify_gcp07_environment.urllib.request.build_opener", return_value=Opener(archive)):
            self.assertEqual(download_release_manifest(REPO, 123, "token"), MANIFEST)

        bad_archive = archive_with(("../manifest.json", json.dumps(MANIFEST)))
        with patch("verify_gcp07_environment.urllib.request.build_opener", return_value=Opener(bad_archive)):
            with self.assertRaises(ValueError):
                download_release_manifest(REPO, 123, "token")

    def test_accepts_the_exact_manual_publish_only_manifest(self):
        self.assertTrue(release_manifest_matches(MANIFEST, RUN, REPO, "39569841235"))

    def test_rejects_arbitrary_manual_dispatches_and_incomplete_publications(self):
        rejected = (
            ({**MANIFEST, "publish_only": False}, RUN),
            ({**MANIFEST, "contains_migrations": True}, RUN),
            ({**MANIFEST, "images": {"api": MANIFEST["images"]["api"]}}, RUN),
            ({**MANIFEST, "images": {**MANIFEST["images"], "web": "example.invalid/web:latest"}}, RUN),
            ({**MANIFEST, "repository": "attacker/repo"}, RUN),
            ({**MANIFEST, "run_id": "999"}, RUN),
            ({**MANIFEST, "source_sha": "d" * 40}, RUN),
            (MANIFEST, {**RUN, "event": "pull_request"}),
            (MANIFEST, {**RUN, "head_branch": "feature"}),
            (MANIFEST, {**RUN, "head_repository": {"full_name": "attacker/repo"}}),
            (MANIFEST, {**RUN, "path": ".github/workflows/other.yml"}),
            (MANIFEST, {**RUN, "conclusion": "failure"}),
        )
        for manifest, run in rejected:
            with self.subTest(manifest=manifest, run=run):
                self.assertFalse(release_manifest_matches(manifest, run, REPO, "39569841235"))

    def test_accepts_digest_pinned_push_publications_and_rejects_manual_flag(self):
        push_run = {**RUN, "event": "push"}
        push_manifest = {**MANIFEST, "publish_only": False,
                         "images": {"api": MANIFEST["images"]["api"]}}
        self.assertTrue(release_manifest_matches(push_manifest, push_run, REPO, "39569841235"))
        self.assertFalse(release_manifest_matches(MANIFEST, push_run, REPO, "39569841235"))


if __name__ == "__main__":
    unittest.main()
