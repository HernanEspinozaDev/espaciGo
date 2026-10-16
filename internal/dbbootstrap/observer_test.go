package dbbootstrap

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestObserverQueriesAreFixedReadOnlyAndTargetExpectedSpatialObjects(t *testing.T) {
	queries := []string{
		observerVersionsSQL, observerExtensionsSQL, observerRolesSQL,
		observerMembershipsSQL, observerDatabaseACLSQL, observerPublicSchemaACLSQL,
		observerRelationOwnersSQL, observerFunctionOwnersSQL,
		observerSecurityDefinerSQL, observerSpatialChecksSQL, observerSpatialIndexSQL,
		observerRuntimePermissionsSQL,
	}
	for i, query := range queries {
		normalized := strings.ToUpper(strings.TrimSpace(query))
		if fields := strings.Fields(normalized); len(fields) == 0 || fields[0] != "SELECT" {
			t.Fatalf("observer query %d is not a fixed SELECT", i)
		}
		for _, forbidden := range []string{" INSERT ", " UPDATE ", " DELETE ", " CREATE ", " DROP ", " ALTER ", " TRUNCATE ", " CALL ", " DO "} {
			if strings.Contains(" "+normalized+" ", forbidden) {
				t.Fatalf("observer query %d contains forbidden operation %q", i, forbidden)
			}
		}
		if strings.Contains(query, "$1") || strings.Contains(query, "%s") {
			t.Fatalf("observer query %d accepts a parameterized SQL fragment", i)
		}
	}
	if !strings.Contains(observerSpatialIndexSQL, "reserva_ensayo_local_ubicacion_gist") ||
		!strings.Contains(observerSpatialChecksSQL, "ST_IsValid") ||
		!strings.Contains(observerSpatialChecksSQL, "ST_Intersects") ||
		!strings.Contains(observerSpatialChecksSQL, "ST_Distance") {
		t.Fatal("fixed observer lost its spatial compatibility probes")
	}
}

func TestScratchObservationPassesOnlyWhenFixedChecksPass(t *testing.T) {
	report := ScratchObservation{
		ServerVersionNum:   180006,
		ServerVersion:      "18.6",
		PostGISFullVersion: "POSTGIS=3.6.4",
		Extensions: []ExtensionObservation{
			{Name: "btree_gist", Version: "1.8"}, {Name: "postgis", Version: "3.6.4"}, {Name: "unaccent", Version: "1.1"},
		},
		RoleInventoryPass:      true,
		DatabaseACLPass:        true,
		RuntimePermissionsPass: true,
		Spatial: SpatialObservation{
			GeometryValid: true, GeometryIntersects: true,
			GeographyDistanceMeters: 111000, IndexAccessMethod: "gist",
			IndexValid: true, IndexReady: true, ActualIndex: "reserva_ensayo_local_ubicacion_gist",
			IndexDefinition: "CREATE INDEX ... USING gist (punto)", ChecksPass: true,
		},
		RuntimeChecks: passingRuntimeChecks(),
	}
	if !report.Passed() {
		t.Fatal("complete positive fixed observations should pass")
	}
	report.Spatial.IndexReady = false
	if report.Passed() {
		t.Fatal("an unready spatial index must fail the observer")
	}
}

func TestRoleInventoryRequiresSafeGADMAndOnlyCloudSQLSuperuserForMigrator(t *testing.T) {
	roles := []RoleObservation{
		{Name: "espacigo_gadm_bootstrap"}, {Name: "espacigo_runtime"},
		{Name: "espacigo_scratch_migrator", CanLogin: true}, {Name: "postgres", Superuser: true},
	}
	valid := []RoleMembershipObservation{{GrantedRole: "cloudsqlsuperuser", Member: "espacigo_scratch_migrator"}}
	if !roleInventoryPass(roles, valid) {
		t.Fatal("reviewed role inventory should pass")
	}
	if roleInventoryPass(roles, append(valid, RoleMembershipObservation{GrantedRole: "espacigo_runtime", Member: "espacigo_gadm_bootstrap"})) {
		t.Fatal("GADM/runtime membership must fail the role inventory")
	}
	if roleInventoryPass(roles, []RoleMembershipObservation{{GrantedRole: "cloudsqlsuperuser", Member: "espacigo_scratch_migrator"}, {GrantedRole: "pg_monitor", Member: "espacigo_scratch_migrator"}}) {
		t.Fatal("an unexpected migrator membership must fail the role inventory")
	}
}

func passingRuntimeChecks() []PermissionCheck {
	checks := make([]PermissionCheck, 16)
	for i := range checks {
		checks[i] = PermissionCheck{Name: "fixed", Passed: true}
	}
	return checks
}

func TestScratchObservationJSONContainsOnlyDeclaredSafeFields(t *testing.T) {
	report := ScratchObservation{
		ServerVersionNum:   180006,
		ServerVersion:      "18.6",
		PostGISFullVersion: "POSTGIS=3.6.4",
		Extensions: []ExtensionObservation{
			{Name: "btree_gist", Version: "1.8"}, {Name: "postgis", Version: "3.6.4"}, {Name: "unaccent", Version: "1.1"},
		},
		RoleInventoryPass: true,
		SecurityDefiner: []SecurityDefinerObservation{{
			Schema: "public", Name: "fixed_fn", IdentityArguments: "p_id uuid", Owner: "espacigo_scratch_migrator",
			Configuration: []string{"search_path=pg_catalog, public"}, ExecuteACL: []string{"PUBLIC:EXECUTE:grantable=false"},
		}},
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	text := string(encoded)
	for _, want := range []string{"postgis_full_version", "security_definer_functions", "search_path=pg_catalog, public", "PUBLIC:EXECUTE"} {
		if !strings.Contains(text, want) {
			t.Fatalf("observer output omitted %q", want)
		}
	}
	for _, forbidden := range []string{"password", "token", "DATABASE_URL", "secret_payload", "row_data"} {
		if strings.Contains(strings.ToLower(text), strings.ToLower(forbidden)) {
			t.Fatalf("observer output leaked forbidden field marker %q", forbidden)
		}
	}
}
