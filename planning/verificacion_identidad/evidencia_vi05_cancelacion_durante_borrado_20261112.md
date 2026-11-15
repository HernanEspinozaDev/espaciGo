# Evidencia — VI-05, cancelación durante el callback de borrado

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; se preservaron los cambios preexistentes y ajenos.

## Alcance y cambio

- `Repository.finishRetention` ahora distingue un fallo de borrado acompañado por cancelación del contexto: devuelve `ctx.Err()` antes de solicitar el UPDATE de retry. Los errores de storage con contexto activo conservan el retry sanitizado existente.
- `TestProcessClaimedRetentionJobDoesNotClassifyCanceledDeletionAsStorageFailure` usa la ruta real de `ProcessClaimedRetentionJob` con un `pgx.Tx` fake. El callback cancela el contexto y devuelve un error sintético; el test requiere `context.Canceled`, cero Exec de retry/completion, ningún commit y rollback diferido.
- TDD: antes del cambio, el test focal falló como se esperaba (`error=<nil>, want context cancellation`) porque el fake aceptó el retry y commit; tras el cambio pasó.

## Verificación local

Con `DATABASE_URL`, `DATABASE_DSN`, `DB_URL`, `ESPACIGO_DATABASE_URL`, `PGHOST`, `PGPORT`, `PGUSER`, `PGPASSWORD`, `PGDATABASE` y `PGPASSFILE` retirados:

- Test focal: PASS.
- Tests unitarios cuyo nombre comienza `TestProcessClaimedRetentionJob`, normal y `-race`: PASS.
- `go vet ./internal/adapters/postgres/documentverification`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- `gofmt -d` en los dos archivos Go y checks de whitespace/diff: PASS.

## Revisión independiente

`deleg_e762fbf3` aprobó el hunk de cancelación y el test sin hallazgos de seguridad/lógica. La sugerencia de mantener la conclusión limitada a no emitir retry/completion Exec ni commit en el fake está reflejada en el alcance y los límites siguientes.

## Límites

El `pgx.Tx` fake no prueba rollback, estado del job, lease, concurrencia ni transacciones reales de PostgreSQL. Al cancelar durante una operación de storage, el resultado externo puede ser ambiguo; esta cobertura solo verifica que el error cancelado no se convierta en retry ni completion dentro del callback transaccional ensayado. La recuperación durable tras expirar una lease y la idempotencia de borrado siguen sin demostrarse. Sin SQL/DDL, ejecución de tests conectados a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
