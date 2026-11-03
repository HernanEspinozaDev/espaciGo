package documentverificationpg

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	reviewCommitDatabase = "espacigo_f5_gate_20261101"
	reviewCommitServer   = "172.28.0.2"
)

func TestParseF5ReviewConnectionURLRejectsNonCanonicalComponents(t *testing.T) {
	valid := "postgres://postgres@127.0.0.1:15434/" + reviewCommitDatabase + "?sslmode=disable"
	if _, err := parseF5ReviewConnectionURL(valid); err != nil {
		t.Fatalf("canonical F5 URL rejected: %v", err)
	}
	for name, dsn := range map[string]string{
		"extra path":         "postgres://postgres@127.0.0.1:15434/" + reviewCommitDatabase + "/extra?sslmode=disable",
		"opaque URL":         "postgres:opaque",
		"forced empty query": "postgres://postgres@127.0.0.1:15434/" + reviewCommitDatabase + "?",
		"fragment":           valid + "#other",
		"wrong host":         "postgres://postgres@localhost:15434/" + reviewCommitDatabase + "?sslmode=disable",
		"encoded path":       "postgres://postgres@127.0.0.1:15434/%2F" + reviewCommitDatabase + "?sslmode=disable",
		"encoded username":   "postgres://%70ostgres@127.0.0.1:15434/" + reviewCommitDatabase + "?sslmode=disable",
		"password component": "postgres://postgres:@127.0.0.1:15434/" + reviewCommitDatabase + "?sslmode=disable",
		"unknown query":      valid + "&application_name=unexpected",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseF5ReviewConnectionURL(dsn); err == nil {
				t.Fatal("non-canonical F5 URL was accepted")
			}
		})
	}
}

func TestReviewCaseAuditPrefixMatchesOnlyExactOrderedPrefix(t *testing.T) {
	lifecycle := []string{"case_created:1", "file_added:2", "case_transitioned:3", "review_decision:4", "revoked:5"}
	tests := []struct {
		name     string
		actual   []string
		count    int
		wantPass bool
	}{
		{name: "empty prefix", actual: nil, count: 0, wantPass: true},
		{name: "one event", actual: lifecycle[:1], count: 1, wantPass: true},
		{name: "complete lifecycle", actual: lifecycle, count: 5, wantPass: true},
		{name: "out of order", actual: []string{"file_added:2", "case_created:1"}, count: 2},
		{name: "revision gap", actual: []string{"case_created:1", "case_transitioned:3"}, count: 2},
		{name: "unexpected event", actual: []string{"case_created:1", "scan_result:2"}, count: 2},
		{name: "count mismatch", actual: lifecycle[:1], count: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reviewCaseAuditPrefixMatches(tt.actual, tt.count); got != tt.wantPass {
				t.Fatalf("reviewCaseAuditPrefixMatches(%v, %d) = %t, want %t", tt.actual, tt.count, got, tt.wantPass)
			}
		})
	}
}

func TestExpectedReviewGrantCleanupRowsRetainsCommittedGrantExpectation(t *testing.T) {
	tests := []struct {
		name                string
		commitAcknowledged  bool
		rows, grantedEvents int
		want                int
	}{
		{name: "acknowledged commit requires row even when absent", commitAcknowledged: true, want: 1},
		{name: "readback row resolves ambiguous commit", rows: 1, grantedEvents: 1, want: 1},
		{name: "append-only grant event resolves ambiguous commit", grantedEvents: 1, want: 1},
		{name: "uncommitted setup leaves no expected row", want: 0},
		{name: "duplicate rows still expect exactly one", rows: 2, grantedEvents: 1, want: 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expectedReviewGrantCleanupRows(tt.commitAcknowledged, tt.rows, tt.grantedEvents); got != tt.want {
				t.Fatalf("expectedReviewGrantCleanupRows(%t, %d, %d) = %d, want %d", tt.commitAcknowledged, tt.rows, tt.grantedEvents, got, tt.want)
			}
		})
	}
}

func reviewCaseAuditPrefixMatches(actual []string, count int) bool {
	expected := [...]string{"case_created:1", "file_added:2", "case_transitioned:3", "review_decision:4", "revoked:5"}
	if count < 0 || count > len(expected) || len(actual) != count {
		return false
	}
	for i := 0; i < count; i++ {
		if actual[i] != expected[i] {
			return false
		}
	}
	return true
}

func expectedReviewGrantCleanupRows(commitAcknowledged bool, rows, grantedEvents int) int {
	if commitAcknowledged || rows > 0 || grantedEvents > 0 {
		return 1
	}
	return 0
}

type reviewGrantIdentity struct {
	id, reviewerID, capability, scopeKind, scopeID, grantorID, reasonCode string
	grantedAt, expiresAt                                                  time.Time
}

func TestReviewGrantIdentityRequiresExactImmutableFieldsAndTimestamps(t *testing.T) {
	want := reviewGrantIdentity{
		id: "g", reviewerID: "r", capability: "review_identity", scopeKind: "identity_subject",
		scopeID: "o", grantorID: "o", reasonCode: "temporary_coverage",
		grantedAt: time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC),
		expiresAt: time.Date(2026, 11, 3, 13, 0, 0, 0, time.UTC),
	}
	tests := []struct {
		name   string
		mutate func(*reviewGrantIdentity)
		want   bool
	}{
		{name: "exact fixture", want: true},
		{name: "wrong id", mutate: func(g *reviewGrantIdentity) { g.id = "other" }},
		{name: "wrong reviewer", mutate: func(g *reviewGrantIdentity) { g.reviewerID = "other" }},
		{name: "wrong capability", mutate: func(g *reviewGrantIdentity) { g.capability = "review_space" }},
		{name: "wrong scope kind", mutate: func(g *reviewGrantIdentity) { g.scopeKind = "space" }},
		{name: "wrong scope", mutate: func(g *reviewGrantIdentity) { g.scopeID = "other" }},
		{name: "wrong grantor", mutate: func(g *reviewGrantIdentity) { g.grantorID = "other" }},
		{name: "wrong grant reason", mutate: func(g *reviewGrantIdentity) { g.reasonCode = "access_review" }},
		{name: "wrong granted timestamp", mutate: func(g *reviewGrantIdentity) { g.grantedAt = g.grantedAt.Add(time.Second) }},
		{name: "wrong expiry timestamp", mutate: func(g *reviewGrantIdentity) { g.expiresAt = g.expiresAt.Add(time.Second) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := want
			if tt.mutate != nil {
				tt.mutate(&actual)
			}
			if got := reviewGrantIdentityMatches(actual, want); got != tt.want {
				t.Fatalf("reviewGrantIdentityMatches(...) = %t, want %t", got, tt.want)
			}
		})
	}
}

func reviewGrantIdentityMatches(actual, expected reviewGrantIdentity) bool {
	return actual.id == expected.id && actual.reviewerID == expected.reviewerID &&
		actual.capability == expected.capability && actual.scopeKind == expected.scopeKind &&
		actual.scopeID == expected.scopeID && actual.grantorID == expected.grantorID &&
		actual.reasonCode == expected.reasonCode && actual.grantedAt.Equal(expected.grantedAt) &&
		actual.expiresAt.Equal(expected.expiresAt)
}

type reviewGrantAuditEvent struct {
	id                                                           int64
	grantID, reviewerID, capability, scopeKind, scopeID, actorID string
	action, reasonCode                                           string
	occurredAt                                                   time.Time
}

func TestReviewGrantAuditLifecycleRequiresExactOrderedRows(t *testing.T) {
	grant := reviewGrantAuditEvent{grantID: "g", reviewerID: "r", capability: "review_identity", scopeKind: "identity_subject", scopeID: "o", actorID: "o", action: "granted", reasonCode: "temporary_coverage"}
	revoke := reviewGrantAuditEvent{grantID: "g", reviewerID: "r", capability: "review_identity", scopeKind: "identity_subject", scopeID: "o", actorID: "r", action: "revoked", reasonCode: "temporary_coverage_ended"}
	actualGrant := grant
	actualGrant.id, actualGrant.occurredAt = 1, time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	actualRevoke := revoke
	actualRevoke.id, actualRevoke.occurredAt = 2, time.Date(2026, 11, 3, 12, 1, 0, 0, time.UTC)
	tests := []struct {
		name   string
		actual []reviewGrantAuditEvent
		want   bool
	}{
		{name: "exact ordered lifecycle", actual: []reviewGrantAuditEvent{actualGrant, actualRevoke}, want: true},
		{name: "missing revoke", actual: []reviewGrantAuditEvent{actualGrant}},
		{name: "extra duplicate event", actual: []reviewGrantAuditEvent{actualGrant, actualGrant, actualRevoke}},
		{name: "reordered lifecycle", actual: []reviewGrantAuditEvent{actualRevoke, actualGrant}},
		{name: "wrong grant actor", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.actorID = "other"; return e }(), actualRevoke}},
		{name: "wrong reviewer", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.reviewerID = "other"; return e }(), actualRevoke}},
		{name: "wrong capability", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.capability = "review_space"; return e }(), actualRevoke}},
		{name: "wrong scope kind", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.scopeKind = "space"; return e }(), actualRevoke}},
		{name: "wrong scope id", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.scopeID = "other"; return e }(), actualRevoke}},
		{name: "wrong grant id", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.grantID = "other"; return e }(), actualRevoke}},
		{name: "wrong revoke reason", actual: []reviewGrantAuditEvent{actualGrant, func() reviewGrantAuditEvent { e := actualRevoke; e.reasonCode = "access_review"; return e }()}},
		{name: "invalid event identity", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.id = 0; return e }(), actualRevoke}},
		{name: "timestamps contradict event order", actual: []reviewGrantAuditEvent{func() reviewGrantAuditEvent { e := actualGrant; e.occurredAt = actualRevoke.occurredAt; return e }(), func() reviewGrantAuditEvent { e := actualRevoke; e.occurredAt = actualGrant.occurredAt; return e }()}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reviewGrantAuditEventsMatch(tt.actual, []reviewGrantAuditEvent{grant, revoke}); got != tt.want {
				t.Fatalf("reviewGrantAuditEventsMatch(...) = %t, want %t", got, tt.want)
			}
		})
	}
}

func reviewGrantAuditEventsMatch(actual, expected []reviewGrantAuditEvent) bool {
	if len(actual) != len(expected) {
		return false
	}
	var previousID int64
	var previousOccurredAt time.Time
	for i, event := range actual {
		want := expected[i]
		if event.id <= previousID || event.occurredAt.IsZero() ||
			(i > 0 && event.occurredAt.Before(previousOccurredAt)) ||
			event.grantID != want.grantID || event.reviewerID != want.reviewerID ||
			event.capability != want.capability || event.scopeKind != want.scopeKind ||
			event.scopeID != want.scopeID || event.actorID != want.actorID ||
			event.action != want.action || event.reasonCode != want.reasonCode {
			return false
		}
		previousID = event.id
		previousOccurredAt = event.occurredAt
	}
	return true
}

func TestReviewGrantCleanupConfirmationRequiresExactAuditLifecycle(t *testing.T) {
	grant := reviewGrantAuditEvent{grantID: "g", reviewerID: "r", capability: "review_identity", scopeKind: "identity_subject", scopeID: "o", actorID: "o", action: "granted", reasonCode: "temporary_coverage"}
	revoke := reviewGrantAuditEvent{grantID: "g", reviewerID: "r", capability: "review_identity", scopeKind: "identity_subject", scopeID: "o", actorID: "r", action: "revoked", reasonCode: "temporary_coverage_ended"}
	actualGrant, actualRevoke := grant, revoke
	actualGrant.id, actualGrant.occurredAt = 1, time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	actualRevoke.id, actualRevoke.occurredAt = 2, time.Date(2026, 11, 3, 12, 1, 0, 0, time.UTC)
	exactLifecycle := []reviewGrantAuditEvent{actualGrant, actualRevoke}
	tests := []struct {
		name                    string
		setupCommitAcknowledged bool
		exists, active, revoked bool
		events                  []reviewGrantAuditEvent
		want                    bool
	}{
		{name: "exact revoked grant", setupCommitAcknowledged: true, exists: true, revoked: true, events: exactLifecycle, want: true},
		{name: "ambiguous setup resolved by exact revoked row", exists: true, revoked: true, events: exactLifecycle, want: true},
		{name: "uncommitted setup with no row or events", want: true},
		{name: "acknowledged commit missing", setupCommitAcknowledged: true},
		{name: "active grant not yet revoked", setupCommitAcknowledged: true, exists: true, active: true, events: []reviewGrantAuditEvent{actualGrant}},
		{name: "missing revoke event", setupCommitAcknowledged: true, exists: true, revoked: true, events: []reviewGrantAuditEvent{actualGrant}},
		{name: "extra grant event", setupCommitAcknowledged: true, exists: true, revoked: true, events: []reviewGrantAuditEvent{actualGrant, actualGrant, actualRevoke}},
		{name: "malformed actor", setupCommitAcknowledged: true, exists: true, revoked: true, events: []reviewGrantAuditEvent{actualGrant, func() reviewGrantAuditEvent { e := actualRevoke; e.actorID = "other"; return e }()}},
		{name: "reordered lifecycle", setupCommitAcknowledged: true, exists: true, revoked: true, events: []reviewGrantAuditEvent{actualRevoke, actualGrant}},
		{name: "orphan event without row", events: []reviewGrantAuditEvent{actualGrant}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reviewGrantCleanupConfirmedWithEvents(tt.setupCommitAcknowledged, tt.exists, tt.active, tt.revoked, tt.events, "g", "r", "o"); got != tt.want {
				t.Fatalf("reviewGrantCleanupConfirmedWithEvents(...) = %t, want %t", got, tt.want)
			}
		})
	}
}

func TestReviewGrantRevocationRequiresExactActiveRowAndGrantEvent(t *testing.T) {
	tests := []struct {
		name                    string
		exists, active, auditOK bool
		want                    bool
	}{
		{name: "ambiguous commit exact active grant", exists: true, active: true, auditOK: true, want: true},
		{name: "missing grant", auditOK: true},
		{name: "mismatched grant audit", exists: true, active: true},
		{name: "inactive grant", exists: true, auditOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reviewGrantRevocationRequiredWithAudit(tt.exists, tt.active, tt.auditOK); got != tt.want {
				t.Fatalf("reviewGrantRevocationRequiredWithAudit(...) = %t, want %t", got, tt.want)
			}
		})
	}
}

func reviewGrantCleanupConfirmedWithEvents(setupCommitAcknowledged, exists, active, revoked bool, actual []reviewGrantAuditEvent, grantID, reviewerID, ownerID string) bool {
	if exists {
		return !active && revoked && reviewGrantAuditEventsMatch(actual, expectedReviewGrantAuditLifecycle(grantID, reviewerID, ownerID, true))
	}
	return !setupCommitAcknowledged && len(actual) == 0
}

func reviewGrantRevocationRequiredWithAudit(exists, active, grantAuditMatches bool) bool {
	return exists && active && grantAuditMatches
}

func TestReviewGrantRevocationRequiresExactlyOneAffectedRow(t *testing.T) {
	for _, rowsAffected := range []int64{0, 2} {
		if reviewGrantRevocationRowsMatch(rowsAffected) {
			t.Errorf("reviewGrantRevocationRowsMatch(%d) = true, want false", rowsAffected)
		}
	}
	if !reviewGrantRevocationRowsMatch(1) {
		t.Fatal("reviewGrantRevocationRowsMatch(1) = false, want true")
	}
}

func reviewGrantRevocationRowsMatch(rowsAffected int64) bool { return rowsAffected == 1 }

func expectedReviewGrantAuditLifecycle(grantID, reviewerID, ownerID string, revoked bool) []reviewGrantAuditEvent {
	events := []reviewGrantAuditEvent{{grantID: grantID, reviewerID: reviewerID, capability: "review_identity", scopeKind: "identity_subject", scopeID: ownerID, actorID: ownerID, action: "granted", reasonCode: "temporary_coverage"}}
	if revoked {
		events = append(events, reviewGrantAuditEvent{grantID: grantID, reviewerID: reviewerID, capability: "review_identity", scopeKind: "identity_subject", scopeID: ownerID, actorID: reviewerID, action: "revoked", reasonCode: "temporary_coverage_ended"})
	}
	return events
}

func TestReviewFixtureCleanupPlanRequiresAcknowledgedOwnership(t *testing.T) {
	tests := []struct {
		name                                          string
		grantCleanupConfirmed, caseCommitAcknowledged bool
		fileCommitAttempted, fileCommitAcknowledged   bool
		wantDeleteFileJob, wantDeleteCase             bool
	}{
		{name: "preflight alone authorizes nothing"},
		{name: "grant cleanup only", grantCleanupConfirmed: true},
		{name: "acknowledged case without file setup", grantCleanupConfirmed: true, caseCommitAcknowledged: true, wantDeleteCase: true},
		{name: "ambiguous start is preserved", grantCleanupConfirmed: true},
		{name: "ambiguous file commit preserves dependent rows", grantCleanupConfirmed: true, caseCommitAcknowledged: true, fileCommitAttempted: true},
		{name: "committed file and case", grantCleanupConfirmed: true, caseCommitAcknowledged: true, fileCommitAttempted: true, fileCommitAcknowledged: true, wantDeleteFileJob: true, wantDeleteCase: true},
		{name: "file ack without owned case", grantCleanupConfirmed: true, fileCommitAcknowledged: true},
		{name: "unconfirmed grant prevents fixture deletion", caseCommitAcknowledged: true, fileCommitAttempted: true, fileCommitAcknowledged: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteFileJob, deleteCase := reviewFixtureCleanupPlan(tt.grantCleanupConfirmed, tt.caseCommitAcknowledged, tt.fileCommitAttempted, tt.fileCommitAcknowledged)
			if deleteFileJob != tt.wantDeleteFileJob || deleteCase != tt.wantDeleteCase {
				t.Fatalf("reviewFixtureCleanupPlan(...) = (%t,%t), want (%t,%t)", deleteFileJob, deleteCase, tt.wantDeleteFileJob, tt.wantDeleteCase)
			}
		})
	}
}

func reviewFixtureCleanupPlan(grantCleanupConfirmed, caseCommitAcknowledged, fileCommitAttempted, fileCommitAcknowledged bool) (deleteFileJob, deleteCase bool) {
	if !grantCleanupConfirmed {
		return false, false
	}
	deleteFileJob = caseCommitAcknowledged && fileCommitAcknowledged
	deleteCase = caseCommitAcknowledged && (!fileCommitAttempted || fileCommitAcknowledged)
	return deleteFileJob, deleteCase
}

// TestPostgresCommittedReviewAndRevoke verifies service/repository transitions
// across independent commits. It requires explicitly supplied synthetic IDs and
// separate private setup/runtime passfiles; it does not create user accounts.
func TestPostgresCommittedReviewAndRevoke(t *testing.T) {
	dsn := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL")
	owner, reviewer := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_OWNER_ID"), os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_REVIEWER_ID")
	if dsn == "" || owner == "" || reviewer == "" {
		t.Skip("F5 database URL and registered/verified synthetic owner/reviewer UUID opt-ins are required")
	}
	for _, name := range []string{"PGPASSWORD", "PGSERVICE", "PGSERVICEFILE", "PGOPTIONS"} {
		if os.Getenv(name) != "" {
			t.Fatalf("refusing connection override from %s", name)
		}
	}
	ownerID, err := strictReviewUUID(owner)
	if err != nil {
		t.Fatal("owner opt-in must be a canonical nonzero UUID")
	}
	reviewerID, err := strictReviewUUID(reviewer)
	if err != nil || ownerID == reviewerID {
		t.Fatal("reviewer opt-in must be a distinct canonical nonzero UUID")
	}
	adminPassfile := os.Getenv("PGPASSFILE")
	if err := validateF5TestPassfile(adminPassfile); err != nil {
		t.Fatal("setup passfile must be an absolute regular 0600 file with immediate parent mode 0700")
	}
	runtimePassfile := os.Getenv("ESPACIGO_DOCUMENT_VERIFICATION_TEST_RUNTIME_PGPASSFILE")
	if err := validateF5TestPassfile(runtimePassfile); err != nil || filepath.Clean(runtimePassfile) == filepath.Clean(adminPassfile) {
		t.Fatal("runtime passfile must be distinct and an absolute regular 0600 file with immediate parent mode 0700")
	}
	adminPassfileInfo, err := os.Lstat(adminPassfile)
	if err != nil {
		t.Fatal("unable to verify administrative passfile identity")
	}
	runtimePassfileInfo, err := os.Lstat(runtimePassfile)
	if err != nil || os.SameFile(adminPassfileInfo, runtimePassfileInfo) {
		t.Fatal("administrative and runtime passfiles must be different files")
	}
	connectionURL, err := parseF5ReviewConnectionURL(dsn)
	if err != nil {
		t.Fatal("refusing a non-canonical F5 administrative connection URL")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	adminURL := *connectionURL
	adminQuery := adminURL.Query()
	adminQuery.Set("passfile", adminPassfile)
	adminURL.RawQuery = adminQuery.Encode()
	adminConfig, err := pgxpool.ParseConfig(adminURL.String())
	if err != nil {
		t.Fatal("invalid F5 integration database URL")
	}
	cc := adminConfig.ConnConfig
	if cc.Host != "127.0.0.1" || cc.Port != 15434 || cc.Database != reviewCommitDatabase || cc.User != "postgres" || len(cc.Fallbacks) != 0 || cc.Config.TLSConfig != nil {
		t.Fatal("refusing a database URL outside the exact loopback F5 endpoint")
	}
	admin, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatal("unable to create F5 setup pool")
	}
	t.Cleanup(admin.Close)
	var database, setupSession, setupCurrent string
	var server net.IP
	var setupBackendPID int32
	if err := admin.QueryRow(ctx, `SELECT current_database(),session_user,current_user,host(inet_server_addr())::inet,pg_backend_pid()`).Scan(&database, &setupSession, &setupCurrent, &server, &setupBackendPID); err != nil {
		t.Fatal("unable to verify F5 setup connection identity")
	}
	if database != reviewCommitDatabase || server == nil || server.String() != reviewCommitServer || setupSession != "postgres" || setupCurrent != "postgres" {
		t.Fatal("refusing unexpected F5 database, server, or setup identity")
	}
	adminReadConfig := adminConfig.Copy()
	adminReadConfig.MaxConns = 1
	adminReadPool, err := pgxpool.NewWithConfig(ctx, adminReadConfig)
	if err != nil {
		t.Fatal("unable to create independent setup-identity readback pool")
	}
	t.Cleanup(adminReadPool.Close)
	var adminReadDatabase, adminReadSession, adminReadCurrent string
	var adminReadServer net.IP
	var adminReadBackendPID int32
	if err := adminReadPool.QueryRow(ctx, `SELECT current_database(),session_user,current_user,host(inet_server_addr())::inet,pg_backend_pid()`).Scan(&adminReadDatabase, &adminReadSession, &adminReadCurrent, &adminReadServer, &adminReadBackendPID); err != nil || adminReadDatabase != reviewCommitDatabase || adminReadServer == nil || adminReadServer.String() != reviewCommitServer || adminReadSession != "postgres" || adminReadCurrent != "postgres" || adminReadBackendPID == setupBackendPID {
		t.Fatal("setup-identity readback must use a separate verified PostgreSQL backend")
	}

	runtimeURL, err := url.Parse(dsn)
	if err != nil {
		t.Fatal("invalid F5 integration database URL")
	}
	runtimeURL.User = url.User("espacigo_api_login")
	runtimeQuery := runtimeURL.Query()
	runtimeQuery.Set("passfile", runtimePassfile)
	runtimeURL.RawQuery = runtimeQuery.Encode()
	runtimeConfig, err := pgxpool.ParseConfig(runtimeURL.String())
	if err != nil {
		t.Fatal("unable to parse restricted runtime connection configuration")
	}
	runtimeCC := runtimeConfig.ConnConfig
	if runtimeCC.Host != "127.0.0.1" || runtimeCC.Port != 15434 || runtimeCC.Database != reviewCommitDatabase || runtimeCC.User != "espacigo_api_login" || len(runtimeCC.Fallbacks) != 0 || runtimeCC.Config.TLSConfig != nil {
		t.Fatal("refusing a restricted runtime URL outside the exact F5 target")
	}
	runtimeConfig.AfterConnect = func(connectCtx context.Context, conn *pgx.Conn) error {
		var sessionUser string
		if err := conn.QueryRow(connectCtx, `SELECT session_user`).Scan(&sessionUser); err != nil || sessionUser != "espacigo_api_login" {
			return fmt.Errorf("runtime login identity mismatch")
		}
		if _, err := conn.Exec(connectCtx, `SET ROLE espacigo_runtime`); err != nil {
			return err
		}
		var login, current string
		var loginCanLogin, loginSuper, loginCreateRole, loginCreateDB, loginReplication, loginBypass, loginInherit bool
		var roleCanLogin, roleSuper, roleCreateRole, roleCreateDB, roleReplication, roleBypass, roleInherit bool
		if err := conn.QueryRow(connectCtx, `SELECT session_user,current_user,
			(SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolcreaterole FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolcreatedb FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolreplication FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolbypassrls FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolinherit FROM pg_catalog.pg_roles WHERE rolname=session_user),
			(SELECT rolcanlogin FROM pg_catalog.pg_roles WHERE rolname=current_user),
			(SELECT rolsuper FROM pg_catalog.pg_roles WHERE rolname=current_user),
			(SELECT rolcreaterole FROM pg_catalog.pg_roles WHERE rolname=current_user),
			(SELECT rolcreatedb FROM pg_catalog.pg_roles WHERE rolname=current_user),
			(SELECT rolreplication FROM pg_catalog.pg_roles WHERE rolname=current_user),
			(SELECT rolbypassrls FROM pg_catalog.pg_roles WHERE rolname=current_user),
			(SELECT rolinherit FROM pg_catalog.pg_roles WHERE rolname=current_user)`).
			Scan(&login, &current, &loginCanLogin, &loginSuper, &loginCreateRole, &loginCreateDB, &loginReplication, &loginBypass, &loginInherit,
				&roleCanLogin, &roleSuper, &roleCreateRole, &roleCreateDB, &roleReplication, &roleBypass, &roleInherit); err != nil {
			return err
		}
		if login != "espacigo_api_login" || current != "espacigo_runtime" ||
			!loginCanLogin || loginSuper || loginCreateRole || loginCreateDB || loginReplication || loginBypass || loginInherit ||
			roleCanLogin || roleSuper || roleCreateRole || roleCreateDB || roleReplication || roleBypass || roleInherit {
			return fmt.Errorf("runtime database roles are not restricted")
		}
		return nil
	}
	runtimePool, err := pgxpool.NewWithConfig(ctx, runtimeConfig)
	if err != nil {
		t.Fatal("unable to create restricted F5 runtime pool")
	}
	t.Cleanup(runtimePool.Close)
	var current string
	if err := runtimePool.QueryRow(ctx, `SELECT current_user`).Scan(&current); err != nil || current != "espacigo_runtime" {
		t.Fatal("runtime pool did not establish SET ROLE espacigo_runtime")
	}
	readConfig := runtimeConfig.Copy()
	readConfig.MaxConns = 1
	readPool, err := pgxpool.NewWithConfig(ctx, readConfig)
	if err != nil {
		t.Fatal("unable to create separate restricted post-commit read pool")
	}
	t.Cleanup(readPool.Close)
	var readSession, readCurrent string
	if err := readPool.QueryRow(ctx, `SELECT session_user,current_user`).Scan(&readSession, &readCurrent); err != nil || readSession != "espacigo_api_login" || readCurrent != "espacigo_runtime" {
		t.Fatal("separate read pool did not establish the restricted runtime role")
	}

	caseID, fileID, grantID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	var grantGrantedAt, grantExpiresAt time.Time
	readGrant := func(readCtx context.Context) (exists, active, revoked bool, err error) {
		var actual reviewGrantIdentity
		var revokedAt *time.Time
		var revokedBy, revocationReason string
		err = adminReadPool.QueryRow(readCtx, `SELECT id::text,reviewer_id::text,capability,scope_kind,scope_id::text,granted_by::text,granted_reason_code,granted_at,expires_at,revoked_at,COALESCE(revoked_by::text,''),COALESCE(revocation_reason_code,'') FROM public.document_verification_reviewer_grant WHERE id=$1`, grantID).Scan(
			&actual.id, &actual.reviewerID, &actual.capability, &actual.scopeKind, &actual.scopeID, &actual.grantorID, &actual.reasonCode,
			&actual.grantedAt, &actual.expiresAt, &nullablePT{v: &revokedAt}, &revokedBy, &revocationReason)
		if err == pgx.ErrNoRows {
			return false, false, false, nil
		}
		if err != nil {
			return false, false, false, err
		}
		expected := reviewGrantIdentity{
			id: grantID, reviewerID: reviewerID, capability: "review_identity", scopeKind: "identity_subject",
			scopeID: ownerID, grantorID: ownerID, reasonCode: "temporary_coverage",
			grantedAt: grantGrantedAt, expiresAt: grantExpiresAt,
		}
		exact := reviewGrantIdentityMatches(actual, expected)
		exists = true
		active = exact && revokedAt == nil && revokedBy == "" && revocationReason == ""
		revoked = exact && revokedAt != nil && revokedBy == reviewerID && revocationReason == "temporary_coverage_ended"
		return
	}
	readGrantAuditEvents := func(readCtx context.Context) ([]reviewGrantAuditEvent, error) {
		rows, err := adminReadPool.Query(readCtx, `SELECT id,grant_id::text,reviewer_id::text,capability,scope_kind,scope_id::text,actor_id::text,action,reason_code,occurred_at FROM public.document_verification_reviewer_grant_event WHERE grant_id=$1 ORDER BY id`, grantID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		events := make([]reviewGrantAuditEvent, 0, 2)
		for rows.Next() {
			var event reviewGrantAuditEvent
			if err := rows.Scan(&event.id, &event.grantID, &event.reviewerID, &event.capability, &event.scopeKind, &event.scopeID, &event.actorID, &event.action, &event.reasonCode, &event.occurredAt); err != nil {
				return nil, err
			}
			events = append(events, event)
		}
		return events, rows.Err()
	}
	fixtureIDsPreflighted := false
	grantSetupCommitAcknowledged := false
	caseSetupCommitAcknowledged := false
	fileFixtureCommitAttempted := false
	fileFixtureCommitAcknowledged := false
	// Register cleanup before any fixture write. Cleanup revokes only the exact
	// grant confirmed by independent row and append-only audit readback.
	t.Cleanup(func() {
		if !fixtureIDsPreflighted {
			return
		}
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cleanupCancel()
		revokeSQL := `UPDATE public.document_verification_reviewer_grant SET revoked_at=clock_timestamp(),revoked_by=$2,revocation_reason_code='temporary_coverage_ended' WHERE id=$1 AND reviewer_id=$2 AND capability='review_identity' AND scope_kind='identity_subject' AND scope_id=$3 AND granted_by=$3 AND granted_reason_code='temporary_coverage' AND granted_at=$4 AND expires_at=$5 AND revoked_at IS NULL`
		exists, active, revoked, readErr := readGrant(cleanupCtx)
		grantAuditEvents, grantAuditErr := readGrantAuditEvents(cleanupCtx)
		grantEventSequenceMatches := grantAuditErr == nil && reviewGrantAuditEventsMatch(grantAuditEvents, expectedReviewGrantAuditLifecycle(grantID, reviewerID, ownerID, false))
		cleanupRevokeGrant := readErr == nil && reviewGrantRevocationRequiredWithAudit(exists, active, grantEventSequenceMatches)
		var revokeErr error
		var revokeRowsMismatch bool
		if cleanupRevokeGrant {
			tag, err := admin.Exec(cleanupCtx, revokeSQL, grantID, reviewerID, ownerID, grantGrantedAt, grantExpiresAt)
			revokeErr = err
			if revokeErr == nil && !reviewGrantRevocationRowsMatch(tag.RowsAffected()) {
				revokeRowsMismatch = true
				revokeErr = fmt.Errorf("grant cleanup affected %d rows, want exactly one", tag.RowsAffected())
			}
			// Resolve both successful and ambiguous mutations through a separate
			// connection before any dependent fixture cleanup is authorized.
			exists, active, revoked, readErr = readGrant(cleanupCtx)
			grantAuditEvents, grantAuditErr = readGrantAuditEvents(cleanupCtx)
		}
		if readErr != nil {
			t.Errorf("independently read exact grant state after cleanup: %v", readErr)
		} else if grantAuditErr != nil {
			t.Errorf("independently read exact reviewer-grant audit after cleanup: %v", grantAuditErr)
		} else if active && cleanupRevokeGrant && reviewGrantAuditEventsMatch(grantAuditEvents, expectedReviewGrantAuditLifecycle(grantID, reviewerID, ownerID, false)) {
			// Retry only after an independent read proves this exact fixture grant
			// and its grant event remain active; never replay without readback.
			retryTag, retryErr := admin.Exec(cleanupCtx, revokeSQL, grantID, reviewerID, ownerID, grantGrantedAt, grantExpiresAt)
			if retryErr == nil && !reviewGrantRevocationRowsMatch(retryTag.RowsAffected()) {
				revokeRowsMismatch = true
				retryErr = fmt.Errorf("grant cleanup retry affected %d rows, want exactly one", retryTag.RowsAffected())
			}
			if retryErr != nil {
				revokeErr = retryErr
			} else {
				revokeErr = nil
			}
			exists, active, revoked, readErr = readGrant(cleanupCtx)
			grantAuditEvents, grantAuditErr = readGrantAuditEvents(cleanupCtx)
			if readErr != nil {
				t.Errorf("independently read exact grant state after cleanup retry: %v", readErr)
			}
			if grantAuditErr != nil {
				t.Errorf("independently read exact reviewer-grant audit after cleanup retry: %v", grantAuditErr)
			}
		}
		if readErr == nil && revoked && !revokeRowsMismatch {
			// A lost mutation acknowledgment is resolved by the independent exact
			// row readback; a zero/multiple-row result is never silently accepted.
			revokeErr = nil
		}
		if revokeErr != nil {
			t.Errorf("revoke exact active reviewer grant: %v", revokeErr)
		}
		grantAuditRows := len(grantAuditEvents)
		grantCleanupConfirmed := readErr == nil && grantAuditErr == nil && reviewGrantCleanupConfirmedWithEvents(grantSetupCommitAcknowledged, exists, active, revoked, grantAuditEvents, grantID, reviewerID, ownerID)
		cleanupFileJob, cleanupCase := reviewFixtureCleanupPlan(grantCleanupConfirmed, caseSetupCommitAcknowledged, fileFixtureCommitAttempted, fileFixtureCommitAcknowledged)
		if !grantCleanupConfirmed {
			t.Errorf("preserving mutable fixtures because reviewer-grant cleanup is unconfirmed (setup_ack=%t exists=%t active=%t revoked=%t audit_rows=%d revoke_attempted=%t revoke_err=%t)", grantSetupCommitAcknowledged, exists, active, revoked, grantAuditRows, cleanupRevokeGrant, revokeErr != nil)
		} else {
			if cleanupFileJob {
				for _, item := range []struct {
					label, sql string
					args       []any
				}{
					{"job", `DELETE FROM public.document_verification_job WHERE file_id=$1 AND kind='scan'`, []any{fileID}},
					{"file", `DELETE FROM public.document_verification_file WHERE id=$1 AND case_id=$2`, []any{fileID, caseID}},
				} {
					if _, err := admin.Exec(cleanupCtx, item.sql, item.args...); err != nil {
						t.Errorf("delete acknowledged test %s fixture: %v", item.label, err)
					}
				}
			}
			if cleanupCase {
				if _, err := admin.Exec(cleanupCtx, `DELETE FROM public.document_verification_case WHERE id=$1 AND purpose='identity' AND subject_id=$2`, caseID, ownerID); err != nil {
					t.Errorf("delete acknowledged exact identity case fixture: %v", err)
				}
			}
		}
		// Independent exact-ID readback always runs, even if transaction setup,
		// revocation, deletion, or commit failed. Audit rows are append-only.
		var activeGrants, grantRows, revokedGrants, cases, files, jobs, caseAudits int
		var createdAudits, fileAddedAudits, submittedAudits, approvedAudits, revokedCaseAudits int
		var grantAudits, validGrantAudits, grantedAudits, revokedAudits int
		readErr = adminReadPool.QueryRow(cleanupCtx, `SELECT
			(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE id=$1 AND revoked_at IS NULL),
			(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE id=$1),
			(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE id=$1 AND reviewer_id=$4 AND capability='review_identity' AND scope_kind='identity_subject' AND scope_id=$5 AND granted_by=$5 AND granted_reason_code='temporary_coverage' AND revoked_at IS NOT NULL AND revoked_by=$4 AND revocation_reason_code='temporary_coverage_ended'),
			(SELECT count(*) FROM public.document_verification_case WHERE id=$2),
			(SELECT count(*) FROM public.document_verification_file WHERE id=$3),
			(SELECT count(*) FROM public.document_verification_job WHERE file_id=$3),
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 OR file_id=$3),
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND purpose='identity' AND subject_id=$5 AND actor_kind='user' AND action='case_created' AND actor_id=$5 AND file_id IS NULL AND from_state IS NULL AND to_state='borrador' AND result_code='borrador' AND reason_code IS NULL AND revision=1),
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND purpose='identity' AND subject_id=$5 AND actor_kind='user' AND action='file_added' AND actor_id=$5 AND file_id=$3 AND from_state='borrador' AND to_state='borrador' AND result_code='pendiente' AND reason_code IS NULL AND revision=2),
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND purpose='identity' AND subject_id=$5 AND actor_kind='user' AND action='case_transitioned' AND actor_id=$5 AND file_id IS NULL AND from_state='borrador' AND to_state='enviada' AND result_code='enviada' AND reason_code IS NULL AND revision=3),
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND purpose='identity' AND subject_id=$5 AND actor_kind='user' AND action='review_decision' AND actor_id=$4 AND file_id IS NULL AND from_state='enviada' AND to_state='aprobada' AND result_code='aprobada' AND reason_code IS NULL AND revision=4),
			(SELECT count(*) FROM public.document_verification_event WHERE case_id=$2 AND purpose='identity' AND subject_id=$5 AND actor_kind='user' AND action='revoked' AND actor_id=$4 AND file_id IS NULL AND from_state='aprobada' AND to_state='revocada' AND result_code='revocada' AND reason_code='solicitud_titular' AND revision=5)`, grantID, caseID, fileID, reviewerID, ownerID).Scan(&activeGrants, &grantRows, &revokedGrants, &cases, &files, &jobs, &caseAudits, &createdAudits, &fileAddedAudits, &submittedAudits, &approvedAudits, &revokedCaseAudits)
		if readErr != nil {
			t.Errorf("verify retained review fixture state by exact IDs: %v", readErr)
		}
		readStateVerified := readErr == nil
		var caseAuditSequence []string
		sequenceErr := adminReadPool.QueryRow(cleanupCtx, `SELECT COALESCE(array_agg(action || ':' || revision::text ORDER BY revision,id),ARRAY[]::text[]) FROM public.document_verification_event WHERE case_id=$1 OR file_id=$2`, caseID, fileID).Scan(&caseAuditSequence)
		if sequenceErr != nil {
			t.Errorf("verify ordered retained case audit prefix: %v", sequenceErr)
		}
		auditReadVerified := false
		if err := adminReadPool.QueryRow(cleanupCtx, `SELECT count(*),
			count(*) FILTER(WHERE reviewer_id=$2 AND capability='review_identity' AND scope_kind='identity_subject' AND scope_id=$3 AND
				((action='granted' AND actor_id=$3 AND reason_code='temporary_coverage') OR
				 (action='revoked' AND actor_id=$2 AND reason_code='temporary_coverage_ended'))),
			count(*) FILTER(WHERE reviewer_id=$2 AND capability='review_identity' AND scope_kind='identity_subject' AND scope_id=$3 AND action='granted' AND actor_id=$3 AND reason_code='temporary_coverage'),
			count(*) FILTER(WHERE reviewer_id=$2 AND capability='review_identity' AND scope_kind='identity_subject' AND scope_id=$3 AND action='revoked' AND actor_id=$2 AND reason_code='temporary_coverage_ended')
			FROM public.document_verification_reviewer_grant_event WHERE grant_id=$1`, grantID, reviewerID, ownerID).Scan(&grantAudits, &validGrantAudits, &grantedAudits, &revokedAudits); err != nil {
			t.Errorf("verify append-only reviewer-grant audit through setup identity: %v", err)
		} else {
			auditReadVerified = true
		}
		// Audit rows are the commit record: service error flags can be stale when
		// a commit acknowledgment is ambiguous. Validate each retained row and
		// its exact ordered lifecycle prefix.
		expectedPrefix := [5]int{}
		if caseAudits >= 0 && caseAudits <= len(expectedPrefix) {
			for i := 0; i < caseAudits; i++ {
				expectedPrefix[i] = 1
			}
		}
		caseAuditPrefixValid := caseAudits >= 0 && caseAudits <= len(expectedPrefix) &&
			createdAudits == expectedPrefix[0] && fileAddedAudits == expectedPrefix[1] &&
			submittedAudits == expectedPrefix[2] && approvedAudits == expectedPrefix[3] &&
			revokedCaseAudits == expectedPrefix[4] && sequenceErr == nil &&
			reviewCaseAuditPrefixMatches(caseAuditSequence, caseAudits)
		// Grant setup commit may have succeeded even if its acknowledgment was lost.
		// Determine whether the exact generated grant exists from readback, not a flag.
		expectedGrantRows := expectedReviewGrantCleanupRows(grantSetupCommitAcknowledged, grantRows, grantedAudits)
		expectedGrantAudits := expectedGrantRows * 2
		expectedGrantedAudits, expectedRevokedAudits := expectedGrantRows, expectedGrantRows
		grantEventsAfterCleanup, grantEventsReadErr := readGrantAuditEvents(cleanupCtx)
		if grantEventsReadErr != nil {
			t.Errorf("verify exact ordered grant audit lifecycle after cleanup: %v", grantEventsReadErr)
		}
		var expectedGrantLifecycle []reviewGrantAuditEvent
		if expectedGrantRows == 1 {
			expectedGrantLifecycle = expectedReviewGrantAuditLifecycle(grantID, reviewerID, ownerID, true)
		}
		grantLifecycleValid := grantEventsReadErr == nil && reviewGrantAuditEventsMatch(grantEventsAfterCleanup, expectedGrantLifecycle)
		if !readStateVerified || !auditReadVerified || !caseAuditPrefixValid || !grantLifecycleValid || activeGrants != 0 || grantRows != expectedGrantRows || revokedGrants != expectedGrantRows || cases != 0 || files != 0 || jobs != 0 || grantAudits != expectedGrantAudits || validGrantAudits != grantAudits || grantedAudits != expectedGrantedAudits || revokedAudits != expectedRevokedAudits {
			t.Errorf("cleanup verification: active grants=%d rows=%d revoked=%d cases=%d files=%d jobs=%d case audit prefix valid=%t count=%d grant audits=%d/%d granted=%d revoked=%d", activeGrants, grantRows, revokedGrants, cases, files, jobs, caseAuditPrefixValid, caseAudits, validGrantAudits, grantAudits, grantedAudits, revokedAudits)
		}
	})

	setup, err := admin.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("begin guarded review fixture setup")
	}
	defer setup.Rollback(context.Background())
	var existing int
	if err := setup.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM public.document_verification_case WHERE id=$1)+
		(SELECT count(*) FROM public.document_verification_reviewer_grant WHERE id=$2)+
		(SELECT count(*) FROM public.document_verification_file WHERE id=$3)+
		(SELECT count(*) FROM public.document_verification_job WHERE file_id=$3)+
		(SELECT count(*) FROM public.document_verification_event WHERE case_id=$1 OR file_id=$3)+
		(SELECT count(*) FROM public.document_verification_reviewer_grant_event WHERE grant_id=$2)`, caseID, grantID, fileID).Scan(&existing); err != nil || existing != 0 {
		t.Fatal("refusing colliding exact review/revoke fixture IDs")
	}
	fixtureIDsPreflighted = true
	if err := setup.QueryRow(ctx, `INSERT INTO public.document_verification_reviewer_grant(id,reviewer_id,capability,scope_kind,scope_id,granted_by,granted_reason_code,expires_at) VALUES($1,$2,'review_identity','identity_subject',$3,$3,'temporary_coverage',clock_timestamp()+interval '1 hour') RETURNING granted_at,expires_at`, grantID, reviewerID, ownerID).Scan(&grantGrantedAt, &grantExpiresAt); err != nil {
		t.Fatal("seed guarded identity-subject reviewer grant")
	}
	if err := setup.Commit(ctx); err != nil {
		t.Fatal("commit synthetic reviewer-grant fixture")
	}
	grantSetupCommitAcknowledged = true
	grantExists, grantActive, grantRevoked, grantReadErr := readGrant(ctx)
	grantAuditEvents, grantAuditReadErr := readGrantAuditEvents(ctx)
	if grantReadErr != nil || grantAuditReadErr != nil || !grantExists || !grantActive || grantRevoked ||
		!reviewGrantAuditEventsMatch(grantAuditEvents, expectedReviewGrantAuditLifecycle(grantID, reviewerID, ownerID, false)) {
		t.Fatal("independent readback did not confirm the exact committed active reviewer grant and grant event")
	}

	repo := New(runtimePool)
	readRepo := New(readPool)
	service, err := dv.NewService(fixedCaseIDRepository{Repository: repo, caseID: caseID})
	if err != nil {
		t.Fatal("construct document verification service")
	}
	created, err := service.Start(ctx, ownerID, dv.CreateInput{Purpose: dv.PurposeIdentity, DocumentExpiresAt: time.Now().UTC().AddDate(1, 0, 0)})
	if err != nil {
		t.Fatalf("create synthetic identity case through service: %v", err)
	}
	if created.ID != caseID {
		t.Fatalf("service created unexpected case ID %q", created.ID)
	}
	caseSetupCommitAcknowledged = true
	file := dv.File{ID: fileID, State: dv.FilePending, Size: 128, MIME: "image/png", Pages: 0, ContentSHA256: [32]byte{0x53, 0x79, 0x6e, 0x74, 0x68, 0x65, 0x74, 0x69, 0x63, 0x20, 0x63, 0x6c, 0x65, 0x61, 0x6e, 0x20, 0x66, 0x69, 0x6c, 0x65, 0x20, 0x66, 0x69, 0x78, 0x74, 0x75, 0x72, 0x65, 0x20, 0x46, 0x35}}
	// Keep file insertion and synthetic inspection finalization atomic. AddFile
	// acquires the repository's normal actor-then-case locks; the outer
	// transaction keeps its pending job invisible until it is completed below.
	outer, err := runtimePool.Begin(ctx)
	if err != nil {
		t.Fatal("begin atomic synthetic file fixture")
	}
	defer outer.Rollback(context.Background())
	fileService, err := dv.NewService(newWithTransactionBeginner(savepointBeginner{tx: outer}))
	if err != nil {
		t.Fatal("construct savepoint-backed file service")
	}
	if err := fileService.AddFile(ctx, ownerID, created.ID, dv.PurposeIdentity, created.Revision, file); err != nil {
		t.Fatalf("add synthetic file metadata through service/repository: %v", err)
	}
	// This exact synthetic clean-inspection fixture avoids claiming a real scan.
	tag, err := outer.Exec(ctx, `UPDATE public.document_verification_file SET state='limpio',inspected_at=clock_timestamp(),scanner_version='synthetic-review-fixture',signatures_at=clock_timestamp() WHERE id=$1 AND case_id=$2 AND state='pendiente'`, fileID, created.ID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("mark exact synthetic metadata fixture clean: rows=%d err=%v", tag.RowsAffected(), err)
	}
	tag, err = outer.Exec(ctx, `UPDATE public.document_verification_job SET state='completado',lease_until=NULL,last_error_code=NULL,updated_at=clock_timestamp() WHERE file_id=$1 AND kind='scan' AND state='pendiente'`, fileID)
	if err != nil || tag.RowsAffected() != 1 {
		t.Fatalf("complete exact synthetic scan job: rows=%d err=%v", tag.RowsAffected(), err)
	}
	fileFixtureCommitAttempted = true
	if err := outer.Commit(ctx); err != nil {
		t.Fatalf("commit atomic synthetic file fixture: %v", err)
	}
	fileFixtureCommitAcknowledged = true
	if err := service.Submit(ctx, ownerID, created.ID, dv.PurposeIdentity, created.Revision+1); err != nil {
		t.Fatalf("submit identity case in committed runtime transaction: %v", err)
	}
	submitted, err := readRepo.LoadOwnCase(ctx, ownerID, created.ID)
	if err != nil || submitted.State != dv.CaseSubmitted || len(submitted.Files) != 1 || submitted.Files[0].State != dv.FileClean {
		t.Fatalf("independent committed submit readback mismatch: state=%q files=%d err=%v", submitted.State, len(submitted.Files), err)
	}
	assertIndependentCaseLifecycle := func(label, expectedState string, expectedRevision uint64, expectedEvents int) {
		t.Helper()
		var state string
		var revision uint64
		if err := adminReadPool.QueryRow(ctx, `SELECT state,revision FROM public.document_verification_case WHERE id=$1`, created.ID).Scan(&state, &revision); err != nil {
			t.Fatalf("%s independent case-state readback: %v", label, err)
		}
		var events []string
		if err := adminReadPool.QueryRow(ctx, `SELECT COALESCE(array_agg(action || ':' || revision::text ORDER BY revision,id),ARRAY[]::text[]) FROM public.document_verification_event WHERE case_id=$1 OR file_id=$2`, created.ID, fileID).Scan(&events); err != nil {
			t.Fatalf("%s independent audit readback: %v", label, err)
		}
		if state != expectedState || revision != expectedRevision || !reviewCaseAuditPrefixMatches(events, expectedEvents) {
			t.Fatalf("%s readback state=%q revision=%d events=%v; want state=%q revision=%d event-prefix-length=%d", label, state, revision, events, expectedState, expectedRevision, expectedEvents)
		}
	}
	assertIndependentCaseLifecycle("submitted baseline before rollback-only review", string(submitted.State), submitted.Revision, 3)
	reviewRollbackTx, err := runtimePool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("begin rollback-only review transaction")
	}
	defer func() { _ = reviewRollbackTx.Rollback(context.Background()) }()
	reviewRollbackService, err := dv.NewService(newWithTransactionBeginner(savepointBeginner{tx: reviewRollbackTx}))
	if err != nil {
		t.Fatal("construct savepoint-backed review service")
	}
	if err := reviewRollbackService.Review(ctx, reviewerID, created.ID, dv.CapabilityIdentity, dv.DecisionApprove, time.Time{}, submitted.Revision); err != nil {
		t.Fatalf("perform review within rollback-only outer transaction: %v", err)
	}
	assertIndependentCaseLifecycle("review before outer rollback", string(submitted.State), submitted.Revision, 3)
	if err := reviewRollbackTx.Rollback(ctx); err != nil {
		t.Fatalf("rollback review outer transaction: %v", err)
	}
	assertIndependentCaseLifecycle("review after outer rollback", string(submitted.State), submitted.Revision, 3)
	if err := service.Review(ctx, reviewerID, created.ID, dv.CapabilityIdentity, dv.DecisionApprove, time.Time{}, submitted.Revision); err != nil {
		t.Fatalf("approve identity case in committed runtime transaction: %v", err)
	}
	approved, err := readRepo.LoadOwnCase(ctx, ownerID, created.ID)
	if err != nil || approved.State != dv.CaseApproved || approved.VerifiedAt.IsZero() || approved.VerifiedUntil.IsZero() {
		t.Fatalf("independent committed approved-state readback mismatch: state=%q err=%v", approved.State, err)
	}
	assertIndependentCaseLifecycle("approved baseline before rollback-only revoke", string(approved.State), approved.Revision, 4)
	revokeRollbackTx, err := runtimePool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		t.Fatal("begin rollback-only revoke transaction")
	}
	defer func() { _ = revokeRollbackTx.Rollback(context.Background()) }()
	revokeRollbackService, err := dv.NewService(newWithTransactionBeginner(savepointBeginner{tx: revokeRollbackTx}))
	if err != nil {
		t.Fatal("construct savepoint-backed revoke service")
	}
	if err := revokeRollbackService.Revoke(ctx, reviewerID, created.ID, dv.PurposeIdentity, dv.ReasonOwnerRequest, approved.Revision); err != nil {
		t.Fatalf("perform revoke within rollback-only outer transaction: %v", err)
	}
	assertIndependentCaseLifecycle("revoke before outer rollback", string(approved.State), approved.Revision, 4)
	if err := revokeRollbackTx.Rollback(ctx); err != nil {
		t.Fatalf("rollback revoke outer transaction: %v", err)
	}
	assertIndependentCaseLifecycle("revoke after outer rollback", string(approved.State), approved.Revision, 4)
	if err := service.Revoke(ctx, reviewerID, created.ID, dv.PurposeIdentity, dv.ReasonOwnerRequest, approved.Revision); err != nil {
		t.Fatalf("revoke identity case in committed runtime transaction: %v", err)
	}
	revoked, err := readRepo.LoadOwnCase(ctx, ownerID, created.ID)
	if err != nil || revoked.State != dv.CaseRevoked || revoked.RevokedAt == nil {
		t.Fatalf("independent committed revoked-state readback mismatch: state=%q revoked_at=%t err=%v", revoked.State, revoked.RevokedAt != nil, err)
	}
	var totalCaseEvents, createdEvents, fileAddedEvents, submittedEvents, approvedEvents, revokedEvents int
	if err := adminReadPool.QueryRow(ctx, `SELECT count(*),
		count(*) FILTER(WHERE case_id=$1 AND actor_kind='user' AND action='case_created' AND actor_id=$2 AND file_id IS NULL AND from_state IS NULL AND to_state='borrador' AND result_code='borrador' AND reason_code IS NULL AND revision=1),
		count(*) FILTER(WHERE case_id=$1 AND actor_kind='user' AND action='file_added' AND actor_id=$2 AND file_id=$4 AND from_state='borrador' AND to_state='borrador' AND result_code='pendiente' AND reason_code IS NULL AND revision=2),
		count(*) FILTER(WHERE case_id=$1 AND actor_kind='user' AND action='case_transitioned' AND actor_id=$2 AND file_id IS NULL AND from_state='borrador' AND to_state='enviada' AND result_code='enviada' AND reason_code IS NULL AND revision=3),
		count(*) FILTER(WHERE case_id=$1 AND actor_kind='user' AND action='review_decision' AND actor_id=$3 AND file_id IS NULL AND from_state='enviada' AND to_state='aprobada' AND result_code='aprobada' AND reason_code IS NULL AND revision=4),
		count(*) FILTER(WHERE case_id=$1 AND actor_kind='user' AND action='revoked' AND actor_id=$3 AND file_id IS NULL AND from_state='aprobada' AND to_state='revocada' AND result_code='revocada' AND reason_code='solicitud_titular' AND revision=5)
		FROM public.document_verification_event WHERE (case_id=$1 AND purpose='identity' AND subject_id=$2) OR file_id=$4`, created.ID, ownerID, reviewerID, fileID).Scan(&totalCaseEvents, &createdEvents, &fileAddedEvents, &submittedEvents, &approvedEvents, &revokedEvents); err != nil {
		t.Fatal("read committed case audit events independently")
	}
	if totalCaseEvents != 5 || createdEvents != 1 || fileAddedEvents != 1 || submittedEvents != 1 || approvedEvents != 1 || revokedEvents != 1 {
		t.Fatalf("committed case audit counts total/create/file/submit/review/revoke=%d/%d/%d/%d/%d/%d", totalCaseEvents, createdEvents, fileAddedEvents, submittedEvents, approvedEvents, revokedEvents)
	}
	var caseAuditSequence []string
	if err := adminReadPool.QueryRow(ctx, `SELECT COALESCE(array_agg(action || ':' || revision::text ORDER BY revision,id),ARRAY[]::text[]) FROM public.document_verification_event WHERE case_id=$1 OR file_id=$2`, created.ID, fileID).Scan(&caseAuditSequence); err != nil || !reviewCaseAuditPrefixMatches(caseAuditSequence, totalCaseEvents) {
		t.Fatalf("committed case audit events are not the exact ordered lifecycle: events=%v err=%v", caseAuditSequence, err)
	}
	var grantEvents, validGrantEvents int
	if err := adminReadPool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE reviewer_id=$2 AND capability='review_identity' AND scope_kind='identity_subject' AND scope_id=$3 AND actor_id=$4 AND action='granted' AND reason_code='temporary_coverage') FROM public.document_verification_reviewer_grant_event WHERE grant_id=$1`, grantID, reviewerID, ownerID, ownerID).Scan(&grantEvents, &validGrantEvents); err != nil || grantEvents != 1 || validGrantEvents != 1 {
		t.Fatalf("committed reviewer grant history count=%d valid=%d err=%v", grantEvents, validGrantEvents, err)
	}
}

type fixedCaseIDRepository struct {
	dv.Repository
	caseID string
}

func (r fixedCaseIDRepository) CreateCase(ctx context.Context, actor string, purpose dv.Purpose, space string, build func(dv.Authorization) (dv.Snapshot, error)) (dv.Snapshot, error) {
	return r.Repository.CreateCase(ctx, actor, purpose, space, func(auth dv.Authorization) (dv.Snapshot, error) {
		snapshot, err := build(auth)
		if err == nil {
			snapshot.ID = r.caseID
		}
		return snapshot, err
	})
}

func strictReviewUUID(value string) (string, error) {
	parsed, err := uuid.Parse(value)
	if err != nil || parsed == uuid.Nil || parsed.String() != value {
		return "", fmt.Errorf("invalid UUID")
	}
	return parsed.String(), nil
}

func parseF5ReviewConnectionURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u == nil || u.String() != raw || u.Opaque != "" || (u.Scheme != "postgres" && u.Scheme != "postgresql") ||
		u.Host != "127.0.0.1:15434" || u.Path != "/"+reviewCommitDatabase || u.RawPath != "" ||
		u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || u.User == nil ||
		u.User.Username() != "postgres" || u.User.String() != "postgres" ||
		(u.RawQuery != "" && u.RawQuery != "sslmode=disable") {
		return nil, fmt.Errorf("non-canonical F5 connection URL")
	}
	return u, nil
}
