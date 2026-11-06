package documentscanhttp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type serverScanner struct {
	called bool
	result clamav.ScanResult
	err    error
}

func (s *serverScanner) ScanReaderDetailed(context.Context, io.Reader) (clamav.ScanResult, error) {
	s.called = true
	return s.result, s.err
}
func validServerResult() clamav.ScanResult {
	at := time.Date(2026, time.November, 6, 0, 0, 0, 0, time.UTC)
	return clamav.ScanResult{State: dv.FileClean, ScannerVersion: "ClamAV 1.0 / daily / Fri Nov 6 00:00:00 2026", SignaturesAt: at}
}
func serverHandler(s *serverScanner) http.Handler {
	return NewServer(ServerConfig{Scanner: s, Authorize: func(*http.Request) bool { return true }, MaxBytes: 1024, Timeout: time.Second, Now: func() time.Time { return time.Date(2026, time.November, 6, 0, 1, 0, 0, time.UTC) }})
}
func TestServerRejectsEmptyBodyWithoutScanning(t *testing.T) {
	s := &serverScanner{result: validServerResult()}
	r := httptest.NewRequest(http.MethodPost, "/v1/scan", nil)
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	serverHandler(s).ServeHTTP(deadlineRecorder{w}, r)
	if w.Code != http.StatusBadRequest || s.called {
		t.Fatalf("status=%d scanner called=%t", w.Code, s.called)
	}
}
func TestServerEncodesDeclaredWireNames(t *testing.T) {
	s := &serverScanner{result: validServerResult()}
	r := httptest.NewRequest(http.MethodPost, "/v1/scan", io.NopCloser(bytes.NewReader([]byte("x"))))
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	serverHandler(s).ServeHTTP(deadlineRecorder{w}, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"scanner_version"`) || strings.Contains(w.Body.String(), `"ScannerVersion"`) {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}
