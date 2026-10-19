-- W8 first observation: narrowly allowlisted PostgreSQL role/catalog metadata.
-- This query is read-only and deliberately excludes configuration settings, statistics,
-- passwords, business data, configuration-variable views, and mutations.
BEGIN READ ONLY;
WITH RECURSIVE selected_roles AS (
    SELECT oid, rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole,
           rolreplication, rolbypassrls, rolinherit
    FROM pg_catalog.pg_roles
    WHERE rolname IN ('espacigo_db_observer', 'pg_read_all_settings',
                      'espacigo_gcp04_w8_catalog_probe')
), login_membership_closure(role_oid) AS (
    SELECT oid FROM pg_catalog.pg_roles WHERE rolname = current_user
    UNION
    SELECT edge.roleid
    FROM login_membership_closure closure
    JOIN pg_catalog.pg_auth_members edge ON edge.member = closure.role_oid
), selected_memberships AS (
    SELECT granted.rolname AS granted_role, member.rolname AS member_role,
           edge.admin_option, edge.inherit_option, edge.set_option
    FROM pg_catalog.pg_auth_members edge
    JOIN pg_catalog.pg_roles granted ON granted.oid = edge.roleid
    JOIN pg_catalog.pg_roles member ON member.oid = edge.member
    WHERE edge.member IN (SELECT role_oid FROM login_membership_closure)
       OR granted.rolname IN ('espacigo_db_observer', 'pg_read_all_settings',
                              'espacigo_gcp04_w8_catalog_probe')
       OR member.rolname IN ('espacigo_db_observer', 'pg_read_all_settings',
                             'espacigo_gcp04_w8_catalog_probe')
), relevant_acl AS (
    SELECT 'database'::text AS object_kind, current_database() AS object_name,
           CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END AS grantee,
           acl.privilege_type, acl.is_grantable
    FROM pg_catalog.pg_database db
    CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(db.datacl, pg_catalog.acldefault('d', db.datdba))) acl
    LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
    WHERE db.datname = current_database()
      AND (acl.grantee = 0 OR acl.grantee IN (SELECT role_oid FROM login_membership_closure)
           OR grantee.rolname IN ('espacigo_db_observer', 'pg_read_all_settings', 'espacigo_gcp04_w8_catalog_probe'))
    UNION ALL
    SELECT 'schema', ns.nspname,
           CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END,
           acl.privilege_type, acl.is_grantable
    FROM pg_catalog.pg_namespace ns
    CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(ns.nspacl, pg_catalog.acldefault('n', ns.nspowner))) acl
    LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
    WHERE ns.nspname = 'public'
      AND (acl.grantee = 0 OR acl.grantee IN (SELECT role_oid FROM login_membership_closure)
           OR grantee.rolname IN ('espacigo_db_observer', 'pg_read_all_settings', 'espacigo_gcp04_w8_catalog_probe'))
    UNION ALL
    SELECT 'ledger', ns.nspname || '.' || cls.relname,
           CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END,
           acl.privilege_type, acl.is_grantable
    FROM pg_catalog.pg_class cls
    JOIN pg_catalog.pg_namespace ns ON ns.oid = cls.relnamespace
    CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(cls.relacl, pg_catalog.acldefault('r', cls.relowner))) acl
    LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
    WHERE ns.nspname = 'public' AND cls.relname = 'schema_migrations'
      AND (acl.grantee = 0 OR acl.grantee IN (SELECT role_oid FROM login_membership_closure)
           OR grantee.rolname IN ('espacigo_db_observer', 'pg_read_all_settings', 'espacigo_gcp04_w8_catalog_probe'))
)
SELECT pg_catalog.jsonb_build_object(
    'format', 'espacigo.gcp04.w8.catalog-probe.v1',
    'target', pg_catalog.jsonb_build_object(
        'database', current_database(), 'login', current_user,
        'server_version_num', current_setting('server_version_num')::integer,
        'transaction_read_only', current_setting('transaction_read_only')::boolean),
    'roles', COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.jsonb_build_object(
        'name', rolname, 'login', rolcanlogin, 'superuser', rolsuper,
        'createdb', rolcreatedb, 'createrole', rolcreaterole,
        'replication', rolreplication, 'bypass_rls', rolbypassrls, 'inherit', rolinherit)
        ORDER BY rolname) FROM selected_roles), '[]'::jsonb),
    'memberships', COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.to_jsonb(selected_memberships)
        ORDER BY granted_role, member_role) FROM selected_memberships), '[]'::jsonb),
    'acl', COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.to_jsonb(relevant_acl)
        ORDER BY object_kind, object_name, grantee, privilege_type) FROM relevant_acl), '[]'::jsonb),
    'scope', pg_catalog.jsonb_build_object('business_rows_read', false,
        'configuration_catalog_read', false, 'statistics_read', false,
        'password_or_secret_read', false, 'mutations', false)
)::text;
COMMIT;
