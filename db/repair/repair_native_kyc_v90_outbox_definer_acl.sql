-- One-time least-privilege ACL repair for the V90 SECURITY DEFINER function.
-- This target's historical outbox table is owned by postgres, not schema_owner.
-- Run only on the pinned native database as its local administrator.
BEGIN;
DO $guard$
DECLARE
    v_runtime oid;
    v_owner oid;
BEGIN
    SELECT oid INTO v_runtime FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime';
    SELECT oid INTO v_owner FROM pg_catalog.pg_roles WHERE rolname='espacigo_schema_owner';
    IF current_database() <> 'espacigo_dms_stage_20261020'
       OR (SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database()) <> 16388
       OR current_setting('server_version_num')::integer <> 180006
       OR session_user <> 'postgres' OR current_user <> 'postgres'
       OR v_runtime IS NULL OR v_owner IS NULL THEN
        RAISE EXCEPTION 'native V90 outbox ACL repair target or actor mismatch';
    END IF;
    IF (SELECT count(*) FROM public.schema_migrations) <> 90
       OR (SELECT min(version) FROM public.schema_migrations) <> 1
       OR (SELECT max(version) FROM public.schema_migrations) <> 90
       OR (SELECT checksum FROM public.schema_migrations WHERE version=90)
          IS DISTINCT FROM '23bbacde2cf992f912926f84945c701422b097598e698b2e8f38158b61622592' THEN
        RAISE EXCEPTION 'native V90 outbox ACL repair requires the exact contiguous V1..V90 ledger';
    END IF;
    IF pg_catalog.pg_get_userbyid((SELECT relowner FROM pg_catalog.pg_class WHERE oid='public.outbox_evento_local'::regclass)) <> 'postgres'
       OR NOT EXISTS (
            SELECT 1 FROM pg_catalog.pg_proc p
            WHERE p.oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure
              AND p.proowner=v_owner AND p.prosecdef
              AND p.proconfig @> ARRAY['search_path=pg_catalog']
       )
       OR NOT pg_catalog.has_function_privilege('espacigo_runtime','public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)','EXECUTE')
       OR EXISTS (
            SELECT 1 FROM pg_catalog.aclexplode(COALESCE(
                (SELECT proacl FROM pg_catalog.pg_proc WHERE oid='public.commit_verified_identity_decision(uuid,uuid,text,text,timestamp with time zone)'::regprocedure),
                pg_catalog.acldefault('f'::"char",v_owner))) a
            WHERE a.grantee=0 AND a.privilege_type='EXECUTE'
       ) THEN
        RAISE EXCEPTION 'native V90 definer ownership or execution ACL differs from source';
    END IF;
    IF EXISTS (
        SELECT 1 FROM unnest(ARRAY[
            'clave_deduplicacion','agregado_tipo','agregado_id','tipo'
        ]) AS cols(col)
        WHERE pg_catalog.has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',col,'SELECT')
    ) OR EXISTS (
        SELECT 1 FROM unnest(ARRAY[
            'id','agregado_tipo','agregado_id','tipo','clave_deduplicacion',
            'version','creada_en','disponible_en'
        ]) AS cols(col)
        WHERE pg_catalog.has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',col,'INSERT')
    ) THEN
        RAISE EXCEPTION 'native V90 outbox function columns already have effective schema-owner grants; inspect before repair';
    END IF;
END
$guard$;

GRANT SELECT (clave_deduplicacion, agregado_tipo, agregado_id, tipo)
    ON TABLE public.outbox_evento_local TO espacigo_schema_owner;
GRANT INSERT (id, agregado_tipo, agregado_id, tipo, clave_deduplicacion, version, creada_en, disponible_en)
    ON TABLE public.outbox_evento_local TO espacigo_schema_owner;

DO $verify$
BEGIN
    IF EXISTS (
        SELECT 1 FROM unnest(ARRAY[
            'clave_deduplicacion','agregado_tipo','agregado_id','tipo'
        ]) AS cols(col)
        WHERE NOT pg_catalog.has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',col,'SELECT')
    ) OR EXISTS (
        SELECT 1 FROM unnest(ARRAY[
            'id','agregado_tipo','agregado_id','tipo','clave_deduplicacion',
            'version','creada_en','disponible_en'
        ]) AS cols(col)
        WHERE NOT pg_catalog.has_column_privilege('espacigo_schema_owner','public.outbox_evento_local',col,'INSERT')
    ) OR pg_catalog.has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','DELETE')
       OR pg_catalog.has_table_privilege('espacigo_schema_owner','public.outbox_evento_local','UPDATE') THEN
        RAISE EXCEPTION 'native V90 outbox function minimal ACL readback failed';
    END IF;
    IF EXISTS (
        SELECT 1 FROM information_schema.column_privileges
        WHERE table_schema='public' AND table_name='outbox_evento_local'
          AND grantee='espacigo_schema_owner'
          AND (
              is_grantable <> 'NO'
              OR (privilege_type='SELECT' AND column_name NOT IN ('clave_deduplicacion','agregado_tipo','agregado_id','tipo'))
              OR (privilege_type='INSERT' AND column_name NOT IN ('id','agregado_tipo','agregado_id','tipo','clave_deduplicacion','version','creada_en','disponible_en'))
              OR privilege_type NOT IN ('SELECT','INSERT')
          )
    ) THEN
        RAISE EXCEPTION 'native V90 outbox function ACL is broader than the required columns';
    END IF;
END
$verify$;
COMMIT;
