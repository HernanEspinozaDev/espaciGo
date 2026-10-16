package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp03scratch"
	"google.golang.org/api/option"
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
