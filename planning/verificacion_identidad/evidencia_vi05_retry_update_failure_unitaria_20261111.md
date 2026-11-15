# Evidencia — VI-05, fallo del Exec de retry (unitaria sintética)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobReturnsSanitizedRetryUpdateFailure` ejecuta `Repository.ProcessClaimedRetentionJob` real con `pgx.Tx` fake; el callback de borrado devuelve un error sintético con marcador de ruta privada.
- El wrapper fake falla el Exec de retry y captura argumentos. El test verifica el código allowlisted `retentionStoreError`, IDs/demora/lease esperados, ausencia del marcador privado, retorno del error de persistencia, ningún commit y un rollback diferido.
- No hay cambio productivo ni filesystem/DB real.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, con todas las variables `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

La revisión independiente `deleg_a69489d8` aprobó sin hallazgos ni sugerencias; confirmó que el retry UPDATE se intercepta tras el fallo del callback y que las aserciones de argumentos/código allowlisted, ausencia del marcador privado, error/rollback son adecuadas. Fake de transaction/Exec y callback sintético: no demuestra SQL parametrizado en ejecución, retry/rollback durable, seguridad global de logs, ni persistencia de lease. Sin SQL/DDL, conexiones a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue.
