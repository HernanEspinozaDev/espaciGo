# Evidencia VI-03 — request HTTP de antecedentes con formato indeterminado

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se amplió `TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack` en `internal/documentverification/transport/http/reviewer_review_service_test.go` con dos fixtures sintéticos:

- Declarado/observado `new` (control existente).
- Declarado `unsure`, observado `undetermined`, con los checks de legibilidad y verificación de formato en falso.

Ambos envían `DecisionNeedsEvidence` por el `ReviewerCasesHandler` real y atraviesan `ReviewerAccessService` y el servicio de dominio reales con autenticador/grant y repository en memoria. Se comprueba HTTP 200 y ETag de revisión 8, readback con estado `requiere_antecedentes` y ambos formatos, ausencia de vencimiento observado en DTO/snapshot/auditoría, y evento con motivo `antecedentes_incompletos` y checklist completa pero no aprobatoria. Se mantienen las aserciones de una transición y las lecturas de preflight/readback.

Es una caracterización con fakes: el nuevo subcaso pasó en la primera ejecución; no se observó RED ni se modificó producción.

## Verificación ejecutada

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack$' -count=1 -v` — PASS (`known format`, `uncertain format requests evidence`).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification/transport/http -run '^TestReviewerIdentityNeedsEvidenceHTTPUsesActualServiceAndReadsBack$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification/transport/http` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_review_service_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/transport/http/reviewer_review_service_test.go` — PASS.

## Límites

No se ejecutó la suite completa del paquete, que incluye harness de BFF. Los fakes no prueban autorización efectiva, BFF→Go, SQL/DDL, PostgreSQL, persistencia/reinicio, atomicidad durable, storage, GCS, Cloud ni proveedores externos.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.