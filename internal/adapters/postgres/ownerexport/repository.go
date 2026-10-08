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
