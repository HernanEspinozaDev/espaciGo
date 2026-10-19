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
WITH limits AS (
    SELECT current_setting('max_connections')::integer AS max_connections,
           current_setting('reserved_connections')::integer AS reserved_connections,
           current_setting('superuser_reserved_connections')::integer AS superuser_reserved_connections
), activity AS (
    SELECT count(*) FILTER (WHERE backend_type = 'client backend')::integer AS visible_client_backends,
           count(*) FILTER (WHERE backend_type IS NULL)::integer AS unclassified_activity_rows
    FROM pg_catalog.pg_stat_activity
), relation_inventory AS (
    SELECT c.oid, n.nspname || '.' || c.relname AS identity, c.relkind AS kind,
           pg_catalog.pg_get_userbyid(c.relowner) AS owner,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_class'::regclass AND d.objid=c.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f','S')
), routine_inventory AS (
    SELECT p.oid, n.nspname || '.' || p.oid::regprocedure::text AS identity,
           pg_catalog.pg_get_userbyid(p.proowner) AS owner, p.prosecdef AS security_definer,
           p.proconfig AS settings,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_proc'::regclass AND d.objid=p.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
    WHERE n.nspname='public'
), type_inventory AS (
    SELECT t.oid, n.nspname || '.' || t.typname AS identity,
           pg_catalog.pg_get_userbyid(t.typowner) AS owner, t.typtype AS kind,
           EXISTS (SELECT 1 FROM pg_catalog.pg_depend d
                   WHERE d.classid='pg_catalog.pg_type'::regclass AND d.objid=t.oid
                     AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
    FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace
    WHERE n.nspname='public' AND t.typrelid=0 AND t.typtype IN ('b','d','e','r','m')
), other_schema_objects AS (
    SELECT count(*)::integer AS object_count FROM (
      SELECT o.oid, 'operator'::text AS kind, o.oprnamespace AS namespace_oid,
             EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_operator'::regclass AND d.objid=o.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e') AS extension_owned
      FROM pg_catalog.pg_operator o
      UNION ALL
      SELECT c.oid, 'collation', c.collnamespace,
             EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_collation'::regclass AND d.objid=c.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
      FROM pg_catalog.pg_collation c
      UNION ALL
      SELECT c.oid, 'conversion', c.connamespace,
             EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_conversion'::regclass AND d.objid=c.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
      FROM pg_catalog.pg_conversion c
      UNION ALL
      SELECT o.oid, 'opclass', o.opcnamespace,
             EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_opclass'::regclass AND d.objid=o.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
      FROM pg_catalog.pg_opclass o
      UNION ALL
      SELECT o.oid, 'opfamily', o.opfnamespace,
             EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_opfamily'::regclass AND d.objid=o.oid AND d.refclassid='pg_catalog.pg_extension'::regclass AND d.deptype='e')
      FROM pg_catalog.pg_opfamily o
    ) objects
    JOIN pg_catalog.pg_namespace n ON n.oid=objects.namespace_oid
    WHERE n.nspname='public' AND NOT extension_owned
), app_roles AS (
    SELECT r.oid, r.rolname AS name, r.rolcanlogin AS login, r.rolsuper AS super,
           r.rolcreatedb AS createdb, r.rolcreaterole AS createrole,
           r.rolreplication AS replication, r.rolbypassrls AS bypassrls,
           r.rolinherit AS inherit,
           (SELECT d.description FROM pg_catalog.pg_shdescription d
            WHERE d.objoid=r.oid AND d.classoid='pg_catalog.pg_authid'::regclass) AS comment
    FROM pg_catalog.pg_roles r
    WHERE r.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker',
      'espacigo_db_observer','espacigo_gadm_bootstrap','espacigo_api_login',
      'espacigo_privacy_worker_login','espacigo_migrator_login','espacigo_observer_login')
), memberships AS (
    SELECT granted.rolname AS granted_role, member.rolname AS member_role,
           m.admin_option, m.inherit_option, m.set_option
    FROM pg_catalog.pg_auth_members m
    JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
    JOIN pg_catalog.pg_roles member ON member.oid=m.member
    WHERE granted.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_db_observer','espacigo_gadm_bootstrap')
       OR member.rolname IN ('espacigo_schema_owner','espacigo_runtime','espacigo_privacy_cleanup_worker','espacigo_db_observer','espacigo_gadm_bootstrap')
), ext AS (
    SELECT e.extname AS name, e.extversion AS version
    FROM pg_catalog.pg_extension e
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
      'capacity', (SELECT to_jsonb(c) || jsonb_build_object('required_connections', :'required_connections'::integer,
        'passes', c.free_slots_lower_bound >= :'required_connections'::integer) FROM capacity c),
      'destination', jsonb_build_object(
        'relations_non_extension',(SELECT count(*)::integer FROM relation_inventory WHERE NOT extension_owned),
        'routines_non_extension',(SELECT count(*)::integer FROM routine_inventory WHERE NOT extension_owned),
        'types_non_extension',(SELECT count(*)::integer FROM type_inventory WHERE NOT extension_owned),
        'other_schema_objects_non_extension',(SELECT object_count FROM other_schema_objects),
        'schema_migrations_present',EXISTS(SELECT 1 FROM relation_inventory WHERE identity='public.schema_migrations' AND NOT extension_owned),
        'relation_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'kind',kind,'owner',owner,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM relation_inventory),
        'routine_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'owner',owner,'security_definer',security_definer,'settings',settings,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM routine_inventory),
        'type_inventory',(SELECT coalesce(jsonb_agg(jsonb_build_object('identity',identity,'kind',kind,'owner',owner,'extension_owned',extension_owned) ORDER BY identity),'[]'::jsonb) FROM type_inventory)),
      'roles',(SELECT coalesce(jsonb_agg(to_jsonb(r)-'oid' ORDER BY r.name),'[]'::jsonb) FROM app_roles r),
      'memberships',(SELECT coalesce(jsonb_agg(to_jsonb(m) ORDER BY m.granted_role,m.member_role),'[]'::jsonb) FROM memberships m),
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
          SELECT 'schema',n.nspname,n.nspowner,x.* FROM pg_catalog.pg_namespace n CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) x WHERE n.nspname='public' AND n.nspacl IS NOT NULL
          UNION ALL
          SELECT 'relation',n.nspname||'.'||c.relname,c.relowner,x.* FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) x WHERE n.nspname='public' AND c.relacl IS NOT NULL
          UNION ALL
          SELECT 'column',n.nspname||'.'||c.relname||'.'||a.attname,c.relowner,x.* FROM pg_catalog.pg_attribute a JOIN pg_catalog.pg_class c ON c.oid=a.attrelid JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) x WHERE n.nspname='public' AND a.attnum>0 AND NOT a.attisdropped AND a.attacl IS NOT NULL
          UNION ALL
          SELECT 'routine',n.nspname||'.'||p.oid::regprocedure::text,p.proowner,x.* FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) x WHERE n.nspname='public' AND p.proacl IS NOT NULL
          UNION ALL
          SELECT 'type',n.nspname||'.'||t.typname,t.typowner,x.* FROM pg_catalog.pg_type t JOIN pg_catalog.pg_namespace n ON n.oid=t.typnamespace CROSS JOIN LATERAL pg_catalog.aclexplode(t.typacl) x WHERE n.nspname='public' AND t.typacl IS NOT NULL
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
        WHERE n.nspname='public' AND t.typrelid=0 AND t.typtype IN ('b','d','e','r','m')
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
