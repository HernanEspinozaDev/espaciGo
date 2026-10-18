"""Local, non-executing planner for a reviewed GCP-03 normalization re-entry.

This module is intentionally not imported by the Cloud Run importer. It only
classifies fixed normalization statements from caller-supplied catalog facts;
it has no database, object-store, or subprocess access.
"""
from __future__ import annotations

from dataclasses import dataclass
import hashlib
import json
import re
from typing import Protocol

from gcp03_origin_normalization import NormalizationValidationError, validate_normalization_sql
from gcp03_origin_private_files import (
    PrivateFile,
    PrivateFileStore,
    copy_validated_private_files,
    validate_private_file_references,
)

FORMAT = "espacigo.g03o.normalization-reentry-plan.v1"
OWNER_RE = re.compile(
    r"^ALTER\s+(?:TABLE|SEQUENCE|TYPE|DOMAIN|FUNCTION|PROCEDURE|AGGREGATE|"
    r"TEXT\s+SEARCH\s+DICTIONARY|TEXT\s+SEARCH\s+CONFIGURATION)\b.*"
    r"\bOWNER\s+TO\s+espacigo_runtime$",
    re.IGNORECASE | re.DOTALL,
)
ALLOWED_OWNER_FACTS = {"postgres", "espacigo_runtime"}
ALLOWED_ACL_FACTS = {"satisfied", "missing"}
ACL_ACTION_RE = re.compile(r"\b(GRANT|REVOKE)\b", re.IGNORECASE)
ACL_ROLE_RE = re.compile(r"\s+(TO|FROM)\s+(.+)$", re.IGNORECASE | re.DOTALL)
TEMP_CREATE_EFFECT = "create on schema public to espacigo_runtime"


class RecoveryPlanError(ValueError):
    """Sanitized fail-closed error; never includes SQL or catalog object names."""


@dataclass(frozen=True)
class PlannedStatement:
    ordinal: int
    statement_id: str
    category: str
    action: str
    sql: str


@dataclass(frozen=True)
class NormalizationRecoveryPlan:
    summary: dict
    pending_sql: bytes


class RecoveryDatabase(Protocol):
    def normalize_owners_and_acls(self, sql: bytes) -> None: ...
    def verify_normalized(self) -> dict: ...
    def sanitize(self) -> dict: ...
    def private_file_refs(self) -> set[str]: ...


class RecoveryStop(RuntimeError):
    def __init__(self, code: str, stage: str, transaction_outcome: str = "not_started"):
        super().__init__(code)
        self.code, self.stage, self.transaction_outcome = code, stage, transaction_outcome


REQUIRED_SANITATION_COUNTS = (
    "valid_sessions", "usable_action_tokens", "pending_notices", "identity_event_leases"
)
REQUIRED_OWNER_POSTCHECKS = (
    "relations_owned_by_runtime", "routines_owned_by_runtime",
    "text_search_owned_by_runtime", "sequences_parent_owner_consistent",
    "observer_write_privileges_absent", "runtime_public_create_absent",
)


def statement_id(sql: str) -> str:
    return hashlib.sha256(sql.encode("utf-8")).hexdigest()


def _acl_effect(sql: str) -> tuple[str, bool]:
    action_match = ACL_ACTION_RE.search(sql)
    if action_match is None:
        raise RecoveryPlanError("an allowlisted statement could not be classified")
    action = action_match.group(1).upper()
    remainder = sql[action_match.end():]
    target_match = ACL_ROLE_RE.search(remainder)
    if target_match is None:
        raise RecoveryPlanError("an allowlisted ACL target could not be classified")
    privileges_and_object = " ".join(remainder[:target_match.start()].split())
    target = " ".join(target_match.group(2).split())
    effect = f"{privileges_and_object} TO {target}".casefold()
    return effect, action == "GRANT"


def build_normalization_recovery_plan(
    normalization_sql: bytes,
    expected_normalization_sha256: str,
    statement_facts: dict[str, str],
    owned_sequence_parent_statement: dict[str, str] | None = None,
    binding: dict | None = None,
) -> NormalizationRecoveryPlan:
    """Classify exact statements as satisfied or pending; never executes them.

    Owner facts must be the current owner role name (`postgres` or
    `espacigo_runtime`). ACL facts must be `satisfied` or `missing`, based on
    catalog checks for the exact statement's privilege effect. Any missing,
    unknown, contradictory, or surplus fact blocks plan creation.
    """
    if not isinstance(normalization_sql, bytes) or not normalization_sql:
        raise RecoveryPlanError("normalization input is missing")
    digest = hashlib.sha256(normalization_sql).hexdigest()
    if not re.fullmatch(r"[0-9a-f]{64}", expected_normalization_sha256) or digest != expected_normalization_sha256:
        raise RecoveryPlanError("normalization input hash mismatch")
    try:
        validate_normalization_sql(normalization_sql, expected_normalization_sha256)
        decoded = normalization_sql.decode("utf-8", "strict")
    except (NormalizationValidationError, UnicodeDecodeError) as exc:
        raise RecoveryPlanError("normalization input failed the fixed allowlist") from exc

    statements = [part.strip() for part in decoded.split(";") if part.strip()]
    ids = [statement_id(part) for part in statements]
    if len(ids) != len(set(ids)):
        raise RecoveryPlanError("normalization input contains duplicate statements")
    if not isinstance(statement_facts, dict) or set(statement_facts) != set(ids):
        raise RecoveryPlanError("catalog fact set does not match the normalization input")

    owner_pending = 0
    sequence_statement_ids = {
        statement_id(sql) for sql in statements
        if re.match(r"^ALTER\s+SEQUENCE\b", sql, re.IGNORECASE)
    }
    sequence_parents = owned_sequence_parent_statement or {}
    table_statement_ids = {
        statement_id(sql) for sql in statements
        if re.match(r"^ALTER\s+TABLE\b", sql, re.IGNORECASE)
    }
    if not isinstance(sequence_parents, dict) or not set(sequence_parents).issubset(sequence_statement_ids) or \
            not set(sequence_parents.values()).issubset(table_statement_ids):
        raise RecoveryPlanError("owned-sequence fact set does not match normalization input")
    if any(statement_facts[parent_id] not in ALLOWED_OWNER_FACTS for parent_id in sequence_parents.values()):
        raise RecoveryPlanError("an owned-sequence parent fact is unexpected or contradictory")
    for sql in statements:
        if OWNER_RE.fullmatch(sql) and statement_facts[statement_id(sql)] == "postgres":
            owner_pending += 1

    planned: list[PlannedStatement] = []
    acl_initial_state: dict[str, bool] = {}
    acl_simulated_state: dict[str, bool] = {}
    for ordinal, sql in enumerate(statements, 1):
        sid = statement_id(sql)
        fact = statement_facts[sid]
        if OWNER_RE.fullmatch(sql):
            if fact not in ALLOWED_OWNER_FACTS:
                raise RecoveryPlanError("an owner fact is unexpected or contradictory")
            action = "skip_satisfied" if fact == "espacigo_runtime" else "apply_pending"
            category = "owner"
            parent_statement_id = sequence_parents.get(sid)
            if parent_statement_id is not None:
                parent_owner = statement_facts[parent_statement_id]
                if parent_owner != fact:
                    raise RecoveryPlanError("owned sequence and parent owners are inconsistent")
                if parent_owner == "postgres":
                    # ALTER TABLE changes ownership of serial/identity-owned
                    # sequences atomically. A second ALTER SEQUENCE would
                    # require the importer to still own the sequence.
                    action = "skip_satisfied"
                elif parent_owner == "espacigo_runtime" and fact == "postgres":
                    raise RecoveryPlanError("owned sequence and parent owners are inconsistent")
        else:
            if fact not in ALLOWED_ACL_FACTS:
                raise RecoveryPlanError("an ACL fact is unexpected or contradictory")
            effect, grant = _acl_effect(sql)
            actual_from_fact = grant if fact == "satisfied" else not grant
            if effect in acl_initial_state and acl_initial_state[effect] != actual_from_fact:
                raise RecoveryPlanError("ACL catalog facts contradict each other")
            acl_initial_state.setdefault(effect, actual_from_fact)
            simulated = acl_simulated_state.get(effect, actual_from_fact)
            # CREATE on public is temporary and needed only when a reviewed
            # owner transfer remains. Avoid opening/reclosing it when every
            # owner action is already satisfied.
            if effect == TEMP_CREATE_EFFECT and grant and owner_pending == 0:
                action = "skip_satisfied"
            else:
                action = "skip_satisfied" if simulated == grant else "apply_pending"
                if action == "apply_pending":
                    simulated = grant
            acl_simulated_state[effect] = simulated
            category = "acl"
        planned.append(PlannedStatement(ordinal, sid, category, action, sql))

    public_actions = [
        {"ordinal": p.ordinal, "statement_sha256": p.statement_id,
         "category": p.category, "action": p.action}
        for p in planned
    ]
    canonical = {
        "format": FORMAT,
        "normalization_sql_sha256": digest,
        "binding": binding or {},
        "catalog_facts_sha256": hashlib.sha256(json.dumps(
            {"statements": sorted(statement_facts.items()),
             "owned_sequence_parent_statement": sorted(sequence_parents.items())},
            sort_keys=True, separators=(",", ":")
        ).encode()).hexdigest(),
        "actions": public_actions,
    }
    plan_hash = hashlib.sha256(json.dumps(canonical, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    pending = [p.sql for p in planned if p.action == "apply_pending"]
    summary = {
        **canonical,
        "plan_sha256": plan_hash,
        "statement_count": len(planned),
        "owner_statement_count": sum(p.category == "owner" for p in planned),
        "acl_statement_count": sum(p.category == "acl" for p in planned),
        "pending_statement_count": len(pending),
        "satisfied_statement_count": len(planned) - len(pending),
        "contains_sql_text": False,
        "execution_performed": False,
    }
    # Pending SQL is returned only in memory so a separately reviewed runner
    # can bind it to this exact plan hash. It is never placed in the summary.
    pending_sql = (";\n".join(pending) + ";\n").encode("utf-8") if pending else b""
    return NormalizationRecoveryPlan(summary=summary, pending_sql=pending_sql)


def execute_reviewed_recovery(
    *,
    plan: NormalizationRecoveryPlan,
    approved_plan_sha256: str,
    database: RecoveryDatabase,
    private_files: list[PrivateFile],
    private_file_store: PrivateFileStore,
    run_id: str,
) -> dict:
    """Run only normalization, sanitation, and private copy in that order.

    The caller must have authenticated and fully validated the recovery
    capsule, preflight report, normalization SQL, and private-file archive.
    This function has no restore/archive API and never retries a failed phase.
    """
    report = {
        "format": "espacigo.g03o.recovery-execution.v1",
        "phase": "recover", "run_id": run_id,
        "status": "stopped", "phase_success": False, "report_complete": False,
        "transaction_outcome": "not_started", "retry": False,
        "normalization_plan_sha256": plan.summary.get("plan_sha256"),
    }
    if not _reviewed_plan_is_intact(plan) or approved_plan_sha256 != plan.summary.get("plan_sha256"):
        report.update(stage="plan_hash_check", error_code="APPROVED_PLAN_HASH_MISMATCH")
        return report
    try:
        if plan.pending_sql:
            report["stage"] = "owner_acl_normalization"
            database.normalize_owners_and_acls(plan.pending_sql)
        report["transaction_outcome"] = "committed"
    except RecoveryStop as exc:
        report.update(stage=exc.stage, error_code=exc.code, transaction_outcome=exc.transaction_outcome)
        return report
    except Exception:
        report.update(stage="owner_acl_normalization", error_code="OWNER_ACL_NORMALIZATION_FAILED",
                      transaction_outcome="uncertain")
        return report

    try:
        report["stage"] = "owner_acl_postcheck"
        owner_check = database.verify_normalized()
        if any(owner_check.get(key) is not True for key in REQUIRED_OWNER_POSTCHECKS):
            report.update(error_code="OWNER_ACL_POSTCHECK_FAILED", transaction_outcome="committed")
            return report
    except Exception:
        report.update(stage="owner_acl_postcheck", error_code="OWNER_ACL_POSTCHECK_FAILED",
                      transaction_outcome="committed")
        return report

    try:
        report["stage"] = "sanitation"
        sanitation = database.sanitize()
        if any(sanitation.get(key) != 0 for key in REQUIRED_SANITATION_COUNTS) or sanitation.get("runtime_role_login") is not False:
            report.update(error_code="SANITATION_POSTCONDITIONS_FAILED", transaction_outcome="uncertain")
            return report
    except RecoveryStop as exc:
        report.update(stage=exc.stage, error_code=exc.code, transaction_outcome=exc.transaction_outcome)
        return report
    except Exception:
        report.update(stage="sanitation", error_code="SANITATION_FAILED", transaction_outcome="uncertain")
        return report

    try:
        report["stage"] = "private_file_validation"
        references = database.private_file_refs()
        reference_report = validate_private_file_references(private_files, references, run_id=run_id)
    except Exception:
        report.update(error_code="PRIVATE_FILE_REFERENCE_MISMATCH", transaction_outcome="committed")
        return report

    try:
        report["stage"] = "private_file_copy"
        copied = copy_validated_private_files(private_files, run_id=run_id, store=private_file_store)
    except Exception:
        report.update(error_code="PRIVATE_FILE_COPY_FAILED", transaction_outcome="uncertain")
        return report

    inventory = sorted(
        f"{hashlib.sha256(str(item['object_name']).encode()).hexdigest()}:{item['bytes']}:{item['sha256']}"
        for item in copied
    )
    report.update(
        status="recovered_and_sanitized", phase_success=True, report_complete=True,
        stage="complete", private_file_count=len(copied),
        private_file_bytes=sum(int(item["bytes"]) for item in copied),
        private_file_inventory_sha256=hashlib.sha256("\n".join(inventory).encode()).hexdigest(),
        database_reference_count=reference_report["database_reference_count"],
        transaction_outcome="committed",
    )
    return report


def _reviewed_plan_is_intact(plan: NormalizationRecoveryPlan) -> bool:
    """Verify that mutable report data and in-memory SQL still match approval."""
    summary = plan.summary
    canonical_keys = (
        "format", "normalization_sql_sha256", "catalog_facts_sha256", "actions", "binding"
    )
    if not isinstance(summary, dict) or any(key not in summary for key in canonical_keys):
        return False
    canonical = {key: summary[key] for key in canonical_keys}
    if canonical["format"] != FORMAT or not re.fullmatch(
        r"[0-9a-f]{64}", str(canonical["normalization_sql_sha256"])
    ) or not re.fullmatch(r"[0-9a-f]{64}", str(canonical["catalog_facts_sha256"])):
        return False
    try:
        expected_plan_hash = hashlib.sha256(json.dumps(
            canonical, sort_keys=True, separators=(",", ":")
        ).encode()).hexdigest()
        actions = canonical["actions"]
        pending_actions = [item for item in actions if item.get("action") == "apply_pending"]
        sql_parts = [part.strip() for part in plan.pending_sql.decode("utf-8", "strict").split(";") if part.strip()]
        pending_ids = [statement_id(part) for part in sql_parts]
        for part in sql_parts:
            validate_normalization_sql(part.encode("utf-8"), hashlib.sha256(part.encode("utf-8")).hexdigest())
    except (AttributeError, TypeError, ValueError, UnicodeDecodeError, NormalizationValidationError):
        return False
    return (
        summary.get("plan_sha256") == expected_plan_hash
        and summary.get("execution_performed") is False
        and summary.get("contains_sql_text") is False
        and summary.get("pending_statement_count") == len(pending_actions) == len(pending_ids)
        and [item.get("statement_sha256") for item in pending_actions] == pending_ids
    )
