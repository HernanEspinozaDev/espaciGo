# Evidencia — VI-05, guards de entrada del claim de retención

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestClaimReadyRetentionJobRejectsInvalidInputBeforeTransaction` recorre cinco entradas sintéticas: repository nil, transaction beginner nil, lease cero, lease negativa e ID inválido en el helper de filtro.
- En todas, verifica `dv.ErrInvalid`, claim nil y cero llamadas a `BeginTx` mediante fake que devuelve error si se le invoca.
- Characterization del guard ya existente; no cambia código productivo ni ejecuta consulta.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestClaimReadyRetentionJobRejectsInvalidInputBeforeTransaction$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

El fake solo acredita que estas entradas salen antes de solicitar transacción; no demuestra conexión ni comportamiento PostgreSQL. Revisión independiente `deleg_99f35b59` aprobó sin hallazgos y confirmó los cinco guards y cero llamadas a `BeginTx`. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
