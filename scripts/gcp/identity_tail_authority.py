"""Exact read-only authority report for identity-tail migrations V95/V96."""

IDENTITY_TAIL_DDL_MIGRATIONS = frozenset((
    'V000095__identity_document_case_declaration_binding.sql',
    'V000096__identity_reviewer_observation_audit.sql',
))
AUTHORITY_FIELDS = {
    'session_user', 'current_user', 'current_schema', 'session_role', 'public_create',
    'schema_owner_exists', 'schema_owner_role', 'runtime_exists', 'runtime_role',
    'privacy_cleanup_worker_exists', 'privacy_cleanup_worker_role',
    'case_owner', 'declaration_owner', 'event_owner', 'purge_owner',
    'purge_security_definer', 'purge_search_path', 'purge_non_owner_acl_count',
    'purge_worker_execute', 'purge_worker_grantor_owner', 'purge_worker_grantable',
    'runtime_purge_execute', 'runtime_case_select', 'runtime_case_expiry_update',
    'runtime_event_insert',
}
ROLE_FIELDS = {
    'can_login', 'inherit', 'superuser', 'create_role', 'create_db',
    'replication', 'bypass_rls',
}


def _require_role(authority, exists_field, role_field, required):
    if type(authority[exists_field]) is not bool or authority[exists_field] is not True:
        raise ValueError('identity-tail preflight role is missing: ' + role_field)
    role = authority[role_field]
    if not isinstance(role, dict) or set(role) != ROLE_FIELDS or any(type(value) is not bool for value in role.values()):
        raise ValueError('identity-tail preflight role attributes are incomplete: ' + role_field)
    if role != required:
        raise ValueError('identity-tail preflight role has unsafe attributes: ' + role_field)


def validate_authority(authority, pending_names):
    """Validate the exact authority shape and guards for pending V95/V96 SQL."""
    if not isinstance(authority, dict) or set(authority) != AUTHORITY_FIELDS:
        raise ValueError('identity-tail preflight lacks the exact DDL authority report')
    if not isinstance(pending_names, (list, tuple, set, frozenset)) or not set(pending_names) <= IDENTITY_TAIL_DDL_MIGRATIONS:
        raise ValueError('identity-tail authority requires exact V95/V96 migration names')
    if not pending_names:
        raise ValueError('identity-tail authority has no pending migration')
    for field in ('session_user', 'current_user', 'current_schema'):
        if not isinstance(authority[field], str):
            raise ValueError('identity-tail preflight identity is incomplete')
    if (authority['session_user'], authority['current_user'], authority['current_schema']) != (
        'espacigo_migrator_login', 'espacigo_schema_owner', 'public'
    ):
        raise ValueError('identity-tail preflight database identity or target schema differs')
    for field in ('public_create', 'purge_security_definer', 'purge_worker_execute',
                  'purge_worker_grantor_owner', 'purge_worker_grantable', 'runtime_purge_execute',
                  'runtime_case_select', 'runtime_case_expiry_update', 'runtime_event_insert'):
        if type(authority[field]) is not bool:
            raise ValueError('identity-tail preflight privilege evidence is not boolean: ' + field)
    for field in ('case_owner', 'declaration_owner', 'event_owner', 'purge_owner'):
        if not isinstance(authority[field], str):
            raise ValueError('identity-tail preflight owner evidence is incomplete: ' + field)
    if not isinstance(authority['purge_search_path'], list) or any(not isinstance(value, str) for value in authority['purge_search_path']):
        raise ValueError('identity-tail preflight function search path is incomplete')
    if type(authority['purge_non_owner_acl_count']) is not int:
        raise ValueError('identity-tail preflight function ACL count is invalid')
    if authority['public_create'] is not True:
        raise ValueError('identity-tail preflight lacks public CREATE')
    for field in ('schema_owner_exists', 'runtime_exists', 'privacy_cleanup_worker_exists'):
        if type(authority[field]) is not bool:
            raise ValueError('identity-tail preflight role existence evidence is not boolean: ' + field)
    for field in ('session_role', 'schema_owner_role', 'runtime_role', 'privacy_cleanup_worker_role'):
        role = authority[field]
        if not isinstance(role, dict) or set(role) != ROLE_FIELDS or any(type(value) is not bool for value in role.values()):
            raise ValueError('identity-tail preflight role attributes are incomplete: ' + field)
    safe_role = {
        'can_login': False, 'inherit': False, 'superuser': False,
        'create_role': False, 'create_db': False, 'replication': False,
        'bypass_rls': False,
    }
    login_role = dict(safe_role, can_login=True)
    if authority['session_role'] != login_role:
        raise ValueError('identity-tail preflight migrator login role has unsafe attributes')
    _require_role(authority, 'schema_owner_exists', 'schema_owner_role', safe_role)
    _require_role(authority, 'runtime_exists', 'runtime_role', safe_role)
    if 'V000095__identity_document_case_declaration_binding.sql' in pending_names:
        _require_role(authority, 'privacy_cleanup_worker_exists', 'privacy_cleanup_worker_role', safe_role)
        for field in ('case_owner', 'declaration_owner', 'purge_owner'):
            if authority[field] != 'espacigo_schema_owner':
                raise ValueError('V95 preflight requires schema-owned identity tables and purge function')
        if authority['purge_security_definer'] is not True or authority['purge_search_path'] != ['search_path=pg_catalog']:
            raise ValueError('V95 purge function definer/search-path contract differs')
        if authority['purge_non_owner_acl_count'] != 1 or authority['purge_worker_execute'] is not True or authority['purge_worker_grantor_owner'] is not True or authority['purge_worker_grantable'] is not False or authority['runtime_purge_execute'] is not False:
            raise ValueError('V95 purge function ACL is not worker-only')
    if 'V000096__identity_reviewer_observation_audit.sql' in pending_names:
        for field in ('case_owner', 'event_owner'):
            if authority[field] != 'espacigo_schema_owner':
                raise ValueError('V96 preflight requires schema-owned verification tables')
        if authority['runtime_case_select'] is not True or authority['runtime_case_expiry_update'] is not True or authority['runtime_event_insert'] is not True:
            raise ValueError('V96 preflight lacks reviewed runtime table/column privileges')
    return authority
