package gcp03origin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
	"github.com/jackc/pgx/v5"
)

const ownerACLMaxInventory = 96

// OwnerACLOwnerCount deliberately reports only catalog type/owner aggregates.
type OwnerACLOwnerCount struct {
	Owner string `json:"owner"`
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

type OwnerACLSequenceCounts struct {
	Dependencies          int64 `json:"dependencies"`
	ParentOwnerMatches    int64 `json:"parent_owner_matches"`
	ParentOwnerMismatches int64 `json:"parent_owner_mismatches"`
	SerialDependencies    int64 `json:"serial_dependencies"`
	IdentityDependencies  int64 `json:"identity_dependencies"`
}

type migrationHistoryEntry struct {
	version  string
	checksum string
}

const canonicalMigrationHistorySHA256V2 = "dd2a4f3cf716137549b584f35bec5900df78c3e02a86354b4bad7ef1a90515ee"

type OwnerACLDiagnosticReport struct {
	Format                        string                 `json:"format"`
	Database                      string                 `json:"database"`
	Login                         string                 `json:"login"`
	ServerVersionMajor            int                    `json:"server_version_major"`
	ReadOnly                      bool                   `json:"read_only"`
	TransactionOutcome            string                 `json:"transaction_outcome"`
	FailureStage                  string                 `json:"failure_stage,omitempty"`
	SQLState                      string                 `json:"sqlstate,omitempty"`
	ReportComplete                bool                   `json:"report_complete"`
	TableInventoryCount           int                    `json:"table_inventory_count"`
	TableInventorySHA256          string                 `json:"table_inventory_sha256,omitempty"`
	TableInventoryWithinLimit     bool                   `json:"table_inventory_within_limit"`
	MigrationHistoryCount         int                    `json:"migration_history_count"`
	MigrationHistorySHA256        string                 `json:"migration_history_sha256,omitempty"`
	MigrationHistorySHA256V2      string                 `json:"migration_history_sha256_v2,omitempty"`
	ExpectedMigration61Present    bool                   `json:"expected_migration_61_present"`
	Owners                        []OwnerACLOwnerCount   `json:"owners_by_role_and_type"`
	Sequences                     OwnerACLSequenceCounts `json:"owned_sequence_parent_owner_counts"`
	RuntimeMembershipSet          bool                   `json:"runtime_membership_set"`
	RuntimeMembershipInherit      bool                   `json:"runtime_membership_inherit"`
	RuntimeMembershipAdmin        bool                   `json:"runtime_membership_admin"`
	RuntimeMembershipEffectiveSet bool                   `json:"runtime_membership_effective_set"`
	RuntimeLogin                  bool                   `json:"runtime_login"`
	PostgresPublicCreate          bool                   `json:"postgres_public_create"`
	RuntimePublicCreate           bool                   `json:"runtime_public_create"`
	ObserverColumnSelectCount     int64                  `json:"observer_column_select_count"`
	ObserverColumnWriteCount      int64                  `json:"observer_column_write_count"`
	AggregateTablesPresent        map[string]bool        `json:"aggregate_tables_present"`
	ValidSessions                 int64                  `json:"valid_sessions"`
	UsableActionTokens            int64                  `json:"usable_action_tokens"`
	PendingNotices                int64                  `json:"pending_notices"`
	IdentityEventLeases           int64                  `json:"identity_event_leases"`
}

func RunOwnerACLDiagnostic(ctx context.Context, config Config, password string) (OwnerACLDiagnosticReport, error) {
	r := OwnerACLDiagnosticReport{Format: "espacigo.g03o.owner-acl-check.v2", ReadOnly: true,
		TransactionOutcome: "not_started", Owners: []OwnerACLOwnerCount{},
		AggregateTablesPresent: map[string]bool{}}
	if config.Phase != PhaseOwnerACLCheck || config.Login != AdminLogin || config.DatabaseURL != AdminDatabaseURL || password == "" || !config.PrivateIP {
		r.FailureStage = "configuration"
		return r, errors.New("owner/ACL diagnostic configuration is invalid")
	}
	conn, closeConnector, err := cloudsql.Connect(ctx, config.DatabaseURL, password, config.Instance, true)
	if err != nil {
		r.FailureStage, r.SQLState = "connect", pgSQLState(err)
		return r, errors.New("owner/ACL diagnostic connection failed")
	}
	defer closeConnector()
	defer conn.Close(context.Background())
	return ownerACLDiagnosticOnConnection(ctx, conn, func(ctx context.Context, tx pgx.Tx) error { return tx.Commit(ctx) })
}

func ownerACLDiagnosticOnConnection(ctx context.Context, conn *pgx.Conn, commit func(context.Context, pgx.Tx) error) (OwnerACLDiagnosticReport, error) {
	r := OwnerACLDiagnosticReport{Format: "espacigo.g03o.owner-acl-check.v2", ReadOnly: true,
		TransactionOutcome: "not_started", Owners: []OwnerACLOwnerCount{}, AggregateTablesPresent: map[string]bool{}}
	tx, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		r.FailureStage, r.SQLState = "begin_read_only", pgSQLState(err)
		return r, errors.New("owner/ACL diagnostic read-only transaction could not begin")
	}
	r.TransactionOutcome = "active"
	fail := func(stage string, cause error) (OwnerACLDiagnosticReport, error) {
		r.FailureStage, r.SQLState = stage, pgSQLState(cause)
		if rb := tx.Rollback(ctx); rb != nil {
			r.TransactionOutcome = "outcome_uncertain"
		} else {
			r.TransactionOutcome = "rolled_back"
		}
		return r, errors.New("owner/ACL diagnostic stopped at a reported stage")
	}
	if _, err = tx.Exec(ctx, "SET TRANSACTION READ ONLY"); err != nil {
		return fail("enforce_read_only", err)
	}
	var versionNum int
	if err = tx.QueryRow(ctx, `SELECT current_database(),current_user,current_setting('server_version_num')::integer`).Scan(&r.Database, &r.Login, &versionNum); err != nil || r.Database != DatabaseName || r.Login != AdminLogin {
		return fail("target_identity", err)
	}
	r.ServerVersionMajor = versionNum / 10000
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f') AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass)`).Scan(&r.TableInventoryCount); err != nil {
		return fail("table_inventory_count", err)
	}
	if r.TableInventoryCount > ownerACLMaxInventory {
		return fail("table_inventory_limit", nil)
	}
	rows, err := tx.Query(ctx, `SELECT c.relname FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN ('r','p','v','m','f') AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass) ORDER BY c.relname`)
	if err != nil {
		return fail("table_inventory_hash", err)
	}
	var inventory strings.Builder
	for rows.Next() {
		var name string
		if err = rows.Scan(&name); err != nil {
			rows.Close()
			return fail("table_inventory_hash", err)
		}
		inventory.WriteString(name)
		inventory.WriteByte('\n')
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return fail("table_inventory_hash", err)
	}
	rows.Close()
	r.TableInventorySHA256 = hashText(inventory.String())
	r.TableInventoryWithinLimit = r.TableInventoryCount <= ownerACLMaxInventory
	var migExists bool
	if err = tx.QueryRow(ctx, `SELECT to_regclass('public.schema_migrations') IS NOT NULL`).Scan(&migExists); err != nil {
		return fail("migration_catalog", err)
	}
	if migExists {
		// Read the rows without relying on ORDER BY alias resolution. The two
		// hashes below sort explicitly: v1 preserves the historical text order,
		// while v2 uses numeric migration version order.
		rows, err = tx.Query(ctx, `SELECT version::text, checksum::text FROM public.schema_migrations`)
		if err != nil {
			return fail("migration_history", err)
		}
		var historyRows []migrationHistoryEntry
		for rows.Next() {
			var v, c string
			if err = rows.Scan(&v, &c); err != nil {
				rows.Close()
				return fail("migration_history", err)
			}
			historyRows = append(historyRows, migrationHistoryEntry{version: v, checksum: c})
			r.MigrationHistoryCount++
			if v == "61" {
				r.ExpectedMigration61Present = true
			}
		}
		if err = rows.Err(); err != nil {
			rows.Close()
			return fail("migration_history", err)
		}
		rows.Close()
		r.MigrationHistorySHA256, r.MigrationHistorySHA256V2, err = migrationHistoryHashes(historyRows)
		if err != nil {
			return fail("migration_history", err)
		}
	}
	rows, err = tx.Query(ctx, `SELECT owner.rolname, CASE WHEN c.relkind IN ('r','p') THEN 'table' WHEN c.relkind='v' THEN 'view' WHEN c.relkind='m' THEN 'materialized_view' WHEN c.relkind='S' THEN 'sequence' WHEN c.relkind IN ('i','I') THEN 'index' WHEN c.relkind='f' THEN 'foreign_table' ELSE 'other' END, count(*) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_roles owner ON owner.oid=c.relowner WHERE n.nspname='public' AND NOT EXISTS (SELECT 1 FROM pg_catalog.pg_depend d WHERE d.classid='pg_class'::regclass AND d.objid=c.oid AND d.deptype='e' AND d.refclassid='pg_extension'::regclass) GROUP BY owner.rolname,2 ORDER BY owner.rolname,2`)
	if err != nil {
		return fail("owner_catalog", err)
	}
	for rows.Next() {
		var x OwnerACLOwnerCount
		if err = rows.Scan(&x.Owner, &x.Type, &x.Count); err != nil {
			rows.Close()
			return fail("owner_catalog", err)
		}
		r.Owners = append(r.Owners, x)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return fail("owner_catalog", err)
	}
	rows.Close()
	if err = tx.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE parent.relowner=seq.relowner),count(*) FILTER(WHERE parent.relowner<>seq.relowner),count(*) FILTER(WHERE d.deptype='a'),count(*) FILTER(WHERE d.deptype='i') FROM pg_catalog.pg_class seq JOIN pg_catalog.pg_namespace ns ON ns.oid=seq.relnamespace JOIN pg_catalog.pg_depend d ON d.classid='pg_class'::regclass AND d.objid=seq.oid AND d.refclassid='pg_class'::regclass AND d.deptype IN ('a','i') JOIN pg_catalog.pg_class parent ON parent.oid=d.refobjid WHERE ns.nspname='public' AND seq.relkind='S'`).Scan(&r.Sequences.Dependencies, &r.Sequences.ParentOwnerMatches, &r.Sequences.ParentOwnerMismatches, &r.Sequences.SerialDependencies, &r.Sequences.IdentityDependencies); err != nil {
		return fail("sequence_catalog", err)
	}
	var runtimeExists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'), COALESCE((SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),false), has_schema_privilege('postgres','public','CREATE'), COALESCE((SELECT has_schema_privilege('espacigo_runtime','public','CREATE') FROM pg_catalog.pg_roles WHERE rolname='espacigo_runtime'),false)`).Scan(&runtimeExists, &r.RuntimeLogin, &r.PostgresPublicCreate, &r.RuntimePublicCreate); err != nil {
		return fail("role_schema_privileges", err)
	}
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT bool_or(m.set_option AND NOT m.inherit_option AND NOT m.admin_option) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles role ON role.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE role.rolname='espacigo_runtime' AND member.rolname='postgres'),false), COALESCE((SELECT bool_or(m.inherit_option) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles role ON role.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE role.rolname='espacigo_runtime' AND member.rolname='postgres'),false), COALESCE((SELECT bool_or(m.admin_option) FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles role ON role.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member WHERE role.rolname='espacigo_runtime' AND member.rolname='postgres'),false), COALESCE((SELECT pg_catalog.pg_has_role('postgres',r.oid,'SET') FROM pg_catalog.pg_roles r WHERE r.rolname='espacigo_runtime'),false)`).Scan(&r.RuntimeMembershipSet, &r.RuntimeMembershipInherit, &r.RuntimeMembershipAdmin, &r.RuntimeMembershipEffectiveSet); err != nil {
		return fail("membership_catalog", err)
	}
	if err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'SELECT')), count(*) FILTER(WHERE has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'INSERT') OR has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'UPDATE') OR has_column_privilege(to_regrole('espacigo_gcp03_observer'),c.oid,a.attnum,'REFERENCES')) FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace JOIN pg_catalog.pg_attribute a ON a.attrelid=c.oid AND a.attnum>0 AND NOT a.attisdropped WHERE n.nspname='public' AND c.relkind IN ('r','p')`).Scan(&r.ObserverColumnSelectCount, &r.ObserverColumnWriteCount); err != nil {
		return fail("observer_column_privileges", err)
	}
	// Each aggregate query runs only after the fixed relation and required columns are present.
	checks := []struct {
		name, table, columns, query string
		dest                        *int64
	}{
		{"valid_sessions", "sesion", "revocada_en,expira_en", `SELECT count(*) FROM public.sesion WHERE revocada_en IS NULL AND expira_en>transaction_timestamp()`, &r.ValidSessions},
		{"usable_action_tokens", "token_accion", "consumido_en,invalidado_en,expira_en", `SELECT count(*) FROM public.token_accion WHERE consumido_en IS NULL AND invalidado_en IS NULL AND expira_en>transaction_timestamp()`, &r.UsableActionTokens},
		{"pending_notices", "aviso_local", "estado", `SELECT count(*) FROM public.aviso_local WHERE estado IN ('pendiente','procesando')`, &r.PendingNotices},
		{"identity_event_leases", "outbox_evento_local", "lease_hasta", `SELECT count(*) FROM public.outbox_evento_local WHERE lease_hasta IS NOT NULL AND lease_hasta>transaction_timestamp()`, &r.IdentityEventLeases},
	}
	for _, c := range checks {
		var present bool
		if err = tx.QueryRow(ctx, `SELECT to_regclass($1) IS NOT NULL`, "public."+c.table).Scan(&present); err != nil {
			return fail("aggregate_catalog", err)
		}
		if present {
			var missing bool
			err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM unnest(string_to_array($2,',')) col WHERE NOT EXISTS(SELECT 1 FROM pg_catalog.pg_attribute a WHERE a.attrelid=to_regclass($1) AND a.attname=col AND a.attnum>0 AND NOT a.attisdropped))`, "public."+c.table, c.columns).Scan(&missing)
			if err != nil {
				return fail("aggregate_catalog", err)
			}
			if !missing {
				if err = tx.QueryRow(ctx, c.query).Scan(c.dest); err != nil {
					return fail("aggregate_count", err)
				}
				r.AggregateTablesPresent[c.name] = true
				continue
			}
		}
		r.AggregateTablesPresent[c.name] = false
	}
	if commit == nil {
		return fail("commit_configuration", nil)
	}
	if err = commit(ctx, tx); err != nil {
		r.FailureStage = "commit"
		r.SQLState = pgSQLState(err)
		r.TransactionOutcome = "outcome_uncertain"
		return r, errors.New("owner/ACL diagnostic commit outcome is uncertain")
	}
	r.TransactionOutcome = "committed"
	r.ReportComplete = true
	return r, nil
}

// migrationHistoryHashes keeps the v1 text ordering used by historical
// diagnostic evidence and adds a deterministic numeric-order v2 hash.
func migrationHistoryHashes(rows []migrationHistoryEntry) (string, string, error) {
	legacy := append([]migrationHistoryEntry(nil), rows...)
	canonical := append([]migrationHistoryEntry(nil), rows...)
	sort.Slice(legacy, func(i, j int) bool {
		if legacy[i].version == legacy[j].version {
			return legacy[i].checksum < legacy[j].checksum
		}
		return legacy[i].version < legacy[j].version
	})
	versions := make(map[int64]struct{}, len(canonical))
	for _, row := range canonical {
		version, err := strconv.ParseInt(row.version, 10, 64)
		if err != nil || version < 1 || strconv.FormatInt(version, 10) != row.version {
			return "", "", fmt.Errorf("invalid migration version")
		}
		if _, exists := versions[version]; exists {
			return "", "", fmt.Errorf("duplicate numeric migration version")
		}
		versions[version] = struct{}{}
	}
	sort.Slice(canonical, func(i, j int) bool {
		left, _ := strconv.ParseInt(canonical[i].version, 10, 64)
		right, _ := strconv.ParseInt(canonical[j].version, 10, 64)
		if left != right {
			return left < right
		}
		if canonical[i].version != canonical[j].version {
			return canonical[i].version < canonical[j].version
		}
		return canonical[i].checksum < canonical[j].checksum
	})
	serialize := func(items []migrationHistoryEntry) string {
		var b strings.Builder
		for _, row := range items {
			fmt.Fprintf(&b, "%s\x00%s\n", row.version, row.checksum)
		}
		return hashText(b.String())
	}
	return serialize(legacy), serialize(canonical), nil
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
