#!/usr/bin/env python3
"""Canonical deployment and preflight wrapper for EspaciGo Frontend.

Interacts safely with the existing Cloud Build trigger 'espacigo-main-rollout'
(ID: 548b8eff-e489-4307-a77f-455b56aa4e57) in project 'espacigo-testprod',
region 'southamerica-west1', account 'pasteleriahijitos@gmail.com'.

Default behavior (no arguments) is STRICTLY READ-ONLY preflight and status.
Triggering a build requires the explicit '--deploy' flag and passes full SHA
verification matching remote origin/main on clean git status.
"""

import argparse
import json
import os
import re
import subprocess
import sys
from typing import Any, Dict, List, Optional, Tuple

# Fixed canonical infrastructure constants
PROJECT_ID = "espacigo-testprod"
REGION = "southamerica-west1"
ACCOUNT = "pasteleriahijitos@gmail.com"

TRIGGER_ID = "548b8eff-e489-4307-a77f-455b56aa4e57"
TRIGGER_NAME = "espacigo-main-rollout"
EXPECTED_REPO = "HernanEspinozaDev/espaciGo"
EXPECTED_REPO_RESOURCE = (
    f"projects/{PROJECT_ID}/locations/{REGION}/connections/espacigo-github/repositories/espacigo"
)
EXPECTED_BRANCH = "main"
EXPECTED_BRANCH_REGEX = "^main$"
EXPECTED_FILENAME = "cloudbuild.yaml"
EXPECTED_SERVICE_ACCOUNT = (
    f"projects/{PROJECT_ID}/serviceAccounts/espacigo-cloud-build-deployer@{PROJECT_ID}.iam.gserviceaccount.com"
)
EXPECTED_GH_USER = "HernanEspinozaDev"

DEPLOY_SCOPE_WEB = "web"
RUN_SERVICES = ["espacigo-test-public", "espacigo-test-private"]
DEFAULT_TIMEOUT_SECS = 60

BUILD_UUID_REGEX = re.compile(
    r"^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$",
    re.IGNORECASE,
)


class DeploymentValidationError(Exception):
    """Raised when validation or preflight checks fail."""


def run_cmd(
    cmd: List[str],
    cwd: Optional[str] = None,
    check: bool = True,
    timeout: int = DEFAULT_TIMEOUT_SECS,
) -> subprocess.CompletedProcess:
    """Run a system command with finite timeout and return CompletedProcess."""
    try:
        return subprocess.run(
            cmd,
            cwd=cwd,
            check=check,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            timeout=timeout,
        )
    except subprocess.TimeoutExpired as e:
        raise DeploymentValidationError(
            f"Command timed out after {timeout}s: {' '.join(cmd)}"
        ) from e
    except subprocess.CalledProcessError as e:
        raise DeploymentValidationError(
            f"Command failed ({' '.join(cmd)}):\n{e.stderr.strip() or e.stdout.strip()}"
        ) from e


def get_git_status(repo_dir: str) -> Tuple[str, str, bool]:
    """Return (current_branch, head_sha, is_clean)."""
    res_branch = run_cmd(["git", "rev-parse", "--abbrev-ref", "HEAD"], cwd=repo_dir)
    branch = res_branch.stdout.strip()

    res_sha = run_cmd(["git", "rev-parse", "HEAD"], cwd=repo_dir)
    sha = res_sha.stdout.strip()

    res_status = run_cmd(["git", "status", "--porcelain"], cwd=repo_dir)
    is_clean = len(res_status.stdout.strip()) == 0

    return branch, sha, is_clean


def get_git_remote_repo(repo_dir: str, remote: str = "origin") -> str:
    """Get remote repository owner/repo string from git remote URL."""
    res = run_cmd(["git", "remote", "get-url", remote], cwd=repo_dir)
    raw_url = res.stdout.strip()
    if not raw_url:
        raise DeploymentValidationError(f"Remote '{remote}' has no URL configured.")
    clean_url = raw_url[:-4] if raw_url.endswith(".git") else raw_url
    if "github.com/" in clean_url:
        repo = clean_url.split("github.com/", 1)[1]
    elif "github.com:" in clean_url:
        repo = clean_url.split("github.com:", 1)[1]
    else:
        repo = clean_url
    return repo


def verify_git_remote_repo(repo_dir: str, remote: str = "origin") -> str:
    """Verify git remote origin repository matches EXPECTED_REPO exactly."""
    repo = get_git_remote_repo(repo_dir, remote=remote)
    if repo != EXPECTED_REPO:
        raise DeploymentValidationError(
            f"Git remote '{remote}' repository mismatch: expected '{EXPECTED_REPO}', got '{repo}'."
        )
    return repo


def get_git_remote_main_sha(repo_dir: str, remote: str = "origin") -> str:
    """Get full commit SHA of refs/heads/main on remote."""
    res = run_cmd(["git", "ls-remote", remote, "refs/heads/main"], cwd=repo_dir)
    output = res.stdout.strip()
    if not output:
        raise DeploymentValidationError(
            f"Could not resolve remote branch refs/heads/main on remote '{remote}'."
        )
    parts = output.split()
    remote_sha = parts[0].strip()
    if not re.match(r"^[0-9a-f]{40}$", remote_sha):
        raise DeploymentValidationError(
            f"Remote SHA '{remote_sha}' is not a valid 40-character hex SHA."
        )
    return remote_sha


def verify_github_identity() -> str:
    """Verify gh CLI identity matches HernanEspinozaDev."""
    res = run_cmd(["gh", "api", "user", "--jq", ".login"])
    gh_user = res.stdout.strip()
    if gh_user != EXPECTED_GH_USER:
        raise DeploymentValidationError(
            f"GitHub identity mismatch: expected '{EXPECTED_GH_USER}', got '{gh_user}'."
        )
    return gh_user


def describe_cloud_build_trigger() -> Dict[str, Any]:
    """Fetch trigger config from Cloud Build via gcloud with explicit params."""
    cmd = [
        "gcloud",
        "builds",
        "triggers",
        "describe",
        TRIGGER_ID,
        f"--project={PROJECT_ID}",
        f"--region={REGION}",
        f"--account={ACCOUNT}",
        "--format=json",
    ]
    res = run_cmd(cmd)
    try:
        return json.loads(res.stdout)
    except json.JSONDecodeError as e:
        raise DeploymentValidationError(f"Failed to parse trigger JSON: {e}") from e


def validate_trigger_config(trigger_data: Dict[str, Any]) -> None:
    """Enforce all security constraints on the Cloud Build trigger."""
    actual_id = trigger_data.get("id", "")
    if actual_id != TRIGGER_ID:
        raise DeploymentValidationError(
            f"Trigger ID mismatch: expected '{TRIGGER_ID}', got '{actual_id}'."
        )

    actual_name = trigger_data.get("name", "")
    if actual_name != TRIGGER_NAME:
        raise DeploymentValidationError(
            f"Trigger name mismatch: expected '{TRIGGER_NAME}', got '{actual_name}'."
        )

    if trigger_data.get("disabled", False) is True:
        raise DeploymentValidationError(f"Trigger '{actual_name}' ({actual_id}) is DISABLED.")

    actual_filename = trigger_data.get("filename", "")
    if actual_filename != EXPECTED_FILENAME:
        raise DeploymentValidationError(
            f"Trigger build filename mismatch: expected '{EXPECTED_FILENAME}', got '{actual_filename}'."
        )

    # Full equality for Service Account
    actual_sa = trigger_data.get("serviceAccount", "")
    if actual_sa != EXPECTED_SERVICE_ACCOUNT:
        raise DeploymentValidationError(
            f"Trigger Service Account mismatch: expected '{EXPECTED_SERVICE_ACCOUNT}', got '{actual_sa}'."
        )

    # Full equality for Repository resource
    repo_event = trigger_data.get("repositoryEventConfig", {})
    repo_resource = repo_event.get("repository", "")
    if repo_resource != EXPECTED_REPO_RESOURCE:
        raise DeploymentValidationError(
            f"Trigger repository resource mismatch: expected '{EXPECTED_REPO_RESOURCE}', got '{repo_resource}'."
        )

    branch_pattern = repo_event.get("push", {}).get("branch", "")
    if branch_pattern != EXPECTED_BRANCH_REGEX:
        raise DeploymentValidationError(
            f"Trigger branch pattern mismatch: expected '{EXPECTED_BRANCH_REGEX}', got '{branch_pattern}'."
        )

    # Approval check: no unknown approval configurations requiring manual intervention
    approval_config = trigger_data.get("approvalConfig", {})
    if approval_config.get("approvalRequired", False) is True:
        raise DeploymentValidationError(
            "Trigger requires manual approval; cannot be automated safely without operator gate."
        )


def get_recent_builds(limit: int = 3) -> List[Dict[str, Any]]:
    """Fetch recent builds triggered by this trigger."""
    cmd = [
        "gcloud",
        "builds",
        "list",
        f"--project={PROJECT_ID}",
        f"--region={REGION}",
        f"--account={ACCOUNT}",
        f"--filter=buildTriggerId={TRIGGER_ID}",
        f"--limit={limit}",
        "--format=json(id,status,createTime,substitutions._DEPLOY_SCOPE,substitutions.COMMIT_SHA,logUrl)",
    ]
    res = run_cmd(cmd, check=True)
    try:
        data = json.loads(res.stdout)
        if not isinstance(data, list):
            raise DeploymentValidationError(
                f"Expected list of builds from Cloud Build, got: {type(data)}"
            )
        return data
    except json.JSONDecodeError as e:
        raise DeploymentValidationError(f"Failed to parse Cloud Build list JSON: {e}") from e


def get_cloud_run_status(service_name: str) -> Dict[str, Any]:
    """Fetch status of a Cloud Run service."""
    cmd = [
        "gcloud",
        "run",
        "services",
        "describe",
        service_name,
        f"--project={PROJECT_ID}",
        f"--region={REGION}",
        f"--account={ACCOUNT}",
        "--format=json(status.url,status.latestReadyRevisionName,status.traffic)",
    ]
    res = run_cmd(cmd, check=True)
    try:
        data = json.loads(res.stdout)
        if not isinstance(data, dict) or "status" not in data:
            raise DeploymentValidationError(
                f"Invalid status payload for Cloud Run service '{service_name}': {data}"
            )
        st = data["status"]
        if not st.get("url") or not st.get("latestReadyRevisionName"):
            raise DeploymentValidationError(
                f"Missing target metadata (url or latestReadyRevisionName) for Cloud Run service '{service_name}'."
            )
        return data
    except json.JSONDecodeError as e:
        raise DeploymentValidationError(
            f"Failed to parse Cloud Run status JSON for '{service_name}': {e}"
        ) from e


def trigger_build(sha: str) -> Dict[str, Any]:
    """Execute gcloud builds triggers run with fixed parameters and verified SHA."""
    if not re.match(r"^[0-9a-f]{40}$", sha):
        raise DeploymentValidationError(f"Invalid commit SHA '{sha}': must be 40-character hex.")

    cmd = [
        "gcloud",
        "builds",
        "triggers",
        "run",
        TRIGGER_ID,
        f"--project={PROJECT_ID}",
        f"--region={REGION}",
        f"--account={ACCOUNT}",
        f"--sha={sha}",
        f"--substitutions=_DEPLOY_SCOPE={DEPLOY_SCOPE_WEB}",
        "--format=json",
    ]
    res = run_cmd(cmd)
    try:
        data = json.loads(res.stdout)
    except json.JSONDecodeError as e:
        raise DeploymentValidationError(f"Failed to parse triggers run JSON: {e}") from e

    if isinstance(data, list) and len(data) > 0:
        data = data[0]

    if isinstance(data, dict):
        if "metadata" in data and isinstance(data["metadata"], dict) and "build" in data["metadata"]:
            build_obj = data["metadata"]["build"]
        else:
            build_obj = data
    else:
        raise DeploymentValidationError(f"Unexpected response format from triggers run: {data}")

    # 1. Validate build ID is a valid UUID
    build_id = build_obj.get("id", "")
    if not BUILD_UUID_REGEX.match(str(build_id)):
        raise DeploymentValidationError(
            f"Invalid or missing build ID in triggers run response: '{build_id}' (expected UUID)."
        )

    # 2. Validate substitutions scope is 'web'
    substitutions = build_obj.get("substitutions", {})
    resp_scope = substitutions.get("_DEPLOY_SCOPE")
    if resp_scope != DEPLOY_SCOPE_WEB:
        raise DeploymentValidationError(
            f"Response build scope mismatch: expected '{DEPLOY_SCOPE_WEB}', got '{resp_scope}'."
        )

    # 3. Validate commit SHA in response metadata
    resp_sha = (
        substitutions.get("COMMIT_SHA")
        or build_obj.get("sourceProvenance", {}).get("resolvedRepoSource", {}).get("commitSha")
        or build_obj.get("source", {}).get("gitSource", {}).get("revision")
    )
    if resp_sha != sha:
        raise DeploymentValidationError(
            f"Response build commit SHA mismatch: expected '{sha}', got '{resp_sha}'."
        )

    return build_obj


def run_preflight(repo_dir: str) -> Dict[str, Any]:
    """Execute full read-only preflight and return gathered state."""
    print("=== [PREFLIGHT] EspaciGo Frontend Cloud Build Verification ===")
    print(f"Target Project       : {PROJECT_ID}")
    print(f"Target Region        : {REGION}")
    print(f"Target Account       : {ACCOUNT}")
    print(f"Trigger ID           : {TRIGGER_ID} ({TRIGGER_NAME})")
    print(f"Allowed Scope        : _DEPLOY_SCOPE={DEPLOY_SCOPE_WEB}")
    print("-" * 65)

    # 1. Local Git State
    branch, head_sha, is_clean = get_git_status(repo_dir)
    print(f"Git Branch           : {branch} (expected: {EXPECTED_BRANCH})")
    print(f"Git HEAD Commit      : {head_sha}")
    print(f"Git Working Tree     : {'CLEAN' if is_clean else 'DIRTY (Uncommitted changes)'}")

    # 2. Remote Origin Repository
    remote_repo = verify_git_remote_repo(repo_dir, remote="origin")
    print(f"Git Remote Repo      : {remote_repo} (verified exact {EXPECTED_REPO})")

    # 3. GitHub CLI Identity
    gh_user = verify_github_identity()
    print(f"GitHub CLI User      : {gh_user} (verified)")

    # 4. Cloud Build Trigger Config
    print("Fetching Cloud Build trigger metadata...")
    trigger_data = describe_cloud_build_trigger()
    validate_trigger_config(trigger_data)
    print(f"Trigger Verification : PASS (Active, pointing to {trigger_data.get('filename')} on {EXPECTED_BRANCH})")

    # 5. Recent Builds
    recent_builds = get_recent_builds(limit=3)
    print(f"Recent Builds        : {len(recent_builds)} found")
    for b in recent_builds:
        b_id = b.get("id", "unknown")
        b_status = b.get("status", "unknown")
        b_time = b.get("createTime", "")
        b_sha = b.get("substitutions", {}).get("COMMIT_SHA") or b.get("commitSha", "unknown")
        print(f"  - Build {b_id[:8]}... | SHA: {b_sha[:8]} | Status: {b_status:<10} | Created: {b_time}")

    # 6. Cloud Run Services Status & Traffic
    print("Checking Cloud Run Services Traffic...")
    services_status = {}
    for svc in RUN_SERVICES:
        st = get_cloud_run_status(svc)
        services_status[svc] = st
        url = st["status"].get("url", "N/A")
        rev = st["status"].get("latestReadyRevisionName", "N/A")
        traffic = st["status"].get("traffic", [])
        print(f"  - Service '{svc}':")
        print(f"      URL            : {url}")
        print(f"      Latest Revision: {rev}")
        print("      Traffic Distribution:")
        if traffic:
            for t in traffic:
                t_rev = t.get("revisionName") or "(latest)"
                t_pct = t.get("percent", 0)
                print(f"        - {t_rev}: {t_pct}%")
        else:
            print("        - None")

    print("-" * 65)
    return {
        "branch": branch,
        "head_sha": head_sha,
        "is_clean": is_clean,
        "remote_repo": remote_repo,
        "gh_user": gh_user,
        "trigger": trigger_data,
        "recent_builds": recent_builds,
        "services": services_status,
    }


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Safe Cloud Build rollout wrapper for EspaciGo Frontend.",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Default mode is strictly READ-ONLY. It verifies trigger config, inspects recent
builds, and checks current Cloud Run traffic without triggering any action.

To dispatch a deployment trigger:
  %(prog)s --deploy

Security requirements for --deploy:
  1. Git working tree must be clean.
  2. Current branch must be 'main'.
  3. Git remote origin URL must match 'HernanEspinozaDev/espaciGo' exactly.
  4. Local HEAD commit must match remote origin/main exactly.
  5. GitHub CLI identity must be 'HernanEspinozaDev'.
  6. Cloud Build trigger configuration must pass validation.
  7. Scope is strictly locked to _DEPLOY_SCOPE=web.
""",
    )
    parser.add_argument(
        "--deploy",
        action="store_true",
        help="Trigger Cloud Build execution for verified HEAD commit.",
    )
    parser.add_argument(
        "--sha",
        type=str,
        default=None,
        help="Explicit full 40-character commit SHA (must match verified HEAD).",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Validate all conditions including remote verification without triggering build.",
    )
    parser.add_argument(
        "--scope",
        type=str,
        default="web",
        help="Deployment scope. Only 'web' is supported by this wrapper (default: web).",
    )

    args = parser.parse_args()

    # Scope gate check
    if args.scope != DEPLOY_SCOPE_WEB:
        print(
            f"[SECURITY ERROR] Scope '{args.scope}' is disallowed.\n"
            f"The frontend deployment wrapper is strictly restricted to scope '{DEPLOY_SCOPE_WEB}'.\n"
            f"'full' scope requires verified database schema evidence and cannot be invoked from here.",
            file=sys.stderr,
        )
        return 1

    # Determine repo directory (assuming scripts/gcp/.. -> repo root)
    script_dir = os.path.dirname(os.path.abspath(__file__))
    repo_dir = os.path.abspath(os.path.join(script_dir, "..", ".."))

    try:
        # 1. Run Preflight
        preflight = run_preflight(repo_dir)

        if not args.deploy and not args.dry_run:
            print("[READ-ONLY] Preflight checks completed successfully.")
            print("No Cloud Build trigger was dispatched.")
            print("Run with --deploy to initiate rollout.")
            return 0

        # 2. Strict Deployment Gates
        print("\n=== [DEPLOY GATES] Validating Deployment Prerequisites ===")

        # Working tree check
        if not preflight["is_clean"]:
            raise DeploymentValidationError(
                "Cannot deploy: git working tree has uncommitted modifications or untracked files."
            )

        # Branch check
        if preflight["branch"] != EXPECTED_BRANCH:
            raise DeploymentValidationError(
                f"Cannot deploy from branch '{preflight['branch']}': must be '{EXPECTED_BRANCH}'."
            )

        # Remote origin repository check
        verify_git_remote_repo(repo_dir, remote="origin")

        # Remote origin/main SHA verification
        print(f"Verifying remote 'origin/refs/heads/{EXPECTED_BRANCH}'...")
        remote_sha = get_git_remote_main_sha(repo_dir, remote="origin")
        local_sha = preflight["head_sha"]

        if local_sha != remote_sha:
            raise DeploymentValidationError(
                f"Local HEAD SHA ({local_sha}) does not match remote origin/main ({remote_sha}).\n"
                "Please push your changes to origin/main or pull latest commits before deploying."
            )
        print(f"Commit SHA Verified   : {local_sha} (matches origin/main)")

        # Explicit SHA argument check (exact without strip)
        target_sha = local_sha
        if args.sha is not None:
            if args.sha != local_sha:
                raise DeploymentValidationError(
                    f"Requested --sha '{args.sha}' does not match verified HEAD '{local_sha}'."
                )
            target_sha = args.sha

        # Prevent duplicate build in progress for target_sha
        for b in preflight.get("recent_builds", []):
            b_status = b.get("status", "")
            b_sha = b.get("substitutions", {}).get("COMMIT_SHA") or b.get("commitSha")
            if b_status in ("WORKING", "QUEUED", "PENDING") and b_sha == target_sha:
                raise DeploymentValidationError(
                    f"Build {b.get('id')} is already in progress ({b_status}) for SHA {target_sha}. Duplicate dispatch aborted."
                )

        if args.dry_run:
            print("\n[DRY RUN] All deployment gates and remote verifications PASSED.")
            print(f"Would execute: gcloud builds triggers run {TRIGGER_ID} --sha={target_sha} --substitutions=_DEPLOY_SCOPE={DEPLOY_SCOPE_WEB}")
            return 0

        # 3. Trigger Dispatch
        print(f"\n=== [DISPATCH] Triggering Cloud Build Rollout for SHA: {target_sha} ===")
        build_result = trigger_build(target_sha)

        build_id = build_result["id"]
        build_status = build_result.get("status", "QUEUED")
        log_url = build_result.get("logUrl", "N/A")

        print("Rollout trigger request accepted!")
        print(f"Build ID             : {build_id}")
        print(f"Initial Status       : {build_status}")
        print(f"Log URL              : {log_url}")
        print("-" * 65)
        print("OPERATOR INSTRUCTIONS:")
        print(f"To monitor build execution logs live:")
        print(
            f"  gcloud builds log {build_id} --project={PROJECT_ID} --region={REGION} --account={ACCOUNT}"
        )
        print(f"To check ongoing status:")
        print(
            f"  gcloud builds describe {build_id} --project={PROJECT_ID} --region={REGION} --account={ACCOUNT} --format='value(status)'"
        )
        print(
            "\nNOTE: Cloud Build execution is asynchronous. Triggering a build ID does not constitute"
            "\ncompletion or guarantee Cloud Run promotion until all pipeline verification gates PASS."
        )
        return 0

    except DeploymentValidationError as e:
        print(f"\n[DEPLOYMENT FAILED] {e}", file=sys.stderr)
        return 1
    except Exception as e:
        print(f"\n[UNEXPECTED ERROR] {e}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())