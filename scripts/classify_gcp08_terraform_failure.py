#!/usr/bin/env python3
"""Emit only a safe, allowlisted classification for private Terraform diagnostics."""
import argparse
import re
import sys


RULES = (
    ("variables", re.compile(
        r"no value for required variable|invalid value for input variable|"
        r"undeclared input variable|invalid type for variable|"
        r"required variable.*not set|failed to parse.*tfvars", re.IGNORECASE)),
    ("iam", re.compile(
        r"iam_permission_denied|permission[_ ]denied|does not have .*permission|"
        r"permission\s+[\"'][^\n]{1,120}[\"']\s+denied|"
        r"insufficient permissions|access denied|\b403\b|forbidden", re.IGNORECASE)),
    ("backend", re.compile(
        r"error acquiring the state lock|failed to .*state|backend .*error|"
        r"bucket .*not found|storage\.buckets\.|state lock|"
        r"no workspace named|workspace .*not found", re.IGNORECASE)),
    ("provider", re.compile(
        r"error configuring provider|failed to instantiate provider|"
        r"could not load plugin|plugin\.6|error retrieving credentials|"
        r"could not find default credentials|unauthenticated|oauth", re.IGNORECASE)),
)
STAGES = {"init", "workspace", "plan", "inspect", "policy"}
GCP_PERMISSION_NAME = (
    r"(?:run|compute|cloudsql|secretmanager|iap|artifactregistry)\."
    r"[A-Za-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*"
    r"|iam\.(?:roles|serviceAccounts)\.[A-Za-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*"
    r"|storage\.(?:objects|managedFolders)\.[A-Za-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*"
    r"|resourcemanager\.projects\.[A-Za-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*"
    r"|serviceusage\.services\.[A-Za-z][A-Za-z0-9]*(?:\.[A-Za-z][A-Za-z0-9]*)*"
)
DENIED_PERMISSION = re.compile(
    rf"(?i:permission)\s+[\"']({GCP_PERMISSION_NAME})[\"']\s+(?i:denied)",
)
STRUCTURED_PERMISSION = re.compile(
    rf"(?:[\"']permission[\"']|\bpermission\b)\s*[:=]\s*[\"']?"
    rf"({GCP_PERMISSION_NAME})(?=[\"'\s,}}\]]|$)",
)
HTTP_STATUS = re.compile(
    r"(?:googleapi:\s*)?Error\s+(\d{3})\b|"
    r"HTTP\s+(\d{3})\b|status\s+code\s+(\d{3})\b", re.IGNORECASE,
)
SAFE_HTTP_STATUSES = {400, 401, 403, 404, 409, 429, 500, 502, 503, 504}
STATUS_REASONS = {
    400: "invalid_argument", 401: "unauthenticated", 403: "permission_denied",
    404: "not_found", 409: "conflict", 429: "quota_exceeded",
    500: "internal_error", 502: "upstream_error", 503: "unavailable",
    504: "timeout",
}
PROVIDER_REASONS = {
    "serviceDisabled": "service_disabled",
    "rateLimitExceeded": "quota_exceeded",
    "quotaExceeded": "quota_exceeded",
    "accessNotConfigured": "service_disabled",
}
PERMISSION_SERVICES = {
    "run": "run.googleapis.com", "compute": "compute.googleapis.com",
    "cloudsql": "sqladmin.googleapis.com", "secretmanager": "secretmanager.googleapis.com",
    "iap": "iap.googleapis.com", "artifactregistry": "artifactregistry.googleapis.com",
    "iam": "iam.googleapis.com", "storage": "storage.googleapis.com",
    "resourcemanager": "cloudresourcemanager.googleapis.com",
    "serviceusage": "serviceusage.googleapis.com",
}
RESOURCE_TYPES = {
    ("run", "services"): "run.googleapis.com/Service",
    ("run", "operations"): "run.googleapis.com/Operation",
    ("compute", "instances"): "compute.googleapis.com/Instance",
    ("compute", "networks"): "compute.googleapis.com/Network",
    ("compute", "subnetworks"): "compute.googleapis.com/Subnetwork",
    ("cloudsql", "instances"): "sqladmin.googleapis.com/Instance",
    ("secretmanager", "secrets"): "secretmanager.googleapis.com/Secret",
    ("iap", "webServices"): "iap.googleapis.com/WebService",
    ("artifactregistry", "repositories"): "artifactregistry.googleapis.com/Repository",
    ("iam", "roles"): "iam.googleapis.com/Role",
    ("iam", "serviceAccounts"): "iam.googleapis.com/ServiceAccount",
    ("storage", "objects"): "storage.googleapis.com/Object",
    ("storage", "managedFolders"): "storage.googleapis.com/ManagedFolder",
    ("resourcemanager", "projects"): "cloudresourcemanager.googleapis.com/Project",
    ("serviceusage", "services"): "serviceusage.googleapis.com/Service",
}
DOMAIN_SERVICES = {service: service for service in PERMISSION_SERVICES.values()}
TERRAFORM_ADDRESSES = {
    "google_cloud_run_v2_service.api[0]": "cloud_run_v2_service.api",
    'google_cloud_run_v2_service.web["public"]': "cloud_run_v2_service.web.public",
    'google_cloud_run_v2_service.web["private"]': "cloud_run_v2_service.web.private",
    'google_cloud_run_v2_service_iam_member.api_invoker["public"]': "cloud_run_iam.api_invoker.public",
    'google_cloud_run_v2_service_iam_member.api_invoker["private"]': "cloud_run_iam.api_invoker.private",
    'google_cloud_run_v2_service_iam_member.deployer_service_updater["api"]': "cloud_run_iam.deployer_updater.api",
    'google_cloud_run_v2_service_iam_member.deployer_service_updater["public"]': "cloud_run_iam.deployer_updater.public",
    'google_cloud_run_v2_service_iam_member.deployer_service_updater["private"]': "cloud_run_iam.deployer_updater.private",
    "google_cloud_run_v2_service_iam_member.public_web_invoker[0]": "cloud_run_iam.public_web_invoker",
    "google_cloud_run_v2_service_iam_member.private_iap_invoker[0]": "cloud_run_iam.private_iap_invoker",
    "google_secret_manager_secret_iam_member.v67_api_secret_access[0]": "secret_manager_iam.v67_api_secret_access",
}
TERRAFORM_ADDRESS_PATTERNS = tuple(
    (address, re.compile(rf"(?m)^\s*with\s+{re.escape(address)}\s*,?\s*$"))
    for address in TERRAFORM_ADDRESSES
) + ((
    "google_iap_web_cloud_run_service_iam_member.private_evaluators[<key>]",
    re.compile(
        r'(?m)^\s*with\s+google_iap_web_cloud_run_service_iam_member'
        r'\.private_evaluators\["[^"\r\n]{1,254}"\]\s*,?\s*$'
    ),
),)
TERRAFORM_ADDRESS_ENUMS = {
    **TERRAFORM_ADDRESSES,
    "google_iap_web_cloud_run_service_iam_member.private_evaluators[<key>]": "iap_iam.private_evaluators",
}
PERMISSION_OPERATIONS = {
    "get": "get", "getIamPolicy": "getIamPolicy", "setIamPolicy": "setIamPolicy",
    "create": "create", "update": "update",
}
OPERATION_PATTERNS = (
    ("getIamPolicy", re.compile(r"\bGetIamPolicy\b")),
    ("setIamPolicy", re.compile(r"\bSetIamPolicy\b")),
    ("create", re.compile(r"\b(?:Error\s+when\s+)?creating\b", re.IGNORECASE)),
    ("update", re.compile(r"\b(?:Error\s+when\s+)?updating\b", re.IGNORECASE)),
    ("read", re.compile(r"\b(?:Error\s+when\s+)?reading\b", re.IGNORECASE)),
    ("get", re.compile(r"\bRequest\s+Method\s*:\s*GET\b", re.IGNORECASE)),
)
STRUCTURED_OPERATION = re.compile(
    r"(?:[\"']operation[\"']|\boperation\b)\s*[:=]\s*[\"']?"
    r"(getIamPolicy|setIamPolicy|get|read|create|update)"
    r"(?=[\"'\s,}\]]|$)", re.IGNORECASE,
)
CANONICAL_OPERATIONS = {
    "getiampolicy": "getIamPolicy", "setiampolicy": "setIamPolicy",
    "get": "get", "read": "read", "create": "create", "update": "update",
}


def classify(stage, diagnostic):
    if stage not in STAGES:
        raise ValueError("invalid stage")
    if stage == "policy":
        return "guard"
    for category, pattern in RULES:
        if pattern.search(diagnostic):
            return category
    return "unclassified"


def denied_permission(diagnostic):
    candidates = {match.group(1) for match in DENIED_PERMISSION.finditer(diagnostic)}
    candidates.update(match.group(1) for match in STRUCTURED_PERMISSION.finditer(diagnostic))
    return next(iter(candidates)) if len(candidates) == 1 else None


def terraform_address(diagnostic):
    candidates = {
        TERRAFORM_ADDRESS_ENUMS[address]
        for address, pattern in TERRAFORM_ADDRESS_PATTERNS
        if pattern.search(diagnostic)
    }
    return next(iter(candidates)) if len(candidates) == 1 else None


def terraform_operation(diagnostic, permission=None):
    if permission:
        method = permission.rsplit(".", 1)[-1]
        if method in PERMISSION_OPERATIONS:
            return PERMISSION_OPERATIONS[method]
    structured = {
        CANONICAL_OPERATIONS[match.group(1).lower()]
        for match in STRUCTURED_OPERATION.finditer(diagnostic)
    }
    if len(structured) == 1:
        return next(iter(structured))
    candidates = {operation for operation, pattern in OPERATION_PATTERNS if pattern.search(diagnostic)}
    return next(iter(candidates)) if len(candidates) == 1 else None


def safe_details(diagnostic):
    """Return only predefined or strictly validated fields; never return resource values."""
    details = []
    status_match = HTTP_STATUS.search(diagnostic)
    status = int(next(group for group in status_match.groups() if group)) if status_match else None
    provider_reason = next(
        (mapped for token, mapped in PROVIDER_REASONS.items()
         if re.search(rf"(?<![A-Za-z0-9]){re.escape(token)}(?![A-Za-z0-9])", diagnostic)),
        None,
    )
    if status in SAFE_HTTP_STATUSES:
        details.append(f"status={status}")
        details.append(f"reason={provider_reason or STATUS_REASONS[status]}")

    permission = denied_permission(diagnostic)
    if permission:
        details.append(f"permission={permission}")
        namespace, resource = permission.split(".", 2)[:2]
        service = PERMISSION_SERVICES.get(namespace)
        if service:
            details.append(f"service={service}")
        resource_type = RESOURCE_TYPES.get((namespace, resource))
        if resource_type:
            details.append(f"resource_type={resource_type}")
    else:
        service = next((
            name for domain, name in DOMAIN_SERVICES.items()
            if re.search(rf"(?<![A-Za-z0-9.-]){re.escape(domain)}(?![A-Za-z0-9.-])", diagnostic)
        ), None)
        if service:
            details.append(f"service={service}")
    address = terraform_address(diagnostic)
    operation = terraform_operation(diagnostic, permission)
    if address:
        details.append(f"address={address}")
    if operation:
        details.append(f"operation={operation}")
    return details


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--stage", choices=sorted(STAGES), required=True)
    parser.add_argument("diagnostic_file")
    args = parser.parse_args()
    try:
        with open(args.diagnostic_file, "r", encoding="utf-8", errors="replace") as stream:
            diagnostic = stream.read(2_000_000)
        category = classify(args.stage, diagnostic)
    except OSError:
        category = "unclassified"
    # Never echo diagnostic text, even when it contains values or credentials.
    details = safe_details(diagnostic) if category == "iam" else []
    detail = f" {' '.join(details)}" if details else ""
    print(f"GCP08_TERRAFORM_FAILURE stage={args.stage} category={category}{detail}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
