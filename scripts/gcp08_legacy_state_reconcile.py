#!/usr/bin/env python3
"""Inspect and reconcile only the two confirmed legacy GCP-08 SQL state bindings."""
import argparse
import copy
import hashlib
import json
import os
import re
import subprocess
from pathlib import Path


PROJECT = "espacigo-testprod"
BUCKET = "espacigo-testprod-tfstate-447373903153-gcp"
STATE_OBJECT = "espacigo/testprod/gcp/default.tfstate"
LEGACY_INSTANCE = "espacigo-testprod-pg"
LEGACY_DATABASE = "espacigo"
V67_INSTANCE = "espacigo-testprod-db"
TARGETS = (
    "google_sql_database_instance.postgres[0]",
    "google_sql_database.application[0]",
)
MAX_STATE_BYTES = 20_000_000
CONFIRM_REMOVE = "REMOVE ONLY GCP08 LEGACY SQL STATE BINDINGS"
CONFIRM_RESTORE = "RESTORE GCP08 STATE BACKUP"
EXPECTED_REPOSITORY = "HernanEspinozaDev/espaciGo"
EXPECTED_REPOSITORY_ID = "1361448198"
EXPECTED_OWNER_ID = "176093371"
EXPECTED_ACTOR_ID = "176093371"
EXPECTED_REF = "refs/heads/main"
EXPECTED_WORKFLOW_REF = (
    "HernanEspinozaDev/espaciGo/.github/workflows/application-rollout.yml@refs/heads/main"
)


class ReconcileError(Exception):
    """A fail-closed state or identity precondition failed."""


def _write_private(path, data):
    path = Path(path)
    flags = os.O_WRONLY | os.O_CREAT | os.O_EXCL
    descriptor = os.open(path, flags, 0o600)
    with os.fdopen(descriptor, "wb") as stream:
        stream.write(data)
    os.chmod(path, 0o600)


def _run_private(command, private_dir, name, runner=subprocess.run, timeout=120, cwd=None):
    """Capture raw command output in a 0600 file; never return it to stdout."""
    private_dir = Path(private_dir)
    private_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_dir, 0o700)
    output_path = private_dir / name
    descriptor = os.open(output_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    env = os.environ.copy()
    for key in tuple(env):
        if key.startswith("TF_LOG"):
            del env[key]
    try:
        with os.fdopen(descriptor, "wb") as output:
            result = runner(
                command, stdout=output, stderr=subprocess.STDOUT, env=env,
                timeout=timeout, check=False, cwd=str(cwd) if cwd is not None else None,
            )
    except (OSError, subprocess.TimeoutExpired) as error:
        raise ReconcileError("private_command_failed") from error
    if result.returncode != 0:
        raise ReconcileError("private_command_failed")
    return output_path


def _state_addresses(state):
    resources = state.get("resources")
    if not isinstance(resources, list):
        raise ReconcileError("state_shape_invalid")
    entries = {}
    for resource in resources:
        if not isinstance(resource, dict):
            raise ReconcileError("state_shape_invalid")
        mode = resource.get("mode")
        resource_type = resource.get("type")
        name = resource.get("name")
        module = resource.get("module")
        if mode not in {"managed", "data"} or not isinstance(resource_type, str) or not isinstance(name, str):
            raise ReconcileError("state_shape_invalid")
        base = _resource_base(resource)
        instances = resource.get("instances")
        if not isinstance(instances, list):
            raise ReconcileError("state_shape_invalid")
        for instance in instances:
            if not isinstance(instance, dict):
                raise ReconcileError("state_shape_invalid")
            address = _instance_address(base, instance)
            if address in entries:
                raise ReconcileError("state_duplicate_address")
            attrs = instance.get("attributes")
            if not isinstance(attrs, dict):
                raise ReconcileError("state_shape_invalid")
            entries[address] = attrs
    return entries


def _resource_base(resource):
    prefix = f"{resource['module']}." if isinstance(resource.get("module"), str) and resource["module"] else ""
    if resource.get("mode") == "data":
        prefix += "data."
    return f"{prefix}{resource['type']}.{resource['name']}"


def _instance_address(base, instance):
    index = instance.get("index_key")
    if isinstance(index, bool):
        raise ReconcileError("state_shape_invalid")
    if isinstance(index, int):
        return f"{base}[{index}]"
    if isinstance(index, str):
        return f"{base}[{json.dumps(index)}]"
    if index is None:
        return base
    raise ReconcileError("state_shape_invalid")


def _required_string(attrs, key):
    value = attrs.get(key)
    return value if isinstance(value, str) else None


def analyze_state(state):
    """Return only fixed aliases, booleans, lineage presence, and the serial."""
    if not isinstance(state, dict):
        raise ReconcileError("state_shape_invalid")
    lineage = state.get("lineage")
    serial = state.get("serial")
    if not isinstance(lineage, str) or not re.fullmatch(r"[0-9a-fA-F-]{36}", lineage):
        raise ReconcileError("state_metadata_invalid")
    if isinstance(serial, bool) or not isinstance(serial, int) or serial < 0:
        raise ReconcileError("state_metadata_invalid")
    entries = _state_addresses(state)
    instance = entries.get(TARGETS[0])
    database = entries.get(TARGETS[1])
    instance_matches = (
        instance is not None
        and _required_string(instance, "name") == LEGACY_INSTANCE
        and _required_string(instance, "project") == PROJECT
    )
    database_instance = _required_string(database, "instance") if database else None
    database_matches = (
        database is not None
        and _required_string(database, "name") == LEGACY_DATABASE
        and _required_string(database, "project") == PROJECT
        and database_instance in {
            LEGACY_INSTANCE,
            f"projects/{PROJECT}/instances/{LEGACY_INSTANCE}",
        }
    )
    return {
        "serial": serial,
        "lineage_present": True,
        "legacy_instance_count": int(instance is not None),
        "legacy_instance_matches": instance_matches,
        "legacy_database_count": int(database is not None),
        "legacy_database_matches": database_matches,
        "database_bound_to_legacy_instance": database_instance in {
            LEGACY_INSTANCE,
            f"projects/{PROJECT}/instances/{LEGACY_INSTANCE}",
        },
        "database_is_v67": database_instance in {
            V67_INSTANCE,
            f"projects/{PROJECT}/instances/{V67_INSTANCE}",
        },
        "eligible": bool(instance_matches and database_matches),
    }


def verify_identity(context):
    expected = {
        "repository": EXPECTED_REPOSITORY,
        "repository_id": EXPECTED_REPOSITORY_ID,
        "owner_id": EXPECTED_OWNER_ID,
        "actor_id": EXPECTED_ACTOR_ID,
        "ref": EXPECTED_REF,
        "workflow_ref": EXPECTED_WORKFLOW_REF,
        "event_name": "workflow_dispatch",
        "environment": "testprod",
    }
    return all(context.get(key) == value for key, value in expected.items())


def state_fingerprint_without_targets(state):
    normalized = copy.deepcopy(state)
    normalized.pop("serial", None)
    resources = normalized.get("resources")
    if not isinstance(resources, list):
        raise ReconcileError("state_shape_invalid")
    retained = []
    for resource in resources:
        instances = resource.get("instances")
        if not isinstance(instances, list):
            raise ReconcileError("state_shape_invalid")
        base = _resource_base(resource)
        retained_instances = [
            instance for instance in instances
            if _instance_address(base, instance) not in TARGETS
        ]
        if retained_instances:
            kept = copy.deepcopy(resource)
            kept["instances"] = retained_instances
            retained.append(kept)
    normalized["resources"] = retained
    payload = json.dumps(normalized, sort_keys=True, separators=(",", ":")).encode("utf-8")
    return hashlib.sha256(payload).hexdigest()


def _read_state(terraform_dir, private_dir, runner=subprocess.run, timeout=120, name="state.json"):
    path = Path(private_dir) / name
    command = ["terraform", "state", "pull"]
    _run_private(command, private_dir, name, runner=runner, timeout=timeout, cwd=terraform_dir)
    raw = path.read_bytes()
    if not raw or len(raw) > MAX_STATE_BYTES:
        raise ReconcileError("state_size_invalid")
    try:
        state = json.loads(raw)
    except (UnicodeDecodeError, json.JSONDecodeError) as error:
        raise ReconcileError("state_json_invalid") from error
    return state


def inspect(terraform_dir, private_dir, runner=subprocess.run, timeout=120):
    workspace = _run_private(
        ["terraform", "workspace", "show"], private_dir, "workspace.log", runner, timeout,
        cwd=terraform_dir,
    ).read_text(encoding="utf-8", errors="replace").strip()
    if workspace != "default":
        raise ReconcileError("workspace_mismatch")
    state = _read_state(terraform_dir, private_dir, runner=runner, timeout=timeout)
    return state, analyze_state(state)


def _gcs_uri(object_name):
    return f"gs://{BUCKET}/{object_name}"


def _object_metadata(uri, private_dir, name, runner=subprocess.run, timeout=120):
    path = _run_private(
        ["gcloud", "storage", "objects", "describe", uri, "--format=json", "--quiet"],
        private_dir, name, runner, timeout,
    )
    try:
        metadata = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as error:
        raise ReconcileError("object_metadata_invalid") from error
    def normalize_alias(primary, alternate):
        values = [metadata[key] for key in (primary, alternate) if key in metadata]
        if not values or any(value != values[0] for value in values[1:]):
            raise ReconcileError("object_metadata_invalid")
        return values[0]

    result = {
        "name": metadata.get("name"),
        "generation": metadata.get("generation"),
        "size": metadata.get("size"),
        "crc32c": normalize_alias("crc32c_hash", "crc32c"),
        "md5Hash": normalize_alias("md5_hash", "md5Hash"),
    }
    for key in ("generation", "size"):
        if isinstance(result[key], int) and not isinstance(result[key], bool):
            result[key] = str(result[key])
    if (not isinstance(result["name"], str) or not result["name"] or not all(
        isinstance(result[key], str) and result[key]
        for key in ("generation", "size", "crc32c", "md5Hash")
    )):
        raise ReconcileError("object_metadata_invalid")
    return result


def _create_and_verify_backup(run_id, run_attempt, private_dir, runner, timeout):
    if not re.fullmatch(r"[1-9][0-9]*", run_id) or not re.fullmatch(r"[1-9][0-9]*", run_attempt):
        raise ReconcileError("run_identity_invalid")
    source_uri = _gcs_uri(STATE_OBJECT)
    backup_object = (
        f"espacigo/testprod/gcp/manual-state-backups/gcp08-legacy-sql/"
        f"{run_id}-{run_attempt}/default.tfstate"
    )
    backup_uri = _gcs_uri(backup_object)
    source = _object_metadata(source_uri, private_dir, "source-object.json", runner, timeout)
    source_generation = source["generation"]
    command = [
        "gcloud", "storage", "cp", f"{source_uri}#{source_generation}", backup_uri,
        "--if-generation-match=0", "--quiet",
    ]
    _run_private(command, private_dir, "gcs-copy.log", runner, timeout)
    backup = _object_metadata(backup_uri, private_dir, "backup-object.json", runner, timeout)
    fields_match = all(source[key] == backup[key] for key in ("size", "crc32c", "md5Hash"))
    if not fields_match or source["name"] != STATE_OBJECT or backup["name"] != backup_object:
        raise ReconcileError("backup_verification_failed")
    return backup_object, backup


def _remove(terraform_dir, private_dir, expected_serial, absence_attested,
            run_id, run_attempt, runner=subprocess.run, timeout=120):
    state, before = inspect(terraform_dir, private_dir, runner=runner, timeout=timeout)
    if not before["eligible"] or not before["database_bound_to_legacy_instance"] or before["database_is_v67"]:
        raise ReconcileError("legacy_binding_mismatch")
    if expected_serial != before["serial"]:
        raise ReconcileError("state_serial_mismatch")
    if absence_attested is not True:
        raise ReconcileError("legacy_absence_not_attested")
    before_fingerprint = state_fingerprint_without_targets(state)
    backup_object, _ = _create_and_verify_backup(run_id, run_attempt, private_dir, runner, timeout)

    # Re-read after the server-side backup to bind it to the exact inspected serial.
    latest, latest_summary = inspect(
        terraform_dir, Path(private_dir) / "latest", runner=runner, timeout=timeout,
    )
    if latest_summary["serial"] != before["serial"] or not latest_summary["eligible"]:
        raise ReconcileError("state_changed_before_remove")
    if state_fingerprint_without_targets(latest) != before_fingerprint:
        raise ReconcileError("state_changed_before_remove")

    command = [
        "terraform", "state", "rm", "-lock=true", "-lock-timeout=5m",
        *TARGETS,
    ]
    _run_private(command, private_dir, "state-rm.log", runner=runner, timeout=timeout,
                 cwd=terraform_dir)
    after = _read_state(terraform_dir, Path(private_dir) / "after", runner=runner,
                        timeout=timeout, name="state-after.json")
    after_summary = analyze_state(after)
    if any(address in _state_addresses(after) for address in TARGETS):
        raise ReconcileError("target_binding_remains")
    if after.get("lineage") != state.get("lineage"):
        raise ReconcileError("state_lineage_changed")
    if after_summary["serial"] <= before["serial"]:
        raise ReconcileError("state_serial_did_not_advance")
    if state_fingerprint_without_targets(after) != before_fingerprint:
        raise ReconcileError("unrelated_state_changed")
    return backup_object, after_summary


def _context_from_env():
    return {
        "repository": os.environ.get("GITHUB_REPOSITORY"),
        "repository_id": os.environ.get("GITHUB_REPOSITORY_ID"),
        "owner_id": os.environ.get("GITHUB_REPOSITORY_OWNER_ID"),
        "actor_id": os.environ.get("GITHUB_ACTOR_ID"),
        "ref": os.environ.get("GITHUB_REF"),
        "workflow_ref": os.environ.get("GITHUB_WORKFLOW_REF"),
        "event_name": os.environ.get("GITHUB_EVENT_NAME"),
        "environment": os.environ.get("GCP08_CLEANUP_ENVIRONMENT"),
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("inspect", "remove"))
    parser.add_argument("--terraform-dir", required=True)
    parser.add_argument("--private-dir", required=True)
    parser.add_argument("--expected-state-serial", type=int)
    parser.add_argument("--run-id")
    parser.add_argument("--run-attempt")
    args = parser.parse_args()
    private_dir = Path(args.private_dir)
    private_dir.mkdir(mode=0o700, parents=True, exist_ok=True)
    os.chmod(private_dir, 0o700)
    try:
        if not verify_identity(_context_from_env()):
            raise ReconcileError("identity_mismatch")
        if args.mode == "inspect":
            _, summary = inspect(args.terraform_dir, private_dir)
            category = "eligible" if summary["eligible"] else "binding_mismatch"
            print(
                f"GCP08_STATE_RECONCILE mode=inspect category={category} "
                f"backend=gcs workspace=default state_serial={summary['serial']} "
                f"legacy_instance_count={summary['legacy_instance_count']} "
                f"legacy_instance_matches={str(summary['legacy_instance_matches']).lower()} "
                f"legacy_database_count={summary['legacy_database_count']} "
                f"legacy_database_matches={str(summary['legacy_database_matches']).lower()} "
                f"database_bound_to_legacy_instance={str(summary['database_bound_to_legacy_instance']).lower()} "
                f"database_is_v67={str(summary['database_is_v67']).lower()}"
            )
            return 0 if summary["eligible"] else 2
        if (os.environ.get("GCP08_STATE_CLEANUP_CONFIRMATION") != CONFIRM_REMOVE
                or os.environ.get("GCP08_LEGACY_ABSENCE_ATTESTATION") != "owner-get-404"
                or args.expected_state_serial is None or not args.run_id or not args.run_attempt):
            raise ReconcileError("operation_gate_failed")
        backup_object, after = _remove(
            args.terraform_dir, private_dir, args.expected_state_serial,
            os.environ.get("GCP08_LEGACY_ABSENCE_ATTESTATION") == "owner-get-404",
            args.run_id, args.run_attempt,
        )
        print(
            "GCP08_STATE_RECONCILE mode=remove category=success "
            f"removed_bindings=2 state_serial={after['serial']} "
            f"backup=verified backup_run={args.run_id}-{args.run_attempt}"
        )
        return 0
    except ReconcileError as error:
        print(f"GCP08_STATE_RECONCILE mode={args.mode} category=blocked reason={error}")
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
