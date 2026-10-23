package postgres_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
)

// Explicit opt-in: uses only newIdentityTestPool's disposable database. Runtime
// config/UI proxying is local; auth HTTP requests and PostgreSQL are not mocked.
func TestAuthRealBrowserLogin(t *testing.T) {
	if os.Getenv("AUTH_BROWSER_FIXTURE_AUTHORIZED") != "1" {
		t.Skip("requires explicit fixture authorization after checking private account policy")
	}
	frontend := os.Getenv("AUTH_BROWSER_FRONTEND_URL")
	if frontend == "" {
		t.Skip("AUTH_BROWSER_FRONTEND_URL is not set")
	}
	parsed, err := url.Parse(frontend)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Scheme != "http" {
		t.Fatal("requires loopback frontend")
	}
	h := newAuthHarness(t)
	h.service, err = identity.NewAuthenticationService(h.repo, password.Hasher{}, h.mail, h.limiter, credentials.Generator{}, func() time.Time { return h.now })
	if err != nil {
		t.Fatal(err)
	}
	// Reserved .invalid, isolated test DB, production registration and verification.
	h.register(t, "browser-fixture@example.invalid")
	h.verify(t)
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	var handler http.Handler
	var origin string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/config.json":
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{"apiBaseUrl":%q,"siteOrigin":%q}`, origin+"/api/v1", origin)
		case strings.HasPrefix(r.URL.Path, "/api/v1/auth/"):
			handler.ServeHTTP(w, r)
		case r.URL.Path == "/health/ready":
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status":"ready"}`))
		default:
			proxy.ServeHTTP(w, r)
		}
	}))
	defer server.Close()
	origin = server.URL
	handler = identityhttp.NewHandler(h.service, h.repo, []string{origin})
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "e2e/auth-real-network.cjs")
	cmd.Dir = filepath.Join(root, "frontend")
	cmd.Env = append(os.Environ(), "AUTH_BROWSER_BASE_URL="+origin)
	// Browser script prints assertions only; never request bodies/credentials.
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("real browser regression failed: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
