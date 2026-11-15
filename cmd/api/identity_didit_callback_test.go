package main

import "testing"

func TestLegacyDiditTestFactoryValidatesCallbackBeforeConstruction(t *testing.T) {
	for _, tc := range []struct {
		name, callback string
		wantErr        bool
	}{{"empty", "", false}, {"allowed", "https://127.0.0.1:4219/verificaciones/retorno", false}, {"other origin", "https://evil.example/verificaciones/retorno", true}, {"userinfo", "https://user@127.0.0.1:4219/verificaciones/retorno", true}, {"query", "https://127.0.0.1:4219/verificaciones/retorno?x=1", true}, {"fragment", "https://127.0.0.1:4219/verificaciones/retorno#x", true}, {"path", "https://127.0.0.1:4219/other", true}, {"port", "https://127.0.0.1:4220/verificaciones/retorno", true}} {
		t.Run(tc.name, func(t *testing.T) {
			values := map[string]string{"DIDIT_CALLBACK_URL": tc.callback, "IDENTITY_DECLARATION_KEYRING_FILE": "unused", "DIDIT_API_KEY": "synthetic", "DIDIT_WORKFLOW_ID": "workflow"}
			_, err := newTestDiditService(env(values), nil, nil, nil)
			if tc.wantErr {
				if err == nil {
					t.Fatal("unsafe callback passed runtime validation")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLegacyDiditTestFactoryRejectsMalformedCallbackOrigins(t *testing.T) {
	for _, origin := range []string{"https://*.example.invalid", "https://example.invalid:", "https://example.invalid:99999", "https://example.invalid.", "https://:443"} {
		t.Run(origin, func(t *testing.T) {
			values := map[string]string{"FRONTEND_PUBLIC_ORIGIN": origin, "DIDIT_CALLBACK_URL": origin + "/verificaciones/retorno"}
			if _, err := newTestDiditService(env(values), nil, nil, nil); err == nil {
				t.Fatal("malformed callback origin accepted before construction")
			}
		})
	}
}

func TestLegacyDiditTestFactoryAcceptsConfiguredCallbackOrigin(t *testing.T) {
	values := map[string]string{"DIDIT_CALLBACK_URL": "https://app.example.invalid/verificaciones/retorno", "FRONTEND_PUBLIC_ORIGIN": "https://app.example.invalid", "IDENTITY_DECLARATION_KEYRING_FILE": "unused", "DIDIT_API_KEY": "synthetic", "DIDIT_WORKFLOW_ID": "workflow"}
	if _, err := newTestDiditService(env(values), nil, nil, nil); err != nil {
		t.Fatalf("configured callback rejected: %v", err)
	}
}

func TestLegacyDiditTestFactoryRejectsCallbackOriginMismatch(t *testing.T) {
	for _, callback := range []string{"https://evil.example.invalid/verificaciones/retorno", "https://app.example.invalid/other", "https://app.example.invalid:444/verificaciones/retorno", "https://app.example.invalid/verificaciones/retorno?x=1", "https://app.example.invalid/verificaciones/%72etorno", "https://app.example.invalid/verificaciones/retorno?"} {
		t.Run(callback, func(t *testing.T) {
			values := map[string]string{"DIDIT_CALLBACK_URL": callback, "FRONTEND_PUBLIC_ORIGIN": "https://app.example.invalid", "IDENTITY_DECLARATION_KEYRING_FILE": "unused", "DIDIT_API_KEY": "synthetic", "DIDIT_WORKFLOW_ID": "workflow"}
			if _, err := newTestDiditService(env(values), nil, nil, nil); err == nil {
				t.Fatal("mismatched callback accepted")
			}
		})
	}
}
