package main

import (
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/zohoauth"
)

func manualProfileEnv() map[string]string {
	return map[string]string{"LOCAL_AUTH_PROTOTYPE": "1", "DATABASE_CONNECTION_MODE": "direct", "DATABASE_HOST": "127.0.0.1", "DATABASE_PORT": "5432", "DATABASE_NAME": "espacigo_dms_stage_20261020", "DATABASE_USER": "espacigo_api_login", "DATABASE_SET_ROLE": "espacigo_runtime", "HTTP_ADDR": "127.0.0.1:18090", "PGPASSFILE": "/home/test/.local/testing/kyc-native/api.pgpass", "CORS_ALLOWED_ORIGINS": manualKYCOrigin, "FRONTEND_PUBLIC_ORIGIN": manualKYCOrigin, "IDENTITY_DECLARATION_KEYRING_FILE": "/private/keyring.json", "IDENTITY_RUT_HMAC_SECRET": "synthetic-private-hmac-secret-material", "DIDIT_ENVIRONMENT": "SANDBOX", "DIDIT_BASE_URL": manualKYCDiditURL, "DIDIT_APPLICATION_ID": manualKYCAppID, "DIDIT_API_KEY": "injected-sandbox-key", "DIDIT_WEBHOOK_SECRET": "injected-sandbox-secret", "DIDIT_WORKFLOW_ID": manualKYCWorkflowID, "DIDIT_WORKFLOW_VERSION": manualKYCWorkflowVersion, "DIDIT_REQUIRE_LIVE_FOR_VERIFIED_BADGE": "true", "DIDIT_CALLBACK_URL": manualKYCCallback, "LOCAL_SMTP_ADDR": "127.0.0.1:11025"}
}

func TestManualDiditSandboxProfileRejectsUnsafeOverrides(t *testing.T) {
	base := manualProfileEnv()
	if err := validateManualDiditSandboxProfile(func(k string) string { return base[k] }); err != nil {
		t.Fatalf("valid synthetic configuration rejected: %v", err)
	}
	for name, mutate := range map[string]func(map[string]string){"another valid workflow UUID": func(v map[string]string) { v["DIDIT_WORKFLOW_ID"] = "123e4567-e89b-42d3-a456-426614174000" }, "version 2": func(v map[string]string) { v["DIDIT_WORKFLOW_VERSION"] = "2" }, "signed version": func(v map[string]string) { v["DIDIT_WORKFLOW_VERSION"] = "+1" }, "zero-padded version": func(v map[string]string) { v["DIDIT_WORKFLOW_VERSION"] = "01" }, "missing api key": func(v map[string]string) { delete(v, "DIDIT_API_KEY") }, "wrong environment": func(v map[string]string) { v["DIDIT_ENVIRONMENT"] = "LIVE" }, "wrong API URL": func(v map[string]string) { v["DIDIT_BASE_URL"] = "https://example.com" }, "wrong app": func(v map[string]string) { v["DIDIT_APPLICATION_ID"] = "other" }, "bad workflow UUID": func(v map[string]string) { v["DIDIT_WORKFLOW_ID"] = "workflow" }, "bad workflow version": func(v map[string]string) { v["DIDIT_WORKFLOW_VERSION"] = "0" }, "sandbox badge enabled": func(v map[string]string) { v["DIDIT_REQUIRE_LIVE_FOR_VERIFIED_BADGE"] = "false" }, "wrong callback": func(v map[string]string) { v["DIDIT_CALLBACK_URL"] = "https://localhost/return" }, "wrong port": func(v map[string]string) { v["HTTP_ADDR"] = "127.0.0.1:8080" }, "wrong SMTP": func(v map[string]string) { v["LOCAL_SMTP_ADDR"] = "smtp.example:11025" }, "ambiguous HMAC": func(v map[string]string) { v["IDENTITY_RUT_HMAC_KEYRING_FILE"] = "/private/hmac.json" }} {
		t.Run(name, func(t *testing.T) {
			v := map[string]string{}
			for k, x := range base {
				v[k] = x
			}
			mutate(v)
			if validateManualDiditSandboxProfile(func(k string) string { return v[k] }) == nil {
				t.Fatal("unsafe configuration accepted")
			}
		})
	}
}

func TestRetiredManualDiditProfileCannotEnableLocalMailer(t *testing.T) {
	getenv := func(string) string { return "127.0.0.1:11025" }
	if _, ok := manualKYCAuthMailer(runtimeProfileName("local-kyc-didit-sandbox-manual"), getenv).(zohoauth.UnavailableMailer); !ok {
		t.Fatal("retired manual sandbox profile enabled a local mailer")
	}
	if _, ok := manualKYCAuthMailer(runtimeProfileKYCWorkerFree, getenv).(zohoauth.UnavailableMailer); !ok {
		t.Fatal("legacy fake profile mailer changed")
	}
	if _, ok := manualKYCAuthMailer(runtimeProfileStandard, getenv).(zohoauth.UnavailableMailer); !ok {
		t.Fatal("standard startup mailer changed")
	}
}
