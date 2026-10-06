package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/jackc/pgx/v5"
)

// createLocalBookingFixture is an explicit admin-only command. It authorizes
// exactly two active, email-verified accounts and creates one private draft.
func createLocalBookingFixture(hostEmail, renterEmail string) error {
	if os.Getenv("LOCAL_AUTH_PROTOTYPE") != "1" || os.Getenv("LOCAL_BOOKING_TRIAL") != "1" {
		return errors.New("local trial disabled")
	}
	hostEmail, renterEmail = strings.ToLower(strings.TrimSpace(hostEmail)), strings.ToLower(strings.TrimSpace(renterEmail))
	if hostEmail == "" || renterEmail == "" || hostEmail == renterEmail {
		return errors.New("invalid fixture participants")
	}
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, cfg.databaseURL())
	if err != nil {
		return err
	}
	defer conn.Close(ctx)
	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var hostID, renterID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, hostEmail).Scan(&hostID); err != nil {
		return errors.New("host must be an active verified local account")
	}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, renterEmail).Scan(&renterID); err != nil {
		return errors.New("renter must be a different active verified local account")
	}
	var existingSpace, existingHost, existingRenter string
	err = tx.QueryRow(ctx, `SELECT espacio_id::text,anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local_fixture WHERE singleton FOR UPDATE`).Scan(&existingSpace, &existingHost, &existingRenter)
	if err == nil {
		if existingHost == hostID && existingRenter == renterID {
			return tx.Commit(ctx)
		}
		return errors.New("the single local fixture is already assigned; explicit operator cleanup is required")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	ids := credentials.Generator{}
	spaceID, err := ids.ID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria)
VALUES($1,$2,'sala_multiproposito','Espacio sintético de ensayo','Espacio de prueba local sintético; no se publica, no representa un inmueble real y solo es visible a los dos participantes autorizados en el ensayo de reserva.',30,8,'Uso sintético controlado','hora',8000,'Dirección sintética local','America/Santiago')`, spaceID, hostID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,'sala_multiproposito',1,'{}'::jsonb)`, spaceID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(singleton,espacio_id,anfitrion_id,arrendatario_id) VALUES(true,$1,$2,$3)`, spaceID, hostID, renterID)
	if err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("Fixture local habilitado: 1 borrador sintético, anfitrión y arrendatario autorizados; sin permisos comerciales ni pago real.")
	return nil
}
