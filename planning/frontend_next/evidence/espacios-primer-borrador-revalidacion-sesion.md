# Primer espacio: revalidación de sesión tras crear borrador

**Fecha:** 2026-10-27  
**Estado:** flujo de UI local verificado con mock; no prueba integración real con Go.

## Comportamiento

Cuando una sesión autenticada aún no contiene el rol `arrendador`, el wizard permite crear el primer borrador. Tras una respuesta exitosa al POST, revalida `/api/backend/auth/session` para recibir las capacidades/roles que concede el backend. La navegación posterior utiliza el mismo ID devuelto por el POST; no reintenta la creación ni genera otro borrador si la fachada se limpia al cambiar la huella de sesión. Las sesiones que ya tienen `arrendador` no hacen esa comprobación adicional.

## Prueba y gates

La E2E sintética de cinco pasos inicia con `roles: ["arrendatario"]`, simula la concesión de `arrendador` al aceptar el POST y verifica el GET de sesión, la redirección al editor, exactamente un POST con `Idempotency-Key` y la reanudación del mismo borrador tras `page.reload()` (título y atributo `calefaccion` rehidratados desde el GET mockeado).

- RED observado antes del cambio: timeout esperando `GET /api/backend/auth/session` después de aceptar la creación.
- Build de producción `pnpm build`: PASS.
- E2E focalizada `pnpm exec playwright test tests/phase7-host-listings.spec.ts --grep 'asistente en 5 pasos'`: PASS (1/1).
- Suites Phase 6 y 7: PASS (13/13).
- `pnpm typecheck`, `pnpm lint`, `pnpm format:check`: PASS tras formatear el test.

Todo se ejecutó localmente con datos de sesión sintéticos y un mock BFF. No se creó una cuenta, no se envió tráfico externo y no se verificó la concesión del rol contra Go/PostgreSQL. No hubo escrituras a la base.
