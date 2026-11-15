# Evidencia — VI-05, validación de entrada de RetentionProcessor.Run

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; cambios ajenos preservados.

## Alcance

- Test-only: `TestRetentionProcessorRunRejectsInvalidInputBeforeClaiming` cubre cinco argumentos inválidos: contexto nil, processor nil, poll cero/negativo y callback de error nil.
- Afirma `dv.ErrInvalid` y cero llamadas de claim/procesamiento y callback.
- Characterization de guard existente; no inicia ticker, no cambia producción y no usa DB.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRunRejectsInvalidInputBeforeClaiming$' -count=1`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido el archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

El test usa fakes y solo valida el guard de la API Go antes de reclamar; no acredita operación, ciclo de vida o shutdown de worker real ni persistencia. Revisión independiente `deleg_39618487` aprobó sin hallazgos; confirmó que cada caso aísla una entrada inválida, retorna antes de crear ticker y no genera actividad de claim/proceso/callback. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
