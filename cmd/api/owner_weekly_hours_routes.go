package main

import "net/http"

func registerOwnerWeeklyHoursRoutes(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/spaces/{spaceID}/weekly-hours", handler)
	mux.Handle("PUT /api/v1/spaces/{spaceID}/weekly-hours", handler)
	mux.Handle("OPTIONS /api/v1/spaces/{spaceID}/weekly-hours", handler)
}
