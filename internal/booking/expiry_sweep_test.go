package booking_test

import (
	"context"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
)

type expirySweepRepo struct {
	booking.Repository
	booking.GuaranteeLifecycleRepository
	booking.SettlementLifecycleRepository
	booking.PaymentLifecycleRepository
	expireDueGuaranteesCalls int
	expireCalls              int
	quoteContextDisabled     bool
	createContextDisabled    bool
}

func (r *expirySweepRepo) Expire(context.Context, time.Time) error {
	r.expireCalls++
	return nil
}

func (r *expirySweepRepo) ExpireDueGuarantees(context.Context, func() time.Time) error {
	r.expireDueGuaranteesCalls++
	return nil
}

func (r *expirySweepRepo) Quote(ctx context.Context, _ string, _, id string, start, end time.Time, _ func() time.Time, _ time.Duration) (booking.Quote, error) {
	r.quoteContextDisabled = booking.GlobalExpirySweepsDisabled(ctx)
	return booking.Quote{ID: id, StartAt: start, EndAt: end}, nil
}

func (r *expirySweepRepo) Create(ctx context.Context, _ string, quoteID, _ string, _ []byte, id, _ string, _ time.Duration, _ func() time.Time) (booking.Reservation, error) {
	r.createContextDisabled = booking.GlobalExpirySweepsDisabled(ctx)
	return booking.Reservation{ID: id, QuoteID: quoteID}, nil
}

func (r *expirySweepRepo) Get(context.Context, string, string) (booking.Detail, error) {
	return booking.Detail{}, nil
}

func (r *expirySweepRepo) Guarantee(context.Context, string, string) (booking.GuaranteeSnapshot, error) {
	return booking.GuaranteeSnapshot{}, nil
}

type paymentAdapter struct{}

func (paymentAdapter) StartPayment(context.Context, string, string) (*booking.PaymentEvent, error) {
	return nil, nil
}
func (paymentAdapter) LookupPayment(context.Context, booking.PaymentOperation) (*booking.PaymentEvent, error) {
	return nil, nil
}
func (paymentAdapter) VerifyPaymentEvent(booking.PaymentEvent) bool { return true }

const (
	renterID  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	spaceID   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	quoteID   = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
	bookingID = "dddddddd-dddd-4ddd-8ddd-dddddddddddd"
)

func newSweepService(t *testing.T, repo *expirySweepRepo, enabled bool, now func() time.Time) *booking.Service {
	t.Helper()
	service, err := booking.NewServiceWithTTLsAndGlobalExpirySweeps(repo, testIDs{}, now, paymentAdapter{}, time.Minute, time.Minute, time.Hour, enabled)
	if err != nil {
		t.Fatal(err)
	}
	return service
}

type testIDs struct{}

func (testIDs) ID() (string, error)             { return "eeeeeeee-eeee-4eee-8eee-eeeeeeeeeeee", nil }
func (testIDs) Token() (identity.Secret, error) { return identity.Secret("synthetic-token"), nil }

func TestIsolatedClockSkipsGlobalBusinessExpiryAndReconciliation(t *testing.T) {
	ctx := context.Background()
	future := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	repo := &expirySweepRepo{}
	service := newSweepService(t, repo, false, func() time.Time { return future })

	if _, err := service.Get(ctx, renterID, bookingID); err != nil {
		t.Fatal(err)
	}
	start := future.Add(time.Hour)
	if _, err := service.Quote(ctx, renterID, booking.QuoteInput{
		SpaceID: spaceID, StartAt: start.Format(time.RFC3339), EndAt: start.Add(time.Hour).Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Request(ctx, renterID, booking.RequestInput{QuoteID: quoteID}, "retry-key"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Guarantee(ctx, renterID, bookingID); err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcilePendingPayments(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileGuarantees(ctx); err != nil {
		t.Fatal(err)
	}
	if err := service.ReconcileSettlements(ctx); err != nil {
		t.Fatal(err)
	}
	service.RunPaymentReconciler(ctx, time.Millisecond)
	service.RunGuaranteeReconciler(ctx, time.Millisecond)
	service.RunSettlementReconciler(ctx, time.Millisecond)
	if repo.expireCalls != 0 || repo.expireDueGuaranteesCalls != 0 {
		t.Fatalf("isolated clock invoked global sweeps: bookings=%d guarantees=%d", repo.expireCalls, repo.expireDueGuaranteesCalls)
	}
	if !repo.quoteContextDisabled || !repo.createContextDisabled {
		t.Fatalf("quote/create did not carry the global-sweep guard: quote=%v create=%v", repo.quoteContextDisabled, repo.createContextDisabled)
	}
}

func TestGlobalBusinessExpiryRemainsEnabledByDefault(t *testing.T) {
	repo := &expirySweepRepo{}
	service, err := booking.NewServiceWithTTLs(repo, testIDs{}, time.Now, paymentAdapter{}, time.Minute, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Get(context.Background(), renterID, bookingID); err != nil {
		t.Fatal(err)
	}
	if repo.expireCalls != 1 {
		t.Fatalf("default service should preserve global expiration; got %d calls", repo.expireCalls)
	}
}
