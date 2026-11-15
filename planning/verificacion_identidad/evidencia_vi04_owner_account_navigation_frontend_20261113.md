# VI-04 — navegación de Mi cuenta al expediente de documentos

**Fecha:** 2026-11-13 (`America/Santiago`)

## Alcance

Subcorte test-only en el checkout autorizado `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend`, rama `frontend-next-hermes`, HEAD `cf6e28a365d2495e78bc64c01686ed55efb17c8a`. En `tests/identity-evidence-upload.spec.ts`, el flujo sintético de propietario inicia sesión con retorno a `/mi-cuenta`, pulsa el enlace real `Documentos` de la navegación de cuenta y confirma la URL `/mi-cuenta/documentos`, el enlace activo (`aria-current="page"`) y que las solicitudes observadas por Playwright permanecen en `127.0.0.1`. Desde esa ruta continúa el caso existente de consentimiento y carga identity sintética.

No se cambió código productivo. Es prueba de la navegación visible desde Mi cuenta hacia el flujo EspaciGo de documentos, no solo de su `href`. La captura de requests corresponde al navegador; no observa llamadas server-side ocultas detrás de una BFF. El mock `page.route("**/api/backend/**")` reemplaza la API: no prueba el BFF/Go, persistencia, autorización real ni la ausencia de llamadas de proveedor dentro de un proceso servidor.

## Verificación ejecutada

- Playwright focal `owner validates identity evidence and submits only three clean roles` — PASS 1/1; incluye click y ruta activa.
- E2E completa en el árbol final — PASS 85, 45 omitidas, 0 fallas (130 casos).
- `pnpm test` — PASS 107/107; `pnpm typecheck`, `pnpm lint` y `pnpm format:check` — PASS.
- Revisión source-only independiente `deleg_cc3f7c91` — PASS, sin findings; verificó click real, destino/estado activo, loopback y límites de API mock.
- Artefacto `.next/standalone` no se reconstruyó: el único cambio fue de test.

Una ejecución E2E completa anterior había observado una falla de Axe en contraste de rutas de reservas, que también apareció en una ejecución aislada; en la verificación completa final no se reprodujo. La evidencia histórica del primer resultado queda en `evidencia_vi04_identity_declared_format_frontend_20261113.md`; el resultado de esta corrida final es el gate vigente de la suite.

## Límites

Todos los datos y respuestas son sintéticos. No se accedió a base persistente, Cloud, proveedor externo ni navegador/perfil del usuario; no se ejecutó SQL/DDL. No se demuestra flujo de producción, BFF→Go, persistencia o autorización. Sin commit, push, deploy o merge. El criterio MVP general permanece parcial.
