package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

type Pinger interface {
	Ping(context.Context) error
}

func NewHandler(pinger Pinger, allowedOrigins []string) http.Handler {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/live", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeStatus(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		writeStatus(w, http.StatusOK, "live")
	})
	mux.HandleFunc("/health/ready", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method != http.MethodGet {
			writeStatus(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		if pinger == nil {
			writeStatus(w, http.StatusServiceUnavailable, "unavailable")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()
		if err := pinger.Ping(ctx); err != nil {
			writeStatus(w, http.StatusServiceUnavailable, "unavailable")
			return
		}
		writeStatus(w, http.StatusOK, "ready")
	})

	return cors(mux, origins)
}

func cors(next http.Handler, origins map[string]struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		if _, ok := origins[origin]; origin != "" && ok {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", http.MethodGet)
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}
		}
		next.ServeHTTP(w, r)
	})
}

func writeStatus(w http.ResponseWriter, status int, value string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
	}{Status: value})
}
