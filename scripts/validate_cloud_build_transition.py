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


BUILTIN_SUBSTITUTIONS = {"BUILD_ID", "COMMIT_SHA", "PROJECT_ID"}


def unescaped_cloud_build_dollars(value: str, allowed: set[str]) -> list[str]:
    """Return shell expansions Cloud Build would parse before bash sees them."""
    found: list[str] = []
    index = 0
    while index < len(value):
        if value[index] != "$":
            index += 1
            continue
        if value.startswith("$$", index):
            index += 2
            continue
        if value.startswith("$(", index):
            found.append("$(...)")
            index += 2
            continue
        if value.startswith("${", index):
            end = value.find("}", index + 2)
            if end < 0:
                found.append("${... (unterminated)")
                index = len(value)
                continue
            name = value[index + 2 : end]
            if name not in allowed:
                found.append("${" + name + "}")
            index = end + 1
            continue
        if index + 1 < len(value) and (value[index + 1].isalpha() or value[index + 1] == "_"):
            end = index + 2
            while end < len(value) and (value[end].isalnum() or value[end] == "_"):
                end += 1
            name = value[index + 1 : end]
            if name not in allowed:
                found.append("$" + name)
            index = end
            continue
        if index + 1 < len(value) and (value[index + 1].isdigit() or value[index + 1] in "@*#?!-"):
            found.append("$" + value[index + 1])
            index += 2
            continue
        # A bare dollar not followed by shell syntax is usually a regex anchor.
        index += 1
    return found


def expand_cloud_build_substitutions(value: str, values: dict[str, str]) -> str:
    """Exercise Cloud Build's $$ escaping and custom/built-in substitution forms."""
    sentinel = "\u0000"
    protected = value.replace("$$", sentinel)
    pattern = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)\}|\$([A-Za-z_][A-Za-z0-9_]*)")

    def replace(match: re.Match[str]) -> str:
        name = match.group(1) or match.group(2)
        if name not in values:
            raise ValueError(f"unknown Cloud Build substitution {name}")
        return values[name]

    return pattern.sub(replace, protected).replace(sentinel, "$")


def all_strings(value: object):
    if isinstance(value, str):
        yield value
    elif isinstance(value, dict):
        for nested in value.values():
            yield from all_strings(nested)
    elif isinstance(value, list):
        for nested in value:
            yield from all_strings(nested)


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
        "google_sql_database_instance.postgres",
        "google_sql_database.application",
    )
    for address in expected_state_forgets:
        removed_block = re.search(
            rf"removed \{{\s*from = {re.escape(address)}\s*lifecycle \{{\s*destroy = false\s*\}}\s*\}}",
            terraform,
        )
        if not removed_block:
            errors.append(f"state address {address} must use a removed block with destroy=false")
    if any(item not in terraform for item in (
        'resource "google_service_account" "cloud_build_deployer"',
        'resource "google_project_iam_custom_role" "cloud_build_deployer"',
        'resource "google_project_iam_custom_role" "cloud_build_bucket_operator"',
        'resource "google_storage_bucket" "cloud_build"',
        '"cloudbuild.googleapis.com"',
        '"containeranalysis.googleapis.com"',
    )):
        errors.append("Terraform must prepare the dedicated Cloud Build identity, minimum roles, bucket, and required APIs")
    cloud_build_bucket = re.search(
        r'resource "google_storage_bucket" "cloud_build" \{(.*?)\n\}',
        terraform,
        re.DOTALL,
    )
    if not cloud_build_bucket or any(setting not in cloud_build_bucket.group(1) for setting in (
        'uniform_bucket_level_access = true',
        'public_access_prevention    = "enforced"',
        'force_destroy               = false',
        'prevent_destroy = true',
        'age            = 30',
        'matches_prefix = ["source/"]',
    )):
        errors.append("the dedicated Cloud Build bucket must be private, protected, and expire source staging after 30 days")
    cloud_build_role = re.search(
        r'resource "google_project_iam_custom_role" "cloud_build_deployer" \{(.*?)\n\}',
        terraform,
        re.DOTALL,
    )
    if not cloud_build_role or any(permission in cloud_build_role.group(1) for permission in (
        "secretmanager.",
        "cloudsql.",
        "run.services.delete",
        "run.jobs.",
    )):
        errors.append("Cloud Build deployer role must not read secrets/SQL, delete services, or run Jobs")
    container_analysis_permissions = set(re.findall(r'"(containeranalysis\.[^"]+)"', cloud_build_role.group(1))) if cloud_build_role else set()
    if container_analysis_permissions != {"containeranalysis.occurrences.list"}:
        errors.append("Cloud Build deployer may only list Container Analysis occurrences to verify image provenance")
    if 'toset(["database-migrator-password", "gcp06-sql-inspector-credential"])' not in terraform:
        errors.append("the existing migrator and SQL inspector secret containers must remain in Terraform state with their optional gates off")
    migrator_grant = re.search(
        r'resource "google_secret_manager_secret_iam_member" "migrator_db_access" \{(.*?)\n\}',
        terraform,
        re.DOTALL,
    )
    if not migrator_grant or 'count     = var.enable_foundation ? 1 : 0' not in migrator_grant.group(1):
        errors.append("the existing migrator secret IAM grant must remain managed when enable_database is false")
    mailpit_start = terraform.find('resource "google_compute_instance" "mailpit" {')
    mailpit_end = terraform.find('\nresource ', mailpit_start + 1)
    mailpit_resource = terraform[mailpit_start:mailpit_end] if mailpit_start >= 0 and mailpit_end >= 0 else ""
    if (
        mailpit_resource.count("ignore_changes") != 1
        or "ignore_changes  = [boot_disk[0].initialize_params[0].image]" not in mailpit_resource
        or "prevent_destroy = true" not in mailpit_resource
    ):
        errors.append("Mailpit must ignore only its immutable COS boot image field and reject instance destruction")

    if not config["serviceAccount"].endswith(
        "espacigo-cloud-build-deployer@espacigo-testprod.iam.gserviceaccount.com"
    ):
        errors.append("expected the user-managed deployment service account")
    if ids[0] != "acquire-and-validate-deploy-lock" or "--if-generation-match=0" not in script:
        errors.append("builds must serialize on a GCS lock")
    if "created_epoch <= prior_epoch" not in script:
        errors.append("deployments must reject builds older than the successful main watermark")
    if 'gcloud builds describe "$BUILD_ID" --project="${_PROJECT}" --region="${_REGION}"' not in script:
        errors.append("the regional parent build lookup must include its configured Cloud Build region")
    if '--if-generation-match="$$generation"' not in script:
        errors.append("the lock must be released conditionally")
    required_deploys = {
        "deploy-api-by-digest",
        "deploy-public-web-by-digest",
        "deploy-private-web-by-digest",
    }
    if not required_deploys.issubset(ids) or "@$${digest}" not in script:
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
    if "gcloud iap web add-iam-policy-binding" in script or "_IAP_MEMBERS" in script:
        errors.append("Cloud Build must not edit human IAP access policies")
    if "service-447373903153@gcp-sa-iap.iam.gserviceaccount.com" not in script or "run.invoker" not in script:
        errors.append("Cloud Build must grant the IAP service agent invocation without managing user access")
    if "REPLACE_WITH" in Path("cloudbuild.yaml").read_text(encoding="utf-8"):
        errors.append("Cloud Build configuration must not contain replacement placeholders")
    if any(substitutions[key] != value for key, value in {
        "_CONTRACT_SECRET_VERSION": "1",
        "_PAYMENT_SECRET_VERSION": "1",
        "_MAILPIT_ADDRESS": "10.81.1.2:1025",
    }.items()):
        errors.append("Cloud Build runtime metadata must match the verified secret versions and private Mailpit endpoint")
    if '[[ "${_MAILPIT_ADDRESS}" == 10.81.1.2:1025 ]]' not in script:
        errors.append("Mailpit must stay pinned to the inventory-verified private SMTP address")
    expected_inputs = {
        "_LOCK_BUCKET": "espacigo-testprod-cloud-build-447373903153",
        "_SOURCE_STAGING_BUCKET": "espacigo-testprod-cloud-build-447373903153",
        "_FILES_BUCKET": "espacigo-test-files-447373903153-gcp02",
        "_API_SUBNET": "espacigo-testprod-api-serverless",
        "_WEB_SUBNET": "espacigo-testprod-serverless",
    }
    if any(substitutions[key] != value for key, value in expected_inputs.items()):
        errors.append("Cloud Build must target the inventory-verified buckets and subnet names")
    if script.count("--vpc-egress=private-ranges-only") != 3 or "--vpc-egress=all-traffic" in script:
        errors.append("all services must route only private destinations through VPC; no Cloud NAT is provisioned")

    if substitutions["_FILES_BUCKET"] == "espacigo-test-files-gcp02":
        errors.append("Cloud Build must use the inventory-verified files bucket name and must not use a stale suffix")

    if config.get("options", {}).get("substitutionOption") != "MUST_MATCH" or image_build.get("options", {}).get("substitutionOption") != "MUST_MATCH":
        errors.append("both Cloud Build configs must reject unknown substitutions")

    for label, document in (("cloudbuild.yaml", config), ("cloudbuild-images.yaml", image_build)):
        allowed = set(document.get("substitutions", {})) | BUILTIN_SUBSTITUTIONS
        unescaped = [reference for item in all_strings(document) for reference in unescaped_cloud_build_dollars(item, allowed)]
        if unescaped:
            errors.append(f"{label} contains unescaped shell variables/commands or unknown substitutions: " + ", ".join(sorted(set(unescaped))))

    try:
        probe = expand_cloud_build_substitutions(
            "$${shell_var} $$(printf ok) ${_REGION} $COMMIT_SHA",
            {"_REGION": "southamerica-west1", "COMMIT_SHA": "a" * 40},
        )
        if probe != "${shell_var} $(printf ok) southamerica-west1 " + "a" * 40:
            errors.append("Cloud Build substitution parser escape fixture produced an unexpected result")
    except ValueError as exc:
        errors.append(f"Cloud Build substitution parser escape fixture failed: {exc}")
    if not unescaped_cloud_build_dollars("$shell_var $(echo test) ${unknown}", {"_REGION"}):
        errors.append("Cloud Build variable scanner must reject unescaped shell variables and command substitutions")

    if errors:
        print("\n".join(f"ERROR: {error}" for error in errors), file=sys.stderr)
        return 1
    print(f"Cloud Build transition config is valid ({len(steps)} orchestration steps, 2 provenance-pinned images).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
