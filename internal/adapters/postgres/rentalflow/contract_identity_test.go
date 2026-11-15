package rentalflowpg

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type contractIdentityQuery struct {
	sql  string
	args []any
}

type contractIdentityTx struct {
	pgx.Tx
	rows         []contractIdentityRow
	queries      []contractIdentityQuery
	setStatement string
	committed    bool
	rollbackCall int
}

func (tx *contractIdentityTx) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	if len(tx.queries) != 0 || tx.setStatement != "" {
		return pgconn.CommandTag{}, pgx.ErrTxClosed
	}
	tx.setStatement = sql
	return pgconn.CommandTag{}, nil
}

func (tx *contractIdentityTx) QueryRow(_ context.Context, sql string, args ...any) pgx.Row {
	tx.queries = append(tx.queries, contractIdentityQuery{sql: sql, args: append([]any(nil), args...)})
	if len(tx.rows) == 0 {
		return contractIdentityRow{err: pgx.ErrNoRows}
	}
	row := tx.rows[0]
	tx.rows = tx.rows[1:]
	return row
}

func (tx *contractIdentityTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (tx *contractIdentityTx) Rollback(context.Context) error {
	tx.rollbackCall++
	return nil
}

type contractIdentityRow struct {
	values []any
	err    error
}

func (row contractIdentityRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}
	if len(dest) != len(row.values) {
		return pgx.ErrNoRows
	}
	for i := range dest {
		value := reflect.ValueOf(dest[i])
		if value.Kind() != reflect.Pointer || value.IsNil() {
			return pgx.ErrNoRows
		}
		source := reflect.ValueOf(row.values[i])
		if !source.IsValid() || !source.Type().AssignableTo(value.Elem().Type()) {
			return pgx.ErrNoRows
		}
		value.Elem().Set(source)
	}
	return nil
}

type contractIdentityBeginner struct{ tx pgx.Tx }

func (b contractIdentityBeginner) Begin(context.Context) (pgx.Tx, error) { return b.tx, nil }

type contractIdentityDB struct {
	DB
	queries int
}

func (db *contractIdentityDB) QueryRow(context.Context, string, ...any) pgx.Row {
	db.queries++
	return contractIdentityRow{values: []any{1}}
}

func TestCurrentForRentalContractUsesSameSnapshot(t *testing.T) {
	ctx := context.Background()
	asOf := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	actorID, quoteID, subjectID, revisionID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	keys, err := declarationcrypto.New("v1", map[string][]byte{"v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := verification.EncodeDeclarationPayload(verification.DeclarationPayload{FirstName: "Ana", LastName: "Pérez", DateOfBirth: "1990-01-01", RUT: "12345678-5", IDCountry: "CHL", DocumentType: "ID"})
	if err != nil {
		t.Fatal(err)
	}
	aad, err := verification.BuildDeclarationAAD(subjectID, revisionID, 1, "1")
	if err != nil {
		t.Fatal(err)
	}
	keyVersion, ciphertext, err := keys.Seal(aad, payload)
	if err != nil {
		t.Fatal(err)
	}
	tx := &contractIdentityTx{rows: []contractIdentityRow{
		{values: []any{1}},
		{values: []any{revisionID, subjectID, revisionID, int64(1), "1", keyVersion, ciphertext, asOf.Add(-time.Hour), asOf.Add(-time.Hour), asOf.Add(time.Hour)}},
	}}
	db := &contractIdentityDB{}
	repo, err := NewRepositoryWithDeclarationKeyring(db, contractIdentityBeginner{tx: tx}, func() time.Time { return asOf }, keys)
	if err != nil {
		t.Fatal(err)
	}

	got, err := repo.CurrentForRentalContract(ctx, actorID, quoteID, subjectID, asOf)
	if err != nil {
		t.Fatalf("expected current identity, got %v", err)
	}
	if got.SubjectID != subjectID || got.LegalName != "Ana Pérez" || got.RUT != "12345678-5" {
		t.Fatalf("unexpected identity projection: %+v", got)
	}
	if db.queries != 0 || !strings.Contains(tx.setStatement, "REPEATABLE READ READ ONLY") || !tx.committed || len(tx.queries) != 2 {
		t.Fatalf("reads escaped transaction or isolation/commit order was missing: db_reads=%d isolation=%q committed=%t tx_reads=%d", db.queries, tx.setStatement, tx.committed, len(tx.queries))
	}
	if !strings.Contains(tx.queries[0].sql, "cotizacion_arriendo_comercial") || !reflect.DeepEqual(tx.queries[0].args, []any{quoteID, actorID, subjectID}) {
		t.Fatalf("quote participation was not scoped: %+v", tx.queries[0])
	}
	identitySQL := tx.queries[1].sql
	for _, required := range []string{"identity_declaration_revision_id", "revision DESC", "verified_until>$2", "document_expires_at>$2", "retention_at>$2"} {
		if !strings.Contains(identitySQL, required) {
			t.Errorf("identity query missing required predicate %q", required)
		}
	}
	if !reflect.DeepEqual(tx.queries[1].args, []any{subjectID, asOf}) {
		t.Fatalf("identity query was not scoped to subject and asOf: %+v", tx.queries[1].args)
	}
}

func TestCurrentForRentalContractFailsClosedWithoutKeys(t *testing.T) {
	asOf := time.Now().UTC()
	tx := &contractIdentityTx{rows: []contractIdentityRow{{values: []any{1}}}}
	db := &contractIdentityDB{}
	repo, err := NewRepositoryWithDeclarationKeyring(db, contractIdentityBeginner{tx: tx}, time.Now, nil)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repo.CurrentForRentalContract(context.Background(), uuid.NewString(), uuid.NewString(), uuid.NewString(), asOf)
	if err != rentalflow.ErrIdentityUnavailable || db.queries != 0 || len(tx.queries) != 1 || tx.rollbackCall != 1 {
		t.Fatalf("got err=%v db_reads=%d tx_reads=%d rollbacks=%d", err, db.queries, len(tx.queries), tx.rollbackCall)
	}
}
