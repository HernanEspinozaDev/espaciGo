# Evidencia — VI-04, revocación repetida de caso terminal sintético

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_revoke_terminal_test.go` añade `TestReviewerRevokeOfAlreadyRevokedCaseIsNeutralWithoutReadback`.
- La solicitud `POST /revoke` atraviesa el handler real, `ReviewerAccessService` y `Service.Revoke`; el actor tiene un grant identity sintético exacto, pero el snapshot parte en estado terminal `revocada` y revisión `8`.
- El servicio deniega una segunda transición; el handler devuelve `404 not found` neutral y no hace lectura de readback. El fake afirma que estado/revisión/revisor/timestamps/auditoría quedan intactos, `SupersededAt` sigue nulo y no se repite la mutación.
- Es caracterización, sin RED ni cambios productivos. La primera versión del test pasó; tras la revisión independiente se añadió la aserción explícita de `SupersededAt == nil` y se repitieron los gates.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerRevokeOfAlreadyRevokedCaseIsNeutralWithoutReadback$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_revoke_terminal_test.go`, `git diff --check` y whitespace del archivo nuevo — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Solo caracteriza la decisión del servicio real y el cableado HTTP con authenticator/repositorio sintéticos. No demuestra asignación real, transacción/atomicidad PostgreSQL, persistencia ni reinicio. La revisión independiente inicial `deleg_235b9d0d` aprobó sin hallazgos de seguridad o lógica y sugirió precisar que `SupersededAt` es nulo; la aserción se incorporó y la revisión de seguimiento `deleg_9649a517` aprobó sin hallazgos ni sugerencias. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
