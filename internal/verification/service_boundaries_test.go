package verification

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockVerificationRepoForBoundaries struct {
	cases            map[string]Case
	listOwnCases     []Case
	pendingCases     []Case
	rejectedCases    []Case
	eligibilities    []Eligibility
	histories        []HistoryEntry
	revokeCase       Case
	revokeErr        error
	reviewCase       Case
	reviewErr        error
	retryCase        Case
	retryErr         error
	revokeCalledWith struct {
		id, reviewer, reason, idem, corr string
	}
}

func newMockVerificationRepoForBoundaries() *mockVerificationRepoForBoundaries {
	return &mockVerificationRepoForBoundaries{cases: make(map[string]Case)}
}

func (m *mockVerificationRepoForBoundaries) Create(_ context.Context, c Case) (Case, error) {
	m.cases[c.ID] = c
	return c, nil
}

func (m *mockVerificationRepoForBoundaries) GetOwn(_ context.Context, owner, id string) (Case, error) {
	c, ok := m.cases[id]
	if !ok || c.OwnerID != owner {
		return Case{}, ErrNotFound
	}
	return c, nil
}

func (m *mockVerificationRepoForBoundaries) ListOwn(_ context.Context, owner string) ([]Case, error) {
	var out []Case
	for _, c := range m.listOwnCases {
		if c.OwnerID == owner {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockVerificationRepoForBoundaries) ListPending(_ context.Context) ([]Case, error) {
	return m.pendingCases, nil
}

func (m *mockVerificationRepoForBoundaries) ListRejected(_ context.Context) ([]Case, error) {
	return m.rejectedCases, nil
}

func (m *mockVerificationRepoForBoundaries) Review(_ context.Context, id, reviewer string, approve bool, reason string, _ func() time.Time) (Case, error) {
	if m.reviewErr != nil {
		return Case{}, m.reviewErr
	}
	if m.reviewCase.ID != "" {
		return m.reviewCase, nil
	}
	c := m.cases[id]
	c.ReviewerID = reviewer
	if approve {
		c.State = "aprobada"
	} else {
		c.State = "rechazada"
		c.ReasonCode = reason
	}
	m.cases[id] = c
	return c, nil
}

func (m *mockVerificationRepoForBoundaries) Retry(_ context.Context, _, _ string, c Case, _ func() time.Time) (Case, error) {
	if m.retryErr != nil {
		return Case{}, m.retryErr
	}
	if m.retryCase.ID != "" {
		return m.retryCase, nil
	}
	m.cases[c.ID] = c
	return c, nil
}

func (m *mockVerificationRepoForBoundaries) Revoke(_ context.Context, id, reviewer, reason, idem, corr string, _ func() time.Time) (Case, error) {
	m.revokeCalledWith = struct{ id, reviewer, reason, idem, corr string }{id, reviewer, reason, idem, corr}
	if m.revokeErr != nil {
		return Case{}, m.revokeErr
	}
	if m.revokeCase.ID != "" {
		return m.revokeCase, nil
	}
	c := m.cases[id]
	c.State = "revocada"
	m.cases[id] = c
	return c, nil
}

func (m *mockVerificationRepoForBoundaries) ListEligibility(_ context.Context, _ string) ([]Eligibility, error) {
	return m.eligibilities, nil
}

func (m *mockVerificationRepoForBoundaries) ListHistory(_ context.Context, _, _ string) ([]HistoryEntry, error) {
	return m.histories, nil
}

func TestServiceOwnAndListingBoundaries(t *testing.T) {
	ctx := context.Background()
	repo := newMockVerificationRepoForBoundaries()
	svc, err := NewService(repo, &ids{}, LocalFixtureProvider{}, func() time.Time { return time.Unix(10, 0) })
	if err != nil {
		t.Fatal(err)
	}

	// Own parameter boundaries
	if _, err := svc.Own(ctx, "", "case-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner in Own")
	}
	if _, err := svc.Own(ctx, "owner-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty id in Own")
	}

	// ListOwn parameter boundaries
	if _, err := svc.ListOwn(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner in ListOwn")
	}

	// Success Own & ListOwn
	repo.cases["case-1"] = Case{ID: "case-1", OwnerID: "owner-1", State: "en_revision"}
	repo.listOwnCases = []Case{{ID: "case-1", OwnerID: "owner-1", State: "en_revision"}}

	c, err := svc.Own(ctx, "owner-1", "case-1")
	if err != nil || c.ID != "case-1" {
		t.Fatalf("unexpected Own result: %+v, err: %v", c, err)
	}
	list, err := svc.ListOwn(ctx, "owner-1")
	if err != nil || len(list) != 1 {
		t.Fatalf("unexpected ListOwn result: %+v, err: %v", list, err)
	}

	// Pending and Rejected delegation
	repo.pendingCases = []Case{{ID: "case-p1", State: "en_revision"}}
	repo.rejectedCases = []Case{{ID: "case-r1", State: "rechazada"}}
	p, err := svc.Pending(ctx)
	if err != nil || len(p) != 1 || p[0].ID != "case-p1" {
		t.Fatalf("unexpected Pending result: %+v, err: %v", p, err)
	}
	r, err := svc.Rejected(ctx)
	if err != nil || len(r) != 1 || r[0].ID != "case-r1" {
		t.Fatalf("unexpected Rejected result: %+v, err: %v", r, err)
	}
}

func TestServiceRevokeBoundaries(t *testing.T) {
	ctx := context.Background()
	repo := newMockVerificationRepoForBoundaries()
	svc, _ := NewService(repo, &ids{}, LocalFixtureProvider{}, func() time.Time { return time.Unix(10, 0) })

	validID := "case-rev-1"
	validReviewer := "admin-rev"
	validReason1 := "aprobacion_fixture_incorrecta"
	validReason2 := "revision_fixture_actualizada"
	validIdem := "idempotency-key-valid-12345"
	validCorr := "corr-12345"

	// Parameter validation
	if _, err := svc.Revoke(ctx, "", validReviewer, validReason1, validIdem, validCorr); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty id")
	}
	if _, err := svc.Revoke(ctx, validID, "", validReason1, validIdem, validCorr); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty reviewer")
	}
	if _, err := svc.Revoke(ctx, validID, validReviewer, "invalid_reason", validIdem, validCorr); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for unknown revocation reason")
	}
	if _, err := svc.Revoke(ctx, validID, validReviewer, validReason1, "short", validCorr); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for idempotencyKey < 8 chars")
	}
	if _, err := svc.Revoke(ctx, validID, validReviewer, validReason1, strings.Repeat("x", 201), validCorr); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for idempotencyKey > 200 chars")
	}
	if _, err := svc.Revoke(ctx, validID, validReviewer, validReason1, validIdem, "   "); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty/whitespace correlationID")
	}
	if _, err := svc.Revoke(ctx, validID, validReviewer, validReason1, validIdem, strings.Repeat("c", 121)); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for correlationID > 120 chars")
	}

	// Success path with both valid reasons
	repo.cases[validID] = Case{ID: validID, State: "aprobada"}
	rev1, err := svc.Revoke(ctx, validID, validReviewer, validReason1, validIdem, validCorr)
	if err != nil || rev1.State != "revocada" {
		t.Fatalf("unexpected Revoke result: %+v err: %v", rev1, err)
	}
	if repo.revokeCalledWith.reason != validReason1 || repo.revokeCalledWith.idem != validIdem || repo.revokeCalledWith.corr != validCorr {
		t.Fatalf("mismatched revoke parameters: %+v", repo.revokeCalledWith)
	}

	rev2, err := svc.Revoke(ctx, validID, validReviewer, validReason2, validIdem, validCorr)
	if err != nil || rev2.State != "revocada" {
		t.Fatalf("unexpected Revoke result: %+v err: %v", rev2, err)
	}
	if repo.revokeCalledWith.reason != validReason2 {
		t.Fatalf("expected reason %s, got: %s", validReason2, repo.revokeCalledWith.reason)
	}
}

func TestServiceEligibilityAndHistoryBoundaries(t *testing.T) {
	ctx := context.Background()
	repo := newMockVerificationRepoForBoundaries()
	svc, _ := NewService(repo, &ids{}, LocalFixtureProvider{}, func() time.Time { return time.Unix(10, 0) })

	// Eligibility boundaries
	if _, err := svc.Eligibility(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner in Eligibility")
	}
	repo.eligibilities = []Eligibility{{Type: "kyc", Eligible: true}}
	elig, err := svc.Eligibility(ctx, "owner-1")
	if err != nil || len(elig) != 1 || !elig[0].Eligible {
		t.Fatalf("unexpected Eligibility result: %+v err: %v", elig, err)
	}

	// History boundaries
	if _, err := svc.History(ctx, "", "case-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner in History")
	}
	if _, err := svc.History(ctx, "owner-1", ""); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty id in History")
	}
	repo.histories = []HistoryEntry{{Sequence: 1, Action: "start", ToState: "en_revision"}}
	hist, err := svc.History(ctx, "owner-1", "case-1")
	if err != nil || len(hist) != 1 || hist[0].Sequence != 1 || hist[0].ToState != "en_revision" {
		t.Fatalf("unexpected History result: %+v err: %v", hist, err)
	}
}

func TestServiceRetryComprehensiveCorrectionBoundaries(t *testing.T) {
	ctx := context.Background()
	repo := newMockVerificationRepoForBoundaries()
	svc, _ := NewService(repo, &ids{}, LocalFixtureProvider{}, func() time.Time { return time.Unix(10, 0) })

	// Parameter boundaries
	if _, err := svc.Retry(ctx, "", "prior-1", "idempotency-1", "corr-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty owner")
	}
	if _, err := svc.Retry(ctx, "owner-1", "", "idempotency-1", "corr-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for empty priorID")
	}
	if _, err := svc.Retry(ctx, "owner-1", "prior-1", "short", "corr-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for idempotency < 8 chars")
	}
	if _, err := svc.Retry(ctx, "owner-1", "prior-1", strings.Repeat("x", 81), "corr-1"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for idempotency > 80 chars")
	}

	// Prior case not in rechazada state
	repo.cases["prior-active"] = Case{ID: "prior-active", OwnerID: "owner-1", State: "en_revision", Type: "kyc"}
	if _, err := svc.Retry(ctx, "owner-1", "prior-active", "idempotency-123", "fixture_vigente_actualizado"); !errors.Is(err, ErrInvalid) {
		t.Fatal("expected ErrInvalid for non-rechazada prior case")
	}

	// validCorrection table matrix
	testCases := []struct {
		reason     string
		correction string
		valid      bool
	}{
		{"documento_vencido", "fixture_vigente_actualizado", true},
		{"documento_vencido", "wrong_correction", false},
		{"antecedentes_incompletos", "antecedentes_fixture_actualizados", true},
		{"antecedentes_incompletos", "wrong_correction", false},
		{"inicio_actividades_no_confirmado", "inicio_actividades_fixture_actualizadas", true},
		{"inicio_actividades_no_confirmado", "wrong_correction", false},
		{"other_unknown_reason", "fixture_vigente_actualizado", false},
	}

	for _, tc := range testCases {
		caseID := "prior-" + tc.reason
		repo.cases[caseID] = Case{ID: caseID, OwnerID: "owner-1", State: "rechazada", ReasonCode: tc.reason, Type: "kyc"}
		repo.retryCase = Case{ID: "retry-case", OwnerID: "owner-1", RetryOf: caseID, State: "en_revision"}
		_, err := svc.Retry(ctx, "owner-1", caseID, "idempotency-12345", tc.correction)
		if tc.valid && err != nil {
			t.Errorf("expected valid retry for (%s, %s), got: %v", tc.reason, tc.correction, err)
		} else if !tc.valid && !errors.Is(err, ErrInvalid) {
			t.Errorf("expected ErrInvalid for (%s, %s), got: %v", tc.reason, tc.correction, err)
		}
	}
}
