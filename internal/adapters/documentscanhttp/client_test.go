package documentscanhttp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func TestClientScanUsesTLSAndAcceptsFreshDetailedResult(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	version := "ClamAV 1.0/DB/" + now.Format("Mon Jan 2 15:04:05 2006")
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/scan" || r.Header.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("unexpected request: %s %s %s", r.Method, r.URL.Path, r.Header.Get("Content-Type"))
		}
		b, _ := io.ReadAll(r.Body)
		if string(b) != "synthetic" {
			t.Errorf("body mismatch")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ScanResponse{State: dv.FileClean, ScannerVersion: version, SignaturesAt: now})
	}))
	defer srv.Close()
	roots := x509.NewCertPool()
	roots.AddCert(srv.Certificate())
	client := srv.Client()
	client.Transport = &http.Transport{TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}}
	a, err := NewClient(ClientConfig{Origin: srv.URL, HTTPClient: client, Timeout: time.Second, MaxBytes: 100, MaxAge: time.Hour, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	got, err := a.ScanReaderDetailed(context.Background(), strings.NewReader("synthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if got.State != dv.FileClean || got.ScannerVersion != version {
		t.Fatalf("unexpected result: %+v", got)
	}
	if err := clamav.ValidateScanResultMetadata(got); err != nil {
		t.Fatal(err)
	}
}

func TestServerDeniesBeforeReadingOrScanning(t *testing.T) {
	called := false
	h := NewServer(ServerConfig{Scanner: scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) {
		called = true
		return clamav.ScanResult{}, nil
	}), Authorize: func(*http.Request) bool { return false }, MaxBytes: 20, Timeout: time.Second})
	req := httptest.NewRequest(http.MethodPost, "/v1/scan", &countReader{remaining: []byte("private")})
	req.Header.Set("Content-Type", "application/octet-stream")
	rw := httptest.NewRecorder()
	h.ServeHTTP(rw, req)
	if rw.Code != http.StatusUnauthorized || called {
		t.Fatalf("status=%d scanner called=%v", rw.Code, called)
	}
}

type scanFunc func(context.Context, io.Reader) (clamav.ScanResult, error)

func (f scanFunc) ScanReaderDetailed(c context.Context, r io.Reader) (clamav.ScanResult, error) {
	return f(c, r)
}

type countReader struct{ remaining []byte }

func (r *countReader) Read(p []byte) (int, error) {
	if r.remaining == nil {
		return 0, io.EOF
	}
	n := copy(p, r.remaining)
	r.remaining = r.remaining[n:]
	return n, nil
}

func TestClientRejectsNonCanonicalOrigin(t *testing.T) {
	for _, raw := range []string{"http://scanner.example", "https://user:pass@scanner.example", "https://scanner.example/?x=1", "https://scanner.example/#x"} {
		u, _ := url.Parse(raw)
		if _, err := NewClient(ClientConfig{Origin: u.String(), HTTPClient: http.DefaultClient, MaxBytes: 1, MaxAge: time.Minute}); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
