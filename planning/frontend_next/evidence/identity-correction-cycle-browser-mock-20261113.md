# VI-04 — ciclo owner de corrección identity en navegador local

**Fecha:** 2026-11-13

**Checkout frontend:** `/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes`

**Rama / HEAD:** `frontend-next-hermes` / `cf6e28a365d2495e78bc64c01686ed55efb17c8a`

## Recorrido probado

Se añadió `frontend/tests/identity-correction-cycle.spec.ts`, un recorrido Playwright visible desde `/acceso` hasta el expediente identity en `requiere_antecedentes` (revisión 7). El propietario reemplaza `identity_front` conservando tres roles únicos; el mock devuelve el reemplazo en estado `pendiente` y la UI oculta «Enviar a revisión». Solo después de una acción explícita de actualizar, el fixture simula el resultado limpio del escaneo, eleva la revisión/ETag a 9 y permite reenviar. El submit verifica el ETag actualizado, los tres roles únicos y limpios, el nuevo ID para el anverso y la transición visible a `en_revision`. La prueba verifica además que las solicitudes del navegador permanezcan en loopback.

Es una caracterización de UI/browser con APIs interceptadas; no se modificó código de producción. El mock de la consulta emula una respuesta de escaneo limpio y no ejecuta scanner alguno.

## Verificación local

- `PLAYWRIGHT_PORT=4203 pnpm test:e2e tests/identity-correction-cycle.spec.ts --workers=1`: 1/1 PASS. Playwright inició y detuvo el standalone local en `127.0.0.1:4203`; se reutilizó el build existente porque el cambio de este corte fue solo de test. El standalone tenía mtime `2026-11-13 08:56` y los archivos source identity consultados tenían mtime anterior.
- `pnpm lint`: PASS, sin warnings de lint.
- `pnpm typecheck`: PASS.
- `pnpm test`: 112/112 PASS. Se observó el warning Node `MODULE_TYPELESS_PACKAGE_JSON` en `src/lib/api/clp.ts`, fuera del alcance de este test.
- `pnpm format:check`: PASS.

## Límites

Todas las solicitudes `/api/backend/*`, incluidas autenticación, lista/detalle, carga, lectura de escaneo y submit, se resolvieron con fixtures Playwright. El POST y las lecturas comparten el estado mutable del fixture, por lo que no hay readback persistente independiente. Los bytes multipart son texto sintético declarado como `image/png`, no una imagen PNG decodificable; se valida el flujo del formulario, no el contenido/codec. El guard de loopback observa eventos de `page.on("request")`: no acredita aislamiento de red del proceso. El flujo no prueba runtime BFF→Go, autorización/grants efectivos, `UploadService`, scanner/ClamAV, PostgreSQL, persistencia, auditoría durable, storage/GCS ni Cloud. No hubo SQL/DDL, acceso a DB persistente, servicios externos, datos reales, commit, push o deploy. El checkout frontend contenía otros cambios locales en curso; se preservaron sin limpiarlos ni modificarlos fuera de alcance.

## Revisión independiente

La revisión source-only inicial `deleg_7433a228` aprobó con observaciones no bloqueantes sobre el escaneo simulado, el alcance del guard de red, el estado compartido del mock y los bytes no decodificables; estos límites quedaron explicitados. La revisión source-only del archivo final `deleg_123b9943` pasó sin concerns ni errores lógicos y confirmó selector de Anverso, secuencia de ETags, bloqueo pendiente y submit tras readback limpio. El revisor no ejecutó pruebas.
