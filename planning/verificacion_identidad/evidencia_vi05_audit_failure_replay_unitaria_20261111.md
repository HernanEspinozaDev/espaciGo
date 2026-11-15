# Evidencia — VI-05, replay local tras fallo de inserción de auditoría

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRetryIsIdempotentAfterAuditFailure` ejecuta `Repository.ProcessClaimedRetentionJob` real con `pgx.Tx` fake y `documentfs.Store` real en filesystem temporal, con objeto identity cifrado sintético.
- Primera pasada: el callback borra el objeto local; el Exec fake de inserción del evento de auditoría devuelve un error. Se afirma la secuencia `complete` → `audit_failed`, ausencia de commit, rollback diferido y objeto local ausente.
- Replay: el callback repite el borrado idempotente sobre el objeto ausente; el fake ejecuta `complete` → `audit` → `commit`.
- Caracterización de flujo Go y Store temporal en fallo antes de commit; sin cambio productivo.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, con todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas del entorno.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check (incluido archivo nuevo): PASS.

## Límites

La revisión independiente `deleg_397e111f` aprobó sin hallazgos ni sugerencias; confirmó que el Exec de auditoría fallido detiene el flujo antes de commit, se ejecuta el rollback diferido y el replay local sobre objeto ausente es idempotente. La falla de inserción de auditoría, el estado de filas y la recuperación del job son sintéticos; solo el filesystem local opera realmente. No demuestra rollback o retry durable de PostgreSQL, comportamiento de triggers/constraints, persistencia del evento, COMMIT real ambiguo, GCS/generaciones, borrado de backups ni recuperación. Sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
