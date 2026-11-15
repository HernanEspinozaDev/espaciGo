package main

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"testing"
)

type privacyLogDeclarationPurger struct {
	err   error
	calls int
}

func (p *privacyLogDeclarationPurger) PurgeExpiredRevisions(context.Context) (int64, error) {
	p.calls++
	return 0, p.err
}

type privacyLogOutboxPurger struct {
	err   error
	calls int
}

func (p *privacyLogOutboxPurger) PurgeDeliveredSessionOutbox(context.Context) (int64, error) {
	p.calls++
	return 0, p.err
}

func TestRetentionCycleDoesNotLogSyntheticPersonalIdentifiers(t *testing.T) {
	ownerEmail := "owner@example.invalid"
	accountID := "77777777-0000-4000-8000-000000000001"
	requestID := "77777777-0000-4000-8000-000000000002"
	evidenceID := "77777777-0000-4000-8000-000000000003"
	first := errors.New("declaration purge failed owner=" + ownerEmail + " account=" + accountID + " request=" + requestID)
	second := errors.New("outbox purge failed evidence=" + evidenceID + " owner=" + ownerEmail)
	d := &privacyLogDeclarationPurger{err: first}
	a := &privacyLogOutboxPurger{err: second}
	var out bytes.Buffer

	runIdentityRetentionCycle(context.Background(), "espacigo_privacy_cleanup_worker", d, a, log.New(&out, "", 0))

	if d.calls != 1 || a.calls != 1 {
		t.Fatalf("purger calls: declaration=%d outbox=%d, want one each", d.calls, a.calls)
	}
	logged := out.String()
	for _, marker := range []string{ownerEmail, accountID, requestID, evidenceID, first.Error(), second.Error()} {
		if strings.Contains(logged, marker) {
			t.Fatalf("synthetic sensitive marker %q leaked in logs: %q", marker, logged)
		}
	}
	for _, expected := range []string{
		"identity declaration retention purge failed; pending rows remain durable",
		"delivered identity session outbox purge failed; pending rows remain durable",
	} {
		if strings.Count(logged, expected) != 1 {
			t.Fatalf("generic log %q count in %q, want one", expected, logged)
		}
	}
}
