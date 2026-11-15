package documentverificationpg

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/jackc/pgx/v5"
)

const (
	ownerDecisionProjectionActor  = "11111111-1111-4111-8111-111111111111"
	ownerDecisionProjectionCaseID = "22222222-2222-4222-8222-222222222222"
)

type ownerDecisionProjectionBeginner struct{ tx *ownerDecisionProjectionTx }

func (b ownerDecisionProjectionBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	return b.tx, nil
}

type ownerDecisionProjectionTx struct {
	pgx.Tx
	listQuery   string
	listArgs    []any
	detailQuery string
	detailArgs  []any
	committed   bool
}

func (tx *ownerDecisionProjectionTx) QueryRow(_ context.Context, query string, args ...any) pgx.Row {
	if strings.Contains(query, "document_verification_lock_active_actor") {
		return ownerDecisionProjectionRow{values: []any{true}}
	}
	tx.detailQuery = query
	tx.detailArgs = append([]any(nil), args...)
	return ownerDecisionProjectionRow{values: ownerDecisionProjectionValues()}
}

func (tx *ownerDecisionProjectionTx) Query(_ context.Context, query string, args ...any) (pgx.Rows, error) {
	if strings.Contains(query, "FROM public.document_verification_file") {
		return &ownerDecisionProjectionRows{}, nil
	}
	tx.listQuery = query
	tx.listArgs = append([]any(nil), args...)
	return &ownerDecisionProjectionRows{rows: [][]any{ownerDecisionProjectionValues()}}, nil
}

func (tx *ownerDecisionProjectionTx) Commit(context.Context) error {
	tx.committed = true
	return nil
}

func (*ownerDecisionProjectionTx) Rollback(context.Context) error { return nil }

func ownerDecisionProjectionValues() []any {
	return []any{
		ownerDecisionProjectionCaseID, dv.PurposeIdentity, ownerDecisionProjectionActor,
		nil, dv.CaseNeedsEvidence, int64(18), "", nil, nil, nil, nil, nil,
		"33333333-3333-4333-8333-333333333333", dv.DocumentFormatUnsure,
		"synthetic-consent-v1", time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC),
		"", dv.ReasonEvidenceIncomplete,
	}
}

type ownerDecisionProjectionRows struct {
	pgx.Rows
	rows    [][]any
	current []any
	index   int
}

func (rows *ownerDecisionProjectionRows) Next() bool {
	if rows.index >= len(rows.rows) {
		return false
	}
	rows.current = rows.rows[rows.index]
	rows.index++
	return true
}

func (rows *ownerDecisionProjectionRows) Scan(dest ...any) error {
	return ownerDecisionProjectionRow{values: rows.current}.Scan(dest...)
}

func (*ownerDecisionProjectionRows) Close()     {}
func (*ownerDecisionProjectionRows) Err() error { return nil }

type ownerDecisionProjectionRow struct{ values []any }

func (row ownerDecisionProjectionRow) Scan(dest ...any) error {
	if len(dest) != len(row.values) {
		return errors.New("owner projection scan destination count mismatch")
	}
	for i, destination := range dest {
		if scanner, ok := destination.(interface{ Scan(any) error }); ok {
			if err := scanner.Scan(row.values[i]); err != nil {
				return err
			}
			continue
		}
		target := reflect.ValueOf(destination)
		if target.Kind() != reflect.Ptr || target.IsNil() {
			return errors.New("owner projection scan target is invalid")
		}
		target = target.Elem()
		if row.values[i] == nil {
			target.SetZero()
			continue
		}
		value := reflect.ValueOf(row.values[i])
		if target.Kind() == reflect.Ptr {
			valueType := target.Type().Elem()
			if !value.Type().AssignableTo(valueType) && !value.Type().ConvertibleTo(valueType) {
				return errors.New("owner projection scan value has an incompatible type")
			}
			pointer := reflect.New(valueType)
			if value.Type() != valueType {
				value = value.Convert(valueType)
			}
			pointer.Elem().Set(value)
			target.Set(pointer)
			continue
		}
		if !value.Type().AssignableTo(target.Type()) && !value.Type().ConvertibleTo(target.Type()) {
			return errors.New("owner projection scan value has an incompatible type")
		}
		if value.Type() != target.Type() {
			value = value.Convert(target.Type())
		}
		target.Set(value)
	}
	return nil
}

func TestOwnerCaseListAndDetailProjectOnlyTheMatchingDecisionEvent(t *testing.T) {
	for _, mode := range []string{"list", "detail"} {
		t.Run(mode, func(t *testing.T) {
			tx := &ownerDecisionProjectionTx{}
			repo := newWithTransactionBeginner(ownerDecisionProjectionBeginner{tx: tx})
			var got dv.Snapshot
			var err error
			if mode == "list" {
				var cases []dv.Snapshot
				cases, err = repo.ListOwnCases(context.Background(), ownerDecisionProjectionActor)
				if err == nil && len(cases) != 1 {
					err = errors.New("owner case list did not return exactly one synthetic case")
				}
				if err == nil {
					got = cases[0]
				}
			} else {
				got, err = repo.LoadOwnCase(context.Background(), ownerDecisionProjectionActor, ownerDecisionProjectionCaseID)
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.OwnerDecisionReason != dv.ReasonEvidenceIncomplete {
				t.Fatalf("OwnerDecisionReason=%q, want %q", got.OwnerDecisionReason, dv.ReasonEvidenceIncomplete)
			}
			if !tx.committed {
				t.Fatal("owner read transaction was not committed")
			}

			query, args := tx.detailQuery, tx.detailArgs
			if mode == "list" {
				query, args = tx.listQuery, tx.listArgs
			}
			for _, fragment := range []string{
				"LEFT JOIN LATERAL",
				"c.state='requiere_antecedentes'",
				"e.case_id=c.id",
				"e.subject_id=c.subject_id",
				"e.action='review_decision'",
				"e.to_state=c.state",
				"e.revision<=c.revision",
				"COALESCE(owner_decision.reason_code,'')",
			} {
				if !strings.Contains(query, fragment) {
					t.Errorf("owner %s query lacks %q", mode, fragment)
				}
			}
			if strings.Contains(query, "'rechazada'") || strings.Contains(query, "'revocada'") {
				t.Errorf("owner %s query projects a decision reason outside the correction state", mode)
			}
			ownerScope := "WHERE c.subject_id=$1"
			if mode == "detail" {
				ownerScope = "WHERE c.id=$1 AND c.subject_id=$2"
			}
			if !strings.Contains(query, ownerScope) {
				t.Errorf("owner %s query lacks owner scope %q", mode, ownerScope)
			}
			if mode == "list" && !reflect.DeepEqual(args, []any{ownerDecisionProjectionActor}) {
				t.Errorf("list query args=%#v", args)
			}
			if mode == "detail" && !reflect.DeepEqual(args, []any{ownerDecisionProjectionCaseID, ownerDecisionProjectionActor}) {
				t.Errorf("detail query args=%#v", args)
			}
		})
	}
}
