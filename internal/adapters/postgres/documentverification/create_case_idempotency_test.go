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

const (
	idempotentCreateActorID    = "11111111-1111-4111-8111-111111111111"
	idempotentCreateRevisionID = "22222222-2222-4222-8222-222222222222"
	idempotentCreateExistingID = "33333333-3333-4333-8333-333333333333"
	idempotentCreateProposedID = "44444444-4444-4444-8444-444444444444"
)

type identityCreateRetryTx struct {
	*createCaseFakeTx
	existingCase   bool
	existingFormat dv.DocumentFormat
	activeQuery    string
	activeArgs     []any
	fileQueryCalls int
}

func (tx *identityCreateRetryTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "FROM public.document_verification_case") && strings.Contains(query, "identity_declaration_revision_id") {
		tx.activeQuery = query
		tx.activeArgs = append([]any(nil), args...)
		if !tx.existingCase {
			return commitSerializationFailureRow{err: pgx.ErrNoRows}
		}
		format := tx.existingFormat
		if format == "" {
			format = dv.DocumentFormatNew
		}
		return commitSerializationFailureRow{values: []any{
			idempotentCreateExistingID, string(dv.PurposeIdentity), idempotentCreateActorID, nil,
			string(dv.CaseSubmitted), int64(8), "", nil, nil, nil, nil, nil,
			idempotentCreateRevisionID, string(format), dv.IdentityCaptureConsentVersion,
			time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC), "",
		}}
	}
	return tx.createCaseFakeTx.QueryRow(ctx, query, args...)
}

func (tx *identityCreateRetryTx) Query(_ context.Context, query string, _ ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_file") {
		tx.fileQueryCalls++
		return emptyCommitSerializationRows{}, nil
	}
	return nil, pgx.ErrNoRows
}

func TestCreateCaseRetryReturnsExistingActiveIdentityCase(t *testing.T) {
	tx := &identityCreateRetryTx{
		createCaseFakeTx: &createCaseFakeTx{
			actorActive:      true,
			declarationID:    idempotentCreateRevisionID,
			declarationFound: true,
		},
		existingCase: true,
	}
	repository := newWithTransactionBeginner(createCaseFakeBeginner{tx: tx})
	buildCalls := 0
	created, err := repository.CreateCase(context.Background(), idempotentCreateActorID, dv.PurposeIdentity, "", func(auth dv.Authorization) (dv.Snapshot, error) {
		buildCalls++
		return dv.Snapshot{
			ID: idempotentCreateProposedID, Purpose: dv.PurposeIdentity, SubjectID: idempotentCreateActorID,
			State: dv.CaseDraft, IdentityDeclarationRevisionID: auth.IdentityDeclarationRevisionID,
			DocumentFormatDeclared: dv.DocumentFormatNew, CaptureConsentVersion: dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
		}, nil
	})
	if err != nil {
		t.Fatalf("retry create case: %v", err)
	}
	if created.ID != idempotentCreateExistingID || created.State != dv.CaseSubmitted || created.Revision != 8 ||
		created.IdentityDeclarationRevisionID != idempotentCreateRevisionID || created.DocumentFormatDeclared != dv.DocumentFormatNew {
		t.Fatalf("retry returned new or incomplete case instead of existing active case: %+v", created)
	}
	if buildCalls != 1 || tx.execCount != 0 || !tx.committed || tx.fileQueryCalls != 1 {
		t.Fatalf("retry side effects/build/files: build=%d exec=%d commit=%v file_queries=%d", buildCalls, tx.execCount, tx.committed, tx.fileQueryCalls)
	}
	if len(tx.activeArgs) != 2 || tx.activeArgs[0] != idempotentCreateActorID || tx.activeArgs[1] != idempotentCreateRevisionID ||
		!strings.Contains(tx.activeQuery, "state IN ('borrador','enviada','en_revision','requiere_antecedentes')") {
		t.Fatalf("active-case lookup not scoped to owner, declaration, and active states: sql=%s args=%#v", tx.activeQuery, tx.activeArgs)
	}
}

func TestCreateCaseRetryRejectsDifferentDeclaredFormat(t *testing.T) {
	tx := &identityCreateRetryTx{
		createCaseFakeTx: &createCaseFakeTx{
			actorActive:      true,
			declarationID:    idempotentCreateRevisionID,
			declarationFound: true,
		},
		existingCase:   true,
		existingFormat: dv.DocumentFormatLegacy,
	}
	repository := newWithTransactionBeginner(createCaseFakeBeginner{tx: tx})
	_, err := repository.CreateCase(context.Background(), idempotentCreateActorID, dv.PurposeIdentity, "", func(auth dv.Authorization) (dv.Snapshot, error) {
		return dv.Snapshot{
			ID: idempotentCreateProposedID, Purpose: dv.PurposeIdentity, SubjectID: idempotentCreateActorID,
			State: dv.CaseDraft, IdentityDeclarationRevisionID: auth.IdentityDeclarationRevisionID,
			DocumentFormatDeclared: dv.DocumentFormatNew, CaptureConsentVersion: dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
		}, nil
	})
	if !errors.Is(err, dv.ErrConflict) {
		t.Fatalf("retry with changed declared format error=%v, want conflict", err)
	}
	if tx.execCount != 0 || tx.committed || tx.fileQueryCalls != 0 {
		t.Fatalf("mismatched retry performed side effects: exec=%d commit=%v file_queries=%d", tx.execCount, tx.committed, tx.fileQueryCalls)
	}
}

func TestCreateCaseFirstCallStillCreatesNewIdentityCase(t *testing.T) {
	tx := &identityCreateRetryTx{
		createCaseFakeTx: &createCaseFakeTx{
			actorActive:      true,
			declarationID:    idempotentCreateRevisionID,
			declarationFound: true,
		},
	}
	repository := newWithTransactionBeginner(createCaseFakeBeginner{tx: tx})
	created, err := repository.CreateCase(context.Background(), idempotentCreateActorID, dv.PurposeIdentity, "", func(auth dv.Authorization) (dv.Snapshot, error) {
		return dv.Snapshot{
			ID: idempotentCreateProposedID, Purpose: dv.PurposeIdentity, SubjectID: idempotentCreateActorID,
			State: dv.CaseDraft, IdentityDeclarationRevisionID: auth.IdentityDeclarationRevisionID,
			DocumentFormatDeclared: dv.DocumentFormatNew, CaptureConsentVersion: dv.IdentityCaptureConsentVersion,
			CaptureConsentedAt: time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC),
		}, nil
	})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if created.ID != idempotentCreateProposedID || !tx.committed || tx.execCount != 2 {
		t.Fatalf("first create did not insert exactly one case and event: case=%+v exec=%d commit=%v", created, tx.execCount, tx.committed)
	}
}
