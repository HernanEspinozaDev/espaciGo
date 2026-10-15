package adminglobal

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const RestrictedLocalBootstrapRole = "espacigo_gadm_bootstrap"

// SetRestrictedLocalBootstrapRole reduces every physical CLI connection to
// the NOLOGIN group role before any application SQL is issued, then verifies
// the temporary login's membership and exact column-level allowlist.
func SetRestrictedLocalBootstrapRole(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return ErrInvalid
	}
	if _, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{RestrictedLocalBootstrapRole}.Sanitize()); err != nil {
		return ErrForbidden
	}
	return ValidateRestrictedLocalBootstrapConnection(ctx, conn)
}

// ValidateRestrictedLocalBootstrapConnection rejects superuser/admin logins,
// broad role memberships, direct grants to the pre-SET ROLE login, ownership,
// table-level grants, and any column privilege outside the V60 allowlist.
func ValidateRestrictedLocalBootstrapConnection(ctx context.Context, conn *pgx.Conn) error {
	if conn == nil {
		return ErrInvalid
	}
	var safe bool
	err := conn.QueryRow(ctx, `
WITH roles AS (
 SELECT s.oid AS session_oid,s.rolname AS session_name,s.rolcanlogin AS session_can_login,
        s.rolsuper AS session_super,s.rolcreatedb AS session_createdb,s.rolcreaterole AS session_createrole,
        s.rolreplication AS session_replication,s.rolbypassrls AS session_bypassrls,s.rolinherit AS session_inherit,
        g.oid AS current_oid,g.rolname AS current_name,g.rolcanlogin AS current_can_login,
        g.rolsuper AS current_super,g.rolcreatedb AS current_createdb,g.rolcreaterole AS current_createrole,
        g.rolreplication AS current_replication,g.rolbypassrls AS current_bypassrls,g.rolinherit AS current_inherit
 FROM pg_catalog.pg_roles s CROSS JOIN pg_catalog.pg_roles g
 WHERE s.rolname=session_user AND g.rolname=current_user
), memberships AS (
 SELECT r.*,
   (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=r.session_oid) AS session_memberships,
   (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.member=r.current_oid) AS group_parents,
   (SELECT count(*) FROM pg_catalog.pg_auth_members m WHERE m.roleid=r.current_oid) AS group_members,
   EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=r.session_oid AND m.roleid=r.current_oid
     AND m.set_option AND NOT m.inherit_option AND NOT m.admin_option) AS exact_membership
 FROM roles r
), acl AS (
 SELECT
   NOT EXISTS (SELECT 1 FROM information_schema.table_privileges WHERE grantee=current_user) AS no_table_grants,
   -- Check sequence ACLs assigned to the GADM group; unrelated PUBLIC grants may predate this role.
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c
     CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) x
     WHERE c.relkind='S' AND x.grantee=current_oid) AS no_sequence_grants,
   NOT EXISTS (SELECT 1 FROM information_schema.routine_privileges WHERE grantee=current_user) AS no_routine_grants,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c
     CROSS JOIN LATERAL pg_catalog.aclexplode(c.relacl) x
     WHERE c.relkind IN ('r','p','v','m','f') AND x.grantee=session_oid) AS no_session_table_grants,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_attribute a
     JOIN pg_catalog.pg_class c ON c.oid=a.attrelid
     CROSS JOIN LATERAL pg_catalog.aclexplode(a.attacl) x
     WHERE c.relkind IN ('r','p','v','m','f') AND a.attnum>0 AND NOT a.attisdropped
       AND x.grantee=session_oid) AS no_session_column_grants,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_proc p
     CROSS JOIN LATERAL pg_catalog.aclexplode(p.proacl) x
     WHERE x.grantee=session_oid) AS no_session_routine_grants,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_class c
     CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(c.relacl,pg_catalog.acldefault('S',c.relowner))) x
     WHERE c.relkind='S' AND x.grantee=session_oid) AS no_session_sequence_grants,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n
     CROSS JOIN LATERAL pg_catalog.aclexplode(n.nspacl) x
     WHERE x.grantee=session_oid) AS no_session_schema_grants,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_database d
     CROSS JOIN LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) x
     WHERE d.datname=current_database() AND x.grantee=session_oid AND x.privilege_type<>'CONNECT') AS no_excess_session_database_grants,
   (SELECT count(*) FROM information_schema.column_privileges WHERE grantee=current_user) AS column_grant_count,
   NOT EXISTS (SELECT 1 FROM pg_catalog.pg_shdepend sd
     WHERE sd.refclassid='pg_catalog.pg_authid'::regclass AND sd.refobjid IN (session_oid,current_oid)
       AND sd.deptype='o') AS owns_no_objects
 FROM memberships
)
SELECT
 session_user<>current_user AND session_name ~ '^gadm_activation_[0-9a-f]{12}$'
 AND session_can_login AND NOT session_super AND NOT session_createdb AND NOT session_createrole
 AND NOT session_replication AND NOT session_bypassrls AND NOT session_inherit
 AND current_name='espacigo_gadm_bootstrap' AND NOT current_can_login AND NOT current_super
 AND NOT current_createdb AND NOT current_createrole AND NOT current_replication AND NOT current_bypassrls AND NOT current_inherit
 AND session_memberships=1 AND exact_membership AND group_parents=0 AND group_members=1
 AND has_database_privilege(current_user,current_database(),'CONNECT')
 AND NOT has_database_privilege(current_user,current_database(),'CREATE')
 AND has_schema_privilege(current_user,'public','USAGE')
 AND NOT has_schema_privilege(current_user,'public','CREATE')
 AND no_table_grants AND no_sequence_grants AND no_routine_grants AND owns_no_objects
 AND no_session_table_grants AND no_session_column_grants AND no_session_routine_grants
 AND no_session_sequence_grants AND no_session_schema_grants AND no_excess_session_database_grants
 AND has_database_privilege(session_name,current_database(),'CONNECT')
 AND NOT has_database_privilege(session_name,current_database(),'CREATE')
 AND NOT has_schema_privilege(session_name,'public','CREATE')
	 AND column_grant_count=23
 AND has_column_privilege(current_user,'public.usuario','id','SELECT')
 AND has_column_privilege(current_user,'public.usuario','estado','SELECT')
 AND has_column_privilege(current_user,'public.usuario','bloqueado_hasta','SELECT')
 AND has_column_privilege(current_user,'public.usuario','id','UPDATE')
 AND NOT has_column_privilege(current_user,'public.usuario','estado','UPDATE')
 AND NOT has_column_privilege(current_user,'public.usuario','correo_normalizado','SELECT')
 AND NOT has_column_privilege(current_user,'public.usuario','hash_clave','SELECT')
 AND has_column_privilege(current_user,'public.bloqueo_cuenta_administrativo_local','cuenta_id','SELECT')
 AND has_column_privilege(current_user,'public.administrador_global_bootstrap','singleton','SELECT')
 AND has_column_privilege(current_user,'public.administrador_global_bootstrap','usuario_id','SELECT')
 AND has_column_privilege(current_user,'public.administrador_global_bootstrap','singleton','UPDATE')
 AND NOT has_column_privilege(current_user,'public.administrador_global_bootstrap','usuario_id','UPDATE')
 AND has_column_privilege(current_user,'public.administrador_global','usuario_id','INSERT')
 AND has_column_privilege(current_user,'public.administrador_global','concedido_en','INSERT')
 AND has_column_privilege(current_user,'public.administrador_global','motivo_codigo','INSERT')
 AND NOT has_table_privilege(current_user,'public.administrador_global','UPDATE')
 AND NOT has_table_privilege(current_user,'public.administrador_global','DELETE')
 AND NOT has_table_privilege(current_user,'public.administrador_global','TRUNCATE')
 AND NOT has_table_privilege(current_user,'public.administrador_global_bootstrap','DELETE')
 AND NOT has_table_privilege(current_user,'public.administrador_global_bootstrap_evento','DELETE')
FROM memberships CROSS JOIN acl`).Scan(&safe)
	if err != nil {
		return fmt.Errorf("validate restricted local bootstrap role: %w", err)
	}
	if !safe {
		return ErrForbidden
	}

	rows, err := conn.Query(ctx, `SELECT table_name,column_name,privilege_type
		FROM information_schema.column_privileges WHERE grantee=current_user`)
	if err != nil {
		return fmt.Errorf("read restricted local bootstrap column grants: %w", err)
	}
	defer rows.Close()
	allowed := map[string]struct{}{
		"usuario.id:SELECT": {}, "usuario.estado:SELECT": {}, "usuario.bloqueado_hasta:SELECT": {}, "usuario.id:UPDATE": {},
		"bloqueo_cuenta_administrativo_local.cuenta_id:SELECT": {},
		"administrador_global_bootstrap.singleton:SELECT":      {}, "administrador_global_bootstrap.usuario_id:SELECT": {}, "administrador_global_bootstrap.singleton:UPDATE": {},
		"administrador_global_bootstrap.singleton:INSERT": {}, "administrador_global_bootstrap.usuario_id:INSERT": {}, "administrador_global_bootstrap.creado_en:INSERT": {},
		"administrador_global.usuario_id:INSERT": {}, "administrador_global.concedido_en:INSERT": {}, "administrador_global.motivo_codigo:INSERT": {},
		"administrador_global.usuario_id:SELECT":          {},
		"administrador_global_bootstrap_evento.id:INSERT": {}, "administrador_global_bootstrap_evento.usuario_objetivo_id:INSERT": {},
		"administrador_global_bootstrap_evento.actor_tipo:INSERT": {}, "administrador_global_bootstrap_evento.accion:INSERT": {},
		"administrador_global_bootstrap_evento.resultado:INSERT": {}, "administrador_global_bootstrap_evento.motivo_codigo:INSERT": {},
		"administrador_global_bootstrap_evento.correlacion_id:INSERT": {}, "administrador_global_bootstrap_evento.ocurrida_en:INSERT": {},
	}
	seen := make(map[string]struct{}, len(allowed))
	for rows.Next() {
		var table, column, privilege string
		if err := rows.Scan(&table, &column, &privilege); err != nil {
			return fmt.Errorf("read restricted local bootstrap column grants: %w", err)
		}
		key := table + "." + column + ":" + privilege
		if _, ok := allowed[key]; !ok {
			return ErrForbidden
		}
		seen[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read restricted local bootstrap column grants: %w", err)
	}
	if len(seen) != len(allowed) {
		return ErrForbidden
	}
	return nil
}
