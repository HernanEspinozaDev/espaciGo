# VI-03 — self-review identity bloqueada antes de mutar el caso

**Fecha:** 2026-11-13
**Checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
**Rama / HEAD:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`

## Alcance

Se añadió `TestIdentityReviewRejectsSelfReviewWithoutMutatingCaseOrAudit` en `internal/documentverification/reviewer_identity_self_review_test.go`. El test pasa por `Service.ReviewIdentityWithReason` con un caso identity sintético cuyo titular es también el revisor autenticado y un grant identity exacto. Comprueba `ErrSelfReview`, conservación completa del snapshot y la auditoría, y una sola transición con el actor autenticado.

Es una caracterización test-only del guard existente para la ruta de decisión identity. La prueba pasó en el primer run; no se afirma RED/GREEN ni se modificó código de producción.

## Verificación

- Focal con `-race`: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification -run '^TestIdentityReviewRejectsSelfReviewWithoutMutatingCaseOrAudit$' -v` — **1/1 PASS**.
- Paquete completo con `-race`: `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go test -race -count=1 ./internal/documentverification` — **PASS**.
- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" go vet ./internal/documentverification` — **PASS**.
- `gofmt` y `git diff --check` — limpios.

El paquete usa un repositorio en memoria. No se habilitaron variables de conexión ni se ejecutaron SQL/DDL, migraciones, pruebas conectadas, Cloud, proveedores externos o sitios públicos.

## Límites

No prueba grants persistidos, MFA, asignación concurrente, transacciones/atomicidad, PostgreSQL, autorización efectiva ni aceptación de producto. VI-03 permanece parcial por sus gates de persistencia y autorización efectiva. Sin commit, push, despliegue o integración a `main`.
