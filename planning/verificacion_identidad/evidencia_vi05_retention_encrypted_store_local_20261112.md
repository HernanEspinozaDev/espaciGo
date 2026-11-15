# Evidencia — VI-05, limpieza local con Store cifrado

- **Fecha:** 2026-11-12.
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`.
- **Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y ampliamente dirty; se preservaron los demás cambios.

## Recorrido ejercido

`TestRetentionProcessorDeletesEncryptedDocumentsInTheirPurposeScope` combina el `RetentionProcessor` real y `documentstorage.Store` real, con jobs y BlobStore sintéticos. Para identity y space, guarda bytes sintéticos, verifica que el fake recibe un envelope y que el processor borra el mismo object ID scoped por owner/caso/archivo (y espacio cuando aplica). El job fake registra una finalización en memoria sin retry y los objetos quedan ausentes.

Es caracterización de primera ejecución en verde; no hubo RED ni cambio productivo en este subcorte.

## Verificación local

Con variables de DB/PG/opt-in retiradas:

- Test focal — PASS.
- `go test -race ./internal/adapters/documentstorage ./internal/adapters/postgres/documentverification ./internal/documentverification/... -count=1` — PASS en todos los paquetes.
- `go vet` de los paquetes listados — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt` — PASS.

## Límites

El BlobStore y el job son fakes: no se ejerce `ProcessClaimedRetentionJob` con transacción ni se prueban SQL, locks, leases, fencing, retry/completion durable, GCS, IAM/CMEK o wiring de runtime. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
