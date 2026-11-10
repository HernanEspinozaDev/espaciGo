# Recuperación tras creación parcial del borrador

- Fecha: 2026-11-10.
- Plan rector: `planning/frontend_next/plan_cierre_flujo_creacion_espacios_20261110.md` (worktree documental `codex/docs/chore/espacios-creation-flow-plan`).
- Rama de implementación: `codex/frontend/fix/space-create-postcommit-recovery`.
- Base revisada: `5830c9ce2dc61d4df13a4139da984b4cafd6d9ba` (`origin/main` al iniciar este corte); los cambios listados aquí siguen sin commit ni push.

## Brecha y cambio

El wizard ejecutaba `POST /spaces`, luego configuraba zona/horario. Si esa configuración posterior fallaba, mostraba solo un error genérico aunque el borrador ya existía; el botón permitía volver a emitir `POST /spaces`. El alta de espacios no ofrece `Idempotency-Key` en el contrato vigente, así que no se añadió un encabezado no soportado.

Ahora, cuando el POST devuelve un ID y falla una operación posterior, el wizard conserva el ID en memoria, informa que el borrador fue creado, mantiene una acción de recuperación visible al recorrer pasos y bloquea una segunda creación desde esa instancia. El enlace abre el editor solo si la sesión vigente sigue siendo del propietario; de lo contrario lleva a «Mis espacios».

## Evidencia ejecutada

- RED controlada: `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-improvements.spec.ts --grep 'recupera el borrador cuando falla weekly-hours' --reporter=list` reprodujo el error genérico después de confirmar exactamente un `POST /spaces`, un `PUT /availability` y un `PUT /weekly-hours` respondido con 503.
- GREEN: `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-improvements.spec.ts --reporter=list` — 2 pruebas aprobadas, incluida la recuperación y la creación exitosa existente. La prueba confirma CTA, editor cargado, CTA visible tras volver entre pasos, botón de creación deshabilitado y un único POST.
- `npx --yes pnpm@12.15.0 build` — pasó; Next.js 16.4.3 generó la aplicación y 32 páginas estáticas.
- `npx --yes pnpm@12.15.0 typecheck` — pasó.
- `npx --yes pnpm@12.15.0 lint` — sin errores; reportó 4 warnings en archivos no modificados (`listing-gallery.tsx` y tooling).
- `npx --yes pnpm@12.15.0 test` — 480 aprobadas, 1 skip opt-in por falta de `BASELINE_DRAFT_ADAPTER_SOURCE`, 0 fallidas.
- `npx --yes pnpm@12.15.0 format:check` y `git diff --check` — pasaron.

## Límites y siguiente gate

Esta evidencia es de UI controlada con respuestas sintéticas; **no** prueba persistencia Go/PostgreSQL, API real, almacenamiento, release ni flujo online. No se creó ninguna cuenta o espacio real, no se consultó ni escribió base de datos, y no se modificaron OpenAPI, Go, SQL, migraciones KYC ni versiones de base de datos.

La limitación de contrato permanece: si el propio `POST /spaces` se confirma en servidor pero se pierde su respuesta antes de que el cliente reciba el ID, esta UI no puede reconciliar con certeza ni garantizar que un reintento manual no duplique. Requiere resolver el contrato/reconciliación en una tarea separada, sin inventar soporte de idempotencia. El estado de recuperación de este corte vive en la instancia montada del wizard; la prueba no afirma recuperación tras recarga. La fase de creación end-to-end sin mocks sigue parcial y requiere la siguiente brecha del plan con un target sintético/local expresamente autorizado.
