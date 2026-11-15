package main

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

const expiryAccount = "11111111-1111-4111-8111-111111111111"
const expiryDeclaration = "22222222-2222-4222-8222-222222222222"
const expiryAttempt = "33333333-3333-4333-8333-333333333333"

type expiryDelegate struct {
	verification.ProviderAttemptRepository
	mu      sync.Mutex
	row     verification.ProviderAttempt
	times   []time.Time
	errAt   int
	calls   int
	wrongID bool
	unknown bool
}

func (d *expiryDelegate) GetAttemptForAccount(_ context.Context, account, id string, now time.Time, ttl time.Duration) (verification.ProviderAttempt, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls++
	d.times = append(d.times, now)
	if d.errAt == d.calls {
		return verification.ProviderAttempt{}, errors.New("delegate read")
	}
	if d.unknown || account != d.row.AccountID || id != d.row.ID || !d.row.RetentionAt.After(now) {
		return verification.ProviderAttempt{}, verification.ErrNotFound
	}
	if (d.row.Status == "CREATED" || d.row.Status == "PENDING_USER_INPUT") && !d.row.CreatedAt.Add(ttl).After(now) {
		d.row.Status = "EXPIRED"
	}
	row := d.row
	if d.wrongID && d.calls == 2 {
		row.ID = "44444444-4444-4444-8444-444444444444"
	}
	return row, nil
}
func expiryFixture(t *testing.T) (*nativeExpiryRepository, *expiryDelegate, time.Time) {
	t.Helper()
	created := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	d := &expiryDelegate{row: verification.ProviderAttempt{ID: expiryAttempt, AccountID: expiryAccount, DeclarationID: expiryDeclaration, CorrelationID: "corr", Status: "PENDING_USER_INPUT", CreatedAt: created, RetentionAt: created.Add(2 * time.Hour)}}
	r, err := newNativeExpiryRepository(d, expiryAccount, func() nativeDeclarationIntent {
		return nativeDeclarationIntent{AccountID: expiryAccount, DeclarationID: expiryDeclaration, CorrelationID: "corr"}
	})
	if err != nil {
		t.Fatal(err)
	}
	return r, d, created
}

type nativeExpirySnapshot struct {
	ClockOverrides int
	Expired        bool
}
type nativeExpiryRepository struct {
	verification.ProviderAttemptRepository
	mu                       sync.Mutex
	accountID                string
	trusted                  func() nativeDeclarationIntent
	attemptID, declarationID string
	overrides                int
	expired                  bool
	uncertain                bool
}

func newNativeExpiryRepository(delegate verification.ProviderAttemptRepository, accountID string, trusted func() nativeDeclarationIntent) (*nativeExpiryRepository, error) {
	if delegate == nil || !canonicalNativeUUID(accountID) || trusted == nil {
		return nil, verification.ErrInvalid
	}
	return &nativeExpiryRepository{ProviderAttemptRepository: delegate, accountID: accountID, trusted: trusted}, nil
}
func (r *nativeExpiryRepository) GetAttemptForAccount(ctx context.Context, accountID, attemptID string, now time.Time, ttl time.Duration) (verification.ProviderAttempt, error) {
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
	if !canonicalNativeUUID(intent.AccountID) || !canonicalNativeUUID(intent.DeclarationID) || intent.CorrelationID == "" || intent.AccountID != r.accountID || attempt.DeclarationID != intent.DeclarationID {
		return attempt, nil
	}
	if r.attemptID == "" {
		r.attemptID, r.declarationID = attemptID, intent.DeclarationID
	} else if r.attemptID != attemptID || r.declarationID != intent.DeclarationID {
		return attempt, nil
	}
	if r.expired {
		if attempt.Status != "EXPIRED" {
			return verification.ProviderAttempt{}, verification.ErrInvalid
		}
		return attempt, nil
	}
	if r.uncertain {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	if (attempt.Status != "CREATED" && attempt.Status != "PENDING_USER_INPUT") || ttl <= 0 {
		return attempt, nil
	}
	deadline := attempt.CreatedAt.Add(ttl)
	if !deadline.Before(attempt.RetentionAt) {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	r.overrides++
	r.uncertain = true
	updated, err := r.ProviderAttemptRepository.GetAttemptForAccount(ctx, accountID, attemptID, deadline, ttl)
	if err != nil {
		return verification.ProviderAttempt{}, err
	}
	if updated.ID != attemptID || updated.AccountID != accountID || updated.DeclarationID != intent.DeclarationID || updated.Status != "EXPIRED" {
		return verification.ProviderAttempt{}, verification.ErrInvalid
	}
	r.expired = true
	r.uncertain = false
	return updated, nil
}
func (r *nativeExpiryRepository) Snapshot() nativeExpirySnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	return nativeExpirySnapshot{ClockOverrides: r.overrides, Expired: r.expired}
}

func TestNativeExpiryUsesExactDeadlineThenPersists(t *testing.T) {
	r, d, created := expiryFixture(t)
	deadline := created.Add(time.Minute)
	callerNow := created
	got, err := r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, callerNow, time.Minute)
	if err != nil || got.Status != "EXPIRED" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	got, err = r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, callerNow.Add(time.Second), time.Minute)
	if err != nil || got.Status != "EXPIRED" {
		t.Fatalf("repeat=%+v err=%v", got, err)
	}
	if len(d.times) != 3 || !d.times[0].Equal(callerNow) || !d.times[1].Equal(deadline) || !d.times[2].Equal(callerNow.Add(time.Second)) {
		t.Fatalf("times=%v", d.times)
	}
	if s := r.Snapshot(); s != (nativeExpirySnapshot{ClockOverrides: 1, Expired: true}) {
		t.Fatalf("snapshot=%+v", s)
	}
}
func TestNativeExpiryDoesNotOverrideUntrustedOrUnknown(t *testing.T) {
	for _, tc := range []struct {
		name    string
		account string
		intent  nativeDeclarationIntent
		missing bool
	}{{"foreign", "44444444-4444-4444-8444-444444444444", nativeDeclarationIntent{expiryAccount, expiryDeclaration, "corr"}, false}, {"untrusted", expiryAccount, nativeDeclarationIntent{expiryAccount, "55555555-5555-4555-8555-555555555555", "corr"}, false}, {"empty correlation", expiryAccount, nativeDeclarationIntent{expiryAccount, expiryDeclaration, ""}, false}, {"unknown", expiryAccount, nativeDeclarationIntent{expiryAccount, expiryDeclaration, "corr"}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			r, d, created := expiryFixture(t)
			if tc.missing {
				d.unknown = true
			}
			r.trusted = func() nativeDeclarationIntent { return tc.intent }
			_, err := r.GetAttemptForAccount(context.Background(), tc.account, expiryAttempt, created, time.Minute)
			if tc.missing && !errors.Is(err, verification.ErrNotFound) {
				t.Fatalf("unknown err=%v", err)
			}
			if r.Snapshot().ClockOverrides != 0 || len(d.times) != 1 {
				t.Fatalf("snapshot=%+v times=%v", r.Snapshot(), d.times)
			}
		})
	}
}
func TestNativeExpiryRejectsRetentionBoundaryAndMismatchedReadback(t *testing.T) {
	t.Run("retention", func(t *testing.T) {
		r, d, created := expiryFixture(t)
		d.row.RetentionAt = created.Add(time.Minute)
		_, err := r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, created, time.Minute)
		if !errors.Is(err, verification.ErrInvalid) || r.Snapshot().ClockOverrides != 0 {
			t.Fatalf("err=%v snap=%+v", err, r.Snapshot())
		}
	})
	t.Run("id", func(t *testing.T) {
		r, d, created := expiryFixture(t)
		d.wrongID = true
		_, err := r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, created, time.Minute)
		if !errors.Is(err, verification.ErrInvalid) {
			t.Fatalf("err=%v", err)
		}
	})
}
func TestNativeExpiryPropagatesDelegateErrors(t *testing.T) {
	r, d, created := expiryFixture(t)
	d.errAt = 1
	_, err := r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, created, time.Minute)
	if err == nil || err.Error() != "delegate read" || r.Snapshot().ClockOverrides != 0 {
		t.Fatalf("err=%v snap=%+v", err, r.Snapshot())
	}
}
func TestNativeExpiryDoesNotRepeatUncertainClockOverride(t *testing.T) {
	r, d, created := expiryFixture(t)
	d.errAt = 2
	_, err := r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, created, time.Minute)
	if err == nil {
		t.Fatal("missing delegate override error")
	}
	d.errAt = 0
	_, err = r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, created, time.Minute)
	if err == nil || d.calls != 3 || r.Snapshot() != (nativeExpirySnapshot{ClockOverrides: 1, Expired: false}) {
		t.Fatal("uncertain override was retried or accepted")
	}
}

func TestNativeExpiryConcurrentReadsOverrideOnce(t *testing.T) {
	r, d, created := expiryFixture(t)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := r.GetAttemptForAccount(context.Background(), expiryAccount, expiryAttempt, created, time.Minute)
			if err != nil || got.Status != "EXPIRED" {
				t.Errorf("got=%+v err=%v", got, err)
			}
		}()
	}
	wg.Wait()
	if r.Snapshot().ClockOverrides != 1 || d.calls != 21 {
		t.Fatalf("snap=%+v calls=%d", r.Snapshot(), d.calls)
	}
}
