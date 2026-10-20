#!/usr/bin/env python3
"""Validate Terraform's full plan against the GCP-07D image-only allowlist."""
import argparse
import hashlib
import json
import re
import sys

IMAGE_PATH = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
ALLOWED = {
    "google_cloud_run_v2_service.api[0]": ("api", "espacigo-api"),
    'google_cloud_run_v2_service.web["public"]': ("web", "espacigo-web"),
    'google_cloud_run_v2_service.web["private"]': ("web", "espacigo-web"),
}


class PolicyError(Exception):
    pass


def image_of(resource, side):
    try:
        containers = resource["template"][0]["containers"]
        if len(containers) != 1:
            raise PolicyError()
        return containers[0]["image"]
    except (KeyError, IndexError, TypeError):
        raise PolicyError() from None


def digest_ok(image, repository):
    return bool(re.fullmatch(re.escape(IMAGE_PATH + repository) + r"@sha256:[0-9a-f]{64}", image or ""))


def mask_unknown(value, unknown):
    if unknown is True:
        return "<UNKNOWN>"
    if isinstance(value, dict) and isinstance(unknown, dict):
        return {key: mask_unknown(child, unknown.get(key)) for key, child in value.items()}
    if isinstance(value, list) and isinstance(unknown, list):
        return [mask_unknown(child, unknown[index] if index < len(unknown) else None)
                for index, child in enumerate(value)]
    return value


def validate(plan, expected, source_sha, release_run_id):
    resources = plan.get("resource_changes")
    if (not isinstance(resources, list) or plan.get("applyable") is not True
            or plan.get("errored") is True or plan.get("complete") is False):
        raise PolicyError()
    summary = []
    seen = set()
    for item in resources:
        actions = item.get("change", {}).get("actions")
        if actions == ["no-op"]:
            continue
        if item.get("mode") == "data" and actions == ["read"]:
            continue
        address = item.get("address")
        if address not in ALLOWED or address in seen or actions != ["update"]:
            raise PolicyError()
        component, repository = ALLOWED[address]
        image = image_of(item["change"]["after"], "after")
        old_image = image_of(item["change"]["before"], "before")
        if old_image == image or not digest_ok(image, repository):
            raise PolicyError()
        if expected.get(component) != image:
            raise PolicyError()
        before = item["change"]["before"]
        after = item["change"]["after"]
        after = mask_unknown(after, item["change"].get("after_unknown", {}))
        before = json.loads(json.dumps(before))
        after = json.loads(json.dumps(after))
        before["template"][0]["containers"][0]["image"] = "<IMAGE>"
        after["template"][0]["containers"][0]["image"] = "<IMAGE>"
        if before != after:
            raise PolicyError()
        seen.add(address)
        summary.append({"address": address, "action": "update", "image": image})
    normalized = {
        "source_sha": source_sha,
        "release_run_id": str(release_run_id),
        "changes": sorted(summary, key=lambda row: row["address"]),
    }
    fingerprint = hashlib.sha256(json.dumps(normalized, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return {**normalized, "fingerprint": fingerprint}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plan_json")
    parser.add_argument("--images", required=True, help="JSON map of selected component to immutable image ref")
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--release-run-id", required=True)
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        with open(args.plan_json, encoding="utf-8") as stream:
            plan = json.load(stream)
        with open(args.images, encoding="utf-8") as stream:
            expected = json.load(stream)
        result = validate(plan, expected, args.source_sha, args.release_run_id)
        with open(args.output, "w", encoding="utf-8") as stream:
            json.dump(result, stream, sort_keys=True, separators=(",", ":"))
            stream.write("\n")
    except (OSError, ValueError, KeyError, TypeError, PolicyError):
        print("Terraform plan rejected by GCP-07D image-only policy", file=sys.stderr)
        return 1
    print(result["fingerprint"])
    return 0


if __name__ == "__main__":
    sys.exit(main())
