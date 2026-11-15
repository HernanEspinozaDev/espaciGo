# VI-03 — aislamiento al cambiar de expediente con una decisión pendiente

- **Fecha:** 2026-11-12.
- **Frontend checkout:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Plan/evidencia checkout:** `/home/nandev/Documentos/projects/espaciGo-main-sync`, rama `codex/marketplace/fix/kyc-06-save-identity-first`, HEAD `274f37e483a078890e92382a159fa49fd23c2fe8`.
- Worktrees compartidos con cambios preexistentes; se preservaron. Este subcorte solo añadió cobertura al escenario Playwright existente, sin cambiar código de producción.

## Cobertura

`frontend/tests/reviewer-queue.spec.ts` ahora prepara dos expedientes identity sintéticos. Mantiene pendiente el POST de decisión para el expediente A, navega a la cola y abre el B, comprueba que el formulario de B tiene sus valores iniciales y luego libera una respuesta 200 tardía del A. Verifica que B siga visible, que no aparezca la confirmación de A, que haya exactamente un POST para A y que el detalle de cada expediente se haya solicitado una sola vez. Así caracteriza el límite de selección visible y evita atribuir la respuesta tardía a otro expediente.

Es una prueba Playwright local con respuestas/API interceptadas, no una prueba de servicios reales. La primera ejecución de desarrollo falló por selector ambiguo (`getByLabel("Decisión")` también coincidía con el formulario); se acotó al formulario y la regresión final pasó. No se observó un fallo del producto ni se afirma RED/GREEN.

## Verificación

- `pnpm typecheck` — PASS.
- `pnpm lint` — PASS.
- `pnpm test` — PASS, 103/103.
- `pnpm format:check` — PASS.
- `git diff --check` — PASS.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/reviewer-queue.spec.ts --reporter=list --workers=2` — PASS, 10/10 en Chromium local con APIs interceptadas.

No se repitió `pnpm build`: el único cambio de esta subfase es un archivo de prueba y Playwright ejecutó el standalone previamente construido. No hubo cambio de runtime/OpenAPI, por lo que no se repitió `api:check`.

## Límites

La respuesta 200 y el readback son sintéticos. Esto no prueba si un POST abortado/cancelado alcanzó Go, autorización/asignación, auditoría, CAS/atomicidad, BFF→Go, PostgreSQL ni persistencia. No ejecuté SQL/DDL ni accedí a bases persistentes o Cloud; V95/V96 no se aplicaron. Sin datos reales, commit, push, integración a `main` o despliegue. VI-03 permanece parcial.
