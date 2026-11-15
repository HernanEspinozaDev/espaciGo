package migrator

import (
	"encoding/json"
	"testing"
)

func TestV84CatalogGuards(t *testing.T) {
	cases := []struct {
		name   string
		change func(*V84DDLAuthority)
	}{
		{"missing catalog", func(a *V84DDLAuthority) { a.Catalog = nil }},
		{"unsafe lookup order", func(a *V84DDLAuthority) { a.Catalog.Schemas = []string{"public", "pg_catalog"} }},
		{"temp schema", func(a *V84DDLAuthority) { a.Catalog.Schemas = []string{"pg_temp_1", "pg_catalog", "public"} }},
		{"unknown owner", func(a *V84DDLAuthority) { a.UsuarioOwner = "other" }},
		{"mismatched ledger owner", func(a *V84DDLAuthority) { a.LedgerOwner = "other" }},
		{"unexpected schema owner", func(a *V84DDLAuthority) { a.PublicSchemaOwner = "other" }},
		{"database create", func(a *V84DDLAuthority) { a.Catalog.DatabaseCreate = true }},
		{"database owned by login", func(a *V84DDLAuthority) { a.Catalog.DatabaseOwner = a.SessionUser }},
		{"database owned by effective role", func(a *V84DDLAuthority) { a.Catalog.DatabaseOwner = a.CurrentUser }},
		{"missing ACL evidence", func(a *V84DDLAuthority) { a.Catalog.ACL = nil }},
		{"public temporary grant option", func(a *V84DDLAuthority) {
			a.Catalog.ACL = append(a.Catalog.ACL, V84ACL{Object: "database", Owner: a.Catalog.DatabaseOwner, Grantee: "PUBLIC", Grantor: a.Catalog.DatabaseOwner, Privilege: "TEMPORARY", Grantable: true})
		}},
		{"public temporary granted by non-owner", func(a *V84DDLAuthority) {
			a.Catalog.ACL = append(a.Catalog.ACL, V84ACL{Object: "database", Owner: a.Catalog.DatabaseOwner, Grantee: "PUBLIC", Grantor: "other", Privilege: "TEMPORARY"})
		}},
		{"helper login", func(a *V84DDLAuthority) { a.Catalog.HelperRole.CanLogin = true }},
		{"helper inherit", func(a *V84DDLAuthority) { a.Catalog.HelperRole.Inherit = true }},
		{"helper superuser", func(a *V84DDLAuthority) { a.Catalog.HelperRole.Superuser = true }},
		{"helper create role", func(a *V84DDLAuthority) { a.Catalog.HelperRole.CreateRole = true }},
		{"helper create db", func(a *V84DDLAuthority) { a.Catalog.HelperRole.CreateDB = true }},
		{"helper replication", func(a *V84DDLAuthority) { a.Catalog.HelperRole.Replication = true }},
		{"helper bypass rls", func(a *V84DDLAuthority) { a.Catalog.HelperRole.BypassRLS = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			payload, _ := json.Marshal(validV84DDLAuthority())
			var a V84DDLAuthority
			if err := json.Unmarshal(payload, &a); err != nil {
				t.Fatal(err)
			}
			tc.change(&a)
			if err := requireV84DDLAuthority(a); err == nil {
				t.Fatal("unsafe catalog accepted")
			}
		})
	}
}

func TestV84CatalogAllowsDefaultPublicTemporaryDatabasePrivilege(t *testing.T) {
	a := validV84DDLAuthority()
	owner := a.Catalog.DatabaseOwner
	a.Catalog.ACL = append(a.Catalog.ACL, V84ACL{
		Object:    "database",
		Owner:     owner,
		Grantee:   "PUBLIC",
		Grantor:   owner,
		Privilege: "TEMPORARY",
	})
	if err := requireV84DDLAuthority(a); err != nil {
		t.Fatalf("default non-grantable PUBLIC TEMPORARY ACL should remain an accepted baseline: %v", err)
	}
}
