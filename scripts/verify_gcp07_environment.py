#!/usr/bin/env python3
"""Fail closed unless GitHub's testprod environment has the agreed protection."""
import json
import os
import sys
import urllib.error
import urllib.request

REVIEWER = "HernanEspinozaDev"
REVIEWER_ID = 176093371
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
    reviewers = [r for r in rules if r.get("type") == "required_reviewers"]
    if len(reviewers) != 1:
        return False
    rule = reviewers[0]
    if rule.get("prevent_self_review") is not True:
        return False
    eligible = rule.get("reviewers") or []
    if (len(eligible) != 1 or eligible[0].get("type") != "User"
            or eligible[0].get("reviewer", {}).get("login") != REVIEWER
            or eligible[0].get("reviewer", {}).get("id") != REVIEWER_ID):
        return False
    branch_policy = environment.get("deployment_branch_policy") or {}
    if branch_policy.get("protected_branches") is not False or branch_policy.get("custom_branch_policies") is not True:
        return False
    branches = policies.get("branch_policies") or []
    return len(branches) == 1 and branches[0].get("name") == "main"


def main():
    output = os.environ.get("GITHUB_OUTPUT")
    token = os.environ.get("GITHUB_TOKEN", "")
    repo = os.environ.get("GITHUB_REPOSITORY", "")
    ready = False
    if token and repo and output:
        base = f"{API}/repos/{repo}/environments/testprod"
        try:
            environment = request_json(base, token)
            policies = request_json(base + "/deployment-branch-policies", token)
            ready = is_ready(environment, policies)
        except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, ValueError, KeyError):
            ready = False
        with open(output, "a", encoding="utf-8") as stream:
            stream.write(f"ready={'true' if ready else 'false'}\n")
    print("testprod protection verified" if ready else "testprod protection unavailable or does not match policy")
    # A negative result is a deliberate fail-closed gate; callers skip privileged jobs.
    return 0


if __name__ == "__main__":
    sys.exit(main())
