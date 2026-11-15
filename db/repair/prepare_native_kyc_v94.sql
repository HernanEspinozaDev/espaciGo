-- Native V94 temporary authority window; no database execution is authorized here.
-- CREATE is temporary. DELETE is table-wide and intentionally persists at V94
-- so the SECURITY DEFINER retirement function can delete only its bounded rows.
BEGIN;
SELECT pg_catalog.pg_advisory_xact_lock(x'45535047'::int,x'4d494752'::int);
DO $guard$
DECLARE owner_oid oid; outbox_oid oid; migrator_oid oid; worker_oid oid; n bigint;
BEGIN
 IF current_database()<>'espacigo_dms_stage_20261020'
 OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database())<>16388
 OR current_setting('server_version_num')<>'180006'
 OR session_user<>'postgres' OR current_user<>'postgres' THEN
  RAISE EXCEPTION 'native V94 preparation target/administrator mismatch';
 END IF;
 IF (SELECT count(*) FROM public.schema_migrations)<>93
 OR (SELECT min(version) FROM public.schema_migrations)<>1
 OR (SELECT max(version) FROM public.schema_migrations)<>93
 OR (SELECT checksum FROM public.schema_migrations WHERE version=93) IS DISTINCT FROM '885d3f926ffc7d3420f2391a257ca2ed68663e71d4cd54f4cf310c79bd8b7292' THEN
  RAISE EXCEPTION 'native V94 requires exact contiguous V1..V93 ledger';
 END IF;
 SELECT oid INTO owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 SELECT oid INTO migrator_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_migrator_login'
 AND rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 SELECT oid INTO worker_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_cleanup_worker'
 AND NOT rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole
 AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 SELECT 'public.outbox_evento_local'::regclass::oid INTO outbox_oid;
 IF owner_oid IS NULL OR migrator_oid IS NULL OR worker_oid IS NULL OR outbox_oid IS NULL
 OR (SELECT relowner FROM pg_catalog.pg_class WHERE oid=outbox_oid)<>'postgres'::regrole
 OR pg_catalog.has_schema_privilege('espacigo_schema_owner','public','CREATE') THEN
  RAISE EXCEPTION 'native V94 owner, relation owner, or CREATE baseline mismatch';
 END IF;
 IF (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=owner_oid OR m.member=owner_oid)<>2
 OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE (m.roleid=owner_oid OR m.member=owner_oid)
   AND NOT (m.grantor='postgres'::regrole AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option
     AND ((m.roleid=owner_oid AND m.member=migrator_oid)
       OR (m.roleid='espacigo_document_verification_fn'::regrole AND m.member=owner_oid)))) THEN
  RAISE EXCEPTION 'native V94 schema-owner membership contract mismatch';
 END IF;
 IF EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE (m.roleid=worker_oid OR m.member=worker_oid)
   AND NOT (m.roleid=worker_oid AND m.member=COALESCE((SELECT oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_privacy_worker_login' AND rolcanlogin AND NOT rolsuper AND NOT rolinherit AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls),0::oid)
   AND NOT m.admin_option AND NOT m.inherit_option AND m.set_option)) THEN
  RAISE EXCEPTION 'native V94 cleanup-worker membership contract mismatch';
 END IF;
 -- Exact direct baseline: V90's 8 INSERT and 4 SELECT column ACL tuples only.
 WITH actual AS (
   SELECT a.attname, x.privilege_type, x.grantor, x.grantee, x.is_grantable
   FROM pg_catalog.pg_attribute a CROSS JOIN LATERAL pg_catalog.aclexplode(
     CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x
   WHERE a.attrelid=outbox_oid AND a.attnum>0 AND NOT a.attisdropped AND x.grantee=owner_oid
 ), expected(attname,privilege_type,grantor,grantee,is_grantable) AS (
   SELECT c.attname, p.privilege_type, 'postgres'::regrole::oid grantor, owner_oid grantee, false is_grantable
   FROM (VALUES ('id'),('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('version'),('creada_en'),('disponible_en')) c(attname)
   CROSS JOIN (VALUES ('INSERT')) p(privilege_type)
   UNION ALL
   SELECT c.attname,'SELECT','postgres'::regrole::oid,owner_oid,false
   FROM (VALUES ('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion')) c(attname)
 ), delta AS ((SELECT * FROM actual EXCEPT SELECT * FROM expected) UNION ALL (SELECT * FROM expected EXCEPT SELECT * FROM actual))
 SELECT count(*) INTO n FROM delta;
 IF n<>0 OR EXISTS (SELECT 1 FROM pg_catalog.aclexplode(COALESCE((SELECT relacl FROM pg_catalog.pg_class WHERE oid=outbox_oid),pg_catalog.acldefault('r'::"char",(SELECT relowner FROM pg_catalog.pg_class WHERE oid=outbox_oid)))) x WHERE x.grantee=owner_oid)
 OR pg_catalog.has_table_privilege('espacigo_runtime',outbox_oid,'DELETE')
 OR pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker',outbox_oid,'DELETE')
 OR pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker',outbox_oid,'UPDATE') THEN
  RAISE EXCEPTION 'native V94 preparation refuses direct ACL or runtime/worker mutation drift';
 END IF;
END
$guard$;
GRANT CREATE ON SCHEMA public TO espacigo_schema_owner;
GRANT SELECT (id, entregada_en, retirar_en, cancelada_en, fallo_terminal_en)
 ON TABLE public.outbox_evento_local TO espacigo_schema_owner;
GRANT UPDATE (id) ON TABLE public.outbox_evento_local TO espacigo_schema_owner;
GRANT DELETE ON TABLE public.outbox_evento_local TO espacigo_schema_owner;
DO $verify$
DECLARE owner_oid oid; outbox_oid oid; n bigint;
BEGIN
 SELECT oid INTO owner_oid FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner';
 SELECT 'public.outbox_evento_local'::regclass::oid INTO outbox_oid;
 WITH actual AS (
   SELECT a.attname,x.privilege_type,x.grantor,x.grantee,x.is_grantable
   FROM pg_catalog.pg_attribute a CROSS JOIN LATERAL pg_catalog.aclexplode(CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::aclitem[] END) x
   WHERE a.attrelid=outbox_oid AND a.attnum>0 AND NOT a.attisdropped AND x.grantee=owner_oid
 ), expected(attname,privilege_type,grantor,grantee,is_grantable) AS (
   SELECT c.attname,p.privilege_type,'postgres'::regrole::oid,owner_oid,false
   FROM (VALUES ('id'),('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion'),('version'),('creada_en'),('disponible_en')) c(attname) CROSS JOIN (VALUES ('INSERT')) p(privilege_type)
   UNION ALL SELECT c.attname,'SELECT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('agregado_tipo'),('agregado_id'),('tipo'),('clave_deduplicacion')) c(attname)
   UNION ALL SELECT c.attname,'SELECT','postgres'::regrole::oid,owner_oid,false FROM (VALUES ('id'),('entregada_en'),('retirar_en'),('cancelada_en'),('fallo_terminal_en')) c(attname)
   UNION ALL SELECT 'id','UPDATE','postgres'::regrole::oid,owner_oid,false
 ), tab AS (SELECT x.privilege_type,x.grantor,x.grantee,x.is_grantable FROM pg_catalog.pg_class c CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('r'::"char",c.relowner))) x WHERE c.oid=outbox_oid AND x.grantee=owner_oid),
 allactual AS (SELECT attname,privilege_type,grantor,grantee,is_grantable FROM actual UNION ALL SELECT NULL::name,privilege_type,grantor,grantee,is_grantable FROM tab),
 allexpected AS (SELECT attname::name,privilege_type,'postgres'::regrole::oid,owner_oid,false FROM expected UNION ALL SELECT NULL::name,'DELETE','postgres'::regrole::oid,owner_oid,false),
 delta AS ((SELECT * FROM allactual EXCEPT SELECT * FROM allexpected) UNION ALL (SELECT * FROM allexpected EXCEPT SELECT * FROM allactual))
 SELECT count(*) INTO n FROM delta;
 IF n<>0 OR NOT pg_catalog.has_schema_privilege(owner_oid,'public','CREATE') THEN RAISE EXCEPTION 'native V94 prepared direct ACL is not exact 19-tuple contract'; END IF;
 IF pg_catalog.has_table_privilege('espacigo_runtime',outbox_oid,'DELETE')
 OR pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker',outbox_oid,'DELETE') OR pg_catalog.has_table_privilege('espacigo_privacy_cleanup_worker',outbox_oid,'UPDATE') THEN RAISE EXCEPTION 'native V94 runtime/worker mutation guard failed'; END IF;
END
$verify$;
COMMIT;
