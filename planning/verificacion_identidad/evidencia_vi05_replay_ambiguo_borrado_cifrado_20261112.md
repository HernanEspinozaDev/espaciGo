# Evidencia VI-05 — replay de borrado cifrado ambiguo

Fecha: 2026-11-12
Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Caracterización local de la repetición segura de una limpieza cuyo primer borrado eliminó el objeto, pero devolvió un error ambiguo. Se añadieron solo pruebas; no cambió código productivo.

- `TestRetentionProcessorReplaysAmbiguousEncryptedDeleteAsIdempotent` compone el `RetentionProcessor`, `documentstorage.Store` cifrado y `Repository.ProcessClaimedRetentionJob` reales con BlobStore y `pgx.Tx` sintéticos. El primer intento retira el objeto y devuelve un error ambiguo; se verifica que el store lo sanea, el repositorio registra un retry con código genérico y la misma identidad de objeto. El segundo intento ante objeto ausente termina en completion y auditoría sintéticas.
- `TestStoreDeleteIsIdempotentWhenObjectIsAlreadyMissing` ejercita `cloudstorage.Store.Delete` contra un servidor HTTP local simulado: objeto ausente al consultar atributos y desaparición entre atributos y DELETE. El segundo caso conserva `ifGenerationMatch` con la generación observada.
- `TestStoreDeleteCanReplayAfterAmbiguousRemoteSuccess` simula que el DELETE cambia el objeto a ausente y luego devuelve HTTP 500. Con retry del cliente desactivado para aislar el resultado de la llamada, el primer `Delete` devuelve error; la repetición consulta el 404, devuelve éxito y no emite otro DELETE.

Las tres caracterizaciones pasaron en su primera ejecución: no son evidencia RED/GREEN ni una corrección productiva.

## Verificación ejecutada

En entorno local con dependencias Go disponibles:

- Pruebas focales `TestRetentionProcessorReplaysAmbiguousEncryptedDeleteAsIdempotent` y `TestStoreDeleteCanReplayAfterAmbiguousRemoteSuccess`: PASS.
- `go test -race ./internal/adapters/cloudstorage -count=1` y `go test -race ./internal/adapters/documentstorage -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -run '^(TestRetentionProcessorReplaysAmbiguousEncryptedDeleteAsIdempotent|TestProcessClaimedRetentionJobRequestsSanitizedRetryUpdateAfterStorageFailure|TestRetentionProcessorDeletesEncryptedDocumentsInTheirPurposeScope)$' -count=1`: PASS.
- `go vet` de `cloudstorage`, `documentstorage` y `postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d`, `git diff --check`, newline final y whitespace de los archivos tocados: PASS.

No se ejecutó ningún test que abra conexión de base de datos. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedor o datos reales. Los resultados de `pgx.Tx` y HTTP son fakes locales; no prueban locks, transacciones PostgreSQL, GCS/IAM, wiring de runtime, borrado físico ni retención legal.

## Estado y siguiente paso

El test cubre la recuperación local de un resultado de borrado incierto; no cierra VI-05 ni sus gates durable/operacionales. Continúa pendiente alinear el cliente/UI Next con la fecha civil `format: date`. El worktree autorizado `frontend-next-hermes` está limpio en `cf6e28a365d2495e78bc64c01686ed55efb17c8a`, pero no contiene `frontend/src/features/document-review/`; no se modificó ese worktree ni el frontend dirty del checkout backend. Se requiere resolver el alcance de alineamiento de frontend en su worktree autorizado antes de integrar el contrato.

No hubo commit, push, deploy ni integración a `main`.
