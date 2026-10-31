package main

import (
	"net/http"

	documentverificationhttp "github.com/HernanEspinozaDev/espaciGo/internal/documentverification/transport/http"
)

func registerDocumentVerificationRoutes(mux *http.ServeMux, cases, downloads http.Handler) {
	if cases != nil {
		documentverificationhttp.RegisterOwnerCasesRoutes(mux, cases)
	}
	if downloads != nil {
		documentverificationhttp.RegisterOwnerDownloadRoute(mux, downloads)
	}
}
