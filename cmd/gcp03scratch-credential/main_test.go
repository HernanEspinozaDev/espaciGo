package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03scratch"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

func TestSetDatabaseRolesUsesExplicitAdminAPIQuery(t *testing.T) {
	for _, test := range []struct {
		name      string
		roles     []string
		wantRoles []string
	}{
		{name: "preserve migration system role", roles: []string{"cloudsqlsuperuser"}, wantRoles: []string{"cloudsqlsuperuser"}},
		{name: "replace with no roles", roles: []string{}, wantRoles: []string{""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var requestPath string
			var query url.Values
			var body string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestPath = r.URL.Path
				query = r.URL.Query()
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Errorf("read request body: %v", err)
				}
				body = string(data)
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"name":"op-test"}`)
			}))
			defer server.Close()

			service, err := sqladmin.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			operation, err := (sqlUserAPI{service: service}).SetDatabaseRoles(context.Background(), gcp03scratch.MigratorLogin, test.roles, true)
			if err != nil {
				t.Fatal(err)
			}
			if operation != "op-test" {
				t.Fatalf("operation = %q", operation)
			}
			if requestPath != "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-pilot01-pg/users" {
				t.Fatalf("unexpected endpoint path: %s", requestPath)
			}
			if got := query["databaseRoles"]; len(got) != len(test.wantRoles) {
				t.Fatalf("databaseRoles query = %#v, want %#v", got, test.wantRoles)
			} else {
				for i := range got {
					if got[i] != test.wantRoles[i] {
						t.Fatalf("databaseRoles query = %#v, want %#v", got, test.wantRoles)
					}
				}
			}
			if query.Get("name") != gcp03scratch.MigratorLogin || query.Get("revokeExistingRoles") != "true" {
				t.Fatalf("missing exact update parameters: %v", query)
			}
			if strings.Contains(body, "databaseRoles") {
				t.Fatalf("roles must be encoded in update query, not ignored body: %s", body)
			}
		})
	}
}

func TestSetPasswordNamesOnlyTheFixedOwnerInTheUpdateQuery(t *testing.T) {
	var requestPath string
	var query url.Values
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		query = r.URL.Query()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"op-test"}`)
	}))
	defer server.Close()

	service, err := sqladmin.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := (sqlUserAPI{service: service}).SetPassword(context.Background(), gcp03scratch.DatabaseOwnerLogin, []byte("local-test-only-password")); err != nil {
		t.Fatal(err)
	}
	if requestPath != "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-pilot01-pg/users" || query.Get("name") != gcp03scratch.DatabaseOwnerLogin {
		t.Fatalf("password update did not name the fixed owner: path=%s query=%v", requestPath, query)
	}
	if query.Has("databaseRoles") || query.Has("revokeExistingRoles") || !strings.Contains(body, "local-test-only-password") {
		t.Fatalf("password update request has unexpected role parameters/body: query=%v body=%s", query, body)
	}
}

func TestDeleteBuiltInUserDeletesOnlyTheFixedProbeLogin(t *testing.T) {
	var requestPath string
	var query url.Values
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		query = r.URL.Query()
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		body = string(data)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"name":"op-delete"}`)
	}))
	defer server.Close()

	service, err := sqladmin.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	operation, err := (sqlUserAPI{service: service}).DeleteBuiltInUser(context.Background(), gcp03scratch.MigratorLogin)
	if err != nil {
		t.Fatal(err)
	}
	if operation != "op-delete" || requestPath != "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-pilot01-pg/users" || query.Get("name") != gcp03scratch.MigratorLogin {
		t.Fatalf("deletion did not target only the fixed probe login: op=%q path=%s query=%v", operation, requestPath, query)
	}
	if strings.Contains(body, "password") || strings.Contains(body, "role") {
		t.Fatalf("delete request unexpectedly carried credentials or role changes: %s", body)
	}
	if _, err := (sqlUserAPI{service: service}).DeleteBuiltInUser(context.Background(), gcp03scratch.DatabaseOwnerLogin); err == nil {
		t.Fatal("database-owner login deletion was accepted")
	}
}

func TestSecretVersionAPIAddVersionValidatesAndPreservesCanonicalResource(t *testing.T) {
	tests := []struct {
		name     string
		response string
		wantName string
		wantErr  bool
	}{
		{
			name:     "project number response",
			response: "projects/" + gcp03scratch.ProjectNumber + "/secrets/espacigo-gcp03-pilot01-migrator-password/versions/17",
			wantName: "projects/" + gcp03scratch.ProjectNumber + "/secrets/espacigo-gcp03-pilot01-migrator-password/versions/17",
		},
		{
			name:     "project ID response",
			response: gcp03scratch.ScratchSecretResource + "/versions/18",
			wantName: gcp03scratch.ScratchSecretResource + "/versions/18",
		},
		{
			name:     "other project rejected",
			response: "projects/other/secrets/espacigo-gcp03-pilot01-migrator-password/versions/19",
			wantName: "projects/other/secrets/espacigo-gcp03-pilot01-migrator-password/versions/19",
			wantErr:  true,
		},
		{
			name:     "other secret rejected",
			response: "projects/" + gcp03scratch.ProjectNumber + "/secrets/not-the-scratch-secret/versions/20",
			wantName: "projects/" + gcp03scratch.ProjectNumber + "/secrets/not-the-scratch-secret/versions/20",
			wantErr:  true,
		},
		{
			name:     "alias rejected",
			response: "projects/" + gcp03scratch.ProjectNumber + "/secrets/espacigo-gcp03-pilot01-migrator-password/versions/latest",
			wantName: "projects/" + gcp03scratch.ProjectNumber + "/secrets/espacigo-gcp03-pilot01-migrator-password/versions/latest",
			wantErr:  true,
		},
		{
			name:     "zero rejected",
			response: "projects/" + gcp03scratch.ProjectNumber + "/secrets/espacigo-gcp03-pilot01-migrator-password/versions/0",
			wantName: "projects/" + gcp03scratch.ProjectNumber + "/secrets/espacigo-gcp03-pilot01-migrator-password/versions/0",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, ":addVersion") {
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"name":%q}`, tt.response)
			}))
			defer server.Close()

			service, err := secretmanager.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithHTTPClient(server.Client()), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			got, err := (secretVersionAPI{service: service}).AddVersion(context.Background(), gcp03scratch.ScratchSecretResource, []byte("synthetic-test-only"))
			if got != tt.wantName {
				t.Fatalf("AddVersion() name = %q, want %q", got, tt.wantName)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("AddVersion() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestCreateBuiltInUserReportsOnlyAllowlistedAPIErrorDiagnostics(t *testing.T) {
	tests := []struct {
		name        string
		status      int
		reason      string
		wantReason  string
		errorReason string
		definitive  bool
	}{
		{name: "bad request", status: http.StatusBadRequest, reason: "INVALID_ARGUMENT", wantReason: "INVALID_ARGUMENT", definitive: true},
		{name: "forbidden", status: http.StatusForbidden, reason: "PERMISSION_DENIED", wantReason: "PERMISSION_DENIED", definitive: true},
		{name: "conflict", status: http.StatusConflict, reason: "ALREADY_EXISTS", wantReason: "ALREADY_EXISTS", definitive: false},
		{name: "rate limited", status: http.StatusTooManyRequests, reason: "RESOURCE_EXHAUSTED", wantReason: "RESOURCE_EXHAUSTED", definitive: false},
		{name: "internal", status: http.StatusInternalServerError, reason: "INTERNAL", wantReason: "INTERNAL", definitive: false},
		{name: "unavailable", status: http.StatusServiceUnavailable, reason: "UNAVAILABLE", wantReason: "UNAVAILABLE", definitive: false},
		{name: "unlisted reason is hidden", status: http.StatusBadRequest, reason: "private-reason-marker", wantReason: "UNCLASSIFIED", definitive: true},
		{name: "legacy error reason allowlisted", status: http.StatusBadRequest, errorReason: "invalidParameter", wantReason: "INVALID_ARGUMENT", definitive: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				wantPath := "/sql/v1beta4/projects/" + gcp03scratch.ProjectID + "/instances/" + gcp03scratch.InstanceName + "/users"
				if r.Method != http.MethodPost || r.URL.Path != wantPath {
					t.Errorf("unexpected API method or path")
					http.Error(w, "test failure", http.StatusBadRequest)
					return
				}
				var requestUser sqladmin.User
				decodeErr := json.NewDecoder(r.Body).Decode(&requestUser)
				if decodeErr != nil || requestUser.Name != gcp03scratch.MigratorLogin || requestUser.Type != "BUILT_IN" || requestUser.Password != "request-private-password-marker" || r.URL.Query().Get("name") != "" {
					t.Errorf("users.insert request contract mismatch: decode_ok=%t name_ok=%t type_ok=%t password_ok=%t no_name_query=%t", decodeErr == nil, requestUser.Name == gcp03scratch.MigratorLogin, requestUser.Type == "BUILT_IN", requestUser.Password == "request-private-password-marker", r.URL.Query().Get("name") == "")
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Private-Marker", "header-private-marker")
				w.WriteHeader(tt.status)
				apiError := map[string]interface{}{"code": tt.status, "message": "response-private-marker", "status": "PERMISSION_DENIED"}
				if tt.reason != "" {
					apiError["details"] = []interface{}{map[string]interface{}{"@type": "type.googleapis.com/google.rpc.ErrorInfo", "reason": tt.reason, "domain": "googleapis.com", "metadata": map[string]string{"credential": "metadata-private-marker"}}}
				}
				if tt.errorReason != "" {
					apiError["errors"] = []interface{}{map[string]string{"reason": tt.errorReason, "message": "legacy-message-private-marker"}}
				}
				_ = json.NewEncoder(w).Encode(map[string]interface{}{"error": apiError})
			}))
			defer server.Close()

			service, err := sqladmin.NewService(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			_, err = (sqlUserAPI{service: service}).CreateBuiltInUser(context.Background(), gcp03scratch.MigratorLogin, []byte("request-private-password-marker"))
			apiErr, ok := err.(cloudAPIError)
			if !ok {
				t.Fatalf("expected safe cloud API error type, got %T", err)
			}
			want := fmt.Sprintf("service=sqladmin.googleapis.com method=users.insert http_status=%d reason=%s", tt.status, tt.wantReason)
			if !strings.Contains(apiErr.Error(), want) {
				t.Fatalf("safe diagnostic %q does not contain expected classification %q", apiErr.Error(), want)
			}
			for _, marker := range []string{"response-private-marker", "metadata-private-marker", "header-private-marker", "legacy-message-private-marker", "request-private-password-marker", "private-reason-marker"} {
				if strings.Contains(apiErr.Error(), marker) {
					t.Fatalf("diagnostic exposed private marker %q", marker)
				}
			}
			if apiErr.Definitive() != tt.definitive {
				t.Fatalf("Definitive() = %t, want %t", apiErr.Definitive(), tt.definitive)
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCloudAPITransportDiagnosticIsSanitizedAndUncertain(t *testing.T) {
	service, err := sqladmin.NewService(context.Background(), option.WithEndpoint("https://sqladmin.invalid/"), option.WithHTTPClient(&http.Client{
		Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("transport-private-marker")
		}),
	}), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	_, err = (sqlUserAPI{service: service}).CreateBuiltInUser(context.Background(), gcp03scratch.MigratorLogin, []byte("request-private-password-marker"))
	apiErr, ok := err.(cloudAPIError)
	if !ok {
		t.Fatalf("expected safe cloud API error type, got %T", err)
	}
	if got, want := apiErr.Error(), "service=sqladmin.googleapis.com method=users.insert http_status=none reason=TRANSPORT_UNCERTAIN"; !strings.Contains(got, want) {
		t.Fatalf("unexpected transport diagnostic %q", got)
	}
	if apiErr.Definitive() || apiErr.NotFound() {
		t.Fatal("transport failure was classified as definitive or not found")
	}
	for _, marker := range []string{"transport-private-marker", "request-private-password-marker"} {
		if strings.Contains(apiErr.Error(), marker) {
			t.Fatalf("diagnostic exposed private marker %q", marker)
		}
	}
}

func TestCloudAPIClientsOverrideCredentialQuotaProject(t *testing.T) {
	for _, clientName := range []string{"sqladmin", "secretmanager"} {
		t.Run(clientName, func(t *testing.T) {
			var quotaProject, authorization string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				quotaProject = r.Header.Get("X-goog-user-project")
				authorization = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				if clientName == "sqladmin" {
					_, _ = io.WriteString(w, `{"items":[]}`)
					return
				}
				_, _ = fmt.Fprintf(w, `{"name":%q}`, gcp03scratch.ScratchSecretResource)
			}))
			defer server.Close()

			// Simulate ADC metadata from another quota project without loading or
			// contacting any real credential source.
			credentials := &google.Credentials{
				JSON:        []byte(`{"quota_project_id":"unrelated-quota-project"}`),
				TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "synthetic-test-token"}),
			}
			ctx := context.Background()
			if clientName == "sqladmin" {
				service, err := newSQLAdminService(ctx, option.WithEndpoint(server.URL+"/"), option.WithCredentials(credentials))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := service.Users.List(gcp03scratch.ProjectID, gcp03scratch.InstanceName).Context(ctx).Do(); err != nil {
					t.Fatal("local SQL Admin HTTP fake returned an unexpected error")
				}
			} else {
				service, err := newSecretManagerService(ctx, option.WithEndpoint(server.URL+"/"), option.WithCredentials(credentials))
				if err != nil {
					t.Fatal(err)
				}
				if _, err := service.Projects.Secrets.Get(gcp03scratch.ScratchSecretResource).Context(ctx).Do(); err != nil {
					t.Fatal("local Secret Manager HTTP fake returned an unexpected error")
				}
			}
			if quotaProject != gcp03scratch.ProjectID {
				t.Fatalf("quota project header = %q; expected explicit target project", quotaProject)
			}
			if authorization != "Bearer synthetic-test-token" {
				t.Fatal("local API fake did not receive the synthetic test authorization")
			}
		})
	}
}
