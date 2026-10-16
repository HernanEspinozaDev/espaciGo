package gcp03restorecredential

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
)

func TestGoogleAPIErrorsSeparateDefinitiveFromUncertainWithoutLeakingResponse(t *testing.T) {
	testPassword := base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	for _, tc := range []struct {
		name       string
		status     int
		definitive bool
		method     func(context.Context, SQLAdmin) error
	}{
		{
			name: "definitive request rejection", status: http.StatusForbidden, definitive: true,
			method: func(ctx context.Context, sql SQLAdmin) error {
				_, err := sql.UpdatePassword(ctx, ProjectID, InstanceName, UserName, []byte(testPassword))
				return err
			},
		},
		{
			name: "uncertain server result", status: http.StatusServiceUnavailable,
			method: func(ctx context.Context, sql SQLAdmin) error {
				_, err := sql.UpdatePassword(ctx, ProjectID, InstanceName, UserName, []byte(testPassword))
				return err
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, `{"error":{"code":`+jsonNumber(tc.status)+`,"message":"response-private-marker","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"PERMISSION_DENIED","metadata":{"payload":"body-private-marker"}}]}}`)
			}))
			defer server.Close()
			sql, _, err := newGoogleAPIClients(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithoutAuthentication())
			if err != nil {
				t.Fatal(err)
			}
			err = tc.method(context.Background(), sql)
			var apiErr cloudAPIError
			if !errors.As(err, &apiErr) || apiErr.Definitive() != tc.definitive {
				t.Fatalf("API failure classification mismatch: type=%T definitive=%v", err, apiErr.Definitive())
			}
			for _, private := range []string{"response-private-marker", "body-private-marker", testPassword} {
				if strings.Contains(err.Error(), private) {
					t.Fatalf("safe API error exposed %q", private)
				}
			}
		})
	}
}

func jsonNumber(n int) string { return fmt.Sprintf("%d", n) }

func TestGoogleAPIClientsUseExactRestoreTargetAndQuotaProject(t *testing.T) {
	password := []byte(base64.RawURLEncoding.EncodeToString(make([]byte, 32)))
	if len(password) != 43 {
		t.Fatal("test password fixture must be 43 bytes")
	}
	var seenQuota []string
	var secretValue, sqlValue string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenQuota = append(seenQuota, r.Header.Get("X-goog-user-project"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/projects/espacigo-testprod/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions":
			_, _ = io.WriteString(w, `{"versions":[]}`)
		case r.Method == http.MethodPost && r.URL.Path == "/v1/projects/espacigo-testprod/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password:addVersion":
			var body struct {
				Payload struct {
					Data string `json:"data"`
				} `json:"payload"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("secret add request JSON invalid: %v", err)
			}
			decoded, err := base64.StdEncoding.DecodeString(body.Payload.Data)
			if err != nil {
				t.Errorf("secret payload is not API Base64: %v", err)
			}
			secretValue = string(decoded)
			_, _ = io.WriteString(w, `{"name":"projects/447373903153/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions/3"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-restore-g03r-1016-pg":
			_, _ = io.WriteString(w, `{"name":"espacigo-gcp03-restore-g03r-1016-pg","state":"RUNNABLE"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-restore-g03r-1016-pg/databases/espacigo_scratch":
			_, _ = io.WriteString(w, `{"name":"espacigo_scratch"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-restore-g03r-1016-pg/users/postgres":
			_, _ = io.WriteString(w, `{"name":"postgres","type":"BUILT_IN"}`)
		case r.Method == http.MethodPut && r.URL.Path == "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-gcp03-restore-g03r-1016-pg/users":
			if r.URL.Query().Get("name") != "postgres" {
				t.Errorf("users.update user query was not fixed to postgres: %v", r.URL.Query())
			}
			var body struct {
				Name, Type, Password string
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("users.update request JSON invalid: %v", err)
			}
			if body.Name != UserName || body.Type != "BUILT_IN" {
				t.Errorf("users.update target fields do not match the fixed built-in user")
			}
			sqlValue = body.Password
			_, _ = io.WriteString(w, `{"name":"operation-1"}`)
		case r.Method == http.MethodGet && r.URL.Path == "/sql/v1beta4/projects/espacigo-testprod/operations/operation-1":
			_, _ = io.WriteString(w, `{"name":"operation-1","status":"DONE"}`)
		default:
			t.Errorf("unexpected fake API request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	credentials := &google.Credentials{
		JSON:        []byte(`{"quota_project_id":"unrelated-project"}`),
		TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "synthetic-test-token"}),
	}
	sql, secrets, err := newGoogleAPIClients(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithCredentials(credentials), option.WithQuotaProject("wrong-quota-project"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	versions, err := secrets.ListVersions(ctx, SecretParent)
	if err != nil || len(versions) != 0 {
		t.Fatalf("exact secret version preflight failed: count=%d err=%v", len(versions), err)
	}
	version, err := secrets.AddVersion(ctx, SecretParent, password)
	if err != nil {
		t.Fatal("fake Secret Manager AddVersion failed")
	}
	if version != "projects/447373903153/secrets/espacigo-gcp03-restore-g03r-1016-postgres-password/versions/3" {
		t.Fatal("fake Secret Manager returned unexpected exact-run version")
	}
	name, state, err := sql.GetInstance(ctx, ProjectID, InstanceName)
	if err != nil || name != InstanceName || state != "RUNNABLE" {
		t.Fatalf("exact instance read failed: name=%q state=%q", name, state)
	}
	database, err := sql.GetDatabase(ctx, ProjectID, InstanceName, DatabaseName)
	if err != nil || database != DatabaseName {
		t.Fatal("exact scratch database read failed")
	}
	user, kind, err := sql.GetUser(ctx, ProjectID, InstanceName, UserName)
	if err != nil || user != UserName || kind != "BUILT_IN" {
		t.Fatal("exact built-in postgres user read failed")
	}
	operation, err := sql.UpdatePassword(ctx, ProjectID, InstanceName, UserName, password)
	if err != nil || operation != "operation-1" {
		t.Fatal("exact built-in postgres password update did not return an operation")
	}
	done, failed, err := sql.GetOperation(ctx, ProjectID, operation)
	if err != nil || !done || failed {
		t.Fatal("fake Cloud SQL operation did not report successful completion")
	}
	if secretValue != string(password) || sqlValue != string(password) || secretValue != sqlValue {
		t.Fatal("fake APIs did not receive the same password bytes in Secret Manager and Cloud SQL")
	}
	if len(seenQuota) != 7 {
		t.Fatalf("fake API call count = %d, want seven", len(seenQuota))
	}
	for _, quota := range seenQuota {
		if quota != ProjectID {
			t.Fatalf("x-goog-user-project = %q, want %q", quota, ProjectID)
		}
	}
}

func TestGoogleAPIsRejectAnyPilot01OrWrongRunTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("wrong-target request unexpectedly reached fake API: %s %s", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer server.Close()
	sql, secrets, err := newGoogleAPIClients(context.Background(), option.WithEndpoint(server.URL+"/"), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := sql.GetInstance(context.Background(), ProjectID, "espacigo-gcp03-pilot01-pg"); err == nil {
		t.Fatal("pilot01 SQL instance target was not rejected locally")
	}
	if _, err := sql.GetDatabase(context.Background(), ProjectID, InstanceName, "pilot01_scratch"); err == nil {
		t.Fatal("wrong database target was not rejected locally")
	}
	if _, _, err := sql.GetUser(context.Background(), ProjectID, InstanceName, "migrator"); err == nil {
		t.Fatal("wrong SQL user target was not rejected locally")
	}
	if _, err := secrets.ListVersions(context.Background(), "projects/espacigo-testprod/secrets/espacigo-gcp03-pilot01-migrator-password"); err == nil {
		t.Fatal("pilot01 secret target was not rejected locally")
	}
	if _, err := sql.UpdatePassword(context.Background(), ProjectID, InstanceName, UserName, []byte(strings.Repeat("x", 43))); err == nil {
		t.Fatal("noncanonical 43-byte password was not rejected locally")
	}
}
