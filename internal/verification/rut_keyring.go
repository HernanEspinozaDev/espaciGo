package verification

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// RutFingerprintSet contains one digest per configured key, with Active identifying the current write key.
type RutFingerprintSet struct {
	Active     string
	Candidates []string
}

// RutHMACKeyring is an immutable snapshot of versioned RUT HMAC keys.
type RutHMACKeyring struct {
	active   string
	keys     map[string][]byte
	versions []string
}

var rutHMACVersionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,31}$`)
var rutFingerprintPattern = regexp.MustCompile(`^([A-Za-z0-9][A-Za-z0-9._-]{0,31}):([0-9a-f]{64})$`)

func NewRutHMACKeyring(active string, keys map[string][]byte) (*RutHMACKeyring, error) {
	if len(keys) == 0 || len(keys) > 64 || !rutHMACVersionPattern.MatchString(active) {
		return nil, fmt.Errorf("invalid RUT HMAC keyring configuration")
	}
	if _, ok := keys[active]; !ok {
		return nil, fmt.Errorf("invalid RUT HMAC keyring configuration")
	}
	copied := make(map[string][]byte, len(keys))
	versions := make([]string, 0, len(keys))
	for version, secret := range keys {
		if !rutHMACVersionPattern.MatchString(version) || len(secret) < 16 {
			return nil, fmt.Errorf("invalid RUT HMAC keyring configuration")
		}
		copied[version] = append([]byte(nil), secret...)
		versions = append(versions, version)
	}
	sort.Strings(versions)
	return &RutHMACKeyring{active: active, keys: copied, versions: versions}, nil
}

func (k *RutHMACKeyring) Fingerprints(rut string) (RutFingerprintSet, error) {
	if k == nil || len(k.keys) == 0 || len(k.keys) != len(k.versions) || k.active == "" {
		return RutFingerprintSet{}, ErrInvalid
	}
	set := RutFingerprintSet{Candidates: make([]string, 0, len(k.versions))}
	for _, version := range k.versions {
		fingerprint, err := ComputeRutFingerprint(rut, k.keys[version], version)
		if err != nil {
			return RutFingerprintSet{}, err
		}
		set.Candidates = append(set.Candidates, fingerprint)
		if version == k.active {
			set.Active = fingerprint
		}
	}
	return set, nil
}

func (k *RutHMACKeyring) Versions() []string {
	if k == nil {
		return nil
	}
	return append([]string(nil), k.versions...)
}

func ValidateRutFingerprintSet(set RutFingerprintSet) error {
	activeMatch := rutFingerprintPattern.FindStringSubmatch(set.Active)
	if activeMatch == nil || len(set.Candidates) == 0 || len(set.Candidates) > 64 {
		return fmt.Errorf("invalid RUT fingerprint set")
	}
	seen := make(map[string]struct{}, len(set.Candidates))
	activePresent := false
	previous := ""
	for _, fingerprint := range set.Candidates {
		match := rutFingerprintPattern.FindStringSubmatch(fingerprint)
		if match == nil {
			return fmt.Errorf("invalid RUT fingerprint set")
		}
		version := match[1]
		if _, exists := seen[version]; exists {
			return fmt.Errorf("invalid RUT fingerprint set")
		}
		seen[version] = struct{}{}
		if previous != "" && strings.Compare(previous, version) >= 0 {
			return fmt.Errorf("invalid RUT fingerprint set")
		}
		previous = version
		if fingerprint == set.Active {
			activePresent = true
		}
	}
	if !activePresent || !rutHMACVersionPattern.MatchString(activeMatch[1]) {
		return fmt.Errorf("invalid RUT fingerprint set")
	}
	return nil
}
