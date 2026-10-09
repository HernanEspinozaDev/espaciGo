// Package reputation implements the local synthetic review and moderation slice.
package reputation

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("reputation: invalid request")
	ErrNotFound = errors.New("reputation: not found")
	ErrConflict = errors.New("reputation: operation conflicts with current state")
)

const SafetyNotice = "ENSAYO LOCAL — RESEÑAS SINTÉTICAS"

type ReviewInput struct {
	Rating  int    `json:"rating"`
	Comment string `json:"comment,omitempty"`
}
type Review struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type,omitempty"`
	Rating     int       `json:"rating"`
	Comment    string    `json:"comment,omitempty"`
	State      string    `json:"state,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	Reused     bool      `json:"reused,omitempty"`
}
type SpaceReviews struct {
	Items   []Review `json:"items"`
	Average float64  `json:"average"`
	Count   int      `json:"count"`
}
type ReportInput struct {
	Reason string `json:"reason_code"`
}
type Report struct {
	ID               string     `json:"id"`
	ReviewID         string     `json:"review_id"`
	SpaceID          string     `json:"space_id"`
	Comment          string     `json:"review_comment,omitempty"`
	Reason           string     `json:"reason_code"`
	State            string     `json:"state"`
	CreatedAt        time.Time  `json:"created_at"`
	ResolvedAt       *time.Time `json:"resolved_at,omitempty"`
	ResolutionReason string     `json:"resolution_reason_code,omitempty"`
	Reused           bool       `json:"reused,omitempty"`
}
type RetentionCounts struct {
	ReviewsPurged int `json:"reviews_purged"`
	ReportsPurged int `json:"reports_purged"`
	NoticesPurged int `json:"notices_purged"`
}

type Repository interface {
	CreateReview(context.Context, string, string, string, ReviewInput, string, []byte, time.Time) (Review, error)
	ListSpaceReviews(context.Context, string) (SpaceReviews, error)
	ListReservationReviews(context.Context, string, string) ([]Review, error)
	Report(context.Context, string, string, string, string, string, string, []byte, time.Time) (Report, error)
	ListReports(context.Context) ([]Report, error)
	Moderate(context.Context, string, string, string, string, string, string, string, time.Time) (Report, error)
	MyRenterReputation(context.Context, string) (SpaceReviews, error)
	PurgeExpired(context.Context, time.Time, int) (RetentionCounts, error)
}
type IDGenerator interface{ ID() (string, error) }
type Service struct {
	repo Repository
	ids  IDGenerator
	now  func() time.Time
}

func New(repo Repository, ids IDGenerator, now func() time.Time) (*Service, error) {
	if repo == nil || ids == nil {
		return nil, ErrInvalid
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, ids: ids, now: now}, nil
}
func (s *Service) Create(ctx context.Context, actor, reservation, key string, in ReviewInput) (Review, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	reservation = strings.ToLower(strings.TrimSpace(reservation))
	key = strings.TrimSpace(key)
	in.Comment = strings.TrimSpace(in.Comment)
	if !validUUID(actor) || !validUUID(reservation) || len(key) < 8 || len(key) > 200 || in.Rating < 1 || in.Rating > 5 {
		return Review{}, ErrInvalid
	}
	id, e := s.ids.ID()
	if e != nil {
		return Review{}, ErrInvalid
	}
	sum := fingerprint(in)
	return s.repo.CreateReview(ctx, id, actor, reservation, in, key, sum, s.now().UTC())
}
func (s *Service) ListSpace(ctx context.Context, id string) (SpaceReviews, error) {
	if !validUUID(id) {
		return SpaceReviews{}, ErrNotFound
	}
	return s.repo.ListSpaceReviews(ctx, strings.ToLower(id))
}
func (s *Service) ListReservation(ctx context.Context, actor, id string) ([]Review, error) {
	if !validUUID(actor) || !validUUID(id) {
		return nil, ErrNotFound
	}
	return s.repo.ListReservationReviews(ctx, strings.ToLower(actor), strings.ToLower(id))
}
func (s *Service) ReportReview(ctx context.Context, actor, reservation, review, key, reason string) (Report, error) {
	actor = strings.ToLower(strings.TrimSpace(actor))
	reservation = strings.ToLower(strings.TrimSpace(reservation))
	review = strings.ToLower(strings.TrimSpace(review))
	key = strings.TrimSpace(key)
	reason = strings.TrimSpace(reason)
	if !validUUID(actor) || !validUUID(reservation) || !validUUID(review) || len(key) < 8 || len(key) > 200 || !validReportReason(reason) {
		return Report{}, ErrInvalid
	}
	id, e := s.ids.ID()
	if e != nil {
		return Report{}, ErrInvalid
	}
	return s.repo.Report(ctx, id, actor, reservation, review, reason, key, fingerprint(struct{ Reason string }{reason}), s.now().UTC())
}
func (s *Service) Reports(ctx context.Context) ([]Report, error) { return s.repo.ListReports(ctx) }
func (s *Service) Moderate(ctx context.Context, admin, report, decision, reason, key, correlation string) (Report, error) {
	admin = strings.ToLower(strings.TrimSpace(admin))
	report = strings.ToLower(strings.TrimSpace(report))
	decision = strings.TrimSpace(decision)
	reason = strings.TrimSpace(reason)
	key = strings.TrimSpace(key)
	correlation = strings.TrimSpace(correlation)
	if !validUUID(admin) || !validUUID(report) || (decision != "desestimar" && decision != "ocultar") || !validResolutionReason(reason) || len(key) < 8 || len(key) > 200 || correlation == "" || len(correlation) > 120 {
		return Report{}, ErrInvalid
	}
	id, e := s.ids.ID()
	if e != nil {
		return Report{}, ErrInvalid
	}
	return s.repo.Moderate(ctx, id, admin, report, decision, reason, key, correlation, s.now().UTC())
}
func (s *Service) MyReputation(ctx context.Context, actor string) (SpaceReviews, error) {
	if !validUUID(actor) {
		return SpaceReviews{}, ErrNotFound
	}
	return s.repo.MyRenterReputation(ctx, strings.ToLower(actor))
}
func (s *Service) PurgeExpired(ctx context.Context, limit int) (RetentionCounts, error) {
	if limit < 1 || limit > 500 {
		return RetentionCounts{}, ErrInvalid
	}
	return s.repo.PurgeExpired(ctx, s.now().UTC(), limit)
}

func validReportReason(s string) bool {
	switch s {
	case "insultos_acoso", "datos_personales", "spam", "ajeno_experiencia":
		return true
	}
	return false
}
func validResolutionReason(s string) bool {
	switch s {
	case "sin_infraccion", "contenido_inadecuado", "duplicado", "error_registro":
		return true
	}
	return false
}
func validUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i, c := range s {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
func fingerprint(v any) []byte { raw, _ := json.Marshal(v); sum := sha256.Sum256(raw); return sum[:] }
