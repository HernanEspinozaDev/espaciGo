package migrator

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// This exercises real Go JSON and Python validation with synthetic authority.
// It does not provide PostgreSQL catalog or effective-privilege evidence.
func TestMigrationAuthorityGoJSONPythonContract(t *testing.T) {
	all, err := DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil {
		t.Fatal(err)
	}
	const script = `import hashlib,json,sys
sys.path.insert(0,sys.argv[1])
from migration_release_manifest import FORMAT,IMAGES,REGISTRY,TARGET,check_plan
payload=json.load(sys.stdin)
rows=payload['migrations']
inventory={'format':'espacigo.migrations.inventory.v1','migrations':rows,'inventory_sha256':hashlib.sha256(''.join(f"{r['checksum']}  {r['name']}\n" for r in rows).encode()).hexdigest()}
manifest={'format':FORMAT,'source_sha':'a'*40,'target':TARGET.copy(),'images':{n:REGISTRY+'/'+r+'@sha256:'+'b'*64 for n,r in IMAGES.items()},'migration_inventory':inventory}
assert check_plan(manifest,payload['plan'])['pending']==1
`
	for _, version := range []int{84, 95, 96} {
		t.Run(strconv.Itoa(version), func(t *testing.T) {
			plan := PlanResult{Database: "espacigo_dms_stage_20261020", SessionUser: "espacigo_migrator_login", CurrentUser: "espacigo_schema_owner", ServerVersion: "18.7", ReadOnly: true, HistoryExists: true}
			var rows []MigrationSummary
			for i, migration := range all[:version] {
				row := MigrationSummary{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum}
				rows = append(rows, row)
				if i < version-1 {
					plan.Applied = append(plan.Applied, HistoryEntry{Version: row.Version, Name: row.Name, Checksum: row.Checksum})
				} else {
					plan.Pending = append(plan.Pending, row)
				}
			}
			if version == 84 {
				authority := validV84DDLAuthority()
				plan.DDLAuthority = &authority
			} else {
				authority := validIdentityTailDDLAuthority()
				plan.IdentityDDLAuthority = &authority
			}
			run := func(plan PlanResult) ([]byte, error) {
				payload, err := json.Marshal(struct {
					Plan       PlanResult         `json:"plan"`
					Migrations []MigrationSummary `json:"migrations"`
				}{plan, rows})
				if err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "python3", "-c", script, filepath.Join("..", "..", "scripts", "gcp"))
				cmd.Stdin = bytes.NewReader(payload)
				return cmd.CombinedOutput()
			}
			if output, err := run(plan); err != nil {
				t.Fatalf("Go authority JSON rejected by Python: %v %s", err, output)
			}
			plan.DDLAuthority, plan.IdentityDDLAuthority = nil, nil
			if output, err := run(plan); err == nil || !bytes.Contains(output, []byte("invalid migration plan contract")) {
				t.Fatalf("expected strict authority-omission rejection: err=%v output=%s", err, output)
			}
		})
	}
}
