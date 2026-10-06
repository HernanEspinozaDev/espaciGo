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
// exactly two active, email-verified accounts for one private synthetic draft.
func createLocalBookingFixture(hostEmail, renterEmail, category string) error {
	if os.Getenv("LOCAL_AUTH_PROTOTYPE") != "1" || os.Getenv("LOCAL_BOOKING_TRIAL") != "1" {
		return errors.New("local trial disabled")
	}
	hostEmail, renterEmail = strings.ToLower(strings.TrimSpace(hostEmail)), strings.ToLower(strings.TrimSpace(renterEmail))
	if hostEmail == "" || renterEmail == "" || hostEmail == renterEmail {
		return errors.New("invalid fixture participants")
	}
	category = strings.TrimSpace(category)
	if category == "" {
		return errors.New("invalid fixture category")
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
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, hostEmail+":"+renterEmail+":"+category); err != nil {
		return err
	}
	var hostID, renterID string
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, hostEmail).Scan(&hostID); err != nil {
		return errors.New("host must be an active verified local account")
	}
	if err = tx.QueryRow(ctx, `SELECT id::text FROM public.usuario WHERE correo_normalizado=$1 AND estado='activo'`, renterEmail).Scan(&renterID); err != nil {
		return errors.New("renter must be a different active verified local account")
	}
	var existingSpace string
	err = tx.QueryRow(ctx, `SELECT f.espacio_id::text FROM public.reserva_ensayo_local_fixture f JOIN public.espacio e ON e.id=f.espacio_id WHERE f.habilitada AND f.anfitrion_id=$1 AND f.arrendatario_id=$2 AND e.categoria_codigo=$3 FOR UPDATE OF f`, hostID, renterID, category).Scan(&existingSpace)
	if err == nil {
		if err = ensureSyntheticFixtureLocation(ctx, tx, existingSpace, category); err != nil {
			return err
		}
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	var categoryName string
	if err = tx.QueryRow(ctx, `SELECT nombre FROM public.categoria_espacio WHERE codigo=$1 AND activa`, category).Scan(&categoryName); err != nil {
		return errors.New("fixture category must be one of the active catalog categories")
	}
	var profileVersion int
	if err = tx.QueryRow(ctx, `SELECT max(version) FROM public.categoria_perfil_atributos WHERE categoria_codigo=$1`, category).Scan(&profileVersion); err != nil || profileVersion < 1 {
		return errors.New("fixture category profile is unavailable")
	}
	ids := credentials.Generator{}
	spaceID, err := ids.ID()
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,zona_horaria)
VALUES($1,$2,$3,$4,'Espacio de prueba local sintético; no se publica, no representa un inmueble real y solo es visible a los dos participantes autorizados en el ensayo local.',30,8,'Uso sintético controlado','hora',8000,'Dirección sintética local','America/Santiago')`, spaceID, hostID, category, "Espacio sintético · "+categoryName)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) VALUES($1,$2,$3,'{}'::jsonb)`, spaceID, category, profileVersion)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',8000)`, spaceID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, spaceID, hostID, renterID)
	if err != nil {
		return err
	}
	if err = ensureSyntheticFixtureLocation(ctx, tx, spaceID, category); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Printf("Fixture local habilitado: categoría %s, participantes autorizados; sin permisos comerciales ni pago real.\n", category)
	return nil
}

func ensureSyntheticFixtureLocation(ctx context.Context, tx pgx.Tx, spaceID, category string) error {
	latitude, longitude, ok := syntheticFixtureCoordinates(category)
	if !ok {
		return errors.New("fixture category has no synthetic location sample")
	}
	_, err := tx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_ubicacion_sintetica(espacio_id,latitud,longitud,es_sintetica)
VALUES($1,$2,$3,true)
ON CONFLICT (espacio_id) DO UPDATE SET latitud=EXCLUDED.latitud,longitud=EXCLUDED.longitud,es_sintetica=true`, spaceID, latitude, longitude)
	return err
}

func syntheticFixtureCoordinates(category string) (float64, float64, bool) {
	latitudes := map[string]float64{
		"oficina":             -33.4560,
		"sala_multiproposito": -33.4632,
		"bodega":              -33.4785,
		"estacionamiento":     -33.4965,
		"local_flexible":      -33.5415,
		"stand":               -33.5505,
		"quincho":             -33.6765,
		"parcela_eventos":     -33.6855,
	}
	latitude, ok := latitudes[category]
	return latitude, -70.6693, ok
}
