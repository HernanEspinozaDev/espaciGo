package postgres

import "github.com/jackc/pgx/v5/pgxpool"

// A Cloud Run CPU quota does not define runtime.NumCPU inside the container.
// Keep every pool in the two-process smoke bounded independently of host CPU.
const GCP04ReplayPoolMaxConns int32 = 4

func GCP04ReplayPoolConfig(databaseURL string) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	config.MaxConns = GCP04ReplayPoolMaxConns
	return config, nil
}
