"""Shared acceptance rules for immutable GCP-07 application release artifacts."""
import re

PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
SHA = re.compile(r"^[0-9a-f]{40}$")


def images_are_digest_pinned(images, require_both=False):
    if not isinstance(images, dict) or not images or set(images) - {"api", "web"}:
        return False
    if require_both and set(images) != {"api", "web"}:
        return False
    for component, image in images.items():
        if not isinstance(image, str):
            return False
        image_name = "espacigo-api" if component == "api" else "espacigo-web"
        if not re.fullmatch(re.escape(PREFIX + image_name) + r"@sha256:[0-9a-f]{64}", image):
            return False
    return True


def release_run_identity_valid(run, repo):
    return (isinstance(run, dict)
            and run.get("status") == "completed"
            and run.get("conclusion") == "success"
            and run.get("event") in {"push", "workflow_dispatch"}
            and run.get("head_branch") == "main"
            and run.get("path") == ".github/workflows/application-release.yml"
            and isinstance(run.get("head_repository"), dict)
            and run["head_repository"].get("full_name") == repo
            and isinstance(run.get("head_sha"), str)
            and SHA.fullmatch(run["head_sha"]) is not None)


def release_manifest_matches(manifest, run, repo, run_id):
    if (not release_run_identity_valid(run, repo)
            or not isinstance(manifest, dict)
            or type(manifest.get("schema")) is not int or manifest.get("schema") != 1
            or manifest.get("repository") != repo
            or str(manifest.get("run_id")) != str(run_id)
            or manifest.get("source_sha") != run.get("head_sha")
            or not isinstance(manifest.get("contains_migrations"), bool)
            or not isinstance(manifest.get("publish_only"), bool)
            or not images_are_digest_pinned(manifest.get("images"))):
        return False

    if run["event"] == "workflow_dispatch":
        return (manifest["publish_only"] is True
                and manifest["contains_migrations"] is False
                and images_are_digest_pinned(manifest.get("images"), require_both=True))
    return manifest["publish_only"] is False
