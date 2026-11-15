-- Close temporary V94 CREATE authority and read back exact direct ACL tuples.
-- Table DELETE intentionally remains at V94 (19 tuples) for the bounded
-- SECURITY DEFINER retirement function; at ledger 93 revoke it with the window.
-- All changes are transactional. If a guard fails, CREATE/revokes roll back;
-- do not independently revoke before the failed guard: human recovery is required.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE n bigint; v bigint; f oid; owner_oid oid; worker_oid oid; migrator_oid oid; outbox_oid oid;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020' OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388 OR current_setting('server_version_num')<>'180006' OR session_user<>'postgres' OR current_user<>'postgres' THEN RAISE EXCEPTION 'native V94 finalizer target/administrator mismatch'; END IF;
 SELECT count(*),max(version) INTO n,v FROM public.schema_migrations;
 IF n NOT IN (93,94) OR v<>n OR (SELECT min(version) FROM public.schema_migrations)<>1 OR (SELECT checksum FROM public.schema_migrations WHERE version=93) IS DISTINCT FROM '885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292' THEN RAISE EXCEPTION 'native V94 finalizer requires contiguous ledger93 or ledger94'; END IF;
 IF v=94 AND ((SELECT name FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'V000094__identity_delivered_session_outbox_retirement.sql'
 OR (SELECT checksum FROM public.schema_migrations WHERE version=94) IS DISTINCT FROM 'a4ee15a45ab352527e93c00e82763056b57b3750064483b8b90aa6891c0672a9') THEN RAISE EXCEPTION 'native V94 finalizer reviewed name/checksum mismatch'; END IF;
 SELECT oid INTO owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 SELECT oid INTO migrator_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_migrator_login' AND rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 SELECT oid INTO worker_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker' AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 SELECT 'public.outbox_evento_local'::regclass::oid INTO outbox_oid;
 IF owner_oid IS NULL OR migrator_oid IS NULL OR worker_oid IS NULL OR outbox_oid IS NULL OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid=outbox_oid)<>'postgres'::regrole THEN RAISE EXCEPTION 'native V94 safe roles or postgres-owned outbox mismatch'; END IF;
 IF (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=owner_oid OR m.member=owner_oid)<>2 OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE (m.roleid=owner_oid OR m.member=owner_oid) AND NOT (m.grantor='postgres'::regrole AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option AND ((m.roleid=owner_oid AND m.member=migrator_oid) OR (m.roleid='espacigo_document_verification_fn'::regrole AND m.member=owner_oid)))) THEN RAISE EXCEPTION 'native V94 schema-owner membership mismatch'; END IF;
 IF EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE (m.roleid=worker_oid OR m.member=worker_oid) AND NOT (m.roleid=worker_oid AND m.member=COALESCE((SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_worker_login' AND rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls),0::oid) AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option)) THEN RAISE EXCEPTION 'native V94 worker membership mismatch'; END IF;
 IF v=94 THEN
  SELECT p.oid,p.proowner INTO f,owner_oid FROM pg_catalog.pg_proc p WHERE p.oid=pg_catalog.to_regprocedure('public.purge_delivered_identity_session_outbox()') AND p.prosecdef AND p.proconfig @> ARRAY['search_path=pg_catalog'];
  IF f IS NULL OR owner_oid<>(SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner') OR pg_catalog.has_function_privilege('espacigo_runtime',f,'EXECUTE') OR NOT pg_catalog.has_function_privilege('espacigo_privacy_cleanup_worker',f,'EXECUTE') OR EXISTS (SELECT 1 FROM pg_catalog.aclexplode(COALESCE((SELECT proacl FROM pg_catalog.pg_proc WHERE oid=f),pg_catalog.acldefault('f'::"char",owner_oid))) a WHERE a.grantee=0 AND a.privilege_type='EXECUTE') THEN RAISE EXCEPTION 'native V94 installed function owner or EXECUTE mismatch'; END IF;
 END IF;
 -- Require exactly the 19 prepared tuples (12 baseline + 7 temporary additions).
 WITH actual AS (SELECT a.attname,x.privilege_type,x.grantor,x.grantee,x.is_grantable FROM pg_catalog.pg_attribute a CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x WHERE a.attrelid=outbox_oid AND a.attnum>0 AND NOT a.attisdropped AND x.grantee=owner_oid),
 expected(attname,privilege_type,grantor,grantee,is_grantable) AS (SELECT c.attname,'INSERT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('id'),('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('version'),('creada_en'),('disponible_en')) c(attname) UNION ALL SELECT c.attname,'SELECT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('id'),('entregada_en'),('retirar_en'),('cancelada_en'),('fallo_terminal_en')) c(attname) UNION ALL SELECT 'id','UPDATE','postgres'::regrole::oid,owner_oid,false),
 tab AS (SELECT x.privilege_type,x.grantor,x.grantee,x.is_grantable FROM pg_catalog.pg_class c CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r'::"char",c.relowner))) x WHERE c.oid=outbox_oid AND x.grantee=owner_oid),
 allactual AS (SELECT attname,privilege_type,grantor,grantee,is_grantable FROM actual UNION ALL SELECT NULL::name,privilege_type,grantor,grantee,is_grantable FROM tab),
 allexpected(attname,privilege_type,grantor,grantee,is_grantable) AS (SELECT attname::name,privilege_type,'postgres'::regrole::oid,owner_oid,false FROM expected UNION ALL SELECT NULL::name,'DELETE','postgres'::regrole::oid,owner_oid,false),
 delta AS ((SELECT * FROM allactual EXCEPT SELECT * FROM allexpected) UNION ALL (SELECT * FROM allexpected EXCEPT SELECT * FROM allactual)) SELECT count(*) INTO n FROM delta;
 IF n<>0 THEN RAISE EXCEPTION 'native V94 prepared direct ACL is not exact 19-tuple contract'; END IF;
END
$guard$;
REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner;
DO $acl$
BEGIN
 IF (SELECT max(version) FROM public.schema_migrations)=93 THEN
  EXECUTE 'REVOKE SELECT (id, entregada_en, retirar_en, cancelada_en, fallo_terminal_en) ON TABLE public.outbox_evento_local FROM espacigo_schema_owner';
  EXECUTE 'REVOKE UPDATE (id) ON TABLE public.outbox_evento_local FROM espacigo_schema_owner';
  EXECUTE 'REVOKE DELETE ON TABLE public.outbox_evento_local FROM espacigo_schema_owner';
 END IF;
END
$acl$;
DO $verify$
DECLARE owner_oid oid; outbox_oid oid; n bigint; v bigint;
BEGIN
 SELECT max(version) INTO v FROM public.schema_migrations;
 SELECT oid INTO owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner';
 SELECT 'public.outbox_evento_local'::regclass::oid INTO outbox_oid;
 IF pg_catalog.has_schema_privilege(owner_oid,'public','CREATE') THEN RAISE EXCEPTION 'native V94 temporary CREATE authority remains'; END IF;
 -- Exact V90 column baseline remains at ledger 93; no relation-scope grants.
 WITH actual AS (SELECT a.attname,x.privilege_type,x.grantor,x.grantee,x.is_grantable FROM pg_catalog.pg_attribute a CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x WHERE a.attrelid=outbox_oid AND a.attnum>0 AND NOT a.attisdropped AND x.grantee=owner_oid),
 expected(attname,privilege_type,grantor,grantee,is_grantable) AS (SELECT c.attname,'INSERT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('id'),('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('version'),('creada_en'),('disponible_en')) c(attname) UNION ALL SELECT c.attname,'SELECT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion')) c(attname)),
 delta AS ((SELECT * FROM actual EXCEPT SELECT * FROM expected) UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual)) SELECT count(*) INTO n FROM delta;
 IF v=93 AND (n<>0 OR EXISTS (SELECT 1 FROM pg_catalog.aclexplode(COALESCE((SELECT relacl FROM pg_catalog.pg_class WHERE oid=outbox_oid),pg_catalog.acldefault('r'::"char",(SELECT relowner FROM pg_catalog.pg_class WHERE oid=outbox_oid)))) x WHERE x.grantee=owner_oid)) THEN RAISE EXCEPTION 'native V94 ledger93 direct ACL differs from exact V90 baseline'; END IF;
 IF v=94 THEN
 WITH actual AS (SELECT a.attname,x.privilege_type,x.grantor,x.grantee,x.is_grantable FROM pg_catalog.pg_attribute a CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x WHERE a.attrelid=outbox_oid AND a.attnum>0 AND NOT a.attisdropped AND x.grantee=owner_oid),
 expected(attname,privilege_type,grantor,grantee,is_grantable) AS (SELECT c.attname,'INSERT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('id'),('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('version'),('creada_en'),('disponible_en')) c(attname) UNION ALL SELECT c.attname,'SELECT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('id'),('entregada_en'),('retirar_en'),('cancelada_en'),('fallo_terminal_en')) c(attname) UNION ALL SELECT 'id','UPDATE','postgres'::regrole::oid,owner_oid,false),
 tab AS (SELECT x.privilege_type,x.grantor,x.grantee,x.is_grantable FROM pg_catalog.pg_class c CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r'::"char",c.relowner))) x WHERE c.oid=outbox_oid AND x.grantee=owner_oid),
 allactual AS (SELECT attname,privilege_type,grantor,grantee,is_grantable FROM actual UNION ALL SELECT NULL::name,privilege_type,grantor,grantee,is_grantable FROM tab),
 allexpected(attname,privilege_type,grantor,grantee,is_grantable) AS (SELECT attname::name,privilege_type,'postgres'::regrole::oid,owner_oid,false FROM expected UNION ALL SELECT NULL::name,'DELETE','postgres'::regrole::oid,owner_oid,false),
 delta AS ((SELECT * FROM allactual EXCEPT SELECT * FROM allexpected) UNION ALL (SELECT * FROM allexpected EXCEPT SELECT * FROM allactual)) SELECT count(*) INTO n FROM delta;
 IF n<>0 OR NOT pg_catalog.has_table_privilege(owner_oid,outbox_oid,'DELETE') OR NOT pg_catalog.has_column_privilege(owner_oid,outbox_oid,'id','UPDATE') OR EXISTS(SELECT 1 FROM pg_catalog.unnest(ARRAY['id','entregada_en','retirar_en','cancelada_en','fallo_terminal_en']) c(col) WHERE NOT pg_catalog.has_column_privilege(owner_oid,outbox_oid,c.col,'SELECT')) THEN RAISE EXCEPTION 'native V94 final direct ACL or required effective privileges mismatch'; END IF;
 END IF;
 IF pg_catalog.has_table_privilege('espacigo_runtime',outbox_oid,'DELETE') OR pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker',outbox_oid,'DELETE') OR pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker',outbox_oid,'UPDATE') THEN RAISE EXCEPTION 'native V94 runtime DELETE or worker mutation guard failed'; END IF;
END
$verify$;
COMMIT;
