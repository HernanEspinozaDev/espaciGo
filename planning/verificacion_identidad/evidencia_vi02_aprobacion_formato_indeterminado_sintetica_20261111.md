# Evidencia — VI-02/VI-03, formato observado indeterminado impide aprobar

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_approval_format_test.go` añade `TestReviewerIdentityApprovalWithUndeterminedFormatDoesNotMutate`.
- POST recorre handler real, `ReviewerAccessService` y `Service.ReviewIdentity` con caso identity tipado enviado, tres roles únicos limpios, grant identity exacto y checklist completa y positiva.
- El formato observado enviado es `DocumentFormatUndetermined`. Se proporciona una fecha no cero solo como sentinel sintético para que la prueba aísle el rechazo del formato indeterminado; no se afirma interpretación, conversión ni persistencia de esa fecha.
- El servicio rechaza la aprobación, el handler responde `400 invalid request`, no hay readback y el repositorio fake no confirma el Snapshot/auditoría candidato. Caracterización sin RED ni cambio productivo; esto no prueba rollback de base de datos.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityApprovalWithUndeterminedFormatDoesNotMutate$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_approval_format_test.go` y `git diff --check` — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

El sentinel de fecha no cierra el contrato civil-time/zona/UTC pendiente. El resto es prueba con authenticator/repositorio in-memory; no demuestra persistencia, atomicidad, adaptador/constraints PostgreSQL, reinicio ni BFF→Go. La revisión independiente `deleg_fc639092` aprobó sin hallazgos ni sugerencias; confirmó fixture, recorrido, rechazo y límites del sentinel. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
