package cloudstorage

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/HernanEspinozaDev/espaciGo/internal/privacy"
	"google.golang.org/api/option"
)

type replayObjectServer struct {
	mu           sync.Mutex
	data         []byte
	generation   int64
	writeCount   int
	loseNext     bool
	requests     []string
	serverErrors []string
}

func (s *replayObjectServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.Method+" "+r.URL.String())
	switch {
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/upload/storage/v1/b/private-bucket/o"):
		if r.URL.Query().Get("name") != "privacy/privacy-replay-v1.json" {
			s.serverErrors = append(s.serverErrors, "wrong object name: "+r.URL.Query().Get("name"))
			http.Error(w, "wrong object", http.StatusBadRequest)
			return
		}
		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/related" {
			s.serverErrors = append(s.serverErrors, "wrong upload content type: "+r.Header.Get("Content-Type"))
			http.Error(w, "wrong upload encoding", http.StatusBadRequest)
			return
		}
		parts := multipart.NewReader(r.Body, params["boundary"])
		metadataPart, err := parts.NextPart()
		if err != nil {
			s.serverErrors = append(s.serverErrors, "missing multipart metadata")
			http.Error(w, "missing metadata", http.StatusBadRequest)
			return
		}
		_, _ = io.Copy(io.Discard, metadataPart)
		contentPart, err := parts.NextPart()
		if err != nil {
			s.serverErrors = append(s.serverErrors, "missing multipart body")
			http.Error(w, "missing object body", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(contentPart)
		if err != nil {
			s.serverErrors = append(s.serverErrors, "multipart body read failed")
			http.Error(w, "read failed", http.StatusBadRequest)
			return
		}
		condition := r.URL.Query().Get("ifGenerationMatch")
		if condition == "0" && s.generation != 0 || condition != "0" && condition != strconv.FormatInt(s.generation, 10) {
			s.serverErrors = append(s.serverErrors, "generation precondition failed: "+condition)
			http.Error(w, "generation precondition failed", http.StatusPreconditionFailed)
			return
		}
		s.data = append([]byte(nil), data...)
		s.generation++
		s.writeCount++
		generation := s.generation
		if s.loseNext {
			s.loseNext = false
			http.Error(w, "response lost after commit", http.StatusServiceUnavailable)
			return
		}
		writeObjectMetadata(w, "privacy/privacy-replay-v1.json", generation, data)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/private-bucket/"):
		if s.generation == 0 {
			s.serverErrors = append(s.serverErrors, "object not found")
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("X-Goog-Generation", strconv.FormatInt(s.generation, 10))
		w.Header().Set("X-Goog-Metageneration", "1")
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(s.data)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(s.data)
	default:
		http.NotFound(w, r)
	}
}

func writeObjectMetadata(w http.ResponseWriter, name string, generation int64, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind": "storage#object", "id": "private-bucket/" + name + "/" + strconv.FormatInt(generation, 10),
		"bucket": "private-bucket", "name": name, "generation": strconv.FormatInt(generation, 10),
		"metageneration": "1", "size": strconv.Itoa(len(data)), "contentType": "application/json", "crc32c": base64.StdEncoding.EncodeToString(checksum),
		"timeCreated": time.Now().UTC().Format(time.RFC3339Nano), "updated": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func newTestReplayRegistry(t *testing.T, endpoint string) *ReplayRegistry {
	t.Helper()
	client, err := storage.NewClient(context.Background(), option.WithEndpoint(endpoint), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return newReplayRegistryWithClient(client, "private-bucket", "privacy")
}

func replayEntry(executionID, requestID string) privacy.SuppressionReplayEntry {
	return privacy.SuppressionReplayEntry{
		ExecutionID: executionID,
		RequestID:   requestID,
		AccountID:   "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		RequestedAt: time.Date(2026, 10, 18, 1, 0, 0, 0, time.UTC),
		CompletedAt: time.Date(2026, 10, 18, 2, 0, 0, 0, time.UTC),
	}
}

func TestReplayRegistryCASPreventsConcurrentInstancesFromLosingEntries(t *testing.T) {
	serverState := &replayObjectServer{}
	server := httptest.NewServer(serverState)
	defer server.Close()
	first := newTestReplayRegistry(t, server.URL)
	second := newTestReplayRegistry(t, server.URL)
	ctx := context.Background()

	firstSnapshot, err := first.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	secondSnapshot, err := second.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	one := replayEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111")
	two := replayEntry("cccccccc-cccc-4ccc-8ccc-cccccccccccc", "22222222-2222-4222-8222-222222222222")
	manifestOne := privacy.SuppressionReplayManifest{Version: 1, Entries: []privacy.SuppressionReplayEntry{one}}
	if _, err := first.CompareAndSwap(ctx, firstSnapshot.Revision, manifestOne); err != nil {
		t.Fatalf("first CAS: %v requests=%v errors=%v", err, serverState.requests, serverState.serverErrors)
	}
	manifestTwo := privacy.SuppressionReplayManifest{Version: 1, Entries: []privacy.SuppressionReplayEntry{two}}
	if _, err := second.CompareAndSwap(ctx, secondSnapshot.Revision, manifestTwo); err != privacy.ErrReplayStoreConflict {
		t.Fatalf("stale generation CAS error=%v, want conflict", err)
	}
	latest, err := second.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := privacy.NormalizeSuppressionReplayManifest(privacy.SuppressionReplayManifest{Version: 1, Entries: append(latest.Manifest.Entries, two)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := second.CompareAndSwap(ctx, latest.Revision, merged); err != nil {
		t.Fatal(err)
	}
	final, err := first.Load(ctx)
	if err != nil || len(final.Manifest.Entries) != 2 || final.Manifest.Entries[0].ExecutionID == "" {
		t.Fatalf("final manifest=%+v err=%v", final.Manifest, err)
	}
}

func TestReplayRegistryWriteResponseLossIsVisibleAsUncertainGeneration(t *testing.T) {
	serverState := &replayObjectServer{loseNext: true}
	server := httptest.NewServer(serverState)
	defer server.Close()
	registry := newTestReplayRegistry(t, server.URL)
	entry := replayEntry("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "11111111-1111-4111-8111-111111111111")
	manifest := privacy.SuppressionReplayManifest{Version: 1, Entries: []privacy.SuppressionReplayEntry{entry}}
	_, err := registry.CompareAndSwap(context.Background(), "", manifest)
	if !errors.Is(err, privacy.ErrReplayStoreUnavailable) && !errors.Is(err, privacy.ErrReplayStoreConflict) {
		t.Fatalf("CAS after committed response loss returned %v, want uncertain result; requests=%v", err, serverState.requests)
	}
	snapshot, err := registry.Load(context.Background())
	if err != nil || len(snapshot.Manifest.Entries) != 1 || snapshot.Manifest.Entries[0] != entry {
		t.Fatalf("committed generation not visible after lost response: snapshot=%+v err=%v requests=%v errors=%v", snapshot, err, serverState.requests, serverState.serverErrors)
	}
}

func TestReplayRegistryRejectsMalformedAndOversizedManifests(t *testing.T) {
	for _, manifest := range []privacy.SuppressionReplayManifest{
		{Version: 2, Entries: []privacy.SuppressionReplayEntry{}},
		{Version: 1, Entries: []privacy.SuppressionReplayEntry{replayEntry("not-a-uuid", "11111111-1111-4111-8111-111111111111")}},
	} {
		if _, err := decodeReplayManifest(mustJSON(t, manifest)); err == nil {
			t.Fatalf("accepted malformed manifest: %+v", manifest)
		}
	}
	if err := validatePrefix("../outside"); err == nil {
		t.Fatal("invalid prefix accepted")
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
