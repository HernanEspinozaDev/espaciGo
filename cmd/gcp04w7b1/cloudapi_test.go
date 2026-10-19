package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04w7b1"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

func mockCloudAPI(t *testing.T, s *httptest.Server) cloudAPI {
	t.Helper()
	ctx := context.Background()
	cred := &google.Credentials{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-only"})}
	opts := []option.ClientOption{option.WithEndpoint(s.URL + "/"), option.WithCredentials(cred)}
	sql, e := sqladmin.NewService(ctx, opts...)
	if e != nil {
		t.Fatal(e)
	}
	sm, e := secretmanager.NewService(ctx, opts...)
	if e != nil {
		t.Fatal(e)
	}
	return cloudAPI{sql: sql, sm: sm}
}

func TestInsertUsesExplicitCloudSQLSuperuserAndSingleRequest(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || r.URL.Path != "/sql/v1beta4/projects/espacigo-testprod/instances/espacigo-testprod-pg/users" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		roles, ok := body["databaseRoles"].([]any)
		if !ok || len(roles) != 1 || roles[0] != gcp04w7b1.AdminRole {
			t.Errorf("role omitted or implicit: %#v", body)
		}
		if body["name"] != gcp04w7b1.Bootstrap || body["type"] != "BUILT_IN" || body["password"] != "synthetic-only" {
			t.Errorf("wrong user request %#v", body)
		}
		_, _ = w.Write([]byte(`{"name":"op-1","status":"PENDING"}`))
	}))
	defer s.Close()
	a := mockCloudAPI(t, s)
	op, e := a.InsertUser(context.Background(), gcp04w7b1.Bootstrap, "BUILT_IN", []string{gcp04w7b1.AdminRole}, []byte("synthetic-only"))
	if e != nil || op != "op-1" {
		t.Fatalf("insert result %q %v", op, e)
	}
	if calls != 1 {
		t.Fatalf("user insert retried %d times", calls)
	}
}

func TestInsertFailureIsNotRetried(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"message":"simulated"}}`))
	}))
	defer s.Close()
	a := mockCloudAPI(t, s)
	_, err := a.InsertUser(context.Background(), gcp04w7b1.Bootstrap, "BUILT_IN", []string{gcp04w7b1.AdminRole}, []byte("synthetic-only"))
	if err == nil || calls != 1 {
		t.Fatalf("uncertain API result retried: err=%v calls=%d", err, calls)
	}
}

func TestCreateSecretIsExactUserManagedShell(t *testing.T) {
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, "/secrets") || r.URL.Query().Get("secretId") != gcp04w7b1.SecretID {
			t.Errorf("unexpected shell request %s %s", r.Method, r.URL.String())
		}
		var body secretmanager.Secret
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Replication == nil || body.Replication.UserManaged == nil || len(body.Replication.UserManaged.Replicas) != 1 || body.Replication.UserManaged.Replicas[0].Location != gcp04w7b1.Region {
			t.Errorf("replication is not pinned: %#v", body.Replication)
		}
		if body.Labels["release_id"] != "b1-test-1" || body.Labels["purpose"] != "gcp04-w7-b1-bootstrap" {
			t.Errorf("ownership labels missing: %#v", body.Labels)
		}
		_, _ = w.Write([]byte(`{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w7-b1-bootstrap-credential","etag":"shell-etag","labels":{"release_id":"b1-test-1","purpose":"gcp04-w7-b1-bootstrap"},"replication":{"userManaged":{"replicas":[{"location":"southamerica-west1"}]}}}`))
	}))
	defer s.Close()
	a := mockCloudAPI(t, s)
	secret, err := a.CreateSecret(context.Background(), "b1-test-1")
	if err != nil || secret.ETag != "shell-etag" || len(secret.Replicas) != 1 || calls != 1 {
		t.Fatalf("secret shell %#v %v calls=%d", secret, err, calls)
	}
}

func TestAddVersionSendsOnlyExpectedEncodedMemoryPayload(t *testing.T) {
	payload := []byte("synthetic-test-payload")
	calls := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodPost || !strings.HasSuffix(r.URL.Path, ":"+"addVersion") {
			t.Errorf("unexpected addVersion path %s", r.URL.Path)
		}
		var body secretmanager.AddSecretVersionRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Payload == nil || body.Payload.Data != base64.StdEncoding.EncodeToString(payload) {
			t.Errorf("unexpected payload encoding")
		}
		_, _ = w.Write([]byte(`{"name":"projects/espacigo-testprod/secrets/espacigo-gcp04-w7-b1-bootstrap-credential/versions/9","etag":"v-etag"}`))
	}))
	defer s.Close()
	a := mockCloudAPI(t, s)
	v, err := a.AddVersion(context.Background(), parent(), payload)
	if err != nil || v.Number != "9" || v.ETag != "v-etag" {
		t.Fatalf("version result %#v %v", v, err)
	}
	if calls != 1 {
		t.Fatalf("addVersion calls=%d", calls)
	}
}

func TestDeleteVersionUsesExactETagAndDeleteShellChecksOwnership(t *testing.T) {
	version := parent() + "/versions/9"
	calls := []string{}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path+" "+r.URL.Query().Get("etag"))
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/versions/9"):
			_ = json.NewEncoder(w).Encode(map[string]any{"name": version, "state": "ENABLED", "etag": "v-etag"})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/versions/9:destroy"):
			_ = json.NewEncoder(w).Encode(map[string]any{"name": version, "state": "DESTROYED", "etag": "v-etag"})
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/"+gcp04w7b1.SecretID):
			_ = json.NewEncoder(w).Encode(map[string]any{"name": parent(), "etag": "s-etag", "labels": map[string]string{"release_id": "run-1", "purpose": "gcp04-w7-b1-bootstrap"}})
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/"+gcp04w7b1.SecretID):
			if r.URL.Query().Get("etag") != "s-etag" {
				t.Errorf("shell ETag missing")
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected cleanup request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer s.Close()
	a := mockCloudAPI(t, s)
	if err := a.DestroyVersion(context.Background(), version, "v-etag"); err != nil {
		t.Fatal(err)
	}
	if err := a.DeleteSecret(context.Background(), parent(), "run-1", "s-etag"); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 4 {
		t.Fatalf("cleanup HTTP calls=%v", calls)
	}
	if err := a.DeleteSecret(context.Background(), parent(), "different-run", "s-etag"); err == nil {
		t.Fatal("secret shell from another run was adopted")
	}
}
