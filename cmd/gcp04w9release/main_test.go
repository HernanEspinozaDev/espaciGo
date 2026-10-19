package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w9release"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

func testAPI(t *testing.T, server *httptest.Server) api {
	t.Helper()
	ctx := context.Background()
	opt := []option.ClientOption{option.WithEndpoint(server.URL + "/"), option.WithHTTPClient(server.Client())}
	sql, e := sqladmin.NewService(ctx, opt...)
	if e != nil {
		t.Fatal(e)
	}
	sm, e := secretmanager.NewService(ctx, opt...)
	if e != nil {
		t.Fatal(e)
	}
	return api{sql: sql, sm: sm}
}
func TestMetadataReaderCollectsCompleteUserAndVersionMetadata(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/users"):
			if r.URL.Path != "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-testprod-pg/users" {
				t.Errorf("unexpected user path %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"items":[{"name":"postgres","type":"BUILT_IN","host":"","databaseRoles":[]},{"name":"observer","type":"BUILT_IN","databaseRoles":["pg_read_all_settings"]}]}`))
		case strings.HasSuffix(r.URL.Path, "/secrets/espacigo-gcp04-w8-catalog-probe-password"):
			_, _ = w.Write([]byte(`{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password","replication":{"userManaged":{"replicas":[{"location":"southamerica-west1"}]}}}`))
		case strings.HasSuffix(r.URL.Path, "/versions"):
			pages++
			if pages == 1 {
				if r.URL.Query().Get("pageSize") != "100" {
					t.Errorf("page size missing")
				}
				_, _ = w.Write([]byte(`{"versions":[{"name":"projects/espacigo-testprod/secrets/x/versions/1","state":"ENABLED","createTime":"2026-10-19T00:00:00Z","etag":"etag-1"}],"nextPageToken":"next"}`))
			} else {
				if r.URL.Query().Get("pageToken") != "next" {
					t.Errorf("next page token missing")
				}
				_, _ = w.Write([]byte(`{"versions":[{"name":"projects/espacigo-testprod/secrets/x/versions/2","state":"DISABLED","destroyTime":"2026-10-19T00:01:00Z"}]}`))
			}
		default:
			t.Errorf("unexpected endpoint %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a := testAPI(t, srv)
	users, e := a.ListUsers(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(users) != 2 || users[1].Name != "observer" || len(users[1].DatabaseRoles) != 1 {
		t.Fatalf("incomplete user inventory %#v", users)
	}
	versions, e := a.ListVersions(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if len(versions) != 2 || versions[0].State != "ENABLED" || versions[0].ETag != "etag-1" || versions[1].DestroyTime == "" {
		t.Fatalf("incomplete version inventory %#v", versions)
	}
}
func TestMetadataReaderRejectsUnexpectedUserPageToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"items":[],"nextPageToken":"unexpected"}`))
	}))
	defer srv.Close()
	a := testAPI(t, srv)
	if _, e := a.ListUsers(context.Background()); e == nil {
		t.Fatal("unexpected user pagination token accepted")
	}
}
func TestWriterUserInsertContainsExplicitRole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || !strings.Contains(r.URL.Path, "/users") {
			t.Errorf("unexpected insert request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if e := json.NewDecoder(r.Body).Decode(&body); e != nil {
			t.Error(e)
		}
		roles, ok := body["databaseRoles"].([]any)
		if !ok || len(roles) != 1 || roles[0] != gcp04w9release.Role {
			t.Errorf("databaseRoles not explicit: %#v", body)
		}
		if body["type"] != "BUILT_IN" || body["name"] != gcp04w9release.Login {
			t.Errorf("unexpected user body %#v", body)
		}
		if body["password"] != "test-only-password" {
			t.Error("test password not present in request body")
		}
		_, _ = w.Write([]byte(`{"name":"operation-1","status":"PENDING"}`))
	}))
	defer srv.Close()
	a := testAPI(t, srv)
	op, e := (writer{a}).CreateUser(context.Background(), gcp04w9release.Login, []string{gcp04w9release.Role}, []byte("test-only-password"))
	if e != nil || op != "operation-1" {
		t.Fatalf("insert result %q %v", op, e)
	}
}
