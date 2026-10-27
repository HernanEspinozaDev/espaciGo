package postgres_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
)

// This fixture is compiled only by go test and never mounted by cmd/api.
type browserTestStatusTracker struct {
	mu     sync.Mutex
	counts map[string]map[int]int
}

func (s *browserTestStatusTracker) inc(path string, code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts[path] == nil {
		s.counts[path] = make(map[int]int)
	}
	s.counts[path][code]++
}

func (s *browserTestStatusTracker) get(path string, code int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.counts[path][code]
}

type browserTestResponseWriter struct {
	http.ResponseWriter
	tracker     *browserTestStatusTracker
	path        string
	wroteHeader bool
}

func (rw *browserTestResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}
	rw.wroteHeader = true
	rw.tracker.inc(rw.path, code)
	if code == http.StatusNoContent {
		// Reproduce transport metadata on an otherwise successful empty reply.
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *browserTestResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func TestVerificationNativeBrowserEndToEnd(t *testing.T) {
	if os.Getenv("AUTH_VERIFICATION_BROWSER") != "1" {
		t.Skip("set AUTH_VERIFICATION_BROWSER=1 to run the native browser integration")
	}
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate browser fixture")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	if _, err := os.Stat(filepath.Join(repoRoot, "frontend", ".next", "standalone", "server.js")); err != nil {
		t.Fatal("requested native browser integration requires a current standalone build")
	}

	h, clock := newAuthHarnessWithAtomicClock(t)
	clock.Store(time.Now().UTC().UnixNano())
	tracker := &browserTestStatusTracker{counts: make(map[string]map[int]int)}
	identityHandler := identityhttp.NewHandler(h.service, h.repo, nil)
	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identityHandler.ServeHTTP(&browserTestResponseWriter{ResponseWriter: w, tracker: tracker, path: r.URL.Path}, r)
	}))
	// Owned loopback test-driver capture only, with no external mail delivery.
	mux.HandleFunc("/__test/verification-mail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.mail.mu.Lock()
		count := len(h.mail.messages)
		var last identity.VerificationDelivery
		if count > 0 {
			last = h.mail.messages[count-1]
		}
		h.mail.mu.Unlock()
		if count == 0 {
			http.Error(w, "no test delivery available", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"token_id": last.TokenID, "token": string(last.Token)})
	})
	mux.HandleFunc("/__test/recovery-mail", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		h.mail.mu.Lock()
		count := len(h.mail.recoveries)
		var last identity.RecoveryDelivery
		if count > 0 {
			last = h.mail.recoveries[count-1]
		}
		h.mail.mu.Unlock()
		if count == 0 {
			http.Error(w, "no test recovery delivery available", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"token_id": last.TokenID, "token": string(last.Token)})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	ctx, cancel := context.WithTimeout(h.ctx, 180*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", filepath.Join(repoRoot, "frontend", "tooling", "verification-native-browser.mjs"))
	cmd.Dir = filepath.Join(repoRoot, "frontend")
	cmd.Env = append(os.Environ(), "API_BACKEND_BASE_URL="+server.URL+"/api/v1", "TEST_DRIVER_URL="+server.URL, "NEXT_PORT=4349", "HOSTNAME=127.0.0.1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		for _, path := range []string{"/api/v1/auth/verification/status", "/api/v1/auth/verification", "/api/v1/auth/password/recovery/status", "/api/v1/auth/password/recovery/consume"} {
			t.Logf("native HTTP %s: 200=%d 204=%d 422=%d", path, tracker.get(path, 200), tracker.get(path, 204), tracker.get(path, 422))
		}
		t.Fatalf("native browser flow failed: %v\n%s", err, out)
	}
	for _, expected := range []struct {
		path   string
		status int
	}{
		{"/api/v1/auth/register", http.StatusCreated},
		{"/api/v1/auth/login", http.StatusForbidden},
		{"/api/v1/auth/verification/status", http.StatusOK},
		{"/api/v1/auth/verification", http.StatusNoContent},
		{"/api/v1/auth/verification", http.StatusUnprocessableEntity},
		{"/api/v1/auth/password/recovery", http.StatusAccepted},
		{"/api/v1/auth/password/recovery/status", http.StatusOK},
		{"/api/v1/auth/password/recovery/consume", http.StatusNoContent},
		{"/api/v1/auth/password/recovery/consume", http.StatusUnprocessableEntity},
		{"/api/v1/auth/login", http.StatusOK},
		{"/api/v1/auth/session", http.StatusOK},
		{"/api/v1/auth/logout", http.StatusNoContent},
	} {
		if tracker.get(expected.path, expected.status) == 0 {
			t.Fatalf("native browser did not exercise %s with status %d", expected.path, expected.status)
		}
	}
	if tracker.get("/api/v1/auth/verification", http.StatusNoContent) != 1 {
		t.Fatal("verification must succeed exactly once")
	}
	if tracker.get("/api/v1/auth/password/recovery/consume", http.StatusNoContent) != 1 {
		t.Fatal("password recovery consume must succeed exactly once")
	}
	t.Log("Native browser/Next/Go/PostgreSQL: pending login denied, verification acknowledged, replay rejected, login/session/logout confirmed")
}
