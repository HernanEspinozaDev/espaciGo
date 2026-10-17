#!/usr/bin/env python3
"""Prepare one run-bound postgres credential and exact Secret Manager version.

This is intentionally not invoked by Terraform or by the build/test workflow.
It requires exact target confirmations and --execute. Password bytes remain in
process memory; they are never accepted through arguments/environment or printed.
"""

from __future__ import annotations

import argparse
import base64
import http.client
import json
import secrets
import subprocess
import sys
import time
from urllib.parse import quote, urlencode

PROJECT = "espacigo-testprod"
PROJECT_NUMBER = "447373903153"
INSTANCE = "espacigo-gcp03-origin-g03o-1017-pg"
REGION = "southamerica-west1"
DATABASE = "espacigo_scratch"
SECRET = "espacigo-gcp03-origin-g03o-1017-postgres-password"
TARGET_CONFIRMATION = f"projects/{PROJECT}/instances/{INSTANCE}/{DATABASE}"
SECRET_CONFIRMATION = f"projects/{PROJECT}/secrets/{SECRET}"
SECRET_RESOURCE = f"projects/{PROJECT_NUMBER}/secrets/{SECRET}"


class ApiFailure(RuntimeError):
    def __init__(self, message: str, *, status: int | None = None, uncertain: bool = False):
        super().__init__(message)
        self.status = status
        self.uncertain = uncertain


def api_request(host: str, path: str, method: str, token: str, body: dict | None = None) -> dict:
    payload = None if body is None else json.dumps(body, separators=(",", ":")).encode()
    headers = {
        "Authorization": f"Bearer {token}",
        "Accept": "application/json",
        "X-Goog-User-Project": PROJECT,
    }
    if payload is not None:
        headers["Content-Type"] = "application/json"
    connection = None
    try:
        connection = http.client.HTTPSConnection(host, timeout=30)
        connection.request(method, path, body=payload, headers=headers)
        response = connection.getresponse()
        raw = response.read()
        status = response.status
    except (OSError, http.client.HTTPException):
        raise ApiFailure("API transport outcome is uncertain; reconcile before retrying", uncertain=method != "GET") from None
    finally:
        if connection is not None:
            try:
                connection.close()
            except OSError:
                pass
    if status < 200 or status >= 300:
        raise ApiFailure(f"API request rejected (HTTP {status})", status=status, uncertain=status >= 500 and method != "GET")
    try:
        return json.loads(raw) if raw else {}
    except (UnicodeDecodeError, json.JSONDecodeError):
        raise ApiFailure("API returned an unreadable response; reconcile before retrying", uncertain=method != "GET") from None


def get_token() -> str:
    try:
        result = subprocess.run(
            ["gcloud", "auth", "print-access-token", f"--project={PROJECT}"],
            check=True,
            capture_output=True,
            text=True,
            timeout=30,
        )
    except (OSError, subprocess.SubprocessError):
        raise ApiFailure("could not obtain a Google API access token") from None
    token = result.stdout.strip()
    if not token or "\n" in token:
        raise ApiFailure("Google API access token was unavailable")
    return token


def canonical_resource_name(value: str, suffix: str) -> str:
    """Accept only this project ID or its known number; return numeric form."""
    accepted = {
        f"projects/{PROJECT}/{suffix}": f"projects/{PROJECT_NUMBER}/{suffix}",
        f"projects/{PROJECT_NUMBER}/{suffix}": f"projects/{PROJECT_NUMBER}/{suffix}",
    }
    try:
        return accepted[value]
    except KeyError:
        raise ApiFailure("Google API returned a resource outside the fixed project/secret target") from None


def canonical_version_name(value: str) -> str:
    for project in (PROJECT, PROJECT_NUMBER):
        prefix = f"projects/{project}/secrets/{SECRET}/versions/"
        if value.startswith(prefix) and value[len(prefix) :].isdigit() and int(value[len(prefix) :]) > 0:
            return f"{SECRET_RESOURCE}/versions/{value[len(prefix):]}"
    raise ApiFailure("Secret Manager returned a version outside the fixed project/secret target")


def validate_target(token: str) -> None:
    base = f"/sql/v1beta4/projects/{PROJECT}/instances/{INSTANCE}"
    instance = api_request("sqladmin.googleapis.com", base, "GET", token)
    settings = instance.get("settings") or {}
    ip_config = settings.get("ipConfiguration") or {}
    labels = settings.get("userLabels") or {}
    if (
        instance.get("name") != INSTANCE
        or instance.get("project") != PROJECT
        or instance.get("region") != REGION
        or instance.get("databaseVersion") != "POSTGRES_18"
        or instance.get("state") != "RUNNABLE"
        or ip_config.get("ipv4Enabled") is not False
        or labels.get("run_id") != "g03o_1017"
    ):
        raise ApiFailure("Cloud SQL instance does not match the fixed private GCP-03 target")
    databases = api_request("sqladmin.googleapis.com", f"{base}/databases", "GET", token)
    if not any(database.get("name") == DATABASE for database in databases.get("items", [])):
        raise ApiFailure("the fixed scratch database is not present on the confirmed Cloud SQL instance")

    secret = api_request("secretmanager.googleapis.com", f"/v1/{SECRET_RESOURCE}", "GET", token)
    if canonical_resource_name(secret.get("name", ""), f"secrets/{SECRET}") != SECRET_RESOURCE:
        raise ApiFailure("Secret Manager resource does not match the fixed GCP-03 target")
    page_token = ""
    while True:
        query = {"pageSize": "100"}
        if page_token:
            query["pageToken"] = page_token
        versions = api_request(
            "secretmanager.googleapis.com",
            f"/v1/{SECRET_RESOURCE}/versions?{urlencode(query)}",
            "GET",
            token,
        )
        if versions.get("versions"):
            raise ApiFailure("the exact secret already has versions; reconcile them before any credential operation")
        page_token = versions.get("nextPageToken", "")
        if not page_token:
            break


def wait_for_sql_operation(token: str, operation_name: str) -> None:
    path = f"/sql/v1beta4/projects/{PROJECT}/operations/{quote(operation_name, safe='')}"
    deadline = time.monotonic() + 600
    while time.monotonic() < deadline:
        operation = api_request("sqladmin.googleapis.com", path, "GET", token)
        if operation.get("status") == "DONE":
            if operation.get("error"):
                raise ApiFailure("Cloud SQL rejected the credential update (operation completed with error)", status=400)
            return
        time.sleep(2)
    raise ApiFailure("Cloud SQL operation is still pending; reconcile the same operation without rotating")


def disable_version(token: str, version_name: str) -> bool:
    path = f"/v1/{version_name}:disable"
    try:
        api_request("secretmanager.googleapis.com", path, "POST", token, {})
        return True
    except ApiFailure:
        return False


def execute() -> str:
    token = get_token()
    validate_target(token)
    password = secrets.token_urlsafe(48)
    secret_path = f"{SECRET_RESOURCE}:addVersion"
    secret_response = api_request(
        "secretmanager.googleapis.com",
        f"/v1/{secret_path}",
        "POST",
        token,
        {"payload": {"data": base64.b64encode(password.encode()).decode("ascii")}},
    )
    version_name = canonical_version_name(secret_response.get("name", ""))
    prefix = f"{SECRET_RESOURCE}/versions/"
    if not version_name.startswith(prefix) or not version_name[len(prefix) :].isdigit():
        raise ApiFailure("secret version was created but its numeric identity is unclear; inspect the exact secret and do not retry")

    user_path = f"/sql/v1beta4/projects/{PROJECT}/instances/{INSTANCE}/users?{urlencode({'name': 'postgres'})}"
    try:
        operation = api_request(
            "sqladmin.googleapis.com",
            user_path,
            "PUT",
            token,
            {"name": "postgres", "password": password},
        )
    except ApiFailure as error:
        if error.status is not None and 400 <= error.status < 500:
            disabled = disable_version(token, version_name)
            if disabled:
                raise ApiFailure("Cloud SQL rejected the update; the unused secret version was disabled; no retry was attempted") from None
            raise ApiFailure(f"Cloud SQL rejected the update; secret version {version_name} may remain enabled and needs reconciliation") from None
        raise ApiFailure(f"credential update outcome is uncertain; keep and reconcile exact secret version {version_name}; do not rotate or rerun") from None
    operation_name = operation.get("name", "")
    if not operation_name:
        raise ApiFailure(f"Cloud SQL returned no operation identity; keep and reconcile exact secret version {version_name}; do not rotate or rerun")
    try:
        wait_for_sql_operation(token, operation_name)
    except ApiFailure:
        raise ApiFailure(f"Cloud SQL operation status is uncertain; keep and reconcile exact secret version {version_name}; do not rotate or rerun") from None
    return version_name


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--confirm-target", required=True)
    parser.add_argument("--confirm-secret", required=True)
    parser.add_argument("--execute", action="store_true", help="allow the fixed Cloud SQL and Secret Manager operations")
    args = parser.parse_args(argv)
    if args.confirm_target != TARGET_CONFIRMATION or args.confirm_secret != SECRET_CONFIRMATION:
        parser.error("target confirmation does not match the fixed GCP-03 run")
    if not args.execute:
        parser.error("--execute is required; this helper never runs from Terraform")
    try:
        version = execute()
    except ApiFailure as error:
        print(f"GCP-03 credential operation stopped: {error}", file=sys.stderr)
        return 1
    print(f"Cloud SQL postgres password set for {TARGET_CONFIRMATION}; exact Secret Manager version: {version}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
