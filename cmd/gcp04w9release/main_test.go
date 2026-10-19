package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w9release"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

func testAPI(t *testing.T, server *httptest.Server) api {
	t.Helper()
	ctx := context.Background()
	cred := &google.Credentials{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token", TokenType: "Bearer"})}
	opt := []option.ClientOption{option.WithEndpoint(server.URL + "/"), option.WithCredentials(cred), quotaProjectOption()}
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
		if got := r.Header.Get("X-Goog-User-Project"); got != gcp04w9release.ProjectID {
			t.Errorf("quota project header = %q", got)
		}
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
				_, _ = w.Write([]byte(`{"versions":[{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/1","state":"ENABLED","createTime":"2026-10-19T00:00:00Z","etag":"etag-1"}],"nextPageToken":"next"}`))
			} else {
				if r.URL.Query().Get("pageToken") != "next" {
					t.Errorf("next page token missing")
				}
				_, _ = w.Write([]byte(`{"versions":[{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/2","state":"DISABLED","destroyTime":"2026-10-19T00:01:00Z"}]}`))
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

func TestSecretCanonicalNameFullLifecycle(t *testing.T) {
	state := ""
	deleted := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Goog-User-Project"); got != gcp04w9release.ProjectID {
			t.Errorf("quota project header = %q", got)
		}
		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/secrets"):
			_, _ = w.Write([]byte(`{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password","etag":"secret-etag","labels":{"release_id":"release-1","purpose":"gcp04-w9-catalog-probe"},"replication":{"userManaged":{"replicas":[{"location":"southamerica-west1"}]}}}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/secrets/espacigo-gcp04-w8-catalog-probe-password:addVersion"):
			state = "ENABLED"
			_, _ = w.Write([]byte(`{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/7","etag":"version-etag"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/versions/7"):
			_, _ = w.Write([]byte(`{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/7","state":"ENABLED","etag":"version-etag"}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/versions/7:destroy"):
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["etag"] != "version-etag" {
				t.Errorf("destroy request did not pin version ETag: %#v, %v", body, err)
			}
			state = "DESTROYED"
			_, _ = w.Write([]byte(`{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/7","state":"DESTROYED","etag":"version-etag"}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/secrets/espacigo-gcp04-w8-catalog-probe-password"):
			_, _ = w.Write([]byte(`{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password","etag":"secret-etag","labels":{"release_id":"release-1","purpose":"gcp04-w9-catalog-probe"},"replication":{"userManaged":{"replicas":[{"location":"southamerica-west1"}]}}}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/versions"):
			if state == "" {
				_, _ = w.Write([]byte(`{"versions":[]}`))
			} else {
				_, _ = w.Write([]byte(`{"versions":[{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/7","state":"` + state + `","etag":"version-etag"}]}`))
			}
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/secrets/espacigo-gcp04-w8-catalog-probe-password"):
			if r.URL.Query().Get("etag") != "secret-etag" {
				t.Errorf("secret delete did not use current ETag: %q", r.URL.RawQuery)
			}
			deleted = true
			_, _ = w.Write([]byte(`{}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	a := testAPI(t, srv)
	w := writer{a}
	if err := w.CreateSecret(context.Background(), "release-1"); err != nil {
		t.Fatal(err)
	}
	created, err := a.ReadSecret(context.Background())
	if err != nil || !gcp04w9release.IsSecretParent(created.Name) || created.Name != gcp04w9release.SecretParentCanonical || created.ETag != "secret-etag" {
		t.Fatalf("canonical created secret was not accepted: %#v, %v", created, err)
	}
	emptyVersions, err := a.ListVersions(context.Background())
	if err != nil || len(emptyVersions) != 0 {
		t.Fatalf("new canonical shell was not empty: %#v, %v", emptyVersions, err)
	}
	version, err := w.AddVersion(context.Background(), created.Name, []byte("test-only-payload"))
	if err != nil || version != "7" {
		t.Fatalf("canonical version create result %q, %v", version, err)
	}
	if err := w.DestroyVersion(context.Background(), "7"); err != nil {
		t.Fatal(err)
	}
	versions, err := a.ListVersions(context.Background())
	if err != nil || len(versions) != 1 || versions[0].State != "DESTROYED" {
		t.Fatalf("destroyed version inventory = %#v, %v", versions, err)
	}
	if err := w.DeleteSecretIfOwned(context.Background(), "release-1", "7"); err != nil {
		t.Fatal(err)
	}
	if !deleted {
		t.Fatal("exact run-owned shell was not deleted")
	}
}

func TestSecretCleanupRefusesUnexpectedVersionInventory(t *testing.T) {
	for _, tc := range []struct {
		name, versions string
	}{
		{name: "active", versions: `[{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/7","state":"ENABLED"}]`},
		{name: "different", versions: `[{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/8","state":"DESTROYED"}]`},
		{name: "multiple", versions: `[{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/7","state":"DESTROYED"},{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password/versions/8","state":"DESTROYED"}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			deleted := false
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/secrets/espacigo-gcp04-w8-catalog-probe-password"):
					_, _ = w.Write([]byte(`{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w8-catalog-probe-password","labels":{"release_id":"release-1","purpose":"gcp04-w9-catalog-probe"}}`))
				case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/versions"):
					_, _ = w.Write([]byte(`{"versions":` + tc.versions + `}`))
				case r.Method == http.MethodDelete:
					deleted = true
					_, _ = w.Write([]byte(`{}`))
				default:
					t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer srv.Close()
			if err := (writer{testAPI(t, srv)}).DeleteSecretIfOwned(context.Background(), "release-1", "7"); err == nil || deleted {
				t.Fatalf("unexpected inventory cleanup result err=%v deleted=%t", err, deleted)
			}
		})
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

func TestWriterCreateSecretBuildsOnlyExactRegionalLabeledShell(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/projects/espacigo-testprod/secrets" || r.URL.Query().Get("secretId") != gcp04w9release.SecretID {
			t.Errorf("unexpected shell create request %s %s", r.Method, r.URL.String())
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		labels, _ := body["labels"].(map[string]any)
		replication, _ := body["replication"].(map[string]any)
		userManaged, _ := replication["userManaged"].(map[string]any)
		replicas, _ := userManaged["replicas"].([]any)
		if labels["release_id"] != "shell-1" || labels["purpose"] != "gcp04-w9-catalog-probe" || len(replicas) != 1 || replicas[0].(map[string]any)["location"] != gcp04w9release.Region {
			t.Errorf("shell request is not exact and regional: %#v", body)
		}
		_, _ = w.Write([]byte(`{"name":"projects/447373903153/secrets/espacigo-gcp04-w8-catalog-probe-password","etag":"shell-etag","labels":{"release_id":"shell-1","purpose":"gcp04-w9-catalog-probe"},"replication":{"userManaged":{"replicas":[{"location":"southamerica-west1"}]}}}`))
	}))
	defer srv.Close()
	if err := (writer{testAPI(t, srv)}).CreateSecret(context.Background(), "shell-1"); err != nil {
		t.Fatal(err)
	}
}

func TestMetadataReaderAcceptsOnlyExactProjectNumberAliases(t *testing.T) {
	for _, tc := range []struct {
		name       string
		resource   string
		wantSecret bool
	}{
		{name: "numeric-project-and-exact-secret", resource: gcp04w9release.SecretParentCanonical, wantSecret: true},
		{name: "wrong-project", resource: "projects/447373903154/secrets/" + gcp04w9release.SecretID},
		{name: "wrong-secret", resource: "projects/447373903153/secrets/other"},
		{name: "version-suffix-is-not-secret", resource: gcp04w9release.SecretParentCanonical + "/versions/1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/versions") {
					_, _ = w.Write([]byte(`{"versions":[{"name":"` + tc.resource + `/versions/1","state":"ENABLED"}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"name":"` + tc.resource + `","etag":"etag","replication":{"userManaged":{"replicas":[{"location":"southamerica-west1"}]}}}`))
			}))
			defer srv.Close()
			a := testAPI(t, srv)
			secret, err := a.ReadSecret(context.Background())
			if tc.wantSecret {
				if err != nil || secret.Name != gcp04w9release.SecretParentCanonical {
					t.Fatalf("exact canonical parent rejected: %#v %v", secret, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("unexpected secret alias accepted: %q", secret.Name)
			}
		})
	}
}

func TestMetadataReaderRejectsVersionsOutsideExactSecretAliases(t *testing.T) {
	for _, name := range []string{
		"projects/447373903154/secrets/" + gcp04w9release.SecretID + "/versions/1",
		gcp04w9release.SecretParentCanonical + "/versions/1/extra",
		gcp04w9release.SecretParentCanonical + "/versions/latest",
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"versions":[{"name":"` + name + `","state":"ENABLED"}]}`))
		}))
		a := testAPI(t, srv)
		if _, err := a.ListVersions(context.Background()); err == nil {
			t.Errorf("unexpected version resource accepted: %q", name)
		}
		srv.Close()
	}
}
