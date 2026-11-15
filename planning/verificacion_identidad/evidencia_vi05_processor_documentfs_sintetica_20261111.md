# Evidencia — VI-05, processor local elimina identity sin afectar space

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/postgres/documentverification/retention_processor_test.go` añade `TestRetentionProcessorDeletesIdentityFromPrivateStore`.
- Ejecuta `RetentionProcessor` real con `documentfs.Store` real sobre `t.TempDir()`; solo el job/claim/operación fenced es fake.
- Siembra documentos identity y space con el mismo owner/case/file sintético; procesa un claim identity y comprueba que identity ya no se lee, mientras los bytes space permanecen iguales.
- El fake observa `complete=1` y `retry=0`; esto no es un estado durable de job. Primera ejecución PASS, sin RED/GREEN ni cambio productivo.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorDeletesIdentityFromPrivateStore$' -count=1` — PASS.
- Suite unitaria de processor filtrada `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Se retiraron de los procesos Go `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

La revisión independiente read-only `deleg_ef640004` aprobó sin hallazgos de seguridad/lógica; confirmó cobertura real del camino processor→documentfs y preservación del objeto space con IDs coincidentes. Precisó que el job fake no demuestra claim/fencing/completion/retry durable. No prueba claim/lease/CAS/retry/commit en PostgreSQL, durabilidad, almacenamiento cloud, GCS/IAM, borrado forense, backup/recuperación, métricas/alertas ni logs/exportaciones. El resultado aplica al processor y filesystem temporal local con job fake, no a un flujo de producción. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
