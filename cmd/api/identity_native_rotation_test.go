package main

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identitydeclaration"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type nativeIdentityCleanupAuthority struct {
	AuditDelete               bool
	PublicCreate              bool
	AuditSelect               map[string]bool
	ClaimsDelete              bool
	OutboxDelete              bool
	AuditACL                  []nativeIdentityCleanupACL
	AuditUnexpectedPrivileges []string
	RoleFlags                 []nativeIdentityCleanupRoleFlags
	Memberships               []nativeIdentityCleanupMembership
}

type nativeIdentityCleanupACL struct {
	Object    string
	Column    string
	Grantee   string
	Grantor   string
	Privilege string
	Grantable bool
}

type nativeIdentityCleanupRoleFlags struct {
	Name        string
	CanLogin    bool
	Superuser   bool
	Inherit     bool
	CreateRole  bool
	CreateDB    bool
	Replication bool
	BypassRLS   bool
}

type nativeIdentityCleanupMembership struct {
	Role    string
	Member  string
	Grantor string
	Admin   bool
	Inherit bool
	Set     bool
}

var nativeIdentityCleanupAuditColumns = []string{"actor_id", "recurso_tipo", "recurso_id", "accion", "correlacion_id"}

type nativeIdentityCleanupAuthorityQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func readNativeIdentityCleanupAuthority(ctx context.Context, queryer nativeIdentityCleanupAuthorityQuerier) (nativeIdentityCleanupAuthority, error) {
	var report nativeIdentityCleanupAuthority
	if err := queryer.QueryRow(ctx, `SELECT has_table_privilege(current_user,'public.evento_auditoria_local','DELETE'), has_schema_privilege(current_user,'public','CREATE'), has_table_privilege(current_user,'public.identity_rut_claim','DELETE'), has_table_privilege(current_user,'public.outbox_evento_local','DELETE')`).Scan(&report.AuditDelete, &report.PublicCreate, &report.ClaimsDelete, &report.OutboxDelete); err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup privilege readback failed")
	}
	membershipRows, err := queryer.Query(ctx, `SELECT granted.rolname::text,member.rolname::text,grantor.rolname::text,m.admin_option,m.inherit_option,m.set_option FROM pg_catalog.pg_auth_members m JOIN pg_catalog.pg_roles granted ON granted.oid=m.roleid JOIN pg_catalog.pg_roles member ON member.oid=m.member JOIN pg_catalog.pg_roles grantor ON grantor.oid=m.grantor ORDER BY granted.rolname,member.rolname,grantor.rolname,m.admin_option,m.inherit_option,m.set_option`)
	if err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup membership readback failed")
	}
	for membershipRows.Next() {
		var membership nativeIdentityCleanupMembership
		if err := membershipRows.Scan(&membership.Role, &membership.Member, &membership.Grantor, &membership.Admin, &membership.Inherit, &membership.Set); err != nil {
			membershipRows.Close()
			return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup membership scan failed")
		}
		report.Memberships = append(report.Memberships, membership)
	}
	if err := membershipRows.Err(); err != nil {
		membershipRows.Close()
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup membership iteration failed")
	}
	membershipRows.Close()
	roleNames := nativeIdentityCleanupAuthorityRoleNames(report.Memberships)
	roleRows, err := queryer.Query(ctx, `SELECT rolname::text,rolcanlogin,rolsuper,rolinherit,rolcreaterole,rolcreatedb,rolreplication,rolbypassrls FROM pg_catalog.pg_roles WHERE rolname::text=ANY($1::text[]) ORDER BY rolname`, roleNames)
	if err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup role readback failed")
	}
	for roleRows.Next() {
		var role nativeIdentityCleanupRoleFlags
		if err := roleRows.Scan(&role.Name, &role.CanLogin, &role.Superuser, &role.Inherit, &role.CreateRole, &role.CreateDB, &role.Replication, &role.BypassRLS); err != nil {
			roleRows.Close()
			return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup role scan failed")
		}
		report.RoleFlags = append(report.RoleFlags, role)
	}
	if err := roleRows.Err(); err != nil {
		roleRows.Close()
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture cleanup role iteration failed")
	}
	roleRows.Close()

	tablePrivilegeRows, err := queryer.Query(ctx, `SELECT privilege,has_table_privilege(current_user,'public.evento_auditoria_local',privilege) FROM unnest(ARRAY['INSERT','UPDATE','TRUNCATE','REFERENCES','TRIGGER','MAINTAIN']::text[]) AS p(privilege) ORDER BY privilege`)
	if err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-table privilege readback failed")
	}
	tablePrivileges := make(map[string]bool)
	for tablePrivilegeRows.Next() {
		var privilege string
		var allowed bool
		if err := tablePrivilegeRows.Scan(&privilege, &allowed); err != nil {
			tablePrivilegeRows.Close()
			return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-table privilege scan failed")
		}
		tablePrivileges[privilege] = allowed
		if allowed {
			report.AuditUnexpectedPrivileges = append(report.AuditUnexpectedPrivileges, privilege+" public.evento_auditoria_local")
		}
	}
	if err := tablePrivilegeRows.Err(); err != nil {
		tablePrivilegeRows.Close()
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-table privilege iteration failed")
	}
	tablePrivilegeRows.Close()

	requiredColumns := make(map[string]bool, len(nativeIdentityCleanupAuditColumns))
	report.AuditSelect = make(map[string]bool, len(nativeIdentityCleanupAuditColumns))
	for _, column := range nativeIdentityCleanupAuditColumns {
		requiredColumns[column] = true
	}
	columnRows, err := queryer.Query(ctx, `SELECT a.attname::text,has_column_privilege(current_user,'public.evento_auditoria_local',a.attname,'SELECT'),has_column_privilege(current_user,'public.evento_auditoria_local',a.attname,'INSERT'),has_column_privilege(current_user,'public.evento_auditoria_local',a.attname,'UPDATE'),has_column_privilege(current_user,'public.evento_auditoria_local',a.attname,'REFERENCES') FROM pg_catalog.pg_attribute a WHERE a.attrelid=pg_catalog.to_regclass('public.evento_auditoria_local') AND a.attnum>0 AND NOT a.attisdropped ORDER BY a.attnum`)
	if err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-column privilege readback failed")
	}
	for columnRows.Next() {
		var column string
		var selectAllowed, insertAllowed, updateAllowed, referencesAllowed bool
		if err := columnRows.Scan(&column, &selectAllowed, &insertAllowed, &updateAllowed, &referencesAllowed); err != nil {
			columnRows.Close()
			return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-column privilege scan failed")
		}
		if requiredColumns[column] {
			report.AuditSelect[column] = selectAllowed
		} else if selectAllowed {
			report.AuditUnexpectedPrivileges = append(report.AuditUnexpectedPrivileges, "SELECT public.evento_auditoria_local."+column)
		}
		if insertAllowed && !tablePrivileges["INSERT"] {
			report.AuditUnexpectedPrivileges = append(report.AuditUnexpectedPrivileges, "INSERT public.evento_auditoria_local."+column)
		}
		if updateAllowed && !tablePrivileges["UPDATE"] {
			report.AuditUnexpectedPrivileges = append(report.AuditUnexpectedPrivileges, "UPDATE public.evento_auditoria_local."+column)
		}
		if referencesAllowed && !tablePrivileges["REFERENCES"] {
			report.AuditUnexpectedPrivileges = append(report.AuditUnexpectedPrivileges, "REFERENCES public.evento_auditoria_local."+column)
		}
	}
	if err := columnRows.Err(); err != nil {
		columnRows.Close()
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-column privilege iteration failed")
	}
	columnRows.Close()
	if len(report.AuditSelect) != len(nativeIdentityCleanupAuditColumns) {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit-column privilege report is incomplete")
	}

	aclRows, err := queryer.Query(ctx, `WITH objects AS (
 SELECT 'public.evento_auditoria_local'::text AS object,''::text AS column_name,COALESCE(c.relacl,pg_catalog.acldefault('r',c.relowner)) AS acl
 FROM pg_catalog.pg_class c WHERE c.oid=pg_catalog.to_regclass('public.evento_auditoria_local')
 UNION ALL
 SELECT 'public.evento_auditoria_local',a.attname::text,CASE WHEN pg_catalog.cardinality(a.attacl)>0 THEN a.attacl ELSE NULL::pg_catalog.aclitem[] END
 FROM pg_catalog.pg_attribute a WHERE a.attrelid=pg_catalog.to_regclass('public.evento_auditoria_local') AND a.attnum>0 AND NOT a.attisdropped
), acl_rows AS (
 SELECT o.object,o.column_name,CASE WHEN x.grantee=0 THEN 'PUBLIC' ELSE pg_catalog.pg_get_userbyid(x.grantee)::text END AS grantee,pg_catalog.pg_get_userbyid(x.grantor)::text AS grantor,x.privilege_type AS privilege,x.is_grantable AS grantable
 FROM objects o CROSS JOIN LATERAL pg_catalog.aclexplode(o.acl) x
)
	SELECT object,column_name,grantee,grantor,privilege,grantable FROM acl_rows WHERE grantee='PUBLIC' OR grantee=ANY($1::text[]) ORDER BY object,column_name,grantee,grantor,privilege,grantable`, roleNames)
	if err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit ACL readback failed")
	}
	defer aclRows.Close()
	for aclRows.Next() {
		var acl nativeIdentityCleanupACL
		if err := aclRows.Scan(&acl.Object, &acl.Column, &acl.Grantee, &acl.Grantor, &acl.Privilege, &acl.Grantable); err != nil {
			return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit ACL scan failed")
		}
		report.AuditACL = append(report.AuditACL, acl)
	}
	if err := aclRows.Err(); err != nil {
		return nativeIdentityCleanupAuthority{}, errors.New("exact-fixture audit ACL iteration failed")
	}
	return report, nil
}

func nativeIdentityCleanupReachableRoleNames(memberships []nativeIdentityCleanupMembership) []string {
	reachable := map[string]bool{"espacigo_schema_owner": true}
	for changed := true; changed; {
		changed = false
		for _, membership := range memberships {
			if reachable[membership.Member] && (membership.Inherit || membership.Set) && !reachable[membership.Role] {
				reachable[membership.Role] = true
				changed = true
			}
		}
	}
	result := make([]string, 0, len(reachable))
	for role := range reachable {
		result = append(result, role)
	}
	sort.Strings(result)
	return result
}

func nativeIdentityCleanupAuthorityRoleNames(memberships []nativeIdentityCleanupMembership) []string {
	roles := nativeIdentityCleanupReachableRoleNames(memberships)
	hasMigrator := false
	for _, role := range roles {
		if role == "espacigo_migrator_login" {
			hasMigrator = true
			break
		}
	}
	if !hasMigrator {
		roles = append(roles, "espacigo_migrator_login")
		sort.Strings(roles)
	}
	return roles
}

func nativeIdentityCleanupRoleIssues(roles []nativeIdentityCleanupRoleFlags, memberships []nativeIdentityCleanupMembership) []string {
	reachable := nativeIdentityCleanupReachableRoleNames(memberships)
	reachableSet := make(map[string]bool, len(reachable))
	for _, role := range reachable {
		reachableSet[role] = true
	}
	expectedRoles := []nativeIdentityCleanupRoleFlags{
		{Name: "espacigo_migrator_login", CanLogin: true},
		{Name: "espacigo_document_verification_fn"},
		{Name: "espacigo_schema_owner"},
	}
	expectedMemberships := []nativeIdentityCleanupMembership{
		{Role: "espacigo_schema_owner", Member: "espacigo_migrator_login", Grantor: "postgres", Set: true},
		{Role: "espacigo_document_verification_fn", Member: "espacigo_schema_owner", Grantor: "postgres", Set: true},
	}
	var issues []string
	matchedRoles := make([]bool, len(expectedRoles))
	seenRoleNames := make(map[string]bool, len(roles))
	for _, actual := range roles {
		matched := false
		for i, expected := range expectedRoles {
			if actual == expected && !matchedRoles[i] {
				matchedRoles[i] = true
				matched = true
				break
			}
		}
		if !matched {
			issues = append(issues, "unsafe cleanup role attributes: "+actual.Name)
			seenRoleNames[actual.Name] = true
		}
	}
	for i, expected := range expectedRoles {
		if !matchedRoles[i] && !seenRoleNames[expected.Name] {
			issues = append(issues, "unsafe cleanup role attributes: "+expected.Name)
		}
	}

	matchedMemberships := make([]bool, len(expectedMemberships))
	for _, actual := range memberships {
		if !reachableSet[actual.Member] && actual.Role != "espacigo_schema_owner" {
			continue
		}
		matched := false
		for i, expected := range expectedMemberships {
			if actual == expected && !matchedMemberships[i] {
				matchedMemberships[i] = true
				matched = true
				break
			}
		}
		if !matched {
			issues = append(issues, "unexpected cleanup role membership: role="+actual.Role+" member="+actual.Member)
		}
	}
	for i, expected := range expectedMemberships {
		if !matchedMemberships[i] {
			issues = append(issues, "required cleanup role membership missing: role="+expected.Role+" member="+expected.Member)
		}
	}
	return issues
}

func nativeIdentityCleanupAuthorityIssues(report nativeIdentityCleanupAuthority) []string {
	var issues []string
	issues = append(issues, nativeIdentityCleanupRoleIssues(report.RoleFlags, report.Memberships)...)
	if !report.AuditDelete {
		issues = append(issues, "DELETE public.evento_auditoria_local")
	}
	for _, column := range nativeIdentityCleanupAuditColumns {
		if !report.AuditSelect[column] {
			issues = append(issues, "SELECT public.evento_auditoria_local."+column)
		}
	}
	if !report.ClaimsDelete {
		issues = append(issues, "DELETE public.identity_rut_claim")
	}
	if !report.OutboxDelete {
		issues = append(issues, "DELETE public.outbox_evento_local")
	}
	expectedACL := []nativeIdentityCleanupACL{{Object: "public.evento_auditoria_local", Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "DELETE"}}
	for _, column := range nativeIdentityCleanupAuditColumns {
		expectedACL = append(expectedACL, nativeIdentityCleanupACL{Object: "public.evento_auditoria_local", Column: column, Grantee: "espacigo_schema_owner", Grantor: "postgres", Privilege: "SELECT"})
	}
	matchedACL := make([]bool, len(expectedACL))
	for _, actual := range report.AuditACL {
		matched := false
		for i, expected := range expectedACL {
			if actual == expected {
				matchedACL[i] = true
				matched = true
				break
			}
		}
		if !matched {
			object := actual.Object
			if actual.Column != "" {
				object += "." + actual.Column
			}
			grantable := ""
			if actual.Grantable {
				grantable = " WITH GRANT OPTION"
			}
			issues = append(issues, "unexpected audit ACL: "+actual.Grantee+" "+actual.Privilege+grantable+" on "+object)
		}
	}
	for i, expected := range expectedACL {
		if matchedACL[i] {
			continue
		}
		effective := report.AuditDelete
		if expected.Column != "" {
			effective = report.AuditSelect[expected.Column]
		}
		if effective {
			object := expected.Object
			if expected.Column != "" {
				object += "." + expected.Column
			}
			issues = append(issues, "required audit privilege is available through an unapproved grant path: "+expected.Privilege+" "+object)
		}
	}
	issues = append(issues, report.AuditUnexpectedPrivileges...)
	if report.PublicCreate {
		issues = append(issues, "unexpected CREATE on public schema")
	}
	return issues
}

// TestNativeIdentityRUTKeyRotationSequential is an explicitly opt-in native
// Save/restart/cross-account RUT-claim collision proof. It deliberately does
// not start an API server or general worker. An additional explicit flag adds
// the in-process synthetic provider/approval/outbox gate, never external Didit.
func TestNativeIdentityRUTKeyRotationSequential(t *testing.T) {
	if os.Getenv("IDENTITY_KYC_RUT_ROTATION_NATIVE_TEST") != "1" {
		t.Skip("set IDENTITY_KYC_RUT_ROTATION_NATIVE_TEST=1 to enable native RUT-key rotation proof")
	}

	// Keep this test's private policy and credentials behind its explicit gate.
	rawDSN, cleanupDSN := os.Getenv(identityDeclarationNativeTestDSNEnv), os.Getenv(identityDeclarationNativeCleanupDSNEnv)
	accountA, accountB := os.Getenv(identityDeclarationNativeTestAccountEnv), os.Getenv(identityDeclarationNativeOtherAccountEnv)
	if rawDSN == "" || cleanupDSN == "" || accountA == "" || accountB == "" {
		t.Fatal("native rotation requires pinned app/cleanup DSNs and two authorized account IDs")
	}
	if err := validateIdentityDeclarationRuntimeDSN(rawDSN); err != nil {
		t.Fatal(err)
	}
	if err := validateIdentityDeclarationCleanupDSN(cleanupDSN); err != nil {
		t.Fatal(err)
	}
	policyPath, err := filepath.Abs(filepath.Join("..", "..", ".local", "testing", "test-account-policy.json"))
	if err != nil {
		t.Fatal("resolve private policy path")
	}
	if err := validateNativeIdentityPrivateFile(policyPath); err != nil {
		t.Fatal(err)
	}
	policy, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatal("read authorized private policy")
	}
	if err := validateNativeIdentityAccountPair(policy, accountA, accountB); err != nil {
		t.Fatal(err)
	}
	appPassfile, err := filepath.Abs(filepath.Join("..", "..", ".local", "testing", "kyc-native", "api.pgpass"))
	if err != nil {
		t.Fatal("resolve app passfile")
	}
	if _, err := parseNativeIdentityConfig(rawDSN, appPassfile); err != nil {
		t.Fatal(err)
	}
	cleanupConfig, err := parseNativeIdentityConfig(cleanupDSN, "/home/nandev/.local/share/espacigo-native-secrets/pgpass")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	source, err := migrator.DiscoverMigrations(filepath.Join("..", "..", "db", "migrations"))
	if err != nil || len(source) < identityDeclarationNativeLedgerVersion {
		t.Fatal("source migration inventory incomplete")
	}
	if err := readNativeIdentityPreflight(ctx, source, identityDeclarationNativeLedgerVersion); err != nil {
		t.Fatal(err)
	}
	if source[94-1].Version != 94 || source[94-1].Checksum != identityDeclarationNativeV94Checksum {
		t.Fatal("pinned V94 checksum mismatch")
	}

	cleanupConfig.MaxConns = 1
	cleanupConfig.AfterConnect = func(c context.Context, conn *pgx.Conn) error {
		if _, e := conn.Exec(c, `SET ROLE espacigo_schema_owner`); e != nil {
			return e
		}
		var session, current string
		if e := conn.QueryRow(c, `SELECT session_user,current_user`).Scan(&session, &current); e != nil {
			return e
		}
		if session != "espacigo_migrator_login" || current != "espacigo_schema_owner" {
			return errors.New("cleanup principal mismatch")
		}
		return nil
	}
	cleanupPool, err := pgxpool.NewWithConfig(ctx, cleanupConfig)
	if err != nil {
		t.Fatal("open authorized cleanup pool")
	}
	t.Cleanup(cleanupPool.Close)
	if err := cleanupPool.QueryRow(ctx, `SELECT 1`).Scan(new(int)); err != nil {
		t.Fatal("cleanup role query failed")
	}
	authority, err := readNativeIdentityCleanupAuthority(ctx, cleanupPool)
	if err != nil {
		t.Fatal(err)
	}
	if issues := nativeIdentityCleanupAuthorityIssues(authority); len(issues) != 0 {
		t.Fatalf("exact-fixture cleanup authority unavailable: %v", issues)
	}

	configureApp := func() *pgxpool.Pool {
		cfg, e := parseNativeIdentityConfig(rawDSN, appPassfile)
		if e != nil {
			t.Fatal("parse app DSN")
		}
		cfg.MaxConns = 1
		cfg.AfterConnect = func(c context.Context, conn *pgx.Conn) error {
			if _, e := conn.Exec(c, `SET ROLE espacigo_runtime`); e != nil {
				return e
			}
			var session, current string
			if e := conn.QueryRow(c, `SELECT session_user,current_user`).Scan(&session, &current); e != nil {
				return e
			}
			if session != "espacigo_api_login" || current != "espacigo_runtime" {
				return errors.New("app role mismatch")
			}
			return nil
		}
		p, e := pgxpool.NewWithConfig(ctx, cfg)
		if e != nil {
			t.Fatal("open app pool")
		}
		return p
	}
	appPool := configureApp()
	t.Cleanup(appPool.Close)
	restartedPool := configureApp()
	t.Cleanup(restartedPool.Close)
	var eligible, claims int
	if err := appPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=ANY($1::uuid[]))+(SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=ANY($1::uuid[])),(SELECT count(*) FROM public.identity_rut_claim)`, []string{accountA, accountB}).Scan(&eligible, &claims); err != nil {
		t.Fatal("rotation empty-scope preflight failed")
	}
	if eligible != 0 || claims != 0 {
		t.Fatal("rotation requires empty declaration scope and no global claims; preserve existing data")
	}
	var activeAccounts int
	if err := appPool.QueryRow(ctx, `SELECT count(*) FROM public.usuario WHERE id=ANY($1::uuid[]) AND estado='activo'`, []string{accountA, accountB}).Scan(&activeAccounts); err != nil || activeAccounts != 2 {
		t.Fatal("both authorized fixture accounts must exist and be active")
	}
	pending, err := identitydeclaration.NewPostgresSessionDeletionQueue(appPool).PendingSessionDeletions(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatal("rotation requires no existing pending deletion work")
	}

	journalDir, err := filepath.Abs(filepath.Join("..", "..", ".local", "testing", "kyc-native", "rotation-preapply"))
	if err != nil {
		t.Fatal("resolve private journal directory")
	}
	if err := validateNativeIdentityPrivateDir(journalDir); err != nil {
		t.Fatal("rotation journal directory must already exist privately")
	}
	repo := identitydeclaration.New(appPool)
	pair, err := newNativeDeclarationIntentPairRepository(repo, journalDir, accountA, accountB)
	if err != nil {
		t.Fatal(err)
	}
	tracked := pair.trackedRepositories()
	if len(tracked) != 2 || tracked[0].intentWritten || tracked[1].intentWritten {
		t.Fatal("unexpected occupied pair journal")
	}
	var approvalAttempts *nativeAttemptJournalRepository
	attemptJournalPath := filepath.Join(journalDir, "provider-attempt-intent.json")
	if os.Getenv("IDENTITY_KYC_RUT_ROTATION_NATIVE_APPROVAL") == "1" {
		approvalAttempts, err = newNativeAttemptJournalRepository(identitydeclaration.New(appPool), attemptJournalPath, accountA, func() nativeDeclarationIntent { return tracked[0].trustedIntent })
		if err != nil {
			t.Fatal("cannot prepare fresh approval attempt journal")
		}
	}
	// Register bounded cleanup before any Save. Cleanup pool was registered first,
	// so Go's LIFO cleanup order keeps it open until both fixture purges complete.
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer ccancel()
		var receipts []nativeDeclarationIntent
		var receiptPaths []string
		for _, r := range tracked {
			if !r.intentWritten {
				continue
			}
			if e := validateNativeIdentityPrivateFile(r.journalPath); e != nil {
				t.Error("unsafe journal; preserve recovery evidence")
				return
			}
			b, e := os.ReadFile(r.journalPath)
			if e != nil {
				t.Error("cannot read trusted receipt; preserve journal")
				return
			}
			var intent nativeDeclarationIntent
			if e = json.Unmarshal(b, &intent); e != nil || intent != r.trustedIntent {
				t.Error("invalid trusted receipt; preserve journal")
				return
			}
			receipts = append(receipts, intent)
			receiptPaths = append(receiptPaths, r.journalPath)
		}
		var attemptScope *nativeAttemptIntent
		if approvalAttempts != nil {
			if expected, written := approvalAttempts.Snapshot(); written {
				actual, e := readNativeAttemptIntent(attemptJournalPath)
				if e != nil || actual != expected {
					t.Error("invalid attempt receipt; preserve all recovery journals")
					return
				}
				attemptScope = &expected
				receiptPaths = append(receiptPaths, attemptJournalPath)
			}
		}
		for _, intent := range receipts {
			var ownAttempt *nativeAttemptIntent
			if attemptScope != nil && attemptScope.AccountID == intent.AccountID {
				ownAttempt = attemptScope
			}
			if e := cleanupNativeApprovalFixture(cctx, cleanupPool, []nativeDeclarationIntent{intent}, ownAttempt); e != nil {
				t.Error("bounded cleanup failed; preserve all journals")
				return
			}
		}
		for _, intent := range receipts {
			var left int
			if e := cleanupPool.QueryRow(cctx, `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE id=$1::uuid)+(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$2::uuid)+(SELECT count(*) FROM public.evento_auditoria_local WHERE correlacion_id=$3 OR recurso_id=$1::uuid)+(SELECT count(*) FROM public.identity_provider_attempt WHERE account_id=$2::uuid)`, intent.DeclarationID, intent.AccountID, intent.CorrelationID).Scan(&left); e != nil || left != 0 {
				t.Error("independent cleanup readback failed; preserve all journals")
				return
			}
		}
		if attemptScope != nil {
			var left int
			if e := cleanupPool.QueryRow(cctx, `SELECT (SELECT count(*) FROM public.identity_provider_attempt WHERE id=$1::uuid)+(SELECT count(*) FROM public.identity_provider_event WHERE attempt_id=$1::uuid)+(SELECT count(*) FROM public.outbox_evento_local WHERE agregado_id=$1::uuid)`, attemptScope.AttemptID).Scan(&left); e != nil || left != 0 {
				t.Error("independent approval cleanup readback failed; preserve all journals")
				return
			}
		}
		for _, path := range receiptPaths {
			if e := os.Remove(path); e != nil {
				t.Error("cannot retire cleaned journal")
				return
			}
		}
		d, e := os.Open(journalDir)
		if e != nil {
			t.Error("cannot open journal directory for retirement sync")
			return
		}
		defer d.Close()
		if e := d.Sync(); e != nil {
			t.Error("cannot sync journal retirement")
		}
	})

	keyring, err := declarationcrypto.New("synthetic-aes", map[string][]byte{"synthetic-aes": []byte("12345678901234567890123456789012")})
	if err != nil {
		t.Fatal(err)
	}
	// Require a wholly empty claims table above before using the test-only tags.
	mkService := func(active string, repository verification.DeclarationRevisionRepository) *verification.DeclarationService {
		keys, e := verification.NewRutHMACKeyring(active, map[string][]byte{"r2-native-old": []byte("synthetic-rut-old-key-material-32bytes"), "r2-native-new": []byte("synthetic-rut-new-key-material-32bytes")})
		if e != nil {
			t.Fatal(e)
		}
		s, e := verification.NewDeclarationServiceWithRutKeyring(repository, keyring, credentials.Generator{}, time.Now, identityDeclarationRetentionPolicy, defaultIdentityDeclarationConsentVersion, keys)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	payload := verification.DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11222333-9", IDCountry: "CHL", DocumentType: "ID"}
	if os.Getenv("IDENTITY_KYC_RUT_ROTATION_NATIVE_RACE") == "1" {
		if approvalAttempts != nil {
			t.Fatal("race and approval variants require separate fresh journals")
		}
		runNativeRotationClaimRace(t, ctx, appPool, restartedPool, cleanupPool, tracked, payload, mkService, accountA, accountB)
		return
	}
	first := mkService("r2-native-old", pair)
	saved, err := first.Save(ctx, accountA, payload, true, defaultIdentityDeclarationConsentVersion)
	if err != nil {
		t.Fatalf("native rotation initial Save failed: %T", err)
	}
	if !tracked[0].intentWritten || saved.ID != tracked[0].trustedIntent.DeclarationID {
		t.Fatal("Save did not match pre-journaled declaration input")
	}
	restarted := mkService("r2-native-new", identitydeclaration.New(restartedPool))
	readback, err := restarted.GetCurrent(ctx, accountA)
	if err != nil || readback.ID != saved.ID || readback.Payload != payload {
		t.Fatal("new-pool active-key rotation did not decrypt the saved server snapshot")
	}
	oldSet, err := first.RutKeyring().Fingerprints(payload.RUT)
	if err != nil {
		t.Fatal(err)
	}
	newSet, err := restarted.RutKeyring().Fingerprints(payload.RUT)
	if err != nil {
		t.Fatal(err)
	}
	if oldSet.Active == newSet.Active {
		t.Fatal("fixture did not actually change the active HMAC version")
	}
	canonical, err := identitydeclaration.NewRutClaimStore(restartedPool).ResolveCandidates(ctx, accountA, saved.ID, newSet)
	if err != nil || canonical != oldSet.Active {
		t.Fatal("native restart did not resolve the persisted legacy canonical fingerprint")
	}
	newOnly := verification.RutFingerprintSet{Active: newSet.Active, Candidates: []string{newSet.Active}}
	if _, err := identitydeclaration.NewRutClaimStore(restartedPool).ResolveCandidates(ctx, accountA, saved.ID, newOnly); !errors.Is(err, verification.ErrRutHMACKeyUnavailable) {
		t.Fatal("missing historical version did not fail closed in PostgreSQL")
	}
	second := mkService("r2-native-new", pair)
	if _, err := second.Save(ctx, accountB, payload, true, defaultIdentityDeclarationConsentVersion); !errors.Is(err, verification.ErrConflict) {
		t.Fatalf("cross-account rotated Save did not conflict: %T", err)
	}
	if !tracked[1].intentWritten {
		t.Fatal("failed Save was not journaled before its transaction")
	}
	var primaryRows, secondaryRows, claimRows int
	if err := cleanupPool.QueryRow(ctx, `SELECT (SELECT count(*) FROM public.identity_declaration_revision WHERE id=$1::uuid AND account_id=$2::uuid),(SELECT count(*) FROM public.identity_declaration_revision WHERE account_id=$3::uuid)+(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$3::uuid)+(SELECT count(*) FROM public.evento_auditoria_local WHERE actor_id=$3::uuid AND recurso_tipo='identity_declaration'),(SELECT count(*) FROM public.identity_rut_claim WHERE account_id=$2::uuid AND declaration_id=$1::uuid AND fingerprint=$4 AND verified_at IS NULL AND expires_at IS NOT NULL)`, saved.ID, accountA, accountB, oldSet.Active).Scan(&primaryRows, &secondaryRows, &claimRows); err != nil || primaryRows != 1 || secondaryRows != 0 || claimRows != 1 {
		t.Fatal("rotated conflict changed canonical claim or left partial secondary rows")
	}
	t.Log("PASS: native Save, new-pool rotation/readback, legacy canonical resolution, missing-version denial and cross-account rollback")
	if approvalAttempts != nil {
		runNativeRotationApproval(t, ctx, appPool, restartedPool, restarted, keyring, approvalAttempts, accountA, saved.ID, oldSet.Active)
	}
}
