package main

import (
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
)

func TestRuntimeDatabaseURLMatchesCloudSQLConnectorContract(t *testing.T) {
	dsn := runtimeDatabaseURL()
	if err := cloudsql.ValidateConnectionURL(dsn); err != nil {
		t.Fatalf("runtime DSN rejected by Cloud SQL URL guard: %v", err)
	}

	config, err := cloudsql.NewPGXConfig(dsn, "test-password")
	if err != nil {
		t.Fatalf("runtime DSN rejected by pgx config builder: %v", err)
	}
	if config.Host != "localhost" || config.Port != 5432 || config.User != gcp04w7b1.Bootstrap || config.Database != gcp04w7b1.Database {
		t.Fatalf("unexpected Cloud SQL connector config: host=%q port=%d user=%q database=%q", config.Host, config.Port, config.User, config.Database)
	}
	if strings.Contains(dsn, gcp04w7b1.PrivateIP) || config.Host == gcp04w7b1.PrivateIP {
		t.Fatal("Cloud SQL connector must dial localhost and let the private-IP connector select the database address")
	}
}
