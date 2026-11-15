package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCommercialRentalFlowConfigurationGate(t *testing.T) {
	tests := []struct {
		name          string
		values        map[string]string
		profile       runtimeProfileName
		isolatedClock bool
		wantEnabled   bool
		wantErr       bool
	}{
		{name: "disabled by default", profile: runtimeProfileStandard},
		{name: "loopback local prototype runtime", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_AUTH_PROTOTYPE": "1", "HTTP_ADDR": "127.0.0.1:18080", "DATABASE_CONNECTION_MODE": "direct", "DATABASE_HOST": "127.0.0.1"}, profile: runtimeProfileStandard, wantEnabled: true},
		{name: "rejects cloud test prototype authentication", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "APP_PROFILE": "cloud-test"}, profile: runtimeProfileStandard, wantErr: true},
		{name: "rejects non-loopback listener", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_AUTH_PROTOTYPE": "1", "HTTP_ADDR": "0.0.0.0:18080", "DATABASE_CONNECTION_MODE": "direct", "DATABASE_HOST": "127.0.0.1"}, profile: runtimeProfileStandard, wantErr: true},
		{name: "rejects Cloud SQL target for prototype authentication", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_AUTH_PROTOTYPE": "1", "HTTP_ADDR": "127.0.0.1:18080", "DATABASE_CONNECTION_MODE": "cloudsql", "DATABASE_HOST": "127.0.0.1"}, profile: runtimeProfileStandard, wantErr: true},
		{name: "rejects non-loopback database target", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_AUTH_PROTOTYPE": "1", "HTTP_ADDR": "127.0.0.1:18080", "DATABASE_CONNECTION_MODE": "direct", "DATABASE_HOST": "10.0.0.8"}, profile: runtimeProfileStandard, wantErr: true},
		{name: "requires authentication", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1"}, profile: runtimeProfileStandard, wantErr: true},
		{name: "requires standard profile", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_AUTH_PROTOTYPE": "1"}, profile: runtimeProfileWorkerFree, wantErr: true},
		{name: "rejects isolated clock", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_AUTH_PROTOTYPE": "1"}, profile: runtimeProfileStandard, isolatedClock: true, wantErr: true},
		{name: "separate from local booking trial", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "1", "LOCAL_BOOKING_TRIAL": "1", "LOCAL_AUTH_PROTOTYPE": "1"}, profile: runtimeProfileStandard, wantErr: true},
		{name: "rejects other truthy values", values: map[string]string{"COMMERCIAL_RENTAL_FLOW_ENABLED": "true"}, profile: runtimeProfileStandard, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			getenv := func(key string) string { return test.values[key] }
			authenticationEnabled, authErr := authPrototypeEnabled(getenv)
			if authErr != nil {
				t.Fatalf("authentication profile: %v", authErr)
			}
			got, err := commercialRentalFlowEnabled(getenv, test.profile, authenticationEnabled, test.isolatedClock)
			if (err != nil) != test.wantErr || got != test.wantEnabled {
				t.Fatalf("enabled=%v err=%v, want enabled=%v err=%v", got, err, test.wantEnabled, test.wantErr)
			}
		})
	}
}

func TestMountCommercialRentalFlowRoutes(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/api/v1/spaces/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})
	if err := mountCommercialRentalFlowRoutes(mux, handler); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/v1/spaces/space-1/rental-availability",
		"/api/v1/rental-quotes",
		"/api/v1/rental-quotes/quote-1/contract-preview",
	} {
		response := httptest.NewRecorder()
		mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusAccepted {
			t.Errorf("%s returned %d, want mounted commercial handler", path, response.Code)
		}
	}
}
