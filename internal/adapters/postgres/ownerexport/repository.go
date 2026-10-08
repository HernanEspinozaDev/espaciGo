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
	files := []privacy.ExportFile{}
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
	exclusions := []string{}
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
