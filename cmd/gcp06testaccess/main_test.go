package main

import (
	"net/url"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp06testaccess"
)

func TestCandidateDatabaseURLHasNoInlinePassword(t *testing.T) {
	dsn := databaseURL("gcp06_test_access_012345abcdef")
	if err := cloudsql.ValidateConnectionURL(dsn); err != nil {
		t.Fatalf("candidate reset URL rejected: %v", err)
	}
	parsed, err := url.Parse(dsn)
	if err != nil || parsed.Host != "localhost:5432" || parsed.Path != "/"+gcp06testaccess.Database {
		t.Fatal("candidate reset URL does not match pinned connector target")
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		t.Fatal("SQL password must never be embedded in the URL")
	}
}

func TestRunRejectsTargetDriftBeforeReadingAnySecret(t *testing.T) {
	getenv := func(key string) string {
		if key == "GCP06_RUN_ID" {
			return "20261021-gcp06b"
		}
		return ""
	}
	if err := run(nil, getenv); err == nil {
		t.Fatal("drifted candidate run was accepted")
	}
}
