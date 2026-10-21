#!/usr/bin/env python3
"""Fail closed unless the Terraform plan restores only the named Cloud Run surface."""
import argparse
import hashlib
import json
import re
import sys

PREFIX = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
SERVICE_IMAGES = {
    "google_cloud_run_v2_service.api[0]": ("api", "espacigo-api", "espacigo-test-api", "espacigo-test-api"),
    'google_cloud_run_v2_service.web["public"]': ("web", "espacigo-web", "espacigo-test-public", "espacigo-test-public-web"),
    'google_cloud_run_v2_service.web["private"]': ("web", "espacigo-web", "espacigo-test-private", "espacigo-test-private-web"),
}
BINDING_PREFIXES = (
    "google_cloud_run_v2_service_iam_member.api_invoker[",
    "google_cloud_run_v2_service_iam_member.public_web_invoker[",
    "google_cloud_run_v2_service_iam_member.private_iap_invoker[",
    "google_cloud_run_v2_service_iam_member.deployer_service_updater[",
    "google_iap_web_cloud_run_service_iam_member.private_evaluators[",
)


class PolicyError(Exception):
    pass


def image_of(value):
    try:
        containers = value["template"][0]["containers"]
        if len(containers) != 1:
            raise PolicyError()
        return containers[0]["image"]
    except (KeyError, IndexError, TypeError):
        raise PolicyError() from None


def address_is_binding(address):
    return isinstance(address, str) and any(address.startswith(prefix) for prefix in BINDING_PREFIXES)


def safe_address(address):
    if address.startswith("google_iap_web_cloud_run_service_iam_member.private_evaluators["):
        return "private-iap-member:" + hashlib.sha256(address.encode()).hexdigest()
    return address


def config_fingerprint(value, unknown, normalize_image=False):
    masked = mask_unknown(value, unknown)
    if normalize_image:
        masked = json.loads(json.dumps(masked))
        masked["template"][0]["containers"][0]["image"] = "<IMAGE>"
    return hashlib.sha256(json.dumps(masked, sort_keys=True, separators=(",", ":")).encode()).hexdigest()


def mask_unknown(value, unknown):
    if unknown is True:
        return "<UNKNOWN>"
    if isinstance(value, dict) and isinstance(unknown, dict):
        return {key: mask_unknown(child, unknown.get(key)) for key, child in value.items()}
    if isinstance(value, list) and isinstance(unknown, list):
        return [mask_unknown(child, unknown[index] if index < len(unknown) else None)
                for index, child in enumerate(value)]
    return value


def expected_images(path):
    with open(path, encoding="utf-8") as stream:
        images = json.load(stream)
    if set(images) != {"api", "web"}:
        raise PolicyError()
    for image, name in ((images["api"], "espacigo-api"), (images["web"], "espacigo-web")):
        if not re.fullmatch(re.escape(PREFIX + name) + r"@sha256:[0-9a-f]{64}", image):
            raise PolicyError()
    return images


def validate(plan, images, source_sha, release_run_id, schema_baseline_sha, candidate_preflight_sha, target_addresses):
    resources = plan.get("resource_changes")
    if (not isinstance(resources, list) or plan.get("applyable") is not True
            or plan.get("errored") is True):
        raise PolicyError()
    changes = []
    changed_addresses = set()
    services = set()
    for item in resources:
        action = item.get("change", {}).get("actions")
        address = item.get("address", "")
        if action == ["no-op"]:
            if address not in target_addresses and item.get("mode") != "data":
                continue
            changed_addresses.add(address)
            continue
        if item.get("mode") == "data" and action == ["read"]:
            if address not in {"data.google_sql_database_instance.gcp06_candidate[0]"}:
                raise PolicyError()
            continue
        if address in SERVICE_IMAGES:
            component, repository, name, account = SERVICE_IMAGES[address]
            if action != ["create"] or address in changed_addresses:
                raise PolicyError()
            after = item.get("change", {}).get("after") or {}
            image = image_of(after)
            if image != images[component] or not re.fullmatch(re.escape(PREFIX + repository) + r"@sha256:[0-9a-f]{64}", image):
                raise PolicyError()
            if (after.get("project") != "espacigo-testprod" or after.get("name") != name
                    or after.get("location") != "southamerica-west1" or after.get("deletion_protection") is not True):
                raise PolicyError()
            template = after.get("template", [{}])[0]
            if template.get("service_account") != f"{account}@espacigo-testprod.iam.gserviceaccount.com":
                raise PolicyError()
            if component == "web":
                iap = after.get("iap_enabled")
                if (name == "espacigo-test-public" and iap is True) or (name == "espacigo-test-private" and iap is not True):
                    raise PolicyError()
            services.add(address)
            changes.append({
                "address": address,
                "action": "create",
                "image": image,
                "config_sha256": config_fingerprint(
                    after, item.get("change", {}).get("after_unknown", {}), normalize_image=True
                ),
            })
        elif address in target_addresses and address_is_binding(address):
            if action != ["create"] or address in changed_addresses:
                raise PolicyError()
            before = item.get("change", {}).get("before")
            after = item.get("change", {}).get("after")
            if not isinstance(after, dict):
                raise PolicyError()
            if address.startswith("google_iap_web_cloud_run_service_iam_member.private_evaluators["):
                if not isinstance(before, dict) or before.get("member") != after.get("member"):
                    raise PolicyError()
                if after.get("role") != "roles/iap.httpsResourceAccessor":
                    raise PolicyError()
            changes.append({
                "address": safe_address(address),
                "action": "create",
                "config_sha256": config_fingerprint(after, item.get("change", {}).get("after_unknown", {})),
            })
        else:
            raise PolicyError()
        changed_addresses.add(address)

    if services != set(SERVICE_IMAGES) or not target_addresses.issubset(changed_addresses):
        raise PolicyError()
    normalized = {
        "source_sha": source_sha,
        "release_run_id": str(release_run_id),
        "schema_baseline_sha": schema_baseline_sha,
        "candidate_preflight_sha": candidate_preflight_sha,
        "changes": sorted(changes, key=lambda row: row["address"]),
    }
    fingerprint = hashlib.sha256(json.dumps(normalized, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return {**normalized, "fingerprint": fingerprint}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plan_json")
    parser.add_argument("--images", required=True)
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--release-run-id", required=True)
    parser.add_argument("--schema-baseline-sha", required=True)
    parser.add_argument("--candidate-preflight-sha", required=True)
    parser.add_argument("--targets", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        with open(args.plan_json, encoding="utf-8") as stream:
            plan = json.load(stream)
        with open(args.targets, encoding="utf-8") as stream:
            targets = {line.strip() for line in stream if line.strip()}
        if not re.fullmatch(r"[0-9a-f]{40}", args.schema_baseline_sha):
            raise PolicyError()
        if not re.fullmatch(r"[0-9a-f]{64}", args.candidate_preflight_sha):
            raise PolicyError()
        result = validate(plan, expected_images(args.images), args.source_sha, args.release_run_id,
                          args.schema_baseline_sha, args.candidate_preflight_sha, targets)
        with open(args.output, "w", encoding="utf-8") as stream:
            json.dump(result, stream, sort_keys=True, separators=(",", ":"))
            stream.write("\n")
        print(result["fingerprint"])
        return 0
    except (OSError, ValueError, KeyError, TypeError, PolicyError):
        print("Terraform restore plan rejected by GCP-08 resource and action allowlist", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
