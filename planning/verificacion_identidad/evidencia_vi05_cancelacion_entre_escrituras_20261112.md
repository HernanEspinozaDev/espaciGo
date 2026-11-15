# Evidencia VI-05 — cancelación entre escrituras de finalización

Fecha: 2026-11-12
Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`

## Regresiones y cambio

Se ampliaron las pruebas de `Repository.ProcessClaimedRetentionJob` con un `pgx.Tx` fake que permite registrar si la ruta productiva continúa después de que una operación cancela el contexto y aun así devuelve éxito:

- `TestProcessClaimedRetentionJobStopsBeforeAuditWhenContextCancelsAfterCompletionWrite`: RED devolvía `nil` y ejecutaba la auditoría/commit cuando el fake cancelaba el contexto tras el UPDATE de completion. GREEN comprueba el contexto después del UPDATE; ahora devuelve `context.Canceled`, no intenta auditar y la transacción fake revierte.
- `TestProcessClaimedRetentionJobRollsBackWhenContextCancelsAfterAuditWrite`: RED devolvía `nil` y confirmaba cuando el fake cancelaba tras el INSERT de auditoría. GREEN comprueba el contexto antes de `Commit`; ahora retorna `context.Canceled` y el fake registra rollback, no commit.

El cambio productivo añade guardas de contexto entre completion, auditoría y commit. Estas pruebas no simulan un driver real ni afirman que un rollback PostgreSQL haya ocurrido.

## Verificación ejecutada

Con `env -i` sin variables DB/PG/opt-in:

- Las dos regresiones nuevas, junto con la regresión previa de cancelación después del borrado, pasaron con `-race`.
- `go test -race ./internal/adapters/postgres/documentverification -run '^(TestProcessClaimedRetentionJob|TestRetentionProcessorReplaysAmbiguousEncryptedDeleteAsIdempotent|TestRetentionProcessorDeletesEncryptedDocumentsInTheirPurposeScope)' -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

El alcance es unitario: `pgx.Tx` fake, sin conexión o test de integración. No valida locks, rollback/commit PostgreSQL, fencing durable ni comportamiento del worker real. No ejecuta SQL/DDL ni accede a bases persistentes, Cloud, GCS o datos reales. No hubo commit, push, deploy ni integración a `main`.
