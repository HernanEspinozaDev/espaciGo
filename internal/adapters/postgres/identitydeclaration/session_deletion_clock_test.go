package identitydeclaration

import (
	"testing"
	"time"
)

func TestSessionDeletionQueueDefaultClockDoesNotFreezeAtConstruction(t *testing.T) {
	q := NewPostgresSessionDeletionQueue(nil)
	afterConstruction := time.Now().UTC()
	observed := q.now()
	if observed.Before(afterConstruction) {
		t.Fatal("default ACK clock is frozen before subsequent outbox creation")
	}
	if observed.Location() != time.UTC {
		t.Fatal("default queue clock must be UTC")
	}
}

func TestSessionDeletionQueueExplicitClockRemainsInjectable(t *testing.T) {
	first := time.Date(2026, 11, 9, 20, 0, 0, 0, time.UTC)
	current := first
	q := NewPostgresSessionDeletionQueue(nil, func() time.Time { return current })
	if !q.now().Equal(first) {
		t.Fatal("injected clock not used")
	}
	current = first.Add(time.Minute)
	if !q.now().Equal(current) {
		t.Fatal("injected clock was frozen")
	}
}
