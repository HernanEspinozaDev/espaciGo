#!/usr/bin/env python3
"""Fail closed unless GitHub's testprod environment has the agreed protection."""
import json
import os
import re
import sys
import urllib.error
import urllib.request

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


def main():
    output = os.environ.get("GITHUB_OUTPUT")
    token = os.environ.get("GITHUB_TOKEN", "")
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    event = os.environ.get("GITHUB_EVENT_NAME", "")
    mode = os.environ.get("MODE", "")
    publish_enabled = os.environ.get("PUBLISH_ENABLED") == "true"
    rollout_enabled = os.environ.get("ROLLOUT_ENABLED") == "true"
    ready = False
    gates_ready = publish_enabled and (
        (event == "workflow_run" and mode == "auto" and rollout_enabled)
        or (event == "workflow_dispatch" and mode == "preview")
        or (event == "workflow_dispatch" and mode == "apply" and rollout_enabled
            and os.environ.get("CONFIRMATION") == "APPLY GCP-07D"
            and os.environ.get("PREVIEW_RUN_ID", "").isdigit()
            and re.fullmatch(r"[0-9a-f]{64}", os.environ.get("FINGERPRINT", "")))
    )
    if token and repo and output and gates_ready:
        base = f"{API}/repos/{repo}/environments/testprod"
        try:
            environment = request_json(base, token)
            policies = request_json(base + "/deployment-branch-policies", token)
            ready = is_ready(environment, policies)
            if ready:
                run_id = (os.environ.get("UPSTREAM_RUN_ID", "") if event == "workflow_run"
                          else os.environ.get("RELEASE_RUN_ID", ""))
                ready = run_id.isdigit() and published_release_exists(repo, run_id, token)
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, ValueError, KeyError, TypeError):
            ready = False
        with open(output, "a", encoding="utf-8") as stream:
            stream.write(f"ready={'true' if ready else 'false'}\n")
    print("testprod protection verified" if ready else "testprod protection unavailable or does not match policy")
    # A negative result is a deliberate fail-closed gate; callers skip privileged jobs.
    return 0


if __name__ == "__main__":
    sys.exit(main())
