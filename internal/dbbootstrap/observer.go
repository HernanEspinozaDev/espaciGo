package dbbootstrap

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// ScratchObservation contains catalog metadata and results from fixed,
// read-only compatibility probes. It deliberately has no field for row data,
// credentials, connection strings, or caller-provided SQL.
type ScratchObservation struct {
	ServerVersionNum       int                          `json:"server_version_num"`
	ServerVersion          string                       `json:"server_version"`
	PostGISFullVersion     string                       `json:"postgis_full_version"`
	Extensions             []ExtensionObservation       `json:"extensions"`
	Roles                  []RoleObservation            `json:"roles"`
	Memberships            []RoleMembershipObservation  `json:"memberships"`
	DatabaseACL            []ACLObservation             `json:"database_acl"`
	PublicSchemaACL        []ACLObservation             `json:"public_schema_acl"`
	RelationOwners         []OwnedObjectObservation     `json:"relation_owners"`
	FunctionOwners         []OwnedObjectObservation     `json:"function_owners"`
	SecurityDefiner        []SecurityDefinerObservation `json:"security_definer_functions"`
	MembershipExpectation  RoleMembershipExpectation    `json:"membership_expectation"`
	RoleInventoryPass      bool                         `json:"role_inventory_contract_pass"`
	DatabaseACLPass        bool                         `json:"database_acl_contract_pass"`
	RuntimePermissionsPass bool                         `json:"runtime_permissions_contract_pass"`
	RuntimeChecks          []PermissionCheck            `json:"runtime_permission_checks"`
	Spatial                SpatialObservation           `json:"spatial_checks"`
}

type RoleMembershipExpectation string

const (
	RoleMembershipManagedCloudSQL RoleMembershipExpectation = "managed_cloudsql"
	RoleMembershipLocalPostgres18 RoleMembershipExpectation = "local_postgres18"
	RoleMembershipRetired         RoleMembershipExpectation = "retired"
)

type PermissionCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
}

type ExtensionObservation struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type RoleObservation struct {
	Name        string `json:"name"`
	CanLogin    bool   `json:"can_login"`
	Superuser   bool   `json:"superuser"`
	CreateDB    bool   `json:"createdb"`
	CreateRole  bool   `json:"createrole"`
	Replication bool   `json:"replication"`
	BypassRLS   bool   `json:"bypassrls"`
	Inherit     bool   `json:"inherit"`
}

type RoleMembershipObservation struct {
	GrantedRole string `json:"granted_role"`
	Member      string `json:"member"`
	AdminOption bool   `json:"admin_option"`
	Inherit     bool   `json:"inherit_option"`
	Set         bool   `json:"set_option"`
}

type ACLObservation struct {
	Grantee   string `json:"grantee"`
	Grantor   string `json:"grantor"`
	Privilege string `json:"privilege"`
	Grantable bool   `json:"grantable"`
}

type OwnedObjectObservation struct {
	Schema            string `json:"schema"`
	Name              string `json:"name"`
	Kind              string `json:"kind"`
	IdentityArguments string `json:"identity_arguments,omitempty"`
	Owner             string `json:"owner"`
	ExtensionOwned    bool   `json:"extension_owned"`
}

type SecurityDefinerObservation struct {
	Schema            string   `json:"schema"`
	Name              string   `json:"name"`
	IdentityArguments string   `json:"identity_arguments"`
	Owner             string   `json:"owner"`
	Configuration     []string `json:"configuration"`
	ExecuteACL        []string `json:"execute_acl"`
}

type SpatialObservation struct {
	GeometryValid           bool    `json:"geometry_valid"`
	GeometryIntersects      bool    `json:"geometry_intersects"`
	GeographyDistanceMeters float64 `json:"geography_distance_meters"`
	ExpectedIndex           string  `json:"expected_index"`
	ActualIndex             string  `json:"actual_index"`
	IndexAccessMethod       string  `json:"index_access_method"`
	IndexValid              bool    `json:"index_valid"`
	IndexReady              bool    `json:"index_ready"`
	IndexDefinition         string  `json:"index_definition"`
	ChecksPass              bool    `json:"checks_pass"`
}

const (
	observerVersionsSQL = `SELECT current_setting('server_version_num')::integer,
	current_setting('server_version'), public.postgis_full_version()`
	observerExtensionsSQL = `SELECT extname, extversion FROM pg_catalog.pg_extension ORDER BY extname`
	observerRolesSQL      = `SELECT rolname, rolcanlogin, rolsuper, rolcreatedb, rolcreaterole,
	rolreplication, rolbypassrls, rolinherit FROM pg_catalog.pg_roles
	WHERE rolname IN ('espacigo_gadm_bootstrap','espacigo_runtime','espacigo_scratch_migrator','postgres')
	ORDER BY rolname`
	observerMembershipsSQL = `SELECT granted.rolname, member.rolname, m.admin_option,
	m.inherit_option, m.set_option FROM pg_catalog.pg_auth_members m
	JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid
	JOIN pg_catalog.pg_roles member ON member.oid=m.member
	WHERE granted.rolname IN ('espacigo_gadm_bootstrap','espacigo_runtime','espacigo_scratch_migrator')
	OR member.rolname IN ('espacigo_gadm_bootstrap','espacigo_runtime','espacigo_scratch_migrator')
	ORDER BY granted.rolname, member.rolname`
	observerDatabaseACLSQL = `SELECT CASE WHEN acl.grantee=0 THEN 'PUBLIC' ELSE grantee.rolname END,
	grantor.rolname, acl.privilege_type, acl.is_grantable
	FROM pg_catalog.pg_database d,
	LATERAL pg_catalog.aclexplode(COALESCE(d.datacl,pg_catalog.acldefault('d',d.datdba))) acl
	LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid=acl.grantee
	LEFT JOIN pg_catalog.pg_roles grantor ON grantor.oid=acl.grantor
	WHERE d.datname='espacigo_scratch' ORDER BY 1,3,2`
	observerPublicSchemaACLSQL = `SELECT CASE WHEN acl.grantee=0 THEN 'PUBLIC' ELSE grantee.rolname END,
	grantor.rolname, acl.privilege_type, acl.is_grantable
	FROM pg_catalog.pg_namespace n,
	LATERAL pg_catalog.aclexplode(COALESCE(n.nspacl,pg_catalog.acldefault('n',n.nspowner))) acl
	LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid=acl.grantee
	LEFT JOIN pg_catalog.pg_roles grantor ON grantor.oid=acl.grantor
	WHERE n.nspname='public' ORDER BY 1,3,2`
	observerRelationOwnersSQL = `SELECT n.nspname, c.relname, c.relkind::text, owner.rolname,
	EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_catalog.pg_class'::regclass
	AND d.objid=c.oid AND d.deptype='e')
	FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
	JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner
	WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','S','f','i','I')
	ORDER BY c.relkind,c.relname`
	observerFunctionOwnersSQL = `SELECT n.nspname, p.proname,
	pg_catalog.pg_get_function_identity_arguments(p.oid), owner.rolname
	FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	JOIN pg_catalog.pg_roles owner ON owner.oid=p.proowner
	WHERE n.nspname='public' ORDER BY p.proname,3`
	observerSecurityDefinerSQL = `SELECT n.nspname, p.proname,
	pg_catalog.pg_get_function_identity_arguments(p.oid), owner.rolname,
	COALESCE(p.proconfig,ARRAY[]::text[]),
	COALESCE(array_agg((CASE WHEN acl.grantee=0 THEN 'PUBLIC' ELSE grantee.rolname END
	|| ':' || acl.privilege_type || ':grantable=' || acl.is_grantable::text)
	ORDER BY CASE WHEN acl.grantee=0 THEN 'PUBLIC' ELSE grantee.rolname END,acl.privilege_type),ARRAY[]::text[])
	FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace
	JOIN pg_catalog.pg_roles owner ON owner.oid=p.proowner
	LEFT JOIN LATERAL pg_catalog.aclexplode(COALESCE(p.proacl,pg_catalog.acldefault('f',p.proowner))) acl ON true
	LEFT JOIN pg_catalog.pg_roles grantee ON grantee.oid=acl.grantee
	WHERE n.nspname='public' AND p.prosecdef
	GROUP BY n.nspname,p.proname,pg_catalog.pg_get_function_identity_arguments(p.oid),p.oid,owner.rolname,p.proconfig
	ORDER BY p.proname,3`
	observerSpatialChecksSQL = `SELECT
	public.ST_IsValid(public.ST_GeomFromText('POLYGON((0 0,0 1,1 1,1 0,0 0))',4326)),
	public.ST_Intersects(public.ST_GeomFromText('POINT(0 0)',4326),public.ST_GeomFromText('POLYGON((-1 -1,-1 1,1 1,1 -1,-1 -1))',4326)),
	public.ST_Distance(public.ST_SetSRID(public.ST_MakePoint(0,0),4326)::public.geography,
	public.ST_SetSRID(public.ST_MakePoint(0,1),4326)::public.geography)`
	observerSpatialIndexSQL = `SELECT i.indexrelid::regclass::text, am.amname, i.indisvalid, i.indisready,
	pg_catalog.pg_get_indexdef(i.indexrelid)
	FROM pg_catalog.pg_index i JOIN pg_catalog.pg_class idx ON idx.oid=i.indexrelid
	JOIN pg_catalog.pg_class tbl ON tbl.oid=i.indrelid
	JOIN pg_catalog.pg_namespace n ON n.oid=tbl.relnamespace
	JOIN pg_catalog.pg_am am ON am.oid=idx.relam
	WHERE n.nspname='public' AND tbl.relname='reserva_ensayo_local_ubicacion_sintetica'
	AND idx.relname='reserva_ensayo_local_ubicacion_gist'`
	observerRuntimePermissionsSQL = `SELECT
	has_database_privilege('espacigo_runtime','espacigo_scratch','CONNECT'),
	not has_database_privilege('espacigo_runtime','espacigo_scratch','CREATE'),
	not has_database_privilege('espacigo_runtime','espacigo_scratch','TEMP'),
	has_schema_privilege('espacigo_runtime','public','USAGE'),
	not has_schema_privilege('espacigo_runtime','public','CREATE'),
	has_table_privilege('espacigo_runtime','public.usuario','SELECT'),
	has_table_privilege('espacigo_runtime','public.usuario','INSERT'),
	has_table_privilege('espacigo_runtime','public.usuario','UPDATE'),
	not has_table_privilege('espacigo_runtime','public.usuario','DELETE'),
	has_table_privilege('espacigo_runtime','public.administrador_global','SELECT'),
	not has_table_privilege('espacigo_runtime','public.administrador_global','INSERT'),
	not has_table_privilege('espacigo_runtime','public.administrador_global','UPDATE'),
	not has_table_privilege('espacigo_runtime','public.administrador_global','DELETE'),
	not has_table_privilege('espacigo_runtime','public.tarifa_espacio','UPDATE'),
	not has_table_privilege('espacigo_runtime','public.tarifa_espacio','DELETE'),
	not has_table_privilege('espacigo_runtime','public.schema_migrations','SELECT')`
)

// ObserveScratch is a fixed catalog/extension observer. It performs SELECTs
// only and uses no caller input in SQL text, identifiers, or expressions.
func ObserveScratch(ctx context.Context, conn *pgx.Conn, membershipExpectation RoleMembershipExpectation) (ScratchObservation, error) {
	if conn == nil {
		return ScratchObservation{}, errors.New("scratch database connection is required")
	}
	if membershipExpectation != RoleMembershipManagedCloudSQL && membershipExpectation != RoleMembershipLocalPostgres18 && membershipExpectation != RoleMembershipRetired {
		return ScratchObservation{}, errors.New("scratch role-membership expectation is invalid")
	}
	var report ScratchObservation
	report.MembershipExpectation = membershipExpectation
	if err := conn.QueryRow(ctx, observerVersionsSQL).Scan(&report.ServerVersionNum, &report.ServerVersion, &report.PostGISFullVersion); err != nil {
		return report, errors.New("could not observe PostgreSQL and PostGIS versions")
	}
	report.PostGISFullVersion = strings.TrimSpace(report.PostGISFullVersion)
	if report.ServerVersionNum <= 0 || report.ServerVersion == "" || report.PostGISFullVersion == "" {
		return report, errors.New("PostgreSQL/PostGIS version observation was incomplete")
	}
	if err := collectObserverRows(ctx, conn, observerExtensionsSQL, func(rows pgx.Rows) error {
		var item ExtensionObservation
		if err := rows.Scan(&item.Name, &item.Version); err != nil {
			return err
		}
		report.Extensions = append(report.Extensions, item)
		return nil
	}); err != nil {
		return report, errors.New("could not observe extension inventory")
	}
	if len(report.Extensions) == 0 {
		return report, errors.New("extension inventory was empty")
	}
	if err := collectObserverRows(ctx, conn, observerRolesSQL, func(rows pgx.Rows) error {
		var item RoleObservation
		if err := rows.Scan(&item.Name, &item.CanLogin, &item.Superuser, &item.CreateDB, &item.CreateRole, &item.Replication, &item.BypassRLS, &item.Inherit); err != nil {
			return err
		}
		report.Roles = append(report.Roles, item)
		return nil
	}); err != nil {
		return report, errors.New("could not observe fixed scratch roles")
	}
	if err := collectObserverRows(ctx, conn, observerMembershipsSQL, func(rows pgx.Rows) error {
		var item RoleMembershipObservation
		if err := rows.Scan(&item.GrantedRole, &item.Member, &item.AdminOption, &item.Inherit, &item.Set); err != nil {
			return err
		}
		report.Memberships = append(report.Memberships, item)
		return nil
	}); err != nil {
		return report, errors.New("could not observe fixed scratch role memberships")
	}
	var err error
	report.DatabaseACL, err = readObserverACL(ctx, conn, observerDatabaseACLSQL)
	if err != nil {
		return report, errors.New("could not observe database ACL")
	}
	report.PublicSchemaACL, err = readObserverACL(ctx, conn, observerPublicSchemaACLSQL)
	if err != nil {
		return report, errors.New("could not observe public schema ACL")
	}
	if err := collectObserverRows(ctx, conn, observerRelationOwnersSQL, func(rows pgx.Rows) error {
		var item OwnedObjectObservation
		if err := rows.Scan(&item.Schema, &item.Name, &item.Kind, &item.Owner, &item.ExtensionOwned); err != nil {
			return err
		}
		report.RelationOwners = append(report.RelationOwners, item)
		return nil
	}); err != nil {
		return report, errors.New("could not observe relation ownership")
	}
	if err := collectObserverRows(ctx, conn, observerFunctionOwnersSQL, func(rows pgx.Rows) error {
		var item OwnedObjectObservation
		item.Kind = "function"
		if err := rows.Scan(&item.Schema, &item.Name, &item.IdentityArguments, &item.Owner); err != nil {
			return err
		}
		report.FunctionOwners = append(report.FunctionOwners, item)
		return nil
	}); err != nil {
		return report, errors.New("could not observe function ownership")
	}
	if err := collectObserverRows(ctx, conn, observerSecurityDefinerSQL, func(rows pgx.Rows) error {
		var item SecurityDefinerObservation
		if err := rows.Scan(&item.Schema, &item.Name, &item.IdentityArguments, &item.Owner, &item.Configuration, &item.ExecuteACL); err != nil {
			return err
		}
		report.SecurityDefiner = append(report.SecurityDefiner, item)
		return nil
	}); err != nil {
		return report, errors.New("could not observe SECURITY DEFINER functions")
	}
	if err := conn.QueryRow(ctx, observerSpatialChecksSQL).Scan(&report.Spatial.GeometryValid, &report.Spatial.GeometryIntersects, &report.Spatial.GeographyDistanceMeters); err != nil {
		return report, errors.New("fixed spatial compatibility probes failed")
	}
	report.Spatial.ExpectedIndex = "public.reserva_ensayo_local_ubicacion_gist"
	if err := conn.QueryRow(ctx, observerSpatialIndexSQL).Scan(&report.Spatial.ActualIndex, &report.Spatial.IndexAccessMethod, &report.Spatial.IndexValid, &report.Spatial.IndexReady, &report.Spatial.IndexDefinition); err != nil {
		return report, errors.New("expected spatial index was not observable")
	}
	if membershipExpectation == RoleMembershipRetired {
		report.DatabaseACLPass = VerifyScratchRetiredDatabaseACL(ctx, conn) == nil
	} else {
		report.DatabaseACLPass = VerifyScratchDatabaseACL(ctx, conn) == nil
	}
	if membershipExpectation == RoleMembershipRetired {
		report.RuntimePermissionsPass = VerifyScratchRetiredRuntimePermissions(ctx, conn) == nil
	} else {
		report.RuntimePermissionsPass = VerifyScratchRuntimePermissions(ctx, conn) == nil
	}
	checks := [16]bool{}
	if err := conn.QueryRow(ctx, observerRuntimePermissionsSQL).Scan(
		&checks[0], &checks[1], &checks[2], &checks[3], &checks[4], &checks[5], &checks[6], &checks[7],
		&checks[8], &checks[9], &checks[10], &checks[11], &checks[12], &checks[13], &checks[14], &checks[15]); err != nil {
		return report, errors.New("could not observe fixed runtime permission checks")
	}
	checkNames := [...]string{"database_connect", "database_no_create", "database_no_temp", "public_schema_usage", "public_schema_no_create", "usuario_select", "usuario_insert", "usuario_update", "usuario_no_delete", "administrador_global_select", "administrador_global_no_insert", "administrador_global_no_update", "administrador_global_no_delete", "tarifa_espacio_no_update", "tarifa_espacio_no_delete", "schema_migrations_hidden"}
	for i, passed := range checks {
		report.RuntimeChecks = append(report.RuntimeChecks, PermissionCheck{Name: checkNames[i], Passed: passed})
	}
	report.RoleInventoryPass = roleInventoryPass(report.Roles, report.Memberships, membershipExpectation)
	report.Spatial.ChecksPass = report.Spatial.Passed()
	return report, nil
}

// Passed indicates whether the fixed positive checks succeeded. The ownership
// and SECURITY DEFINER inventories remain observations for human review.
func (r ScratchObservation) Passed() bool {
	if len(r.RuntimeChecks) != 16 {
		return false
	}
	for _, check := range r.RuntimeChecks {
		if !check.Passed {
			return false
		}
	}
	return r.ServerVersionNum > 0 && r.ServerVersion != "" && r.PostGISFullVersion != "" &&
		requiredExtensionsPresent(r.Extensions) && r.RoleInventoryPass && r.DatabaseACLPass &&
		r.RuntimePermissionsPass && r.Spatial.Passed()
}

func (s SpatialObservation) Passed() bool {
	return s.GeometryValid && s.GeometryIntersects &&
		s.GeographyDistanceMeters >= 110000 && s.GeographyDistanceMeters <= 112000 &&
		s.IndexAccessMethod == "gist" && s.IndexValid && s.IndexReady &&
		strings.HasSuffix(s.ActualIndex, "reserva_ensayo_local_ubicacion_gist") &&
		strings.Contains(strings.ToLower(s.IndexDefinition), "using gist (punto)")
}

func requiredExtensionsPresent(extensions []ExtensionObservation) bool {
	want := map[string]bool{"btree_gist": false, "postgis": false, "unaccent": false}
	for _, extension := range extensions {
		if _, ok := want[extension.Name]; ok && extension.Version != "" {
			want[extension.Name] = true
		}
	}
	for _, present := range want {
		if !present {
			return false
		}
	}
	return true
}

func roleInventoryPass(roles []RoleObservation, memberships []RoleMembershipObservation, expectation RoleMembershipExpectation) bool {
	byName := make(map[string]RoleObservation, len(roles))
	for _, role := range roles {
		byName[role.Name] = role
	}
	for _, name := range []string{"espacigo_gadm_bootstrap", "espacigo_runtime", "espacigo_scratch_migrator", "postgres"} {
		if _, ok := byName[name]; !ok {
			return false
		}
	}
	for _, name := range []string{"espacigo_gadm_bootstrap", "espacigo_runtime"} {
		role := byName[name]
		if role.CanLogin || role.Superuser || role.CreateDB || role.CreateRole || role.Replication || role.BypassRLS || role.Inherit {
			return false
		}
	}
	var migratorMemberships []RoleMembershipObservation
	for _, membership := range memberships {
		if membership.GrantedRole == "espacigo_gadm_bootstrap" || membership.Member == "espacigo_gadm_bootstrap" ||
			membership.GrantedRole == "espacigo_runtime" || membership.Member == "espacigo_runtime" {
			return false
		}
		if membership.Member == "espacigo_scratch_migrator" {
			migratorMemberships = append(migratorMemberships, membership)
		}
	}
	switch expectation {
	case RoleMembershipManagedCloudSQL:
		return byName["espacigo_scratch_migrator"].CanLogin && len(migratorMemberships) == 1 && migratorMemberships[0].GrantedRole == "cloudsqlsuperuser"
	case RoleMembershipLocalPostgres18:
		if !byName["espacigo_scratch_migrator"].CanLogin {
			return false
		}
		for _, membership := range migratorMemberships {
			if membership.GrantedRole == "cloudsqlsuperuser" {
				return false
			}
		}
		return true
	case RoleMembershipRetired:
		return !byName["espacigo_scratch_migrator"].CanLogin && len(migratorMemberships) == 0
	default:
		return false
	}
}

func collectObserverRows(ctx context.Context, conn *pgx.Conn, query string, scan func(pgx.Rows) error) error {
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := scan(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}

func readObserverACL(ctx context.Context, conn *pgx.Conn, query string) ([]ACLObservation, error) {
	items := make([]ACLObservation, 0)
	err := collectObserverRows(ctx, conn, query, func(rows pgx.Rows) error {
		var item ACLObservation
		if err := rows.Scan(&item.Grantee, &item.Grantor, &item.Privilege, &item.Grantable); err != nil {
			return err
		}
		items = append(items, item)
		return nil
	})
	return items, err
}
