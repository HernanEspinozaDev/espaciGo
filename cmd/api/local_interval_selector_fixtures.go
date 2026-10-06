package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/jackc/pgx/v5"
)

// createLocalIntervalSelectorFixtures appends three synthetic fixtures and
// one private manual block per tariff unit. It never edits/deletes old rows.
func createLocalIntervalSelectorFixtures(args []string) error {
	if os.Getenv("LOCAL_AUTH_PROTOTYPE") != "1" || os.Getenv("LOCAL_BOOKING_TRIAL") != "1" {
		return errors.New("local trial disabled")
	}
	flags := flag.NewFlagSet("local-booking-interval-selector-fixtures", flag.ContinueOnError)
	hostEmail := flags.String("host-email", "", "active verified synthetic host account")
	renterEmail := flags.String("renter-email", "", "active verified synthetic renter account")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("invalid interval selector fixture arguments")
	}
	*hostEmail = strings.ToLower(strings.TrimSpace(*hostEmail))
	*renterEmail = strings.ToLower(strings.TrimSpace(*renterEmail))
	if *hostEmail == "" || *renterEmail == "" || *hostEmail == *renterEmail {
		return errors.New("two different participant emails are required")
	}
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('m06-local-interval-selector-fixtures',0))`); err != nil {
		return err
	}
	var hostID, renterID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, *hostEmail).Scan(&hostID); err != nil {
		return errors.New("host must be an active verified local account")
	}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, *renterEmail).Scan(&renterID); err != nil || hostID == renterID {
		return errors.New("renter must be a different active verified local account")
	}
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		return err
	}
	today := time.Now().In(loc)
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	ids := credentials.Generator{}
	fixtures := []struct {
		unit, category, blockID string
		blockStart              time.Time
	}{
		{"hora", "oficina", "55555555-5555-4555-8555-000000000001", today.AddDate(0, 0, 2).Add(12 * time.Hour)},
		{"dia", "bodega", "55555555-5555-4555-8555-000000000002", today.AddDate(0, 0, 3).Add(12 * time.Hour)},
		{"mes", "local_flexible", "55555555-5555-4555-8555-000000000003", today.AddDate(0, 0, 11).Add(12 * time.Hour)},
	}
	for _, fixture := range fixtures {
		title := "M06 selector · " + fixture.unit
		var spaceID string
		err = tx.QueryRow(ctx, `SELECT f.espacio_id::text FROM public.reserva_ensayo_local_fixture f JOIN public.espacio e ON e.id=f.espacio_id WHERE f.habilitada AND f.anfitrion_id=$1 AND f.arrendatario_id=$2 AND e.titulo=$3`, hostID, renterID, title).Scan(&spaceID)
		if errors.Is(err, pgx.ErrNoRows) {
			var profileVersion int
			if err = tx.QueryRow(ctx, `SELECT max(version) FROM public.categoria_perfil_atributos WHERE categoria_codigo=$1`, fixture.category).Scan(&profileVersion); err != nil || profileVersion < 1 {
				return errors.New("fixture category profile unavailable")
			}
			spaceID, err = ids.ID()
			if err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,$3,$4,repeat('Fixture sintético para selector local de intervalos; no representa inmueble real y no se publica. ',2),24,4,'Uso sintético controlado',$5,8000,'Dirección sintética local','America/Santiago')`, spaceID, hostID, fixture.category, title, fixture.unit); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,$2,$3,'{}'::jsonb)`, spaceID, fixture.category, profileVersion); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,$2,8000)`, spaceID, fixture.unit); err != nil {
				return err
			}
			if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, spaceID, hostID, renterID); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			var actualUnit string
			if err = tx.QueryRow(ctx, `SELECT modalidad FROM public.tarifa_espacio WHERE espacio_id=$1 ORDER BY version DESC LIMIT 1`, spaceID).Scan(&actualUnit); err != nil || actualUnit != fixture.unit {
				return errors.New("existing selector fixture has an unexpected tariff; preserving it")
			}
		}
		var blockSpaceID string
		err = tx.QueryRow(ctx, `SELECT espacio_id::text FROM public.ocupacion WHERE id=$1`, fixture.blockID).Scan(&blockSpaceID)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, err = tx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo,motivo) VALUES($1,$2,NULL,tstzrange($3,$4,'[)'),'bloqueo_manual',true,'bloque sintético M06 selector')`, fixture.blockID, spaceID, fixture.blockStart.UTC(), fixture.blockStart.Add(time.Hour).UTC()); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else if blockSpaceID != spaceID {
			return errors.New("selector sample block ID is already used by another space; preserving it")
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("Fixtures sintéticos M06 listos para hora/día/mes; uno por tarifa conserva un bloqueo manual local. Fixtures previos conservados.")
	return nil
}
