package documentverificationpg

import (
	"context"
	"errors"
	"testing"
	"time"

	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type rejectingScanOutcomeBeginner struct{ called bool }

func (b *rejectingScanOutcomeBeginner) BeginTx(context.Context, pgx.TxOptions) (pgx.Tx, error) {
	b.called = true
	return nil, errors.New("unexpected transaction")
}

func TestRecordScanOutcomeRejectsBlockedWithoutScannerMetadata(t *testing.T) {
	for _, tc := range []struct {
		name    string
		version string
		signed  time.Time
	}{{name: "missing version", signed: time.Now().UTC()}, {name: "missing signature time", version: "ClamAV 1.0"}} {
		t.Run(tc.name, func(t *testing.T) {
			beginner := &rejectingScanOutcomeBeginner{}
			repo := newWithTransactionBeginner(beginner)
			claim := ScanClaim{
				JobID:      uuid.NewString(),
				FileID:     uuid.NewString(),
				CaseID:     uuid.NewString(),
				LeaseUntil: time.Now().Add(time.Minute),
			}
			outcome := ScanOutcome{
				State:          dv.FileBlocked,
				ErrorCode:      "malware_detected",
				ReasonCode:     dv.ReasonMalwareDetected,
				ScannerVersion: tc.version,
				SignaturesAt:   tc.signed,
			}
			if err := repo.RecordScanOutcome(context.Background(), claim, outcome); !errors.Is(err, dv.ErrInvalid) {
				t.Fatalf("RecordScanOutcome error=%v, want ErrInvalid", err)
			}
			if beginner.called {
				t.Fatal("invalid blocked outcome reached PostgreSQL")
			}
		})
	}
}
