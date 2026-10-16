package gcp03scratch

import (
	"strings"
	"testing"
)

func TestEmbeddedPhasesContainFixedScratchGuards(t *testing.T) {
	for name, sql := range map[string]string{
		"pre-migrate": PreMigrate,
		"retire":      RetireMigrator,
	} {
		if !strings.Contains(sql, "espacigo_scratch") || !strings.Contains(sql, "espacigo_scratch_migrator") {
			t.Errorf("%s SQL is missing exact scratch guards", name)
		}
		if strings.Contains(sql, "\\set") {
			t.Errorf("%s SQL contains a psql-only directive", name)
		}
	}
}
