package gcp03scratch

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestInsertWithDeadlockRetryRetriesOnlyDeadlockVictims(t *testing.T) {
	t.Run("retries deadlock with same callback then returns definitive overlap", func(t *testing.T) {
		calls := 0
		retries, err := insertWithDeadlockRetry(context.Background(), func(context.Context) error {
			calls++
			switch calls {
			case 1:
				return &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}
			case 2:
				return &pgconn.PgError{Code: "23P01", Message: "exclusion violation"}
			default:
				t.Fatalf("unexpected extra attempt %d", calls)
				return nil
			}
		})
		var pgErr *pgconn.PgError
		if calls != 2 || retries != 1 || !errors.As(err, &pgErr) || pgErr.Code != "23P01" {
			t.Fatalf("calls=%d retries=%d err=%v; want 2 calls, 1 retry, final SQLSTATE 23P01", calls, retries, err)
		}
	})

	t.Run("does not retry non-deadlock failures", func(t *testing.T) {
		calls := 0
		failure := &pgconn.PgError{Code: "23503", Message: "foreign key violation"}
		retries, err := insertWithDeadlockRetry(context.Background(), func(context.Context) error {
			calls++
			return failure
		})
		if calls != 1 || retries != 0 || !errors.Is(err, failure) {
			t.Fatalf("calls=%d retries=%d err=%v; want one call, zero retries, original error", calls, retries, err)
		}
	})

	t.Run("bounds repeated deadlocks", func(t *testing.T) {
		calls := 0
		retries, err := insertWithDeadlockRetry(context.Background(), func(context.Context) error {
			calls++
			return &pgconn.PgError{Code: "40P01", Message: "deadlock detected"}
		})
		var pgErr *pgconn.PgError
		if calls != concurrentDeadlockMaxAttempts || retries != concurrentDeadlockMaxAttempts-1 || !errors.As(err, &pgErr) || pgErr.Code != "40P01" {
			t.Fatalf("calls=%d retries=%d err=%v; want bounded deadlock failure after %d attempts", calls, retries, err, concurrentDeadlockMaxAttempts)
		}
	})
}
