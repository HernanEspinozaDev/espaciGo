package model

import (
	"errors"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/filter"
)

func TestDecodeBudgetRejectsLimitBelowUsageAndReportsUsage(t *testing.T) {
	b := NewDecodeBudget(12)
	if err := b.Charge(5); err != nil {
		t.Fatal(err)
	}
	if got := b.Used(); got != 5 {
		t.Fatalf("Used() = %d, want 5", got)
	}
	if err := b.SetLimit(8); err != nil {
		t.Fatalf("raise/lower limit above usage: %v", err)
	}
	if err := b.SetLimit(4); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("SetLimit below usage = %v, want aggregate decode limit exceeded", err)
	}
	if got := b.Limit(); got != 8 {
		t.Fatalf("limit below already-used output was accepted: Limit() = %d, want 8", got)
	}
}

func TestDecodeBudgetDoesNotLowerLimitBelowOutstandingReservation(t *testing.T) {
	b := NewDecodeBudget(12)
	_, reservation, err := b.ReserveDecode(8)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.SetLimit(7); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("SetLimit below reservation = %v, want aggregate decode limit exceeded", err)
	}
	if got := b.Limit(); got != 12 {
		t.Fatalf("limit below outstanding reservation was accepted: Limit() = %d, want 12", got)
	}
	if err := b.FinishDecode(reservation, 6, true); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeBudgetFinishDecodeSaturatesOnUsageOverflow(t *testing.T) {
	const maxInt64 = int64(^uint64(0) >> 1)
	b := NewDecodeBudget(-1)
	if err := b.Charge(maxInt64 - 1); err != nil {
		t.Fatal(err)
	}
	_, reservation, err := b.ReserveDecode(2)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.FinishDecode(reservation, 2, true); !errors.Is(err, filter.ErrDecodeLimitExceeded) {
		t.Fatalf("FinishDecode overflow error = %v, want aggregate decode limit exceeded", err)
	}
	if got := b.Used(); got != maxInt64 {
		t.Fatalf("Used() after overflow = %d, want saturated %d", got, maxInt64)
	}
}

func TestDecodeBudgetZeroUsesDefaultAndNegativeDisablesCap(t *testing.T) {
	if got := NewDecodeBudget(0).Limit(); got != 512<<20 {
		t.Fatalf("zero limit = %d, want 512 MiB", got)
	}
	b := NewDecodeBudget(-1)
	if err := b.Charge(1 << 62); err != nil {
		t.Fatalf("negative limit should disable cap: %v", err)
	}
}
