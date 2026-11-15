# VI-03 — recuperación explícita tras conflicto de decisión

- **Fecha:** 2026-11-12.
- **Frontend checkout:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Plan checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Solo se amplió el test/harness Playwright ya existente; no se cambió runtime. Se preservaron los cambios previos de los worktrees dirty.

## Escenario sintético

`requires readback and a deliberate new submit after a review conflict` produce primero un POST 409 y mantiene el caso mock `en_revision`, revisión 7. La UI oculta la selección hasta un GET explícito. Ese GET devuelve el estado aún revisable; la prueba comprueba que el formulario renace vacío y que no se produjo un segundo POST. Solo un nuevo clic deliberado envía otra decisión con `expected_revision: 7`; el mock responde 200 y la UI presenta `requiere_antecedentes`. Se verifican exactamente dos POST y tres GET de detalle (carga, reconciliación y readback del éxito).

Fue caracterización en verde en la primera ejecución; no hubo RED/GREEN ni cambio productivo.

## Verificación

- `pnpm typecheck` — PASS.
- `pnpm lint` — PASS.
- `pnpm test` — PASS, 103/103.
- `pnpm format:check` y `git diff --check` — PASS.
- Playwright completo `tests/reviewer-queue.spec.ts` — PASS, 12/12 Chromium local, APIs interceptadas. Incluye aislamiento de navegación, respuesta perdida, refresco fallido y éxito, recuperación 409→GET→nuevo submit y expiración de sesión.

No se reconstruyó el standalone porque esta ampliación solo tocó pruebas; se usó el build verificado de la implementación inmediatamente anterior. No cambió OpenAPI y no se repitió `api:check`.

## Límites

La secuencia 409/200, el estado del caso y los GET son fixtures; no prueban semántica HTTP/Go, grants/asignación, BFF→Go, CAS efectivo, transacción, auditoría ni persistencia. No ejecuté SQL/DDL ni accedí a bases persistentes o Cloud; V95/V96 siguen sin aplicar. Sin datos reales, commit, push, integración a `main` o despliegue. VI-03 continúa parcial.
