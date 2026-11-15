# Evidencia — VI-05, fallo al iniciar transacción del claim

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestClaimReadyRetentionJobPropagatesBeginFailure` llama `Repository.ClaimReadyRetentionJob` con un `transactionBeginner` fake que devuelve error.
- Afirma error propagado, claim nil y una única solicitud `BeginTx` con `pgx.ReadCommitted`.
- No se obtiene transacción ni se ejecuta consulta; characterization del branch de inicio fallido, sin cambios productivos.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestClaimReadyRetentionJobPropagatesBeginFailure$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

El beginner es fake; no acredita conexión, apertura o rollback real de PostgreSQL. La revisión independiente `deleg_95620db2` aprobó sin hallazgos y confirmó error propagado, claim nil, una llamada y opción `ReadCommitted`. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
