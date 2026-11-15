package documentverificationpg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/jackc/pgx/v5"
)

const (
	staleDeclarationOwnerID = "11111111-1111-4111-8111-111111111111"
	staleDeclarationCaseID  = "22222222-2222-4222-8222-222222222222"
	oldDeclarationRevision  = "33333333-3333-4333-8333-333333333333"
	newDeclarationRevision  = "44444444-4444-4444-8444-444444444444"
)

type declarationRevisionRow func(...any) error

func (r declarationRevisionRow) Scan(dest ...any) error { return r(dest...) }

type declarationRevisionTx struct {
	pgx.Tx
	query                      string
	caseDeclarationRevisionID  string
	currentDeclarationRevision string
	matchedLatestRevision      bool
}

func queryMatchesLatestDeclarationRevision(query string) bool {
	normalized := strings.Join(strings.Fields(query), " ")
	return strings.Contains(normalized, "state IN ('aprobada','revocada','vencida')") &&
		strings.Contains(normalized, "current_case.identity_declaration_revision_id=(") &&
		strings.Contains(normalized, "SELECT revision.id FROM public.identity_declaration_revision revision") &&
		strings.Contains(normalized, "WHERE revision.account_id=current_case.subject_id") &&
		strings.Contains(normalized, "ORDER BY revision.revision DESC LIMIT 1")
}

func (tx *declarationRevisionTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.query = query
	tx.matchedLatestRevision = queryMatchesLatestDeclarationRevision(query)
	if tx.matchedLatestRevision && tx.caseDeclarationRevisionID != tx.currentDeclarationRevision {
		// The synthetic old approval cannot match the newer saved declaration.
		return declarationRevisionRow(func(...any) error { return pgx.ErrNoRows })
	}
	if strings.Contains(query, "SELECT current_case.id::text") {
		return declarationRevisionRow(func(dest ...any) error {
			if len(dest) != 1 {
				return errors.New("unexpected identityCurrent scan shape")
			}
			value, ok := dest[0].(*string)
			if !ok {
				return errors.New("identityCurrent expected a string destination")
			}
			*value = staleDeclarationCaseID
			return nil
		})
	}
	return declarationRevisionRow(func(dest ...any) error {
		if len(dest) != 5 {
			return errors.New("unexpected CurrentForAccount scan shape")
		}
		values := []any{
			staleDeclarationOwnerID,
			identityverification.StateApproved,
			time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
			time.Date(2027, 11, 1, 12, 0, 0, 0, time.UTC),
			nil,
		}
		for i, target := range dest {
			if scanner, ok := target.(interface{ Scan(any) error }); ok {
				if err := scanner.Scan(values[i]); err != nil {
					return err
				}
				continue
			}
			stringTarget, ok := target.(*string)
			if !ok {
				return errors.New("unexpected CurrentForAccount scan destination")
			}
			*stringTarget = values[i].(string)
		}
		return nil
	})
}

func (tx *declarationRevisionTx) Commit(context.Context) error   { return nil }
func (tx *declarationRevisionTx) Rollback(context.Context) error { return nil }

type declarationRevisionBeginner struct{ tx *declarationRevisionTx }

func (b declarationRevisionBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

func TestIdentityCurrentAllowsApprovalForCurrentDeclarationRevision(t *testing.T) {
	tx := &declarationRevisionTx{
		caseDeclarationRevisionID:  oldDeclarationRevision,
		currentDeclarationRevision: oldDeclarationRevision,
	}
	current, err := identityCurrent(context.Background(), tx, staleDeclarationOwnerID)
	if err != nil || !current {
		t.Fatalf("approval for matching declaration revision should remain current: current=%t err=%v", current, err)
	}
	if !tx.matchedLatestRevision {
		t.Fatal("identityCurrent query did not compare the case with the latest declaration revision")
	}
}

func TestIdentityCurrentRejectsApprovalForOlderDeclarationRevision(t *testing.T) {
	tx := &declarationRevisionTx{
		caseDeclarationRevisionID:  oldDeclarationRevision,
		currentDeclarationRevision: newDeclarationRevision,
	}
	current, err := identityCurrent(context.Background(), tx, staleDeclarationOwnerID)
	if err != nil {
		t.Fatalf("identityCurrent returned error: %v", err)
	}
	if current {
		t.Fatalf("approval for declaration %s remained current after declaration %s was saved", oldDeclarationRevision, newDeclarationRevision)
	}
	if !tx.matchedLatestRevision {
		t.Fatal("identityCurrent query did not compare the case with the latest declaration revision")
	}
}

func TestCurrentForAccountAllowsApprovalForCurrentDeclarationRevision(t *testing.T) {
	tx := &declarationRevisionTx{
		caseDeclarationRevisionID:  oldDeclarationRevision,
		currentDeclarationRevision: oldDeclarationRevision,
	}
	repository := newWithTransactionBeginner(declarationRevisionBeginner{tx: tx})
	current, err := repository.CurrentForAccount(context.Background(), staleDeclarationOwnerID)
	if err != nil || current.State != identityverification.StateApproved {
		t.Fatalf("approval for matching declaration revision should be returned: verification=%+v err=%v", current, err)
	}
	if !tx.matchedLatestRevision {
		t.Fatal("CurrentForAccount query did not compare the case with the latest declaration revision")
	}
}

func TestCurrentForAccountRejectsApprovalForOlderDeclarationRevision(t *testing.T) {
	tx := &declarationRevisionTx{
		caseDeclarationRevisionID:  oldDeclarationRevision,
		currentDeclarationRevision: newDeclarationRevision,
	}
	repository := newWithTransactionBeginner(declarationRevisionBeginner{tx: tx})
	_, err := repository.CurrentForAccount(context.Background(), staleDeclarationOwnerID)
	if !errors.Is(err, identityverification.ErrNotFound) {
		t.Fatalf("CurrentForAccount error=%v, want ErrNotFound after declaration revision changed", err)
	}
	if !tx.matchedLatestRevision {
		t.Fatal("CurrentForAccount query did not compare the case with the latest declaration revision")
	}
}
