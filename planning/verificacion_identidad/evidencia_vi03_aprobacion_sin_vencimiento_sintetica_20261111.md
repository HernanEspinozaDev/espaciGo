# Evidencia — VI-03, aprobación sin vencimiento observado (sintética)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_approval_expiry_test.go` añade `TestReviewerIdentityApprovalWithoutObservedExpiryDoesNotMutate`.
- POST atraviesa handler real, `ReviewerAccessService` y `Service.ReviewIdentity` con caso identity tipado enviado, tres roles únicos limpios y grant sintético exacto. Se envía formato observado `new` y checklist completa con todos sus valores verdaderos, pero se omite `document_expires_at_observed`.
- El servicio rechaza la aprobación; el handler responde `400 invalid request`. Se comprueba el intento de transición, una lectura previa y cero readback. El fake aplica el callback sobre una copia y solo confirma Snapshot/auditoría si el callback termina sin error; su estado intacto solo demuestra ausencia de commit en el fake, no rollback PostgreSQL.
- No se envía ni afirma una fecha. Caracterización fail-closed, sin RED ni cambio productivo, y no fija semántica civil-time/zona/UTC.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityApprovalWithoutObservedExpiryDoesNotMutate$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_approval_expiry_test.go` y `git diff --check` — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Solo caracteriza la decisión HTTP/servicio con authenticator y repositorio sintéticos in-memory; no demuestra persistencia, atomicidad, adaptador/constraints PostgreSQL, BFF→Go ni reinicio. La revisión independiente `deleg_c97872fb` aprobó sin hallazgos de seguridad o lógica y sugirió precisar el límite del fake; se aclara que Snapshot/auditoría intactos solo prueban que el repositorio in-memory no confirmó el candidato, no rollback de base de datos. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
