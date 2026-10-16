package main

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/database/cloudsql"
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
		var connectorErr error
		closeConnector, connectorErr = cloudsql.ConfigurePGX(ctx, poolConfig.ConnConfig, cfg.cloudSQLInstance, cfg.cloudSQLPrivateIP)
		if connectorErr != nil {
			return nil, nil, connectorErr
		}
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		closeConnector()
		return nil, nil, errors.New("database pool initialization failed")
	}
	return pool, closeConnector, nil
}
