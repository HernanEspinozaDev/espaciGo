package main

import (
	"net/url"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
)

func TestDatabaseURLUsesPinnedConnectorShapeWithoutInlinePassword(t *testing.T) {
	dsn := databaseURL(runtimeLogin)
	if err := cloudsql.ValidateConnectionURL(dsn); err != nil {
		t.Fatalf("candidate bootstrap URL rejected: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("candidate bootstrap URL did not parse")
	}
	if parsed.User.Username() != runtimeLogin || parsed.Host != "localhost:5432" || parsed.Path != "/"+databaseName {
		t.Fatal("candidate bootstrap URL differs from the pinned connector contract")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		t.Fatal("database password must never be embedded in the URL")
	}
}

func TestValidateTargetPinsGCP06OwnerAndActivationLogin(t *testing.T) {
	if err := validateTarget(
		runID, connectionName, databaseName, "gadm_activation_012345abcdef",
		"GCP06 BOOTSTRAP 20261020-gcp06a hernan.espinoza.dev@gmail.com",
	); err != nil {
		t.Fatalf("expected exact candidate target to pass: %v", err)
	}
}

func TestValidateTargetRejectsDriftedScope(t *testing.T) {
	base := []string{
		runID, connectionName, databaseName, "gadm_activation_012345abcdef",
		"GCP06 BOOTSTRAP 20261020-gcp06a hernan.espinoza.dev@gmail.com",
	}
	cases := []struct {
		name  string
		index int
		value string
	}{
		{"run", 0, "20261021-gcp06b"},
		{"instance", 1, "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"},
		{"database", 2, "espacigo"},
		{"login", 3, "postgres"},
		{"confirmation", 4, "yes"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			args := append([]string(nil), base...)
			args[test.index] = test.value
			if err := validateTarget(args[0], args[1], args[2], args[3], args[4]); err == nil {
				t.Fatal("expected target mismatch rejection")
			}
		})
	}
}
