# Evidencia — VI-05, claim space sin scope no elimina documentos (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/postgres/documentverification/retention_processor_test.go` añade `TestRetentionProcessorRejectsSpaceClaimWithoutScope`.
- Pasa un claim sintético `PurposeSpace` sin `SpaceID` al `RetentionProcessor` real, con job/storage fakes.
- El fake ejecuta el callback de operación fenced y observa un error y un retry sintéticos, cero completados y cero llamadas de eliminación identity/space.
- Es caracterización del guard de scope existente; sin RED/GREEN ni cambio productivo. El fake no verifica claim/update/lease ni retry durable.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorRejectsSpaceClaimWithoutScope$' -count=1` — PASS.
- Suite unitaria filtrada `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Se retiraron de los procesos Go `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

El retry/código son observaciones del fake, no resultado durable ni demora persistida. La revisión independiente read-only `deleg_7e9da59c` aprobó sin hallazgos ni sugerencias; confirmó que quitar el guard de `SpaceID` causa una llamada al delete fake y el test falla. No demuestra idempotencia, borrado físico/ausencia de generaciones, claim/lease/commit, PostgreSQL, GCS, backup/recuperación, métricas/alertas o privacidad de logs. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
