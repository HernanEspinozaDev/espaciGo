-- GCP06-ROWS-001: bounded aggregate verification of the imported V1-V64 data.
-- Separate from cmd/gcp06sqlinspect Phase A; it reads schema_migrations and
-- application tables. Review/authorize this query and its login before use.
-- No row values, identifiers, emails, hashes, tokens, or credentials are output.
\set ON_ERROR_STOP on

SELECT (
    current_database() = 'espacigo_gcp06_candidate_20261020-gcp06a'
    AND session_user = current_user
    AND current_setting('server_version_num')::integer >= 180000
) AS gcp06_target_ok \gset
\if :gcp06_target_ok
\else
\echo GCP06_STOP_TARGET
\quit
\endif

BEGIN TRANSACTION READ ONLY;
SET LOCAL statement_timeout = '10s';

SELECT jsonb_build_object(
    'query_id', 'GCP06-ROWS-001',
    'run_id', '20261020-gcp06a',
    'database', current_database(),
    'server_major', current_setting('server_version_num')::integer / 10000,
    'session_user', session_user,
    'current_user', current_user,
    'observed_at', to_char(transaction_timestamp() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'),
    'ledger', (
        SELECT jsonb_build_object(
            'rows', count(*)::integer,
            'min_version', min(version)::integer,
            'max_version', max(version)::integer,
            'distinct_versions', count(DISTINCT version)::integer,
            'duplicate_rows', (count(*) - count(DISTINCT version))::integer,
            'contiguous_v1_v64', coalesce(
                count(*) = 64
                AND count(version) = 64
                AND count(DISTINCT version) = 64
                AND min(version) = 1
                AND max(version) = 64,
                false
            )
        )
        FROM public.schema_migrations
    ),
    'staged', jsonb_build_object(
        'migration_count', (SELECT count(*)::integer FROM public.schema_migrations),
        'migration_max', (SELECT max(version)::integer FROM public.schema_migrations),
        'users', (SELECT count(*)::integer FROM public.usuario),
        'spaces', (SELECT count(*)::integer FROM public.espacio),
        'active_spaces', (SELECT count(*)::integer FROM public.espacio WHERE estado = 'activa'),
        'catalog_candidates', (
            SELECT count(*)::integer
            FROM public.espacio e
            JOIN public.categoria_espacio c ON c.codigo = e.categoria_codigo AND c.activa
            JOIN LATERAL (
                SELECT 1 FROM public.tarifa_espacio t0
                WHERE t0.espacio_id = e.id
                ORDER BY t0.version DESC LIMIT 1
            ) t ON true
            JOIN public.usuario u ON u.id = e.propietario_id AND u.estado = 'activo'
            WHERE e.estado = 'activa'
              AND NOT EXISTS (
                  SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b
                  WHERE b.cuenta_id = u.id
              )
              AND EXISTS (
                  SELECT 1
                  FROM public.elegibilidad_verificacion_local el
                  JOIN public.verificacion v
                    ON v.id = el.verificacion_id
                   AND v.usuario_id = el.usuario_id
                   AND v.tipo = el.tipo
                   AND v.estado = 'aprobada'
                  WHERE el.usuario_id = u.id
                    AND el.tipo = 'kyc'
                    AND el.estado = 'elegible'
              )
              AND NOT EXISTS (
                  SELECT 1 FROM public.reserva_ensayo_local_fixture fx
                  WHERE fx.espacio_id = e.id
                    AND fx.anfitrion_id = e.propietario_id
                    AND fx.habilitada
              )
        ),
        'synthetic_kyc', (
            SELECT count(*)::integer FROM public.verificacion
            WHERE clave_idempotencia LIKE 'gcp06-synthetic-kyc-%'
        ),
        'bookings', (SELECT count(*)::integer FROM public.reserva_ensayo_local),
        'active_users', (SELECT count(*)::integer FROM public.usuario WHERE estado = 'activo'),
        'admins', (SELECT count(*)::integer FROM public.rol_usuario WHERE rol = 'administrador'),
        'global_admins', (SELECT count(*)::integer FROM public.administrador_global),
        'disabled_hashes', (
            SELECT count(*)::integer FROM public.usuario
            WHERE hash_clave = '!gcp06-disabled!' AND estado = 'activo'
        )
    )
)::text;

COMMIT;
