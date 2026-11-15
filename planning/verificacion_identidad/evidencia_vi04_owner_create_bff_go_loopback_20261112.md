# Evidencia VI-04 — alta identity owner BFF→Go (2026-11-12)

- **Estado:** caracterización implementada; gates parent-owned PASS; revisión independiente source-only `deleg_f5dee05f` aprobada (`passed=true`, sin concerns ni errores lógicos).
- **Backend:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Frontend autorizado:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Archivos nuevos de la caracterización:** `internal/documentverification/transport/http/identity_owner_create_bff_go_integration_test.go` y `frontend/tooling/identity-owner-create-bff-go-integration.mjs`. Sin cambios de producción.

## Recorrido caracterizado

El test Go con build tag `bffintegration` inicia un `httptest.NewServer` con las rutas owner registradas, `OwnerCasesHandler` y `documentverification.Service.Start` reales. Un proceso Node transpila el export `POST` real de la ruta Next y el módulo generado de operaciones, los carga desde un directorio temporal aislado y envía la petición por loopback al handler Go. Authenticator y repositorio son sintéticos en memoria.

El caso verifica una única solicitud `POST /api/v1/document-verification/cases` con JSON exacto de identity (`purpose`, formato declarado `new`, consentimiento sintético afirmativo y versión `identity-capture-local-v1`), Bearer sintético, sin Cookie y sin campos de titular, revisión de declaración, espacio o vencimiento. Comprueba la actividad de autenticación, un único `CreateCase` de actor identity sin scope de espacio, cero lecturas/listados/transiciones adicionales y que el snapshot recibe la revisión de declaración desde la autorización sintética del repositorio, no desde el body.

La respuesta exige 201, ETag coherente con revisión 1, `Cache-Control: private, no-store`, `Vary: Authorization`, ausencia de `Set-Cookie` y DTO identity con conjunto exacto de propiedades, sin identificadores internos de titular/declaración/revisor ni datos de storage. El fake asigna revisión 1 al caso nuevo, igual que el repositorio de memoria de dominio; la fuente del adapter PostgreSQL también devuelve revisión 1 después de una inserción nueva, pero ese adapter no se ejecutó.

## Pruebas

- Primer intento focal: falló porque el fake nuevo devolvía revisión 0 y el harness esperaba el ETag inicial `"1"`. No fue RED de producto; se corrigió el fake para modelar el contrato de alta del repositorio. No se modificó producción.
- Focal final, con entorno aislado y `-race`: `go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFCreatesOwnerIdentityCaseThroughRegisteredGoRoute$' -v` — PASS; el harness Node reportó 1 test, 1 PASS, 0 fail/skip.
- `go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- En el frontend autorizado: `node --check tooling/identity-owner-create-bff-go-integration.mjs` y Prettier `--check` del mismo archivo — PASS.
- `gofmt -d`, `git diff --check` y comprobación explícita de whitespace en los archivos del corte/plan — PASS.
- Orden de gates: suite Go etiquetada, vet tagged y build corrieron después de corregir el fake y antes de actualizar plan/evidencia; después de esas ediciones documentales se repitieron el focal con `-race`, Node syntax/Prettier y diff/whitespace.

## Límites

La ruta Next se transpila y se ejecuta en Node con dos imports sustituidos por módulos temporales; esto no demuestra el build, resolución normal de módulos o runtime standalone Next. La solicitud la construye directamente el harness: no recorre UI ni el adapter cliente, cuya creación frontend tiene evidencia mock separada. Authenticator, autorización de la revisión de declaración y repositorio son fakes; no se demuestra autenticación/grants efectivos, adapter PostgreSQL, SQL, transacción, persistencia, idempotencia ante una creación repetida, lectura tras reinicio, consentimiento legal, captura/carga, scanner, storage/GCS o Cloud. No se usaron datos reales, cuentas, credenciales, DB/Cloud ni servicios externos; no se ejecutó SQL/DDL. Sin commit, push, despliegue o integración a `main`.

La revisión independiente `deleg_f5dee05f` sugirió explicitar dos límites no bloqueantes: no se envió Cookie, por lo que no se caracteriza aquí el rechazo de Cookie en este POST; y la respuesta nueva trae `files: []`, así que el test no ejercita allowlisting de propiedades anidadas de archivos.
