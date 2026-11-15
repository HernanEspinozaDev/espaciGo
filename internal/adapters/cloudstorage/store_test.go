package cloudstorage

import (
	"context"
	"encoding/json"
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
	var deletedGeneration string
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
		case r.Method == http.MethodGet && r.URL.Query().Get("alt") == "json" && strings.Contains(r.URL.Path, "/private-bucket/"):
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"name":"runs/g03o-1017/`+id+`.blob","generation":"7","size":"`+strconv.Itoa(len(data))+`"}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/private-bucket/"):
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(data)
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/private-bucket/"):
			deletedGeneration = r.URL.Query().Get("ifGenerationMatch")
			if deletedGeneration != "7" {
				http.Error(w, "missing observed generation precondition", http.StatusPreconditionFailed)
				return
			}
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
	if deletedGeneration != "7" {
		t.Fatalf("Delete used ifGenerationMatch=%q, want observed generation 7", deletedGeneration)
	}
}

func TestStoreDeleteDoesNotRemoveReplacementGeneration(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	var currentGeneration int64 = 8
	var deletePrecondition string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Query().Get("alt") == "json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"name":"`+id+`.blob","generation":"7","size":"1"}`)
		case r.Method == http.MethodDelete:
			deletePrecondition = r.URL.Query().Get("ifGenerationMatch")
			if deletePrecondition != strconv.FormatInt(currentGeneration, 10) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPreconditionFailed)
				_, _ = io.WriteString(w, `{"error":{"code":412,"message":"generation changed"}}`)
				return
			}
			currentGeneration = -1
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
	store := newStoreWithClient(client, "private-bucket")
	if err := store.Delete(context.Background(), id); err == nil {
		t.Fatal("Delete ignored a generation change between metadata read and delete")
	}
	if deletePrecondition != "7" || currentGeneration != 8 {
		t.Fatalf("delete precondition=%q current generation=%d; replacement generation was removed", deletePrecondition, currentGeneration)
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
		http.Error(w, "synthetic storage failure", http.StatusForbidden)
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

func TestStoreMetadataRoundTripAndBoundedRead(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	want := []byte("encrypted synthetic document")
	wantMetadata := map[string]string{"key_version": "v1"}
	var stored []byte
	var storedMetadata map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/storage/v1/b/private-bucket/o"):
			if r.URL.Query().Get("ifGenerationMatch") != "0" {
				http.Error(w, "missing create-only precondition", http.StatusBadRequest)
				return
			}
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				http.Error(w, "invalid multipart", http.StatusBadRequest)
				return
			}
			parts := multipart.NewReader(r.Body, params["boundary"])
			metadataPart, err := parts.NextPart()
			if err != nil {
				http.Error(w, "missing metadata", http.StatusBadRequest)
				return
			}
			var upload struct {
				Metadata map[string]string `json:"metadata"`
			}
			if err := json.NewDecoder(metadataPart).Decode(&upload); err != nil {
				http.Error(w, "invalid metadata", http.StatusBadRequest)
				return
			}
			storedMetadata = upload.Metadata
			objectPart, err := parts.NextPart()
			if err != nil {
				http.Error(w, "missing object bytes", http.StatusBadRequest)
				return
			}
			stored, _ = io.ReadAll(objectPart)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"bucket": "private-bucket", "name": id + ".blob", "generation": "7",
				"size": strconv.Itoa(len(stored)), "metadata": storedMetadata,
			})
		case r.Method == http.MethodGet && r.URL.Query().Get("alt") == "json":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"name": id + ".blob", "generation": "7", "size": strconv.Itoa(len(stored)),
				"metageneration": "1", "metadata": storedMetadata,
			})
		case r.Method == http.MethodGet:
			if r.Header.Get("x-goog-if-generation-match") != "7" || r.Header.Get("x-goog-if-metageneration-match") != "1" {
				http.Error(w, "read did not pin the observed object and metadata generations", http.StatusPreconditionFailed)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(stored)
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
	store := newStoreWithClient(client, "private-bucket")
	if err := store.PutWithMetadata(context.Background(), id, want, wantMetadata); err != nil {
		t.Fatal(err)
	}
	if string(stored) != string(want) || storedMetadata["key_version"] != "v1" {
		t.Fatalf("stored object = %q metadata=%v", stored, storedMetadata)
	}
	got, metadata, err := store.GetWithMetadata(context.Background(), id, int64(len(want)))
	if err != nil || string(got) != string(want) || metadata["key_version"] != "v1" {
		t.Fatalf("GetWithMetadata() = %q, %v, %v", got, metadata, err)
	}
	got, metadata, err = store.GetWithMetadata(context.Background(), id, int64(len(want)-1))
	if err == nil || got != nil || metadata != nil {
		t.Fatalf("bounded GetWithMetadata() = %q, %v, %v; want size rejection", got, metadata, err)
	}
}

func TestValidateMetadataRejectsUnsafeValuesAndBounds(t *testing.T) {
	tooMany := make(map[string]string)
	for i := 0; i < 33; i++ {
		tooMany["key"+strconv.Itoa(i)] = "v"
	}
	tooLarge := make(map[string]string)
	for i := 0; i < 9; i++ {
		tooLarge["k"+strconv.Itoa(i)] = strings.Repeat("x", 1000)
	}
	for name, metadata := range map[string]map[string]string{
		"uppercase key":       {"Key": "v1"},
		"empty value":         {"key": ""},
		"line break":          {"key": "v1\nother"},
		"delete control":      {"key": "v1\x7f"},
		"non-ascii":           {"key": "versión"},
		"oversized value":     {"key": strings.Repeat("x", 1025)},
		"too many entries":    tooMany,
		"oversized aggregate": tooLarge,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := validateMetadata(metadata); err == nil {
				t.Fatalf("validateMetadata(%q) accepted unsafe metadata", name)
			}
		})
	}

	input := map[string]string{"key_version": "v1"}
	clone, err := validateMetadata(input)
	if err != nil {
		t.Fatal(err)
	}
	input["key_version"] = "v2"
	if clone["key_version"] != "v1" {
		t.Fatalf("validated metadata aliases caller map: %v", clone)
	}
}
