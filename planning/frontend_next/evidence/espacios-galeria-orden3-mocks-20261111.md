# Orden 3 — galería: cobertura de UI con mocks

- Fecha: 2026-11-11.
- Worktree: `espaciGo-spaces-wizard-recovery`.
- Rama: `codex/frontend/fix/space-01-create-postcommit-recovery`.
- HEAD de partida: `647cd491985d619370990d3f226b5df61e36b42b`.
- Alcance de esta rebanada: verificar el orden DOM tras reordenar, la purga/revocación ante 403 de miniatura privada, el rechazo de fotos `ready` con variantes requeridas ausentes o `created_at` ausente/inválido, y la entrada drag-and-drop a la cola local mediante un archivo sintético.

## Revisión y cambio de prueba

`frontend/tests/phase7-host-listings.spec.ts` tenía un helper llamado `visiblePhotoIds` que devolvía `photos.slice(...)`, el estado interno de la fixture, en vez de leer las tarjetas visibles. Esto permitía que las aserciones de reordenamiento pasaran aunque el DOM no reflejara el orden del servicio. Ahora lee `data-photo-id` desde las tarjetas `data-testid="gallery-photo"` renderizadas. La UI expone esos atributos en `frontend/src/features/host-listings/ui/listing-gallery.tsx`.

La comparación con `planning/openapi.yaml:9993-10058` identificó otra brecha: `SpaceMediaPhoto` requiere `variants` con 1–2 elementos; solo `SyntheticSpaceGalleryPhoto` lleva `fixture_code: synthetic-png-v1`. El adaptador clasificaba como sintético cualquier registro no pendiente que omitiera `variants`. Ahora `mapPhotoToGalleryPhoto` reconoce sintéticos solo por ese marcador y rechaza con `ApiProtocolError` una foto no sintética/no pendiente sin variantes `display`/`thumbnail` utilizables. `frontend/tests/phase-gallery-contract-guard.spec.ts` modela un registro `ready` incompleto y verifica que la UI no muestra tarjeta, no solicita miniatura y mantiene la galería como no confirmada. El test dio RED con el artefacto anterior (una tarjeta inválida visible) y GREEN después del cambio y rebuild.

La misma lectura del contrato confirmó que `created_at` es obligatorio y de formato `date-time`. El adaptador devolvía una fecha local actual cuando faltaba el campo, ocultando una respuesta incompatible. Ahora exige un `created_at` válido para toda foto no pendiente en el subconjunto RFC3339 producido por Go `time.Time` (que no representa segundos intercalares `:60`) y rechaza valores ausentes o inválidos con `ApiProtocolError`; el DTO `pending` puede omitirlo. La prueba controlada incluye variantes válidas para aislar el defecto, y cubre tanto ausencia como fecha inválida. El caso de campo ausente dio RED contra el adaptador previo (tarjetas recibidas 1, esperadas 0); un segundo RED con `2026-02-30T12:00:00Z` mostró que `Date.parse` normaliza fechas imposibles y también dejó una tarjeta. El guard final valida calendario, hora, zona y parseo; tras rebuild, las pruebas contractuales pasan. Los mocks de cola/403/incertidumbre recibieron timestamps contractuales fijos para no depender del fallback local. El envelope `202` del caso de cola incluye `reused: false` y su foto usa el DTO `pending` exacto (`id`, `space_id`, `state`) sin `created_at`; la carga queda pendiente.

La prueba controlada de edición comprueba carga/lectura visible, portada, cola, reemplazo pendiente y reconciliación, reordenamiento y CAS obsoleto, borrado, límite 30/31 y ausencia de POST cuando se supera el cupo. `phase-gallery-queue.spec.ts` cubre el límite de dos cargas, cancelación queued sin POST y reintento explícito con misma clave/bytes. `phase-gallery-uncertain.spec.ts` cubre bloqueo de mutaciones mientras la galería no está confirmada y recuperación solo tras refresh autoritativo exitoso.

`frontend/tests/phase-gallery-forbidden-purge.spec.ts` agrega un recorrido controlado: carga dos tarjetas sintéticas con variantes contractuales; una miniatura devuelve una Blob URL y la otra retiene su respuesta hasta que el 403 simulado está armado. Al liberar el 403, comprueba el estado visible de denegación, desaparición del título/tarjetas privadas, revocación de la Blob URL previa y cero mutaciones. El handler de la segunda variante exige `GET` y la prueba registra exactamente una solicitud `GET` a esa ruta. Los mocks válidos de cola, incertidumbre y 403 incluyen ahora al menos una variante WebP de acuerdo con OpenAPI.

La brecha de entrada drag-and-drop indicada en el plan canónico se implementó en `frontend/src/features/host-listings/ui/gallery-upload-batch.tsx`. Selector y drop usan `enqueueFiles` y comparten validación JPG/PNG/WebP, límite 10 MiB, cupo 30 y creación de tareas de la cola existente. Una superficie `div` persistente maneja transferencias `Files`; no cancela `dragenter`, `dragover` ni `dragleave` de payloads ajenos. En `drop` cancela la acción predeterminada incluso para payload no-file para evitar navegación, pero solo encola transferencias `Files`; durante `dragover`/`drop` de archivos cancela la navegación y, con la cola deshabilitada, fija `dropEffect=none` sin encolar ni limpiar errores. El botón nativo conserva semántica/activación de teclado; cuando está deshabilitado `pointer-events-none` hace que el hit-test de la zona visible llegue a la superficie persistente. La cobertura Chromium verifica ese hit-test, despacha eventos sintéticos y confirma cancelación, ausencia de POST y feedback conservado; también cubre selector por teclado, arrastres no-file y geometría full-width.

## Verificación ejecutada

- Revalidación parental consolidada del candidato actual (2026-11-11): `api:check` — PASS, 200 operaciones; `test` — 481 tests, 480 passed, 0 failed, 1 opt-in skipped; `typecheck` — PASS; `lint` — 0 errores y 4 advertencias en los mismos archivos preexistentes; `format:check` — PASS; `build` — PASS con Next.js 16.4.3.
- Sobre el standalone construido por esa corrida, las cuatro suites focales (`phase-gallery-contract-guard`, `phase-gallery-queue`, `phase-gallery-uncertain`, `phase-gallery-forbidden-purge`) terminaron **12 passed** y `phase7-host-listings.spec.ts --grep 'Fotos con selección de imágenes'` terminó **1 passed**. `git diff --check` — PASS. El padre repitió las mismas suites el 2026-11-11 durante la corrida de cierre: galería 12/12 y recorrido seleccionado 1/1. Todo fue local y con BFF mocks; no hubo llamadas a API Go ni acceso a bases.
- `npx --yes pnpm@12.15.0 build` — PASS en el candidato final tras incorporar el guard de timestamp; Next.js 16.4.3 compiló y generó páginas; Playwright usó el standalone reconstruido en loopback.
- `phase-gallery-contract-guard.spec.ts`, `phase-gallery-queue.spec.ts`, `phase-gallery-uncertain.spec.ts` y `phase-gallery-forbidden-purge.spec.ts` — **12 passed**, 0 fallidos, Chromium local, tras el guard contractual de timestamp y el rebuild final.
- `phase7-host-listings.spec.ts --grep 'Fotos con selección de imágenes'` — **1 passed**; recorrido controlado de pestañas/galería con sus respuestas BFF mockeadas.
- `phase-gallery-queue.spec.ts` completo tras el cambio de superficie persistente — **7 passed**, 0 fallidos, Chromium local; cubre envío por drop, filtrado de arrastres no-file, selector por teclado, ancho completo, hit-test y dragover/drop deshabilitados, feedback conservado, cola y recuperación incierta.
- Playwright focal posterior a la barrera CAS — **7 passed** en 5 archivos; Chromium local, puertos 4205, dividido en un grupo de 6 y un caso de wizard; sin llamadas a API Go ni DB.
- Corrida combinada de 18/18: evidencia histórica de un candidato previo de superficie persistente; no se volvió a ejecutar para esta revalidación final. No se usa como gate actual. Las pruebas focales actuales son 12/12 para galería y 1/1 para el recorrido phase7 seleccionado.
- Las corridas anteriores de 13 tests y de 5 casos de contrato/403/cola/incertidumbre corresponden a candidatos más antiguos; se conservan como historial y no validan el dropzone actual.
- `pnpm test` — 481 casos, 480 passed, 0 fallidos y 1 omitido opt-in; `typecheck` — PASS.
- `lint` — 0 errores, 4 advertencias existentes en archivos no modificados; `format:check` — PASS en la corrida parental consolidada; `git diff --check` — PASS tras esta actualización documental.
- RED/GREEN del timestamp: el caso de `created_at` ausente falló contra el comportamiento anterior porque se mostró una tarjeta en vez de reportar galería no confirmada; con el guard y rebuild, el conjunto contractual queda en 12/12. La verificación no contactó Go ni base de datos.
- La prueba de reordenamiento ahora consulta el orden de las tarjetas del DOM después del éxito y después del 412.
- No se ejecutó la prueba Axe separada en estas corridas focales.

## Revisión independiente del dropzone

- `deleg_d6946f75` encontró tres bloqueantes: prevenir eventos ajenos a archivos y `dragleave`, limpiar un error al descartar un drop deshabilitado, y ofrecer un dropzone sin semántica de teclado.
- Las regresiones previas a la corrección dieron **4 fallos RED** esperados: `dragenter` prevenido, payload text/plain prevenido, ausencia de rol button y estado disabled no nativo. Luego una regresión geométrica detectó el botón inline (**404.8 px** frente al contenedor de **453.3 px**); `block w-full` hizo pasar esa prueba tras nuevo build.
- `deleg_deaa9e97` revisó la edición anterior a la superficie persistente y señaló correctamente que un `dispatchEvent` dirigido al botón nativo deshabilitado no probaba su entrega por el navegador. No reportó problemas de seguridad; recomendó una superficie receptora permanente y cobertura más directa. El primer RED del test de superficie falló por ausencia de ese target; tras agregarlo, el test valida que el hit-test real del DOM sobre el botón deshabilitado resuelve a la superficie y que sus handlers cancelan el `dragover` de archivos con `dropEffect=none` y el `drop`, sin encolar ni borrar el error.
- La corrección actual conserva el botón nativo accesible y sitúa los handlers en un `div` persistente; deshabilitado, el botón usa `pointer-events-none` para exponer esa superficie al hit-testing. El resultado observado fue queue **7/7** y combinado **18/18** tras reconstruir. La prueba usa DataTransfer/eventos sintéticos controlados: no simula un arrastre desde el sistema operativo ni prueba Go/persistencia.
- `deleg_d6e91a0a` re-revisó este candidato final en HEAD `647cd491985d619370990d3f226b5df61e36b42b`: **PASS**, sin riesgos de seguridad ni bloqueos lógicos. Confirmó superficie persistente, hit-test en estado disabled, cancelación de dragover/drop de archivos, `dropEffect=none`, ausencia de POST/pérdida de error, filtrado de payloads ajenos, semántica de botón/teclado y ancho. La revisión fue solo de fuente; no ejecutó comandos ni pruebas. Su única sugerencia coincide con el límite ya declarado: los eventos DataTransfer son sintéticos y no prueban un arrastre externo originado por el sistema operativo.

## Revisión independiente del guard `created_at`

- La revisión inicial `deleg_3e0fee4f` detectó correctamente que `Date.parse` normalizaba `2026-02-30T12:00:00Z`. El padre reprodujo el fallo con una tarjeta visible, añadió esa fecha imposible al caso controlado y endureció el guard para validar calendario, hora y zona antes del parseo.
- Revisión final read-only `deleg_87e16948`: **PASS**, sin hallazgos de seguridad o lógica. Confirmó la obligatoriedad de fecha para `ready`, la excepción del DTO `pending` y que el `202` de cola usa solo `id`, `space_id`, `state`. El límite de segundos `00–59` quedó explícito como el subconjunto emitido por `time.Time` del API actual; `time.Parse(time.RFC3339, "2016-12-31T23:59:60Z")` devolvió error en un probe Go local. La única sugerencia —una aserción directa adicional del tipo `ApiProtocolError`— es no bloqueante: la prueba observable exige galería no confirmada, cero tarjetas y cero lecturas de variantes, y el adaptador lanza esa clase. Esta revisión no ejecutó tests/build ni accedió servicios.

Una invocación inicial pasó incorrectamente `--` a `pnpm test:e2e`, por lo que Playwright ejecutó la suite amplia en vez del filtro solicitado: **152 passed, 63 skipped, 3 failed** (el RED intencional de dropzone y dos casos fuera de este corte en `phase6-account.spec.ts`). Esos dos fallos ajenos no se investigaron ni se presentan como gate aprobado; el caso RED se reprodujo de forma aislada y después la suite focal y corrida combinada actuales pasaron.

## Revisión independiente del 403 mock

- `deleg_6443017f` reportó un posible desacuerdo entre la denegación y la purga observables. La inspección directa de `host-listings-facade.ts:1845-1849, 195-202, 1906-1917` y `host-listings-runtime.ts:8-13` confirma la cadena `403 → #handleAuthError() → clear() → #onPrivatePurge → privateBlobUrls.clear()`, que explica el estado vacío y la revocación observada; no se justificó cambiar código de producto.
- Sí se aplicó su observación válida sobre el mock: la ruta ahora reconoce solo `GET` y exige una única solicitud GET exacta. Después de ese cambio de prueba, Playwright focal del caso terminó **1 passed**; typecheck, lint y formato volvieron a pasar.
- La revisión independiente fue de fuente; el revisor no pudo ejecutar pruebas en su entorno. La ejecución indicada aquí corresponde al padre y sigue siendo evidencia de UI con BFF mock.

## Límites

- Son pruebas de UI con respuestas BFF simuladas; el 403 también es simulado. No prueban Go, PostgreSQL, GCS, conversión/persistencia real ni la autorización efectiva del propietario.
- No se inició API, no se accedió ni modificó ninguna base, no se cambiaron controles de seguridad y no se crearon cuentas ni medios persistidos.
- El test nativo opt-in histórico verifica 403 de una segunda cuenta con `page.request`, pero no se reejecutó por falta de target integrado aislado autorizado. El nuevo caso verifica solo la respuesta visible de UI y purga local, no la autorización backend. El ownership y readback integrados siguen pendientes.
- La brecha de UI de entrada drag-and-drop quedó cubierta solo a nivel de dropzone/cola mockeada; no prueba que los bytes sintéticos sean imágenes válidas ni que un servidor los acepte.
- Orden 3 continúa **parcial**: ownership, almacenamiento, autorización y readback reales siguen pendientes para un target integrado aislado autorizado; el caso 403 continúa mock y no acredita enforcement backend.

## Revalidación local posterior — 2026-11-11

- Tras el cambio independiente de CTA de orden 4 se repitieron `api:check` (inventario de 200 operaciones), `test` (480 passed, 0 failed, 1 opt-in skipped), `typecheck`, `lint` (0 errores; 4 advertencias existentes), `format:check` y `build`; todos finalizaron correctamente.
- Las cuatro suites focales de galería volvieron a pasar **12/12** sobre el build nuevo. La prueba usa BFF mock/fixtures; no amplía la evidencia a Go, PostgreSQL, GCS ni persistencia.
- La suite completa no se declara aprobada: los fallos históricos de ejecución amplia y los gates integrados descritos arriba siguen sin resolución/target autorizado.

## Revalidación parental de orden 3 — 2026-11-11

- En el worktree `espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `api:check` confirmó 200 operaciones y `build` completó con Next.js 16.4.3.
- Sobre ese build, `phase-gallery-contract-guard`, `phase-gallery-queue`, `phase-gallery-uncertain` y `phase-gallery-forbidden-purge` pasaron **12/12**; el recorrido `phase7-host-listings.spec.ts --grep 'Fotos con selección de imágenes'` pasó **1/1**. Puertos locales comprobados libres antes de iniciar; resultados con Chromium y BFF mocks.
- En el mismo árbol: `test` registró 481 casos (480 pasaron, 0 fallaron, 1 opt-in omitido); `typecheck`, `lint` (0 errores, 4 advertencias preexistentes), `format:check` y `git diff --check` pasaron.
- Límite sin cambio: no se inició Go/API ni se accedió a PostgreSQL, GCS o datos persistidos. Los eventos `DataTransfer` siguen siendo sintéticos; no se probó arrastre originado por el sistema operativo ni AXE en estas corridas focales. Orden 3 permanece **parcial** hasta disponer de un target integrado aislado autorizado y verificar ownership, almacenamiento y readback reales.
- Revisión independiente read-only `deleg_58ba0b4e`: **PASS**, sin defectos de seguridad o lógica en los archivos/hunks de orden 3. Sugirió como mejora no bloqueante una aserción unitaria directa de `ApiProtocolError`; se añadió en la cobertura posterior de abajo. El revisor no ejecutó pruebas/build ni usó servicios.

## Revalidación Axe de galería poblada — 2026-11-11

- Se amplió `tests/phase-gallery-forbidden-purge.spec.ts` para auditar con Axe la galería poblada antes del 403 mock, en escritorio **1280×800** y móvil **390×844**.
- Ejecución local: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 exec playwright test tests/phase-gallery-forbidden-purge.spec.ts --reporter=line` — **1/1 aprobada**; ambas vistas devolvieron cero violaciones.
- Standalone local en loopback y respuestas BFF simuladas; no se inició Go/API ni se accedió a DB/GCS. La cobertura no demuestra arrastre desde el sistema operativo ni persistencia/ownership real.
- Sin cambio de código productivo; no hubo commit, push, `main` ni despliegue.

## Aserción directa del error de protocolo — 2026-11-11

- `tooling/host-listings-adapter-contract.test.mjs` transpila la clase real `ApiProtocolError` de `src/lib/api/errors.ts` e importa esa misma clase en el módulo de adaptador aislado; los DTO `ready` inválidos ahora deben lanzar exactamente esa clase, no solo algún `Error`.
- Focal: **3/3 aprobadas**. Suite unitaria completa posterior: **489 total, 488 aprobadas, 0 fallidas y 1 opt-in omitida**. ESLint y Prettier focales — PASS.
- Sin cambio productivo ni de contrato; esta prueba no acredita el recorrido HTTP, API real, DB, GCS ni persistencia.

## Sondeo descartado de `dropEffect` sintético — 2026-11-11

- Se probó una aserción de `dropEffect="copy"` leyendo `DataTransfer` después de `dispatchEvent` en Chromium. El fixture devolvió `none` incluso con payload `Files` y `effectAllowed="copy"`, mientras `dragover` sí quedó `defaultPrevented`. Este sondeo no permite distinguir el comportamiento de una sesión nativa de arrastre del ciclo sintético; no se conservó la aserción ni cambios de prueba asociados.
- Sin defecto productivo demostrado. Tras revertir el sondeo, las cuatro suites contract-guard/queue/uncertain/forbidden-purge pasaron **12/12**; el límite de arrastre originado por el sistema operativo permanece explícito.

## Revalidación tras revisión local — 2026-11-11

- La prueba de drop no-file ahora exige `defaultPrevented` al soltar el payload, y conserva la ausencia de POST/error borrado; `dragenter`, `dragover` y `dragleave` no-file siguen sin cancelarse. La regresión dio RED contra el handler anterior y GREEN tras mover `preventDefault()` al inicio del `onDrop`.
- Se endureció `mapPhotoToGalleryPhoto` contra `planning/openapi.yaml:10015-10057`: las fotos `ready` no sintéticas requieren IDs UUID sin normalización, `alt_text` (máximo 250), booleanos `visible`/`cover`, posición entera 1–30, estado reconocido, fecha `date-time` válida y 1–2 variantes utilizables. Cada variante aceptada requiere WebP, ruta privada exacta, tamaño entero 1–10 MiB, SHA-256 hexadecimal minúsculo de 64 caracteres y dimensiones enteras 1–1600; una variante inválida no se descarta si otra sobrevive. El DTO `pending` valida ambos UUID y puede omitir `created_at`. No se introdujo un validador genérico de JSON Schema ni se ejecutaron llamadas a API real.
- `tooling/host-listings-adapter-contract.test.mjs` prueba estos casos con el adaptador real y comprueba `ApiProtocolError`: **6/6 aprobadas**. El typecheck y la build Next.js 16.4.3 pasaron.
- La primera corrida de `pnpm check` posterior a la validación detectó una respuesta mock inválida en `phase7-host-listings.spec.ts`: al superar nueve cargas, el SHA generado dejaba de tener 64 caracteres hexadecimales. Se corrigió solo el fixture para formar 64 dígitos hex válidos. La prueba afectada pasó aislada (**1/1**) y el E2E completo posterior terminó **220 tests: 157 aprobados, 63 omitidos, 0 fallidos**.
- La primera corrida de `pnpm check` pasó API (200 operaciones), ESLint (0 errores/4 advertencias), typecheck, unitarias **492 total (491 aprobadas, 0 fallidas, 1 opt-in omitida)**, Prettier y build, pero quedó con exit 1 en E2E por el fixture descrito. Tras corregir el fixture, la nueva corrida consolidada `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0** con esos gates y E2E **220 (157 aprobados, 63 omitidos, 0 fallidos)**; build Next.js 16.4.3 generó 32/32 páginas estáticas.
- Resultado local con Chromium, standalone y BFF mocks. No se inició Go/API ni se accedió a PostgreSQL, GCS o Cloud SQL. Orden 3 continúa parcial respecto de ownership, almacenamiento y readback integrado.

## Revalidación focal tras completar los gates locales de orden 2 — 2026-11-11

- Sobre el último standalone generado por `pnpm check`, las cuatro suites de galería pasaron **12/12**; el recorrido de editor con selección de fotos/portada/reordenamiento pasó **1/1**.
- Chromium local y BFF route mocks únicamente. No se inició Go/API ni se accedió a base, GCS o almacenamiento persistente. Se mantienen los límites de eventos DataTransfer sintéticos, ownership y readback integrado.

## Continuación de orden 3 — rechazo de drop inválido y lectura estable de reemplazo — 2026-11-11

- `phase-gallery-queue.spec.ts` suma un caso que suelta un `image/gif` sintético: `drop` previene navegación, se conserva el mensaje permitido de validación, no aparece estado de cola y no se emite POST. La suite focal quedó **8/8**; en la corrida final las cuatro suites de galería sumaron **13/13** (guard contractual 3, cola 8, incertidumbre 1 y purga ante 403 1).
- El recorrido del editor instrumenta `URL.revokeObjectURL`: tras cargar la miniatura sintética del ID de la foto, confirma que el DELETE/refresh retira su tarjeta y revoca esa misma Blob URL. La ruta mock ahora responde para las fotos sintéticas vigentes, incluso IDs creados durante la sesión; focal del editor **1/1**. Esto cubre ciclo de UI/URL, no borrado ni lectura de storage real.
- La corrida consolidada previa encontró un fallo en la comparación literal de Blob URL durante una sustitución pendiente (`phase7-host-listings.spec.ts`): una URL `blob:` nueva para la misma foto disparó una aserción de identidad más estricta que el comportamiento de producto requerido. Se ajustó el test para fijar el `data-photo-id` previo y exigir preview Blob durante la incertidumbre/error; al resolverse `ready`, exige el ID nuevo y su preview Blob. No se cambió código productivo.
- Tras ambos ajustes, el recorrido focal `navegación por pestañas` pasó **1/1** y se repitió la corrida consolidada desde el inicio: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check`, **exit 0**: OpenAPI 200; ESLint 0 errores/4 warnings; typecheck; unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright **221 (158 aprobados/63 omitidos/0 fallidos)**.
- La evidencia sigue siendo Chromium local con standalone y BFF mocks. No prueba bytes aceptados por Go, ownership, storage ni readback persistido; no se inició API/worker ni se accedió a bases/GCS/Cloud SQL. Orden 3 sigue parcial por el gate integrado aislado ausente.

## Revalidación del límite de entrada — 2026-11-11

- En `frontend/tests/phase-gallery-queue.spec.ts` se añadió una regresión para el límite exacto por archivo: 10 MiB llega al POST mock y queda confirmada por la respuesta sintética; 10 MiB + 1 byte y extensión/MIME incompatibles muestran el error de validación sin generar POST adicionales.
- La prueba usa un buffer cero-sintético para comprobar solo tamaño y metadata declarada; no es una imagen decodificable ni acredita el límite/validación de bytes del backend.
- Focal de `phase-gallery-queue.spec.ts`: **9/9**. La corrida consolidada `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 con **32/32** páginas; Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**. Las cuatro suites de galería sumaron **14/14**.
- Ejecución local con Chromium, standalone y BFF mocks sobre HEAD `647cd491985d619370990d3f226b5df61e36b42b`. No se inició Go/API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL; orden 3 continúa parcial para ownership, almacenamiento y readback integrados.

## DELETE con ETag obsoleto — continuación local/mock — 2026-11-11

- `frontend/tests/phase-gallery-uncertain.spec.ts` ahora simula un DELETE de foto con `If-Match: "2"` rechazado por `412 precondition_failed`, después de recuperar una incertidumbre previa de portada.
- El recorrido comprueba que la tarjeta/ID de la foto no desaparece, no se envía un segundo DELETE, las acciones de mutación (borrar y marcar portada) quedan deshabilitadas y solo una actualización explícita restaura el estado y las acciones.
- Verificación focal final: cuatro suites contract-guard/queue/uncertain/forbidden-purge — **14/14**; recorrido del editor `phase7-host-listings.spec.ts --grep 'Fotos con selección de imágenes'` — **1/1**. Prettier, ESLint focal, typecheck y `git diff --check` — PASS.
- Ejecución Chromium local con BFF fixtures/mocks. El 412 y el ETag son simulados: no prueba CAS de Go/PostgreSQL, ownership, borrado en storage ni readback persistido. Sin llamadas a API/worker, sin acceso a bases/GCS y sin cambios de código productivo o controles de seguridad.

## Revocación de Blob URLs de galería al cerrar sesión — 2026-11-11

- `phase-gallery-uncertain.spec.ts` instrumenta `URL.revokeObjectURL` y ahora, después de reconciliar la incertidumbre/412 en la galería, captura las dos URLs Blob visibles, cierra sesión desde el control del workspace y exige que la ruta vuelva a `/acceso`, que desaparezcan las tarjetas y el título privado, y que se revoquen todas las URLs capturadas.
- El mock de logout responde 204 y no sustituye el comportamiento de auth/runtime; el test registra la salida visible y el ciclo Blob del navegador. El DELETE previo continúa siendo un único `412` sintético.
- Cuatro suites contract-guard/queue/uncertain/forbidden-purge: **14/14**; recorrido del editor de fotos/portada/reordenamiento **1/1**. Tras formato: Prettier, ESLint focal, typecheck y `git diff --check` PASS.
- Ejecución local Chromium con BFF mocks. No prueba enforcement de logout del servidor, ownership, storage ni readback persistido; no se inició Go/API/worker ni se accedió/modificó base o GCS, y no se alteraron controles de seguridad. Orden 3 permanece parcial por los gates integrados pendientes.

## Contrato de variantes de galería — continuación local — 2026-11-11

- La revisión del DTO encontró que `SpaceMediaPhoto` referenciaba `SpaceMediaVariant` con `name: [display, thumbnail, avatar]`, mientras el adaptador de galería solo acepta `display` y `thumbnail`; la ruta Go para variantes de fotos de espacio también solo admite esos dos nombres (`internal/gallery/transport/http/handler.go:509`).
- Se añadió una regresión de contrato que compara el enum OpenAPI exacto. Dio RED con `avatar` presente; se corrigió `planning/openapi.yaml` a `[display, thumbnail]` y se regeneró `frontend/src/lib/api/generated/openapi.d.ts`. No se cambió el endpoint ni el adaptador.
- `api:check` confirmó tipos/allowlist/inventario actuales (200 operaciones); `pnpm test` — **494 total, 493 aprobadas, 0 fallidas, 1 opt-in omitida**; typecheck PASS; ESLint **0 errores/4 warnings preexistentes**. El formateo del test se verificó por separado; `planning/openapi.yaml` conserva su estilo compacto del proyecto y no se reescribió con Prettier.
- Es corrección del contrato y prueba local, no comprobación de payload real. Sin Go/API/DB/GCS/Cloud SQL, sin operaciones persistentes ni despliegue; los gates integrados de orden 3 siguen pendientes.

## Revalidación Go local sin DB — 2026-11-11

- Se ejecutaron sin caché, con `GOPROXY=off` y `GOTOOLCHAIN=local`, `go test ./internal/gallery/transport/http -count=1` y `go test ./internal/media -count=1`; ambos paquetes pasaron.
- Los tests de estos paquetes usan stubs/fixtures en proceso; no se inició API/worker ni se conectó a PostgreSQL. Esto amplía la evidencia local Go del orden 3, pero no prueba el recorrido browser→BFF→Go→DB, ownership real, almacenamiento externo ni readback persistido.
- Orden 3 continúa parcial hasta el gate integrado autorizado. Sin cambios backend, SQL, migraciones, seguridad, almacenamiento persistente, commit, push o despliegue.

## Descarte local y revalidación de autoridad — 2026-11-11

- `phase-gallery-queue.spec.ts` comprueba que `pending`/`not_found` no exponen «Descartar carga fallida»; un lookup autoritativo `failed` sí lo permite. Al iniciar una nueva consulta, la UI revoca esa facultad antes de resolver la petición; un resultado posterior `pending` la mantiene revocada, y solo un nuevo `failed` la habilita otra vez.
- El descarte final quita solo la fila local, sin repetir el POST ni hacer DELETE; el recheck es explícito y conserva la misma clave. La prueba controla una respuesta pendiente mediante una barrera, usa archivo sintético y mocks BFF; no implica reversión ni borrado remoto.
- `pnpm check` consolidado — **exit 0**: OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **494 (493 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3; Playwright **223 (160 aprobados/63 omitidos/0 fallidos)**. El caso nuevo pasó en Chromium local.
- `git diff --check` — PASS. Sin API/worker, acceso a DB/GCS/Cloud SQL, cambio productivo, migraciones, controles, commit, push, integración ni despliegue. Orden 3 sigue parcial para ownership, almacenamiento y readback integrados.

## Lookup de estado no disponible — recuperación explícita — 2026-11-11

- Se agregó un recorrido UI donde el POST de carga responde 422 y el lookup de estado responde 503. La fila permanece pendiente, no habilita reintento ni descarte, y conserva disponible «Consultar estado de carga».
- Solo tras el click de consulta explícita y un lookup autoritativo `failed` aparece el descarte local. El escenario confirma un único POST, dos GET de estado, ninguna foto agregada y cero DELETE; no infiere si el servidor almacenó o canceló bytes.
- Prueba focal nueva **1/1**; las cuatro suites focales de galería **16/16**. La corrida consolidada `pnpm check` finalizó exit 0: OpenAPI 200; ESLint 0 errores/4 warnings; typecheck; unitarias **494 (493 aprobadas/1 opt-in omitida)**; build Next.js 16.4.3 **32/32**; Playwright **224 (161 aprobados/63 omitidos/0 fallidos)**.
- Solo Chromium/standalone con BFF mocks y archivo sintético. Sin API/worker/DB/GCS/Cloud SQL, sin cambio productivo o de seguridad y sin commit/push/integración/despliegue. Orden 3 integrado permanece parcial.

## Reconciliación explícita a `ready` — revalidación final — 2026-11-11

- Se corrigió el fixture de `explicit ready lookup recovers a prior unavailable check without replay`: el POST mock ahora incrementa su contador y conserva el callback que materializa la foto al reconciliar el lookup `ready`; la prueba asegura un POST, dos lookups, una lectura posterior de galería, tarjeta visible de la foto confirmada y cero DELETE.
- Reejecución focal tras la corrección: **1/1 aprobada** en Chromium local. ESLint focal, `tsc --noEmit`, Prettier y `git diff --check` pasaron.
- Corrida final consolidada sobre el árbol corregido: `pnpm check` **exit 0**; OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; unitarias **494 (493 aprobadas, 0 fallidas, 1 opt-in omitida)**; typecheck y formato PASS; build Next.js 16.4.3 **32/32**; Playwright **162 aprobadas, 63 omitidas, 0 fallidas**.
- Evidencia solo de UI/cola con archivo y BFF mocks sintéticos. No demuestra estado remoto, ownership, Go, almacenamiento ni readback persistido. No se inició API/worker ni se accedió a PostgreSQL/GCS/Cloud SQL; sin cambios de producción/seguridad, commit, push, integración ni despliegue. Orden 3 permanece parcial por el gate integrado y falta del target aislado autorizado.

## Revalidación focal del contrato del adaptador — 2026-11-11

- Se volvió a ejecutar `node --test tooling/host-listings-adapter-contract.test.mjs` desde `frontend/`: **7/7 aprobadas**. Incluye comparación OpenAPI de variantes, mapeo de DTO `pending`/`ready`, validación de timestamp/metadata y rechazo mediante la clase real `ApiProtocolError`, incluidos UUID inválidos.
- Alcance unitario del adaptador, sin build/E2E en esta rebanada. No inició API/worker ni accedió a DB/GCS; no prueba payload Go, ownership, storage o readback persistido. Sin cambios de código productivo ni de seguridad.

## Revalidación Axe sobre galería poblada — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 exec playwright test tests/phase-gallery-forbidden-purge.spec.ts --reporter=line` — **1/1 aprobada** en Chromium local.
- El caso evalúa la galería poblada con Axe a **1280×800** y **390×844** antes del 403 simulado; ambas mediciones reportaron cero violaciones. Solo standalone y BFF mocks; no acredita enforcement de Go, autorización real, DB, almacenamiento o readback.

## Reejecución combinada de galería — 2026-11-11

- Con los puertos de ensayo libres, las cuatro suites `phase-gallery-contract-guard`, `phase-gallery-queue`, `phase-gallery-uncertain` y `phase-gallery-forbidden-purge` pasaron **17/17** en Chromium local.
- Incluye contrato `ready/pending`, validación de subida/drop, 403 y purga visual, recuperación/lookup, retry deliberado con misma identidad, y descartes limitados a `failed` autoritativo. BFF mocks y fixtures sintéticos únicamente; no demuestra API Go, ownership, storage ni readback. Sin DB/servicios persistentes.

## Aceptación de variantes individuales según OpenAPI — 2026-11-11

- La revisión independiente cuestionó si `ready` debía exigir simultáneamente `display` y `thumbnail`. Se adjudicó contra el contrato vigente: `SpaceMediaPhoto.variants` especifica `minItems: 1`, `maxItems: 2`, y cada elemento limita `name` a `display` o `thumbnail`; no exige que ambos estén presentes. La regla del adaptador sigue rechazando cero, más de dos o cualquier variante malformada, sin endurecer el contrato.
- Se añadió una caracterización unitaria explícita para cada variante individual aceptada y su ruta BFF reconstruida. `node --test tooling/host-listings-adapter-contract.test.mjs` pasó **8/8**; Prettier focal PASS; la suite unitaria completa terminó **495 total (494 aprobadas, 0 fallidas, 1 opt-in omitida)**.
- Es test-only; no se rehizo build/E2E consolidado porque no cambió producto. El gate real Go/storage/ownership/readback no queda acreditado.

## Seguimiento de revisión: UUID helper y timestamps Go — 2026-11-11

- El harness contractual transpila ahora el `domain/validation.ts` real en vez de imitar `isValidUUID`; mantiene `ApiProtocolError` real para las aserciones. Se añadieron casos de `created_at` con fracción nanosegundo y offset numérico, además de rechazo de `+24:00`.
- Una sonda local aislada con Go stdlib confirmó parseo y serialización JSON de `2026-11-11T12:00:00.123456789Z` y `2026-11-11T12:00:00+05:30`; `+24:00` parsea pero no puede serializarse a JSON por Go, y el adaptador lo rechaza. El spec pasó **9/9**; `pnpm test` pasó **496 total (495 PASS, 0 FAIL, 1 opt-in omitida)**; Prettier focal y `git diff --check` PASS.
- Solo tests/sonda local; no cambió el adaptador ni se inició servicio/API/DB. No sustituye el gate integrado.

## Variante única de galería: render de `display` — continuación de orden 3 — 2026-11-11

- Al aplicar la cardinalidad OpenAPI (`variants` 1–2, `thumbnail` y/o `display`) al recorrido visible, se detectó que `ListingGallery` pedía siempre `thumbnail`. Una foto `ready` válida con solo `display` recibía 404 en el mock estricto y mostraba el error de carga aunque su variante disponible fuera utilizable.
- `ListingGallery` ahora prefiere `thumbnail` cuando está declarada; si no, carga `display`; si no hay variantes declaradas, conserva el intento previo de `thumbnail`. El mock de `phase7-host-listings.spec.ts` ahora entrega solo variantes declaradas y comprueba para una foto `display`-only la petición correcta y la Blob URL visible; los casos con ambas variantes mantienen cobertura de miniatura.
- El recorrido focal `phase7-host-listings.spec.ts --grep 'Fotos con selección'` pasó **1/1** tras rebuild. Consolidación `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` — **exit 0**: OpenAPI 200 operaciones; ESLint 0 errores/4 warnings preexistentes; typecheck; unitarias **496 (495 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright **225 (162 aprobadas, 63 omitidas, 0 fallidas)**.
- Evidencia solo local Chromium/standalone y BFF mocks sintéticos. No prueba Go/API, autorización/ownership, storage ni readback integrado; no se accedió a DB/GCS/Cloud SQL ni se tocaron controles o migraciones. Orden 3 sigue parcial hasta target integrado aislado autorizado.

## Lectura binaria de variante privada a través del BFF — continuación de orden 3 — 2026-11-11

- `frontend/tooling/backend-gateway-credentials.test.mjs` ahora ejecuta el handler BFF real para `GET /spaces/{spaceId}/gallery/{photoId}/variants/thumbnail`; solo el `fetch` upstream se sustituye por una respuesta de bytes sintéticos. Comprueba preservación byte a byte, `image/webp`, `no-store`, `x-request-id`, URL backend exacta y Bearer sintético sin reenviar cookie de navegador (`credentials: omit`, `cache: no-store`, `redirect: manual`).
- La suite BFF focal pasó **6/6**. La suite completa `node --test tooling/*.test.mjs` pasó **503 total: 502 aprobadas, 0 fallidas y 1 omitida opt-in**. ESLint focal, Prettier focal y `git diff --check` — PASS.
- Es caracterización de transporte del BFF con upstream simulado; los bytes no se decodifican y no prueban autorización/ownership de Go, almacenamiento, GCS ni readback persistido. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL; no hubo cambios productivos, migraciones ni controles de seguridad. El orden 3 integrado permanece parcial y bloqueado por falta de target aislado autorizado.

## Revalidación local de orden 3 tras orden 2 — 2026-11-11

- Las cuatro suites de galería (`phase-gallery-contract-guard`, cola, incertidumbre y purga) pasaron **17/17** en Chromium local. El recorrido de edición de fotos, portada y reordenamiento pasó **1/1**.
- `node --test tooling/host-listings-adapter-contract.test.mjs tooling/backend-gateway-credentials.test.mjs` pasó **15/15**; `GOPROXY=off GOTOOLCHAIN=local go test ./internal/gallery/transport/http ./internal/media -count=1` pasó ambos paquetes.
- TypeScript (`tsc --noEmit`), ESLint focal, Prettier focal y `git diff --check` pasaron. No se modificó código productivo en esta revalidación; no se ejecutó build.
- Evidencia local con Chromium, fixtures/BFF mocks y tests Go offline. No se inició API/worker, ni se accedió a DB/GCS/Cloud SQL; ownership, storage y readback integrado siguen sin probarse. El gate integrado continúa parcial por faltar el target aislado autorizado y sus artefactos privados requeridos.

## Reemplazo de foto con la galería al límite — continuación local/mock — 2026-11-12

- `phase7-host-listings.spec.ts` ahora lleva el mock a 30 fotos y reemplaza específicamente la portada sin usar un cupo adicional. Verifica el PUT dirigido al ID de portada con el ETag vigente, cero POST de carga nuevos, 30 tarjetas y readback de 30 fotos; la portada sigue asignada a la foto presente y el DELETE posterior usa la revisión renovada por el reemplazo.
- La primera ejecución focal reveló que la expectativa de una lectura de galería era demasiado estricta: `replacePhoto` reconcilia detalle/fotos y el componente hace además su refresh explícito. Se ajustó la regresión para exigir ambas lecturas; la corrida final del recorrido pasó **1/1**.
- Reejecución de las cuatro suites contract-guard/queue/uncertain/forbidden-purge: **17/17**. `tsc --noEmit`, ESLint, Prettier y `git diff --check` — PASS. No se modificó código productivo ni se ejecutó build.
- Solo Chromium local, standalone y BFF mocks. No prueba límite/ownership en Go, CAS PostgreSQL, almacenamiento ni readback persistido; no se inició API/worker ni se accedió a DB/GCS/Cloud SQL o controles de seguridad. Orden 3 sigue parcial hasta el gate integrado en target aislado autorizado.

## Barrera completa de acciones tras DELETE 412 — continuación local/mock — 2026-11-12

- `phase-gallery-uncertain.spec.ts` amplía el escenario de DELETE rechazado por revisión obsoleta: exige que upload, marcar portada, mover antes/después, reemplazar y eliminar estén deshabilitados; la actualización explícita de galería continúa habilitada. Tras GET exitoso confirma que se restauran las acciones válidas y se conservan las restricciones de primer/último elemento al reordenar.
- Cuatro suites contract-guard/queue/uncertain/forbidden-purge: **17/17**; recorrido del editor de fotos, portada y reordenamiento: **1/1**. `tsc --noEmit`, ESLint focal, Prettier focal y `git diff --check` PASS. Solo se modificó cobertura; no se reconstruyó el standalone.
- Evidencia Chromium local con fixture/BFF mocks. No prueba CAS/ownership Go/PostgreSQL, storage o readback persistido. Sin API/worker, DB/GCS/Cloud SQL, cambios de producción/seguridad ni operaciones persistentes; el gate integrado del orden 3 sigue parcial por falta de target aislado autorizado.

## Revisión de recuperación tras DELETE 412 — ETag renovado — 2026-11-12

- La revisión source-only observó que la fixture anterior devolvía la misma revisión antes y después del 412, lo que no modelaba un conflicto CAS reproducible. La fixture ahora avanza el ETag externo a `"3"` antes de rechazar la primera eliminación (`If-Match: "2"`). Tras «Actualizar galería», una segunda eliminación usa `If-Match: "3"` y recibe 204 con la revisión renovada; se comprueba la secuencia de cabeceras y la tarjeta eliminada.
- La implementación del facade `loadPhotos` obtiene en paralelo la lista de galería y el detalle del espacio, exige ETag en el detalle y actualiza `selectedListing`; por eso el refresh explícito también renueva la revisión, no solo las fotos.
- Playwright focal `phase-gallery-uncertain.spec.ts` terminó **1/1** tras la nueva secuencia. La línea detectada por Prettier se ajustó manualmente; la verificación consolidada de formato queda registrada en la evidencia de orden 5. No se cambió código productivo. Solo Chromium/standalone y BFF mock; no acredita CAS Go/PostgreSQL, ownership, storage o readback persistido. Gate integrado sigue parcial.

## Lectura HTTP de variante privada — caracterización Go no mutante — 2026-11-12

- `internal/gallery/transport/http/handler_test.go` ahora recorre `GET /spaces/{spaceID}/gallery/{photoID}/variants/thumbnail` por el handler y `media.Service` reales, con repositorio y object store in-memory. Verifica el scope de owner/espacio/foto pasado al repositorio, variante `thumbnail`, key/generation solicitadas, digest/tamaño confirmados por el servicio, bytes y headers `image/webp`, `private, no-store`, `nosniff` y SHA-256. El nombre no contractual `avatar` devuelve 404 sin leer storage.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/gallery/transport/http ./internal/gallery ./internal/media -count=1` PASS; `go vet` de los mismos paquetes, `gofmt -d` y `git diff --check` PASS.
- Es una caracterización HTTP con repositorio/object store falsos; no ejecuta PostgreSQL, GCS ni storage persistente y no prueba ownership efectivo del adaptador real. Sin API/worker ni cambios de producción o controles; el gate integrado del orden 3 continúa parcial.

## Revalidación local de orden 3 tras avance de orden 2 — 2026-11-12

- La validación encontró el cliente OpenAPI generado desactualizado respecto del contrato de galería (descripción `no-store`/revalidación por solicitud y exclusión de `avatar`). Se regeneró únicamente `frontend/src/lib/api/generated/openapi.d.ts` con el generador del proyecto; el diff quedó limitado al endpoint público de variante y al enum de variantes. `pnpm api:check` pasó después: 200 operaciones y allowlist/inventario vigentes.
- `node --test tooling/host-listings-adapter-contract.test.mjs tooling/backend-gateway-credentials.test.mjs`: **15/15**. Go `./internal/gallery/transport/http ./internal/gallery ./internal/media -count=1`, `go vet` y `gofmt -d`: PASS.
- Frontend: Prettier PASS; ESLint **0 errores y 1 warning** preexistente en `listing-gallery.tsx:546` por `<img>`; `tsc --noEmit` PASS; build Next.js 16.4.3 con **32/32** páginas generadas; Playwright Chromium local **17/17** entre contract-guard/cola/incertidumbre/purga y **1/1** del recorrido de edición con selección de imágenes, portada y reordenamiento; `git diff --check HEAD` PASS.
- Solo entorno local, standalone y fixtures/BFF mocks sintéticos. No se inició API/worker ni se accedió a DB, GCS o Cloud SQL; no se alteraron controles de seguridad, migraciones ni datos persistentes. Estas pruebas no demuestran ownership del adaptador real, almacenamiento durable ni readback integrado. El gate integrado de orden 3 sigue bloqueado hasta contar con target aislado autorizado.

## Reconciliación de reemplazo incierto — caracterización local — 2026-11-12

- La revisión detectó una posible brecha cuando un reemplazo devuelve 503 y el estado posterior es `not_found`. El rastreo confirmó que `getPhotoUploadStatus` reconcilia por `loadPhotos` para `failed`/`not_found`; no se confirmó un defecto de producción.
- `phase7-host-listings.spec.ts` ahora simula 503 → consulta de estado 404/`not_found`: conserva visible la foto previa y bloquea reemplazo, portada, reordenamiento y eliminación mientras el resultado está incierto. La lectura autoritativa de detalle/galería limpia el bloqueo; el reintento explícito reutiliza idempotency key, archivo y ETag originales. La prueba comprueba que no hubo POST de carga, reordenamiento, eliminación ni cambio de portada durante la incertidumbre.
- Playwright focal de edición: **1/1**; Prettier, ESLint focal, `tsc --noEmit` y `git diff --check`: PASS. Solo standalone local y mocks sintéticos; no acredita persistencia, ownership real ni CAS en PostgreSQL.

## Readback de la primera foto después de crear un borrador — 2026-11-12

- La caracterización del recorrido `/anfitrion/espacios/nuevo` ahora cuenta el GET inicial de galería, envía una foto sintética al borrador recién creado y exige un GET posterior cuyo DTO incluya el mismo `photoId` antes de afirmar que la carga quedó confirmada; también verifica la portada asignada. Playwright focal `phase7-host-listings.spec.ts --grep 'asistente en 5 pasos'`: **1/1**.
- Corrida consolidada en el árbol final: `pnpm check` exit 0, unitarias **503 (502/0/1 opt-in omitida)**, build **32/32**, Playwright default **226 (163/63/0)**; calendario **6/6**, galería pública **2/2**. Go offline tests/vet de publiccatalog, occupancy, spaces HTTP, gallery y media PASS.
- Es un readback del fixture BFF/Playwright, no de Go/PostgreSQL/storage ni persistencia. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL; orden 3 integrado sigue parcial por falta de target autorizado.

## Primera carga real del servicio/handler con fakes — caracterización local — 2026-11-12

- `internal/media/service_upload_test.go::TestUploadSpaceCreatesReadyPhotoFromSyntheticBytes` ejecuta la conversión real de PNG sintético a variantes WebP y verifica SHA-256 de entrada, reserva, orden `plan→Put→finish→record` para `display` y `thumbnail`, finalización `ready`, metadatos de bytes/generación y URLs de API. Repositorio y object store son in-memory.
- `internal/gallery/transport/http/handler_test.go::TestUploadSpaceCreatesReadyPhotoThroughHTTPHandler` envía multipart sintético por el handler HTTP y servicio reales; valida `201`, `no-store`, dos variantes WebP, una sola reserva con clave idempotente, dos escrituras al fake y finalización ready. No inicia `cmd/api`.
- Corrida final local con opt-ins de corpus/experimentos deshabilitados: `GOPROXY=off GOTOOLCHAIN=local go test -v ./internal/media ./internal/gallery/transport/http -count=1` PASS; `go vet` de ambos paquetes, `gofmt -d` y `git diff --check` PASS. Los tres tests opt-in de corpus/encoder quedaron omitidos; sus recursos privados/scratch no se usaron.
- Solo prueba de conversión, servicio y transporte con repositorio/object store falsos. No demuestra SQL, ownership efectivo, idempotencia/CAS PostgreSQL, GCS, persistencia tras reinicio ni browser→BFF. Sin migraciones/DDL, API/worker, cuentas/espacios, GCS ni cambios de controles; el gate integrado del orden 3 sigue parcial.

## Resultado incierto de escritura de objeto — caracterización local — 2026-11-12

- `TestUploadSpacePutFailureFinishesAmbiguousWriteAndFailsReservation` simula que el object store conserva los bytes de `display` pero devuelve error, como resultado incierto. El servicio devuelve `ErrUnavailable`, intenta finalizar la intención exacta de escritura, marca una vez la reserva fallida y no procesa `thumbnail`, registra variantes ni finaliza la foto. La prueba confirma ID/token/tiempo sintéticos enviados a `FailReservation`.
- `TestUploadSpaceRecordFailureDeletesWrittenGenerationAndFailsReservation` simula error al registrar la metadata después de un PUT confirmado; verifica DELETE del key/generation exactos, eliminación de los bytes en el fake y fallo de reserva, sin continuar a `thumbnail` ni finalizar.
- `TestUploadSpaceFinishFailureDeletesWrittenGenerationAndFailsReservation` simula un error al cerrar la intención después del PUT; comprueba la misma eliminación por generación, el fallo de reserva y que no se intente registrar metadata ni procesar otra variante.
- Suite final `GOPROXY=off GOTOOLCHAIN=local go test -v ./internal/media ./internal/gallery/transport/http -count=1`, con opt-ins de corpus/encoder deshabilitados: ambos paquetes PASS; `go vet`, `gofmt -d` y `git diff --check` PASS. Tres experimentos opt-in omitidos.
- El almacén y repositorio son falsos: no prueba las escrituras ambiguas, cleanup/Generation ni recuperación del worker real, ni SQL/GCS/persistencia. Sin cambios de producción, migraciones, API/worker o datos.

## Rechazo definitivo de archivo y reemplazo explícito — UI/cola local — 2026-11-12

- La prueba `tooling/gallery-upload-queue.test.mjs` se ejecutó primero en RED: ante `Error` con `status=422` y `code=validation_error`, la cola hacía lookup y devolvía `failed`, no el rechazo definitivo esperado. `internal/gallery/transport/http/handler.go` asigna ese contrato a errores de validación de media; en `internal/media/service.go`, `ValidateInput` ocurre antes de reservar la operación, y el handler normaliza el error a `422 validation_error`.
- `GalleryUploadQueue` ahora identifica ese error contractual como `rejected`, no consulta el estado de una carga rechazada, no ofrece reintentar el mismo archivo, permite quitar la fila y libera el selector para un archivo distinto. El test de cola confirma cero lookups, un solo POST, sin reintento implícito y descarte local; `node --test tooling/gallery-upload-queue.test.mjs` pasó **12/12**.
- `phase-gallery-queue.spec.ts` agrega un recorrido browser que recibe 422 sintético, comprueba el mensaje accionable, la ausencia de `GET upload-status`, quita el archivo y carga otro con clave idempotente distinta. La suite final, tras reconstruir el standalone, pasó **14/14**. El build Next 16.4.3 generó **32/32** páginas; `tsc --noEmit`, ESLint y Prettier focal PASS.
- La primera ejecución E2E anterior al build falló porque Playwright inicia `.next/standalone`, cuyo `server.js` era más antiguo que los cambios fuente. Se verificaron las fechas locales, se ejecutó `pnpm build` y el test focal pasó **1/1** antes de repetir la suite completa. No se atribuye aquel fallo a un defecto de la app.
- Go offline: `GOPROXY=off GOTOOLCHAIN=local go test ./internal/media ./internal/gallery/transport/http -count=1` y `go vet` de ambos paquetes PASS; `gofmt -d` y `git diff --check` PASS. Todos los recorridos browser usan BFF mocks/fixtures sintéticos; los tests Go usan repositorio/object store falsos. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL; no se tocaron migraciones, controles de seguridad, datos persistentes ni infraestructura. Persistencia, ownership efectivo e integración browser→BFF→Go→DB/storage siguen sin demostrar; orden 3 permanece parcial.

## Rechazo temprano de bytes de imagen inválidos — caracterización local — 2026-11-12

- `TestUploadSpaceRejectsInvalidImageBeforeReservation` envía bytes sintéticos no decodificables por `Service.UploadSpace`; verifica `ErrInvalid`, resultado vacío y cero eventos de reserva/fallo, escrituras o borrados en repositorio/object store falsos. Caracteriza el guard ya existente de `ValidateInput` antes de reservar.
- El test focal pasó; `GOPROXY=off GOTOOLCHAIN=local go test ./internal/media ./internal/gallery ./internal/gallery/transport/http -count=1`, `go vet` de los tres paquetes, `gofmt -d` y `git diff --check` PASS.
- No cambia producción ni el contrato; no se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o recursos persistentes. Esto no prueba enforcement HTTP integrado, ownership ni storage real; el gate integrado de orden 3 continúa parcial por falta de target aislado autorizado.

## Fallos de planificación y finalización de carga — caracterización local — 2026-11-12

- `TestUploadSpacePlanFailureFailsReservationBeforeObjectWrite` simula el rechazo al planificar `display`: el servicio devuelve el error original, falla una sola vez la reserva con ID/token/tiempo esperados y no ejecuta PUT, DELETE ni almacena bytes.
- `TestUploadSpaceFinalizeFailureFailsReservationAfterRecordingVariants` simula un error al finalizar después de registrar las dos variantes. El servicio devuelve el error original y falla la reserva una vez; el fake conserva los dos objetos y no hay DELETE inline, por lo que esta prueba no afirma recuperación/limpieza del worker.
- Focales PASS; `go test -count=1` y `go vet` pasaron para `internal/media`, `internal/gallery` e `internal/gallery/transport/http`; `gofmt -d` y comprobaciones whitespace/diff PASS. Son tests con repositorio/object store in-memory: no prueban PostgreSQL, ownership real, GCS, cleanup persistente ni readback tras reinicio. Sin API/worker, DB, cambios de producción o controles de seguridad; orden 3 sigue parcial.

## Cancelación después de planificar una variante — 2026-11-12

- La regresión `TestUploadSpaceCancellationAfterPlanningStopsBeforeObjectWrite` cancela el contexto dentro del stub de `PlanVariant`, que devuelve éxito, y exige `context.Canceled`, reserva fallida una sola vez y cero PUT/DELETE/objetos. RED observado antes del cambio: el servicio devolvía éxito y continuaba con la escritura fake.
- `Service.upload` ahora revisa `ctx.Err()` tras una planificación exitosa y antes de llamar al object store; al detectar cancelación, marca fallida la reserva con contexto independiente y devuelve el error de cancelación.
- El test focal, `go test` y `go vet` offline de `internal/media`, `internal/gallery` e `internal/gallery/transport/http` pasaron. La evidencia es de servicio con repositorio/object store fake; no acredita escritura concurrente real, cleanup de worker, PostgreSQL, GCS, ownership ni readback persistido. No se inició API/worker ni se accedió a DB o controles; orden 3 integrado sigue parcial.

## Fallo al planificar la segunda variante — 2026-11-12

- `TestUploadSpaceThumbnailPlanFailureFailsReservationAfterDisplayWrite` inyecta el fallo únicamente al planificar `thumbnail`, después de almacenar y registrar `display`. RED inicial: el fake fallaba ya en `display`, por lo que el prefijo observado no incluía la primera variante; se amplió el fake para seleccionar la variante donde falla la planificación.
- GREEN: se propaga el error de planificación, resultado vacío, una sola marca de reserva fallida, secuencia exacta hasta `plan:thumbnail` y ausencia de PUT de miniatura o borrado inline. El objeto `display` permanece en el fake; no se ejecuta ni se acredita cleanup del worker.
- El test focal y `go test`/`go vet` offline de `internal/media`, `internal/gallery` e `internal/gallery/transport/http` PASS; `gofmt -d` y `git diff --check HEAD` PASS. No cambia producción ni prueba PostgreSQL/GCS, ownership, persistencia o readback. Sin API/worker ni acceso a recursos persistentes; orden 3 integrado sigue parcial.

## Fallo ambiguo de PUT en la segunda variante — 2026-11-12

- `TestUploadSpaceThumbnailPutFailureFinishesAmbiguousWriteAndFailsReservation` extiende el fake para emular un error de PUT solo en `thumbnail`, después de almacenar sintéticamente sus bytes. Caracteriza el límite posterior a la variante `display` ya completada.
- Verifica `ErrUnavailable`, resultado vacío, orden exacto `plan/put/finish` de thumbnail seguido por `fail`, ausencia de `record:thumbnail` y de DELETE inline, y conservación de los dos objetos en el fake. No ejecuta ni demuestra la recuperación del worker.
- El test focal pasó como caracterización; `go test` y `go vet` offline de `internal/media`, `internal/gallery` e `internal/gallery/transport/http`, `gofmt -d` y `git diff --check HEAD` PASS. Solo cambió el fake/test; sin API/worker, DB/GCS, ownership ni persistencia. El gate integrado de orden 3 sigue parcial.

## Fallo al finalizar la escritura de la segunda variante — 2026-11-12

- `TestUploadSpaceThumbnailFinishFailureDeletesOnlyThumbnailGeneration` inyecta el fallo en `FinishVariantWrite` solo para `thumbnail`, después de completar y registrar `display`.
- Verifica `ErrUnavailable`, el orden hasta `finish:thumbnail`, un único DELETE por key de miniatura y generación exactos, reserva fallida una vez, ausencia de `record:thumbnail` y conservación de `display` en el fake. El caso caracteriza el borrado inline de la generación actual, no la recuperación posterior del worker.
- Test focal, `go test`/`go vet` offline de `internal/media`, `internal/gallery` e `internal/gallery/transport/http`, `gofmt -d` y `git diff --check HEAD` PASS. Solo cambió fake/test; no producción, API/worker, DB/GCS, ownership ni persistencia. Orden 3 integrado continúa parcial.

## Rechazo HTTP 413 antes de reservar carga — 2026-11-12

- La inspección de `internal/gallery/transport/http/handler.go` confirma que `multipartParseError` mapea `http.MaxBytesError` a `413 upload_too_large`; el límite por bytes del archivo también devuelve ese par antes de invocar `media.UploadSpace`. El POST de galería ya emitía 413 pero no declaraba esa respuesta en `planning/openapi.yaml`.
- Se añadió la respuesta 413 con el esquema Error al POST y se regeneró únicamente `frontend/src/lib/api/generated/openapi.d.ts`; `pnpm api:check` confirma 200 operaciones e inventario/allowlist actuales. No se alteró la ruta ni un control de seguridad.
- La regresión de `gallery-upload-queue.test.mjs` dio RED antes de la corrección: el 413 se reconciliaba como `failed` en lugar de `rejected`. La cola ahora considera definitivo solo `413/upload_too_large` además del par existente `422/validation_error`; no hace GET de estado para un rechazo previo a reserva.
- Verificación final: suite Node `pnpm test` **506 total (505 PASS, 0 fallidas, 1 omitida opt-in)**; Playwright Chromium/standalone con BFF mock, `phase-gallery-queue.spec.ts --grep @controlled` **15/15**; `pnpm build` Next.js 16.4.3, **32/32**; `pnpm api:check` **200 operaciones**; `pnpm typecheck` PASS; ESLint **0 errores, 4 warnings**; Prettier PASS para TS/MJS/defs generadas; `git diff --check HEAD` PASS.
- `prettier --check ../planning/openapi.yaml` sigue señalando formato en otros hunks ya modificados del archivo (entre ellos líneas 68, 2809–3068, 6487, 7542–7580, 7920 y 7952). La comparación con la salida formateada no encontró cambios en el hunk 413; no se reformató el resto del contrato para evitar churn ajeno.
- Solo fixtures/BFF mocks y build local. Sin API/worker, DB, GCS, Cloud SQL, datos persistentes, migraciones o cambios de controles. No prueba el request path integrado, ownership, SQL/storage ni readback persistido; el gate integrado de orden 3 continúa parcial hasta autorización del target y sus precondiciones.

## Revalidación de límites del cleanup `CleanOnce` — 2026-11-13

- Se reejecutaron las pruebas fake-backed de `CleanOnce`: retención de intención mientras un `Put` concurrente aún puede crear el objeto; completado idempotente cuando la escritura terminal ya no existe; fallos de lookup de generación y delete marcados para retry; propagación de errores en claim/fail/complete; y límites de lote inválidos rechazados antes de reclamar trabajo.
- `GOPROXY=off GOTOOLCHAIN=local go test -run '^TestCleanOnce' -count=1 ./internal/media` y la misma selección con `-race` pasaron. También pasaron `go test -count=1 ./internal/media ./internal/gallery ./internal/gallery/transport/http`, `go vet` de esos paquetes, `gofmt -d` y `git diff --check HEAD`.
- Estas pruebas usan repositorio/object store falsos y no arrancan el worker. No acreditan cleanup real tras fallo de finalización, acceso por generación en GCS, transacciones/ownership PostgreSQL ni readback tras reinicio. El target aislado autorizado para orden 2 no se usó en esta revalidación ni autoriza integración de orden 3; el gate integrado de galería sigue parcial.

## Readback de galería real y fotos sintéticas — caracterización Go local — 2026-11-13

- `TestGalleryListMergesSyntheticAndRealMediaInPositionOrder` recorre `GET /api/v1/spaces/{spaceID}/gallery` con `gallery.Service`, `media.Service` y el handler HTTP reales. Repositorios in-memory devuelven una foto `ready` real sintética en posición 1 y una foto de fixture en posición 2; la prueba exige el scope owner/space, orden combinado, y la URL privada de variante reconstruida por `media.Service`.
- `GOPROXY=off GOTOOLCHAIN=local go test -count=1 ./internal/gallery ./internal/gallery/transport/http ./internal/media` pasó; también pasó la misma prueba de handler con `-coverpkg=./internal/media` y `go vet` de los tres paquetes. La cobertura de `media.Service.ListSpace` a través del handler quedó en **83.3%**; `gofmt` y `git diff --check HEAD` pasaron.
- Es una caracterización con repositorios/object store sintéticos: no prueba el adaptador PostgreSQL, ownership efectivo, almacenamiento persistente ni readback tras reinicio. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL; el gate integrado de orden 3 sigue parcial y la aprobación de orden 2 no se extiende a este orden.

## Fallo cerrado del listado de media — caracterización Go local — 2026-11-13

- `TestGalleryListFailsClosedWhenRealMediaListingFails` simula `media.Repository.ListSpace` fallando con `context.DeadlineExceeded` mientras hay entradas sintéticas y una foto real sintética disponibles. El handler responde `503 media_unavailable`; no entrega una lista parcial ni filtra los IDs o el error interno, conserva el `request_id` de respuesta y verifica que el servicio de media recibió el scope owner/space provisto por el auth stub.
- Prueba focal `TestGalleryList(FailsClosedWhenRealMediaListingFails|MergesSyntheticAndRealMediaInPositionOrder)` — PASS; `GOPROXY=off GOTOOLCHAIN=local go test -race -count=1 ./internal/gallery/transport/http ./internal/gallery ./internal/media` y `go vet` de los tres paquetes — PASS; `gofmt -d` y `git diff --check HEAD` — PASS.
- No cambió código productivo; la prueba es una caracterización de la ruta de error con repositorios in-memory. No se inició API/worker ni se accedió a PostgreSQL/GCS/Cloud SQL; no prueba ownership efectivo ni persistencia. Orden 3 integrado sigue parcial y no hereda la autorización integrada de orden 2.

## Revalidación local/mock de orden 3 — 2026-11-13

- `pnpm build` pasó con Next.js 16.4.3, TypeScript y generación de **32/32** páginas.
- Playwright Chromium sobre standalone local y BFF fixtures: contract-guard, queue, uncertain y forbidden-purge **20/20**; recorrido `phase7-host-listings.spec.ts --grep 'Fotos con selección de imágenes'` **1/1**.
- El recorrido `phase7-host-listings.spec.ts --grep 'asistente en 5 pasos'` pasó **1/1**: crea un borrador sintético, entra al editor, carga la primera imagen, confirma el mismo ID en el readback mock, marca portada y verifica que `POST /spaces` no se repita. Es UI/BFF fixture; no acredita persistencia.
- `node --test tooling/gallery-upload-queue.test.mjs tooling/host-listings-adapter-contract.test.mjs tooling/backend-gateway-credentials.test.mjs`: **29/29 passed**.
- Con `env -i`, `GOPROXY=off` y `GOTOOLCHAIN=local`, `go test -race -count=1` y `go vet` pasaron para `internal/media`, `internal/gallery` e `internal/gallery/transport/http`; `gofmt -l` no reportó archivos. ESLint focal: 0 errores y 1 warning conocido `@next/next/no-img-element` en `listing-gallery.tsx:546`; Prettier focal y `git diff --check HEAD` pasaron.
- Esta revalidación no modificó código fuente. Solo Chromium/BFF mocks y repositorios/object stores falsos; no se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL. No prueba ownership real, almacenamiento durable ni readback persistido. Orden 3 integrado permanece parcial y la autorización del orden 2 no se extiende.

## Caracterización fake-backed de lecturas del servicio de galería — 2026-11-13

- Se añadieron pruebas directas de `gallery.Service.List` y `Content`: la lista y la recuperación de metadata pasan exactamente owner/space/photo al repositorio; la lectura válida devuelve los bytes solo cuando coinciden tamaño y SHA-256.
- Casos de archivo inaccesible, tamaño incorrecto y contenido de igual tamaño con SHA-256 distinto devuelven `ErrFileStore`. Todo usa repositorio y `FileStore` falsos; no cambia el código productivo.
- `GOPROXY=off GOTOOLCHAIN=local go test -race -count=1 ./internal/gallery ./internal/gallery/transport/http ./internal/media` y `go vet` de esos paquetes pasaron. `internal/gallery` alcanzó **20.0%** de cobertura de sentencias; `List` 66.7%, `Metadata` 66.7% y `Content` 90.0%. La cobertura restante no se persiguió por porcentaje.
- Es evidencia unitaria local, no integración del adaptador PostgreSQL, autorización efectiva, object store durable ni readback tras reinicio. No se inició API/worker ni se accedió a DB, GCS o Cloud SQL; el gate integrado de orden 3 sigue parcial.

## Variante privada: rechazo previo a leer bytes — 2026-11-13

- `TestVariantRouteDoesNotReadObjectWhenSpaceScopedPhotoIsUnavailable` ejecuta la ruta HTTP y `media.Service.ContentSpace` reales con fakes: el lookup scoped por owner/espacio/foto devuelve `media.ErrNotFound`; el handler retorna 404 y no llama `GetVariant` ni lee el object store. La respuesta no contiene los bytes sintéticos configurados en el fake.
- El test focal `-race`, `go test -race -count=1 ./internal/gallery ./internal/gallery/transport/http ./internal/media`, `go vet` de esos paquetes y `gofmt` pasaron. No se modificó código productivo.
- Límite: verifica el orden de llamadas y la propagación del scope hasta el repositorio falso; no demuestra el filtro SQL, autorización efectiva, GCS ni persistencia. Sin API/worker ni acceso a DB/GCS/Cloud SQL; orden 3 integrado permanece parcial.

## Lectura directa de variante privada en `media.Service` — 2026-11-13

- Se añadió `internal/media/service_content_test.go` para caracterizar `Service.ContentSpace` con repositorio y object store falsos. La prueba positiva verifica el scope owner/espacio/foto, la consulta de variante owner/foto/nombre, la key y generación solicitadas, la URL privada reconstruida y los bytes solo si coinciden tamaño y SHA-256.
- Casos sintéticos de owner inválido o foto scoped ausente paran antes de consultar la variante/storage; metadata ausente detiene la lectura del objeto; fallo de storage, tamaño discordante o SHA-256 discordante devuelven `ErrUnavailable` sin entregar metadatos ni bytes parciales.
- `go test -race -count=1 -run '^TestContentSpace' ./internal/media` pasó. También pasaron `go test -race -count=1` y `go vet` para `internal/media`, `internal/gallery` e `internal/gallery/transport/http`; `gofmt` y `git diff --check HEAD` pasaron. La medición local reporta `ContentSpace` **100%**, `GetSpace` **100%** y `internal/media` **70.9%** de sentencias; cobertura es diagnóstico, no gate de aceptación.
- Solo caracterización de servicio con fakes/bytes sintéticos; no prueba el adaptador PostgreSQL, ownership efectivo, GCS, almacenamiento durable ni readback tras reinicio. No hubo cambio productivo o de seguridad, API/worker, acceso a DB/GCS/Cloud SQL ni operaciones persistentes. Orden 3 integrado sigue parcial.

## Orden 3 — rechazo HTTP de carga al alcanzar el límite — 2026-11-13

- `TestUploadSpaceMapsGalleryLimitBeforeWritingObjects` envía una imagen PNG sintética al handler HTTP y ejecuta `media.Service` real con un repositorio fake que devuelve `media.ErrLimit` en la reserva. Verifica `409 gallery_limit`; la reserva se intenta una vez y no hay planificación, finalización ni `Put` de objetos.
- El test focal, `go test -race -count=1 ./internal/gallery/transport/http ./internal/gallery ./internal/media`, `go vet` de los tres paquetes, `gofmt` y `git diff --check HEAD` pasaron con `GOPROXY=off` y `GOTOOLCHAIN=local`.
- Límite: caracteriza el contrato HTTP y el corte antes del almacenamiento cuando la reserva rechaza por capacidad; no prueba que PostgreSQL compute 30/31 correctamente ni concurrencia/atomicidad, ownership efectivo, GCS o readback persistido. No se inició API/worker ni se accedió a DB/servicios persistentes. Orden 3 integrado sigue parcial.
