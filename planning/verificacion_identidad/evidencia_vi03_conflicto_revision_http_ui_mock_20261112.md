# VI-03 — contrato local del conflicto de revisión

- **Fecha:** 2026-11-12.
- **Backend:** rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- **Frontend:** rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- Se añadieron caracterizaciones de tests en ambos worktrees compartidos; no cambió código productivo. Los cambios preexistentes se preservaron.

## Contrato probado

La prueba HTTP Go `TestReviewerIdentityReviewConflictReturnsNeutral409WithoutReadback` envía una decisión identity sintética. El servicio fake permite el preflight y devuelve `dv.ErrConflict` al registrar la decisión. El handler responde `409` con cuerpo exacto `case revision conflict\n`, sin ETag, manteniendo `Cache-Control: private, no-store`; el fake confirma preflight y un intento de decisión, sin readback posterior.

El fixture Playwright de `requires readback and a deliberate new submit after a review conflict` ahora devuelve ese mismo 409 de texto plano —no un envelope JSON inventado— y comprueba status/body. Tras el conflicto, la UI exige un GET explícito; la prueba confirma que no reenvía sola y que solo un nuevo clic deliberado genera el segundo POST.

## Verificación local

- Go: prueba focal, repetida bajo `-race`, `go vet` del paquete HTTP, `gofmt -d` y `git diff --check` — PASS.
- Frontend: Playwright completo `tests/reviewer-queue.spec.ts` — 12/12 PASS; `pnpm typecheck`, `pnpm lint`, `pnpm test` — 103/103, `pnpm format:check` y `git diff --check` — PASS.
- No se reconstruyó el standalone: únicamente cambiaron tests y fixtures; el build de la implementación se verificó antes de esta ampliación.

La prueba Go usa un servicio fake y la prueba de navegador intercepta API/auth; el test demuestra el mapping del conflicto y compatibilidad de presentación con el contrato textual, no una llamada integrada BFF→Go. No prueba CAS/transacción real, autorización/asignación, auditoría durable ni PostgreSQL. No ejecuté SQL/DDL ni accedí a bases persistentes o Cloud; V95/V96 no aplicadas. Sin datos reales, commit, push, integración a `main` o despliegue. VI-03 continúa parcial.
