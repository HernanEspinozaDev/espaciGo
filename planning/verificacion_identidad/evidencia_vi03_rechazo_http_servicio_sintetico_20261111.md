# Evidencia — VI-03, rechazo HTTP→servicio (sintético)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_reject_service_test.go` añade `TestReviewerIdentityRejectHTTPUsesActualServiceAndReadsBack`.
- POST atraviesa handler HTTP real, `ReviewerAccessService` y `Service.ReviewIdentity` real, con expediente sintético tipado enviado, revisión `7`, declaración/consentimiento sintéticos y tres roles únicos limpios; el grant identity/scope coincide con titular y principal.
- `DecisionReject` con `ReasonDocumentInvalid`, formato observado `new` y checklist completa con `names_match=false` produce `rechazada`, revisión `8`, reviewer autenticado y readback HTTP con ETag fuerte. Se afirman campos del evento auditado, preservación de timestamps de verificación en cero y una transición/dos lecturas.
- Se omite la fecha observada; el test no decide semántica civil-time/zona ni UTC. Caracterización sin RED ni cambio productivo.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityRejectHTTPUsesActualServiceAndReadsBack$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_reject_service_test.go` y `git diff --check` — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Solo caracteriza decisión/audit/readback con authenticator y repositorio sintéticos en memoria. No demuestra autenticación/grants reales, atomicidad, adaptador/constraints PostgreSQL, persistencia, reinicio, integración BFF→Go ni custodia/lectura real de documentos. La revisión independiente `deleg_35f509b0` aprobó sin hallazgos ni sugerencias; validó fixture, grant, decisión, audit, readback y omisión de fecha. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
