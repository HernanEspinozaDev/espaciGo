#!/usr/bin/env python3
"""Validate Terraform's full plan against the GCP-07D image-only allowlist."""
import argparse
import hashlib
import json
import re
import sys
from pathlib import Path

IMAGE_PATH = "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/"
ALLOWED = {
    "google_cloud_run_v2_service.api[0]": ("api", "espacigo-api"),
    'google_cloud_run_v2_service.web["public"]': ("web", "espacigo-web"),
    'google_cloud_run_v2_service.web["private"]': ("web", "espacigo-web"),
}
BOOTSTRAP_IAM = {
    'google_cloud_run_v2_service_iam_member.api_invoker["public"]': (
        "espacigo-test-api", "roles/run.invoker",
        "serviceAccount:espacigo-test-public-web@espacigo-testprod.iam.gserviceaccount.com"),
    'google_cloud_run_v2_service_iam_member.api_invoker["private"]': (
        "espacigo-test-api", "roles/run.invoker",
        "serviceAccount:espacigo-test-private-web@espacigo-testprod.iam.gserviceaccount.com"),
    'google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]': (
        "espacigo-test-api", "projects/espacigo-testprod/roles/espacigoCloudRunServiceUpdater",
        "serviceAccount:espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com"),
    'google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]': (
        "espacigo-test-public", "projects/espacigo-testprod/roles/espacigoCloudRunServiceUpdater",
        "serviceAccount:espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com"),
    'google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]': (
        "espacigo-test-private", "projects/espacigo-testprod/roles/espacigoCloudRunServiceUpdater",
        "serviceAccount:espacigo-test-deployer@espacigo-testprod.iam.gserviceaccount.com"),
    "google_cloud_run_v2_service_iam_member.public_web_invoker[0]": (
        "espacigo-test-public", "roles/run.invoker", "allUsers"),
    "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]": (
        "espacigo-test-private", "roles/run.invoker",
        "service-447373903153@gcp-sa-iap.iam.gserviceaccount.com"),
    "google_secret_manager_secret_iam_member.v67_api_secret_access[0]": (
        "espacigo-test-database-password-v67", "roles/secretmanager.secretAccessor",
        "serviceAccount:espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com"),
    'google_iap_web_cloud_run_service_iam_member.private_evaluators["user:pasteleriahijitos@gmail.com"]': (
        "projects/espacigo-testprod/iap_web/cloud_run-southamerica-west1/services/espacigo-test-private",
        "roles/iap.httpsResourceAccessor", "user:pasteleriahijitos@gmail.com"),
}
API_ENV_NAMES = {
    "APP_PROFILE", "LOCAL_VERIFICATION_IP_LIMIT", "DATABASE_CONNECTION_MODE",
    "CLOUD_SQL_INSTANCE_CONNECTION_NAME", "DATABASE_NAME", "DATABASE_PASSWORD_FILE",
    "LOCAL_CONTRACT_ENCRYPTION_KEY_FILE", "LOCAL_PAYMENT_WEBHOOK_SECRET_FILE",
    "DATABASE_USER", "CORS_ALLOWED_ORIGINS", "DATABASE_SET_ROLE",
    "FRONTEND_PUBLIC_ORIGIN", "DATABASE_POOL_MAX_CONNS", "CLOUD_SQL_PRIVATE_IP",
    "LOCAL_BOOKING_TRIAL", "LOCAL_SMTP_ADDR", "M03_EVIDENCE_DIR",
    "CLOUD_STORAGE_BUCKET", "CLOUD_STORAGE_OBJECT_PREFIX", "BLOB_STORE_BACKEND",
    "LOCAL_PRIVACY_REPLAY_BACKEND", "LOCAL_PRIVACY_REPLAY_BUCKET", "LOCAL_PRIVACY_REPLAY_PREFIX",
}
WEB_ENV_NAMES = {
    "FRONTEND_ENTRY_MODE", "NG_ALLOWED_HOSTS", "API_INTERNAL_BASE_URL",
    "API_PUBLIC_BASE_URL", "API_SERVICE_AUDIENCE", "FRONTEND_PUBLIC_ORIGIN",
    "PRIVATE_DEMO_ORIGIN", "IAP_JWT_AUDIENCE",
}
API_VOLUMES = {"database-secret", "contract-secret", "payment-secret"}
COMPUTED_ROOT_FIELDS = {
    "create_time", "update_time", "uid", "etag", "generation", "observed_generation",
    "latest_created_revision", "latest_ready_revision", "reconciling", "satisfies_pzs",
    "uri", "url", "conditions", "terminal_condition", "traffic_statuses",
    "effective_annotations", "effective_labels", "id",
}


def _empty(value):
    return value is None or value is False or value == "" or value == [] or value == {}


def _unknown_paths(value, path=""):
    if value is True:
        yield path
    elif isinstance(value, dict):
        for key, child in value.items():
            yield from _unknown_paths(child, f"{path}.{key}" if path else key)
    elif isinstance(value, list):
        for index, child in enumerate(value):
            yield from _unknown_paths(child, f"{path}[{index}]")


def _unknowns_are_computed_or_referenced(change):
    for path in _unknown_paths(change.get("after_unknown") or {}):
        normalized = re.sub(r"\[\d+\]", "[*]", path)
        root = path.split(".", 1)[0]
        if root in COMPUTED_ROOT_FIELDS:
            continue
        if normalized == "template[*].revision":
            continue
        if re.fullmatch(r"template\[\*\]\.containers\[\*\]\.env\[\*\]\.value", normalized):
            continue
        return False
    return True


def web_api_references_are_exact():
    main_tf = Path(__file__).resolve().parents[1] / "infra/terraform/environments/testprod/gcp/main.tf"
    try:
        source = main_tf.read_text(encoding="utf-8")
    except OSError:
        return False
    expected = (
        r'name\s*=\s*"API_INTERNAL_BASE_URL"\s+value\s*=\s*"\$\{google_cloud_run_v2_service\.api\[0\]\.uri\}/api/v1"',
        r'name\s*=\s*"API_SERVICE_AUDIENCE"\s+value\s*=\s*google_cloud_run_v2_service\.api\[0\]\.uri',
    )
    return all(len(re.findall(pattern, source)) == 1 for pattern in expected)


def _env_map(container, expected_names, allow_unknown_names=()):
    env = container.get("env") or []
    result = {}
    for item in env:
        if not isinstance(item, dict) or set(item) - {"name", "value"}:
            return None
        name = item.get("name")
        if (name not in expected_names or name in result
                or (not isinstance(item.get("value"), str) and not (name in allow_unknown_names and item.get("value") is None))
                or (name in allow_unknown_names and item.get("value") is not None)):
            return None
        result[name] = item["value"]
    return result if set(result) == expected_names else None


def _has_only_configured_root_fields(after, api):
    configured = {"name", "project", "location", "ingress", "deletion_protection", "template", "iap_enabled"}
    computed_or_unset = {
        "annotations", "labels", "description", "client", "client_version", "custom_audiences",
        "binary_authorization", "encryption_key", "launch_stage", "scaling", "traffic",
        "build_config", "satisfies_pzs", "serverless_network_endpoint_group", "expire_time",
        "create_time", "update_time", "uid", "etag", "generation", "observed_generation",
        "latest_created_revision", "latest_ready_revision", "reconciling", "uri", "url",
        "conditions", "terminal_condition", "traffic_statuses", "id", "effective_annotations",
        "effective_labels",
    }
    if set(after) - configured - computed_or_unset:
        return False
    return all(_empty(after[key]) for key in set(after) & computed_or_unset)


def _template_config_matches(change, api, database_password_version):
    after = change.get("after", {})
    templates = after.get("template")
    if not isinstance(templates, list) or len(templates) != 1:
        return False
    template = templates[0]
    if not isinstance(template, dict):
        return False
    required_template = {"service_account", "max_instance_request_concurrency", "scaling", "vpc_access", "containers"}
    if api:
        required_template.add("volumes")
    optional_empty = {"annotations", "labels", "max_instance_request_timeout", "session_affinity",
                      "encryption_key", "revision", "service_scaling",
                      "gpu_zonal_redundancy_disabled", "health_check_disabled"}
    optional_values = {"timeout": "300s", "execution_environment": "EXECUTION_ENVIRONMENT_GEN2"}
    if not api:
        optional_empty.add("volumes")
    if not required_template <= set(template):
        return False
    if set(template) - required_template - optional_empty - set(optional_values):
        return False
    if any(not _empty(template[k]) for k in set(template) & optional_empty):
        return False
    if any(template.get(key) not in (None, value) for key, value in optional_values.items()):
        return False
    service = "api" if api else ("private-web" if after.get("iap_enabled") is True else "public-web")
    if template.get("service_account") != f"espacigo-test-{service}@espacigo-testprod.iam.gserviceaccount.com":
        return False
    if template.get("max_instance_request_concurrency") != (80 if api else 40):
        return False
    scaling = template.get("scaling")
    if (not isinstance(scaling, list) or len(scaling) != 1 or not isinstance(scaling[0], dict)
            or set(scaling[0]) - {"min_instance_count", "max_instance_count"}):
        return False
    if scaling[0] != ({"min_instance_count": 1, "max_instance_count": 1} if api else {"min_instance_count": 0, "max_instance_count": 1}):
        return False
    vpc = template.get("vpc_access")
    if (not isinstance(vpc, list) or len(vpc) != 1 or not isinstance(vpc[0], dict)
            or set(vpc[0]) - {"egress", "network_interfaces"}):
        return False
    if vpc[0].get("egress") != "PRIVATE_RANGES_ONLY":
        return False
    interfaces = vpc[0].get("network_interfaces")
    if not isinstance(interfaces, list) or len(interfaces) != 1:
        return False
    network = interfaces[0]
    if not isinstance(network, dict):
        return False
    allowed_subnets = {"espacigo-testprod-serverless", "espacigo-testprod-api-serverless"} if api else {"espacigo-testprod-serverless"}
    if set(network) - {"network", "subnetwork", "tag"} or network.get("network") != "espacigo-testprod" or network.get("subnetwork") not in allowed_subnets or network.get("tag") not in (None, ""):
        return False
    containers = template.get("containers")
    if not isinstance(containers, list) or len(containers) != 1:
        return False
    container = containers[0]
    if not isinstance(container, dict):
        return False
    container_allowed = {"image", "resources", "volume_mounts", "env", "name", "command", "args", "ports", "working_dir", "depends_on", "startup_probe", "liveness_probe", "base_image_uri"}
    if set(container) - container_allowed:
        return False
    if not _empty(container.get("name")):
        return False
    for forbidden in ("command", "args", "ports", "working_dir", "depends_on", "startup_probe", "liveness_probe", "base_image_uri"):
        if not _empty(container.get(forbidden)):
            return False
    expected_env = API_ENV_NAMES if api else WEB_ENV_NAMES
    unknown_env_names = set()
    after_unknown = change.get("after_unknown") or {}
    try:
        unknown_items = after_unknown["template"][0]["containers"][0]["env"]
        for index, item in enumerate(container.get("env") or []):
            if (index < len(unknown_items) and isinstance(unknown_items[index], dict)
                    and unknown_items[index].get("value") is True):
                unknown_env_names.add(item.get("name"))
    except (KeyError, IndexError, TypeError):
        pass
    allowed_unknown_env = {"API_INTERNAL_BASE_URL", "API_SERVICE_AUDIENCE"} if not api else set()
    if (not unknown_env_names <= allowed_unknown_env
            or (not api and unknown_env_names != allowed_unknown_env)
            or (not api and not web_api_references_are_exact())):
        return False
    env = _env_map(container, expected_env, unknown_env_names)
    if env is None:
        return False
    if api:
        if (env.get("APP_PROFILE") != "cloud-test"
                or env.get("LOCAL_VERIFICATION_IP_LIMIT") != "20"
                or env.get("DATABASE_CONNECTION_MODE") != "cloudsql"
                or env.get("CLOUD_SQL_INSTANCE_CONNECTION_NAME") != "espacigo-testprod:southamerica-west1:espacigo-testprod-db"
                or env.get("DATABASE_NAME") != "espacigo_dms_stage_20261020"
                or env.get("DATABASE_PASSWORD_FILE") != "/var/secrets/database/password"
                or env.get("LOCAL_CONTRACT_ENCRYPTION_KEY_FILE") != "/var/secrets/contract/key"
                or env.get("LOCAL_PAYMENT_WEBHOOK_SECRET_FILE") != "/var/secrets/payment/webhook"
                or env.get("DATABASE_USER") != "espacigo_api_login"
                or env.get("DATABASE_SET_ROLE") != "espacigo_runtime"
                or env.get("DATABASE_POOL_MAX_CONNS") != "4"
                or env.get("CLOUD_SQL_PRIVATE_IP") != "true"
                or env.get("LOCAL_BOOKING_TRIAL") != "1"
                or env.get("M03_EVIDENCE_DIR") != "/var/lib/espacigo/evidence"
                or env.get("BLOB_STORE_BACKEND") != "gcs"
                or env.get("LOCAL_PRIVACY_REPLAY_BACKEND") != "gcs"):
            return False
        if (env.get("CORS_ALLOWED_ORIGINS") != "https://espacigo.testingpage.store,https://espacigo-demo.testingpage.store"
                or env.get("FRONTEND_PUBLIC_ORIGIN") != "https://espacigo-demo.testingpage.store"
                or not re.fullmatch(r"espacigo-test-files-[a-z0-9][a-z0-9-]{2,20}", env.get("CLOUD_STORAGE_BUCKET", ""))
                or env.get("CLOUD_STORAGE_OBJECT_PREFIX") not in {"", "g03o-1017"}
                or env.get("LOCAL_PRIVACY_REPLAY_PREFIX") not in {"", "g03o-1017/privacy-replay"}
                or not re.fullmatch(r"(?:[0-9]{1,3}\.){3}[0-9]{1,3}:1025", env.get("LOCAL_SMTP_ADDR", ""))):
            return False
        volumes = template.get("volumes")
        if not isinstance(volumes, list) or {v.get("name") for v in volumes if isinstance(v, dict)} != API_VOLUMES or len(volumes) != 3:
            return False
        secrets = {}
        for volume in volumes:
            if (not isinstance(volume, dict) or set(volume) - {"name", "secret"}
                    or not isinstance(volume.get("secret"), list) or len(volume["secret"]) != 1):
                return False
            secret = volume["secret"][0]
            if (not isinstance(secret, dict) or set(secret) - {"secret", "items"}
                    or not isinstance(secret.get("items"), list) or len(secret["items"]) != 1):
                return False
            item = secret["items"][0]
            if not isinstance(item, dict) or set(item) - {"path", "version"}:
                return False
            secrets[volume["name"]] = (secret["secret"], item.get("path"), item.get("version"))
        if secrets.get("database-secret") != ("espacigo-test-database-password-v67", "password", database_password_version):
            return False
        if secrets.get("contract-secret", (None,))[0] != "espacigo-test-contract-encryption-key" or secrets["contract-secret"][1] != "key" or not re.fullmatch(r"[1-9][0-9]*", str(secrets["contract-secret"][2])):
            return False
        if secrets.get("payment-secret") != ("espacigo-test-payment-webhook-secret", "webhook", "latest"):
            return False
        mounts = container.get("volume_mounts") or []
        if any(not isinstance(mount, dict) or set(mount) - {"name", "mount_path"} for mount in mounts):
            return False
        expected_mounts = {("database-secret", "/var/secrets/database"), ("contract-secret", "/var/secrets/contract"), ("payment-secret", "/var/secrets/payment")}
        if {(m.get("name"), m.get("mount_path")) for m in mounts if isinstance(m, dict)} != expected_mounts or len(mounts) != 3:
            return False
        resources = container.get("resources")
        if (not isinstance(resources, list) or len(resources) != 1
                or not isinstance(resources[0], dict)
                or set(resources[0]) - {"limits", "cpu_idle", "startup_cpu_boost"}
                or resources[0].get("limits") != {"cpu": "1", "memory": "1Gi"}
                or resources[0].get("cpu_idle") is not False
                or not _empty(resources[0].get("startup_cpu_boost"))):
            return False
    else:
        if template.get("volumes") not in (None, []) or container.get("volume_mounts") not in (None, []):
            return False
        mode = "private" if after.get("iap_enabled") else "public"
        if env.get("FRONTEND_ENTRY_MODE") != mode or env.get("API_PUBLIC_BASE_URL") != "/api/v1":
            return False
        expected_host = "espacigo-demo.testingpage.store" if mode == "private" else "espacigo.testingpage.store"
        if env.get("NG_ALLOWED_HOSTS") != expected_host:
            return False
        if env.get("FRONTEND_PUBLIC_ORIGIN") != f"https://{expected_host}":
            return False
        if env.get("API_INTERNAL_BASE_URL") is not None:
            return False
        if env.get("API_SERVICE_AUDIENCE") is not None:
            return False
        if env.get("PRIVATE_DEMO_ORIGIN") != "https://espacigo-demo.testingpage.store":
            return False
        if mode == "private" and not env.get("IAP_JWT_AUDIENCE", "").endswith("/services/espacigo-test-private"):
            return False
        if mode == "public" and env.get("IAP_JWT_AUDIENCE") != "":
            return False
        resources = container.get("resources")
        if (not isinstance(resources, list) or len(resources) != 1
                or not isinstance(resources[0], dict)
                or set(resources[0]) - {"limits", "cpu_idle", "startup_cpu_boost"}
                or resources[0].get("limits") != {"cpu": "1", "memory": "1Gi"}
                or resources[0].get("cpu_idle") is not True
                or not _empty(resources[0].get("startup_cpu_boost"))):
            return False
    return True


class PolicyError(Exception):
    pass


def initial_iam_scope(address, resource_name):
    if address.startswith("google_secret_manager_secret_iam_member."):
        return {"project": "espacigo-testprod", "secret_id": resource_name}
    if address.startswith("google_iap_web_cloud_run_service_iam_member."):
        return {"project": "espacigo-testprod", "location": "southamerica-west1",
                "cloud_run_service_name": resource_name}
    return {"project": "espacigo-testprod", "location": "southamerica-west1", "name": resource_name}


def initial_iam_summary(address, action="create"):
    expected = BOOTSTRAP_IAM.get(address)
    if expected is None or action not in {"create", "no-op"}:
        return None
    resource_name, role, member = expected
    return {"address": address, "action": action, **initial_iam_scope(address, resource_name),
            "role": role, "member": member}


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


def initial_service_matches(address, resource, database_password_version):
    after = resource.get("after")
    if not isinstance(after, dict):
        return False
    service_names = {
        "google_cloud_run_v2_service.api[0]": "espacigo-test-api",
        'google_cloud_run_v2_service.web["public"]': "espacigo-test-public",
        'google_cloud_run_v2_service.web["private"]': "espacigo-test-private",
    }
    api = address == "google_cloud_run_v2_service.api[0]"
    if (after.get("name") != service_names.get(address)
            or after.get("project") != "espacigo-testprod"
            or after.get("location") != "southamerica-west1"
            or after.get("ingress") != "INGRESS_TRAFFIC_ALL"
            or after.get("deletion_protection") is not True
            or not _has_only_configured_root_fields(after, api)
            or not _unknowns_are_computed_or_referenced(resource)
            or not _template_config_matches(resource, api, database_password_version)):
        return False
    if address == 'google_cloud_run_v2_service.web["public"]' and after.get("iap_enabled") is not False:
        return False
    if address == 'google_cloud_run_v2_service.web["private"]' and after.get("iap_enabled") is not True:
        return False
    if api:
        try:
            containers = after["template"][0]["containers"]
            env = {item["name"]: item.get("value") for item in containers[0]["env"]}
            volumes = after["template"][0]["volumes"]
            db_volume = next(volume for volume in volumes if volume.get("name") == "database-secret")
            secret = db_volume["secret"][0]
            secret_items = secret["items"]
            has_pinned_password = any(
                item.get("path") == "password" and item.get("version") == database_password_version
                for item in secret_items
            )
        except (KeyError, IndexError, StopIteration, TypeError):
            return False
        if (env.get("CLOUD_SQL_INSTANCE_CONNECTION_NAME")
                != "espacigo-testprod:southamerica-west1:espacigo-testprod-db"
                or env.get("DATABASE_NAME") != "espacigo_dms_stage_20261020"
                or env.get("DATABASE_USER") != "espacigo_api_login"
                or env.get("DATABASE_SET_ROLE") != "espacigo_runtime"
                or not has_pinned_password
                or "@sha256:" not in image_of(after, "after")):
            return False
    return True


def initial_iam_matches(address, resource):
    expected = BOOTSTRAP_IAM.get(address)
    after = resource.get("after")
    if expected is None or not isinstance(after, dict):
        return False
    name, role, member = expected
    scope = initial_iam_scope(address, name)
    allowed_fields = set(scope) | {"role", "member", "condition", "id", "etag"}
    unknown = resource.get("after_unknown") or {}
    if (not isinstance(unknown, dict)
            or any(key not in {"id", "etag"} or not isinstance(value, bool)
                   for key, value in unknown.items())):
        return False
    actions = resource.get("actions")
    before = resource.get("before")
    if actions == ["create"]:
        if before is not None:
            return False
    elif actions == ["no-op"]:
        if not isinstance(before, dict):
            return False
        if (set(before) - allowed_fields or not _empty(before.get("condition"))
                or before.get("role") != role or before.get("member") != member
                or any(before.get(key) != value for key, value in scope.items())):
            return False
    else:
        return False
    if (set(after) - allowed_fields
            or not _empty(after.get("condition"))
            or after.get("role") != role
            or after.get("member") != member
            or any(after.get(key) != value for key, value in scope.items())):
        return False
    return True


def validate(plan, expected, source_sha, release_run_id, mode="update", database_password_version=""):
    resources = plan.get("resource_changes")
    if (not isinstance(resources, list) or plan.get("applyable") is not True
            or plan.get("errored") is True or plan.get("complete") is False):
        raise PolicyError()
    if mode not in {"update", "bootstrap"}:
        raise PolicyError()
    summary = []
    iam_summary = []
    seen = set()
    for item in resources:
        actions = item.get("change", {}).get("actions")
        address = item.get("address")
        if mode == "bootstrap" and address in BOOTSTRAP_IAM:
            if not initial_iam_matches(address, item.get("change", {})) or address in seen:
                raise PolicyError()
            iam_summary.append(initial_iam_summary(address, actions[0]))
            seen.add(address)
            continue
        if actions == ["no-op"]:
            continue
        if item.get("mode") == "data" and actions == ["read"]:
            continue
        if address not in ALLOWED or address in seen:
            raise PolicyError()
        component, repository = ALLOWED[address]
        image = image_of(item["change"]["after"], "after")
        if not digest_ok(image, repository):
            raise PolicyError()
        if expected.get(component) != image:
            raise PolicyError()
        if mode == "bootstrap":
            if actions != ["create"] or not initial_service_matches(
                    address, item["change"], database_password_version):
                raise PolicyError()
        else:
            if actions != ["update"]:
                raise PolicyError()
            old_image = image_of(item["change"]["before"], "before")
            if old_image == image:
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
        summary.append({"address": address, "action": actions[0], "image": image})
    if mode == "bootstrap" and (
            not set(ALLOWED).issubset(seen)
            or seen - set(ALLOWED) != set(BOOTSTRAP_IAM)):
        raise PolicyError()
    normalized = {
        "mode": mode,
        "source_sha": source_sha,
        "release_run_id": str(release_run_id),
        "changes": sorted(summary, key=lambda row: row["address"]),
    }
    if mode == "bootstrap":
        normalized["iam_changes"] = sorted(iam_summary, key=lambda row: row["address"])
    fingerprint = hashlib.sha256(json.dumps(normalized, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return {**normalized, "fingerprint": fingerprint}


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("plan_json")
    parser.add_argument("--images", required=True, help="JSON map of selected component to immutable image ref")
    parser.add_argument("--source-sha", required=True)
    parser.add_argument("--release-run-id", required=True)
    parser.add_argument("--mode", choices=("update", "bootstrap"), default="update")
    parser.add_argument("--database-password-version", default="")
    parser.add_argument("--output", required=True)
    args = parser.parse_args()
    try:
        with open(args.plan_json, encoding="utf-8") as stream:
            plan = json.load(stream)
        with open(args.images, encoding="utf-8") as stream:
            expected = json.load(stream)
        result = validate(plan, expected, args.source_sha, args.release_run_id,
                          args.mode, args.database_password_version)
        with open(args.output, "w", encoding="utf-8") as stream:
            json.dump(result, stream, sort_keys=True, indent=2)
            stream.write("\n")
    except (OSError, ValueError, KeyError, TypeError, PolicyError):
        print("Terraform plan rejected by GCP-07D image-only policy", file=sys.stderr)
        return 1
    print(result["fingerprint"])
    return 0


if __name__ == "__main__":
    sys.exit(main())
