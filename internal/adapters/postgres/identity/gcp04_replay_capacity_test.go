package postgres

import "testing"

func TestAssessGCP04ReplayCapacityRequiresWritablePG18AndExactRole(t *testing.T) {
	facts := GCP04ReplayCapacityFacts{
		Database: "postgres", CurrentUser: "postgres", ServerVersionNum: 180004,
		MaxConnections: 24, SuperuserReserved: 2, ReservedConnections: 1,
		OtherConnections: 4, RoleCanLogin: true, RoleCanCreateDatabase: true,
	}
	assessment, err := AssessGCP04ReplayCapacity(facts)
	if err != nil {
		t.Fatal(err)
	}
	if GCP04ReplayConnectionDemand != 17 || assessment.Demand != 17 || assessment.OrdinaryCapacity != 21 || assessment.AvailableOrdinarySlots != 17 {
		t.Fatalf("unexpected connection budget assessment: %+v", assessment)
	}
}

func TestAssessGCP04ReplayCapacityFailsBeforeMutationWhenDemandDoesNotFit(t *testing.T) {
	facts := GCP04ReplayCapacityFacts{
		Database: "postgres", CurrentUser: "postgres", ServerVersionNum: 180004,
		MaxConnections: 24, SuperuserReserved: 2, ReservedConnections: 1,
		OtherConnections: 5, RoleCanLogin: true, RoleCanCreateDatabase: true,
	}
	if assessment, err := AssessGCP04ReplayCapacity(facts); err == nil || assessment.AvailableOrdinarySlots != 16 {
		t.Fatalf("capacity one slot below the budget was accepted: assessment=%+v err=%v", assessment, err)
	}
}

func TestAssessGCP04ReplayCapacityRejectsWrongServerRoleAndRecovery(t *testing.T) {
	base := GCP04ReplayCapacityFacts{
		Database: "postgres", CurrentUser: "postgres", ServerVersionNum: 180004,
		MaxConnections: 24, SuperuserReserved: 2, ReservedConnections: 1,
		OtherConnections: 0, RoleCanLogin: true, RoleCanCreateDatabase: true,
	}
	tests := []struct {
		name   string
		mutate func(*GCP04ReplayCapacityFacts)
	}{
		{name: "wrong database", mutate: func(f *GCP04ReplayCapacityFacts) { f.Database = "app" }},
		{name: "wrong role", mutate: func(f *GCP04ReplayCapacityFacts) { f.CurrentUser = "runner" }},
		{name: "old server", mutate: func(f *GCP04ReplayCapacityFacts) { f.ServerVersionNum = 170006 }},
		{name: "role cannot login", mutate: func(f *GCP04ReplayCapacityFacts) { f.RoleCanLogin = false }},
		{name: "role cannot create database", mutate: func(f *GCP04ReplayCapacityFacts) { f.RoleCanCreateDatabase = false }},
		{name: "standby server", mutate: func(f *GCP04ReplayCapacityFacts) { f.InRecovery = true }},
		{name: "reserved slots exceed server limit", mutate: func(f *GCP04ReplayCapacityFacts) { f.ReservedConnections = 24 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			facts := base
			test.mutate(&facts)
			if _, err := AssessGCP04ReplayCapacity(facts); err == nil {
				t.Fatal("unsafe preflight facts were accepted")
			}
		})
	}
}
