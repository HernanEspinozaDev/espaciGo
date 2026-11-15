# Evidencia frontend VI-04 — recorrido owner unido con APIs mock

- **Fecha:** 2026-11-13
- **Checkout:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`
- **Rama / HEAD observado:** `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`
- **Artefacto:** `frontend/tests/identity-full-owner-flow.spec.ts` (nuevo, no committed)

## Alcance

El test Playwright local une en una sola navegación: login sintético, guardado de declaración y lectura posterior, creación del expediente propio, carga de `identity_front`, `identity_back` e `identity_selfie`, bloqueo de envío mientras siguen `pendiente`, consulta explícita de la inspección sintética limpia, envío con ETag vigente y una lectura explícita posterior que muestra `Enviada`.

`page.route("**/api/backend/**")` intercepta los endpoints del recorrido y la ruta global aborta cualquier origen distinto de `http://127.0.0.1:4211`; un listener comprueba que no se originó ninguna solicitud externa. El mock exige una parte `file` y una `evidence_role`, nombres/bytes/MIME sintéticos y cada rol único; compara los cuerpos de declaración y creación, y exige un GET de declaración posterior al POST antes de crear el caso. Después de que el browser recibe y muestra las tres respuestas `pendiente`, el helper fixture `completeSyntheticScan` simula por separado el resultado asíncrono del scanner; no lo activa el POST de carga ni un GET. El GET de lista es de solo lectura y solo revela el resultado limpio tras la acción explícita; el GET de detalle asocia el ETag con la revisión del snapshot y registra los tres roles limpios. Submit exige ese mismo ETag y que la revisión leída aún sea vigente. El consentimiento de declaración devuelto se toma del `consent_version` enviado por el cliente, validado por patrón, para conservar el round-trip del artefacto standalone local sin asumir el valor de su variable de build.

## Verificación local

- `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" PLAYWRIGHT_PORT=4211 pnpm exec playwright test tests/identity-full-owner-flow.spec.ts --workers=1 --output "$TMPDIR/identity-full-owner-flow-review2"` — PASS, 1/1 (2.7 s), Chromium local; el servidor standalone escuchó en loopback.
- `pnpm lint` — PASS.
- `pnpm typecheck` — PASS.
- `pnpm test` — 114/114 PASS. Node emitió el warning no bloqueante `MODULE_TYPELESS_PACKAGE_JSON` para `src/lib/api/clp.ts`.
- `pnpm format:check` — PASS; Prettier no cambió el test.
- Whitespace del archivo nuevo: cero líneas con trailing whitespace; `git diff --check` — PASS.

No se reconstruyó el standalone porque el único cambio fue un test. El escenario es una caracterización de UI con APIs interceptadas, no integra navegador→BFF→Go, autenticación/autorización efectiva, scanner, almacenamiento, SQL/PostgreSQL, reinicio ni persistencia. El expediente y archivos son sintéticos; no se utilizaron proveedor, Cloud, SQL/DDL o base persistente. Sin commit, push, despliegue o integración a `main`. El worktree compartido conserva sus otros cambios.

La revisión `deleg_20524c7f` del candidato anterior no aprobó: su observación de red externa era un falso positivo (la ruta global ya abortaba orígenes no-loopback); sus gaps reales de orden del GET de declaración, estado de scanner y ETag se corrigieron. La re-revisión `deleg_38f8ad7b` pidió separar explícitamente la transición fake del tercer POST y vincular ETag con revisión del snapshot; el candidato actual lo hace con `completeSyntheticScan`, respuestas pendientes contadas y ETag derivado del detalle. Playwright focal 1/1, lint, typecheck, Node 114/114 y format check pasaron en ese candidato. Revisión independiente del candidato actual pendiente.

## Addendum — aislamiento de solicitudes same-origin (2026-11-13)

La revisión source-only `deleg_e440468f` encontró un hueco en la versión anterior del guard: se abortaban orígenes externos, pero se continuaban solicitudes same-origin fuera de `/api/backend/**`. El test ahora permite continuar solo GET/HEAD a las rutas de página observadas (`/`, `/acceso`, `/recuperar-acceso`, `/registro`, `/saas`, `/pricing`, `/mi-cuenta` y las rutas de cuenta usadas/prefetch observadas) y recursos estáticos locales explícitos; todas las demás solicitudes same-origin se registran y abortan. `/api/backend/**` sigue cubierto por su mock específico. Una primera allowlist más estrecha bloqueó prefetch legítimo de páginas; se ajustó a las rutas observadas.

- Playwright focal con Next standalone local en `127.0.0.1:4211`, puerto comprobado libre antes y después: `PLAYWRIGHT_PORT=4211 pnpm exec playwright test tests/identity-full-owner-flow.spec.ts --workers=1 --output "$TMPDIR/identity-full-owner-flow-strict-isolation-review-2"` — PASS, 1/1.
- `pnpm lint`, `pnpm typecheck`, `pnpm test` (114/114) y `pnpm format:check` — PASS. Node emitió el warning no bloqueante `MODULE_TYPELESS_PACKAGE_JSON` para `src/lib/api/clp.ts`.
- No se reconstruyó el standalone porque el cambio siguió siendo test-only. La re-revisión source-only `deleg_1666db4c` detectó dos hallazgos, corregidos en el candidato final; la re-revisión `deleg_b8fe1a14` aprobó ese candidato (PASS, véase el cierre posterior).

La corrección solo asegura que este ensayo no continúe solicitudes same-origin no allowlisted ni tráfico externo. No demuestra BFF→Go, auth/autorización efectiva, scanner real ni persistencia. Sin SQL/DDL, DB, Cloud, proveedor, commit, push, merge o deploy.

## Revalidación correctiva tras `deleg_1666db4c` (2026-11-13)

La revisión source-only señaló dos huecos: el `continue()` de la allowlist podía saltar otros handlers coincidentes y el test no impedía/observaba una transición incidental del scanner fixture desde rutas GET/POST. Se corrigieron ambos:

- Las páginas locales allowlisted ahora requieren un GET/HEAD con señal de navegación HTML o RSC (`Accept: text/html`, `text/x-component`, `RSC: 1` o `Next-Router-Prefetch: 1`); recursos estáticos conservan su allowlist explícita. Las rutas permitidas usan `route.fallback()`.
- Dos probes browser sintéticos verifican abort: GET `/api/unexpected-local-probe` fuera del prefijo mock y GET `/` con `Accept: application/json` aunque `/` sea ruta de página allowlisted. Antes de la segunda guarda se observó RED: Playwright recibió `false` donde esperaba que el fetch se abortara. Con la guarda por tipo de contenido, el test focal pasó.
- El estado scanner quedó encapsulado tras un getter y una única operación `completeAfterPendingUploads`; las cargas exigen estado `pendiente`, el test comprueba cero transiciones antes del helper explícito y exactamente una transición después de recibir las tres respuestas pendientes.

Verificación del candidato final en `frontend-next-hermes` (`HEAD cf6e28a365d2495e78bc64c01686ed55efb17c8a`): Playwright focal en Chromium/Next standalone local `127.0.0.1:4211` — 1/1 PASS; puerto libre antes y después. `pnpm lint`, `pnpm typecheck`, `pnpm test` (114/114) y `pnpm format:check` — PASS; Node emitió el warning no bloqueante `MODULE_TYPELESS_PACKAGE_JSON` para `src/lib/api/clp.ts`. Prettier dejó el test sin cambios. Re-revisión source-only independiente `deleg_b8fe1a14` — PASS, sin concerns ni errores lógicos; el revisor no ejecutó tests ni navegador.

El resultado sigue limitado a UI/browser con APIs sintéticas interceptadas; no demuestra BFF→Go, autorización efectiva, scanner real, storage, SQL/PostgreSQL, atomicidad, reinicio o persistencia. Sin SQL/DDL, base persistente/Cloud, proveedores externos, commit, push, merge o deploy.

## Revalidación del artefacto y recorrido local — 2026-11-15

En el checkout `frontend-next-hermes` (`HEAD cf6e28a365d2495e78bc64c01686ed55efb17c8a`, worktree previamente dirty y preservado), se ejecutó `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$TMPDIR" NEXT_TELEMETRY_DISABLED=1 node_modules/.bin/next build`: compilación y TypeScript PASS; generación estática **31/31**. Después, con Node `v26.12.1`, pnpm `12.15.0`, puerto `4211` libre antes y después, y `PLAYWRIGHT_PORT=4211`, `node_modules/.bin/playwright test tests/identity-full-owner-flow.spec.ts --workers=1 --output <directorio-único-en-TMPDIR>` terminó **1 PASS, 0 FAIL** (test 1.6 s; corrida 2.7 s). La salida standalone sirvió solo en `127.0.0.1`; el test mantiene sus guards de red y todas las respuestas de API son sintéticas/interceptadas.

Esta revalidación no ejecutó lint ni la suite completa (los resultados del 13 de noviembre arriba son evidencia histórica). No hubo cambios de fuente, solicitudes externas, BFF→Go, scanner real, storage, SQL/DDL, PostgreSQL, Cloud o proveedor. No demuestra persistencia o aceptación de producto.
