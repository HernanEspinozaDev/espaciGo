package main

import (
	"context"
	"errors"
	"net"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newDatabasePool(ctx context.Context, cfg config) (*pgxpool.Pool, func(), error) {
	poolConfig, err := pgxpool.ParseConfig(cfg.databaseURL())
	if err != nil {
		return nil, nil, errors.New("invalid database connection configuration")
	}
	poolConfig.MaxConns = cfg.dbPoolMaxConns
	poolConfig.MinConns = cfg.dbPoolMinConns
	closeConnector := func() {}
	if cfg.dbConnectionMode == "cloudsql" {
		options := make([]cloudsqlconn.Option, 0, 1)
		if cfg.cloudSQLPrivateIP {
			options = append(options, cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
		}
		dialer, dialerErr := cloudsqlconn.NewDialer(ctx, options...)
		if dialerErr != nil {
			return nil, nil, errors.New("Cloud SQL connector initialization failed")
		}
		poolConfig.ConnConfig.DialFunc = func(dialCtx context.Context, _, _ string) (net.Conn, error) {
			return dialer.Dial(dialCtx, cfg.cloudSQLInstance)
		}
		closeConnector = func() { _ = dialer.Close() }
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		closeConnector()
		return nil, nil, errors.New("database pool initialization failed")
	}
	return pool, closeConnector, nil
}
