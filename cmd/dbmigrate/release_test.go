package main

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestReleaseRequiresEmbeddedSHAAndRejectsOtherModes(t *testing.T) {
	previous := releaseSourceSHA
	releaseSourceSHA = strings.Repeat("a", 40)
	t.Cleanup(func() { releaseSourceSHA = previous })
	sha, fingerprint := releaseSourceSHA, strings.Repeat("b", 64)
	if err := validateReleaseInvocation(true, false, sha, fingerprint, 83); err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		enabled, conflicting bool
		sha, fingerprint     string
		count                int
	}{
		{false, false, sha, fingerprint, 83}, {true, true, sha, fingerprint, 83}, {true, false, strings.Repeat("c", 40), fingerprint, 83}, {true, false, sha, "invalid", 83}, {true, false, sha, fingerprint, 82},
	} {
		if validateReleaseInvocation(check.enabled, check.conflicting, check.sha, check.fingerprint, check.count) == nil {
			t.Fatal("unsafe release invocation accepted")
		}
	}
	releaseSourceSHA = ""
	if validateReleaseInvocation(true, false, sha, fingerprint, 83) == nil {
		t.Fatal("unlabelled binary accepted")
	}
}

func TestReleaseRejectsOtherTargetsBeforeReadingSecretOrConnecting(t *testing.T) {
	for _, settings := range []connectionSettings{
		{mode: "direct"},
		{mode: "cloudsql", privateIP: true, instance: "other:region:instance"},
		{mode: "cloudsql", privateIP: true, instance: "espacigo-testprod:southamerica-west1:espacigo-testprod-db", databaseURL: "postgres://espacigo_api_login@localhost/espacigo_dms_stage_20261020", setRole: "espacigo_runtime"},
		{mode: "cloudsql", privateIP: true, instance: "espacigo-testprod:southamerica-west1:espacigo-testprod-db", databaseURL: "postgres://espacigo_migrator_login@localhost/other", setRole: "espacigo_schema_owner"},
	} {
		if runCloudRelease(context.Background(), io.Discard, settings, t.TempDir(), strings.Repeat("a", 40), strings.Repeat("b", 64), 83) == nil {
			t.Fatal("wrong target accepted")
		}
	}
}
