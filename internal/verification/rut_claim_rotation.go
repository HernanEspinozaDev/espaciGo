package verification

import (
	"context"
	"errors"
	"time"
)

// ErrRutHMACKeyUnavailable rejects writes when a retained historical claim
// depends on a key absent from the configured keyring. It is an operational
// configuration failure, not invalid identity input.
var ErrRutHMACKeyUnavailable = errors.New("identity fingerprint key configuration is incomplete")

// RotatingRutClaimStore resolves every configured key version to one durable
// canonical claim. The database, not a process-local cache, owns that decision.
// Legacy interfaces remain for old test adapters; rotated runtime requires this
// additional contract and must fail closed when it is unavailable.
type RotatingRutClaimStore interface {
	ClaimCandidates(context.Context, string, string, RutFingerprintSet, time.Time, time.Time) (string, error)
	ResolveCandidates(context.Context, string, string, RutFingerprintSet) (string, error)
}
