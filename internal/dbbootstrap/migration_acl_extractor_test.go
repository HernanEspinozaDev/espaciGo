package dbbootstrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractMigrationRuntimeACL(t *testing.T) {
	cases := []struct {
		name, src string
		want      int
		reject    bool
	}{
		{"plain", "GRANT SELECT ON TABLE public.items TO espacigo_runtime;", 1, false},
		{"dynamic", "DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN EXECUTE 'REVOKE DELETE ON public.items FROM espacigo_runtime'; END IF; END $$;", 1, false},
		{"conditional", "DO $x$ BEGIN IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='espacigo_runtime') THEN GRANT SELECT ON TABLE public.items TO espacigo_runtime; END IF; END $x$;", 1, false},
		{"unknown", "DO $$ DECLARE target_role text := 'espacigo_runtime'; BEGIN EXECUTE format('GRANT SELECT ON TABLE public.items TO %I', target_role); END $$;", 0, true},
		{"dynamic_concatenation", "DO $$ DECLARE target_role text := 'espacigo_runtime'; BEGIN EXECUTE 'GRANT SELECT ON TABLE public.items TO ' || quote_ident(target_role); END $$;", 0, true},
		{"nested_comment", "/* outer /* inner */ GRANT SELECT ON TABLE public.items TO espacigo_runtime; */", 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractMigrationRuntimeACL(1, []byte(c.src))
			if c.reject {
				if err == nil {
					t.Fatal("expected fail-closed rejection")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != c.want {
				t.Fatalf("got %d effects", len(got))
			}
		})
	}
}

func TestCheckedInMigrationRuntimeACLIsExtractedOrRejected(t *testing.T) {
	files, err := filepath.Glob("../../db/migrations/V*.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		_, err = ExtractMigrationRuntimeACL(0, b)
		if err != nil {
			t.Errorf("%s: %v", f, err)
		}
		if _, err := ExtractMigrationACLForGrantee(0, b, "PUBLIC"); err != nil {
			t.Errorf("%s PUBLIC ACL: %v", f, err)
		}
	}
}

func TestV80RuntimeACLIsExtracted(t *testing.T) {
	path := filepath.Join("../../db/migrations", "V000080__document_verification_space_ownership_supersession.sql")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	effects, err := ExtractMigrationRuntimeACL(80, source)
	if err != nil {
		t.Fatal(err)
	}
	want := RuntimeACLEffect{
		Action: "GRANT", ObjectKind: "TABLE", Object: "public.document_verification_case",
		Privilege: "UPDATE", Column: "superseded_at", Condition: "role-exists:espacigo_runtime",
	}
	for _, effect := range effects {
		if effect == want {
			return
		}
	}
	t.Fatalf("V80 runtime grant not extracted; got %d effects", len(effects))
}

func TestExtractMigrationRuntimeACLRejectsEffectsOutsideRoleCondition(t *testing.T) {
	source := []byte(`DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime') THEN
        EXECUTE 'GRANT SELECT ON TABLE public.conditional_items TO espacigo_runtime';
    END IF;
END $$;
GRANT UPDATE ON TABLE public.unconditional_items TO espacigo_runtime;`)
	if _, err := ExtractMigrationRuntimeACL(83, source); err == nil || !strings.Contains(err.Error(), "condition") {
		t.Fatalf("mixed conditional and unconditional ACL effects must fail closed, got %v", err)
	}
}

func TestExtractMigrationRuntimeACLRejectsUnknownACLBranchCondition(t *testing.T) {
	source := []byte(`DO $$ BEGIN
    IF current_setting('application_name') = 'test' THEN
        EXECUTE 'GRANT SELECT ON TABLE public.conditional_items TO espacigo_runtime';
    END IF;
END $$;`)
	if _, err := ExtractMigrationRuntimeACL(83, source); err == nil || !strings.Contains(err.Error(), "condition") {
		t.Fatalf("unknown procedural ACL condition must fail closed, got %v", err)
	}
}

func TestExtractMigrationACLForGranteeMatchesTargetClause(t *testing.T) {
	source := []byte(`GRANT SELECT ON TABLE public.items TO another_role;
REVOKE ALL ON FUNCTION public.purge_expired(integer) FROM PUBLIC;`)
	effects, err := ExtractMigrationACLForGrantee(1, source, "PUBLIC")
	if err != nil {
		t.Fatal(err)
	}
	want := RuntimeACLEffect{
		Action: "REVOKE", Grantee: "PUBLIC", ObjectKind: "FUNCTION",
		Object: "public.purge_expired(integer)", Privilege: "ALL PRIVILEGES",
	}
	if len(effects) != 1 || effects[0] != want {
		t.Fatalf("got %+v, want [%+v]", effects, want)
	}
}

func TestExtractMigrationACLForGranteeIgnoresACLTextInNonExecutableSQL(t *testing.T) {
	for _, source := range []string{
		`SELECT 'GRANT SELECT ON TABLE public.items TO PUBLIC';`,
		`SELECT $$GRANT SELECT ON TABLE public.items TO PUBLIC$$;`,
		`SELECT $$EXECUTE 'REVOKE ALL ON FUNCTION public.purge_expired(integer) FROM PUBLIC'$$;`,
		`SELECT 1 /* GRANT SELECT ON TABLE public.items TO PUBLIC */;`,
		`DO $$ BEGIN PERFORM 'EXECUTE ''REVOKE ALL ON FUNCTION public.purge_expired(integer) FROM PUBLIC'''; END $$;`,
	} {
		effects, err := ExtractMigrationACLForGrantee(1, []byte(source), "PUBLIC")
		if err != nil {
			t.Errorf("rejected unrelated SQL text %q: %v", source, err)
			continue
		}
		if len(effects) != 0 {
			t.Errorf("extracted ACL from unrelated SQL text %q: %+v", source, effects)
		}
	}
}

func TestV81V82PublicACLIsExtracted(t *testing.T) {
	for _, tc := range []struct {
		version int
		name    string
		want    int
	}{
		{81, "V000081__document_verification_runtime_scope_helpers.sql", 4},
		{82, "V000082__document_verification_retention_precedence.sql", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("../../db/migrations", tc.name))
			if err != nil {
				t.Fatal(err)
			}
			effects, err := ExtractMigrationACLForGrantee(tc.version, source, "PUBLIC")
			if err != nil {
				t.Fatal(err)
			}
			if len(effects) != tc.want {
				t.Fatalf("got %d PUBLIC ACL effects, want %d: %+v", len(effects), tc.want, effects)
			}
			for _, effect := range effects {
				if effect.Action != "REVOKE" || effect.Grantee != "PUBLIC" {
					t.Errorf("unexpected PUBLIC ACL effect: %+v", effect)
				}
			}
		})
	}
}

func TestExtractMigrationACLForGranteeRejectsDynamicTargets(t *testing.T) {
	for _, source := range []string{
		`DO $$ DECLARE target_role text := 'PUBLIC'; BEGIN EXECUTE 'GRANT SELECT ON TABLE public.items TO ' || quote_ident(target_role); END $$;`,
		`DO $$ DECLARE target_role text := 'PUBLIC'; BEGIN EXECUTE format('GRANT SELECT ON TABLE public.items TO %s', target_role); END $$;`,
		`DO $$ DECLARE acl_sql text := 'REVOKE ALL ON FUNCTION public.purge_expired(integer) FROM PUBLIC'; BEGIN EXECUTE acl_sql; END $$;`,
	} {
		if _, err := ExtractMigrationACLForGrantee(1, []byte(source), "PUBLIC"); err == nil || !strings.Contains(err.Error(), "unsupported dynamic ACL") {
			t.Errorf("dynamic PUBLIC target was not rejected explicitly: err=%v source=%s", err, source)
		}
	}
}
