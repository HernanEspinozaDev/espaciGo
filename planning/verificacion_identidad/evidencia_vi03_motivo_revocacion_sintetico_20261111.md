# Evidencia — VI-03, validación del motivo de revocación sintético

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty antes de este subcorte; se preservaron los cambios ajenos.

## Alcance

- `internal/documentverification/service_test.go` reemplaza la caracterización de un motivo libre por `TestRevokeRejectsInvalidReasonsBeforeRepository`, con tres entradas sintéticas inválidas: vacío, código no admitido y texto libre.
- Cada caso parte de una aprobación fixture válida y un grant identity ligado al revisor/titular esperados, para aislar la validación del motivo. El test requiere `ErrInvalid`, contador del repositorio sin incremento y Snapshot/auditoría sin cambios.
- Es una caracterización del comportamiento existente: pasó en la primera ejecución. No se observó RED y no cambió código productivo.

## Verificación local

- Test focal `TestRevokeRejectsInvalidReasonsBeforeRepository` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS (`documentverification`, `inspection` y `transport/http`).
- `go vet ./internal/documentverification/...` — PASS.
- `gofmt -d internal/documentverification/service_test.go` y `git diff --check` — PASS, sin salida.
- Los comandos Go retiraron las variables `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Esta cobertura solo demuestra el rechazo previo a la llamada del repositorio fake y la ausencia de mutación en memoria. No demuestra el comportamiento de una transacción PostgreSQL ni persistencia, autorización integrada o reinicio. No se ejecutaron SQL/DDL, migraciones, conexiones/consultas a bases, Cloud, proveedores externos ni datos reales. Sin commit, push, integración a `main` o despliegue. Revisión independiente read-only `deleg_f7967567`: aprobada sin hallazgos de seguridad o lógica; confirmó que el fixture tiene un grant ligado al scope correcto y que los asserts verifican ausencia de llamada al repositorio y de mutación.
