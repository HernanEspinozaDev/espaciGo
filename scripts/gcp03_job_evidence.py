#!/usr/bin/env python3
"""Require both Cloud Run task success and matching application phase evidence."""
from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any

SAFE_CODE = re.compile(r"^[A-Z][A-Z0-9_]{0,63}$")
SAFE_SUBSTEP = re.compile(r"^[a-z][a-z0-9_]{0,63}$")


def _object_payload(entry: dict[str, Any]) -> dict[str, Any] | None:
    value = entry.get("jsonPayload")
    if isinstance(value, dict):
        return value
    text = entry.get("textPayload")
    if isinstance(text, str) and text.startswith("{"):
        try:
            decoded = json.loads(text)
        except (json.JSONDecodeError, TypeError):
            return None
        return decoded if isinstance(decoded, dict) else None
    return None


def _app_container(phase: str) -> str:
    return phase


def _stream(entry: dict[str, Any]) -> str:
    name = entry.get("logName")
    if not isinstance(name, str):
        return ""
    return name.rsplit("%2F", 1)[-1].rsplit("/", 1)[-1].lower()


def validate(execution: dict[str, Any], logs: list[dict[str, Any]], phase: str, run_id: str) -> tuple[bool, dict[str, Any]]:
    status = execution.get("status") or {}
    completed = any(c.get("type") == "Completed" and c.get("status") == "True" for c in status.get("conditions", []))
    succeeded = status.get("succeededCount") == 1
    summary: dict[str, Any] = {"phase": phase, "run_id": run_id, "cloud_run_completed": completed, "succeeded_count": status.get("succeededCount"), "application_evidence": "missing"}
    if not completed or not succeeded:
        summary["result"] = "cloud_run_task_not_successful"
        return False, summary

    main_container = _app_container(phase)
    failures: list[dict[str, str]] = []
    successes: list[dict[str, Any]] = []
    for entry in logs:
        labels = entry.get("labels") or {}
        if labels.get("container_name") != main_container:
            continue
        payload = _object_payload(entry)
        if not payload or payload.get("phase") != phase or payload.get("run_id") != run_id:
            continue
        if "error_code" in payload:
            item = {"error_code": payload.get("error_code", "")}
            if isinstance(payload.get("substep"), str):
                item["substep"] = payload["substep"]
            exit_code = payload.get("application_exit_code")
            if isinstance(exit_code, int) and 0 <= exit_code <= 255:
                item["application_exit_code"] = exit_code
            failures.append(item)
        elif (_stream(entry) == "stdout" and isinstance(payload.get("format"), str)
              and payload["format"].startswith("espacigo-gcp03-")):
            successes.append(payload)

    if failures:
        failure = failures[-1]
        code = failure.get("error_code", "")
        step = failure.get("substep", "")
        summary["application_evidence"] = "failure"
        summary["error_code"] = code if isinstance(code, str) and SAFE_CODE.fullmatch(code) else "UNSAFE_OR_UNKNOWN_CODE"
        exit_code = failure.get("application_exit_code")
        if isinstance(exit_code, int) and 0 <= exit_code <= 255:
            summary["application_exit_code"] = exit_code
        if isinstance(step, str) and SAFE_SUBSTEP.fullmatch(step):
            summary["substep"] = step
        summary["result"] = "application_phase_failed_despite_cloud_run_success"
        return False, summary

    matching = [p for p in successes if p.get("phase") == phase and p.get("run_id") == run_id]
    if len(matching) != 1:
        summary["result"] = "application_success_evidence_missing_or_ambiguous"
        return False, summary
    evidence = matching[0]
    if phase == "retire" and evidence.get("commit_outcome") != "committed":
        summary["result"] = "retirement_commit_not_confirmed"
        return False, summary
    if phase == "retire-diagnose":
        diag = evidence.get("retirement_diagnosis")
        if not isinstance(diag, dict) or diag.get("read_only_transaction") is not True:
            summary["result"] = "read_only_diagnosis_not_confirmed"
            return False, summary
    summary["application_evidence"] = "success"
    summary["result"] = "success"
    return True, summary


def _read_json(path: str) -> Any:
    return json.loads(Path(path).read_text(encoding="utf-8"))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--execution-json", required=True)
    parser.add_argument("--logs-json", required=True)
    parser.add_argument("--phase", required=True)
    parser.add_argument("--run-id", required=True)
    args = parser.parse_args()
    try:
        execution = _read_json(args.execution_json)
        logs = _read_json(args.logs_json)
        if not isinstance(execution, dict) or not isinstance(logs, list) or not all(isinstance(e, dict) for e in logs):
            raise ValueError("input JSON shape is invalid")
    except (OSError, json.JSONDecodeError, ValueError):
        print(json.dumps({"result": "invalid_evidence_input"}, sort_keys=True))
        return 2
    valid, summary = validate(execution, logs, args.phase, args.run_id)
    print(json.dumps(summary, sort_keys=True))
    return 0 if valid else 1


if __name__ == "__main__":
    sys.exit(main())
