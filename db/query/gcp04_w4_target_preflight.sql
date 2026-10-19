-- Read-only, sanitized target inventory for a future reviewed Cloud Run Job.
-- Execute only through the Cloud SQL connector over the private address.
-- The invoking role needs database/catalog read access; no data rows are read.
\set ON_ERROR_STOP on
\if :{?required_connections}
\else
  DO $invalid$
  BEGIN
    RAISE EXCEPTION 'required_connections is required and must be a reviewed positive integer';
  END
  $invalid$;
\endif
SELECT (:'required_connections' ~ '^[1-9][0-9]{0,5}$') AS required_connections_valid \gset
\if :required_connections_valid
\else
  DO $invalid$
  BEGIN
    RAISE EXCEPTION 'required_connections must be a positive integer from 1 to 999999';
  END
  $invalid$;
\endif
BEGIN READ ONLY;
WITH RECURSIVE limits AS (
    SELECT current_setting('max_connections')::integer AS max_connections,
           current_setting('reserved_connections')::integer AS reserved_connections,
           current_setting('superuser_reserved_connections')::integer AS superuser_reserved_connections
), activity AS (
    SELECT count(*) FILTER (WHERE backend_type = 'client backend')::integer AS visible_client_backends,
           count(*) FILTER (WHERE backend_type IS NULL)::integer AS unclassified_activity_rows
    FROM pg_catalog.pg_stat_activity
), user_schemas AS (
    SELECT n.oid, n.nspname AS name, pg_catalog.pg_get_userbyid(n.nspowner) AS owner,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_namespace'::regclass AND d.objid=n.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_namespace n
    WHERE n.nspname <> 'information_schema' AND n.nspname !~ '^pg_'
), relation_inventory AS (
    SELECT c.oid, n.nspname || '.' || c.relname AS identity, n.nspname AS schema_name,
           c.relkind AS kind, pg_catalog.pg_get_userbyid(c.relowner) AS owner,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
           OR (c.relkind='c' AND EXISTS (SELECT 1 FROM pg_catalog.pg_type typ
               JOIN pg_catalog.pg_depend type_dep ON type_dep.classid='pg_catalog.pg_type'::regclass
                 AND type_dep.objid=typ.oid AND type_dep.refclassid='pg_catalog.pg_extension'::regclass
                 AND type_dep.deptype='e' WHERE typ.typrelid=c.oid))
           OR (c.relkind IN ('i','I') AND EXISTS (SELECT 1 FROM pg_catalog.pg_index ix
               JOIN pg_catalog.pg_depend parent_dep ON parent_dep.classid='pg_catalog.pg_class'::regclass
                 AND parent_dep.objid=ix.indrelid AND parent_dep.refclassid='pg_catalog.pg_extension'::regclass
                 AND parent_dep.deptype='e' WHERE ix.indexrelid=c.oid)) AS extension_owned
    FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
    JOIN user_schemas s ON s.oid=n.oid
    WHERE c.relkind IN ('r','p','v','m','f','S','c','i','I')
), routine_inventory AS (
    SELECT p.oid, n.nspname || '.' || p.oid::regprocedure::text AS identity,
           n.nspname AS schema_name, pg_catalog.pg_get_userbyid(p.proowner) AS owner,
           p.prosecdef AS security_definer, p.proconfig AS settings,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
    JOIN user_schemas s ON s.oid=n.oid
), type_inventory AS (
    SELECT t.oid, n.nspname || '.' || t.typname AS identity, n.nspname AS schema_name,
           pg_catalog.pg_get_userbyid(t.typowner) AS owner, t.typtype AS kind,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_type'::regclass AND d.objid=t.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
    JOIN user_schemas s ON s.oid=n.oid
    WHERE t.typrelid=0 AND t.typtype IN ('b','c','d','e','r','m')
), other_schema_inventory AS (
    SELECT 'operator'::text AS kind, o.oid::regoperator::text AS identity,
           n.nspname AS schema_name, pg_catalog.pg_get_userbyid(o.oprowner) AS owner,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_operator'::regclass AND d.objid=o.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_operator o JOIN pg_catalog.pg_namespace n ON n.oid=o.oprnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'collation', n.nspname||'.'||c.collname, n.nspname, pg_catalog.pg_get_userbyid(c.collowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_collation'::regclass AND d.objid=c.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
    FROM pg_catalog.pg_collation c JOIN pg_catalog.pg_namespace n ON n.oid=c.collnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'conversion', n.nspname||'.'||c.conname, n.nspname, pg_catalog.pg_get_userbyid(c.conowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_conversion'::regclass AND d.objid=c.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
    FROM pg_catalog.pg_conversion c JOIN pg_catalog.pg_namespace n ON n.oid=c.connamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'opclass', n.nspname||'.'||o.opcname, n.nspname, pg_catalog.pg_get_userbyid(o.opcowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_opclass'::regclass AND d.objid=o.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
    FROM pg_catalog.pg_opclass o JOIN pg_catalog.pg_namespace n ON n.oid=o.opcnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'opfamily', n.nspname||'.'||o.opfname, n.nspname, pg_catalog.pg_get_userbyid(o.opfowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_opfamily'::regclass AND d.objid=o.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
    FROM pg_catalog.pg_opfamily o JOIN pg_catalog.pg_namespace n ON n.oid=o.opfnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'text_search_dictionary', n.nspname||'.'||d.dictname, n.nspname, pg_catalog.pg_get_userbyid(d.dictowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend dep WHERE dep.classid='pg_catalog.pg_ts_dict'::regclass AND dep.objid=d.oid AND dep.refclassid='pg_catalog.pg_extension'::regclass AND dep.deptype='e')
    FROM pg_catalog.pg_ts_dict d JOIN pg_catalog.pg_namespace n ON n.oid=d.dictnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'text_search_parser', n.nspname||'.'||p.prsname, n.nspname, NULL::text,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend dep WHERE dep.classid='pg_catalog.pg_ts_parser'::regclass AND dep.objid=p.oid AND dep.refclassid='pg_catalog.pg_extension'::regclass AND dep.deptype='e')
    FROM pg_catalog.pg_ts_parser p JOIN pg_catalog.pg_namespace n ON n.oid=p.prsnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'text_search_template', n.nspname||'.'||t.tmplname, n.nspname, NULL::text,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend dep WHERE dep.classid='pg_catalog.pg_ts_template'::regclass AND dep.objid=t.oid AND dep.refclassid='pg_catalog.pg_extension'::regclass AND dep.deptype='e')
    FROM pg_catalog.pg_ts_template t JOIN pg_catalog.pg_namespace n ON n.oid=t.tmplnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'text_search_configuration', n.nspname||'.'||c.cfgname, n.nspname, pg_catalog.pg_get_userbyid(c.cfgowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend dep WHERE dep.classid='pg_catalog.pg_ts_config'::regclass AND dep.objid=c.oid AND dep.refclassid='pg_catalog.pg_extension'::regclass AND dep.deptype='e')
    FROM pg_catalog.pg_ts_config c JOIN pg_catalog.pg_namespace n ON n.oid=c.cfgnamespace JOIN user_schemas s ON s.oid=n.oid
    UNION ALL
    SELECT 'extended_statistics', n.nspname||'.'||e.stxname, n.nspname, pg_catalog.pg_get_userbyid(e.stxowner),
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend dep WHERE dep.classid='pg_catalog.pg_statistic_ext'::regclass AND dep.objid=e.oid AND dep.refclassid='pg_catalog.pg_extension'::regclass AND dep.deptype='e')
    FROM pg_catalog.pg_statistic_ext e JOIN pg_catalog.pg_namespace n ON n.oid=e.stxnamespace JOIN user_schemas s ON s.oid=n.oid
), app_roles AS (
    SELECT r.oid, r.rolname AS name, r.rolcanlogin AS login, r.rolsuper AS super,
           r.rolcreatedb AS createdb, r.rolcreaterole AS createrole,
           r.rolreplication AS replication, r.rolbypassrls AS bypassrls,
           r.rolinherit AS inherit,
           (SELECT d.description FROM pg_catalog.pg_shdescription d
            WHERE d.objoid=r.oid AND d.classoid='pg_catalog.pg_authid'::regclass) AS comment
    FROM pg_catalog.pg_roles r
    WHERE left(r.rolname,9)='espacigo_'
), login_roots AS (
    SELECT oid, name FROM app_roles WHERE login
), role_closure(root_oid, reachable_oid) AS (
    SELECT oid, oid FROM login_roots
    UNION
    SELECT c.root_oid, m.roleid
    FROM role_closure c JOIN pg_catalog.pg_auth_members m ON m.member=c.reachable_oid
    WHERE m.inherit_option OR m.set_option OR m.admin_option
), memberships AS (
    SELECT DISTINCT root.name AS app_login, granted.rolname AS granted_role,
           member.rolname AS member_role, m.admin_option, m.inherit_option, m.set_option,
           'transitive-login-membership'::text AS edge_context
    FROM role_closure c
    JOIN login_roots root ON root.oid=c.root_oid
    JOIN pg_catalog.pg_auth_members m ON m.member=c.reachable_oid
      AND (m.inherit_option OR m.set_option OR m.admin_option)
    JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
    JOIN pg_catalog.pg_roles member ON member.oid=m.member
    UNION
    SELECT NULL::text, granted.rolname, member.rolname, m.admin_option, m.inherit_option, m.set_option,
           'direct-edge-touches-application-role'::text
    FROM pg_catalog.pg_auth_members m
    JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
    JOIN pg_catalog.pg_roles member ON member.oid=m.member
    WHERE granted.oid IN (SELECT oid FROM app_roles) OR member.oid IN (SELECT oid FROM app_roles)
), approved_extensions(name, version) AS (
    VALUES ('btree_gist','1.8'), ('plpgsql','1.0'), ('postgis','3.6.4'), ('unaccent','1.1')
), ext AS (
    SELECT e.extname AS name, e.extversion AS version,
           a.version AS approved_version, (e.extversion=a.version) AS approved_exact
    FROM pg_catalog.pg_extension e LEFT JOIN approved_extensions a ON a.name=e.extname
), all_user_object_inventory AS (
    SELECT 'relation'::text AS kind, identity, schema_name, owner, extension_owned FROM relation_inventory
    UNION ALL
    SELECT 'routine', identity, schema_name, owner, extension_owned FROM routine_inventory
    UNION ALL
    SELECT 'type', identity, schema_name, owner, extension_owned FROM type_inventory
    UNION ALL
    SELECT kind, identity, schema_name, owner, extension_owned FROM other_schema_inventory
), capacity AS (
    SELECT limits.max_connections, limits.reserved_connections, limits.superuser_reserved_connections,
           activity.visible_client_backends, activity.unclassified_activity_rows,
           greatest(limits.max_connections-limits.reserved_connections-limits.superuser_reserved_connections
             -activity.visible_client_backends-activity.unclassified_activity_rows,0)::integer AS free_slots_lower_bound
    FROM limits CROSS JOIN activity
), report AS (
    SELECT jsonb_build_object(
      'database', jsonb_build_object('name',current_database(),'server_version_num',current_setting('server_version_num')::integer,
        'server_version',current_setting('server_version'),'user',current_user,
        'transaction_read_only',current_setting('transaction_read_only')::boolean,
        'owner',(SELECT pg_catalog.pg_get_userbyid(d.datdba) FROM pg_catalog.pg_database d WHERE d.datname=current_database()),
        'public_schema_owner',(SELECT pg_catalog.pg_get_userbyid(n.nspowner) FROM pg_catalog.pg_namespace n WHERE n.nspname='public')),
      'public_effective_privileges', jsonb_build_object(
        'database_connect', has_database_privilege('public', current_database(), 'CONNECT'),
        'database_create', has_database_privilege('public', current_database(), 'CREATE'),
        'database_temporary', has_database_privilege('public', current_database(), 'TEMP'),
        'public_schema_usage', has_schema_privilege('public', 'public', 'USAGE'),
        'public_schema_create', has_schema_privilege('public', 'public', 'CREATE')),
      'capacity', (SELECT to_jsonb(c) || jsonb_build_object('required_connections', :'required_connections'::integer,
        'passes', c.free_slots_lower_bound >= :'required_connections'::integer) FROM capacity c),
      'destination', jsonb_build_object(
        'relations_non_extension',(SELECT count(*)::integer FROM relation_inventory WHERE NOT extension_owned),
        'routines_non_extension',(SELECT count(*)::integer FROM routine_inventory WHERE NOT extension_owned),
        'types_non_extension',(SELECT count(*)::integer FROM type_inventory WHERE NOT extension_owned),
        'other_schema_objects_non_extension',(SELECT count(*)::integer FROM other_schema_inventory WHERE NOT extension_owned),
        'non_public_schemas_non_extension',(SELECT count(*)::integer FROM user_schemas WHERE name<>'public' AND NOT extension_owned),
        'non_public_objects_non_extension',(SELECT count(*)::integer FROM all_user_object_inventory WHERE schema_name<>'public' AND NOT extension_owned),
        'unapproved_extensions',(SELECT count(*)::integer FROM ext WHERE approved_exact IS DISTINCT FROM TRUE),
        'empty_destination_candidate',
          (NOT EXISTS(SELECT 1 FROM all_user_object_inventory WHERE NOT extension_owned)
           AND NOT EXISTS(SELECT 1 FROM user_schemas WHERE name<>'public' AND NOT extension_owned)
           AND to_regclass('public.schema_migrations') IS NULL
           AND NOT EXISTS(SELECT 1 FROM ext WHERE approved_exact IS DISTINCT FROM TRUE)),
        'schema_migrations_present',EXISTS(SELECT 1 FROM relation_inventory WHERE identity='public.schema_migrations' AND NOT extension_owned),
        'schema_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('name',name,'owner',owner,'extension_owned',extension_owned) ORDER BY name),'[]'::jsonb) FROM user_schemas),
        'relation_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'schema',schema_name,'kind',kind,'owner',owner,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM relation_inventory),
        'routine_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'schema',schema_name,'owner',owner,'security_definer',security_definer,'settings',settings,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM routine_inventory),
        'type_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'schema',schema_name,'kind',kind,'owner',owner,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM type_inventory),
        'other_schema_object_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'schema',schema_name,'kind',kind,'owner',owner,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM other_schema_inventory),
        'non_public_object_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'schema',schema_name,'kind',kind,'owner',owner,'extension_owned',extension_owned) ORDER BY schema_name,kind,identity),'[]'::jsonb) FROM all_user_object_inventory WHERE schema_name<>'public')),
      'roles',(SELECT coalesce(jsonb_agg(to_jsonb(r)-'oid' ORDER BY r.name),'[]'::jsonb) FROM app_roles r),
      'memberships',(SELECT coalesce(jsonb_agg(to_jsonb(m) ORDER BY m.app_login NULLS LAST,m.granted_role,m.member_role,m.edge_context),'[]'::jsonb) FROM memberships m),
      'extensions',(SELECT coalesce(jsonb_agg(to_jsonb(e) ORDER BY e.name),'[]'::jsonb) FROM ext e),
      'schema_migrations',jsonb_build_object('present',to_regclass('public.schema_migrations') IS NOT NULL),
      'acl_entries',(SELECT coalesce(jsonb_agg(jsonb_build_object('kind',a.kind,'identity',a.identity,
        'object_owner',pg_catalog.pg_get_userbyid(a.object_owner),'grantor',pg_catalog.pg_get_userbyid(a.grantor),
        'grantee',CASE WHEN a.grantee=0 THEN 'PUBLIC' ELSE pg_catalog.pg_get_userbyid(a.grantee) END,
        'privilege',a.privilege_type,'grantable',a.is_grantable)
        ORDER BY a.kind,a.identity,a.object_owner,a.grantor,a.grantee,a.privilege_type,a.is_grantable),'[]'::jsonb)
        FROM (
          SELECT 'database'::text AS kind,d.datname::text AS identity,d.datdba AS object_owner,x.*
            FROM pg_catalog.pg_database d CROSS JOIN LATERAL pg_catalog.aclexplode(d.datacl) x WHERE d.datname=current_database() AND d.datacl IS NOT NULL
          UNION ALL
          SELECT 'schema',n.nspname,n.nspowner,x.* FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) x WHERE n.oid IN (SELECT oid FROM user_schemas) AND n.nspacl IS NOT NULL
          UNION ALL
          SELECT 'relation',n.nspname||'.'||c.relname,c.relowner,x.* FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) x WHERE n.oid IN (SELECT oid FROM user_schemas) AND c.relacl IS NOT NULL
          UNION ALL
          SELECT 'column',n.nspname||'.'||c.relname||'.'||a.attname,c.relowner,x.* FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) x WHERE n.oid IN (SELECT oid FROM user_schemas) AND a.attnum>0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
          UNION ALL
          SELECT 'routine',n.nspname||'.'||p.oid::regprocedure::text,p.proowner,x.* FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) x WHERE n.oid IN (SELECT oid FROM user_schemas) AND p.proacl IS NOT NULL
          UNION ALL
          SELECT 'type',n.nspname||'.'||t.typname,t.typowner,x.* FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(t.typacl) x WHERE n.oid IN (SELECT oid FROM user_schemas) AND t.typacl IS NOT NULL
          UNION ALL
          SELECT 'default',coalesce(n.nspname,'')||':'||d.defaclobjtype::text,d.defaclrole,x.* FROM pg_catalog.pg_default_acl d LEFT JOIN pg_catalog.pg_namespace n ON n.oid=d.defaclnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(d.defaclacl) x
        ) a),
      'owner_effective_privileges',(SELECT coalesce(jsonb_agg(x.row ORDER BY x.kind,x.identity),'[]'::jsonb) FROM (
        SELECT 'relation'::text AS kind,n.nspname||'.'||c.relname AS identity,
          jsonb_build_object('kind','relation','identity',n.nspname||'.'||c.relname,'owner',pg_catalog.pg_get_userbyid(c.relowner),
            'required_privileges',spec.required,'missing_effective_privileges',
            to_jsonb(ARRAY(SELECT p FROM unnest(spec.required) p WHERE NOT CASE WHEN c.relkind='S' THEN has_sequence_privilege(c.relowner,c.oid,p) ELSE has_table_privilege(c.relowner,c.oid,p) END ORDER BY p))) AS row
        FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
        CROSS JOIN LATERAL (SELECT CASE WHEN c.relkind='S' THEN ARRAY['SELECT','USAGE','UPDATE']::text[] ELSE ARRAY['SELECT','INSERT','UPDATE','DELETE','TRUNCATE','REFERENCES','TRIGGER','MAINTAIN']::text[] END AS required) spec
        WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f','S')
        UNION ALL
        SELECT 'routine',n.nspname||'.'||p.oid::regprocedure::text,
          jsonb_build_object('kind','routine','identity',n.nspname||'.'||p.oid::regprocedure::text,'owner',pg_catalog.pg_get_userbyid(p.proowner),
            'required_privileges',ARRAY['EXECUTE']::text[],'missing_effective_privileges',ARRAY(SELECT 'EXECUTE' WHERE NOT has_function_privilege(p.proowner,p.oid,'EXECUTE')))
        FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'
        UNION ALL
        SELECT 'type',n.nspname||'.'||t.typname,
          jsonb_build_object('kind','type','identity',n.nspname||'.'||t.typname,'owner',pg_catalog.pg_get_userbyid(t.typowner),
            'required_privileges',ARRAY['USAGE']::text[],'missing_effective_privileges',ARRAY(SELECT 'USAGE' WHERE NOT has_type_privilege(t.typowner,t.oid,'USAGE')))
        FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
        WHERE n.nspname='public' AND t.typrelid=0 AND t.typtype IN ('b','c','d','e','r','m')
        UNION ALL
        SELECT 'schema',n.nspname,jsonb_build_object('kind','schema','identity',n.nspname,'owner',pg_catalog.pg_get_userbyid(n.nspowner),
            'required_privileges',ARRAY['USAGE','CREATE']::text[],'missing_effective_privileges',ARRAY(SELECT p FROM unnest(ARRAY['USAGE','CREATE']::text[]) p WHERE NOT has_schema_privilege(n.nspowner,n.oid,p) ORDER BY p))
        FROM pg_catalog.pg_namespace n WHERE n.nspname='public'
        UNION ALL
        SELECT 'database',d.datname,jsonb_build_object('kind','database','identity',d.datname,'owner',pg_catalog.pg_get_userbyid(d.datdba),
            'required_privileges',ARRAY['CONNECT','CREATE','TEMP']::text[],'missing_effective_privileges',ARRAY(SELECT p FROM unnest(ARRAY['CONNECT','CREATE','TEMP']::text[]) p WHERE NOT has_database_privilege(d.datdba,d.oid,p) ORDER BY p))
        FROM pg_catalog.pg_database d WHERE d.datname=current_database()
      ) x),
      'required_connections', :'required_connections'::integer
    ) AS report
)
SELECT report FROM report;
COMMIT;
