-- GCP-06 local staging only. Apply after restoring a V1-V61 logical snapshot
-- into a disposable database; never apply to the source or a Cloud database.
\set ON_ERROR_STOP on

BEGIN;
SET CONSTRAINTS ALL DEFERRED;

DO $guard$
DECLARE
    history_rows bigint;
    history_max bigint;
BEGIN
    IF current_database() !~ '^espacigo_gcp06_stage(_[a-z0-9-]+)?$' THEN
        RAISE EXCEPTION 'refusing GCP-06 sanitizer outside a named staging database';
    END IF;
    IF to_regclass('public.schema_migrations') IS NULL OR to_regclass('public.usuario') IS NULL THEN
        RAISE EXCEPTION 'staging database is missing V1-V61 schema objects';
    END IF;
    SELECT count(*), max(version) INTO history_rows, history_max FROM public.schema_migrations;
    IF history_rows <> 61 OR history_max <> 61 THEN
        RAISE EXCEPTION 'staging schema must be exactly V1-V61';
    END IF;
    IF EXISTS (
        SELECT 1 FROM pg_inherits i
        JOIN pg_class c ON c.oid=i.inhrelid
        JOIN pg_namespace n ON n.oid=c.relnamespace
        WHERE n.nspname='public'
    ) THEN
        RAISE EXCEPTION 'staging sanitizer requires a non-partitioned public schema';
    END IF;
END
$guard$;

CREATE TEMP TABLE gcp06_tainted_row (
    relid oid NOT NULL,
    row_tid tid NOT NULL,
    PRIMARY KEY (relid, row_tid)
) ON COMMIT DROP;

-- Only deterministic dataset accounts enter the copied snapshot. Global
-- administrators, local test identities and retained privacy/audit work are
-- removed together with every row that references them.
INSERT INTO pg_temp.gcp06_tainted_row(relid, row_tid)
SELECT 'public.usuario'::regclass, u.ctid
FROM public.usuario u
WHERE u.correo_normalizado !~ '^u[0-9]{4}@dataset[.]invalid$'
   OR EXISTS (SELECT 1 FROM public.administrador_global ag WHERE ag.usuario_id=u.id)
   OR EXISTS (SELECT 1 FROM public.administrador_global_bootstrap b WHERE b.usuario_id=u.id);

DO $seed_sensitive$
DECLARE table_name text;
BEGIN
    FOREACH table_name IN ARRAY ARRAY[
        'sesion','token_accion','historial_clave_local','verificacion',
        'aviso_local','aviso_local_ciclo','outbox_evento_local','outbox_evento_ciclo_local',
        'evento_auditoria_local','solicitud_titular','ejecucion_baja_local',
        'baja_archivo_pendiente_local','reaplicacion_baja_local',
        'verificacion_archivo_retiro_pendiente_local','administrador_global',
        'administrador_global_bootstrap','administrador_global_bootstrap_evento',
        'bloqueo_cuenta_administrativo_local','reserva_ensayo_local_fixture',
        'resena_autoria_marca_local'
    ] LOOP
        IF to_regclass('public.' || table_name) IS NOT NULL THEN
            EXECUTE format(
                'INSERT INTO pg_temp.gcp06_tainted_row(relid,row_tid) SELECT %L::regclass, t.ctid FROM ONLY public.%I t ON CONFLICT DO NOTHING',
                'public.' || table_name, table_name);
        END IF;
    END LOOP;
    IF to_regclass('public.rol_usuario') IS NOT NULL THEN
        INSERT INTO pg_temp.gcp06_tainted_row(relid,row_tid)
        SELECT 'public.rol_usuario'::regclass, ru.ctid FROM public.rol_usuario ru
        WHERE ru.rol='administrador' ON CONFLICT DO NOTHING;
    END IF;
END
$seed_sensitive$;

-- Compute the transitive foreign-key closure. A row is tainted whenever it
-- points to a tainted parent, so mixed-participant records are discarded too.
DO $taint_closure$
DECLARE
    fk record;
    parent_columns text;
    child_columns text;
    statement text;
    inserted bigint;
    pass_inserted bigint;
BEGIN
    LOOP
        pass_inserted := 0;
        FOR fk IN
            SELECT c.oid AS child_oid, c.conrelid, c.confrelid,
                   c.conkey, c.confkey,
                   format('%I.%I', child_ns.nspname, child.relname) AS child_name,
                   format('%I.%I', parent_ns.nspname, parent.relname) AS parent_name
            FROM pg_constraint c
            JOIN pg_class child ON child.oid=c.conrelid AND child.relkind='r'
            JOIN pg_namespace child_ns ON child_ns.oid=child.relnamespace AND child_ns.nspname='public'
            JOIN pg_class parent ON parent.oid=c.confrelid AND parent.relkind='r'
            JOIN pg_namespace parent_ns ON parent_ns.oid=parent.relnamespace AND parent_ns.nspname='public'
            WHERE c.contype='f'
            ORDER BY c.oid
        LOOP
            SELECT string_agg(format('p.%I', a.attname), ',' ORDER BY k.ordinality)
              INTO parent_columns
              FROM unnest(fk.confkey) WITH ORDINALITY k(attnum, ordinality)
              JOIN pg_attribute a ON a.attrelid=fk.confrelid AND a.attnum=k.attnum;
            SELECT string_agg(format('c.%I', a.attname), ',' ORDER BY k.ordinality)
              INTO child_columns
              FROM unnest(fk.conkey) WITH ORDINALITY k(attnum, ordinality)
              JOIN pg_attribute a ON a.attrelid=fk.conrelid AND a.attnum=k.attnum;
            statement := format(
                'INSERT INTO pg_temp.gcp06_tainted_row(relid,row_tid) '
                'SELECT %L::regclass,c.ctid FROM ONLY %s c '
                'WHERE EXISTS (SELECT 1 FROM ONLY %s p '
                'JOIN pg_temp.gcp06_tainted_row t ON t.relid=%L::regclass AND t.row_tid=p.ctid '
                'WHERE jsonb_build_array(%s)=jsonb_build_array(%s)) ON CONFLICT DO NOTHING',
                'public.' || (SELECT relname FROM pg_class WHERE oid=fk.conrelid),
                fk.child_name, fk.parent_name,
                'public.' || (SELECT relname FROM pg_class WHERE oid=fk.confrelid),
                parent_columns, child_columns);
            EXECUTE statement;
            GET DIAGNOSTICS inserted = ROW_COUNT;
            pass_inserted := pass_inserted + inserted;
        END LOOP;
        EXIT WHEN pass_inserted=0;
    END LOOP;
END
$taint_closure$;

-- Delete leaf rows first, respecting immediate foreign keys. Remaining cycles
-- are deleted in one deferred transaction; a non-deferrable cycle aborts all.
DO $delete_tainted$
DECLARE
    rel record;
    fk record;
    parent_columns text;
    child_columns text;
    predicates text;
    statement text;
    deleted bigint;
    pass_deleted bigint;
BEGIN
    LOOP
        pass_deleted := 0;
        FOR rel IN
            SELECT DISTINCT t.relid, n.nspname, c.relname
            FROM pg_temp.gcp06_tainted_row t
            JOIN pg_class c ON c.oid=t.relid
            JOIN pg_namespace n ON n.oid=c.relnamespace
            ORDER BY c.relname
        LOOP
            predicates := '';
            FOR fk IN
                SELECT conkey, confkey, conrelid, confrelid,
                       format('%I.%I', child_ns.nspname, child.relname) AS child_name
                FROM pg_constraint f
                JOIN pg_class child ON child.oid=f.conrelid AND child.relkind='r'
                JOIN pg_namespace child_ns ON child_ns.oid=child.relnamespace AND child_ns.nspname='public'
                WHERE f.contype='f' AND f.confrelid=rel.relid
                ORDER BY f.oid
            LOOP
                SELECT string_agg(format('p.%I', a.attname), ',' ORDER BY k.ordinality)
                  INTO parent_columns
                  FROM unnest(fk.confkey) WITH ORDINALITY k(attnum, ordinality)
                  JOIN pg_attribute a ON a.attrelid=fk.confrelid AND a.attnum=k.attnum;
                SELECT string_agg(format('c.%I', a.attname), ',' ORDER BY k.ordinality)
                  INTO child_columns
                  FROM unnest(fk.conkey) WITH ORDINALITY k(attnum, ordinality)
                  JOIN pg_attribute a ON a.attrelid=fk.conrelid AND a.attnum=k.attnum;
                predicates := predicates || format(
                    ' AND NOT EXISTS (SELECT 1 FROM ONLY %s c WHERE jsonb_build_array(%s)=jsonb_build_array(%s))',
                    fk.child_name, child_columns, parent_columns);
            END LOOP;
            statement := format(
                'WITH deleted_rows AS (' ||
                ' DELETE FROM ONLY %I.%I p USING pg_temp.gcp06_tainted_row t' ||
                ' WHERE t.relid=%L::regclass AND t.row_tid=p.ctid%s RETURNING p.ctid AS row_tid' ||
                ') DELETE FROM pg_temp.gcp06_tainted_row t' ||
                ' WHERE t.relid=%L::regclass AND t.row_tid IN (SELECT row_tid FROM deleted_rows)',
                rel.nspname, rel.relname, 'public.' || rel.relname, predicates,
                'public.' || rel.relname);
            EXECUTE statement;
            GET DIAGNOSTICS deleted = ROW_COUNT;
            pass_deleted := pass_deleted + deleted;
        END LOOP;
        EXIT WHEN pass_deleted=0;
    END LOOP;
    IF EXISTS (SELECT 1 FROM pg_temp.gcp06_tainted_row) THEN
        -- Force constraint validation at transaction end for any deferred cycle.
        FOR rel IN
            SELECT DISTINCT t.relid, n.nspname, c.relname
            FROM pg_temp.gcp06_tainted_row t
            JOIN pg_class c ON c.oid=t.relid
            JOIN pg_namespace n ON n.oid=c.relnamespace
        LOOP
            EXECUTE format(
                'WITH deleted_rows AS (' ||
                ' DELETE FROM ONLY %I.%I p USING pg_temp.gcp06_tainted_row t' ||
                ' WHERE t.relid=%L::regclass AND t.row_tid=p.ctid RETURNING p.ctid AS row_tid' ||
                ') DELETE FROM pg_temp.gcp06_tainted_row t' ||
                ' WHERE t.relid=%L::regclass AND t.row_tid IN (SELECT row_tid FROM deleted_rows)',
                rel.nspname, rel.relname, 'public.' || rel.relname,
                'public.' || rel.relname);
        END LOOP;
    END IF;
    IF EXISTS (SELECT 1 FROM pg_temp.gcp06_tainted_row) THEN
        RAISE EXCEPTION 'GCP-06 staging taint closure could not be removed';
    END IF;
END
$delete_tainted$;

-- Remove account identifiers and password verifiers from the candidate while
-- retaining the user UUIDs required by synthetic marketplace relationships.
UPDATE public.usuario
SET correo_original='u-' || replace(id::text,'-','') || '@transfer.invalid',
    correo_normalizado='u-' || replace(id::text,'-','') || '@transfer.invalid',
    hash_clave='!gcp06-disabled!',
    baja_solicitada_en=NULL,
    intentos_fallidos_consecutivos=0,
    bloqueado_hasta=NULL,
    actualizado_en=transaction_timestamp();

UPDATE public.perfil_usuario
SET nombre_visible='Usuario Sintetico', telefono_normalizado=NULL,
    actualizado_en=transaction_timestamp();

-- Public-catalog rehearsals need a KYC gate, but copied verification facts,
-- evidence references and reviewer identities are sensitive. Reconstruct a
-- fixture-only approval for active synthetic listing owners. This marker is
-- not a real verification and may only be imported into an allowlisted,
-- isolated GCP-06 candidate.
DO $synthetic_public_eligibility$
DECLARE
    reviewer_id constant uuid := 'fffffff0-0000-4000-8000-000000000064';
BEGIN
    IF to_regclass('public.verificacion') IS NULL
       OR to_regclass('public.elegibilidad_verificacion_local') IS NULL
       OR NOT EXISTS (SELECT 1 FROM information_schema.columns
                      WHERE table_schema='public' AND table_name='espacio' AND column_name='estado') THEN
        RETURN;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM public.espacio e JOIN public.usuario u ON u.id=e.propietario_id
        WHERE e.estado='activa' AND u.estado='activo'
    ) THEN
        RETURN;
    END IF;

    INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado)
    VALUES(reviewer_id,'gcp06-reviewer@transfer.invalid','gcp06-reviewer@transfer.invalid','!gcp06-disabled!','activo');
    INSERT INTO public.perfil_usuario(usuario_id,nombre_visible)
    VALUES(reviewer_id,'Revisor Sintetico');

    INSERT INTO public.verificacion(
        id,usuario_id,tipo,estado,proveedor_ref,referencia_evidencia,
        clave_idempotencia,revisor_id,resuelta_en
    )
    SELECT ('00000000-0000-4000-8000-' || right(md5('gcp06-kyc:' || owner.id::text),12))::uuid,
           owner.id,'kyc','aprobada','local-fixture-v1',
           'fixture:' || ('00000000-0000-4000-8000-' || right(md5('gcp06-evidence:' || owner.id::text),12)),
           'gcp06-synthetic-kyc-' || replace(owner.id::text,'-',''),
           reviewer_id,transaction_timestamp()
    FROM (
        SELECT DISTINCT e.propietario_id AS id
        FROM public.espacio e JOIN public.usuario u ON u.id=e.propietario_id
        WHERE e.estado='activa' AND u.estado='activo'
    ) owner;

    INSERT INTO public.elegibilidad_verificacion_local(
        usuario_id,tipo,verificacion_id,estado,concedida_en
    )
    SELECT v.usuario_id,'kyc',v.id,'elegible',transaction_timestamp()
    FROM public.verificacion v
    WHERE v.proveedor_ref='local-fixture-v1'
      AND v.clave_idempotencia LIKE 'gcp06-synthetic-kyc-%';
END
$synthetic_public_eligibility$;

DO $postconditions$
DECLARE
    non_fixture_verifications boolean;
BEGIN
    IF EXISTS (SELECT 1 FROM public.rol_usuario WHERE rol='administrador')
       OR EXISTS (SELECT 1 FROM public.administrador_global)
       OR EXISTS (SELECT 1 FROM public.administrador_global_bootstrap)
       OR EXISTS (SELECT 1 FROM public.sesion)
       OR EXISTS (SELECT 1 FROM public.token_accion)
       OR EXISTS (SELECT 1 FROM public.usuario WHERE correo_normalizado !~ '^(u-[0-9a-f]{32}@transfer[.]invalid|gcp06-reviewer@transfer[.]invalid)$'
                    OR hash_clave <> '!gcp06-disabled!' OR estado <> 'activo') THEN
        RAISE EXCEPTION 'GCP-06 staging sanitizer postcondition failed';
    END IF;
    IF to_regclass('public.verificacion') IS NOT NULL THEN
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM public.verificacion
                  WHERE proveedor_ref <> ''local-fixture-v1''
                     OR referencia_evidencia !~ ''^fixture:[0-9a-f-]{36}$''
                     OR clave_idempotencia NOT LIKE ''gcp06-synthetic-kyc-%'')'
          INTO non_fixture_verifications;
        IF non_fixture_verifications THEN
            RAISE EXCEPTION 'GCP-06 staging retained non-fixture verification data';
        END IF;
    END IF;
END
$postconditions$;

COMMIT;
