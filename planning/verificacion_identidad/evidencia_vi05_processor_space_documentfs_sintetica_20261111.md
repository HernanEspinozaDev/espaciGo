# Evidencia — VI-05, processor local elimina space sin afectar identity

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/adapters/postgres/documentverification/retention_processor_test.go` añade `TestRetentionProcessorDeletesSpaceFromPrivateStore`.
- Ejecuta el `RetentionProcessor` y `documentfs.Store` reales en `t.TempDir()`; el adaptador de jobs permanece fake.
- Siembra identity y space con el mismo owner/case/file sintético; un claim `PurposeSpace` borra space, `ReadSpaceDocument` retorna `os.ErrNotExist` y los bytes identity permanecen iguales.
- El fake ve `complete=1`, `retry=0`; no es completion durable. Caracterización existente; primera ejecución PASS, sin RED/GREEN ni cambio productivo.

## Verificación

- Test focal `go test ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessorDeletesSpaceFromPrivateStore$' -count=1` — PASS.
- Suite unitaria de processor filtrada `go test -race ./internal/adapters/postgres/documentverification -run '^TestRetentionProcessor' -count=1` — PASS.
- `go vet ./internal/adapters/postgres/documentverification`, gofmt focal y `git diff --check` — PASS.
- Se retiraron de los procesos Go las variables DB/PG, incluidas `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

La revisión independiente read-only `deleg_35c98813` aprobó sin hallazgos de seguridad/lógica; confirmó eliminación space y preservación de identity con IDs owner/case/file coincidentes. Precisó que `retentionFake` suministra el claim y simula completion en memoria, sin demostrar claim/fence/commit durable. No prueba claim/lease/fencing/completion/retry durable, PostgreSQL, GCS/IAM, borrado forense, backup/recuperación ni worker real. Es cobertura del processor y filesystem temporal local, con job fake; no evidencia de flujo de producción. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
