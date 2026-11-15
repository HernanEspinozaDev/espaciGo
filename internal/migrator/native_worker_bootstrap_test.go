package migrator

import (
	"context"
	"errors"
	"testing"
)

func TestNativeWorkerBootstrapRejectsUnsafePreflightBeforeDDL(t *testing.T) {
	valid := nativeWorkerBootstrapEvidence{
		Database: "espacigo_dms_stage_20261020", DatabaseOID: 42, Caller: "postgres", CurrentUser: "postgres", Superuser: true,
		LedgerVersions: 85, LedgerExact: true, AuthorityExact: true,
		RuntimePresent: true, RuntimeNoLogin: true, CleanupPresent: true, CleanupNoLogin: true,
		LoginAbsent: true, MembershipAbsent: true, TargetSessions: 0,
	}
	cases := map[string]func(*nativeWorkerBootstrapEvidence){
		"database":            func(e *nativeWorkerBootstrapEvidence) { e.Database = "other" },
		"oid":                 func(e *nativeWorkerBootstrapEvidence) { e.DatabaseOID++ },
		"caller":              func(e *nativeWorkerBootstrapEvidence) { e.Caller = "app" },
		"unsafe runtime":      func(e *nativeWorkerBootstrapEvidence) { e.RuntimeNoLogin = false },
		"unsafe cleanup role": func(e *nativeWorkerBootstrapEvidence) { e.CleanupNoLogin = false },
		"login exists":        func(e *nativeWorkerBootstrapEvidence) { e.LoginAbsent = false },
		"membership drift":    func(e *nativeWorkerBootstrapEvidence) { e.MembershipAbsent = false },
		"sessions":            func(e *nativeWorkerBootstrapEvidence) { e.TargetSessions = 1 },
		"ledger":              func(e *nativeWorkerBootstrapEvidence) { e.LedgerExact = false },
		"authority":           func(e *nativeWorkerBootstrapEvidence) { e.AuthorityExact = false },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			e := valid
			mutate(&e)
			if _, err := proposeNativeWorkerBootstrap(e, 42); err == nil {
				t.Fatal("unsafe evidence accepted")
			}
		})
	}
}

type workerBootstrapFakeTx struct {
	failAt     int
	execs      int
	rolledBack bool
	committed  bool
	readback   bool
}

func (f *workerBootstrapFakeTx) Exec(context.Context, string, ...any) error {
	f.execs++
	if f.failAt == f.execs {
		return errors.New("injected")
	}
	return nil
}
func (f *workerBootstrapFakeTx) Readback(context.Context) error { f.readback = true; return nil }
func (f *workerBootstrapFakeTx) Commit(context.Context) error   { f.committed = true; return nil }
func (f *workerBootstrapFakeTx) Rollback(context.Context) error { f.rolledBack = true; return nil }

func TestNativeWorkerBootstrapProposalRollsBackOnDDLFailure(t *testing.T) {
	e := nativeWorkerBootstrapEvidence{Database: "espacigo_dms_stage_20261020", DatabaseOID: 42, Caller: "postgres", CurrentUser: "postgres", Superuser: true, LedgerVersions: 85, LedgerExact: true, AuthorityExact: true, RuntimePresent: true, RuntimeNoLogin: true, CleanupPresent: true, CleanupNoLogin: true, LoginAbsent: true, MembershipAbsent: true}
	proposal, err := proposeNativeWorkerBootstrap(e, 42)
	if err != nil {
		t.Fatal(err)
	}
	f := &workerBootstrapFakeTx{failAt: 2}
	if err := executeNativeWorkerBootstrap(context.Background(), f, proposal); err == nil {
		t.Fatal("expected injected failure")
	}
	if !f.rolledBack || f.committed || f.readback {
		t.Fatalf("transaction outcome: %+v", f)
	}
}

func TestNativeWorkerBootstrapProposalRequiresReadbackBeforeCommit(t *testing.T) {
	e := nativeWorkerBootstrapEvidence{Database: "espacigo_dms_stage_20261020", DatabaseOID: 42, Caller: "postgres", CurrentUser: "postgres", Superuser: true, LedgerVersions: 85, LedgerExact: true, AuthorityExact: true, RuntimePresent: true, RuntimeNoLogin: true, CleanupPresent: true, CleanupNoLogin: true, LoginAbsent: true, MembershipAbsent: true}
	p, err := proposeNativeWorkerBootstrap(e, 42)
	if err != nil {
		t.Fatal(err)
	}
	f := &workerBootstrapFakeTx{}
	if err := executeNativeWorkerBootstrap(context.Background(), f, p); err != nil {
		t.Fatal(err)
	}
	if !f.readback || !f.committed || f.rolledBack {
		t.Fatalf("transaction outcome: %+v", f)
	}
}
