# Evidencia — VI-05, CAS miss al reclamar job de retención

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestClaimReadyRetentionJobReturnsNoWorkWhenConditionalUpdateLosesCAS` usa el método real `Repository.ClaimReadyRetentionJob` con `pgx.Tx` fake.
- El fake sintetiza candidate/locks/recheck sin scan activo y responde `pgx.ErrNoRows` al `UPDATE ... RETURNING` condicional.
- Afirma `(nil,nil)`, cinco consultas hasta el UPDATE, cero commits y una llamada al rollback diferido; no hay SQL inesperado.
- Characterization de rama ya existente; no cambia código productivo.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestClaimReadyRetentionJobReturnsNoWorkWhenConditionalUpdateLosesCAS$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

La respuesta CAS es sintética; el test no valida `RowsAffected`, SQL, concurrencia, locks, transacción ni PostgreSQL durable. La revisión independiente `deleg_158255ae` aprobó sin bloqueos y sugirió verificar las cinco consultas en secuencia; esa aserción se añadió y los gates se repitieron. No demuestra CAS ni semántica de DB real. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
