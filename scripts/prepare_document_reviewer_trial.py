#!/usr/bin/env python3
"""Assign/withdraw only the selected synthetic document review fixture.

Administrative preparation, not a product assignment-management endpoint.
Uses the same explicitly authorized local container Unix socket as setup;
never reads passwords, alters ACLs, creates accounts or modifies a scanner state.
The existing SQL guard/audit triggers record each assignment transition.
"""
import argparse
import json
import os
from pathlib import Path
import subprocess
import uuid

ROOT = Path(__file__).resolve().parents[1]
PRIVATE = ROOT / ".local/testing/space-gallery-demo"
CONTAINER = "espacigo-f5-gate-db-20261101"
DATABASE = "espacigo_f5_gate_20261101"
IMAGE = "sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677"


def identifier(value):
    if not isinstance(value, str) or str(uuid.UUID(value)) != value:
        raise ValueError("Noncanonical synthetic fixture identifier")
    return value


def private_json(path):
    info = path.lstat()
    if not path.is_file() or path.is_symlink() or info.st_mode & 0o777 != 0o600:
        raise ValueError("Fixture must be an existing private regular mode-0600 file")
    return json.loads(path.read_text())


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--authorized-local-synthetic-only", action="store_true", required=True)
    parser.add_argument("--purpose", choices=("identity", "space"), required=True)
    parser.add_argument("--operation", choices=("grant", "withdraw"), required=True)
    parser.add_argument("--fixture", choices=("reviewer-identity-fixture.json", "reviewer-space-fixture.json", "reviewer-outside-scope.json"))
    args = parser.parse_args()
    info = json.loads(subprocess.check_output(["docker", "inspect", CONTAINER], text=True))[0]
    if not info["State"]["Running"] or info["Image"] != IMAGE:
        raise SystemExit("Selected local target image/running identity mismatch")
    fixture = private_json(PRIVATE / (args.fixture or ("reviewer-" + args.purpose + "-fixture.json")))
    owner, reviewer = identifier(fixture["owner_id"]), identifier(fixture["reviewer_id"])
    case = identifier(fixture["case_id"])
    if owner == reviewer or fixture["purpose"] != args.purpose:
        raise SystemExit("Conflicting fixture actor/purpose")
    scope = owner if args.purpose == "identity" else identifier(fixture["space_id"])
    capability = "review_" + args.purpose
    kind = "identity_subject" if args.purpose == "identity" else "space"
    history_path = PRIVATE / "reviewer-assignment-history.json"
    history = private_json(history_path) if history_path.exists() else {"entries": []}
    variables = {"owner": owner, "reviewer": reviewer, "case": case, "scope": scope, "capability": capability, "kind": kind}
    sql = """BEGIN;
SELECT set_config('espacigo.fixture_owner', :'owner',true) AS owner_setting,
       set_config('espacigo.fixture_reviewer', :'reviewer',true) AS reviewer_setting,
       set_config('espacigo.fixture_case', :'case',true) AS case_setting,
       set_config('espacigo.fixture_scope', :'scope',true) AS scope_setting,
       set_config('espacigo.fixture_capability', :'capability',true) AS capability_setting,
       set_config('espacigo.fixture_kind', :'kind',true) AS kind_setting
\\gset
DO $preflight$ BEGIN
 IF current_database()<>'espacigo_f5_gate_20261101' OR current_user<>'postgres' OR current_setting('server_version_num')<>'180006'
    OR (SELECT count(*) FROM public.usuario)<>2
    OR (SELECT count(*) FROM public.schema_migrations)<>83
 THEN RAISE EXCEPTION 'Synthetic target preflight failed'; END IF;
 -- Readers lock the reviewer before the grant row. An UPDATE row trigger
 -- alone acquires that account lock too late to preserve this order.
 IF NOT document_verification_private.document_verification_lock_active_actor(current_setting('espacigo.fixture_reviewer')::uuid)
 THEN RAISE EXCEPTION 'Synthetic reviewer inactive'; END IF;
 IF NOT EXISTS(SELECT 1 FROM public.document_verification_case c JOIN public.usuario u ON u.id=c.subject_id
    WHERE c.id=current_setting('espacigo.fixture_case')::uuid
      AND c.subject_id=current_setting('espacigo.fixture_owner')::uuid AND u.estado='activo'
      AND c.superseded_at IS NULL
      AND ((c.purpose='identity' AND current_setting('espacigo.fixture_kind')='identity_subject')
        OR (c.purpose='space' AND current_setting('espacigo.fixture_kind')='space'
         AND c.space_id=current_setting('espacigo.fixture_scope')::uuid
         AND EXISTS(SELECT 1 FROM public.espacio e WHERE e.id=c.space_id AND e.propietario_id=c.subject_id))))
 THEN RAISE EXCEPTION 'Current selected synthetic case scope mismatch'; END IF;
END $preflight$;
"""
    if args.operation == "grant":
        sql += """DO $no_other$ BEGIN
 IF EXISTS(SELECT 1 FROM public.document_verification_reviewer_grant
    WHERE reviewer_id=current_setting('espacigo.fixture_reviewer')::uuid AND revoked_at IS NULL
      AND (expires_at IS NULL OR expires_at>clock_timestamp()))
 THEN RAISE EXCEPTION 'Preserve existing assignments; withdraw only owned fixture first'; END IF;
END $no_other$;
WITH written AS (
 INSERT INTO public.document_verification_reviewer_grant
 (reviewer_id,capability,scope_kind,scope_id,granted_by,granted_reason_code,expires_at)
 VALUES(:'reviewer'::uuid,:'capability',:'kind',:'scope'::uuid,:'owner'::uuid,'temporary_coverage',now()+interval '48 hours')
 RETURNING id,reviewer_id,capability,scope_kind,scope_id
)
SELECT json_build_object('grant_id',id,'reviewer_id',reviewer_id,'capability',capability,'scope_kind',scope_kind,'scope_id',scope_id,'operation','granted') FROM written;
"""
    else:
        entries = [e for e in history["entries"] if e["operation"] == "granted" and e["capability"] == capability and e["scope_id"] == scope]
        withdrawn = {e["grant_id"] for e in history["entries"] if e["operation"] == "revoked"}
        active = [e for e in entries if e["grant_id"] not in withdrawn]
        if len(active) != 1:
            raise SystemExit("Exactly one owned synthetic assignment must be selected")
        variables["grant"] = identifier(active[0]["grant_id"])
        sql += """WITH written AS (
 UPDATE public.document_verification_reviewer_grant
 SET revoked_at=clock_timestamp(),revoked_by=:'owner'::uuid,revocation_reason_code='temporary_coverage_ended'
 WHERE id=:'grant'::uuid AND reviewer_id=:'reviewer'::uuid AND capability=:'capability'
   AND scope_kind=:'kind' AND scope_id=:'scope'::uuid AND revoked_at IS NULL
 RETURNING id,reviewer_id,capability,scope_kind,scope_id
)
SELECT json_build_object('grant_id',id,'reviewer_id',reviewer_id,'capability',capability,'scope_kind',scope_kind,'scope_id',scope_id,'operation','revoked') FROM written;
"""
    sql += "COMMIT;\n"
    argv = ["docker", "exec", "-i", "-u", "postgres", CONTAINER, "psql", "-XAtq", "-v", "ON_ERROR_STOP=1", "-d", DATABASE]
    for key, value in variables.items():
        argv.extend(["-v", key + "=" + value])
    result = subprocess.run(argv, input=sql, capture_output=True, text=True)
    if result.returncode:
        # Private diagnostics only; never propagate SQL parameter values to logs.
        diagnostic = PRIVATE / "reviewer-assignment-error.private.log"
        diagnostic.write_text(result.stderr)
        diagnostic.chmod(0o600)
        raise SystemExit("Assignment preparation failed; private diagnostic retained")
    records = [json.loads(line) for line in result.stdout.splitlines() if line.startswith("{")]
    if len(records) != 1:
        raise SystemExit("Ambiguous assignment write: inspect exact target; do not replay")
    record = records[0]
    # Fresh target readback proves both current state and the automatic audit.
    query = """BEGIN READ ONLY;
SELECT json_build_object('state_ok', (revoked_at IS NULL)=(:'active'='true'),
 'audit_ok',EXISTS(SELECT 1 FROM public.document_verification_reviewer_grant_event e WHERE e.grant_id=g.id AND e.action=:'action'))
FROM public.document_verification_reviewer_grant g WHERE id=:'grant_id'::uuid AND reviewer_id=:'reviewer'::uuid;
ROLLBACK;"""
    check = argv + ["-v", "grant_id=" + identifier(record["grant_id"]), "-v", "active=" + str(args.operation == "grant").lower(), "-v", "action=" + record["operation"]]
    readback = subprocess.run(check, input=query, capture_output=True, text=True, check=True)
    observed = [json.loads(line) for line in readback.stdout.splitlines() if line.startswith("{")]
    if observed != [{"state_ok": True, "audit_ok": True}]:
        raise SystemExit("Assignment/audit postcommit readback failed; do not replay")
    history["entries"].append(record)
    history_path.write_text(json.dumps(history, indent=2) + "\n")
    history_path.chmod(0o600)
    print("SYNTHETIC_REVIEW_ASSIGNMENT_" + record["operation"].upper() + "_AND_AUDIT_READBACK_PASS")


if __name__ == "__main__":
    main()
