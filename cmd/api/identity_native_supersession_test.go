package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

// nativeSupersessionRepository bounds a test account to two durable declaration intents.
type nativeSupersessionRepository struct {
	mu        sync.Mutex
	base      verification.DeclarationRevisionRepository
	accountID string
	slots     []*nativeDeclarationIntentRepository
	used      int
}

func newNativeSupersessionRepository(base verification.DeclarationRevisionRepository, dir, accountID string) (*nativeSupersessionRepository, error) {
	if base == nil || !canonicalNativeUUID(accountID) || !filepath.IsAbs(dir) {
		return nil, verification.ErrInvalid
	}
	if err := validateNativeIdentityPrivateDir(dir); err != nil {
		return nil, err
	}
	paths := []string{filepath.Join(dir, "declaration-test-intent.json"), filepath.Join(dir, "declaration-test-intent-2.json")}
	for _, p := range paths {
		if _, err := os.Lstat(p); err == nil {
			return nil, verification.ErrInvalid
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	return &nativeSupersessionRepository{base: base, accountID: accountID, slots: []*nativeDeclarationIntentRepository{{delegate: base, journalPath: paths[0]}, {delegate: base, journalPath: paths[1]}}}, nil
}
func validateNativeIdentityPrivateDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 || !ok || st.Uid != uint32(os.Getuid()) {
		return verification.ErrInvalid
	}
	return nil
}
func (r *nativeSupersessionRepository) trackedRepositories() []*nativeDeclarationIntentRepository {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]*nativeDeclarationIntentRepository(nil), r.slots...)
}
func (r *nativeSupersessionRepository) CreateRevision(ctx context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if in.AccountID != r.accountID {
		return verification.DeclarationRevision{}, identity.ErrForbidden
	}
	if r.used >= len(r.slots) {
		return verification.DeclarationRevision{}, verification.ErrInvalid
	}
	slot := r.slots[r.used]
	// Consume the slot before attempting the write: an uncertain delegate result is never replayed.
	r.used++
	return slot.CreateRevision(ctx, in, seal)
}
func (r *nativeSupersessionRepository) GetRevision(ctx context.Context, accountID, id string) (verification.DeclarationRevision, error) {
	if accountID != r.accountID {
		return verification.DeclarationRevision{}, identity.ErrForbidden
	}
	return r.base.GetRevision(ctx, accountID, id)
}
func (r *nativeSupersessionRepository) GetCurrent(ctx context.Context, accountID string) (verification.DeclarationRevision, error) {
	if accountID != r.accountID {
		return verification.DeclarationRevision{}, identity.ErrForbidden
	}
	return r.base.GetCurrent(ctx, accountID)
}

func TestNativeSupersessionJournalsTwoSequentialWritesAndConsumesUncertainSlot(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const acct = "123e4567-e89b-42d3-a456-426614174000"
	cause := errors.New("uncertain write")
	base := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}, writeErr: cause}
	r, err := newNativeSupersessionRepository(base, dir, acct)
	if err != nil {
		t.Fatal(err)
	}
	for i, in := range []verification.DeclarationRevisionInput{nativeIntentInput(), {ID: "123e4567-e89b-42d3-a456-426614174011", AccountID: acct, CorrelationID: "trusted-correlation-2"}} {
		_, got := r.CreateRevision(context.Background(), in, nil)
		if !errors.Is(got, cause) {
			t.Fatalf("write %d: %v", i, got)
		}
		intent, e := os.ReadFile(filepath.Join(dir, []string{"declaration-test-intent.json", "declaration-test-intent-2.json"}[i]))
		if e != nil {
			t.Fatal(e)
		}
		var parsed nativeDeclarationIntent
		if e = json.Unmarshal(intent, &parsed); e != nil || parsed.AccountID != acct || parsed.DeclarationID != in.ID {
			t.Fatalf("journal not durable before/after failed write: %v", e)
		}
	}
	if _, err = r.CreateRevision(context.Background(), verification.DeclarationRevisionInput{ID: "123e4567-e89b-42d3-a456-426614174012", AccountID: acct, CorrelationID: "third"}, nil); err == nil {
		t.Fatal("third write accepted")
	}
	if base.writes != 2 {
		t.Fatalf("writes=%d", base.writes)
	}
}
func TestNativeSupersessionRejectsUnknownAccountAndTracksExactlyTwo(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const acct = "123e4567-e89b-42d3-a456-426614174000"
	base := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
	r, err := newNativeSupersessionRepository(base, dir, acct)
	if err != nil {
		t.Fatal(err)
	}
	in := nativeIntentInput()
	in.AccountID = "123e4567-e89b-42d3-a456-426614174099"
	if _, err = r.CreateRevision(context.Background(), in, nil); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("err=%v", err)
	}
	if len(r.trackedRepositories()) != 2 || base.writes != 0 {
		t.Fatalf("slots/writes=%d/%d", len(r.trackedRepositories()), base.writes)
	}
}
