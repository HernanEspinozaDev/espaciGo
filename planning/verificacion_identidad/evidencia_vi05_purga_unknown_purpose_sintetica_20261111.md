# Evidencia — VI-05, propósito desconocido no elimina documentos (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/postgres/documentverification/retention_processor_test.go` añade `TestRetentionProcessorRejectsUnknownPurposeWithoutDeleting`.
- La prueba envía un propósito desconocido sintético al `RetentionProcessor` real, con job y almacenamiento fake.
- El fake ejecuta el callback dentro de su operación fenced y observa error, un intento de retry sintético y cero completados; los contadores de eliminación identity y space permanecen en cero.
- Es caracterización de comportamiento existente; no hubo RED/GREEN ni cambio productivo. El fake no ejecuta el repository/adaptador ni prueba retry durable en PostgreSQL.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRejectsUnknownPurposeWithoutDeleting$' -count=1` — PASS.
- Processor unitario `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Se retiraron de los procesos Go `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

No demuestra claim/lease/commit/retry durable, idempotencia real del adapter, borrado físico, ausencia de generaciones, métricas/alertas, PostgreSQL, bucket/GCS, recuperación/backup ni logs/exportaciones. La revisión independiente read-only `deleg_39db52eb` aprobó sin hallazgos de seguridad o lógica; precisó que el contador retry y el código observado son solo conducta del fake, sin retry durable, demora persistida o limpieza de lease demostradas. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
