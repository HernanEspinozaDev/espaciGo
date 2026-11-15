package main

import (
	"errors"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	"github.com/HernanEspinozaDev/espaciGo/internal/verification"
)

const defaultDiditConsentVersion = "didit-consent-v1"

// newTestDiditService exists only for isolated legacy service tests. The
// production API no longer constructs or mounts an external-provider service.
func newTestDiditService(
	getenv func(string) string,
	repo verification.ProviderAttemptRepository,
	declarationService *verification.DeclarationService,
	now func() time.Time,
	customStores ...any,
) (*verification.DiditService, error) {
	callbackURL := strings.TrimSpace(getenv("DIDIT_CALLBACK_URL"))
	frontendOrigin := strings.TrimSpace(getenv("FRONTEND_PUBLIC_ORIGIN"))
	if err := verification.ValidateDiditCallbackConfig(verification.DiditServiceConfig{CallbackURL: callbackURL, AllowedCallbackOrigin: frontendOrigin}); err != nil {
		return nil, errors.New("didit callback URL is invalid")
	}
	if repo == nil || declarationService == nil {
		return nil, nil
	}
	keyring, err := loadIdentityDeclarationKeyring(getenv)
	if err != nil || keyring == nil {
		return nil, err
	}
	apiKey := strings.TrimSpace(getenv("DIDIT_API_KEY"))
	workflowID := strings.TrimSpace(getenv("DIDIT_WORKFLOW_ID"))
	if apiKey == "" || workflowID == "" {
		return nil, nil
	}
	consentVersion, err := identityDeclarationConsentVersion(getenv)
	if err != nil {
		return nil, err
	}
	environment := strings.ToUpper(strings.TrimSpace(getenv("DIDIT_ENVIRONMENT")))
	if environment == "" {
		environment = "SANDBOX"
	}
	sandbox := environment != "LIVE"
	client := verification.NewHTTPDiditClient(apiKey, strings.TrimSpace(getenv("DIDIT_WEBHOOK_SECRET")), sandbox, 15*time.Second)
	if baseURL := strings.TrimSpace(getenv("DIDIT_BASE_URL")); baseURL != "" {
		client.BaseURL = baseURL
	}
	appID := strings.TrimSpace(getenv("DIDIT_APPLICATION_ID"))
	if appID == "" {
		appID = "espacigo-local-app"
	}
	workflowVersion := strings.TrimSpace(getenv("DIDIT_WORKFLOW_VERSION"))
	if workflowVersion == "" {
		workflowVersion = "1"
	}
	var rutClaims verification.RutClaimStore
	var deletionQueue verification.SessionDeletionQueue
	for _, store := range customStores {
		if rc, ok := store.(verification.RutClaimStore); ok && rc != nil {
			rutClaims = rc
		}
		if dq, ok := store.(verification.SessionDeletionQueue); ok && dq != nil {
			deletionQueue = dq
		}
	}
	if rutClaims == nil || deletionQueue == nil {
		return nil, nil
	}
	rutKeys := declarationService.RutKeyring()
	if rutKeys == nil {
		return nil, errors.New("identity RUT HMAC snapshot is unavailable")
	}
	diditConsentVersion := strings.TrimSpace(getenv("DIDIT_CONSENT_VERSION"))
	if diditConsentVersion == "" {
		diditConsentVersion = defaultDiditConsentVersion
	}
	if len(diditConsentVersion) > 20 || strings.ContainsAny(diditConsentVersion, " \t\r\n") {
		return nil, errors.New("didit consent version is invalid")
	}
	return verification.NewDiditService(
		repo,
		declarationService,
		client,
		keyring,
		credentials.Generator{},
		now,
		verification.DiditServiceConfig{
			Environment:                 environment,
			CallbackURL:                 callbackURL,
			AllowedCallbackOrigin:       frontendOrigin,
			ApplicationID:               appID,
			WorkflowID:                  workflowID,
			WorkflowVersion:             workflowVersion,
			ConsentVersion:              diditConsentVersion,
			DeclarationConsentVersion:   consentVersion,
			AttemptTTL:                  verification.DefaultDiditAttemptTTL,
			Retention:                   identityDeclarationRetentionPolicy,
			RequireLiveForVerifiedBadge: strings.EqualFold(strings.TrimSpace(getenv("DIDIT_REQUIRE_LIVE_FOR_VERIFIED_BADGE")), "true"),
			RutHMACKeyring:              rutKeys,
		},
		rutClaims,
		deletionQueue,
	)
}
