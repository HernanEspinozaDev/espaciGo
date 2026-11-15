package verification

import (
	"context"
	"errors"
	"strings"
	"time"
)

type LatestProviderAttemptRepository interface {
	GetLatestAttemptForAccount(context.Context, string, string, string, string, time.Time) (ProviderAttempt, error)
}

type DiditStatusInspection struct {
	ID               string    `json:"id"`
	Status           string    `json:"status"`
	ProviderStatus   string    `json:"provider_status"`
	EvaluationStatus string    `json:"evaluation_status"`
	Environment      string    `json:"environment"`
	BadgeResult      string    `json:"badge_result"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	CheckedAt        time.Time `json:"checked_at"`
}

func normalizedProviderStatus(status string) string {
	value := strings.Join(strings.Fields(strings.ToUpper(status)), "_")
	if len(value) > 64 {
		return ""
	}
	for _, c := range value {
		if c != '_' && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return ""
		}
	}
	return value
}

func (s *DiditService) InspectStatus(ctx context.Context, accountID, attemptID string) (DiditStatusInspection, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) || !canonicalDeclarationUUID(attemptID) {
		return DiditStatusInspection{}, ErrInvalid
	}
	now := s.now().UTC().Truncate(time.Microsecond)
	a, err := s.repo.GetAttemptForAccount(ctx, accountID, attemptID, now, s.cfg.AttemptTTL)
	if err != nil {
		return DiditStatusInspection{}, err
	}
	if a.AccountID != accountID || a.ID != attemptID || a.Provider != DiditProviderName || a.Environment != s.cfg.Environment || a.ApplicationID != s.cfg.ApplicationID {
		return DiditStatusInspection{}, ErrNotFound
	}
	base := DiditStatusInspection{ID: a.ID, Status: a.Status, EvaluationStatus: a.Status, Environment: a.Environment, BadgeResult: s.computeBadgeResult(a, false), CreatedAt: a.CreatedAt.UTC(), UpdatedAt: a.UpdatedAt.UTC(), CheckedAt: now}
	if a.Status == "EXPIRED" || a.Status == "FAILED" || a.Status == "REJECTED" {
		return base, nil
	}
	if a.DeclarationID == "" && a.Status == "APPROVED" {
		_, currentErr := s.declarations.GetCurrent(ctx, accountID)
		if currentErr == nil {
			base.EvaluationStatus = "IN_REVIEW"
			base.BadgeResult = "NOT_VERIFIED"
		} else if !errors.Is(currentErr, ErrNotFound) {
			return DiditStatusInspection{}, currentErr
		}
		return base, nil
	}
	decl, err := s.declarations.GetRevision(ctx, accountID, a.DeclarationID)
	if err != nil {
		return DiditStatusInspection{}, err
	}
	current, err := s.declarations.GetCurrent(ctx, accountID)
	if err != nil {
		return DiditStatusInspection{}, err
	}
	superseded := current.ID != a.DeclarationID
	eval := a.Status
	providerStatus := ""
	checked := now
	if a.ProviderSessionID != "" {
		providerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		decision, getErr := s.client.GetSessionDecision(providerCtx, a.ProviderSessionID)
		if getErr != nil {
			if errors.Is(getErr, ErrDiditRateLimited) {
				return DiditStatusInspection{}, ErrDiditRateLimited
			}
			return DiditStatusInspection{}, ErrDiditUnavailable
		}
		if decision.SessionID != a.ProviderSessionID || decision.VendorData != a.CorrelationID || decision.WorkflowID != a.WorkflowID {
			return DiditStatusInspection{}, ErrInvalid
		}
		providerStatus = normalizedProviderStatus(decision.Status)
		if providerStatus == "" {
			return DiditStatusInspection{}, ErrDiditUnavailable
		}
		input := &IdentityVerificationAttempt{ID: a.ID, AccountID: a.AccountID, Status: a.Status, IDCountry: decl.Payload.IDCountry, DocumentType: decl.Payload.DocumentType, DeclaredFirstName: decl.Payload.FirstName, DeclaredLastName: decl.Payload.LastName, DeclaredDateOfBirth: decl.Payload.DateOfBirth, DeclaredRUTNormalized: decl.Payload.RUT, DiditSessionID: a.ProviderSessionID, WorkflowID: a.WorkflowID, Environment: a.Environment, ConsentVersion: decl.ConsentVersion}
		eval, _, _ = EvaluateDecision(input, &decision)
		if providerStatus == "DECLINED" || providerStatus == "REJECTED" {
			eval = "REJECTED"
		}
		checked = s.now().UTC().Truncate(time.Microsecond)
	} else if a.ProviderSessionID == "" {
		eval = "PENDING_USER_INPUT"
	}
	if superseded {
		// The old session remains inspectable, but cannot verify a newer declaration.
		eval = "IN_REVIEW"
	}
	badge := s.computeBadgeResult(a, superseded)
	if badge == "VERIFIED" && eval != "APPROVED" {
		badge = "IN_REVIEW"
	}
	return DiditStatusInspection{ID: a.ID, Status: a.Status, ProviderStatus: providerStatus, EvaluationStatus: eval, Environment: a.Environment, BadgeResult: badge, CreatedAt: a.CreatedAt.UTC(), UpdatedAt: a.UpdatedAt.UTC(), CheckedAt: checked}, nil
}

func (s *DiditService) GetLatestStatus(ctx context.Context, accountID string) (DiditStatusInspection, error) {
	if s == nil || !canonicalDeclarationUUID(accountID) {
		return DiditStatusInspection{}, ErrInvalid
	}
	repo, ok := s.repo.(LatestProviderAttemptRepository)
	if !ok {
		return DiditStatusInspection{}, ErrInvalid
	}
	a, err := repo.GetLatestAttemptForAccount(ctx, accountID, DiditProviderName, s.cfg.Environment, s.cfg.ApplicationID, s.now().UTC().Truncate(time.Microsecond))
	if err != nil {
		return DiditStatusInspection{}, err
	}
	return s.InspectStatus(ctx, accountID, a.ID)
}
