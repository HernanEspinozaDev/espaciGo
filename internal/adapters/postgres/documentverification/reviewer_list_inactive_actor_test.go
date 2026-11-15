package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type reviewerActorListBeginner struct {
	tx         *reviewerActorListTx
	beginCalls int
	options    pgx.TxOptions
}

func (b *reviewerActorListBeginner) BeginTx(_ context.Context, options pgx.TxOptions) (pgx.Tx, error) {
	b.beginCalls++
	b.options = options
	return b.tx, nil
}

type reviewerActorListTx struct {
	pgx.Tx
	active              bool
	actor               string
	lockActorCalls      int
	candidateQueryCalls int
	commitCalls         int
	rollbackCalls       int
	unexpectedQuery     string
}

func (tx *reviewerActorListTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "document_verification_lock_active_actor") {
		tx.lockActorCalls++
		if len(args) != 1 {
			tx.unexpectedQuery = "actor lock received unexpected argument count"
		} else if actor, ok := args[0].(string); !ok {
			tx.unexpectedQuery = "actor lock received non-string actor"
		} else {
			tx.actor = actor
		}
		return reviewerActorListRow{active: tx.active}
	}
	tx.unexpectedQuery = query
	return reviewerActorListRow{err: errors.New("unexpected reviewer list QueryRow")}
}

func (tx *reviewerActorListTx) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	tx.candidateQueryCalls++
	if !strings.Contains(query, "FROM public.document_verification_case c") {
		tx.unexpectedQuery = "unexpected reviewer list Query"
		return nil, errors.New(tx.unexpectedQuery)
	}
	if len(args) != 1 {
		tx.unexpectedQuery = "candidate query received unexpected argument count"
		return nil, errors.New(tx.unexpectedQuery)
	}
	queryActor, ok := args[0].(string)
	if !ok || queryActor != tx.actor {
		tx.unexpectedQuery = "candidate query actor did not match locked actor"
		return nil, errors.New(tx.unexpectedQuery)
	}
	if !tx.active {
		tx.unexpectedQuery = "inactive reviewer reached candidate query"
		return nil, errors.New(tx.unexpectedQuery)
	}
	return &emptyReviewerActorListRows{}, nil
}

func (tx *reviewerActorListTx) Commit(context.Context) error {
	tx.commitCalls++
	return nil
}

func (tx *reviewerActorListTx) Rollback(context.Context) error {
	tx.rollbackCalls++
	return nil
}

type reviewerActorListRow struct {
	active bool
	err    error
}

func (row reviewerActorListRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != 1 {
		return errors.New("actor lock row received unexpected scan destinations")
	}
	active, ok := dest[0].(*bool)
	if !ok {
		return errors.New("actor lock row expected a boolean destination")
	}
	*active = row.active
	return nil
}

type emptyReviewerActorListRows struct{ pgx.Rows }

func (*emptyReviewerActorListRows) Next() bool { return false }
func (*emptyReviewerActorListRows) Close()     {}
func (*emptyReviewerActorListRows) Err() error { return nil }

func TestListReviewerCasesChecksActorActivityBeforeListing(t *testing.T) {
	const actor = "11111111-1111-4111-8111-111111111111"
	for _, tc := range []struct {
		name                  string
		active                bool
		wantCandidateQueries  int
		wantCommitCalls       int
		wantCapabilityFailure bool
	}{
		{name: "inactive actor fails before candidate query", wantCapabilityFailure: true},
		{name: "active actor reaches candidate query", active: true, wantCandidateQueries: 1, wantCommitCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := &reviewerActorListTx{active: tc.active}
			beginner := &reviewerActorListBeginner{tx: tx}
			repository := newWithTransactionBeginner(beginner)

			cases, err := repository.ListReviewerCases(context.Background(), actor)

			if tc.wantCapabilityFailure {
				if !errors.Is(err, dv.ErrCapability) || cases != nil {
					t.Fatalf("cases=%v err=%v, want nil cases and ErrCapability", cases, err)
				}
			} else if err != nil || cases == nil || len(cases) != 0 {
				t.Fatalf("active actor control cases=%v err=%v, want empty successful queue", cases, err)
			}
			if beginner.beginCalls != 1 || beginner.options.IsoLevel != pgx.RepeatableRead {
				t.Fatalf("BeginTx calls=%d isolation=%v, want one RepeatableRead transaction", beginner.beginCalls, beginner.options.IsoLevel)
			}
			if tx.lockActorCalls != 1 || tx.actor != actor || tx.candidateQueryCalls != tc.wantCandidateQueries || tx.unexpectedQuery != "" {
				t.Fatalf("actor lock calls=%d actor=%q candidate queries=%d unexpected query=%q", tx.lockActorCalls, tx.actor, tx.candidateQueryCalls, tx.unexpectedQuery)
			}
			if tx.commitCalls != tc.wantCommitCalls || tx.rollbackCalls != 1 {
				t.Fatalf("commit calls=%d rollback calls=%d, want %d/1", tx.commitCalls, tx.rollbackCalls, tc.wantCommitCalls)
			}
		})
	}
}
