package main

import (
	"errors"
	"net"
	"net/http"
)

func mountCommercialRentalFlowRoutes(mux *http.ServeMux, handler http.Handler) error {
	if mux == nil || handler == nil {
		return errors.New("commercial rental route dependencies are unavailable")
	}
	mux.Handle("/api/v1/spaces/{spaceID}/rental-availability", handler)
	mux.Handle("/api/v1/rental-quotes", handler)
	mux.Handle("/api/v1/rental-quotes/", handler)
	return nil
}

func commercialRentalFlowEnabled(getenv func(string) string, profile runtimeProfileName, authenticationEnabled, isolatedDatasetClock bool) (bool, error) {
	if getenv == nil {
		return false, errors.New("commercial rental flow configuration is unavailable")
	}
	value := getenv("COMMERCIAL_RENTAL_FLOW_ENABLED")
	if value == "" {
		return false, nil
	}
	if value != "1" {
		return false, errors.New("COMMERCIAL_RENTAL_FLOW_ENABLED must be 1 when configured")
	}
	if getenv("APP_PROFILE") != "" {
		return false, errors.New("commercial rental flow cannot use a cloud prototype authentication profile")
	}
	if !authenticationEnabled || getenv("LOCAL_AUTH_PROTOTYPE") != "1" || !commercialRentalFlowLocalRuntime(getenv) {
		return false, errors.New("commercial rental flow requires an explicit loopback local prototype runtime")
	}
	if profile != runtimeProfileStandard || isolatedDatasetClock {
		return false, errors.New("commercial rental flow requires the standard runtime profile and production clock")
	}
	if getenv("LOCAL_BOOKING_TRIAL") == "1" {
		return false, errors.New("commercial rental flow cannot be combined with the local booking trial")
	}
	return true, nil
}

func commercialRentalFlowLocalRuntime(getenv func(string) string) bool {
	if getenv("DATABASE_CONNECTION_MODE") != "direct" || !isLoopbackIP(getenv("DATABASE_HOST")) {
		return false
	}
	httpAddress := getenv("HTTP_ADDR")
	host, _, err := net.SplitHostPort(httpAddress)
	return err == nil && httpAddress != "" && isLoopbackIP(host)
}

func isLoopbackIP(value string) bool {
	ip := net.ParseIP(value)
	return ip != nil && ip.IsLoopback()
}
