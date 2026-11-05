package operationspg

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	"github.com/jackc/pgx/v5"
)

// This opt-in test runs inside the selected Docker trial over its Unix socket.
// It uses only the two existing synthetic accounts and rolls back every write.
// The native UI test independently proves a persistent real commit.
type keepTransaction struct{ pgx.Tx }

func (keepTransaction) Commit(context.Context) error   { return nil }
func (keepTransaction) Rollback(context.Context) error { return nil }

func TestVacancyCurrentOwnerExactIntervalAtomicRollback(t *testing.T) {
	if os.Getenv("ESPACIGO_REPUBLICATION_TRANSACTION_TEST") != "1" {
		t.Skip("requires the authorized synthetic fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, "host=/var/run/postgresql dbname=espacigo_f5_gate_20261101 user=postgres sslmode=disable")
	if err != nil {
		t.Fatal("authorized Unix-socket connection unavailable")
	}
	defer conn.Close(context.Background())
	var db, engine string
	if err = conn.QueryRow(ctx, `SELECT current_database(),current_setting('server_version_num')`).Scan(&db, &engine); err != nil || db != "espacigo_f5_gate_20261101" || engine != "180006" {
		t.Fatal("wrong integration target")
	}
	reservation := os.Getenv("ESPACIGO_REPUBLICATION_RESERVATION_ID")
	if reservation == "" {
		t.Fatal("fixture identifier required")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(context.Background())
	var host, renter, space, occupation string
	var start, end time.Time
	if err = tx.QueryRow(ctx, `SELECT anfitrion_id::text,arrendatario_id::text,espacio_id::text,ocupacion_id::text,inicio,termino FROM public.reserva_ensayo_local WHERE id=$1`, reservation).Scan(&host, &renter, &space, &occupation, &start, &end); err != nil {
		t.Fatal("fixture unavailable")
	}
	var preContract string
	if err = tx.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(c)::text,'' ORDER BY id)) FROM public.contrato_ensayo_local c WHERE reserva_id=$1`, reservation).Scan(&preContract); err != nil {
		t.Fatal("contract fixture unavailable")
	}
	var othersBefore string
	if err = tx.QueryRow(ctx, `SELECT md5(COALESCE(string_agg(row_to_json(o)::text,'' ORDER BY id),'')) FROM public.ocupacion o WHERE espacio_id=$1 AND id<>$2`, space, occupation).Scan(&othersBefore); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET estado='finalizada' WHERE id=$1`, reservation); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en)
 SELECT gen_random_uuid(),id,'checkout',arrendatario_id,now(),zona_horaria,'{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.456,"longitude":-70.6693}'::jsonb,'registrada','transaction-synthetic-checkout',decode(repeat('00',32),'hex'),now() FROM public.reserva_ensayo_local r WHERE id=$1 AND NOT EXISTS(SELECT 1 FROM public.operacion_arriendo_ensayo_local o WHERE o.reserva_id=r.id AND o.tipo='checkout')`, reservation); err != nil {
		t.Fatal(err)
	}
	// No grant is changed: exercise the API's existing least-privileged role.
	if _, err = tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal(err)
	}
	scope, err := lockOperationScope(ctx, tx, host, reservation, operation.Receipt)
	if err != nil {
		t.Fatal("current owner scope rejected", err)
	}
	writer := &candidateWriter{tx: keepTransaction{tx}, actor: host, reservation: reservation, kind: operation.Receipt, state: scope.state, start: scope.start, end: scope.end, zone: scope.zone, host: scope.host, renter: scope.renter, owner: scope.owner, space: scope.space, occupation: scope.occupation}
	var active bool
	if err = tx.QueryRow(ctx, `SELECT activo FROM public.ocupacion WHERE id=$1`, occupation).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if !active {
		t.Fatal("requires fresh native fixture before persistent confirmation")
	}
	if err = writer.verifyReturnOccupation(ctx, true); err != nil {
		t.Fatal("valid interval rejected", err)
	}
	writer.end = end.Add(time.Minute)
	if err = writer.verifyReturnOccupation(ctx, true); err == nil {
		t.Fatal("mismatched interval accepted")
	}
	writer.end = end
	// Model a historical host differing from the current owner only inside
	// this rollback transaction; this does not implement ownership transfer.
	if _, err = tx.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET anfitrion_id=$2,arrendatario_id=$3 WHERE id=$1`, reservation, renter, host); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal(err)
	}
	if _, err = lockOperationScope(ctx, tx, renter, reservation, operation.Receipt); !errors.Is(err, operation.ErrNotFound) {
		t.Fatal("historical host retained authority", err)
	}
	if _, err = lockOperationScope(ctx, tx, host, reservation, operation.Receipt); err != nil {
		t.Fatal("current owner cannot confirm return", err)
	}
	if _, err = tx.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE public.reserva_ensayo_local SET anfitrion_id=$2,arrendatario_id=$3 WHERE id=$1`, reservation, host, renter); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `SET LOCAL ROLE espacigo_runtime`); err != nil {
		t.Fatal(err)
	}
	var fileID string
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&fileID); err != nil {
		t.Fatal(err)
	}
	writer.fileID = fileID
	if _, err = tx.Exec(ctx, `INSERT INTO public.operacion_arriendo_archivo_candidato_local(archivo_id,reserva_id,actor_id,tipo,estado,creada_en,proximo_intento_en)VALUES($1,$2,$3,'recepcion','reservado',now(),now())`, fileID, reservation, host); err != nil {
		t.Fatal(err)
	}
	input := operation.Input{Kind: operation.Receipt}
	canonical, _ := json.Marshal(input)
	digest := sha256.Sum256(canonical)
	item, reused, err := writer.Apply(ctx, fileID, "transaction-owner-confirmation", input, operation.Evidence{ID: fileID, Fixture: "synthetic-png-v1", MIME: "image/png", SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SizeBytes: 100}, digest[:], time.Now)
	if err != nil || reused || item.Kind != operation.Receipt {
		t.Fatal("atomic return failed", err)
	}
	if err = tx.QueryRow(ctx, `SELECT activo FROM public.ocupacion WHERE id=$1`, occupation).Scan(&active); err != nil || active {
		t.Fatal("exact interval not released", err)
	}
	var postContract, othersAfter string
	if err = tx.QueryRow(ctx, `SELECT md5(string_agg(row_to_json(c)::text,'' ORDER BY id)) FROM public.contrato_ensayo_local c WHERE reserva_id=$1`, reservation).Scan(&postContract); err != nil || postContract != preContract {
		t.Fatal("contract history changed", err)
	}
	if err = tx.QueryRow(ctx, `SELECT md5(COALESCE(string_agg(row_to_json(o)::text,'' ORDER BY id),'')) FROM public.ocupacion o WHERE espacio_id=$1 AND id<>$2`, space, occupation).Scan(&othersAfter); err != nil || othersBefore != othersAfter {
		t.Fatal("other intervals changed", err)
	}
	// Replay uses the same key/payload and verifies the same inactive occupation.
	var retryID string
	if err = tx.QueryRow(ctx, `SELECT gen_random_uuid()::text`).Scan(&retryID); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO public.operacion_arriendo_archivo_candidato_local(archivo_id,reserva_id,actor_id,tipo,estado,creada_en,proximo_intento_en)VALUES($1,$2,$3,'recepcion','reservado',now(),now())`, retryID, reservation, host); err != nil {
		t.Fatal(err)
	}
	writer.done = false
	writer.fileID = retryID
	replay, reused, err := writer.Apply(ctx, retryID, "transaction-owner-confirmation", input, operation.Evidence{}, digest[:], time.Now)
	if err != nil || !reused || replay.ID != item.ID {
		t.Fatal("same-intent replay failed", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = conn.QueryRow(ctx, `SELECT activo FROM public.ocupacion WHERE id=$1`, occupation).Scan(&active); err != nil || !active {
		t.Fatal("rollback isolation failed", err)
	}
	t.Log("CURRENT_OWNER_EXACT_INTERVAL_CONTRACT_OTHER_BLOCKS_IDEMPOTENCY_AND_ROLLBACK_PASS")
}
