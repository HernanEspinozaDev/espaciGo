package migrator

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	documentverificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/documentverification"
	rentalflowpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/rentalflow"
	"github.com/HernanEspinozaDev/espaciGo/internal/identityverification"
	"github.com/HernanEspinozaDev/espaciGo/internal/rentalflow"
	rentalflowhttp "github.com/HernanEspinozaDev/espaciGo/internal/rentalflow/transport/http"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification/declarationcrypto"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// assertContractIdentityReaderPostgres is a real-PostgreSQL characterization of
// the A08 reader. Direct case insertion is a synthetic reader fixture, not
// evidence of the human review lifecycle.
func assertContractIdentityReaderPostgres(t *testing.T, ctx context.Context, adminURL string, admin *pgx.Conn) {
	t.Helper()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.User = url.User("espacigo_api_login")
	cfg, err := pgxpool.ParseConfig(u.String())
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 2
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE espacigo_runtime")
		return err
	}
	runtime, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	ids := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	owner, renter, stranger, reviewer := ids[0], ids[1], ids[2], ids[3]
	for _, id := range ids {
		if _, err := admin.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado) VALUES($1,$2,$2,'synthetic-only','activo')`, id, id+"@identity-reader.invalid"); err != nil {
			t.Fatal(err)
		}
	}
	spaceID, quoteID := uuid.NewString(), uuid.NewString()
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado,zona_horaria) VALUES($1,$2,'oficina','A08 reader fixture',repeat('Synthetic fixture description. ',4),10,1,'Synthetic','hora',10000,'Synthetic','activa','UTC')`, spaceID, owner); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',10000)`, spaceID); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	intervalStart := now.Add(24 * time.Hour)
	intervalStart = time.Date(intervalStart.Year(), intervalStart.Month(), intervalStart.Day(), 10, 0, 0, 0, time.UTC)
	intervalEnd := intervalStart.Add(time.Hour)
	weekday := int16((int(intervalStart.Weekday())+6)%7 + 1)
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio_horario_semanal(espacio_id,activo) VALUES($1,true)`, spaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.espacio_horario_semanal_tramo(espacio_id,dia_iso,orden,apertura_minuto,cierre_minuto) VALUES($1,$2,1,600,660)`, spaceID, weekday); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.cotizacion_arriendo_comercial(id,espacio_id,propietario_id,arrendatario_id,titulo_espacio,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,creada_en,vence_en) VALUES($1,$2,$3,$4,'A08 reader fixture',1,'hora',10000,'CLP',1,10000,$5,$6,'UTC',$7,$8)`, quoteID, spaceID, owner, renter, intervalStart, intervalEnd, now, now.Add(rentalflow.QuoteLifetime)); err != nil {
		t.Fatal(err)
	}

	keys, err := declarationcrypto.New("a08-test-v1", map[string][]byte{"a08-test-v1": make([]byte, 32)})
	if err != nil {
		t.Fatal(err)
	}
	caseIDs := map[string]string{}
	for i, subject := range []string{owner, renter} {
		declarationID := uuid.NewString()
		first, last, rut := "Owner", "Synthetic", "12345678-5"
		if i == 1 {
			first, last, rut = "Renter", "Synthetic", "11111111-1"
		}
		payload, err := verification.EncodeDeclarationPayload(verification.DeclarationPayload{FirstName: first, LastName: last, DateOfBirth: "1990-01-01", RUT: rut, IDCountry: "CHL", DocumentType: "ID"})
		if err != nil {
			t.Fatal(err)
		}
		aad, err := verification.BuildDeclarationAAD(subject, declarationID, 1, "1")
		if err != nil {
			t.Fatal(err)
		}
		keyVersion, ciphertext, err := keys.Seal(aad, payload)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at) VALUES($1,$2,1,$3,$4,'1','identity-declaration-v1',$5,$6,$5)`, declarationID, subject, keyVersion, ciphertext, now.Add(-time.Hour), now.Add(24*time.Hour)); err != nil {
			t.Fatal(err)
		}
		caseID := uuid.NewString()
		if _, err := admin.Exec(ctx, `INSERT INTO public.document_verification_case(id,purpose,subject_id,state,revision,reviewer_id,verified_at,verified_until,document_expires_at,created_at,updated_at,identity_declaration_revision_id,document_format_declared,capture_consent_version,capture_consented_at) VALUES($1,'identity',$2,'aprobada',1,$3,$4,$5,$5,$4,$4,$6,'new','identity-capture-local-v1',$4)`, caseID, subject, reviewer, now.Add(-time.Hour), now.Add(24*time.Hour), declarationID); err != nil {
			t.Fatal(err)
		}
		caseIDs[subject] = caseID
	}
	repo, err := rentalflowpg.NewRepositoryWithDeclarationKeyring(runtime, runtime, func() time.Time { return now }, keys)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ actor, subject, name, rut string }{{owner, renter, "Renter Synthetic", "11111111-1"}, {renter, owner, "Owner Synthetic", "12345678-5"}} {
		got, err := repo.CurrentForRentalContract(ctx, tc.actor, quoteID, tc.subject, now)
		if err != nil || got.SubjectID != tc.subject || got.LegalName != tc.name || got.RUT != tc.rut {
			t.Fatalf("participant identity projection mismatch: got=%+v err=%v", got, err)
		}
	}
	documentVerificationRepo := documentverificationpg.New(runtime)
	identityGate, err := identityverification.NewService(documentVerificationRepo)
	if err != nil {
		t.Fatal(err)
	}
	previewService, err := rentalflow.NewContractPreviewService(repo, repo, repo, identityGate, repo, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	preview, err := previewService.Preview(ctx, renter, quoteID)
	if err != nil || len(preview.Parties) != 2 || preview.Parties[0].LegalName != "Owner Synthetic" || preview.Parties[0].RUT != "12345678-5" || preview.Parties[1].LegalName != "Renter Synthetic" || preview.Parties[1].RUT != "11111111-1" {
		t.Fatalf("contract preview service identity projection mismatch: preview=%+v err=%v", preview, err)
	}
	handler := rentalflowhttp.New(commercialQuoteIntegrationAuthenticator{accountID: renter}, nil, nil, previewService, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+quoteID+"/contract-preview", nil)
	request.Header.Set("Authorization", "Bearer synthetic-integration-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("contract preview HTTP returned %d: %s", response.Code, response.Body.String())
	}
	var envelope struct {
		Data rentalflow.ContractPreview `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if len(envelope.Data.Parties) != 2 || envelope.Data.Parties[0].LegalName != "Owner Synthetic" || envelope.Data.Parties[1].LegalName != "Renter Synthetic" {
		t.Fatalf("contract preview HTTP identity projection mismatch: %+v", envelope.Data.Parties)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.document_verification_case SET verified_until=$2,updated_at=$2 WHERE id=$1`, caseIDs[owner], now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, renter, quoteID, owner, now); !errors.Is(err, rentalflow.ErrIdentityUnavailable) {
		t.Fatalf("identity reader accepted a case at its verification-expiry boundary: %v", err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.document_verification_case SET verified_until=$2,updated_at=$3 WHERE id=$1`, caseIDs[owner], now.Add(24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.document_verification_case SET document_expires_at=$2,updated_at=$2 WHERE id=$1`, caseIDs[owner], now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, renter, quoteID, owner, now); !errors.Is(err, rentalflow.ErrIdentityUnavailable) {
		t.Fatalf("identity reader accepted an expired identity document: %v", err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.document_verification_case SET document_expires_at=$2,updated_at=$3 WHERE id=$1`, caseIDs[owner], now.Add(24*time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.identity_declaration_revision SET retention_at=$2 WHERE id=(SELECT identity_declaration_revision_id FROM public.document_verification_case WHERE id=$1)`, caseIDs[owner], now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, renter, quoteID, owner, now); !errors.Is(err, rentalflow.ErrIdentityUnavailable) {
		t.Fatalf("identity reader accepted a declaration at its retention boundary: %v", err)
	}
	if _, err := admin.Exec(ctx, `UPDATE public.identity_declaration_revision SET retention_at=$2 WHERE id=(SELECT identity_declaration_revision_id FROM public.document_verification_case WHERE id=$1)`, caseIDs[owner], now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, stranger, quoteID, owner, now); err != rentalflow.ErrNotFound {
		t.Fatalf("stranger disclosed identity: %v", err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, owner, quoteID, stranger, now); err != rentalflow.ErrNotFound {
		t.Fatalf("quote subject mismatch disclosed identity: %v", err)
	}

	// A newer saved declaration invalidates the old approval's exact-revision binding.
	newID := uuid.NewString()
	payload, err := verification.EncodeDeclarationPayload(verification.DeclarationPayload{FirstName: "New", LastName: "Revision", DateOfBirth: "1990-01-01", RUT: "12345678-5", IDCountry: "CHL", DocumentType: "ID"})
	if err != nil {
		t.Fatal(err)
	}
	aad, err := verification.BuildDeclarationAAD(owner, newID, 2, "1")
	if err != nil {
		t.Fatal(err)
	}
	keyVersion, ciphertext, err := keys.Seal(aad, payload)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, `INSERT INTO public.identity_declaration_revision(id,account_id,revision,key_version,ciphertext,schema_version,consent_version,consent_given_at,retention_at,created_at) VALUES($1,$2,2,$3,$4,'1','identity-declaration-v1',$5,$6,$5)`, newID, owner, keyVersion, ciphertext, now.Add(time.Second), now.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, renter, quoteID, owner, now.Add(2*time.Second)); err != rentalflow.ErrIdentityUnavailable {
		t.Fatalf("newer declaration failed to invalidate approval: %v", err)
	}
	// Commit a revocation after the participant read establishes the snapshot,
	// but before the declaration query. The in-flight read must stay coherent;
	// a later read must observe the revocation and withhold identity.
	revokedAt := now.Add(time.Second)
	hooked := false
	interposed, err := rentalflowpg.NewRepositoryWithDeclarationKeyring(
		runtime,
		&rentalIdentityInterposingBeginner{
			pool:         runtime,
			hookObserved: &hooked,
			afterParticipant: func() error {
				tag, err := admin.Exec(ctx, `UPDATE public.document_verification_case SET state='revocada',revoked_at=$2,updated_at=$2 WHERE id=$1`, caseIDs[renter], revokedAt)
				if err != nil {
					return err
				}
				if tag.RowsAffected() != 1 {
					return errors.New("synthetic identity case was not revoked")
				}
				return nil
			},
		},
		func() time.Time { return now },
		keys,
	)
	if err != nil {
		t.Fatal(err)
	}
	inFlight, err := interposed.CurrentForRentalContract(ctx, owner, quoteID, renter, now)
	if err != nil || !hooked || inFlight.LegalName != "Renter Synthetic" {
		t.Fatalf("repeatable-read identity snapshot mismatch: hooked=%t identity=%+v err=%v", hooked, inFlight, err)
	}
	if _, err := repo.CurrentForRentalContract(ctx, owner, quoteID, renter, revokedAt.Add(time.Second)); !errors.Is(err, rentalflow.ErrIdentityUnavailable) {
		t.Fatalf("a later read disclosed an identity after revocation: %v", err)
	}
	if _, err := previewService.Preview(ctx, renter, quoteID); !errors.Is(err, rentalflow.ErrIdentityRequired) {
		t.Fatalf("preview service did not block a revoked party: %v", err)
	}
	revokedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/rental-quotes/"+quoteID+"/contract-preview", nil)
	revokedRequest.Header.Set("Authorization", "Bearer synthetic-integration-token")
	revokedResponse := httptest.NewRecorder()
	handler.ServeHTTP(revokedResponse, revokedRequest)
	body := revokedResponse.Body.String()
	if revokedResponse.Code != http.StatusConflict || strings.Contains(body, "Owner Synthetic") || strings.Contains(body, "Renter Synthetic") || strings.Contains(body, "12345678-5") || strings.Contains(body, "11111111-1") {
		t.Fatalf("revoked HTTP preview leaked identity or returned the wrong status: status=%d body=%q", revokedResponse.Code, body)
	}
}

type rentalIdentityInterposingBeginner struct {
	pool             *pgxpool.Pool
	afterParticipant func() error
	hookObserved     *bool
}

func (b *rentalIdentityInterposingBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := b.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &rentalIdentityInterposingTx{Tx: tx, afterParticipant: b.afterParticipant, hookObserved: b.hookObserved}, nil
}

type rentalIdentityInterposingTx struct {
	pgx.Tx
	afterParticipant func() error
	hookObserved     *bool
	hooked           bool
}

func (tx *rentalIdentityInterposingTx) QueryRow(ctx context.Context, query string, args ...any) pgx.Row {
	row := tx.Tx.QueryRow(ctx, query, args...)
	if tx.hooked || !strings.Contains(query, "cotizacion_arriendo_comercial") {
		return row
	}
	tx.hooked = true
	return rentalIdentityInterposingRow{Row: row, afterScan: func() error {
		if tx.hookObserved != nil {
			*tx.hookObserved = true
		}
		return tx.afterParticipant()
	}}
}

type rentalIdentityInterposingRow struct {
	pgx.Row
	afterScan func() error
}

func (row rentalIdentityInterposingRow) Scan(dest ...any) error {
	if err := row.Row.Scan(dest...); err != nil {
		return err
	}
	return row.afterScan()
}
