// Package ownerexport composes module-owned, read-only archive projections.
// It delegates selection to each module's repository contract and only handles
// ZIP-level metadata and private evidence file assembly.
package ownerexport

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	bookingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/booking"
	conversationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/conversation"
	disputepg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/dispute"
	pricingpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/pricing"
	spacespg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/spaces"
	verificationpg "github.com/HernanEspinozaDev/espaciGo/internal/adapters/postgres/verification"
	"github.com/HernanEspinozaDev/espaciGo/internal/booking"
	"github.com/HernanEspinozaDev/espaciGo/internal/conversation"
	"github.com/HernanEspinozaDev/espaciGo/internal/dispute"
	"github.com/HernanEspinozaDev/espaciGo/internal/pricing"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"github.com/HernanEspinozaDev/espaciGo/internal/spaces"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
	"github.com/jackc/pgx/v5/pgxpool"
)

type sectionReader interface {
	ExportOwnArchiveSections(context.Context, string) (map[string]json.RawMessage, error)
}

type Repository struct {
	pool         *pgxpool.Pool
	verification *verificationpg.Repository
	files        verification.EvidenceStorage
	spaces       sectionReader
	pricing      sectionReader
	bookings     sectionReader
	conversation sectionReader
	disputes     sectionReader
}

func New(pool *pgxpool.Pool, files verification.EvidenceStorage) *Repository {
	return &Repository{
		pool:         pool,
		verification: verificationpg.New(pool), files: files,
		spaces:  spacespg.New(pool, credentials.Generator{}),
		pricing: pricingpg.New(pool), bookings: bookingpg.New(pool),
		conversation: conversationpg.New(pool), disputes: disputepg.New(pool),
	}
}

type verificationCase struct {
	ID         string     `json:"id"`
	Type       string     `json:"type"`
	State      string     `json:"state"`
	ReasonCode string     `json:"reason_code,omitempty"`
	RetryOf    string     `json:"retry_of,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	ResolvedAt *time.Time `json:"resolved_at,omitempty"`
}
type evidenceRecord struct {
	ID             string    `json:"id"`
	VerificationID string    `json:"verification_id"`
	FixtureCode    string    `json:"fixture_code"`
	MIMEType       string    `json:"mime_type"`
	SizeBytes      int64     `json:"size_bytes"`
	SHA256         string    `json:"sha256"`
	CreatedAt      time.Time `json:"created_at"`
	ArchiveFile    string    `json:"archive_file,omitempty"`
}

func (r *Repository) ExportAdditionalOwnData(ctx context.Context, owner string) (map[string]json.RawMessage, []privacy.ExportFile, []string, error) {
	if r == nil || r.verification == nil || owner == "" {
		return nil, nil, nil, privacy.ErrInvalid
	}
	sections := map[string]json.RawMessage{}
	type photoRecord struct {
		ID      string    `json:"id"`
		State   string    `json:"state"`
		Fixture string    `json:"fixture_code"`
		MIME    string    `json:"mime_type"`
		SHA     string    `json:"sha256"`
		Size    int64     `json:"size_bytes"`
		Created time.Time `json:"created_at"`
		Archive string    `json:"archive_file,omitempty"`
	}
	photoRows, photoErr := r.pool.Query(ctx, `SELECT id::text,estado,fixture_code,mime_type,sha256,size_bytes,creada_en FROM public.foto_perfil_sintetica_local WHERE usuario_id=$1 AND archivo_id IS NOT NULL ORDER BY creada_en,id`, owner)
	if photoErr != nil {
		return nil, nil, nil, photoErr
	}
	photoFiles := []privacy.ExportFile{}
	photoExclusions := []string{}
	photos := []photoRecord{}
	for photoRows.Next() {
		var p photoRecord
		if e := photoRows.Scan(&p.ID, &p.State, &p.Fixture, &p.MIME, &p.SHA, &p.Size, &p.Created); e != nil {
			photoRows.Close()
			return nil, nil, nil, e
		}
		if r.files != nil {
			if content, e := r.files.Get(ctx, p.ID); e == nil {
				p.Archive = "files/profile/photo-" + p.ID + ".png"
				photoFiles = append(photoFiles, privacy.ExportFile{Name: p.Archive, MediaType: "image/png", Description: "Foto sintética propia", Content: content})
			} else {
				photoExclusions = append(photoExclusions, "foto sintética "+p.ID+" sin archivo disponible; omitida del ZIP")
			}
		} else {
			photoExclusions = append(photoExclusions, "storage privado no disponible; foto sintética omitida del ZIP")
		}
		photos = append(photos, p)
	}
	if photoErr = photoRows.Err(); photoErr != nil {
		photoRows.Close()
		return nil, nil, nil, photoErr
	}
	photoRows.Close()
	photoJSON, _ := json.Marshal(map[string]any{"photos": photos})
	sections["synthetic_profile_photo"] = photoJSON
	rows, e := r.pool.Query(ctx, `SELECT id::text,adaptador,referencia_ficticia,estado,creada_en,actualizada_en,revocada_en FROM public.cuenta_cobro_sintetica_local WHERE usuario_id=$1 ORDER BY creada_en,id`, owner)
	if e != nil {
		return nil, nil, nil, e
	}
	payouts := []map[string]any{}
	for rows.Next() {
		var id, adapter, state string
		var ref *string
		var created, updated time.Time
		var revoked *time.Time
		if e = rows.Scan(&id, &adapter, &ref, &state, &created, &updated, &revoked); e != nil {
			rows.Close()
			return nil, nil, nil, e
		}
		payouts = append(payouts, map[string]any{"id": id, "adapter": adapter, "reference": ref, "state": state, "created_at": created, "updated_at": updated, "revoked_at": revoked})
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		return nil, nil, nil, e
	}
	rows.Close()
	historyRows, e := r.pool.Query(ctx, `SELECT h.cuenta_id::text,h.accion,h.ocurrida_en,h.correlacion_id FROM public.cuenta_cobro_sintetica_historial_local h WHERE h.usuario_id=$1 ORDER BY h.ocurrida_en,h.id`, owner)
	if e != nil {
		return nil, nil, nil, e
	}
	hist := []map[string]any{}
	for historyRows.Next() {
		var id, action, corr string
		var at time.Time
		if e = historyRows.Scan(&id, &action, &at, &corr); e != nil {
			historyRows.Close()
			return nil, nil, nil, e
		}
		hist = append(hist, map[string]any{"account_id": id, "action": action, "occurred_at": at, "correlation_id": corr})
	}
	if e = historyRows.Err(); e != nil {
		historyRows.Close()
		return nil, nil, nil, e
	}
	historyRows.Close()
	payoutJSON, _ := json.Marshal(map[string]any{"accounts": payouts, "history": hist})
	sections["synthetic_payout_accounts"] = payoutJSON
	type galleryPhotoRecord struct {
		ID      string    `json:"id"`
		SpaceID string    `json:"space_id"`
		Fixture string    `json:"fixture_code"`
		MIME    string    `json:"mime_type"`
		SHA     string    `json:"sha256"`
		Size    int64     `json:"size_bytes"`
		Created time.Time `json:"created_at"`
		Archive string    `json:"archive_file,omitempty"`
	}
	galleryRows, err := r.pool.Query(ctx, `SELECT id::text,espacio_id::text,fixture_code,mime_type,sha256,size_bytes,creada_en FROM public.espacio_galeria_sintetica_local WHERE propietario_id=$1 AND estado='activa' AND archivo_id IS NOT NULL ORDER BY espacio_id,creada_en,id`, owner)
	if err != nil {
		return nil, nil, nil, err
	}
	galleryRecords := []galleryPhotoRecord{}
	galleryFiles := []privacy.ExportFile{}
	galleryExclusions := []string{}
	for galleryRows.Next() {
		var item galleryPhotoRecord
		if err := galleryRows.Scan(&item.ID, &item.SpaceID, &item.Fixture, &item.MIME, &item.SHA, &item.Size, &item.Created); err != nil {
			galleryRows.Close()
			return nil, nil, nil, err
		}
		if r.files != nil {
			if content, e := r.files.Get(ctx, item.ID); e == nil {
				item.Archive = "files/spaces/" + item.SpaceID + "/gallery/" + item.ID + ".png"
				galleryFiles = append(galleryFiles, privacy.ExportFile{Name: item.Archive, MediaType: item.MIME, Description: "Imagen sintética propia de galería", Content: content})
			} else {
				galleryExclusions = append(galleryExclusions, "imagen de galería sintética "+item.ID+" sin archivo privado disponible; omitida del ZIP")
			}
		} else {
			galleryExclusions = append(galleryExclusions, "storage privado no disponible; imagen de galería sintética omitida del ZIP")
		}
		galleryRecords = append(galleryRecords, item)
	}
	if err := galleryRows.Err(); err != nil {
		galleryRows.Close()
		return nil, nil, nil, err
	}
	galleryRows.Close()
	galleryJSON, err := json.Marshal(map[string]any{"items": galleryRecords})
	if err != nil {
		return nil, nil, nil, err
	}
	sections["synthetic_space_gallery"] = galleryJSON
	// Reputation export is owner-scoped. The authored comment is included only
	// for the author; received reviews export score/state without another
	// participant's free text or identifiers. Reports contain structured codes.
	var reputationJSON []byte
	if err := r.pool.QueryRow(ctx, `SELECT jsonb_build_object(
	  'reviews_authored', COALESCE((SELECT jsonb_agg(jsonb_build_object('rating',x.puntuacion,'comment',x.comentario,'target_type',x.destinatario_tipo,'state',x.estado,'created_at',x.creada_en,'retention_until',x.retirar_en) ORDER BY x.creada_en,x.id) FROM public.resena_ensayo_local x WHERE x.autor_id=$1), '[]'::jsonb),
	  'reviews_received', COALESCE((SELECT jsonb_agg(jsonb_build_object('rating',x.puntuacion,'target_type',x.destinatario_tipo,'state',x.estado,'created_at',x.creada_en,'retention_until',x.retirar_en) ORDER BY x.creada_en,x.id) FROM public.resena_ensayo_local x WHERE x.destinatario_id=$1), '[]'::jsonb),
	  'reviews_for_owned_spaces', COALESCE((SELECT jsonb_agg(jsonb_build_object('rating',x.puntuacion,'state',x.estado,'created_at',x.creada_en,'retention_until',x.retirar_en) ORDER BY x.creada_en,x.id) FROM public.resena_ensayo_local x JOIN public.espacio e ON e.id=x.espacio_id WHERE e.propietario_id=$1 AND x.destinatario_tipo='espacio'), '[]'::jsonb),
	  'reports', COALESCE((SELECT jsonb_agg(jsonb_build_object('reason_code',q.motivo_codigo,'state',q.estado,'created_at',q.creada_en,'resolved_at',q.resolver_en,'resolution_reason_code',q.motivo_resolucion_codigo,'retention_until',q.retirar_en,'history',COALESCE((SELECT jsonb_agg(jsonb_build_object('from',h.estado_anterior,'to',h.estado_nuevo,'reason_code',h.motivo_codigo,'at',h.ocurrida_en) ORDER BY h.secuencia) FROM public.reporte_resena_historial_local h WHERE h.reporte_id=q.id),'[]'::jsonb)) ORDER BY q.creada_en,q.id) FROM public.reporte_resena_ensayo_local q LEFT JOIN public.resena_ensayo_local x ON x.id=q.resena_id WHERE q.anfitrion_id=$1 OR x.autor_id=$1 OR x.destinatario_id=$1), '[]'::jsonb),
	  'notice_delivery', COALESCE((SELECT jsonb_agg(jsonb_build_object('event_type',a.tipo_evento,'state',a.estado,'cycle',a.ciclo,'attempts',a.intentos_total,'created_at',a.creada_en,'delivered_at',a.entregada_en,'terminal_failure_at',a.fallo_terminal_en,'cancelled_at',a.cancelada_en,'retention_until',a.retirar_en,'cycles',COALESCE((SELECT jsonb_agg(jsonb_build_object('cycle',c.ciclo,'state',c.estado,'started_at',c.iniciada_en,'finished_at',c.finalizada_en,'attempts',c.intentos,'result_code',c.codigo_resultado) ORDER BY c.ciclo) FROM public.aviso_local_ciclo c WHERE c.aviso_id=a.id),'[]'::jsonb)) ORDER BY a.creada_en,a.id) FROM public.aviso_local a WHERE a.destinatario_id=$1), '[]'::jsonb)
	)`, owner).Scan(&reputationJSON); err != nil {
		return nil, nil, nil, err
	}
	sections["synthetic_reputation_and_notices"] = json.RawMessage(reputationJSON)
	for _, reader := range []sectionReader{r.spaces, r.pricing, r.bookings, r.conversation, r.disputes} {
		part, err := reader.ExportOwnArchiveSections(ctx, owner)
		if err != nil {
			return nil, nil, nil, err
		}
		for key, raw := range part {
			if _, exists := sections[key]; exists {
				return nil, nil, nil, fmt.Errorf("duplicate owner-export section %s", key)
			}
			sections[key] = raw
		}
	}
	cases, err := r.verification.ListOwn(ctx, owner)
	if err != nil {
		return nil, nil, nil, err
	}
	caseItems := make([]verificationCase, 0, len(cases))
	for _, item := range cases {
		caseItems = append(caseItems, verificationCase{ID: item.ID, Type: item.Type, State: item.State, ReasonCode: item.ReasonCode, RetryOf: item.RetryOf, CreatedAt: item.CreatedAt, ResolvedAt: item.ResolvedAt})
	}
	caseJSON, err := json.Marshal(caseItems)
	if err != nil {
		return nil, nil, nil, err
	}
	sections["verifications"] = caseJSON
	eligibility, err := r.verification.ListEligibility(ctx, owner)
	if err != nil {
		return nil, nil, nil, err
	}
	eligibilityJSON, err := json.Marshal(eligibility)
	if err != nil {
		return nil, nil, nil, err
	}
	sections["verification_eligibility"] = eligibilityJSON
	history := make(map[string][]verification.HistoryEntry, len(cases))
	for _, item := range cases {
		entries, err := r.verification.ListHistory(ctx, owner, item.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		history[item.ID] = entries
	}
	historyJSON, err := json.Marshal(history)
	if err != nil {
		return nil, nil, nil, err
	}
	sections["verification_history"] = historyJSON
	files := append([]privacy.ExportFile{}, photoFiles...)
	files = append(files, galleryFiles...)
	photoExclusions = append(photoExclusions, galleryExclusions...)
	exclusionCounts := 0
	evidenceItems := []evidenceRecord{}
	for _, item := range cases {
		stored, err := r.verification.ListOwnEvidence(ctx, owner, item.ID)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, evidence := range stored {
			entry := evidenceRecord{ID: evidence.ID, VerificationID: evidence.VerificationID, FixtureCode: evidence.FixtureCode, MIMEType: evidence.MIMEType, SizeBytes: evidence.SizeBytes, SHA256: evidence.SHA256, CreatedAt: evidence.CreatedAt}
			if r.files == nil {
				exclusionCounts++
				evidenceItems = append(evidenceItems, entry)
				continue
			}
			content, readErr := r.files.Get(ctx, evidence.ID)
			if readErr != nil {
				exclusionCounts++
				evidenceItems = append(evidenceItems, entry)
				continue
			}
			entry.ArchiveFile = "files/evidence/" + evidence.ID + ".png"
			evidenceItems = append(evidenceItems, entry)
			files = append(files, privacy.ExportFile{Name: entry.ArchiveFile, MediaType: evidence.MIMEType, Description: "Evidencia sintética propia de verificación", Content: content})
		}
	}
	evidenceJSON, err := json.Marshal(evidenceItems)
	if err != nil {
		return nil, nil, nil, err
	}
	sections["synthetic_evidence"] = evidenceJSON
	exclusions := append([]string{}, photoExclusions...)
	if exclusionCounts > 0 {
		exclusions = append(exclusions, fmt.Sprintf("%d evidencia(s) propia(s) referenciada(s) sin archivo disponible; omitidas del ZIP", exclusionCounts))
	}
	return sections, files, exclusions, nil
}

var _ privacy.CompleteExportRepository = (*Repository)(nil)
var _ spaces.ArchiveSectionRepository = (*spacespg.Repository)(nil)
var _ pricing.ArchiveSectionRepository = (*pricingpg.Repository)(nil)
var _ booking.ArchiveSectionRepository = (*bookingpg.Repository)(nil)
var _ conversation.ArchiveSectionRepository = (*conversationpg.Repository)(nil)
var _ dispute.ArchiveSectionRepository = (*disputepg.Repository)(nil)
