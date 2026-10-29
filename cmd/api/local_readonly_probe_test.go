package main

import (
	"context"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5"
)

type probeFakeTx struct {
	readOnly              bool
	queries               []string
	committed, rolledBack bool
	database, role        string
}

func (tx *probeFakeTx) QueryRow(_ context.Context, query string, _ ...any) pgx.Row {
	tx.queries = append(tx.queries, query)
	return probeFakeRow{database: tx.database, role: tx.role}
}
func (tx *probeFakeTx) Commit(context.Context) error { tx.committed = true; return nil }
func (tx *probeFakeTx) Rollback(context.Context) error {
	if !tx.committed {
		tx.rolledBack = true
	}
	return nil
}

type probeFakeRow struct{ database, role string }

func (row probeFakeRow) Scan(dest ...any) error {
	if len(dest) == 2 {
		*(dest[0].(*string)) = row.database
		*(dest[1].(*string)) = row.role
	}
	return nil
}

type probeFakeDB struct{ tx *probeFakeTx }

func (db probeFakeDB) BeginTx(_ context.Context, opts pgx.TxOptions) (probeTx, error) {
	db.tx.readOnly = opts.AccessMode == pgx.ReadOnly
	return db.tx, nil
}

func newProbeFakeTx() *probeFakeTx {
	return &probeFakeTx{database: probeDatabase, role: probeRole}
}

func TestLocalReadOnlyProbeAllowsOnlyExactTarget(t *testing.T) {
	base := baseProbeTarget()
	if err := validateLocalReadOnlyTarget(base); err != nil {
		t.Fatal(err)
	}
	cases := []config{base, base, base, base, base}
	cases[0].dbHost = "localhost"
	cases[1].dbPort = 5433
	cases[2].dbName = "other"
	cases[3].dbUser = "other"
	cases[4].dbConnectionMode = "cloudsql"
	for i, cfg := range cases {
		if validateLocalReadOnlyTarget(cfg) == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	base.dbSetRole = probeRole
	if validateLocalReadOnlyTarget(base) == nil {
		t.Fatal("accepted DATABASE_SET_ROLE")
	}
}

func TestProbeURLUsesPassfileWithoutPassword(t *testing.T) {
	cfg := baseProbeTarget()
	cfg.dbSSLMode = "disable"
	parsed, err := url.Parse(localReadOnlyProbeURL(cfg))
	if err != nil {
		t.Fatal(err)
	}
	if _, hasPassword := parsed.User.Password(); hasPassword {
		t.Fatal("passfile URL contains password")
	}
	if parsed.User.Username() != probeRole {
		t.Fatalf("user=%q", parsed.User.Username())
	}
}

func TestProbeUsesReadOnlyTransactionAndInnocuousQueries(t *testing.T) {
	tx := newProbeFakeTx()
	if err := executeLocalReadOnlyProbe(context.Background(), probeFakeDB{tx}); err != nil {
		t.Fatal(err)
	}
	if !tx.readOnly || !tx.committed || tx.rolledBack || len(tx.queries) != 2 {
		t.Fatalf("tx=%+v queries=%v", tx, tx.queries)
	}
	if tx.queries[0] != probeIdentityQuery || tx.queries[1] != probeSpaceAggregationQuery {
		t.Fatalf("queries=%q", tx.queries)
	}
}

func TestProbeRejectsDifferentDatabaseIdentityBeforeSpaceQuery(t *testing.T) {
	tx := newProbeFakeTx()
	tx.database = "other"
	if err := executeLocalReadOnlyProbe(context.Background(), probeFakeDB{tx}); err == nil {
		t.Fatal("accepted different database identity")
	}
	if len(tx.queries) != 1 || tx.queries[0] != probeIdentityQuery {
		t.Fatalf("unexpected queries: %q", tx.queries)
	}
}

func TestProbeRejectsMutatingProfileAndUnusedPasswordFile(t *testing.T) {
	for _, key := range []string{
		"APP_PROFILE",
		"LOCAL_AUTH_PROTOTYPE",
		"LOCAL_BOOKING_TRIAL",
		"LOCAL_DATASET_DEALS_ISOLATED_CLOCK",
		"DATABASE_PASSWORD_FILE",
		"PGPASSWORD",
	} {
		t.Run(key, func(t *testing.T) {
			if err := validateLocalReadOnlyProbeEnvironment(func(name string) string {
				if name == key {
					return "set"
				}
				return ""
			}); err == nil {
				t.Fatalf("accepted conflicting %s", key)
			}
		})
	}
	if err := validateLocalReadOnlyProbeEnvironment(func(string) string { return "" }); err != nil {
		t.Fatalf("rejected clean probe environment: %v", err)
	}
}

func TestProbeRejectsConnectionEnvironmentOverrides(t *testing.T) {
	for _, key := range []string{"PGSERVICE", "PGHOST", "PGOPTIONS"} {
		t.Run(key, func(t *testing.T) {
			if err := validateLocalReadOnlyProbeEnvironment(func(name string) string {
				if name == key {
					return "set"
				}
				return ""
			}); err == nil {
				t.Fatalf("accepted %s", key)
			}
		})
	}
}

func TestProbeRejectsEffectiveTargetMismatch(t *testing.T) {
	cases := []func(*pgx.ConnConfig){
		func(c *pgx.ConnConfig) { c.Host = "localhost" },
		func(c *pgx.ConnConfig) { c.Port++ },
		func(c *pgx.ConnConfig) { c.Database = "other" },
		func(c *pgx.ConnConfig) { c.User = "other" },
		func(c *pgx.ConnConfig) { c.Password = "" },
		func(c *pgx.ConnConfig) { c.Fallbacks = append(c.Fallbacks, nil) },
	}
	for i, mutate := range cases {
		c := &pgx.ConnConfig{Host: "127.0.0.1", Port: 5432, Database: probeDatabase, User: probeRole, Password: "nonempty"}
		mutate(c)
		if validateProbeConnConfig(c) == nil {
			t.Errorf("case %d accepted", i)
		}
	}
}
