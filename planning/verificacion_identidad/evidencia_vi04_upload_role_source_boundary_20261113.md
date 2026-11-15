# Evidencia — VI-04, origen de `evidence_role` en carga identity

- **Fecha:** 2026-11-13 (`America/Santiago`).
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se conservaron cambios preexistentes.

## Caracterización sintética

`TestTypedIdentityUploadUsesRequestedRoleInsteadOfInspectorMetadata` ejercita `UploadService.Upload` y el servicio de dominio reales con repository, inspector y document store sintéticos en memoria. La solicitud tipada pide `identity_front`, mientras el inspector fake devuelve el metadato contradictorio `identity_back`. El test verifica que el servicio liga el rol validado de la solicitud al recibo y al snapshot que la transición agrega, con revisión 1→2 y una inspección/un `Put`, sin `Delete`.

El rol es una declaración de la carga; `UploadInspector.Inspect` no recibe un rol y el inspector real devuelve metadatos derivados del contenido. Por eso, esta caracterización confirma la autoridad del rol solicitado en el servicio, no rechaza el resultado del inspector ni afirma que la imagen permita inferir el lado del documento.

La primera ejecución falló por una fixture tipada incoherente (conservaba un archivo legacy sin rol); tras hacer el snapshot draft sintético válido, una aserción inicial esperaba un índice incorrecto de archivo. Ambos fueron errores de fixture/aserción, no RED del producto. Con la fixture corregida, la prueba focal con `-race -count=1` pasó. No se afirma RED/GREEN ni hubo cambio de producción.

## Gates

Con variables de DB/PG/opt-in de integración retiradas:

- Focal `go test -race -count=5 ./internal/documentverification -run '^TestTypedIdentityUploadUsesRequestedRoleInsteadOfInspectorMetadata$' -v` — PASS, 5/5.
- `go test -race -count=1 ./internal/documentverification/...` — PASS en dominio, inspección y HTTP.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` y `git diff --check` de paths versionados del corte — PASS; escaneo explícito de whitespace del archivo nuevo — PASS.
- Escaneo estático de líneas añadidas: cero hardcoded secrets, shell execution, eval/exec o SQL dinámico.

Revisión source-only independiente `deleg_82a9ce82` — **PASS**, sin concerns de seguridad ni errores lógicos. Confirmó que la fixture cumple precondiciones typed-identity y que las aserciones fallarían si se usara el rol del inspector en vez del solicitado; las observaciones opcionales ratificaron limitar el reclamo a fakes e in-memory. Las gates no incluyen integración de DB ni Cloud.

## Límites

Solo fakes e información sintética en memoria. No prueba autenticación/grants efectivos, persistencia, SQL/PostgreSQL, atomicidad/concurrencia, scanner, almacenamiento real/GCS, runtime BFF ni Cloud. El control flow de un `Put` fallido intenta un Delete compensatorio y no registra retry persistente en esa rama; si un storage real persistió pese al error y el Delete falló, podría quedar un objeto sin metadata. La prueba fake-only y sus límites están en `evidencia_vi05_upload_double_failure_characterization_20261113.md`; no acreditan que esa condición ocurra en storage real. Tracking durable requiere una vía persistente autorizada y queda bloqueado por la prohibición vigente de SQL/DDL. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, despliegue o merge.
