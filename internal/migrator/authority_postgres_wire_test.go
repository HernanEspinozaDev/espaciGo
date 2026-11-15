package migrator

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
)

// Exercise the production contract/authority validator with an unchanged Plan
// collected from the marked PostgreSQL fixture, not hand-built authority data.
// This intentionally does not claim the fixture is the operational cloud target.
func assertPostgresAuthorityPythonWire(t *testing.T, ctx context.Context, migrationURL, database string, migrations []Migration, version int) {
	t.Helper()
	plan, err := Plan(ctx, migrationURL, prefixMigrationDirForCombined(t, migrations, version), "espacigo_schema_owner")
	if err != nil {
		t.Fatalf("collect PostgreSQL authority plan before V%d: %v", version, err)
	}
	if plan.Database != database || !plan.ReadOnly || !plan.HistoryExists || len(plan.Applied) != version-1 || len(plan.Pending) != 1 || plan.Pending[0].Version != int64(version) {
		t.Fatalf("unexpected PostgreSQL wire boundary before V%d", version)
	}
	payload := struct {
		Plan     PlanResult         `json:"plan"`
		Expected []MigrationSummary `json:"expected"`
	}{Plan: plan}
	for _, migration := range migrations[:version] {
		payload.Expected = append(payload.Expected, MigrationSummary{Version: migration.Version, Name: migration.Name, Checksum: migration.Checksum})
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	const script = `import json,sys
sys.path.insert(0,sys.argv[1])
from migration_release_manifest import validate_plan_contract
payload=json.load(sys.stdin)
plan=payload['plan']
assert plan['database']==sys.argv[2] and plan['database'].startswith('migrator_test_')
assert plan['read_only'] is True and plan['history_exists'] is True
assert plan['applied']==payload['expected'][:-1]
assert plan['pending']==payload['expected'][-1:]
validate_plan_contract(plan)
# Explicit negative mutation only: never rewrite the positive catalog report.
missing={key:value for key,value in plan.items() if key not in ('ddl_authority','identity_ddl_authority')}
try:
 validate_plan_contract(missing)
except ValueError as error:
 assert str(error)=='invalid migration plan contract',str(error)
else:
 raise AssertionError('real catalog authority could be omitted')
`
	cmd := exec.CommandContext(ctx, "python3", "-c", script, filepath.Join("..", "..", "scripts", "gcp"), database)
	cmd.Stdin = bytes.NewReader(encoded)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PostgreSQL authority Go JSON rejected before V%d: %v %s", version, err, output)
	}
}
