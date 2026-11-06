package documentscanhttp

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
)

type unwrappingWriter struct{ http.ResponseWriter }

func (w unwrappingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func TestServerOversizedPartialTLSBodyDoesNotDrainWithoutDeadline(t *testing.T) {
	var scans atomic.Int64
	cfg := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) {
		scans.Add(1)
		return validServerResult(), nil
	}))
	cfg.Timeout = 50 * time.Millisecond
	handler := NewServer(cfg)
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { handler.ServeHTTP(unwrappingWriter{w}, r) }))
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	conn, err := tls.Dial("tcp", u.Host, s.Client().Transport.(*http.Transport).TLSClientConfig.Clone())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	// The peer advertises more than the cap, sends cap+1, then withholds the rest.
	if _, err := fmt.Fprintf(conn, "POST /v1/scan HTTP/1.1\r\nHost: %s\r\nContent-Type: application/octet-stream\r\nContent-Length: 100\r\n\r\n%s", u.Host, strings.Repeat("x", 11)); err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "POST"})
	if err != nil {
		t.Fatalf("oversized partial body did not finish within budget: %v", err)
	}
	defer response.Body.Close()
	if _, err := io.Copy(io.Discard, response.Body); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode < 400 || scans.Load() != 0 || time.Since(started) > 500*time.Millisecond {
		t.Fatalf("status=%d scans=%d elapsed=%v", response.StatusCode, scans.Load(), time.Since(started))
	}
}
