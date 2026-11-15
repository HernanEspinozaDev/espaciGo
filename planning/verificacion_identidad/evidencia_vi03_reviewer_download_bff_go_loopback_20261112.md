# Evidencia VI-03 — descarga reviewer BFF→Go por loopback

Fecha local (`America/Santiago`): 2026-11-12

Backend: `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.

Frontend: `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.

## Alcance

Se agregaron exclusivamente estos tests, sin cambios productivos:

- `internal/documentverification/transport/http/reviewer_download_bff_go_integration_test.go`
- `frontend/tooling/reviewer-download-bff-go-integration.mjs`

La caracterización llama el export `GET` del BFF Next y la operación generada `downloadAssignedCleanDocument`; el launcher Go recibe `ESPACIGO_FRONTEND_ROOT`, fija el directorio del proceso Node a esa raíz y ejecuta el script desde el worktree autorizado `frontend-next-hermes` (HEAD indicado arriba). No evalúa otra copia de `frontend/` anidada en el checkout backend. El upstream es un `httptest` loopback que registra las rutas reales de `RegisterReviewerCasesRoutes`. Cruza `ReviewerCasesHandler`, `ReviewerAccessService.Download` y `documentstorage.Store` AES-GCM real con autenticador, repository/grant y BlobStore en memoria sintéticos. El fixture contiene un caso identity en revisión, archivo limpio con retención futura, digest/tamaño coincidentes y un grant identity del reviewer para el subject exacto. El adapter almacena/recupera solo en el BlobStore fake.

El escenario positivo observa un GET upstream de ruta exacta con Bearer sintético, una autorización fake, una lectura del repository con actor/caso exactos y un `Get` del BlobStore. Esa solicitud positiva no envía Cookie; el upstream observa Cookie/body/Content-Type vacíos. Luego el harness invoca el mismo export GET con Cookie sintético y verifica 400, `error.code=cookies_not_supported`, mensaje genérico, `private, no-store` y ausencia de Set-Cookie. El servidor Go registra solo el GET positivo (una llamada total), por lo que la solicitud con Cookie no alcanzó el upstream. La respuesta positiva verifica status 200, bytes PNG sintéticos exactos, `Content-Type: image/png`, `Content-Disposition: attachment; filename="space-document.png"`, `Cache-Control: private, no-store`, ausencia de Set-Cookie y de marcadores seleccionados de almacenamiento en el body. Esto caracteriza el rechazo de Cookie de esta ruta, no una política general de filtrado de headers.

## Verificación parent-owned

- Focal: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" ESPACIGO_FRONTEND_ROOT=/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFDownloadsReviewerIdentityEvidenceFromRegisteredGoHandler$' -v` — PASS (1 Go test; Node 1/1, con escenario binario positivo y rechazo de Cookie).
- Suites: mismo entorno aislado con `go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS (ambos paquetes).
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `node --check tooling/reviewer-download-bff-go-integration.mjs` y `node_modules/.bin/prettier --check tooling/reviewer-download-bff-go-integration.mjs` — PASS.
- `gofmt -d` vacío, whitespace explícito en ambos archivos y `git diff --check` — PASS.

La entrega inicial del executor falló al iniciar porque no pasó `ESPACIGO_FRONTEND_ROOT`. En el primer run parent con el entorno corregido, Node pasó y el test Go falló porque el fixture esperaba dos lecturas de repository; la ruta real ejecuta una. Se corrigió la expectativa a una. Para evitar etiquetar como PNG bytes arbitrarios, el fixture final genera un PNG sintético 1×1 con `image/png` estándar y pasa a Node la base64 de esos mismos bytes. En el follow-up de Cookie, el primer comando del executor informó `no tests to run` y no se cuenta como evidencia; la ejecución parent en el checkout backend correcto encontró un assertion de prueba que esperaba `code` en el nivel raíz, mientras el contrato BFF lo anida bajo `error`. Se corrigió la aserción al contrato observado (`error.code`, mensaje genérico y cache-control); el focal y la suite final pasaron. No se afirma RED/GREEN por estos errores de configuración/expectativa; los resultados listados corresponden al árbol final.

## Revisión independiente

`deleg_38aef5de` revisó una versión anterior del candidato y aprobó (`passed: true`, sin concerns ni errores lógicos); su única sugerencia fue precisar que el request no envía Cookie, por lo que su ausencia upstream no prueba stripping. Este límite ya está explicitado en «Alcance». La fuente cambió después para generar un PNG sintético válido y derivar dinámicamente su base64, además de corregir el conteo del repository; por ello esa revisión no cierra el gate del árbol final. La revisión independiente source-only `deleg_92a43cbd` aprobó la versión positiva (`passed: true`, listas de concerns/errores vacías), sin repetir pruebas; sugirió añadir Cookie entrante. El caso se agregó después y pasó las pruebas parent-owned. La revisión independiente final `deleg_cc9801bd` aprobó el árbol actual (`passed: true`, sin concerns de seguridad ni errores lógicos); fue source-only y no repitió pruebas. Su sugerencia opcional de afirmar el formato seguro de `request_id` queda fuera de este subcorte; el límite de claim permanece: se prueba rechazo de Cookie en esta ruta, no filtrado general de headers.

## Límites y seguridad

La autenticación, grant, snapshot, repository y BlobStore son sintéticos. Esto no prueba asignación/auth persistidas, SQL/PostgreSQL, GCS, scanner, runtime standalone de Next, runtime `cmd/api`, persistencia, concurrencia/atomicidad, autorización productiva o Cloud. El test no usa bases persistentes, no ejecuta SQL/DDL y no contacta proveedores/servicios externos; no se usan datos reales ni credenciales reales. No hubo commit, push, deploy ni integración a `main`.