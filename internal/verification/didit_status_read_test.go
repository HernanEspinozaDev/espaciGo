package verification

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const inspectionAccount = "11111111-1111-4111-8111-111111111111"
const inspectionAttempt = "22222222-2222-4222-8222-222222222222"

func statusReadFixture(t *testing.T) (*DiditService, *memProviderAttemptRepo, *fakeDiditClient, *declarationMemoryRepository) {
	t.Helper()
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	keys := newTestKeyring(t)
	declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	declarations, err := NewDeclarationService(declRepo, keys, &counterUUIDs{}, clock, func(at time.Time) time.Time { return at.Add(30 * 24 * time.Hour) }, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := declarations.Save(context.Background(), inspectionAccount, DeclarationPayload{FirstName: "Ana María", LastName: "Pérez", DateOfBirth: "1990-01-02", RUT: "12.345.670-K", IDCountry: "CHL", DocumentType: "ID"}, true, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	repo := newMemProviderAttemptRepo()
	repo.declarations = declRepo
	repo.attempts[inspectionAttempt] = ProviderAttempt{ID: inspectionAttempt, AccountID: inspectionAccount, DeclarationID: saved.ID, Provider: DiditProviderName, Environment: "SANDBOX", ApplicationID: "inspection-app", WorkflowID: "inspection-workflow", WorkflowVersion: "1", CorrelationID: "33333333-3333-4333-8333-333333333333", ProviderSessionID: "inspection-session", Status: "IN_REVIEW", CreatedAt: now, UpdatedAt: now, RetentionAt: now.Add(24 * time.Hour)}
	client := &fakeDiditClient{decision: DiditDecisionResponse{SessionID: "inspection-session", WorkflowID: "inspection-workflow", VendorData: "33333333-3333-4333-8333-333333333333", Status: "Approved", IDCountry: "CHL", DocumentType: "ID", FullName: "Ana María Pérez", DateOfBirth: "1990-01-02", IdentificationNumber: "12345670-K", Liveness: true, FaceMatch: true}}
	svc, err := NewDiditService(repo, declarations, client, keys, &counterUUIDs{}, clock, DiditServiceConfig{Environment: "SANDBOX", ApplicationID: "inspection-app", WorkflowID: "inspection-workflow", WorkflowVersion: "1", ConsentVersion: "2026-11-v1", RequireLiveForVerifiedBadge: true, RutHMACSecret: []byte(strings.Repeat("s", 32))}, NewMemoryRutClaimRegistry())
	if err != nil {
		t.Fatal(err)
	}
	return svc, repo, client, declRepo
}

func TestInspectionFreshDecisionDoesNotWriteOrPromote(t *testing.T) {
	svc, repo, client, _ := statusReadFixture(t)
	client.decision.FullName = "Another Synthetic Identity"
	before := repo.attempts[inspectionAttempt]
	result, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if result.ProviderStatus != "APPROVED" || result.EvaluationStatus != "IN_REVIEW" || result.Status != "IN_REVIEW" || result.BadgeResult == "VERIFIED" {
		t.Fatalf("incorrect status separation: %+v", result)
	}
	if !reflect.DeepEqual(before, repo.attempts[inspectionAttempt]) || len(repo.events) != 0 || len(repo.outbox) != 0 || len(client.created) != 0 || len(client.deleted) != 0 {
		t.Fatal("inspection wrote state or changed provider session")
	}
}
func TestInspectionSupersededStillShowsProviderOutcomeWithoutApproval(t *testing.T) {
	svc, _, client, _ := statusReadFixture(t)
	_, err := svc.declarations.Save(context.Background(), inspectionAccount, DeclarationPayload{FirstName: "Ana Actualizada", LastName: "Pérez", DateOfBirth: "1990-01-02", RUT: "12.345.670-K", IDCountry: "CHL", DocumentType: "ID"}, true, "2026-11-v1")
	if err != nil {
		t.Fatal(err)
	}
	got, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderStatus != "APPROVED" || got.EvaluationStatus != "IN_REVIEW" || got.BadgeResult != "NOT_VERIFIED" || client.decisionGets != 1 {
		t.Fatalf("superseded read hid provider outcome or granted approval: %+v", got)
	}
}

func TestInspectionMatchingSandboxNeverPromotesBadge(t *testing.T) {
	svc, _, _, _ := statusReadFixture(t)
	got, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProviderStatus != "APPROVED" || got.EvaluationStatus != "APPROVED" || got.BadgeResult == "VERIFIED" {
		t.Fatalf("sandbox inspection: %+v", got)
	}
}
func TestInspectionTerminalDoesNotFetchOrResurrect(t *testing.T) {
	for _, state := range []string{"EXPIRED", "FAILED", "REJECTED"} {
		t.Run(state, func(t *testing.T) {
			svc, repo, client, _ := statusReadFixture(t)
			a := repo.attempts[inspectionAttempt]
			a.Status = state
			repo.attempts[inspectionAttempt] = a
			got, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
			if err != nil {
				t.Fatal(err)
			}
			if got.EvaluationStatus != state || client.decisionGets != 0 || got.BadgeResult == "VERIFIED" {
				t.Fatalf("terminal state resurrected: %+v, provider reads=%d", got, client.decisionGets)
			}
		})
	}
}
func TestInspectionDetachedApprovalSurvivesPurgedDeclaration(t *testing.T) {
	svc, repo, client, decl := statusReadFixture(t)
	a := repo.attempts[inspectionAttempt]
	a.Status = "APPROVED"
	a.DeclarationID = ""
	repo.attempts[inspectionAttempt] = a
	decl.rows = map[string]DeclarationRevision{}
	got, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "APPROVED" || got.EvaluationStatus != "APPROVED" || got.BadgeResult == "VERIFIED" || client.decisionGets != 0 {
		t.Fatalf("detached sandbox approval: %+v", got)
	}
}
func TestInspectionProviderFailureIsSafeUnavailable(t *testing.T) {
	svc, _, client, _ := statusReadFixture(t)
	client.decErr = errors.New("sensitive-provider-detail")
	_, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
	if !errors.Is(err, ErrDiditUnavailable) || strings.Contains(err.Error(), "sensitive-provider-detail") {
		t.Fatalf("unsafe provider error: %v", err)
	}
}
func TestInspectionForeignAccountNeverCallsProvider(t *testing.T) {
	svc, _, client, _ := statusReadFixture(t)
	_, err := svc.InspectStatus(context.Background(), "44444444-4444-4444-8444-444444444444", inspectionAttempt)
	if !errors.Is(err, ErrNotFound) || client.decisionGets != 0 {
		t.Fatalf("foreign account: err=%v calls=%d", err, client.decisionGets)
	}
}
func TestInspectionWithoutSessionDoesNotCallProvider(t *testing.T) {
	svc, repo, client, _ := statusReadFixture(t)
	a := repo.attempts[inspectionAttempt]
	a.ProviderSessionID = ""
	a.Status = "CREATED"
	repo.attempts[inspectionAttempt] = a
	got, err := svc.InspectStatus(context.Background(), inspectionAccount, inspectionAttempt)
	if err != nil || got.ProviderStatus != "" || client.decisionGets != 0 || got.BadgeResult == "VERIFIED" {
		t.Fatalf("pending inspection: %+v err=%v", got, err)
	}
}

type inspectionLatestRepo struct {
	*memProviderAttemptRepo
	requestedAccount string
}

func (r *inspectionLatestRepo) GetLatestAttemptForAccount(_ context.Context, account, provider, environment, app string, now time.Time) (ProviderAttempt, error) {
	r.requestedAccount = account
	var latest ProviderAttempt
	for _, a := range r.attempts {
		if a.AccountID == account && a.Provider == provider && a.Environment == environment && a.ApplicationID == app && a.RetentionAt.After(now) && (latest.ID == "" || a.CreatedAt.After(latest.CreatedAt) || a.CreatedAt.Equal(latest.CreatedAt) && a.ID > latest.ID) {
			latest = a
		}
	}
	if latest.ID == "" {
		return ProviderAttempt{}, ErrNotFound
	}
	return latest, nil
}

func TestInspectionLatestIsOwnerScopedAndDoesNotCreate(t *testing.T) {
	svc, repo, client, _ := statusReadFixture(t)
	wrapped := &inspectionLatestRepo{memProviderAttemptRepo: repo}
	svc.repo = wrapped
	foreign := repo.attempts[inspectionAttempt]
	foreign.ID = "55555555-5555-4555-8555-555555555555"
	foreign.AccountID = "44444444-4444-4444-8444-444444444444"
	foreign.CreatedAt = foreign.CreatedAt.Add(time.Minute)
	repo.attempts[foreign.ID] = foreign
	got, err := svc.GetLatestStatus(context.Background(), inspectionAccount)
	if err != nil || got.ID != inspectionAttempt || wrapped.requestedAccount != inspectionAccount || len(client.created) != 0 {
		t.Fatalf("latest scope: %+v err=%v", got, err)
	}
}

func TestSavedDeclarationFullNameReachesProviderExpectedDetails(t *testing.T) {
	svc, repo, client, _ := statusReadFixture(t)
	repo.attempts = make(map[string]ProviderAttempt)
	svc.ids = &counterUUIDs{n: 10}
	client.createResp = CreateSessionResponse{SessionID: "new-inspection-session", SessionURL: "https://verify.didit.me/session/new-inspection-session"}
	_, err := svc.StartOrResume(context.Background(), inspectionAccount, "full-name-characterization", StartDiditIntentInput{ConsentGiven: true, ConsentVersion: "2026-11-v1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.created) != 1 {
		t.Fatalf("provider calls=%d", len(client.created))
	}
	details := client.created[0].ExpectedDetails
	if details.FirstName != "Ana María" || details.LastName != "Pérez" || details.DateOfBirth != "1990-01-02" || details.IdentificationNumber != "12345670-K" || details.IDCountry != "CHL" || !reflect.DeepEqual(details.ExpectedDocumentTypes, []string{"ID"}) {
		t.Fatal("saved declaration was altered before provider request")
	}
}
