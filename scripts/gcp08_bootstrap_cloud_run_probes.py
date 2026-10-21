#!/usr/bin/env python3
"""Run fixed read-only GCP probes and emit only allowlisted diagnostics."""
import argparse
import json
import os
import re
import subprocess
import sys
import tempfile
import urllib.error
import urllib.parse
import urllib.request
from pathlib import Path

from classify_gcp08_terraform_failure import SAFE_HTTP_STATUSES, classify, safe_details


PROJECT = "espacigo-testprod"
REGION = "southamerica-west1"
API_ROOT = "https://run.googleapis.com/v2"
IAP_API_ROOT = "https://iap.googleapis.com/v1"
SQLADMIN_API_ROOT = "https://sqladmin.googleapis.com/sql/v1beta4"
IAM_API_ROOT = "https://iam.googleapis.com/v1"
COMPUTE_API_ROOT = "https://compute.googleapis.com/compute/v1"
SERVICE_USAGE_API_ROOT = "https://serviceusage.googleapis.com/v1"
RESOURCE_MANAGER_API_ROOT = "https://cloudresourcemanager.googleapis.com/v3"
DEFAULT_PRIVATE_SERVICE_NAME = "espacigo-test-private"
RUNTIME_SERVICE_ACCOUNTS = {
    "api": "espacigo-test-api@espacigo-testprod.iam.gserviceaccount.com",
    "public": "espacigo-test-public-web@espacigo-testprod.iam.gserviceaccount.com",
    "private": "espacigo-test-private-web@espacigo-testprod.iam.gserviceaccount.com",
}
TESTED_RUN_PERMISSIONS = tuple(sorted({
    "run.services.get", "run.services.getIamPolicy", "run.services.setIamPolicy",
    "run.services.update",
}))
TESTED_RUN_PARENT_PERMISSIONS = tuple(sorted({
    "run.operations.get", "run.services.create", "serviceusage.services.get",
}))
TESTED_SA_PERMISSIONS = tuple(sorted({
    "iam.serviceAccounts.actAs", "iam.serviceAccounts.get",
}))
MAX_RESPONSE_BYTES = 2_000_000
PROBES = (
    ("cloud_run_service.api.describe", "describe", "espacigo-test-api"),
    ("cloud_run_service.public.describe", "describe", "espacigo-test-public"),
    ("cloud_run_service.private.describe", "describe", "espacigo-test-private"),
    ("cloud_run_services.list", "list", None),
    ("cloud_run_service.api.get_iam_policy", "get-iam-policy", "espacigo-test-api"),
    ("cloud_run_service.public.get_iam_policy", "get-iam-policy", "espacigo-test-public"),
    ("cloud_run_service.private.get_iam_policy", "get-iam-policy", "espacigo-test-private"),
    ("cloud_run_service.private.iap_get_iam_policy", "iap-get-iam-policy", "private"),
    ("cloud_sql_instance.v67.get", "sql-instance-get", "v67"),
    ("cloud_run_service.api.test_iam_permissions", "run-test-iam", "espacigo-test-api"),
    ("cloud_run_service.public.test_iam_permissions", "run-test-iam", "espacigo-test-public"),
    ("cloud_run_service.private.test_iam_permissions", "run-test-iam", "espacigo-test-private"),
    ("cloud_run_create_parent.project.test_iam_permissions", "run-parent-test-iam", "project"),
    ("runtime_service_account.api.test_act_as", "sa-test-iam", "api"),
    ("runtime_service_account.public.test_act_as", "sa-test-iam", "public"),
    ("runtime_service_account.private.test_act_as", "sa-test-iam", "private"),
    ("compute_network.testprod.get", "compute-network-get", "espacigo-testprod"),
    ("compute_subnetwork.serverless.get", "compute-subnetwork-get", "espacigo-testprod-serverless"),
    ("compute_subnetwork.api_serverless.get", "compute-subnetwork-get", "espacigo-testprod-api-serverless"),
    ("serviceusage.run_api.get", "serviceusage-service-get", "run.googleapis.com"),
)
PROBE_BY_ENUM = {target: (operation, service) for target, operation, service in PROBES}
NOT_FOUND = re.compile(r"\b404\b|\bNOT_FOUND\b|not found|does not exist", re.IGNORECASE)
AUTH_ERROR = re.compile(
    r"\b401\b|\bUNAUTHENTICATED\b|invalid_grant|invalid credentials|"
    r"could not find default credentials|unable to refresh|problem refreshing.*(?:auth|token)|"
    r"error retrieving credentials|failed to refresh|credential refresh|"
    r"expired (?:access )?token|oauth", re.IGNORECASE,
)
SAFE_CATEGORIES = {"success", "not_found", "permission_denied", "auth_error", "other"}
SAFE_DETAIL_KEYS = {"status", "permission", "service"}
SAFE_RUN_PERMISSIONS = {
    "run.services.get", "run.services.getIamPolicy", "run.services.setIamPolicy",
    "run.services.create", "run.services.update", "run.services.list",
    "run.operations.get",
}
SAFE_IAP_PERMISSIONS = {"iap.webServices.getIamPolicy"}
SAFE_SQL_PERMISSIONS = {"cloudsql.instances.get"}
SAFE_SA_PERMISSIONS = {"iam.serviceAccounts.actAs", "iam.serviceAccounts.get"}
SAFE_COMPUTE_PERMISSIONS = {"compute.networks.get", "compute.subnetworks.get"}
SAFE_SERVICE_USAGE_PERMISSIONS = {"serviceusage.services.get"}
SAFE_PERMISSIONS = (
    SAFE_RUN_PERMISSIONS | SAFE_IAP_PERMISSIONS | SAFE_SQL_PERMISSIONS | SAFE_SA_PERMISSIONS
    | SAFE_COMPUTE_PERMISSIONS | SAFE_SERVICE_USAGE_PERMISSIONS
)


def classify_probe(diagnostic):
    """Map private gcloud text diagnostics into the closed probe category set."""
    category = classify("plan", diagnostic)
    if category == "iam":
        return "permission_denied"
    details = safe_details(diagnostic)
    status = next((int(item.split("=", 1)[1]) for item in details if item.startswith("status=")), None)
    if status == 404 or NOT_FOUND.search(diagnostic):
        return "not_found"
    if status == 401 or AUTH_ERROR.search(diagnostic):
        return "auth_error"
    return "other"


def build_gcloud_command(target):
    if target not in PROBE_BY_ENUM:
        raise ValueError("invalid target")
    operation, service = PROBE_BY_ENUM[target]
    if operation != "get-iam-policy":
        raise ValueError("describe probes use the REST transport")
    return [
        "gcloud", "run", "services", operation, service,
        f"--region={REGION}", f"--project={PROJECT}", "--format=none", "--quiet",
    ]


def build_rest_url(target):
    if target not in PROBE_BY_ENUM:
        raise ValueError("invalid target")
    operation, service = PROBE_BY_ENUM[target]
    if operation == "list":
        resource = "/".join(urllib.parse.quote(part, safe="") for part in (
            "projects", PROJECT, "locations", REGION, "services",
        ))
        return f"{API_ROOT}/{resource}"
    if operation == "iap-get-iam-policy":
        raise ValueError("IAP policy probe requires validated private configuration")
    if operation == "sql-instance-get":
        raise ValueError("Cloud SQL probe requires validated V67 configuration")
    if operation == "run-test-iam":
        resource = "/".join(urllib.parse.quote(part, safe="") for part in (
            "projects", PROJECT, "locations", REGION, "services", service,
        ))
        return f"{API_ROOT}/{resource}:testIamPermissions"
    if operation == "run-parent-test-iam":
        # Cloud Run create checks its location parent. Its testIamPermissions
        # endpoint only accepts a service path, so test the project-level grant
        # inherited by that location through Resource Manager and label that scope.
        return f"{RESOURCE_MANAGER_API_ROOT}/projects/{PROJECT}:testIamPermissions"
    if operation == "sa-test-iam":
        if service not in RUNTIME_SERVICE_ACCOUNTS:
            raise ValueError("invalid runtime service-account probe")
        account = RUNTIME_SERVICE_ACCOUNTS[service]
        return f"{IAM_API_ROOT}/projects/{PROJECT}/serviceAccounts/{urllib.parse.quote(account, safe='')}:testIamPermissions"
    if operation == "compute-network-get":
        if service != "espacigo-testprod":
            raise ValueError("invalid Compute Engine network probe")
        return f"{COMPUTE_API_ROOT}/projects/{PROJECT}/global/networks/{service}"
    if operation == "compute-subnetwork-get":
        if service not in {"espacigo-testprod-serverless", "espacigo-testprod-api-serverless"}:
            raise ValueError("invalid Compute Engine subnetwork probe")
        return f"{COMPUTE_API_ROOT}/projects/{PROJECT}/regions/{REGION}/subnetworks/{service}"
    if operation == "serviceusage-service-get":
        if service != "run.googleapis.com":
            raise ValueError("invalid Service Usage probe")
        return f"{SERVICE_USAGE_API_ROOT}/projects/{PROJECT}/services/{service}"
    if operation != "describe":
        raise ValueError("IAM policy probes use the gcloud read command")
    resource = "/".join(urllib.parse.quote(part, safe="") for part in (
        "projects", PROJECT, "locations", REGION, "services", service,
    ))
    return f"{API_ROOT}/{resource}"


def _error_object(payload):
    if not isinstance(payload, dict):
        return {}
    error = payload.get("error")
    return error if isinstance(error, dict) else {}


def _structured_status(http_status, payload):
    error = _error_object(payload)
    body_code = error.get("code")
    if isinstance(body_code, int) and not isinstance(body_code, bool):
        if http_status is not None and body_code != http_status:
            return None
        return body_code if body_code in SAFE_HTTP_STATUSES else None
    return http_status if http_status in SAFE_HTTP_STATUSES else None


def classify_rest_response(http_status, payload):
    if isinstance(http_status, int) and 200 <= http_status < 300:
        return "success"
    error = _error_object(payload)
    code = _structured_status(http_status, payload)
    symbolic_status = error.get("status") if isinstance(error.get("status"), str) else ""
    details = error.get("details") if isinstance(error.get("details"), list) else []
    reasons = {
        item.get("reason") for item in details
        if isinstance(item, dict) and isinstance(item.get("reason"), str)
    }
    if code == 403 or symbolic_status == "PERMISSION_DENIED" or "IAM_PERMISSION_DENIED" in reasons:
        return "permission_denied"
    if code == 404 or symbolic_status == "NOT_FOUND" or "NOT_FOUND" in reasons:
        return "not_found"
    if code == 401 or symbolic_status == "UNAUTHENTICATED":
        return "auth_error"
    return "other"


def safe_rest_fields(http_status, payload):
    """Extract only approved HTTP status, permission names, and API domains."""
    fields = []
    status = _structured_status(http_status, payload)
    if status is not None:
        fields.append(f"status={status}")

    error = _error_object(payload)
    details = error.get("details") if isinstance(error.get("details"), list) else []
    permission = None
    service = None
    for item in details:
        if not isinstance(item, dict):
            continue
        domain = item.get("domain")
        if domain in {
            "run.googleapis.com", "iap.googleapis.com", "sqladmin.googleapis.com",
            "compute.googleapis.com", "serviceusage.googleapis.com",
        }:
            service = domain
        metadata = item.get("metadata")
        candidate = metadata.get("permission") if isinstance(metadata, dict) else None
        if isinstance(candidate, str) and candidate in SAFE_PERMISSIONS:
            permission = candidate
            if candidate in SAFE_IAP_PERMISSIONS:
                service = "iap.googleapis.com"
            elif candidate in SAFE_SQL_PERMISSIONS:
                service = "sqladmin.googleapis.com"
            elif candidate in SAFE_SA_PERMISSIONS:
                service = "iam.googleapis.com"
            elif candidate in SAFE_COMPUTE_PERMISSIONS:
                service = "compute.googleapis.com"
            elif candidate in SAFE_SERVICE_USAGE_PERMISSIONS:
                service = "serviceusage.googleapis.com"
            else:
                service = "run.googleapis.com"
    if permission:
        fields.append(f"permission={permission}")
    if service:
        fields.append(f"service={service}")
    return fields


def _emit(target, category, fields=()):
    if category not in SAFE_CATEGORIES:
        category = "other"
    safe_fields = []
    for field in fields:
        key, separator, value = field.partition("=")
        if separator and key in SAFE_DETAIL_KEYS and re.fullmatch(r"[A-Za-z0-9._/-]{1,100}", value):
            safe_fields.append(f"{key}={value}")
    suffix = " " + " ".join(safe_fields) if safe_fields else ""
    print(f"GCP08_CLOUD_RUN_PROBE stage=bootstrap target={target} category={category}{suffix}", flush=True)


def _safe_gcloud_fields(diagnostic):
    return [
        item for item in safe_details(diagnostic)
        if item.split("=", 1)[0] in SAFE_DETAIL_KEYS
    ]


def _write_private_response(path, body):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb") as response_file:
        response_file.write(body[:MAX_RESPONSE_BYTES])


def _read_private_response(path):
    try:
        return json.loads(path.read_text(encoding="utf-8", errors="strict"))
    except (OSError, UnicodeError, json.JSONDecodeError):
        return {}


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        # Never forward the short-lived bearer token to a redirected host.
        return None


def _fetch_rest_response(url, token, opener_factory=urllib.request.build_opener, timeout=120,
                         method="GET", body=None, read_success_body=False):
    request = urllib.request.Request(
        url, data=body,
        headers={"Authorization": f"Bearer {token}", "Accept": "application/json",
                 **({"Content-Type": "application/json"} if body is not None else {})},
        method=method,
    )
    opener = opener_factory(_NoRedirect)
    try:
        with opener.open(request, timeout=timeout) as response:
            status = response.getcode()
            if isinstance(status, int) and 200 <= status < 300:
                return status, response.read(MAX_RESPONSE_BYTES + 1) if read_success_body else b""
            return status, response.read(MAX_RESPONSE_BYTES + 1)
    except urllib.error.HTTPError as error:
        try:
            return error.code, error.read(MAX_RESPONSE_BYTES + 1)
        finally:
            error.close()
    except (OSError, urllib.error.URLError, TimeoutError):
        return None, b""


def _iap_rest_url(project_number, private_service_name):
    if not re.fullmatch(r"[1-9][0-9]{5,19}", project_number):
        raise ValueError("invalid IAP project number")
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,62}[a-z0-9]", private_service_name):
        raise ValueError("invalid IAP service name")
    resource = f"projects/{project_number}/iap_web/cloud_run-{REGION}/services/{private_service_name}:getIamPolicy"
    return f"{IAP_API_ROOT}/{resource}"


def _sqladmin_rest_url(operation, instance_name):
    if operation != "sql-instance-get":
        raise ValueError("invalid Cloud SQL probe")
    if not isinstance(instance_name, str) or not re.fullmatch(r"[a-z][a-z0-9-]{0,61}[a-z0-9]", instance_name):
        raise ValueError("invalid Cloud SQL instance name")
    parts = ["projects", PROJECT, "instances", instance_name]
    return f"{SQLADMIN_API_ROOT}/" + "/".join(urllib.parse.quote(part, safe="") for part in parts)


def read_probe_configuration(var_file):
    """Read only the validated identifiers needed to construct fixed GET probes."""
    try:
        config = json.loads(Path(var_file).read_text(encoding="utf-8"))
        if not isinstance(config, dict):
            config = {}
    except (OSError, UnicodeError, json.JSONDecodeError):
        config = {}
    project_number = config.get("project_number")
    if isinstance(project_number, int) and not isinstance(project_number, bool):
        project_number = str(project_number)
    return {
        "project_number": project_number,
        # The Terraform variable is pinned to this default by validation; it is
        # commonly omitted from tfvars because it is not an override.
        "private_service_name": config.get("private_service_name", DEFAULT_PRIVATE_SERVICE_NAME),
        "sql_instance_name": config.get("api_v67_database_instance_name"),
    }


def _run_rest_probe(target, private_root, runner, opener_factory, timeout,
                    project_number=None, private_service_name=None,
                    sql_instance_name=None):
    operation, _ = PROBE_BY_ENUM[target]
    if operation == "iap-get-iam-policy":
        if project_number is None or private_service_name is None:
            _emit(target, "other")
            return False
        try:
            url = _iap_rest_url(project_number, private_service_name)
        except ValueError:
            _emit(target, "other")
            return False
        method, body = "POST", b"{}"
    elif operation == "sql-instance-get":
        try:
            url = _sqladmin_rest_url(operation, sql_instance_name)
        except ValueError:
            _emit(target, "other")
            return False
        method, body = "GET", None
    elif operation in {"run-test-iam", "run-parent-test-iam", "sa-test-iam"}:
        url = build_rest_url(target)
        if operation == "run-test-iam":
            requested_permissions = TESTED_RUN_PERMISSIONS
        elif operation == "run-parent-test-iam":
            requested_permissions = TESTED_RUN_PARENT_PERMISSIONS
        else:
            requested_permissions = TESTED_SA_PERMISSIONS
        method = "POST"
        body = json.dumps({"permissions": list(requested_permissions)}, separators=(",", ":")).encode()
    else:
        url = build_rest_url(target)
        method, body = "GET", None
    with tempfile.TemporaryDirectory(prefix="probe-", dir=private_root) as temporary:
        private_dir = Path(temporary)
        os.chmod(private_dir, 0o700)
        sdk_config = private_dir / "cloud-sdk"
        sdk_config.mkdir(mode=0o700)
        env = os.environ.copy()
        env["CLOUDSDK_CONFIG"] = str(sdk_config)
        env["CLOUDSDK_CORE_DISABLE_PROMPTS"] = "1"
        try:
            token_result = runner(
                ["gcloud", "auth", "print-access-token"], capture_output=True, text=True,
                env=env, timeout=min(timeout, 60), check=False,
            )
            token = token_result.stdout.strip() if token_result.returncode == 0 else ""
        except (OSError, subprocess.TimeoutExpired):
            token = ""
        if not token or re.search(r"\s", token):
            _emit(target, "auth_error")
            return False

        http_status, response_body = _fetch_rest_response(
            url, token, opener_factory, timeout, method=method, body=body,
            read_success_body=operation in {"run-test-iam", "run-parent-test-iam", "sa-test-iam"},
        )
        if isinstance(http_status, int) and 200 <= http_status < 300:
            if operation in {"run-test-iam", "run-parent-test-iam", "sa-test-iam"}:
                response_path = private_dir / "response.json"
                try:
                    _write_private_response(response_path, response_body)
                    payload = _read_private_response(response_path)
                except OSError:
                    _emit(target, "other")
                    return False
                actual = payload.get("permissions") if isinstance(payload, dict) else None
                actual = set(actual) if isinstance(actual, list) and all(isinstance(item, str) for item in actual) else set()
                if operation == "sa-test-iam":
                    allowed = SAFE_SA_PERMISSIONS
                elif operation == "run-parent-test-iam":
                    allowed = SAFE_RUN_PERMISSIONS | SAFE_SERVICE_USAGE_PERMISSIONS
                else:
                    allowed = SAFE_RUN_PERMISSIONS
                granted = actual & allowed
                if operation == "run-parent-test-iam":
                    method, resource_scope = "projects.testIamPermissions", "project"
                elif operation == "run-test-iam":
                    method, resource_scope = "services.testIamPermissions", "cloud_run_service"
                else:
                    method, resource_scope = "serviceAccounts.testIamPermissions", "service_account"
                print(
                    f"GCP08_PERMISSION_TEST stage=bootstrap target={target} "
                    f"method={method} resource={resource_scope} category=success",
                    flush=True,
                )
                for permission in requested_permissions:
                    result = "granted" if permission in granted else "not_granted"
                    print(f"GCP08_PERMISSION_TEST stage=bootstrap target={target} permission={permission} result={result}", flush=True)
            else:
                _emit(target, "success")
            return True
        response_path = private_dir / "response.json"
        try:
            _write_private_response(response_path, response_body)
        except OSError:
            _emit(target, "other")
            return False
        truncated = len(response_body) > MAX_RESPONSE_BYTES
        payload = {} if truncated else _read_private_response(response_path)
        category = classify_rest_response(http_status, payload)
        fields = safe_rest_fields(http_status, payload)
        _emit(target, category, fields)
        return category == "success"


def _run_gcloud_probe(target, private_root, runner, timeout):
    command = build_gcloud_command(target)
    with tempfile.TemporaryDirectory(prefix="probe-", dir=private_root) as temporary:
        private_dir = Path(temporary)
        os.chmod(private_dir, 0o700)
        log_path = private_dir / "probe.log"
        sdk_config = private_dir / "cloud-sdk"
        sdk_config.mkdir(mode=0o700)
        env = os.environ.copy()
        env["CLOUDSDK_CONFIG"] = str(sdk_config)
        env["CLOUDSDK_CORE_DISABLE_PROMPTS"] = "1"
        try:
            descriptor = os.open(log_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
            with os.fdopen(descriptor, "w", encoding="utf-8") as log:
                result = runner(command, stdout=log, stderr=subprocess.STDOUT, env=env,
                                timeout=timeout, check=False)
        except (OSError, subprocess.TimeoutExpired):
            try:
                diagnostic = log_path.read_text(encoding="utf-8", errors="replace")[:MAX_RESPONSE_BYTES]
            except OSError:
                diagnostic = ""
            category = classify_probe(diagnostic)
            _emit(target, category, _safe_gcloud_fields(diagnostic))
            return False
        if result.returncode == 0:
            _emit(target, "success")
            return True
        try:
            diagnostic = log_path.read_text(encoding="utf-8", errors="replace")[:MAX_RESPONSE_BYTES]
        except OSError:
            diagnostic = ""
        category = classify_probe(diagnostic)
        _emit(target, category, _safe_gcloud_fields(diagnostic))
        return False


def run_probe(target, private_root, runner=subprocess.run,
              opener_factory=urllib.request.build_opener, timeout=120,
              project_number=None, private_service_name=None,
              sql_instance_name=None):
    """Run one fixed read-only probe; tokens and response bodies never reach stdout."""
    if target not in PROBE_BY_ENUM:
        raise ValueError("invalid target")
    operation, _ = PROBE_BY_ENUM[target]
    if operation in {
        "describe", "list", "iap-get-iam-policy", "sql-instance-get",
        "run-test-iam", "run-parent-test-iam", "sa-test-iam",
    }:
        return _run_rest_probe(target, private_root, runner, opener_factory, timeout,
                               project_number, private_service_name,
                               sql_instance_name)
    return _run_gcloud_probe(target, private_root, runner, timeout)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--private-dir", required=True)
    parser.add_argument("--var-file", required=True)
    args = parser.parse_args()
    private_root = Path(args.private_dir)
    private_root.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_root, 0o700)
    config = read_probe_configuration(args.var_file)
    for target, _, _ in PROBES:
        run_probe(target, private_root, **config)
    return 0


if __name__ == "__main__":
    sys.exit(main())
