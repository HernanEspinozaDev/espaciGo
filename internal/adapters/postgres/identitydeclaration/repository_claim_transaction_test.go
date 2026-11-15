package identitydeclaration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type revisionTxFake struct {
	pgx.Tx
	owner                  string
	verified               *time.Time
	auditErr               error
	updateRows             int64
	revision, claim, audit bool
	commits, rollbacks     int
}
type revisionRowFake func(...any) error

func (r revisionRowFake) Scan(dest ...any) error { return r(dest...) }
func (f *revisionTxFake) QueryRow(_ context.Context, sql string, _ ...any) pgx.Row {
	switch {
	case strings.HasPrefix(sql, "SELECT COALESCE(MAX(revision)"):
		return revisionRowFake(func(d ...any) error { *d[0].(*int64) = 1; return nil })
	case strings.HasPrefix(sql, "INSERT INTO public.identity_declaration_revision"):
		return revisionRowFake(func(d ...any) error {
			f.revision = true
			*d[0].(*string) = "85000000-0000-4000-8000-000000000002"
			*d[1].(*string) = "85000000-0000-4000-8000-000000000001"
			*d[2].(*int64) = 1
			*d[3].(*string) = "v1"
			*d[4].(*[]byte) = bytesForRevision()
			*d[5].(*string) = "1"
			*d[6].(*string) = "consent-v1"
			*d[7].(*time.Time) = testConsentTime
			*d[8].(*time.Time) = testConsentTime.Add(time.Hour)
			*d[9].(*time.Time) = testConsentTime
			return nil
		})
	case strings.HasPrefix(sql, "SELECT account_id::text,COALESCE(declaration_id::text,''),verified_at FROM public.identity_rut_claim"):
		return revisionRowFake(func(d ...any) error {
			if f.owner == "" {
				return pgx.ErrNoRows
			}
			*d[0].(*string) = f.owner
			*d[1].(*string) = ""
			*d[2].(**time.Time) = f.verified
			return nil
		})
	default:
		panic("unexpected QueryRow: " + sql)
	}
}

var testConsentTime = time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC)

func bytesForRevision() []byte { return make([]byte, 29) }
func (f *revisionTxFake) Exec(_ context.Context, sql string, _ ...any) (pgconn.CommandTag, error) {
	switch {
	case strings.HasPrefix(sql, "SELECT pg_advisory_xact_lock"):
		return pgconn.NewCommandTag("SELECT 1"), nil
	case strings.HasPrefix(sql, "INSERT INTO public.identity_rut_claim"):
		f.claim = true
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.HasPrefix(sql, "UPDATE public.identity_rut_claim SET declaration_id"):
		return pgconn.NewCommandTag("UPDATE " + itoa(int(f.updateRows))), nil
	case strings.HasPrefix(sql, "INSERT INTO public.evento_auditoria_local"):
		f.audit = true
		return pgconn.NewCommandTag("INSERT 0 1"), f.auditErr
	default:
		panic("unexpected Exec: " + sql)
	}
}
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	return "1"
}
func (f *revisionTxFake) Commit(context.Context) error { f.commits++; return nil }
func (f *revisionTxFake) Rollback(context.Context) error {
	f.rollbacks++
	f.revision = false
	f.claim = false
	f.audit = false
	return nil
}
func fixtureInput(fp string) RevisionInput {
	return RevisionInput{ID: "85000000-0000-4000-8000-000000000002", AccountID: "85000000-0000-4000-8000-000000000001", CorrelationID: "corr", RutFingerprint: fp, SchemaVersion: "1", ConsentVersion: "consent-v1", ConsentGivenAt: testConsentTime, RetentionAt: testConsentTime.Add(time.Hour)}
}
func goodSeal(int64) (string, []byte, error) { return "v1", bytesForRevision(), nil }

func TestCreateRevisionTransactionHappyPath(t *testing.T) {
	f := &revisionTxFake{}
	in := fixtureInput("v1:" + strings.Repeat("a", 64))
	_, err := createRevisionInTx(context.Background(), f, in, goodSeal)
	if err != nil {
		t.Fatal(err)
	}
	if !f.revision || !f.claim || !f.audit {
		t.Fatalf("writes revision/claim/audit=%v/%v/%v", f.revision, f.claim, f.audit)
	}
	if err = f.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.commits != 1 {
		t.Fatalf("commits=%d", f.commits)
	}
}
func TestCreateRevisionTransactionOtherOwnerRollsBack(t *testing.T) {
	f := &revisionTxFake{owner: "85000000-0000-4000-8000-000000000003"}
	_, err := createRevisionInTx(context.Background(), f, fixtureInput("v1:"+strings.Repeat("a", 64)), goodSeal)
	if !errors.Is(err, verification.ErrConflict) {
		t.Fatalf("err=%v", err)
	}
	_ = f.Rollback(context.Background())
	if f.revision || f.claim || f.audit || f.commits != 0 {
		t.Fatalf("durable fake writes after rollback: %+v", f)
	}
}
func TestCreateRevisionTransactionAuditErrorRollsBack(t *testing.T) {
	f := &revisionTxFake{auditErr: errors.New("audit failed")}
	_, err := createRevisionInTx(context.Background(), f, fixtureInput("v1:"+strings.Repeat("a", 64)), goodSeal)
	if err == nil {
		t.Fatal("expected audit error")
	}
	_ = f.Rollback(context.Background())
	if f.revision || f.claim || f.audit {
		t.Fatalf("writes remain after rollback: %+v", f)
	}
}
func TestCreateRevisionTransactionOwnClaimUpdateAndVerifiedPreserved(t *testing.T) {
	f := &revisionTxFake{owner: "85000000-0000-4000-8000-000000000001", updateRows: 1}
	if _, err := createRevisionInTx(context.Background(), f, fixtureInput("v1:"+strings.Repeat("a", 64)), goodSeal); err != nil {
		t.Fatal(err)
	}
	if !f.revision || !f.audit || f.claim {
		t.Fatalf("pending claim path: %+v", f)
	}
	verified := testConsentTime
	f = &revisionTxFake{owner: "85000000-0000-4000-8000-000000000001", verified: &verified}
	if _, err := createRevisionInTx(context.Background(), f, fixtureInput("v1:"+strings.Repeat("a", 64)), goodSeal); err != nil {
		t.Fatal(err)
	}
	if f.claim {
		t.Fatal("verified claim was updated")
	}
}
func TestCreateRevisionTransactionZeroAffectedUpdateConflicts(t *testing.T) {
	f := &revisionTxFake{owner: "85000000-0000-4000-8000-000000000001", updateRows: 0}
	err := claimRutForRevision(context.Background(), f, "85000000-0000-4000-8000-000000000001", "85000000-0000-4000-8000-000000000002", "v1:"+strings.Repeat("a", 64), testConsentTime.Add(time.Hour), testConsentTime)
	if !errors.Is(err, verification.ErrConflict) {
		t.Fatalf("zero-row update err=%v", err)
	}
}
func TestCreateRevisionRejectsMalformedFingerprintBeforeSeal(t *testing.T) {
	called := false
	in := fixtureInput("malformed")
	_, err := (*Repository)(nil).CreateRevision(context.Background(), in, func(int64) (string, []byte, error) { called = true; return "v1", bytesForRevision(), nil })
	if !errors.Is(err, verification.ErrInvalid) || called {
		t.Fatalf("err=%v seal called=%v", err, called)
	}
}
