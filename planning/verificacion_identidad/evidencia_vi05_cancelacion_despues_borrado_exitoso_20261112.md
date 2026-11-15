# Evidencia VI-05 — cancelación después de borrado exitoso

Fecha: 2026-11-12
Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
HEAD: `274f37e483a078890e92382a159fa49fd23c2fe8`

## Cambio y regresión

`TestProcessClaimedRetentionJobStopsWhenContextCancelsAfterSuccessfulDelete` ejecuta `Repository.ProcessClaimedRetentionJob` con una asignación/lease sintéticas y un `pgx.Tx` fake que registra las escrituras y no las rechaza automáticamente por contexto cancelado. El callback de borrado devuelve éxito y cancela el contexto antes de retornar.

- RED observado antes del cambio: el método ejecutaba completion/auditoría y retornaba `nil` después de la cancelación.
- GREEN: `finishRetention` consulta `ctx.Err()` al terminar con éxito el callback y antes de escribir completion/auditoría; ahora retorna `context.Canceled`, la transacción fake recibe rollback y no hay escrituras de completion.

El código no intenta deshacer el borrado de objeto ya realizado. La ejecución posterior podrá repetir el borrado idempotente; esa propiedad solo tiene cobertura local simulada en `evidencia_vi05_replay_ambiguo_borrado_cifrado_20261112.md`.

## Verificación ejecutada

Con entorno de test `env -i` sin variables de DB/PG/opt-in:

- Test focal con `-race`: PASS después del cambio.
- Pruebas de cancelación, retry saneado y replay/composición relacionadas, con `-race`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` y `git diff --check`: PASS.

No se ejecutó test de integración ni conexión a base. El fake prueba orden/cancelación en el método real, no la semántica de `pgx`, locks, rollback/commit PostgreSQL ni fencing durable. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, GCS, proveedor o datos reales. No hubo commit, push, deploy ni integración a `main`.
