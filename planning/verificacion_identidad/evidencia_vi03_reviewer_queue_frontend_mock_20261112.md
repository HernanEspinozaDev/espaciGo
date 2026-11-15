# Evidencia — VI-03, cola Next de revisión de identidad (mock local)

- **Fecha:** 2026-11-12.
- **Checkout / rama / HEAD base:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes` / `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`.
- **Estado:** cambios sin commit en el worktree; no se publicó ni integró. El checkout backend compartido no se modificó para ejecutar estas pruebas.

## Alcance

Se alineó el OpenAPI/cliente Next con `GET /document-verification/reviewer/cases` y se implementó la ruta `/mi-cuenta/revisiones`. La feature tiene parser de resumen mínimo, puerto, fachada, adaptador OpenAPI y vista. Solo consulta asignaciones servidas por el backend; descarta otros propósitos y no retiene en la vista declaración, datos identificatorios ni claves de almacenamiento. No agrega `review_identity` a capabilities de sesión ni concede autorización de revisor. La navegación de cuenta incluye la ruta y `return_to` la permite.

El recorrido Playwright usa Chromium local y mocks sintéticos interceptados para toda llamada `/api/backend/**`: sesión/login y lista de asignaciones. Verifica el Bearer sintético, estado de carga, presentación de un solo caso identity con estado/revisión/fecha civil, ausencia del caso space y de los campos privados, jerarquía de landmarks y enlace de navegación. Al expirar la sesión sintética, la vista pasa a no disponible y desaparece la fila; dos pruebas adicionales verifican el estado vacío (`items: []`) y un 503 neutro sin mostrar el detalle del servidor. No hay BFF→Go en este ensayo.

## Gates locales observados

- `pnpm api:check`: PASS; tipos generados, allowlist e inventario actuales, 186 operaciones.
- `node --test tooling/*.test.mjs`: 82/82 PASS, 0 fallos, 0 omitidos. Node emitió el warning preexistente `MODULE_TYPELESS_PACKAGE_JSON` al cargar `src/lib/api/clp.ts`.
- `pnpm lint`: PASS.
- `pnpm typecheck`: PASS.
- `pnpm format:check`: PASS.
- `pnpm build`: PASS con Next.js 16.4.3; `/mi-cuenta/revisiones` aparece en el artefacto standalone; 31/31 páginas generadas.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/reviewer-queue.spec.ts --reporter=list --output=/home/nandev/.hermes/cache/scratch/reviewer-queue-playwright-states-output`: PASS, Chromium local 3/3, sobre el standalone construido y API interceptada. Los tests cubren carga→resultado→expiración de sesión, cola vacía y 503 sin filtración de detalle.
- `git diff --check`: PASS.

## Límites y gates pendientes

La evidencia acredita contrato y comportamiento UI frente a fixtures sintéticos, no autorización efectiva del servidor, identidad del revisor, MFA, BFF→Go, PostgreSQL, transacción, persistencia/reinicio, storage, Cloud ni datos reales. No se ejecutó SQL/DDL ni se accedió a bases persistentes o Cloud. No se probó pantalla online, navegador del usuario, proveedor externo, deploy, commit, push o integración a `main`. La cobertura visual se limita a estas respuestas mock; no incluye UI de detalle/decisión, visor de evidencias, MFA ni revisión real. VI-03 sigue parcial hasta completar sus gates de integración/privacidad/operación autorizados.
