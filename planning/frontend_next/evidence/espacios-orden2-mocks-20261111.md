# Orden 2 — cobertura local controlada de horario y bloqueos

- Fecha: 2026-11-11.
- Worktree: `espaciGo-spaces-wizard-recovery`.
- Rama: `codex/frontend/fix/space-01-create-postcommit-recovery`.
- HEAD base inspeccionado: `647cd491985d619370990d3f226b5df61e36b42b`.
- Alcance: zona IANA, horario semanal, CAS/If-Match, refresh y rango de bloqueos manuales, reload/readback y horas DST. Solo Playwright local con mocks y fixtures sintéticos.

## Cobertura añadida

- `frontend/tests/weekly-schedule-editor.spec.ts`: conserva los intervalos fraccionarios existentes; el mock de GET tras reload es la fuente de rehidratación. Los handlers usan el ETag fuerte compartido (`planning/openapi.yaml:4265-4289, 4576`; `internal/spaces/transport/http/handler.go:468-501`; `internal/booking/transport/http/owner_weekly_hours_handler.go:101-135`), y el wizard envía el ETag de zona al PUT semanal (`frontend/src/features/host-listings/ui/listing-wizard.tsx:438-459`). El test espera que el navegador observe la respuesta de disponibilidad 200 con ETag `v2` (`page.waitForResponse`) y que el siguiente PUT semanal lleve `If-Match: v2` (`page.waitForRequest`). El mock pausa el manejo de ese PUT; solo entonces el test avanza el estado sintético del escritor concurrente y ETag semanal a `v3` y libera el handler, que devuelve 412. Se comprueban exactamente dos PUT semanales (200 y 412), sin replay; tras recargar, el GET rehidrata el valor previamente guardado `20:00`, no el cambio no guardado `20:30`.
- Para los límites UTC esperados del refresh de bloqueos, `frontend/src/features/host-listings/ui/listing-editor.tsx:313-334` expande un rango de fechas válido: resta un día al inicio y añade un día al extremo final, que se representa como medianoche exclusiva más un día.
- `frontend/tests/availability-block-calendar.spec.ts`: rango vacío conserva el botón disponible y no hace GET; un refresh con rango válido envía los límites esperados. En `America/Santiago`, una hora inexistente del cambio de primavera y una hora ambigua del cambio de otoño muestran el error accesible y no emiten POST. Se mantienen conflicto 409, creación/listado/eliminación mockeados, lectura tras reload, ausencia de GET si falta zona IANA, navegación a su configuración, Axe y overflow en 1280×800 y 390×844.
- `frontend/tests/phase7-host-listings.spec.ts`: el nombre del caso del wizard ahora refleja el contrato real: `POST /spaces` no usa `Idempotency-Key`; la prueba comprueba su ausencia y rehidrata `America/Coyhaique` y el horario tras reload.
- `frontend/tests/nonhourly-listing-schedule.spec.ts` y `frontend/tests/nonhourly-wizard-schedule.spec.ts`: mantienen la cobertura de tarifas por día/mes, zona editable y cero lecturas/escrituras de horario semanal.
- El editor ahora trata una zona guardada pero inválida como no configurada también en bloqueos por fecha: refrescar no emite GET, el panel orienta a guardar una zona IANA válida, el botón «Crear bloqueo» queda deshabilitado, la creación no emite POST y el guardado de horario no emite PUT. RED observado al no aparecer el aviso para el fixture `Invalid/Zone`; la prueba del estado disabled también dio RED antes de corregir la condición del botón a la validación IANA existente.
- Se añadió el borde de rango invertido adyacente (`Desde=2026-12-20`, `Hasta=2026-12-19`): conserva el botón disponible, muestra el error accesible y no emite GET, además del caso ya existente de rango vacío.

## Verificación

- `npx --yes pnpm@12.15.0 build` — PASS; build Next.js 16.4.3 del código de producto sin cambios en este corte.
- Playwright focal posterior a la barrera CAS — **7 passed** en 5 archivos (1 horario/CAS, 1 bloqueos, 2 editores no-horarios, 2 wizard no-horarios, 1 wizard de cinco pasos); Chromium local, puerto 4205, dividido en un grupo de 6 y un caso de wizard; sin llamadas a API Go ni DB.
- Corrida combinada previa a la prueba 403 de galería, 7 archivos y filtros focales — **11 passed**, 0 fallidos, Chromium local, puerto 4205; incluye los casos CAS/rango y cuatro casos de galería.
- Corrida combinada anterior a la barrera CAS final — **13 passed**, 0 fallidos, 9 archivos (`$TMPDIR/spaces-order2-order3-contract-guard-final2-20261111`). Es evidencia histórica del candidato previo; no valida la secuencia de timing actual y quedó supersedida por las corridas posteriores.
- Re-ejecución focal de reconciliación — **7 passed**, 0 fallidos, 5 archivos, Chromium local en `127.0.0.1:4205` (`$TMPDIR/order2-reconcile-focused-20261111`). Incluye CAS/reload, rango/DST, unidades día/mes y wizard; solo fixtures y BFF mocks, sin API Go ni DB.
- Corrida combinada posterior al build de la superficie persistente de galería — **18 passed**, 0 fallidos, 9 archivos, Chromium local; incluye CAS/rango y los siete casos de cola/galería. Las corridas previas de 14/14, 17/17 y 18/18 preceden a esa edición y quedan solo como historial.
- `typecheck` — PASS.
- `lint` — PASS, 0 errores y 4 advertencias existentes fuera de los archivos cambiados.
- `format:check` — PASS.
- `git diff --check` — PASS después de la edición y el formato del candidato actual.

## Revisión independiente

- El re-review `deleg_c1d4bb81` detectó correctamente que `route.fulfill()` por sí solo no demostraba que el navegador hubiera observado la respuesta v2; ese hallazgo quedó corregido en el candidato actual.
- El re-review independiente más reciente `deleg_06f23bdd` del candidato con barrera pasó: verificó que el navegador observa la respuesta 200/v2, el PUT semanal observado usa `If-Match: v2`, su handler queda detenido hasta que el test avanza el estado sintético a v3, y se recibe exactamente un 412 sin replay. También verificó que el conflicto conserva el intento `20:30`, el reload/readback mock devuelve `20:00` y el rango coincide con `listing-editor.tsx:313-334`. La revisión fue solo de fuente: no ejecutó pruebas/build ni accedió a servicios externos; los gates de ejecución se atribuyen a las corridas parentales.
- La inspección independiente confirma que los límites UTC del rango coinciden con `listing-editor.tsx:313-334`. Todo esto es mock/source-level; no sustituye evidencia de persistencia real.

## Revalidación tras cubrir zona IANA inválida — 2026-11-11

- `typecheck`, ESLint focal, Prettier y `git diff --check` — PASS; `pnpm build` reconstruyó Next.js 16.4.3 después del cambio en `listing-editor.tsx`.
- Corrida local Playwright en Chromium, puerto 4215, BFF mock apuntando a `127.0.0.1:18082` sin listener: **7/7 passed** en los cinco archivos citados arriba. Incluye zona persistida `Invalid/Zone`, ausencia de GET/PUT/POST, rango invertido, DST, refresh y readback/reload; conserva CAS 412 sin replay y las rutas de tarifas por día/mes.
- El test confirmó RED antes del fix: con `Invalid/Zone`, el editor no mostraba el aviso IANA válido en bloqueos. Tras el fix se mostró la instrucción y pasó el caso sin llamadas de mutación/refresh no autorizadas por la UI.
- No se inició Go/API ni worker; `18082` no tenía listener. No se consultó ni mutó base, GCS o Cloud SQL. El ensayo usó fixtures sintéticos únicamente.
- Evidencia de UI/mock únicamente: no demuestra validación Go, ownership, persistencia ni readback integrado.

## Revisión adicional de affordance y CAS — 2026-11-11

- `listing-editor.tsx` deshabilita los botones «Actualizar fechas» y «Crear bloqueo» cuando la zona guardada no pasa `isValidIanaTimeZone`; los guardas del handler permanecen como defensa. La regresión local primero falló porque «Crear bloqueo» seguía habilitado; tras el cambio, el caso verificó aviso visible, botón disabled y ausencia de GET/POST.
- El CAS mock semanal ahora incorpora el cambio de zona: `/availability` devuelve la revisión v2, el primer PUT semanal la consume y actualiza a v3; el segundo usa v3, queda retenido hasta que el escritor sintético avanza a v4 y termina en 412 sin replay. `revisionEvents` verifica el orden. Después de reload, el GET mock rehidrata la zona `America/Coyhaique` y el horario guardado previo. Esto acredita solo la secuencia controlada del navegador/BFF mock, no el CAS de Go/DB.
- La prueba de zona inválida exige que «Actualizar fechas» también esté deshabilitado; falló antes del ajuste del affordance. Después cambia solo el DTO sintético a `America/Santiago` y recarga: la lista hace exactamente un GET, ambos botones se reactivan y no aumenta el contador de POST/PUT. La regresión pasó 1/1; luego la corrida consolidada completa volvió a pasar.
- Revalidación consolidada posterior a estos cambios: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI 200 operaciones, ESLint 0 errores/4 warnings, typecheck, unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js 16.4.3 y Playwright **220 (157 aprobados/63 omitidos/0 fallidos)**. Los gates integrados siguen pendientes por falta de target aislado autorizado.

## Límites y seguridad

- Esta evidencia prueba comportamiento de UI contra BFF mockeado; no prueba Go, PostgreSQL, almacenamiento, persistencia real ni aceptación online.
- Siguiendo la instrucción del revisor, este corte no inició API, no volvió a acceder a la base persistente compartida, no modificó su esquema/datos/roles y no modificó sus controles de seguridad. No creó cuentas ni recursos persistidos.
- Antes de la instrucción correctiva se había ejecutado únicamente un preflight SQL de solo lectura, en transacción `READ ONLY`, con el rol migrador; no hubo DML/DDL. No se repitió después de la instrucción.
- El gate integrado del orden 1/2 permanece bloqueado hasta que exista un target aislado autorizado; el análisis y cobertura no mutante del orden 2 continúan.

## Rango de un día y coincidencia estricta IANA — 2026-11-11

- Se detectó una divergencia de validación: Go `internal/adapters/postgres/spaces/repository.go:470-475` rechaza zonas con espacios externos antes de `time.LoadLocation`, mientras que `isValidIanaTimeZone` validaba `tz.trim()` y aceptaba el valor que el backend rechaza. La nueva aserción para `" America/Santiago "` produjo RED (`true !== false`); el validador frontend ahora exige que la cadena ya esté recortada y valida exactamente el valor recibido, sin normalizarlo. La prueba focal `node --test --test-name-pattern='host-listings domain enforces publication eligibility and IANA timezone requirement' tooling/host-listings-domain.test.mjs` pasó después del ajuste.
- `availability-block-calendar.spec.ts` cubre ahora el rango inclusivo de un solo día (`2026-12-10`): una lectura GET genera `from=2026-12-09T00:00:00.000Z` y `to=2026-12-12T00:00:00.000Z`, conserva el bloqueo de ese día y deja contadores POST/PUT/DELETE en cero. Se conserva la prueba de rango vacío/invertido sin GET.
- Build Next.js 16.4.3 y Playwright focal de horario/CAS y bloqueos: **2/2 passed**. Corrida consolidada posterior a ambos cambios, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check`, terminó **exit 0**: OpenAPI 200; ESLint 0 errores/4 warnings; typecheck; unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build **32/32**; Playwright **221 (158 aprobados/63 omitidos/0 fallidos)**.
- Todo lo nuevo continúa en test/unit/UI con fixtures sintéticos y respuestas mock; no se inició Go/API/worker ni se consultó o alteró base, storage, roles o controles de seguridad. El gate integrado no está aprobado ni sustituido por estos resultados.

## Cambio de zona IANA, revisión compartida y recuperación tras reload — 2026-11-11

- `weekly-schedule-editor.spec.ts` ahora cubre un cambio real de formulario desde `America/Santiago` a `America/Coyhaique`: el mock de `PUT /availability` conserva la zona y avanza la revisión sintética a `schedule-timezone-v2`; el siguiente `PUT /weekly-hours` usa ese ETag exacto. Tras reload, el editor rehidrata `America/Coyhaique` y los intervalos guardados.
- La segunda edición deja la zona sin cambios, recibe `schedule-saved-v3`, y retiene el `PUT /weekly-hours` hasta que un escritor concurrente sintético avanza a `schedule-external-v4`; el `412` no se reintenta y reload vuelve a mostrar los intervalos previos junto con la zona ya guardada.
- Focal `weekly-schedule-editor.spec.ts`: **1/1**. Corrida final `pnpm check`: **exit 0**, unitarias 492 (491 aprobadas, 1 opt-in omitida), Playwright 220 (157 aprobados, 63 omitidos, 0 fallidos), además de OpenAPI/typecheck/lint/formato/build.
- Solo test y respuestas BFF mockeadas; no valida el CAS real del servidor ni la persistencia. No se inició API/worker, ni se accedió a base/GCS/Cloud SQL o se cambiaron controles de seguridad.

## Refresh de bloqueos: fallo temporal y recuperación explícita — 2026-11-11

- `availability-block-calendar.spec.ts` ahora simula un `GET` de lista que responde 503 una sola vez. La UI muestra el mensaje genérico seguro, conserva la última fila previamente confirmada, vuelve a habilitar «Actualizar fechas» y mantiene el rango seleccionado.
- El reintento explícito devuelve 200 para el mismo rango, quita el error y conserva la fila; los contadores confirman cero POST/PUT/DELETE tanto durante fallo como recuperación. Antes del reintento el fixture añade un bloqueo externo sintético: no aparece por mutación local del cliente, se muestra únicamente tras el GET 200 y permanece al recargar, con exactamente una lectura adicional y sin escrituras de aplicación.
- Playwright focal **1/1**; la corrida consolidada posterior `pnpm check` terminó **exit 0** (OpenAPI 200, typecheck, lint 0 errores/4 warnings, unitarias 491/492 con 1 opt-in omitida, E2E 157/220 con 63 omitidas).
- Solo fixture/BFF mock. No se inició API/worker ni se accedió a base, GCS o Cloud SQL; el gate integrado sigue bloqueado.

## Rehidratación de zona para tarifas día/mes — 2026-11-11

- Se amplió `nonhourly-listing-schedule.spec.ts`: para `dia` y `mes`, el test mock ahora actualiza la zona IANA al guardar `PUT /availability`, la devuelve desde el detalle al recargar y verifica que el editor rehidrate `America/Punta_Arenas`. Los contadores mantienen exactamente un PUT de zona y cero GET/PUT `/weekly-hours`.
- Se reejecutaron además los dos recorridos sintéticos del wizard día/mes. Suites focales `nonhourly-listing-schedule.spec.ts` + `nonhourly-wizard-schedule.spec.ts`: **4/4 passed**.
- La corrida consolidada posterior `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI 200, ESLint 0 errores/4 warnings, typecheck, unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**, build **32/32** y Playwright **221 (158 aprobados/63 omitidos/0 fallidos)**.
- Solo BFF mocks y fixtures locales; los POST/PUT del wizard/editor no alcanzaron API real. No se inició Go/API/worker ni se accedió/modificó base o storage; no se tocaron controles de seguridad.

## Respuesta obsoleta tras refresh solapado — 2026-11-11

- `availability-block-calendar.spec.ts` retiene un GET de lista con una instantánea anterior, introduce un bloqueo externo sintético y dispara un segundo refresh. La respuesta más reciente muestra la fila nueva; al liberar la primera respuesta retrasada, la fila continúa visible y no se repuebla el estado con la instantánea vieja.
- El escenario compara contadores antes/después y confirma que la carrera ejecuta solo GET, sin nuevos POST/PUT/DELETE ni mutaciones de horario. Es una carrera controlada del navegador/BFF mock, no una garantía del servidor ni una prueba de persistencia.
- Playwright focal de bloqueos **1/1**; la corrida consolidada posterior `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI 200; ESLint 0 errores/4 warnings; typecheck; unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**; build **32/32**; Playwright **221 (158 aprobados/63 omitidos/0 fallidos)**.

## Compatibilidad explícita de alias aceptados por Intl — 2026-11-11

- La decisión vigente no exige una allowlist canónica ni rechaza aliases admitidos por el runtime. En Node local se verificó que `Intl.DateTimeFormat` acepta `EST` y rechaza `Local`, `local`, `../UTC` y `America/Santiago`.
- `host-listings-domain.test.mjs` ahora fija `isValidIanaTimeZone("EST") === true`. El recorrido público sintético también verifica que el adaptador preserve el horario y la UI lo renderice para `EST`, en contraste con la zona inválida.
- Suite de dominio `node --test tooling/host-listings-domain.test.mjs`: **7/7**; `test:e2e:public-calendar`: **6/6**; typecheck, Prettier focal y ESLint focal PASS. Esto solo fija consistencia de validación/UI local; no acredita normalización o aceptación en Go/DB.
- El E2E del alias comprueba los valores visibles `14:15`–`16:45` tras seleccionar martes. La primera corrida de esa aserción no encontró los campos porque los períodos se muestran al seleccionar el día; se ajustó la interacción del test, sin cambio productivo, y la corrida final quedó 6/6.

## Horario semanal deshabilitado y rehidratación — 2026-11-11

- Se amplió `weekly-schedule-editor.spec.ts` para deshabilitar el horario semanal por hora mediante la UI y comprobar el `PUT /weekly-hours` mock con `enabled: false`, conservando los siete días y sus períodos configurados.
- Tras recargar, el mock de lectura independiente devuelve el estado deshabilitado y los mismos períodos; el test confirma el checkbox apagado y rehidrata los tramos del martes sin cambiar la zona horaria.
- Ejecución focal: `PLAYWRIGHT_PORT=4223 API_BACKEND_BASE_URL=http://127.0.0.1:18091/api/v1 npx --yes pnpm@12.15.0 exec playwright test tests/weekly-schedule-editor.spec.ts --grep 'preserves fractional slots' --output=/tmp/playwright-order2-weekly-disabled-final` — **1 passed**. El BFF fue interceptado con fixture sintético; no había API Go en el target y no se accedió a PostgreSQL.
- Validaciones locales de esta edición: `typecheck` y ESLint focal PASS; Prettier focal PASS tras ajuste de formato. La cobertura sigue siendo controlada/mock; no prueba persistencia backend ni cierra el gate integrado del orden 2.

## Tramo semanal de medianoche — preset 00:00–24:00 — 2026-11-11

- Se amplió el recorrido controlado de `weekly-schedule-editor.spec.ts`: tras guardar/deshabilitar y rehidratar el horario, el usuario reactiva el horario del lunes, aplica el preset `24 horas 00:00–24:00`, y guarda con el ETag sintético vigente (`schedule-disabled-v5`). El mock responde con la revisión siguiente (`schedule-boundary-v6`).
- El test verifica el cuerpo exacto de los siete días: lunes `00:00`–`24:00`, el martes conserva ambos tramos configurados y los demás días siguen vacíos. Tras recargar, exige GET semanal nuevo, checkbox activo y rehidratación de `00:00`/`24:00` sin deriva.
- En la primera corrida, la expectativa parcial de array con `toMatchObject` falló porque el mock correctamente conservó el segundo tramo del martes y los siete días. Se sustituyó por la comparación exacta de los siete días; no hubo cambio de código productivo.
- Verificación final: cuatro specs de horario semanal, bloqueos y tarifas día/mes **6/6**; recorrido del wizard de cinco pasos **1/1**; Prettier, ESLint focal, typecheck y `git diff --check` PASS. El build no se repitió porque solo cambió el test.
- Chromium local con BFF mock y fixtures sintéticos. No se inició API/worker ni se accedió/modificó PostgreSQL, storage, roles o guards; no acredita aceptación o persistencia Go/DB. Gate integrado del orden 2 permanece pendiente por falta de target aislado autorizado.

## Zona horaria con CAS obsoleto antes de guardar horario semanal — 2026-11-11

- Se amplió el recorrido `weekly-schedule-editor.spec.ts` para que, tras guardar el horario sintético `00:00`–`24:00`, un escritor externo mock actualice la zona a `America/Punta_Arenas` y avance la revisión a `schedule-external-v7`.
- El navegador mantiene su revisión anterior y un intento local distinto (`America/Santiago` y cierre de martes `20:30`). La prueba verifica que `PUT /availability` envíe el ETag anterior y reciba 412, que no ocurra ningún `PUT /weekly-hours`, que el intento local siga visible hasta recargar, y que el reload lea la zona externa y el horario previamente confirmado (`00:00`–`24:00` y martes `20:00`).
- Verificación posterior al cambio: specs de horario semanal, bloqueos y tarifas día/mes **6/6**; wizard de cinco pasos **1/1**; Prettier, ESLint focal, typecheck y `git diff --check` PASS. Solo cambió cobertura de prueba/fixture; no se reconstruyó producto.
- El escritor, ETag y 412 son controlados por el BFF mock. Esto no demuestra CAS Go/DB ni persistencia real. No se inició API/worker ni se accedió/modificó PostgreSQL, storage o controles de seguridad; gate integrado sigue pendiente.

## Rango válido sin resultados para bloqueos — 2026-11-11

- `availability-block-calendar.spec.ts` ahora hace que el GET mock devuelva solo bloqueos que intersectan el intervalo UTC solicitado, en vez de devolver siempre todas las filas.
- RED observado antes de corregir el fixture: la consulta válida sin resultados no mostró el estado vacío porque el BFF mock devolvía todas las filas fuera de rango. Tras aplicar intersección inicio/fin con el rango pedido, el test pasó.
- Tras recuperar una lista válida, el test cambia a `Desde=2026-12-24` / `Hasta=2026-12-26`: comprueba un GET con `from=2026-12-23T00:00:00.000Z` y `to=2026-12-28T00:00:00.000Z`, estado vacío para ese período y ausencia de las dos filas fuera de rango. POST/PUT/DELETE y mutaciones semanales no aumentan. El reload posterior vuelve a consultar el rango inicial sintético y rehidrata las filas correspondientes.
- Playwright focal del spec **1/1**; suite de horario/bloqueos/tarifas día-mes **6/6** y wizard **1/1**. Prettier, ESLint focal, typecheck y `git diff --check` PASS; sin build, porque solo cambiaron tests.
- Escenario de BFF mock y fixture sintético; no demuestra filtro backend real ni persistencia. No se inició API/worker ni se accedió/modificó base, storage o controles de seguridad. El gate integrado permanece pendiente.

## Fallo temporal al guardar horario semanal — 2026-11-11

- `weekly-schedule-editor.spec.ts` ahora simula un `PUT /weekly-hours` 503 luego de que el `PUT /availability` sin cambio de zona respondió 200. El error visible es genérico, el intento `20:45` queda en el editor hasta recargar, y los contadores exigen un solo PUT semanal y un solo PUT de disponibilidad, sin replay automático.
- Tras `page.reload()`, un nuevo GET semanal rehidrata el valor autoritativo del fixture (`20:00`) y ambos contadores de escritura permanecen sin cambios.
- La primera corrida dio RED porque el mock todavía no respondía 503 a `weekly-hours` (la espera de respuesta agotó el timeout); se añadió solo la rama 503 al fixture, sin cambio de producto. El test focal terminó 1/1 y la suite semanal/bloqueos/tarifas día-mes terminó 6/6 después de formatear.
- `typecheck`, ESLint focal, Prettier focal y `git diff --check` pasaron. Solo BFF mocks/fixtures: el 503 y el GET de reload no demuestran commit, persistencia ni recuperación Go/DB; no se inició API/worker ni se accedió a base, storage o controles de seguridad. Orden 2 integrado sigue parcial.

## Validación de bloque manual sin llamadas de escritura — 2026-11-11

- `availability-block-calendar.spec.ts` cubre un intervalo manual con fin anterior al inicio, un intervalo de duración cero y motivo de 501 caracteres Unicode. La UI muestra las validaciones esperadas y las aserciones exigen cero POST de bloque y cero PUT de horario en los tres casos; no se modificó código de producto.
- Suite local de horario semanal, bloqueos y tarifas día/mes: **6/6**; `typecheck`, ESLint focal, Prettier focal y `git diff --check` PASS.
- Comprobación de UI con mocks/fixtures solamente; no demuestra validación Go ni persistencia. No se inició API/worker ni se accedió a la base persistente o controles de seguridad. Gate integrado sigue parcial.

## Lectura semanal fallida y recuperación explícita — 2026-11-11

- RED en `weekly-schedule-editor.spec.ts`: tras un GET `/weekly-hours` sintético 503, la pantalla no comunicaba que faltaba el horario y permitía entrar a una acción que podría guardar valores por defecto. La aserción falló por ausencia del mensaje esperado.
- Se corrigió `listing-editor.tsx`: el fallo de lectura muestra una alerta fija indicando recargar antes de editar; checkbox, tramos, zona y Guardar quedan deshabilitados, y el handler también retorna sin mutar si el horario no está disponible. Un nuevo GET correcto tras reload vuelve a habilitar la edición y rehidrata `20:00`; los contadores confirman cero PUT durante el error y la recuperación.
- La primera suite combinada posterior falló porque el fixture de bloqueos no servía GET semanal válido; se agregó una respuesta contract-shaped de siete días vacíos. No era un fallo del producto. Repetición final: horario/bloqueos/tarifas día-mes **6/6**; build Next.js 16.4.3 **32/32**, typecheck, ESLint focal, Prettier focal y `git diff --check` PASS.
- RED/GREEN solo con BFF mock/fixtures; no acredita respuesta o persistencia Go/DB. Sin API/worker, base persistente ni cambios a controles de seguridad. El gate integrado continúa parcial.

## Creación confirmada y fallo del GET inmediato — 2026-11-11

- Se amplió el test de calendario para simular un `POST /availability/blocks` 201 con DTO confirmado seguido de un `GET` 503. La UI conserva la fila devuelta por el POST; muestra el error de lectura; un refresh explícito recibe 200, mantiene la fila y no repite el POST ni emite PUT/DELETE. Luego, `DELETE` 204 seguido de otro GET 503 conserva la confirmación de borrado; el refresh 200 no repite DELETE y la fila sigue ausente. La prueba continúa con reload y rehidratación desde el GET mock.
- RED observado: el test esperaba el texto preciso `Bloqueo manual guardado.` y falló porque el aviso decía `Se actualizará la lista`, afirmación falsa tras fallar el GET. GREEN: `listing-editor.tsx` ahora anuncia únicamente el guardado confirmado; la prueba focal pasó **1/1** después de rebuild.
- Revalidación final del árbol reunido: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` **exit 0**: OpenAPI 200; ESLint 0 errores/4 warnings; typecheck; unitarias **494 (493/0/1 opt-in omitida)**; Prettier; build **32/32**; Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**. El escenario ampliado aparece como `availability-block-calendar.spec.ts` aprobado en la corrida completa.
- Solo Chromium/standalone y BFF fixture. El loopback `18092` no tenía listener para el test focal; los puertos `4213`, `4214` y `18082` estaban libres antes del check. No se inició API/worker ni se accedió o mutó DB, storage, roles o controles de seguridad. Gate integrado sigue parcial por falta de target aislado autorizado.

## Revalidación focal de horario/CAS y bloqueos — 2026-11-11

- Con `4213` y `18082` libres, se ejecutó `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 exec playwright test tests/weekly-schedule-editor.spec.ts tests/availability-block-calendar.spec.ts --reporter=line` — **2/2 aprobadas** en Chromium local.
- Se revalidó el CAS semanal con cambio de zona, respuesta 412 sin replay y recuperación por reload; también la creación de bloqueo manual con fallo de lectura posterior y reconciliación explícita. Todo usa BFF mocks/fixtures; no prueba CAS Go/DB ni persistencia. No se inició API/worker ni se accedió a servicios persistentes.

## Wall times válidas alrededor de transiciones DST — 2026-11-11

- `frontend/tooling/local-date-time.test.mjs` añade casos de conversión exacta para horas válidas antes y después del salto de primavera y de la repetición de otoño en `America/Santiago`. Los instantes UTC esperados también se contrastaron formateando esos instantes con `Intl.DateTimeFormat` en la zona IANA.
- `node --test tooling/local-date-time.test.mjs`: **5/5**; ESLint focal, Prettier y `tsc --noEmit`: PASS. Corrida Playwright combinada de `weekly-schedule-editor.spec.ts`, `availability-block-calendar.spec.ts` y `nonhourly-listing-schedule.spec.ts`: **4/4** en Chromium local. Se revalidaron CAS/412, rehidratación IANA para día/mes sin GET/PUT semanal, guardas de no POST para horas inexistentes/ambiguas y flujo mock de rango, refresh y reload.
- El test E2E se ejecutó con Next standalone en `127.0.0.1:4213`; el puerto `18082` no tenía listener y las solicitudes BFF del recorrido se resolvieron con fixtures Playwright. No se inició API/worker ni se accedió o modificó DB, storage, cuentas o controles de seguridad.
- Esta evidencia amplía conversión frontend/BFF mock, no valida reglas Go, datos de tzdata del servidor ni persistencia. El gate browser→BFF→Go→DB sigue parcial/bloqueado.

## Offsets IANA no horarios y rollover UTC — 2026-11-11

- Se añadieron pruebas de caracterización para `Asia/Kathmandu` (`+05:45`) al cruzar a la fecha UTC anterior y `America/St_Johns` (`-03:30`). Los instantes UTC esperados se contrastaron de forma independiente con `Intl.DateTimeFormat` en cada zona.
- `node --test tooling/local-date-time.test.mjs`: **7/7**; ESLint focal, Prettier y `tsc --noEmit`: PASS. Repetición final de las tres suites Playwright del orden 2 después de la edición de tests: **4/4**.
- Sin cambios de producción ni llamadas reales: Next standalone y BFF fixtures locales; no se inició API/worker ni se tocó DB, cuentas, storage o controles de seguridad. No acredita comportamiento de tzdata/validación Go ni persistencia; el gate integrado permanece bloqueado.

## Límite Unicode del motivo de bloqueo — 2026-11-11

- El recorrido UI de `availability-block-calendar.spec.ts` ahora envía un motivo de **500 codepoints** (`😀` repetido 500 veces) y verifica el DTO exacto de POST; la fixture responde 409 por ocupación conflictiva, sin crear la fila. El caso existente de 501 caracteres sigue mostrando validación y cero POST. El borde se contrastó con OpenAPI `ManualBlockInput.maxLength: 500` y Go `len([]rune(reason))`, pero no se ejecutó Go.
- La primera corrida falló solo porque la expectativa del test decía `18:00Z` aunque el fin local de entrada era `15:30` en `America/Santiago`; se corrigió la expectativa a `18:30Z`, sin cambios de producción. En el árbol corregido, las tres suites Playwright del orden 2 pasaron **4/4** y `node --test tooling/local-date-time.test.mjs` pasó **9/9**; ESLint focal, Prettier y `tsc --noEmit` también PASS.
- Todo el POST fue interceptado por fixture local; no hubo persistencia ni acceso a servicios reales. Gate integrado sigue bloqueado.

## Bloqueo manual con offset IANA fraccionario — 2026-11-11

- El recorrido de `availability-block-calendar.spec.ts` ahora rehidrata `Asia/Kathmandu` tras reload, verifica la zona en el editor y envía un bloqueo con horas locales `09:15`–`11:45`. La aserción del POST exige `2026-12-15T03:30:00Z`–`2026-12-15T06:00:00Z`; una conversión independiente con `zoneinfo` confirmó ambos instantes para `+05:45`.
- El fixture refleja la zona configurada en la respuesta de lista y en el DTO creado. La prueba verifica una sola tentativa/creación, el readback visible y cero mutaciones del horario semanal. Es una caracterización de UI con BFF fixture, sin cambio de código de producto.
- Playwright focal de bloqueos **1/1**; corrida combinada horario semanal/bloqueos/tarifas día-mes **4/4**; `local-date-time.test.mjs` **9/9**; ESLint focal, Prettier focal, `tsc --noEmit` y `git diff --check` — PASS.
- No se inició Go/API/worker ni se accedió a PostgreSQL, almacenamiento o servicios externos. Esto verifica conversión frontend→payload, no equivalencia de tzdata/reglas Go ni persistencia. El gate integrado del orden 2 continúa parcial/bloqueado por falta de target aislado autorizado.

## Bloque manual válido que cruza el salto DST de Santiago — 2026-11-11

- `availability-block-calendar.spec.ts` ahora rehidrata `America/Santiago`, selecciona el rango local 5–6 de septiembre de 2026 y envía un bloqueo desde `2026-09-05T23:30` hasta `2026-09-06T01:30`, cruzando la hora inexistente del inicio DST. El POST debe contener exactamente `2026-09-06T03:30:00Z`–`2026-09-06T04:30:00Z`; `zoneinfo` independiente confirmó ambos instantes. El GET del rango verifica `from=2026-09-04T00:00:00.000Z` y `to=2026-09-08T00:00:00.000Z`.
- El test verifica zona visible tras reload, una tentativa/creación, readback visible y ausencia de escrituras semanales. El fixture usa IDs únicos por cada alta sintética para no colisionar con el bloqueo previo de `Asia/Kathmandu`.
- Playwright focal de bloqueos **1/1** y combinado horario semanal/bloqueos/tarifas día-mes **4/4**; unitarias IANA/DST **9/9**; ESLint, Prettier, typecheck y `git diff --check` — PASS. No se reconstruyó porque solo cambió test/fixture.
- Solo Chromium local y BFF fixture. No se inició Go/API/worker ni se accedió a PostgreSQL, storage o servicios externos. No acredita tzdata/CAS/persistencia Go; el gate integrado del orden 2 permanece parcial y bloqueado por falta de target aislado autorizado.

## Bloque manual usando la zona recién guardada — 2026-11-11

- En `availability-block-calendar.spec.ts` se amplió el fixture CAS para recorrer en la misma sesión guardar `Pacific/Easter` (PUT de zona con `If-Match` v1→v2 y PUT semanal con v2→v3), cambiar al panel de disponibilidad sin reload y crear un bloqueo. La conversión independiente con `zoneinfo` para el 25 de diciembre de 2026 es `09:15`–`10:45` local → `14:15Z`–`15:45Z`.
- El caso comprueba el rango seleccionado 24–26 de diciembre (`from=2026-12-23T00:00:00.000Z`, `to=2026-12-28T00:00:00.000Z`), una sola creación, payload UTC exacto y readback con `time_zone: Pacific/Easter`. Por tanto prueba que el editor usa la zona configurada en la sesión, no una zona distinta recién seleccionada sin guardar.
- Playwright combinado horario semanal/bloqueos/tarifas día-mes **4/4**; unitarias IANA/DST **9/9**; ESLint, Prettier, `tsc --noEmit` y `git diff --check` — PASS.
- La prueba Go `TestValidTimeZoneUsesIANAZoneDatabase` incorpora `Pacific/Easter`; `GOPROXY=off GOTOOLCHAIN=local go test ./internal/occupancy -count=1` PASS. El paquete también cubre el parseo UTC/RFC3339 creciente del rango. Es prueba pura del servicio, sin DB; no valida conversión horaria Go ni CAS del repositorio.
- Evidencia solo Chromium y BFF fixtures sintéticos. Los ETags/CAS y el readback son simulados: no acreditan CAS de Go ni persistencia. Sin API/workers/DB y sin alterar controles de seguridad; gate integrado del orden 2 sigue parcial y bloqueado por el target aislado/artefactos faltantes.

## Rango con inicio vacío — 2026-11-12

- `availability-block-calendar.spec.ts` ahora prueba el rango con `Desde` vacío y `Hasta=2026-12-10`, separado del caso ya cubierto de `Hasta` vacío. «Actualizar fechas» muestra el error accesible, queda disponible para corregir y no produce GET; los contadores verifican cero POST, PUT o DELETE de bloque y cero mutaciones de horario.
- La corrida combinada Playwright de horario semanal, bloqueos y tarifas día/mes pasó **4/4** en Chromium local. `node --test tooling/local-date-time.test.mjs` pasó **9/9** y `GOPROXY=off GOTOOLCHAIN=local go test ./internal/occupancy -count=1` pasó. TypeScript, ESLint/Prettier focal también pasaron.
- Es cobertura no mutante con fixture/BFF mock; no hubo cambio de producto, build, API/worker, DB ni cambios de controles de seguridad. No prueba CAS ni persistencia Go/DB; el gate integrado del orden 2 sigue parcial por falta del target aislado autorizado y artefactos privados requeridos.

## Bloque manual conserva la zona guardada ante un selector sin guardar — 2026-11-12

- En `availability-block-calendar.spec.ts`, después de guardar `Pacific/Easter`, se cambia el selector a `America/Santiago` sin guardar, se vuelve a disponibilidad y se crea un bloqueo local `10:15`–`11:45`. El payload permanece basado en la zona vigente del espacio (`Pacific/Easter`): `2026-12-26T15:15:00Z`–`2026-12-26T16:45:00Z`; la zona pendiente de Santiago produciría `13:15Z` para el inicio. `zoneinfo` verificó ambas conversiones.
- El fixture comprueba el readback con `time_zone: Pacific/Easter`, una creación y cero PUT de zona/horario durante ese flujo. Corrida Playwright de horario semanal, bloqueos y tarifas día/mes: **4/4**; `local-date-time.test.mjs`: **9/9**; Go `internal/occupancy`: PASS; `tsc --noEmit`, ESLint y Prettier focal: PASS.
- Solo Chromium local y fixture/BFF mock. Sin API/worker, DB, controles de seguridad ni build. No demuestra CAS/convertibilidad Go o persistencia; el gate integrado sigue parcial por falta del target aislado autorizado y artefactos privados requeridos.

## Alcance del CAS frente a mutaciones de bloque manual — 2026-11-12

- Se cotejó `planning/openapi.yaml:4473-4525,4617-4638`: la creación y desactivación de bloque manual no declaran `If-Match`; el CAS por ETag corresponde al recurso de espacio/horario semanal. `availability-block-calendar.spec.ts` registra las cabeceras reales del mock para cada POST/DELETE y exige que no se envíe una precondición no soportada. La suite semanal mantiene por separado la secuencia ETag vigente → PUT → 412 y ausencia de replay.
- Playwright combinado de horario/bloqueos/tarifas día-mes **4/4**, unitarias IANA/DST **9/9**, Go `internal/occupancy` PASS; `tsc --noEmit`, ESLint, Prettier y `git diff --check` PASS.
- Caracterización solo frontend con BFF fixture. No acredita CAS backend ni persistencia. Sin API/worker, DB o cambios de seguridad; gate integrado continúa bloqueado por target aislado y artefactos faltantes.

## Identificadores IANA especiales y traversal — 2026-11-12

- `host-listings-domain.test.mjs` comprueba que `Local` (ubicación especial de Go, no identificador IANA para este contrato) y `../UTC` se rechazan en frontend, junto con zona desconocida y espacios externos. Go `validTimeZone` contiene las guardas correspondientes para `Local` y segmentos `..`; no se cambió la validación de producción.
- Corrida Playwright orden 2 **4/4**, pruebas de dominio y conversión fecha/hora **16/16**, Go `internal/occupancy` PASS; typecheck, ESLint y Prettier focal PASS. Pruebas de caracterización local; no son comprobación de tzdata compartida en producción.
- Sin API/worker/DB ni cambios de controles; gate integrado continúa parcial por falta del target aislado autorizado y los artefactos privados requeridos.

## Conflicto CAS de zona y bloqueo manual requiere reload — 2026-11-12

- Se añadió primero una regresión E2E: el fixture avanza la zona vigente a `America/Santiago` y el ETag a una revisión externa mientras la vista conserva `Pacific/Easter`; el guardado de zona/horario recibe 412. RED observado: faltaba el aviso de revisión obsoleta en el panel de disponibilidad.
- En `listing-editor.tsx`, un 412 de cualquier guardado del editor protegido por ETag marca obsoleta la instantánea de calendario para la sesión/espacio actual. La UI muestra el aviso y deshabilita «Crear bloqueo»; el handler también rechaza el envío obsoleto. La prueba verifica el 412 con el ETag anterior, ausencia de PUT semanal posterior y cero POST de bloque antes de reload.
- El test recarga la vista, confirma que el GET mock rehidrata `America/Santiago`, reactiva el formulario y crea el bloqueo `2026-12-26 10:15–11:15` local como `13:15–14:15Z`, con readback sintético. La conversión se confirmó independientemente con `zoneinfo`; mocks no prueban persistencia real.
- Build Next.js 16.4.3 **32/32**; Playwright de `availability-block-calendar`, `weekly-schedule-editor`, `nonhourly-listing-schedule` y `nonhourly-wizard-schedule`: **6/6**; `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs`: **16/16**; `GOPROXY=off GOTOOLCHAIN=local go test ./internal/spaces/transport/http ./internal/occupancy -count=1` PASS. `tsc --noEmit`, ESLint focal, Prettier focal y `git diff --check` — PASS.
- Solo Chromium local, BFF mocks y fixture sintético. No se inició API/worker ni se accedió o mutó DB, GCS, Cloud SQL, cuentas o controles de seguridad. Gate integrado de orden 2 sigue parcial/bloqueado por falta de target aislado y artefactos privados requeridos.

## Límites de rango de calendario revisados — 2026-11-12

- OpenAPI define `from`/`to` como timestamps UTC terminados en `Z`; el handler exige exactamente un valor de cada uno. Se amplió `TestCalendarAPIConfiguresQueriesCreatesAndDeletesWithOwnerChecks` con casos de lista de bloqueos para `from`/`to` faltantes, `to` vacío, `from` repetido y parámetro adicional; todos responden 400 `invalid_request`. `occupancy.parseInterval` exige timestamps RFC 3339 con `Z` y `to > from`; su prueba unitaria existente cubre intervalo válido, extremos iguales, orden invertido, fecha sin hora y offset no normalizado a UTC.
- La regresión de interfaz cubre rango de un día y de varios días con el query window emitido, `Desde`/`Hasta` faltantes y rango invertido sin GET. El contrato no declara una duración máxima: se conserva ese alcance y no se introduce una regla de producto arbitraria. Esto no acredita límites ajenos al contrato ni persistencia integrada.

## Adjudicación de revisión independiente — 2026-11-12

- Se revisó la observación de que la eliminación de un bloqueo sigue disponible tras un 412 de horario/zona. La política vigente de esta rebanada bloquea la creación hasta reload porque su conversión depende de la zona/revisión rehidratada; la eliminación opera sobre el ID explícito del bloqueo y su endpoint no declara `If-Match` en OpenAPI. No se amplió el guard a una mutación para la que el contrato no ofrece CAS; la prueba de calendario conserva la verificación de que POST/DELETE no inventan esa cabecera.
- Se descartó la hipótesis de que un GET semanal fallido reutilice el horario anterior del snapshot: `selectListing` resuelve ese fallo no-auth como `null` y publica `weeklySchedule: null`; el editor marca el horario no disponible y mantiene bloqueada la edición/guardado hasta reload exitoso. Es adjudicación por trazado de fuente, no por acceso a DB ni a API live.
- La revisión independiente fue source-only y no ejecutó tests. La revalidación consolidada del árbol se registra en la evidencia de orden 5; el gate browser→BFF→Go→DB/readback de orden 2 continúa parcial por falta del target aislado autorizado y artefactos privados.

## Caracterización adicional del parser UTC — 2026-11-11 local (2026-11-12 UTC)

- Se amplió `TestParseIntervalRequiresFiniteRFC3339IncreasingBounds` para rechazar día inexistente, hora `24:00`, segundo `:60` y sufijo `z` minúsculo; `TestParseIntervalPreservesFractionalUTCPrecision` caracteriza fracciones de nanosegundos y normalización a `time.UTC`. Es cobertura del parser real en Go, no integración HTTP/DB ni cambio productivo.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/occupancy ./internal/spaces/transport/http -count=1` PASS; `gofmt -d internal/occupancy/service_test.go` y `git diff --check HEAD` sin diferencias/errores. Sin API/worker, DB, GCS ni Cloud SQL; el gate integrado continúa bloqueado por falta del target aislado autorizado y artefactos privados requeridos.

## Rango HTTP→servicio sin lecturas de repositorio para intervalos inválidos — 2026-11-12

- Se añadió `TestCalendarBlockRangeRejectsInvalidTimestampsBeforeRepositoryRead` en `internal/spaces/transport/http/handler_test.go`. El handler real usa `occupancy.Service` real y un repositorio espía en memoria: fecha sin hora, offset distinto de UTC, fecha imposible, extremos iguales e intervalo invertido deben responder 422 `validation_error` sin llamar al método de listado del repositorio. Un intervalo UTC válido responde 200, llama una sola vez y llega como límites UTC exactos. Es una caracterización no mutante; no usa PostgreSQL.
- Verificación focal Go PASS; corrida `GOPROXY=off GOTOOLCHAIN=local go test ./internal/spaces/transport/http ./internal/occupancy -count=1` PASS; `go vet ./internal/spaces/transport/http ./internal/occupancy` PASS; `gofmt -d` y `git diff --check HEAD` limpios.
- `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs`: **16/16**. Playwright local/BFF fixtures para horario semanal, bloqueos y tarifas día/mes: **4/4** en Chromium. Puertos `4224` y `18092` quedaron libres al finalizar. No hubo build, API/worker, DB, cuenta, GCS, Cloud SQL ni cambios de controles.
- Esta cobertura comprueba el límite handler→dominio→repositorio con fake; no demuestra ejecución SQL, CAS PostgreSQL ni persistencia. El gate integrado del orden 2 permanece parcial/bloqueado.

## Query HTTP malformado rechazado antes de consultar disponibilidad — 2026-11-12

- La caracterización detectó que `URL.Query()` descarta silenciosamente errores de escape: con `from`/`to` válidos y un componente `%ZZ`, el handler respondía 200 y consultaba el repositorio. Se añadió una regresión que exige 400 `invalid_request` y cero lecturas; primero falló con 200 y calendario vacío.
- El handler ahora parsea `RawQuery` con `url.ParseQuery` antes de validar la ventana en ambos GET (disponibilidad y bloqueos); el test verifica `%ZZ` con respuesta 400 y cero lecturas en ambos endpoints, y que un intervalo invertido o `spaceId` inválido en disponibilidad responde 422 sin leer su repositorio. `go test ./internal/spaces/transport/http -run '^TestCalendarBlockRangeRejectsInvalidTimestampsBeforeRepositoryRead$' -count=1`, suites `internal/spaces/transport/http` + `internal/occupancy`, `go vet`, `gofmt -d` y `git diff --check` PASS.
- La revisión independiente source-only `deleg_776c3e67` no encontró defectos ni concerns de seguridad. Su sugerencia de caracterizar explícitamente el segundo call site fue incorporada y el test focal y las suites Go se repitieron.
- Es validación HTTP con handler/servicio real y repositorio fake; no ejecuta SQL ni acredita CAS/persistencia. Sin API/worker, DB, GCS/Cloud SQL o cambios de roles/controles.

## Servicio Go — validación/delegación de zona y revisión CAS — 2026-11-12

- Se añadió `TestSetTimeZoneCASValidatesAndForwardsIANARevision` en `internal/occupancy/service_test.go`. Con repositorio fake, rechaza UUID propietario genérico y con hex inválido, UUID de espacio inválido, zona vacía/desconocida/`Local`/ruta `../UTC` y revisión cero, exigiendo cero llamadas al repositorio. El caso válido usa `Pacific/Easter`, comprueba que se reenvían propietario, espacio, zona y revisión exactos y que se devuelve la revisión renovada del stub.
- Test focal y suite `internal/occupancy` PASS; suites `internal/spaces/transport/http` + `internal/occupancy` PASS; `go vet`, `gofmt -d` y `git diff --check` PASS. Perfil local: `SetTimeZoneCAS` 100% de statements del método; es cobertura del servicio con fake, no CAS PostgreSQL.
- `deleg_121eed36` revisó la versión inicial sin defectos ni concerns y sugirió las clases de entrada incorporadas; el seguimiento `deleg_ea491581` aprobó el test ampliado sin blockers. Ambas revisiones fueron source-only y no ejecutaron pruebas. El fake no demuestra la atomicidad CAS de PostgreSQL. Sin API/worker, DB/GCS/Cloud SQL ni cambios de controles; el gate integrado de orden 2 permanece parcial.

## Servicio Go — configuración y lectura de zona IANA — 2026-11-12

- Se añadieron `TestSetTimeZoneValidatesAndForwardsIANAZone` y `TestTimeZoneValidatesAndForwardsSpaceLookup`. Con repositorio fake, el primero rechaza zona vacía/desconocida/reservada/path-like sin delegación y verifica el envío exacto de `Pacific/Easter`; el segundo rechaza UUID de propietario/espacio inválido sin lectura y verifica el valor/zona devuelto por el stub.
- Focales, suite `internal/occupancy`, suite combinada `internal/spaces/transport/http` + `internal/occupancy`, `go vet`, `gofmt -d` y diff-check PASS. Perfil de cobertura del paquete: `SetTimeZoneCAS`, `SetTimeZone` y `TimeZone` **100% de statements**; es validación/delegación unitaria con fake, no persistencia/readback PostgreSQL.
- `deleg_b0f0b39d` revisó source-only los tres tests de servicio sin encontrar defectos ni concerns; señaló añadir UUID inválidos explícitos para `SetTimeZone`, y se incorporaron owner/space inválidos con aserción de cero llamadas. Tras ello, las pruebas focales, suites `occupancy` + `spaces/transport/http`, `go vet`, `gofmt -d` y diff-check volvieron a pasar; los métodos de zona permanecen en 100% statements con fake. La revisión no ejecutó tests y no demuestra CAS/persistencia PostgreSQL. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL; el gate integrado de orden 2 permanece parcial.

## Servicio Go — consulta de rango de disponibilidad y bloqueos — 2026-11-12

- Se añadieron `TestAvailabilityValidatesAndForwardsUTCWindow` y `TestListBlocksValidatesAndForwardsUTCWindow`. Los casos inválidos (owner/space, inicio/final vacío, límites iguales/invertidos y offset distinto de `Z` en disponibilidad) exigen `ErrInvalid` y cero llamadas al repositorio. Los casos válidos comprueban delegación de límites en UTC con precisión nanosegundo, owner/space exactos y resultado del fake.
- Focales, suite `internal/occupancy`, suite combinada `internal/spaces/transport/http` + `internal/occupancy`, `go vet`, `gofmt -d` y diff-check PASS. Cobertura del paquete: `Availability` y `ListBlocks` 100% de statements con fake; no prueba consultas SQL, autorización ni persistencia/readback.
- Revisión independiente source-only `deleg_64d5b4b6`: PASS, sin bloqueantes ni sugerencias. Confirmó que entradas inválidas llaman al servicio y exigen `ErrInvalid` más cero llamadas al repositorio; los casos válidos comprueban llamada única, owner/space, instantes UTC normalizados con precisión fraccional y objeto completo devuelto. La revisión no ejecutó pruebas y el fake no demuestra SQL, autorización ni persistencia.
- Revalidación local coordinada posterior a la revisión: `go test ./internal/occupancy ./internal/spaces/transport/http -count=1`, `go vet` focal, `gofmt -d` y `git diff --check HEAD` PASS. Frontend Next.js 16.4.3 `pnpm build` PASS (32/32 páginas); Playwright Chromium local para horario semanal, bloqueos manuales y tarifas día/mes **6/6**; unitarias IANA/DST y dominio anfitrión **16/16**. Puerto de ensayo 4217 quedó libre. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL; el gate integrado de orden 2 permanece parcial y bloqueado por falta de target aislado autorizado y readback.

## Rango HTTP de disponibilidad — rechazo de offset no UTC — 2026-11-12

- Ampliada `TestCalendarBlockRangeRejectsInvalidTimestampsBeforeRepositoryRead`: el `GET /availability` con límites `-03:00` responde 422 `validation_error` y cero lecturas del repositorio. La ruta de bloques ya tenía este caso; esta caracterización cierra el mismo límite para la ruta de disponibilidad. Cambio solo de test, sin cambio productivo.
- RED/GREEN no aplica: el handler ya compartía `queryWindow`, así que es una caracterización de comportamiento existente. El test focal y las suites `internal/spaces/transport/http` + `internal/occupancy` pasaron; `go vet`, `gofmt -d` y `git diff --check HEAD` también. Solo handler/servicio con repositorio contador en memoria; sin API/worker, DB, GCS, Cloud SQL ni cambios de controles.

## Aislamiento de bloqueos al cambiar de espacio — 2026-11-12

- Se añadió `frontend/tests/availability-block-scope.spec.ts`: con dos espacios sintéticos en `America/Santiago`, mantiene pendiente el GET de bloqueos de A, navega por «Mis Espacios» a B, comprueba el bloque de B y libera después la respuesta tardía de A; B permanece visible y A no aparece. Se verifican únicamente GETs y cero mutaciones. El recorrido cubre la transición visible entre rutas; no intenta cambiar el `listingId` dentro de una misma instancia montada del editor.
- La primera ejecución focal falló por selector ambiguo entre el enlace de navegación y el breadcrumb; el selector se limitó a nombre exacto y la repetición pasó. Consolidación local final: Playwright Chromium horario/bloqueos/tarifas **7/7**, unitarias IANA/DST y dominio anfitrión **16/16**, `pnpm typecheck`, ESLint y Prettier focal PASS; Go `internal/spaces/transport/http` + `internal/occupancy`, `go vet`, `gofmt -d` y `git diff --check HEAD` PASS. Se usó el standalone local ya construido; no hubo cambios de código de producción.
- Todo es evidencia de browser/BFF mock, fixtures y repositorios en memoria; no acredita API/Go live, SQL, autorización, CAS PostgreSQL ni persistencia. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL; no se alteraron controles. El gate integrado de orden 2 sigue detenido a la espera de un target aislado autorizado y readback.

## Rango de fechas durante refresh y consulta del rango nuevo — 2026-11-12

- `availability-block-calendar.spec.ts` mantiene pendiente una lectura GET sintética. Mientras `calendarLoading` está activo, verifica que los campos `Desde`/`Hasta`, el botón de refresh y el formulario de creación estén deshabilitados; así el usuario no puede cambiar la ventana de una lectura en curso. Al completar el GET, cambia explícitamente al rango 2026-12-24–26 y refresca: la URL usa `from=2026-12-23T00:00:00.000Z` y `to=2026-12-28T00:00:00.000Z`, muestra el estado vacío del nuevo período y no presenta filas exclusivas del rango anterior. Después restaura y confirma el rango previo antes de continuar el resto del recorrido.
- La primera caracterización intentó editar fechas durante el GET pendiente y falló porque la UI ya deshabilita esos campos; no se forzó el DOM ni se cambió el producto. Se reformuló como regresión de los estados disabled y de refresh explícito del rango nuevo, respetando el flujo visible.
- Verificación consolidada del árbol final: Playwright Chromium con fixtures para horario semanal, bloqueos y tarifas día/mes **6/6**; pruebas Node IANA/dominio **16/16**; Go offline `internal/occupancy` + `internal/spaces/transport/http` y `go vet` PASS; `pnpm typecheck`, ESLint focal, Prettier y `git diff --check HEAD` PASS. `pnpm build` PASS (32/32 páginas) antes de la edición, que fue solo de test; no se necesitó reconstruir el standalone.
- Solo BFF mocks/fixtures y repositorio en memoria; cero escrituras externas. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL; no se alteraron roles ni controles. Esto no cierra el gate integrado browser→BFF→Go→DB/readback.

## DELETE de bloqueo por ID tras conflicto de zona — 2026-11-12

- Se amplió `availability-block-calendar.spec.ts` para caracterizar el alcance adjudicado del 412 de zona/horario: se deshabilita crear bloqueos hasta reload, pero un bloqueo manual ya existente conserva su acción de DELETE por ID. El mock elimina el bloque por UUID; la prueba verifica que desaparezca, no se emita otro POST y el DELETE no incluya `If-Match`, que no está en el contrato OpenAPI de estos endpoints.
- El fixture de DELETE ahora resuelve cualquier ID perteneciente a la ruta del espacio, en vez de aceptar solo `createdBlockId`, permitiendo comprobar el bloqueo previamente guardado.
- Se añadió un resultado ambiguo de creación: el mock agrega el bloqueo y responde 503. La UI no reintenta POST ni hace GET automático; el usuario puede ejecutar refresh explícito, que muestra el bloqueo ya confirmado por el fixture. Se comprueba un único intento POST y ausencia de `If-Match`; esto caracteriza reconciliación visual, no un commit real del servidor.
- Se cubrió el espejo para DELETE: el mock retira el bloqueo y devuelve 503. La UI conserva la fila local hasta refresh explícito; el GET posterior la retira y no se repite DELETE. Se verifica un solo intento sin `If-Match`. Es estado del fixture, no evidencia de una eliminación durable ni de autorización en Go.
- `weekly-schedule-editor.spec.ts` simula un PUT que actualiza el horario semanal en el fixture y responde 503. La UI conserva el valor editado sin reintentar; tras reload, GET muestra el horario actualizado (20:15), con exactamente un PUT. Caracteriza resultado incierto/CAS en UI, no persistencia ni comparación SQL real.
- En otro punto del mismo recorrido, cambiar a `America/Coyhaique` confirma primero el PUT de zona y el PUT semanal responde 503 sin commit. El reload rehidrata la zona nueva y el horario anterior (`20:00`), evidenciando la escritura parcial secuencial en el fixture sin afirmar atomicidad entre endpoints ni iniciar reintentos.
- La prueba intenta guardar de nuevo antes del reload: la revisión compartida ya quedó obsoleta por el commit simulado, por lo que disponibilidad devuelve 412 y no se emite un segundo PUT de horario. El valor local se conserva hasta reload, que reconcilia 20:15. Esto prueba la secuencia con ETags sintéticos, no CAS concurrente en PostgreSQL.
- Validación final tras formatear los specs: Playwright Chromium **6/6** (horario semanal, bloqueos, editores/wizard día-mes) en `127.0.0.1:4233`; Node IANA/dominio **16/16**; `pnpm typecheck`, ESLint/Prettier focal de ambos specs y `git diff --check` PASS. Puerto de ensayo y endpoint backend simulado `18129` quedaron libres. No se reconstruyó porque solo cambiaron tests.
- Solo standalone local y BFF fixtures sintéticos; las mutaciones simuladas no alcanzaron Go ni DB. No se inició API/worker ni se tocó base persistente o controles. La cobertura no prueba autorización backend/CAS SQL/persistencia; el gate integrado sigue parcial.

## Mensaje de guardado parcial de zona y horario — 2026-11-12

- Se agregó una aserción RED para el caso en que `PUT /availability` confirma la zona y el `PUT /weekly-hours` posterior responde 503. `listing-editor.tsx` ahora comunica explícitamente que la zona quedó guardada, que el resultado del horario no está confirmado y recomienda recargar para reconciliar antes de volver a guardar. En 412 tras el primer PUT, mantiene el guard de revisión obsoleta y agrega contexto de que el horario no se confirmó.
- El test verifica el mensaje después del 503; otro escenario conserva el estado incierto: fixture que aplica el horario y responde 503, sin replay, revisión obsoleta en el siguiente intento y rehidratación del estado simulado tras reload. No se afirma atomicidad entre endpoints ni persistencia real.
- Una corrida consolidada detectó una lectura GET del fixture de calendario que aún podía estar en vuelo desde el refresh de rango/lectura posterior a una creación exitosa. El spec ahora espera explícitamente ambos GETs y la reactivación de controles antes de tomar la línea base del POST ambiguo; la aserción de cero refresh automático queda sincronizada, sin cambiar el comportamiento de producto.
- Verificación local: build Next.js 16.4.3 **32/32**; Playwright Chromium, horario semanal/bloqueos/tarifas día-mes **6/6**; Node IANA/dominio **16/16**; `pnpm typecheck`, ESLint focal, Prettier y `git diff --check` PASS. Puertos `4238` y `18134` quedaron libres al terminar.
- Solo standalone local y BFF mocks/fixtures; no se inició API/worker ni se accedió o modificó DB persistente, storage, cuentas o controles de seguridad. No acredita CAS SQL ni persistencia Go/DB; gate integrado del orden 2 sigue parcial/bloqueado por falta de target aislado autorizado.

## Conflicto CAS semanal y creación de bloque manual — 2026-11-12

- Se amplió `weekly-schedule-editor.spec.ts` para continuar el escenario de CAS: tras el 412 sintético del segundo `PUT /weekly-hours`, el editor cambia al panel de bloqueos; un GET fixture devuelve un bloqueo ya existente, la UI identifica la revisión de calendario obsoleta y «Crear bloqueo» está deshabilitado. La eliminación por ID permanece habilitada, pero el test no la ejecuta: contadores POST/DELETE en cero.
- Antes de liberar ese PUT semanal, el fixture lo retiene después de confirmar el PUT de zona. Mientras el guardado sigue pendiente, el botón semanal y «Crear bloqueo» quedan deshabilitados; una fila fixture existente mantiene «Eliminar bloqueo» habilitado. El test cambia al panel, espera el GET fixture y exige cero POST/DELETE. Después libera la respuesta 412 y confirma la misma affordance sin ejecutar la eliminación; luego continúa el escenario de obsolescencia/reload. Caracterización de UI solamente.
- Un refresh manual adicional también devuelve 200, pero conserva el aviso de revisión obsoleta y «Crear bloqueo» deshabilitado; leer solo los bloqueos no renueva la ficha/ETag. Solo tras `page.reload()`, el BFF fixture sirve un nuevo GET y el formulario vuelve a habilitarse; sigue habiendo cero POST. Esto caracteriza el guard del cliente y no ejecuta una creación manual.

- Revalidación final de la rebanada: Playwright Chromium horario/bloqueos/tarifas día-mes **6/6**; Node IANA/dominio **16/16**; typecheck, ESLint, Prettier y `git diff --check` PASS. `GOPROXY=off GOTOOLCHAIN=local go test` y `go vet` pasaron para `internal/occupancy`, `internal/spaces/transport/http` y `internal/booking/transport/http`; `gofmt -d` limpio. Solo cambiaron test/fixture y documentación; no se reconstruyó en esta rebanada.
- No se inició API/worker ni se accedió a DB persistente, storage, cuentas o controles de seguridad. La cobertura usa ETags y respuestas sintéticas; no demuestra CAS SQL ni persistencia. Gate integrado del orden 2 permanece bloqueado por falta de target aislado autorizado.

## Contrato Go local de horario semanal — 2026-11-12

- Se agregaron caracterizaciones unitarias con servicio/repository fake: `PUT /weekly-hours` rechaza `time_zone`, JSON adicional y rutas/query fuera del contrato sin llamar al servicio. El servicio CAS rechaza UUID owner/espacio inválidos, revisión cero y horario inválido antes de invocar el repository; valida nil service/repository; normaliza a UTC el ETag con offset `-03:00`; y el dispatcher falla cerrado si el repository no implementa la interfaz owner-scoped.
- `decodeOwnerWeeklyHours`, `ownerWeeklyHoursSpaceID`, `OwnerWeeklyHours` y `SaveOwnerWeeklyHoursCAS` alcanzan 100% de cobertura por función en sus paquetes. La corrida consolidada `go test`/`go vet` offline pasó para `internal/occupancy`, `internal/spaces/transport/http`, `internal/booking/transport/http` e `internal/booking`; no se inició API ni se accedió a PostgreSQL. Es cobertura de handler/servicio y fakes, no CAS SQL, persistencia ni autorización integrada.

## Comparación de horario sin mutar entradas — 2026-11-12

- `internal/adapters/postgres/booking/weekly_hours_equivalence_test.go` añade una caracterización del comparador usado para reconocer horarios semanales equivalentes: días y períodos en distinto orden, zonas distintas (metadata de espacio) y deep-equality de ambas entradas antes/después. Protege el camino de no-op contra ordenamientos incidentales sobre slices del schedule recibido o leído.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/adapters/postgres/booking -run 'TestWeeklyScheduleEquivalent' -count=1` y `go vet ./internal/adapters/postgres/booking` — PASS; `gofmt` y `git diff --check` — PASS.
- Es caracterización del helper puro con valores sintéticos; no ejecuta transacción, CAS PostgreSQL ni prueba persistencia. Sin API/worker, DB, GCS/Cloud SQL o cambios de controles de seguridad; el gate integrado del orden 2 permanece parcial/bloqueado.

## Compatibilidad local del alias de zona `EST` — 2026-11-12

- El dominio frontend ya caracteriza `Intl.DateTimeFormat` aceptando `EST`; se contrastó el mismo identificador con `time.LoadLocation` del runtime Go que incluye `time/tzdata`. Ambos aceptaron `EST`, `America/Santiago` y `Pacific/Easter`, y rechazaron `../UTC`; la validación Go además rechaza explícitamente `Local`, que `LoadLocation` por sí solo acepta.
- Se incorporó `EST` a la tabla positiva de `TestValidTimeZoneUsesIANAZoneDatabase` para fijar esta compatibilidad en Go sin alterar validadores ni normalizar el alias. `go test ./internal/occupancy -run '^TestValidTimeZoneUsesIANAZoneDatabase$' -count=1`, `go vet ./internal/occupancy`, `node --test tooling/host-listings-domain.test.mjs` (**7/7**) y `gofmt -d` — PASS.
- Es comprobación de runtimes locales, no garantiza igualdad de bases tzdata entre despliegues ni define un catálogo canónico. Sin API/worker, DB/GCS/Cloud SQL o cambios de controles; el gate integrado del orden 2 sigue parcial/bloqueado.

## Revalidación final local del orden 2 — 2026-11-12

- Tras añadir ambas caracterizaciones Go, Playwright Chromium para horario semanal, bloqueos, cambio de espacio y tarifas día/mes pasó **7/7** en el standalone existente (puerto 4270; backend solo fixture en 18170). Node IANA/dominio pasó **16/16**.
- Go offline pasó `internal/adapters/postgres/booking` con `TEST_DATABASE_URL` no definido, además de `internal/occupancy`, `internal/spaces/transport/http`, `internal/booking` e `internal/booking/transport/http`; `go vet`, `gofmt -d` y `git diff --check` también PASS.
- Solo se añadieron pruebas/evidencia; no se cambió código productivo en esta subrebanada ni se reconstruyó. Los GET/PUT/POST/DELETE de browser se resuelven con fixtures y no alcanzan Go live ni una base; los tests fake/source no prueban CAS SQL, autorización integrada o persistencia. No se inició API/worker ni se accedió a DB persistente, GCS/Cloud SQL o controles de seguridad. Gate integrado continúa pendiente del target aislado autorizado.

## Instantes DST exactos en Go para `America/Santiago` — 2026-11-12

- Se amplió `TestWeeklyHoursDSTGapsAndRepeatedTimes`: además de reconocer el hueco de primavera y las dos ocurrencias de `23:00` en el pliegue de otoño, exige los UTC exactos `2026-09-06T04:00:00Z` para la primera hora válida `01:00`, y `2026-04-05T02:00:00Z` / `2026-04-05T03:00:00Z` para ambas ocurrencias de `23:00`.
- Los instantes se contrastaron con Python `zoneinfo`; Node IANA/dominio pasó **16/16**. `go test` focal DST y paquete `internal/booking`, `go vet`, `gofmt -d` y `git diff --check` — PASS.
- Caracterización de reglas tzdata locales con datos sintéticos; no toca la DB y no prueba equivalencia de tzdata en despliegues ni disponibilidad integrada. Sin API/worker o cambios de controles; el gate integrado sigue parcial/bloqueado.

## Servicio Go de bloqueos manuales con repository fake — 2026-11-12

- El perfil de cobertura local encontró `CreateBlock` y `DeleteBlock` sin cobertura. Se añadieron pruebas fake-backed de validación UUID, timestamps RFC3339, orden del rango, motivo vacío/límite Unicode de 500 codepoints, error de generación de ID, normalización UTC, trimming, forwarding owner-scoped y propagación de errores del repository. Se cubrieron además los casos de dependencias nil de `NewService`.
- Corrida offline con `TEST_DATABASE_URL` sin definir: `go test` y `go vet` pasaron para `internal/occupancy`, `internal/spaces/transport/http`, `internal/booking`, `internal/booking/transport/http` e `internal/adapters/postgres/booking`. `go test -coverprofile=... ./internal/occupancy` — **100.0%** de cobertura total; cada función del paquete, incluida `CreateBlock` y `DeleteBlock`, **100%**. `gofmt -d` y `git diff --check` — PASS.
- Las operaciones de creación/borrado del test se ejecutan exclusivamente contra un fake en memoria; no se llamó un repository PostgreSQL ni se inició API/worker. Esto caracteriza servicio y validación, no persistencia, CAS SQL o autorización integrada. Gate integrado sigue parcial/bloqueado.

## Validación HTTP de alta de bloqueo manual antes de cualquier escritura — 2026-11-12

- `TestManualBlockCreateHTTPRejectsInvalidInputBeforeRepositoryWrite` envía seis cuerpos inválidos a `POST /spaces/{id}/availability/blocks` mediante el handler y `occupancy.Service` reales: fecha sin hora, offset `-03:00`, límites iguales, rango invertido, motivo vacío tras trim y motivo de 501 codepoints Unicode. Cada respuesta es 422 `validation_error`; el contador del repositorio fake permanece en cero.
- Esto complementa las validaciones unitarias del servicio con el contrato HTTP y confirma el mapeo de error y ausencia de llamada downstream antes de escritura. Es caracterización de una ruta de creación simulada con fake, no una mutación persistente; la escritura de DB real, owner/ACL SQL y carreras de reservas quedan fuera.
- Revalidación final local: Playwright Chromium de `weekly-schedule-editor.spec.ts`, `availability-block-calendar.spec.ts`, `availability-block-scope.spec.ts`, `nonhourly-listing-schedule.spec.ts`, `nonhourly-wizard-schedule.spec.ts` y el caso de `wizard-improvements.spec.ts` **8/8**; `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs` **16/16**. Go offline `go test` y `go vet` PASS para `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`; `gofmt -d` y `git diff --check` PASS.
- Playwright usó el standalone local y fixtures BFF; los tests Go no recibieron variables de conexión PostgreSQL y usan repositorios fake. No se inició API/worker, no se accedió/modificó base persistente, storage, cuentas o controles de seguridad. El listener observado en `127.0.0.1:15439` quedó intacto y fuera de las pruebas. La cobertura no demuestra CAS SQL, autorización integrada ni persistencia; el gate integrado de orden 2 sigue parcial porque falta un perfil/harness integrado autorizado; el target DB-only disponible no se usó.

## Bordes de validación de horario semanal y zona — 2026-11-12

- Se añadieron tests de caracterización para forma de siete días incompleta, más de dos períodos, reloj `9:00` no canónico, zona IANA inexistente/path-like, período habilitado de duración cero y ordenación de períodos sin mutar el input. No se cambió código productivo.
- Corrida `go test -coverprofile` sobre `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`: los cuatro paquetes PASS; cobertura de `parseWeeklyClock` **100%**, `normalizeWeeklyDays` **100%**, `ValidateWeeklyHours` **95.7%**, `IntervalFitsWeeklyHours` **95.6%**, y todo `internal/occupancy` **100%**. `internal/booking` total **29.4%** corresponde al paquete amplio que también incluye reserva, pago y administración; no representa cobertura de la fase de horarios. `go vet`, `gofmt -d` y `git diff --check` PASS.
- El statement restante de “weekday no visto” no es alcanzable: con exactamente siete elementos, valores 1–7 y sin duplicados, todo weekday está presente; cualquier omisión fuerza un duplicado rechazado antes. El guard de `segmentEnd <= cursor` es defensivo frente a límites de zona sin avance; no se construyó una zona artificial ni se alteró la lógica para subir cobertura.
- Ejecución offline con variables PostgreSQL explícitamente desactivadas y repositorios falsos. Sin API/worker, acceso a base persistente, storage, cuentas o controles; no prueba CAS SQL, persistencia ni autorización integrada. Gate integrado de orden 2 continúa parcial.

## Precondición semanal ligada al espacio — 2026-11-12

- Se amplió `TestOwnerWeeklyHoursPUTChecksIfMatchBeforeBodyAndMapsCAS` con una etiqueta fuerte y bien formada generada para otro UUID de espacio, junto con un body JSON malformado. El handler responde 412 `precondition_failed` antes de decodificar el body o llamar al servicio; el fake registra cero escrituras y la respuesta no emite ETag renovado.
- El test focal pasó con todos sus subtests; también pasaron los paquetes offline `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`, `go vet`, `gofmt -d` y `git diff --check`.
- Es caracterización del límite HTTP usando autenticador y servicio fake: el `If-Match` de otro recurso se rechaza antes del CAS del repositorio. No demuestra concurrencia PostgreSQL, persistencia ni autorización integrada. Variables de conexión DB y API no definidas; sin API/worker ni alteración de controles.

## Revalidación focal de orden 2 — 2026-11-12

- Se repitieron en Chromium local los casos focales de zona IANA/horario CAS, bloqueo manual, rango/refresh/reload, cambio de espacio con GET pendiente y tarifa día/mes: **8/8**. `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs`: **16/16**.
- Con variables de conexión PostgreSQL desactivadas: `go test -count=1` y `go vet` pasaron para `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`; `gofmt -d` y `git diff --check` PASS.
- BFF/Playwright usa fixtures, y Go ejecuta suites offline. No se inició API/worker ni se accedió a DB, GCS, Cloud SQL o al listener ajeno observado en sesiones previas; no hubo cambios de código en esta revalidación. CAS SQL, autorización integrada y persistencia siguen sin comprobarse; el gate integrado permanece detenido hasta contar con perfil/harness aislado autorizado.

## Continuación no mutante de orden 2 — revalidación de CAS, rango y refresh — 2026-11-12

- Reejecuté en Chromium local los seis specs de horario semanal, bloqueos, tarifas día/mes y recuperación de wizard, con el filtro de escenarios de zona IANA, CAS 412, rango/refresh, rehidratación y cambio de espacio: **8/8**. La lista de bloqueos con refresh pendiente sigue aislada por espacio; zona/horario y bloqueos se cubren únicamente con fixtures/BFF mock.
- `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs`: **16/16**. Con variables PostgreSQL retiradas y `GOPROXY=off`, `go test -count=1` y `go vet` pasaron para `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`; `gofmt -d` no mostró diferencias.
- Los puertos de ensayo 4292/18131 estaban libres antes y después; no se inició API/worker ni se accedió a DB/GCS/Cloud SQL o al listener ajeno. `git diff --check HEAD` PASS. No se detectó en esta relectura una brecha contractual nueva que justificara inventar CAS para POST/DELETE manual; no hubo cambios de código de producto o pruebas.
- Evidencia solo de UI/handler/servicio con fixtures y fakes; no demuestra CAS PostgreSQL, autorización integrada ni readback persistido. El gate integrado permanece detenido por falta de perfil/harness autorizado y las precondiciones específicas del target.

## Horario semanal deshabilitado — intervalo positivo y límite vacío — 2026-11-12

- `internal/booking/weekly_hours_test.go` añade `TestIntervalFitsWeeklyHoursIgnoresDisabledSchedule`: con un horario semanal existente pero `Enabled=false`, un intervalo positivo fuera de los períodos configurados no queda restringido; un intervalo de duración cero sigue siendo rechazado. Es una caracterización del contrato local del helper, sin cambio de producción ni intento de elevar cobertura mediante entradas inválidas.
- RED/GREEN no aplica: el comportamiento existente del helper ya cumple la regla y se conserva como test-only characterization. Focal, paquete `internal/booking`, suite offline de los cuatro paquetes de orden 2, `go vet`, `gofmt -d` y `git diff --check HEAD` PASS.
- Tras el cambio: Playwright focal de zona IANA/CAS, horario, bloqueos, refresh/reload, cambio de espacio y tarifas día/mes **8/8**; Node fecha/zona/dominio **16/16**. Todos los recorridos browser usan BFF fixtures; Go usa datos en proceso. Sin API/worker ni acceso a DB/GCS/Cloud SQL o controles. No demuestra persistencia ni CAS SQL; el gate integrado del orden 2 permanece detenido hasta contar con target/harness y autorización específicos.

## Intervalos que abarcan períodos semanales adyacentes — 2026-11-12

- Análisis del contrato: `ValidateWeeklyHours` rechaza solapamientos estrictos (`open < previous.close`) pero permite períodos adyacentes; la validación frontend usa la misma condición. Reproduje una brecha real con un horario sintético válido y desordenado (`13:00–17:00`, `09:00–13:00`): una reserva de jueves `12:00–14:00` cruzaba el límite continuo y `IntervalFitsWeeklyHours` la rechazaba.
- TDD: `TestIntervalFitsWeeklyHoursAcrossAdjacentPeriods` pasó primero por `ValidateWeeklyHours` y falló después en la aserción de ajuste por la causa esperada. El cambio mínimo en `internal/booking/weekly_hours.go` ordena los tramos del día y acumula cobertura contigua; no salva huecos, que siguen rechazados por la prueba existente de pausa.
- Verificación posterior: test de regresión y test de horario deshabilitado focal PASS; `go test -count=1` para los cuatro paquetes de orden 2, `go vet`, `gofmt -d` y `git diff --check HEAD` PASS. La validación Node **16/16** y Playwright mock **8/8** corresponden a la revalidación frontend separada; no las presento como integración del cambio Go.
- Sin API/worker, DB/GCS/Cloud SQL o modificaciones de seguridad. Esta corrección valida lógica Go en proceso; no acredita CAS SQL, autorización integrada ni persistencia. El gate integrado continúa detenido por falta de target/harness aislado autorizado.

## Cobertura medida tras la corrección — 2026-11-12

- `go test -coverprofile="$TMPDIR/order2-booking-current.cover" ./internal/booking` PASS; `go tool cover -func`: `IntervalFitsWeeklyHours` **98.1%**, `ValidateWeeklyHours` **95.7%**, `parseWeeklyClock` **100%**, `normalizeWeeklyDays` **100%**. El 30.1% total del paquete incluye reserva/pagos/administración y no describe esta tarea.
- Las únicas instrucciones del archivo de horario sin ejecutar son el guard defensivo de `segmentEnd <= cursor` (la hora UTC crece estrictamente y `ZoneBounds` produce un límite futuro o cero) y la verificación de día omitido después de validar exactamente siete weekdays únicos del conjunto 1..7. No se fabricó zona horaria ni fixture inválido para forzar esos casos.
- Esta medición es cobertura de tests Go en proceso; no reemplaza el test Playwright/BFF para UI ni demuestra DB/CAS, ownership o persistencia. No se levantaron servicios ni se tocaron controles.

## Reejecución centrada en CAS, rango y scope de bloqueos — 2026-11-12

- Con standalone Next local y BFF fixtures, `weekly-schedule-editor.spec.ts`, `availability-block-calendar.spec.ts` y `availability-block-scope.spec.ts`: **3/3 passed**. Confirma el escenario CAS 412/reload, rangos y cambio horario de bloqueo, y aislamiento del refresh pendiente al navegar entre espacios con la misma zona. No llama a API Go ni a base.
- `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs`: **16/16**. Puertos 4292/18131 libres al cierre.
- El código Go de períodos adyacentes se valida separadamente con la regresión/package coverage y paquetes Go descritos arriba; esta reejecución browser no se presenta como integración backend. Sin API/worker, DB/GCS/Cloud SQL ni cambios de seguridad; el gate integrado sigue detenido.

## Tramos adyacentes sobre cambio de horario IANA — 2026-11-12

- Se añadió la caracterización `TestWeeklyHoursAdjacentPeriodsAcrossDSTTransition`: horario dominical sintético, ingresado fuera de orden, con `01:00–02:00` y `02:00–04:00` en `America/New_York`; el intervalo UTC `06:30–07:30Z` verifica pared local `01:30 EST–03:30 EDT` y se acepta a través de los dos segmentos de offset.
- El test focal pasa como caracterización del comportamiento corregido. Los paquetes `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`, `go vet` y `gofmt -d` también pasan. Solo se modificó cobertura de test en esta vuelta; no hay cambio de producción nuevo.
- No API/worker, DB/GCS/Cloud SQL ni controles de seguridad. Prueba lógica Go con zona IANA embebida; no es CAS SQL ni persistencia integrada.

## Auditoría de alcance restante: CAS, rango, refresh y reload — 2026-11-12

- Fuente versionada `V000019__m06_local_weekly_hours.sql` limita weekdays a 1..7, tramos a órdenes 1..2, minutos/rangos y excluye solapamientos; `readWeeklyDays` construye los siete días canónicos y las rutas de guardado usan `ValidateWeeklyHours`. Por eso no añadí fixtures de horarios malformados como sustituto de datos persistibles.
- HTTP de bloqueos valida query incompleta/duplicada/inesperada y rangos RFC3339 inválidos, offset no permitido, fecha imposible, límites iguales o invertidos antes de tocar el repositorio (`handler_test.go`). UI mock cubre rango UTC, cambio DST, conflicto, refresh explícito después de fallo, readback/reload y respuesta pendiente al cambiar de espacio.
- El contrato no define `If-Match` para POST/DELETE de bloqueos; el CAS semanal y 412/reload sí tienen cobertura UI/HTTP fake. No se añadió un precondition inventado. Cobertura local no mutante de orden 2 revisada; persistencia, ownership/CAS SQL y autorización integrada siguen pendientes del target/harness aislado autorizado.
- Este inventario se deriva de migraciones/código/tests versionados; no es consulta ni evidencia de estado runtime de ninguna base. Sin API/worker ni cambio de controles.

## Revalidación focal ampliada del orden 2 — 2026-11-12

- Corrida Chromium local con BFF fixtures de los seis specs (`weekly-schedule-editor`, `availability-block-calendar`, `availability-block-scope`, `nonhourly-listing-schedule`, `nonhourly-wizard-schedule`, `wizard-improvements`) y filtro de IANA/CAS/rango/refresh/reload: **8/8 passed**. Incluye tarifas día/mes y recuperación del borrador sin horario semanal.
- Node fecha/zona/dominio: **16/16 passed**. Standalone local en 127.0.0.1:4292; API mock apuntaba a 127.0.0.1:18131 sin listener; ambos puertos libres al finalizar.
- Es verificación mock/frontend independiente de los cuatro paquetes Go offline, que ya pasaron con la prueba IANA/DST de tramos adyacentes. No se inició API/worker, no se accedió a DB/GCS/Cloud SQL y no se alteraron controles. El gate integrado permanece detenido.

## Alta de bloqueo: decoder HTTP estricto y query allowlist — 2026-11-12

- `TestManualBlockCreateHTTPRejectsUnexpectedBodyAndQueryBeforeRepositoryWrite` caracteriza tres rechazos del POST real de handler: campo no contractual `owner_id`, dos valores JSON concatenados y query parameter no permitido. Cada caso exige 400 `invalid_request` y cero llamadas al repositorio fake.
- El test focal pasa como caracterización de controles existentes (`DisallowUnknownFields`, cuerpo único y ausencia de query en POST); no hubo cambio de producción ni de controles.
- Revalidación final: el test focal y `go test` + `go vet` offline de `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http` pasaron; `gofmt -d` y `git diff --check HEAD` limpios. El alcance es handler/servicio con repositorio en memoria; no demuestra CAS SQL, autorización integrada ni persistencia/readback. Sin API/worker ni acceso a base/storage.

## Alta de bloqueo: flujo HTTP válido de extremo a servicio — 2026-11-12

- `TestManualBlockCreateHTTPForwardsValidatedUTCBlockAndReturnsRepositoryBlock` recorre el handler y `occupancy.Service` reales hasta un repositorio capturador in-memory. Verifica HTTP 201, owner/space/ID enviados, timestamps UTC con nanosegundos preservados, motivo recortado y DTO de respuesta igual al bloque devuelto por el repositorio.
- Se combinó con la caracterización de campos/query no permitidos y rechazos de rango/motivo. Revalidación final de los cuatro paquetes Go de orden 2 y `go vet` offline PASS; `gofmt -d` y diff-check se verifican al cerrar este corte. Sin cambios de producción.
- El repositorio es fake; la respuesta refleja su objeto retornado y no constituye lectura posterior ni persistencia. No demuestra CAS SQL, autorización efectiva ni ownership integrado. No API/worker ni acceso a DB/storage.

## Revalidación focal por continuidad de orden 2 — 2026-11-12

- En Chromium/standalone local, seis specs focales de horario/zona, calendario de bloqueos, aislamiento ante respuesta pendiente al cambiar de espacio, tarifas día/mes y recuperación del wizard: **9/9 passed**. Node fecha/zona/dominio: **16/16 passed**.
- Go offline `go test` y `go vet` de `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`: PASS; `gofmt -d` y `git diff --check HEAD`: PASS. El standalone usó `127.0.0.1:4292`; el BFF fixture apuntó a `127.0.0.1:18131` sin listener; ambos puertos quedaron libres tras la corrida.
- No hubo API/worker, DB/storage ni solicitudes a sistemas externos. Las pruebas UI usaron BFF fixtures y las pruebas Go repositorios in-memory; no acreditan CAS SQL, ownership efectivo ni readback persistido. Gate integrado sigue detenido hasta contar con target aislado y operaciones autorizadas.

## Cobertura semanal de la hora repetida de otoño — 2026-11-12

- Se añadieron `TestWeeklyHoursIntervalAcrossFallBackTransition` y `TestWeeklyHoursFallBackDoesNotCollapseRepeatedWallClockSpan` para `America/New_York`, domingo 2026-11-01. Cubren un intervalo válido `01:30 EDT → 01:30 EST` dentro de `01:00–02:00` y el rechazo de `01:30 EDT → 01:45 EST` cuando el horario abre solo `01:30–01:45`, porque los segmentos a ambos lados del retroceso contienen tiempo local fuera del horario.
- Ambos tests focales pasan. Un overlay Go controlado que omitió segmentar por `ZoneBounds` hizo fallar el caso negativo en su aserción prevista; el source versionado permaneció byte-igual. Separadamente, `go test`/`go vet` offline de `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`, `gofmt -d` y diff-check PASS.
- Solo aumentó cobertura Go local; sin API/worker, DB/storage ni cambios de producción/seguridad. No es prueba de CAS SQL, ownership integrado o persistencia. El gate integrado continúa detenido.

## Rechazo visible de horas DST inválidas al crear bloqueo — 2026-11-12

- El E2E local intenta crear bloqueos en `America/Santiago` para `2026-04-04 23:30` (hora repetida por el retroceso DST) y `2026-09-06 00:30` (hora inexistente por el salto DST). Antes del segundo caso genera y reemplaza una alerta distinta de rango inválido para que la aserción no pueda pasar por el mensaje obsoleto. Ambos deben mostrar la alerta accesible de transición y no aumentar POST intentados/aceptados, preconditions, GET de rango ni escrituras de horario.
- Chromium/standalone con BFF fixture: `availability-block-calendar.spec.ts` **1/1**; Node IANA/DST **9/9**; Prettier focal, ESLint focal y `tsc --noEmit` PASS. Go offline `go test` y `go vet` de los cuatro paquetes de orden 2, `gofmt -d` y `git diff --check HEAD` PASS.
- Solo cambia cobertura frontend; no se modificó producción ni controles, no se inició API/worker y no se accedió a DB/storage. No demuestra CAS SQL, ownership efectivo ni readback persistido; el gate integrado del orden 2 sigue detenido por falta de target/harness aislado autorizado.

## Revisión de cobertura Go actual — 2026-11-12

- `go test -coverprofile` offline revalidó `internal/booking`: `IntervalFitsWeeklyHours` **98.1%**, `ValidateWeeklyHours` **95.7%**, `parseWeeklyClock` y `normalizeWeeklyDays` **100%**; `internal/occupancy` **100%** total, incluyendo zona, rango, lectura y mutaciones fake-backed. El **30.1%** global de `internal/booking` incluye reserva/pagos/administración y no es métrica del orden 2.
- Los únicos bloques sin cubrir en `weekly_hours.go` son weekday ausente en `ValidateWeeklyHours` (imposible con siete días, IDs únicos 1..7 y rechazo previo de duplicado/fuera de rango) y guard de segmento sin avance (defensa contra `ZoneBounds` no progresivo). No se fabricaron entradas fuera del contrato para perseguir cobertura.
- El test/vet offline de los cuatro paquetes de orden 2 volvió a pasar. Esta medición no ejecutó DB/API/worker; no prueba CAS SQL, autorización/ownership efectivo ni persistencia. El gate integrado permanece bloqueado por falta del target/harness autorizado.

## Lectura source-only de CAS semanal y rango de bloqueos — 2026-11-12

- En `owner_weekly_hours.go`, la ruta de guardado bloquea la cuenta activa y la fila del espacio (`FOR UPDATE`), compara `actualizado_en` con la revisión esperada antes de tocar el horario, y actualiza horario/revisión dentro de una transacción. El fake HTTP/servicio no prueba que esta secuencia serialize correctamente en PostgreSQL.
- En `occupancy/repository.go`, el listado requiere propietario, espacio en borrador y bloqueo manual activo; aplica intersección de rango half-open `[)` y ordena por inicio/ID. La creación usa `tstzrange(...,'[)')` y mapea conflicto de exclusión; la eliminación es soft-delete condicionada al mismo propietario/espacio/estado/tipo. No aparece precondición de revisión en POST/DELETE, en línea con el contrato vigente.
- Es revisión estática de fuente, no ejecución SQL ni validación de grants/ownership runtime. Sin DB/API/worker y sin cambios de código, migración o controles; los gates de CAS SQL, propiedad efectiva y readback siguen pendientes del target aislado autorizado.

## Revalidación no mutante de orden 2 — 2026-11-13

- En standalone Next local con BFF fixtures, `weekly-schedule-editor.spec.ts`, `availability-block-calendar.spec.ts` y `availability-block-scope.spec.ts` pasaron **3/3** con `--grep @controlled`. Se cubrieron CAS 412/reload, filtros y refresh de rangos, cambio de zona horaria y aislamiento de lectura pendiente entre espacios; no se llamó a API Go ni a base.
- `pnpm test`: **507 tests, 506 aprobados, 0 fallidos, 1 opt-in omitida**. Incluye Node IANA y la transición de media hora de `Australia/Lord_Howe` (salto y repetición). Go offline con `GOPROXY=off GOTOOLCHAIN=local`: cobertura `internal/booking` **30.1% global** (no representativa del orden 2), `IntervalFitsWeeklyHours` **98.1%**, `ValidateWeeklyHours` **95.7%**, `parseWeeklyClock`/`normalizeWeeklyDays` **100%**; `internal/occupancy` **100%**. Pasaron además tests de `internal/spaces/transport/http` e `internal/booking/transport/http`, `go vet` de los cuatro paquetes, `gofmt -d` y `git diff --check HEAD`.
- La cobertura Go de Lord Howe verifica en proceso horarios adyacentes a través del salto de 30 minutos, hueco no cubierto y hora repetida; el test Node valida conversión de pared a UTC y rechazo de horas ambigua/inexistente. Los tests son locales y sintéticos, no prueban zonas configuradas en runtime ni persistencia.
- Alcance y límites: no se inició API/worker; no se accedió a base persistente, GCS, Cloud SQL ni servicios externos; no se alteraron controles ni migraciones. CAS SQL, autorización/ownership efectivos y readback persistido siguen sin comprobarse. El gate integrado queda detenido por falta de autorización para ejecutar migraciones/harness sobre el target aislado.

## Contrato HTTP de errores para calendario y bloqueos — 2026-11-13

- Se añadió `TestCalendarHTTPMapsAvailabilityErrorsWithoutChangingContract`, caracterización test-only a través del handler de errores de calendario. Fixtures falsos verifican 409 `time_zone_required`, 409 `occupancy_conflict` al solaparse un bloqueo, 404 `not_found` al borrar un ID ausente, 409 `time_zone_in_use` y `time_zone_locked` al intentar cambiar la zona, y 500 `internal_error` sanitizado ante un error interno.
- Test focal y suites offline de `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`, `go vet`, `gofmt -d` y `git diff --check HEAD` PASS. La cobertura del handler HTTP de espacios pasó de **78.7% a 80.0%**; `calendarError` **100%**, `queryWindow` **100%** y `serveCalendar` **85.7%**. La cobertura global de ese handler no es gate de producto ni de integración.
- No se modificó código de producción, contrato, migraciones o controles. Solo handler y servicios con fakes; no se inició API/worker ni se accedió a base persistente, DB aislada, storage o servicios externos. No demuestra autorización efectiva, CAS SQL ni persistencia/readback; el gate integrado sigue detenido.

## Errores de escritura CAS del horario semanal — 2026-11-13

- Se añadió `TestOwnerWeeklyHoursMapsSaveFailuresWithoutRenewingRevision`, caracterización del handler con servicio fake: una agenda inválida mapea a 422 `invalid_request`, un espacio no encontrado a 404 `not_found` y un error interno a 500 `internal_error` sanitizado. En los tres casos exige una sola llamada al servicio y ausencia de ETag renovado.
- La prueba focal pasó. Las suites offline de los cuatro paquetes de orden 2 y `go vet` pasaron; `internal/booking/transport/http` reportó 64.7% global, `ServeHTTP` de horario semanal 76.8%, `replyError` 80.0% y `decodeOwnerWeeklyHours`/`ownerWeeklyHoursSpaceID` 100%. La cobertura global del paquete no es gate de la fase.
- Revalidación local conjunta: Playwright controlado de horario, calendario y scope **3/3**; Node IANA/dominio **17/17**; `gofmt -d` y `git diff --check HEAD` limpios. Solo mocks/fakes; sin API/worker, bases, storage o cambios de producción/controles. No prueba CAS SQL, autorización efectiva ni readback persistido.

## Snapshot de fuente V1–V61 con pin de inventario — 2026-11-13

- Se añadió `cmd/dbmigrate/stageSpacesOrder2Snapshot`, helper filesystem-only que valida las 84 migraciones contiguas antes de crear el snapshot; verifica el inventario completo con el pin revisado `4b4757ae2975e8a72c817ecc02fbcb89b38b6d6d8c8bc0688c4d6bed7ba87907`, cuyo formato exacto es una línea ordenada `version filename sha256(contents)\n`. Después copia bytes originales de V1–V61 a un directorio `0700`, con archivos `0600`, re-descubre el inventario y devuelve limpieza explícita.
- La discrepancia con `3921ff…` quedó adjudicada: ese digest corresponde al formato distinto de `internal/migrator.inventoryFingerprint` (`checksum + dos espacios + filename + LF`), no al pin de orden 2. El cálculo independiente directo de los bytes fuente con el formato revisado reprodujo `4b4757…` para V1–V84; no se cambió ninguna migración.
- `go test ./cmd/dbmigrate -run '^TestStageSpacesOrder2Snapshot' -count=1`, `go test -race -count=1 ./cmd/dbmigrate`, `go vet ./cmd/dbmigrate`, `gofmt` y `git diff --check HEAD` — PASS. Los tests verifican bytes, cantidad, permisos, limpieza y rechazo ante deriva del inventario.
- El helper aún no está conectado a la CLI y no ejecuta SQL. Esta evidencia solo prueba staging local y fail-closed con fuentes; no cierra el gate integrado, no se conectó a PostgreSQL, no se aplicó DDL/migración ni se iniciaron API/workers. La autorización de orden 2 conserva su target y alcance exactos.

## Reejecución no mutante de orden 2 — 2026-11-13

- Chromium local con BFF fixtures: `weekly-schedule-editor.spec.ts`, `availability-block-calendar.spec.ts` y `availability-block-scope.spec.ts` **3/3 passed**. El recorrido cubre el guardado de zona IANA, horarios fraccionarios, CAS 412 sin replay, validación/refresh de rangos, bloqueos manuales, reload/readback mock y aislamiento de una lectura tardía al cambiar entre espacios. Se usó standalone local en `127.0.0.1:4294`; el backend fixture apuntó a `127.0.0.1:18082` sin listener.
- `node --test tooling/local-date-time.test.mjs tooling/host-listings-domain.test.mjs`: **17/17 passed**, incluidos horarios DST inexistentes/ambiguos, offsets no horarios y validación IANA.
- Con `env -i`, `GOPROXY=off` y `GOTOOLCHAIN=local`, `go test -count=1`, `go test -race -count=1` y `go vet` pasaron para `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`; `gofmt -l` no reportó archivos.
- `127.0.0.1:4294` y `127.0.0.1:18082` quedaron libres al terminar. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL; no se modificaron controles ni se hizo cambio de código en esta reejecución. La cobertura permanece mock/fake y no acredita CAS SQL, autorización efectiva o persistencia/readback; el gate integrado del orden 2 sigue parcial.

## Revalidación tras continuar el análisis de orden 2 — 2026-11-13

- Se repitió con entorno limpio el foco Chromium/BFF mock `weekly-schedule-editor.spec.ts`, `availability-block-calendar.spec.ts` y `availability-block-scope.spec.ts`: **3/3**. Node `local-date-time.test.mjs` + `host-listings-domain.test.mjs`: **17/17**. Go `-race` y `go vet` pasaron para `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`; `gofmt -l` no reportó archivos.
- Se amplió la verificación no mutante al comparador puro de horario: `go test -race -count=1 -run '^TestWeeklyScheduleEquivalent' ./internal/adapters/postgres/booking` y `go vet ./internal/adapters/postgres/booking` pasaron. Solo corrieron esos tests de equivalencia; quedaron fuera los tests PostgreSQL opt-in del paquete.
- Los puertos `4294` y `18082` quedaron libres. No se inició API/worker ni se conectó a PostgreSQL, GCS o Cloud SQL; no se alteraron controles. No surgió una brecha de fuente/cobertura concreta que justifique otra modificación de producto o inventar CAS para POST/DELETE de bloqueos. La cobertura continúa mock/fake/source-only; CAS SQL, autorización efectiva y readback persistido permanecen pendientes del gate integrado.

## Probe local de aceptación IANA Node→Go — 2026-11-13

- Para buscar divergencias entre el validador de interfaz `Intl.DateTimeFormat` y `internal/occupancy.validTimeZone`, se generó temporalmente en scratch la lista aceptada por Node `v26.12.1`: **430 identificadores**, compuesta por 418 valores de `Intl.supportedValuesOf("timeZone")` y 12 alias adicionales aceptados por ese runtime.
- Se ejecutó un test Go temporal mediante `-overlay` en scratch, contra la función `validTimeZone` real y sin editar el worktree: **430/430 aceptados** por Go `go1.27.1`. El probe solo compara el conjunto Node→Go de esos runtimes y no establece paridad en sentido inverso ni compatibilidad con tzdata de despliegue.
- `go test` pasó; los archivos de sonda/lista/overlay quedaron fuera del repositorio. No se inició API/worker ni se conectó a PostgreSQL, GCS o Cloud SQL; no se alteró código, migraciones ni controles. Sin discrepancia concreta que justifique una modificación productiva en esta rebanada.

## Desfase inverso `Factory` en validación IANA — 2026-11-13

- Se completó el sentido inverso del probe: el test temporal vía overlay enumeró **598** entradas de `/usr/local/go/lib/time/zoneinfo.zip`; la función Go `validTimeZone` aceptó las 598. Al probarlas en Node `v26.12.1`, `Intl.DateTimeFormat` rechazó solo `Factory`. El probe anterior Node→Go se mantiene: **430/430** IDs reconocidos por Node fueron aceptados por Go.
- El desfase es funcional: el servicio aceptaba guardar `Factory`, pero la UI trata esa zona como no configurada y bloquea el uso del horario/bloqueos. IANA define `Factory` como una zona válida para instalaciones cuyo huso real aún se desconoce; por eso no representa una localidad concreta apta para el calendario de un espacio. La exclusión alinea el requisito de dominio con la UI, no redefine el conjunto de identificadores de tzdb.
- **TDD RED:** al añadir `Factory` a las entradas rechazadas, fallaron `TestValidTimeZoneUsesIANAZoneDatabase` y los casos de `SetTimeZoneCAS`/`SetTimeZone` (Go devolvía `nil` en vez de `ErrInvalid`). La prueba de dominio frontend quedó como caracterización explícita de que `Intl` rechaza `Factory`.
- **GREEN:** `internal/occupancy.validTimeZone` ahora rechaza `Factory` junto a `Local` y traversal. La guardia solo corrige validación de dato de zona; no modifica autenticación, autorización, grants, roles ni CAS. Los casos de helper y ambos servicios pasan.
- Verificación final: Playwright order 2 **7/7** (horario/CAS, bloqueos/rangos/refresh/reload, scope y editor/wizard día-mes); Node IANA/DST/dominio **17/17**; Go `-race`/`go vet` en booking, occupancy, spaces HTTP y booking HTTP; comparador puro `TestWeeklyScheduleEquivalent*` con `-race`/`vet`; `gofmt -l` limpio; ESLint y Prettier focal PASS; `git diff --check HEAD` PASS tras actualizar esta evidencia.
- Sin API/worker, conexión a PostgreSQL, GCS/Cloud SQL ni cambios de migración/controles de seguridad. Evidencia íntegramente local; no demuestra CAS SQL, autorización efectiva o persistencia/readback.

## Medición de cobertura Go para calendario — 2026-11-13

- `go test -count=1 -coverprofile=... ./internal/occupancy` informó **100.0%** de sentencias; `NewService`, configuración/lectura de zona, CAS, disponibilidad, listado/creación/baja de bloqueos, parseo de rangos/instantes, `validTimeZone` y UUID quedaron al 100% en esta suite unitaria con stubs.
- `internal/spaces/transport/http` informó **80.0%** de cobertura global, porque el paquete comparte archivo con endpoints de espacios/tarifas ajenos al orden 2. En calendario, `queryWindow` y `calendarError` quedaron en **100%**; `serveCalendar`, **85.7%**. La revisión HTML atribuye los bloques no cubiertos a fallbacks de dependencia/ruta/método, el guard de query en PUT y la revalidación RFC3339 defensiva tras `parseIfMatch`; las rutas ordinarias de zona, CAS, disponibilidad, bloques, rangos y errores de calendario sí se ejecutaron. No se añadieron casos solo para elevar el porcentaje.
- Los perfiles quedaron fuera del repositorio, bajo scratch. Estas cifras miden ejecución de servicios/handlers con fakes y fixtures; no prueban locks, SQL, ownership real, persistencia ni readback de PostgreSQL. No se conectó a ninguna base ni se inició API/worker.

## Revalidación de paridad IANA tras excluir `Factory` — 2026-11-13

- Se repitió el probe Node→Go contra el validador actual, usando el corpus local de **430** identificadores reconocidos por Node **v26.12.1**; `validTimeZone` en Go **1.27.1** aceptó **430/430**.
- En el sentido inverso, se probaron los **598** nombres de `GOROOT/lib/time/zoneinfo.zip`: el validador Go aceptó **597** y `Intl.DateTimeFormat` aceptó exactamente los mismos **597**; ambos rechazaron únicamente `Factory`. El runtime Node reporta **ICU 78.3 / tz 2026e**. Comparación de conjuntos programática: `go_only=[]`, `node_only=[]`, `rejected_by_both=[Factory]`.
- La coincidencia se limita a los corpus y runtimes observados (el conjunto Node→Go son los 418 identificadores canónicos más 12 alias del probe previo); no demuestra compatibilidad del tzdata de producción ni equivalencia universal entre motores.
- Revalidación focal del orden 2: Playwright Chromium/standalone con BFF mocks **7/7**; Node `local-date-time` + dominio **17/17**; Go `-race -count=1` y `go vet` en `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http` — PASS. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL; no se modificó código ni controles de seguridad. Persisten los límites de evidencia fake/mock/source-only y el gate integrado separado.

## Caracterización de query no admitido en PUT de zona — 2026-11-13

- Se añadió un caso a `TestTimezoneCASPreconditionsAndRevision`: un `PUT /api/v1/spaces/{id}/availability?page=1` con body y `If-Match` válidos devuelve `400 invalid_request`; el fake confirma cero llamadas CAS y que zona/revisión sigan intactas. Es una caracterización de la guardia ya existente; no se cambió código productivo ni el control.
- `GOPROXY=off GOTOOLCHAIN=local go test -race -count=1 ./internal/booking ./internal/occupancy ./internal/spaces/transport/http ./internal/booking/transport/http` y `go vet` de los cuatro paquetes pasaron.
- Cobertura con perfil local: `internal/spaces/transport/http` **80.4%** global; `serveCalendar` **87.9%**, `queryWindow` y `calendarError` **100%**. El porcentaje restante incluye rutas generales de espacios/tarifas y fallbacks fuera de este caso; no se añadieron pruebas solo para elevarlo.
- No se inició API/worker ni se accedió a base, GCS o Cloud SQL. La prueba usa HTTP `httptest`, fake calendar y repositorio en memoria; no demuestra CAS SQL ni readback persistido.

## Horario semanal: weekdays inválidos antes de CAS — 2026-11-13

- Se amplió `TestOwnerWeeklyHoursServiceRejectsInvalidCASBeforeRepositoryWrite` con fixtures fake para un weekday duplicado y otro fuera del rango ISO (`8`). Ambos casos requieren `ErrInvalid` y cero llamadas de escritura al repositorio; no se modificó el validador ni el control CAS.
- El test focal con `-race` pasó. También pasaron `go test -race -count=1` y `go vet` offline en `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`.
- Cobertura solo de la validación del servicio con fake; no acredita CAS SQL ni persistencia. Sin conexión a PostgreSQL/GCS/Cloud SQL ni arranque de API/workers.

## Reejecución final local/mock de orden 2 — 2026-11-13

- Chromium con BFF fixtures/mocks: **7/7** en los cinco specs controlados (`weekly-schedule-editor`, `availability-block-calendar`, `availability-block-scope`, `nonhourly-listing-schedule`, `nonhourly-wizard-schedule`). Incluye rechazo CAS 412 sin replay, bloqueos manuales, rangos, refresco/reload y aislamiento al cambiar de espacio.
- Node `local-date-time` + dominio anfitrión: **17/17**. Go `-race` y `go vet` en `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`: PASS.
- El servidor Next standalone fue iniciado por Playwright en `127.0.0.1:4294` y se cerró al terminar; comprobación posterior confirmó `4294`, `4203` y `18082` sin listeners. No se inició API/worker ni hubo conexión a base, GCS o Cloud SQL.
- Límites: evidencia local/mock/fake; no prueba CAS SQL, readback persistido, ni tzdata del despliegue. El gate integrado permanece separado.

## Alcance owner/espacio en lectura HTTP de bloqueos — 2026-11-13

- `TestCalendarBlockListForwardsAuthenticatedOwnerAndSpaceScope` ejecuta el handler real con dos principales sintéticos (`ownerA` y `ownerB`) y confirma que el fake del repositorio recibe el owner autenticado y el ID de espacio de la ruta en ambos GET válidos.
- El test focal `-race`, la suite `-race` de los cuatro paquetes de orden 2 y `go vet` pasaron. No hubo cambios productivos ni en autorización.
- Límite: prueba que el handler/servicio propagan el scope; el fake no prueba que una consulta SQL filtre correctamente ni que haya aislamiento persistido. No se accedió a PostgreSQL/GCS/Cloud SQL ni se iniciaron API/workers.

## Horario semanal usa weekday local IANA — 2026-11-13

- Se añadió `TestIntervalFitsWeeklyHoursUsesLocalWeekdayForIANAZone` en `internal/booking/weekly_hours_test.go`. El rango sintético `2030-01-03T00:00Z`–`00:30Z` corresponde a miércoles 21:00–21:30 en `America/Santiago`; con horario abierto miércoles 21:00–22:00 debe aceptarse. El mismo rango debe rechazarse con `UTC`, cuya fecha cae en jueves. La prueba verifica explícitamente el desplazamiento horario y contrasta el resultado bajo ambas zonas.
- Pasaron `env -i ... go test -race -count=1 ./internal/occupancy ./internal/booking ./internal/spaces/transport/http ./internal/booking/transport/http`, `go vet` en los mismos cuatro paquetes, `gofmt` y `git diff --check HEAD`. No se modificó código productivo.
- Evidencia unitaria local con zona tzdb del runtime de prueba; no acredita tzdata del despliegue, CAS SQL ni persistencia. No se inició API/worker y no hubo conexión a base, GCS o Cloud SQL; no se alteraron controles de seguridad. El gate integrado sigue separado.

## Revalidación local/mock de orden 2 tras el caso IANA — 2026-11-13

- Chromium con Next standalone local y BFF fixtures/mocks: **7/7** en `weekly-schedule-editor`, `availability-block-calendar`, `availability-block-scope`, `nonhourly-listing-schedule` y `nonhourly-wizard-schedule`. Incluye CAS 412 sin replay, manejo de zona/horario, rango, refresh/reload y scope de bloqueos.
- Node `local-date-time.test.mjs` + `host-listings-domain.test.mjs`: **17/17**. En la misma versión de código, `go test -race -count=1` y `go vet` pasaron en `internal/occupancy`, `internal/booking`, `internal/spaces/transport/http` e `internal/booking/transport/http`; `gofmt` y `git diff --check HEAD` pasaron.
- Playwright levantó el standalone en `127.0.0.1:4294` y lo cerró al finalizar; la comprobación inmediata no encontró listeners en `4294`, `4203` ni `18082`. Una comprobación posterior encontró `4203` ocupado por un `next-server` ubicado en el otro worktree (`/home/nandev/Documentos/projects/espaciGo-frontend-next-hermes/frontend/.next/standalone`); se dejó intacto. `4294` y `18082` seguían libres. Sin API/worker, conexión a DB/almacenamiento ni modificación de controles. La evidencia de UI usa BFF mocks; no acredita CAS SQL ni readback persistido.

## Encabezados If-Match duplicados en horario semanal — 2026-11-13

- Se añadió un caso a `TestOwnerWeeklyHoursPUTChecksIfMatchBeforeBodyAndMapsCAS` que agrega dos valores de encabezado `If-Match` idénticos y envía body válido. El handler responde `400 invalid_request`, el fake registra cero guardados y no se publica ETag nuevo. Es una caracterización del rechazo fail-closed de revisiones múltiples a nivel HTTP; no modifica el control productivo.
- El test focal con `-race`, `go test -race -count=1` y `go vet` en los cuatro paquetes de orden 2 pasaron. `gofmt` y `git diff --check HEAD` pasaron.
- Solo handler/servicio con fake y solicitud `httptest`; no demuestra CAS PostgreSQL ni persistencia. No se conectó a base ni se inició API/worker; sin cambios de controles de seguridad.

## CORS rechaza origen no listado en horario semanal — 2026-11-13

- `TestOwnerWeeklyHoursCORSRejectsUnlistedOriginBeforeReadingSchedule` envía un GET con origen fuera del allowlist y verifica `403 origin_denied`, cero lecturas del servicio y ausencia de `Access-Control-Allow-Origin`. El allowlist y el handler productivo no cambiaron.
- El test focal con `-race`, suite Go `-race`/`go vet` en los cuatro paquetes de orden 2, `gofmt` y `git diff --check HEAD` pasaron. Cobertura diagnóstica después del caso: `ownerWeeklyHoursHandler.ServeHTTP` **79.7%**, paquete `internal/booking/transport/http` **64.9%**; el paquete mezcla handlers de otros órdenes, por lo que el total no es criterio de aceptación.
- La revisión de bloques no cubiertos dejó fuera fallos de generador/configuración/autenticación y parseo defensivo que el parser estricto de ETag ya valida; no se añadieron pruebas para elevar porcentajes. Todo es local con `httptest` y fake. Sin API/worker, DB/almacenamiento ni cambios de seguridad.

## Revisión no mutante de CAS ETag y revalidación del orden 2 — 2026-11-13

- La cobertura focal previa de `parseIfMatch` era **90.0%**. Se añadieron cuatro casos source-level a `TestParseIfMatchPreconditionErrors`: payload sin separador ID/timestamp, timestamp inválido, zona offset no UTC y representación fraccionaria no canónica. Todos exigen `ErrInvalidPrecondition`; no se cambió el parser ni el contrato. `parseIfMatch` quedó en **100.0%**; tras las nuevas pruebas de ruta, el paquete `internal/spaces/transport/http` midió **82.8%** global, solo como diagnóstico y no como criterio de aceptación.
- `TestCalendarRejectsUnsupportedMethodAndResourceRoutesWithoutRepositoryCalls` cubre cinco métodos/rutas de calendario no definidos: 404 y cero llamadas fake a zona, horario, disponibilidad, lista, alta y baja. `TestCalendarRouteFailsClosedWithoutServiceOrOnMalformedPath` cubre el servicio ausente y prefijo/segmento malformados con 404 y cero llamadas. Son caracterizaciones del allowlist/fail-closed existente; no alteran rutas ni controles.
- Revisión de alcance: `serveCalendar` pasó de **87.9%** a **97.8%**; `queryWindow` y `calendarError` permanecen en **100%**; el paquete `internal/spaces/transport/http` midió **82.8%**. La única rama no cubierta en `serveCalendar` intenta parsear un timestamp que `parseIfMatch` ya valida y normaliza antes de la llamada; no es alcanzable mediante una petición válida y no se forzó. `ValidateWeeklyHours` **95.7%**, `IntervalFitsWeeklyHours` **98.1%** e `internal/occupancy` **100%**. No se persiguieron ramas defensivas ajenas al comportamiento observable ni se añadió cobertura solo por porcentaje. CAS se atribuye a los endpoints de zona/horario que definen `If-Match`; no se inventó precondición para POST/DELETE de bloqueos manuales.
- Sobre el árbol actual pasaron `go test -race -count=1` y `go vet` en `internal/booking`, `internal/occupancy`, `internal/spaces/transport/http` e `internal/booking/transport/http`; Node local IANA/dominio **17/17**; Playwright Chromium controlado **7/7** en cinco specs de horario, CAS, bloqueos, rango, refresh/reload y scope. También pasaron `gofmt` y `git diff --check HEAD`.
- Playwright usó Next standalone y BFF mocks/fixtures en loopback (`4294`; API `18082` sin listener); ambos puertos quedaron libres al terminar. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL; no se alteraron controles. Esto no prueba CAS SQL, readback persistido ni tzdata de despliegue; el gate integrado continúa separado.

## Allowlist HTTP del horario semanal — caracterización no mutante — 2026-11-13

- `TestOwnerWeeklyHoursRejectsUnsupportedMethodBeforeService` caracteriza `PATCH /weekly-hours`: `405 method_not_allowed`, `Allow: GET, PUT, OPTIONS`, cero llamadas al servicio y sin ETag. `TestOwnerWeeklyHoursRejectsOversizedBodyBeforeService` envía un JSON de más de 16 KiB con If-Match sintético válido y verifica `400 invalid_request`, cero llamadas al servicio y sin ETag. Ambos prueban guards existentes; no cambian métodos, límites, controles ni autorización.
- Ambos tests focales y `go test -race -count=1`/`go vet` pasaron en booking, occupancy, spaces HTTP y booking HTTP. La cobertura diagnóstica actual de `ownerWeeklyHoursHandler.ServeHTTP` es **84.1%**; el paquete `internal/booking/transport/http` **65.4%**. La cobertura no es criterio de aceptación.
- Reejecutados en el árbol actual: Node IANA/DST/dominio **17/17** y Playwright Chromium controlado **7/7** para horario/CAS, bloqueos, scope, rango, refresh/reload y tarifas día/mes. Los puertos de ensayo `4294` y `18082` quedaron libres.
- Solo tests locales/fixtures; no prueba CAS SQL ni persistencia. Sin API/worker, acceso a DB/GCS/Cloud SQL o modificación de controles; el gate integrado continúa separado.

## Auditoría no mutante de cobertura de horario e intervalos — 2026-11-13

- Se midieron sentencias con `go test -coverprofile` y `go tool cover -func` en `internal/booking` y `internal/occupancy`; perfiles guardados solo en `$HOME/.hermes/cache/scratch/order2-coverage-final-20261113/`.
- `weekly_hours.go`: `ValidateWeeklyHours` **95.7%**, `parseWeeklyClock` **100%**, `IntervalFitsWeeklyHours` **98.1%**, `normalizeWeeklyDays` **100%**. `internal/occupancy` total **100%**; sus funciones de IANA, rango UTC, CAS y bloqueos medidos están al 100% de sentencias. El total del paquete booking es **30.1%** y no representa específicamente el flujo de orden 2.
- No se añadieron pruebas para fabricar las líneas no cubiertas: la rama `ValidateWeeklyHours` que busca un día faltante tras aceptar siete valores de weekday es redundante, porque el rango 1–7 y la unicidad se validan antes; la defensa de `IntervalFitsWeeklyHours` ante un segmento que no avanza no tiene un caso canónico reproducido. Se mantienen como límites de medición, sin cambiar producción ni afirmar cobertura de rama.
- La cobertura local caracteriza únicamente funciones Go con unit tests/fakes. No acredita consultas SQL, CAS transaccional, persistencia, adaptadores ni readback integrado; el gate integrado permanece separado.

## Orden 2 — límite de cuerpo en creación de bloque manual — 2026-11-13

- `TestManualBlockCreateHTTPRejectsUnexpectedBodyAndQueryBeforeRepositoryWrite` envía JSON válido con más de 64 KiB por espacios finales y verifica `400 invalid_request` más cero escrituras al repositorio fake. `TestManualBlockCreateHTTPRejectsNonJSONBeforeRepositoryWrite` envía el cuerpo con `Content-Type: text/plain` y verifica `415 unsupported_media_type` más cero escrituras. Ambos caracterizan límites/contrato existentes sin cambiar producción.
- Ambos tests focales, `go test -race -count=1` y `go vet` de los cuatro paquetes de orden 2 pasaron. Node IANA/DST/dominio **17/17** y Playwright controlado **7/7** pasaron en el árbol actual; puertos `4294` y `18082` quedaron libres. Cobertura de sentencias actual de `decode`: **100%**; `serveCalendar` **97.8%**, `queryWindow`/`calendarError` **100%** y paquete HTTP **83.2%** (diagnóstico).
- Esta prueba ejercita solo el handler y el fake de repositorio; no prueba validación de SQL, CAS de bloqueos ni persistencia/readback. No se accedió a DB, API o workers.

## Orden 2 — límite de la evidencia fake-backed para rango SQL — 2026-11-13

- Revisión source-only de `internal/adapters/postgres/occupancy/repository.go:150-175`: `ListBlocks` filtra espacio, propietario, borrador, tipo `bloqueo_manual`, activo y solapamiento `&& tstzrange($3,$4,'[)')`; ordena por inicio/ID y normaliza los timestamps devueltos a UTC. El código fuente no equivale a ejecutar esa consulta.
- `internal/adapters/postgres/occupancy/repository_integration_test.go:22-136` cubre propietario/espacio, bloqueos solapados y adyacentes, listado y baja lógica con PostgreSQL desechable. No se ejecutó: exige `TEST_DATABASE_URL` y crea/elimina una base. No hay dependencia `pgxmock`/`sqlmock` ni seam mock de repositorio SQL; no se añadió una aserción textual que simule ejecución.
- Las pruebas locales actuales validan rango y scope hasta el servicio/fake, además de UI con BFF fixtures; no verifican predicados SQL, concurrencia transaccional ni readback durable. Ese gate sigue pendiente de target aislado específicamente autorizado; sin conexión ni operaciones de DB en esta continuación.
