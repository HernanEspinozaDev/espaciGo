package main

import (
	"errors"
	"net"
	"net/url"
	"path/filepath"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/zohoauth"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	"regexp"
	"strings"
)

const (
	manualKYCAPIAddr         = "127.0.0.1:18090"
	manualKYCCallback        = "https://127.0.0.1:4219/verificaciones/retorno"
	manualKYCOrigin          = "https://127.0.0.1:4219"
	manualKYCDiditURL        = "https://verification.didit.me"
	manualKYCAppID           = "806fe3f4-4c9b-4567-bf6f-ce8c3e688fce"
	manualKYCWorkflowID      = "a4b973d4-4578-44fc-8580-7554e39821ce"
	manualKYCWorkflowVersion = "1"
)

var manualKYCUUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-8][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func validateManualDiditSandboxProfile(getenv func(string) string) error {
	bad := func() error { return errors.New("invalid local Didit Sandbox manual runtime configuration") }
	if getenv("LOCAL_AUTH_PROTOTYPE") != "1" || getenv("APP_PROFILE") != "" {
		return bad()
	}
	if getenv("DATABASE_CONNECTION_MODE") != "direct" || getenv("DATABASE_HOST") != "127.0.0.1" || getenv("DATABASE_PORT") != "5432" || getenv("DATABASE_NAME") != "espacigo_dms_stage_20261020" || getenv("DATABASE_USER") != "espacigo_api_login" || getenv("DATABASE_SET_ROLE") != "espacigo_runtime" {
		return bad()
	}
	if getenv("HTTP_ADDR") != manualKYCAPIAddr || strings.TrimSpace(getenv("PGPASSFILE")) == "" || strings.TrimSpace(getenv("DATABASE_PASSWORD_FILE")) != "" {
		return bad()
	}
	passfile := strings.TrimSpace(getenv("PGPASSFILE"))
	if !filepath.IsAbs(passfile) || filepath.Clean(passfile) != passfile || !strings.HasSuffix(passfile, "/.local/testing/kyc-native/api.pgpass") {
		return bad()
	}
	if getenv("CORS_ALLOWED_ORIGINS") != manualKYCOrigin || getenv("FRONTEND_PUBLIC_ORIGIN") != manualKYCOrigin || getenv("DIDIT_CALLBACK_URL") != manualKYCCallback {
		return bad()
	}
	if strings.TrimSpace(getenv("IDENTITY_DECLARATION_KEYRING_FILE")) == "" || strings.TrimSpace(getenv("IDENTITY_RUT_HMAC_KEYRING_FILE")) == "" && strings.TrimSpace(getenv("IDENTITY_RUT_HMAC_SECRET")) == "" {
		return bad()
	}
	if strings.TrimSpace(getenv("IDENTITY_RUT_HMAC_KEYRING_FILE")) != "" && strings.TrimSpace(getenv("IDENTITY_RUT_HMAC_SECRET")) != "" {
		return bad()
	}
	if getenv("DIDIT_ENVIRONMENT") != "SANDBOX" || getenv("DIDIT_BASE_URL") != manualKYCDiditURL || getenv("DIDIT_APPLICATION_ID") != manualKYCAppID || strings.TrimSpace(getenv("DIDIT_API_KEY")) == "" || strings.TrimSpace(getenv("DIDIT_WEBHOOK_SECRET")) == "" || getenv("DIDIT_REQUIRE_LIVE_FOR_VERIFIED_BADGE") != "true" {
		return bad()
	}
	if getenv("DIDIT_WORKFLOW_ID") != manualKYCWorkflowID || getenv("DIDIT_WORKFLOW_VERSION") != manualKYCWorkflowVersion {
		return bad()
	}
	smtp := strings.TrimSpace(getenv("LOCAL_SMTP_ADDR"))
	host, port, err := net.SplitHostPort(smtp)
	ip := net.ParseIP(host)
	if err != nil || ip == nil || !ip.IsLoopback() || port != "11025" {
		return bad()
	}
	for _, key := range []string{"DATABASE_URL", "PGPASSWORD", "PGSERVICE", "PGSERVICEFILE", "PGOPTIONS", "CLOUD_SQL_INSTANCE_CONNECTION_NAME", "CLOUD_SQL_PRIVATE_IP"} {
		if strings.TrimSpace(getenv(key)) != "" {
			return bad()
		}
	}
	parsed, err := url.Parse(manualKYCDiditURL)
	if err != nil || parsed.Scheme != "https" {
		return bad()
	}
	return nil
}

func manualKYCAuthMailer(_ runtimeProfileName, _ func(string) string) identity.AuthenticationMailer {
	return zohoauth.UnavailableMailer{}
}
