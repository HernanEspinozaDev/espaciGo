package dbbootstrap

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

type roleAttributes struct {
	canLogin    bool
	superuser   bool
	createDB    bool
	createRole  bool
	replication bool
	bypassRLS   bool
	inherit     bool
}

func readRoleAttributes(ctx context.Context, conn *pgx.Conn, role string) (roleAttributes, error) {
	var attrs roleAttributes
	err := conn.QueryRow(ctx, `SELECT rolcanlogin, rolsuper, rolcreatedb, rolcreaterole,
		rolreplication, rolbypassrls, rolinherit
		FROM pg_catalog.pg_roles WHERE rolname = $1`, role).Scan(
		&attrs.canLogin, &attrs.superuser, &attrs.createDB, &attrs.createRole,
		&attrs.replication, &attrs.bypassRLS, &attrs.inherit,
	)
	if err != nil {
		return roleAttributes{}, errors.New("could not verify required scratch role")
	}
	return attrs, nil
}

func roleHasAnyMembership(ctx context.Context, conn *pgx.Conn, role string) (bool, error) {
	var hasMembership bool
	err := conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM pg_catalog.pg_auth_members m
		JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
		JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
		WHERE granted_role.rolname = $1 OR member_role.rolname = $1
	)`, role).Scan(&hasMembership)
	if err != nil {
		return false, errors.New("could not verify scratch role memberships")
	}
	return hasMembership, nil
}

func requireRestrictedNoLoginRole(ctx context.Context, conn *pgx.Conn, role string, mustNotInherit bool) error {
	attrs, err := readRoleAttributes(ctx, conn, role)
	if err != nil {
		return err
	}
	if attrs.canLogin || attrs.superuser || attrs.createDB || attrs.createRole || attrs.replication || attrs.bypassRLS || (mustNotInherit && attrs.inherit) {
		return errors.New("scratch group role has unsafe attributes")
	}
	membership, err := roleHasAnyMembership(ctx, conn, role)
	if err != nil {
		return err
	}
	if membership {
		return errors.New("scratch group role has unexpected memberships")
	}
	return nil
}

// VerifyScratchBootstrap checks the fixed extension and restricted-role
// preconditions shared by the bootstrap and migration phases.
func VerifyScratchBootstrap(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return errors.New("scratch database connection is required")
	}
	var database string
	if err := conn.QueryRow(ctx, "SELECT current_database()").Scan(&database); err != nil || database != "espacigo_scratch" {
		return errors.New("connected database is not the reviewed GCP-03 scratch database")
	}
	if _, err := ScratchExtensionVersions(ctx, conn); err != nil {
		return err
	}
	if err := requireRestrictedNoLoginRole(ctx, conn, "espacigo_runtime", true); err != nil {
		return err
	}
	return requireRestrictedNoLoginRole(ctx, conn, "espacigo_gadm_bootstrap", true)
}

// VerifyScratchBootstrapRolesCreated accepts only PostgreSQL 18's automatic
// ADMIN membership from the creating login while Cloud SQL Admin API cleanup
// is pending. Migration preflight uses VerifyScratchBootstrap, which rejects
// every remaining membership.
func VerifyScratchBootstrapRolesCreated(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return errors.New("scratch database connection is required")
	}
	for _, role := range []string{"espacigo_runtime", "espacigo_gadm_bootstrap"} {
		attrs, err := readRoleAttributes(ctx, conn, role)
		if err != nil {
			return err
		}
		if attrs.canLogin || attrs.superuser || attrs.createDB || attrs.createRole || attrs.replication || attrs.bypassRLS || attrs.inherit {
			return errors.New("scratch group role has unsafe attributes")
		}
	}
	var valid bool
	err := conn.QueryRow(ctx, `WITH relevant AS (
		SELECT granted.rolname AS granted_name, member.rolname AS member_name,
		       m.admin_option, m.inherit_option, m.set_option
		FROM pg_catalog.pg_auth_members m
		JOIN pg_catalog.pg_roles granted ON granted.oid = m.roleid
		JOIN pg_catalog.pg_roles member ON member.oid = m.member
		WHERE granted.rolname IN ('espacigo_runtime', 'espacigo_gadm_bootstrap')
		   OR member.rolname IN ('espacigo_runtime', 'espacigo_gadm_bootstrap')
	), expected AS (
		SELECT count(*) AS membership_count
		FROM relevant
		WHERE granted_name IN ('espacigo_runtime', 'espacigo_gadm_bootstrap')
		  AND member_name = 'espacigo_scratch_migrator'
		  AND admin_option AND NOT inherit_option AND NOT set_option
	)
	SELECT (SELECT membership_count FROM expected) <= 2
	   AND NOT EXISTS (
		SELECT 1 FROM relevant
		WHERE NOT (granted_name IN ('espacigo_runtime', 'espacigo_gadm_bootstrap')
		  AND member_name = 'espacigo_scratch_migrator' AND admin_option AND NOT inherit_option AND NOT set_option)
	   )`).Scan(&valid)
	if err != nil {
		return errors.New("could not verify automatic scratch role memberships")
	}
	if !valid {
		return errors.New("scratch group roles have unexpected memberships; reconcile the temporary login through Cloud SQL Admin API")
	}
	return nil
}

// VerifyScratchDatabaseACL requires TEMP only for the migration login. The
// PUBLIC ACL is inspected directly because effective privilege checks alone
// can hide inherited grants through PUBLIC.
func VerifyScratchDatabaseACL(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return errors.New("scratch database connection is required")
	}
	var publicTemp, migratorTemp, runtimeTemp, migratorCreate, runtimeCreate, publicSchemaCreate bool
	err := conn.QueryRow(ctx, `SELECT
		EXISTS (
			SELECT 1
			FROM pg_catalog.pg_database d,
			     LATERAL pg_catalog.aclexplode(COALESCE(d.datacl, pg_catalog.acldefault('d', d.datdba))) acl
			WHERE d.datname = 'espacigo_scratch'
			  AND acl.grantee = 0 AND acl.privilege_type = 'TEMPORARY'
		),
		has_database_privilege('espacigo_scratch_migrator', 'espacigo_scratch', 'TEMP'),
		has_database_privilege('espacigo_runtime', 'espacigo_scratch', 'TEMP'),
		has_schema_privilege('espacigo_scratch_migrator', 'public', 'CREATE'),
		has_schema_privilege('espacigo_runtime', 'public', 'CREATE'),
		EXISTS (
			SELECT 1
			FROM pg_catalog.pg_namespace n,
			     LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl, pg_catalog.acldefault('n', n.nspowner))) acl
			WHERE n.nspname = 'public' AND acl.grantee = 0 AND acl.privilege_type = 'CREATE'
		)`).Scan(&publicTemp, &migratorTemp, &runtimeTemp, &migratorCreate, &runtimeCreate, &publicSchemaCreate)
	if err != nil {
		return errors.New("could not inspect scratch database TEMP ACL")
	}
	if publicTemp || !migratorTemp || runtimeTemp || !migratorCreate || runtimeCreate || publicSchemaCreate {
		return errors.New("scratch database TEMP/schema privileges differ from the migrator-only contract")
	}
	return nil
}

// ScratchExtensionVersions reports the fixed supported extension set for
// review logs without exposing connection credentials or executing caller SQL.
func ScratchExtensionVersions(ctx context.Context, conn *pgx.Conn) (map[string]string, error) {
	if conn == nil {
		return nil, errors.New("scratch database connection is required")
	}
	rows, err := conn.Query(ctx, `SELECT extname, extversion FROM pg_catalog.pg_extension
		WHERE extname IN ('btree_gist', 'postgis', 'unaccent') ORDER BY extname`)
	if err != nil {
		return nil, errors.New("could not read scratch extension versions")
	}
	defer rows.Close()
	versions := make(map[string]string, 3)
	for rows.Next() {
		var name, version string
		if err := rows.Scan(&name, &version); err != nil {
			return nil, errors.New("could not read scratch extension versions")
		}
		versions[name] = version
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("could not read scratch extension versions")
	}
	if len(versions) != 3 || versions["btree_gist"] == "" || versions["postgis"] == "" || versions["unaccent"] == "" {
		return nil, errors.New("one or more required scratch extensions are missing")
	}
	return versions, nil
}

// VerifyScratchRuntimePermissions checks the most important positive and
// negative grants after GrantRuntimePermissions has run against all migrations.
func VerifyScratchRuntimePermissions(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return errors.New("scratch database connection is required")
	}
	if err := requireRestrictedNoLoginRole(ctx, conn, "espacigo_runtime", true); err != nil {
		return err
	}
	if err := requireRestrictedNoLoginRole(ctx, conn, "espacigo_gadm_bootstrap", true); err != nil {
		return err
	}
	if err := VerifyScratchDatabaseACL(ctx, conn); err != nil {
		return err
	}
	var checks [16]bool
	err := conn.QueryRow(ctx, `SELECT
		has_database_privilege('espacigo_runtime', 'espacigo_scratch', 'CONNECT'),
		not has_database_privilege('espacigo_runtime', 'espacigo_scratch', 'CREATE'),
		not has_database_privilege('espacigo_runtime', 'espacigo_scratch', 'TEMP'),
		has_schema_privilege('espacigo_runtime', 'public', 'USAGE'),
		not has_schema_privilege('espacigo_runtime', 'public', 'CREATE'),
		has_table_privilege('espacigo_runtime', 'public.usuario', 'SELECT'),
		has_table_privilege('espacigo_runtime', 'public.usuario', 'INSERT'),
		has_table_privilege('espacigo_runtime', 'public.usuario', 'UPDATE'),
		not has_table_privilege('espacigo_runtime', 'public.usuario', 'DELETE'),
		has_table_privilege('espacigo_runtime', 'public.administrador_global', 'SELECT'),
		not has_table_privilege('espacigo_runtime', 'public.administrador_global', 'INSERT'),
		not has_table_privilege('espacigo_runtime', 'public.administrador_global', 'UPDATE'),
		not has_table_privilege('espacigo_runtime', 'public.administrador_global', 'DELETE'),
		not has_table_privilege('espacigo_runtime', 'public.tarifa_espacio', 'UPDATE'),
		not has_table_privilege('espacigo_runtime', 'public.tarifa_espacio', 'DELETE'),
		not has_table_privilege('espacigo_runtime', 'public.schema_migrations', 'SELECT')`).Scan(
		&checks[0], &checks[1], &checks[2], &checks[3], &checks[4], &checks[5],
		&checks[6], &checks[7], &checks[8], &checks[9], &checks[10], &checks[11],
		&checks[12], &checks[13], &checks[14], &checks[15],
	)
	if err != nil {
		return errors.New("could not verify scratch runtime grants")
	}
	for _, passed := range checks {
		if !passed {
			return errors.New("scratch runtime grants differ from the reviewed privilege contract")
		}
	}
	return nil
}

// VerifyScratchMigratorRetired checks that the temporary login has no active
// attributes or memberships after its privileges are withdrawn.
func VerifyScratchMigratorRetired(ctx context.Context, conn *pgx.Conn) error {
	attrs, err := readRoleAttributes(ctx, conn, "espacigo_scratch_migrator")
	if err != nil {
		return err
	}
	if attrs.canLogin || attrs.superuser || attrs.createDB || attrs.createRole || attrs.replication || attrs.bypassRLS || attrs.inherit {
		return errors.New("temporary scratch login still has active privileges")
	}
	var unexpectedMembership bool
	err = conn.QueryRow(ctx, `SELECT EXISTS (
		SELECT 1
		FROM pg_catalog.pg_auth_members m
		JOIN pg_catalog.pg_roles granted_role ON granted_role.oid = m.roleid
		JOIN pg_catalog.pg_roles member_role ON member_role.oid = m.member
		WHERE (member_role.rolname = 'espacigo_scratch_migrator'
		       AND granted_role.rolname <> 'cloudsqlsuperuser')
		   OR granted_role.rolname = 'espacigo_scratch_migrator'
	)`).Scan(&unexpectedMembership)
	if err != nil {
		return errors.New("could not verify temporary scratch login memberships")
	}
	if unexpectedMembership {
		return errors.New("temporary scratch login has an unexpected role membership")
	}
	var tempPrivilege, createPrivilege bool
	err = conn.QueryRow(ctx, `SELECT
		has_database_privilege('espacigo_scratch_migrator', 'espacigo_scratch', 'TEMP'),
		has_schema_privilege('espacigo_scratch_migrator', 'public', 'CREATE')`).Scan(&tempPrivilege, &createPrivilege)
	if err != nil {
		return errors.New("could not verify retired scratch login object privileges")
	}
	if tempPrivilege || createPrivilege {
		return errors.New("temporary scratch login retains database or schema privileges")
	}
	return nil
}

// GrantRuntimePermissions applies the least-privilege table grants after migrations.
// Versioned rates and private simulations are append/read only for user flows;
// the administrator-guarded local suppression path may delete private simulations.
func GrantRuntimePermissions(ctx context.Context, conn *pgx.Conn) error {
	_, err := conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.usuario, public.sesion, public.token_accion TO espacigo_runtime;
 GRANT DELETE ON public.sesion, public.token_accion TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.historial_clave_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.outbox_evento_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.outbox_evento_ciclo_local TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.purge_expired_local_credential_notices(timestamptz,integer) TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.purge_expired_local_privacy_rows(timestamptz,integer) TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.purge_expired_local_contracts_and_publication_history(timestamptz,integer) TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.claim_expired_local_verification_files(timestamptz,integer) TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.complete_expired_local_verification_file(uuid,timestamptz) TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.fail_expired_local_verification_file(uuid,timestamptz) TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.evento_auditoria_local TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.bloqueo_cuenta_administrativo_local TO espacigo_runtime;
 GRANT UPDATE (bloqueada_por) ON public.bloqueo_cuenta_administrativo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.bloqueo_cuenta_historial_local TO espacigo_runtime;
 GRANT UPDATE (actor_id) ON public.bloqueo_cuenta_historial_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.disputa_ensayo_local TO espacigo_runtime;
 GRANT UPDATE (estado,cerrada_por,motivo_cierre_codigo,cerrada_en,anfitrion_id,arrendatario_id,abierta_por) ON public.disputa_ensayo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.disputa_ensayo_historial TO espacigo_runtime;
 GRANT UPDATE (actor_id) ON public.disputa_ensayo_historial TO espacigo_runtime;
 GRANT USAGE, SELECT ON SEQUENCE public.disputa_ensayo_historial_secuencia_seq TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.rol_usuario, public.aceptacion_terminos TO espacigo_runtime;
 GRANT SELECT ON public.administrador_global, public.administrador_global_bootstrap TO espacigo_runtime;
 REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON public.administrador_global, public.administrador_global_bootstrap, public.administrador_global_bootstrap_evento FROM espacigo_runtime;
 GRANT DELETE ON public.rol_usuario TO espacigo_runtime;
 GRANT UPDATE (concedido_por) ON public.rol_usuario TO espacigo_runtime;
 GRANT UPDATE (retirar_en) ON public.aceptacion_terminos TO espacigo_runtime;
 GRANT SELECT ON public.version_terminos TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.perfil_usuario TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE (estado,resuelta_en,motivo_resolucion_codigo,retirar_en) ON public.solicitud_titular TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.verificacion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.verificacion_historial_local TO espacigo_runtime;
 GRANT USAGE, SELECT ON SEQUENCE public.verificacion_historial_local_id_seq TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.elegibilidad_verificacion_local TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.verificacion_evidencia_sintetica TO espacigo_runtime;
 GRANT SELECT ON public.categoria_espacio, public.categoria_perfil_atributos TO espacigo_runtime;
 GRANT SELECT ON public.catalogo_ubicacion_publica, public.region_publica, public.comuna_publica, public.atributo_filtro_publico TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.espacio TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.espacio_eliminacion_historial_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.espacio_eliminacion_historial_local_id_seq TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.espacio_publicacion_historial_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.espacio_publicacion_historial_local_id_seq TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.espacio_galeria_sintetica_local TO espacigo_runtime;
	GRANT UPDATE (estado,retirada_en,proximo_intento_en,limpia_en,intentos_limpieza,ultimo_codigo_error,archivo_id,publicada_en_catalogo) ON public.espacio_galeria_sintetica_local TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_galeria_archivo_candidato_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_horario_semanal, public.espacio_horario_semanal_tramo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.espacio_caracteristicas TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.ocupacion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.tarifa_espacio TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.simulacion_precio_privada TO espacigo_runtime;
 GRANT SELECT, DELETE ON public.reserva_ensayo_local_fixture TO espacigo_runtime;
 GRANT UPDATE (habilitada,anfitrion_id,arrendatario_id) ON public.reserva_ensayo_local_fixture TO espacigo_runtime;
 GRANT SELECT ON public.reserva_ensayo_local_ubicacion_sintetica TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.cotizacion_reserva_ensayo TO espacigo_runtime;
 GRANT UPDATE (retirar_en,anfitrion_id,arrendatario_id) ON public.cotizacion_reserva_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_ensayo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_ensayo_transicion, public.reserva_pago_ensayo TO espacigo_runtime;
	GRANT UPDATE (actor_id) ON public.reserva_ensayo_transicion TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.operacion_arriendo_ensayo_local, public.operacion_arriendo_evidencia_ensayo_local, public.operacion_arriendo_historial_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (archivo_retirado_en,retirar_archivo_en) ON public.operacion_arriendo_evidencia_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (comentarios,observacion,reserva_id,actor_id) ON public.operacion_arriendo_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (reserva_id,actor_id) ON public.operacion_arriendo_historial_ensayo_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.operacion_arriendo_historial_ensayo_local_secuencia_seq TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE, DELETE ON public.operacion_arriendo_archivo_candidato_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.reclamo_dano_ensayo_local, public.reclamo_dano_descargo_ensayo_local, public.reclamo_dano_historial_ensayo_local, public.reclamo_dano_resolucion_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (estado,descripcion,reserva_id,anfitrion_id,arrendatario_id,checkout_operacion_id,checkout_evidencia_id) ON public.reclamo_dano_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (descripcion,actor_id) ON public.reclamo_dano_descargo_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (actor_id) ON public.reclamo_dano_historial_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (administrador_id) ON public.reclamo_dano_resolucion_ensayo_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.reclamo_dano_historial_ensayo_local_secuencia_seq TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_ensayo_operacion TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_fake_resultado_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_pago_evento_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_pago_evento_aplicacion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_cancelacion_ensayo TO espacigo_runtime;
	GRANT UPDATE (arrendatario_id) ON public.reserva_cancelacion_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.reserva_devolucion_ensayo TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_garantia_ensayo_local, public.reserva_garantia_operacion_ensayo_local TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_garantia_resultado_fake_ensayo_local, public.reserva_garantia_evento_aplicacion_ensayo_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.reserva_garantia_evento_ensayo_local TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_decision_financiera_ensayo_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.reserva_finanzas_historial_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (actor_id) ON public.reserva_finanzas_historial_ensayo_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.reserva_finanzas_historial_ensayo_local_secuencia_seq TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reserva_liquidacion_ensayo_local, public.reserva_liquidacion_operacion_ensayo_local, public.reserva_liquidacion_resultado_fake_ensayo_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.reserva_comprobante_ensayo_local, public.reserva_liquidacion_historial_ensayo_local TO espacigo_runtime;
	GRANT UPDATE (actor_id) ON public.reserva_liquidacion_historial_ensayo_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.reserva_liquidacion_historial_ensayo_local_secuencia_seq TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reserva_devolucion_intento_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.contrato_ensayo_local, public.contrato_ensayo_firma TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.documento_privado_sintetico_local TO espacigo_runtime;
 GRANT UPDATE (estado,actualizada_en,firmado_en) ON public.contrato_ensayo_local TO espacigo_runtime;
 GRANT UPDATE (estado,motivo,actualizada_en) ON public.contrato_ensayo_firma TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.contrato_ensayo_historial TO espacigo_runtime;
 GRANT SELECT, INSERT, DELETE ON public.mensaje_reserva_ensayo TO espacigo_runtime;
	GRANT UPDATE (autor_id) ON public.mensaje_reserva_ensayo TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE, DELETE ON public.reserva_mensaje_lectura TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.ejecucion_baja_local TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.baja_archivo_pendiente_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.foto_perfil_sintetica_local TO espacigo_runtime;
	GRANT UPDATE (estado,retirada_en,limpia_en,intentos_limpieza,proximo_intento_en,ultimo_codigo_error,archivo_id,mime_type,sha256,size_bytes) ON public.foto_perfil_sintetica_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.cuenta_cobro_sintetica_local TO espacigo_runtime;
	GRANT UPDATE (referencia_ficticia,estado,actualizada_en,revocada_en) ON public.cuenta_cobro_sintetica_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.cuenta_cobro_sintetica_historial_local TO espacigo_runtime;
	GRANT USAGE, SELECT ON SEQUENCE public.cuenta_cobro_sintetica_historial_local_id_seq TO espacigo_runtime;
	GRANT SELECT, INSERT, DELETE ON public.m02_operacion_idempotente_local TO espacigo_runtime;
	GRANT SELECT, INSERT ON public.reserva_vinculo_purgado_local TO espacigo_runtime;
	GRANT SELECT, INSERT, UPDATE ON public.reaplicacion_baja_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.resena_ensayo_local TO espacigo_runtime;
 GRANT UPDATE (estado,autor_id,destinatario_tipo,destinatario_id,comentario) ON public.resena_ensayo_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.resena_autoria_marca_local TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.reporte_resena_ensayo_local, public.reporte_resena_historial_local TO espacigo_runtime;
 GRANT UPDATE (estado,resolver_en,resuelta_por,motivo_resolucion_codigo,retirar_en,anfitrion_id) ON public.reporte_resena_ensayo_local TO espacigo_runtime;
 GRANT UPDATE (actor_id) ON public.reporte_resena_historial_local TO espacigo_runtime;
 GRANT USAGE, SELECT ON SEQUENCE public.reporte_resena_historial_local_secuencia_seq TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.aviso_local, public.aviso_local_ciclo TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.aviso_local_recuperacion_auditoria TO espacigo_runtime;
 GRANT EXECUTE ON FUNCTION public.purge_expired_local_communication(timestamptz,integer) TO espacigo_runtime;
	`)
	if err != nil {
		return err
	}
	// The runtime may remove unreferenced rates only through the narrowly
	// scoped SECURITY DEFINER function used by publication retirement. It may
	// not delete immutable rate history directly. Privacy suppression still
	// needs table DELETE for the account's private simulations.
	_, err = conn.Exec(ctx, `REVOKE DELETE ON public.tarifa_espacio FROM espacigo_runtime;
REVOKE UPDATE,DELETE ON public.espacio_publicacion_historial_local,public.espacio_eliminacion_historial_local FROM espacigo_runtime;
GRANT EXECUTE ON FUNCTION public.purge_unreferenced_local_publication_rates(uuid) TO espacigo_runtime;
GRANT DELETE ON public.simulacion_precio_privada TO espacigo_runtime;`)
	return err
}
