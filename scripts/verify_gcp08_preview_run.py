#!/usr/bin/env python3
"""Verify that an approved GCP-08 preview artifact came from a successful main run."""
import json
import os
import urllib.request


def read(url, token):
    request = urllib.request.Request(url, headers={
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    with urllib.request.urlopen(request, timeout=20) as response:
        return json.loads(response.read())


def main():
    try:
        repo = os.environ["REPOSITORY"]
        run_id = os.environ["PREVIEW_RUN_ID"]
        token = os.environ["GH_TOKEN"]
        if not run_id.isdigit():
            raise ValueError()
        base = f"https://api.github.com/repos/{repo}/actions/runs/{run_id}"
        run = read(base, token)
        jobs = read(base + "/jobs?per_page=100", token).get("jobs", [])
        artifacts = read(base + "/artifacts?per_page=100", token).get("artifacts", [])
        valid = (run.get("status") == "completed" and run.get("conclusion") == "success"
                 and run.get("event") == "workflow_dispatch" and run.get("head_branch") == "main"
                 and run.get("path") == ".github/workflows/gcp08-cloud-run-restore.yml"
                 and (run.get("head_repository") or {}).get("full_name") == repo
                 and any(job.get("name") == "restore" and job.get("conclusion") == "success" for job in jobs)
                 and any(item.get("name") == f"gcp08-preview-{run_id}" and not item.get("expired") for item in artifacts))
        if not valid:
            raise ValueError()
    except Exception:
        print("selected preview run is not a successful GCP-08 preview from main")
        return 1
    print("successful main GCP-08 preview run verified")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
