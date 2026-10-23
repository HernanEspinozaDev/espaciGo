package postgres_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/credentials"
	password "github.com/HernanEspinozaDev/espaciGo/internal/adapters/password/bcrypt"
	"github.com/HernanEspinozaDev/espaciGo/internal/identity"
	identityhttp "github.com/HernanEspinozaDev/espaciGo/internal/identity/transport/http"
	"github.com/jackc/pgx/v5"
)

// Explicit opt-in: uses only newIdentityTestPool's disposable database. Runtime
// config/UI proxying is local; auth HTTP requests and PostgreSQL are not mocked.
func TestAuthRealBrowserLogin(t *testing.T) {
	if os.Getenv("AUTH_BROWSER_FIXTURE_AUTHORIZED") != "1" {
		t.Skip("requires explicit fixture authorization after checking private account policy")
	}
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if err := validateAuthBrowserTestDatabase(adminURL, os.Getenv("AUTH_BROWSER_DISPOSABLE_ROOT"), gcp04ReplayCloudMode()); err != nil {
		t.Fatalf("refusing to connect to an unverified test database: %v", err)
	}
	frontend := os.Getenv("AUTH_BROWSER_FRONTEND_URL")
	if frontend == "" {
		t.Skip("AUTH_BROWSER_FRONTEND_URL is not set")
	}
	parsed, err := url.Parse(frontend)
	if err != nil || parsed.Hostname() != "127.0.0.1" || parsed.Scheme != "http" {
		t.Fatal("requires loopback frontend")
	}
	if err := verifyAuthBrowserDisposableContainer(os.Getenv("AUTH_BROWSER_DISPOSABLE_ROOT")); err != nil {
		t.Fatalf("refusing to use an unverified PostgreSQL runner: %v", err)
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

const authBrowserMarkerPrefix = "espacigo-auth-browser-disposable-v1\n"

// Validate the destructive test harness target before newAuthHarness can connect,
// CREATE DATABASE, or install cleanup that DROPs a database. TCP and Cloud modes
// are rejected. The runner owns a mode-0700 /tmp root and marker and mounts only
// that runner's networkless, tmpfs PostgreSQL socket below it.
func validateAuthBrowserTestDatabase(dsn, root string, cloudMode bool) error {
	if cloudMode {
		return fmt.Errorf("Cloud SQL replay mode is forbidden")
	}
	if dsn == "" || root == "" {
		return fmt.Errorf("disposable test DSN and runner root are required")
	}
	root = filepath.Clean(root)
	if filepath.Dir(root) != "/tmp" || !strings.HasPrefix(filepath.Base(root), "espacigo-auth-browser.") {
		return fmt.Errorf("runner root must be a dedicated /tmp/espacigo-auth-browser.* directory")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode().Perm() != 0700 || !ownedByCurrentUser(rootInfo) {
		return fmt.Errorf("runner root must be an owned, private directory")
	}
	markerPath := filepath.Join(root, ".disposable-postgres")
	markerInfo, err := os.Lstat(markerPath)
	if err != nil || !markerInfo.Mode().IsRegular() || markerInfo.Mode().Perm() != 0600 || !ownedByCurrentUser(markerInfo) {
		return fmt.Errorf("runner marker must be an owned mode-0600 regular file")
	}
	marker, err := os.ReadFile(markerPath)
	if err != nil || !strings.HasPrefix(string(marker), authBrowserMarkerPrefix) {
		return fmt.Errorf("runner marker does not identify the disposable database harness")
	}
	containerName := strings.TrimSpace(strings.TrimPrefix(string(marker), authBrowserMarkerPrefix))
	if !regexp.MustCompile(`^espacigo-auth-browser-[0-9]+$`).MatchString(containerName) {
		return fmt.Errorf("runner marker does not name the isolated database container")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() != "localhost" || u.Path != "/postgres" || u.User == nil || u.User.Username() != "postgres" {
		return fmt.Errorf("DSN must address the local fixture administrator database")
	}
	if _, hasPassword := u.User.Password(); hasPassword {
		return fmt.Errorf("fixture DSN must not contain a password")
	}
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil || cfg.Host != filepath.Join(root, "socket") {
		return fmt.Errorf("DSN must use the socket inside the disposable runner root")
	}
	socketInfo, err := os.Lstat(cfg.Host)
	if err != nil || !socketInfo.IsDir() {
		return fmt.Errorf("disposable PostgreSQL socket directory is unavailable")
	}
	return nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Getuid())
}

func TestAuthBrowserDatabaseTargetValidationFailsBeforeConnect(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "espacigo-auth-browser.validation-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "socket"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".disposable-postgres"), []byte(authBrowserMarkerPrefix+"espacigo-auth-browser-12345\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dsn := "postgres://postgres@localhost/postgres?host=" + url.QueryEscape(filepath.Join(root, "socket"))
	if err := validateAuthBrowserTestDatabase(dsn, root, false); err != nil {
		t.Fatalf("valid isolated socket rejected before connect: %v", err)
	}
	for name, candidate := range map[string]struct {
		dsn, root string
		cloud     bool
	}{
		"cloud replay":               {dsn, root, true},
		"tcp loopback":               {"postgres://postgres@127.0.0.1/postgres", root, false},
		"socket outside marker root": {"postgres://postgres@localhost/postgres?host=" + url.QueryEscape("/tmp/another/socket"), root, false},
		"missing marker":             {dsn, filepath.Join(root, "missing"), false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := validateAuthBrowserTestDatabase(candidate.dsn, candidate.root, candidate.cloud); err == nil {
				t.Fatal("unsafe database target accepted")
			}
		})
	}
}

func verifyAuthBrowserDisposableContainer(root string) error {
	marker, err := os.ReadFile(filepath.Join(root, ".disposable-postgres"))
	if err != nil || !strings.HasPrefix(string(marker), authBrowserMarkerPrefix) {
		return fmt.Errorf("missing disposable PostgreSQL container marker")
	}
	name := strings.TrimSpace(strings.TrimPrefix(string(marker), authBrowserMarkerPrefix))
	if !regexp.MustCompile(`^espacigo-auth-browser-[0-9]+$`).MatchString(name) {
		return fmt.Errorf("invalid disposable PostgreSQL container name")
	}
	output, err := exec.Command("docker", "inspect", "--format", "{{json .}}", name).Output()
	if err != nil {
		return fmt.Errorf("cannot verify disposable PostgreSQL container")
	}
	var inspected struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
		HostConfig struct {
			NetworkMode string            `json:"NetworkMode"`
			Tmpfs       map[string]string `json:"Tmpfs"`
		} `json:"HostConfig"`
		Config struct {
			Env []string `json:"Env"`
		} `json:"Config"`
		Mounts []struct {
			Type        string `json:"Type"`
			Source      string `json:"Source"`
			Destination string `json:"Destination"`
			RW          bool   `json:"RW"`
		} `json:"Mounts"`
	}
	if json.Unmarshal(output, &inspected) != nil || !inspected.State.Running || inspected.HostConfig.NetworkMode != "none" || inspected.HostConfig.Tmpfs["/var/lib/postgresql"] == "" {
		return fmt.Errorf("PostgreSQL container must be running, networkless and backed by tmpfs")
	}
	pgdataInTmpfs := false
	for _, env := range inspected.Config.Env {
		if env == "PGDATA=/var/lib/postgresql/auth-browser-data" {
			pgdataInTmpfs = true
		}
	}
	if !pgdataInTmpfs {
		return fmt.Errorf("PostgreSQL PGDATA must be inside the runner tmpfs")
	}
	socketSource, err := filepath.EvalSymlinks(filepath.Join(root, "socket"))
	if err != nil {
		return fmt.Errorf("cannot resolve disposable PostgreSQL socket")
	}
	socketMount := false
	for _, mount := range inspected.Mounts {
		switch {
		case mount.Type == "bind" && mount.Destination == "/var/run/postgresql" && mount.RW:
			source, resolveErr := filepath.EvalSymlinks(mount.Source)
			if resolveErr == nil && source == socketSource {
				socketMount = true
			}
		default:
			return fmt.Errorf("PostgreSQL container has an unexpected persistent mount")
		}
	}
	if !socketMount || len(inspected.Mounts) != 1 {
		mounts := make([]string, 0, len(inspected.Mounts))
		for _, mount := range inspected.Mounts {
			mounts = append(mounts, mount.Type+":"+mount.Destination)
		}
		return fmt.Errorf("PostgreSQL runner socket mount layout is invalid (types/destinations: %s)", strings.Join(mounts, ","))
	}
	return nil
}
