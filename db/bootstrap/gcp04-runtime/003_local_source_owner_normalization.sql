-- LOCAL BASELINE SOURCE ONLY. Run only after canonical V1-V64 completed.
-- This changes ownership of application objects after the canonical replay;
-- it never changes migration files or ledger rows and is not a Cloud SQL plan.
\set ON_ERROR_STOP on

BEGIN;
DO $ownership$
DECLARE
    object_row record;
    alter_kind text;
BEGIN
    IF current_database() <> 'espacigo' THEN
        RAISE EXCEPTION 'refusing local owner normalization in unexpected database %', current_database();
    END IF;

    FOR object_row IN
        SELECT c.relkind, n.nspname, c.relname
        FROM pg_catalog.pg_class c
        JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = 'public'
          AND c.relkind IN ('r', 'p', 'v', 'm', 'f', 'S')
          AND c.relowner = 'postgres'::regrole
          AND NOT EXISTS (
              SELECT 1 FROM pg_catalog.pg_depend d
              WHERE d.classid = 'pg_catalog.pg_class'::regclass
                AND d.objid = c.oid AND d.deptype = 'e'
          )
          AND NOT EXISTS (
              SELECT 1 FROM pg_catalog.pg_depend d
              WHERE c.relkind = 'S'
                AND d.classid = 'pg_catalog.pg_class'::regclass
                AND d.objid = c.oid
                AND d.refclassid = 'pg_catalog.pg_class'::regclass
                AND d.deptype IN ('a', 'i')
          )
        ORDER BY c.relkind, c.relname
    LOOP
        alter_kind := CASE object_row.relkind
            WHEN 'S' THEN 'SEQUENCE'
            WHEN 'v' THEN 'VIEW'
            WHEN 'm' THEN 'MATERIALIZED VIEW'
            WHEN 'f' THEN 'FOREIGN TABLE'
            ELSE 'TABLE'
        END;
        EXECUTE format('ALTER %s %I.%I OWNER TO espacigo_schema_owner', alter_kind, object_row.nspname, object_row.relname);
    END LOOP;

    FOR object_row IN
        SELECT p.prokind, n.nspname, p.proname, pg_catalog.pg_get_function_identity_arguments(p.oid) AS identity_arguments
        FROM pg_catalog.pg_proc p
        JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
        WHERE n.nspname = 'public'
          AND p.proowner = 'postgres'::regrole
          AND NOT EXISTS (
              SELECT 1 FROM pg_catalog.pg_depend d
              WHERE d.classid = 'pg_catalog.pg_proc'::regclass
                AND d.objid = p.oid AND d.deptype = 'e'
          )
        ORDER BY p.proname, identity_arguments
    LOOP
        alter_kind := CASE object_row.prokind
            WHEN 'p' THEN 'PROCEDURE'
            WHEN 'a' THEN 'AGGREGATE'
            ELSE 'FUNCTION'
        END;
        EXECUTE format('ALTER %s %I.%I(%s) OWNER TO espacigo_schema_owner', alter_kind,
                       object_row.nspname, object_row.proname, object_row.identity_arguments);
    END LOOP;

    FOR object_row IN
        SELECT t.typtype, n.nspname, t.typname
        FROM pg_catalog.pg_type t
        JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
        WHERE n.nspname = 'public'
          AND t.typowner = 'postgres'::regrole
          AND t.typrelid = 0
          AND t.typtype IN ('b', 'd', 'e', 'r', 'm')
          AND NOT EXISTS (
              SELECT 1 FROM pg_catalog.pg_depend d
              WHERE d.classid = 'pg_catalog.pg_type'::regclass
                AND d.objid = t.oid AND d.deptype = 'e'
          )
        ORDER BY t.typname
    LOOP
        alter_kind := CASE object_row.typtype WHEN 'd' THEN 'DOMAIN' ELSE 'TYPE' END;
        EXECUTE format('ALTER %s %I.%I OWNER TO espacigo_schema_owner', alter_kind, object_row.nspname, object_row.typname);
    END LOOP;

    FOR object_row IN
        SELECT d.dictname AS object_name
        FROM pg_catalog.pg_ts_dict d
        JOIN pg_catalog.pg_namespace n ON n.oid = d.dictnamespace
        WHERE n.nspname = 'public' AND d.dictowner = 'postgres'::regrole
        ORDER BY d.dictname
    LOOP
        EXECUTE format('ALTER TEXT SEARCH DICTIONARY public.%I OWNER TO espacigo_schema_owner', object_row.object_name);
    END LOOP;

    FOR object_row IN
        SELECT c.cfgname AS object_name
        FROM pg_catalog.pg_ts_config c
        JOIN pg_catalog.pg_namespace n ON n.oid = c.cfgnamespace
        WHERE n.nspname = 'public' AND c.cfgowner = 'postgres'::regrole
        ORDER BY c.cfgname
    LOOP
        EXECUTE format('ALTER TEXT SEARCH CONFIGURATION public.%I OWNER TO espacigo_schema_owner', object_row.object_name);
    END LOOP;
END
$ownership$;
COMMIT;
