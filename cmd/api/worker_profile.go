package main

import (
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/evidencefs"
	"github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type runtimeProfileName string

const (
	runtimeProfileStandard         runtimeProfileName = "standard"
	runtimeProfileWorkerFree       runtimeProfileName = "local-http-worker-free"
	runtimeProfileWizardLocal      runtimeProfileName = "local-space-wizard"
	runtimeProfileSpacesOrder2     runtimeProfileName = "local-spaces-order2"
	runtimeProfileDocumentScanOnly runtimeProfileName = "local-document-scan-only"
	documentScanAPIAddr                               = "127.0.0.1:18082"
)

func runtimeProfile(getenv func(string) string) (runtimeProfileName, error) {
	p := strings.TrimSpace(getenv("API_RUNTIME_PROFILE"))
	if p == "" {
		p = string(runtimeProfileStandard)
	}
	switch runtimeProfileName(p) {
	case runtimeProfileStandard:
		return runtimeProfileStandard, nil
	case runtimeProfileWorkerFree, runtimeProfileWizardLocal, runtimeProfileSpacesOrder2, runtimeProfileDocumentScanOnly:
		if getenv("LOCAL_AUTH_PROTOTYPE") != "1" {
			return "", errors.New("local-http-worker-free requires LOCAL_AUTH_PROTOTYPE=1")
		}
		if getenv("APP_PROFILE") != "" {
			return "", errors.New("local-http-worker-free cannot use APP_PROFILE")
		}
		httpAddr := getenv("HTTP_ADDR")
		httpHost, _, err := net.SplitHostPort(httpAddr)
		if err != nil || strings.TrimSpace(httpAddr) != httpAddr {
			return "", errors.New("local-http-worker-free requires an explicit loopback HTTP_ADDR")
		}
		hostIP := net.ParseIP(strings.Trim(httpHost, "[]"))
		if strings.ToLower(httpHost) != "localhost" && (hostIP == nil || !hostIP.IsLoopback()) {
			return "", errors.New("local-http-worker-free requires a loopback HTTP_ADDR")
		}
		if strings.TrimSpace(getenv("DATABASE_CONNECTION_MODE")) != "direct" {
			return "", errors.New("local-http-worker-free requires direct database connection mode")
		}
		host := strings.ToLower(strings.TrimSpace(getenv("DATABASE_HOST")))
		ip := net.ParseIP(host)
		if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return "", errors.New("local-http-worker-free requires a loopback database host")
		}
		b := getenv("BLOB_STORE_BACKEND")
		if b != "" && b != "filesystem" {
			return "", errors.New("local-http-worker-free requires filesystem blob storage")
		}
		replayBackend := strings.TrimSpace(getenv("LOCAL_PRIVACY_REPLAY_BACKEND"))
		if replayBackend != "" && replayBackend != "filesystem" {
			return "", errors.New("local-http-worker-free requires filesystem privacy replay storage")
		}
		root := strings.TrimSpace(getenv("M03_EVIDENCE_DIR"))
		if root == "" || !filepath.IsAbs(root) {
			return "", errors.New("local-http-worker-free requires an absolute M03_EVIDENCE_DIR")
		}
		if _, err := evidencefs.New(root); err != nil {
			return "", errors.New("local-http-worker-free requires an existing private evidence directory")
		}
		profile := runtimeProfileName(p)
		if profile == runtimeProfileWizardLocal {
			if !isAuthorizedWizardRuntime(getenv, httpAddr) {
				return "", errors.New("local-space-wizard requires the exact isolated wizard target")
			}
		} else if profile == runtimeProfileSpacesOrder2 {
			if !isAuthorizedSpacesOrder2Runtime(getenv, httpAddr) {
				return "", errors.New("local-spaces-order2 requires the exact approved isolated target")
			}
		} else if !isAuthorizedF5Runtime(getenv, httpAddr) {
			return "", errors.New("local worker profile requires the authorized F5 loopback API and least-privilege database target")
		}
		if profile == runtimeProfileDocumentScanOnly {
			_, enabled, scanErr := documentScanConfigFromEnv(getenv)
			if scanErr != nil || !enabled {
				return "", errors.New("document scan profile requires explicit private ClamAV address and signature freshness")
			}
			documentRoot := getenv("DOCUMENT_VERIFICATION_STORAGE_ROOT")
			keyPath := getenv("DOCUMENT_VERIFICATION_ENCRYPTION_KEY_FILE")
			if documentRoot == "" || filepath.Clean(documentRoot) != documentRoot || !filepath.IsAbs(documentRoot) || keyPath == "" || filepath.Clean(keyPath) != keyPath || !filepath.IsAbs(keyPath) || pathsOverlap(documentRoot, root) {
				return "", errors.New("document scan profile requires separate absolute private storage and key paths")
			}
			keyInfo, keyErr := os.Lstat(keyPath)
			if keyErr != nil || !keyInfo.Mode().IsRegular() || keyInfo.Mode()&os.ModeSymlink != 0 || keyInfo.Mode().Perm() != 0o600 {
				return "", errors.New("document scan profile requires an existing 0600 encryption key file")
			}
		}
		return profile, nil
	default:
		return "", errors.New("API_RUNTIME_PROFILE must be standard, local-http-worker-free, local-space-wizard, local-spaces-order2, or local-document-scan-only")
	}
}

func isAuthorizedWizardRuntime(getenv func(string) string, httpAddr string) bool {
	return getenv("DATABASE_HOST") == "127.0.0.1" && getenv("DATABASE_PORT") == "15436" && getenv("DATABASE_NAME") == "espacigo_wizard_20261106" && getenv("DATABASE_USER") == "espacigo_api_login" && getenv("DATABASE_SET_ROLE") == "espacigo_runtime" && httpAddr == "127.0.0.1:18086"
}

func isAuthorizedSpacesOrder2Runtime(getenv func(string) string, httpAddr string) bool {
	return getenv("DATABASE_HOST") == "127.0.0.1" && getenv("DATABASE_PORT") == "15440" && getenv("DATABASE_NAME") == "espacigo_spaces_order2_20261113" && getenv("DATABASE_USER") == "espacigo_api_login" && getenv("DATABASE_SET_ROLE") == "espacigo_runtime" && httpAddr == "127.0.0.1:18088"
}

func isAuthorizedF5Runtime(getenv func(string) string, httpAddr string) bool {
	return getenv("DATABASE_HOST") == "127.0.0.1" && getenv("DATABASE_PORT") == "15434" && getenv("DATABASE_NAME") == "espacigo_f5_gate_20261101" && getenv("DATABASE_USER") == "espacigo_api_login" && getenv("DATABASE_SET_ROLE") == "espacigo_runtime" && httpAddr == documentScanAPIAddr
}

type documentProfileRoute struct {
	method string
	path   string
}

var documentProfileExactRoutes = map[documentProfileRoute]struct{}{
	{"GET", "/openapi.yaml"}:        {},
	{"GET", "/api/v1/auth/terms"}:   {},
	{"POST", "/api/v1/auth/login"}:  {},
	{"GET", "/api/v1/auth/session"}: {},
	{"POST", "/api/v1/auth/logout"}: {},
}

// restrictDocumentVerificationRoutes limits the isolated profile to its required HTTP surface.
func restrictDocumentVerificationRoutes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		allowed := false
		if r.URL.Path == "/health/live" && r.Method == http.MethodGet {
			allowed = true
		}
		if r.URL.Path == "/health/ready" && (r.Method == http.MethodGet || r.Method == http.MethodOptions) {
			allowed = true
		}
		if _, ok := documentProfileExactRoutes[documentProfileRoute{method: r.Method, path: r.URL.Path}]; ok {
			allowed = true
		}
		if isDocumentVerificationCasesRoute(r.Method, r.URL.Path) {
			allowed = true
		}
		if r.Method == http.MethodGet && r.URL.Path == "/api/v1/spaces" {
			allowed = true
		}
		if r.Method == http.MethodGet && isOwnerDocumentDownloadPath(r.URL.Path) {
			allowed = true
		}
		if !allowed {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func isDocumentVerificationCasesRoute(method, path string) bool {
	const casesPath = "/api/v1/document-verification/cases"
	if path == casesPath || path == casesPath+"/" {
		return method == http.MethodGet || method == http.MethodPost
	}
	if !strings.HasPrefix(path, casesPath+"/") {
		return false
	}
	segments := strings.Split(strings.TrimPrefix(path, casesPath+"/"), "/")
	if len(segments) == 1 {
		return method == http.MethodGet && documentverification.ValidateID(segments[0]) == nil
	}
	if len(segments) != 2 || documentverification.ValidateID(segments[0]) != nil {
		return false
	}
	return method == http.MethodPost && (segments[1] == "submit" || segments[1] == "files")
}

func isOwnerDocumentDownloadPath(path string) bool {
	parts := strings.Split(path, "/")
	return len(parts) == 9 &&
		parts[1] == "api" && parts[2] == "v1" &&
		parts[3] == "document-verification" && parts[4] == "space-cases" &&
		documentverification.ValidateID(parts[5]) == nil && parts[6] == "files" &&
		documentverification.ValidateID(parts[7]) == nil && parts[8] == "content"
}

func spacesOrder2RouteAllowed(method, path string) bool {
	if method == http.MethodGet && (path == "/health/live" || path == "/health/ready" || path == "/openapi.yaml") {
		return true
	}
	if method == http.MethodGet && path == "/api/v1/auth/terms" ||
		method == http.MethodPost && (path == "/api/v1/auth/login" || path == "/api/v1/auth/logout") ||
		method == http.MethodGet && path == "/api/v1/auth/session" {
		return true
	}
	if method == http.MethodGet && (path == "/api/v1/spaces" || path == "/api/v1/spaces/") {
		return true
	}
	parts := strings.Split(path, "/")
	if len(parts) < 5 || parts[1] != "api" || parts[2] != "v1" || parts[3] != "spaces" || documentverification.ValidateID(parts[4]) != nil {
		return false
	}
	if len(parts) == 5 {
		// The owner draft read supplies the ETag used by order-2 settings edits.
		return method == http.MethodGet
	}
	if len(parts) == 6 && parts[5] == "availability" {
		return method == http.MethodGet || method == http.MethodPut
	}
	if len(parts) == 6 && parts[5] == "weekly-hours" {
		return method == http.MethodGet || method == http.MethodPut
	}
	if len(parts) == 7 && parts[5] == "availability" && parts[6] == "timezone" {
		return method == http.MethodGet
	}
	if len(parts) == 7 && parts[5] == "availability" && parts[6] == "blocks" {
		return method == http.MethodGet || method == http.MethodPost
	}
	if len(parts) == 8 && parts[5] == "availability" && parts[6] == "blocks" &&
		documentverification.ValidateID(parts[7]) == nil {
		return method == http.MethodDelete
	}
	return false
}

func restrictSpacesOrder2Routes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !spacesOrder2RouteAllowed(r.Method, r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func handlerForRuntimeProfile(profile runtimeProfileName, handler http.Handler) http.Handler {
	switch profile {
	case runtimeProfileDocumentScanOnly:
		return restrictDocumentVerificationRoutes(handler)
	case runtimeProfileSpacesOrder2:
		return restrictSpacesOrder2Routes(handler)
	default:
		return handler
	}
}

type namedWorker struct {
	name   string
	launch func()
}

func validateWorkerRegistry(profile runtimeProfileName, workers []namedWorker) error {
	if profile != runtimeProfileStandard && profile != runtimeProfileWorkerFree && profile != runtimeProfileWizardLocal && profile != runtimeProfileSpacesOrder2 && profile != runtimeProfileDocumentScanOnly {
		return errors.New("invalid runtime profile for background workers")
	}
	documentScanWorkers := 0
	documentRetentionWorkers := 0
	for _, worker := range workers {
		if worker.name == "" || worker.launch == nil {
			return errors.New("invalid background worker registration")
		}
		if worker.name == "document-verification-scan" {
			documentScanWorkers++
		}
		if worker.name == "document-verification-retention" {
			documentRetentionWorkers++
		}
	}
	if profile == runtimeProfileDocumentScanOnly && (documentScanWorkers != 1 || documentRetentionWorkers != 1) {
		return errors.New("document-scan-only profile requires exactly one document verification scanner and retention worker")
	}
	if profile != runtimeProfileDocumentScanOnly && (documentScanWorkers != 0 || documentRetentionWorkers != 0) {
		return errors.New("document verification workers are restricted to the document-scan-only profile")
	}
	return nil
}

func launchWorkers(profile runtimeProfileName, workers []namedWorker) error {
	if err := validateWorkerRegistry(profile, workers); err != nil {
		return err
	}
	if profile == runtimeProfileWorkerFree || profile == runtimeProfileWizardLocal || profile == runtimeProfileSpacesOrder2 {
		return nil
	}
	for _, worker := range workers {
		if profile == runtimeProfileDocumentScanOnly && worker.name != "document-verification-scan" && worker.name != "document-verification-retention" {
			continue
		}
		worker.launch()
	}
	return nil
}
