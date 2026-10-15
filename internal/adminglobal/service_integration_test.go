package adminglobal_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/devauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/fakebooking"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	gallerypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/gallery"
	identitypg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/identity"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminglobal"
	"github.com/HernanEspinozaDev/espaciGo/internal/adminlocal"
	bookingdomain "github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"github.com/HernanEspinozaDev/espaciGo/internal/migrator"
	spacesdomain "github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestGlobalBootstrapDirectoryAndLastMasterConcurrency(t *testing.T) {
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; needs a disposable PostgreSQL 18/PostGIS 3.6 instance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	admin, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	dbName := fmt.Sprintf("espacigo_gadm01_%08x", uint32(time.Now().UnixNano()))
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, e := admin.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()); e != nil {
			t.Errorf("drop isolated database: %v", e)
		}
	}()
	u, err := url.Parse(adminURL)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + dbName
	_, source, _, _ := runtime.Caller(0)
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(source), "..", "..", "db", "migrations"))
	if _, err = migrator.Run(ctx, u.String(), migrationDir); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	const owner = "00000000-0000-4000-8000-000000000001"
	const other = "00000000-0000-4000-8000-000000000002"
	const legacy = "00000000-0000-4000-8000-000000000003"
	const landlord = "00000000-0000-4000-8000-000000000004"
	const renter = "00000000-0000-4000-8000-000000000005"
	const pending = "00000000-0000-4000-8000-000000000006"
	_, err = pool.Exec(ctx, `INSERT INTO public.usuario(id,correo_original,correo_normalizado,hash_clave,estado,creado_en)
		VALUES($1,'owner@example.invalid','owner@example.invalid','synthetic','activo','2026-01-01'),
		($2,'other@example.invalid','other@example.invalid','synthetic','activo','2026-01-01'),
		($3,'legacy@example.invalid','legacy@example.invalid','synthetic','activo','2026-01-01'),
		($4,'host@example.invalid','host@example.invalid','synthetic','activo','2026-01-01'),
		($5,'renter@example.invalid','renter@example.invalid','synthetic','activo','2026-01-01'),
		($6,'pending@example.invalid','pending@example.invalid','synthetic','correo_pendiente','2026-01-01')`, owner, other, legacy, landlord, renter, pending)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ id, name, role string }{{owner, "Owner Test", "arrendatario"}, {other, "Second Master", "arrendatario"}, {legacy, "Legacy Admin", "administrador"}, {landlord, "Test Host", "arrendador"}, {renter, "Test Renter", "arrendatario"}, {pending, "Pending User", "arrendatario"}} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.perfil_usuario(usuario_id,nombre_visible) VALUES($1,$2)`, item.id, item.name); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO public.rol_usuario(usuario_id,rol) VALUES($1,$2)`, item.id, item.role); err != nil {
			t.Fatal(err)
		}
	}
	corr := "10000000-0000-4000-8000-000000000001"
	if err = adminglobal.Bootstrap(ctx, pool, "00000000-0000-4000-8000-000000000099", "bootstrap", corr); err == nil {
		t.Fatal("unknown target accepted")
	}
	if err = adminglobal.Bootstrap(ctx, pool, pending, "bootstrap", corr); err == nil {
		t.Fatal("unverified target accepted")
	}
	var bootstrapWG sync.WaitGroup
	bootstrapResults := make(chan error, 2)
	for _, candidate := range []string{owner, other} {
		bootstrapWG.Add(1)
		go func(id string) {
			defer bootstrapWG.Done()
			bootstrapResults <- adminglobal.Bootstrap(ctx, pool, id, "bootstrap", corr)
		}(candidate)
	}
	bootstrapWG.Wait()
	close(bootstrapResults)
	twins, rejectedRaces := 0, 0
	for e := range bootstrapResults {
		if e == nil {
			twins++
		} else {
			rejectedRaces++
		}
	}
	if twins != 1 || rejectedRaces != 1 {
		t.Fatalf("concurrent bootstrap results success=%d rejected=%d", twins, rejectedRaces)
	}
	var master string
	if err = pool.QueryRow(ctx, `SELECT usuario_id::text FROM public.administrador_global_bootstrap WHERE singleton=true`).Scan(&master); err != nil {
		t.Fatal(err)
	}
	loser := owner
	if master == owner {
		loser = other
	}
	if err = adminglobal.Bootstrap(ctx, pool, master, "bootstrap", corr); err != nil {
		t.Fatalf("same-target replay should be idempotent: %v", err)
	}
	if err = adminglobal.Bootstrap(ctx, pool, loser, "bootstrap", corr); err == nil {
		t.Fatal("different-target replay accepted")
	}
	if _, err = pool.Exec(ctx, `DELETE FROM public.administrador_global WHERE usuario_id=$1`, master); err != nil {
		t.Fatal(err)
	}
	credentialFile := makeLocalCLICredentials(t, u.String(), dbName)
	root := repositoryRoot(source)
	if err = runBootstrapCLI(ctx, root, credentialFile, "recover", master); err != nil {
		t.Fatalf("gadm-admin recover failed to restore the bound UUID: %v", err)
	}
	var restored bool
	if err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM public.administrador_global WHERE usuario_id=$1)`, master).Scan(&restored); err != nil || !restored {
		t.Fatalf("recover did not restore membership: restored=%v err=%v", restored, err)
	}
	if err = runBootstrapCLI(ctx, root, credentialFile, "recover", loser); err == nil {
		t.Fatal("gadm-admin recover accepted a different UUID")
	}
	var recoverySuccess, recoveryReject int
	if err = pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE resultado='exito'),count(*) FILTER (WHERE resultado='rechazo') FROM public.administrador_global_bootstrap_evento WHERE accion='recuperacion'`).Scan(&recoverySuccess, &recoveryReject); err != nil {
		t.Fatal(err)
	}
	if recoverySuccess != 1 || recoveryReject != 1 {
		t.Fatalf("recover audit counts success=%d rejection=%d; want 1/1", recoverySuccess, recoveryReject)
	}
	var rejected int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.administrador_global_bootstrap_evento WHERE resultado='rechazo'`).Scan(&rejected); err != nil || rejected < 3 {
		t.Fatalf("bootstrap rejects not audited: count=%d err=%v", rejected, err)
	}
	service, err := adminglobal.New(pool, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	const publicationA = "80000000-0000-4000-8000-000000000001"
	const publicationB = "80000000-0000-4000-8000-000000000002"
	for _, id := range []string{publicationA, publicationB} {
		if _, err = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado) VALUES($1,$2,'oficina','Global managed space','Descripción sintética suficientemente larga para cumplir el mínimo de cien caracteres en una ficha local de ensayo.',20,4,'Cuidar los bienes y respetar el horario','hora',12000,'Dirección de ensayo','borrador')`, id, landlord); err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',12000)`, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) SELECT $1,'oficina',max(version),'{}'::jsonb FROM public.categoria_perfil_atributos WHERE categoria_codigo='oficina'`, publicationB); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) SELECT $1,'oficina',max(version),' {"wifi":true}'::jsonb FROM public.categoria_perfil_atributos WHERE categoria_codigo='oficina'`, publicationA); err != nil {
		t.Fatal(err)
	}
	publicationPage, err := service.ListPublications(ctx, master, corr, adminglobal.PublicationFilter{OwnerID: landlord, Category: "oficina", State: "borrador", PageSize: 1})
	if err != nil || publicationPage.MatchingTotal != 2 || len(publicationPage.Items) != 1 || publicationPage.NextCursor == "" || publicationPage.Items[0].PubliclyVisible || publicationPage.Items[0].BasePriceCLP != "12000" {
		t.Fatalf("publication inventory/filter/page: %+v err=%v", publicationPage, err)
	}
	publicationNext, err := service.ListPublications(ctx, master, corr, adminglobal.PublicationFilter{OwnerID: landlord, Category: "oficina", State: "borrador", PageSize: 1, Cursor: publicationPage.NextCursor})
	if err != nil || len(publicationNext.Items) != 1 || publicationNext.Items[0].ID == publicationPage.Items[0].ID {
		t.Fatalf("publication cursor repeated item: %+v err=%v", publicationNext, err)
	}
	if _, err = service.ListPublications(ctx, master, corr, adminglobal.PublicationFilter{OwnerID: landlord, Category: "bodega", State: "borrador", PageSize: 1, Cursor: publicationPage.NextCursor}); !errors.Is(err, adminglobal.ErrInvalid) {
		t.Fatalf("publication cursor accepted changed filter: %v", err)
	}
	var publicationVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationA).Scan(&publicationVersion); err != nil {
		t.Fatal(err)
	}
	newTitle := "Global correction"
	newPrice := "18000"
	updatedPublication, err := service.UpdatePublication(ctx, master, publicationA, corr, adminglobal.PublicationContentUpdate{Title: &newTitle, BasePriceCLP: &newPrice, ExpectedUpdatedAt: publicationVersion, Reason: "revision_administrativa"})
	if err != nil || updatedPublication.ID != publicationA || updatedPublication.OwnerID != landlord {
		t.Fatalf("global publication update: %+v err=%v", updatedPublication, err)
	}
	if _, err = service.UpdatePublication(ctx, master, publicationA, corr, adminglobal.PublicationContentUpdate{Title: &newTitle, ExpectedUpdatedAt: publicationVersion, Reason: "revision_administrativa"}); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("stale publication update returned %v", err)
	}
	var attributeSchemaVersion int
	if err = pool.QueryRow(ctx, `SELECT perfil_version FROM public.espacio_caracteristicas WHERE espacio_id=$1`, publicationA).Scan(&attributeSchemaVersion); err != nil {
		t.Fatal(err)
	}
	var attributesVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationA).Scan(&attributesVersion); err != nil {
		t.Fatal(err)
	}
	updatedAttributes, err := service.UpdatePublication(ctx, master, publicationA, corr, adminglobal.PublicationContentUpdate{AttributeSchemaVersion: &attributeSchemaVersion, Attributes: map[string]any{"wifi": false}, ExpectedUpdatedAt: attributesVersion, Reason: "correccion_perfil"})
	if err != nil || updatedAttributes.ID != publicationA {
		t.Fatalf("category attribute correction: %+v err=%v", updatedAttributes, err)
	}
	var wifi bool
	if err = pool.QueryRow(ctx, `SELECT (valores->>'wifi')::boolean FROM public.espacio_caracteristicas WHERE espacio_id=$1`, publicationA).Scan(&wifi); err != nil || wifi {
		t.Fatalf("versioned category attribute not persisted: wifi=%v err=%v", wifi, err)
	}
	const parcelID = "80000000-0000-4000-8000-000000000003"
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio(id,propietario_id,categoria_codigo,titulo,descripcion,superficie_m2,capacidad_maxima,reglas_uso,modalidad_tarifa,precio_base_clp,direccion,estado) VALUES($1,$2,'parcela_eventos','Parcela de ensayo','Descripción sintética suficientemente larga para validar las áreas parciales en la corrección global.',100,20,'Uso de prueba','hora',12000,'Dirección de ensayo','borrador')`, parcelID, landlord); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.tarifa_espacio(espacio_id,version,modalidad,precio_base_clp) VALUES($1,1,'hora',12000)`, parcelID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio_caracteristicas(espacio_id,categoria_codigo,perfil_version,valores) SELECT $1,'parcela_eventos',max(version),'{}'::jsonb FROM public.categoria_perfil_atributos WHERE categoria_codigo='parcela_eventos'`, parcelID); err != nil {
		t.Fatal(err)
	}
	var parcelSchema int
	if err = pool.QueryRow(ctx, `SELECT c.perfil_version,e.actualizado_en FROM public.espacio_caracteristicas c JOIN public.espacio e ON e.id=c.espacio_id WHERE e.id=$1`, parcelID).Scan(&parcelSchema, &publicationVersion); err != nil {
		t.Fatal(err)
	}
	for _, code := range []string{"superficie_exterior_util_m2", "superficie_cubierta_util_m2"} {
		if _, err = service.UpdatePublication(ctx, master, parcelID, corr, adminglobal.PublicationContentUpdate{AttributeSchemaVersion: &parcelSchema, Attributes: map[string]any{code: 101.0}, ExpectedUpdatedAt: publicationVersion, Reason: "revision_administrativa"}); !errors.Is(err, adminglobal.ErrInvalid) {
			t.Fatalf("administrator accepted %s above the total parcel area: %v", code, err)
		}
	}
	if _, err = service.UpdatePublication(ctx, master, parcelID, corr, adminglobal.PublicationContentUpdate{AttributeSchemaVersion: &parcelSchema, Attributes: map[string]any{"superficie_exterior_util_m2": 80.0, "superficie_cubierta_util_m2": 90.0}, ExpectedUpdatedAt: publicationVersion, Reason: "revision_administrativa"}); err != nil {
		t.Fatalf("valid parcel areas rejected: %v", err)
	}
	var rateCount int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.tarifa_espacio WHERE espacio_id=$1`, publicationA).Scan(&rateCount); err != nil || rateCount != 2 {
		t.Fatalf("tariff version history count=%d err=%v", rateCount, err)
	}
	const verificationID = "90000000-0000-4000-8000-000000000001"
	if _, err = pool.Exec(ctx, `INSERT INTO public.verificacion(id,usuario_id,tipo,estado,referencia_evidencia,clave_idempotencia,revisor_id,creada_en,resuelta_en) VALUES($1,$2,'kyc','aprobada','fixture:90000000-0000-4000-8000-000000000001','gadm02-kyc',$3,now(),now())`, verificationID, landlord, legacy); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.elegibilidad_verificacion_local(usuario_id,tipo,verificacion_id,estado,concedida_en) VALUES($1,'kyc',$2,'elegible',now())`, landlord, verificationID); err != nil {
		t.Fatal(err)
	}
	var draftVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationA).Scan(&draftVersion); err != nil {
		t.Fatal(err)
	}
	activated, err := service.SetPublicationState(ctx, master, publicationA, corr, adminglobal.PublicationStateInput{State: "activa", ExpectedUpdatedAt: draftVersion, Reason: "revision_administrativa"})
	if err != nil || activated.State != "activa" || activated.OwnerID != landlord {
		t.Fatalf("policy-authorized publish: %+v err=%v", activated, err)
	}
	publicVisible, err := service.ListPublications(ctx, master, corr, adminglobal.PublicationFilter{Visible: ptr(true), PageSize: 10})
	if err != nil || publicVisible.MatchingTotal != 1 || publicVisible.Items[0].ID != publicationA {
		t.Fatalf("actual visibility filter: %+v err=%v", publicVisible, err)
	}
	if _, err = service.SetPublicationState(ctx, master, publicationA, corr, adminglobal.PublicationStateInput{State: "activa", ExpectedUpdatedAt: draftVersion, Reason: "revision_administrativa"}); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("stale publication transition returned %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.espacio SET zona_horaria='America/Santiago' WHERE id=$1`, publicationB); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.reserva_ensayo_local_fixture(espacio_id,anfitrion_id,arrendatario_id,habilitada) VALUES($1,$2,$3,true)`, publicationB, landlord, renter); err != nil {
		t.Fatal(err)
	}
	localPayment, err := fakebooking.New([]byte("integration-only-local-payment-webhook-key-32-bytes"))
	if err != nil {
		t.Fatal(err)
	}
	bookingService, err := bookingdomain.NewService(bookingpg.New(pool), credentials.Generator{}, time.Now, localPayment)
	if err != nil {
		t.Fatal(err)
	}
	service.SetWeeklyHoursManager(bookingService)
	adminSchedule, err := service.PublicationSchedule(ctx, master, publicationB, landlord, corr)
	if err != nil || adminSchedule.SpaceID != publicationB || adminSchedule.TimeZone != "America/Santiago" {
		t.Fatalf("global schedule read: %+v err=%v", adminSchedule, err)
	}
	var scheduleVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&scheduleVersion); err != nil {
		t.Fatal(err)
	}
	weekly := bookingdomain.WeeklyHours{SpaceID: publicationB, Enabled: false, Days: make([]bookingdomain.WeeklyDay, 7)}
	for index := range weekly.Days {
		weekly.Days[index] = bookingdomain.WeeklyDay{Weekday: index + 1, Periods: []bookingdomain.WeeklyPeriod{}}
	}
	savedSchedule, err := service.SavePublicationSchedule(ctx, master, publicationB, landlord, corr, scheduleVersion, "revision_administrativa", weekly)
	if err != nil || savedSchedule.SpaceID != publicationB || savedSchedule.Enabled {
		t.Fatalf("global schedule write: %+v err=%v", savedSchedule, err)
	}
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&scheduleVersion); err != nil {
		t.Fatal(err)
	}
	if _, err = bookingService.SaveWeeklyHours(ctx, landlord, publicationB, weekly); err != nil {
		t.Fatalf("owner weekly schedule write: %v", err)
	}
	var ownerScheduleVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&ownerScheduleVersion); err != nil || !ownerScheduleVersion.After(scheduleVersion) {
		t.Fatalf("owner weekly schedule did not advance shared publication version: prior=%s updated=%s err=%v", scheduleVersion, ownerScheduleVersion, err)
	}
	if _, err = service.SavePublicationSchedule(ctx, master, publicationB, landlord, corr, scheduleVersion, "revision_administrativa", weekly); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("stale admin schedule write after owner update returned %v", err)
	}
	const bookingID = "82000000-0000-4000-8000-000000000001"
	const quoteID = "82000000-0000-4000-8000-000000000002"
	const occupancyID = "82000000-0000-4000-8000-000000000003"
	bookingTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer bookingTx.Rollback(ctx)
	if _, err = bookingTx.Exec(ctx, `SET CONSTRAINTS ALL DEFERRED`); err != nil {
		t.Fatal(err)
	}
	bookingStart := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Minute)
	bookingEnd := bookingStart.Add(time.Hour)
	if _, err = bookingTx.Exec(ctx, `INSERT INTO public.ocupacion(id,espacio_id,reserva_id,intervalo,tipo,activo) VALUES($1,$2,$3,tstzrange($4,$5,'[)'),'reserva',true)`, occupancyID, publicationB, bookingID, bookingStart, bookingEnd); err != nil {
		t.Fatal(err)
	}
	if _, err = bookingTx.Exec(ctx, `INSERT INTO public.cotizacion_reserva_ensayo(id,espacio_id,anfitrion_id,arrendatario_id,tarifa_version,modalidad,precio_unitario_clp,moneda,unidades,subtotal_clp,inicio,termino,zona_horaria,condiciones_snapshot,categoria_codigo,perfil_version,perfil_valores_snapshot,creada_en,vence_en) VALUES($1,$2,$3,$4,1,'hora',12000,'CLP',1,12000,$5,$6,'America/Santiago','Condiciones sintéticas de ensayo','oficina',1,'{}'::jsonb,now(),now()+interval '15 minutes')`, quoteID, publicationB, landlord, renter, bookingStart, bookingEnd); err != nil {
		t.Fatal(err)
	}
	if _, err = bookingTx.Exec(ctx, `INSERT INTO public.reserva_ensayo_local(id,cotizacion_id,espacio_id,anfitrion_id,arrendatario_id,clave_idempotencia,huella_solicitud,ocupacion_id,estado,precio_unitario_clp,unidades,subtotal_clp,modalidad,moneda,inicio,termino,zona_horaria,condiciones_snapshot,pago_vence_en,creada_en,actualizada_en) VALUES($1,$2,$3,$4,$5,'gadm02-resource-booking',decode(repeat('00',32),'hex'),$6,'pagada',12000,1,12000,'hora','CLP',$7,$8,'America/Santiago','Condiciones sintéticas de ensayo',now()+interval '15 minutes',now(),now())`, bookingID, quoteID, publicationB, landlord, renter, occupancyID, bookingStart, bookingEnd); err != nil {
		t.Fatal(err)
	}
	if err = bookingTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	resourcePage, err := service.ListAccountResources(ctx, master, renter, corr, "reservations", "", 1)
	if err != nil || resourcePage.MatchingTotal != 1 || len(resourcePage.Items) != 1 || resourcePage.Items[0].ID != bookingID || resourcePage.Items[0].Participant != "arrendatario" {
		t.Fatalf("account reservation resources: %+v err=%v", resourcePage, err)
	}
	resourceDetail, err := service.GetAccountResource(ctx, master, renter, bookingID, corr, "reservations")
	if err != nil || resourceDetail["state"] != "pagada" {
		t.Fatalf("account reservation detail: %+v err=%v", resourceDetail, err)
	}
	const photoID = "81000000-0000-4000-8000-000000000001"
	const fileID = "81000000-0000-4000-8000-000000000002"
	const photoTwoID = "81000000-0000-4000-8000-000000000003"
	const fileTwoID = "81000000-0000-4000-8000-000000000004"
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia) VALUES($1,$2,$3,$4,'synthetic-png-v1','image/png',repeat('a',64),100,'activa',now(),'gadm02-gallery')`, photoID, publicationB, landlord, fileID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.espacio_galeria_sintetica_local(id,espacio_id,propietario_id,archivo_id,fixture_code,mime_type,sha256,size_bytes,estado,creada_en,clave_idempotencia) VALUES($1,$2,$3,$4,'synthetic-png-v1','image/png',repeat('b',64),100,'activa',now(),'gadm02-gallery-two')`, photoTwoID, publicationB, landlord, fileTwoID); err != nil {
		t.Fatal(err)
	}
	service.SetPublicationGallery(gallerypg.New(pool))
	photos, err := service.PublicationGallery(ctx, master, publicationB, landlord, corr)
	if err != nil || len(photos) != 2 || photos[0].ID != photoID || photos[0].State != "activa" {
		t.Fatalf("global gallery read: %+v err=%v", photos, err)
	}
	var galleryVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&galleryVersion); err != nil {
		t.Fatal(err)
	}
	ownerGallery := gallerypg.New(pool)
	if _, err = ownerGallery.SetPublicInCatalog(ctx, landlord, publicationB, photoID, true); err != nil {
		t.Fatalf("owner gallery visibility update: %v", err)
	}
	if _, err = service.SetPublicationPhotoVisibility(ctx, master, publicationB, landlord, photoID, corr, false, galleryVersion, "revision_administrativa"); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("stale admin gallery write after owner visibility returned %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&galleryVersion); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetPublicationPhotoVisibility(ctx, master, publicationB, landlord, photoID, corr, true, galleryVersion, "revision_administrativa"); err != nil {
		t.Fatalf("global photo visibility: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&galleryVersion); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SelectPublicationCover(ctx, master, publicationB, landlord, photoID, corr, galleryVersion, "revision_administrativa"); err != nil {
		t.Fatalf("global cover select: %v", err)
	}
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&galleryVersion); err != nil {
		t.Fatal(err)
	}
	removedPhoto, err := service.RemovePublicationPhoto(ctx, master, publicationB, landlord, photoID, corr, galleryVersion, "moderacion_contenido")
	if err != nil || !removedPhoto.Removed {
		t.Fatalf("global photo remove: %+v err=%v", removedPhoto, err)
	}
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&galleryVersion); err != nil {
		t.Fatal(err)
	}
	if _, err = ownerGallery.Remove(ctx, landlord, publicationB, photoTwoID); err != nil {
		t.Fatalf("owner non-cover photo removal: %v", err)
	}
	if _, err = service.SetPublicationPhotoVisibility(ctx, master, publicationB, landlord, photoID, corr, false, galleryVersion, "revision_administrativa"); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("stale admin gallery write after owner remove returned %v", err)
	}
	retirementService, err := spacesdomain.NewService(spacespg.New(pool, credentials.Generator{}))
	if err != nil {
		t.Fatal(err)
	}
	service.SetPublicationRetirer(retirementService)
	var retirementVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationA).Scan(&retirementVersion); err != nil {
		t.Fatal(err)
	}
	retired, err := service.RetirePublication(ctx, master, publicationA, landlord, corr, retirementVersion, "revision_administrativa")
	if err != nil || retired.State != "eliminada" || retired.SpaceID != publicationA {
		t.Fatalf("global retirement: %+v err=%v", retired, err)
	}
	retriedRetirement, err := service.RetirePublication(ctx, master, publicationA, landlord, corr, retirementVersion, "revision_administrativa")
	if err != nil || !retriedRetirement.Reused {
		t.Fatalf("global retirement retry: %+v err=%v", retriedRetirement, err)
	}
	var retirementActor string
	if err = pool.QueryRow(ctx, `SELECT actor_id::text FROM public.espacio_eliminacion_historial_local WHERE espacio_id=$1`, publicationA).Scan(&retirementActor); err != nil || retirementActor != master {
		t.Fatalf("retirement actor=%s err=%v", retirementActor, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE public.espacio SET estado='activa' WHERE id=$1`, publicationB); err != nil {
		t.Fatal(err)
	}
	var blockedRetirementVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.espacio WHERE id=$1`, publicationB).Scan(&blockedRetirementVersion); err != nil {
		t.Fatal(err)
	}
	if _, err = service.RetirePublication(ctx, master, publicationB, landlord, corr, blockedRetirementVersion, "revision_administrativa"); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("retirement ignored active booking fixture: %v", err)
	}
	auth, err := identity.NewAuthenticationService(identitypg.NewIdentityRepository(pool), password.Hasher{}, devauth.Mailer{}, devauth.NewIPLimiter(50), credentials.Generator{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	globalToken := issueSyntheticSession(t, ctx, pool, master, "synthetic-global-bearer")
	legacyToken := issueSyntheticSession(t, ctx, pool, legacy, "synthetic-legacy-bearer")
	if _, err = auth.AuthorizeCapability(ctx, globalToken, identity.CapabilityAccountsList, identity.UserOperation); err != nil {
		t.Fatalf("global capability denied: %v", err)
	}
	managementPrincipal, err := auth.AuthorizeCapability(ctx, globalToken, identity.CapabilityAccountsUpdate, identity.UserOperation)
	if err != nil || time.Since(managementPrincipal.AuthenticatedAt) > time.Minute {
		t.Fatalf("recent authentication timestamp missing/stale: %+v err=%v", managementPrincipal, err)
	}
	if _, err = auth.AuthorizeCapability(ctx, legacyToken, identity.CapabilityLegacyAdministration, identity.UserOperation); err != nil {
		t.Fatalf("legacy capability changed: %v", err)
	}
	if _, err = auth.AuthorizeCapability(ctx, legacyToken, identity.CapabilityAccountsList, identity.UserOperation); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("legacy admin escalated to directory: %v", err)
	}
	if _, err = pool.Exec(ctx, `DELETE FROM public.administrador_global WHERE usuario_id=$1`, master); err != nil {
		t.Fatal(err)
	}
	if _, err = auth.AuthorizeCapability(ctx, globalToken, identity.CapabilityAccountsList, identity.UserOperation); !errors.Is(err, identity.ErrForbidden) {
		t.Fatalf("old bearer kept removed capability: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'recuperacion_local')`, master); err != nil {
		t.Fatal(err)
	}
	blocker, _ := adminlocal.New(pool, time.Now)
	if _, err = blocker.BlockAccount(ctx, master, renter, "revision_administrativa", corr); err != nil {
		t.Fatal(err)
	}
	restrictedToken := issueSyntheticSession(t, ctx, pool, renter, "synthetic-restricted-bearer")
	if _, err = auth.AuthorizeCapability(ctx, restrictedToken, identity.CapabilityAccountsList, identity.UserOperation); !errors.Is(err, identity.ErrRestrictedMode) {
		t.Fatalf("restricted account gained directory access: %v", err)
	}
	page, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 2})
	if err != nil {
		t.Fatal(err)
	}
	if page.MatchingTotal != 6 || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("unexpected first page: %+v", page)
	}
	second, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 2, Cursor: page.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Items) != 2 || second.Items[0].AccountID == page.Items[1].AccountID {
		t.Fatalf("keyset cursor repeated/omitted tie page: %+v", second)
	}
	filtered, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, Role: identity.RoleAdministrator})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.MatchingTotal != 1 || filtered.Items[0].AccountID != legacy {
		t.Fatalf("role filter leaked global bootstrap owner or missed legacy: %+v", filtered)
	}
	byEmail, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, EmailPrefix: "LEGACY@EXAMPLE"})
	if err != nil || byEmail.MatchingTotal != 1 || byEmail.Items[0].AccountID != legacy {
		t.Fatalf("email prefix filter: %+v %v", byEmail, err)
	}
	byName, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, NamePrefix: "second"})
	if err != nil || byName.MatchingTotal != 1 || byName.Items[0].AccountID != other {
		t.Fatalf("name prefix filter: %+v %v", byName, err)
	}
	blockedFilter, err := service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, AdministrativeBlocked: ptr(true)})
	if err != nil || blockedFilter.MatchingTotal != 1 || blockedFilter.Items[0].AccountID != renter {
		t.Fatalf("administrative blocked filter: %+v %v", blockedFilter, err)
	}
	lookup, err := service.LookupAccount(ctx, master, legacy, corr)
	if err != nil || lookup.AccountID != legacy {
		t.Fatalf("minimal detail lookup: %+v %v", lookup, err)
	}
	if _, err = service.LookupAccount(ctx, master, "00000000-0000-4000-8000-000000000099", corr); !errors.Is(err, adminglobal.ErrNotFound) {
		t.Fatalf("missing detail returned %v", err)
	}
	var profileVersion time.Time
	if err = pool.QueryRow(ctx, `SELECT actualizado_en FROM public.perfil_usuario WHERE usuario_id=$1`, legacy).Scan(&profileVersion); err != nil {
		t.Fatal(err)
	}
	newPhone := "987654321"
	profile, err := service.UpdateAccountProfile(ctx, master, legacy, corr, adminglobal.AccountProfileInput{DisplayName: "Legacy Updated", Phone: &newPhone, ExpectedAt: profileVersion, Reason: "correccion_perfil"})
	if err != nil || profile.Name != "Legacy Updated" || profile.Email != "legacy@example.invalid" {
		t.Fatalf("allowlisted profile update: %+v err=%v", profile, err)
	}
	if _, err = service.UpdateAccountProfile(ctx, master, legacy, corr, adminglobal.AccountProfileInput{DisplayName: "Stale", ExpectedAt: profileVersion, Reason: "correccion_perfil"}); !errors.Is(err, adminglobal.ErrConflict) {
		t.Fatalf("stale profile version returned %v", err)
	}
	if _, err = service.SetAccountRole(ctx, master, legacy, "arrendador", "revision_administrativa", corr, true); err != nil {
		t.Fatalf("grant ordinary role: %v", err)
	}
	if _, err = service.SetAccountRole(ctx, master, legacy, "arrendador", "revision_administrativa", corr, true); err != nil {
		t.Fatalf("idempotent role grant: %v", err)
	}
	if _, err = service.SetAccountRole(ctx, master, legacy, "administrador_global", "revision_administrativa", corr, true); !errors.Is(err, adminglobal.ErrInvalid) {
		t.Fatalf("runtime global role grant accepted: %v", err)
	}
	if _, err = service.SetAccountBlocked(ctx, master, legacy, "revision_administrativa", corr, true); err != nil {
		t.Fatalf("global account block: %v", err)
	}
	if _, err = auth.AuthorizeCapability(ctx, legacyToken, identity.CapabilityLegacyAdministration, identity.UserOperation); !errors.Is(err, identity.ErrUnauthorized) {
		t.Fatalf("blocked account bearer remained active: %v", err)
	}
	if _, err = service.SetAccountBlocked(ctx, master, legacy, "riesgo_resuelto", corr, false); err != nil {
		t.Fatalf("global account unblock: %v", err)
	}
	if _, err = service.RevokeAccountSessions(ctx, master, landlord, "gestion_operativa", corr); err != nil {
		t.Fatalf("revoke sessions: %v", err)
	}
	if _, err = service.ListAccounts(ctx, legacy, corr, adminglobal.AccountFilter{PageSize: 10}); !errors.Is(err, adminglobal.ErrForbidden) {
		t.Fatalf("legacy admin accessed global directory: %v", err)
	}
	if _, err = service.ListAccounts(ctx, renter, corr, adminglobal.AccountFilter{PageSize: 10}); !errors.Is(err, adminglobal.ErrForbidden) {
		t.Fatalf("renter accessed global directory: %v", err)
	}
	if _, err = service.ListAccounts(ctx, master, corr, adminglobal.AccountFilter{PageSize: 10, Role: identity.Role("administrador' OR true --")}); !errors.Is(err, adminglobal.ErrInvalid) {
		t.Fatalf("role filter injection not rejected: %v", err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO public.administrador_global(usuario_id,concedido_en,motivo_codigo) VALUES($1,now(),'concesion_administrativa'),($2,now(),'concesion_administrativa') ON CONFLICT DO NOTHING`, owner, other); err != nil {
		t.Fatal(err)
	}
	var globalWG sync.WaitGroup
	globalBlockResults := make(chan error, 2)
	for _, pair := range [][2]string{{owner, other}, {other, owner}} {
		globalWG.Add(1)
		go func(actor, target string) {
			defer globalWG.Done()
			_, e := service.SetAccountBlocked(ctx, actor, target, "revision_administrativa", corr, true)
			globalBlockResults <- e
		}(pair[0], pair[1])
	}
	globalWG.Wait()
	close(globalBlockResults)
	globalSuccess, globalRejected := 0, 0
	for e := range globalBlockResults {
		if e == nil {
			globalSuccess++
		} else if errors.Is(e, adminglobal.ErrForbidden) || errors.Is(e, adminglobal.ErrConflict) {
			globalRejected++
		} else {
			t.Errorf("unexpected concurrent global block: %v", e)
		}
	}
	if globalSuccess != 1 || globalRejected != 1 {
		t.Fatalf("global-block last-master race success=%d rejected=%d", globalSuccess, globalRejected)
	}
	var survivor, blockedMaster string
	if err = pool.QueryRow(ctx, `SELECT u.id::text FROM public.administrador_global g JOIN public.usuario u ON u.id=g.usuario_id WHERE NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id) LIMIT 1`).Scan(&survivor); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT cuenta_id::text FROM public.bloqueo_cuenta_administrativo_local WHERE cuenta_id IN($1,$2)`, owner, other).Scan(&blockedMaster); err != nil {
		t.Fatal(err)
	}
	if _, err = service.SetAccountBlocked(ctx, survivor, blockedMaster, "riesgo_resuelto", corr, false); err != nil {
		t.Fatalf("restore second master after concurrency test: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, pair := range [][2]string{{owner, owner}, {other, other}} {
		wg.Add(1)
		go func(actor, target string) {
			defer wg.Done()
			_, e := blocker.BlockAccount(ctx, actor, target, "revision_administrativa", corr)
			errs <- e
		}(pair[0], pair[1])
	}
	wg.Wait()
	close(errs)
	success, conflict := 0, 0
	for e := range errs {
		if e == nil {
			success++
		} else if errors.Is(e, adminlocal.ErrConflict) {
			conflict++
		} else {
			t.Errorf("unexpected concurrent block result: %v", e)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("last-master race results success=%d conflict=%d", success, conflict)
	}
	var active int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM public.administrador_global g JOIN public.usuario u ON u.id=g.usuario_id WHERE u.estado='activo' AND NOT EXISTS(SELECT 1 FROM public.bloqueo_cuenta_administrativo_local b WHERE b.cuenta_id=u.id)`).Scan(&active); err != nil || active != 1 {
		t.Fatalf("global admins after concurrent blocks=%d err=%v", active, err)
	}
}

func repositoryRoot(source string) string {
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func makeLocalCLICredentials(t *testing.T, databaseURL, databaseName string) string {
	t.Helper()
	root := repositoryRootFromTest(t)
	localTesting := filepath.Join(root, ".local", "testing")
	if err := os.MkdirAll(localTesting, 0o700); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(localTesting, "gadm01-cli-")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(directory, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	path := filepath.Join(directory, "credentials.env")
	data := []byte("DATABASE_URL=" + databaseURL + "\nEXPECTED_DATABASE=" + databaseName + "\n")
	if err = os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func repositoryRootFromTest(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test source directory")
	}
	return repositoryRoot(source)
}

func runBootstrapCLI(ctx context.Context, root, credentialFile, command, accountID string) error {
	process := exec.CommandContext(ctx, "go", "run", "./cmd/gadm-admin", command, "--account-id", accountID, "--credentials", credentialFile)
	process.Dir = root
	process.Env = make([]string, 0, len(os.Environ())+1)
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "GADM_BOOTSTRAP_ENV=") {
			process.Env = append(process.Env, item)
		}
	}
	process.Env = append(process.Env, "GADM_BOOTSTRAP_ENV=local-test")
	output, err := process.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func ptr[T any](value T) *T { return &value }

func issueSyntheticSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, accountID, token string) identity.Secret {
	t.Helper()
	id, err := (credentials.Generator{}).ID()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	hash := identity.CredentialHash(identity.Secret(token))
	if _, err = pool.Exec(ctx, `INSERT INTO public.sesion(id,usuario_id,token_hash,creada_en,ultima_actividad_en,expira_en) VALUES($1,$2,$3,$4,$4,$5)`, id, accountID, hash, now, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	return identity.Secret(token)
}
