package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/HernanEspinozaDev/espaciGo/internal/gcp04credential"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/secretmanager/v1"
	"google.golang.org/api/sqladmin/v1beta4"
)

type staticTokenSource struct{}

func (staticTokenSource) Token() (*oauth2.Token, error) {
	return &oauth2.Token{AccessToken: "fake-client-token", TokenType: "Bearer", Expiry: time.Now().Add(time.Hour)}, nil
}

func TestStorageClientUsesSharedCredentialsAndPinnedQuotaProject(t *testing.T) {
	t.Setenv("STORAGE_EMULATOR_HOST", "")
	requestHeaders := make(chan http.Header, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"kind": "storage#bucket", "id": "gcp04-preflight-test", "name": "gcp04-preflight-test",
			"storageClass": "STANDARD", "location": "US",
		})
	}))
	defer server.Close()

	credentials, err := cloudAuthCredentialsFromTokenSource(staticTokenSource{})
	if err != nil {
		t.Fatal(err)
	}
	clientOptions := cloudClientOptions(credentials)
	if _, err := sqladmin.NewService(context.Background(), clientOptions...); err != nil {
		t.Fatalf("initialize Cloud SQL client with shared auth credentials: %v", err)
	}
	if _, err := secretmanager.NewService(context.Background(), clientOptions...); err != nil {
		t.Fatalf("initialize Secret Manager client with shared auth credentials: %v", err)
	}
	options := append(clientOptions, option.WithEndpoint(server.URL+"/storage/v1/"))
	client, err := storage.NewClient(context.Background(), options...)
	if err != nil {
		t.Fatalf("initialize Storage client with quota-pinned auth credentials: %v", err)
	}
	defer client.Close()
	if _, err := client.Bucket("gcp04-preflight-test").Attrs(context.Background()); err != nil {
		t.Fatalf("read fake bucket metadata: %v", err)
	}
	headers := <-requestHeaders
	if got := headers.Get("Authorization"); got != "Bearer fake-client-token" {
		t.Fatalf("Authorization header = %q, want the injected token source", got)
	}
	if got := headers.Get("X-goog-user-project"); got != gcp04credential.ProjectID {
		t.Fatalf("quota project header = %q, want the fixed GCP-04 project", got)
	}
}
