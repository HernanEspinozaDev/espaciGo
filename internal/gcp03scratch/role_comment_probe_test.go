package gcp03scratch

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyRoleCommentErrorRecordsPermissionDenialWithoutLeakingDetails(t *testing.T) {
	outcome, state, expected := classifyRoleCommentError(&pgconn.PgError{
		Code:          "42501",
		Message:       "permission denied for a fixed role",
		Detail:        "sensitive server detail must not be returned",
		InternalQuery: "COMMENT ON ROLE ...",
	})
	if outcome != "permission_denied" || state != "42501" || !expected {
		t.Fatalf("unexpected permission-denial result: %q %q %t", outcome, state, expected)
	}
}

func TestClassifyRoleCommentErrorStopsOnUnexpectedFailures(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "unexpected SQLSTATE", err: &pgconn.PgError{Code: "42704", Message: "secret"}, want: "unexpected_sql_error"},
		{name: "transport", err: errors.New("secret transport detail"), want: "non_postgresql_error"},
	} {
		t.Run(test.name, func(t *testing.T) {
			outcome, _, expected := classifyRoleCommentError(test.err)
			if outcome != test.want || expected {
				t.Fatalf("unexpected failure classification: %q %t", outcome, expected)
			}
		})
	}
}
