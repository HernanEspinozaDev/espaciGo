BEGIN TRANSACTION READ ONLY;
SET TRANSACTION READ ONLY;
SELECT json_build_object(
  'identity', json_build_object(
    'database', current_database(), 'login', current_user,
    'server_version_num', current_setting('server_version_num')::integer,
    'actor', (SELECT json_build_object('login',rolcanlogin,'superuser',rolsuper,'createrole',rolcreaterole) FROM pg_roles WHERE rolname=current_user)),
  'runtime_role', (SELECT json_build_object('exists',true,'login',rolcanlogin,'superuser',rolsuper,'createdb',rolcreatedb,'createrole',rolcreaterole,'inherit',rolinherit)
    FROM pg_roles WHERE rolname='espacigo_runtime'),
  'runtime_membership', COALESCE((SELECT json_agg(json_build_object(
      'grantor',g.rolname,'set',m.set_option,'inherit',m.inherit_option,'admin',m.admin_option)
    ORDER BY g.rolname) FROM pg_auth_members m
    JOIN pg_roles r ON r.oid=m.roleid JOIN pg_roles u ON u.oid=m.member JOIN pg_roles g ON g.oid=m.grantor
    WHERE r.rolname='espacigo_runtime' AND u.rolname=current_user),'[]'::json),
  'public_schema_privileges', json_build_object(
    'login_create',has_schema_privilege(current_user,'public','CREATE'),
    'runtime_create',has_schema_privilege('espacigo_runtime','public','CREATE')),
  'relations', COALESCE((SELECT json_agg(json_build_object(
      'schema',n.nspname,'name',c.relname,'kind',c.relkind,'owner',pg_get_userbyid(c.relowner))
    ORDER BY n.nspname,c.relname,c.relkind) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f','S','i','I')
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),'[]'::json),
  'owned_sequence_dependencies', COALESCE((SELECT json_agg(json_build_object(
      'sequence',format('%I.%I',sn.nspname,s.relname),'sequence_owner',pg_get_userbyid(s.relowner),
      'parent',format('%I.%I',pn.nspname,p.relname),'parent_owner',pg_get_userbyid(p.relowner),'dependency',d.deptype)
    ORDER BY sn.nspname,s.relname) FROM pg_depend d JOIN pg_class s ON s.oid=d.objid
    JOIN pg_namespace sn ON sn.oid=s.relnamespace JOIN pg_class p ON p.oid=d.refobjid
    JOIN pg_namespace pn ON pn.oid=p.relnamespace
    WHERE d.classid='pg_class'::regclass AND d.refclassid='pg_class'::regclass AND s.relkind='S'
      AND sn.nspname='public' AND d.deptype IN ('a','i')),'[]'::json),
  'routines', COALESCE((SELECT json_agg(json_build_object(
      'identity',format('%I.%I(%s)',n.nspname,p.proname,pg_get_function_identity_arguments(p.oid)),
      'kind',p.prokind,'owner',pg_get_userbyid(p.proowner)) ORDER BY n.nspname,p.proname,p.prokind,p.oid)
    FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace WHERE n.nspname='public'
      AND NOT EXISTS (SELECT 1 FROM pg_depend d WHERE d.classid='pg_proc'::regclass AND d.objid=p.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)),'[]'::json),
  'text_search', json_build_object(
    'dictionaries',COALESCE((SELECT json_agg(json_build_object('schema',n.nspname,'name',d.dictname,'owner',pg_get_userbyid(d.dictowner)) ORDER BY n.nspname,d.dictname)
      FROM pg_ts_dict d JOIN pg_namespace n ON n.oid=d.dictnamespace WHERE n.nspname='public'),'[]'::json),
    'configurations',COALESCE((SELECT json_agg(json_build_object('schema',n.nspname,'name',c.cfgname,'owner',pg_get_userbyid(c.cfgowner)) ORDER BY n.nspname,c.cfgname)
      FROM pg_ts_config c JOIN pg_namespace n ON n.oid=c.cfgnamespace WHERE n.nspname='public'),'[]'::json)),
  'observer_column_privileges', COALESCE((SELECT json_agg(json_build_object(
      'table',format('%I.%I',n.nspname,c.relname),'column',a.attname,
      'select',has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'SELECT'),
      'insert',has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'INSERT'),
      'update',has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'UPDATE'),
      'references',has_column_privilege('espacigo_gcp03_observer',c.oid,a.attnum,'REFERENCES'))
    ORDER BY c.relname,a.attnum) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    JOIN pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped
    WHERE n.nspname='public' AND c.relkind IN ('r','p')),'[]'::json),
  'migration_history', (SELECT json_build_object('count',count(*),'versions',COALESCE(json_agg(json_build_array(version::text,checksum::text) ORDER BY version),'[]'::json)) FROM public.schema_migrations)
)::text;
COMMIT;
