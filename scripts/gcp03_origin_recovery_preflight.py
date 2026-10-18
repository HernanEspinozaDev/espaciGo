"""Read-only catalog adapter and plan-only local recovery entrypoint.

This module has no archive restore support and never calls pg_restore. It reads
the already-restored database through psql and emits a private, hash-bound plan.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat
import subprocess

from gcp03_origin_recovery_plan import (RecoveryPlanError, build_normalization_recovery_plan,
    execute_reviewed_recovery, statement_id)

DIAGNOSTIC_SHA256 = "3325288140b1f2de8bb944f7c4c1599fcf56f1a19531dc334bf171629f99391a"
DIAGNOSTIC_TABLE_SHA256 = "5d78e5d55d9021a935b7210977395c3dae2540c983e4266107b2583619aaabe1"
DIAGNOSTIC_MIGRATION_SHA256 = "5dd344d805ecdf9c500eabac8e2013b0186a38066d3d17599d59ecef62c54cdd"
NORMALIZATION_SHA256 = "865697a41f9ca70881522f7558093e8ac4107dfa5a4e58dbf749d5f670eff842"
EXPECTED_CURRENT = {"database": "espacigo_scratch", "login": "postgres", "server_version_major": 18,
    "table_inventory_count": 96, "table_inventory_sha256": DIAGNOSTIC_TABLE_SHA256,
    "migration_history_count": 61, "migration_history_sha256": DIAGNOSTIC_MIGRATION_SHA256,
    "actor_login": True, "actor_superuser": False, "actor_createrole": True,
    "runtime_role_login": False, "runtime_membership_row_count": 1,
    "runtime_membership_set": True, "runtime_membership_inherit": False,
    "runtime_membership_admin": False, "runtime_membership_effective_set": True,
    "postgres_public_create": True, "runtime_public_create": False,
    "observer_column_select_count": 5, "observer_column_write_count": 0,
    "owned_sequence_dependency_count": 11,
    "relation_type_counts": {"index": 267, "other": 2, "sequence": 11, "table": 96}}
OWNER = re.compile(r"^ALTER\s+(TABLE|SEQUENCE|TYPE|DOMAIN|FUNCTION|PROCEDURE|AGGREGATE|TEXT\s+SEARCH\s+DICTIONARY|TEXT\s+SEARCH\s+CONFIGURATION)\s+(.+)\s+OWNER\s+TO\s+espacigo_runtime$", re.I | re.S)
ACL = re.compile(r"^(GRANT|REVOKE)\s+SELECT\s*\(([^)]+)\)\s+ON\s+TABLE\s+(.+)\s+(TO|FROM)\s+(espacigo_gcp03_observer|espacigo_runtime)$", re.I | re.S)
SCHEMA_ACL = re.compile(r"^(GRANT|REVOKE)\s+CREATE\s+ON\s+SCHEMA\s+(.+)\s+(TO|FROM)\s+(espacigo_runtime)$", re.I | re.S)


class CatalogReader:
    def __init__(self, *, host: str, port: int, user: str, database: str, psql: str = "psql", passfile: Path | None = None):
        if database != "espacigo_scratch" or user != "postgres":
            raise RecoveryPlanError("recovery target identity is not the reviewed restored copy")
        if passfile is not None and (passfile.is_symlink() or not passfile.is_file() or stat.S_IMODE(passfile.stat().st_mode) != 0o600):
            raise RecoveryPlanError("database credential file must be regular mode 0600")
        self.host, self.port, self.user, self.database, self.psql = host, port, user, database, psql
        self.passfile = passfile

    def query(self, sql: str, **variables: str) -> str:
        args = [self.psql, "-X", "-qAt", "--set=ON_ERROR_STOP=1", "--host=" + self.host,
                "--port=" + str(self.port), "--username=" + self.user, "--dbname=" + self.database]
        for key, value in variables.items():
            args.extend(["--set", f"{key}={value}"])
        env = os.environ.copy()
        if self.passfile is not None:
            env["PGPASSFILE"] = str(self.passfile)
        proc = subprocess.run(args + ["-c", "BEGIN TRANSACTION READ ONLY;\n" + sql + ";\nCOMMIT;"], env=env,
                              stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, check=False, text=True)
        if proc.returncode:
            raise RecoveryPlanError("read-only catalog preflight failed")
        return proc.stdout.strip()

    def identity_and_inventory(self) -> dict:
        raw = self.query("""SELECT json_build_object(
          'database',current_database(),'login',current_user,
          'server_version_major',current_setting('server_version_num')::int/10000,
          'actor_login',(SELECT rolcanlogin FROM pg_roles WHERE rolname=current_user),
          'actor_superuser',(SELECT rolsuper FROM pg_roles WHERE rolname=current_user),
          'actor_createrole',(SELECT rolcreaterole FROM pg_roles WHERE rolname=current_user),
          'runtime_role_login',(SELECT rolcanlogin FROM pg_roles WHERE rolname='espacigo_runtime'),
          'runtime_membership_row_count',(SELECT count(*) FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles g ON g.oid=m.member WHERE r.rolname='espacigo_runtime' AND g.rolname=current_user),
          'runtime_membership_set',(SELECT bool_or(m.set_option AND NOT m.inherit_option AND NOT m.admin_option) FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles g ON g.oid=m.member WHERE r.rolname='espacigo_runtime' AND g.rolname=current_user),
          'runtime_membership_inherit',(SELECT bool_or(m.inherit_option) FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles g ON g.oid=m.member WHERE r.rolname='espacigo_runtime' AND g.rolname=current_user),
          'runtime_membership_admin',(SELECT bool_or(m.admin_option) FROM pg_auth_members m JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles g ON g.oid=m.member WHERE r.rolname='espacigo_runtime' AND g.rolname=current_user),
          'runtime_membership_effective_set',pg_has_role(current_user,'espacigo_runtime','SET'),
          'postgres_public_create',has_schema_privilege('postgres','public','CREATE'),
          'runtime_public_create',has_schema_privilege('espacigo_runtime','public','CREATE'),
          'owned_sequence_dependency_count',(SELECT count(*) FROM pg_depend d JOIN pg_class s ON s.oid=d.objid JOIN pg_class p ON p.oid=d.refobjid JOIN pg_namespace n ON n.oid=s.relnamespace WHERE d.classid='pg_class'::regclass AND d.refclassid='pg_class'::regclass AND s.relkind='S' AND n.nspname='public' AND d.deptype IN ('a','i')),
          'observer_column_select_count',(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped WHERE n.nspname='public' AND c.relkind IN ('r','p') AND has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'SELECT')),
          'observer_column_write_count',(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped WHERE n.nspname='public' AND c.relkind IN ('r','p') AND (has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'INSERT') OR has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'UPDATE') OR has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'REFERENCES'))),
          'table_names',(SELECT COALESCE(json_agg(c.relname ORDER BY c.relname),'[]'::json)
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
            WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f')
              AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass
                AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),
          'relation_type_counts',(SELECT COALESCE(json_object_agg(kind,count),'{}'::json) FROM (
            SELECT CASE WHEN c.relkind IN ('r','p') THEN 'table' WHEN c.relkind IN ('i','I') THEN 'index' WHEN c.relkind='S' THEN 'sequence' ELSE 'other' END kind,count(*)
            FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public'
              AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)
            GROUP BY 1) counts),
          'migration_rows',(SELECT COALESCE(json_agg(json_build_array(version::text,checksum::text) ORDER BY version),'[]'::json)
            FROM public.schema_migrations))::text""")
        value = json.loads(raw)
        table_bytes = "".join(item + "\n" for item in value.pop("table_names"))
        migration_rows = value.pop("migration_rows")
        value["relation_type_counts"] = {key: int(count) for key, count in value["relation_type_counts"].items()}
        migration_bytes = "".join(f"{row[0]}\0{row[1]}\n" for row in migration_rows)
        value["table_inventory_count"] = len(table_bytes.splitlines())
        value["table_inventory_sha256"] = hashlib.sha256(table_bytes.encode()).hexdigest()
        value["migration_history_count"] = len(migration_rows)
        value["migration_history_sha256"] = hashlib.sha256(migration_bytes.encode()).hexdigest()
        return value

    @staticmethod
    def _sql_literal(value: str) -> str:
        return "'" + value.replace("'", "''") + "'"

    def owner_fact(self, kind: str, object_name: str) -> tuple[str, str | None]:
        literal = self._sql_literal(object_name)
        parent = None
        if kind in {"TABLE", "SEQUENCE"}:
            query = f"""SELECT pg_get_userbyid(c.relowner),
              CASE WHEN c.relkind='S' THEN (SELECT format('%I.%I',pn.nspname,pc.relname)
                FROM pg_depend d JOIN pg_class pc ON pc.oid=d.refobjid
                JOIN pg_namespace pn ON pn.oid=pc.relnamespace
                WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.refclassid='pg_class'::regclass
                  AND d.deptype IN ('a','i') LIMIT 1) END
              FROM pg_class c WHERE c.oid=to_regclass({literal})"""
            raw = self.query(query)
            parts = raw.split("|") if raw else []
            if not parts or parts[0] in {"", "NULL"}:
                raise RecoveryPlanError("allowlisted relation is missing from the restored catalog")
            return parts[0], (parts[1] if len(parts) > 1 and parts[1] not in {"", "NULL"} else None)
        if kind in {"FUNCTION", "PROCEDURE", "AGGREGATE"}:
            qualified, signature = object_name.split("(", 1)
            signature = signature[:-1]
            # The fixed normalization statements carry argument names. Resolve
            # by the catalog's canonical identity signature, not by evaluating SQL.
            query = f"""SELECT pg_get_userbyid(p.proowner) FROM pg_proc p
              JOIN pg_namespace n ON n.oid=p.pronamespace
              WHERE format('%I.%I',n.nspname,p.proname)={self._sql_literal(qualified)}
                AND lower(pg_get_function_identity_arguments(p.oid))=lower({self._sql_literal(signature)})"""
            raw = self.query(query)
        elif kind in {"TEXT SEARCH DICTIONARY", "TEXT SEARCH CONFIGURATION"}:
            schema, name = object_name.split(".", 1)
            catalog = "pg_ts_dict" if kind.endswith("DICTIONARY") else "pg_ts_config"
            ownercol = "dictowner" if catalog == "pg_ts_dict" else "cfgowner"
            namecol = "dictname" if catalog == "pg_ts_dict" else "cfgname"
            namespace_col = "dictnamespace" if catalog == "pg_ts_dict" else "cfgnamespace"
            raw = self.query(f"SELECT pg_get_userbyid(x.{ownercol}) FROM {catalog} x JOIN pg_namespace n ON n.oid=x.{namespace_col} WHERE n.nspname={self._sql_literal(schema)} AND x.{namecol}={self._sql_literal(name)}")
        else:
            raw = self.query(f"SELECT pg_get_userbyid((SELECT typowner FROM pg_type WHERE oid=to_regtype({literal})))")
        if not raw or raw == "NULL":
            raise RecoveryPlanError("allowlisted object is missing or ambiguous in the restored catalog")
        return raw, parent

    def statement_facts(self, sql: bytes) -> tuple[dict[str, str], dict[str, str]]:
        statements = [part.strip() for part in sql.decode("utf-8", "strict").split(";") if part.strip()]
        facts: dict[str, str] = {}
        tables: dict[str, str] = {}
        sequences: dict[str, tuple[str, str | None]] = {}
        for statement in statements:
            match = OWNER.fullmatch(statement)
            if match and match.group(1).upper() == "TABLE":
                tables[statement_id(statement)] = match.group(2)
        for statement in statements:
            sid = statement_id(statement)
            match = OWNER.fullmatch(statement)
            if match:
                kind, name = match.group(1).upper(), match.group(2)
                if kind == "TEXT SEARCH DICTIONARY" or kind == "TEXT SEARCH CONFIGURATION":
                    owner, parent = self.owner_fact(kind, name)
                else:
                    owner, parent = self.owner_fact(kind, name)
                if owner not in {"postgres", "espacigo_runtime"}:
                    raise RecoveryPlanError("catalog contains an unexpected object owner")
                facts[sid] = owner
                if kind == "SEQUENCE":
                    sequences[sid] = (owner, parent)
                continue
            schema_acl = SCHEMA_ACL.fullmatch(statement)
            if schema_acl:
                action, schema, _, role = schema_acl.groups()
                present = self.query(f"SELECT has_schema_privilege({self._sql_literal(role)},{self._sql_literal(schema)},'CREATE')") == "t"
                facts[sid] = "satisfied" if present == (action.upper() == "GRANT") else "missing"
                continue
            acl = ACL.fullmatch(statement)
            if acl:
                action, columns, table, _, role = acl.groups()
                column_list = [item.strip() for item in columns.split(",")]
                present = all(self.query(f"SELECT has_column_privilege({self._sql_literal(role)},{self._sql_literal(table)},{self._sql_literal(col)},'SELECT')") == "t" for col in column_list)
                facts[sid] = "satisfied" if present == (action.upper() == "GRANT") else "missing"
                continue
            raise RecoveryPlanError("normalization statement is outside catalog preflight coverage")
        parent_map: dict[str, str] = {}
        for seq_id, (_, parent) in sequences.items():
            if parent is None:
                continue
            matches = [table_id for table_id, table in tables.items()
                       if self.query(f"SELECT to_regclass({self._sql_literal(table)})=to_regclass({self._sql_literal(parent)} )") == "t"]
            if len(matches) != 1:
                raise RecoveryPlanError("owned sequence parent does not match one reviewed table statement")
            parent_fact = facts[matches[0]]
            if parent_fact not in {"postgres", "espacigo_runtime"}:
                raise RecoveryPlanError("owned sequence parent owner is unknown")
            parent_map[seq_id] = matches[0]
        actual = json.loads(self.query("""SELECT json_build_object(
          'tables',(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f') AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),
          'sequences',(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind='S' AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),
          'routines',(SELECT count(*) FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public' AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),
          'dictionaries',(SELECT count(*) FROM pg_ts_dict d JOIN pg_namespace n ON n.oid=d.dictnamespace WHERE n.nspname='public'),
          'configurations',(SELECT count(*) FROM pg_ts_config c JOIN pg_namespace n ON n.oid=c.cfgnamespace WHERE n.nspname='public'),
          'types',(SELECT count(*) FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE n.nspname='public' AND t.typtype IN ('d','e','r','m') AND NOT EXISTS(SELECT 1 FROM pg_depend d WHERE d.classid='pg_type'::regclass AND d.objid=t.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)))::text"""))
        expected = {
            "tables": sum(bool(re.match(r"^ALTER\s+TABLE\b", item, re.I)) for item in statements),
            "sequences": sum(bool(re.match(r"^ALTER\s+SEQUENCE\b", item, re.I)) for item in statements),
            "routines": sum(bool(re.match(r"^ALTER\s+(?:FUNCTION|PROCEDURE|AGGREGATE)\b", item, re.I)) for item in statements),
            "dictionaries": sum(bool(re.match(r"^ALTER\s+TEXT\s+SEARCH\s+DICTIONARY\b", item, re.I)) for item in statements),
            "configurations": sum(bool(re.match(r"^ALTER\s+TEXT\s+SEARCH\s+CONFIGURATION\b", item, re.I)) for item in statements),
            "types": sum(bool(re.match(r"^ALTER\s+(?:TYPE|DOMAIN)\b", item, re.I)) for item in statements),
        }
        # Production callers keep the reviewed fixed baseline. The optional
        # override exists only so the isolated synthetic PostgreSQL fixture can
        # exercise the same catalog inspection and plan/apply path.
        expected = getattr(self, "expected_statement_object_counts", expected)
        if actual != expected:
            raise RecoveryPlanError("catalog object inventory has missing or unexpected objects")
        return facts, parent_map


def validate_historical_diagnostic(data: bytes, expected_sha256: str) -> dict:
    if hashlib.sha256(data).hexdigest() != expected_sha256 or expected_sha256 != DIAGNOSTIC_SHA256:
        raise RecoveryPlanError("historical diagnostic hash mismatch")
    report = json.loads(data)
    required = report.get("application_report", {})
    if report.get("format") != "espacigo.g03o.owner-acl-diagnostic-evidence.v1" or \
       required.get("format") != "espacigo.g03o.owner-acl-check.v1" or \
       required.get("report_complete") is not True or required.get("read_only") is not True or \
       required.get("transaction_outcome") != "committed" or required.get("database") != "espacigo_scratch" or \
       required.get("server_version_major") != 18 or required.get("table_inventory_count") != 96 or \
       required.get("table_inventory_sha256") != DIAGNOSTIC_TABLE_SHA256 or \
       required.get("migration_history_count") != 61 or required.get("migration_history_sha256") != DIAGNOSTIC_MIGRATION_SHA256 or \
       required.get("owned_sequence_parent_owner_counts", {}).get("dependencies") != EXPECTED_CURRENT["owned_sequence_dependency_count"] or \
       required.get("expected_migration_61_present") is not True or required.get("runtime_membership_set") is not True or \
       required.get("runtime_membership_inherit") is not False or required.get("runtime_membership_admin") is not False:
        raise RecoveryPlanError("historical diagnostic does not match the reviewed restored-copy baseline")
    return required


def build_bound_plan(reader: CatalogReader, normalization: bytes, diagnostic_bytes: bytes, output_path: Path) -> dict:
    diag = validate_historical_diagnostic(diagnostic_bytes, DIAGNOSTIC_SHA256)
    current = reader.identity_and_inventory()
    if current != EXPECTED_CURRENT:
        raise RecoveryPlanError("current restored-copy identity, migration, or table inventory drifted")
    if hashlib.sha256(normalization).hexdigest() != NORMALIZATION_SHA256:
        raise RecoveryPlanError("reviewed normalization hash mismatch")
    facts, sequence_parents = reader.statement_facts(normalization)
    binding = {"historical_diagnostic_sha256": DIAGNOSTIC_SHA256,
               "table_inventory_sha256": current["table_inventory_sha256"],
               "migration_history_sha256": current["migration_history_sha256"],
               "database": current["database"], "login": current["login"],
               "server_version_major": current["server_version_major"],
               "owned_sequence_dependency_count": current["owned_sequence_dependency_count"],
               "historical_owner_sequence_count": diag["owned_sequence_parent_owner_counts"]["dependencies"]}
    plan = build_normalization_recovery_plan(normalization, NORMALIZATION_SHA256, facts,
        owned_sequence_parent_statement=sequence_parents, binding=binding)
    output = {"format": "espacigo.g03o.recovery-preflight.v1", **plan.summary}
    output_path.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
    fd = os.open(output_path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "w", encoding="utf-8") as stream:
        json.dump(output, stream, sort_keys=True, indent=2)
        stream.write("\n")
    return output


def execute_with_current_preflight(*, reader: CatalogReader, normalization: bytes,
        diagnostic_bytes: bytes, approved_plan_sha256: str, database, private_files,
        private_file_store, run_id: str) -> dict:
    """Rebuild the plan from current catalog state immediately before apply."""
    try:
        diag = validate_historical_diagnostic(diagnostic_bytes, DIAGNOSTIC_SHA256)
        current = reader.identity_and_inventory()
        if current != EXPECTED_CURRENT or hashlib.sha256(normalization).hexdigest() != NORMALIZATION_SHA256:
            return {"format": "espacigo.g03o.recovery-execution.v1", "status": "stopped",
                "phase_success": False, "report_complete": False, "stage": "current_preflight",
                "error_code": "CURRENT_STATE_OR_ARTIFACT_DRIFT", "transaction_outcome": "not_started", "retry": False}
        facts, sequence_parents = reader.statement_facts(normalization)
        binding = {"historical_diagnostic_sha256": DIAGNOSTIC_SHA256,
            "table_inventory_sha256": current["table_inventory_sha256"],
            "migration_history_sha256": current["migration_history_sha256"],
            "database": current["database"], "login": current["login"],
            "server_version_major": current["server_version_major"],
            "owned_sequence_dependency_count": current["owned_sequence_dependency_count"],
            "historical_owner_sequence_count": diag["owned_sequence_parent_owner_counts"]["dependencies"]}
        plan = build_normalization_recovery_plan(normalization, NORMALIZATION_SHA256, facts,
            owned_sequence_parent_statement=sequence_parents, binding=binding)
    except Exception:
        return {"format": "espacigo.g03o.recovery-execution.v1", "status": "stopped",
            "phase_success": False, "report_complete": False, "stage": "current_preflight",
            "error_code": "CURRENT_CATALOG_PREFLIGHT_FAILED", "transaction_outcome": "not_started", "retry": False}
    if plan.summary["plan_sha256"] != approved_plan_sha256:
        return {"format": "espacigo.g03o.recovery-execution.v1", "status": "stopped",
            "phase_success": False, "report_complete": False, "stage": "plan_hash_check",
            "error_code": "CURRENT_PLAN_DIFFERS_FROM_APPROVED_PLAN", "transaction_outcome": "not_started", "retry": False}
    return execute_reviewed_recovery(plan=plan, approved_plan_sha256=approved_plan_sha256,
        database=database, private_files=private_files, private_file_store=private_file_store, run_id=run_id)


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--socket", required=True, help="Unix socket directory for an already-restored local copy")
    parser.add_argument("--normalization", type=Path, required=True)
    parser.add_argument("--diagnostic", type=Path, required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    for path in (args.normalization, args.diagnostic):
        if path.is_symlink() or not path.is_file() or stat.S_IMODE(path.stat().st_mode) != 0o600:
            raise SystemExit("preflight inputs must be regular mode 0600 files")
    reader = CatalogReader(host=args.socket, port=5432, user="postgres", database="espacigo_scratch")
    report = build_bound_plan(reader, args.normalization.read_bytes(), args.diagnostic.read_bytes(), args.output)
    print(json.dumps({"status": "plan_ready", "plan_sha256": report["plan_sha256"],
                      "pending_statement_count": report["pending_statement_count"],
                      "execution_performed": False, "report_complete": True}, sort_keys=True))


if __name__ == "__main__":
    main()
