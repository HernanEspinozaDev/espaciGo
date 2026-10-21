#!/usr/bin/env python3
"""Validate private Terraform inputs and produce only the exact GCP-08 targets."""
import json
import os
import re
import sys

PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"


def main():
    try:
        raw = os.environ["TFVARS_JSON"]
        data = json.loads(raw)
        if not isinstance(data, dict):
            raise ValueError()
        if (data.get("project_id") != "espacigo-testprod" or data.get("region") != "southamerica-west1"
                or data.get("enable_foundation") is not True or data.get("enable_services") is not True
                or data.get("enable_database") is not False or data.get("enable_edge") is not False
                or data.get("enable_mailpit") is not False
                or data.get("enable_gcp06_jobs_reviewed") is not False
                or data.get("api_gcp06_candidate_run_id") != "20261020-gcp06a"
                or not isinstance(data.get("mailpit_private_address"), str) or not data["mailpit_private_address"]
                or not re.fullmatch(r"[1-9][0-9]*", str(data.get("api_gcp06_candidate_password_version", "")))
                or not re.fullmatch(r"[1-9][0-9]*", str(data.get("api_contract_encryption_key_version", "")))
                or data.get("confirm_runtime_secret_versions") is not True
                or data.get("confirm_runtime_database_bootstrap") is not True):
            raise ValueError()
        evaluators = data.get("iap_evaluator_members")
        if not isinstance(evaluators, list) or not evaluators:
            raise ValueError()
        if any(not isinstance(m, str) or not re.fullmatch(r"(?:user|group):[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+", m)
               for m in evaluators):
            raise ValueError()
        images_path = os.path.join(os.environ["RUNNER_TEMP"], "gcp08-images.json")
        with open(images_path, encoding="utf-8") as stream:
            images = json.load(stream)
        if set(images) != {"api", "web"}:
            raise ValueError()
        for key, repo in (("api", "espacigo-api"), ("web", "espacigo-web")):
            if not re.fullmatch(re.escape(PREFIX + repo) + r"@sha256:[0-9a-f]{64}", images[key]):
                raise ValueError()
        data["api_image"] = images["api"]
        data["frontend_image"] = images["web"]
        output = os.path.join(os.environ["RUNNER_TEMP"], "gcp08-terraform.tfvars.json")
        with open(output, "w", encoding="utf-8") as stream:
            json.dump(data, stream, sort_keys=True, separators=(",", ":"))
        os.chmod(output, 0o600)

        addresses = [
            'google_cloud_run_v2_service.api[0]',
            'google_cloud_run_v2_service.web["public"]',
            'google_cloud_run_v2_service.web["private"]',
            'google_cloud_run_v2_service_iam_member.api_invoker["public"]',
            'google_cloud_run_v2_service_iam_member.api_invoker["private"]',
            'google_cloud_run_v2_service_iam_member.public_web_invoker[0]',
            'google_cloud_run_v2_service_iam_member.private_iap_invoker[0]',
            'google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]',
            'google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]',
            'google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]',
        ]
        addresses.extend(f'google_iap_web_cloud_run_service_iam_member.private_evaluators[{json.dumps(m)}]'
                         for m in sorted(set(evaluators)))
        target_path = os.path.join(os.environ["RUNNER_TEMP"], "gcp08-targets.txt")
        with open(target_path, "w", encoding="utf-8") as stream:
            stream.write("\n".join(addresses) + "\n")
        os.chmod(target_path, 0o600)
        with open(os.environ["GITHUB_ENV"], "a", encoding="utf-8") as stream:
            stream.write(f"GCP08_TFVARS={output}\nGCP08_TARGETS={target_path}\n")
    except Exception:
        print("private restore inputs do not satisfy the exact candidate and runtime preconditions", file=sys.stderr)
        return 1
    print("candidate, runtime-secret versions, access list, and digest inputs passed structural checks")
    return 0


if __name__ == "__main__":
    sys.exit(main())
