# Evidencia — VI-05, CAS miss de completion tras borrado local

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRejectsCompletionCASMissAfterDeletion` ejecuta el repository real con un `pgx.Tx` fake y `documentfs.Store` real bajo `t.TempDir()`.
- El primer callback borra el documento identity sintético; el UPDATE de completion fake retorna `UPDATE 0`. El método devuelve `dv.ErrConflict`, no intenta auditoría ni commit, y el rollback diferido se ejecuta.
- El objeto local ya ausente se vuelve a borrar en una pasada de replay fake válida, que completa completion/auditoría/commit.
- Caracterización del guard `RowsAffected()!=1` y de idempotencia local ante replay; sin cambio productivo.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, retirando todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL`.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check (incluido el archivo nuevo): PASS.

## Límites

La revisión independiente `deleg_bbfacc1f` aprobó sin hallazgos ni sugerencias; confirmó que `UPDATE 0` detiene el flujo antes de auditoría/commit, recorre rollback y que el replay usa únicamente transacción fake más Store temporal idempotente. El resultado `UPDATE 0`, estado de lease/filas y replay son sintéticos; solo opera el filesystem temporal. No demuestra CAS/filas afectadas, locks, rollback/reclaim/auditoría PostgreSQL durable, GCS/generaciones, backups o recuperación. Sin SQL/DDL, conexiones a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
