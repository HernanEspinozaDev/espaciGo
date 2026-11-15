package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type lateReadyAttemptRepo struct {
	runtimeAttemptRepo
	attempt     verification.ProviderAttempt
	confirmed   bool
	readbackErr error
}

func (r *lateReadyAttemptRepo) PrepareAttempt(ctx context.Context, in verification.PrepareAttemptInput) (verification.ProviderAttempt, bool, error) {
	a, _, err := r.runtimeAttemptRepo.PrepareAttempt(ctx, in)
	r.attempt = a
	return a, false, err
}
func (r *lateReadyAttemptRepo) MarkAttemptSessionReady(context.Context, string, string, time.Time) (verification.ProviderAttempt, error) {
	return verification.ProviderAttempt{}, verification.ErrConflict
}
func (r *lateReadyAttemptRepo) GetAttemptForAccount(context.Context, string, string, time.Time, time.Duration) (verification.ProviderAttempt, error) {
	if r.readbackErr != nil {
		return verification.ProviderAttempt{}, r.readbackErr
	}
	a := r.attempt
	if r.confirmed {
		a.Status = "PENDING_USER_INPUT"
		a.ProviderSessionID = "synthetic-late-session"
		return a, nil
	}
	a.Status = "EXPIRED"
	a.DeclarationID = ""
	return a, nil
}

type lateReadyClient struct {
	calls  int
	cancel context.CancelFunc
}

func (c *lateReadyClient) CreateSession(context.Context, verification.CreateSessionRequest) (verification.CreateSessionResponse, error) {
	c.calls++
	if c.cancel != nil {
		c.cancel()
	}
	return verification.CreateSessionResponse{SessionID: "synthetic-late-session", SessionURL: "https://verify.didit.me/session/synthetic-late-session"}, nil
}
func (*lateReadyClient) GetSessionDecision(context.Context, string) (verification.DiditDecisionResponse, error) {
	return verification.DiditDecisionResponse{}, verification.ErrInvalid
}
func (*lateReadyClient) VerifyWebhookSignature(string, []byte) bool { return false }

type lateReadyQueue struct {
	sessions []string
	err      error
}

func (q *lateReadyQueue) EnqueueSessionDeletion(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if q.err != nil {
		return q.err
	}
	q.sessions = append(q.sessions, id)
	return nil
}
func (q *lateReadyQueue) PendingSessionDeletions(context.Context) ([]string, error) {
	return append([]string(nil), q.sessions...), nil
}
func (*lateReadyQueue) AcknowledgeSessionDeletion(context.Context, string) error { return nil }

func TestDiditKnownLateSessionQueuesAfterReadyFailure(t *testing.T) {
	for _, scenario := range []string{"normal", "canceled-request", "queue-failure", "confirmed-ready", "readback-unavailable"} {
		t.Run(scenario, func(t *testing.T) {
			values := runtimeRotationEnvironment(t)
			delete(values, "IDENTITY_RUT_HMAC_KEYRING_FILE")
			values["IDENTITY_RUT_HMAC_SECRET"] = "synthetic-hmac-legacy-key-material"
			now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
			declarations, err := newRuntimeIdentityDeclarationService(env(values), &runtimeDeclarationRepo{rows: map[string]verification.DeclarationRevision{}}, now)
			if err != nil {
				t.Fatal(err)
			}
			account := "11111111-1111-4111-8111-111111111111"
			if _, err := declarations.Save(context.Background(), account, verification.DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-9", IDCountry: "CHL", DocumentType: "ID"}, true, defaultIdentityDeclarationConsentVersion); err != nil {
				t.Fatal(err)
			}
			keys, err := loadIdentityDeclarationKeyring(env(values))
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client := &lateReadyClient{}
			if scenario == "canceled-request" {
				client.cancel = cancel
			}
			queue := &lateReadyQueue{}
			queueFailure := errors.New("synthetic deletion queue failure")
			if scenario == "queue-failure" {
				queue.err = queueFailure
			}
			repo := &lateReadyAttemptRepo{confirmed: scenario == "confirmed-ready"}
			readbackFailure := errors.New("synthetic readback unavailable")
			if scenario == "readback-unavailable" {
				repo.readbackErr = readbackFailure
			}
			svc, err := verification.NewDiditService(repo, declarations, client, keys, &lateReadyIDs{}, now, verification.DiditServiceConfig{WorkflowID: "synthetic-workflow", ConsentVersion: defaultDiditConsentVersion, DeclarationConsentVersion: defaultIdentityDeclarationConsentVersion, RutHMACSecret: []byte(values["IDENTITY_RUT_HMAC_SECRET"]), RutHMACKeyVersion: "v1"}, &fakeRutClaimStore{}, queue)
			if err != nil {
				t.Fatal(err)
			}
			started, err := svc.StartOrResume(ctx, account, "late-ready", verification.StartDiditIntentInput{ConsentGiven: true, ConsentVersion: defaultDiditConsentVersion})
			if scenario == "confirmed-ready" {
				if err != nil || started.RedirectURL != "https://verify.didit.me/session/synthetic-late-session" || len(queue.sessions) != 0 || client.calls != 1 {
					t.Fatal("confirmed ready readback must recover without deleting the session or repeating CreateSession")
				}
				return
			}
			if !errors.Is(err, verification.ErrConflict) || started.RedirectURL != "" || client.calls != 1 {
				t.Fatalf("ready failure must remain an error without redirect or provider replay: error=%v calls=%d", err, client.calls)
			}
			if scenario == "readback-unavailable" {
				if !errors.Is(err, readbackFailure) || len(queue.sessions) != 0 {
					t.Fatal("uncertain readback must remain explicit and cannot authorize remote deletion")
				}
			} else if scenario == "queue-failure" {
				if !errors.Is(err, queueFailure) {
					t.Fatal("failure to persist deletion intent was hidden")
				}
				if len(queue.sessions) != 0 {
					t.Fatal("failed enqueue was represented as persisted")
				}
			} else if len(queue.sessions) != 1 || queue.sessions[0] != "synthetic-late-session" {
				t.Fatal("known session was lost after ready failure instead of queued for deletion")
			}
		})
	}
}

type lateReadyIDs struct{ n int }

func (g *lateReadyIDs) ID() (string, error) {
	g.n++
	switch g.n {
	case 1:
		return "55555555-5555-4555-8555-555555555555", nil
	case 2:
		return "66666666-6666-4666-8666-666666666666", nil
	default:
		return "77777777-7777-4777-8777-777777777777", nil
	}
}
