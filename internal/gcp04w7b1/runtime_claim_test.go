package gcp04w7b1

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/option"
)

func testRuntimeEnvelope() RuntimeRelease {
	return RuntimeRelease{
		Release: Release{Decision: "approved", Stage: "b1_probe", ReleaseID: "b1-run-1", ResourceReleaseID: "b1-run-1", Reviewer: "HernanEspinozaDev", ExpiresAt: time.Now().Add(10 * time.Minute), Target: ProjectID + "/" + Instance, Login: Bootstrap, Role: AdminRole, SecretID: SecretID, QuerySHA256: QuerySHA256, Runs: 1, MaxDurationSeconds: 120, SecretETag: "secret-etag", CreateOperationID: "create-op-1", Version: "7", VersionETag: "version-etag"},
		Pins:    RuntimePins{ImageDigest: "southamerica-west1-docker.pkg.dev/espacigo-testprod/espacigo-testprod/gcp04-w7-b1@sha256:" + strings.Repeat("a", 64), BinarySHA256: strings.Repeat("b", 64)},
	}
}

func TestParseRuntimeReleaseRejectsInvalidExpiryAndPinDrift(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(*RuntimeRelease)
		version string
		image   string
		binary  string
	}{
		{name: "expired", mutate: func(r *RuntimeRelease) { r.Release.ExpiresAt = time.Now().Add(-time.Minute) }, version: "7", image: testRuntimeEnvelope().Pins.ImageDigest, binary: strings.Repeat("b", 64)},
		{name: "secret version mismatch", version: "8", image: testRuntimeEnvelope().Pins.ImageDigest, binary: strings.Repeat("b", 64)},
		{name: "image drift", version: "7", image: "wrong", binary: strings.Repeat("b", 64)},
		{name: "binary drift", version: "7", image: testRuntimeEnvelope().Pins.ImageDigest, binary: strings.Repeat("c", 64)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := testRuntimeEnvelope()
			if tc.mutate != nil {
				tc.mutate(&env)
			}
			data, _ := json.Marshal(env)
			if _, err := ParseRuntimeRelease(data, ReleaseDigest(data), tc.version, tc.image, tc.binary, time.Now()); err == nil {
				t.Fatal("invalid runtime release accepted")
			}
		})
	}
	env := testRuntimeEnvelope()
	data, _ := json.Marshal(env)
	if _, err := ParseRuntimeRelease(data, strings.Repeat("0", 64), "7", env.Pins.ImageDigest, env.Pins.BinarySHA256, time.Now()); err == nil {
		t.Fatal("wrong release digest accepted")
	}
}

type markerServer struct {
	mu             sync.Mutex
	body           []byte
	created        int
	conflicts      int
	loseNext       bool
	requests       []string
	invalidRequest string
}

func (s *markerServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.Method+" "+r.URL.String())
	if r.Method != http.MethodPost || !strings.HasPrefix(r.URL.Path, "/upload/storage/v1/b/claim-bucket/o") || r.URL.Query().Get("name") != "w7/runs/b1-run-1/probe.once.json" || r.URL.Query().Get("ifGenerationMatch") != "0" {
		s.invalidRequest = r.Method + " " + r.URL.String()
		http.Error(w, "unexpected request", http.StatusBadRequest)
		return
	}
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/related" {
		http.Error(w, "expected multipart upload", http.StatusBadRequest)
		return
	}
	parts := multipart.NewReader(r.Body, params["boundary"])
	_, _ = parts.NextPart()
	part, err := parts.NextPart()
	if err != nil {
		http.Error(w, "missing object body", http.StatusBadRequest)
		return
	}
	data, _ := io.ReadAll(part)
	if s.created > 0 {
		s.conflicts++
		http.Error(w, "object exists", http.StatusPreconditionFailed)
		return
	}
	s.created++
	s.body = append([]byte(nil), data...)
	if s.loseNext {
		s.loseNext = false
		http.Error(w, "response lost after commit", http.StatusServiceUnavailable)
		return
	}
	writeMarkerMetadata(w, data)
}

func writeMarkerMetadata(w http.ResponseWriter, data []byte) {
	checksum := make([]byte, 4)
	binary.BigEndian.PutUint32(checksum, crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"kind": "storage#object", "id": "claim-bucket/w7/runs/b1-run-1/probe.once.json/1",
		"bucket": "claim-bucket", "name": "w7/runs/b1-run-1/probe.once.json", "generation": "1",
		"metageneration": "1", "size": strconv.Itoa(len(data)), "contentType": "application/json",
		"crc32c": base64.StdEncoding.EncodeToString(checksum), "timeCreated": time.Now().UTC().Format(time.RFC3339Nano), "updated": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func newTestClaimStore(t *testing.T, endpoint string) *GCSClaimStore {
	t.Helper()
	client, err := storage.NewClient(context.Background(), option.WithEndpoint(endpoint), option.WithoutAuthentication())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return &GCSClaimStore{client: client, bucket: "claim-bucket", prefix: "w7/runs"}
}

func TestGCSClaimCreatesImmutableMarkerOnceAndBindsPins(t *testing.T) {
	state := &markerServer{}
	server := httptest.NewServer(state)
	defer server.Close()
	store := newTestClaimStore(t, server.URL)
	env := testRuntimeEnvelope()
	if err := store.Claim(context.Background(), env, strings.Repeat("c", 64), strings.Repeat("d", 64)); err != nil {
		t.Fatal(err)
	}
	if err := store.Claim(context.Background(), env, strings.Repeat("c", 64), strings.Repeat("d", 64)); !errors.Is(err, ErrRunAlreadyClaimed) {
		t.Fatalf("second claim=%v", err)
	}
	var marker RunClaim
	if err := json.Unmarshal(state.body, &marker); err != nil {
		t.Fatal(err)
	}
	if marker.ReleaseID != env.Release.ReleaseID || marker.ResourceReleaseID != env.Release.ResourceReleaseID || marker.SecretVersion != "7" || marker.QuerySHA256 != QuerySHA256 || marker.ImageDigest != env.Pins.ImageDigest || marker.BinarySHA256 != env.Pins.BinarySHA256 || marker.PlanSHA256 != strings.Repeat("d", 64) {
		t.Fatalf("claim did not bind exact release pins: %+v", marker)
	}
	if state.created != 1 || state.conflicts != 1 || state.invalidRequest != "" {
		t.Fatalf("unexpected server state: %+v", state)
	}
}

func TestGCSClaimLostResponseFailsClosedAndNextProcessCannotClaim(t *testing.T) {
	state := &markerServer{loseNext: true}
	server := httptest.NewServer(state)
	defer server.Close()
	store := newTestClaimStore(t, server.URL)
	if err := store.Create(context.Background(), "w7/runs/b1-run-1/probe.once.json", []byte(`{"claim":true}`)); !errors.Is(err, ErrRunClaimUncertain) {
		t.Fatalf("lost response error=%v", err)
	}
	if err := store.Create(context.Background(), "w7/runs/b1-run-1/probe.once.json", []byte(`{"claim":true}`)); !errors.Is(err, ErrRunAlreadyClaimed) {
		t.Fatalf("reconcile retry error=%v", err)
	}
	if state.created != 1 || state.conflicts != 1 || state.invalidRequest != "" {
		t.Fatalf("marker was overwritten or request widened: %+v", state)
	}
}

func TestGCSClaimSubprocess(t *testing.T) {
	if os.Getenv("GCP04_W7_CLAIM_CHILD") == "1" {
		store := newTestClaimStore(t, os.Getenv("GCP04_W7_CLAIM_ENDPOINT"))
		err := store.Create(context.Background(), "w7/runs/b1-run-1/probe.once.json", []byte(`{"claim":true}`))
		if err != nil && !errors.Is(err, ErrRunAlreadyClaimed) {
			t.Fatalf("child claim: %v", err)
		}
		return
	}
	state := &markerServer{}
	server := httptest.NewServer(state)
	defer server.Close()
	start := make(chan struct{})
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestGCSClaimSubprocess$")
			cmd.Env = append(os.Environ(), "GCP04_W7_CLAIM_CHILD=1", "GCP04_W7_CLAIM_ENDPOINT="+server.URL)
			<-start
			out, err := cmd.CombinedOutput()
			if err != nil {
				errs <- fmt.Errorf("subprocess: %w: %s", err, out)
				return
			}
			errs <- nil
		}()
	}
	close(start)
	for i := 0; i < 2; i++ {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if state.created != 1 || state.conflicts != 1 || state.invalidRequest != "" {
		t.Fatalf("two processes did not converge on one immutable claim: %+v", state)
	}
}
