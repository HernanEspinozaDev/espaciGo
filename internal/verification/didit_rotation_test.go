package verification

import (
	"context"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
)

// This store intentionally implements only the historical interface.
type diditRotationLegacyClaims struct{}

func (diditRotationLegacyClaims) Claim(context.Context, string, string, string, time.Time, time.Time) error {
	return nil
}
func (diditRotationLegacyClaims) MarkVerified(context.Context, string, string, string, time.Time) error {
	return nil
}
func (diditRotationLegacyClaims) RevokeVerified(context.Context, string, time.Time) {}

func TestNewDiditServiceRejectsKeyringWithoutRotatingStore(t *testing.T) {
	ids := &counterUUIDs{}
	now := func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
	declarationKeys := newTestKeyring(t)
	declarations, err := NewDeclarationService(declRepo, declarationKeys, ids, now, func(at time.Time) time.Time { return at.Add(24 * time.Hour) }, "consent-v1")
	if err != nil {
		t.Fatal(err)
	}
	decisionKeys, err := declarationcrypto.New("v1", map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	rutKeys, err := NewRutHMACKeyring("v2", map[string][]byte{"v1": []byte("0123456789abcdef"), "v2": []byte("abcdef0123456789")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = NewDiditService(newMemProviderAttemptRepo(), declarations, &fakeDiditClient{}, decisionKeys, ids, now, DiditServiceConfig{
		WorkflowID: "workflow", ConsentVersion: "consent-v1", RutHMACKeyring: rutKeys,
	}, diditRotationLegacyClaims{})
	if err != ErrInvalid {
		t.Fatalf("NewDiditService error = %v, want ErrInvalid", err)
	}
}
