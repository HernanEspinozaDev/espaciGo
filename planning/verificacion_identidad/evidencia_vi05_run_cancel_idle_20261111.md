# Evidencia — VI-05, cancelación durante pase idle de `Run`

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y muy dirty; cambios preexistentes preservados.

## Alcance

- Test-only characterization: `TestRetentionProcessorRunStopsAfterCancellationDuringIdlePass` ejecuta `RetentionProcessor.Run` real; el jobs fake cancela contexto durante un claim vacío `(nil,nil)`.
- Afirma retorno graceful, una llamada de claim, cero procesamiento, callbacks de error y borrados. El intervalo configurado es una hora, por lo que la salida se observa por cancelación y no por esperar el siguiente tick.
- No hay cambios de producción.

## Verificación local

- Test focal repetido `-count=50`: PASS.
- `go test ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go test -race ./internal/adapters/postgres/documentverification -count=1`: PASS.
- `go vet ./internal/adapters/postgres/documentverification ./internal/adapters/documentfs ./cmd/api`: PASS.
- `go build -o /dev/null ./cmd/api`: PASS.
- gofmt y diff-check, incluido archivo nuevo: PASS.
- Variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `DATABASE_URL`, y `PG*` retiradas de comandos Go.

## Límites

Fake-only: no demuestra scheduler/ticker de producción, shutdown del proceso, claim durable, DB, storage remoto, métricas ni logs globales. Revisión independiente `deleg_aa892640` aprobó sin hallazgos; confirmó una llamada de claim, cero trabajo y salida por cancelación del select sin esperar el ticker. Sin SQL/DDL, bases persistentes, Cloud, proveedores o datos reales. Sin commit, push, integración a `main` o despliegue.
