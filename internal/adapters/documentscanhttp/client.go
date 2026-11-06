package documentscanhttp

import (
	"bytes"
	"context"

	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
)

const maxClientBytes int64 = 10 << 20

type ClientConfig struct {
	Origin     string
	HTTPClient *http.Client
	MaxBytes   int64
	MaxAge     time.Duration
	Now        func() time.Time
	Timeout    time.Duration
}
type Client struct {
	origin string
	http   *http.Client
	max    int64
	age    time.Duration
	now    func() time.Time
}

func NewClient(c ClientConfig) (*Client, error) {
	u, e := url.Parse(c.Origin)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || strings.Contains(c.Origin, "#") || u.Path != "" || u.RawPath != "" || c.MaxBytes <= 0 || c.MaxBytes > maxClientBytes || c.MaxAge <= 0 || c.MaxAge > maxSignatureAge || c.Timeout < 0 || c.HTTPClient == nil {
		return nil, errors.New("documentscanhttp: invalid configuration")
	}
	if c.Timeout <= 0 && c.HTTPClient.Timeout <= 0 {
		return nil, errors.New("documentscanhttp: finite timeout required")
	}
	if c.Now == nil {
		c.Now = func() time.Time { return time.Now().UTC() }
	}
	hc := *c.HTTPClient
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if c.Timeout > 0 {
		hc.Timeout = c.Timeout
	}
	return &Client{origin: strings.TrimSuffix(c.Origin, "/"), http: &hc, max: c.MaxBytes, age: c.MaxAge, now: c.Now}, nil
}

func (c *Client) ScanReaderDetailed(ctx context.Context, r io.Reader) (clamav.ScanResult, error) {
	bad := func() (clamav.ScanResult, error) {
		return clamav.ScanResult{}, errors.New("documentscanhttp: scan unavailable")
	}
	if c == nil || ctx == nil || r == nil || ctx.Err() != nil {
		return bad()
	}
	// LimitReader's max+1 is safe because configuration is bounded to 10 MiB.
	b, e := io.ReadAll(io.LimitReader(scanInputReader{ctx: ctx, reader: r}, c.max+1))
	if e != nil || int64(len(b)) > c.max || len(b) == 0 {
		return bad()
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, c.origin+"/v1/scan", bytes.NewReader(b))
	if e != nil {
		return bad()
	}
	// Retain a single upload intent even on a reused transport connection.
	req.GetBody = nil
	req.Header.Set("Content-Type", "application/octet-stream")
	resp, e := c.http.Do(req)
	if e != nil {
		return bad()
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "application/json" {
		return bad()
	}
	data, e := io.ReadAll(io.LimitReader(resp.Body, 8193))
	if e != nil || len(data) > 8192 {
		return bad()
	}
	wire, e := decodeScanResponse(data)
	if e != nil {
		return bad()
	}
	result := clamav.ScanResult{State: wire.State, ScannerVersion: wire.ScannerVersion, SignaturesAt: wire.SignaturesAt}
	if clamav.ValidateScanResultMetadata(result) != nil {
		return bad()
	}
	now := c.now().UTC()
	t := result.SignaturesAt.UTC()
	if t.After(now.Add(5*time.Minute)) || now.Sub(t) > c.age {
		return bad()
	}
	return result, nil
}

// The worker supplies an in-memory reader. Cancellation is checked at each
// read boundary; an arbitrary Reader that never returns cannot be interrupted.
type scanInputReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r scanInputReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.reader.Read(p)
	if cancelled := r.ctx.Err(); cancelled != nil {
		return 0, cancelled
	}
	return n, err
}
