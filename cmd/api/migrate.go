package main

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	"github.com/jackc/pgx/v5"
)

// Separate one-shot local bootstrap uses admin credentials. The running API
// keeps the NOSUPERUSER/NOCREATEDB runtime role and never owns migration DDL.
func migrateLocal() error {
	if os.Getenv("LOCAL_AUTH_PROTOTYPE") != "1" {
		return errors.New("local profile required")
	}
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if _, err := migrator.Run(ctx, cfg.databaseURL(), "/migrations"); err != nil {
		return err
	}
	conn, err := pgx.Connect(ctx, cfg.databaseURL())
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	_, err = conn.Exec(ctx, `GRANT USAGE ON SCHEMA public TO espacigo_runtime;
 GRANT SELECT, INSERT, UPDATE ON public.usuario, public.sesion, public.token_accion TO espacigo_runtime;
 GRANT SELECT, INSERT ON public.rol_usuario, public.aceptacion_terminos TO espacigo_runtime;
 GRANT SELECT ON public.version_terminos TO espacigo_runtime;`)
	return err
}
