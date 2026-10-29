package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	probeIdentityQuery         = `SELECT current_database(), current_user`
	probeSpaceAggregationQuery = `SELECT count(*) FROM public.espacio`
	probeDatabase              = "espacigo_dms_stage_20261020"
	probeRole                  = "espacigo_runtime"
)

type probeTx interface {
	QueryRow(context.Context, string, ...any) pgx.Row
	Commit(context.Context) error
	Rollback(context.Context) error
}
type probeDB interface {
	BeginTx(context.Context, pgx.TxOptions) (probeTx, error)
}
type pgxProbeDB struct{ pool *pgxpool.Pool }

func (db pgxProbeDB) BeginTx(ctx context.Context, opts pgx.TxOptions) (probeTx, error) {
	return db.pool.BeginTx(ctx, opts)
}

type probeTxOptions = pgx.TxOptions

func baseProbeTarget() config {
	return config{dbConnectionMode: "direct", dbHost: "127.0.0.1", dbPort: 5432, dbName: probeDatabase, dbUser: probeRole}
}
func validateLocalReadOnlyTarget(c config) error {
	if c.dbConnectionMode != "direct" || c.dbHost != "127.0.0.1" || c.dbPort != 5432 || c.dbName != probeDatabase || c.dbUser != probeRole || c.dbSetRole != "" || c.cloudSQLInstance != "" || c.cloudSQLPrivateIP {
		return errors.New("target is not allowlisted")
	}
	return nil
}
func localReadOnlyProbeURL(c config) string {
	u := &url.URL{Scheme: "postgres", User: url.User(c.dbUser), Host: net.JoinHostPort(c.dbHost, strconv.Itoa(int(c.dbPort))), Path: "/" + c.dbName}
	q := url.Values{}
	q.Set("sslmode", c.dbSSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}
func executeLocalReadOnlyProbe(ctx context.Context, db probeDB) error {
	tx, err := db.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return errors.New("transaction unavailable")
	}
	defer tx.Rollback(ctx)
	var database, role string
	if err = tx.QueryRow(ctx, probeIdentityQuery).Scan(&database, &role); err != nil {
		return errors.New("identity query failed")
	}
	if database != probeDatabase || role != probeRole {
		return errors.New("database identity rejected")
	}
	var count int64
	if err = tx.QueryRow(ctx, probeSpaceAggregationQuery).Scan(&count); err != nil {
		return errors.New("aggregation query failed")
	}
	return tx.Commit(ctx)
}
func validateLocalReadOnlyProbeEnvironment(getenv func(string) string) error {
	for _, key := range []string{
		"APP_PROFILE",
		"LOCAL_AUTH_PROTOTYPE",
		"LOCAL_BOOKING_TRIAL",
		"LOCAL_DATASET_DEALS_ISOLATED_CLOCK",
	} {
		if getenv(key) != "" {
			return errors.New("incompatible API profile settings")
		}
	}
	for _, key := range []string{
		"PGHOST", "PGPORT", "PGDATABASE", "PGUSER", "PGPASSWORD",
		"PGAPPNAME", "PGCONNECT_TIMEOUT", "PGSSLMODE", "PGSSLKEY",
		"PGSSLCERT", "PGSSLSNI", "PGSSLROOTCERT", "PGSSLPASSWORD",
		"PGSSLNEGOTIATION", "PGTARGETSESSIONATTRS", "PGSERVICE",
		"PGSERVICEFILE", "PGTZ", "PGOPTIONS", "PGMINPROTOCOLVERSION",
		"PGMAXPROTOCOLVERSION", "PGCHANNELBINDING", "PGREQUIREAUTH",
		"DATABASE_PASSWORD_FILE",
	} {
		if getenv(key) != "" {
			return errors.New("unsupported PostgreSQL environment override")
		}
	}
	return nil
}

func validateProbeConnConfig(cc *pgx.ConnConfig) error {
	if cc == nil || cc.Host != "127.0.0.1" || cc.Port != 5432 || cc.Database != probeDatabase || cc.User != probeRole || cc.Password == "" || len(cc.Fallbacks) != 0 {
		return errors.New("effective target is not allowlisted")
	}
	return nil
}

func runLocalReadOnlyProbe() error {
	if err := validateLocalReadOnlyProbeEnvironment(os.Getenv); err != nil {
		return err
	}
	cfg, err := loadConfigMode(os.Getenv, true)
	if err != nil {
		return errors.New("invalid probe configuration")
	}
	if err = validateLocalReadOnlyTarget(cfg); err != nil {
		return errors.New("target rejected")
	}
	pc, err := pgxpool.ParseConfig(localReadOnlyProbeURL(cfg))
	if err != nil {
		return errors.New("probe connection configuration invalid")
	}
	if err = validateProbeConnConfig(pc.ConnConfig); err != nil {
		return errors.New("effective probe target rejected")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return errors.New("probe connection unavailable")
	}
	defer pool.Close()
	if err = executeLocalReadOnlyProbe(ctx, pgxProbeDB{pool: pool}); err != nil {
		return err
	}
	log.Print("local read-only PostgreSQL probe succeeded")
	return nil
}
