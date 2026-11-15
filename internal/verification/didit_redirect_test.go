package verification

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPDiditClientDoesNotFollowProviderRedirects(t *testing.T) {
	redirectTargetRequests := 0
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirectTargetRequests++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"session_id":"synthetic-session","url":"https://verify.didit.me/session/synthetic-session"}`))
	}))
	defer redirectTarget.Close()

	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirectTarget.URL, http.StatusFound)
	}))
	defer provider.Close()

	client := NewHTTPDiditClient("local-fake-api-key", "", true, time.Second)
	client.BaseURL = provider.URL
	if _, err := client.CreateSession(context.Background(), CreateSessionRequest{}); err == nil {
		t.Fatal("provider redirect unexpectedly produced a session response")
	}
	if redirectTargetRequests != 0 {
		t.Fatalf("provider HTTP client followed %d redirect(s)", redirectTargetRequests)
	}
}
