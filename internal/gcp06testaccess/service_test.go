package gcp06testaccess

import "testing"

func TestValidateTargetPinsOnlyCandidateAndEphemeralLogin(t *testing.T) {
	if err := ValidateTarget(RunID, Instance, Database, "gcp06_test_access_012345abcdef"); err != nil {
		t.Fatalf("exact candidate rejected: %v", err)
	}
	for _, test := range []struct{ run, instance, database, login string }{
		{"20261021-gcp06b", Instance, Database, "gcp06_test_access_012345abcdef"},
		{RunID, "espacigo-testprod:southamerica-west1:espacigo-testprod-pg", Database, "gcp06_test_access_012345abcdef"},
		{RunID, Instance, "espacigo", "gcp06_test_access_012345abcdef"},
		{RunID, Instance, Database, "postgres"},
	} {
		if err := ValidateTarget(test.run, test.instance, test.database, test.login); err == nil {
			t.Fatalf("unsafe candidate target accepted: %+v", test)
		}
	}
}

func TestTargetSetDigestIsStableAndOrderIndependent(t *testing.T) {
	ids := []string{"00000000-0000-4000-8000-000000000002", "00000000-0000-4000-8000-000000000001"}
	want := TargetSetDigest(ids)
	if got := TargetSetDigest([]string{ids[1], ids[0]}); got != want {
		t.Fatalf("digest depends on row order: %s != %s", got, want)
	}
	if got := TargetSetDigest([]string{ids[0]}); got == want {
		t.Fatal("digest did not change when the cohort changed")
	}
}
