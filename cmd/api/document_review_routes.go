package main

import "errors"

// Reviewer access in this cut is explicit, loopback and worker-free. It does
// not enable booking, payment, signature or document-retention workers.
func localDocumentReviewEnabled(getenv func(string) string, profile runtimeProfileName) (bool, error) {
	value := getenv("LOCAL_DOCUMENT_REVIEW_TRIAL")
	if value == "" {
		return false, nil
	}
	if value != "1" || profile != runtimeProfileWorkerFree || getenv("LOCAL_BOOKING_TRIAL") != "" {
		return false, errors.New("document review trial requires the standalone local worker-free profile")
	}
	return true, nil
}
