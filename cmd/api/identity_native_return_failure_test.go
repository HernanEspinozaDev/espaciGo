package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type nativeReturnFailureSnapshot struct {
	Faults      int
	Transitions int
	Recovered   bool
}

type nativeReturnFailureRepository struct {
	verification.ProviderAttemptRepository
	mu            sync.Mutex
	accountID     string
	trusted       func() nativeDeclarationIntent
	attemptID     string
	declarationID string
	faults        int
	transitions   int
	recovered     bool
}

func newNativeReturnFailureRepository(delegate verification.ProviderAttemptRepository, accountID string, trustedDeclaration func() nativeDeclarationIntent) (*nativeReturnFailureRepository, error) {
	if delegate == nil || !canonicalNativeUUID(accountID) || trustedDeclaration == nil {
		return nil, verification.ErrInvalid
	}
	return &nativeReturnFailureRepository{ProviderAttemptRepository: delegate, accountID: accountID, trusted: trustedDeclaration}, nil
}

func (r *nativeReturnFailureRepository) GetAttemptForAccount(ctx context.Context, accountID, attemptID string, now time.Time, ttl time.Duration) (verification.ProviderAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	attempt, err := r.ProviderAttemptRepository.GetAttemptForAccount(ctx, accountID, attemptID, now, ttl)
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	if !canonicalNativeUUID(attemptID) || attempt.ID != attemptID || attempt.AccountID != accountID {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	if accountID != r.accountID {
		return attempt, nil
	}
	intent := r.trusted()
	if !canonicalNativeUUID(intent.AccountID) || !canonicalNativeUUID(intent.DeclarationID) || intent.CorrelationID == "" || intent.AccountID != r.accountID || attempt.AccountID != r.accountID || attempt.DeclarationID != intent.DeclarationID {
		return attempt, nil
	}
	if r.attemptID == "" {
		r.attemptID, r.declarationID = attempt.ID, intent.DeclarationID
	} else if attempt.ID != r.attemptID || intent.DeclarationID != r.declarationID {
		return attempt, nil
	}
	if attempt.Status != "PENDING_USER_INPUT" || r.recovered {
		return attempt, nil
	}
	if r.faults == 0 {
		r.faults++
		return verification.ProviderAttempt{}, verification.ErrDiditUnavailable
	}
	if r.transitions == 0 {
		r.transitions++
		if err := r.ProviderAttemptRepository.MarkAttemptFailed(ctx, attempt.ID, now); err != nil {
			return verification.ProviderAttempt{}, err
		}
		updated, readErr := r.ProviderAttemptRepository.GetAttemptForAccount(ctx, accountID, attemptID, now, ttl)
		if readErr != nil {
			return verification.ProviderAttempt{}, readErr
		}
		if updated.Status != "FAILED" || updated.ID != attemptID || updated.AccountID != accountID || updated.DeclarationID != intent.DeclarationID {
			return verification.ProviderAttempt{}, verification.ErrInvalid
		}
		r.recovered = true
		return updated, nil
	}
	return attempt, nil
}

func (r *nativeReturnFailureRepository) Snapshot() nativeReturnFailureSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return nativeReturnFailureSnapshot{Faults: r.faults, Transitions: r.transitions, Recovered: r.recovered}
}

type nativeReturnFailureDelegate struct {
	verification.ProviderAttemptRepository
	attempt         verification.ProviderAttempt
	getErr, markErr error
	getErrAt        int
	afterMark       func(*nativeReturnFailureDelegate)
	gets, marks     int
}

func (d *nativeReturnFailureDelegate) GetAttemptForAccount(_ context.Context, _, _ string, _ time.Time, _ time.Duration) (verification.ProviderAttempt, error) {
	d.gets++
	if d.getErr != nil && (d.getErrAt == 0 || d.gets == d.getErrAt) {
		return verification.ProviderAttempt{}, d.getErr
	}
	return d.attempt, nil
}
func (d *nativeReturnFailureDelegate) MarkAttemptFailed(_ context.Context, _ string, _ time.Time) error {
	d.marks++
	if d.markErr != nil {
		return d.markErr
	}
	d.attempt.Status = "FAILED"
	if d.afterMark != nil {
		d.afterMark(d)
	}
	return nil
}

func nativeReturnTestRepository(t *testing.T) (*nativeReturnFailureRepository, *nativeReturnFailureDelegate, string, string) {
	t.Helper()
	account, declaration := "11111111-1111-4111-8111-111111111111", "22222222-2222-4222-8222-222222222222"
	delegate := &nativeReturnFailureDelegate{attempt: verification.ProviderAttempt{ID: "33333333-3333-4333-8333-333333333333", AccountID: account, DeclarationID: declaration, Status: "PENDING_USER_INPUT"}}
	repo, err := newNativeReturnFailureRepository(delegate, account, func() nativeDeclarationIntent {
		return nativeDeclarationIntent{AccountID: account, DeclarationID: declaration, CorrelationID: "synthetic-return-correlation"}
	})
	if err != nil {
		t.Fatal("constructor rejected valid scope")
	}
	return repo, delegate, account, declaration
}

func TestNativeReturnFailureRetryTransitionsOnlyOnce(t *testing.T) {
	repo, d, account, _ := nativeReturnTestRepository(t)
	ctx, now := context.Background(), time.Now()
	if _, err := repo.GetAttemptForAccount(ctx, account, d.attempt.ID, now, time.Minute); !errors.Is(err, verification.ErrDiditUnavailable) {
		t.Fatalf("first read err=%v", err)
	}
	got, err := repo.GetAttemptForAccount(ctx, account, d.attempt.ID, now, time.Minute)
	if err != nil || got.Status != "FAILED" || d.marks != 1 || d.gets != 3 {
		t.Fatalf("second read status=%s err=%v marks=%d gets=%d", got.Status, err, d.marks, d.gets)
	}
	got, err = repo.GetAttemptForAccount(ctx, account, d.attempt.ID, now, time.Minute)
	if err != nil || got.Status != "FAILED" || d.marks != 1 {
		t.Fatalf("third read status=%s err=%v marks=%d", got.Status, err, d.marks)
	}
	if s := repo.Snapshot(); s != (nativeReturnFailureSnapshot{Faults: 1, Transitions: 1, Recovered: true}) {
		t.Fatalf("snapshot=%+v", s)
	}
}

func TestNativeReturnFailureDoesNotConsumeForUntrustedReads(t *testing.T) {
	for _, tc := range []struct {
		name                                 string
		account, attemptAccount, declaration string
		getErr                               error
	}{
		{"foreign", "44444444-4444-4444-8444-444444444444", "", "", nil},
		{"untrusted declaration", "11111111-1111-4111-8111-111111111111", "11111111-1111-4111-8111-111111111111", "55555555-5555-4555-8555-555555555555", nil},
		{"unknown", "11111111-1111-4111-8111-111111111111", "", "", verification.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo, d, account, decl := nativeReturnTestRepository(t)
			d.getErr = tc.getErr
			if tc.attemptAccount != "" {
				d.attempt.AccountID = tc.attemptAccount
				d.attempt.DeclarationID = tc.declaration
			}
			_, err := repo.GetAttemptForAccount(context.Background(), tc.account, d.attempt.ID, time.Now(), time.Minute)
			if tc.getErr != nil && !errors.Is(err, tc.getErr) {
				t.Fatalf("err=%v", err)
			}
			if s := repo.Snapshot(); s.Faults != 0 || s.Transitions != 0 || s.Recovered {
				t.Fatalf("unexpected consumption: %+v", s)
			}
			if d.marks != 0 {
				t.Fatal("untrusted read mutated attempt")
			}
			_ = account
			_ = decl
		})
	}
}

func TestNativeReturnFailurePropagatesTransitionAndReadbackErrors(t *testing.T) {
	for _, stage := range []string{"mark", "readback"} {
		t.Run(stage, func(t *testing.T) {
			repo, d, account, _ := nativeReturnTestRepository(t)
			ctx := context.Background()
			now := time.Now()
			_, _ = repo.GetAttemptForAccount(ctx, account, d.attempt.ID, now, time.Minute)
			boom := errors.New("repository failure")
			if stage == "mark" {
				d.markErr = boom
			} else {
				d.getErr = boom
				d.getErrAt = 3
			}
			_, err := repo.GetAttemptForAccount(ctx, account, d.attempt.ID, now, time.Minute)
			if !errors.Is(err, boom) {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestNativeReturnFailureConcurrentReadsRemainBounded(t *testing.T) {
	repo, d, account, _ := nativeReturnTestRepository(t)
	const callers = 24
	var wg sync.WaitGroup
	var mu sync.Mutex
	faults := 0
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := repo.GetAttemptForAccount(context.Background(), account, d.attempt.ID, time.Now(), time.Minute)
			mu.Lock()
			defer mu.Unlock()
			if errors.Is(err, verification.ErrDiditUnavailable) {
				faults++
			} else if err != nil || got.Status != "FAILED" {
				t.Errorf("concurrent read status=%q err=%v", got.Status, err)
			}
		}()
	}
	wg.Wait()
	if faults != 1 || d.marks != 1 || repo.Snapshot() != (nativeReturnFailureSnapshot{Faults: 1, Transitions: 1, Recovered: true}) {
		t.Fatalf("faults=%d marks=%d snapshot=%+v", faults, d.marks, repo.Snapshot())
	}
}

func TestNativeReturnFailureConstructorRequiresTrustedScope(t *testing.T) {
	if _, err := newNativeReturnFailureRepository(nil, "11111111-1111-4111-8111-111111111111", func() nativeDeclarationIntent { return nativeDeclarationIntent{} }); err == nil {
		t.Fatal("nil delegate accepted")
	}
}

func TestNativeReturnFailureRejectsRequestedAttemptMismatch(t *testing.T) {
	repo, d, account, _ := nativeReturnTestRepository(t)
	_, err := repo.GetAttemptForAccount(context.Background(), account, "44444444-4444-4444-8444-444444444444", time.Now(), time.Minute)
	if !errors.Is(err, verification.ErrInvalid) || d.marks != 0 || repo.Snapshot().Faults != 0 {
		t.Fatal("mismatched requested attempt consumed fault or was accepted")
	}
}

func TestNativeReturnFailureRejectsMismatchedTransitionReadback(t *testing.T) {
	repo, d, account, _ := nativeReturnTestRepository(t)
	ctx, now := context.Background(), time.Now()
	id := d.attempt.ID
	_, _ = repo.GetAttemptForAccount(ctx, account, id, now, time.Minute)
	d.afterMark = func(d *nativeReturnFailureDelegate) { d.attempt.AccountID = "44444444-4444-4444-8444-444444444444" }
	_, err := repo.GetAttemptForAccount(ctx, account, id, now, time.Minute)
	if !errors.Is(err, verification.ErrInvalid) || repo.Snapshot().Recovered {
		t.Fatal("mismatched transition readback was accepted")
	}
}
