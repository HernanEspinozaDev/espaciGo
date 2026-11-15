package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

type reviewerCaseScopeReadTx struct {
	*reviewerAuditFakeTx
	grants                  []dv.ReviewerGrant
	grantQueryCalls         int
	fileQueryCalls          int
	spaceID                 *string
	spaceOwner              bool
	spaceOwnerErr           error
	spaceLockCalls          int
	spaceLockArgs           []any
	caseReadCalls           int
	rollbackCalls           int
	currentDeclarationCalls int
}

func (tx *reviewerCaseScopeReadTx) Rollback(ctx context.Context) error {
	tx.rollbackCalls++
	return tx.reviewerAuditFakeTx.Rollback(ctx)
}

func (tx *reviewerCaseScopeReadTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "document_verification_lock_space_owner") {
		tx.spaceLockCalls++
		tx.spaceLockArgs = append([]any(nil), args...)
		if tx.spaceOwnerErr != nil {
			return commitSerializationFailureRow{err: tx.spaceOwnerErr}
		}
		return commitSerializationFailureRow{values: []any{tx.spaceOwner}}
	}
	if strings.Contains(query, "SELECT space_id::text,subject_id::text FROM public.document_verification_case") {
		if len(args) != 1 || args[0] != tx.caseID {
			return commitSerializationFailureRow{err: errors.New("reviewer load scope read used unexpected id")}
		}
		var space any
		if tx.spaceID != nil {
			space = *tx.spaceID
		}
		return commitSerializationFailureRow{values: []any{space, tx.subject}}
	}
	if strings.Contains(query, "SELECT subject_id::text,space_id::text FROM public.document_verification_case") {
		if len(args) != 1 || args[0] != tx.caseID {
			return commitSerializationFailureRow{err: errors.New("case scope read used unexpected id")}
		}
		var space any
		if tx.spaceID != nil {
			space = *tx.spaceID
		}
		return commitSerializationFailureRow{values: []any{tx.subject, space}}
	}
	if strings.Contains(query, identityRevisionCurrentQueryShape) {
		tx.currentDeclarationCalls++
		return tx.reviewerAuditFakeTx.QueryRow(ctx, query, args...)
	}
	if strings.Contains(query, "SELECT "+reviewerSnapshotColumns) {
		tx.caseReadCalls++
		purpose := string(dv.PurposeIdentity)
		var space any
		if tx.spaceID != nil {
			purpose = string(dv.PurposeSpace)
			space = *tx.spaceID
		}
		return commitSerializationFailureRow{values: []any{
			tx.caseID, purpose, tx.subject, space, "en_revision", int64(7), "",
			nil, nil, nil, nil, nil, tx.declarationID, "new", dv.IdentityCaptureConsentVersion, tx.consentedAt, "",
		}}
	}
	return tx.reviewerAuditFakeTx.QueryRow(ctx, query, args...)
}

func (tx *reviewerCaseScopeReadTx) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_reviewer_grant") {
		tx.grantQueryCalls++
		if len(args) != 1 || args[0] != tx.actor {
			return nil, errors.New("grant query used unexpected reviewer")
		}
		rows := make([][]any, 0, len(tx.grants))
		for _, grant := range tx.grants {
			rows = append(rows, []any{grant.ReviewerID, string(grant.Capability), grant.ScopeID})
		}
		return &reviewerAuditFakeRows{rows: rows}, nil
	}
	if strings.Contains(query, "FROM public.document_verification_file") {
		tx.fileQueryCalls++
		return &reviewerAuditFakeRows{}, nil
	}
	return tx.reviewerAuditFakeTx.Query(ctx, query, args...)
}

type reviewerCaseScopeReadBeginner struct{ tx pgx.Tx }

func (b reviewerCaseScopeReadBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestWithReviewerCaseForReadRevalidatesIdentityGrantScope(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	consentedAt := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name          string
		grantScope    string
		wantErr       error
		wantCallback  bool
		wantFileReads int
	}{
		{name: "exact identity grant", grantScope: subject, wantCallback: true, wantFileReads: 1},
		{name: "wrong identity scope", grantScope: "55555555-5555-4555-8555-555555555555", wantErr: dv.ErrNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := &reviewerAuditFakeTx{
				actor: actor, subject: subject, caseID: caseID,
				declarationID: declarationID, consentedAt: consentedAt,
			}
			tx := reviewerCaseScopeReadTx{
				reviewerAuditFakeTx: base,
				grants:              []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: tc.grantScope}},
			}
			repo := newWithTransactionBeginner(reviewerCaseScopeReadBeginner{tx: &tx})
			callbackCalled := false
			err := repo.WithReviewerCaseForRead(context.Background(), actor, caseID, func(_ context.Context, snapshot dv.Snapshot, authorization dv.Authorization) error {
				callbackCalled = true
				if snapshot.ID != caseID || snapshot.Purpose != dv.PurposeIdentity || snapshot.SubjectID != subject || snapshot.State != dv.CaseInReview || snapshot.Files == nil {
					t.Fatalf("callback snapshot=%+v", snapshot)
				}
				if authorization.ActorID != actor || authorization.SubjectID != subject || len(authorization.ReviewerGrants) != 1 {
					t.Fatalf("callback authorization=%+v", authorization)
				}
				grant := authorization.ReviewerGrants[0]
				if grant.ReviewerID != actor || grant.Capability != dv.CapabilityIdentity || grant.ScopeID != subject {
					t.Fatalf("callback grant=%+v", grant)
				}
				return nil
			})
			if !errors.Is(err, tc.wantErr) || callbackCalled != tc.wantCallback || tx.committed != tc.wantCallback || len(base.execs) != 0 {
				t.Fatalf("error=%v callback=%t committed=%t execs=%d", err, callbackCalled, tx.committed, len(base.execs))
			}
			if tx.grantQueryCalls != 1 || tx.fileQueryCalls != tc.wantFileReads {
				t.Fatalf("grant reads=%d file reads=%d, want 1/%d", tx.grantQueryCalls, tx.fileQueryCalls, tc.wantFileReads)
			}
		})
	}
}

func TestWithReviewerCaseForReadRevalidatesSpaceOwnerBeforeCallback(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
		spaceID       = "66666666-6666-4666-8666-666666666666"
		otherSpaceID  = "77777777-7777-4777-8777-777777777777"
	)
	lockErr := errors.New("synthetic reviewer space-owner lock failure")
	tests := []struct {
		name           string
		ownerMatches   bool
		ownerErr       error
		grant          dv.ReviewerGrant
		wantErr        error
		wantCallback   bool
		wantCaseReads  int
		wantGrantReads int
		wantFileReads  int
	}{
		{
			name:           "exact space grant and owner",
			ownerMatches:   true,
			grant:          dv.ReviewerGrant{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: spaceID},
			wantCallback:   true,
			wantCaseReads:  1,
			wantGrantReads: 1,
			wantFileReads:  1,
		},
		{
			name:    "owner mismatch stops before case or evidence read",
			grant:   dv.ReviewerGrant{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: spaceID},
			wantErr: dv.ErrNotFound,
		},
		{
			name:         "owner lock error stops before case or evidence read",
			ownerMatches: true,
			ownerErr:     lockErr,
			grant:        dv.ReviewerGrant{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: spaceID},
			wantErr:      lockErr,
		},
		{
			name:           "wrong space grant stops before evidence read",
			ownerMatches:   true,
			grant:          dv.ReviewerGrant{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: otherSpaceID},
			wantErr:        dv.ErrNotFound,
			wantCaseReads:  1,
			wantGrantReads: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			consentedAt := time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC)
			spaceIDValue := spaceID
			base := &reviewerAuditFakeTx{
				actor: actor, subject: subject, caseID: caseID,
				declarationID: declarationID, consentedAt: consentedAt,
			}
			tx := &reviewerCaseScopeReadTx{
				reviewerAuditFakeTx: base,
				grants:              []dv.ReviewerGrant{tc.grant},
				spaceID:             &spaceIDValue,
				spaceOwner:          tc.ownerMatches,
				spaceOwnerErr:       tc.ownerErr,
			}
			repo := newWithTransactionBeginner(reviewerCaseScopeReadBeginner{tx: tx})
			callbackCalled := false
			err := repo.WithReviewerCaseForRead(context.Background(), actor, caseID, func(_ context.Context, snapshot dv.Snapshot, authorization dv.Authorization) error {
				callbackCalled = true
				if snapshot.ID != caseID || snapshot.Purpose != dv.PurposeSpace || snapshot.SubjectID != subject || snapshot.SpaceID != spaceID || snapshot.Files == nil {
					t.Fatalf("callback snapshot=%+v", snapshot)
				}
				if authorization.ActorID != actor || authorization.SubjectID != subject || authorization.SpaceOwnerID != subject || len(authorization.ReviewerGrants) != 1 {
					t.Fatalf("callback authorization=%+v", authorization)
				}
				grant := authorization.ReviewerGrants[0]
				if grant.ReviewerID != actor || grant.Capability != dv.CapabilitySpace || grant.ScopeID != spaceID {
					t.Fatalf("callback grant=%+v", grant)
				}
				return nil
			})

			if !errors.Is(err, tc.wantErr) || callbackCalled != tc.wantCallback || tx.committed != tc.wantCallback || len(base.execs) != 0 {
				t.Fatalf("error=%v callback=%t committed=%t writes=%d", err, callbackCalled, tx.committed, len(base.execs))
			}
			if tx.spaceLockCalls != 1 || len(tx.spaceLockArgs) != 2 || tx.spaceLockArgs[0] != spaceID || tx.spaceLockArgs[1] != subject {
				t.Fatalf("space lock calls=%d args=%v, want exact space and case owner", tx.spaceLockCalls, tx.spaceLockArgs)
			}
			if tx.caseReadCalls != tc.wantCaseReads || tx.grantQueryCalls != tc.wantGrantReads || tx.fileQueryCalls != tc.wantFileReads {
				t.Fatalf("case/grant/file reads=%d/%d/%d, want %d/%d/%d", tx.caseReadCalls, tx.grantQueryCalls, tx.fileQueryCalls,
					tc.wantCaseReads, tc.wantGrantReads, tc.wantFileReads)
			}
			if tx.rollbackCalls != 1 {
				t.Fatalf("rollback calls=%d, want one deferred rollback invocation", tx.rollbackCalls)
			}
		})
	}
}

func TestLoadReviewerCaseRevalidatesSpaceOwnerAndGrantBeforeReadingFiles(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
		spaceID       = "66666666-6666-4666-8666-666666666666"
		otherSpaceID  = "77777777-7777-4777-8777-777777777777"
	)
	lockErr := errors.New("synthetic reviewer space-owner lock failure")
	tests := []struct {
		name           string
		ownerMatches   bool
		ownerErr       error
		grantScope     string
		wantErr        error
		wantSnapshot   bool
		wantCaseReads  int
		wantGrantReads int
		wantFileReads  int
	}{
		{
			name:           "exact owner and space grant",
			ownerMatches:   true,
			grantScope:     spaceID,
			wantSnapshot:   true,
			wantCaseReads:  1,
			wantGrantReads: 1,
			wantFileReads:  1,
		},
		{
			name:       "owner mismatch stops before snapshot and files",
			grantScope: spaceID,
			wantErr:    dv.ErrNotFound,
		},
		{
			name:         "owner lock error stops before snapshot and files",
			ownerMatches: true,
			ownerErr:     lockErr,
			grantScope:   spaceID,
			wantErr:      lockErr,
		},
		{
			name:           "wrong space grant stops before files",
			ownerMatches:   true,
			grantScope:     otherSpaceID,
			wantErr:        dv.ErrNotFound,
			wantCaseReads:  1,
			wantGrantReads: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			spaceIDValue := spaceID
			base := &reviewerAuditFakeTx{
				actor: actor, subject: subject, caseID: caseID,
				declarationID: declarationID, consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
			}
			tx := &reviewerCaseScopeReadTx{
				reviewerAuditFakeTx: base,
				grants:              []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilitySpace, ScopeID: tc.grantScope}},
				spaceID:             &spaceIDValue,
				spaceOwner:          tc.ownerMatches,
				spaceOwnerErr:       tc.ownerErr,
			}
			repo := newWithTransactionBeginner(reviewerCaseScopeReadBeginner{tx: tx})
			snapshot, err := repo.LoadReviewerCase(context.Background(), actor, caseID)

			if !errors.Is(err, tc.wantErr) || (snapshot.ID != "") != tc.wantSnapshot || len(base.execs) != 0 {
				t.Fatalf("snapshot=%+v err=%v writes=%d, want snapshot=%v error=%v and no writes", snapshot, err, len(base.execs), tc.wantSnapshot, tc.wantErr)
			}
			if tc.wantSnapshot && (snapshot.Purpose != dv.PurposeSpace || snapshot.SubjectID != subject || snapshot.SpaceID != spaceID || snapshot.Files == nil) {
				t.Fatalf("returned snapshot=%+v, want exact synthetic space and owner", snapshot)
			}
			if tx.spaceLockCalls != 1 || len(tx.spaceLockArgs) != 2 || tx.spaceLockArgs[0] != spaceID || tx.spaceLockArgs[1] != subject {
				t.Fatalf("space lock calls=%d args=%v, want exact space and case owner", tx.spaceLockCalls, tx.spaceLockArgs)
			}
			if tx.caseReadCalls != tc.wantCaseReads || tx.grantQueryCalls != tc.wantGrantReads || tx.fileQueryCalls != tc.wantFileReads || tx.currentDeclarationCalls != 0 {
				t.Fatalf("case/grant/file/freshness reads=%d/%d/%d/%d, want %d/%d/%d/0",
					tx.caseReadCalls, tx.grantQueryCalls, tx.fileQueryCalls, tx.currentDeclarationCalls,
					tc.wantCaseReads, tc.wantGrantReads, tc.wantFileReads)
			}
			if tx.committed != tc.wantSnapshot || tx.rollbackCalls != 1 {
				t.Fatalf("committed=%v rollback calls=%d, want %v/1", tx.committed, tx.rollbackCalls, tc.wantSnapshot)
			}
		})
	}
}

func TestLoadReviewerCaseRejectsSelfReviewBeforeGrantOrFileReads(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	base := &reviewerAuditFakeTx{
		actor: actor, subject: actor, caseID: caseID, declarationID: declarationID,
		consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
	}
	tx := &reviewerCaseScopeReadTx{
		reviewerAuditFakeTx: base,
		grants:              []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: actor}},
	}
	repo := newWithTransactionBeginner(reviewerCaseScopeReadBeginner{tx: tx})

	snapshot, err := repo.LoadReviewerCase(context.Background(), actor, caseID)
	if !errors.Is(err, dv.ErrNotFound) || snapshot.ID != "" {
		t.Fatalf("snapshot=%+v err=%v, want empty snapshot and not-found", snapshot, err)
	}
	if tx.caseReadCalls != 1 || tx.grantQueryCalls != 0 || tx.currentDeclarationCalls != 0 || tx.fileQueryCalls != 0 {
		t.Fatalf("case/grant/freshness/file reads=%d/%d/%d/%d, want 1/0/0/0", tx.caseReadCalls, tx.grantQueryCalls, tx.currentDeclarationCalls, tx.fileQueryCalls)
	}
	if tx.committed || tx.rollbackCalls != 1 || len(base.execs) != 0 {
		t.Fatalf("committed=%v rollback calls=%d writes=%d, want no commit, one rollback invocation, and no writes", tx.committed, tx.rollbackCalls, len(base.execs))
	}
}

func TestWithReviewerCaseForReadPropagatesCallbackErrorWithoutCommit(t *testing.T) {
	const (
		actor         = "11111111-1111-4111-8111-111111111111"
		subject       = "22222222-2222-4222-8222-222222222222"
		caseID        = "33333333-3333-4333-8333-333333333333"
		declarationID = "44444444-4444-4444-8444-444444444444"
	)
	callbackErr := errors.New("synthetic reviewer detail callback failure")
	base := &reviewerAuditFakeTx{
		actor: actor, subject: subject, caseID: caseID, declarationID: declarationID,
		consentedAt: time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC),
	}
	tx := &reviewerCaseScopeReadTx{
		reviewerAuditFakeTx: base,
		grants:              []dv.ReviewerGrant{{ReviewerID: actor, Capability: dv.CapabilityIdentity, ScopeID: subject}},
	}
	repo := newWithTransactionBeginner(reviewerCaseScopeReadBeginner{tx: tx})
	callbackCalls := 0

	err := repo.WithReviewerCaseForRead(context.Background(), actor, caseID, func(context.Context, dv.Snapshot, dv.Authorization) error {
		callbackCalls++
		return callbackErr
	})
	if !errors.Is(err, callbackErr) || callbackCalls != 1 {
		t.Fatalf("error=%v callback calls=%d, want callback error after one call", err, callbackCalls)
	}
	if tx.committed || tx.rollbackCalls != 1 {
		t.Fatalf("committed=%v rollback calls=%d, want no commit and one deferred rollback invocation", tx.committed, tx.rollbackCalls)
	}
	if tx.caseReadCalls != 1 || tx.grantQueryCalls != 1 || tx.currentDeclarationCalls != 1 || tx.fileQueryCalls != 1 || len(base.execs) != 0 {
		t.Fatalf("case/grant/freshness/file reads=%d/%d/%d/%d writes=%d, want one scoped read each and no writes", tx.caseReadCalls, tx.grantQueryCalls, tx.currentDeclarationCalls, tx.fileQueryCalls, len(base.execs))
	}
}
