# Evidencia — VI-01 / VI-04, fronteras de transporte HTTP, CORS, Auth, JSON y mapeo de errores en verification/transport/http

- **Fecha:** 2026-11-14.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / commit `c9873fc4` (worktree local).
- **Identidad de Git verificada:** `HernanEspinozaDev` (autor y committer alineados con política del repositorio).
- **Modalidad:** Test-only local con fakes en memoria (`testAuth`, `mockAuthWithErr`, `failingRepoForHandler`, `evidenceRepoFake`, `evidenceStoreFake`). Cero persistencia PostgreSQL, cero SQL/DDL, sin llamadas a servicios Cloud, sin mutación de bases persistentes, sin commit, push, deploy ni integración a `main`.

## 1. Alcance cubierto

Se implementó la suite [handler_boundaries_test.go](file:///home/nandev/Documentos/projects/espaciGo-main-sync/internal/verification/transport/http/handler_boundaries_test.go) para caracterizar y blindar exhaustivamente las compuertas de transporte HTTP de `internal/verification/transport/http/handler.go`:

1. **Fronteras de CORS, OPTIONS, Query Params y Rutas Retiradas (`TestHandlerCORSAndQueryBoundaries`):**
   - Origen permitido (`http://localhost:4200`): emisión de cabeceras `Access-Control-Allow-Origin`, `Access-Control-Allow-Headers` y `Access-Control-Allow-Methods` (`GET, POST, DELETE, OPTIONS`).
   - Método `OPTIONS`: retorno estricto de `204 No Content`.
   - Origen no permitido (`http://malicious-origin.com`): rechazo fail-closed con `403 Forbidden` (`origin_denied`).
   - Rechazo de query parameters (`r.URL.RawQuery != ""`): respuesta con `400 Bad Request` (`invalid_request`) indicando «No se admiten parámetros de URL».
   - Rutas de proveedores externos retirados (Didit webhooks e intents): respuesta fail-closed con `404 Not Found` en todas sus variantes.

2. **Fronteras de autenticación y autorización (`TestHandlerAuthBoundaries`):**
   - Rechazo fail-closed (`401 Unauthorized`) ante cabecera `Authorization` ausente o con formato no Bearer (`Token 123`, `Bearer` sin token o más de dos tokens).
   - Denegación de acceso (`403 Forbidden`) a endpoints administrativos (`/api/v1/admin/verifications`) cuando el principal carece del rol `administrador`.
   - Propagación de fallos genéricos del autenticador como `401 Unauthorized`.

3. **Fronteras de decodificación y parseo JSON (`TestHandlerDecodeAndJSONBoundaries`):**
   - Rechazo con `415 Unsupported Media Type` ante `Content-Type` distinto de `application/json` (ej. `text/plain`).
   - Rechazo con `400 Bad Request` (`invalid_request`) ante JSON sintácticamente corrupto.
   - Rechazo con `400 Bad Request` ante carga útil con múltiples objetos JSON concatenados («Solo se admite un objeto JSON»).

4. **Mapeo de errores de servicio y declaración no configurada (`TestHandlerServiceErrorMappingAndUnconfiguredDeclarationBoundaries`):**
   - Solicitud a `/api/v1/verifications/identity-declaration` cuando el servicio de declaración no está inyectado (`h.declaration == nil`): respuesta con `503 Service Unavailable` (`service_unavailable`).
   - Mapeo unívoco de errores en `serviceError`:
     - `verification.ErrInvalid` -> `422 Unprocessable Entity` (`invalid_input`).
     - `verification.ErrNotFound` -> `404 Not Found` (`not_found`).
     - `verification.ErrConflict` -> `409 Conflict` (`state_conflict`).
     - Error genérico/desconocido -> `500 Internal Server Error` (`internal_error`).
   - Propagación de fallos de repositorio como `500 Internal Server Error` en `/api/v1/verifications` (`ListOwn`), `/api/v1/admin/verifications` (`Pending`) y `/api/v1/admin/verifications/failed` (`Rejected`).

5. **Validación de subida de evidencia (`TestHandlerEvidenceUploadValidationBoundaries`):**
   - Rechazo con `400 Bad Request` ante cuerpo JSON inválido al subir evidencia.
   - Rechazo con `422 Unprocessable Entity` (`invalid_input`) ante código de fixture no autorizado o inesperado.

## 2. Resultados de ejecución local

```bash
$ go test -race -v -run 'TestHandler.*Boundaries' ./internal/verification/transport/http
=== RUN   TestHandlerCORSAndQueryBoundaries
--- PASS: TestHandlerCORSAndQueryBoundaries (0.00s)
=== RUN   TestHandlerAuthBoundaries
--- PASS: TestHandlerAuthBoundaries (0.00s)
=== RUN   TestHandlerDecodeAndJSONBoundaries
--- PASS: TestHandlerDecodeAndJSONBoundaries (0.00s)
=== RUN   TestHandlerServiceErrorMappingAndUnconfiguredDeclarationBoundaries
--- PASS: TestHandlerServiceErrorMappingAndUnconfiguredDeclarationBoundaries (0.00s)
=== RUN   TestHandlerEvidenceUploadValidationBoundaries
--- PASS: TestHandlerEvidenceUploadValidationBoundaries (0.00s)
PASS
ok      github.com/HernanEspinozaDev/espaciGo/internal/verification/transport/http  1.022s
```

Cobertura en funciones de `internal/verification/transport/http`:
- `NewHandler`: **100,0%**
- `NewHandlerWithDeclaration`: **100,0%**
- `NewHandlerWithDeclarationWithoutExternalProvider`: **100,0%**
- `buildHandler`: **100,0%**
- `isExternalProviderRoute`: **100,0%**
- `ServeHTTP`: **88,9%**
- `serveEvidence`: **68,7%**
- `writeEvidenceContent`: **100,0%**
- `authorize`: **100,0%**
- `decode`: **100,0%**
- `serviceError`: **100,0%**
- `writeError`: **100,0%**
- `writeJSON`: **100,0%**
- `handleSaveIdentityDeclaration`: **87,5%**
- `handleGetIdentityDeclaration`: **100,0%**
- `identityDeclarationResponse`: **100,0%**
- Cobertura global de statements en `internal/verification/transport/http`: **86,1%** (incremento desde 77,3%).

## 3. Conformidad con directivas y límites

- `git diff --check`: limpio.
- `go vet ./internal/verification/transport/http`: limpio.
- Cero DDL, cero mutación de PostgreSQL y cero llamadas a Cloud Console / APIs remotas.
- Mantiene abiertas las compuertas de persistencia real y migración DB para fases posteriores autorizadas.
