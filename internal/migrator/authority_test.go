package migrator

import "testing"

func validV84DDLAuthority() V84DDLAuthority {
	return V84DDLAuthority{
		SessionUser:         "espacigo_migrator_login",
		CurrentUser:         "espacigo_schema_owner",
		CurrentSchema:       "public",
		SessionRole:         V84RoleAttributes{CanLogin: true},
		SchemaOwnerRole:     V84RoleAttributes{},
		PublicSchemaOwner:   "pg_database_owner",
		PublicUsage:         true,
		PublicCreate:        true,
		UsuarioOwner:        "espacigo_schema_owner",
		UsuarioIDReferences: true,
		LedgerOwner:         "espacigo_schema_owner",
		LedgerSelect:        true,
		LedgerInsert:        true,
		Memberships:         append([]V84Membership(nil), v84MembershipContract...),
		Catalog:             &V84AuthorityCatalog{Schemas: []string{"pg_catalog", "public"}, DatabaseOwner: "postgres", ACL: []V84ACL{{Object: "public", Owner: "pg_database_owner", Grantor: "pg_database_owner", Grantee: "PUBLIC", Privilege: "USAGE"}}},
	}
}

func TestRequireV84DDLAuthorityChecksIdentityAndEffectivePrivileges(t *testing.T) {
	if err := requireV84DDLAuthority(validV84DDLAuthority()); err != nil {
		t.Fatalf("valid effective authority rejected: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(*V84DDLAuthority)
	}{
		{name: "wrong session user", mutate: func(a *V84DDLAuthority) { a.SessionUser = "postgres" }},
		{name: "wrong current user", mutate: func(a *V84DDLAuthority) { a.CurrentUser = "postgres" }},
		{name: "wrong schema", mutate: func(a *V84DDLAuthority) { a.CurrentSchema = "other" }},
		{name: "session role is superuser", mutate: func(a *V84DDLAuthority) { a.SessionRole.Superuser = true }},
		{name: "session role cannot log in", mutate: func(a *V84DDLAuthority) { a.SessionRole.CanLogin = false }},
		{name: "session role inherits privileges", mutate: func(a *V84DDLAuthority) { a.SessionRole.Inherit = true }},
		{name: "session role can create roles", mutate: func(a *V84DDLAuthority) { a.SessionRole.CreateRole = true }},
		{name: "session role can create databases", mutate: func(a *V84DDLAuthority) { a.SessionRole.CreateDB = true }},
		{name: "session role can replicate", mutate: func(a *V84DDLAuthority) { a.SessionRole.Replication = true }},
		{name: "session role can bypass row security", mutate: func(a *V84DDLAuthority) { a.SessionRole.BypassRLS = true }},
		{name: "schema owner can log in", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.CanLogin = true }},
		{name: "schema owner is superuser", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.Superuser = true }},
		{name: "schema owner can bypass row security", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.BypassRLS = true }},
		{name: "schema owner inherits privileges", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.Inherit = true }},
		{name: "schema owner can create roles", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.CreateRole = true }},
		{name: "schema owner can create databases", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.CreateDB = true }},
		{name: "schema owner can replicate", mutate: func(a *V84DDLAuthority) { a.SchemaOwnerRole.Replication = true }},
		{name: "missing schema usage", mutate: func(a *V84DDLAuthority) { a.PublicUsage = false }},
		{name: "missing schema create", mutate: func(a *V84DDLAuthority) { a.PublicCreate = false }},
		{name: "missing referenced column privilege", mutate: func(a *V84DDLAuthority) { a.UsuarioIDReferences = false }},
		{name: "missing ledger select", mutate: func(a *V84DDLAuthority) { a.LedgerSelect = false }},
		{name: "missing ledger insert", mutate: func(a *V84DDLAuthority) { a.LedgerInsert = false }},
		{name: "missing owner evidence", mutate: func(a *V84DDLAuthority) { a.UsuarioOwner = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			authority := validV84DDLAuthority()
			tc.mutate(&authority)
			if err := requireV84DDLAuthority(authority); err == nil {
				t.Fatal("invalid V84 DDL authority was accepted")
			}
		})
	}
}

func TestV84AuthorityRejectsUnreviewedMemberships(t *testing.T) {
	a := validV84DDLAuthority()
	a.Memberships = []V84Membership{{Role: "pg_write_all_data", Member: "espacigo_schema_owner", Set: true}}
	if err := requireV84DDLAuthority(a); err == nil {
		t.Fatal("V84 gate accepted an unreviewed privilege path")
	}
}

func TestRequireV86DDLAuthorityUsesOnlyRequiredSchemaAndLedgerPrivileges(t *testing.T) {
	authority := validV84DDLAuthority()
	authority.UsuarioIDReferences = false
	if err := requireV86DDLAuthority(authority); err != nil {
		t.Fatalf("V86 rejected authority without unused usuario REFERENCES privilege: %v", err)
	}
	authority.PublicCreate = false
	if err := requireV86DDLAuthority(authority); err == nil {
		t.Fatal("V86 accepted authority without schema CREATE")
	}
}

func TestRequireV87DDLAuthorityUsesOwnerAndLedgerWithoutSchemaCreate(t *testing.T) {
	authority := validV84DDLAuthority()
	authority.PublicCreate = false
	authority.UsuarioIDReferences = false
	if err := requireV87DDLAuthority(authority); err != nil {
		t.Fatalf("V87 rejected owner/ledger authority without unused CREATE or REFERENCES: %v", err)
	}
	authority.LedgerInsert = false
	if err := requireV87DDLAuthority(authority); err == nil {
		t.Fatal("V87 accepted authority without migration-ledger INSERT")
	}
}
