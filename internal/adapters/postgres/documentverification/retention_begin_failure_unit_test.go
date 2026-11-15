package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type retentionBeginFailure struct {
	calls   int
	options pgx.TxOptions
	err     error
}

func (b *retentionBeginFailure) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	b.calls++
	b.options = options
	return nil, b.err
}

func TestProcessClaimedRetentionJobDoesNotDeleteWhenTransactionCannotBegin(t *testing.T) {
	claim := RetentionClaim{
		JobID: "00000000-0000-4000-8000-0000000000f1", FileID: "00000000-0000-4000-8000-0000000000f2",
		CaseID: "00000000-0000-4000-8000-0000000000f3", SubjectID: "00000000-0000-4000-8000-0000000000f4",
		Purpose: dv.PurposeIdentity, LeaseUntil: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
	}
	beginErr := errors.New("synthetic BeginTx failure")
	beginner := &retentionBeginFailure{err: beginErr}
	repo := newWithTransactionBeginner(beginner)
	deleteCalls := 0
	err := repo.ProcessClaimedRetentionJob(context.Background(), claim, func(context.Context) error {
		deleteCalls++
		return nil
	}, time.Minute)
	if !errors.Is(err, beginErr) || beginner.calls != 1 || beginner.options.IsoLevel != pgx.ReadCommitted || deleteCalls != 0 {
		t.Fatalf("error=%v begin calls=%d isolation=%v delete calls=%d; want BeginTx error, one ReadCommitted begin, and no deletion", err, beginner.calls, beginner.options.IsoLevel, deleteCalls)
	}
}
