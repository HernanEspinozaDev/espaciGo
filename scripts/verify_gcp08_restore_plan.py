#!/usr/bin/env python3
"""Validate the exact service destinations and IAM bindings in a GCP-08 plan."""
import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
SERVICE_INFO = {
    "google_cloud_run_v2_service.api[0]": {
        "component": "api", "repository": "espacigo-api", "name": "espacigo-test-api", "account": "espacigo-test-api",
    },
    'google_cloud_run_v2_service.web["public"]': {
        "component": "web", "repository": "espacigo-web", "name": "espacigo-test-public", "account": "espacigo-test-public-web",
    },
    'google_cloud_run_v2_service.web["private"]': {
        "component": "web", "repository": "espacigo-web", "name": "espacigo-test-private", "account": "espacigo-test-private-web",
    },
}
IAP_MEMBER_PREFIX = "google_iap_web_cloud_run_service_iam_member.private_evaluators["


class PolicyError(Exception):
    pass


def digest(value):
    return hashlib.sha256(value.encode("utf-8")).hexdigest()


def expected_targets(inputs):
    addresses = {
        *SERVICE_INFO,
        'google_cloud_run_v2_service_iam_member.api_invoker["public"]',
        'google_cloud_run_v2_service_iam_member.api_invoker["private"]',
        "google_cloud_run_v2_service_iam_member.public_web_invoker[0]",
        "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]",
        'google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]',
        'google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]',
        'google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]',
    }
    addresses.update(f"{IAP_MEMBER_PREFIX}{json.dumps(member)}]"
                     for member in set(inputs["iap_evaluator_members"]))
    return addresses


def verify_api_url_expressions(source):
    text = Path(source).read_text(encoding="utf-8")
    required = (
        r'name\s*=\s*"API_INTERNAL_BASE_URL"\s+value\s*=\s*"\$\{google_cloud_run_v2_service\.api\[0\]\.uri\}/api/v1"',
        r'name\s*=\s*"API_PUBLIC_BASE_URL"\s+value\s*=\s*"/api/v1"',
        r'name\s*=\s*"API_SERVICE_AUDIENCE"\s+value\s*=\s*google_cloud_run_v2_service\.api\[0\]\.uri',
    )
    if any(re.search(pattern, text) is None for pattern in required):
        raise PolicyError()


def env_list(resource):
    try:
        containers = resource["template"][0]["containers"]
        if len(containers) != 1:
            raise PolicyError()
        entries = containers[0].get("env", [])
        result = {}
        for entry in entries:
            name = entry.get("name")
            if not isinstance(name, str) or name in result:
                raise PolicyError()
            result[name] = (entry.get("value"), entry)
        return result, containers[0]
    except (KeyError, IndexError, TypeError):
        raise PolicyError() from None


def env_unknown(change, entries, name):
    try:
        after = change["after"]["template"][0]["containers"][0]["env"]
        unknown = change.get("after_unknown", {}).get("template", [{}])[0].get("containers", [{}])[0].get("env", [])
        index = next(i for i, entry in enumerate(after) if entry.get("name") == name)
        return index < len(unknown) and unknown[index].get("value") is True
    except (KeyError, IndexError, TypeError, StopIteration):
        return False


def template_of(resource):
    try:
        templates = resource["template"]
        if len(templates) != 1:
            raise PolicyError()
        return templates[0]
    except (KeyError, TypeError):
        raise PolicyError() from None


def check_unknown_or_exact(change, env, name, expected):
    if name not in env:
        raise PolicyError()
    value = env.get(name, (None,))[0]
    if expected is None:
        if value is None and env_unknown(change, env, name):
            return
        raise PolicyError()
    if value == expected:
        return
    raise PolicyError()


def validate_service(address, item, inputs, images):
    info = SERVICE_INFO[address]
    change = item["change"]
    if change.get("actions") != ["create"]:
        raise PolicyError()
    after = change.get("after") or {}
    component = info["component"]
    image = images[component]
    if not re.fullmatch(re.escape(PREFIX + info["repository"]) + r"@sha256:[0-9a-f]{64}", image):
        raise PolicyError()
    if (after.get("image") is not None
            or after.get("project") != "espacigo-testprod"
            or after.get("name") != info["name"]
            or after.get("location") != "southamerica-west1"
            or after.get("ingress") != "INGRESS_TRAFFIC_ALL"
            or after.get("deletion_protection") is not True):
        raise PolicyError()
    if component == "web":
        is_private = info["name"] == "espacigo-test-private"
        if after.get("iap_enabled") is not is_private:
            raise PolicyError()
    elif after.get("iap_enabled") is True:
        raise PolicyError()

    template = template_of(after)
    if template.get("service_account") != f"{info['account']}@espacigo-testprod.iam.gserviceaccount.com":
        raise PolicyError()
    if template.get("vpc_access", [{}])[0].get("egress") != "PRIVATE_RANGES_ONLY":
        raise PolicyError()
    env, container = env_list(after)
    if container.get("image") != image:
        raise PolicyError()

    if component == "api":
        run_id = "20261020-gcp06a"
        connection = f"espacigo-testprod:southamerica-west1:espacigo-gcp06-{run_id}-pg"
        database = f"espacigo_gcp06_candidate_{run_id}"
        private_origin = inputs.get("private_demo_origin_override") or f"https://{inputs['private_hostname']}"
        fixed_env = {
            "APP_PROFILE": "cloud-test",
            "DATABASE_CONNECTION_MODE": "cloudsql",
            "CLOUD_SQL_INSTANCE_CONNECTION_NAME": connection,
            "DATABASE_NAME": database,
            "DATABASE_PASSWORD_FILE": "/var/secrets/database/password",
            "LOCAL_CONTRACT_ENCRYPTION_KEY_FILE": "/var/secrets/contract/key",
            "LOCAL_PAYMENT_WEBHOOK_SECRET_FILE": "/var/secrets/payment/webhook",
            "DATABASE_USER": "espacigo_api_login",
            "DATABASE_SET_ROLE": "espacigo_runtime",
            "CORS_ALLOWED_ORIGINS": f"https://{inputs['public_hostname']},https://{inputs['private_hostname']}",
            "FRONTEND_PUBLIC_ORIGIN": private_origin,
            "LOCAL_SMTP_ADDR": inputs["mailpit_private_address"],
            "CLOUD_SQL_PRIVATE_IP": "true",
            "LOCAL_BOOKING_TRIAL": "1",
        }
        if any(env.get(key, (None,))[0] != value for key, value in fixed_env.items()):
            raise PolicyError()
        db_volume = next((v for v in template.get("volumes", []) if v.get("name") == "database-secret"), None)
        if not isinstance(db_volume, dict):
            raise PolicyError()
        secret = db_volume.get("secret", [{}])[0]
        item_secret = secret.get("items", [{}])[0]
        if (secret.get("secret") != "espacigo-test-database-password-gcp06"
                or item_secret.get("version") != str(inputs["api_gcp06_candidate_password_version"])
                or item_secret.get("path") != "password"):
            raise PolicyError()
        contract_volume = next((v for v in template.get("volumes", []) if v.get("name") == "contract-secret"), None)
        if not isinstance(contract_volume, dict):
            raise PolicyError()
        contract = contract_volume.get("secret", [{}])[0]
        contract_item = contract.get("items", [{}])[0]
        if (contract.get("secret") != "espacigo-test-contract-encryption-key"
                or contract_item.get("version") != str(inputs["api_contract_encryption_key_version"])
                or contract_item.get("path") != "key"):
            raise PolicyError()
        payment_volume = next((v for v in template.get("volumes", []) if v.get("name") == "payment-secret"), None)
        if not isinstance(payment_volume, dict):
            raise PolicyError()
        payment = payment_volume.get("secret", [{}])[0]
        payment_item = payment.get("items", [{}])[0]
        if (payment.get("secret") != "espacigo-test-payment-webhook-secret"
                or payment_item.get("version") != "latest" or payment_item.get("path") != "webhook"):
            raise PolicyError()
        public = {
            "service": info["name"], "ingress": "INGRESS_TRAFFIC_ALL", "iap_enabled": False,
            "database_instance": connection, "database": database, "database_user": "espacigo_api_login",
            "database_set_role": "espacigo_runtime",
            "database_secret": "espacigo-test-database-password-gcp06",
            "database_secret_version": str(inputs["api_gcp06_candidate_password_version"]),
            "contract_secret": "espacigo-test-contract-encryption-key",
            "contract_secret_version": str(inputs["api_contract_encryption_key_version"]),
            "authenticated_callers": ["public-web runtime service account", "private-web runtime service account"],
            "public_invoker": False,
        }
    else:
        is_private = info["name"] == "espacigo-test-private"
        host = inputs["private_hostname"] if is_private else inputs["public_hostname"]
        private_origin = inputs.get("private_demo_origin_override") or f"https://{inputs['private_hostname']}"
        fixed_env = {
            "FRONTEND_ENTRY_MODE": "private" if is_private else "public",
            "NG_ALLOWED_HOSTS": host,
            "API_PUBLIC_BASE_URL": "/api/v1",
            "FRONTEND_PUBLIC_ORIGIN": f"https://{host}",
            "PRIVATE_DEMO_ORIGIN": private_origin,
            "IAP_JWT_AUDIENCE": f"/projects/{inputs['project_number']}/locations/southamerica-west1/services/espacigo-test-private" if is_private else "",
        }
        if any(env.get(key, (None,))[0] != value for key, value in fixed_env.items()):
            raise PolicyError()
        check_unknown_or_exact(change, env, "API_INTERNAL_BASE_URL", None)
        check_unknown_or_exact(change, env, "API_SERVICE_AUDIENCE", None)
        public = {
            "service": info["name"], "ingress": "INGRESS_TRAFFIC_ALL", "iap_enabled": is_private,
            "frontend_entry_mode": "private" if is_private else "public",
            "allowed_host": host, "frontend_origin": f"https://{host}",
            "api_internal_base": "Cloud Run URI for espacigo-test-api + /api/v1",
            "api_service_audience": "Cloud Run URI for espacigo-test-api",
            "api_public_base_path": "/api/v1",
            "iap_audience": fixed_env["IAP_JWT_AUDIENCE"] if is_private else None,
            "public_invoker": not is_private,
        }

    return {
        "address": address,
        "action": "create",
        "image": image,
        "ingress": "INGRESS_TRAFFIC_ALL",
        "runtime_service_account": f"{info['account']}@espacigo-testprod.iam.gserviceaccount.com",
        "service_policy": public,
        "config_sha256": config_fingerprint(after, change.get("after_unknown", {}), normalize_image=True),
    }


def config_fingerprint(value, unknown, normalize_image=False):
    masked = mask_unknown(value, unknown)
    if normalize_image:
        masked = json.loads(json.dumps(masked))
        masked["template"][0]["containers"][0]["image"] = "<IMAGE>"
    return digest(json.dumps(masked, sort_keys=True, separators=(",", ":")))


def mask_unknown(value, unknown):
    if unknown is True:
        return "<UNKNOWN>"
    if isinstance(value, dict) and isinstance(unknown, dict):
        return {key: mask_unknown(child, unknown.get(key)) for key, child in value.items()}
    if isinstance(value, list) and isinstance(unknown, list):
        return [mask_unknown(child, unknown[index] if index < len(unknown) else None)
                for index, child in enumerate(value)]
    return value


def iap_member_from_address(address):
    if not address.startswith(IAP_MEMBER_PREFIX) or not address.endswith("]"):
        raise PolicyError()
    try:
        member = json.loads(address[len(IAP_MEMBER_PREFIX):-1])
    except ValueError:
        raise PolicyError() from None
    if not isinstance(member, str):
        raise PolicyError()
    return member


def validate_binding(address, after, inputs, action="create"):
    role = "roles/run.invoker"
    if address.startswith("google_cloud_run_v2_service_iam_member.api_invoker["):
        key = json.loads(address.split("[", 1)[1][:-1])
        service = "espacigo-test-api"
        member = f"serviceAccount:espacigo-test-{key}-web@espacigo-testprod.iam.gserviceaccount.com"
        expected_address_keys = {"public", "private"}
        if key not in expected_address_keys:
            raise PolicyError()
    elif address == "google_cloud_run_v2_service_iam_member.public_web_invoker[0]":
        service, member = "espacigo-test-public", "allUsers"
    elif address == "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]":
        service = "espacigo-test-private"
        member = f"serviceAccount:service-{inputs['project_number']}@gcp-sa-iap.iam.gserviceaccount.com"
    elif address.startswith("google_cloud_run_v2_service_iam_member.deployer_service_updater["):
        key = json.loads(address.split("[", 1)[1][:-1])
        service = {"api": "espacigo-test-api", "public": "espacigo-test-public", "private": "espacigo-test-private"}.get(key)
        if service is None:
            raise PolicyError()
        role = "projects/espacigo-testprod/roles/espacigoCloudRunServiceUpdater"
        member = "serviceAccount:espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com"
    elif address.startswith(IAP_MEMBER_PREFIX):
        service = "espacigo-test-private"
        role = "roles/iap.httpsResourceAccessor"
        member = iap_member_from_address(address)
        if member not in set(inputs["iap_evaluator_members"]):
            raise PolicyError()
    else:
        raise PolicyError()

    if (after.get("project") != "espacigo-testprod" or after.get("location") != "southamerica-west1"
            or after.get("name", after.get("cloud_run_service_name")) != service
            or after.get("role") != role or after.get("member") != member):
        raise PolicyError()
    summary = {
        "address": safe_address(address), "action": action,
        "service": service, "role": role,
        "member": member if not address.startswith(IAP_MEMBER_PREFIX) else None,
        "member_sha256": digest(member) if address.startswith(IAP_MEMBER_PREFIX) else None,
        "config_sha256": config_fingerprint(after, {}),
    }
    if summary["member"] is None:
        summary.pop("member")
    if summary["member_sha256"] is None:
        summary.pop("member_sha256")
    return summary


def safe_address(address):
    if address.startswith(IAP_MEMBER_PREFIX):
        return "private-iap-member:" + digest(address)
    return address


def load_images(path):
    with open(path, encoding="utf-8") as stream:
        images = json.load(stream)
    if set(images) != {"api", "web"}:
        raise PolicyError()
    for image, name in ((images["api"], "espacigo-api"), (images["web"], "espacigo-web")):
        if not re.fullmatch(re.escape(PREFIX + name) + r"@sha256:[0-9a-f]{64}", image):
            raise PolicyError()
    return images


def validate(plan, images, inputs, source_sha, release_run_id, schema_baseline_sha, candidate_preflight_sha,
             target_addresses, state_addresses, terraform_config_sha256):
    resources = plan.get("resource_changes")
    if (not isinstance(resources, list) or plan.get("applyable") is not True or plan.get("errored") is True):
        raise PolicyError()
    expected = expected_targets(inputs)
    if target_addresses != expected or not target_addresses.issubset(state_addresses):
        raise PolicyError()
    changes = []
    bindings = []
    changed_addresses = set()
    created_services = set()
    runtime_details = {}
    for item in resources:
        address = item.get("address", "")
        action = item.get("change", {}).get("actions")
        after = item.get("change", {}).get("after")
        if item.get("mode") == "data" and action == ["read"]:
            if address != "data.google_sql_database_instance.gcp06_candidate[0]":
                raise PolicyError()
            continue
        if action == ["no-op"]:
            if address not in target_addresses:
                continue
            if address in SERVICE_INFO:
                raise PolicyError()
            bindings.append(validate_binding(address, after or {}, inputs, action="no-op"))
            changed_addresses.add(address)
            continue
        if address in SERVICE_INFO:
            service_summary = validate_service(address, item, inputs, images)
            created_services.add(address)
            runtime_details[address] = service_summary.pop("service_policy")
            changes.append(service_summary)
        elif address in target_addresses:
            binding_summary = validate_binding(address, after or {}, inputs)
            bindings.append(binding_summary)
            changes.append(binding_summary)
        else:
            raise PolicyError()
        changed_addresses.add(address)

    if created_services != set(SERVICE_INFO) or not target_addresses.issubset(changed_addresses):
        raise PolicyError()
    iap_member_rows = sorted(row["member_sha256"] for row in bindings if "member_sha256" in row)
    runtime = {
        "database_instance": "espacigo-testprod:southamerica-west1:espacigo-gcp06-20261020-gcp06a-pg",
        "database_name": "espacigo_gcp06_candidate_20261020-gcp06a",
        "database_user": "espacigo_api_login",
        "database_set_role": "espacigo_runtime",
        "database_secret_id": "espacigo-test-database-password-gcp06",
        "database_secret_version": str(inputs["api_gcp06_candidate_password_version"]),
        "contract_secret_id": "espacigo-test-contract-encryption-key",
        "contract_secret_version": str(inputs["api_contract_encryption_key_version"]),
        "api_internal_base": "Cloud Run URI for espacigo-test-api + /api/v1",
        "api_service_audience": "Cloud Run URI for espacigo-test-api",
        "public_origin": f"https://{inputs['public_hostname']}",
        "private_origin": f"https://{inputs['private_hostname']}",
        "api_ingress": "INGRESS_TRAFFIC_ALL",
        "web_ingress": "INGRESS_TRAFFIC_ALL",
        "api_public_invoker": False,
        "private_web_iap": True,
        "private_web_iap_audience": f"/projects/{inputs['project_number']}/locations/southamerica-west1/services/espacigo-test-private",
        "private_iap_evaluator_member_hashes": iap_member_rows,
    }
    normalized = {
        "source_sha": source_sha,
        "release_run_id": str(release_run_id),
        "schema_baseline_sha": schema_baseline_sha,
        "candidate_preflight_sha": candidate_preflight_sha,
        "target_count": len(target_addresses),
        "target_scope_sha256": digest("\n".join(sorted(target_addresses))),
        "terraform_config_sha256": terraform_config_sha256,
        "runtime": runtime,
        "services": runtime_details,
        "changes": sorted(changes, key=lambda row: row["address"]),
        "bindings": sorted(bindings, key=lambda row: row["address"]),
    }
    fingerprint = digest(json.dumps(normalized, sort_keys=True, separators=(",", ":")))
    return {**normalized, "fingerprint": fingerprint}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plan_json")
    parser.add_argument("--images", required=True)
    parser.add_argument("--inputs", required=True)
    parser.add_argument("--targets", required=True)
    parser.add_argument("--state-list", required=True)
    parser.add_argument("--terraform-config", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--release-run-id", required=True)
    parser.add_argument("--schema-baseline-sha", required=True)
    parser.add_argument("--candidate-preflight-sha", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        with open(args.plan_json, encoding="utf-8") as stream:
            plan = json.load(stream)
        with open(args.inputs, encoding="utf-8") as stream:
            inputs = json.load(stream)
        with open(args.targets, encoding="utf-8") as stream:
            targets = {line.strip() for line in stream if line.strip()}
        with open(args.state_list, encoding="utf-8") as stream:
            state_addresses = {line.strip() for line in stream if line.strip()}
        if not re.fullmatch(r"[0-9a-f]{40}", args.schema_baseline_sha):
            raise PolicyError()
        if not re.fullmatch(r"[0-9a-f]{64}", args.candidate_preflight_sha):
            raise PolicyError()
        verify_api_url_expressions(args.terraform_config)
        with open(args.terraform_config, "rb") as stream:
            terraform_config_sha256 = hashlib.sha256(stream.read()).hexdigest()
        result = validate(plan, load_images(args.images), inputs, args.source_sha, args.release_run_id,
                          args.schema_baseline_sha, args.candidate_preflight_sha, targets, state_addresses,
                          terraform_config_sha256)
        with open(args.output, "w", encoding="utf-8") as stream:
            json.dump(result, stream, sort_keys=True, separators=(",", ":"))
            stream.write("\n")
        print(result["fingerprint"])
        return 0
    except (OSError, ValueError, KeyError, IndexError, TypeError, PolicyError):
        print("Terraform restore plan rejected by destination, ingress, authentication, state, or resource policy", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
