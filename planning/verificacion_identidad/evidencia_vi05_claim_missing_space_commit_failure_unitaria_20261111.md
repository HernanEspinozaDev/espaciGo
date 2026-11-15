# Evidencia — VI-05, Commit fallido al declinar claim por espacio ausente

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestClaimReadyRetentionJobPropagatesCommitFailureWhenSpaceRowIsMissing` configura el lock-row fake como ausente y el Commit fake como error.
- Afirma que el método propaga el error y no devuelve claim; limita las consultas a candidato→lock de espacio con el ID exacto; verifica un intento de Commit, ningún commit exitoso registrado por el fake, y un rollback diferido.
- El fake `Commit` marca éxito solo cuando no se inyecta error. No se modificó código productivo.

## Verificación local

- Tests focales de espacio ausente y Commit fallido: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

El error y el estado de transacción son sintéticos; un Commit real fallido puede ser ambiguo y no se simula aquí. Revisión independiente `deleg_ef0c26e9` aprobó sin hallazgos y confirmó candidato, lock falso, propagación del error, claim nil y rollback fake diferido. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
