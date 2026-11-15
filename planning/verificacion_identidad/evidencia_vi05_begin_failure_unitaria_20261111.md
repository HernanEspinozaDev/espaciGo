# Evidencia — VI-05, fallo al iniciar transacción

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobDoesNotDeleteWhenTransactionCannotBegin` ejercita `Repository.ProcessClaimedRetentionJob` con un `BeginTx` sintético que retorna error.
- Afirma retorno del error, una sola solicitud de transacción con aislamiento `ReadCommitted` y cero llamadas al callback de borrado.
- Test-only characterization del fail-closed previo a la transacción; sin cambio productivo ni RED observado.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS con `ESPACIGO_*`, `PG*` y `DATABASE_URL` retiradas.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

Solo caracteriza propagación del error y ausencia del callback ante un BeginTx fake fallido; no prueba conexión, transacción, SQL ni storage. No se ejecutó SQL/DDL ni se accedió a bases, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue. Revisión independiente `deleg_ba0c3427`: aprobada sin hallazgos; confirmó una solicitud `ReadCommitted`, propagación del error y cero llamadas al borrado.
