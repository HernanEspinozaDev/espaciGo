package verification

import (
	"context"
	"strings"
	"time"
)

func (r *MemoryRutClaimRegistry) ClaimCandidates(ctx context.Context, accountID, declarationID string, set RutFingerprintSet, expiresAt, now time.Time) (string, error) {
	return r.claimMemory(ctx, accountID, declarationID, set, expiresAt, now, true)
}

// Complete the read phase before changing any entry, just like the durable
// adapter. A configured version covers every identity using that version,
// not only the fingerprints of the RUT being checked in this call.
func (r *MemoryRutClaimRegistry) canonicalMemoryLocked(set RutFingerprintSet, checkVersions bool) (string, rutClaimEntry, bool, error) {
	if checkVersions {
		versions := map[string]bool{}
		for _, fp := range set.Candidates {
			version, _, _ := strings.Cut(fp, ":")
			versions[version] = true
		}
		for fp := range r.byDigest {
			version, _, _ := strings.Cut(fp, ":")
			if !versions[version] {
				return "", rutClaimEntry{}, false, ErrRutHMACKeyUnavailable
			}
		}
	}
	var canonical string
	var entry rutClaimEntry
	found := false
	for _, fp := range set.Candidates {
		if candidate, ok := r.byDigest[fp]; ok {
			if found || candidate.Verified != candidate.ExpiresAt.IsZero() {
				return "", rutClaimEntry{}, false, ErrConflict
			}
			canonical, entry, found = fp, candidate, true
		}
	}
	return canonical, entry, found, nil
}

func (r *MemoryRutClaimRegistry) claimMemory(ctx context.Context, accountID, declarationID string, set RutFingerprintSet, expiresAt, now time.Time, checkVersions bool) (string, error) {
	if r == nil || ctx == nil || !canonicalDeclarationUUID(accountID) || !canonicalDeclarationUUID(declarationID) || ValidateRutFingerprintSet(set) != nil || now.IsZero() || !expiresAt.After(now) {
		return "", ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	fp, entry, found, err := r.canonicalMemoryLocked(set, checkVersions)
	if err != nil {
		return "", err
	}
	expired := found && !entry.Verified && !entry.ExpiresAt.After(now)
	if found && !expired && entry.AccountID != accountID {
		return "", ErrConflict
	}
	canonical := fp
	if !found || expired {
		canonical = set.Active
	}
	for key, other := range r.byDigest {
		if other.AccountID == accountID && key != canonical && (other.Verified || other.ExpiresAt.IsZero()) {
			return "", ErrConflict
		}
	}
	if expired {
		delete(r.byDigest, fp)
	}
	for key, other := range r.byDigest {
		if other.AccountID == accountID && key != canonical && !other.Verified {
			delete(r.byDigest, key)
		}
	}
	verified := found && !expired && entry.Verified
	if verified {
		expiresAt = time.Time{}
	}
	if r.byDigest == nil {
		r.byDigest = make(map[string]rutClaimEntry)
	}
	_, digest, _ := strings.Cut(canonical, ":")
	r.byDigest[canonical] = rutClaimEntry{AccountID: accountID, DeclarationID: declarationID, RutFingerprint: canonical, DigestHex: digest, ExpiresAt: expiresAt.UTC(), Verified: verified}
	return canonical, nil
}

func (r *MemoryRutClaimRegistry) ResolveCandidates(ctx context.Context, accountID, declarationID string, set RutFingerprintSet) (string, error) {
	if r == nil || ctx == nil || !canonicalDeclarationUUID(accountID) || !canonicalDeclarationUUID(declarationID) || ValidateRutFingerprintSet(set) != nil {
		return "", ErrInvalid
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	fp, entry, found, err := r.canonicalMemoryLocked(set, true)
	if err != nil {
		return "", err
	}
	if !found || entry.AccountID != accountID || entry.DeclarationID != declarationID {
		return "", ErrConflict
	}
	return fp, nil
}
