# Evidencia — VI-05, `Run` continúa tras error de procesamiento de job reclamado

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- Test-only characterization: `TestRetentionProcessorRunContinuesAfterClaimedJobProcessingError` ejecuta `RetentionProcessor.Run` real con jobs/storage fakes.
- El primer claim se entrega, pero el método fake `ProcessClaimedRetentionJob` devuelve un error sintético antes de invocar el borrado; el callback lo recibe. En el siguiente poll, el fake vuelve a entregar un claim, invoca el borrado identity de scope exacto y cancela contexto tras éxito.
- Afirma dos claims con lease configurado, dos llamadas de procesamiento, un callback de error y una única eliminación identity; no cambia código productivo.

## Verificación local

- Test focal repetido `-count=50`: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

El error del primer procesamiento y el claim repetido son sintéticos; no prueba persistencia de lease/retry, atomicidad, selección/eligibilidad SQL, recuperación PostgreSQL, GCS, worker real, métricas ni logs globales. Revisión independiente `deleg_dbcea085` aprobó sin hallazgos bloqueantes; confirmó que el error se notifica y el loop retoma tras el poll, acotado a fake. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
