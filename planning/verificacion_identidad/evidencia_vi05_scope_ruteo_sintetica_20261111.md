# Evidencia — VI-05, scope exacto de borrado por propósito (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- Se fortaleció `TestRetentionProcessorRoutesIdentityAndSpaceDeletes` en `internal/adapters/postgres/documentverification/retention_processor_test.go`; el fake ahora captura los argumentos de ambas operaciones de borrado.
- Con IDs sintéticos diferenciados, el caso identity comprueba que la llamada recibe exactamente subject/case/file del claim y que no se invoca el método de espacios.
- El caso space comprueba que recibe exactamente subject/space/case/file y que no se invoca el método identity.
- Es caracterización del ruteo de `RetentionProcessor` existente; la primera ejecución pasó, no hubo RED/GREEN ni cambio productivo. Los argumentos del fake no demuestran borrado ni scoping en un adapter de almacenamiento real.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRoutesIdentityAndSpaceDeletes$' -count=1` — PASS.
- Suite unitaria filtrada `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Los procesos Go no recibieron variables de conexión DB/PG (incluyendo `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`).

## Límites

La revisión independiente read-only `deleg_0202b32a` aprobó sin hallazgos de seguridad o lógica; confirmó que los IDs diferenciados detectan ruteo cruzado y argumentos incorrectos/omitidos. Aclaró que los argumentos capturados por el fake no prueban borrado, éxito durable ni aislamiento en almacenamiento real. No prueba borrado físico, IAM/autorización, generaciones, idempotencia, transacción/fence, claim/lease/retry durable, PostgreSQL, bucket/GCS, backup/recuperación, métricas/alertas o privacidad de logs. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
