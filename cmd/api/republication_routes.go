package main

import (
	"errors"
	"net/http"
)

// Local republication deliberately does not mount the booking/payment/contract
// router and cannot be enabled outside the validated loopback worker-free profile.
func localRepublicationEnabled(getenv func(string) string, profile runtimeProfileName) (bool, error) {
	flag := getenv("LOCAL_REPUBLICATION_TRIAL")
	if flag == "" {
		return false, nil
	}
	if flag != "1" || profile != runtimeProfileWorkerFree || getenv("LOCAL_BOOKING_TRIAL") != "" {
		return false, errors.New("local republication requires the standalone worker-free trial")
	}
	return true, nil
}
func registerLocalRepublicationRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/local/booking-trial/spaces/{spaceID}/returns", handler)
	mux.Handle("GET /api/v1/local/booking-trial/reservations/{reservationID}/operations", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/check-out", handler)
	mux.Handle("POST /api/v1/local/booking-trial/reservations/{reservationID}/reception", handler)
}
