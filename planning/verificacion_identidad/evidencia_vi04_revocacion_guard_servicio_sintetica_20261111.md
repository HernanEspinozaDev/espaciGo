# Evidencia — VI-04, guard de revisión del servicio al revocar (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_revoke_service_revision_test.go` añade `TestReviewerRevokeStaleRevisionIsRejectedByServiceBeforeMutation`.
- POST atraviesa handler real, `ReviewerAccessService` y `Service.Revoke`, con snapshot identity aprobada revisión `8`, grant exacto sintético y revisión solicitada `7`.
- Un repositorio fake deliberadamente **no** prefiltra la revisión; invoca el callback recibido para que el guard `x.Revision != rev` de `Service.transact` produzca `ErrConflict`. La respuesta HTTP es `409 case revision conflict`; el fake confirma que el callback se invocó, pero no confirma Snapshot ni auditoría, y el handler no hace readback.
- Este test complementa `TestReviewerRevokeStaleRevisionReturnsConflictWithoutReadback`, cuyo fake corta la transición antes del callback. Ambos son caracterizaciones, sin RED ni cambio productivo.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerRevokeStaleRevisionIsRejectedByServiceBeforeMutation$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_revoke_service_revision_test.go` y `git diff --check` — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

El fake intencionalmente no implementa CAS; prueba el guard de dominio y mapeo HTTP, no el contrato de serialización/atomicidad del adaptador real, PostgreSQL, persistencia ni reinicio. Authenticator y almacenamiento también son sintéticos. La revisión independiente `deleg_bbcc1261` aprobó sin hallazgos ni sugerencias; confirmó que `Service.transact` rechaza revisión 7 frente a snapshot 8 antes de la mutación y que el handler evita readback. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
