# VI-03 — detalle y visor de evidencia en Next (mocks)

- **Fecha:** 2026-11-12
- **Frontend:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`
- **Rama / HEAD:** `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`
- **Plan:** `/home/nandev/Documentos/projects/espaciGo-main-sync/planning/verificacion_identidad/plan_verificacion_identidad_espacigo.md`
- **Estado:** subcorte local implementado y verificado; cambios siguen sin commit. No cierra VI-03.

## Implementación

- Se añadió la página de detalle asignado `/mi-cuenta/revisiones/[caseId]`, con parser allowlistado de declaración/caso/archivos. No expone storage keys ni mensajes crudos de API.
- El detalle muestra una acción explícita para cada archivo de identidad con rol conocido y estado `limpio`. El binario se solicita al endpoint privado solo tras el clic; la fachada valida IDs canónicos, estado limpio, MIME JPEG/PNG, tamaño máximo de 10 MiB y que bytes/MIME de respuesta coincidan con los metadatos. La API usa Bearer sin credenciales de navegador y `cache: no-store`.
- La vista crea una Blob URL privada registrada; el request se cancela al desmontar la vista y las Blob URL se revocan al desmontar/expirar la sesión. La UI muestra un estado neutro si falla la lectura.
- Se añadieron contrato OpenAPI y cliente generado para GET del detalle y contenido. El backend fuente declara el mismo path de contenido; la prueba frontend intercepta la API y no ejecuta BFF→Go.

## Verificación local ejecutada

En el frontend autorizado:

- `pnpm api:check` — PASS; inventario actual de 188 operaciones.
- `pnpm test` — PASS, 89/89 (0 fallos, 0 omitidos).
- `pnpm lint`, `pnpm typecheck`, `pnpm format:check`, `git diff --check` — PASS.
- `pnpm build` — PASS; Next.js compiló y generó 31 páginas estáticas.
- `PLAYWRIGHT_PORT=4203 pnpm exec playwright test tests/reviewer-queue.spec.ts --reporter=list --workers=2 --output=/home/nandev/.hermes/cache/scratch/reviewer-queue-final-output` — PASS, 5/5 con Playwright local y API interceptada. Incluye cola vacía y error 503 neutral de la cola; detalle allowlisted, descarga bajo demanda y expiración de sesión con comprobación de revocación Blob URL.
- Revisión explícita de 14 archivos nuevos de la feature/tests: existen, tienen newline final y no contienen whitespace final. El puerto 4203 quedó libre al terminar.
- Node emitió una advertencia no fatal `MODULE_TYPELESS_PACKAGE_JSON` al cargar `frontend/src/lib/api/clp.ts`; la suite y gates terminaron con código cero.

## Límites

- Las pruebas de fachada/Playwright usan blobs y respuestas sintéticas. No prueban asignación/revocación efectiva, BFF→Go, autorización integrada, transacción, PostgreSQL, GCS, IAM, cifrado en runtime ni persistencia.
- No se ejecutó SQL/DDL, no se consultaron ni modificaron bases persistentes o Cloud, no se usaron cuentas/documentos/proveedores reales. V96 no se aplicó.
- La UI Next permanece read-only: checklist y decisión auditada siguen pendientes. No se habilita revisión real ni se afirma cierre de VI-03.
- No hubo commit, push, integración a `main` ni deploy. El checkout frontend permanece sucio con cambios de trabajo previos preservados.
