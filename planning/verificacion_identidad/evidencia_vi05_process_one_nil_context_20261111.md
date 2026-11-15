# Evidencia — VI-05, ProcessOne rechaza contexto nil

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Cambio y TDD

- `RetentionProcessor.ProcessOne` ahora valida contexto nil junto con sus dependencias y devuelve `dv.ErrInvalid` antes de leer el contexto o reclamar un job.
- Nuevo `TestRetentionProcessorProcessOneRejectsNilContextBeforeClaiming` afirma `ErrInvalid` y cero claim, proceso o borrado.
- **RED observado:** antes del guard, el test focal falló con panic por nil pointer en `ctx.Err()` (`retention_processor.go:55`).
- **GREEN:** después del guard mínimo, el test focal pasó.

## Verificación local

- Test focal: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Cambio de guard Go; fake-only y no prueba persistencia, worker, PostgreSQL o Cloud. Revisión independiente `deleg_30c14bfc` aprobó sin hallazgos; confirmó el guard antes de `ctx.Err()`/claim, no intervención en contextos válidos y aserciones de cero trabajo. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
