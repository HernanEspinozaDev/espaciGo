# Evidencia — VI-05, CAS miss al persistir retry (unitaria sintética)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRejectsRetryCASMiss` ejecuta el método real con `pgx.Tx` fake; el callback de borrado produce un error sintético con marcador de ruta privada.
- El Exec fake del retry retorna `UPDATE 0`. Se exige `dv.ErrConflict`, una llamada al callback, IDs/demora/lease correctos, código allowlisted `retentionStoreError`, ausencia del marcador privado, ningún commit y un rollback diferido.
- Caracterización del guard `RowsAffected()!=1` en la rama de retry; sin cambio productivo ni filesystem/DB real.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, retirando todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL`.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

La revisión independiente `deleg_a71b0732` aprobó sin hallazgos ni sugerencias; confirmó que `UPDATE 0` retorna conflicto antes de commit, se ejecuta rollback, los argumentos están acotados/sanitizados y los límites no sobredeclaran cobertura real. El resultado `UPDATE 0`, lease, filas y callback son sintéticos. No demuestra la semántica de CAS/filas afectadas, locks, SQL, retry/rollback durable ni privacidad global de logs. Sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
