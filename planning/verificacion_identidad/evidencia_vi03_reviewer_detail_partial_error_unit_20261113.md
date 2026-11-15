# Evidencia VI-03 — detalle reviewer parcial acompañado de error

Fecha: 2026-11-13 (hora local `-03:00`)

## Alcance

Se añadió `TestReviewerIdentityDetailDoesNotSerializePopulatedValueWithError` en `internal/documentverification/transport/http/reviewer_detail_partial_error_test.go`. El servicio fake entrega al handler un `ReviewerCaseDetail` con snapshot y declaración de identidad sintéticos completos, y simultáneamente `dv.ErrDocumentStorage`.

El contrato esperado es respuesta neutral `503 temporarily unavailable`, sin ETag ni datos de nombre, apellido, RUT o nacimiento. También se exige una sola llamada a `LoadDetail`. Así se caracteriza que el handler ignora cualquier valor parcial cuando el servicio informa error y no serializa datos privados.

El test pasó en su primera ejecución: es caracterización del manejo de errores existente, no se observó RED ni se cambió producción. El fake no prueba que un adapter real produzca simultáneamente valor y error.

## Verificación ejecutada

- Preflight del paquete HTTP: sin `TestMain` ni `init`; el test solo llama al handler con autenticador/servicio fakes y datos sintéticos.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test ./internal/documentverification/transport/http -run '^TestReviewerIdentityDetailDoesNotSerializePopulatedValueWithError$' -count=1 -v` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=20 ./internal/documentverification/transport/http -run '^TestReviewerIdentityDetailDoesNotSerializePopulatedValueWithError$'` — PASS.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification/transport/http` — PASS.
- `gofmt -d internal/documentverification/transport/http/reviewer_detail_partial_error_test.go` — sin diferencias.
- `git diff --check -- internal/documentverification/transport/http/reviewer_detail_partial_error_test.go` — PASS.

## Límites

No se ejecutó la suite completa del paquete ni harness BFF/integración. No se prueban `ReviewerAccessService`, grants efectivos, DB, SQL/DDL, persistencia, GCS, Cloud ni proveedores externos.

- Checkout: `/home/nandev/Documentos/projects/espaciGo-main-sync`
- Rama: `codex/marketplace/fix/kyc-06-save-identity-first`
- HEAD observado: `274f37e483a078890e92382a159fa49fd23c2fe8`

Sin SQL/DDL, acceso a DB/Cloud, datos reales, commit, push, deploy ni integración a `main`; se preservó el estado previo del worktree.