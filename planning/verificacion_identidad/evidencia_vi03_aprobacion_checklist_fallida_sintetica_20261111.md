# Evidencia — VI-03, checklist negativa impide aprobar (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_approval_checklist_test.go` añade `TestReviewerIdentityApprovalWithNegativeChecklistDoesNotMutate`.
- POST atraviesa handler HTTP real, `ReviewerAccessService` y `Service.ReviewIdentity` con caso identity tipado enviado, tres roles únicos limpios, grant identity exacto, formato observado `new` y checklist completa salvo `names_match=false`.
- Se incluye una fecha no cero como sentinel para aislar la condición de checklist; no se afirma interpretación, conversión, comparación ni persistencia de esa fecha.
- La aprobación se rechaza con `400 invalid request`; el fake ve el intento de transición, una lectura previa y cero readbacks, y no confirma el Snapshot/auditoría candidato. La checklist es completa, con `names_match=false` y los otros seis campos verdaderos, por lo que el caso falla específicamente el requisito `AllPassed`. Esto caracteriza la rama en memoria, no rollback PostgreSQL.
- Sin RED ni cambio productivo.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityApprovalWithNegativeChecklistDoesNotMutate$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_approval_checklist_test.go` y `git diff --check` — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

El sentinel no resuelve el contrato civil-time/zona/UTC. Authenticator, repositorio y almacenamiento son sintéticos; no se demuestra autenticación/grants reales, atomicidad, persistencia, adaptador/constraints PostgreSQL, reinicio, BFF→Go ni lectura real de documentos. La revisión independiente `deleg_9e6a8d93` aprobó sin hallazgos de seguridad/lógica ni sugerencias; confirmó que solo un campo de checklist falla, no hay readback y la evidencia no sobreafirma rollback. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
