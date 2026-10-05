package verification

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

type ids struct{ n int }

func (g *ids) ID() (string, error) {
	g.n++
	return "00000000-0000-4000-8000-00000000000" + string(rune('0'+g.n)), nil
}

type memoryRepo struct {
	items map[string]Case
	byKey map[string]string
}

func newMemoryRepo() *memoryRepo {
	return &memoryRepo{items: map[string]Case{}, byKey: map[string]string{}}
}
func (r *memoryRepo) Create(_ context.Context, c Case) (Case, error) {
	key := c.OwnerID + "/" + c.Idempotency
	if id := r.byKey[key]; id != "" {
		return r.items[id], nil
	}
	r.items[c.ID] = c
	r.byKey[key] = c.ID
	return c, nil
}
func (r *memoryRepo) GetOwn(_ context.Context, owner, id string) (Case, error) {
	c, ok := r.items[id]
	if !ok || c.OwnerID != owner {
		return Case{}, ErrNotFound
	}
	return c, nil
}
func (r *memoryRepo) ListOwn(context.Context, string) ([]Case, error) { return nil, nil }
func (r *memoryRepo) ListPending(context.Context) ([]Case, error)     { return nil, nil }
func (r *memoryRepo) Review(_ context.Context, id, reviewer string, approve bool, reason string) (Case, error) {
	c, ok := r.items[id]
	if !ok {
		return Case{}, ErrNotFound
	}
	if c.State != "en_revision" {
		return Case{}, ErrConflict
	}
	c.ReviewerID = reviewer
	c.ResolvedAt = timePtr(time.Unix(2, 0))
	c.State = "rechazada"
	c.ReasonCode = reason
	if approve {
		c.State = "aprobada"
		c.ReasonCode = ""
	}
	r.items[id] = c
	return c, nil
}
func (r *memoryRepo) Retry(_ context.Context, owner, prior string, c Case) (Case, error) {
	old, e := r.GetOwn(context.Background(), owner, prior)
	if e != nil {
		return Case{}, e
	}
	if old.State != "rechazada" {
		return Case{}, ErrConflict
	}
	c.Type = old.Type
	c.RetryOf = prior
	r.items[c.ID] = c
	return c, nil
}
func timePtr(t time.Time) *time.Time { return &t }

func TestSyntheticCaseReviewRetryAndSafeSerialization(t *testing.T) {
	repo := newMemoryRepo()
	svc, err := NewService(repo, &ids{}, LocalFixtureProvider{}, func() time.Time { return time.Unix(1, 0) })
	if err != nil {
		t.Fatal(err)
	}
	item, err := svc.Start(context.Background(), "owner", "kyc", "request-0001")
	if err != nil {
		t.Fatal(err)
	}
	if item.State != "en_revision" || item.Provider != "local-fixture-v1" || len(item.EvidenceRef) < 8 {
		t.Fatalf("unexpected fixture case: %+v", item)
	}
	if _, err := svc.Review(context.Background(), item.ID, "admin", "rechazada", ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing reason err=%v", err)
	}
	rejected, err := svc.Review(context.Background(), item.ID, "admin", "rechazada", "antecedentes_incompletos")
	if err != nil || rejected.State != "rechazada" {
		t.Fatalf("review: %+v err=%v", rejected, err)
	}
	if _, err := svc.Retry(context.Background(), "other", item.ID, "retry-key-01", true); !errors.Is(err, ErrNotFound) {
		t.Fatalf("other owner retry err=%v", err)
	}
	if _, err := svc.Retry(context.Background(), "owner", item.ID, "retry-key-01", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("uncorrected retry err=%v", err)
	}
	retried, err := svc.Retry(context.Background(), "owner", item.ID, "retry-key-01", true)
	if err != nil || retried.RetryOf != item.ID || retried.Type != "kyc" || retried.State != "en_revision" {
		t.Fatalf("retry: %+v err=%v", retried, err)
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var output map[string]any
	if err = json.Unmarshal(encoded, &output); err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"owner_id", "reviewer_id", "idempotency"} {
		if _, ok := output[hidden]; ok {
			t.Errorf("serialized private field %q", hidden)
		}
	}
}

func TestReviewRejectsUnknownReasonAndApprovalReason(t *testing.T) {
	svc, _ := NewService(newMemoryRepo(), &ids{}, LocalFixtureProvider{}, time.Now)
	for _, tc := range []struct{ outcome, reason string }{{"rechazada", "free text"}, {"aprobada", "antecedentes_incompletos"}} {
		if _, err := svc.Review(context.Background(), "id", "admin", tc.outcome, tc.reason); !errors.Is(err, ErrInvalid) {
			t.Errorf("%+v err=%v", tc, err)
		}
	}
}
