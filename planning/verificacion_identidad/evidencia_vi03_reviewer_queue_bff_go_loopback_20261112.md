# Evidencia VI-03 — cola reviewer BFF→Go loopback (2026-11-12)

## Alcance

Caracterización local del `GET` exportado por la ruta BFF de Next al `ReviewerCasesHandler` Go registrado en `http.NewServeMux`, pasando por `ReviewerAccessService.List` y `documentverification.Service`. Un `httptest.NewServer` sirve de upstream loopback. Se ejecuta el export TypeScript transpilado; en un directorio temporal se sustituye solo la configuración del upstream y se transpila el allowlist generado. Authenticator, reader/repository y document store usan fakes sintéticos.

Archivos nuevos:
- `internal/documentverification/transport/http/reviewer_queue_bff_go_integration_test.go`
- `frontend/tooling/reviewer-queue-bff-go-integration.mjs`

## Aserciones

- Una sola solicitud upstream `GET /api/v1/document-verification/reviewer/cases`, con `Bearer synthetic-reviewer-session`; sin Cookie, body ni Content-Type. El autenticador fake observa una llamada con rol vacío y actividad `UserOperation`; el reader fake recibe una llamada con el actor reviewer sintético.
- La lista fake incluye un caso identity accionable, un self-review y un caso terminal aprobado. El servicio real conserva solo el caso actionable del fixture y omite self-review/terminal; esto prueba proyección en memoria, no autorización o grants reales.
- BFF devuelve 200 JSON con `Cache-Control: private, no-store`, sin Set-Cookie, `{items}` exacto y un DTO identity de seis claves: `id`, `purpose`, `state`, `revision`, `document_format_declared`, `files`. Se comprueban los valores sintéticos, `files: []` y la ausencia de subject/legal declaration/revision, consentimiento y storage metadata.

## Verificación ejecutada por el padre

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFGetsReviewerQueueFromRegisteredGoHandler$' -v` — PASS, 1/1.
- Suites: mismo entorno aislado con `go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `node --check tooling/reviewer-queue-bff-go-integration.mjs` y `node_modules/.bin/prettier --check tooling/reviewer-queue-bff-go-integration.mjs` — PASS.
- `gofmt -d` del test Go vacío y checks explícitos de whitespace de ambos archivos — PASS. Los archivos nuevos siguen untracked; `git diff --check` no los incluye hasta stagearlos.
- La revisión independiente `deleg_86e2acec` aprobó (`passed: true`) sin errores lógicos ni defectos concretos. Su aclaración read-only `deleg_4e5fd9bc` confirmó que el límite fake-only es una limitación de evidencia, no un defecto: devolvió `passed: true`, listas `security_concerns`/`logic_errors` vacías y mantuvo ese límite como sugerencia. Revisión source-only; no repitió pruebas.

## Límites

Es evidencia sintética de transporte por el export BFF y handler Go reales dentro de un harness local. El `ReviewerCaseReader` fake representa resultados ya asignados: no acredita grants exactos/vigentes, autorización efectiva, SQL/predicados, CAS, atomicidad, PostgreSQL, persistencia/reinicio, Next standalone/runtime/middleware, scanner/storage, Cloud ni datos/personas reales. Sin cambios productivos, SQL/DDL, conexiones a bases, commit, push, integración a `main` o deploy. Caracterización; no se afirma RED/GREEN.
