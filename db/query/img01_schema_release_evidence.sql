-- Read-only release evidence for IMG-01. Run through the canonical local SSH
-- forward via the existing bastion to the fixed Cloud SQL database. This query returns catalog metadata
-- and migration checksums only; it never reads application rows or secrets.
\pset format unaligned
\pset tuples_only on
BEGIN READ ONLY;
SELECT jsonb_build_object(
    'format', 'espacigo.img01.schema-evidence.v1',
    'captured_at', pg_catalog.clock_timestamp(),
    'target', jsonb_build_object(
        'project_id', 'espacigo-testprod',
        'instance_connection_name', 'espacigo-testprod:southamerica-west1:espacigo-testprod-db',
        'database', current_database(),
        'server_version_num', current_setting('server_version_num')::integer,
        'transaction_read_only', current_setting('transaction_read_only'),
        'session_user', session_user,
        'current_user', current_user,
        'database_owner', pg_catalog.pg_get_userbyid(d.datdba),
        'public_schema_owner', pg_catalog.pg_get_userbyid(n.nspowner),
        'schema_migrations_owner', pg_catalog.pg_get_userbyid(history.relowner),
        'schema_migrations_kind', history.relkind::text
    ),
    'migration_ledger', jsonb_build_object(
        'count', ledger.row_count,
        'first_version', ledger.first_version,
        'last_version', ledger.last_version,
        'entries', ledger.entries
    ),
    'relations', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'name', c.relname,
            'kind', c.relkind::text,
            'owner', pg_catalog.pg_get_userbyid(c.relowner)
        ) ORDER BY c.relname)
        FROM pg_catalog.pg_class c
        JOIN pg_catalog.pg_namespace ns ON ns.oid = c.relnamespace
        WHERE ns.nspname = 'public'
          AND c.relname IN ('media_asset','media_variant','media_variant_intent',
                            'space_media','gallery_item_order','rol_usuario_evento')
    ), '[]'::jsonb),
    'routines', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'name', p.proname,
            'arguments', pg_catalog.oidvectortypes(p.proargtypes),
            'owner', pg_catalog.pg_get_userbyid(p.proowner),
            'security_definer', p.prosecdef,
            'configuration', COALESCE(p.proconfig, ARRAY[]::text[]),
            'public_execute_revoked', NOT EXISTS (
                SELECT 1
                FROM pg_catalog.aclexplode(COALESCE(p.proacl, pg_catalog.acldefault('f', p.proowner))) acl
                WHERE acl.grantee = 0 AND acl.privilege_type = 'EXECUTE'
            )
        ) ORDER BY p.proname, pg_catalog.oidvectortypes(p.proargtypes))
        FROM pg_catalog.pg_proc p
        JOIN pg_catalog.pg_namespace ns ON ns.oid = p.pronamespace
        WHERE ns.nspname = 'public'
          AND p.proname IN ('claim_private_media_cleanup',
                            'complete_private_media_cleanup',
                            'fail_private_media_cleanup',
                            'finalize_pending_suppression_cleanup')
    ), '[]'::jsonb),
    'roles', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'name', r.rolname,
            'can_login', r.rolcanlogin,
            'superuser', r.rolsuper,
            'createdb', r.rolcreatedb,
            'createrole', r.rolcreaterole,
            'replication', r.rolreplication,
            'bypassrls', r.rolbypassrls,
            'inherit', r.rolinherit
        ) ORDER BY r.rolname)
        FROM pg_catalog.pg_roles r
        WHERE r.rolname IN ('espacigo_api_login','espacigo_runtime',
                            'espacigo_privacy_cleanup_worker',
                            'espacigo_migrator_login','espacigo_schema_owner')
           OR r.rolname IN (session_user, current_user)
    ), '[]'::jsonb),
    'memberships', COALESCE((
        SELECT jsonb_agg(jsonb_build_object(
            'granted_role', granted.rolname,
            'member', member.rolname,
            'grantor_oid', m.grantor::bigint,
            'admin_option', m.admin_option,
            'inherit_option', m.inherit_option,
            'set_option', m.set_option
        ) ORDER BY granted.rolname, member.rolname)
        FROM pg_catalog.pg_auth_members m
        JOIN pg_catalog.pg_roles granted ON granted.oid = m.roleid
        JOIN pg_catalog.pg_roles member ON member.oid = m.member
        WHERE granted.rolname IN ('espacigo_api_login','espacigo_runtime',
                                  'espacigo_privacy_cleanup_worker',
                                  'espacigo_migrator_login','espacigo_schema_owner')
           OR member.rolname IN ('espacigo_api_login','espacigo_runtime',
                                 'espacigo_privacy_cleanup_worker',
                                 'espacigo_migrator_login','espacigo_schema_owner')
    ), '[]'::jsonb),
    'runtime_privileges', jsonb_build_object(
        'database_connect', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN pg_catalog.has_database_privilege('espacigo_runtime', current_database(), 'CONNECT') ELSE false END,
        'database_create', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN pg_catalog.has_database_privilege('espacigo_runtime', current_database(), 'CREATE') ELSE false END,
        'database_temporary', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN pg_catalog.has_database_privilege('espacigo_runtime', current_database(), 'TEMPORARY') ELSE false END,
        'public_usage', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN pg_catalog.has_schema_privilege('espacigo_runtime', 'public', 'USAGE') ELSE false END,
        'public_create', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN pg_catalog.has_schema_privilege('espacigo_runtime', 'public', 'CREATE') ELSE false END,
        'space_media_select', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') AND pg_catalog.to_regclass('public.space_media') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_runtime', 'public.space_media', 'SELECT') ELSE false END,
        'space_media_insert', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') AND pg_catalog.to_regclass('public.space_media') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_runtime', 'public.space_media', 'INSERT') ELSE false END,
        'space_media_update', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') AND pg_catalog.to_regclass('public.space_media') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_runtime', 'public.space_media', 'UPDATE') ELSE false END,
        'space_media_delete', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') AND pg_catalog.to_regclass('public.space_media') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_runtime', 'public.space_media', 'DELETE') ELSE false END,
        'role_event_insert', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') AND pg_catalog.to_regclass('public.rol_usuario_evento') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_runtime', 'public.rol_usuario_evento', 'INSERT') ELSE false END,
        'ledger_select', pg_catalog.has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'SELECT'),
        'ledger_insert', pg_catalog.has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'INSERT'),
        'ledger_update', pg_catalog.has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'UPDATE'),
        'ledger_delete', pg_catalog.has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'DELETE')
    ),
    'worker_privileges', jsonb_build_object(
        'claim_execute', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regprocedure('public.claim_private_media_cleanup(integer,timestamp with time zone)') IS NOT NULL THEN pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker', 'public.claim_private_media_cleanup(integer,timestamp with time zone)', 'EXECUTE') ELSE false END,
        'complete_execute', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regprocedure('public.complete_private_media_cleanup(uuid,bigint,timestamp with time zone)') IS NOT NULL THEN pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker', 'public.complete_private_media_cleanup(uuid,bigint,timestamp with time zone)', 'EXECUTE') ELSE false END,
        'fail_execute', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regprocedure('public.fail_private_media_cleanup(uuid,bigint,timestamp with time zone)') IS NOT NULL THEN pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker', 'public.fail_private_media_cleanup(uuid,bigint,timestamp with time zone)', 'EXECUTE') ELSE false END,
        'finalize_execute', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regprocedure('public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone)') IS NOT NULL THEN pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker', 'public.finalize_pending_suppression_cleanup(uuid,timestamp with time zone)', 'EXECUTE') ELSE false END,
        'media_asset_select', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regclass('public.media_asset') IS NOT NULL THEN pg_catalog.has_column_privilege('espacigo_privacy_cleanup_worker', 'public.media_asset', 'estado', 'SELECT') ELSE false END,
        'media_variant_select', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regclass('public.media_variant') IS NOT NULL THEN pg_catalog.has_column_privilege('espacigo_privacy_cleanup_worker', 'public.media_variant', 'object_key', 'SELECT') ELSE false END,
        'media_intent_select', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regclass('public.media_variant_intent') IS NOT NULL THEN pg_catalog.has_column_privilege('espacigo_privacy_cleanup_worker', 'public.media_variant_intent', 'object_key', 'SELECT') ELSE false END,
        'media_asset_table_update', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regclass('public.media_asset') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker', 'public.media_asset', 'UPDATE') ELSE false END,
        'media_variant_table_delete', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regclass('public.media_variant') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker', 'public.media_variant', 'DELETE') ELSE false END,
        'media_intent_table_delete', CASE WHEN EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker') AND pg_catalog.to_regclass('public.media_variant_intent') IS NOT NULL THEN pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker', 'public.media_variant_intent', 'DELETE') ELSE false END
    )
)::text
FROM pg_catalog.pg_database d
JOIN pg_catalog.pg_namespace n ON n.nspname = 'public'
JOIN pg_catalog.pg_class history ON history.relnamespace = n.oid AND history.relname = 'schema_migrations'
CROSS JOIN LATERAL (
    SELECT count(*)::integer AS row_count,
           min(version)::integer AS first_version,
           max(version)::integer AS last_version,
           COALESCE(jsonb_agg(jsonb_build_object(
               'version', version, 'name', name, 'checksum', checksum
           ) ORDER BY version), '[]'::jsonb) AS entries
    FROM public.schema_migrations
) ledger
WHERE d.datname = current_database()
  AND current_database() = 'espacigo_dms_stage_20261020';
COMMIT;
