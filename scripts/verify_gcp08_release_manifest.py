#!/usr/bin/env python3
"""Verify a successful main image release and its immutable manifest for GCP-08."""
import json
import os
import re
import subprocess
import urllib.request

API = "https://api.github.com"
PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
SHA = re.compile(r"^[0-9a-f]{40}$")


def request_json(url, token):
    request = urllib.request.Request(url, headers={
        "Accept": "application/vnd.github+json",
        "Authorization": f"Bearer {token}",
        "X-GitHub-Api-Version": "2022-11-28",
    })
    with urllib.request.urlopen(request, timeout=20) as response:
        return json.loads(response.read())


def main(path):
    try:
        repo = os.environ["REPOSITORY"]
        run_id = os.environ["RELEASE_RUN_ID"]
        token = os.environ["GH_TOKEN"]
        baseline = os.environ.get("SCHEMA_BASELINE_SHA", "")
        candidate_evidence = os.environ.get("CANDIDATE_PREFLIGHT_SHA", "")
        run = request_json(f"{API}/repos/{repo}/actions/runs/{run_id}", token)
        jobs = request_json(f"{API}/repos/{repo}/actions/runs/{run_id}/jobs?per_page=100", token).get("jobs", [])
        artifacts = request_json(f"{API}/repos/{repo}/actions/runs/{run_id}/artifacts?per_page=100", token).get("artifacts", [])
        with open(path, encoding="utf-8") as stream:
            manifest = json.load(stream)

        event = run.get("event")
        images = manifest.get("images")
        if (manifest.get("schema") != 1 or manifest.get("repository") != repo
                or str(manifest.get("run_id")) != str(run_id)
                or run.get("status") != "completed" or run.get("conclusion") != "success"
                or event not in {"push", "workflow_dispatch"} or run.get("head_branch") != "main"
                or run.get("path") != ".github/workflows/application-release.yml"
                or (run.get("head_repository") or {}).get("full_name") != repo
                or manifest.get("source_sha") != run.get("head_sha")
                or not SHA.fullmatch(manifest.get("source_sha", ""))
                or not isinstance(manifest.get("contains_migrations"), bool)
                or manifest.get("contains_migrations") is not False
                or manifest.get("publish_only") is not (event == "workflow_dispatch")
                or not re.fullmatch(r"[0-9a-f]{40}", baseline)
                or not re.fullmatch(r"[0-9a-f]{64}", candidate_evidence)
                or not isinstance(images, dict) or set(images) != {"api", "web"}):
            raise ValueError()
        if not any(job.get("name") == "publish digest-pinned images and attestations"
                   and job.get("conclusion") == "success" for job in jobs):
            raise ValueError()
        if not any(item.get("name") == f"gcp07d-release-{run_id}" and not item.get("expired") for item in artifacts):
            raise ValueError()
        subprocess.run(["git", "merge-base", "--is-ancestor", manifest["source_sha"], "HEAD"],
                       check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        later = subprocess.check_output(
            ["git", "diff", "--name-only", f"{manifest['source_sha']}..HEAD"], text=True
        ).splitlines()
        application_paths = ("cmd/", "internal/", "frontend/", "db/migrations/")
        if any(path.endswith((".go",)) or path in {"go.mod", "go.sum", "go.work", "go.work.sum", "Dockerfile", ".dockerignore", "planning/openapi.yaml"}
               or path.startswith(application_paths) for path in later):
            raise ValueError()
        for component, image in images.items():
            name = "espacigo-api" if component == "api" else "espacigo-web"
            if not re.fullmatch(re.escape(PREFIX + name) + r"@sha256:[0-9a-f]{64}", image):
                raise ValueError()
        with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as stream:
            stream.write(f"GCP08_SOURCE_SHA={manifest['source_sha']}\n")
        with open(os.environ["RUNNER_TEMP"] + "/gcp08-images.json", "w", encoding="utf-8") as stream:
            json.dump(images, stream, sort_keys=True, separators=(",", ":"))
        os.chmod(os.environ["RUNNER_TEMP"] + "/gcp08-images.json", 0o600)
    except Exception:
        print("release is invalid, stale, contains migrations, or lacks verified GCP-08 preconditions")
        return 1
    print("eligible main release and immutable digests verified")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(os.sys.argv[1]))
