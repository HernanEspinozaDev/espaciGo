package cloudstorage

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func TestStorePrivateObjectLifecycleAndCreateOnlyWrite(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	var name string
	var data []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/storage/v1/b/private-bucket/o"):
			if r.URL.Query().Get("ifGenerationMatch") != "0" || r.URL.Query().Get("name") != "runs/g03o-1017/"+id+".blob" {
				http.Error(w, "missing create-only precondition", http.StatusBadRequest)
				return
			}
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				http.Error(w, "invalid multipart", http.StatusBadRequest)
				return
			}
			reader := multipart.NewReader(r.Body, params["boundary"])
			metadata, err := reader.NextPart()
			if err != nil {
				http.Error(w, "missing metadata", http.StatusBadRequest)
				return
			}
			_, _ = io.Copy(io.Discard, metadata)
			part, err := reader.NextPart()
			if err != nil {
				http.Error(w, "missing object bytes", http.StatusBadRequest)
				return
			}
			name = r.URL.Query().Get("name")
			data, _ = io.ReadAll(part)
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"bucket":"private-bucket","name":"`+name+`","generation":"1","size":"`+strconv.Itoa(len(data))+`"}`)
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/private-bucket/"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/private-bucket/"):
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := storage.NewClient(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	store := newStoreWithPrefix(client, "private-bucket", "runs/g03o-1017")
	want := []byte("private synthetic bytes")
	if err := store.Put(context.Background(), id, want); err != nil {
		t.Fatal(err)
	}
	if name != "runs/g03o-1017/"+id+".blob" || string(data) != string(want) {
		t.Fatalf("stored object = %q %q", name, data)
	}
	got, err := store.Get(context.Background(), id)
	if err != nil || string(got) != string(want) {
		t.Fatalf("Get() = %q, %v", got, err)
	}
	if err := store.Delete(context.Background(), id); err != nil {
		t.Fatal(err)
	}
}

func TestStoreRejectsInvalidIDAndCanceledContextBeforeAccess(t *testing.T) {
	store := &Store{}
	if err := store.Put(context.Background(), "../public/file", []byte("x")); err == nil {
		t.Fatal("Put accepted path traversal")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.Get(ctx, "11111111-1111-4111-8111-111111111111"); err == nil {
		t.Fatal("Get ignored canceled context")
	}
}

func TestStoreDeleteReturnsRemoteFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "synthetic storage failure", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, err := storage.NewClient(context.Background(), option.WithEndpoint(server.URL), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	store := newStoreWithClient(client, "private-bucket")
	if err := store.Delete(context.Background(), "11111111-1111-4111-8111-111111111111"); err == nil {
		t.Fatal("Delete swallowed a remote storage failure")
	}
}

func TestValidatePrefixRejectsTraversalAndAcceptsExactRunPrefix(t *testing.T) {
	for _, prefix := range []string{"../other", "runs/../other", "/absolute", "runs//g03o-1017", "runs\\g03o-1017"} {
		if err := validatePrefix(prefix); err == nil {
			t.Fatalf("validatePrefix(%q) accepted unsafe prefix", prefix)
		}
	}
	if err := validatePrefix("runs/g03o-1017"); err != nil {
		t.Fatalf("validatePrefix rejected reviewed run prefix: %v", err)
	}
}
