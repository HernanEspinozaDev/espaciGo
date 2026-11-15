# Evidencia — VI-05, worker local reporta error de claim y cancela

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `TestRetentionProcessorRunReportsClaimFailureAndStopsOnCancellation` ejerce `RetentionProcessor.Run` con repositorio de jobs fake que devuelve un error de claim sintético.
- Comprueba que el callback recibe ese error, cancela el contexto, `Run` termina sin error y no inicia procesamiento ni llama a borrar documentos.
- Caracterización de conducta existente; no cambio productivo. El callback de prueba no simula logging, alerta ni un worker conectado.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRunReportsClaimFailureAndStopsOnCancellation$' -count=1` — PASS.
- Suite de processor `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Procesos Go sin variables DB/PG, incluidas URLs de test, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

La revisión independiente read-only `deleg_2e46721c` aprobó sin hallazgos; confirmó callback único, recepción del error de claim y salida por cancelación sin procesar ni borrar. El repositorio fake no prueba fallos reales de PostgreSQL, log/alerta de producción, ejecución continuada del worker, retry durable, fencing, retención efectiva, GCS ni recuperación. No hubo SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o deploy.
