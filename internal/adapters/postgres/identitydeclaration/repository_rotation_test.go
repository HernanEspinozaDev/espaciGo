package identitydeclaration

import (
	"testing"
	"time"
)

func TestValidInputRejectsMalformedRutCandidatesBeforeDatabase(t *testing.T) {
	valid := RevisionInput{
		ID: "85000000-0000-4000-8000-000000000002", AccountID: "85000000-0000-4000-8000-000000000001",
		CorrelationID: "synthetic-correlation", RutFingerprint: "v2:" + stringsRepeat("b", 64),
		RutFingerprintCandidates: []string{"v1:" + stringsRepeat("a", 64)},
		SchemaVersion:            "1", ConsentVersion: "consent-v1", ConsentGivenAt: time.Now().UTC(), RetentionAt: time.Now().UTC().Add(time.Hour),
	}
	seal := Sealer(func(int64) (string, []byte, error) { return "synthetic-key", make([]byte, 32), nil })
	if validInput(valid, seal) {
		t.Fatal("candidate set excluding active fingerprint was accepted")
	}
	valid.RutFingerprintCandidates = []string{valid.RutFingerprint, valid.RutFingerprint, "v1:" + stringsRepeat("a", 64)}
	if validInput(valid, seal) {
		t.Fatal("duplicate candidate versions were accepted")
	}
	valid.RutFingerprintCandidates = []string{"not-a-fingerprint"}
	if validInput(valid, seal) {
		t.Fatal("malformed candidate was accepted")
	}
}

func TestValidInputAcceptsLegacyFingerprintWithoutCandidates(t *testing.T) {
	in := RevisionInput{
		ID: "85000000-0000-4000-8000-000000000002", AccountID: "85000000-0000-4000-8000-000000000001",
		CorrelationID: "synthetic-correlation", RutFingerprint: "v1:" + stringsRepeat("a", 64),
		SchemaVersion: "1", ConsentVersion: "consent-v1", ConsentGivenAt: time.Now().UTC(), RetentionAt: time.Now().UTC().Add(time.Hour),
	}
	if !validInput(in, Sealer(func(int64) (string, []byte, error) { return "synthetic-key", make([]byte, 32), nil })) {
		t.Fatal("legacy fingerprint input rejected")
	}
}
