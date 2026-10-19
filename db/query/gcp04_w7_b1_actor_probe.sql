-- GCP04-W7 B1: read-only observation of the exact temporary bootstrap actor.
-- Do not run until a separate release exists. Never reads passwords or app rows.
BEGIN READ ONLY;
WITH actor AS (
    SELECT oid, rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole,
           rolreplication, rolbypassrls, rolinherit
    FROM pg_catalog.pg_roles
    WHERE rolname = 'espacigo_gcp04_w7_bootstrap'
), actor_memberships AS (
    SELECT granted.rolname AS granted_role,
           member.rolname AS member_role,
           edge.admin_option, edge.inherit_option, edge.set_option
    FROM pg_catalog.pg_auth_members edge
    JOIN pg_catalog.pg_roles granted ON granted.oid = edge.roleid
    JOIN pg_catalog.pg_roles member ON member.oid = edge.member
    WHERE edge.member = (SELECT oid FROM actor)
), relevant_acl AS (
    SELECT 'database'::text AS object_kind, db.datname AS object_name,
           CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END AS grantee,
           acl.privilege_type, acl.is_grantable,
           CASE WHEN acl.grantor = 0 THEN 'PUBLIC' ELSE grantor.rolname END AS grantor
    FROM pg_catalog.pg_database db
    CROSS JOIN LATERAL pg_catalog.aclexplode(
        COALESCE(db.datacl, pg_catalog.acldefault('d', db.datdba))) acl
    LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
    LEFT JOIN pg_catalog.pg_roles grantor ON grantor.oid = acl.grantor
    WHERE db.datname = current_database()
      AND (acl.grantee = 0 OR acl.grantee = (SELECT oid FROM actor))
    UNION ALL
    SELECT 'schema', ns.nspname,
           CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END,
           acl.privilege_type, acl.is_grantable,
           CASE WHEN acl.grantor = 0 THEN 'PUBLIC' ELSE grantor.rolname END
    FROM pg_catalog.pg_namespace ns
    CROSS JOIN LATERAL pg_catalog.aclexplode(
        COALESCE(ns.nspacl, pg_catalog.acldefault('n', ns.nspowner))) acl
    LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
    LEFT JOIN pg_catalog.pg_roles grantor ON grantor.oid = acl.grantor
    WHERE ns.nspname = 'public'
      AND (acl.grantee = 0 OR acl.grantee = (SELECT oid FROM actor))
    UNION ALL
    SELECT 'ledger', ns.nspname || '.' || cls.relname,
           CASE WHEN acl.grantee = 0 THEN 'PUBLIC' ELSE grantee.rolname END,
           acl.privilege_type, acl.is_grantable,
           CASE WHEN acl.grantor = 0 THEN 'PUBLIC' ELSE grantor.rolname END
    FROM pg_catalog.pg_class cls
    JOIN pg_catalog.pg_namespace ns ON ns.oid = cls.relnamespace
    CROSS JOIN LATERAL pg_catalog.aclexplode(
        COALESCE(cls.relacl, pg_catalog.acldefault('r', cls.relowner))) acl
    LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid = acl.grantee
    LEFT JOIN pg_catalog.pg_roles grantor ON grantor.oid = acl.grantor
    WHERE ns.nspname = 'public' AND cls.relname = 'schema_migrations'
      AND (acl.grantee = 0 OR acl.grantee = (SELECT oid FROM actor))
)
SELECT pg_catalog.jsonb_build_object(
    'format', 'espacigo.gcp04.w7.b1-actor-probe.v1',
    'target', pg_catalog.jsonb_build_object(
        'database', current_database(),
        'session_user', session_user,
        'current_user', current_user,
        'server_version_num', current_setting('server_version_num')::integer,
        'transaction_read_only', current_setting('transaction_read_only')::boolean),
    'actor', COALESCE((SELECT pg_catalog.jsonb_build_object(
        'name', rolname, 'login', rolcanlogin, 'superuser', rolsuper,
        'createdb', rolcreatedb, 'createrole', rolcreaterole,
        'replication', rolreplication, 'bypass_rls', rolbypassrls,
        'inherit', rolinherit) FROM actor), 'null'::jsonb),
    'memberships', COALESCE((SELECT pg_catalog.jsonb_agg(
        pg_catalog.to_jsonb(actor_memberships)
        ORDER BY granted_role, member_role) FROM actor_memberships), '[]'::jsonb),
    'owners', pg_catalog.jsonb_build_object(
        'database', pg_catalog.pg_get_userbyid(db.datdba),
        'public_schema', pg_catalog.pg_get_userbyid(ns.nspowner),
        'migration_ledger', CASE WHEN cls.oid IS NULL THEN NULL
                                 ELSE pg_catalog.pg_get_userbyid(cls.relowner) END),
    'capabilities', pg_catalog.jsonb_build_object(
        'create_role_attribute', COALESCE((SELECT rolcreaterole FROM actor), false),
        'database_connect_grant_option', EXISTS (SELECT 1 FROM relevant_acl
            WHERE object_kind='database' AND object_name=current_database()
              AND grantee='espacigo_gcp04_w7_bootstrap'
              AND privilege_type='CONNECT' AND is_grantable),
        'public_schema_usage_grant_option', EXISTS (SELECT 1 FROM relevant_acl
            WHERE object_kind='schema' AND object_name='public'
              AND grantee='espacigo_gcp04_w7_bootstrap'
              AND privilege_type='USAGE' AND is_grantable),
        'ledger_select_grant_option', CASE WHEN cls.oid IS NULL THEN NULL ELSE EXISTS (
            SELECT 1 FROM relevant_acl WHERE object_kind='ledger'
              AND grantee='espacigo_gcp04_w7_bootstrap'
              AND privilege_type='SELECT' AND is_grantable) END,
        'database_owner', pg_catalog.pg_get_userbyid(db.datdba)='espacigo_gcp04_w7_bootstrap',
        'public_schema_owner', pg_catalog.pg_get_userbyid(ns.nspowner)='espacigo_gcp04_w7_bootstrap',
        'ledger_owner', CASE WHEN cls.oid IS NULL THEN NULL ELSE
            pg_catalog.pg_get_userbyid(cls.relowner)='espacigo_gcp04_w7_bootstrap' END),
    'public_acl', COALESCE((SELECT pg_catalog.jsonb_agg(pg_catalog.to_jsonb(relevant_acl)
        ORDER BY object_kind, object_name, privilege_type, is_grantable)
        FROM relevant_acl WHERE grantee='PUBLIC'), '[]'::jsonb),
    'scope', pg_catalog.jsonb_build_object('business_rows_read', false,
        'password_or_secret_read', false, 'mutations', false)
)
FROM pg_catalog.pg_database db
JOIN pg_catalog.pg_namespace ns ON ns.nspname='public'
LEFT JOIN pg_catalog.pg_class cls ON cls.relnamespace=ns.oid
    AND cls.relname='schema_migrations' AND cls.relkind IN ('r','p')
WHERE db.datname=current_database();
COMMIT;
