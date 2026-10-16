package gcp03scratch

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
)

func TestRetiredVerificationNDJSONRoundTripsLargeNestedReportsAndValidatesIntegrity(t *testing.T) {
	for _, passed := range []bool{true, false} {
		t.Run(strconv.FormatBool(passed), func(t *testing.T) {
			report := largeRetiredVerificationReport(passed)
			canonical, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if len(canonical) <= 256*1024 {
				t.Fatalf("fixture must exceed Cloud Logging entry size: %d bytes", len(canonical))
			}
			var output bytes.Buffer
			if err := dbbootstrap.WriteIntegrityNDJSON(&output, report, report.Passed); err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			if len(lines) < 3 || output.Len() <= 256*1024 {
				t.Fatalf("large nested report was not split: records=%d bytes=%d", len(lines), output.Len())
			}
			for _, line := range lines {
				if len(line) > 32*1024 {
					t.Fatalf("record exceeds fixed bound: %d bytes", len(line))
				}
			}
			var reconstructed RetiredVerificationReport
			gotPassed, err := dbbootstrap.ReadIntegrityNDJSON(bytes.NewReader(output.Bytes()), &reconstructed)
			if err != nil {
				t.Fatal(err)
			}
			if gotPassed != passed || !reflect.DeepEqual(reconstructed, report) {
				t.Fatalf("complete nested report round trip mismatch: pass=%t/%t owners=%d/%d ACL=%v/%v",
					gotPassed, passed, len(reconstructed.Observation.FunctionOwners), len(report.Observation.FunctionOwners),
					reconstructed.SchemaHistoryACL, report.SchemaHistoryACL)
			}

			missing := append([]string(nil), lines[:len(lines)-1]...)
			if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(missing, "\n")+"\n"), &reconstructed); err == nil {
				t.Fatal("missing report chunk was accepted")
			}

			duplicate := append([]string(nil), lines...)
			duplicate = append(duplicate, lines[1])
			if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(duplicate, "\n")+"\n"), &reconstructed); err == nil {
				t.Fatal("duplicate report chunk was accepted")
			}

			corrupted := append([]string(nil), lines...)
			marker := `"data":"`
			position := strings.Index(corrupted[1], marker)
			if position < 0 {
				t.Fatal("payload data field is missing")
			}
			position += len(marker)
			if corrupted[1][position] == 'A' {
				corrupted[1] = corrupted[1][:position] + "B" + corrupted[1][position+1:]
			} else {
				corrupted[1] = corrupted[1][:position] + "A" + corrupted[1][position+1:]
			}
			if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(corrupted, "\n")+"\n"), &reconstructed); err == nil {
				t.Fatal("corrupted report chunk was accepted")
			}
		})
	}
}

func TestRetiredMembershipRevocationNDJSONRoundTripsAndValidatesManifestStatus(t *testing.T) {
	for _, passed := range []bool{true, false} {
		t.Run(strconv.FormatBool(passed), func(t *testing.T) {
			report := largeRetiredMembershipRevocationReport(passed)
			canonical, err := json.Marshal(report)
			if err != nil {
				t.Fatal(err)
			}
			if len(canonical) <= 256*1024 {
				t.Fatalf("nested revocation fixture must exceed Cloud Logging entry size: %d bytes", len(canonical))
			}
			var output bytes.Buffer
			if err := dbbootstrap.WriteIntegrityNDJSON(&output, report, report.Passed); err != nil {
				t.Fatal(err)
			}
			var reconstructed RetiredMembershipRevocationReport
			gotPassed, err := dbbootstrap.ReadIntegrityNDJSON(bytes.NewReader(output.Bytes()), &reconstructed)
			if err != nil || gotPassed != passed || !reflect.DeepEqual(reconstructed, report) {
				t.Fatalf("full nested revocation report round trip mismatch: pass=%t/%t err=%v", gotPassed, passed, err)
			}

			lines := strings.Split(strings.TrimSpace(output.String()), "\n")
			manifestStatus := `"passed":true`
			changedStatus := `"passed":false`
			if !passed {
				manifestStatus, changedStatus = changedStatus, manifestStatus
			}
			if !strings.Contains(lines[0], manifestStatus) {
				t.Fatal("manifest pass status was not found")
			}
			lines[0] = strings.Replace(lines[0], manifestStatus, changedStatus, 1)
			if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(lines, "\n")+"\n"), &reconstructed); err == nil {
				t.Fatal("manifest pass state that disagreed with the nested revocation payload was accepted")
			}

			corrupted := strings.Split(strings.TrimSpace(output.String()), "\n")
			marker := `"data":"`
			position := strings.Index(corrupted[1], marker)
			if position < 0 {
				t.Fatal("payload data field is missing")
			}
			position += len(marker)
			replacement := "A"
			if corrupted[1][position] == 'A' {
				replacement = "B"
			}
			corrupted[1] = corrupted[1][:position] + replacement + corrupted[1][position+1:]
			if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(corrupted, "\n")+"\n"), &reconstructed); err == nil {
				t.Fatal("corrupted nested revocation payload was accepted")
			}
		})
	}
}

func largeRetiredMembershipRevocationReport(passed bool) RetiredMembershipRevocationReport {
	post := largeRetiredVerificationReport(passed)
	return RetiredMembershipRevocationReport{
		PrecheckPassed: passed, ExpectedMembership: true, MutationAttempted: true,
		RevokeStatementSucceeded: passed, Revoked: passed, Postcheck: &post,
		DiagnosticsCollected: true, Passed: passed,
	}
}

func largeRetiredVerificationReport(passed bool) RetiredVerificationReport {
	observation := dbbootstrap.ScratchObservation{
		ServerVersionNum:   180006,
		ServerVersion:      "PostgreSQL 18.6",
		PostGISFullVersion: "PostGIS 3.6.4",
		Extensions: []dbbootstrap.ExtensionObservation{
			{Name: "btree_gist", Version: "1.8"}, {Name: "postgis", Version: "3.6.4"}, {Name: "unaccent", Version: "1.1"},
		},
		Roles: []dbbootstrap.RoleObservation{
			{Name: "espacigo_gadm_bootstrap"},
			{Name: "espacigo_runtime"},
			{Name: MigratorLogin},
			{Name: "postgres", CanLogin: true},
		},
		MembershipExpectation:  dbbootstrap.RoleMembershipRetired,
		RoleInventoryPass:      true,
		DatabaseACLPass:        true,
		RuntimePermissionsPass: true,
		Spatial: dbbootstrap.SpatialObservation{
			GeometryValid: true, GeometryIntersects: true, GeographyDistanceMeters: 110574.39,
			ExpectedIndex: "public.reserva_ensayo_local_ubicacion_gist", ActualIndex: "reserva_ensayo_local_ubicacion_gist",
			IndexAccessMethod: "gist", IndexValid: true, IndexReady: true,
			IndexDefinition: "CREATE INDEX reserva_ensayo_local_ubicacion_gist ON public.reserva_ensayo_local_ubicacion_sintetica USING gist (punto)",
			ChecksPass:      true,
		},
	}
	checkNames := []string{
		"database_connect", "database_no_create", "database_no_temp", "public_schema_usage", "public_schema_no_create",
		"usuario_select", "usuario_insert", "usuario_update", "usuario_no_delete", "administrador_global_select",
		"administrador_global_no_insert", "administrador_global_no_update", "administrador_global_no_delete",
		"tarifa_espacio_no_update", "tarifa_espacio_no_delete", "schema_migrations_hidden",
	}
	for _, name := range checkNames {
		observation.RuntimeChecks = append(observation.RuntimeChecks, dbbootstrap.PermissionCheck{Name: name, Passed: true})
	}
	for i := 0; i < 1031; i++ {
		observation.FunctionOwners = append(observation.FunctionOwners, dbbootstrap.OwnedObjectObservation{
			Schema: "public", Name: "function_" + strconv.Itoa(i), Kind: "function",
			IdentityArguments: "argument_" + strings.Repeat("signature_type, ", 20), Owner: "gcp03_lifecycle_admin",
		})
	}
	return RetiredVerificationReport{
		Observation: observation,
		SchemaHistoryACL: []dbbootstrap.ACLObservation{{
			Grantee: "postgres", Grantor: MigratorLogin, Privilege: "SELECT", Grantable: false,
		}},
		MigratorCanTemp: false, MigratorCanCreate: false, DatabaseOwner: "postgres",
		PublicSchemaOwner: "pg_database_owner", HistoryOwner: MigratorLogin,
		MigrationHistory: true, MigratorRetired: passed, NoRoleMemberships: passed,
		OwnerHistoryACL: true, Passed: passed,
	}
}
