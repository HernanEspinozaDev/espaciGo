# Evidencia — VI-05, `Run` vuelve a consultar después de una cola vacía

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- Test-only characterization: `TestRetentionProcessorRunPollsAgainAfterNoWork` llama al `RetentionProcessor.Run` real con job fake que devuelve `(nil,nil)` en el primer claim y un claim identity en el segundo.
- Comprueba dos claims con lease configurado, un único proceso y borrado exacto del scope identity; ningún callback de error; cancelación graceful tras el segundo procesamiento.
- La repetición focal `-count=50` fue estable. No hay cambios de producción.

## Verificación local

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRunPollsAgainAfterNoWork$' -count=50`: PASS.
- Suite `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- Suite `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Fake-only: caracteriza que el loop reintenta tras no encontrar trabajo, no ticker de producción, cola real, claim/lease durable, PostgreSQL, almacenamiento persistente, métricas o logs. Revisión independiente `deleg_8871f662` aprobó sin hallazgos bloqueantes; confirmó secuencia, scopes/contadores y estabilidad `-count=50`. Sugirió opcionalmente verificar poll interval solo si fuera contrato explícito; no se amplió porque esta prueba cubre el reintento tras `(nil,nil)`, no precisión temporal del ticker. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
