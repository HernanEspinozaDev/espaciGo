package verification

import (
	"testing"
	"time"
)

func TestDeclarationRutKeyringIsSharedImmutableSnapshot(t *testing.T) {
	repo := &declarationMemoryRepository{rows: map[string]DeclarationRevision{}}
	svc, _ := newDeclarationServiceFixture(t, repo, func(at time.Time) time.Time { return at.Add(30 * 24 * time.Hour) })
	if svc.RutKeyring() != svc.rutKeyring || svc.RutKeyring() == nil {
		t.Fatal("runtime cannot reuse the declaration's exact keyring snapshot")
	}
	var absent *DeclarationService
	if absent.RutKeyring() != nil {
		t.Fatal("nil declaration service exposed a keyring")
	}
}
