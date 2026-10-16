package gcp03scratch

import (
	"context"
	"errors"

	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/jackc/pgx/v5"
)

// ObserveScratch executes only the fixed read-only compatibility observer
// against the reviewed scratch target as the migrator login.
func ObserveScratch(ctx context.Context, config Config, password string) (dbbootstrap.ScratchObservation, error) {
	if config.Phase != PhaseObserve || config.Login != MigratorLogin || config.DatabaseURL != DatabaseURL ||
		config.PasswordFile != PasswordFile || config.Instance != InstanceConnection || !config.PrivateIP {
		return dbbootstrap.ScratchObservation{}, errors.New("GCP-03 observer target configuration is invalid")
	}
	var report dbbootstrap.ScratchObservation
	err := withScratchConnection(ctx, config, password, func(ctx context.Context, conn *pgx.Conn) error {
		var err error
		report, err = dbbootstrap.ObserveScratch(ctx, conn)
		return err
	})
	return report, err
}
