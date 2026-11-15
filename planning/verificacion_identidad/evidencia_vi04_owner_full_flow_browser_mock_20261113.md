# Evidencia VI-04 — recorrido owner unido con APIs mock

- **Fecha:** 2026-11-13
- **Checkout de evidencia:** `/home/nandev/Documentos/projects/espaciGo-main-sync`
- **Rama / HEAD observado:** `codex/marketplace/fix/kyc-06-save-identity-first` / `274f37e483a078890e92382a159fa49fd23c2fe8`
- **Checkout del test:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`
- **Rama / HEAD frontend:** `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`
- **Artefacto:** `frontend/tests/identity-full-owner-flow.spec.ts` (untracked en el checkout frontend)

## Alcance verificado

El test browser une un solo recorrido del titular: login sintético, guardado y lectura de declaración, creación del expediente `identity`, carga de los tres roles únicos (`identity_front`, `identity_back`, `identity_selfie`), bloqueo del envío mientras las evidencias están `pendiente`, consulta explícita de la inspección sintética limpia, envío con ETag actual y lectura posterior del caso en estado `enviada`.

`page.route("**/api/backend/**")` intercepta las operaciones de la aplicación; una ruta global aborta orígenes distintos de `http://127.0.0.1:4211` y el listener comprueba que no hubo solicitud externa. Los mocks comparan el body exacto de declaración y creación, requieren exactamente una parte `file` y una `evidence_role` por carga, validan rol, nombre, MIME y bytes sintéticos, y exigen el GET de declaración posterior al guardado antes de crear el caso. Tras recibir y mostrar las tres respuestas `pendiente`, el helper de fixture `completeSyntheticScan` simula por separado un resultado asíncrono del scanner; no lo dispara el tercer POST ni un GET. El GET de lista es de solo lectura y solo revela el resultado limpio tras el clic explícito. El GET de detalle deriva el ETag de la revisión del snapshot, comprueba que coincide con la revisión vigente y registra los tres roles limpios; submit exige ese mismo ETag y que la revisión no haya cambiado. El submit y la lectura posterior distinguen `enviada` de `en_revision`. El `consent_version` del readback de declaración refleja el valor del request sintético, de modo que la prueba no asume que la configuración pública usada al compilar el standalone coincide con el entorno del runner.

## Ejecución parent-verified

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" PLAYWRIGHT_PORT=4211 pnpm exec playwright test tests/identity-full-owner-flow.spec.ts --workers=1 --output "$TMPDIR/identity-full-owner-flow-review2"` — PASS, Chromium local, 1/1 en 2.7 s.
- `pnpm lint` — PASS.
- `pnpm typecheck` — PASS.
- `pnpm test` — 114/114 PASS; Node mostró el warning no bloqueante `MODULE_TYPELESS_PACKAGE_JSON` al cargar `src/lib/api/clp.ts`.
- `pnpm format:check` — PASS.
- Prettier no modificó el test; cero líneas con trailing whitespace y `git diff --check` — PASS.

El único cambio de comportamiento es un test, por lo que no se reconstruyó el standalone. El escenario valida UI con APIs interceptadas; no ejercita Next BFF→Go, auth/grants efectivos, inspección/scanner real, storage, SQL/PostgreSQL, atomicidad durable, reinicio o persistencia. Los datos son sintéticos. No se ejecutó SQL/DDL, migración, consulta a base persistente, Cloud o proveedor externo. Sin cambios de producción, commit, push, merge o deploy.

La revisión `deleg_20524c7f` del candidato inicial no aprobó: la observación de red fue un falso positivo (la ruta global ya abortaba orígenes no-loopback), y se corrigieron sus gaps reales. La revisión `deleg_38f8ad7b` del candidato siguiente pidió separar la transición del scanner fake del tercer POST y vincular el ETag a la revisión del snapshot. El candidato actual lo hace mediante `completeSyntheticScan`, respuestas pendientes contadas, ETag derivado del detalle y contraste de revisión al enviar. Su focal 1/1, lint, typecheck, suite Node 114/114 y format check pasaron. Revisión independiente read-only del candidato actual pendiente.

## Addendum — aislamiento de solicitudes same-origin (2026-11-13)

La revisión source-only `deleg_e440468f` señaló que la versión anterior abortaba orígenes externos, pero dejaba continuar solicitudes same-origin fuera de `/api/backend/**`. Se corrigió en `frontend/tests/identity-full-owner-flow.spec.ts`: solo GET/HEAD a rutas de página observadas y recursos estáticos explícitos pueden continuar; cualquier otro path same-origin se registra y aborta. `/api/backend/**` permanece bajo mock específico. La allowlist incluye las rutas observadas durante prefetch de la navegación; no transforma esas páginas en APIs ni afirma prueba de backend.

- Playwright focal en Chromium/Next standalone local `127.0.0.1:4211` — 1/1 PASS con `identity-full-owner-flow-strict-isolation-review-2`; puerto cerrado tras la prueba.
- `pnpm lint`, `pnpm typecheck`, `pnpm test` (114/114) y `pnpm format:check` — PASS; warning Node no bloqueante `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`.
- Re-revisión source-only del guard corregido `deleg_1666db4c` pendiente. No hubo rebuild porque el cambio es test-only.

La corrección bloquea tráfico externo y same-origin no allowlisted en este ensayo. No prueba BFF→Go, autorización efectiva, inspección real, ni persistencia. Sin SQL/DDL, DB, Cloud, proveedor, commit, push, merge o deploy.

## Revalidación correctiva tras `deleg_1666db4c` (2026-11-13)

La revisión source-only encontró dos huecos en el candidato anterior: uso de `route.continue()` en recursos allowlisted y falta de una barrera verificable contra transiciones incidentales del scanner fixture por GET/POST. El test ahora usa `route.fallback()` para páginas/assets, restringe páginas a señales HTML/RSC (`Accept: text/html`, `text/x-component`, `RSC: 1` o `Next-Router-Prefetch: 1`) y mantiene la allowlist explícita de recursos estáticos.

Se agregaron probes sintéticos que prueban el bloqueo tanto de una API same-origin fuera de `/api/backend/**` (`GET /api/unexpected-local-probe`) como de un fetch JSON a la ruta de página allowlisted `/`. El segundo probe falló antes del guard por contenido (`Expected: true`, `Received: false`) y pasó después. El scanner fixture expone lectura y una sola transición explícita; verifica que los tres uploads hayan respondido `pendiente`, que no existan transiciones antes de `completeSyntheticScan` y que haya exactamente una después. Los POST de upload también requieren estado pendiente.

En el checkout frontend `frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`: Playwright focal con `PLAYWRIGHT_PORT=4211` — 1/1 PASS; el listener quedó cerrado. `pnpm lint`, `pnpm typecheck`, `pnpm test` (114/114) y `pnpm format:check` — PASS; warning Node no bloqueante `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`. El test no requiere rebuild y Prettier no produjo cambios. Re-revisión source-only independiente `deleg_b8fe1a14` — PASS, sin concerns ni errores lógicos; no ejecutó tests ni navegador.

No prueba BFF→Go, autorización efectiva, inspección real, storage, base de datos, atomicidad durable, reinicio o persistencia. Sin SQL/DDL, DB persistente/Cloud, proveedor externo, commit, push, merge o deploy.
