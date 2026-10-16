package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03scratch"
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
