# VI-04 — recuperación de carga identity incierta y liberación del File local

**Fecha:** 2026-11-13 (`America/Santiago`)

## Alcance del corte

Se corrigió en el checkout frontend autorizado `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, base HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a` (worktree compartido y con cambios previos ajenos a este corte).

- `src/features/document-verification/ui/document-verification-view.tsx`: después de una lectura explícita exitosa del caso seleccionado, `handleRefresh` libera el `File` local si el estado autoritativo ya no es editable. Si el caso sigue editable, se conserva el archivo para un reintento deliberado; `needsRefresh` sigue bloqueando carga hasta la actualización.
- `tests/identity-upload-uncertain-recovery.spec.ts`: recorrido Playwright local con login, caso y tres roles limpios sintéticos. El POST de carga recibe `409`; no hay reintento automático. La primera lectura explícita devuelve `en_revision`, ocultando el formulario; la siguiente devuelve `requiere_antecedentes`. Se comprueba que el control reaparece sin archivo seleccionado, que «Cargar evidencia» queda deshabilitado y que solo hubo un POST de carga.
- `tooling/reviewer-queue-facade.test.mjs`: se reemplazó la fecha de vencimiento fija `2026-11-12` por el día civil siguiente en `America/Santiago`. La prueba unitaria de aprobación fallaba el `2026-11-13` porque su fixture ya estaba vencido y el dominio rechazaba correctamente la decisión. Es un ajuste de fixture, no un cambio de comportamiento productivo.

## Ciclo RED/GREEN

- **RED observado:** tras añadir la prueba antes del cambio de UI, el test terminó en la aserción esperada: el botón «Cargar evidencia» estaba habilitado después de la secuencia `409` → `en_revision` → `requiere_antecedentes`, porque el estado React aún conservaba el `File` aunque el `<input>` se hubiese desmontado. No fue un fallo de setup.
- **GREEN:** tras liberar el estado local al confirmar un caso no editable, la prueba nueva y `owner validates identity evidence and submits only three clean roles` pasaron juntas: 2/2.

## Verificación ejecutada

Checkout frontend, con entorno local aislado; build y Playwright sobre el servidor standalone en `127.0.0.1:4203`:

- `pnpm build` — PASS después del cambio de UI.
- Playwright focal (`uncertain identity upload` y flujo owner existente) — 2 pasaron.
- `pnpm test:e2e --output test-results/identity-phase-suite` — 84 pasaron, 45 omitidos, 129 totales. Las pruebas `@native` quedaron omitidas; no se seleccionó API nativa ni base de datos.
- `pnpm test` — 107 pasaron, 0 fallaron después de corregir la fecha de fixture. La primera ejecución reportó el caso de aprobación vencido descrito arriba; se verificó su causa y se repitió la suite completa.
- Prueba focal `facade submits identity decisions with authenticated scope and expected revision` — PASS.
- `pnpm typecheck`, `pnpm lint` y `pnpm format:check` — PASS.
- `git diff --check` — PASS para el hunk de UI en el diff tracked. `pnpm format:check` cubrió también los tests nuevos que aún están untracked.

**Revisión source-only independiente:** `deleg_15b77dab` — PASS, sin concerns de seguridad, errores lógicos ni sugerencias. Confirmó que la regresión reproduce el `File` retenido en el estado local, cubre la lectura no editable y el estado posterior editable, y no afirma comportamiento BFF/persistencia. También validó el cálculo dinámico de fecha civil en Santiago del fixture reviewer.

El frontend atendió únicamente fixtures sintéticos; Playwright interceptó las llamadas `/api/backend/**`. No se ejecutó SQL/DDL, no se accedió a bases persistentes, Cloud, proveedores externos ni datos/personas reales.

## Límites y estado

Esta evidencia demuestra retención/liberación de estado local en UI y su recorrido de navegador con respuestas API simuladas. No prueba el BFF real, Go, autenticación/grants efectivos, inspección/scanner, almacenamiento, PostgreSQL, persistencia o recuperación tras reinicio. VI-04 y el MVP permanecen parciales en los gates persistentes y de almacenamiento real. No hubo commit, push, despliegue o integración a `main`.
