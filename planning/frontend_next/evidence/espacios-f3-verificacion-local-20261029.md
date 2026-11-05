# F3 — verificación local y compuerta de persistencia

**Corte:** 2026-10-29. **Checkout:** `/home/nandev/Documentos/projects/espaciGo`, rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`, HEAD `3723e51e0b3d7658915c753163b43c6f705e9e17`.

## Reconciliación del alcance

Este archivo empezó como evidencia previa a la reapertura autorizada. La actualización posterior al final conserva el baseline mock y registra el E2E persistido que supersede su antiguo bloqueo.

La cobertura de presentación y flujos mock de F3 está implementada: Mi Cuenta/listado, wizard de cinco pasos, creación idempotente, revalidación de sesión/rol, editor, errores por campo, perfiles dinámicos y aislamiento al cambiar de sesión. La matriz sintética documentada en `espacios-formularios-perfiles-v1.md` cubre nueve perfiles v1/116 atributos. La verificación de hoy confirma el estado del árbol actual, no solo el snapshot histórico.

En el snapshot inicial, F3 permanecía parcial porque aún no se había ejecutado el recorrido persistido. La autorización local posterior del owner para F3–F6 permite usar únicamente el target sintético F6 y sus límites documentados; la integración realizada se detalla abajo. No se amplió a datos reales, cloud ni publicación.

## Baseline de verificación mock ejecutado

Desde `frontend/`, sin habilitar ninguna variable `PHASE*_NATIVE_API_E2E`:

- `npm run api:check` — PASS; cliente/allowlist/inventario sincronizados, 185 operaciones.
- `npm run typecheck` — PASS.
- `npm run lint` — PASS.
- `npm test` — PASS, 281/281; incluye validadores del dominio, sincronización CAS de fachada y contrato de perfil.
- `npm run format:check` — PASS.
- `npm run build` — PASS con Next.js 16.4.3; generó el artefacto standalone local.
- `PLAYWRIGHT_PORT=4203 npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase7-host-listings.spec.ts --project=chromium-local --workers=1 --grep '@phase7'` — PASS, 22/22. Incluye listado, wizard, edición, matriz de nueve perfiles, validación accesible, reglas de categoría, sesión y Axe/overflow en escritorio/móvil.
- `PLAYWRIGHT_PORT=4204 npm exec --yes --package=pnpm@12.15.0 -- pnpm exec playwright test tests/phase6-account.spec.ts --project=chromium-local --workers=1 --grep 'Mis espacios'` — PASS, 1/1; tarjeta de Mi Cuenta (carga, error, vacío y etiquetas).
- Preflight comprobó libres 4203/4204; ambos servidores fueron iniciados por Playwright en `127.0.0.1` y terminaron al salir la suite.

Los dos grupos Playwright anteriores usan mocks. No equivalen a escritura o lectura persistida. La prueba integrada posterior y sus límites se registran a continuación.

## Gate de cierre y acción exacta

El bloqueo de autorización descrito en el snapshot inicial quedó resuelto por la instrucción expresa del owner del 2026-10-29. El estado actualizado y el alcance exacto son los siguientes.

## Actualización posterior — integración persistida F3 (2026-10-29)

- Checkout: rama `codex/backend/feature/local-close-02-espacios-galeria-documentos`, HEAD `3723e51e0b3d7658915c753163b43c6f705e9e17` más cambios locales sin commit.
- Target sintético existente: PostgreSQL `espacigo_f6_synth_20261028`, loopback `127.0.0.1:15432`; API worker-free `127.0.0.1:18083`; Next HTTPS `127.0.0.1:4205`. Se verificó identidad del target y rol API no-superuser; no se recreó la base.
- E2E nativo `tests/phase7-native.spec.ts`, Playwright `chromium-local`, un worker: **1/1 PASS**. Recorrió autenticación/logout, creación y edición, listado/rehidratación tras reload, ownership entre las dos cuentas sintéticas autorizadas, CAS/ETag y retirada lógica. Los errores de campo y perfiles dinámicos continúan acreditados por la matriz Playwright mock anterior; no se presentan como integración Go.
- Lectura independiente posterior en una transacción PostgreSQL `READ ONLY`: `row_count=1`, `final_state=eliminada`, `api_rolsuper=false`. Es una retirada lógica persistida, no un borrado físico.
- Se reutilizaron las identidades autorizadas; no hubo altas repetidas, resets, revocaciones masivas ni impresión de credenciales/tokens. No se usaron documentos/personas reales, Cloud, DDL fuera del target sintético, push, PR o deploy.
- Evidencia de ejecución privada bajo `.local/testing/f6-runtime/`; este documento conserva únicamente resultados no sensibles.

**Estado F3:** completada para el alcance local sintético/browser→BFF→Go→PostgreSQL indicado; no implica aceptación del marketplace, producción ni F6 integral.
