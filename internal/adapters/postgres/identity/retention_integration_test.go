package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type noOpPrivacyCleaner struct{}

func (noOpPrivacyCleaner) Delete(context.Context, string) error { return nil }

func TestLocalReservationRetentionPurgeRespectsDeadlineAndKeepsHistoricalFacts(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "retention-host@ejemplo.invalid")
	renterID := h.register(t, "retention-renter@ejemplo.invalid")
	adminID := h.register(t, "retention-admin@ejemplo.invalid")
	reservationID := "85000000-0000-4000-8000-000000000003"
	quoteID := "85000000-0000-4000-8000-000000000002"
	occupancyID := "85000000-0000-4000-8000-000000000004"
	spaceID := "85000000-0000-4000-8000-000000000001"
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, hostID, renterID, "cancelada_arrendatario")
	deadline := h.now.AddDate(2, 0, 0)
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2,actualizada_en=$3 WHERE id=$1`, reservationID, deadline, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE id=$1`, occupancyID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_ensayo_transicion(id,reserva_id,secuencia,estado_anterior,estado_nuevo,actor_id,motivo,creada_en) VALUES('85000000-0000-4000-8000-000000000005',$1,1,'pendiente_de_pago','cancelada_arrendatario',$2,'ensayo', $3)`, reservationID, hostID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_cancelacion_ensayo(id,reserva_id,arrendatario_id,clave_idempotencia,huella_solicitud,motivo,creada_en) VALUES('85000000-0000-4000-8000-000000000006',$1,$2,'retention-cancel',decode(repeat('22',32),'hex'),'synthetic',$3)`, reservationID, renterID, h.now); err != nil {
		t.Fatal(err)
	}
	disputeID := "85000000-0000-4000-8000-000000000007"
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.disputa_ensayo_local(id,reserva_id,anfitrion_id,arrendatario_id,abierta_por,motivo_codigo,estado,clave_idempotencia,huella_solicitud,abierta_en,cerrada_por,motivo_cierre_codigo,cerrada_en) VALUES($1,$2,$3,$4,$3,'ensayo_privacidad','cerrada','retention-dispute',decode(repeat('33',32),'hex'),$5,$6,'ensayo_finalizado',$7)`, disputeID, reservationID, hostID, renterID, h.now.Add(-time.Hour), adminID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.disputa_ensayo_historial(disputa_id,estado_anterior,estado_nuevo,actor_id,motivo_codigo,ocurrida_en) VALUES($1,NULL,'abierta',$2,'ensayo_privacidad',$3),($1,'abierta','cerrada',$4,'ensayo_finalizado',$5)`, disputeID, hostID, h.now.Add(-time.Hour), adminID, h.now); err != nil {
		t.Fatal(err)
	}
	runtime := newRuntimeIdentityRepository(t, h)
	service, err := privacy.NewService(runtime)
	if err != nil {
		t.Fatal(err)
	}
	before, err := service.PurgeExpiredReservationLinks(h.ctx, deadline.Add(-time.Microsecond), 20)
	if err != nil || before.Purged != 0 || before.Scanned != 0 {
		t.Fatalf("purge before retention deadline=%+v err=%v", before, err)
	}
	atDeadline, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || atDeadline.Scanned != 1 || atDeadline.Purged != 1 || atDeadline.Deferred != 0 {
		t.Fatalf("purge at inclusive retention deadline=%+v err=%v", atDeadline, err)
	}
	var host, renter, quoteHost, quoteRenter, cancelRenter, disputeHost, disputeRenter, openedBy string
	var closer string
	var transitionActor, openingActor string
	var state string
	if err := h.pool.QueryRow(h.ctx, `SELECT COALESCE(r.anfitrion_id::text,''),COALESCE(r.arrendatario_id::text,''),COALESCE(q.anfitrion_id::text,''),COALESCE(q.arrendatario_id::text,''),COALESCE(c.arrendatario_id::text,''),COALESCE(d.anfitrion_id::text,''),COALESCE(d.arrendatario_id::text,''),COALESCE(d.abierta_por::text,''),COALESCE(d.cerrada_por::text,''),r.estado
		FROM public.reserva_ensayo_local r JOIN public.cotizacion_reserva_ensayo q ON q.id=r.cotizacion_id JOIN public.reserva_cancelacion_ensayo c ON c.reserva_id=r.id JOIN public.disputa_ensayo_local d ON d.reserva_id=r.id WHERE r.id=$1`, reservationID).Scan(&host, &renter, &quoteHost, &quoteRenter, &cancelRenter, &disputeHost, &disputeRenter, &openedBy, &closer, &state); err != nil {
		t.Fatal(err)
	}
	if host != "" || renter != "" || quoteHost != "" || quoteRenter != "" || cancelRenter != "" || disputeHost != "" || disputeRenter != "" || openedBy != "" || closer != adminID || state != "cancelada_arrendatario" {
		t.Fatalf("historical linkage treatment mismatch: %q %q %q %q %q %q %q %q %q state=%s", host, renter, quoteHost, quoteRenter, cancelRenter, disputeHost, disputeRenter, openedBy, closer, state)
	}
	if err := h.pool.QueryRow(h.ctx, `SELECT (SELECT COALESCE(actor_id::text,'') FROM public.reserva_ensayo_transicion WHERE reserva_id=$1),(SELECT COALESCE(actor_id::text,'') FROM public.disputa_ensayo_historial WHERE disputa_id=$2 AND estado_nuevo='abierta')`, reservationID, disputeID).Scan(&transitionActor, &openingActor); err != nil {
		t.Fatal(err)
	}
	if transitionActor != "" || openingActor != "" {
		t.Fatalf("historical actors not unlinked: reservation=%q dispute=%q", transitionActor, openingActor)
	}
	var reservations, quotes, occupations, purges int
	var subtotal int64
	if err := h.pool.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM public.reserva_ensayo_local WHERE id=$1),(SELECT count(*) FROM public.cotizacion_reserva_ensayo WHERE id=$2),(SELECT count(*) FROM public.ocupacion WHERE id=$3),(SELECT count(*) FROM public.reserva_vinculo_purgado_local WHERE reserva_id=$1),(SELECT subtotal_clp FROM public.cotizacion_reserva_ensayo WHERE id=$2)`, reservationID, quoteID, occupancyID).Scan(&reservations, &quotes, &occupations, &purges, &subtotal); err != nil {
		t.Fatal(err)
	}
	if reservations != 1 || quotes != 1 || occupations != 1 || purges != 1 || subtotal != 12000 {
		t.Fatalf("purge removed historical facts: reservations=%d quotes=%d occupations=%d markers=%d subtotal=%d", reservations, quotes, occupations, purges, subtotal)
	}
	replay, err := service.PurgeExpiredReservationLinks(h.ctx, deadline.Add(time.Hour), 20)
	if err != nil || replay.Purged != 0 || replay.Scanned != 0 {
		t.Fatalf("purge replay was not idempotent: %+v err=%v", replay, err)
	}
}

func TestLocalReservationRetentionDefersOpenDisputeAndRecalculatesFromClosure(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "retention-open-host@ejemplo.invalid")
	renterID := h.register(t, "retention-open-renter@ejemplo.invalid")
	reservationID := "85100000-0000-4000-8000-000000000003"
	deadline := h.now.AddDate(2, 0, 0)
	seedPrivacyReviewReservation(t, h, "85100000-0000-4000-8000-000000000001", "85100000-0000-4000-8000-000000000002", reservationID, "85100000-0000-4000-8000-000000000004", hostID, renterID, "cancelada_arrendatario")
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2,actualizada_en=$3 WHERE id=$1`, reservationID, deadline, h.now); err != nil {
		t.Fatal(err)
	}
	disputeID := "85100000-0000-4000-8000-000000000005"
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.disputa_ensayo_local(id,reserva_id,anfitrion_id,arrendatario_id,abierta_por,motivo_codigo,estado,clave_idempotencia,huella_solicitud,abierta_en) VALUES($1,$2,$3,$4,$3,'ensayo_privacidad','abierta','retention-open',decode(repeat('44',32),'hex'),$5)`, disputeID, reservationID, hostID, renterID, h.now); err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || blocked.Purged != 0 || blocked.Deferred != 1 {
		t.Fatalf("open dispute did not defer purge: %+v err=%v", blocked, err)
	}
	closedAt := deadline.Add(10 * time.Minute)
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.disputa_ensayo_local SET estado='cerrada',cerrada_por=$2,motivo_cierre_codigo='ensayo_finalizado',cerrada_en=$3 WHERE id=$1`, disputeID, hostID, closedAt); err != nil {
		t.Fatal(err)
	}
	tooEarly, err := service.PurgeExpiredReservationLinks(h.ctx, closedAt.AddDate(0, 24, 0).Add(-time.Microsecond), 20)
	if err != nil || tooEarly.Purged != 0 {
		t.Fatalf("dispute closure restarted retention incorrectly: %+v err=%v", tooEarly, err)
	}
	var recalculated time.Time
	if err := h.pool.QueryRow(h.ctx, `SELECT vinculos_retirar_en FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&recalculated); err != nil {
		t.Fatal(err)
	}
	if !recalculated.Equal(closedAt.AddDate(0, 24, 0)) {
		t.Fatalf("deadline=%s want=%s", recalculated, closedAt.AddDate(0, 24, 0))
	}
	due, err := service.PurgeExpiredReservationLinks(h.ctx, recalculated, 20)
	if err != nil || due.Purged != 1 {
		t.Fatalf("purge after new deadline=%+v err=%v", due, err)
	}
}

func TestLocalSuppressionReplayRestoresAnOlderDatabaseSnapshotIdempotently(t *testing.T) {
	h := newAuthHarness(t)
	targetID := h.register(t, "retention-replay-target@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "retention-replay-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.SaveProfile(h.ctx, targetID, "Cuenta Replay", nil); err != nil {
		t.Fatal(err)
	}
	request, err := h.repo.CreateRightsRequest(h.ctx, targetID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	completed, err := service.ExecuteSuppression(h.ctx, adminID, request.ID, "replay-source-operation", "replay-source", func() time.Time { return h.now }, noOpPrivacyCleaner{})
	if err != nil || completed.Status != "completada" {
		t.Fatalf("source suppression: %+v err=%v", completed, err)
	}
	manifest, err := service.ExportSuppressionReplayManifest(h.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Version != 1 || len(manifest.Entries) != 1 || manifest.Entries[0].AccountID != targetID {
		t.Fatalf("bad exported registry: %+v", manifest)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"retention-replay-target@ejemplo.invalid", "synthetic-hash", "token"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("registry exposed forbidden field %q", forbidden)
		}
	}
	// Simulate restoring a snapshot from before execution: the target account and
	// original rights request are restored to active/en-review state; V25 rows from
	// the later execution are absent. Only the replay registry survives externally.
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.usuario SET estado='activo',correo_original='retention-replay-target@ejemplo.invalid',correo_normalizado='retention-replay-target@ejemplo.invalid',hash_clave='synthetic-restored-hash',baja_iniciada_en=NULL,preferencia_uso='arrendar' WHERE id=$1`, targetID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.solicitud_titular SET estado='en_revision',resuelta_en=NULL,motivo_resolucion_codigo=NULL,retirar_en=NULL WHERE id=$1`, request.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `DELETE FROM public.ejecucion_baja_local WHERE solicitud_id=$1`, request.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'arrendatario') ON CONFLICT DO NOTHING`, targetID); err != nil {
		t.Fatal(err)
	}
	restoredService := service
	restoreID := "85200000-0000-4000-8000-000000000001"
	results, err := restoredService.ReplaySuppressionManifest(h.ctx, manifest, restoreID, adminID, h.now, noOpPrivacyCleaner{})
	if err != nil || len(results) != 1 || results[0].Status != "reaplicada" {
		t.Fatalf("replay results=%+v err=%v", results, err)
	}
	var accountState, hash string
	if err := h.pool.QueryRow(h.ctx, `SELECT estado,hash_clave FROM public.usuario WHERE id=$1`, targetID).Scan(&accountState, &hash); err != nil {
		t.Fatal(err)
	}
	if accountState != "desidentificado" || hash != "" {
		t.Fatalf("restored account not minimized: state=%s credential_present=%t", accountState, hash != "")
	}
	again, err := restoredService.ReplaySuppressionManifest(h.ctx, manifest, restoreID, adminID, h.now, noOpPrivacyCleaner{})
	if err != nil || len(again) != 1 || again[0].Status != "reaplicada" || !again[0].Reused {
		t.Fatalf("idempotent replay=%+v err=%v", again, err)
	}
	otherRestore, err := restoredService.ReplaySuppressionManifest(h.ctx, manifest, "85200000-0000-4000-8000-000000000002", adminID, h.now, noOpPrivacyCleaner{})
	if err != nil || len(otherRestore) != 1 || otherRestore[0].Status != "ya_presente" {
		t.Fatalf("replay over already minimized account=%+v err=%v", otherRestore, err)
	}
	var replayRows, executions int
	if err := h.pool.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM public.reaplicacion_baja_local WHERE usuario_id=$1),(SELECT count(*) FROM public.ejecucion_baja_local WHERE solicitud_id=$2)`, targetID, request.ID).Scan(&replayRows, &executions); err != nil {
		t.Fatal(err)
	}
	if replayRows != 2 || executions != 1 {
		t.Fatalf("replay side effects duplicated: replay rows=%d executions=%d", replayRows, executions)
	}
}

func TestLocalSuppressionReplayAfterPgDumpRestore(t *testing.T) {
	h := newAuthHarness(t)
	targetID := h.register(t, "retention-backup-target@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "retention-backup-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.repo.SaveProfile(h.ctx, targetID, "Cuenta de respaldo", nil); err != nil {
		t.Fatal(err)
	}
	request, err := h.repo.CreateRightsRequest(h.ctx, targetID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	runtimeRepo := newRuntimeIdentityRepository(t, h)
	sourceService, err := privacy.NewService(runtimeRepo)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	backup := filepath.Join(tmp, "pre-suppression.dump")
	sourceURL := h.pool.Config().ConnConfig.ConnString()
	if output, err := exec.Command("pg_dump", "--format=custom", "--no-owner", "--no-privileges", "--file", backup, "--dbname", sourceURL).CombinedOutput(); err != nil {
		t.Fatalf("create isolated test backup: %v %s", err, string(output))
	}
	suppressed, err := sourceService.ExecuteSuppression(h.ctx, adminID, request.ID, "source-backup-test", "source-backup", func() time.Time { return h.now }, noOpPrivacyCleaner{})
	if err != nil || suppressed.Status != "completada" {
		t.Fatalf("source suppression=%+v err=%v", suppressed, err)
	}
	manifest, err := sourceService.ExportSuppressionReplayManifest(h.ctx)
	if err != nil || len(manifest.Entries) != 1 {
		t.Fatalf("export registry=%+v err=%v", manifest, err)
	}
	registryPath := filepath.Join(t.TempDir(), "completed-suppressions-v1.json")
	if err := sourceService.ExportSuppressionReplayManifestToFile(h.ctx, registryPath); err != nil {
		t.Fatalf("write external restore registry: %v", err)
	}
	info, err := os.Stat(registryPath)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("restore registry permissions=%v err=%v", info, err)
	}
	registryBytes, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	var onDisk privacy.SuppressionReplayManifest
	if err := json.Unmarshal(registryBytes, &onDisk); err != nil || len(onDisk.Entries) != 1 || onDisk.Entries[0].AccountID != manifest.Entries[0].AccountID {
		t.Fatalf("restore registry content=%+v err=%v", onDisk, err)
	}
	for _, forbidden := range []string{"@ejemplo.invalid", "hash_clave", "token"} {
		if strings.Contains(string(registryBytes), forbidden) {
			t.Fatalf("restore registry contains sensitive field %q", forbidden)
		}
	}
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	restoreDB := fmt.Sprintf("privacy_restore_test_%d", time.Now().UnixNano())
	adminConn, err := pgx.Connect(h.ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adminConn.Exec(h.ctx, "CREATE DATABASE "+pgx.Identifier{restoreDB}.Sanitize()); err != nil {
		adminConn.Close(h.ctx)
		t.Fatal(err)
	}
	if err := adminConn.Close(h.ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		admin, err := pgx.Connect(cleanupCtx, adminURL)
		if err != nil {
			t.Errorf("connect to remove isolated restore database: %v", err)
			return
		}
		defer admin.Close(cleanupCtx)
		if _, err := admin.Exec(cleanupCtx, "DROP DATABASE IF EXISTS "+pgx.Identifier{restoreDB}.Sanitize()); err != nil {
			t.Errorf("remove isolated restore database: %v", err)
		}
	})
	parsed.Path = "/" + restoreDB
	restoreURL := parsed.String()
	if output, err := exec.Command("pg_restore", "--exit-on-error", "--no-owner", "--no-privileges", "--dbname", restoreURL, backup).CombinedOutput(); err != nil {
		t.Fatalf("restore isolated pre-suppression backup: %v %s", err, string(output))
	}
	if err := adminConnExecGrant(restoreURL); err != nil {
		t.Fatalf("restore database runtime grants: %v", err)
	}
	config, err := pgxpool.ParseConfig(restoreURL)
	if err != nil {
		t.Fatal(err)
	}
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	restoredPool, err := pgxpool.NewWithConfig(h.ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restoredPool.Close)
	restoredService, err := privacy.NewService(identitypg.NewIdentityRepository(restoredPool))
	if err != nil {
		t.Fatal(err)
	}
	results, err := restoredService.ReplaySuppressionManifest(h.ctx, manifest, "85300000-0000-4000-8000-000000000001", adminID, h.now, noOpPrivacyCleaner{})
	if err != nil || len(results) != 1 || results[0].Status != "reaplicada" {
		t.Fatalf("restored backup replay=%+v err=%v", results, err)
	}
	var state, hash string
	var profiles int
	if err := restoredPool.QueryRow(h.ctx, `SELECT estado,hash_clave FROM public.usuario WHERE id=$1`, targetID).Scan(&state, &hash); err != nil {
		t.Fatal(err)
	}
	if err := restoredPool.QueryRow(h.ctx, `SELECT count(*) FROM public.perfil_usuario WHERE usuario_id=$1`, targetID).Scan(&profiles); err != nil {
		t.Fatal(err)
	}
	if state != "desidentificado" || hash != "" || profiles != 0 {
		t.Fatalf("restored identity did not reapply suppression: state=%s credential_present=%t profiles=%d", state, hash != "", profiles)
	}
	again, err := restoredService.ReplaySuppressionManifest(h.ctx, manifest, "85300000-0000-4000-8000-000000000001", adminID, h.now, noOpPrivacyCleaner{})
	if err != nil || len(again) != 1 || again[0].Status != "reaplicada" || !again[0].Reused {
		t.Fatalf("restored replay retry=%+v err=%v", again, err)
	}
}

func adminConnExecGrant(databaseURL string) error {
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		return err
	}
	defer conn.Close(context.Background())
	if err := dbbootstrap.GrantRuntimePermissions(context.Background(), conn); err != nil {
		return err
	}
	return nil
}
