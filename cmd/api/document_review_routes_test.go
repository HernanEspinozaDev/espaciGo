package main

import "testing"

func TestDocumentReviewRequiresExplicitWorkerFreeLocalScope(t *testing.T) {
	for _, tc := range []struct {
		value, booking string
		profile        runtimeProfileName
		want           bool
	}{
		{"1", "", runtimeProfileWorkerFree, true},
		{"1", "1", runtimeProfileWorkerFree, false},
		{"1", "", runtimeProfileStandard, false},
		{"1", "", runtimeProfileDocumentScanOnly, false},
		{"yes", "", runtimeProfileWorkerFree, false},
	} {
		env := map[string]string{"LOCAL_DOCUMENT_REVIEW_TRIAL": tc.value, "LOCAL_BOOKING_TRIAL": tc.booking}
		got, err := localDocumentReviewEnabled(func(k string) string { return env[k] }, tc.profile)
		if tc.want != (got && err == nil) {
			t.Fatalf("unexpected reviewer profile gate for %s", tc.profile)
		}
	}
	got, err := localDocumentReviewEnabled(func(string) string { return "" }, runtimeProfileWorkerFree)
	if got || err != nil {
		t.Fatal("review enabled without explicit authorization flag")
	}
}
