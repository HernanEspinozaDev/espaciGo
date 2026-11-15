package migrator

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	rentalflowpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/rentalflow"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// assertSuppressionSerializesCommercialQuote forces the real suppression path
// to hold the account advisory lock while waiting for its row lock. A concurrent
// quote must then wait on that same advisory lock and reject after suppression
// commits, leaving no quote for the retired account.
func assertSuppressionSerializesCommercialQuote(
	t *testing.T,
	ctx context.Context,
	adminURL string,
	admin *pgx.Conn,
	service *privacy.Service,
	adminID, requestID, renterID, spaceID string,
	executeAt time.Time,
) (privacy.SuppressionExecution, error) {
	t.Helper()

	quoteURL, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	quoteURL.User = url.User("espacigo_api_login")
	quoteConfig, err := pgxpool.ParseConfig(quoteURL.String())
	if err != nil {
		t.Fatal(err)
	}
	quoteConfig.MaxConns = 1
	quoteConfig.ConnConfig.RuntimeParams["application_name"] = "commercial-quote-writer-test"
	quoteConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	quotePool, err := pgxpool.NewWithConfig(ctx, quoteConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer quotePool.Close()

	quoteRepository, err := rentalflowpg.NewRepository(quotePool, quotePool, func() time.Time { return executeAt })
	if err != nil {
		t.Fatal(err)
	}
	listing, err := quoteRepository.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	date := executeAt.AddDate(0, 0, 1).Format("2006-01-02")
	intervals, err := quoteRepository.PublicIntervals(ctx, listing, date, 1)
	if err != nil || len(intervals) == 0 {
		t.Fatalf("load quote interval for suppression concurrency test: intervals=%d err=%v", len(intervals), err)
	}

	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	observer, err := pgx.Connect(raceCtx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close(context.Background())

	// Pause suppression after it takes the advisory lock but before it can
	// update the account. This lets the test observe both real transactions at
	// their intended lock waits instead of relying on sleeps or scheduling luck.
	rowLockTx, err := admin.Begin(raceCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rowLockTx.Rollback(context.Background()) }()
	var rowLockBackendPID int32
	if err := rowLockTx.QueryRow(raceCtx, `SELECT pg_backend_pid()`).Scan(&rowLockBackendPID); err != nil {
		t.Fatal(err)
	}
	var lockedRenter string
	if err := rowLockTx.QueryRow(raceCtx, `SELECT id::text FROM public.usuario WHERE id=$1 FOR UPDATE`, renterID).Scan(&lockedRenter); err != nil {
		t.Fatal(err)
	}

	type suppressionOutcome struct {
		result privacy.SuppressionExecution
		err    error
	}
	suppressionDone := make(chan suppressionOutcome, 1)
	go func() {
		result, err := service.ExecuteSuppression(raceCtx, adminID, requestID, "commercial-quote-suppression", "commercial-quote-suppression", func() time.Time { return executeAt }, nil)
		suppressionDone <- suppressionOutcome{result: result, err: err}
	}()
	waitForBlockedBy := func(applicationName string, blockerPID int32) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var blocked bool
			if err := observer.QueryRow(raceCtx, `SELECT EXISTS(
				SELECT 1 FROM pg_catalog.pg_stat_activity a
WHERE a.application_name=$1 AND a.state='active' AND a.wait_event_type='Lock'
  AND $2=ANY(pg_catalog.pg_blocking_pids(a.pid)))`, applicationName, blockerPID).Scan(&blocked); err != nil {
				t.Fatal(err)
			}
			if blocked {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s to block on backend PID %d", applicationName, blockerPID)
			}
			select {
			case <-raceCtx.Done():
				t.Fatalf("context ended waiting for %s to block on backend PID %d: %v", applicationName, blockerPID, raceCtx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	waitForAdvisoryLockWait := func(applicationName, accountID string) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var blocked bool
			if err := observer.QueryRow(raceCtx, `SELECT EXISTS(
SELECT 1
FROM pg_catalog.pg_stat_activity a
JOIN pg_catalog.pg_locks l ON l.pid=a.pid
WHERE a.application_name=$1 AND a.state='active' AND a.wait_event_type='Lock'
  AND l.locktype='advisory' AND NOT l.granted AND l.objsubid=1
  AND l.classid=((hashtextextended($2::text,0)>>32)&4294967295)::oid
  AND l.objid=(hashtextextended($2::text,0)&4294967295)::oid)`, applicationName, accountID).Scan(&blocked); err != nil {
				t.Fatal(err)
			}
			if blocked {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for advisory lock on account %s for application %s", accountID, applicationName)
			}
			select {
			case <-raceCtx.Done():
				t.Fatalf("context ended waiting for advisory lock on account %s: %v", accountID, raceCtx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	waitForBlockedBy("commercial-quote-suppression-test", rowLockBackendPID)

	type quoteOutcome struct{ err error }
	quoteDone := make(chan quoteOutcome, 1)
	go func() {
		_, err := quoteRepository.CreateValidatedQuote(raceCtx, renterID, listing, intervals[0], 1, 8000, executeAt)
		quoteDone <- quoteOutcome{err: err}
	}()
	waitForAdvisoryLockWait("commercial-quote-writer-test", renterID)

	if err := rowLockTx.Commit(raceCtx); err != nil {
		t.Fatal(err)
	}
	var result privacy.SuppressionExecution
	select {
	case outcome := <-suppressionDone:
		result = outcome.result
		if outcome.err != nil {
			return privacy.SuppressionExecution{}, outcome.err
		}
	case <-raceCtx.Done():
		t.Fatalf("suppression did not finish after row lock release: %v", raceCtx.Err())
	}
	select {
	case outcome := <-quoteDone:
		if !errors.Is(outcome.err, rentalflow.ErrNotFound) {
			t.Fatalf("concurrent quote should reject the suppressed renter as not found, got %v", outcome.err)
		}
	case <-raceCtx.Done():
		t.Fatalf("quote did not finish after suppression released its advisory lock: %v", raceCtx.Err())
	}

	var renterQuotes int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE arrendatario_id=$1`, renterID).Scan(&renterQuotes); err != nil || renterQuotes != 0 {
		t.Fatalf("concurrent suppression left quotes for renter: count=%d err=%v", renterQuotes, err)
	}
	return result, nil
}

// assertQuoteBeforeSuppressionDeletesCommercialQuote exercises the opposite
// lock order: quote owns the account locks first, suppression waits, then
// suppression must delete the committed quote before anonymizing the renter.
func assertQuoteBeforeSuppressionDeletesCommercialQuote(
	t *testing.T,
	ctx context.Context,
	adminURL string,
	admin *pgx.Conn,
	service *privacy.Service,
	adminID, renterID, spaceID string,
) {
	t.Helper()
	request, err := service.RequestRight(ctx, renterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	reviewAt := time.Now().UTC().Truncate(time.Microsecond)
	review, err := service.ReviewSuppression(ctx, adminID, request.ID, "quote-first-suppression-review", "quote-first-suppression", func() time.Time { return reviewAt })
	if err != nil || review.Outcome != "elegible" {
		t.Fatalf("review quote-first renter suppression: outcome=%s err=%v", review.Outcome, err)
	}

	quoteURL, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	quoteURL.User = url.User("espacigo_api_login")
	quoteConfig, err := pgxpool.ParseConfig(quoteURL.String())
	if err != nil {
		t.Fatal(err)
	}
	quoteConfig.MaxConns = 1
	quoteConfig.ConnConfig.RuntimeParams["application_name"] = "commercial-quote-writer-test"
	quoteConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	quotePool, err := pgxpool.NewWithConfig(ctx, quoteConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer quotePool.Close()
	executeAt := time.Now().UTC().Add(time.Minute).Truncate(time.Microsecond)
	quoteRepository, err := rentalflowpg.NewRepository(quotePool, quotePool, func() time.Time { return executeAt })
	if err != nil {
		t.Fatal(err)
	}
	listing, err := quoteRepository.PublicListing(ctx, spaceID)
	if err != nil {
		t.Fatal(err)
	}
	intervals, err := quoteRepository.PublicIntervals(ctx, listing, executeAt.AddDate(0, 0, 1).Format("2006-01-02"), 1)
	if err != nil || len(intervals) == 0 {
		t.Fatalf("load quote interval for quote-first suppression test: intervals=%d err=%v", len(intervals), err)
	}

	raceCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	observer, err := pgx.Connect(raceCtx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer observer.Close(context.Background())
	spaceLockTx, err := admin.Begin(raceCtx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = spaceLockTx.Rollback(context.Background()) }()
	var spaceLockBackendPID int32
	if err := spaceLockTx.QueryRow(raceCtx, `SELECT pg_backend_pid()`).Scan(&spaceLockBackendPID); err != nil {
		t.Fatal(err)
	}
	var lockedSpace string
	if err := spaceLockTx.QueryRow(raceCtx, `SELECT id::text FROM public.espacio WHERE id=$1 FOR UPDATE`, spaceID).Scan(&lockedSpace); err != nil {
		t.Fatal(err)
	}

	type quoteOutcome struct {
		quote rentalflow.Quote
		err   error
	}
	quoteDone := make(chan quoteOutcome, 1)
	go func() {
		quote, err := quoteRepository.CreateValidatedQuote(raceCtx, renterID, listing, intervals[0], 1, 8000, executeAt)
		quoteDone <- quoteOutcome{quote: quote, err: err}
	}()
	waitForBlockedBy := func(applicationName string, blockerPID int32) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var blocked bool
			if err := observer.QueryRow(raceCtx, `SELECT EXISTS(
				SELECT 1 FROM pg_catalog.pg_stat_activity a
WHERE a.application_name=$1 AND a.state='active' AND a.wait_event_type='Lock'
  AND $2=ANY(pg_catalog.pg_blocking_pids(a.pid)))`, applicationName, blockerPID).Scan(&blocked); err != nil {
				t.Fatal(err)
			}
			if blocked {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s to block on backend PID %d", applicationName, blockerPID)
			}
			select {
			case <-raceCtx.Done():
				t.Fatalf("context ended waiting for %s to block on backend PID %d: %v", applicationName, blockerPID, raceCtx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	waitForAdvisoryLock := func(applicationName, accountID string, granted bool) {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for {
			var present bool
			if err := observer.QueryRow(raceCtx, `SELECT EXISTS(
SELECT 1
FROM pg_catalog.pg_stat_activity a
JOIN pg_catalog.pg_locks l ON l.pid=a.pid
WHERE a.application_name=$1 AND a.state='active'
  AND l.locktype='advisory' AND l.granted=$2 AND l.objsubid=1
  AND l.classid=((hashtextextended($3::text,0)>>32)&4294967295)::oid
  AND l.objid=(hashtextextended($3::text,0)&4294967295)::oid)`, applicationName, granted, accountID).Scan(&present); err != nil {
				t.Fatal(err)
			}
			if present {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for advisory lock state granted=%v on account %s for application %s", granted, accountID, applicationName)
			}
			select {
			case <-raceCtx.Done():
				t.Fatalf("context ended waiting for advisory lock state granted=%v: %v", granted, raceCtx.Err())
			case <-time.After(10 * time.Millisecond):
			}
		}
	}
	waitForBlockedBy("commercial-quote-writer-test", spaceLockBackendPID)
	waitForAdvisoryLock("commercial-quote-writer-test", renterID, true)

	suppressionDone := make(chan error, 1)
	go func() {
		result, err := service.ExecuteSuppression(raceCtx, adminID, request.ID, "quote-first-suppression", "quote-first-suppression", func() time.Time { return executeAt }, nil)
		if err == nil && result.Status != "completada" && result.Status != "limpieza_pendiente" {
			err = errors.New("quote-first suppression returned an unexpected status: " + result.Status)
		}
		suppressionDone <- err
	}()
	waitForAdvisoryLock("commercial-quote-suppression-test", renterID, false)
	if err := spaceLockTx.Commit(raceCtx); err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-quoteDone:
		if outcome.err != nil || outcome.quote.ID == "" {
			t.Fatalf("quote holding the account lock should commit before suppression: id=%q err=%v", outcome.quote.ID, outcome.err)
		}
	case <-raceCtx.Done():
		t.Fatalf("quote did not finish after releasing the space row lock: %v", raceCtx.Err())
	}
	select {
	case err := <-suppressionDone:
		if err != nil {
			t.Fatalf("suppression failed after quote committed: %v", err)
		}
	case <-raceCtx.Done():
		t.Fatalf("suppression did not finish after quote released its advisory lock: %v", raceCtx.Err())
	}
	var remaining int
	if err := admin.QueryRow(ctx, `SELECT count(*) FROM public.cotizacion_arriendo_comercial WHERE arrendatario_id=$1`, renterID).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatalf("suppression retained quote committed first: remaining=%d err=%v", remaining, err)
	}
}
