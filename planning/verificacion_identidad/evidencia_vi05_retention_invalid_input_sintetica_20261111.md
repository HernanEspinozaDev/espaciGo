# Evidencia — VI-05, inputs inválidos antes de abrir transacción

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- `TestProcessClaimedRetentionJobRejectsInvalidInputBeforeTransaction` caracteriza el repository con transaction beginner contador que falla si se invoca.
- Seis escenarios: callback nil, retry cero/negativo, job ID inválido, lease cero y repository nil.
- Cada caso devuelve `dv.ErrInvalid`, no llama `BeginTx` ni el callback de borrado.
- Test-only; el comportamiento pasó en primera ejecución, sin RED observado ni cambio productivo.

## Verificación local

- Test focal: PASS.
- Paquete completo `internal/adapters/postgres/documentverification`, normal y `-race`: PASS, retirando variables `ESPACIGO_*`, `PG*` y `DATABASE_URL`.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.

## Límites

Solo valida guards de entrada y que no se inicia la transacción fake; no prueba conexión, SQL, autorización o durabilidad. Sin SQL/DDL, conexión a bases, Cloud, proveedores, datos reales, commit, push, integración a `main` o despliegue. Revisión independiente `deleg_67b90ccf`: aprobada sin hallazgos; confirmó los seis escenarios, cero aperturas de transacción y cero callbacks de borrado, con alcance de characterization only.
