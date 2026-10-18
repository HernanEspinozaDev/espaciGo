"""Fixed grammar for owner/ACL SQL in a reviewed GCP-03 normalization file."""
from __future__ import annotations
import hashlib
import re

class NormalizationValidationError(ValueError):
    pass

def validate_normalization_sql(sql: bytes, expected_sha256: str) -> None:
    if hashlib.sha256(sql).hexdigest() != expected_sha256 or not sql or b"\x00" in sql:
        raise NormalizationValidationError("NORMALIZATION_SQL_HASH_OR_FORMAT_INVALID")
    try:
        decoded = sql.decode("utf-8", "strict")
    except UnicodeDecodeError as exc:
        raise NormalizationValidationError("NORMALIZATION_SQL_ENCODING_INVALID") from exc
    if "\\" in decoded or "--" in decoded or "/*" in decoded or "*/" in decoded:
        raise NormalizationValidationError("NORMALIZATION_SQL_CONTAINS_FORBIDDEN_OPERATION")
    ident_part = r'(?:[A-Za-z_][A-Za-z0-9_$]*|"(?:[^"\\]|"")+")'
    ident = rf"{ident_part}(?:\.{ident_part})*"
    # Function identity arguments can contain PostgreSQL built-in multiword
    # types, typmods, arrays, and quoted user-defined type identifiers. Keep
    # the signature to a character allowlist; semicolons, literals, comments,
    # and backslashes remain rejected before this grammar is considered.
    type_list = r'[A-Za-z_0-9$., ()\[\]"]*'
    function_ident = rf"{ident}\s*\(\s*{type_list}\s*\)"
    role = r"(?:espacigo_runtime|espacigo_gadm_bootstrap|espacigo_gcp03_observer|PUBLIC)"
    column_select = rf"SELECT\s*\(\s*{ident_part}(?:\s*,\s*{ident_part})*\s*\)"
    privilege = rf"(?:ALL(?:\s+PRIVILEGES)?|SELECT|INSERT|UPDATE|DELETE|TRUNCATE|REFERENCES|TRIGGER|USAGE|CREATE|EXECUTE|{column_select})"
    privileges = rf"{privilege}(?:\s*,\s*{privilege})*"
    grant_object = rf"(?:ALL\s+(?:TABLES|SEQUENCES|FUNCTIONS)\s+IN\s+SCHEMA\s+{ident}|(?:TABLE|SEQUENCE|SCHEMA|TYPE|DOMAIN)\s+{ident}|(?:FUNCTION|PROCEDURE|AGGREGATE)\s+{function_ident})"
    object_owner = re.compile(rf"ALTER\s+(?:TABLE|SEQUENCE|TYPE|DOMAIN|SCHEMA|TEXT\s+SEARCH\s+DICTIONARY|TEXT\s+SEARCH\s+CONFIGURATION)\s+{ident}\s+OWNER\s+TO\s+{role}|ALTER\s+(?:FUNCTION|PROCEDURE|AGGREGATE)\s+{function_ident}\s+OWNER\s+TO\s+{role}", re.IGNORECASE)
    grants = re.compile(rf"(?:GRANT\s+{privileges}\s+ON\s+{grant_object}\s+TO\s+{role}|REVOKE\s+{privileges}\s+ON\s+{grant_object}\s+FROM\s+{role}|ALTER\s+DEFAULT\s+PRIVILEGES\s+FOR\s+ROLE\s+{ident}\s+IN\s+SCHEMA\s+{ident}\s+(?:GRANT\s+{privileges}\s+ON\s+(?:TABLES|SEQUENCES|FUNCTIONS)\s+TO\s+{role}|REVOKE\s+{privileges}\s+ON\s+(?:TABLES|SEQUENCES|FUNCTIONS)\s+FROM\s+{role}))", re.IGNORECASE)
    statements = [part.strip() for part in decoded.split(";") if part.strip()]
    if not statements or any(not (object_owner.fullmatch(stmt) or grants.fullmatch(stmt)) for stmt in statements):
        raise NormalizationValidationError("NORMALIZATION_SQL_OUTSIDE_ALLOWLIST")
