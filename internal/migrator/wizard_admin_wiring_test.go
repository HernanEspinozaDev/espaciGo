package migrator

import "testing"

func TestWizardAdminEngineVersionGuard(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version int
		wantErr bool
	}{
		{name: "PostgreSQL 18.6", version: 180006},
		{name: "PostgreSQL 18.7", version: 180007, wantErr: true},
		{name: "PostgreSQL 17", version: 170000, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := guardWizardServerVersion(tc.version)
			if (err != nil) != tc.wantErr {
				t.Fatalf("guardWizardServerVersion(%d) error = %v, wantErr %t", tc.version, err, tc.wantErr)
			}
		})
	}
}
