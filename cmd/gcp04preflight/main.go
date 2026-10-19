package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

const (
	expectedProject          = "espacigo-testprod"
	expectedConnectionName   = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	expectedPrivateIP        = "10.189.0.3"
	expectedDatabase         = "espacigo"
	expectedDatabaseUser     = "espacigo_gcp04_w7_observer"
	expectedQuerySHA256      = "d0d3101667aff59899e231ca4ecd27c66358fd4e83c4615aae97ab873b4c2701"
	defaultQueryPath         = "/target_preflight.sql"
	maximumTimeout           = 15 * time.Minute
	maximumCredentialsLength = 4096
)

var numericSecretVersion = regexp.MustCompile(`^[1-9][0-9]{0,18}$`)

type config struct {
	ProjectID           string
	ConnectionName      string
	ExpectedPrivateIP   string
	Database            string
	DatabaseUser        string
	PasswordFile        string
	SecretVersion       string
	RequiredConnections int
	Timeout             time.Duration
	Concurrency         int
	QueryFile           string
}

type output struct {
	Format               string          `json:"format"`
	Status               string          `json:"status"`
	Target               targetOutput    `json:"target"`
	Network              networkOutput   `json:"network"`
	QuerySHA256          string          `json:"query_sha256"`
	ObserverContractPass bool            `json:"observer_contract_pass"`
	CapacityGatePass     bool            `json:"capacity_gate_pass"`
	DestinationCandidate bool            `json:"destination_candidate_observed"`
	RestoreApproval      string          `json:"restore_approval"`
	Report               json.RawMessage `json:"report"`
}

type targetOutput struct {
	ProjectID           string `json:"project_id"`
	InstanceConnection  string `json:"instance_connection_name"`
	PrivateIP           string `json:"private_ip"`
	Database            string `json:"database"`
	DatabaseUser        string `json:"database_user"`
	SecretVersion       string `json:"secret_version"`
	RequiredConnections int    `json:"required_connections"`
}

type networkOutput struct {
	ExpectedPrivateIP string `json:"expected_private_ip"`
	ObservedRemoteIP  string `json:"observed_remote_ip"`
}

func main() {
	cfg := configFromEnvironment()
	var timeoutSeconds int
	flag.IntVar(&cfg.RequiredConnections, "required-connections", cfg.RequiredConnections, "approved minimum free connection budget")
	flag.IntVar(&timeoutSeconds, "timeout-seconds", int(cfg.Timeout.Seconds()), "whole-run deadline in seconds")
	flag.IntVar(&cfg.Concurrency, "concurrency", cfg.Concurrency, "must remain 1 for this single-query preflight")
	flag.StringVar(&cfg.QueryFile, "query-file", cfg.QueryFile, "pinned psql source query file")
	flag.Parse()
	cfg.Timeout = time.Duration(timeoutSeconds) * time.Second

	result, gateFailed, err := run(context.Background(), cfg)
	if err != nil {
		var ipMismatch *remoteAddressMismatchError
		if errors.As(err, &ipMismatch) {
			_ = json.NewEncoder(os.Stdout).Encode(struct {
				Format  string        `json:"format"`
				Status  string        `json:"status"`
				Error   string        `json:"error"`
				Network networkOutput `json:"network"`
			}{"espacigo.gcp04.target-preflight.v1", "failed", "Cloud SQL connector transport did not reach the pinned private IP", networkOutput{ExpectedPrivateIP: ipMismatch.Expected, ObservedRemoteIP: ipMismatch.Observed}})
			os.Exit(1)
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]string{"format": "espacigo.gcp04.target-preflight.v1", "status": "failed", "error": "preflight did not complete"})
		os.Exit(1)
	}
	_ = json.NewEncoder(os.Stdout).Encode(result)
	if gateFailed {
		os.Exit(2)
	}
}

func configFromEnvironment() config {
	return config{
		ProjectID:           os.Getenv("GCP04_PREFLIGHT_PROJECT_ID"),
		ConnectionName:      os.Getenv("CLOUD_SQL_INSTANCE_CONNECTION_NAME"),
		ExpectedPrivateIP:   os.Getenv("GCP04_PREFLIGHT_EXPECTED_PRIVATE_IP"),
		Database:            os.Getenv("DATABASE_NAME"),
		DatabaseUser:        os.Getenv("DATABASE_USER"),
		PasswordFile:        os.Getenv("DATABASE_PASSWORD_FILE"),
		SecretVersion:       os.Getenv("GCP04_PREFLIGHT_SECRET_VERSION"),
		RequiredConnections: 0,
		Timeout:             120 * time.Second,
		Concurrency:         1,
		QueryFile:           defaultQueryPath,
	}
}

func (c config) validate() error {
	if c.ProjectID != expectedProject || c.ConnectionName != expectedConnectionName || !cloudsql.ValidInstanceConnectionName(c.ConnectionName) {
		return errors.New("Cloud SQL project/instance identity is not the reviewed target")
	}
	parts := strings.Split(c.ConnectionName, ":")
	if len(parts) != 3 || parts[0] != c.ProjectID || parts[1] != "southamerica-west1" || parts[2] != "espacigo-testprod-pg" {
		return errors.New("Cloud SQL connection name does not match the fixed target")
	}
	ip := net.ParseIP(c.ExpectedPrivateIP)
	if ip == nil || !ip.IsPrivate() || c.ExpectedPrivateIP != expectedPrivateIP {
		return errors.New("Cloud SQL private IP does not match the reviewed target")
	}
	if c.Database != expectedDatabase || c.DatabaseUser != expectedDatabaseUser {
		return errors.New("database identity is not the reviewed observer target")
	}
	if c.PasswordFile == "" || c.SecretVersion == "" || !numericSecretVersion.MatchString(c.SecretVersion) {
		return errors.New("numeric credential secret version and mounted file are required")
	}
	if c.RequiredConnections < 1 || c.RequiredConnections > 999999 {
		return errors.New("required connection budget must be from 1 to 999999")
	}
	if c.Timeout < time.Second || c.Timeout > maximumTimeout {
		return errors.New("preflight timeout is outside the allowed range")
	}
	if c.Concurrency != 1 {
		return errors.New("preflight concurrency must be exactly one")
	}
	if c.QueryFile == "" {
		return errors.New("pinned query file is required")
	}
	return nil
}

func readPasswordFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0022 != 0 {
		return "", errors.New("mounted credential file is not a protected regular file")
	}
	if info.Size() == 0 || info.Size() > maximumCredentialsLength {
		return "", errors.New("mounted credential file size is invalid")
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 || len(data) > maximumCredentialsLength {
		return "", errors.New("mounted credential file could not be read within its limit")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
	if password == "" || strings.ContainsAny(password, "\x00\r\n") {
		return "", errors.New("mounted credential file has an invalid shape")
	}
	return password, nil
}

func compileQuery(source []byte) (string, string, error) {
	digest := sha256.Sum256(source)
	digestHex := hex.EncodeToString(digest[:])
	if digestHex != expectedQuerySHA256 {
		return "", "", errors.New("target preflight SQL digest differs from the reviewed source")
	}
	marker := "BEGIN READ ONLY;\n"
	start := strings.Index(string(source), marker)
	if start < 0 {
		return "", "", errors.New("psql read-only wrapper was not found")
	}
	body := string(source[start+len(marker):])
	if !strings.HasSuffix(body, "COMMIT;\n") {
		return "", "", errors.New("psql transaction terminator differs from the reviewed source")
	}
	body = strings.TrimSuffix(body, "COMMIT;\n")
	if strings.Contains(body, "\\") || strings.Contains(body, ":{") || strings.Contains(body, ":required_connections") {
		return "", "", errors.New("unsupported psql meta command remains in SQL body")
	}
	if strings.Count(body, ":'required_connections'") != 3 {
		return "", "", errors.New("required connection parameter shape differs from the reviewed query")
	}
	body = strings.ReplaceAll(body, ":'required_connections'", "$1")
	return strings.TrimSpace(body), digestHex, nil
}

func run(ctx context.Context, c config) (output, bool, error) {
	if err := c.validate(); err != nil {
		return output{}, false, err
	}
	source, err := os.ReadFile(c.QueryFile)
	if err != nil {
		return output{}, false, errors.New("target preflight SQL could not be read")
	}
	query, queryDigest, err := compileQuery(source)
	if err != nil {
		return output{}, false, err
	}
	password, err := readPasswordFile(c.PasswordFile)
	if err != nil {
		return output{}, false, err
	}
	deadline, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	databaseURL := "postgres://" + url.PathEscape(c.DatabaseUser) + "@localhost:5432/" + url.PathEscape(c.Database) + "?sslmode=disable"
	conn, closeConnector, err := cloudsql.Connect(deadline, databaseURL, password, c.ConnectionName, true)
	if err != nil {
		return output{}, false, errors.New("Cloud SQL private connector session failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	observedIP, err := validateRemoteAddress(conn.PgConn().Conn().RemoteAddr(), c.ExpectedPrivateIP)
	if err != nil {
		return output{}, false, err
	}

	tx, err := conn.BeginTx(deadline, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return output{}, false, errors.New("read-only target transaction could not begin")
	}
	var report []byte
	if err := tx.QueryRow(deadline, query, c.RequiredConnections).Scan(&report); err != nil {
		_ = tx.Rollback(context.Background())
		return output{}, false, errors.New("target inventory query failed")
	}
	if err := tx.Commit(deadline); err != nil {
		return output{}, false, errors.New("read-only target transaction could not commit")
	}
	if len(report) == 0 || len(report) > 8<<20 || !json.Valid(report) {
		return output{}, false, errors.New("target inventory report is missing or invalid")
	}
	validation, err := validateReport(report, c)
	if err != nil {
		return output{}, false, err
	}
	status := "passed"
	if !validation.CapacityPass || !validation.ObserverContractPass {
		status = "gate_failed"
	}
	return output{
		Format: "espacigo.gcp04.target-preflight.v1", Status: status,
		Target: targetOutput{ProjectID: c.ProjectID, InstanceConnection: c.ConnectionName,
			PrivateIP: c.ExpectedPrivateIP, Database: c.Database, DatabaseUser: c.DatabaseUser,
			SecretVersion: c.SecretVersion, RequiredConnections: c.RequiredConnections},
		Network:     networkOutput{ExpectedPrivateIP: c.ExpectedPrivateIP, ObservedRemoteIP: observedIP},
		QuerySHA256: queryDigest, ObserverContractPass: validation.ObserverContractPass,
		CapacityGatePass: validation.CapacityPass, DestinationCandidate: validation.DestinationCandidate,
		RestoreApproval: "not_granted", Report: json.RawMessage(report),
	}, status == "gate_failed", nil
}

type reportValidation struct {
	ObserverContractPass bool
	CapacityPass         bool
	DestinationCandidate bool
}

type reportRole struct {
	Name        string  `json:"name"`
	Login       bool    `json:"login"`
	Super       bool    `json:"super"`
	CreateDB    bool    `json:"createdb"`
	CreateRole  bool    `json:"createrole"`
	Replication bool    `json:"replication"`
	BypassRLS   bool    `json:"bypassrls"`
	Inherit     bool    `json:"inherit"`
	Comment     *string `json:"comment"`
}

type reportMembership struct {
	AppLogin    *string `json:"app_login"`
	GrantedRole string  `json:"granted_role"`
	MemberRole  string  `json:"member_role"`
	Admin       bool    `json:"admin_option"`
	Inherit     bool    `json:"inherit_option"`
	Set         bool    `json:"set_option"`
	Context     string  `json:"edge_context"`
}

type reportExtension struct {
	Name          string  `json:"name"`
	Version       string  `json:"version"`
	Approved      *string `json:"approved_version"`
	ApprovedExact bool    `json:"approved_exact"`
}

type reportACLEntry struct {
	Kind        string `json:"kind"`
	Identity    string `json:"identity"`
	ObjectOwner string `json:"object_owner"`
	Grantor     string `json:"grantor"`
	Grantee     string `json:"grantee"`
	Privilege   string `json:"privilege"`
	Grantable   bool   `json:"grantable"`
}

type reportEffectivePrivilege struct {
	Kind     string   `json:"kind"`
	Identity string   `json:"identity"`
	Owner    string   `json:"owner"`
	Required []string `json:"required_privileges"`
	Missing  []string `json:"missing_effective_privileges"`
}

type publicEffectivePrivileges struct {
	DatabaseConnect    bool `json:"database_connect"`
	DatabaseCreate     bool `json:"database_create"`
	DatabaseTemporary  bool `json:"database_temporary"`
	PublicSchemaUsage  bool `json:"public_schema_usage"`
	PublicSchemaCreate bool `json:"public_schema_create"`
}

func (p publicEffectivePrivileges) matchesReviewedBaseline() bool {
	return p.DatabaseConnect && !p.DatabaseCreate && !p.DatabaseTemporary && p.PublicSchemaUsage && !p.PublicSchemaCreate
}

func validateReport(raw []byte, c config) (reportValidation, error) {
	var report struct {
		Database struct {
			Name                string `json:"name"`
			ServerVersionNum    int    `json:"server_version_num"`
			ServerVersion       string `json:"server_version"`
			User                string `json:"user"`
			TransactionReadOnly bool   `json:"transaction_read_only"`
			Owner               string `json:"owner"`
			PublicSchemaOwner   string `json:"public_schema_owner"`
		} `json:"database"`
		PublicEffectivePrivileges publicEffectivePrivileges `json:"public_effective_privileges"`
		Capacity                  struct {
			MaxConnections               int  `json:"max_connections"`
			ReservedConnections          int  `json:"reserved_connections"`
			SuperuserReservedConnections int  `json:"superuser_reserved_connections"`
			VisibleClientBackends        int  `json:"visible_client_backends"`
			UnclassifiedActivityRows     int  `json:"unclassified_activity_rows"`
			FreeSlotsLowerBound          int  `json:"free_slots_lower_bound"`
			RequiredConnections          int  `json:"required_connections"`
			Passes                       bool `json:"passes"`
		} `json:"capacity"`
		Destination struct {
			RelationsNonExtension      int               `json:"relations_non_extension"`
			RoutinesNonExtension       int               `json:"routines_non_extension"`
			TypesNonExtension          int               `json:"types_non_extension"`
			OtherNonExtension          int               `json:"other_schema_objects_non_extension"`
			NonPublicSchemas           int               `json:"non_public_schemas_non_extension"`
			NonPublicObjects           int               `json:"non_public_objects_non_extension"`
			UnapprovedExtensions       int               `json:"unapproved_extensions"`
			EmptyCandidate             bool              `json:"empty_destination_candidate"`
			MigrationsPresent          bool              `json:"schema_migrations_present"`
			SchemaInventory            []json.RawMessage `json:"schema_inventory"`
			RelationInventory          []json.RawMessage `json:"relation_inventory"`
			RoutineInventory           []json.RawMessage `json:"routine_inventory"`
			TypeInventory              []json.RawMessage `json:"type_inventory"`
			OtherSchemaObjectInventory []json.RawMessage `json:"other_schema_object_inventory"`
			NonPublicObjectInventory   []json.RawMessage `json:"non_public_object_inventory"`
		} `json:"destination"`
		Roles               []reportRole       `json:"roles"`
		Memberships         []reportMembership `json:"memberships"`
		RequiredConnections int                `json:"required_connections"`
		Extensions          []json.RawMessage  `json:"extensions"`
		SchemaMigrations    struct {
			Present bool `json:"present"`
		} `json:"schema_migrations"`
		ACLEntries     []json.RawMessage `json:"acl_entries"`
		OwnerEffective []json.RawMessage `json:"owner_effective_privileges"`
	}
	if err := requireJSONKeys(raw, "database", "public_effective_privileges", "capacity", "destination", "roles", "memberships", "extensions", "schema_migrations", "acl_entries", "owner_effective_privileges", "required_connections"); err != nil {
		return reportValidation{}, errors.New("target inventory report is incomplete")
	}
	for _, key := range []string{"roles", "memberships", "extensions", "acl_entries", "owner_effective_privileges"} {
		if err := requireJSONArray(rawObjectField(raw, key)); err != nil {
			return reportValidation{}, errors.New("target inventory report contains an invalid collection")
		}
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return reportValidation{}, errors.New("target inventory report schema is invalid")
	}
	if err := requireJSONKeys(rawObjectField(raw, "database"), "name", "server_version_num", "server_version", "user", "transaction_read_only", "owner", "public_schema_owner"); err != nil {
		return reportValidation{}, errors.New("target database report is incomplete")
	}
	if err := requireJSONKeys(rawObjectField(raw, "public_effective_privileges"), "database_connect", "database_create", "database_temporary", "public_schema_usage", "public_schema_create"); err != nil {
		return reportValidation{}, errors.New("target PUBLIC effective privilege report is incomplete")
	}
	if err := requireJSONKeys(rawObjectField(raw, "capacity"), "max_connections", "reserved_connections", "superuser_reserved_connections", "visible_client_backends", "unclassified_activity_rows", "free_slots_lower_bound", "required_connections", "passes"); err != nil {
		return reportValidation{}, errors.New("target capacity report is incomplete")
	}
	if err := requireJSONKeys(rawObjectField(raw, "destination"), "relations_non_extension", "routines_non_extension", "types_non_extension", "other_schema_objects_non_extension", "non_public_schemas_non_extension", "non_public_objects_non_extension", "unapproved_extensions", "empty_destination_candidate", "schema_migrations_present", "schema_inventory", "relation_inventory", "routine_inventory", "type_inventory", "other_schema_object_inventory", "non_public_object_inventory"); err != nil {
		return reportValidation{}, errors.New("target destination report is incomplete")
	}
	for _, key := range []string{"schema_inventory", "relation_inventory", "routine_inventory", "type_inventory", "other_schema_object_inventory", "non_public_object_inventory"} {
		if err := requireJSONArray(rawObjectField(rawObjectField(raw, "destination"), key)); err != nil {
			return reportValidation{}, errors.New("target destination report contains an invalid inventory")
		}
	}
	if err := requireJSONKeys(rawObjectField(raw, "schema_migrations"), "present"); err != nil {
		return reportValidation{}, errors.New("target migration report is incomplete")
	}
	for _, role := range report.Roles {
		if err := requireJSONKeys(roleRaw(raw, role.Name), "name", "login", "super", "createdb", "createrole", "replication", "bypassrls", "inherit", "comment"); err != nil {
			return reportValidation{}, errors.New("target role report is incomplete")
		}
	}
	for _, membership := range report.Memberships {
		if err := requireJSONKeys(membershipRaw(raw, membership), "app_login", "granted_role", "member_role", "admin_option", "inherit_option", "set_option", "edge_context"); err != nil {
			return reportValidation{}, errors.New("target membership report is incomplete")
		}
	}
	for _, item := range report.Extensions {
		if err := requireJSONKeys(item, "name", "version", "approved_version", "approved_exact"); err != nil {
			return reportValidation{}, errors.New("target extension report is incomplete")
		}
		var extension reportExtension
		if err := json.Unmarshal(item, &extension); err != nil || extension.Name == "" || extension.Version == "" {
			return reportValidation{}, errors.New("target extension report has invalid field types")
		}
	}
	for _, item := range report.Destination.SchemaInventory {
		if err := requireJSONKeys(item, "name", "owner", "extension_owned"); err != nil {
			return reportValidation{}, errors.New("target schema inventory is incomplete")
		}
		if !hasNonemptyOwner(item) {
			return reportValidation{}, errors.New("target schema owner is missing")
		}
	}
	for _, item := range report.Destination.RelationInventory {
		if err := requireJSONKeys(item, "identity", "schema", "kind", "owner", "extension_owned"); err != nil {
			return reportValidation{}, errors.New("target relation inventory is incomplete")
		}
		if !hasNonemptyOwner(item) {
			return reportValidation{}, errors.New("target relation owner is missing")
		}
	}
	for _, item := range report.Destination.RoutineInventory {
		if err := requireJSONKeys(item, "identity", "schema", "owner", "security_definer", "settings", "extension_owned"); err != nil {
			return reportValidation{}, errors.New("target routine inventory is incomplete")
		}
		if !hasNonemptyOwner(item) {
			return reportValidation{}, errors.New("target routine owner is missing")
		}
	}
	for _, item := range report.Destination.TypeInventory {
		if err := requireJSONKeys(item, "identity", "schema", "kind", "owner", "extension_owned"); err != nil {
			return reportValidation{}, errors.New("target type inventory is incomplete")
		}
		if !hasNonemptyOwner(item) {
			return reportValidation{}, errors.New("target type owner is missing")
		}
	}
	for _, inventory := range [][]json.RawMessage{report.Destination.OtherSchemaObjectInventory, report.Destination.NonPublicObjectInventory} {
		for _, item := range inventory {
			if err := requireJSONKeys(item, "identity", "schema", "kind", "owner", "extension_owned"); err != nil {
				return reportValidation{}, errors.New("target object inventory is incomplete")
			}
			if !ownerFieldValidForKind(item) {
				var entry struct {
					Kind string `json:"kind"`
				}
				_ = json.Unmarshal(item, &entry)
				return reportValidation{}, fmt.Errorf("target object owner field is invalid for %s", entry.Kind)
			}
		}
	}
	for _, item := range report.Destination.NonPublicObjectInventory {
		if err := requireJSONKeys(item, "owner"); err != nil {
			return reportValidation{}, errors.New("target non-public object owner is missing")
		}
	}
	for _, item := range report.Destination.OtherSchemaObjectInventory {
		if err := requireJSONKeys(item, "owner"); err != nil {
			return reportValidation{}, errors.New("target object owner is missing")
		}
	}
	for _, item := range report.ACLEntries {
		if err := requireJSONKeys(item, "kind", "identity", "object_owner", "grantor", "grantee", "privilege", "grantable"); err != nil {
			return reportValidation{}, errors.New("target ACL report is incomplete")
		}
		var acl reportACLEntry
		if err := json.Unmarshal(item, &acl); err != nil || acl.Kind == "" || acl.Identity == "" || acl.ObjectOwner == "" || acl.Grantor == "" || acl.Grantee == "" || acl.Privilege == "" {
			return reportValidation{}, errors.New("target ACL report has invalid field types")
		}
	}
	for _, item := range report.OwnerEffective {
		if err := requireJSONKeys(item, "kind", "identity", "owner", "required_privileges", "missing_effective_privileges"); err != nil {
			return reportValidation{}, errors.New("target effective-privilege report is incomplete")
		}
		var effective reportEffectivePrivilege
		if err := json.Unmarshal(item, &effective); err != nil || effective.Kind == "" || effective.Identity == "" || effective.Owner == "" || effective.Required == nil || effective.Missing == nil {
			return reportValidation{}, errors.New("target effective-privilege report has invalid field types")
		}
	}
	if report.Database.Name != c.Database || report.Database.User != c.DatabaseUser || !report.Database.TransactionReadOnly || report.Database.ServerVersionNum < 180000 || report.Database.ServerVersionNum >= 190000 || report.Database.ServerVersion == "" {
		return reportValidation{}, errors.New("target SQL identity, PostgreSQL 18 engine, or read-only invariant failed")
	}
	if report.Database.Owner == "" || report.Database.PublicSchemaOwner == "" {
		return reportValidation{}, errors.New("target database ownership report is incomplete")
	}
	capacity := report.Capacity
	if capacity.MaxConnections <= 0 || capacity.ReservedConnections < 0 || capacity.SuperuserReservedConnections < 0 || capacity.VisibleClientBackends < 0 || capacity.UnclassifiedActivityRows < 0 || capacity.RequiredConnections != c.RequiredConnections || report.RequiredConnections != c.RequiredConnections {
		return reportValidation{}, errors.New("target capacity values are outside the valid range")
	}
	free := capacity.MaxConnections - capacity.ReservedConnections - capacity.SuperuserReservedConnections - capacity.VisibleClientBackends - capacity.UnclassifiedActivityRows
	if free < 0 {
		free = 0
	}
	capacityPass := free >= c.RequiredConnections
	if capacity.FreeSlotsLowerBound != free || capacity.Passes != capacityPass || report.Database.ServerVersionNum/10000 != 18 {
		return reportValidation{}, errors.New("target capacity report is internally inconsistent")
	}
	if report.Destination.MigrationsPresent != report.SchemaMigrations.Present {
		return reportValidation{}, errors.New("target destination migration indicators disagree")
	}
	if countInventoryNonExtension(report.Destination.RelationInventory) != report.Destination.RelationsNonExtension ||
		countInventoryNonExtension(report.Destination.RoutineInventory) != report.Destination.RoutinesNonExtension ||
		countInventoryNonExtension(report.Destination.TypeInventory) != report.Destination.TypesNonExtension ||
		countInventoryNonExtension(report.Destination.OtherSchemaObjectInventory) != report.Destination.OtherNonExtension {
		return reportValidation{}, errors.New("target destination inventory counts disagree with details")
	}
	var nonPublicSchemas, nonPublicObjects, unapprovedExtensions int
	for _, item := range report.Destination.SchemaInventory {
		var value struct {
			Name           string `json:"name"`
			ExtensionOwned bool   `json:"extension_owned"`
		}
		_ = json.Unmarshal(item, &value)
		if value.Name != "public" && !value.ExtensionOwned {
			nonPublicSchemas++
		}
	}
	for _, item := range report.Destination.NonPublicObjectInventory {
		var value struct {
			ExtensionOwned bool `json:"extension_owned"`
		}
		_ = json.Unmarshal(item, &value)
		if !value.ExtensionOwned {
			nonPublicObjects++
		}
	}
	for _, item := range report.Extensions {
		var value struct {
			ApprovedExact bool `json:"approved_exact"`
		}
		_ = json.Unmarshal(item, &value)
		if !value.ApprovedExact {
			unapprovedExtensions++
		}
	}
	if nonPublicSchemas != report.Destination.NonPublicSchemas || nonPublicObjects != report.Destination.NonPublicObjects || unapprovedExtensions != report.Destination.UnapprovedExtensions {
		return reportValidation{}, errors.New("target destination aggregate counts disagree with inventories")
	}
	destinationDerivedEmpty := report.Destination.RelationsNonExtension == 0 && report.Destination.RoutinesNonExtension == 0 && report.Destination.TypesNonExtension == 0 && report.Destination.OtherNonExtension == 0 && report.Destination.NonPublicSchemas == 0 && report.Destination.NonPublicObjects == 0 && report.Destination.UnapprovedExtensions == 0 && !report.SchemaMigrations.Present
	if report.Destination.EmptyCandidate != destinationDerivedEmpty {
		return reportValidation{}, errors.New("target destination candidate indicator is inconsistent")
	}
	login, loginFound := findRole(report.Roles, c.DatabaseUser)
	observerGroup, groupFound := findRole(report.Roles, "espacigo_db_observer")
	seenRoleNames := make(map[string]struct{}, len(report.Roles))
	for _, role := range report.Roles {
		if _, exists := seenRoleNames[role.Name]; exists {
			return reportValidation{}, errors.New("target role report contains duplicate role identities")
		}
		seenRoleNames[role.Name] = struct{}{}
	}
	observerContract := report.PublicEffectivePrivileges.matchesReviewedBaseline() && loginFound && groupFound && login.Login && !login.Super && !login.CreateDB && !login.CreateRole && !login.Replication && !login.BypassRLS && login.Inherit &&
		!observerGroup.Login && !observerGroup.Super && !observerGroup.CreateDB && !observerGroup.CreateRole && !observerGroup.Replication && !observerGroup.BypassRLS && !observerGroup.Inherit
	observerContract = observerContract && validateObserverACL(report.ACLEntries, report.Database.Owner, report.Database.PublicSchemaOwner,
		[][]json.RawMessage{report.Destination.SchemaInventory, report.Destination.RelationInventory, report.Destination.RoutineInventory,
			report.Destination.TypeInventory, report.Destination.OtherSchemaObjectInventory, report.Destination.NonPublicObjectInventory},
		c.Database, c.DatabaseUser, "espacigo_db_observer", report.Destination.MigrationsPresent)
	loginEdges := 0
	for _, edge := range report.Memberships {
		if edge.GrantedRole == "espacigo_db_observer" && edge.MemberRole != c.DatabaseUser {
			observerContract = false
		}
		if (edge.AppLogin != nil && *edge.AppLogin == c.DatabaseUser) || edge.MemberRole == c.DatabaseUser || edge.GrantedRole == c.DatabaseUser {
			if edge.GrantedRole != "espacigo_db_observer" || edge.MemberRole != c.DatabaseUser || edge.Admin || !edge.Inherit || edge.Set {
				observerContract = false
			}
			if edge.AppLogin != nil && *edge.AppLogin == c.DatabaseUser {
				loginEdges++
				if edge.Context != "transitive-login-membership" {
					observerContract = false
				}
			} else if edge.Context != "direct-edge-touches-application-role" {
				observerContract = false
			}
		}
	}
	if loginEdges != 1 {
		observerContract = false
	}
	return reportValidation{ObserverContractPass: observerContract, CapacityPass: capacityPass, DestinationCandidate: report.Destination.EmptyCandidate}, nil
}

type observerACLKey struct {
	kind      string
	identity  string
	privilege string
}

func validateObserverACL(entries []json.RawMessage, databaseOwner, schemaOwner string,
	collections [][]json.RawMessage, databaseName, loginName, groupName string, ledgerPresent bool) bool {
	if ownedByObserver(databaseOwner, loginName, groupName) || ownedByObserver(schemaOwner, loginName, groupName) {
		return false
	}
	for _, collection := range collections {
		for _, raw := range collection {
			var item struct {
				Owner string `json:"owner"`
			}
			if json.Unmarshal(raw, &item) != nil || ownedByObserver(item.Owner, loginName, groupName) {
				return false
			}
		}
	}
	allowed := map[observerACLKey]struct{}{
		{kind: "database", identity: databaseName, privilege: "CONNECT"}: {},
		{kind: "schema", identity: "public", privilege: "USAGE"}:         {},
	}
	if ledgerPresent {
		allowed[observerACLKey{kind: "relation", identity: "public.schema_migrations", privilege: "SELECT"}] = struct{}{}
	}
	seen := make(map[observerACLKey]int, len(allowed))
	for _, raw := range entries {
		var item reportACLEntry
		if json.Unmarshal(raw, &item) != nil || ownedByObserver(item.ObjectOwner, loginName, groupName) {
			return false
		}
		if item.Grantee == loginName {
			return false
		}
		if item.Grantee != groupName {
			continue
		}
		key := observerACLKey{kind: item.Kind, identity: item.Identity, privilege: item.Privilege}
		if item.Grantable {
			return false
		}
		if _, ok := allowed[key]; !ok {
			return false
		}
		seen[key]++
	}
	for key := range allowed {
		if seen[key] == 0 {
			return false
		}
	}
	return true
}

func ownedByObserver(owner, loginName, groupName string) bool {
	return owner == loginName || owner == groupName
}

func countInventoryNonExtension(items []json.RawMessage) int {
	count := 0
	for _, item := range items {
		var value struct {
			ExtensionOwned bool `json:"extension_owned"`
		}
		_ = json.Unmarshal(item, &value)
		if !value.ExtensionOwned {
			count++
		}
	}
	return count
}

func validateRemoteAddress(address net.Addr, expectedIP string) (string, error) {
	if address == nil {
		return "", errors.New("connector returned no remote address")
	}
	host, _, err := net.SplitHostPort(address.String())
	if err != nil {
		return "", errors.New("connector remote address is not a TCP endpoint")
	}
	observed := net.ParseIP(host)
	expected := net.ParseIP(expectedIP)
	if observed == nil || expected == nil {
		return "", errors.New("connector remote address differs from pinned private IP")
	}
	if !observed.Equal(expected) {
		return observed.String(), &remoteAddressMismatchError{Expected: expected.String(), Observed: observed.String()}
	}
	return observed.String(), nil
}

type remoteAddressMismatchError struct {
	Expected string
	Observed string
}

func (e *remoteAddressMismatchError) Error() string {
	return "connector remote address differs from pinned private IP"
}

func requireJSONKeys(raw json.RawMessage, keys ...string) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return errors.New("expected JSON object")
	}
	for _, key := range keys {
		if _, ok := object[key]; !ok {
			return errors.New("required JSON key is missing")
		}
	}
	return nil
}

func requireJSONArray(raw json.RawMessage) error {
	trimmed := strings.TrimSpace(string(raw))
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return errors.New("expected JSON array")
	}
	var values []json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return errors.New("invalid JSON array")
	}
	return nil
}

func rawObjectField(raw json.RawMessage, key string) json.RawMessage {
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	return object[key]
}

func hasNonemptyOwner(raw json.RawMessage) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil {
		return false
	}
	var owner string
	return json.Unmarshal(object["owner"], &owner) == nil && owner != ""
}

func ownerFieldValidForKind(raw json.RawMessage) bool {
	var item struct {
		Kind  string          `json:"kind"`
		Owner json.RawMessage `json:"owner"`
	}
	if json.Unmarshal(raw, &item) != nil {
		return false
	}
	if strings.TrimSpace(string(item.Owner)) == "null" {
		return item.Kind == "text_search_parser" || item.Kind == "text_search_template"
	}
	var owner string
	return json.Unmarshal(item.Owner, &owner) == nil && owner != ""
}

func findRole(roles []reportRole, name string) (reportRole, bool) {
	for _, role := range roles {
		if role.Name == name {
			return role, true
		}
	}
	return reportRole{}, false
}

func roleRaw(raw json.RawMessage, name string) json.RawMessage {
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	var roles []json.RawMessage
	_ = json.Unmarshal(object["roles"], &roles)
	for _, item := range roles {
		var role struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(item, &role)
		if role.Name == name {
			return item
		}
	}
	return nil
}

func membershipRaw(raw json.RawMessage, wanted reportMembership) json.RawMessage {
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	var edges []json.RawMessage
	_ = json.Unmarshal(object["memberships"], &edges)
	for _, item := range edges {
		var edge reportMembership
		_ = json.Unmarshal(item, &edge)
		if edge.GrantedRole == wanted.GrantedRole && edge.MemberRole == wanted.MemberRole && edge.Context == wanted.Context && sameOptionalString(edge.AppLogin, wanted.AppLogin) {
			return item
		}
	}
	return nil
}

func sameOptionalString(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func runForLocalIntegration(ctx context.Context, conn *pgx.Conn, query string, required int, expectedDatabase, expectedUser string) (json.RawMessage, bool, error) {
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, false, err
	}
	var report []byte
	if err := tx.QueryRow(ctx, query, required).Scan(&report); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, false, err
	}
	cfg := config{Database: expectedDatabase, DatabaseUser: expectedUser, RequiredConnections: required}
	validation, err := validateReport(report, cfg)
	return report, !validation.CapacityPass || !validation.ObserverContractPass, err
}
