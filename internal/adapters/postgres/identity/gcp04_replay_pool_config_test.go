package postgres

import "testing"

func TestGCP04ReplayPoolConfigFixesMaxConnsIndependentOfCPUAndURL(t *testing.T) {
	for _, databaseURL := range []string{
		"postgres://postgres@localhost:5432/postgres?sslmode=disable",
		"postgres://postgres@localhost:5432/postgres?sslmode=disable&pool_max_conns=24",
	} {
		config, err := GCP04ReplayPoolConfig(databaseURL)
		if err != nil {
			t.Fatalf("gcp04ReplayPoolConfig(%q): %v", databaseURL, err)
		}
		if config.MaxConns != 4 {
			t.Fatalf("MaxConns = %d, want fixed cap 4 for %q", config.MaxConns, databaseURL)
		}
	}
}
