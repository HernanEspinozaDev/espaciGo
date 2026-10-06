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

// createLocalCatalogPaginationFixtures appends ten synthetic examples to one
// already-authorized participant pair. It never rewrites or removes fixtures.
func createLocalCatalogPaginationFixtures(args []string) error {
	if os.Getenv("LOCAL_AUTH_PROTOTYPE") != "1" || os.Getenv("LOCAL_BOOKING_TRIAL") != "1" {
		return errors.New("local trial disabled")
	}
	flags := flag.NewFlagSet("local-booking-pagination-fixtures", flag.ContinueOnError)
	hostEmail := flags.String("host-email", "", "active verified synthetic host account")
	renterEmail := flags.String("renter-email", "", "active verified synthetic renter account")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("invalid pagination fixture arguments")
	}
	*hostEmail = strings.ToLower(strings.TrimSpace(*hostEmail))
	*renterEmail = strings.ToLower(strings.TrimSpace(*renterEmail))
	if (*hostEmail == "") != (*renterEmail == "") || (*hostEmail != "" && *hostEmail == *renterEmail) {
		return errors.New("host and renter emails must both be supplied and be different")
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
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('m05-local-pagination-fixtures',0))`); err != nil {
		return err
	}
	var hostID, renterID string
	if *hostEmail != "" {
		err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, *hostEmail).Scan(&hostID)
		if err == nil {
			err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, *renterEmail).Scan(&renterID)
		}
		if err == nil && hostID == renterID {
			err = errors.New("fixture participants must be different accounts")
		}
	} else {
		err = tx.QueryRow(ctx, `SELECT f.anfitrion_id::text,f.arrendatario_id::text FROM public.reserva_ensayo_local_fixture f JOIN public.espacio e ON e.id=f.espacio_id WHERE f.habilitada AND e.titulo LIKE 'M05 paginación %' ORDER BY f.espacio_id LIMIT 1`).Scan(&hostID, &renterID)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local_fixture WHERE habilitada ORDER BY espacio_id LIMIT 1`).Scan(&hostID, &renterID)
		}
	}
	if err != nil {
		return errors.New("pagination examples require an existing authorized pair or two active verified synthetic accounts")
	}
	categories := []string{"oficina", "sala_multiproposito", "bodega", "estacionamiento", "local_flexible", "stand", "quincho", "parcela_eventos"}
	ids := credentials.Generator{}
	for i := 1; i <= 10; i++ {
		category := categories[(i-1)%len(categories)]
		title := fmt.Sprintf("M05 paginación %02d · %s", i, category)
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.reserva_ensayo_local_fixture f JOIN public.espacio e ON e.id=f.espacio_id WHERE f.anfitrion_id=$1 AND f.arrendatario_id=$2 AND e.titulo=$3)`, hostID, renterID, title).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		var categoryName string
		if err = tx.QueryRow(ctx, `SELECT nombre FROM public.categoria_espacio WHERE codigo=$1 AND activa`, category).Scan(&categoryName); err != nil {
			return err
		}
		var profileVersion int
		if err = tx.QueryRow(ctx, `SELECT max(version) FROM public.categoria_perfil_atributos WHERE categoria_codigo=$1`, category).Scan(&profileVersion); err != nil || profileVersion < 1 {
			return errors.New("category profile unavailable")
		}
		spaceID, e := ids.ID()
		if e != nil {
			return e
		}
		price := int64(8000 + i*500)
		if _, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria) VALUES($1,$2,$3,$4,'Ejemplo sintético de paginación local; no representa un inmueble real y solo es visible a participantes autorizados.',30,8,'Uso sintético controlado','hora',$5,'Dirección sintética local','America/Santiago')`, spaceID, hostID, category, title, price); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,$2,$3,'{}'::jsonb)`, spaceID, category, profileVersion); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',$2)`, spaceID, price); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, spaceID, hostID, renterID); err != nil {
			return err
		}
		latitude, longitude, ok := syntheticFixtureCoordinates(category)
		if !ok {
			return errors.New("synthetic category location unavailable")
		}
		latitude += float64(i) * 0.00007
		if _, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_ubicacion_sintetica(espacio_id,latitud,longitud,es_sintetica) VALUES($1,$2,$3,true)`, spaceID, latitude, longitude); err != nil {
			return err
		}
	}
	var total int
	if err = tx.QueryRow(ctx, `SELECT count(*) FROM public.reserva_ensayo_local_fixture f WHERE f.habilitada AND f.anfitrion_id=$1 AND f.arrendatario_id=$2`, hostID, renterID).Scan(&total); err != nil {
		return err
	}
	if total < 11 {
		return errors.New("pagination examples did not reach three default-sized pages")
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("Ejemplos sintéticos listos para %d fixtures autorizados (al menos tres páginas predeterminadas); fixtures anteriores conservados.\n", total)
	return nil
}
