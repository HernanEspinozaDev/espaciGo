package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type retentionClaimBeginFailure struct {
	calls   int
	options pgx.TxOptions
	err     error
}

func (b *retentionClaimBeginFailure) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	b.calls++
	b.options = options
	return nil, b.err
}

func TestClaimReadyRetentionJobPropagatesBeginFailure(t *testing.T) {
	beginErr := errors.New("synthetic retention claim begin failure")
	beginner := &retentionClaimBeginFailure{err: beginErr}
	repo := newWithTransactionBeginner(beginner)
	claim, err := repo.ClaimReadyRetentionJob(context.Background(), time.Minute)
	if !errors.Is(err, beginErr) || claim != nil {
		t.Fatalf("ClaimReadyRetentionJob()=(%+v,%v); want nil claim and BeginTx error", claim, err)
	}
	if beginner.calls != 1 || beginner.options != (pgx.TxOptions{IsoLevel: pgx.ReadCommitted}) {
		t.Fatalf("BeginTx calls=%d options=%+v; want one ReadCommitted transaction request", beginner.calls, beginner.options)
	}
}
