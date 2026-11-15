# Evidencia — VI-03, decisión de revisión HTTP→servicio (sintética)

- **Fecha:** 2026-11-11.
- **Checkout / rama / HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Worktree:** compartido y dirty; se preservaron los cambios previos.

## Alcance

- `internal/documentverification/transport/http/reviewer_review_service_test.go` añade `TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack`.
- POST atraviesa handler HTTP real, `ReviewerAccessService` y `Service.ReviewIdentity` real, con repositorio in-memory y authenticator sintéticos. El expediente tipado identity está enviado, revision `7`, tiene declaración/consentimiento sintéticos y los tres roles únicos limpios. El grant es identity y scope exactos.
- Una checklist completa con `names_match=false` más `DecisionNeedsEvidence`/`ReasonEvidenceIncomplete` produce estado `requiere_antecedentes`, revisión `8`, reviewer autenticado y readback HTTP con ETag fuerte. Se comprueban actor, conteo de una transición/dos lecturas (preflight y readback) y evento auditado con checklist.
- Se omite `document_expires_at_observed`; esta prueba no fija semántica civil-time/zona ni UTC. Es caracterización; no se observó RED ni cambió código productivo.

## Verificación

- `go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack$' -count=1` — PASS.
- `go test -race ./internal/documentverification/... -count=1` — PASS.
- `go vet ./internal/documentverification/...` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_review_service_test.go` y `git diff --check` — PASS.
- Las ejecuciones Go retiraron `ESPACIGO_DOCUMENT_VERIFICATION_TEST_DATABASE_URL`, `IDENTITY_TEST_DATABASE_URL`, `TEST_DATABASE_URL`, `ESPACIGO_DATABASE_URL`, `DATABASE_URL`, `PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD` y `PGPASSFILE`.

## Límites

Caracteriza el camino de decisión HTTP/servicio y el evento con repositorio in-memory. No prueba autenticación real, asignación real, atomicidad, adaptador/constraints PostgreSQL, persistencia, reinicio, integración BFF→Go ni custodia/lectura real de documentos. La revisión independiente `deleg_03475f7d` aprobó sin hallazgos ni sugerencias; verificó el fixture/flujo y que se omite la fecha observada sin fijar semántica temporal. No se ejecutaron SQL/DDL, migraciones, consultas/conexiones a bases, Cloud, proveedores ni datos reales. Sin commit, push, integración a `main` o despliegue.
