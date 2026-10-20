package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestConfigRequiresPinnedIdentityBudgetAndSingleConcurrency(t *testing.T) {
	cfg := config{
		ProjectID: expectedProject, ConnectionName: expectedConnectionName, ExpectedPrivateIP: expectedPrivateIP,
		Database: expectedDatabase, DatabaseUser: expectedDatabaseUser, PasswordFile: "/mounted/password",
		SecretVersion: "12", RequiredConnections: 24, Timeout: 2 * time.Minute, Concurrency: 1,
		QueryFile: "/target_preflight.sql",
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("valid target rejected: %v", err)
	}
	cfg.ConnectionName = "other-project:southamerica-west1:espacigo-testprod-pg"
	if err := cfg.validate(); err == nil {
		t.Fatal("connection name from another project passed")
	}
	cfg.ConnectionName = expectedConnectionName
	cfg.Concurrency = 2
	if err := cfg.validate(); err == nil {
		t.Fatal("concurrency above one passed")
	}
	cfg.Concurrency = 1
	cfg.SecretVersion = "latest"
	if err := cfg.validate(); err == nil {
		t.Fatal("non-numeric secret version passed")
	}
}

func TestPublicEffectivePrivilegesRequireReviewedBaseline(t *testing.T) {
	baseline := publicEffectivePrivileges{
		DatabaseConnect: true, PublicSchemaUsage: true,
	}
	if !baseline.matchesReviewedBaseline() {
		t.Fatal("reviewed PUBLIC ACL baseline was rejected")
	}
	for name, mutate := range map[string]func(*publicEffectivePrivileges){
		"database connect":     func(p *publicEffectivePrivileges) { p.DatabaseConnect = false },
		"database create":      func(p *publicEffectivePrivileges) { p.DatabaseCreate = true },
		"database temporary":   func(p *publicEffectivePrivileges) { p.DatabaseTemporary = true },
		"public schema usage":  func(p *publicEffectivePrivileges) { p.PublicSchemaUsage = false },
		"public schema create": func(p *publicEffectivePrivileges) { p.PublicSchemaCreate = true },
	} {
		t.Run(name, func(t *testing.T) {
			changed := baseline
			mutate(&changed)
			if changed.matchesReviewedBaseline() {
				t.Fatal("unexpected PUBLIC effective privilege was accepted")
			}
		})
	}
}

func TestCompileQueryPinsAndTranslatesPsqlWrapper(t *testing.T) {
	queryPath := filepath.Join("..", "..", "db", "query", "gcp04_w4_target_preflight.sql")
	source, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatal(err)
	}
	query, digest, err := compileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	if digest != expectedQuerySHA256 {
		t.Fatalf("query digest = %s", digest)
	}
	if got := countSubstrings(query, "$1"); got != 3 {
		t.Fatalf("query parameter count = %d, want 3", got)
	}
	if got := countSubstrings(query, "\\"); got != 0 {
		t.Fatalf("query retained %d psql meta commands", got)
	}
	if _, _, err := compileQuery(append(source, []byte("-- unreviewed change\n")...)); err == nil {
		t.Fatal("changed query passed its pinned digest")
	}
}

func TestPasswordFileRequiresProtectedRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "password")
	if err := os.WriteFile(path, []byte("synthetic-test-password\n"), 0600); err != nil {
		t.Fatal(err)
	}
	password, err := readPasswordFile(path)
	if err != nil || password != "synthetic-test-password" {
		t.Fatalf("protected password file rejected: %v", err)
	}
	if err := os.Chmod(path, 0664); err != nil {
		t.Fatal(err)
	}
	if _, err := readPasswordFile(path); err == nil {
		t.Fatal("group/world-writable password file passed")
	}
}

func TestValidateRemoteAddressPinsObservedPrivateIP(t *testing.T) {
	got, err := validateRemoteAddress(&net.TCPAddr{IP: net.ParseIP(expectedPrivateIP), Port: 5432}, expectedPrivateIP)
	if err != nil || got != expectedPrivateIP {
		t.Fatalf("pinned connector address rejected: ip=%q err=%v", got, err)
	}
	if _, err := validateRemoteAddress(&net.TCPAddr{IP: net.ParseIP("10.189.0.4"), Port: 5432}, expectedPrivateIP); err == nil {
		t.Fatal("connector address mismatch passed")
	} else {
		var mismatch *remoteAddressMismatchError
		if !errors.As(err, &mismatch) || mismatch.Expected != expectedPrivateIP || mismatch.Observed != "10.189.0.4" {
			t.Fatalf("connector mismatch evidence is incomplete: %#v", err)
		}
	}
	if _, err := validateRemoteAddress(&net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5432}, expectedPrivateIP); err == nil {
		t.Fatal("loopback connector address passed")
	}
}

func TestLocalObserverReadOnlyIntegration(t *testing.T) {
	dsn := os.Getenv("GCP04_W7_TEST_DSN")
	if dsn == "" {
		t.Skip("requires the dedicated local PostgreSQL observer fixture")
	}
	queryPath := os.Getenv("GCP04_W7_TEST_QUERY_FILE")
	if queryPath == "" {
		t.Fatal("GCP04_W7_TEST_QUERY_FILE is required with the local DSN")
	}
	required := 1
	if value := os.Getenv("GCP04_W7_TEST_REQUIRED_CONNECTIONS"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &required); err != nil || required < 1 {
			t.Fatal("local integration budget must be a positive integer")
		}
	}
	localDatabaseUser := os.Getenv("GCP04_W7_TEST_DATABASE_USER")
	if localDatabaseUser == "" {
		localDatabaseUser = "espacigo_observer_login"
	}
	source, err := os.ReadFile(queryPath)
	if err != nil {
		t.Fatal(err)
	}
	query, _, err := compileQuery(source)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("local observer connection failed: %v", err)
	}
	defer conn.Close(context.Background())
	var databaseName, databaseUser string
	if err := conn.QueryRow(ctx, "SELECT current_database(), current_user").Scan(&databaseName, &databaseUser); err != nil {
		t.Fatal(err)
	}
	if databaseName != expectedDatabase || databaseUser != localDatabaseUser {
		t.Fatalf("local fixture connected as %q/%q instead of the dedicated read-only observer", databaseName, databaseUser)
	}
	report, gateFailed, err := runForLocalIntegration(ctx, conn, query, required, databaseName, databaseUser)
	if err != nil {
		t.Fatalf("local observer preflight failed: %v", err)
	}
	if gateFailed {
		validation, validationErr := validateReport(report, config{Database: databaseName, DatabaseUser: databaseUser, RequiredConnections: required})
		var decoded struct {
			ACLEntries []reportACLEntry                          `json:"acl_entries"`
			Database   struct{ Owner, PublicSchemaOwner string } `json:"database"`
		}
		_ = json.Unmarshal(report, &decoded)
		var observerACL []reportACLEntry
		for _, entry := range decoded.ACLEntries {
			if entry.Grantee == "espacigo_db_observer" {
				observerACL = append(observerACL, entry)
			}
		}
		t.Fatalf("local fixture rejected budget: validation=%+v validation_err=%v db_owner=%s schema_owner=%s observer_acl=%+v report_len=%d", validation, validationErr, decoded.Database.Owner, decoded.Database.PublicSchemaOwner, observerACL, len(report))
	}
	localConfig := config{Database: databaseName, DatabaseUser: databaseUser, RequiredConnections: required}
	validReport, err := validateReport(report, localConfig)
	if err != nil || !validReport.ObserverContractPass || !validReport.CapacityPass {
		t.Fatalf("legitimate local observer report was not accepted: validation=%+v err=%v", validReport, err)
	}
	var mutated map[string]any
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	publicPrivileges := mutated["public_effective_privileges"].(map[string]any)
	publicPrivileges["database_temporary"] = true
	mutatedPublic, _ := json.Marshal(mutated)
	validation, err := validateReport(mutatedPublic, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("unexpected PUBLIC effective privileges passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	memberships := mutated["memberships"].([]any)
	memberships = append(memberships, map[string]any{
		"app_login": nil, "granted_role": "espacigo_db_observer", "member_role": "gcp04_temporary_provisioner",
		"admin_option": true, "inherit_option": false, "set_option": false,
		"edge_context": "direct-edge-touches-application-role",
	})
	mutated["memberships"] = memberships
	provisionerEdge, _ := json.Marshal(mutated)
	validation, err = validateReport(provisionerEdge, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer group with an additional temporary provisioner membership passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	delete(mutated, "destination")
	incomplete, _ := json.Marshal(mutated)
	if _, err := validateReport(incomplete, localConfig); err == nil {
		t.Fatal("inventory without destination details passed schema validation")
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	delete(mutated, "owner_effective_privileges")
	incomplete, _ = json.Marshal(mutated)
	if _, err := validateReport(incomplete, localConfig); err == nil {
		t.Fatal("inventory without effective ACL details passed schema validation")
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	aclEntries := mutated["acl_entries"].([]any)
	aclEntries = append(aclEntries, map[string]any{"kind": "relation", "identity": "public.schema_migrations", "object_owner": "espacigo_schema_owner", "grantor": "espacigo_schema_owner", "grantee": "espacigo_db_observer", "privilege": "DELETE", "grantable": false})
	mutated["acl_entries"] = aclEntries
	excessiveDelete, _ := json.Marshal(mutated)
	validation, err = validateReport(excessiveDelete, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer group with ledger DELETE passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	aclEntries = mutated["acl_entries"].([]any)
	aclEntries = append(aclEntries, map[string]any{"kind": "schema", "identity": "public", "object_owner": "espacigo_schema_owner", "grantor": "espacigo_schema_owner", "grantee": "espacigo_db_observer", "privilege": "CREATE", "grantable": false})
	mutated["acl_entries"] = aclEntries
	excessiveCreate, _ := json.Marshal(mutated)
	validation, err = validateReport(excessiveCreate, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer group with schema CREATE passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	aclEntries = mutated["acl_entries"].([]any)
	aclEntries = append(aclEntries, map[string]any{"kind": "default", "identity": "public:r", "object_owner": "espacigo_schema_owner", "grantor": "espacigo_schema_owner", "grantee": "espacigo_db_observer", "privilege": "INSERT", "grantable": false})
	mutated["acl_entries"] = aclEntries
	defaultWrite, _ := json.Marshal(mutated)
	validation, err = validateReport(defaultWrite, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer default write ACL passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	aclEntries = mutated["acl_entries"].([]any)
	for i, raw := range aclEntries {
		entry := raw.(map[string]any)
		if entry["grantee"] == "espacigo_db_observer" && entry["kind"] == "schema" {
			entry["grantable"] = true
			aclEntries[i] = entry
			break
		}
	}
	mutated["acl_entries"] = aclEntries
	grantOption, _ := json.Marshal(mutated)
	validation, err = validateReport(grantOption, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer grant option passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	database := mutated["database"].(map[string]any)
	database["owner"] = "espacigo_db_observer"
	ownedDatabase, _ := json.Marshal(mutated)
	validation, err = validateReport(ownedDatabase, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer group owning the database passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	destination := mutated["destination"].(map[string]any)
	relations := destination["relation_inventory"].([]any)
	relations[0].(map[string]any)["owner"] = databaseUser
	ownedObject, _ := json.Marshal(mutated)
	validation, err = validateReport(ownedObject, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer login owning a database object passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	database = mutated["database"].(map[string]any)
	database["public_schema_owner"] = databaseUser
	ownedSchema, _ := json.Marshal(mutated)
	validation, err = validateReport(ownedSchema, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer login owning public schema passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	destination = mutated["destination"].(map[string]any)
	relations = destination["relation_inventory"].([]any)
	relations[0].(map[string]any)["owner"] = "espacigo_db_observer"
	ownedObject, _ = json.Marshal(mutated)
	validation, err = validateReport(ownedObject, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer group owning a database object passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	roles := mutated["roles"].([]any)
	for _, item := range roles {
		role := item.(map[string]any)
		if role["name"] == databaseUser {
			role["createdb"] = true
		}
	}
	overprivileged, _ := json.Marshal(mutated)
	validation, err = validateReport(overprivileged, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("overprivileged observer passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	memberships = mutated["memberships"].([]any)
	memberships = append(memberships, map[string]any{"app_login": databaseUser, "granted_role": "cloudsqlsuperuser", "member_role": databaseUser, "admin_option": false, "inherit_option": true, "set_option": false, "edge_context": "transitive-login-membership"})
	mutated["memberships"] = memberships
	overprivilegedMembership, _ := json.Marshal(mutated)
	validation, err = validateReport(overprivilegedMembership, localConfig)
	if err != nil || validation.ObserverContractPass {
		t.Fatalf("observer with unexpected administrative membership passed: validation=%+v err=%v", validation, err)
	}
	if err := json.Unmarshal(report, &mutated); err != nil {
		t.Fatal(err)
	}
	capacity := mutated["capacity"].(map[string]any)
	capacity["free_slots_lower_bound"] = capacity["free_slots_lower_bound"].(float64) + 1
	inconsistentCapacity, _ := json.Marshal(mutated)
	if _, err := validateReport(inconsistentCapacity, localConfig); err == nil {
		t.Fatal("internally inconsistent connection capacity passed")
	}
	var decoded map[string]any
	if err := json.Unmarshal(report, &decoded); err != nil || decoded["database"] == nil {
		t.Fatalf("local report is not a database inventory: %v", err)
	}
}

func countSubstrings(value, substring string) int {
	count := 0
	for len(value) > 0 {
		index := strings.Index(value, substring)
		if index < 0 {
			break
		}
		count++
		value = value[index+len(substring):]
	}
	return count
}
