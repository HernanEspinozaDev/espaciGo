# Evidencia VI-03 — aprobación bloqueada con formato observado indeterminado

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se amplió `TestReviewerIdentityApprovalWithUndeterminedFormatDoesNotMutate` en `internal/documentverification/transport/http/reviewer_approval_format_test.go` con casos `new` y `unsure` declarados. Ambos envían una aprobación por el `ReviewerCasesHandler` real, con `ReviewerAccessService` y servicio de dominio reales, repository en memoria, grant sintético, tres archivos limpios sintéticos y checklist completa, pero con `document_format_observed=undetermined`.

En ambos casos, el test exige `400 invalid request`, verifica que el handler alcanzó una sola transición con el actor esperado y confirma que el snapshot y la auditoría fake permanecen sin cambios. Esto completa el par de límites con la caracterización positiva: declaración `unsure` sí puede aprobarse si el revisor observa un formato concreto (`legacy`), pero no si la observación sigue indeterminada.

Es test-only de caracterización: la primera ejecución pasó; no se observó RED ni se modificó producción. El fake prueba únicamente la respuesta del handler y la ausencia de mutación en memoria, no rollback durable.

## Verificación ejecutada

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityApprovalWithUndeterminedFormatDoesNotMutate$' -count=1 -v` — PASS (`new`, `unsure`).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification/transport/http -run '^TestReviewerIdentityApprovalWithUndeterminedFormatDoesNotMutate$' -v` — PASS, 20/20 (40 subcasos).
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification/transport/http` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_approval_format_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/transport/http/reviewer_approval_format_test.go` — PASS.
- Búsqueda de `TestMain`/`init` en el paquete HTTP — sin coincidencias; se ejecutó solo el test focal, no la suite que contiene harness BFF.

## Límites

No demuestra persistencia/reinicio, atomicidad de auditoría, grants efectivos, SQL/DDL, PostgreSQL, BFF→Go, GCS, Cloud ni proveedores externos. No ejecuté suites de integración.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.