# Evidencia VI-02/VI-03 — aprobación con formato declarado «unsure»

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se amplió `TestReviewerAccessReviewIdentityPersistsObservedDecision` en `internal/documentverification/reviewer_access_test.go` con dos fixtures table-driven: formato declarado `new` (ya cubierto) y `unsure` (nuevo caso). Ambos recorren `NewService` y `ReviewerAccessService` reales con repository en memoria, grant identity sintético, tres archivos limpios sintéticos, checklist aprobado y observación manual `legacy` con vencimiento futuro.

La prueba comprueba que `unsure` no impide la aprobación cuando el revisor aporta un formato observado válido; que la declaración original permanece `unsure`; que el formato observado y el vencimiento quedan en el snapshot y evento de auditoría; que el checklist se conserva; y que `VerifiedUntil` no excede el vencimiento observado. Se conserva el caso previo `new` con observación `legacy`.

Es una caracterización con doubles en memoria. La primera ejecución pasó; no se observó RED ni se modificó código de producción.

## Verificación ejecutada

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification -run '^TestReviewerAccessReviewIdentityPersistsObservedDecision$' -count=1 -v` — PASS (`new`, `unsure`).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification -run '^TestReviewerAccessReviewIdentityPersistsObservedDecision$' -v` — PASS, 20/20 (40 subcasos).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — PASS.
- `gofmt -d internal/documentverification/reviewer_access_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/reviewer_access_test.go` — PASS.

## Límites

El repository y la autorización son fixtures sintéticos; esto no prueba persistencia/reinicio, atomicidad durable, grants efectivos, SQL/DDL, PostgreSQL, BFF/HTTP, GCS, Cloud ni proveedores externos. El subcorte solo afirma la semántica de dominio/composición con servicios reales y fakes en memoria. No ejecuté suites de integración.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.