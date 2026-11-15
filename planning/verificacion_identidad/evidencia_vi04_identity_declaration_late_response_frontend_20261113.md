# Evidencia VI-04: respuesta tardía de declaración tras cambio de cuenta

**Fecha:** 2026-11-13 (`America/Santiago`)

## Checkout y alcance

Checkout frontend autorizado: `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`. `frontend/tests/identity-declaration-late-response.spec.ts` es una caracterización E2E test-only; no se modificó código productivo.

El test intercepta las respuestas BFF con Playwright. Un `page.addInitScript` demora la entrega al consumidor del primer GET de declaración de A después de haber recibido la respuesta mockeada, omitiendo su `AbortSignal` para modelar una operación que resuelve tras quedar obsoleta. A cierra sesión, B inicia sesión y recibe su propia declaración; solo entonces se libera la respuesta A.

## Resultado parent-owned

- Primer intento del harness falló esperando un enlace `Mi Cuenta` inexistente en la portada pública; era un error de navegación de la prueba, no un RED de producto.
- Se conservó el realm SPA mediante la ruta de retorno validada `/mi-cuenta/verificaciones`. `history.replaceState({}, ...)` deja a Next sincronizar `useSearchParams` con ese parámetro; pasar el estado interno existente de Next hacía que su parche de history omitiera esa sincronización. La corrección fue únicamente en el test.
- `pnpm test:e2e --grep 'late A declaration GET cannot overwrite account B after logout/login' --workers=1 --output test-results/identity-declaration-late-response-parent-green-20261113` — PASS, 1/1.
- `pnpm typecheck`, `pnpm lint`, `pnpm format:check` — PASS.
- Suite completa `pnpm test:e2e --workers=1 --output test-results/identity-declaration-late-response-final-suite-20261113` — 88 passed, 45 skipped, 0 failed (133).
- Revisión source-only independiente `deleg_0a4735d9`: PASS para el objetivo acotado, sin error lógico evidente; no ejecutó tests/build. Señaló acoplamiento a `window.fetch` + `page.route` y que las aserciones cubren cuatro campos, carga/error y no toda la pantalla.

## Límites

Es una simulación del cliente con respuestas BFF interceptadas; no prueba aislamiento real de sesiones, autenticación backend, Go, PostgreSQL, persistencia ni garantías frente a respuestas tardías en producción. El test caracteriza el comportamiento observable del bundle standalone local en este escenario sintético. No se ejecutó SQL/DDL ni se accedió a bases persistentes, Cloud, proveedores, sitios públicos, cuentas o datos reales. Sin commit, push, despliegue o integración a `main`.
