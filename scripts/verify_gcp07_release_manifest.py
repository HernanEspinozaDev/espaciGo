#!/usr/bin/env python3
"""Validate a release run and its digest manifest without printing untrusted data."""
import json
import os
import re
import subprocess
import sys
import urllib.request

PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"


def affects_application(path):
    return (path.endswith(".go") or path in {"go.mod", "go.sum", "go.work", "go.work.sum",
            "Dockerfile", ".dockerignore", "planning/openapi.yaml"}
            or path.startswith(("cmd/", "internal/", "db/migrations/", "frontend/")))


def main(path):
    try:
        repo = os.environ["REPOSITORY"]
        run_id = os.environ["RELEASE_RUN_ID"]
        token = os.environ["GH_TOKEN"]
        with open(path, encoding="utf-8") as stream:
            manifest = json.load(stream)
        request = urllib.request.Request(
            f"https://api.github.com/repos/{repo}/actions/runs/{run_id}",
            headers={"Accept": "application/vnd.github+json", "Authorization": f"Bearer {token}",
                     "X-GitHub-Api-Version": "2022-11-28"})
        with urllib.request.urlopen(request, timeout=20) as response:
            run = json.loads(response.read())
        if (manifest.get("schema") != 1 or manifest.get("repository") != repo
                or str(manifest.get("run_id")) != str(run_id) or run.get("status") != "completed"
                or run.get("conclusion") != "success" or run.get("event") != "push"
                or run.get("head_branch") != "main" or run.get("path") != ".github/workflows/application-release.yml"
                or manifest.get("source_sha") != run.get("head_sha")
                or not isinstance(manifest.get("contains_migrations"), bool)
                or not isinstance(manifest.get("publish_only"), bool)):
            raise ValueError()
        source_sha = manifest["source_sha"]
        current = subprocess.check_output(["git", "rev-parse", "HEAD"], text=True).strip()
        subprocess.run(["git", "merge-base", "--is-ancestor", source_sha, current], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        later_paths = subprocess.check_output(["git", "diff", "--name-only", source_sha, current], text=True).splitlines()
        if any(affects_application(path) for path in later_paths):
            raise ValueError()
        images = manifest.get("images")
        if not isinstance(images, dict) or not images or set(images) - {"api", "web"}:
            raise ValueError()
        for component, image in images.items():
            name = "espacigo-api" if component == "api" else "espacigo-web"
            if not re.fullmatch(re.escape(PREFIX + name) + r"@sha256:[0-9a-f]{64}", image):
                raise ValueError()
        with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as stream:
            stream.write(f"GCP07D_SOURCE_SHA={manifest['source_sha']}\n")
        os.makedirs(os.path.join(os.environ["RUNNER_TEMP"], "gcp07d"), mode=0o700, exist_ok=True)
        with open(os.path.join(os.environ["RUNNER_TEMP"], "gcp07d", "images.json"), "w") as stream:
            json.dump(images, stream, sort_keys=True, separators=(",", ":"))
    except Exception:
        print("release manifest is invalid, stale, or not from a successful main release", file=sys.stderr)
        return 1
    print("verified successful main release and immutable image digests")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1]))
