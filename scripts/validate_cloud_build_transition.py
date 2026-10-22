#!/usr/bin/env python3
"""Local/CI structural validation for the Cloud Build-only rollout."""
from __future__ import annotations

import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError as exc:  # pragma: no cover - runner dependency diagnostic
    raise SystemExit("PyYAML is required to validate cloudbuild.yaml") from exc


def main() -> int:
    config = yaml.safe_load(Path("cloudbuild.yaml").read_text(encoding="utf-8"))
    image_build = yaml.safe_load(Path("cloudbuild-images.yaml").read_text(encoding="utf-8"))
    steps = config["steps"]
    ids = [step["id"] for step in steps]
    script = "\n".join(arg for step in steps for arg in step.get("args", []))
    image_steps = image_build["steps"]
    image_script = "\n".join(arg for step in image_steps for arg in step.get("args", []))
    substitutions = config["substitutions"]
    errors: list[str] = []

    actions_release = Path(".github/workflows/application-release.yml")
    actions_rollout = Path(".github/workflows/application-rollout.yml")
    if actions_release.exists() or actions_rollout.exists():
        errors.append("the former Actions image publisher and Terraform rollout must stay disabled")
    if not Path(".github/workflows/application-quality.yml").exists() or not Path(".github/workflows/frontend-quality.yml").exists():
        errors.append("GitHub Actions quality workflows must remain enabled")
    if "validate_cloud_build_transition.py" not in Path(".github/workflows/application-quality.yml").read_text(encoding="utf-8"):
        errors.append("the retained application quality workflow must validate both Cloud Build configs")

    terraform = Path("infra/terraform/environments/testprod/gcp/main.tf").read_text(encoding="utf-8")
    forbidden_managed_resources = (
        'resource "google_cloud_run_v2_service" "api"',
        'resource "google_cloud_run_v2_service" "web"',
        'resource "google_sql_database_instance" "postgres"',
        'resource "google_sql_database" "application"',
    )
    if any(resource in terraform for resource in forbidden_managed_resources):
        errors.append("Terraform must not manage application Cloud Run services or Cloud SQL databases")
    expected_state_forgets = (
        "google_cloud_run_v2_service.api",
        "google_cloud_run_v2_service.web",
        "google_cloud_run_v2_service_iam_member.api_invoker",
        "google_cloud_run_v2_service_iam_member.deployer_service_updater",
        "google_cloud_run_v2_service_iam_member.public_web_invoker",
        "google_cloud_run_v2_service_iam_member.private_iap_invoker",
        "google_iap_web_cloud_run_service_iam_member.private_evaluators",
    )
    if any(f"from = {address}" not in terraform for address in expected_state_forgets):
        errors.append("known Cloud Run and IAP state bindings must use non-destroying removed blocks")

    if not config["serviceAccount"].endswith(
        "espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com"
    ):
        errors.append("expected the user-managed deployment service account")
    if ids[0] != "acquire-and-validate-deploy-lock" or "--if-generation-match=0" not in script:
        errors.append("builds must serialize on a GCS lock")
    if "created_epoch <= prior_epoch" not in script:
        errors.append("deployments must reject builds older than the successful main watermark")
    if '--if-generation-match="$generation"' not in script:
        errors.append("the lock must be released conditionally")
    required_deploys = {
        "deploy-api-by-digest",
        "deploy-public-web-by-digest",
        "deploy-private-web-by-digest",
    }
    if not required_deploys.issubset(ids) or "@${digest}" not in script:
        errors.append("API and SSR must deploy by digest")
    if "gcloud builds submit /workspace" not in script or "cloudbuild-images.yaml" not in script:
        errors.append("the orchestrator must wait for a provenance-producing image build")
    if "--target=api" not in image_script or image_steps[0].get("waitFor") != ["-"]:
        errors.append("the API image must use the root Dockerfile api target")
    if not any(step.get("dir") == "frontend" for step in image_steps):
        errors.append("the SSR image must use the frontend build context")
    if image_build.get("options", {}).get("requestedVerifyOption") != "VERIFIED":
        errors.append("the child image build must require Cloud Build provenance")
    if "--show-provenance" not in script or "docker push" in script:
        errors.append("the orchestrator must verify native provenance without explicit docker push")
    if "--port=8080" not in script or "--port=4000" not in script:
        errors.append("API and web ports must stay explicit")
    if "httpGet.path=/health/ready" not in script or "httpGet.path=/healthz" not in script:
        errors.append("API and SSR readiness probes must use their health routes")
    if re.search(r"migrate|terraform apply", script, re.IGNORECASE):
        errors.append("builds must not execute migrations or Terraform apply")
    if not (
        substitutions["_REGION"] == "southamerica-west1"
        and substitutions["_DATABASE_INSTANCE"] == "espacigo-testprod-db"
        and substitutions["_DATABASE_NAME"] == "espacigo_dms_stage_20261020"
    ):
        errors.append("region and external PostgreSQL target must stay pinned")
    if "_CONTRACT_SECRET_VERSION" not in script or "_PAYMENT_SECRET_VERSION" not in script:
        errors.append("runtime secret versions must fail closed")
    if "--iap" not in script or len(image_build["images"]) != 2:
        errors.append("native private IAP and exactly two published images are required")
    if "gcloud iap web add-iam-policy-binding" not in script or "run.invoker" not in script:
        errors.append("Cloud Build must own the private IAP and service invocation bindings")

    if errors:
        print("\n".join(f"ERROR: {error}" for error in errors), file=sys.stderr)
        return 1
    print(f"Cloud Build transition config is valid ({len(steps)} orchestration steps, 2 provenance-pinned images).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
