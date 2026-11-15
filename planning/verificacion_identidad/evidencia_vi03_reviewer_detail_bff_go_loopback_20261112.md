# Evidencia VI-03 — detalle reviewer BFF→Go loopback (2026-11-12)

## Alcance

Caracterización local del `GET` de detalle exportado por la ruta BFF de Next hacia un `httptest.NewServer` loopback con `http.ServeMux` y `RegisterReviewerCasesRoutes`. El camino Go alcanza el `ReviewerCasesHandler`, `ReviewerAccessService.LoadDetail` y `documentverification.Service` reales. Se reemplaza solo la configuración BFF del upstream por la URL loopback y se transpila el allowlist generado. Authenticator, reader/repository con grant sintético, declaration reader y document store son fakes.

Archivos nuevos:
- `internal/documentverification/transport/http/reviewer_detail_bff_go_integration_test.go`
- `frontend/tooling/reviewer-detail-bff-go-integration.mjs`

## Aserciones

- Exactamente una solicitud upstream `GET /api/v1/document-verification/reviewer/cases/{caseID}` con Bearer sintético; sin Cookie, body ni Content-Type.
- Authenticator fake: una llamada, rol vacío, `UserOperation`. El reader/repository fake observa dos lecturas para el actor y caso esperados con un grant identity de scope sintético exacto; entre ellas, el declaration reader se invoca una vez con subject/revisión sintéticos. `ReviewerAccessService.LoadDetail` y handler revalidan el estado actual alrededor de esa lectura.
- Cero lecturas del file store; esta ruta devuelve metadatos DTO, no bytes de evidencia.
- BFF responde 200, `ETag: "5"`, `Cache-Control: private, no-store`, sin Set-Cookie. DTO top-level exacto: `id`, `purpose`, `state`, `revision`, `document_format_declared`, `files`, `identity_declaration`; la declaración contiene exactamente `first_name`, `last_name`, `rut`, `date_of_birth`. Valores del fixture sintéticos; se verifica ausencia de subject ID, declaration revision ID/campo y storage/object keys/signed URL.

## Verificación parent-owned

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFGetsReviewerDetailFromRegisteredGoHandler$' -v` — PASS, 1/1; el subtest Node reporta 1/1.
- Suites: mismo entorno aislado con `go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `node --check tooling/reviewer-detail-bff-go-integration.mjs` y `node_modules/.bin/prettier --check tooling/reviewer-detail-bff-go-integration.mjs` — PASS.
- `gofmt -d` del test Go vacío y comprobación explícita de whitespace de ambos archivos — PASS. Ambos archivos fuente siguen untracked; `git diff --check` no los incluye, por lo que no se presenta como gate de esos archivos.
- Revisión independiente source-only `deleg_1621c063`: `passed: true`, `security_concerns: []`, `logic_errors: []`. No halló defectos concretos. Sugirió conservar el límite fake-only (ya indicado) y precisar que el caso no prueba el forwarding/rechazo de cualquier encabezado entrante; el claim aquí se limita a los headers efectivamente observados en esta ejecución. La revisión no repitió pruebas.

## Límites

El grant, autenticación, declaración y snapshot de asignación son sintéticos; no se prueba asignación/grants persistidos, predicados SQL, autorización efectiva ni semántica PostgreSQL. Tampoco se prueba runtime/standalone/middleware de Next, transacción/atomicidad/CAS, persistencia/reinicio, storage/scanner, GCS/Cloud o datos/personas reales. Sin cambios productivos, SQL/DDL, conexión a bases, proveedores, credenciales, commit, push, integración a `main` o deploy. Caracterización; no se afirma RED/GREEN.
