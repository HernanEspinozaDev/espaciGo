package postgres_test

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	damageclaimpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/damageclaim"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	operationspg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/operations"
	ownerexportpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/ownerexport"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/damageclaim"
	damageclaimhttp "github.com/HernanEspinozaDev/espaciGo/internal/damageclaim/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/dbbootstrap"
	"github.com/HernanEspinozaDev/espaciGo/internal/operation"
	operationhttp "github.com/HernanEspinozaDev/espaciGo/internal/operation/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

type defenseEvidenceFixture struct {
	h                                 *authHarness
	host, renter, outsider, admin     string
	reservation, claim, checkEvidence string
	store                             *evidencefs.Store
	operations                        *operation.Service
	claims                            *damageclaim.Service
}

func newDefenseEvidenceFixture(t *testing.T, h *authHarness, suffix int) defenseEvidenceFixture {
	t.Helper()
	emailBase := fmt.Sprintf("defense-evidence-%d", suffix)
	host := h.register(t, emailBase+"-host@ejemplo.invalid")
	h.verify(t)
	renter := h.register(t, emailBase+"-renter@ejemplo.invalid")
	h.verify(t)
	outsider := h.register(t, emailBase+"-outsider@ejemplo.invalid")
	h.verify(t)
	admin := h.register(t, emailBase+"-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, admin); err != nil {
		t.Fatal(err)
	}
	uuid := func(offset int) string { return fmt.Sprintf("%08x-0000-4000-8000-%012x", 88000000+suffix, offset) }
	spaceID, quoteID, reservationID, occupancyID := uuid(1), uuid(2), uuid(3), uuid(4)
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, host, renter, "finalizada")
	root := t.TempDir() + "/private-defense-evidence"
	if err := os.Mkdir(root, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := evidencefs.New(root)
	if err != nil {
		t.Fatal(err)
	}
	checkoutID, checkoutEvidenceID := uuid(5), uuid(6)
	blob, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Put(h.ctx, checkoutEvidenceID, blob); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(blob)
	location := `{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}`
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en) VALUES($1,$2,'checkout',$3,$4,'UTC',$5::jsonb,'registrada','defense-checkout-key',decode(repeat('11',32),'hex'),$4)`, checkoutID, reservationID, renter, h.now.Add(-time.Hour), location); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en) VALUES($1,$2,'synthetic-png-v1','image/png',$3,$4,$5)`, checkoutEvidenceID, checkoutID, hex.EncodeToString(digest[:]), len(blob), h.now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	claims, err := damageclaim.NewWithEvidenceStore(damageclaimpg.New(h.pool), store, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	claim, err := claims.Open(h.ctx, host, reservationID, "defense-claim-open-key", damageclaim.Input{Description: "motivo sintetico del reclamo"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = claims.Defend(h.ctx, renter, reservationID, "defense-claim-defend-key", damageclaim.Input{Description: "descargo sintetico propio"}); err != nil {
		t.Fatal(err)
	}
	runtimePool := newRuntimePool(t, h)
	operations, err := operation.NewService(operationspg.New(runtimePool), store, credentials.Generator{}, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	return defenseEvidenceFixture{h: h, host: host, renter: renter, outsider: outsider, admin: admin, reservation: reservationID, claim: claim.ID, checkEvidence: checkoutEvidenceID, store: store, operations: operations, claims: claims}
}

func TestSyntheticDamageDefenseEvidenceParticipantAdminExportIsolation(t *testing.T) {
	h := newAuthHarness(t)
	f := newDefenseEvidenceFixture(t, h, 1)
	operationHandler := operationhttp.NewHandler(h.service, f.operations, nil)
	hostSession := h.login(t, "defense-evidence-1-host@ejemplo.invalid")
	hostCreate := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/"+f.reservation+"/damage-claim/defense/evidence", strings.NewReader(`{}`))
	hostCreate.Header.Set("Authorization", "Bearer "+string(hostSession.Token))
	hostCreate.Header.Set("Idempotency-Key", "host-cannot-submit-defense-evidence")
	hostDenied := httptest.NewRecorder()
	operationHandler.ServeHTTP(hostDenied, hostCreate)
	if hostDenied.Code != http.StatusNotFound {
		t.Fatalf("host defense evidence submission status=%d body=%s", hostDenied.Code, hostDenied.Body.String())
	}
	renterSession := h.login(t, "defense-evidence-1-renter@ejemplo.invalid")
	create := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/"+f.reservation+"/damage-claim/defense/evidence", strings.NewReader(`{}`))
	create.Header.Set("Authorization", "Bearer "+string(renterSession.Token))
	create.Header.Set("Idempotency-Key", "defense-evidence-create-001")
	created := httptest.NewRecorder()
	operationHandler.ServeHTTP(created, create)
	if created.Code != http.StatusCreated {
		t.Fatalf("create defense evidence status=%d body=%s", created.Code, created.Body.String())
	}
	validateOpenAPIJSON(t, created.Body.Bytes(), "LocalRentalOperationEnvelope")
	var createdEnvelope struct {
		Data operation.Item `json:"data"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &createdEnvelope); err != nil {
		t.Fatal(err)
	}
	item := createdEnvelope.Data
	if len(item.Evidence) != 1 || item.Evidence[0].Fixture != "synthetic-png-v1" {
		t.Fatalf("generated defense evidence=%+v", item.Evidence)
	}
	// A repeated HTTP submission reuses the same server-generated attachment.
	retry := httptest.NewRequest(http.MethodPost, "/api/v1/local/booking-trial/reservations/"+f.reservation+"/damage-claim/defense/evidence", strings.NewReader(`{}`))
	retry.Header.Set("Authorization", "Bearer "+string(renterSession.Token))
	retry.Header.Set("Idempotency-Key", "defense-evidence-create-001")
	retried := httptest.NewRecorder()
	operationHandler.ServeHTTP(retried, retry)
	validateOpenAPIJSON(t, retried.Body.Bytes(), "LocalRentalOperationEnvelope")
	var retriedEnvelope struct {
		Data operation.Item `json:"data"`
	}
	if retried.Code != http.StatusOK || json.Unmarshal(retried.Body.Bytes(), &retriedEnvelope) != nil || !retriedEnvelope.Data.Reused || len(retriedEnvelope.Data.Evidence) != 1 || retriedEnvelope.Data.Evidence[0].ID != item.Evidence[0].ID {
		t.Fatalf("idempotent HTTP retry status=%d item=%+v body=%s", retried.Code, retriedEnvelope.Data, retried.Body.String())
	}
	claim, err := f.claims.Get(h.ctx, f.host, f.reservation)
	if err != nil || claim.Defense == nil || len(claim.Defense.Evidence) != 1 || claim.Defense.Evidence[0].ID != item.Evidence[0].ID {
		t.Fatalf("claim defense projection=%+v err=%v", claim.Defense, err)
	}
	for _, email := range []string{fmt.Sprintf("defense-evidence-%d-host@ejemplo.invalid", 1), fmt.Sprintf("defense-evidence-%d-renter@ejemplo.invalid", 1)} {
		session := h.login(t, email)
		req := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+f.reservation+"/evidence/"+item.Evidence[0].ID, nil)
		req.Header.Set("Authorization", "Bearer "+string(session.Token))
		response := httptest.NewRecorder()
		operationHandler.ServeHTTP(response, req)
		if response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), mustSyntheticPNG(t)) {
			t.Fatalf("participant evidence status=%d body bytes=%d", response.Code, response.Body.Len())
		}
	}
	outsiderSession := h.login(t, fmt.Sprintf("defense-evidence-%d-outsider@ejemplo.invalid", 1))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+f.reservation+"/evidence/"+item.Evidence[0].ID, nil)
	req.Header.Set("Authorization", "Bearer "+string(outsiderSession.Token))
	response := httptest.NewRecorder()
	operationHandler.ServeHTTP(response, req)
	if response.Code != http.StatusNotFound {
		t.Fatalf("third-party evidence status=%d body=%s", response.Code, response.Body.String())
	}
	claimHandler := damageclaimhttp.NewHandler(h.service, f.claims, nil)
	adminSession := h.login(t, fmt.Sprintf("defense-evidence-%d-admin@ejemplo.invalid", 1))
	adminReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/local/damage-claims/"+f.claim+"/evidence/"+item.Evidence[0].ID, nil)
	adminReq.Header.Set("Authorization", "Bearer "+string(adminSession.Token))
	adminResp := httptest.NewRecorder()
	claimHandler.ServeHTTP(adminResp, adminReq)
	if adminResp.Code != http.StatusOK || !bytes.Equal(adminResp.Body.Bytes(), mustSyntheticPNG(t)) {
		t.Fatalf("admin evidence status=%d body bytes=%d", adminResp.Code, adminResp.Body.Len())
	}
	archiveService, err := privacy.NewService(h.repo, ownerexportpg.New(h.pool, f.store))
	if err != nil {
		t.Fatal(err)
	}
	archive, err := archiveService.ExportOwnArchive(h.ctx, f.renter)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Sections map[string]json.RawMessage `json:"sections"`
	}
	for _, file := range zr.File {
		if file.Name == "data.json" {
			reader, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(reader)
			_ = reader.Close()
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(raw, &data); e != nil {
				t.Fatal(e)
			}
		}
	}
	var section struct {
		Items []struct {
			EvidenceID    string `json:"evidence_id"`
			Archive       string `json:"archive_file"`
			FileAvailable bool   `json:"file_available"`
		} `json:"items"`
	}
	if err = json.Unmarshal(data.Sections["synthetic_damage_claim_defenses"], &section); err != nil || len(section.Items) != 1 || section.Items[0].EvidenceID != item.Evidence[0].ID || !section.Items[0].FileAvailable {
		t.Fatalf("own defense export section=%+v err=%v", section, err)
	}
	found := false
	for _, file := range zr.File {
		if file.Name == section.Items[0].Archive {
			found = true
			reader, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(reader)
			_ = reader.Close()
			if e != nil || !bytes.Equal(raw, mustSyntheticPNG(t)) {
				t.Fatalf("exported image mismatch err=%v", e)
			}
		}
	}
	if !found {
		t.Fatalf("ZIP omitted defense evidence %q", section.Items[0].Archive)
	}
	hostArchive, err := archiveService.ExportOwnArchive(h.ctx, f.host)
	if err != nil {
		t.Fatal(err)
	}
	hostZip, err := zip.NewReader(bytes.NewReader(hostArchive), int64(len(hostArchive)))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range hostZip.File {
		if file.Name == section.Items[0].Archive {
			t.Fatalf("host archive included renter-authored defense evidence")
		}
	}
}

func mustSyntheticPNG(t *testing.T) []byte {
	t.Helper()
	blob, err := verification.SyntheticPNG()
	if err != nil {
		t.Fatal(err)
	}
	return blob
}

func TestSyntheticDamageDefenseEvidenceRetentionUsesRecoverableCleanup(t *testing.T) {
	h := newAuthHarness(t)
	f := newDefenseEvidenceFixture(t, h, 2)
	item, err := f.operations.Record(h.ctx, f.renter, f.reservation, "defense-evidence-retain-001", operation.Input{Kind: operation.DefenseEvidence})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.claims.Resolve(h.ctx, f.admin, f.claim, "defense-claim-resolve-key", damageclaim.ResolutionInput{Outcome: "rechazado", ReasonCode: "hecho_no_acreditado"}); err != nil {
		t.Fatal(err)
	}
	var deadline time.Time
	if err = h.pool.QueryRow(h.ctx, `SELECT vinculos_retirar_en FROM public.reserva_ensayo_local WHERE id=$1`, f.reservation).Scan(&deadline); err != nil {
		t.Fatal(err)
	}
	h.now = deadline
	purger, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	first, err := purger.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || first.Purged != 0 || first.Deferred != 1 {
		t.Fatalf("first retention pass=%+v err=%v", first, err)
	}
	var queued int
	if err = h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.operacion_arriendo_archivo_candidato_local
		WHERE archivo_id=$1 AND estado='pendiente_limpieza' AND ultimo_codigo_error='retencion_vencida' AND proximo_intento_en<=clock_timestamp()`, item.Evidence[0].ID).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("retention cleanup not durable queued=%d err=%v", queued, err)
	}
	if err = f.operations.CleanRetiredOnce(h.ctx, 20); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Get(h.ctx, item.Evidence[0].ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("retention left private blob err=%v", err)
	}
	operationsHandler := operationhttp.NewHandler(h.service, f.operations, nil)
	renterSession := h.login(t, "defense-evidence-2-renter@ejemplo.invalid")
	listRequest := httptest.NewRequest(http.MethodGet, "/api/v1/local/booking-trial/reservations/"+f.reservation+"/operations", nil)
	listRequest.Header.Set("Authorization", "Bearer "+string(renterSession.Token))
	listResponse := httptest.NewRecorder()
	operationsHandler.ServeHTTP(listResponse, listRequest)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("post-cleanup operations HTTP status=%d body=%s", listResponse.Code, listResponse.Body.String())
	}
	validateOpenAPIJSON(t, listResponse.Body.Bytes(), "LocalRentalOperationListEnvelope")
	var operationList struct {
		Data struct {
			Items []operation.Item `json:"items"`
		} `json:"data"`
	}
	if err = json.Unmarshal(listResponse.Body.Bytes(), &operationList); err != nil {
		t.Fatal(err)
	}
	foundRetired := false
	for _, listed := range operationList.Data.Items {
		if listed.Kind == operation.DefenseEvidence {
			foundRetired = true
			if listed.Evidence == nil || len(listed.Evidence) != 0 {
				t.Fatalf("retired operation evidence must remain as an empty list: %+v", listed.Evidence)
			}
		}
	}
	if !foundRetired {
		t.Fatalf("post-cleanup operations omitted the retained defense operation")
	}
	second, err := purger.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || second.Purged != 1 {
		t.Fatalf("second retention pass=%+v err=%v", second, err)
	}
	var retiredAt *time.Time
	var retainedMetadata int
	if err = h.pool.QueryRow(h.ctx, `SELECT archivo_retirado_en,(SELECT count(*) FROM public.operacion_arriendo_evidencia_ensayo_local WHERE id=$1) FROM public.operacion_arriendo_evidencia_ensayo_local WHERE id=$1`, item.Evidence[0].ID).Scan(&retiredAt, &retainedMetadata); err != nil || retiredAt == nil || retainedMetadata != 1 {
		t.Fatalf("retained evidence facts archived_at=%v rows=%d err=%v", retiredAt, retainedMetadata, err)
	}
	adminClaim, err := f.claims.GetAdmin(h.ctx, f.claim)
	if err != nil || adminClaim.Defense == nil || len(adminClaim.Defense.Evidence) != 0 {
		t.Fatalf("retained claim exposes expired blob=%+v err=%v", adminClaim.Defense, err)
	}
}

func TestSyntheticDamageDefenseEvidenceSuppressionQueuesRecoverableCleanup(t *testing.T) {
	h := newAuthHarness(t)
	f := newDefenseEvidenceFixture(t, h, 3)
	item, err := f.operations.Record(h.ctx, f.renter, f.reservation, "defense-evidence-suppress-001", operation.Input{Kind: operation.DefenseEvidence})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.claims.Resolve(h.ctx, f.admin, f.claim, "defense-suppress-resolve", damageclaim.ResolutionInput{Outcome: "rechazado", ReasonCode: "hecho_no_acreditado"}); err != nil {
		t.Fatal(err)
	}
	privacyService, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	request, err := privacyService.RequestRight(h.ctx, f.renter, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	review, err := privacyService.ReviewSuppression(h.ctx, f.admin, request.ID, "defense-suppress-review", "defense-evidence-suppress", func() time.Time { return h.now })
	if err != nil || review.Outcome != "elegible" {
		t.Fatalf("suppression review=%+v err=%v", review, err)
	}
	cleaner := &failOncePrivateCleaner{store: f.store, fail: true}
	execution, err := privacyService.ExecuteSuppression(h.ctx, f.admin, request.ID, "defense-suppress-execute", "defense-evidence-suppress", func() time.Time { return h.now }, cleaner)
	if err != nil || execution.Status != "limpieza_pendiente" {
		t.Fatalf("suppression execution=%+v err=%v", execution, err)
	}
	if err = privacyService.RunSuppressionCleanupOnce(h.ctx, func() time.Time { return h.now }, cleaner); err != nil {
		t.Fatal(err)
	}
	if _, err = f.store.Get(h.ctx, item.Evidence[0].ID); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("baja cleanup left defense blob: %v", err)
	}
	var retiredAt *time.Time
	if err = h.pool.QueryRow(h.ctx, `SELECT archivo_retirado_en FROM public.operacion_arriendo_evidencia_ensayo_local WHERE id=$1`, item.Evidence[0].ID).Scan(&retiredAt); err != nil || retiredAt == nil {
		t.Fatalf("baja did not mark defense blob retired: at=%v err=%v", retiredAt, err)
	}
	if _, _, err = f.operations.EvidenceContent(h.ctx, f.renter, f.reservation, item.Evidence[0].ID); !errors.Is(err, operation.ErrNotFound) {
		t.Fatalf("retired defense evidence remained accessible after baja: %v", err)
	}
	claim, err := f.claims.GetAdmin(h.ctx, f.claim)
	if err != nil || claim.Defense == nil || len(claim.Defense.Evidence) != 0 {
		t.Fatalf("claim exposed defense blob after baja: defense=%+v err=%v", claim.Defense, err)
	}
}

type failOncePrivateCleaner struct {
	store *evidencefs.Store
	fail  bool
}

func (c *failOncePrivateCleaner) Delete(ctx context.Context, id string) error {
	if c.fail {
		c.fail = false
		return errors.New("synthetic private deletion failure")
	}
	return c.store.Delete(ctx, id)
}

func TestRatifiedLocalRetentionPurgesDueRowsAndRecoversPrivateFileDeletion(t *testing.T) {
	h := newAuthHarness(t)
	ownerID := h.register(t, "retention-owner@ejemplo.invalid")
	h.verify(t)
	reviewerID := h.register(t, "retention-reviewer@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "retention-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}

	resolvedAt := h.now.AddDate(-3, 0, 0)
	fileDue := resolvedAt.AddDate(0, 0, 90)
	metadataDue := resolvedAt.AddDate(2, 0, 0)
	verificationRepo := verificationpg.New(h.pool)
	verifications, err := verification.NewService(verificationRepo, credentials.Generator{}, verification.LocalFixtureProvider{}, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	caseItem, err := verifications.Start(h.ctx, ownerID, "kyc", "retention-case-0001")
	if err != nil {
		t.Fatal(err)
	}
	privateRoot := t.TempDir()
	if err := os.Chmod(privateRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	privateStore, err := evidencefs.New(privateRoot)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := verification.NewEvidenceService(verificationRepo, verificationRepo, privateStore, credentials.Generator{}, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	file, err := evidence.Upload(h.ctx, ownerID, caseItem.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = verifications.Review(h.ctx, caseItem.ID, reviewerID, "aprobada", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.verificacion SET resuelta_en=$2 WHERE id=$1`, caseItem.ID, resolvedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.verificacion_historial_local SET ocurrida_en=$2 WHERE verificacion_id=$1`, caseItem.ID, resolvedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.elegibilidad_verificacion_local SET concedida_en=$2 WHERE verificacion_id=$1`, caseItem.ID, resolvedAt); err != nil {
		t.Fatal(err)
	}

	// Exercise all non-file classes in the same bounded worker pass. Terms
	// retain the shared version row; request/decision and unconverted quote
	// rows use their own independently ratified five-year/90-day deadlines.
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.aceptacion_terminos SET retirar_en=$2 WHERE usuario_id=$1`, ownerID, fileDue); err != nil {
		t.Fatal(err)
	}
	rights, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	request, err := rights.RequestRight(h.ctx, ownerID, "acceso", "web")
	if err != nil {
		t.Fatal(err)
	}
	resolvedRequestAt := fileDue.AddDate(-5, 0, 0)
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.solicitud_titular SET estado='resuelta',resuelta_en=$2,motivo_resolucion_codigo='derecho_tramitado',retirar_en=$2::timestamptz+interval '5 years' WHERE id=$1`, request.ID, resolvedRequestAt); err != nil {
		t.Fatal(err)
	}
	spaceID, quoteID, reservationID, occupancyID := "89000000-0000-4000-8000-000000000001", "89000000-0000-4000-8000-000000000002", "89000000-0000-4000-8000-000000000003", "89000000-0000-4000-8000-000000000004"
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, reviewerID, ownerID, "aprobada_host")
	unconvertedQuoteID := "89000000-0000-4000-8000-000000000005"
	quoteCreated, quoteExpires := fileDue.Add(-91*24*time.Hour), fileDue.Add(-90*24*time.Hour)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.cotizacion_reserva_ensayo
		SELECT (jsonb_populate_record(NULL::public.cotizacion_reserva_ensayo,
			to_jsonb(q)||jsonb_build_object('id',$2::uuid,'creada_en',$3::timestamptz,'vence_en',$4::timestamptz,'retirar_en',$5::timestamptz))).*
		FROM public.cotizacion_reserva_ensayo q WHERE q.id=$1`, quoteID, unconvertedQuoteID, quoteCreated, quoteExpires, fileDue); err != nil {
		t.Fatal(err)
	}

	runtimeRepo := newRuntimeIdentityRepository(t, h)
	retention, err := privacy.NewService(runtimeRepo)
	if err != nil {
		t.Fatal(err)
	}
	cleaner := &failOncePrivateCleaner{store: privateStore, fail: true}
	before, err := retention.PurgeExpiredLocalRetentions(h.ctx, fileDue.Add(-time.Nanosecond), 20, cleaner)
	if err != nil || before.EvidenceFilesRemoved != 0 || before.TermsPurged != 0 || before.RightsPurged != 0 || before.QuotesPurged != 0 {
		t.Fatalf("early retention pass=%+v err=%v", before, err)
	}
	failed, err := retention.PurgeExpiredLocalRetentions(h.ctx, fileDue, 20, cleaner)
	if err != nil || failed.EvidenceFilesDeferred != 1 || failed.TermsPurged == 0 || failed.RightsPurged != 1 || failed.QuotesPurged != 1 {
		t.Fatalf("failed-file pass=%+v err=%v", failed, err)
	}
	if _, err := os.Stat(filepath.Join(privateRoot, file.ID+".png")); err != nil {
		t.Fatalf("file should remain after the simulated deletion failure: %v", err)
	}
	var storedFileID string
	if err := h.pool.QueryRow(h.ctx, `SELECT archivo_id::text FROM public.verificacion_evidencia_sintetica WHERE id=$1`, file.ID).Scan(&storedFileID); err != nil || storedFileID != file.ID {
		t.Fatalf("reference cleared before filesystem success: file_id=%q err=%v", storedFileID, err)
	}

	// Construct a fresh repository/service after the failed unlink to model a
	// backend restart; the persisted lease and retry time drive recovery.
	restartedService, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := restartedService.PurgeExpiredLocalRetentions(h.ctx, fileDue.Add(11*time.Second), 20, cleaner)
	if err != nil || recovered.EvidenceFilesRemoved != 1 || recovered.VerificationsPurged != 0 {
		t.Fatalf("recovered blob pass=%+v err=%v", recovered, err)
	}
	if _, err := os.Stat(filepath.Join(privateRoot, file.ID+".png")); !os.IsNotExist(err) {
		t.Fatalf("private file remains after recovery: %v", err)
	}
	metadata, err := verificationRepo.ListOwnEvidence(h.ctx, ownerID, caseItem.ID)
	if err != nil || len(metadata) != 1 || metadata[0].FileAvailable || metadata[0].FileID != "" {
		t.Fatalf("two-year metadata/reference state=%+v err=%v", metadata, err)
	}
	if _, _, err := evidence.OwnContent(h.ctx, ownerID, caseItem.ID, file.ID); !errors.Is(err, verification.ErrNotFound) {
		t.Fatalf("retired private file read err=%v, want not found", err)
	}

	metadataPurged, err := retention.PurgeExpiredLocalRetentions(h.ctx, metadataDue, 20, cleaner)
	if err != nil || metadataPurged.VerificationsPurged != 1 {
		t.Fatalf("two-year verification pass=%+v err=%v", metadataPurged, err)
	}
	var cases, histories, eligibility, evidenceRows int
	if err := h.pool.QueryRow(h.ctx, `SELECT
		(SELECT count(*) FROM public.verificacion WHERE id=$1),
		(SELECT count(*) FROM public.verificacion_historial_local WHERE verificacion_id=$1),
		(SELECT count(*) FROM public.elegibilidad_verificacion_local WHERE verificacion_id=$1),
		(SELECT count(*) FROM public.verificacion_evidencia_sintetica WHERE verificacion_id=$1)`, caseItem.ID).Scan(&cases, &histories, &eligibility, &evidenceRows); err != nil {
		t.Fatal(err)
	}
	if cases+histories+eligibility+evidenceRows != 0 {
		t.Fatalf("verification metadata/history/projection survived deadline: cases=%d history=%d eligibility=%d evidence=%d", cases, histories, eligibility, evidenceRows)
	}
	var acceptances, rightsRows, quoteRows, convertedQuoteRows, reservationRows int
	if err := h.pool.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM public.aceptacion_terminos WHERE usuario_id=$1),
		(SELECT count(*) FROM public.solicitud_titular WHERE id=$2),
		(SELECT count(*) FROM public.cotizacion_reserva_ensayo WHERE id=$3),
		(SELECT count(*) FROM public.cotizacion_reserva_ensayo WHERE id=$4),
		(SELECT count(*) FROM public.reserva_ensayo_local WHERE id=$5)`, ownerID, request.ID, unconvertedQuoteID, quoteID, reservationID).Scan(&acceptances, &rightsRows, &quoteRows, &convertedQuoteRows, &reservationRows); err != nil {
		t.Fatal(err)
	}
	if acceptances+rightsRows+quoteRows != 0 || convertedQuoteRows != 1 || reservationRows != 1 {
		t.Fatalf("retention changed unrelated/converted data: terms=%d rights=%d unconverted_quotes=%d converted_quotes=%d reservations=%d", acceptances, rightsRows, quoteRows, convertedQuoteRows, reservationRows)
	}
}

type noOpPrivacyCleaner struct{}

func (noOpPrivacyCleaner) Delete(context.Context, string) error { return nil }

func TestResolvedDamageClaimPrivacyEvaluationMinimizesTextAndPurgesLinksAtRatifiedDeadline(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "claim-privacy-host@ejemplo.invalid")
	h.verify(t)
	renterID := h.register(t, "claim-privacy-renter@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "claim-privacy-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES ($1,'administrador')`, adminID); err != nil {
		t.Fatal(err)
	}
	spaceID, quoteID := "86000000-0000-4000-8000-000000000001", "86000000-0000-4000-8000-000000000002"
	reservationID, occupancyID := "86000000-0000-4000-8000-000000000003", "86000000-0000-4000-8000-000000000004"
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, hostID, renterID, "finalizada")
	operationID, evidenceID := "86000000-0000-4000-8000-000000000005", "86000000-0000-4000-8000-000000000006"
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(
		id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,comentarios,observacion,
		resultado,clave_idempotencia,huella_solicitud,creada_en)
		VALUES($1,$2,'checkout',$3,$4,'UTC','{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}'::jsonb,'comentario privado retiro','',
		'registrada','claim-privacy-checkout',decode(repeat('11',32),'hex'),$4)`, operationID, reservationID, renterID, h.now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en) VALUES($1,$2,'synthetic-png-v1','image/png',repeat('a',64),16,$3)`, evidenceID, operationID, h.now.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	claims, err := damageclaim.New(damageclaimpg.New(h.pool), func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	claim, err := claims.Open(h.ctx, hostID, reservationID, "claim-privacy-open", damageclaim.Input{Description: "detalle privado del anfitrion"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = claims.Defend(h.ctx, renterID, reservationID, "claim-privacy-defense", damageclaim.Input{Description: "descargo privado del arrendatario"}); err != nil {
		t.Fatal(err)
	}
	resolved, err := claims.Resolve(h.ctx, adminID, claim.ID, "claim-privacy-resolve", damageclaim.ResolutionInput{Outcome: "rechazado", ReasonCode: "hecho_no_acreditado"})
	if err != nil || resolved.State != "resuelta" || resolved.Resolution == nil {
		t.Fatalf("claim resolution=%+v err=%v", resolved, err)
	}
	requester, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	request, err := requester.RequestRight(h.ctx, renterID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	review, err := runtime.ReviewSuppression(h.ctx, adminID, request.ID, "claim-privacy-review", "claim-privacy", func() time.Time { return h.now })
	if err != nil || review.Outcome != "elegible" || len(review.Obligations) != 0 || len(review.PendingChecks) != 0 {
		t.Fatalf("resolved claim left a false/open blocker: review=%+v err=%v", review, err)
	}
	execution, err := runtime.ExecuteSuppression(h.ctx, adminID, request.ID, "claim-privacy-execute", "claim-privacy", func() time.Time { return h.now }, noOpPrivacyCleaner{})
	if err != nil || execution.Status != "completada" {
		t.Fatalf("actual privacy execution=%+v err=%v", execution, err)
	}
	var claimText, defenseText, operationComments string
	var claimState, reservationState string
	var deadline time.Time
	if err := h.pool.QueryRow(h.ctx, `SELECT c.descripcion,d.descripcion,o.comentarios,c.estado,r.estado,r.vinculos_retirar_en
		FROM public.reclamo_dano_ensayo_local c JOIN public.reclamo_dano_descargo_ensayo_local d ON d.reclamo_id=c.id
		JOIN public.reserva_ensayo_local r ON r.id=c.reserva_id JOIN public.operacion_arriendo_ensayo_local o ON o.id=c.checkout_operacion_id WHERE c.id=$1`, claim.ID).
		Scan(&claimText, &defenseText, &operationComments, &claimState, &reservationState, &deadline); err != nil {
		t.Fatal(err)
	}
	wantDeadline := resolved.Resolution.ResolvedAt.AddDate(0, 24, 0)
	if claimText != "Texto libre retirado por baja local de privacidad." || defenseText != "Texto libre retirado por baja local de privacidad." || operationComments != "" || claimState != "resuelta" || reservationState != "finalizada" || !deadline.Equal(wantDeadline) {
		t.Fatalf("privacy minimization/retention mismatch claim=%q defense=%q comments=%q claimState=%s reservation=%s deadline=%s want=%s", claimText, defenseText, operationComments, claimState, reservationState, deadline, wantDeadline)
	}
	var active bool
	if err := h.pool.QueryRow(h.ctx, `SELECT activo FROM public.ocupacion WHERE id=$1`, occupancyID).Scan(&active); err != nil || !active {
		t.Fatalf("suppression changed historical occupancy active=%t err=%v", active, err)
	}
	if purged, err := runtime.PurgeExpiredReservationLinks(h.ctx, deadline.Add(-time.Microsecond), 20); err != nil || purged.Purged != 0 {
		t.Fatalf("links purged before 24-month deadline: result=%+v err=%v", purged, err)
	}
	if purged, err := runtime.PurgeExpiredReservationLinks(h.ctx, deadline, 20); err != nil || purged.Purged != 1 {
		t.Fatalf("links were not purged at inclusive deadline: result=%+v err=%v", purged, err)
	}
	var linkedClaimReservation, linkedClaimHost, linkedClaimRenter, linkedCheckout, linkedEvidence, linkedOperationReservation, linkedOperationActor string
	var retainedClaimState string
	if err := h.pool.QueryRow(h.ctx, `SELECT COALESCE(c.reserva_id::text,''),COALESCE(c.anfitrion_id::text,''),COALESCE(c.arrendatario_id::text,''),COALESCE(c.checkout_operacion_id::text,''),COALESCE(c.checkout_evidencia_id::text,''),COALESCE(o.reserva_id::text,''),COALESCE(o.actor_id::text,''),c.estado
		FROM public.reclamo_dano_ensayo_local c JOIN public.operacion_arriendo_ensayo_local o ON o.id=$2 WHERE c.id=$1`, claim.ID, operationID).
		Scan(&linkedClaimReservation, &linkedClaimHost, &linkedClaimRenter, &linkedCheckout, &linkedEvidence, &linkedOperationReservation, &linkedOperationActor, &retainedClaimState); err != nil {
		t.Fatal(err)
	}
	if linkedClaimReservation != "" || linkedClaimHost != "" || linkedClaimRenter != "" || linkedCheckout != "" || linkedEvidence != "" || linkedOperationReservation != "" || linkedOperationActor != "" || retainedClaimState != "resuelta" {
		t.Fatalf("expired historical links not minimized: claim=[%q %q %q %q %q] operation=[%q %q] state=%s", linkedClaimReservation, linkedClaimHost, linkedClaimRenter, linkedCheckout, linkedEvidence, linkedOperationReservation, linkedOperationActor, retainedClaimState)
	}
	retained, err := claims.GetAdmin(h.ctx, claim.ID)
	if err != nil || retained.ReservationID != "" || retained.HostID != "" || retained.RenterID != "" || retained.State != "resuelta" || retained.Description != "Texto libre retirado por baja local de privacidad." || len(retained.Evidence) != 0 || retained.Defense == nil || retained.Defense.ActorID != "" || retained.Defense.Description != "Texto libre retirado por baja local de privacidad." || retained.Resolution == nil || retained.Resolution.ActorID != "" {
		t.Fatalf("admin historical view should remain readable but minimized: claim=%+v err=%v", retained, err)
	}
	for _, transition := range retained.History {
		if transition.ActorID != "" {
			t.Fatalf("retained claim history still identifies an actor: %+v", transition)
		}
	}

	// Exercise the actual administrative HTTP representation after purging links and
	// validate it against the public OpenAPI envelope contract.
	adminSession := h.login(t, "claim-privacy-admin@ejemplo.invalid")
	api := damageclaimhttp.NewHandler(h.service, claims, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/local/damage-claims/"+claim.ID, nil)
	req.Header.Set("Authorization", "Bearer "+string(adminSession.Token))
	response := httptest.NewRecorder()
	api.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("admin detail after purge status=%d body=%s", response.Code, response.Body.String())
	}
	validateOpenAPIJSON(t, response.Body.Bytes(), "LocalDamageClaimEnvelope")
	var wire map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &wire); err != nil {
		t.Fatal(err)
	}
	data := wire["data"].(map[string]any)
	for _, key := range []string{"reservation_id", "host_id", "renter_id", "checkout_operation_id", "checkout_evidence_id"} {
		if value, present := data[key]; !present || value != nil {
			t.Fatalf("purged %s should be JSON null, got present=%t value=%#v", key, present, value)
		}
	}
	if data["description"] != "Texto libre retirado por baja local de privacidad." || data["state"] != "resuelta" {
		t.Fatalf("historical outcome was not preserved: %#v", data)
	}
	evidence, ok := data["evidence"].([]any)
	if !ok || len(evidence) != 0 {
		t.Fatalf("purged evidence must be []: %#v", data["evidence"])
	}
	history, ok := data["history"].([]any)
	if !ok || len(history) != len(retained.History) {
		t.Fatalf("historical transitions were not returned as an array: %#v", data["history"])
	}
	for i, raw := range history {
		item := raw.(map[string]any)
		if actor, present := item["actor_id"]; !present || actor != nil {
			t.Fatalf("history[%d] actor should be JSON null after unlinking: %#v", i, item)
		}
	}
	for _, nestedKey := range []string{"defense", "resolution"} {
		nested, ok := data[nestedKey].(map[string]any)
		if !ok {
			t.Fatalf("historical %s missing: %#v", nestedKey, data[nestedKey])
		}
		if actor, present := nested["actor_id"]; !present || actor != nil {
			t.Fatalf("%s actor should be JSON null after unlinking: %#v", nestedKey, nested)
		}
	}
	var transitionCount, occupancyCount int
	if err := h.pool.QueryRow(h.ctx, `SELECT (SELECT count(*) FROM public.reserva_ensayo_transicion WHERE reserva_id=$1),(SELECT count(*) FROM public.ocupacion WHERE id=$2)`, reservationID, occupancyID).Scan(&transitionCount, &occupancyCount); err != nil {
		t.Fatal(err)
	}
	if transitionCount != 2 || occupancyCount != 1 {
		t.Fatalf("historical facts were removed: transitions=%d occupancy=%d", transitionCount, occupancyCount)
	}
}

func validateOpenAPIJSON(t *testing.T, payload []byte, schemaName string) {
	t.Helper()
	openapi, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "planning", "openapi.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := yaml.Unmarshal(openapi, &document); err != nil {
		t.Fatal(err)
	}
	documentJSON, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("openapi.json", bytes.NewReader(documentJSON)); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("openapi.json#/components/schemas/" + schemaName)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err := json.Unmarshal(payload, &value); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(value); err != nil {
		t.Fatalf("admin response does not match OpenAPI %s: %v", schemaName, err)
	}
}

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

func TestLocalReservationRetentionWaitsForFakePaymentReconciliation(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "retention-payment-host@ejemplo.invalid")
	renterID := h.register(t, "retention-payment-renter@ejemplo.invalid")
	reservationID := "85200000-0000-4000-8000-000000000003"
	quoteID := "85200000-0000-4000-8000-000000000002"
	occupancyID := "85200000-0000-4000-8000-000000000004"
	spaceID := "85200000-0000-4000-8000-000000000001"
	operationID := "85200000-0000-4000-8000-000000000010"
	eventID := "85200000-0000-4000-8000-000000000011"
	providerEventID := "retention-reconcile-event"
	deadline := h.now
	terminalAt := deadline.AddDate(0, -24, 0)
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, hostID, renterID, "cancelada_arrendatario")
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2,actualizada_en=$3 WHERE id=$1`, reservationID, deadline, terminalAt); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.ocupacion SET activo=false,desactivada_en=$2 WHERE id=$1`, occupancyID, terminalAt); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_pago_ensayo_operacion(id,reserva_id,arrendatario_id,clave_idempotencia,huella_solicitud,resultado_solicitado,estado,resultado_final,creada_en,actualizada_en)
		VALUES($1,$2,$3,'retention-payment',decode(repeat('66',32),'hex'),'exito','vencida',NULL,$4,$4)`, operationID, reservationID, renterID, terminalAt); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_pago_fake_resultado_ensayo(operacion_id,estado,proveedor_evento_id,resultado,registrado_en)
		VALUES($1,'resultado',$2,'exito_simulado',$3)`, operationID, providerEventID, terminalAt); err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	missingEvent, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || missingEvent.Scanned != 1 || missingEvent.Purged != 0 || missingEvent.Deferred != 1 {
		t.Fatalf("fake result without authenticated event was not deferred: %+v err=%v", missingEvent, err)
	}
	assertReservationLinksPreserved := func(stage string) {
		t.Helper()
		var host, renter string
		if err := h.pool.QueryRow(h.ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&host, &renter); err != nil {
			t.Fatalf("%s load reservation links: %v", stage, err)
		}
		if host != hostID || renter != renterID {
			t.Fatalf("%s removed links before payment reconciliation: host=%q renter=%q", stage, host, renter)
		}
	}
	assertReservationLinksPreserved("missing event")
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_pago_evento_ensayo(id,operacion_id,proveedor_evento_id,resultado,huella_payload,autenticado_en,recibido_en)
		VALUES($1,$2,$3,'exito_simulado',decode(repeat('77',32),'hex'),$4,$4)`, eventID, operationID, providerEventID, terminalAt); err != nil {
		t.Fatal(err)
	}
	unprocessed, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || unprocessed.Purged != 0 || unprocessed.Deferred != 1 {
		t.Fatalf("event without application was not deferred: %+v err=%v", unprocessed, err)
	}
	assertReservationLinksPreserved("event without application")
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_pago_evento_aplicacion_ensayo(evento_id,estado,codigo_resultado,procesado_en,creada_en)
		VALUES($1,'pendiente_conciliacion','resultado_tardio',NULL,$2)`, eventID, terminalAt); err != nil {
		t.Fatal(err)
	}
	pendingReconciliation, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || pendingReconciliation.Purged != 0 || pendingReconciliation.Deferred != 1 {
		t.Fatalf("pending reconciliation was not deferred: %+v err=%v", pendingReconciliation, err)
	}
	assertReservationLinksPreserved("pending reconciliation")
	// A terminal ignored result represents an explicit completed reconciliation;
	// once its processing row is final, retention may resume without changing the
	// already-terminal reservation or its payment facts.
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.reserva_pago_evento_aplicacion_ensayo SET estado='ignorada',codigo_resultado='operacion_terminal',procesado_en=$2 WHERE evento_id=$1`, eventID, deadline); err != nil {
		t.Fatal(err)
	}
	conciliated, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || conciliated.Purged != 1 || conciliated.Deferred != 0 {
		t.Fatalf("retention did not resume after reconciliation completed: %+v err=%v", conciliated, err)
	}
	var host, renter string
	var paymentResults, processedEvents, purgeMarkers int
	if err := h.pool.QueryRow(h.ctx, `SELECT COALESCE(r.anfitrion_id::text,''),COALESCE(r.arrendatario_id::text,''),
		(SELECT count(*) FROM public.reserva_pago_fake_resultado_ensayo WHERE operacion_id=$2),
		(SELECT count(*) FROM public.reserva_pago_evento_aplicacion_ensayo WHERE evento_id=$3 AND estado='ignorada' AND procesado_en IS NOT NULL),
		(SELECT count(*) FROM public.reserva_vinculo_purgado_local WHERE reserva_id=$1)
		FROM public.reserva_ensayo_local r WHERE r.id=$1`, reservationID, operationID, eventID).Scan(&host, &renter, &paymentResults, &processedEvents, &purgeMarkers); err != nil {
		t.Fatal(err)
	}
	if host != "" || renter != "" || paymentResults != 1 || processedEvents != 1 || purgeMarkers != 1 {
		t.Fatalf("post-reconciliation purge state links=%q/%q payment=%d processed=%d markers=%d", host, renter, paymentResults, processedEvents, purgeMarkers)
	}
}

func TestLocalReservationRetentionDefersGuaranteeAndFinancialDecisionObligations(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "retention-guarantee-host@ejemplo.invalid")
	renterID := h.register(t, "retention-guarantee-renter@ejemplo.invalid")
	adminID := h.register(t, "retention-guarantee-admin@ejemplo.invalid")
	reservationID := "85300000-0000-4000-8000-000000000003"
	quoteID := "85300000-0000-4000-8000-000000000002"
	occupancyID := "85300000-0000-4000-8000-000000000004"
	spaceID := "85300000-0000-4000-8000-000000000001"
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, hostID, renterID, "cancelada_arrendatario")
	purgeAt := h.now.AddDate(0, 24, 0)
	if _, err := h.pool.Exec(h.ctx, `UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2,actualizada_en=$3 WHERE id=$1`, reservationID, purgeAt, h.now); err != nil {
		t.Fatal(err)
	}
	guaranteeID := "85300000-0000-4000-8000-000000000010"
	authorizationID := "85300000-0000-4000-8000-000000000011"
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_garantia_ensayo_local(id,reserva_id,politica_version,moneda,previsto_clp,estado,creada_en,actualizada_en)
		VALUES($1,$2,'garantia_local_fija_v1','CLP',50000,'pendiente_autorizacion',$3,$3)`, guaranteeID, reservationID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.reserva_garantia_operacion_ensayo_local(id,garantia_id,tipo,clave_idempotencia,huella_solicitud,importe_clp,resultado_solicitado,estado,primer_intento_en,vence_en,creada_en,actualizada_en)
		VALUES($1,$2,'autorizacion','retention-pending-auth',decode(repeat('88',32),'hex'),50000,'sin_respuesta','pendiente',$3,$3,$3,$3)`, authorizationID, guaranteeID, h.now); err != nil {
		t.Fatal(err)
	}
	service, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	guaranteeBlocked, err := service.PurgeExpiredReservationLinks(h.ctx, purgeAt, 20)
	if err != nil || guaranteeBlocked.Purged != 0 || guaranteeBlocked.Deferred != 1 {
		t.Fatalf("pending guarantee did not defer link purge: %+v err=%v", guaranteeBlocked, err)
	}
	assertLinks := func(stage string) {
		t.Helper()
		var host, renter string
		if err := h.pool.QueryRow(h.ctx, `SELECT anfitrion_id::text,arrendatario_id::text FROM public.reserva_ensayo_local WHERE id=$1`, reservationID).Scan(&host, &renter); err != nil {
			t.Fatalf("%s read links: %v", stage, err)
		}
		if host != hostID || renter != renterID {
			t.Fatalf("%s purged links while obligation remained: host=%q renter=%q", stage, host, renter)
		}
	}
	assertLinks("guarantee pending")
	if _, err = h.pool.Exec(h.ctx, `UPDATE public.reserva_garantia_operacion_ensayo_local SET estado='vencida',ultimo_resultado='cancelada',completada_en=$2,actualizada_en=$2 WHERE id=$1`, authorizationID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(h.ctx, `UPDATE public.reserva_garantia_ensayo_local SET estado='cancelada',actualizada_en=$2 WHERE id=$1`, guaranteeID, h.now); err != nil {
		t.Fatal(err)
	}
	checkoutID, evidenceID := "85300000-0000-4000-8000-000000000012", "85300000-0000-4000-8000-000000000013"
	claimID := "85300000-0000-4000-8000-000000000014"
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,comentarios,observacion,resultado,clave_idempotencia,huella_solicitud,creada_en)
		VALUES($1,$2,'checkout',$3,$4,'UTC','{"source":"synthetic-fixture-v1","location_code":"santiago-demo-center-v1","latitude":-33.45,"longitude":-70.66}'::jsonb,'','', 'registrada','retention-checkout-01',decode(repeat('99',32),'hex'),$4)`, checkoutID, reservationID, renterID, h.now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.operacion_arriendo_evidencia_ensayo_local(id,operacion_id,fixture_code,mime_type,sha256,size_bytes,creada_en) VALUES($1,$2,'synthetic-png-v1','image/png',repeat('b',64),16,$3)`, evidenceID, checkoutID, h.now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.reclamo_dano_ensayo_local(id,reserva_id,anfitrion_id,arrendatario_id,checkout_operacion_id,checkout_evidencia_id,descripcion,estado,clave_idempotencia,huella_solicitud,abierto_en,plazo_reclamo_hasta)
		VALUES($1,$2,$3,$4,$5,$6,'synthetic claim','resuelta','retention-claim-01',decode(repeat('aa',32),'hex'),$7::timestamptz,$7::timestamptz+interval '24 hours')`, claimID, reservationID, hostID, renterID, checkoutID, evidenceID, h.now.Add(-72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.reclamo_dano_resolucion_ensayo_local(reclamo_id,resultado,motivo_codigo,administrador_id,resuelta_en,clave_idempotencia,huella_solicitud)
		VALUES($1,'rechazado','hecho_no_acreditado',$2,$3,'retention-resolution-01',decode(repeat('ab',32),'hex'))`, claimID, adminID, h.now.Add(-48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err = h.pool.Exec(h.ctx, `INSERT INTO public.reserva_decision_financiera_ensayo_local(id,garantia_id,reserva_id,reclamo_id,resultado_reclamo,deduccion_clp,motivo_codigo,administrador_id,clave_idempotencia,huella_solicitud,estado,creada_en,actualizada_en)
		VALUES('85300000-0000-4000-8000-000000000015',$1,$2,$3,'rechazado',0,'sin_deduccion',$4,'retention-finance-01',decode(repeat('ac',32),'hex'),'pendiente',$5,$5)`, guaranteeID, reservationID, claimID, adminID, h.now); err != nil {
		t.Fatal(err)
	}
	decisionBlocked, err := service.PurgeExpiredReservationLinks(h.ctx, purgeAt, 20)
	if err != nil || decisionBlocked.Purged != 0 || decisionBlocked.Deferred != 1 {
		t.Fatalf("pending financial decision did not defer link purge: %+v err=%v", decisionBlocked, err)
	}
	assertLinks("financial decision pending")
	if _, err = h.pool.Exec(h.ctx, `UPDATE public.reserva_decision_financiera_ensayo_local SET estado='sin_deduccion',actualizada_en=$2 WHERE reserva_id=$1`, reservationID, h.now); err != nil {
		t.Fatal(err)
	}
	cleared, err := service.PurgeExpiredReservationLinks(h.ctx, purgeAt, 20)
	if err != nil || cleared.Purged != 1 || cleared.Deferred != 0 {
		t.Fatalf("resolved local financial obligations did not allow purge: %+v err=%v", cleared, err)
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
	// Multiple API workers may finish the same request after a retry or lease
	// recovery. The row lock plus primary key must leave one shared registry row.
	runtimeRepos := []*identitypg.IdentityRepository{
		newRuntimeIdentityRepository(t, h),
		newRuntimeIdentityRepository(t, h),
	}
	finishErrors := make(chan error, 8)
	for i := 0; i < cap(finishErrors); i++ {
		go func(index int) {
			_, finishErr := runtimeRepos[index%len(runtimeRepos)].FinishSuppression(h.ctx, request.ID, h.now)
			finishErrors <- finishErr
		}(i)
	}
	for i := 0; i < cap(finishErrors); i++ {
		if finishErr := <-finishErrors; finishErr != nil {
			t.Fatalf("concurrent replay registry finish: %v", finishErr)
		}
	}
	var durableRows int
	if err := h.pool.QueryRow(h.ctx, `SELECT count(*) FROM public.registro_replay_baja_local WHERE ejecucion_origen_id=$1`, manifest.Entries[0].ExecutionID).Scan(&durableRows); err != nil || durableRows != 1 {
		t.Fatalf("concurrent durable replay rows=%d err=%v", durableRows, err)
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
	// A newly constructed service with a separate runtime pool represents an API
	// process restart. The registry stays readable after source rows disappear.
	restartedService, err := privacy.NewService(newRuntimeIdentityRepository(t, h))
	if err != nil {
		t.Fatal(err)
	}
	registryAfterRestart, err := restartedService.ExportSuppressionReplayManifest(h.ctx)
	if err != nil || len(registryAfterRestart.Entries) != 1 || registryAfterRestart.Entries[0].ExecutionID != manifest.Entries[0].ExecutionID {
		t.Fatalf("durable replay after process restart/source retention=%+v err=%v", registryAfterRestart, err)
	}
	if _, err := h.pool.Exec(h.ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'arrendatario') ON CONFLICT DO NOTHING`, targetID); err != nil {
		t.Fatal(err)
	}
	restoredService := restartedService
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
	var sourceExecutionCount int
	if err := restoredPool.QueryRow(h.ctx, `SELECT count(*) FROM public.ejecucion_baja_local`).Scan(&sourceExecutionCount); err != nil || sourceExecutionCount != 0 {
		t.Fatalf("restored database unexpectedly contains source execution count=%d err=%v", sourceExecutionCount, err)
	}
	priorInfo, err := os.Stat(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	olderMtime := priorInfo.ModTime().Add(-time.Hour)
	if err := os.Chtimes(registryPath, olderMtime, olderMtime); err != nil {
		t.Fatal(err)
	}
	workerCtx, stopWorker := context.WithCancel(h.ctx)
	workerDone := make(chan struct{})
	go func() {
		restoredService.RunSuppressionCleanupWorker(workerCtx, time.Hour, noOpPrivacyCleaner{}, registryPath)
		close(workerDone)
	}()
	workerDeadline := time.Now().Add(5 * time.Second)
	for {
		updated, statErr := os.Stat(registryPath)
		if statErr == nil && updated.ModTime().After(olderMtime) {
			break
		}
		if time.Now().After(workerDeadline) {
			stopWorker()
			<-workerDone
			t.Fatalf("worker did not refresh external registry after restoring older database: %v", statErr)
		}
		time.Sleep(10 * time.Millisecond)
	}
	stopWorker()
	<-workerDone
	registryBytes, err = os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	var restoredRegistry privacy.SuppressionReplayManifest
	if err := json.Unmarshal(registryBytes, &restoredRegistry); err != nil || restoredRegistry.Version != 1 || len(restoredRegistry.Entries) != 1 || restoredRegistry.Entries[0].ExecutionID != manifest.Entries[0].ExecutionID {
		t.Fatalf("restored database worker discarded external suppression registry: manifest=%+v err=%v", restoredRegistry, err)
	}
	results, err := restoredService.ReplaySuppressionManifest(h.ctx, restoredRegistry, "85300000-0000-4000-8000-000000000001", adminID, h.now, noOpPrivacyCleaner{})
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
	again, err := restoredService.ReplaySuppressionManifest(h.ctx, restoredRegistry, "85300000-0000-4000-8000-000000000001", adminID, h.now, noOpPrivacyCleaner{})
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

func TestSuppressionAndRetentionPreserveUnstartedSettlementObligation(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "settlement-obligation-host@ejemplo.invalid")
	h.verify(t)
	renterID := h.register(t, "settlement-obligation-renter@ejemplo.invalid")
	h.verify(t)
	adminID := h.register(t, "settlement-obligation-admin@ejemplo.invalid")
	h.verify(t)
	if _, err := h.pool.Exec(h.ctx, "INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,'administrador')", adminID); err != nil {
		t.Fatal(err)
	}
	spaceID := "87200000-0000-4000-8000-000000000001"
	quoteID := "87200000-0000-4000-8000-000000000002"
	reservationID := "87200000-0000-4000-8000-000000000003"
	occupancyID := "87200000-0000-4000-8000-000000000004"
	seedPrivacyReviewReservation(t, h, spaceID, quoteID, reservationID, occupancyID, hostID, renterID, "finalizada")
	checkoutID := "87200000-0000-4000-8000-000000000005"
	if _, err := h.pool.Exec(h.ctx, "UPDATE public.cotizacion_reserva_ensayo SET liquidacion_politica_version='liquidacion_local_0pct_v1',liquidacion_comision_bps=0 WHERE id=$1", quoteID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, "UPDATE public.reserva_ensayo_local SET liquidacion_politica_version='liquidacion_local_0pct_v1',liquidacion_comision_bps=0 WHERE id=$1", reservationID); err != nil {
		t.Fatal(err)
	}
	checkoutSQL := "INSERT INTO public.operacion_arriendo_ensayo_local(id,reserva_id,tipo,actor_id,ocurrio_en,zona_horaria,ubicacion_sintetica,resultado,clave_idempotencia,huella_solicitud,creada_en) VALUES($1,$2,'checkout',$3,$4,'UTC','{\"source\":\"synthetic-fixture-v1\",\"location_code\":\"santiago-demo-center-v1\",\"latitude\":-33.45,\"longitude\":-70.66}'::jsonb,'registrada','settlement-checkout',decode(repeat('11',32),'hex'),$4)"
	if _, err := h.pool.Exec(h.ctx, checkoutSQL, checkoutID, reservationID, renterID, h.now.Add(-25*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, "INSERT INTO public.reserva_pago_ensayo(id,reserva_id,resultado,clave_idempotencia,creada_en,importe_clp) VALUES('87200000-0000-4000-8000-000000000006',$1,'exito_simulado','confirmed-rent',$2,12000)", reservationID, h.now.Add(-30*time.Hour)); err != nil {
		t.Fatal(err)
	}
	requester, err := privacy.NewService(h.repo)
	if err != nil {
		t.Fatal(err)
	}
	request, err := requester.RequestRight(h.ctx, hostID, "supresion", "web")
	if err != nil {
		t.Fatal(err)
	}
	review, err := h.repo.ReviewSuppression(h.ctx, adminID, request.ID, "unstarted-settlement-review", "unstarted-settlement", func() time.Time { return h.now })
	if err != nil || review.Outcome != "bloqueada" || !reflect.DeepEqual(review.Obligations, []string{"liquidacion_pendiente"}) {
		t.Fatalf("unstarted settlement was not detected: review=%+v err=%v", review, err)
	}
	execution, err := h.repo.ExecuteSuppression(h.ctx, adminID, request.ID, "unstarted-settlement-execute", "unstarted-settlement", func() time.Time { return h.now })
	if err != nil || execution.Status != "bloqueada" || !reflect.DeepEqual(execution.Obligations, []string{"liquidacion_pendiente"}) {
		t.Fatalf("suppression execution must recheck missing intent: execution=%+v err=%v", execution, err)
	}
	deadline := h.now.AddDate(2, 0, 0)
	if _, err := h.pool.Exec(h.ctx, "UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2 WHERE id=$1", reservationID, deadline); err != nil {
		t.Fatal(err)
	}
	runtime := newRuntimeIdentityRepository(t, h)
	purger, err := privacy.NewService(runtime)
	if err != nil {
		t.Fatal(err)
	}
	purged, err := purger.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || purged.Purged != 0 || purged.Deferred != 1 {
		t.Fatalf("purgator must preserve identifiers before settlement intent: result=%+v err=%v", purged, err)
	}
	var host, renter string
	var markers, intents int
	var accountState string
	if err := h.pool.QueryRow(h.ctx, "SELECT COALESCE(r.anfitrion_id::text,''),COALESCE(r.arrendatario_id::text,''),(SELECT count(*) FROM public.reserva_vinculo_purgado_local WHERE reserva_id=$1),(SELECT count(*) FROM public.reserva_liquidacion_ensayo_local WHERE reserva_id=$1),u.estado FROM public.reserva_ensayo_local r JOIN public.usuario u ON u.id=r.anfitrion_id WHERE r.id=$1", reservationID).Scan(&host, &renter, &markers, &intents, &accountState); err != nil {
		t.Fatal(err)
	}
	if host != hostID || renter != renterID || markers != 0 || intents != 0 || accountState != "activo" {
		t.Fatalf("missing-intent obligation lost identifiers or created effects: host=%q renter=%q markers=%d intents=%d account=%s", host, renter, markers, intents, accountState)
	}
	// Preserved links still allow the due operation to be initiated.
	repo := bookingpg.New(h.pool)
	fingerprint := make([]byte, 32)
	for i := range fingerprint {
		fingerprint[i] = 7
	}
	settlement, reused, err := repo.BeginSettlement(h.ctx, hostID, reservationID, "unstarted-settlement-key", fingerprint, "87200000-0000-4000-8000-000000000007", "87200000-0000-4000-8000-000000000008", "exito", func() time.Time { return h.now })
	if err != nil || reused || settlement.State != "pendiente" || settlement.TransferCLP != 12000 {
		t.Fatalf("preserved settlement could not begin: settlement=%+v reused=%v err=%v", settlement, reused, err)
	}
}

func TestLocalSettlementHistoryActorLinksAreUnlinkedAtRetention(t *testing.T) {
	h := newAuthHarness(t)
	hostID := h.register(t, "settlement-history-host@ejemplo.invalid")
	renterID := h.register(t, "settlement-history-renter@ejemplo.invalid")
	reservationID := "87300000-0000-4000-8000-000000000003"
	quoteID := "87300000-0000-4000-8000-000000000002"
	occupancyID := "87300000-0000-4000-8000-000000000004"
	seedPrivacyReviewReservation(t, h, "87300000-0000-4000-8000-000000000001", quoteID, reservationID, occupancyID, hostID, renterID, "finalizada")
	deadline := h.now.AddDate(2, 0, 0)
	settlementID := "87300000-0000-4000-8000-000000000005"
	operationID := "87300000-0000-4000-8000-000000000006"
	if _, err := h.pool.Exec(h.ctx, "UPDATE public.reserva_ensayo_local SET vinculos_retirar_en=$2 WHERE id=$1", reservationID, deadline); err != nil {
		t.Fatal(err)
	}
	settlementSQL := "INSERT INTO public.reserva_liquidacion_ensayo_local(id,reserva_id,anfitrion_id,politica_version,moneda,arriendo_confirmado_clp,devoluciones_confirmadas_clp,comision_bps,comision_clp,transferencia_clp,estado,clave_idempotencia,huella_solicitud,creada_en,actualizada_en,completada_en) VALUES($1,$2,$3,'liquidacion_local_0pct_v1','CLP',12000,0,0,0,12000,'liquidada','history-settlement',decode(repeat('22',32),'hex'),$4,$4,$4)"
	if _, err := h.pool.Exec(h.ctx, settlementSQL, settlementID, reservationID, hostID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, "INSERT INTO public.reserva_liquidacion_operacion_ensayo_local(id,liquidacion_id,resultado_solicitado,creada_en,actualizada_en) VALUES($1,$2,'exito',$3,$3)", operationID, settlementID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, "INSERT INTO public.reserva_liquidacion_historial_ensayo_local(liquidacion_id,accion,actor_id,codigo,ocurrida_en) VALUES($1,'liquidacion_confirmada',$2,'fake_confirmed',$3)", settlementID, hostID, h.now); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(h.ctx, "INSERT INTO public.reserva_finanzas_historial_ensayo_local(reserva_id,tipo,actor_id,detalle_codigo,ocurrida_en) VALUES($1,'liquidacion_confirmada',$2,'fake_confirmed',$3)", reservationID, renterID, h.now); err != nil {
		t.Fatal(err)
	}
	runtime := newRuntimeIdentityRepository(t, h)
	service, err := privacy.NewService(runtime)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.PurgeExpiredReservationLinks(h.ctx, deadline, 20)
	if err != nil || result.Purged != 1 || result.Deferred != 0 {
		t.Fatalf("purge completed settlement history: result=%+v err=%v", result, err)
	}
	var settlementActor, financeActor, action, code string
	var preservedAmount int64
	var settlementAt, financeAt time.Time
	readSQL := "SELECT COALESCE(h.actor_id::text,''),COALESCE(f.actor_id::text,''),h.accion,h.codigo,s.transferencia_clp,h.ocurrida_en,f.ocurrida_en FROM public.reserva_liquidacion_historial_ensayo_local h JOIN public.reserva_liquidacion_ensayo_local s ON s.id=h.liquidacion_id JOIN public.reserva_finanzas_historial_ensayo_local f ON f.reserva_id=s.reserva_id AND f.tipo='liquidacion_confirmada' WHERE s.reserva_id=$1"
	if err := h.pool.QueryRow(h.ctx, readSQL, reservationID).Scan(&settlementActor, &financeActor, &action, &code, &preservedAmount, &settlementAt, &financeAt); err != nil {
		t.Fatal(err)
	}
	if settlementActor != "" || financeActor != "" || action != "liquidacion_confirmada" || code != "fake_confirmed" || preservedAmount != 12000 || !settlementAt.Equal(h.now) || !financeAt.Equal(h.now) {
		t.Fatalf("actor links/facts retention mismatch: actors=%q/%q action=%q code=%q amount=%d times=%s/%s", settlementActor, financeActor, action, code, preservedAmount, settlementAt, financeAt)
	}
	var tableUpdate, tableDelete, columnUpdate, financeUpdate bool
	if err := h.pool.QueryRow(h.ctx, "SELECT has_table_privilege('espacigo_runtime','public.reserva_liquidacion_historial_ensayo_local','UPDATE'),has_table_privilege('espacigo_runtime','public.reserva_liquidacion_historial_ensayo_local','DELETE'),has_column_privilege('espacigo_runtime','public.reserva_liquidacion_historial_ensayo_local','actor_id','UPDATE'),has_table_privilege('espacigo_runtime','public.reserva_finanzas_historial_ensayo_local','UPDATE')").Scan(&tableUpdate, &tableDelete, &columnUpdate, &financeUpdate); err != nil {
		t.Fatal(err)
	}
	if tableUpdate || tableDelete || !columnUpdate || financeUpdate {
		t.Fatalf("history permission scope update/delete/actor-column/finance-update=%t/%t/%t/%t", tableUpdate, tableDelete, columnUpdate, financeUpdate)
	}
}
