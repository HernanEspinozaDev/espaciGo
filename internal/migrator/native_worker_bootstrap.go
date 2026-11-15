package migrator

import (
	"context"
	"errors"
	"fmt"
)

const nativeWorkerBootstrapDatabase = "espacigo_dms_stage_20261020"

type nativeWorkerBootstrapEvidence struct {
	Database                       string
	DatabaseOID                    uint32
	Caller, CurrentUser            string
	Superuser                      bool
	LedgerVersions                 int
	LedgerExact, AuthorityExact    bool
	RuntimePresent, RuntimeNoLogin bool
	CleanupPresent, CleanupNoLogin bool
	LoginAbsent, MembershipAbsent  bool
	TargetSessions                 int
}

type nativeWorkerBootstrapProposal struct{ statements []string }

// proposeNativeWorkerBootstrap accepts only an exact, preflighted target snapshot.
// It deliberately carries no credential material; the CLI owns credential creation.
func proposeNativeWorkerBootstrap(e nativeWorkerBootstrapEvidence, expectedOID uint32) (nativeWorkerBootstrapProposal, error) {
	if expectedOID == 0 || e.Database != nativeWorkerBootstrapDatabase || e.DatabaseOID != expectedOID {
		return nativeWorkerBootstrapProposal{}, errors.New("native worker bootstrap target identity mismatch")
	}
	if e.Caller != "postgres" || e.CurrentUser != "postgres" || !e.Superuser {
		return nativeWorkerBootstrapProposal{}, errors.New("native worker bootstrap requires the pinned administrator")
	}
	if e.LedgerVersions != 85 || !e.LedgerExact || !e.AuthorityExact {
		return nativeWorkerBootstrapProposal{}, errors.New("native worker bootstrap requires exact V1..V85 ledger and authority")
	}
	if !e.RuntimePresent || !e.RuntimeNoLogin || !e.CleanupPresent || !e.CleanupNoLogin {
		return nativeWorkerBootstrapProposal{}, errors.New("native worker bootstrap found unsafe prerequisite roles")
	}
	if !e.LoginAbsent || !e.MembershipAbsent {
		return nativeWorkerBootstrapProposal{}, errors.New("native worker bootstrap refuses pre-existing login or membership drift")
	}
	if e.TargetSessions != 0 {
		return nativeWorkerBootstrapProposal{}, errors.New("native worker bootstrap refuses active target-role sessions")
	}
	return nativeWorkerBootstrapProposal{statements: []string{
		`CREATE ROLE espacigo_privacy_worker_login LOGIN NOINHERIT NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS`,
		`GRANT espacigo_privacy_cleanup_worker TO espacigo_privacy_worker_login WITH ADMIN FALSE, INHERIT FALSE, SET TRUE`,
	}}, nil
}

type nativeWorkerBootstrapTx interface {
	Exec(context.Context, string, ...any) error
	Readback(context.Context) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

func executeNativeWorkerBootstrap(ctx context.Context, tx nativeWorkerBootstrapTx, proposal nativeWorkerBootstrapProposal) (err error) {
	if tx == nil || len(proposal.statements) != 2 {
		return errors.New("invalid native worker bootstrap transaction proposal")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(context.Background())
		}
	}()
	for _, statement := range proposal.statements {
		if err = tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("native worker bootstrap statement failed")
		}
	}
	if err = tx.Readback(ctx); err != nil {
		return errors.New("native worker bootstrap readback failed")
	}
	if err = tx.Commit(ctx); err != nil {
		return errors.New("native worker bootstrap commit failed")
	}
	return nil
}
