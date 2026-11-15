package verification

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestStartOrResumeBuildsDiditCallbackFromPersistedAttemptID(t *testing.T) {
	for _, origin := range []string{"https://127.0.0.1:4219", "https://app.example.invalid"} {
		t.Run(origin, func(t *testing.T) {
			ctx := context.Background()
			now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
			ids := &counterUUIDs{}
			declRepo := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
			keys := newTestKeyring(t)
			declSvc, err := NewDeclarationService(declRepo, keys, ids, func() time.Time { return now }, func(v time.Time) time.Time { return v.Add(24 * time.Hour) }, "callback-test-v1")
			if err != nil {
				t.Fatal(err)
			}
			account := "11111111-1111-4111-8111-111111111111"
			if _, err := declSvc.Save(ctx, account, DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-9", IDCountry: "CHL", DocumentType: "ID"}, true, "callback-test-v1"); err != nil {
				t.Fatal(err)
			}
			repo := newMemProviderAttemptRepo()
			client := &fakeDiditClient{createResp: CreateSessionResponse{SessionID: "session", SessionURL: "https://verify.didit.me/session/session"}}
			claims := NewMemoryRutClaimRegistry()
			svc, err := NewDiditService(repo, declSvc, client, keys, ids, func() time.Time { return now }, DiditServiceConfig{WorkflowID: "workflow", ConsentVersion: "callback-test-v1", CallbackURL: origin + "/verificaciones/retorno", AllowedCallbackOrigin: origin, RutHMACSecret: []byte("0123456789abcdef")}, claims)
			if err != nil {
				t.Fatal(err)
			}
			started, err := svc.StartOrResume(ctx, account, "callback-start", StartDiditIntentInput{ConsentGiven: true, ConsentVersion: "callback-test-v1"})
			if err != nil {
				t.Fatal(err)
			}
			if len(client.created) != 1 {
				t.Fatalf("CreateSession calls=%d", len(client.created))
			}
			callback, err := url.Parse(client.created[0].Callback)
			if err != nil {
				t.Fatal(err)
			}
			if client.created[0].Callback == "" || callback.Scheme+"://"+callback.Host != origin || callback.Path != "/verificaciones/retorno" {
				t.Fatalf("unexpected callback: %q", client.created[0].Callback)
			}
			q := callback.Query()
			if len(q) != 1 || q.Get("attempt_id") != started.ID {
				t.Fatalf("callback query=%v; attempt=%q", q, started.ID)
			}
			if strings.Contains(client.created[0].Callback, "Synthetic") || strings.Contains(client.created[0].Callback, "Person") || strings.Contains(client.created[0].Callback, "11.222.333-9") || strings.Contains(client.created[0].Callback, "1990-01-02") {
				t.Fatalf("PII leaked in callback: %q", client.created[0].Callback)
			}
		})
	}
}

func TestDiditCallbackOriginMustBeConcreteAndReplaceLoopbackFallback(t *testing.T) {
	for _, tc := range []struct {
		name, origin, callbackOrigin string
		wantErr                      bool
	}{
		{"default loopback", "", "https://127.0.0.1:4219", false},
		{"concrete production", "https://app.example.invalid", "https://app.example.invalid", false},
		{"explicit replaces loopback", "https://app.example.invalid", "https://127.0.0.1:4219", true},
		{"wrong port", "https://app.example.invalid", "https://app.example.invalid:444", true},
		{"wildcard", "https://*.example.invalid", "https://*.example.invalid", true},
		{"empty port", "https://example.invalid:", "https://example.invalid:", true},
		{"large port", "https://example.invalid:99999", "https://example.invalid:99999", true},
		{"empty hostname", "https://:443", "https://:443", true},
		{"trailing dot", "https://example.invalid.", "https://example.invalid.", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
			ids := &counterUUIDs{}
			keys := newTestKeyring(t)
			ds, err := NewDeclarationService(&declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}, keys, ids, func() time.Time { return now }, func(v time.Time) time.Time { return v.Add(24 * time.Hour) }, "callback-test-v1")
			if err != nil {
				t.Fatal(err)
			}
			client := &fakeDiditClient{}
			_, err = NewDiditService(newMemProviderAttemptRepo(), ds, client, keys, ids, func() time.Time { return now }, DiditServiceConfig{WorkflowID: "workflow", ConsentVersion: "callback-test-v1", CallbackURL: tc.callbackOrigin + "/verificaciones/retorno", AllowedCallbackOrigin: tc.origin}, NewMemoryRutClaimRegistry())
			if (err != nil) != tc.wantErr {
				t.Fatalf("constructor error=%v; wantErr=%v", err, tc.wantErr)
			}
			if len(client.created) != 0 {
				t.Fatal("constructor unexpectedly called provider")
			}
		})
	}
}

func TestStartOrResumeOmitsCallbackWhenUnconfiguredAndRejectsUnsafeCallback(t *testing.T) {
	for _, tc := range []struct {
		name, callback string
		wantErr        bool
	}{{"unset", "", false}, {"wrong origin", "https://evil.example/verificaciones/retorno", true}, {"userinfo", "https://user@127.0.0.1:4219/verificaciones/retorno", true}, {"query", "https://127.0.0.1:4219/verificaciones/retorno?x=1", true}, {"fragment", "https://127.0.0.1:4219/verificaciones/retorno#x", true}, {"wrong port", "https://127.0.0.1:4220/verificaciones/retorno", true}} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
			ids := &counterUUIDs{}
			dr := &declarationMemoryRepository{rows: make(map[string]DeclarationRevision)}
			keys := newTestKeyring(t)
			ds, err := NewDeclarationService(dr, keys, ids, func() time.Time { return now }, func(v time.Time) time.Time { return v.Add(24 * time.Hour) }, "callback-test-v1")
			if err != nil {
				t.Fatal(err)
			}
			account := "11111111-1111-4111-8111-111111111111"
			if _, err = ds.Save(context.Background(), account, DeclarationPayload{FirstName: "Synthetic", LastName: "Person", DateOfBirth: "1990-01-02", RUT: "11.222.333-9", IDCountry: "CHL", DocumentType: "ID"}, true, "callback-test-v1"); err != nil {
				t.Fatal(err)
			}
			client := &fakeDiditClient{createResp: CreateSessionResponse{SessionID: "s", SessionURL: "https://verify.didit.me/session/s"}}
			svc, err := NewDiditService(newMemProviderAttemptRepo(), ds, client, keys, ids, func() time.Time { return now }, DiditServiceConfig{WorkflowID: "workflow", ConsentVersion: "callback-test-v1", CallbackURL: tc.callback}, NewMemoryRutClaimRegistry())
			if tc.wantErr {
				if err == nil {
					t.Fatal("unsafe callback accepted")
				}
				if len(client.created) != 0 {
					t.Fatal("CreateSession called for unsafe callback")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = svc.StartOrResume(context.Background(), account, "callback-start", StartDiditIntentInput{ConsentGiven: true, ConsentVersion: "callback-test-v1"})
			if err != nil {
				t.Fatal(err)
			}
			if len(client.created) != 1 || client.created[0].Callback != "" {
				t.Fatalf("expected omitted callback, got %+v", client.created)
			}
		})
	}
}
