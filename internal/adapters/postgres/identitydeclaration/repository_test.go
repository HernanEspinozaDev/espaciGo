package identitydeclaration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

func TestPurgeExpiredRevisionsUsesRepositoryOperation(t *testing.T) {
	repo := (*Repository)(nil)
	_, err := repo.PurgeExpiredRevisions(context.Background())
	if err == nil {
		t.Fatal("nil repository must be rejected")
	}
}

func TestPurgeDeliveredSessionOutboxRejectsNilRepository(t *testing.T) {
	var repo *Repository
	if _, err := repo.PurgeDeliveredSessionOutbox(context.Background()); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("nil repository err=%v; want ErrInvalid", err)
	}
}

func TestCreateRevisionRejectsInvalidInputBeforeSealing(t *testing.T) {
	cases := []RevisionInput{
		{ID: "bad", AccountID: "85000000-0000-4000-8000-000000000001", SchemaVersion: "1", ConsentVersion: "v", ConsentGivenAt: time.Now(), RetentionAt: time.Now().Add(time.Hour)},
		{ID: "85000000-0000-4000-8000-000000000003", AccountID: "85000000-0000-4000-8000-000000000001", SchemaVersion: "2", ConsentVersion: "v", ConsentGivenAt: time.Now(), RetentionAt: time.Now().Add(time.Hour)},
	}
	for _, in := range cases {
		t.Run(in.ID, func(t *testing.T) {
			called := false
			_, err := (*Repository)(nil).CreateRevision(context.Background(), in, func(int64) (string, []byte, error) { called = true; return "", nil, nil })
			if !errors.Is(err, verification.ErrInvalid) || called {
				t.Fatalf("err=%v called=%v", err, called)
			}
		})
	}
}

func TestProviderAttemptRepositoryRejectsInvalidAndNilReceiverInputs(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	var repo *Repository

	if _, _, err := repo.PrepareAttempt(ctx, verification.PrepareAttemptInput{}); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("PrepareAttempt nil repo err=%v; want ErrInvalid", err)
	}
	if _, err := repo.MarkAttemptSessionReady(ctx, "bad-id", "sess-1", now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("MarkAttemptSessionReady err=%v; want ErrInvalid", err)
	}
	if err := repo.MarkAttemptFailed(ctx, "bad-id", now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("MarkAttemptFailed err=%v; want ErrInvalid", err)
	}
	if _, err := repo.GetAttemptForAccount(ctx, "bad", "bad", now, time.Minute); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("GetAttemptForAccount err=%v; want ErrInvalid", err)
	}
	if _, err := repo.FindAttemptForWebhook(ctx, "", "SANDBOX", "app", "corr", "sess"); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("FindAttemptForWebhook err=%v; want ErrInvalid", err)
	}
	if _, _, err := repo.InsertOrGetWebhookEvent(ctx, verification.WebhookEventRecordInput{}); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("InsertOrGetWebhookEvent err=%v; want ErrInvalid", err)
	}
	if _, err := repo.SaveAttemptDecisionAndCompleteEvent(ctx, "bad", "bad", "APPROVED", "v1", verification.DecisionAlgorithmV1, make([]byte, 32), now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("SaveAttemptDecisionAndCompleteEvent err=%v; want ErrInvalid", err)
	}
	if _, err := repo.UpdateAttemptStatusAndCompleteEvent(ctx, "bad", "bad", "EXPIRED", "PROCESSED", now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("UpdateAttemptStatusAndCompleteEvent err=%v; want ErrInvalid", err)
	}
	if err := repo.CompleteWebhookEvent(ctx, "bad", "IGNORED", now); !errors.Is(err, verification.ErrInvalid) {
		t.Fatalf("CompleteWebhookEvent err=%v; want ErrInvalid", err)
	}
}
