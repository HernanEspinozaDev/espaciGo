-- ACL reproducible for the isolated F6 synthetic cluster only.
-- Apply as the cluster administrator after verifying the exact database.
-- The API login is deliberately not a superuser and receives no role memberships.
DO $$
BEGIN
    IF current_database() <> 'espacigo_f6_synth_20261028' THEN
        RAISE EXCEPTION 'wrong synthetic F6 database';
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname = 'espacigo_f6_f3_api') THEN
        CREATE ROLE espacigo_f6_f3_api LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
    END IF;
END
$$;
ALTER ROLE espacigo_f6_f3_api LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS INHERIT;
REVOKE ALL PRIVILEGES ON DATABASE espacigo_f6_synth_20261028 FROM espacigo_f6_f3_api;
GRANT CONNECT ON DATABASE espacigo_f6_synth_20261028 TO espacigo_f6_f3_api;
REVOKE CREATE, TEMPORARY ON DATABASE espacigo_f6_synth_20261028 FROM espacigo_f6_f3_api;
REVOKE ALL PRIVILEGES ON SCHEMA public FROM espacigo_f6_f3_api;
GRANT USAGE ON SCHEMA public TO espacigo_f6_f3_api;
REVOKE CREATE ON SCHEMA public FROM espacigo_f6_f3_api;
REVOKE ALL PRIVILEGES ON ALL TABLES IN SCHEMA public FROM espacigo_f6_f3_api;
REVOKE ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public FROM espacigo_f6_f3_api;
REVOKE ALL PRIVILEGES ON ALL FUNCTIONS IN SCHEMA public FROM espacigo_f6_f3_api;

-- Normal registration, explicit email verification, login, session and logout.
GRANT SELECT, INSERT, UPDATE ON public.usuario TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, UPDATE ON public.sesion TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, UPDATE ON public.token_accion TO espacigo_f6_f3_api;
GRANT SELECT, INSERT ON public.aceptacion_terminos TO espacigo_f6_f3_api;
GRANT SELECT ON public.version_terminos, public.rol_usuario, public.administrador_global TO espacigo_f6_f3_api;
GRANT INSERT ON public.rol_usuario, public.rol_usuario_evento TO espacigo_f6_f3_api;
GRANT SELECT ON public.bloqueo_cuenta_administrativo_local TO espacigo_f6_f3_api;

-- F3 space draft CRUD, category profiles, location, weekly hours, pricing and retirement history.
GRANT SELECT ON public.categoria_espacio, public.categoria_perfil_atributos,
    public.catalogo_ubicacion_publica, public.region_publica, public.comuna_publica,
    public.atributo_filtro_publico TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, UPDATE ON public.espacio TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_caracteristicas,
    public.espacio_horario_semanal, public.espacio_horario_semanal_tramo TO espacigo_f6_f3_api;
GRANT SELECT, INSERT ON public.tarifa_espacio TO espacigo_f6_f3_api;
GRANT SELECT, INSERT ON public.espacio_publicacion_historial_local,
    public.espacio_eliminacion_historial_local TO espacigo_f6_f3_api;
GRANT USAGE, SELECT ON SEQUENCE public.espacio_publicacion_historial_local_id_seq,
    public.espacio_eliminacion_historial_local_id_seq TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, DELETE ON public.simulacion_precio_privada TO espacigo_f6_f3_api;
GRANT SELECT, DELETE ON public.reserva_ensayo_local_fixture TO espacigo_f6_f3_api;
-- Publication serializes against fixture changes with SELECT ... FOR UPDATE.
GRANT UPDATE (habilitada) ON public.reserva_ensayo_local_fixture TO espacigo_f6_f3_api;
GRANT EXECUTE ON FUNCTION public.purge_unreferenced_local_publication_rates(uuid) TO espacigo_f6_f3_api;

-- F3 retirement eligibility reads related synthetic booking/finance history only.
GRANT SELECT ON public.reserva_ensayo_local, public.operacion_arriendo_ensayo_local,
    public.reserva_liquidacion_ensayo_local, public.reclamo_dano_ensayo_local,
    public.disputa_ensayo_local, public.reserva_devolucion_ensayo,
    public.reserva_pago_ensayo_operacion, public.reserva_pago_evento_aplicacion_ensayo,
    public.reserva_pago_evento_ensayo, public.reserva_garantia_ensayo_local,
    public.reserva_garantia_operacion_ensayo_local,
    public.reserva_garantia_evento_aplicacion_ensayo_local,
    public.reserva_garantia_evento_ensayo_local,
    public.reserva_decision_financiera_ensayo_local,
    public.reserva_liquidacion_operacion_ensayo_local TO espacigo_f6_f3_api;

-- F4 gallery and media lifecycle; no TRUNCATE or direct asset/variant deletion.
GRANT SELECT, INSERT, UPDATE ON public.espacio_galeria_sintetica_local TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_galeria_archivo_candidato_local,
    public.gallery_item_order, public.media_variant_intent TO espacigo_f6_f3_api;
GRANT SELECT, INSERT, UPDATE ON public.media_asset, public.space_media TO espacigo_f6_f3_api;
GRANT SELECT, INSERT ON public.media_variant TO espacigo_f6_f3_api;
