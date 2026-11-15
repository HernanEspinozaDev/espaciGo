# Evidencia — VI-05, claim exitoso de job de retención

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestClaimReadyRetentionJobReturnsClaimAfterCommit` llama al método real `Repository.ClaimReadyRetentionJob` con `pgx.Tx`/BeginTx fake.
- El fake sintetiza candidato, locks de caso/archivo, ausencia de scan activo y `UPDATE ... RETURNING` con intento incrementado y lease.
- Afirma los datos del claim devuelto, argumentos del UPDATE, una única transacción `ReadCommitted`, orden candidato→locks→recheck→UPDATE→commit→rollback diferido, un Commit y ninguna consulta inesperada.
- `TestClaimReadyRetentionJobNormalizesCandidateCutoffToUTC` ejercita el helper con ID de job y hora en zona sintética no UTC; afirma filtros bindados en consulta candidata y UPDATE, con cutoff normalizado a UTC. El llamado público sin filtros afirma argumentos nulos.
- `TestClaimReadyRetentionJobLocksSpaceBeforeClaimForSpacePurpose` modela `PurposeSpace` con lock-row presente; afirma que el ID exacto del espacio se envía antes de los locks de caso/archivo y que el claim retorna tras el commit fake.
- La primera ejecución focal falló porque el fixture fake devolvía lease cero; el fake se corrigió para modelar el valor de lease del `RETURNING` antes de repetir los gates.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestClaimReadyRetentionJobReturnsClaimAfterCommit$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de los comandos Go.

## Límites

El fake no ejecuta ni valida SQL, `RowsAffected`, locks, aislamiento, lease temporal ni persistencia PostgreSQL; tampoco comprueba atomicidad ni comportamiento concurrente real. Revisiones independientes `deleg_7a041724`, `deleg_9b5d6850` y follow-up `deleg_b14204f2` aprobaron sin hallazgos bloqueantes; este último verificó el branch de lock de espacio presente, el ID exacto y el orden sintético de llamadas, manteniendo explícitos los límites de PostgreSQL. Sin SQL/DDL contra bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
