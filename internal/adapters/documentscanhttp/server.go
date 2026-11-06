package documentscanhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/HernanEspinozaDev/espaciGo/internal/adapters/clamav"
)

type DetailedScanner interface {
	ScanReaderDetailed(context.Context, io.Reader) (clamav.ScanResult, error)
}
type Authorizer func(*http.Request) bool

type ServerConfig struct {
	Scanner   DetailedScanner
	Authorize Authorizer
	MaxBytes  int64
	Timeout   time.Duration
	MaxAge    time.Duration
	Now       func() time.Time
}

const (
	defaultMaxBytes int64 = 10 << 20
	defaultMaxAge         = maxSignatureAge
	maxAllowedAge         = maxSignatureAge
	maxFutureSkew         = 5 * time.Minute
)

// NewServer fails closed when configuration is invalid. Request body reads
// use ResponseController's connection read deadline; configure ReadHeaderTimeout
// and WriteTimeout on the owning http.Server as well.
func NewServer(c ServerConfig) http.Handler {
	if c.Scanner == nil || c.Authorize == nil || c.MaxBytes < 1 || c.MaxBytes > defaultMaxBytes || c.Timeout <= 0 || c.Timeout > time.Hour {
		return http.NotFoundHandler()
	}
	if c.MaxAge == 0 {
		c.MaxAge = defaultMaxAge
	}
	if c.MaxAge < 0 || c.MaxAge > maxAllowedAge {
		return http.NotFoundHandler()
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/scan" || r.URL.RawPath != "" || r.RequestURI != "/v1/scan" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !c.Authorize(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Content-Type") != "application/octet-stream" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), c.Timeout)
		defer cancel()
		deadline, _ := ctx.Deadline()
		rc := http.NewResponseController(w)
		if err := rc.SetReadDeadline(deadline); err != nil {
			http.Error(w, "scan unavailable", http.StatusServiceUnavailable)
			return
		}
		body, err := io.ReadAll(io.LimitReader(scanInputReader{ctx: ctx, reader: r.Body}, c.MaxBytes+1))
		if err != nil || ctx.Err() != nil || int64(len(body)) > c.MaxBytes || len(body) == 0 {
			// Do not clear the deadline and let net/http drain an incomplete
			// oversized body indefinitely after the handler returns.
			r.Close = true
			w.Header().Set("Connection", "close")
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		_ = rc.SetReadDeadline(time.Time{})
		result, err := c.Scanner.ScanReaderDetailed(ctx, bytes.NewReader(body))
		if err != nil || ctx.Err() != nil || clamav.ValidateScanResultMetadata(result) != nil {
			http.Error(w, "scan unavailable", http.StatusBadGateway)
			return
		}
		now := c.Now()
		if now.IsZero() || result.SignaturesAt.After(now.Add(maxFutureSkew)) || now.Sub(result.SignaturesAt) > c.MaxAge {
			http.Error(w, "scan unavailable", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ScanResponse{State: result.State, ScannerVersion: result.ScannerVersion, SignaturesAt: result.SignaturesAt})
	})
}
