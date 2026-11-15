# Evidencia — VI-04, conflicto de revisión al revocar (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_revoke_conflict_test.go` añade `TestReviewerRevokeStaleRevisionReturnsConflictWithoutReadback`.
- POST al handler real atraviesa `ReviewerAccessService` y `Service.Revoke` con revisor/grant identity sintéticos exactos. El snapshot in-memory aprobado está en revisión `8`; la solicitud usa revisión obsoleta `7`.
- El fake del repositorio retorna `ErrConflict` antes de ejecutar el callback; el handler responde `409 case revision conflict`, no hace readback y las aserciones comprueban que Snapshot y auditoría no cambian.
- Caracterización de comportamiento existente, sin RED ni cambio productivo.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerRevokeStaleRevisionReturnsConflictWithoutReadback$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_revoke_conflict_test.go`, `git diff --check` y whitespace — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Prueba únicamente handler/servicio real con authenticator y repositorio sintéticos en memoria; no valida la serialización CAS ni el HTTP/SQL del adaptador PostgreSQL, persistencia ni reinicio. La revisión independiente `deleg_d3298cca` aprobó sin hallazgos ni sugerencias; confirmó que el fake devuelve `ErrConflict` antes del callback y que el handler omite readback. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
