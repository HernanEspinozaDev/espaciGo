package migrator

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestV97AuthorityManagerIsTestOnly(t *testing.T) {
	manager := runtime.FuncForPC(reflect.ValueOf(manageV97AuthorityWithConn).Pointer())
	if manager == nil {
		t.Fatal("V97 authority manager function metadata is unavailable")
	}
	file, _ := manager.FileLine(manager.Entry())
	if !strings.HasSuffix(file, "_test.go") {
		t.Fatalf("V97 authority manager is compiled from production file %q", file)
	}
}

func TestV97AuthorityManifestIsPinnedToTheExactV97Inventory(t *testing.T) {
	if v97AuthorityDatabasePrefix != "migrator_test_" || v97AuthorityUser != "postgres" {
		t.Fatal("V97 authority target constants changed")
	}
	if v97AuthorityMigration != "V000097__commercial_rental_quote_snapshots.sql" ||
		v97AuthorityChecksum != "0574033dde36b66efc00f36e20fcedbb424badc20c7109f941f78786fb5d8e14" ||
		v97AuthorityInventory != "134a3f0825eb1895acc564f2b05472ae83c625648791d2a8905c3500412d42a4" {
		t.Fatal("V97 authority manifest constants changed")
	}

	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateV97MigrationInventory(all); err != nil {
		t.Fatalf("valid exact V1..V97 inventory rejected: %v", err)
	}
	if err := validateV97MigrationInventory(all[:96]); err == nil {
		t.Fatal("V96-only inventory unexpectedly accepted for V97 authority")
	}
	changed := append([]Migration(nil), all...)
	changed[96].Checksum = strings.Repeat("0", 64)
	if err := validateV97MigrationInventory(changed); err == nil {
		t.Fatal("changed V97 checksum unexpectedly accepted")
	}
}

func TestV97AuthorityModeRequiresBaselineOnlyForFinalize(t *testing.T) {
	cases := []struct {
		mode     string
		baseline string
		wantErr  bool
	}{
		{mode: "inspect"},
		{mode: "prepare"},
		{mode: "finalize", baseline: strings.Repeat("a", 64)},
		{mode: "unknown", wantErr: true},
		{mode: "prepare", baseline: strings.Repeat("a", 64), wantErr: true},
		{mode: "finalize", baseline: strings.Repeat("A", 64), wantErr: true},
	}
	for _, test := range cases {
		t.Run(test.mode+"/"+test.baseline, func(t *testing.T) {
			err := validateV97AuthorityMode(test.mode, test.baseline)
			if (err != nil) != test.wantErr {
				t.Fatalf("validate mode returned %v, wantErr=%v", err, test.wantErr)
			}
		})
	}
}

func TestV97AuthorityGrantsOnlyExactDDLPrivileges(t *testing.T) {
	if v97GrantCreateSQL != `GRANT CREATE ON SCHEMA public TO espacigo_schema_owner` ||
		v97GrantUsuarioReferencesSQL != `GRANT REFERENCES (id) ON TABLE public.usuario TO espacigo_schema_owner` ||
		v97GrantEspacioReferencesSQL != `GRANT REFERENCES (id) ON TABLE public.espacio TO espacigo_schema_owner` {
		t.Fatal("V97 temporary DDL grants exceed or differ from the reviewed column-scoped contract")
	}
	if v97RevokeCreateSQL != `REVOKE CREATE ON SCHEMA public FROM espacigo_schema_owner` ||
		v97RevokeUsuarioReferencesSQL != `REVOKE REFERENCES (id) ON TABLE public.usuario FROM espacigo_schema_owner` ||
		v97RevokeEspacioReferencesSQL != `REVOKE REFERENCES (id) ON TABLE public.espacio FROM espacigo_schema_owner` {
		t.Fatal("V97 temporary DDL revocation contract changed")
	}
}
