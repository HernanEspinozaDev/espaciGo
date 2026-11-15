package migrator

import "testing"

func TestRequireV84MembershipsExactEdgesAndFlags(t *testing.T) {
	if err := requireV84Memberships([]V84Membership{
		{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Set: true},
		{Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Set: true},
	}); err != nil {
		t.Fatal(err)
	}
	for name, edges := range map[string][]V84Membership{
		"missing":             {{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Set: true}},
		"extra touching edge": {{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Set: true}, {Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Set: true}, {Role: "espacigo_schema_owner", Member: "other", Set: true}},
		"admin option":        {{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Set: true, Admin: true}, {Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Set: true}},
		"inherit option":      {{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Set: true, Inherit: true}, {Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Set: true}},
		"set disabled":        {{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login"}, {Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Set: true}},
	} {
		t.Run(name, func(t *testing.T) {
			if err := requireV84Memberships(edges); err == nil {
				t.Fatal("accepted membership contract mismatch")
			}
		})
	}
}
