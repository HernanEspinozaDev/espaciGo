# Evidencia — VI-05, replay local tras fallo de completion Exec

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRetryIsIdempotentAfterCompletionFailure` invoca `Repository.ProcessClaimedRetentionJob` real con un `pgx.Tx` fake y `documentfs.Store` real en filesystem temporal, usando objeto identity cifrado sintético.
- Primera pasada: el callback elimina el objeto local; el fake devuelve error en el Exec de completion. Se afirma que no hay auditoría ni commit, que el rollback diferido ocurre y que el objeto fue eliminado localmente.
- Replay: el borrado repetido de objeto ausente retorna éxito; el fake completa los Exec de completion/auditoría y commit.
- Caracterización de flujo Go y borrado idempotente local ante fallo de actualización; sin cambios productivos.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, con todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

La revisión independiente `deleg_e4929855` aprobó sin hallazgos ni sugerencias; confirmó que el Exec de completion falla tras borrar el objeto local, evita auditoría/commit, recorre rollback y permite replay sobre objeto ausente en el Store temporal. El error de Exec, estado de lease/filas y replay son sintéticos; solo el filesystem temporal opera realmente. No prueba filas afectadas, rollback/reclaim o auditoría durable de PostgreSQL, triggers, GCS/generaciones, backups o recuperación. Sin SQL/DDL, conexiones a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
