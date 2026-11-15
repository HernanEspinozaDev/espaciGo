# Evidencia — VI-03, límites de seguridad y revisión de casos de espacio en reviewer cases

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e0c0344d567bb44cf96ea4f` dirty.
- **Alcance:** Caracterización exhaustiva de límites de seguridad de ruteo, autenticación y revisión de casos de espacio (`PurposeSpace`) en `internal/documentverification/transport/http/reviewer_cases.go`.
- **Pruebas implementadas:** `TestReviewerCasesHandlerSecurityBoundariesAndSpaceReview` en `internal/documentverification/transport/http/reviewer_cases_test.go`.

## Verificaciones cubiertas

1. **Formato de fecha civil para límites no alineados:** `observedExpiryDate` formatea directamente como fecha UTC (`2031-04-05`) ante límites no alineados con el inicio del día local.
2. **Disponibilidad fail-closed:** Receptor nil `(*ReviewerCasesHandler)(nil)` y dependencias nulas (`auth == nil` o `service == nil`) responden de forma inmediata `503 Service Unavailable`.
3. **Fallo de autenticación no prohibitivo:** Errores de autenticación distintos de `ErrForbidden` (ej. `ErrUnauthorized`) retornan `401 Unauthorized` acompañado de la cabecera `WWW-Authenticate: Bearer`.
4. **Prefijos de ruta no conformes:** Solicitudes con prefijo de URL sin delimitador de barra (`/api/v1/document-verification/reviewer-cases-suffix`) son rechazadas con `404 Not Found`.
5. **Métodos HTTP no permitidos en colección:** Solicitud `POST` sobre la ruta de colección responde `405 Method Not Allowed` junto a la cabecera `Allow: GET`.
6. **Revisión de casos de espacio (`PurposeSpace`):**
   - Manejo y propagación de errores de dominio de revisión mediante `ownerError`: `ErrConflict` (409), `ErrTransition` (404), `ErrNotFound` (404), `ErrCapability` (404), `ErrInvalid` (400), y fallos de almacenamiento (503).
   - Rechazo estricto con `400 Bad Request` si se intenta adjuntar un checklist de identidad (`identity_review_checklist`) a un caso de espacio.

## Resultados de ejecución local

- `go test -race -count=20 -run '^TestReviewerCasesHandlerSecurityBoundariesAndSpaceReview$' ./internal/documentverification/transport/http`: PASS.
- `go test -race -count=1 ./internal/documentverification/transport/http`: PASS.
- `go vet ./internal/documentverification/transport/http`: PASS.
- Cobertura de statements en `ReviewerCasesHandler.ServeHTTP`: subió a **100,0%**.
- Cobertura total del paquete `./internal/documentverification/transport/http`: subió a **91,4%**.

## Límites estrictos

- Entorno puramente unitario con fakes sintéticos en memoria.
- Sin llamadas SQL/DDL, sin acceso a bases persistentes ni Cloud, sin commit, push, deploy ni integración a `main`.
