# VI-03 — continuidad del propósito en readbacks de mutación reviewer

- **Fecha:** 2026-11-12.
- **Checkout/rama/HEAD:** `/home/nandev/Documentos/projects/espaciGo-main-sync` / `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Se preservó el worktree compartido dirty; sin commit.

## Cambio y cobertura

El handler ahora lleva el propósito esperado hasta el GET de readback posterior a una decisión o revocación: review usa el propósito del preflight autorizado por el servicio; revoke usa el propósito enviado y validado por `Revoke`. El readback debe coincidir tanto en ID como en propósito, aunque el propósito retornado sea otro valor permitido (`identity`↔`space`). Si no coincide, devuelve 404 neutral antes de ETag o serialización.

- `TestReviewerDecisionReadbackRejectsMismatchedCaseOrPurpose` añade el caso de readback con el mismo ID y propósito válido `space` después de un preflight `identity`.
- `TestReviewerRevokeReadbackRejectsDifferentAllowedPurpose` prueba el mismo límite en la ruta de revocación.
- Ambos reprodujeron antes del cambio un 200 con ETag `"8"` y DTO del propósito `space`; después pasan esperando 404 sin ETag. Los casos previos de ID diferente y propósito no admitido siguen cubiertos.

Esta validación evita que una respuesta de otro propósito se presente como resultado de la mutación solicitada. No revierte una mutación que el backend hubiera confirmado antes del readback fallido; la UI debe tratar el resultado como incierto y reconciliar explícitamente, nunca repetir automáticamente.

## Verificación

Con entorno aislado `env -i` (solo `PATH`, `HOME`, `TMPDIR`; sin variables de DB/opt-in):

- Pruebas focales de decisión/revocación y cambio de propósito — PASS después del cambio.
- `go test -race ./internal/documentverification ./internal/documentverification/transport/http -count=1` — PASS.
- `go vet ./internal/documentverification ./internal/documentverification/transport/http` — PASS.
- `go build -o /dev/null ./cmd/api` — PASS.
- `gofmt`, `gofmt -d` y `git diff --check` — PASS.

## Límites

Mocks/servicio secuencial sintético: no demuestra que una mutación se haya confirmado, grants/autorización efectiva, auditoría, SQL, transacción, persistencia, PostgreSQL, BFF o Cloud. No se ejecutó SQL/DDL ni se accedió a bases persistentes; V95/V96 no se aplicaron. Sin datos reales, commit, push, integración a `main` o despliegue.
