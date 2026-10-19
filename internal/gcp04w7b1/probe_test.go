package gcp04w7b1

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCompilePinnedProbe(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "db", "query", "gcp04_w7_b1_actor_probe.sql"))
	if err != nil {
		t.Fatal(err)
	}
	query, err := CompileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(query) == 0 {
		t.Fatal("compiled query is empty")
	}
	if _, err := CompileQuery(append(source, []byte("\n-- changed\n")...)); err == nil {
		t.Fatal("modified SQL passed digest gate")
	}
}

func TestAssessReportsObservedAttributesWithoutAssumingCompatibility(t *testing.T) {
	ledgerGrant, ledgerOwner := true, false
	r := ProbeReport{
		Format:       Format,
		Target:       ProbeTarget{Database: Database, Session: Bootstrap, Current: Bootstrap, Version: 180000, ReadOnly: true},
		Actor:        &Actor{Name: Bootstrap, Login: true, Superuser: false, CreateDB: true, CreateRole: true, Inherit: true},
		Memberships:  []Membership{{GrantedRole: AdminRole, MemberRole: Bootstrap, Admin: false, Inherit: true, Set: true}},
		Capabilities: Capabilities{CreateRoleAttribute: true, DatabaseConnectEffectiveGO: true, SchemaUsageEffectiveGO: true, LedgerSelectEffectiveGO: &ledgerGrant, LedgerOwner: &ledgerOwner},
		Scope:        ProbeScope{},
	}
	data, _ := json.Marshal(r)
	result, err := Assess(data)
	if err != nil {
		t.Fatal(err)
	}
	attrs := result["actor_attributes_observed"].(*Actor)
	if attrs.Superuser {
		t.Fatal("test fixture expected SUPERUSER=false")
	}
	if !attrs.CreateDB || !attrs.CreateRole || !attrs.Inherit {
		t.Fatal("observed attributes were normalized or assumed")
	}
	if result["observer_bootstrap_compatibility"] != "not_accredited_by_b1_probe" || result["mutation_authorized"] != false {
		t.Fatalf("probe overclaimed compatibility or authority: %#v", result)
	}
	matrix := result["capability_matrix"].(map[string]any)
	if matrix["database_connect_effective_grant_option"] != true || matrix["database_connect_direct_grant_option"] != false {
		t.Fatalf("effective inherited capability was conflated with direct ACL: %#v", matrix)
	}
}

func TestAssessFailsClosedForWrongIdentityAndSuperuser(t *testing.T) {
	for name, mutate := range map[string]func(*ProbeReport){
		"different session":      func(r *ProbeReport) { r.Target.Session = "postgres" },
		"different current user": func(r *ProbeReport) { r.Target.Current = "other" },
		"superuser":              func(r *ProbeReport) { r.Actor.Superuser = true },
		"not read only":          func(r *ProbeReport) { r.Target.ReadOnly = false },
	} {
		t.Run(name, func(t *testing.T) {
			r := ProbeReport{Format: Format, Target: ProbeTarget{Database: Database, Session: Bootstrap, Current: Bootstrap, Version: 180000, ReadOnly: true}, Actor: &Actor{Name: Bootstrap, Login: true}, Scope: ProbeScope{}}
			mutate(&r)
			data, _ := json.Marshal(r)
			if _, err := Assess(data); err == nil {
				t.Fatal("unsafe report accepted")
			}
		})
	}
}
