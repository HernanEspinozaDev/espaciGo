#!/usr/bin/env python3
"""Fail closed unless GitHub's testprod environment has the agreed protection."""
import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

from verify_gcp08_runtime_preflight import load_valid

API = "https://api.github.com"


def request_json(url, token):
    req = urllib.request.Request(url, headers={
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    with urllib.request.urlopen(req, timeout=20) as response:
        return json.loads(response.read())


def is_ready(environment, policies):
    rules = environment.get("protection_rules") or []
    # Required reviewers gate every environment job, which would make each
    # automatic release require a manual approval. Initial rollout approval is
    # handled by the explicit rollout gate and manual apply flow instead.
    if any(r.get("type") == "required_reviewers" for r in rules):
        return False
    branch_policy = environment.get("deployment_branch_policy") or {}
    if branch_policy.get("protected_branches") is not False or branch_policy.get("custom_branch_policies") is not True:
        return False
    branches = policies.get("branch_policies") or []
    return len(branches) == 1 and branches[0].get("name") == "main"


def published_release_exists(repo, run_id, token):
    base = f"{API}/repos/{repo}/actions/runs/{run_id}"
    run = request_json(base, token)
    if (run.get("status") != "completed" or run.get("conclusion") != "success"
            or run.get("event") != "push" or run.get("head_branch") != "main"
            or run.get("path") != ".github/workflows/application-release.yml"
            or (run.get("head_repository") or {}).get("full_name") != repo):
        return False
    jobs = request_json(base + "/jobs?per_page=100", token).get("jobs", [])
    if not any(job.get("name") == "publish digest-pinned images and attestations"
               and job.get("conclusion") == "success" for job in jobs):
        return False
    artifacts = request_json(base + "/artifacts?per_page=100", token).get("artifacts", [])
    return any(a.get("name") == f"gcp07d-release-{run_id}" and not a.get("expired") for a in artifacts)


def successful_preview_exists(repo, run_id, token):
    base = f"{API}/repos/{repo}/actions/runs/{run_id}"
    run = request_json(base, token)
    if (run.get("status") != "completed" or run.get("conclusion") != "success"
            or run.get("event") != "workflow_dispatch" or run.get("head_branch") != "main"
            or run.get("path") != ".github/workflows/application-rollout.yml"
            or (run.get("head_repository") or {}).get("full_name") != repo):
        return False
    artifacts = request_json(base + "/artifacts?per_page=100", token).get("artifacts", [])
    return any(a.get("name") == f"gcp07d-preview-{run_id}" and not a.get("expired") for a in artifacts)


def event_gate_ready(event, mode, publish_enabled, rollout_enabled,
                     confirmation="", preview_run_id="", fingerprint=""):
    # application-release.yml uses this helper on push only to validate the
    # pre-existing Environment. Its own job condition gates publication.
    if event == "push":
        return True
    if not publish_enabled:
        return False
    if event == "workflow_run":
        return mode == "auto" and rollout_enabled
    if event == "workflow_dispatch" and mode == "manual-publish":
        return publish_enabled
    if event == "workflow_dispatch" and mode == "preview":
        return True
    if event == "workflow_dispatch" and mode == "apply":
        return (rollout_enabled and confirmation == "APPLY GCP-07D"
                and preview_run_id.isdigit()
                and re.fullmatch(r"[0-9a-f]{64}", fingerprint) is not None)
    if event == "workflow_dispatch" and mode == "bootstrap-preview":
        return publish_enabled
    if event == "workflow_dispatch" and mode == "bootstrap-apply":
        return (rollout_enabled and confirmation == "APPLY GCP-08 INITIAL RESTORE"
                and preview_run_id.isdigit()
                and re.fullmatch(r"[0-9a-f]{64}", fingerprint) is not None)
    return False


def v67_runtime_ready(password_version, preflight_sha, preflight_path, repo, token):
    if not load_valid(preflight_path, preflight_sha, password_version):
        return False
    try:
        with open(preflight_path, encoding="utf-8") as stream:
            record = json.load(stream)
        review = record["review"]
        pull = request_json(f"{API}/repos/{repo}/pulls/{review['pr']}", token)
        reviews = request_json(f"{API}/repos/{repo}/pulls/{review['pr']}/reviews?per_page=100", token)
        if not (pull.get("merged") is True and (pull.get("base") or {}).get("ref") == "main"
                and (pull.get("head") or {}).get("sha") == review["commit"]
                and any(item.get("user", {}).get("login") == "HernanEspinozaDev"
                        and item.get("state") == "APPROVED"
                        and item.get("commit_id") == review["commit"] for item in reviews)):
            return False
        for ref in record["evidence_refs"]:
            match = re.fullmatch(r"https://github\.com/HernanEspinozaDev/espaciGo/actions/runs/([1-9][0-9]*)", ref)
            if not match:
                continue
            run = request_json(f"{API}/repos/{repo}/actions/runs/{match.group(1)}", token)
            if (run.get("status") != "completed" or run.get("conclusion") != "success"
                    or run.get("head_sha") != review["commit"]
                    or (run.get("repository") or {}).get("full_name") != repo):
                return False
        return True
    except (OSError, ValueError, KeyError, TypeError, urllib.error.HTTPError, urllib.error.URLError, TimeoutError):
        return False


def main():
    output = os.environ.get("GITHUB_OUTPUT")
    token = os.environ.get("GITHUB_TOKEN", "")
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    event = os.environ.get("GITHUB_EVENT_NAME", "")
    mode = os.environ.get("MODE", "")
    publish_enabled = os.environ.get("PUBLISH_ENABLED") == "true"
    rollout_enabled = os.environ.get("ROLLOUT_ENABLED") == "true"
    ready = False
    eligible = event_gate_ready(
        event, mode, publish_enabled, rollout_enabled,
        os.environ.get("CONFIRMATION", ""), os.environ.get("PREVIEW_RUN_ID", ""),
        os.environ.get("FINGERPRINT", ""))
    if eligible and mode in {"auto", "preview", "apply", "bootstrap-preview", "bootstrap-apply"}:
        eligible = v67_runtime_ready(
            os.environ.get("V67_RUNTIME_PASSWORD_VERSION", ""),
            os.environ.get("V67_RUNTIME_PREFLIGHT_SHA", ""),
            os.environ.get("V67_RUNTIME_PREFLIGHT_PATH", str(
                Path(__file__).resolve().parents[1] / "planning/gcp/gcp08_v67_runtime_preflight.json")),
            repo, token)
    if token and repo and output and eligible:
        base = f"{API}/repos/{repo}/environments/testprod"
        try:
            environment = request_json(base, token)
            policies = request_json(base + "/deployment-branch-policies", token)
            ready = is_ready(environment, policies)
            if ready and event == "workflow_run":
                run_id = os.environ.get("UPSTREAM_RUN_ID", "")
                ready = run_id.isdigit() and published_release_exists(repo, run_id, token)
            elif ready and event == "workflow_dispatch" and mode in {"preview", "apply", "bootstrap-preview"}:
                run_id = os.environ.get("RELEASE_RUN_ID", "")
                ready = run_id.isdigit() and published_release_exists(repo, run_id, token)
                if ready and mode == "apply":
                    preview_run_id = os.environ.get("PREVIEW_RUN_ID", "")
                    ready = preview_run_id.isdigit() and successful_preview_exists(repo, preview_run_id, token)
            elif ready and event == "workflow_dispatch" and mode == "bootstrap-apply":
                run_id = os.environ.get("RELEASE_RUN_ID", "")
                preview_run_id = os.environ.get("PREVIEW_RUN_ID", "")
                ready = (run_id.isdigit() and published_release_exists(repo, run_id, token)
                         and preview_run_id.isdigit() and successful_preview_exists(repo, preview_run_id, token))
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, ValueError, KeyError, TypeError):
            ready = False
    if output:
        with open(output, "a", encoding="utf-8") as stream:
            stream.write(f"ready={'true' if ready else 'false'}\n")
    print("testprod protection verified" if ready else "testprod protection unavailable or does not match policy")
    # A negative result is a deliberate fail-closed gate; callers skip privileged jobs.
    return 0


if __name__ == "__main__":
    sys.exit(main())
