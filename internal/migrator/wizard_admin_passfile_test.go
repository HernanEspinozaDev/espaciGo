package migrator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWizardPassfileGuard(t *testing.T) {
	// The guard resolves the approved location from cwd; isolate cwd and env.
	for _, key := range []string{"PGPASSFILE", "PGOPTIONS", "PGSERVICE", "PGSERVICEFILE", "PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD", "PGSSLMODE"} {
		old, had := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if had {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}

	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, string, string)
		wantOK  bool
	}{
		{name: "correct-passfile", wantOK: true},
		{name: "missing-env", prepare: func(t *testing.T, root, pass string) {}},
		{name: "other-path", prepare: func(t *testing.T, root, pass string) { t.Setenv("PGPASSFILE", filepath.Join(root, "other")) }},
		{name: "relative-path", prepare: func(t *testing.T, root, pass string) {
			t.Setenv("PGPASSFILE", ".local/testing/wizard-improvements/postgres/pgpass")
		}},
		{name: "symlink-file", prepare: func(t *testing.T, root, pass string) {
			_ = os.Remove(pass)
			_ = os.Symlink(filepath.Join(root, "dummy"), pass)
			t.Setenv("PGPASSFILE", pass)
		}},
		{name: "symlink-parent", prepare: func(t *testing.T, root, pass string) {
			dir := filepath.Dir(pass)
			_ = os.Remove(pass)
			_ = os.Remove(dir)
			_ = os.Symlink(filepath.Join(root, "elsewhere"), dir)
			t.Setenv("PGPASSFILE", pass)
		}},
		{name: "mode-0644", prepare: func(t *testing.T, root, pass string) { _ = os.Chmod(pass, 0644); t.Setenv("PGPASSFILE", pass) }},
		{name: "dir-0755", prepare: func(t *testing.T, root, pass string) {
			_ = os.Chmod(filepath.Join(root, ".local"), 0755)
			t.Setenv("PGPASSFILE", pass)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			oldwd, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chdir(root); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(oldwd) })
			pass := filepath.Join(root, ".local", "testing", "wizard-improvements", "postgres", "pgpass")
			for _, d := range []string{filepath.Join(root, ".local"), filepath.Join(root, ".local", "testing"), filepath.Join(root, ".local", "testing", "wizard-improvements"), filepath.Dir(pass)} {
				if err := os.Mkdir(d, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(pass, []byte("127.0.0.1:15436:db:user:synthetic-placeholder\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "dummy"), []byte("dummy"), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.prepare != nil {
				tc.prepare(t, root, pass)
			} else {
				t.Setenv("PGPASSFILE", pass)
			}
			err = validateWizardPassfile()
			if (err == nil) != tc.wantOK {
				t.Fatalf("validateWizardPassfile() error state mismatch: got err=%v", err)
			}
		})
	}
}
