package mediapg

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIMG01FTargetGuardAllowsOnlyExactSyntheticTarget(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want bool
	}{
		{"proposed tmpfs target", "postgres://postgres@127.0.0.1:15432/espacigo_f6_synth_20261028?sslmode=disable", true},
		{"persistent native database", "postgres://postgres@127.0.0.1:5432/espacigo_dms_stage_20261020?sslmode=disable", false},
		{"different database", "postgres://postgres@127.0.0.1:15432/other?sslmode=disable", false},
		{"non-loopback host", "postgres://postgres@example.invalid:15432/espacigo_f6_synth_20261028?sslmode=disable", false},
		{"different role", "postgres://espacigo_runtime@127.0.0.1:15432/espacigo_f6_synth_20261028?sslmode=disable", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := pgxpool.ParseConfig(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			got := img01fTargetAllowed(cfg.ConnConfig.Host, cfg.ConnConfig.Port, cfg.ConnConfig.Database, cfg.ConnConfig.User)
			if got != tc.want {
				t.Fatalf("target allowed=%t, want %t", got, tc.want)
			}
		})
	}
}
