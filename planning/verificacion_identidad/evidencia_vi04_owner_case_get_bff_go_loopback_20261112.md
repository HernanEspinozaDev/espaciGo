# Evidencia VI-04 — GET del caso identity del titular por BFF→Go local

- **Fecha local:** 2026-11-12 (`America/Santiago`).
- **Alcance:** caracterización de transporte local, solo pruebas; sin cambios de producción.
- **Backend:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Frontend autorizado:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Archivos nuevos:** `internal/documentverification/transport/http/identity_owner_get_bff_go_integration_test.go` y `frontend/tooling/identity-owner-get-bff-go-integration.mjs`.

## Comportamiento caracterizado

`TestNextBFFGetsOwnIdentityCaseThroughRegisteredGoRoute` inicia un `httptest` loopback con `RegisterOwnerCasesRoutes`, `OwnerCasesHandler` y `documentverification.Service` reales. Autenticación y repositorio son fakes sintéticos en memoria. Un proceso Node transpila e invoca el export `GET` real de Next y la operación generada desde el frontend autorizado; el harness sustituye la config por la URL de loopback y reescribe dos imports hacia módulos sintéticos temporales, por lo que no cubre la resolución/build normal de Next. No usa el `frontend/` anidado en el checkout backend.

- Una solicitud `GET /api/v1/document-verification/cases/<UUID sintético>` llega a Go con Bearer sintético, sin Cookie, Content-Type ni body. El fake de auth recibe una llamada con actividad `UserOperation`; el repositorio recibe una lectura del titular/caso exactos y cero transiciones.
- Respuesta BFF: HTTP 200, `ETag: "8"`, `Cache-Control: private, no-store`, `Vary: Authorization`, sin `Set-Cookie`.
- DTO de titular con conjunto exacto de claves: `id`, `purpose`, `state`, `revision`, `document_format_declared`, `files`, `owner_decision_message`. Estado `requiere_antecedentes`, revisión 8 y formato `new`; copy allowlisted exacto: «Faltan antecedentes para continuar con la revisión. Revisa los documentos solicitados y completa lo que falta antes de enviar nuevamente.»
- Tres archivos sintéticos cubren `identity_front`, `identity_back` e `identity_selfie`; cada objeto contiene solo `id`, `evidence_role`, `state`, `size`, `mime` y `pages`.
- El fixture incluye deliberadamente metadata interna de declaración/consentimiento, revisor y fecha de vencimiento, además de UUIDs centinela; la respuesta la omite. Se verifica que no se serialicen subject ID, revisión de declaración, reviewer ID, auditoría, storage key ni `document_expires_at`.

## Verificación parent-owned

- Focal final: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=5 ./internal/documentverification/transport/http -run '^TestNextBFFGetsOwnIdentityCaseThroughRegisteredGoRoute$' -v` — PASS; 5/5 ejecuciones Go y Node 1/1 en cada una.
- Suites: mismo entorno aislado con `go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS en ambos paquetes.
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `node --check tooling/identity-owner-get-bff-go-integration.mjs` y `node_modules/.bin/prettier --check tooling/identity-owner-get-bff-go-integration.mjs` — PASS.
- `gofmt -d` sin diferencias; whitespace explícito en los cuatro archivos modificados/nuevos: 0 líneas con trailing whitespace. `git diff --check` se ejecutó, pero no incluye archivos untracked; para estos se usó la comprobación explícita de whitespace.

El primer intento del executor incluyó Cookie en el request de prueba y el BFF lo rechazó antes de upstream; se retiró Cookie del caso positivo para modelar la solicitud Bearer soportada. El rechazo de Cookie se caracteriza por separado en `evidencia_vi03_reviewer_download_bff_go_loopback_20261112.md`. La revisión source-only inicial `deleg_3a92c89b` encontró que el test capturaba metadatos de la solicitud sin sincronización; se añadió `sync.Mutex`, se protege la escritura y se copia la captura bajo lock antes de las aserciones. Los resultados focales anteriores a ese ajuste no se presentan como gate final. No se atribuye RED/GREEN a ajustes de setup ni se modificó producción.

## Revisión y límites

La revisión source-only inicial `deleg_3a92c89b` detectó el race y no aprobó esa versión; tras corregirlo, la re-revisión independiente `deleg_22f84634` aprobó (`passed: true`, sin concerns ni errores lógicos), sin ejecutar pruebas. La sugerencia no bloqueante de conservar `TMPDIR` aislado ya se cumple; las limitaciones de fake/runtime permanecen explícitas. Los fakes no prueban autenticación, ownership/grants efectivos, PostgreSQL, SQL, persistencia, concurrencia/atomicidad, Next standalone/runtime, UI de navegador, storage/scanner, GCS o Cloud. La caracterización usa solo UUIDs/valores sintéticos y dependencias locales; no ejecutó SQL/DDL ni accedió a bases persistentes o Cloud. Sin commit, push, deploy ni integración a `main`.
