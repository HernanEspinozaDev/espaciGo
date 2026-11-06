package documentscanhttp

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

type deadlineRecorder struct{ *httptest.ResponseRecorder }

func (w deadlineRecorder) SetReadDeadline(time.Time) error { return nil }

type trackedInput struct {
	calls int
	data  string
}

func (r *trackedInput) Read(p []byte) (int, error) {
	r.calls++
	if r.data == "" {
		return 0, io.EOF
	}
	n := copy(p, r.data)
	r.data = r.data[n:]
	return n, nil
}
func boundaryServerConfig(scanner DetailedScanner) ServerConfig {
	return ServerConfig{Scanner: scanner, Authorize: func(*http.Request) bool { return true }, MaxBytes: 10, Timeout: 100 * time.Millisecond, MaxAge: time.Hour, Now: func() time.Time { return validServerResult().SignaturesAt }}
}
func serverRequest(handler http.Handler, method, path, media, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", media)
	w := httptest.NewRecorder()
	handler.ServeHTTP(deadlineRecorder{w}, r)
	return w
}
func TestServerDenialDoesNotReadBody(t *testing.T) {
	calls := 0
	cfg := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) { calls++; return validServerResult(), nil }))
	cfg.Authorize = func(*http.Request) bool { return false }
	body := &trackedInput{data: "synthetic"}
	r := httptest.NewRequest("POST", "/v1/scan", body)
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	NewServer(cfg).ServeHTTP(w, r)
	if w.Code != 401 || body.calls != 0 || calls != 0 {
		t.Fatalf("status=%d reads=%d scans=%d", w.Code, body.calls, calls)
	}
}
func TestServerConfigurationAndRequestBoundaries(t *testing.T) {
	base := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) { return validServerResult(), nil }))
	changes := map[string]func(*ServerConfig){"nil-scanner": func(c *ServerConfig) { c.Scanner = nil }, "nil-auth": func(c *ServerConfig) { c.Authorize = nil }, "zero-bytes": func(c *ServerConfig) { c.MaxBytes = 0 }, "overflow": func(c *ServerConfig) { c.MaxBytes = math.MaxInt64 }, "zero-timeout": func(c *ServerConfig) { c.Timeout = 0 }, "negative-age": func(c *ServerConfig) { c.MaxAge = -1 }, "relaxed-age": func(c *ServerConfig) { c.MaxAge = 49 * time.Hour }}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			cfg := base
			change(&cfg)
			if w := serverRequest(NewServer(cfg), "POST", "/v1/scan", "application/octet-stream", "x"); w.Code < 400 {
				t.Fatalf("invalid config accepted: %d", w.Code)
			}
		})
	}
	cases := []struct {
		method, path, media, body string
		status                    int
	}{
		{"GET", "/v1/scan", "application/octet-stream", "x", 405}, {"POST", "/wrong", "application/octet-stream", "x", 404},
		{"POST", "/v1//scan", "application/octet-stream", "x", 404}, {"POST", "/v1/other/../scan", "application/octet-stream", "x", 404}, {"POST", "/v1/%73can", "application/octet-stream", "x", 404},
		{"POST", "/v1/scan?x=1", "application/octet-stream", "x", 404}, {"POST", "/v1/scan", "text/plain", "x", 400}, {"POST", "/v1/scan", "application/octet-stream", "", 400}, {"POST", "/v1/scan", "application/octet-stream", "12345678901", 400},
	}
	for _, tc := range cases {
		t.Run(tc.method+tc.path+tc.media+tc.body, func(t *testing.T) {
			calls := 0
			cfg := base
			cfg.Scanner = scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) { calls++; return validServerResult(), nil })
			w := serverRequest(NewServer(cfg), tc.method, tc.path, tc.media, tc.body)
			if w.Code != tc.status || calls != 0 || w.Header().Get("Location") != "" {
				t.Fatalf("status=%d scans=%d redirect=%s", w.Code, calls, w.Header().Get("Location"))
			}
		})
	}
}
func TestServerRejectsMissingReadDeadlineCapability(t *testing.T) {
	body := &trackedInput{data: "x"}
	calls := 0
	cfg := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) { calls++; return validServerResult(), nil }))
	r := httptest.NewRequest("POST", "/v1/scan", body)
	r.Header.Set("Content-Type", "application/octet-stream")
	w := httptest.NewRecorder()
	NewServer(cfg).ServeHTTP(w, r)
	if w.Code != 503 || body.calls != 0 || calls != 0 {
		t.Fatalf("unsupported deadline fell open: status=%d reads=%d scans=%d", w.Code, body.calls, calls)
	}
}
func TestServerResultsAndFreshness(t *testing.T) {
	now := validServerResult().SignaturesAt
	cases := []struct {
		name   string
		result clamav.ScanResult
		err    error
		status int
	}{
		{"clean", validServerResult(), nil, 200}, {"malware", clamav.ScanResult{State: dv.FileBlocked, ScannerVersion: validServerResult().ScannerVersion, SignaturesAt: now}, nil, 200},
		{"error", clamav.ScanResult{}, errors.New("private-scanner-message"), 502},
		{"malformed", clamav.ScanResult{State: dv.FileClean}, nil, 502},
	}
	for _, age := range []struct {
		name   string
		offset time.Duration
	}{{"stale", -49 * time.Hour}, {"future", 6 * time.Minute}} {
		at := now.Add(age.offset)
		cases = append(cases, struct {
			name   string
			result clamav.ScanResult
			err    error
			status int
		}{age.name, clamav.ScanResult{State: dv.FileClean, ScannerVersion: "ClamAV 1.0/DB/" + at.Format("Mon Jan 2 15:04:05 2006"), SignaturesAt: at}, nil, 502})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) { return tc.result, tc.err }))
			cfg.MaxAge = 0
			w := serverRequest(NewServer(cfg), "POST", "/v1/scan", "application/octet-stream", "x")
			if w.Code != tc.status || strings.Contains(w.Body.String(), "private-scanner-message") {
				t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
			}
			if w.Code == 200 {
				wire, err := decodeScanResponse(w.Body.Bytes())
				if err != nil || wire.State != tc.result.State {
					t.Fatal("invalid declared response")
				}
			}
		})
	}
}
func TestServerCancellationBeforeAndDuringBody(t *testing.T) {
	for _, during := range []bool{false, true} {
		t.Run(fmt.Sprint(during), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var input io.Reader
			tracked := &trackedInput{data: "x"}
			cancelled := &cancelInput{cancel: cancel}
			if during {
				input = cancelled
			} else {
				input = tracked
				cancel()
			}
			calls := 0
			cfg := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) { calls++; return validServerResult(), nil }))
			r := httptest.NewRequest("POST", "/v1/scan", input).WithContext(ctx)
			r.Header.Set("Content-Type", "application/octet-stream")
			w := httptest.NewRecorder()
			NewServer(cfg).ServeHTTP(deadlineRecorder{w}, r)
			if w.Code < 400 || calls != 0 {
				t.Fatal("cancelled request scanned")
			}
			if (!during && tracked.calls != 0) || (during && cancelled.calls != 1) {
				t.Fatal("body read after cancellation")
			}
		})
	}
}
func TestServerRejectsLateScannerSuccess(t *testing.T) {
	cfg := boundaryServerConfig(scanFunc(func(ctx context.Context, r io.Reader) (clamav.ScanResult, error) {
		<-ctx.Done()
		return validServerResult(), nil
	}))
	cfg.Timeout = time.Millisecond
	w := serverRequest(NewServer(cfg), "POST", "/v1/scan", "application/octet-stream", "x")
	if w.Code != 502 {
		t.Fatalf("late success status=%d", w.Code)
	}
}
func TestServerSlowTLSBodyHasRealReadDeadline(t *testing.T) {
	var scans atomic.Int64
	cfg := boundaryServerConfig(scanFunc(func(context.Context, io.Reader) (clamav.ScanResult, error) {
		scans.Add(1)
		return validServerResult(), nil
	}))
	cfg.Timeout = 50 * time.Millisecond
	s := httptest.NewUnstartedServer(NewServer(cfg))
	s.Config.ReadHeaderTimeout = time.Second
	s.Config.WriteTimeout = time.Second
	s.StartTLS()
	defer s.Close()
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}
	tlsConfig := s.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	conn, err := tls.Dial("tcp", u.Host, tlsConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	_, err = fmt.Fprintf(conn, "POST /v1/scan HTTP/1.1\r\nHost: %s\r\nContent-Type: application/octet-stream\r\nContent-Length: 10\r\nConnection: close\r\n\r\nx", u.Host)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: "POST"})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, response.Body)
	if response.StatusCode < 400 || scans.Load() != 0 || time.Since(started) > time.Second {
		t.Fatalf("slow body status=%d scans=%d elapsed=%v", response.StatusCode, scans.Load(), time.Since(started))
	}
}
