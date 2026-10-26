#!/usr/bin/env python3
"""Unit tests for scripts/gcp/deploy-frontend.sh and scripts/gcp/release.py.

Uses Python standard library unittest to verify preflight checks, security
gates, git verification, identity checks, scope constraints, and mock
Cloud Build trigger dispatches without any cloud mutations or network access.
"""

import io
import json
import os
import subprocess
import sys
import unittest
from unittest import mock

# Add scripts/gcp to sys.path so we can import release directly
SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
if SCRIPT_DIR not in sys.path:
    sys.path.insert(0, SCRIPT_DIR)

import release

VALID_BUILD_UUID = "548b8eff-e489-4307-a77f-455b56aa4e57"
VALID_SHA = "12c2b75690623d38c64bbd99bb6522c06eec4e1f"

VALID_TRIGGER_DATA = {
    "id": "548b8eff-e489-4307-a77f-455b56aa4e57",
    "name": "espacigo-main-rollout",
    "filename": "cloudbuild.yaml",
    "disabled": False,
    "serviceAccount": "projects/espacigo-testprod/serviceAccounts/espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com",
    "repositoryEventConfig": {
        "repository": "projects/espacigo-testprod/locations/southamerica-west1/connections/espacigo-github/repositories/espacigo",
        "push": {"branch": "^main$"},
    },
    "approvalConfig": {},
}

VALID_RECENT_BUILDS = [
    {
        "id": "32a84b0e-1111-2222-3333-444455556666",
        "status": "SUCCESS",
        "createTime": "2026-10-26T00:00:00Z",
        "substitutions": {
            "_DEPLOY_SCOPE": "web",
            "COMMIT_SHA": "0000000000000000000000000000000000000000",
        },
        "logUrl": "https://console.cloud.google.com/test-log-1",
    }
]

VALID_CLOUD_RUN_STATUS = {
    "status": {
        "url": "https://espacigo-test-public.run.app",
        "latestReadyRevisionName": "espacigo-test-public-00001-xyz",
        "traffic": [
            {
                "revisionName": "espacigo-test-public-00001-xyz",
                "percent": 100,
            }
        ],
    }
}

VALID_TRIGGER_BUILD_OBJ = {
    "id": VALID_BUILD_UUID,
    "status": "QUEUED",
    "logUrl": f"https://console.cloud.google.com/test-log-{VALID_BUILD_UUID}",
    "substitutions": {
        "_DEPLOY_SCOPE": "web",
        "COMMIT_SHA": VALID_SHA,
    },
}


class TestDeployFrontend(unittest.TestCase):
    """Test suite for release.py and deploy-frontend.sh wrapper."""

    def setUp(self):
        super().setUp()
        self._orig_subprocess_run = subprocess.run

        def denied_subprocess_run(*args, **kwargs):
            cmd = args[0] if args else kwargs.get("cmd", kwargs.get("args", []))
            # Allow only explicit help commands (e.g. bash deploy-frontend.sh --help)
            if isinstance(cmd, list) and "--help" in cmd:
                return self._orig_subprocess_run(*args, **kwargs)
            raise AssertionError(f"Subprocess execution denied in unit test: {cmd}")

        patcher_sp = mock.patch("subprocess.run", side_effect=denied_subprocess_run)
        self.mock_deny_sp = patcher_sp.start()
        self.addCleanup(patcher_sp.stop)

    def _patch_preflight_mocks(
        self,
        mock_git,
        mock_remote_repo,
        mock_gh,
        mock_trigger,
        mock_builds,
        mock_run,
        branch="main",
        sha=VALID_SHA,
        is_clean=True,
        remote_repo="HernanEspinozaDev/espaciGo",
        gh_user="HernanEspinozaDev",
        trigger_data=None,
        recent_builds=None,
        cloud_run_status=None,
    ):
        mock_git.return_value = (branch, sha, is_clean)
        mock_remote_repo.return_value = remote_repo
        mock_gh.return_value = gh_user
        mock_trigger.return_value = trigger_data or VALID_TRIGGER_DATA
        mock_builds.return_value = recent_builds if recent_builds is not None else VALID_RECENT_BUILDS
        mock_run.return_value = cloud_run_status or VALID_CLOUD_RUN_STATUS

    def test_help_flag(self):
        """Verify --help displays usage and returns 0."""
        with mock.patch.object(sys, "argv", ["release.py", "--help"]):
            with self.assertRaises(SystemExit) as cm:
                with mock.patch("sys.stdout", new_callable=io.StringIO):
                    release.main()
            self.assertEqual(cm.exception.code, 0)

    def test_scope_gate_rejects_full_or_arbitrary(self):
        """Verify wrapper rejects --scope full or anything other than web."""
        with mock.patch.object(sys, "argv", ["release.py", "--scope", "full"]):
            with mock.patch("sys.stderr", new_callable=io.StringIO) as fake_err:
                exit_code = release.main()
            self.assertEqual(exit_code, 1)
            err_msg = fake_err.getvalue()
            self.assertIn("disallowed", err_msg)
            self.assertIn("full", err_msg)
            self.assertIn("database schema evidence", err_msg)

    def test_validate_trigger_config_valid(self):
        """Valid trigger configuration passes validation without raising."""
        try:
            release.validate_trigger_config(VALID_TRIGGER_DATA)
        except release.DeploymentValidationError as e:
            self.fail(f"Valid trigger config unexpectedly failed: {e}")

    def test_validate_trigger_config_failures(self):
        """Verify each trigger constraint failure raises DeploymentValidationError."""
        cases = [
            ({"id": "wrong-id"}, "Trigger ID mismatch"),
            ({"name": "wrong-name"}, "Trigger name mismatch"),
            ({"disabled": True}, "is DISABLED"),
            ({"filename": "Dockerfile"}, "build filename mismatch"),
            (
                # Suffix-only SA must fail (full equality required)
                {"serviceAccount": "espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com"},
                "Service Account mismatch",
            ),
            (
                # Wrong project in SA must fail
                {"serviceAccount": "projects/other-proj/serviceAccounts/espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com"},
                "Service Account mismatch",
            ),
            (
                # Suffix-only repository must fail (full equality required)
                {"repositoryEventConfig": {"repository": "connections/espacigo-github/repositories/espacigo"}},
                "repository resource mismatch",
            ),
            (
                {"repositoryEventConfig": {"repository": "projects/other-proj/locations/southamerica-west1/connections/espacigo-github/repositories/espacigo"}},
                "repository resource mismatch",
            ),
            (
                {"repositoryEventConfig": {"repository": VALID_TRIGGER_DATA["repositoryEventConfig"]["repository"], "push": {"branch": "^feature.*$"}}},
                "branch pattern mismatch",
            ),
            ({"approvalConfig": {"approvalRequired": True}}, "requires manual approval"),
        ]

        for override, expected_err in cases:
            bad_data = dict(VALID_TRIGGER_DATA)
            bad_data.update(override)
            with self.subTest(override=override):
                with self.assertRaises(release.DeploymentValidationError) as cm:
                    release.validate_trigger_config(bad_data)
                self.assertIn(expected_err, str(cm.exception))

    def test_verify_git_remote_repo_exact_matching(self):
        """Verify remote origin URL repo extraction and exact matching."""
        valid_urls = [
            "https://github.com/HernanEspinozaDev/espaciGo.git",
            "git@github.com:HernanEspinozaDev/espaciGo.git",
            "https://github.com/HernanEspinozaDev/espaciGo",
            "HernanEspinozaDev/espaciGo",
        ]
        for url in valid_urls:
            with self.subTest(url=url):
                with mock.patch("release.run_cmd") as mock_rc:
                    mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=f"{url}\n", stderr="")
                    res = release.verify_git_remote_repo("/test/repo")
                    self.assertEqual(res, release.EXPECTED_REPO)

        invalid_urls = [
            "https://github.com/OtherDev/espaciGo.git",
            "https://github.com/HernanEspinozaDev/otherProject.git",
            "git@github.com:attacker/espaciGo.git",
        ]
        for url in invalid_urls:
            with self.subTest(url=url):
                with mock.patch("release.run_cmd") as mock_rc:
                    mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=f"{url}\n", stderr="")
                    with self.assertRaises(release.DeploymentValidationError) as cm:
                        release.verify_git_remote_repo("/test/repo")
                    self.assertIn("repository mismatch", str(cm.exception))

    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_default_read_only_preflight(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run):
        """Default invocation (no args) is strictly READ-ONLY and prints status with traffic & SHA."""
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run)

        with mock.patch.object(sys, "argv", ["release.py"]):
            with mock.patch("sys.stdout", new_callable=io.StringIO) as fake_out:
                with mock.patch("release.trigger_build") as mock_dispatch:
                    exit_code = release.main()
                    mock_dispatch.assert_not_called()

        self.assertEqual(exit_code, 0)
        out = fake_out.getvalue()
        self.assertIn("[READ-ONLY] Preflight checks completed successfully.", out)
        self.assertIn("No Cloud Build trigger was dispatched.", out)
        self.assertIn("Git Remote Repo      : HernanEspinozaDev/espaciGo (verified exact HernanEspinozaDev/espaciGo)", out)
        self.assertIn("Traffic Distribution:", out)
        self.assertIn("espacigo-test-public-00001-xyz: 100%", out)
        self.assertIn("SHA: 00000000", out)

    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_deploy_fails_if_git_working_tree_dirty(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run):
        """--deploy fails if git status is not clean without making unmocked network calls."""
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run, is_clean=False)

        with mock.patch.object(sys, "argv", ["release.py", "--deploy"]):
            with mock.patch("sys.stderr", new_callable=io.StringIO) as fake_err:
                exit_code = release.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("uncommitted modifications", fake_err.getvalue())

    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_deploy_fails_if_branch_not_main(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run):
        """--deploy fails if current branch is not main without making unmocked network calls."""
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run, branch="feature/new-button")

        with mock.patch.object(sys, "argv", ["release.py", "--deploy"]):
            with mock.patch("sys.stderr", new_callable=io.StringIO) as fake_err:
                exit_code = release.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("must be 'main'", fake_err.getvalue())

    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.get_git_remote_main_sha")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_deploy_fails_if_remote_sha_mismatch(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_remote_sha, mock_builds, mock_run):
        """--deploy fails if local HEAD does not match origin/main remote SHA."""
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run)
        mock_remote_sha.return_value = "0000000000000000000000000000000000000000"

        with mock.patch.object(sys, "argv", ["release.py", "--deploy"]):
            with mock.patch("sys.stderr", new_callable=io.StringIO) as fake_err:
                exit_code = release.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("does not match remote origin/main", fake_err.getvalue())

    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.get_git_remote_main_sha")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_deploy_fails_if_requested_sha_differs_or_untrimmed(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_remote_sha, mock_builds, mock_run):
        """--sha requires exact match without whitespace stripping."""
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run)
        mock_remote_sha.return_value = VALID_SHA

        # Whitespace-padded SHA must NOT be stripped or accepted
        with mock.patch.object(sys, "argv", ["release.py", "--deploy", f"--sha= {VALID_SHA} "]):
            with mock.patch("sys.stderr", new_callable=io.StringIO) as fake_err:
                exit_code = release.main()
        self.assertEqual(exit_code, 1)
        self.assertIn("does not match verified HEAD", fake_err.getvalue())

    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.get_git_remote_main_sha")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_deploy_prevents_duplicate_in_progress_build(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_remote_sha, mock_builds, mock_run):
        """--deploy fails if a build for the same SHA is already in progress (QUEUED/WORKING)."""
        in_progress_builds = [
            {
                "id": "existing-uuid-1111-2222-3333-444455556666",
                "status": "WORKING",
                "createTime": "2026-10-26T02:00:00Z",
                "substitutions": {
                    "_DEPLOY_SCOPE": "web",
                    "COMMIT_SHA": VALID_SHA,
                },
            }
        ]
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run, recent_builds=in_progress_builds)
        mock_remote_sha.return_value = VALID_SHA

        with mock.patch.object(sys, "argv", ["release.py", "--deploy"]):
            with mock.patch("sys.stderr", new_callable=io.StringIO) as fake_err:
                exit_code = release.main()

        self.assertEqual(exit_code, 1)
        self.assertIn("already in progress", fake_err.getvalue())

    @mock.patch("release.trigger_build")
    @mock.patch("release.get_cloud_run_status")
    @mock.patch("release.get_recent_builds")
    @mock.patch("release.get_git_remote_main_sha")
    @mock.patch("release.describe_cloud_build_trigger")
    @mock.patch("release.verify_github_identity")
    @mock.patch("release.verify_git_remote_repo")
    @mock.patch("release.get_git_status")
    def test_deploy_success_flow(self, mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_remote_sha, mock_builds, mock_run, mock_trigger_build):
        """--deploy with valid conditions calls trigger_build and prints verified build ID."""
        self._patch_preflight_mocks(mock_git, mock_remote_repo, mock_gh, mock_trigger, mock_builds, mock_run)
        mock_remote_sha.return_value = VALID_SHA
        mock_trigger_build.return_value = VALID_TRIGGER_BUILD_OBJ

        with mock.patch.object(sys, "argv", ["release.py", "--deploy"]):
            with mock.patch("sys.stdout", new_callable=io.StringIO) as fake_out:
                exit_code = release.main()

        self.assertEqual(exit_code, 0)
        mock_trigger_build.assert_called_once_with(VALID_SHA)
        out = fake_out.getvalue()
        self.assertIn(VALID_BUILD_UUID, out)
        self.assertIn("QUEUED", out)
        self.assertIn("OPERATOR INSTRUCTIONS", out)
        self.assertIn(f"gcloud builds log {VALID_BUILD_UUID}", out)

    def test_get_recent_builds_error_propagation(self):
        """get_recent_builds propagates errors instead of returning empty list."""
        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.side_effect = release.DeploymentValidationError("gcloud builds list failed")
            with self.assertRaises(release.DeploymentValidationError):
                release.get_recent_builds()

        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout="NOT_JSON", stderr="")
            with self.assertRaises(release.DeploymentValidationError):
                release.get_recent_builds()

    def test_get_cloud_run_status_error_propagation(self):
        """get_cloud_run_status propagates errors and checks required target metadata."""
        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.side_effect = release.DeploymentValidationError("gcloud run services describe failed")
            with self.assertRaises(release.DeploymentValidationError):
                release.get_cloud_run_status("espacigo-test-public")

        # Missing url in metadata
        with mock.patch("release.run_cmd") as mock_rc:
            bad_payload = json.dumps({"status": {"latestReadyRevisionName": "rev-1"}})
            mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=bad_payload, stderr="")
            with self.assertRaises(release.DeploymentValidationError) as cm:
                release.get_cloud_run_status("espacigo-test-public")
            self.assertIn("Missing target metadata", str(cm.exception))

    def test_trigger_build_validation(self):
        """trigger_build validates UUID, scope, and commit SHA in response."""
        # 1. Valid operation metadata wrapping build
        op_response = json.dumps({
            "metadata": {
                "build": {
                    "id": VALID_BUILD_UUID,
                    "status": "QUEUED",
                    "substitutions": {
                        "_DEPLOY_SCOPE": "web",
                        "COMMIT_SHA": VALID_SHA,
                    },
                }
            }
        })
        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=op_response, stderr="")
            res = release.trigger_build(VALID_SHA)
            self.assertEqual(res["id"], VALID_BUILD_UUID)

        # 2. Invalid UUID
        bad_id_resp = json.dumps({
            "id": "unknown",
            "substitutions": {"_DEPLOY_SCOPE": "web", "COMMIT_SHA": VALID_SHA},
        })
        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=bad_id_resp, stderr="")
            with self.assertRaises(release.DeploymentValidationError) as cm:
                release.trigger_build(VALID_SHA)
            self.assertIn("Invalid or missing build ID", str(cm.exception))

        # 3. Scope mismatch
        bad_scope_resp = json.dumps({
            "id": VALID_BUILD_UUID,
            "substitutions": {"_DEPLOY_SCOPE": "full", "COMMIT_SHA": VALID_SHA},
        })
        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=bad_scope_resp, stderr="")
            with self.assertRaises(release.DeploymentValidationError) as cm:
                release.trigger_build(VALID_SHA)
            self.assertIn("scope mismatch", str(cm.exception))

        # 4. SHA mismatch
        bad_sha_resp = json.dumps({
            "id": VALID_BUILD_UUID,
            "substitutions": {"_DEPLOY_SCOPE": "web", "COMMIT_SHA": "0000000000000000000000000000000000000000"},
        })
        with mock.patch("release.run_cmd") as mock_rc:
            mock_rc.return_value = subprocess.CompletedProcess(args=[], returncode=0, stdout=bad_sha_resp, stderr="")
            with self.assertRaises(release.DeploymentValidationError) as cm:
                release.trigger_build(VALID_SHA)
            self.assertIn("commit SHA mismatch", str(cm.exception))

    def test_run_cmd_timeout_handling(self):
        """run_cmd raises DeploymentValidationError on subprocess.TimeoutExpired."""
        with mock.patch("subprocess.run") as mock_subproc:
            mock_subproc.side_effect = subprocess.TimeoutExpired(cmd=["gcloud", "test"], timeout=60)
            with self.assertRaises(release.DeploymentValidationError) as cm:
                release.run_cmd(["gcloud", "test"])
            self.assertIn("timed out after 60s", str(cm.exception))

    def test_shell_script_help_execution(self):
        """Verify deploy-frontend.sh wrapper executes successfully with --help."""
        sh_path = os.path.join(SCRIPT_DIR, "deploy-frontend.sh")
        res = subprocess.run([sh_path, "--help"], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        self.assertEqual(res.returncode, 0)
        self.assertIn("Safe Cloud Build rollout wrapper for EspaciGo Frontend.", res.stdout)


if __name__ == "__main__":
    unittest.main()
