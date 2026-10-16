package gcp03scratch

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
)

func TestRetiredVerificationNDJSONRoundTripsCompleteDiagnostics(t *testing.T) {
	report := RetiredVerificationReport{
		Observation: dbbootstrap.ScratchObservation{
			Roles:                  []dbbootstrap.RoleObservation{{Name: MigratorLogin, CanLogin: false, Inherit: false}},
			Memberships:            []dbbootstrap.RoleMembershipObservation{{GrantedRole: "cloudsqlsuperuser", Member: MigratorLogin, AdminOption: false, Inherit: true, Set: true}},
			DatabaseACL:            []dbbootstrap.ACLObservation{{Grantee: "postgres", Grantor: "owner", Privilege: "CONNECT"}},
			PublicSchemaACL:        []dbbootstrap.ACLObservation{{Grantee: "postgres", Grantor: "owner", Privilege: "USAGE"}},
			RuntimeChecks:          []dbbootstrap.PermissionCheck{{Name: "database_no_temp", Passed: false}},
			MembershipExpectation:  dbbootstrap.RoleMembershipRetired,
			RoleInventoryPass:      false,
			DatabaseACLPass:        false,
			RuntimePermissionsPass: false,
		},
		SchemaHistoryACL:  make([]dbbootstrap.ACLObservation, 450),
		MigratorCanTemp:   true,
		MigratorCanCreate: true,
		DatabaseOwner:     "postgres",
		PublicSchemaOwner: "pg_database_owner",
		HistoryOwner:      MigratorLogin,
		MigrationHistory:  true,
		MigratorRetired:   false,
		NoRoleMemberships: false,
		OwnerHistoryACL:   false,
		Passed:            false,
	}
	for i := range report.SchemaHistoryACL {
		report.SchemaHistoryACL[i] = dbbootstrap.ACLObservation{Grantee: "postgres", Grantor: MigratorLogin, Privilege: "SELECT", Grantable: false}
	}
	var output bytes.Buffer
	if err := dbbootstrap.WriteIntegrityNDJSON(&output, report, report.Passed); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) < 3 {
		t.Fatalf("large history ACL should be split into bounded records, got %d lines", len(lines))
	}
	for _, line := range lines {
		if len(line) > 32*1024 {
			t.Fatalf("record exceeds fixed bound: %d bytes", len(line))
		}
	}
	var reconstructed RetiredVerificationReport
	passed, err := dbbootstrap.ReadIntegrityNDJSON(bytes.NewReader(output.Bytes()), &reconstructed)
	if err != nil {
		t.Fatal(err)
	}
	if passed || reconstructed.Passed || reconstructed.MigratorCanTemp != report.MigratorCanTemp ||
		reconstructed.MigratorCanCreate != report.MigratorCanCreate || reconstructed.HistoryOwner != report.HistoryOwner ||
		len(reconstructed.SchemaHistoryACL) != len(report.SchemaHistoryACL) ||
		reconstructed.SchemaHistoryACL[0] != report.SchemaHistoryACL[0] ||
		reconstructed.Observation.Memberships[0] != report.Observation.Memberships[0] {
		t.Fatalf("round trip lost report diagnostics: %#v", reconstructed)
	}

	corrupted := false
	for i, line := range lines {
		if strings.Contains(line, `"section":"schema_migrations_acl"`) {
			lines[i] = strings.Replace(line, `"SELECT"`, `"DELETE"`, 1)
			corrupted = true
			break
		}
	}
	if !corrupted {
		t.Fatal("expected a serialized schema-history ACL section")
	}
	if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(lines, "\n")+"\n"), &reconstructed); err == nil {
		t.Fatal("corrupted history ACL record was accepted")
	}

	lines = strings.Split(strings.TrimSpace(output.String()), "\n")
	removed := false
	for i := 1; i < len(lines); i++ {
		if strings.Contains(lines[i], `"section":"schema_migrations_acl"`) {
			lines = append(lines[:i], lines[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		t.Fatal("expected a serialized schema-history ACL section")
	}
	if _, err := dbbootstrap.ReadIntegrityNDJSON(strings.NewReader(strings.Join(lines, "\n")+"\n"), &reconstructed); err == nil {
		t.Fatal("missing history ACL record was accepted")
	}
}
