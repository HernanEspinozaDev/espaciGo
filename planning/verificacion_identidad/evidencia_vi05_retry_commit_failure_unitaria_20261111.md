# Evidencia — VI-05, fallo al confirmar el reintento

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobReturnsRetryCommitFailure` ejercita el método real del repository con `pgx.Tx` sintético: el callback de borrado falla con un marcador de ruta privada, el Exec fake acepta el UPDATE de retry y el Commit fake retorna error.
- Afirma propagación del error de commit, una llamada al callback, orden `delete → retry → commit_failed`, argumentos exactos del retry (IDs, demora, lease y `retention_store_error` allowlisted), ausencia del marcador privado, cero commit confirmado y rollback diferido.
- Characterization unitaria de la rama de retry ante error de Commit; sin cambio productivo ni RED observado.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS con variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

El `Commit` fallido y UPDATE de retry son fakes; no se prueba atomicidad, estado durable, resultado ambiguo de PostgreSQL, re-claim ni recuperación. Sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue. Revisión independiente `deleg_fdfe5d29`: aprobada sin hallazgos; confirmó la secuencia y los argumentos sanitizados, con los límites de durabilidad/atomicidad explícitos.
