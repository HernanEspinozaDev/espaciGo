package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

// nativeDeclarationIntentPairRepository is a deliberately bounded test helper:
// each authorized account receives one independent, exclusive recovery journal.
type nativeDeclarationIntentPairRepository struct {
	mu                 sync.Mutex
	slotA, slotB       *nativeDeclarationIntentRepository
	accountA, accountB string
}

func newNativeDeclarationIntentPairRepository(delegate verification.DeclarationRevisionRepository, journalDir, accountA, accountB string) (*nativeDeclarationIntentPairRepository, error) {
	if delegate == nil {
		return nil, errors.New("native declaration pair delegate is nil")
	}
	if !canonicalNativeUUID(accountA) || !canonicalNativeUUID(accountB) || accountA == accountB {
		return nil, errors.New("native declaration pair account IDs must be distinct canonical UUIDs")
	}
	if !filepath.IsAbs(journalDir) {
		return nil, errors.New("native declaration pair journal directory must be absolute")
	}
	info, err := os.Lstat(journalDir)
	if err != nil {
		return nil, fmt.Errorf("inspect native declaration pair journal directory: %w", err)
	}
	owner, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0700 || !ok || owner.Uid != uint32(os.Getuid()) {
		return nil, errors.New("native declaration pair journal directory must be owned mode 0700 and not a symlink")
	}
	paths := []string{filepath.Join(journalDir, "declaration-test-intent.json"), filepath.Join(journalDir, "declaration-test-intent-2.json")}
	for _, path := range paths {
		if _, err := os.Lstat(path); err == nil {
			return nil, fmt.Errorf("native declaration pair journal already exists: %s", filepath.Base(path))
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("inspect native declaration pair journal: %w", err)
		}
	}
	return &nativeDeclarationIntentPairRepository{
		slotA:    &nativeDeclarationIntentRepository{delegate: delegate, journalPath: paths[0]},
		slotB:    &nativeDeclarationIntentRepository{delegate: delegate, journalPath: paths[1]},
		accountA: accountA, accountB: accountB,
	}, nil
}

func (r *nativeDeclarationIntentPairRepository) trackedRepositories() []*nativeDeclarationIntentRepository {
	r.mu.Lock()
	defer r.mu.Unlock()
	return []*nativeDeclarationIntentRepository{r.slotA, r.slotB}
}

func (r *nativeDeclarationIntentPairRepository) selected(accountID string) (*nativeDeclarationIntentRepository, error) {
	switch accountID {
	case r.accountA:
		return r.slotA, nil
	case r.accountB:
		return r.slotB, nil
	default:
		return nil, identity.ErrForbidden
	}
}

func (r *nativeDeclarationIntentPairRepository) CreateRevision(ctx context.Context, in verification.DeclarationRevisionInput, seal verification.DeclarationSealer) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tracked, err := r.selected(in.AccountID)
	if err != nil {
		return verification.DeclarationRevision{}, err
	}
	return tracked.CreateRevision(ctx, in, seal)
}
func (r *nativeDeclarationIntentPairRepository) GetRevision(ctx context.Context, accountID, id string) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tracked, err := r.selected(accountID)
	if err != nil {
		return verification.DeclarationRevision{}, err
	}
	return tracked.GetRevision(ctx, accountID, id)
}
func (r *nativeDeclarationIntentPairRepository) GetCurrent(ctx context.Context, accountID string) (verification.DeclarationRevision, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	tracked, err := r.selected(accountID)
	if err != nil {
		return verification.DeclarationRevision{}, err
	}
	return tracked.GetCurrent(ctx, accountID)
}

func TestNativeDeclarationIntentPairJournalsEachAccountBeforeDelegateWrite(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const a = "123e4567-e89b-42d3-a456-426614174000"
	const b = "123e4567-e89b-42d3-a456-426614174010"
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
	pair, err := newNativeDeclarationIntentPairRepository(repo, dir, a, b)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(dir, "declaration-test-intent.json"), filepath.Join(dir, "declaration-test-intent-2.json")}
	inputs := []verification.DeclarationRevisionInput{nativeIntentInput(), {ID: "123e4567-e89b-42d3-a456-426614174011", AccountID: b, CorrelationID: "trusted-correlation-b"}}
	for i, in := range inputs {
		repo.writeErr = nil
		if i == 1 {
			repo.writeErr = verification.ErrConflict
		}
		repo.beforeWrite = func() error {
			data, e := os.ReadFile(paths[i])
			if e != nil {
				return e
			}
			var intent nativeDeclarationIntent
			if e = json.Unmarshal(data, &intent); e != nil {
				return e
			}
			if intent.AccountID != in.AccountID || intent.DeclarationID != in.ID || intent.CorrelationID != in.CorrelationID {
				return errors.New("wrong account journal at delegate boundary")
			}
			return nil
		}
		_, gotErr := pair.CreateRevision(context.Background(), in, nil)
		if !errors.Is(gotErr, repo.writeErr) {
			t.Fatalf("create error=%v", gotErr)
		}
	}
	if repo.writes != 2 {
		t.Fatalf("delegate writes=%d", repo.writes)
	}
	tracked := pair.trackedRepositories()
	if len(tracked) != 2 || tracked[0].journalPath != paths[0] || tracked[1].journalPath != paths[1] {
		t.Fatalf("tracked slots=%+v", tracked)
	}
}

func TestNativeDeclarationIntentPairRejectsUnknownAndPreservesOccupiedJournals(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "private")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	const a = "123e4567-e89b-42d3-a456-426614174000"
	const b = "123e4567-e89b-42d3-a456-426614174010"
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
	pair, err := newNativeDeclarationIntentPairRepository(repo, dir, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pair.GetCurrent(context.Background(), "123e4567-e89b-42d3-a456-426614174099"); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("unknown account error=%v", err)
	}
	unknown := nativeIntentInput()
	unknown.AccountID = "123e4567-e89b-42d3-a456-426614174099"
	if _, err = pair.CreateRevision(context.Background(), unknown, nil); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("unknown account create error=%v", err)
	}
	if _, err = pair.GetRevision(context.Background(), unknown.AccountID, unknown.ID); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("unknown account revision error=%v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 0 {
		t.Fatal("unknown account created a journal or prevented directory inspection")
	}
	if repo.writes != 0 {
		t.Fatalf("unknown account caused %d writes", repo.writes)
	}
	if err = os.WriteFile(filepath.Join(dir, "declaration-test-intent-2.json"), []byte("sentinel"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = newNativeDeclarationIntentPairRepository(repo, dir, a, b); err == nil {
		t.Fatal("occupied journal accepted")
	}
	data, err := os.ReadFile(filepath.Join(dir, "declaration-test-intent-2.json"))
	if err != nil || string(data) != "sentinel" {
		t.Fatalf("occupied journal changed: %q %v", data, err)
	}
}

func TestNativeDeclarationIntentPairConstructorRejectsUnsafeInputs(t *testing.T) {
	const a = "123e4567-e89b-42d3-a456-426614174000"
	const b = "123e4567-e89b-42d3-a456-426614174010"
	dir := t.TempDir()
	repo := &intentBoundaryRepo{rows: map[string]verification.DeclarationRevision{}}
	for name, call := range map[string]func() error{
		"nil delegate":       func() error { _, e := newNativeDeclarationIntentPairRepository(nil, dir, a, b); return e },
		"duplicate IDs":      func() error { _, e := newNativeDeclarationIntentPairRepository(repo, dir, a, a); return e },
		"malformed ID":       func() error { _, e := newNativeDeclarationIntentPairRepository(repo, dir, a, "B"); return e },
		"relative directory": func() error { _, e := newNativeDeclarationIntentPairRepository(repo, "relative", a, b); return e },
		"symlink directory": func() error {
			link := filepath.Join(t.TempDir(), "link")
			if e := os.Symlink(dir, link); e != nil {
				return e
			}
			_, e := newNativeDeclarationIntentPairRepository(repo, link, a, b)
			return e
		},
		"unsafe permissions": func() error {
			unsafe := filepath.Join(t.TempDir(), "unsafe")
			if e := os.Mkdir(unsafe, 0755); e != nil {
				return e
			}
			_, e := newNativeDeclarationIntentPairRepository(repo, unsafe, a, b)
			return e
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); err == nil {
				t.Fatal("unsafe constructor input accepted")
			}
		})
	}
}
