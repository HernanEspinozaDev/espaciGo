package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

type nativeAttemptIntent struct {
	AccountID       string `json:"account_id"`
	DeclarationID   string `json:"declaration_id"`
	AttemptID       string `json:"attempt_id"`
	CorrelationID   string `json:"correlation_id"`
	Provider        string `json:"provider"`
	Environment     string `json:"environment"`
	ApplicationID   string `json:"application_id"`
	WorkflowID      string `json:"workflow_id"`
	WorkflowVersion string `json:"workflow_version"`
}

type nativeAttemptJournalRepository struct {
	verification.ProviderAttemptRepository
	mu                sync.Mutex
	path              string
	accountID         string
	trusted           func() nativeDeclarationIntent
	saved             nativeAttemptIntent
	written           bool
	beforeCommit      func(context.Context, verification.CommitWebhookDecisionInput, nativeAttemptIntent) error
	commitError       func(error)
	retentionObserved bool
}

func newNativeAttemptJournalRepository(delegate verification.ProviderAttemptRepository, path string, accountID string, trusted func() nativeDeclarationIntent) (*nativeAttemptJournalRepository, error) {
	if delegate == nil || trusted == nil || !canonicalNativeUUID(accountID) || !filepath.IsAbs(path) {
		return nil, verification.ErrInvalid
	}
	if _, err := os.Lstat(path); err == nil {
		return nil, verification.ErrInvalid
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return &nativeAttemptJournalRepository{ProviderAttemptRepository: delegate, path: path, accountID: accountID, trusted: trusted}, nil
}

func (r *nativeAttemptJournalRepository) PrepareAttempt(ctx context.Context, in verification.PrepareAttemptInput) (verification.ProviderAttempt, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	invalid := func() (verification.ProviderAttempt, bool, error) {
		return verification.ProviderAttempt{}, false, verification.ErrInvalid
	}
	if r.written || in.AccountID != r.accountID || in.Provider != "didit" || in.Environment != "SANDBOX" || in.ApplicationID != "local-kyc-test-app" || in.WorkflowID != "local-kyc-test-workflow" || in.WorkflowVersion != "1" || !canonicalNativeUUID(in.AccountID) || !canonicalNativeUUID(in.DeclarationID) || !canonicalNativeUUID(in.ID) || !canonicalNativeUUID(in.CorrelationID) {
		return invalid()
	}
	declaration := r.trusted()
	if declaration.AccountID != in.AccountID || declaration.DeclarationID != in.DeclarationID || declaration.CorrelationID == "" {
		return invalid()
	}
	intent := nativeAttemptIntent{in.AccountID, in.DeclarationID, in.ID, in.CorrelationID, in.Provider, in.Environment, in.ApplicationID, in.WorkflowID, in.WorkflowVersion}
	if err := persistNativeAttemptIntent(r.path, intent); err != nil {
		return verification.ProviderAttempt{}, false, err
	}
	r.saved, r.written = intent, true
	attempt, reused, err := r.ProviderAttemptRepository.PrepareAttempt(ctx, in)
	if err != nil {
		return attempt, reused, err
	}
	if reused || attempt.ID != in.ID || attempt.AccountID != in.AccountID || attempt.DeclarationID != in.DeclarationID || attempt.CorrelationID != in.CorrelationID || attempt.Provider != in.Provider || attempt.Environment != in.Environment || attempt.ApplicationID != in.ApplicationID || attempt.WorkflowID != in.WorkflowID || attempt.WorkflowVersion != in.WorkflowVersion {
		return verification.ProviderAttempt{}, false, verification.ErrInvalid
	}
	return attempt, false, nil
}

func (r *nativeAttemptJournalRepository) Snapshot() (nativeAttemptIntent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saved, r.written
}

func (r *nativeAttemptJournalRepository) CommitWebhookDecision(ctx context.Context, in verification.CommitWebhookDecisionInput) (verification.ProviderAttempt, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.beforeCommit != nil {
		if !r.written || in.AttemptID != r.saved.AttemptID {
			return verification.ProviderAttempt{}, verification.ErrInvalid
		}
		if err := r.beforeCommit(ctx, in, r.saved); err != nil {
			return verification.ProviderAttempt{}, err
		}
		r.retentionObserved = true
	}
	out, err := r.ProviderAttemptRepository.CommitWebhookDecision(ctx, in)
	if err != nil && r.commitError != nil {
		r.commitError(err)
	}
	return out, err
}

func (r *nativeAttemptJournalRepository) RetentionObserved() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.retentionObserved
}

func persistNativeAttemptIntent(path string, intent nativeAttemptIntent) error {
	dir := filepath.Dir(path)
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 || !ok || owner.Uid != uint32(os.Getuid()) {
		return verification.ErrInvalid
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if err = f.Chmod(0600); err == nil {
		err = json.NewEncoder(f).Encode(intent)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func readNativeAttemptIntent(path string) (nativeAttemptIntent, error) {
	var got nativeAttemptIntent
	if err := validateNativeIdentityPrivateFile(path); err != nil {
		return got, err
	}
	f, err := os.Open(path)
	if err != nil {
		return got, err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		return got, err
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nativeAttemptIntent{}, fmt.Errorf("invalid trailing journal data")
	}
	encoded, err := json.Marshal(got)
	if err != nil {
		return nativeAttemptIntent{}, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &raw); err != nil || len(raw) != 9 {
		return nativeAttemptIntent{}, verification.ErrInvalid
	}
	if !canonicalNativeUUID(got.AccountID) || !canonicalNativeUUID(got.DeclarationID) || !canonicalNativeUUID(got.AttemptID) || !canonicalNativeUUID(got.CorrelationID) || got.Provider != "didit" || got.Environment != "SANDBOX" || got.ApplicationID != "local-kyc-test-app" || got.WorkflowID != "local-kyc-test-workflow" || got.WorkflowVersion != "1" || strings.TrimSpace(got.AccountID) != got.AccountID {
		return nativeAttemptIntent{}, verification.ErrInvalid
	}
	return got, nil
}

// The behavior tests follow below; a deliberately narrow fake proves the durable boundary.
type nativeAttemptJournalFake struct {
	verification.ProviderAttemptRepository
	in     verification.PrepareAttemptInput
	row    verification.ProviderAttempt
	reused bool
	err    error
	before func() error
	calls  int
}

func (f *nativeAttemptJournalFake) PrepareAttempt(_ context.Context, in verification.PrepareAttemptInput) (verification.ProviderAttempt, bool, error) {
	f.calls++
	f.in = in
	if f.before != nil {
		if err := f.before(); err != nil {
			return verification.ProviderAttempt{}, false, err
		}
	}
	return f.row, f.reused, f.err
}
func nativeAttemptJournalFixture(t *testing.T) (string, string, nativeDeclarationIntent) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	account := "123e4567-e89b-42d3-a456-426614174000"
	decl := nativeDeclarationIntent{AccountID: account, DeclarationID: "123e4567-e89b-42d3-a456-426614174001", CorrelationID: "123e4567-e89b-42d3-a456-426614174003"}
	return filepath.Join(dir, "attempt.json"), account, decl
}
func nativeAttemptJournalInput(d nativeDeclarationIntent) verification.PrepareAttemptInput {
	// Production generates a new correlation for the provider attempt; it is not
	// the correlation of the earlier declaration-save operation.
	return verification.PrepareAttemptInput{ID: "123e4567-e89b-42d3-a456-426614174002", AccountID: d.AccountID, DeclarationID: d.DeclarationID, CorrelationID: "123e4567-e89b-42d3-a456-426614174004", Provider: "didit", Environment: "SANDBOX", ApplicationID: "local-kyc-test-app", WorkflowID: "local-kyc-test-workflow", WorkflowVersion: "1"}
}
func nativeAttemptJournalRow(in verification.PrepareAttemptInput) verification.ProviderAttempt {
	return verification.ProviderAttempt{ID: in.ID, AccountID: in.AccountID, DeclarationID: in.DeclarationID, CorrelationID: in.CorrelationID, Provider: in.Provider, Environment: in.Environment, ApplicationID: in.ApplicationID, WorkflowID: in.WorkflowID, WorkflowVersion: in.WorkflowVersion}
}
func TestNativeAttemptJournalPersistsBeforeDelegateAndReadback(t *testing.T) {
	path, account, d := nativeAttemptJournalFixture(t)
	fake := &nativeAttemptJournalFake{}
	in := nativeAttemptJournalInput(d)
	fake.row = nativeAttemptJournalRow(in)
	r, err := newNativeAttemptJournalRepository(fake, path, account, func() nativeDeclarationIntent { return d })
	if err != nil {
		t.Fatal(err)
	}
	fake.before = func() error {
		got, e := readNativeAttemptIntent(path)
		if e != nil {
			return e
		}
		if got.AttemptID != in.ID {
			return errors.New("journal absent before delegate")
		}
		return nil
	}
	if _, reused, err := r.PrepareAttempt(context.Background(), in); err != nil || reused {
		t.Fatalf("reused=%v err=%v", reused, err)
	}
	got, ok := r.Snapshot()
	if !ok || got.AttemptID != in.ID {
		t.Fatalf("snapshot=%+v %v", got, ok)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v err=%v", info, err)
	}
}
func TestNativeAttemptJournalRejectsDeclarationMismatchBeforeDelegate(t *testing.T) {
	path, a, d := nativeAttemptJournalFixture(t)
	fake := &nativeAttemptJournalFake{}
	r, e := newNativeAttemptJournalRepository(fake, path, a, func() nativeDeclarationIntent { return d })
	if e != nil {
		t.Fatal(e)
	}
	in := nativeAttemptJournalInput(d)
	in.DeclarationID = "123e4567-e89b-42d3-a456-426614174009"
	if _, _, e = r.PrepareAttempt(context.Background(), in); !errors.Is(e, verification.ErrInvalid) {
		t.Fatalf("err=%v", e)
	}
	if fake.calls != 0 {
		t.Fatalf("delegate calls=%d", fake.calls)
	}
	if _, e = os.Lstat(path); !errors.Is(e, os.ErrNotExist) {
		t.Fatalf("journal created: %v", e)
	}
}
func TestNativeAttemptJournalRejectsOccupiedJournalAndUnsafeParent(t *testing.T) {
	for _, unsafe := range []bool{false, true} {
		t.Run(fmt.Sprint(unsafe), func(t *testing.T) {
			path, a, d := nativeAttemptJournalFixture(t)
			if unsafe {
				dir := filepath.Dir(path)
				if e := os.Chmod(dir, 0755); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := os.WriteFile(path, []byte("sentinel"), 0600); e != nil {
					t.Fatal(e)
				}
			}
			fake := &nativeAttemptJournalFake{}
			r, e := newNativeAttemptJournalRepository(fake, path, a, func() nativeDeclarationIntent { return d })
			if e == nil {
				in := nativeAttemptJournalInput(d)
				_, _, e = r.PrepareAttempt(context.Background(), in)
			}
			if e == nil || fake.calls != 0 {
				t.Fatalf("err=%v calls=%d", e, fake.calls)
			}
		})
	}
}
func TestNativeAttemptJournalRejectsUnknownAccount(t *testing.T) {
	path, a, d := nativeAttemptJournalFixture(t)
	fake := &nativeAttemptJournalFake{}
	r, e := newNativeAttemptJournalRepository(fake, path, a, func() nativeDeclarationIntent { return d })
	if e != nil {
		t.Fatal(e)
	}
	in := nativeAttemptJournalInput(d)
	in.AccountID = "123e4567-e89b-42d3-a456-426614174099"
	if _, _, e = r.PrepareAttempt(context.Background(), in); !errors.Is(e, verification.ErrInvalid) || fake.calls != 0 {
		t.Fatalf("err=%v calls=%d", e, fake.calls)
	}
}
func TestNativeAttemptJournalRetainsIntentOnDelegateErrorAndRejectsBadRows(t *testing.T) {
	for _, tc := range []string{"delegate error", "mismatched row", "reused row"} {
		t.Run(tc, func(t *testing.T) {
			path, a, d := nativeAttemptJournalFixture(t)
			fake := &nativeAttemptJournalFake{}
			in := nativeAttemptJournalInput(d)
			fake.row = nativeAttemptJournalRow(in)
			cause := errors.New("uncertain delegate failure")
			switch tc {
			case "delegate error":
				fake.err = cause
			case "mismatched row":
				fake.row.AccountID = "123e4567-e89b-42d3-a456-426614174099"
			case "reused row":
				fake.reused = true
			}
			r, e := newNativeAttemptJournalRepository(fake, path, a, func() nativeDeclarationIntent { return d })
			if e != nil {
				t.Fatal(e)
			}
			_, _, e = r.PrepareAttempt(context.Background(), in)
			if tc == "delegate error" && !errors.Is(e, cause) {
				t.Fatalf("err=%v", e)
			}
			if tc != "delegate error" && !errors.Is(e, verification.ErrInvalid) {
				t.Fatalf("err=%v", e)
			}
			if _, e = readNativeAttemptIntent(path); e != nil {
				t.Fatalf("journal not retained: %v", e)
			}
		})
	}
}
