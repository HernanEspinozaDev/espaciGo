# Evidencia VI-04: upload owner identity con servicio real BFF→Go

Fecha local: 2026-11-12 (`America/Santiago`).

## Alcance y checkout

Se añadió únicamente `internal/documentverification/transport/http/identity_owner_upload_service_bff_go_integration_test.go` en el checkout backend `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`. Se reutilizó sin editar el harness frontend existente `tooling/identity-bff-go-integration.mjs` del checkout autorizado `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.

Es una caracterización local test-only: export POST de la ruta Next y operación generada transpilan a módulos temporales; Node envía un multipart loopback hacia rutas owner Go registradas en `httptest`. El handler es real y se construyen `documentverification.Service` y `UploadService` reales. Autenticador, repositorio en memoria, inspector y store de documentos son sintéticos.

## Comportamiento verificado

- Una única petición upstream POST a `/api/v1/document-verification/cases/{caseID}/files`, Bearer sintético, sin Cookie, `If-Match: "3"` y `multipart/form-data`.
- La autenticación fake ve el principal sintético, rol vacío y actividad `UserOperation`.
- La solicitud ejecuta el servicio real de carga: un `LoadOwnCase`, una transición del repositorio en memoria y revisión 3→4; el snapshot queda con un archivo pending `identity_front`.
- El inspector fake recibe propósito identity, ID del archivo y bytes sintéticos; retorna metadata `image/png` y copia sintética de bytes. El store fake recibe el mismo owner/caso/ID/bytes; no se llama a rutas de space ni a delete.
- El harness Next verifica la respuesta 201, ETag `"4"`, `private, no-store`, ausencia de `Set-Cookie`, DTO de archivo con rol/MIME/tamaño y ausencia de nombre original/`object_key`.

El primer resultado del nuevo test fue PASS como caracterización de comportamiento existente; no se atribuye RED/GREEN ni defecto corregido.

## Gates parent-owned

En el checkout backend, con entorno `env -i` y `ESPACIGO_FRONTEND_ROOT` apuntando al checkout Next autorizado:

- Focal `go test -race -tags=bffintegration -count=1 ./internal/documentverification/transport/http -run '^TestNextBFFUploadsOwnerIdentityThroughActualUploadService$' -v`: PASS; Node subtest 1/1.
- Focal repetido con `-count=5`: PASS 5/5; cada iteración ejecutó Node 1/1.
- `go test -race -tags=bffintegration -count=1 ./internal/documentverification ./internal/documentverification/transport/http`: PASS en ambos paquetes.
- `go vet -tags=bffintegration ./internal/documentverification ./internal/documentverification/transport/http`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- En frontend, `node --check tooling/identity-bff-go-integration.mjs` y Prettier `--check` sobre ese script existente: PASS; no se modificó.
- `gofmt -d` y verificación explícita de whitespace de la nueva prueba: limpios. `git diff --check` pasó para cambios versionados; la nueva prueba aún no está versionada, por eso se comprobó su whitespace por separado.
- Revisión independiente source-only `deleg_dfc889d2`: aprobada (`passed=true`, sin concerns de seguridad ni errores lógicos). Sugirió mantener explícito que bytes sintéticos etiquetados como `image/png` no prueban estructura PNG ni escaneo; que repositorio/store en memoria no prueban atomicidad PostgreSQL ni durabilidad; y que no se comprueba por separado el filename multipart ni cada campo parseado. Estos límites quedan registrados; la respuesta y el servicio verifican el rol y bytes observados. La falta de aserción independiente del filename es no bloqueante y no se amplió este corte.

## Límites y seguridad de ejecución

Las pruebas usan una ruta Next transpilada y entorno local sintético; no prueban el build/runtime normal de Next, UI ni adapter cliente. El repositorio y autenticador fake no prueban autorización efectiva, grants, ownership persistido ni comportamiento PostgreSQL. Inspector/store fakes no decodifican imágenes, no escanean ni usan almacenamiento real; no se prueba GCS/Cloud. El test no demuestra atomicidad o persistencia tras reinicio.

No se ejecutó SQL/DDL, consultas o conexión a bases persistentes, Cloud, servicios externos, cuenta real, commit, push, deploy ni integración a `main`. Se preservaron las modificaciones existentes de ambos worktrees.
