# Evidencia — VI-05, replay local tras fallo de commit (sintética)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRetryIsIdempotentAfterCommitFailure` invoca `Repository.ProcessClaimedRetentionJob` real con `pgx.Tx` fake y `documentfs.Store` real en `t.TempDir()` con objeto identity cifrado/sintético.
- Primera pasada: el callback borra el objeto local; el fake falla en `Commit`; el método devuelve ese error, ejecuta rollback diferido y el objeto local ya no existe.
- Replay: el fake mantiene lease/filas sintéticas válidas; el callback repite el borrado sobre el objeto ausente, que es idempotente en `documentfs.Store`; la finalización/auditoría fake y commit sintético pasan.
- Caracterización de compatibilidad entre borrado local idempotente y reintento tras un error de commit simulado; sin cambios productivos.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS; entorno de pruebas sin variables `ESPACIGO_*`, `PG*` ni `DATABASE_URL`.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluidos archivos nuevos: PASS.

## Límites

La revisión independiente `deleg_b38d67a6` aprobó sin hallazgos; confirmó que se borra el objeto local antes del error de commit fake, que el replay borra el objeto ausente y que el fake no representa transacción PostgreSQL durable. El error de commit, lease y filas son sintéticos; el Store real solo es filesystem temporal local. No prueba el resultado ambiguo de un COMMIT real, rollback PostgreSQL, persistencia/reclaim de la lease, transacción/auditoría durable, GCS/generaciones, borrado físico de backups ni recuperación. No ejecutar SQL/DDL, consultar bases, usar Cloud/proveedores/datos reales, hacer commit, push, integración a `main` o despliegue.
