-- One-time, F5-only pre-V81 role/schema preparation. As postgres, run:
-- psql -X -P pager=off --single-transaction -v ON_ERROR_STOP=1 -h 127.0.0.1 -p 15434 \
--   -U postgres -d espacigo_f5_gate_20261101 \
--   -f db/repair/f5_pre_v81_role_contract.sql \
--   -c '\password espacigo_api_login' -c '\password espacigo_migrator_login'
-- from a PTY, with the approved PGPASSFILE already selected. PostgreSQL 18's
-- psql docs specify that -1 wraps -f/-c actions in one transaction, options run
-- in order, and ON_ERROR_STOP rolls back on command failure. \password prompts
-- and sends an encrypted ALTER ROLE value; cleartext stays out of files, argv,
-- and URL. Reference: https://www.postgresql.org/docs/18/app-psql.html
-- This prepares only objects required by V81; it does not apply migrations,
-- grant public-schema CREATE, transfer existing ownership, or write the ledger.
SELECT pg_catalog.pg_advisory_xact_lock(1163087943, 1296648018);

DO $pre_v81_target_guard$
BEGIN
    IF current_database() <> 'espacigo_f5_gate_20261101'
       OR session_user <> 'postgres'
       OR current_user <> 'postgres'
       OR current_setting('server_version_num') <> '180006' THEN
        RAISE EXCEPTION 'F5 pre-V81 baseline guard requires the exact PostgreSQL 18.6 target and postgres identity';
    END IF;
END;
$pre_v81_target_guard$;

-- Keep this fingerprint aligned with runtime_acl_f5_baseline.go. pg_temp is
-- excluded from the inventory so this session-only helper cannot change it.
CREATE OR REPLACE FUNCTION pg_temp.f5_pre_v81_public_acl_matches()
RETURNS boolean
LANGUAGE sql STABLE
AS $f5_baseline$
WITH effective_public_acl AS (
    SELECT 'pg_catalog.pg_database'::regclass::oid AS classid, d.oid AS objid,
           0::smallint AS objsubid, a.grantor, 0::oid AS grantee,
           a.privilege_type::text AS privilege_type, a.is_grantable
      FROM pg_catalog.pg_database d
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) a
     WHERE d.datname=pg_catalog.current_database() AND a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_namespace'::regclass::oid, n.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_namespace n
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) a
     WHERE a.grantee=0 AND n.oid<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_class'::regclass::oid, c.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_class c
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault((CASE WHEN c.relkind='S' THEN 's' ELSE 'r' END)::"char",c.relowner))) a
     WHERE a.grantee=0 AND c.relnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_class'::regclass::oid, x.attrelid, x.attnum,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_attribute x
      JOIN pg_catalog.pg_class c ON c.oid=x.attrelid
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(x.attacl,pg_catalog.acldefault('c',c.relowner))) a
     WHERE x.attnum>0 AND NOT x.attisdropped AND a.grantee=0
       AND c.relnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_proc'::regclass::oid, p.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_proc p
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) a
     WHERE a.grantee=0 AND p.pronamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_type'::regclass::oid, t.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_type t
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(t.typacl,pg_catalog.acldefault('T',t.typowner))) a
     WHERE a.grantee=0 AND t.typnamespace<>pg_catalog.pg_my_temp_schema()
    UNION ALL
    SELECT 'pg_catalog.pg_language'::regclass::oid, l.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_language l
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(l.lanacl,pg_catalog.acldefault('l',l.lanowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_foreign_data_wrapper'::regclass::oid, f.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_foreign_data_wrapper f
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(f.fdwacl,pg_catalog.acldefault('F',f.fdwowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_foreign_server'::regclass::oid, s.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_foreign_server s
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(s.srvacl,pg_catalog.acldefault('S',s.srvowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_tablespace'::regclass::oid, s.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_tablespace s
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(s.spcacl,pg_catalog.acldefault('t',s.spcowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_largeobject_metadata'::regclass::oid, l.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_largeobject_metadata l
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(l.lomacl,pg_catalog.acldefault('L',l.lomowner))) a
     WHERE a.grantee=0
    UNION ALL
    SELECT 'pg_catalog.pg_parameter_acl'::regclass::oid, p.oid, 0::smallint,
           a.grantor, 0::oid, a.privilege_type::text, a.is_grantable
      FROM pg_catalog.pg_parameter_acl p
      CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.paracl,pg_catalog.acldefault('p',pg_catalog.to_regrole('postgres')::oid))) a
     WHERE a.grantee=0
), public_fingerprint AS (
    SELECT count(*)::bigint AS row_count,
           pg_catalog.md5(COALESCE(pg_catalog.string_agg(
               classid::text || '|' || objid::text || '|' || objsubid::text || '|' ||
               grantor::text || '|' || privilege_type || '|' || is_grantable::text,
               chr(10) ORDER BY classid,objid,objsubid,grantor,privilege_type,is_grantable),'')) AS fingerprint
      FROM effective_public_acl
), default_fingerprint AS (
    SELECT count(*)::bigint AS row_count,
           pg_catalog.md5(COALESCE(pg_catalog.string_agg(
               defaclrole::text || '|' || defaclnamespace::text || '|' || defaclobjtype::text || '|' || COALESCE(defaclacl::text,'NULL'),
               chr(10) ORDER BY defaclrole,defaclnamespace,defaclobjtype),'')) AS fingerprint
      FROM pg_catalog.pg_default_acl
)
SELECT pg_catalog.current_database()='espacigo_f5_gate_20261101'
   AND current_setting('server_version_num')::integer=180006
   AND public_fingerprint.row_count=5762
   AND public_fingerprint.fingerprint='527e02ed8bf413337ae39caf6e4edaa7'
   AND default_fingerprint.row_count=0
   AND default_fingerprint.fingerprint='d41d8cd98f00b204e9800998ecf8427e'
  FROM public_fingerprint CROSS JOIN default_fingerprint
$f5_baseline$;

DO $pre_v81_baseline_guard$
BEGIN
    IF NOT pg_temp.f5_pre_v81_public_acl_matches() THEN
        RAISE EXCEPTION 'F5 pre-V81 preparation requires the approved PUBLIC/default ACL baseline';
    END IF;
END;
$pre_v81_baseline_guard$;

DO $pre_v81_role_guard$
DECLARE
    history_count bigint;
    history_min bigint;
    history_max bigint;
    history_distinct bigint;
    missing_relations bigint;
    non_postgres_owners bigint;
    unexpected_ledger_acl boolean;
BEGIN
    IF current_database() <> 'espacigo_f5_gate_20261101'
       OR session_user <> 'postgres'
       OR current_user <> 'postgres'
       OR current_setting('server_version_num') <> '180006' THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation requires the exact PostgreSQL 18.6 target and postgres identity';
    END IF;

    IF to_regclass('public.schema_migrations') IS NULL THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation requires the existing migration ledger';
    END IF;
    SELECT count(*), min(version), max(version), count(DISTINCT version)
      INTO history_count, history_min, history_max, history_distinct
      FROM public.schema_migrations;
    IF history_count <> 80 OR history_min <> 1 OR history_max <> 80 OR history_distinct <> 80
       OR EXISTS (SELECT 1 FROM public.schema_migrations WHERE version >= 81) THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation requires an exact V1-V80 ledger';
    END IF;

    SELECT count(*) FILTER (WHERE c.oid IS NULL),
           count(*) FILTER (WHERE c.oid IS NOT NULL AND owner.rolname <> 'postgres')
      INTO missing_relations, non_postgres_owners
      FROM (VALUES ('public.schema_migrations'), ('public.usuario'), ('public.espacio')) AS required(name)
      LEFT JOIN pg_catalog.pg_class c ON c.oid = to_regclass(required.name)
      LEFT JOIN pg_catalog.pg_roles owner ON owner.oid = c.relowner;
    IF missing_relations <> 0 OR non_postgres_owners <> 0 THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation requires postgres-owned ledger, usuario, and espacio relations';
    END IF;

    WITH actual AS (
        SELECT acl.grantee, acl.grantor, acl.privilege_type, acl.is_grantable
          FROM pg_catalog.pg_class c
          CROSS JOIN LATERAL pg_catalog.aclexplode(
              COALESCE(c.relacl, pg_catalog.acldefault('r', c.relowner))
          ) acl
         WHERE c.oid = 'public.schema_migrations'::regclass
    ), expected AS (
        SELECT acl.grantee, acl.grantor, acl.privilege_type, acl.is_grantable
          FROM pg_catalog.pg_class c
          CROSS JOIN LATERAL pg_catalog.aclexplode(
              pg_catalog.acldefault('r', c.relowner)
          ) acl
         WHERE c.oid = 'public.schema_migrations'::regclass
    )
    SELECT EXISTS (SELECT * FROM actual EXCEPT ALL SELECT * FROM expected)
        OR EXISTS (SELECT * FROM expected EXCEPT ALL SELECT * FROM actual)
      INTO unexpected_ledger_acl;
    IF unexpected_ledger_acl THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation refuses unexpected direct ACL on migration ledger';
    END IF;
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl) THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation refuses unexpected default ACL state';
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_catalog.pg_roles
         WHERE rolname = ANY (ARRAY[
             'espacigo_runtime', 'espacigo_api_login', 'espacigo_schema_owner',
             'espacigo_migrator_login', 'espacigo_document_verification_fn'
         ])
    ) OR EXISTS (
        SELECT 1 FROM pg_catalog.pg_namespace WHERE nspname = 'document_verification_private'
    ) THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation refuses pre-existing application roles or helper schema';
    END IF;
END;
$pre_v81_role_guard$;

CREATE ROLE espacigo_runtime
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_api_login
    LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_schema_owner
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_migrator_login
    LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;
CREATE ROLE espacigo_document_verification_fn
    NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS NOINHERIT;

CREATE SCHEMA document_verification_private AUTHORIZATION espacigo_document_verification_fn;

GRANT espacigo_runtime TO espacigo_api_login
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_schema_owner TO espacigo_migrator_login
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;
GRANT espacigo_document_verification_fn TO espacigo_schema_owner
    WITH ADMIN FALSE, INHERIT FALSE, SET TRUE;

GRANT SELECT (id, estado), UPDATE (id)
    ON TABLE public.usuario TO espacigo_document_verification_fn;
GRANT SELECT (id, propietario_id), UPDATE (id)
    ON TABLE public.espacio TO espacigo_document_verification_fn;
GRANT SELECT, INSERT ON TABLE public.schema_migrations TO espacigo_schema_owner;

DO $pre_v81_ledger_acl_postcondition$
DECLARE
    ledger_owner_oid oid;
    schema_owner_oid oid;
    differs boolean;
BEGIN
    SELECT c.relowner, owner_role.oid
      INTO ledger_owner_oid, schema_owner_oid
      FROM pg_catalog.pg_class c
      JOIN pg_catalog.pg_roles owner_role ON owner_role.rolname='espacigo_schema_owner'
     WHERE c.oid='public.schema_migrations'::regclass;

    WITH actual AS (
        SELECT acl.grantee, acl.grantor, acl.privilege_type, acl.is_grantable
          FROM pg_catalog.pg_class c
          CROSS JOIN LATERAL pg_catalog.aclexplode(
              COALESCE(c.relacl, pg_catalog.acldefault('r', c.relowner))
          ) acl
         WHERE c.oid='public.schema_migrations'::regclass
    ), expected AS (
        SELECT acl.grantee, acl.grantor, acl.privilege_type, acl.is_grantable
          FROM pg_catalog.pg_class c
          CROSS JOIN LATERAL pg_catalog.aclexplode(
              pg_catalog.acldefault('r', c.relowner)
          ) acl
         WHERE c.oid='public.schema_migrations'::regclass
        UNION ALL
        SELECT schema_owner_oid, ledger_owner_oid, 'SELECT'::text, false
        UNION ALL
        SELECT schema_owner_oid, ledger_owner_oid, 'INSERT'::text, false
    )
    SELECT EXISTS (SELECT * FROM actual EXCEPT ALL SELECT * FROM expected)
        OR EXISTS (SELECT * FROM expected EXCEPT ALL SELECT * FROM actual)
      INTO differs;

    IF differs
       OR EXISTS (SELECT 1 FROM pg_catalog.pg_default_acl)
       OR EXISTS (
           SELECT 1 FROM pg_catalog.pg_attribute
            WHERE attrelid='public.schema_migrations'::regclass
              AND attnum > 0 AND NOT attisdropped AND attacl IS NOT NULL
       ) THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation produced an unexpected migration-ledger ACL contract';
    END IF;
END;
$pre_v81_ledger_acl_postcondition$;

DO $post_v81_baseline_guard$
BEGIN
    IF NOT pg_temp.f5_pre_v81_public_acl_matches() THEN
        RAISE EXCEPTION 'F5 pre-V81 role preparation changed the approved PUBLIC/default ACL baseline';
    END IF;
END;
$post_v81_baseline_guard$;
