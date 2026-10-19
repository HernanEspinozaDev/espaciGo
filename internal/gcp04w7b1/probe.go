package gcp04w7b1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	Format       = "espacigo.gcp04.w7.b1-actor-probe.v1"
	ProjectID    = "espacigo-testprod"
	Instance     = "espacigo-testprod-pg"
	Connection   = "espacigo-testprod:southamerica-west1:espacigo-testprod-pg"
	Region       = "southamerica-west1"
	PrivateIP    = "10.189.0.3"
	Database     = "espacigo"
	Bootstrap    = "espacigo_gcp04_w7_bootstrap"
	AdminRole    = "cloudsqlsuperuser"
	Observer     = "espacigo_gcp04_w7_observer"
	ObserverRole = "espacigo_db_observer"
	SecretID     = "espacigo-gcp04-w7-b1-bootstrap-credential"
	QuerySHA256  = "9f9427086cb31d446edeb64e007e1f5e24999feee4cb245930964e1507ec3826"
)

// CompileQuery accepts only the reviewed, single read-only catalog SELECT.
func CompileQuery(source []byte) (string, error) {
	sum := sha256.Sum256(source)
	if hex.EncodeToString(sum[:]) != QuerySHA256 {
		return "", errors.New("B1 probe SQL digest mismatch")
	}
	body := string(source)
	begin, end := "BEGIN READ ONLY;\n", "\nCOMMIT;\n"
	if !strings.HasPrefix(body, "-- GCP04-W7 B1:") || strings.Count(body, begin) != 1 || !strings.HasSuffix(body, end) {
		return "", errors.New("B1 probe wrapper differs from the reviewed form")
	}
	statement := strings.TrimSpace(strings.TrimSuffix(body[strings.Index(body, begin)+len(begin):], end))
	statement = strings.TrimSuffix(statement, ";")
	if !strings.HasPrefix(statement, "WITH ") || !strings.Contains(statement, "SELECT pg_catalog.jsonb_build_object(") || strings.Contains(statement, ";") {
		return "", errors.New("B1 probe must contain exactly one JSON catalog SELECT")
	}
	for _, forbidden := range []string{"pg_authid", "pg_shadow", "pg_stat_", "pg_settings", "insert ", "update ", "delete ", "create ", "alter ", "drop ", "set role"} {
		if strings.Contains(strings.ToLower(statement), forbidden) {
			return "", errors.New("B1 probe contains a forbidden catalog or mutation token")
		}
	}
	return statement, nil
}

type ProbeReport struct {
	Format               string                `json:"format"`
	Target               ProbeTarget           `json:"target"`
	Actor                *Actor                `json:"actor"`
	Memberships          []Membership          `json:"memberships"`
	RecursiveMemberships []RecursiveMembership `json:"recursive_memberships"`
	Owners               Owners                `json:"owners"`
	Capabilities         Capabilities          `json:"capabilities"`
	ActorACL             []ACL                 `json:"actor_acl"`
	PublicACL            []ACL                 `json:"public_acl"`
	Scope                ProbeScope            `json:"scope"`
}
type ProbeTarget struct {
	Database string `json:"database"`
	Session  string `json:"session_user"`
	Current  string `json:"current_user"`
	Version  int    `json:"server_version_num"`
	ReadOnly bool   `json:"transaction_read_only"`
}
type Actor struct {
	Name        string `json:"name"`
	Login       bool   `json:"login"`
	Superuser   bool   `json:"superuser"`
	CreateDB    bool   `json:"createdb"`
	CreateRole  bool   `json:"createrole"`
	Replication bool   `json:"replication"`
	BypassRLS   bool   `json:"bypass_rls"`
	Inherit     bool   `json:"inherit"`
}
type Membership struct {
	GrantedRole string `json:"granted_role"`
	MemberRole  string `json:"member_role"`
	Admin       bool   `json:"admin_option"`
	Inherit     bool   `json:"inherit_option"`
	Set         bool   `json:"set_option"`
}
type RecursiveMembership struct {
	GrantedRole string   `json:"granted_role"`
	MemberRole  string   `json:"member_role"`
	Admin       bool     `json:"admin_option"`
	Inherit     bool     `json:"inherit_option"`
	Set         bool     `json:"set_option"`
	Effective   bool     `json:"effective_inherit"`
	Depth       int      `json:"depth"`
	RolePath    []string `json:"role_path"`
}
type Owners struct {
	Database string  `json:"database"`
	Schema   string  `json:"public_schema"`
	Ledger   *string `json:"migration_ledger"`
}
type Capabilities struct {
	CreateRoleAttribute        bool  `json:"create_role_attribute"`
	DatabaseConnectDirectGO    bool  `json:"database_connect_direct_grant_option"`
	DatabaseConnectEffectiveGO bool  `json:"database_connect_effective_grant_option"`
	SchemaUsageDirectGO        bool  `json:"public_schema_usage_direct_grant_option"`
	SchemaUsageEffectiveGO     bool  `json:"public_schema_usage_effective_grant_option"`
	LedgerSelectDirectGO       *bool `json:"ledger_select_direct_grant_option"`
	LedgerSelectEffectiveGO    *bool `json:"ledger_select_effective_grant_option"`
	DatabaseOwner              bool  `json:"database_owner"`
	SchemaOwner                bool  `json:"public_schema_owner"`
	LedgerOwner                *bool `json:"ledger_owner"`
}
type ACL struct {
	Kind      string `json:"object_kind"`
	Object    string `json:"object_name"`
	Grantee   string `json:"grantee"`
	Privilege string `json:"privilege_type"`
	Grantable bool   `json:"is_grantable"`
	Grantor   string `json:"grantor"`
}
type ProbeScope struct {
	BusinessRows bool `json:"business_rows_read"`
	Secrets      bool `json:"password_or_secret_read"`
	Mutations    bool `json:"mutations"`
}

// Assess validates identity and returns observations only. It deliberately
// does not normalize attributes or claim bootstrap compatibility.
func Assess(raw []byte) (map[string]any, error) {
	var r ProbeReport
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, errors.New("B1 probe report shape invalid")
	}
	if r.Format != Format || r.Target.Database != Database || r.Target.Session != Bootstrap || r.Target.Current != Bootstrap || r.Target.Version < 180000 || r.Target.Version >= 190000 || !r.Target.ReadOnly || r.Actor == nil || r.Actor.Name != Bootstrap || !r.Actor.Login || r.Actor.Superuser || r.Scope.BusinessRows || r.Scope.Secrets || r.Scope.Mutations {
		return nil, errors.New("B1 probe target, self identity, SUPERUSER=false, read-only scope, or role inventory failed")
	}
	roleMembership := false
	for _, m := range r.Memberships {
		if m.MemberRole == Bootstrap && m.GrantedRole == AdminRole {
			roleMembership = true
		}
	}
	return map[string]any{
		"format":                                "espacigo.gcp04.w7.b1-assessment.v1",
		"status":                                "observed",
		"target":                                r.Target,
		"actor_attributes_observed":             r.Actor,
		"cloudsqlsuperuser_membership_observed": roleMembership,
		"memberships":                           r.Memberships,
		"recursive_memberships":                 r.RecursiveMemberships,
		"owners":                                r.Owners,
		"capability_matrix": map[string]any{
			"create_role_attribute":                      observation(r.Actor.CreateRole),
			"createdb_attribute":                         observation(r.Actor.CreateDB),
			"inherit_attribute":                          observation(r.Actor.Inherit),
			"database_connect_direct_grant_option":       r.Capabilities.DatabaseConnectDirectGO,
			"database_connect_effective_grant_option":    r.Capabilities.DatabaseConnectEffectiveGO,
			"database_owner":                             r.Capabilities.DatabaseOwner,
			"public_schema_usage_direct_grant_option":    r.Capabilities.SchemaUsageDirectGO,
			"public_schema_usage_effective_grant_option": r.Capabilities.SchemaUsageEffectiveGO,
			"public_schema_owner":                        r.Capabilities.SchemaOwner,
			"ledger_select_direct_grant_option":          observationNullable(r.Capabilities.LedgerSelectDirectGO),
			"ledger_select_effective_grant_option":       observationNullable(r.Capabilities.LedgerSelectEffectiveGO),
			"ledger_owner":                               observationNullable(r.Capabilities.LedgerOwner),
		},
		"actor_acl":                        r.ActorACL,
		"public_acl":                       r.PublicACL,
		"observer_bootstrap_compatibility": "not_accredited_by_b1_probe",
		"mutation_authorized":              false,
	}, nil
}

// RunProbe executes the one pinned catalog query on an already-open connection.
// The caller owns connector lifecycle and the private credential file.
func RunProbe(ctx context.Context, conn *pgx.Conn, source []byte) (map[string]any, error) {
	if conn == nil {
		return nil, errors.New("B1 SQL connection required")
	}
	if conn.Config().Database != Database || conn.Config().User != Bootstrap {
		return nil, errors.New("B1 SQL identity differs from the pinned target")
	}
	remote, ok := conn.PgConn().Conn().RemoteAddr().(*net.TCPAddr)
	if !ok || remote.IP.String() != PrivateIP {
		return nil, errors.New("B1 SQL socket is outside the pinned private endpoint")
	}
	query, err := CompileQuery(source)
	if err != nil {
		return nil, err
	}
	deadline, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	tx, err := conn.BeginTx(deadline, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, errors.New("B1 read-only transaction could not begin")
	}
	var raw []byte
	if err := tx.QueryRow(deadline, query).Scan(&raw); err != nil {
		_ = tx.Rollback(context.Background())
		return nil, errors.New("B1 pinned catalog query failed")
	}
	if err := tx.Commit(deadline); err != nil {
		return nil, errors.New("B1 read-only transaction did not commit")
	}
	return Assess(raw)
}

func RunProbeOnce(ctx context.Context, r Release, conn *pgx.Conn, source []byte, j Journal, leaseDir string, now time.Time) (map[string]any, error) {
	if err := ValidateRelease(r, "b1_probe", now); err != nil || !numericVersion(r.Version) || j == nil {
		return nil, errors.New("no active exact B1 probe release")
	}
	if err := AcquireLease(leaseDir, leaseKey(r.ReleaseID, "probe"), now); err != nil {
		return nil, err
	}
	stamp := func(phase string) error {
		return j.Save(JournalRecord{ReleaseID: r.ReleaseID, Stage: r.Stage, Phase: phase, Target: r.Target, Login: r.Login, Secret: secretParent(), Version: r.Version, At: time.Now().UTC().Format(time.RFC3339)})
	}
	if err := stamp("probe_started"); err != nil {
		return nil, err
	}
	ctx, cancel := stageContext(ctx, r, now)
	defer cancel()
	result, err := RunProbe(ctx, conn, source)
	if err != nil {
		_ = stamp("probe_outcome_incomplete")
		return nil, errors.New("B1 read-only probe failed; reconcile before retry")
	}
	if err = stamp("probe_complete"); err != nil {
		return nil, errors.New("B1 probe succeeded but journal completion is uncertain")
	}
	return result, nil
}

func ReadCredentialFile(path string) ([]byte, error) {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode()&os.ModeSymlink != 0 || fi.Mode().Perm() != 0600 || fi.Size() == 0 || fi.Size() > 4096 {
		return nil, errors.New("protected credential file unavailable")
	}
	b, err := os.ReadFile(filepath.Clean(path))
	if err != nil || len(b) == 0 || len(b) > 4096 {
		return nil, errors.New("protected credential file unavailable")
	}
	return b, nil
}

func numericVersion(s string) bool {
	if s == "" || s[0] < '1' || s[0] > '9' {
		return false
	}
	for _, r := range s[1:] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func observation(v bool) string {
	if v {
		return "observed_true"
	}
	return "observed_false"
}
func observationNullable(v *bool) any {
	if v == nil {
		return "unknown"
	}
	return *v
}
