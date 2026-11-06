package documentscanhttp

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
	dv "github.com/HernanEspinozaDev/espaciGo/internal/documentverification"
)

func clientFixture(at time.Time, state dv.FileState) string {
	b, _ := json.Marshal(ScanResponse{State: state, ScannerVersion: "ClamAV 1.0/DB/" + at.Format("Mon Jan 2 15:04:05 2006"), SignaturesAt: at})
	return string(b)
}
func strictClient(t *testing.T, s *httptest.Server, now time.Time, max int64) *Client {
	t.Helper()
	c, err := NewClient(ClientConfig{Origin: s.URL, HTTPClient: s.Client(), MaxBytes: max, MaxAge: 48 * time.Hour, Timeout: time.Second, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func TestClientDetailedProtocolNegativeAndPositiveMatrix(t *testing.T) {
	now := time.Date(2026, 11, 6, 12, 0, 0, 0, time.UTC)
	clean := clientFixture(now, dv.FileClean)
	cases := []struct {
		name        string
		status      int
		media, body string
		want        dv.FileState
	}{
		{"clean", 200, "application/json", clean, dv.FileClean},
		{"malware", 200, "application/json", clientFixture(now, dv.FileBlocked), dv.FileBlocked},
		{"unauthorized", 401, "application/json", clean, ""}, {"forbidden", 403, "application/json", clean, ""}, {"unavailable", 500, "application/json", clean, ""},
		{"wrong-media", 200, "text/html", clean, ""}, {"malformed", 200, "application/json", "{", ""},
		{"trailing", 200, "application/json", clean + " {}", ""}, {"unknown-field", 200, "application/json", strings.TrimSuffix(clean, "}") + `,"unexpected":true}`, ""},
		{"oversized", 200, "application/json", strings.Repeat(" ", 8193) + clean, ""},
		{"stale", 200, "application/json", clientFixture(now.Add(-49*time.Hour), dv.FileClean), ""},
		{"future", 200, "application/json", clientFixture(now.Add(6*time.Minute), dv.FileClean), ""},
		{"inconsistent-date", 200, "application/json", strings.Replace(clean, `"signatures_at":"2026-11-06T12:00:00Z"`, `"signatures_at":"2026-11-06T11:59:00Z"`, 1), ""},
		{"invalid-state", 200, "application/json", strings.Replace(clean, `"state":"`+string(dv.FileClean)+`"`, `"state":"pending"`, 1), ""},
		{"duplicate-state", 200, "application/json", strings.TrimSuffix(clean, "}") + `,"state":"` + string(dv.FileClean) + `"}`, ""},
		{"noncanonical-field", 200, "application/json", strings.Replace(clean, `"state"`, `"State"`, 1), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				body, _ := io.ReadAll(r.Body)
				if string(body) != "synthetic" {
					t.Error("request body changed")
				}
				w.Header().Set("Content-Type", tc.media)
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer s.Close()
			got, err := strictClient(t, s, now, 100).ScanReaderDetailed(context.Background(), strings.NewReader("synthetic"))
			if tc.want != "" {
				if err != nil || got.State != tc.want {
					t.Fatalf("wanted %s, got %+v, %v", tc.want, got, err)
				}
			} else if err == nil || got != (clamav.ScanResult{}) {
				t.Fatalf("failed scan produced result %+v error %v", got, err)
			}
			if calls.Load() != 1 {
				t.Fatalf("attempts=%d", calls.Load())
			}
		})
	}
}

func TestClientRejectsUnboundedOrInvalidConfiguration(t *testing.T) {
	base := ClientConfig{Origin: "https://scanner.invalid", HTTPClient: &http.Client{}, MaxBytes: 10, MaxAge: time.Hour, Timeout: time.Second}
	tests := map[string]func(*ClientConfig){
		"no-timeout": func(c *ClientConfig) { c.Timeout = 0 }, "negative-timeout": func(c *ClientConfig) { c.Timeout = -1; c.HTTPClient = &http.Client{Timeout: time.Second} },
		"long-age": func(c *ClientConfig) { c.MaxAge = 49 * time.Hour }, "zero-age": func(c *ClientConfig) { c.MaxAge = 0 },
		"zero-size": func(c *ClientConfig) { c.MaxBytes = 0 }, "too-large": func(c *ClientConfig) { c.MaxBytes = (10 << 20) + 1 }, "overflow": func(c *ClientConfig) { c.MaxBytes = math.MaxInt64 },
		"force-query": func(c *ClientConfig) { c.Origin += "?" }, "empty-fragment": func(c *ClientConfig) { c.Origin += "#" }, "path": func(c *ClientConfig) { c.Origin += "/v1/scan" }, "escaped": func(c *ClientConfig) { c.Origin += "/%73can" }, "nil-http": func(c *ClientConfig) { c.HTTPClient = nil },
	}
	for name, change := range tests {
		t.Run(name, func(t *testing.T) {
			c := base
			change(&c)
			if _, err := NewClient(c); err == nil {
				t.Fatal("invalid config accepted")
			}
		})
	}
	base.Timeout = 0
	base.HTTPClient = &http.Client{Timeout: time.Second}
	if _, err := NewClient(base); err != nil {
		t.Fatalf("bounded inherited timeout rejected: %v", err)
	}
}

type errorInput struct{}

func (errorInput) Read([]byte) (int, error) { return 0, errors.New("synthetic input failure") }

type cancelInput struct {
	cancel context.CancelFunc
	calls  int
}

func (r *cancelInput) Read(b []byte) (int, error) {
	r.calls++
	if r.calls == 1 {
		b[0] = 'x'
		r.cancel()
		return 1, nil
	}
	return 0, errors.New("read after cancellation")
}
func TestClientInvalidInputNeverSends(t *testing.T) {
	var calls atomic.Int64
	now := time.Now().UTC().Truncate(time.Second)
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, clientFixture(now, dv.FileClean))
	}))
	defer s.Close()
	c := strictClient(t, s, now, 5)
	for _, r := range []io.Reader{nil, strings.NewReader(""), strings.NewReader("123456"), errorInput{}} {
		got, err := c.ScanReaderDetailed(context.Background(), r)
		if err == nil || got != (clamav.ScanResult{}) {
			t.Fatal("invalid input accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.ScanReaderDetailed(ctx, strings.NewReader("x")); err == nil {
		t.Fatal("cancelled accepted")
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	r := &cancelInput{cancel: cancel}
	if _, err := c.ScanReaderDetailed(ctx, r); err == nil {
		t.Fatal("input cancellation accepted")
	}
	if r.calls != 1 {
		t.Fatalf("reader called after cancellation: %d", r.calls)
	}
	if calls.Load() != 0 {
		t.Fatalf("invalid input dispatched %d requests", calls.Load())
	}
}
func TestClientNeverFollowsRedirectOrReplays(t *testing.T) {
	for _, status := range []int{307, 308} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var origin, destination atomic.Int64
			dest := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { destination.Add(1) }))
			defer dest.Close()
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				origin.Add(1)
				http.Redirect(w, r, dest.URL+"/v1/scan", status)
			}))
			defer s.Close()
			if _, err := strictClient(t, s, time.Now(), 100).ScanReaderDetailed(context.Background(), strings.NewReader("synthetic")); err == nil {
				t.Fatal("redirect accepted")
			}
			if origin.Load() != 1 || destination.Load() != 0 {
				t.Fatalf("origin=%d destination=%d", origin.Load(), destination.Load())
			}
		})
	}
}
func TestClientCancelsInFlightHTTP(t *testing.T) {
	entered := make(chan struct{})
	done := make(chan struct{})
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		close(entered)
		<-r.Context().Done()
		close(done)
	}))
	defer s.Close()
	c := strictClient(t, s, time.Now(), 100)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-entered:
			cancel()
		case <-ctx.Done():
		}
	}()
	if got, err := c.ScanReaderDetailed(ctx, strings.NewReader("x")); err == nil || got != (clamav.ScanResult{}) {
		t.Fatal("cancel returned success")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("HTTP handler did not terminate")
	}
}

type observingTransport func(*http.Request) (*http.Response, error)

func (f observingTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestClientDisablesRequestBodyReplayHook(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	calls := 0
	h := &http.Client{Transport: observingTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.GetBody != nil {
			t.Error("POST replay hook enabled")
		}
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(clientFixture(now, dv.FileClean)))}, nil
	})}
	c, err := NewClient(ClientConfig{Origin: "https://scanner.invalid", HTTPClient: h, MaxBytes: 10, MaxAge: time.Hour, Timeout: time.Second, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.ScanReaderDetailed(context.Background(), strings.NewReader("x")); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal("multiple attempts")
	}
}
