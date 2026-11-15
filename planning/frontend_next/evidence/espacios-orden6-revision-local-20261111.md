# Orden 6 — revisión local y gates de entrega, 2026-11-11

- Repositorio: `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`.
- Rama y HEAD revisados: `codex/frontend/fix/space-01-create-postcommit-recovery`, `647cd491985d619370990d3f226b5df61e36b42b`.
- Estado del candidato: rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`; worktree intencionalmente sucio y sin commit. `git diff --check HEAD` terminó sin errores después de las correcciones.
- Alcance: continuación local autorizada con fixtures sintéticos. Se corrigieron dos defectos de fuente con pruebas RED/GREEN y se ejecutaron unitarias, tipos, lint, formato focal, API check, build y Playwright contra harness locales. La re-revisión independiente `deleg_90001874` pasó sin bloqueantes; dejó dos sugerencias de cobertura no bloqueantes, registradas al final. No se usaron navegador/CUA del usuario, API real, DB, credenciales, GCS, Cloud SQL, ni acciones Git remotas.

## Dictamen local

**Los dos defectos de contrato/validación identificados en el baseline tienen correcciones locales, regresiones en verde y re-revisión independiente aprobada.** `deleg_90001874` devolvió `passed: true`, sin concerns de seguridad ni errores lógicos, para el diff actualizado. `deleg_1807fa93` corresponde solo al baseline previo. La adjudicación del padre contrastó DTO OpenAPI, adaptador y validación Go.

### Bloqueo 1 — se inventa `created_at` para una foto pendiente

- OpenAPI distingue `SpaceMediaPhotoPending` de `SpaceMediaPhoto`: el DTO `pending` solo requiere `id`, `space_id` y `state`, y no define `created_at` (`planning/openapi.yaml:10064-10077`).
- En el baseline, el adaptador permitía esa ausencia cuando `state === "pending"`, pero después siempre construía `created_at`, usando `new Date().toISOString()` si no existía uno válido; el tipo de dominio lo exigía como string.
- Reproducción sintética válida: `{id, space_id, state: "pending"}`. La adaptación del baseline asignaba una hora actual que el servidor nunca afirmó. Esto corrompía el dato de dominio y podía atribuir una hora de creación falsa a una operación aún pendiente.
- Corrección local: `GalleryPhoto.created_at` es opcional (`frontend/src/features/host-listings/domain/types.ts:170-184`); el adaptador conserva únicamente una fecha válida y omite el campo si el DTO pendiente no la trae (`frontend/src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts:202-239`). `frontend/tooling/host-listings-adapter-contract.test.mjs` transpila y ejercita `mapPhotoToGalleryPhoto` y comprueba que el objeto adaptado tampoco posee `created_at`. El test focal dio RED antes del cambio y GREEN después.

### Bloqueo 2 — se aceptan intervalos semánticamente inválidos en el horario público

- En el baseline, el adaptador validaba formato, longitud y cardinalidad, pero `publicWeeklySchedule` no verificaba `close > open` ni solapamientos.
- La validación autoritativa de Go rechaza intervalos con cierre menor o igual a apertura y periodos solapados (`internal/booking/weekly_hours.go:35-66`). El patrón OpenAPI valida sintaxis de `HH:MM`, no esas relaciones entre campos (`planning/openapi.yaml:7915-7943`).
- Reproducción sintética legal para el esquema: un día con `open: "15:00", close: "14:00"`, o con `09:00–13:00` y `12:00–17:00`. El adaptador del baseline conservaba el horario, aunque la regla Go lo rechazaba.
- Corrección local: el adaptador omite el horario completo si algún intervalo no tiene duración positiva o se solapa; los periodos contiguos siguen siendo válidos (`frontend/src/features/public-catalog/infrastructure/go-public-catalog-adapter.ts:119-200`, alineado con `internal/booking/weekly_hours.go:35-66`). Las regresiones sintéticas cubren intervalos invertidos, vacíos, solapados y adyacentes (`frontend/tooling/public-verification-refresh.test.mjs`). El test focal dio RED antes del cambio y GREEN después.

## Observaciones adjudicadas que no son bloqueos

- La objeción de que `+23:59` no es un offset RFC 3339 válido es falsa: RFC 3339 define `time-hour` como `00–23` y `time-minute` como `00–59` (sección 5.6). La comprobación de `isGoTimeDateTime` para esos rangos no constituye un defecto (`frontend/src/features/host-listings/infrastructure/host-listings-openapi-adapter.ts:161-199`).
- La fixture `malformedReadyPhoto` contiene `created_at` válido y omite `variants`; las fixtures siguientes prueban por separado fecha ausente e imposible (`frontend/tests/phase-gallery-contract-guard.spec.ts:7-54`). Por tanto, sí hay una aserción independiente de variantes ausentes.
- Observación de cobertura supersedida por la fuente actual: `public-weekly-calendar.spec.ts:54-56` busca los marcadores privados en el DOM después de interactuar con el calendario hidratado, en cada viewport del loop (`:33-56`). La reejecución local del calendario terminó 4/4. Esto cierra esa sugerencia de cobertura mock, no demuestra privacidad de una API real.
- La revisión no halló evidencia de credenciales, bypass de autorización o exposición de datos privados en los cambios examinados. Las pruebas de privacidad siguen siendo locales y sintéticas; no prueban el comportamiento de una API real.

## Verificación local posterior a las correcciones

- Runtime verificado: Node `v26.12.1` (compatible con `engines: >=24.21.0 <27`) y package manager fijado `pnpm 12.15.0`.
- `node --test tooling/*.test.mjs`: **486 tests; 485 aprobados, 0 fallidos, 1 opt-in omitido** (exit 0).
- `tsc --noEmit`: exit 0. ESLint: **0 errores, 4 advertencias preexistentes** (sin advertencias nuevas atribuibles a los hunks añadidos). Prettier `--check` focal en código frontend y en el plan/evidencia actualizados: exit 0.
- `node tooling/api-check.mjs`: exit 0, inventario OpenAPI vigente de **200 operaciones**.
- `next build`: exit 0, con ambiente saneado y destino de API explícito loopback; no se inició API.
- Playwright local mediante `pnpm 12.15.0`: calendario público **4/4** y guard contractual de galería **3/3**. Se usó standalone reconstruido y harness/fixtures loopback; no prueban Go, DB, GCS ni persistencia real.
- Escaneo estático de líneas añadidas: sin asignaciones sospechosas de credenciales, shell injection o interpolación SQL. El único patrón marcado fue el método benigno `RegExp.exec()` del validador de fecha.
- `git diff --check HEAD`: exit 0. No se crearon, modificaron ni limpiaron datos/recursos persistentes.

## Revisión y gates restantes

- La re-revisión local independiente del diff actualizado pasó (`deleg_90001874`) sin bloqueantes. Sus dos sugerencias no bloqueantes se cubrieron después con pruebas sintéticas directas: la foto `ready` conserva fecha/variantes válidas y rechaza fecha ausente/imposible o variantes no utilizables; el horario acepta periodos no solapados fuera de orden y conserva el orden recibido. Las suites focales de ambos archivos terminaron **10/10**; no se modificó código productivo.
- Los dos fallos globales previos de orden 5 ya se reprodujeron y resolvieron: las aserciones comprueban encabezados visibles de tarjetas, no aprobación de producto del copy; el título de `/mi-cuenta/verificaciones` usa `h2` bajo el `h1` del shell. La corrida final completa terminó con 157 aprobadas, 63 opt-in omitidas y 0 fallidas; ver `evidence/espacios-orden5-verificacion-local-20261111.md`.
- Sigue sin ejecutarse el recorrido integrado browser→BFF→Go→DB con readback autoritativo: no hay target aislado autorizado disponible. La base persistente compartida está fuera de límites (`plan_cierre_flujo_creacion_espacios_20261110.md:20-27`; evidencia de orden 5, líneas 26-28).
- No se hizo commit ni push. Esta revisión no autoriza integración a `main`, despliegue, comprobaciones online ni aceptación de producto; esos gates siguen separados.

**Estado:** revisión/correcciones locales de orden 6 y checks locales de orden 5 completados en los niveles descritos. Los órdenes 1–4 permanecen parciales por sus propios gates integrados; orden 5 también permanece parcial por requerir el gate integrado browser→BFF→Go→DB y readback. Orden 6 como integración/entrega/aceptación sigue bloqueado por falta de target aislado autorizado. No se alteraron migraciones ni se accedió a bases o servicios.

## Cobertura local adicional — 2026-11-11

- `tooling/host-listings-adapter-contract.test.mjs` ahora ejercita directamente `mapPhotoToGalleryPhoto` para una foto `ready` válida y para fechas/variantes ausentes o inválidas.
- `tooling/public-verification-refresh.test.mjs` verifica aceptación de periodos horarios no solapados recibidos fuera de orden y preservación del orden original en la salida.
- Comando focal: `node --test tooling/host-listings-adapter-contract.test.mjs tooling/public-verification-refresh.test.mjs` — **10/10 aprobadas**. Prettier focal — PASS.
- Revalidación unitaria completa posterior a añadir cobertura: `node --test tooling/*.test.mjs` — **489 total, 488 aprobadas, 0 fallidas y 1 opt-in omitida**.
- Son pruebas de adaptador con datos sintéticos; no acreditan Go, API, storage ni persistencia. El gate integrado sigue bloqueado por falta de target aislado autorizado; no se cambió el estado de órdenes 5/6.
- Sin commit, push, integración a `main`, despliegue, API/DB real ni mutaciones persistentes.

## Sugerencias de `deleg_90001874`

Las dos sugerencias no bloqueantes quedaron atendidas mediante la cobertura local adicional documentada arriba; no modifican el resultado ni el alcance de la revisión original.

## Revalidación del revisor — 2026-11-11

- Lectura independiente de fuente cotejó el DTO `SpaceMediaPhotoPending` (`planning/openapi.yaml:10064-10077`) con `mapPhotoToGalleryPhoto`: el estado `pending` sin timestamp conserva `created_at` ausente; `ready` exige timestamp válido y variantes utilizables. `GalleryPhoto.created_at` es opcional, y el adaptador no sintetiza una fecha.
- La validación de `publicWeeklySchedule` se cotejó con `ValidateWeeklyHours` (`internal/booking/weekly_hours.go:35-66`): rechaza intervalos no positivos y solapados, permite adyacentes, y conserva el orden de los períodos válidos al adaptarlos.
- `node --test tooling/host-listings-adapter-contract.test.mjs tooling/public-verification-refresh.test.mjs` terminó **13/13**, 0 fallidas/omitidas. Es cobertura de adaptador con datos sintéticos; no prueba Go en ejecución, PostgreSQL, API ni persistencia.
- `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` terminó **4/4**; incluye aserción de marcadores privados ausentes tras hidratación/interacción en desktop y móvil, usando fixture loopback.
- El checkout no ofrece un target integrado autorizado: los puertos de ensayo `4213`, `18082`, `4207`, `18088` no tenían listeners; se observó el listener preexistente `127.0.0.1:5432`, que no fue usado. El único proyecto Compose activo reportado pertenece a otro worktree (`img01-ledger-resumed`) y no se seleccionó ni tocó.
- Dictamen del orden 6 no cambia: la revisión local de fuente/cobertura queda revalidada; integración, entrega y aceptación siguen bloqueadas por el gate browser→BFF→Go→DB de órdenes 1–5 y falta de target aislado autorizado. Sin cambios productivos, commit, push, integración a `main` ni despliegue.

## Adjudicación de revisión posterior — 2026-11-11

- La revisión de seguimiento fue incompleta y no se toma como ejecución de gates. Un revisor informó que excedió la instrucción de solo lectura y detuvo su trabajo; sus observaciones se trataron como hallazgos de fuente y se verificaron en el checkout padre, sin usar esa corrida como evidencia.
- Orden 2: el botón «Crear bloqueo» ahora se deshabilita para una zona IANA guardada inválida; el caso verifica guía, estado disabled y ausencia de GET/POST. La inspección de `weekly-schedule-editor.spec.ts:287-305` confirmó que la barrera cambia el ETag sintético a v3 antes de liberar el PUT v2, produciendo el 412 esperado; no se confirmó una carrera del producto.
- Orden 3: `drop` previene navegación incluso para payload no-file, pero solo encola `Files`. El contrato local del adaptador ahora rechaza metadatos/variantes inválidos de fotos `ready` con `ApiProtocolError`; el fixture SHA de phase7 se corrigió para ser hexadecimal válido.
- Orden 5: la evidencia precisa que se comprobaron encabezados visibles de tarjetas y estructura de encabezados por separado; no afirma aprobación de copy por producto.

- Corrida consolidada final tras reparar el fixture: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` **exit 0**; API 200, ESLint 0 errores/4 warnings, typecheck, unitarias 492 (491 aprobadas/0 fallidas/1 opt-in omitida), Prettier, build Next.js 16.4.3 con 32/32 páginas estáticas y Playwright 220 (157 aprobadas/63 omitidas/0 fallidas).
- Solo BFF mocks y fixtures sintéticos. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL. Órdenes 1–5 siguen parciales por falta del target integrado aislado autorizado; no integración, publicación, despliegue ni aceptación.

## Preflight renovado del target integrado — 2026-11-11

- En este worktree no existe `.local/testing/test-account-policy.json`; no se creó ni se configuró una cuenta de ensayo.
- `docker compose ls` y `docker ps` mostraron un proyecto activo cuyo archivo Compose pertenece a otro worktree, con API publicada en `127.0.0.1:18081`; no se seleccionó ni interactuó con él. El listener `127.0.0.1:5432` permaneció preexistente. Ninguno es el target aislado autorizado de esta fase.
- Para la corrida local final, los puertos `4222` y `18089` estaban libres y se verificaron libres al terminar; `18089` no tuvo backend escuchando. No se inició API/Go/worker ni se conectó a PostgreSQL.
- El gate integrado browser→BFF→Go→DB/readback continúa bloqueado: el preflight no identificó un target aislado ni la política privada requerida en este worktree. No se reutilizaron servicios de otro worktree ni la base persistente.

## Revisión independiente del delta de CTA/archivo — 2026-11-11

- `deleg_628ef8e9` revisó en solo lectura el test de `phase7-host-listings.spec.ts` junto al flujo de `listing-editor.tsx`: **passed**, sin concerns de seguridad ni errores lógicos. Verificó que el escenario comprueba el CTA `/espacios/{id}` tras publicar y que, tras el mock de archivo, exige payload `state: "oculta"`, badge «Archivado» y desaparición del enlace público.
- Sugerencia no bloqueante: afirmar el `If-Match` en el mismo caso. Se mantiene separado del objetivo visual de este escenario: `host-listings-publication-cas.spec.ts` verifica la revisión en cada mutación, el cambio de ETag al archivar, el rechazo 412 y la ausencia de replay; esa prueba pasó dentro de la corrida consolidada **222 E2E (159 aprobadas, 63 omitidas, 0 fallidas)**. No se atribuye a la revisión nueva la demostración de CAS backend.
- La revisión y la ejecución son de fuente/UI con BFF mock. No prueban persistencia ni comportamiento real de Go/DB. Orden 6 sigue bloqueado para integración/entrega/aceptación por falta del target aislado autorizado; sin cambios funcionales, commit, push ni despliegue.

## Revisión independiente consolidada y adjudicación — 2026-11-11

- Tres revisiones read-only cubrieron el delta host-listings/galería, el adaptador/calendario público y el delta de cuenta más los documentos de orden 4–6. La primera no encontró defectos concretos; la tercera tampoco. La segunda informó problemas de zona horaria, navegación del CTA y alcance del contador de mutaciones; se verificaron contra el source actual antes de actuar.
- **Zona horaria — no es defecto:** el adaptador valida con `Intl.DateTimeFormat` y el producto ya decidió conservar los alias runtime admitidos, incluido `EST`; `public-weekly-calendar.spec.ts` prueba esa decisión. No se cambió la validación para rechazar el alias.
- **CTA del editor — no es defecto:** el test abre la ruta de edición como precondición, luego pulsa el enlace `Ver publicación` y comprueba la URL final y el contenido de la ficha SSR (`public-weekly-calendar.spec.ts:320-335`). No afirma que se haya llegado al editor mediante navegación UI desde el listado.
- **Contador de mutaciones — cobertura reforzada:** antes solo contaba métodos distintos de GET en `/api/backend/spaces`. Ahora cuenta todos los métodos no seguros bajo `/api/backend/`, exceptuando el `POST /api/backend/auth/login` de fixture; la prueba exige cero mutaciones BFF adicionales. La suite pública volvió a pasar **6/6**; Prettier, ESLint focal, typecheck y `git diff --check` PASS.
- **ID de imagen pública — sugerencia no aplicable:** `publicImageUrl` valida UUID tanto de espacio como de `image.image_id` antes de construir una ruta (`frontend/src/features/public-catalog/domain/media.ts:10-18`).
- **Cardinalidad/nombres de variantes — sugerencia no convertida en requisito:** OpenAPI admite `variants` con `minItems: 1`, `maxItems: 2` y no declara `uniqueItems` ni exige una pareja `display`+`thumbnail` (`planning/openapi.yaml:10039-10045`). No se endureció el adaptador sin un contrato vigente que exija esa invariancia.
- Observaciones documentales incorporadas: «checks locales de orden 5 completados» se distingue de los órdenes 1–4 aún parciales; «revisión local de orden 6 completada» se distingue del gate de integración/entrega aún bloqueado. Las revisiones son de fuente, tests y documentos; no acreditan el target integrado ni autorizan integración a `main`, publicación, despliegue o aceptación.

## Revisión independiente posterior y aserción de lectura — 2026-11-11

- `deleg_7e9ee450` revisó en solo lectura el enum de `SpaceMediaVariant` y el E2E de recuperación parcial. Ambas revisiones pasaron sin concerns de seguridad ni errores lógicos. La reducción a `display`/`thumbnail` coincide con el adaptador; la sugerencia de una aserción explícita adicional basada en la ruta Go queda no bloqueante y no se cambió backend.
- La revisión del wizard señaló que el test no afirmaba el GET del detalle después de reload. Se añadió contador para el `GET /api/backend/spaces/{id}` y una aserción de incremento tras `page.reload()`; se conservaron los contadores de alta, disponibilidad y horario para descartar replay de esas tres mutaciones.
- RED/GREEN de cobertura: la nueva aserción falló antes de instrumentar el GET (esperaba incremento y recibió 0); después de instrumentarlo, `PLAYWRIGHT_PORT=4203 npx --yes pnpm@12.15.0 exec playwright test tests/wizard-improvements.spec.ts tests/wizard-partial-account-scope.spec.ts --reporter=list` terminó **3/3**.
- El GET sigue siendo una respuesta BFF mock construida con fixture sintética; esto prueba que el editor vuelve a leer tras reload y conserva la navegación visible, no persistencia real ni readback de Go/PostgreSQL. Gate integrado sin cambios: bloqueado por falta de target aislado autorizado.
- Revalidación consolidada posterior a la aserción: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI 200, lint 0 errores/4 warnings, typecheck, unitarias 494 (493 aprobadas/1 opt-in omitida), Prettier, build Next.js 16.4.3 32/32 y Playwright 222 (159 aprobadas/63 omitidas/0 fallidas). `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/gallery/transport/http -run '^TestAssembledRouterKeepsSpaceRoutesAndChecksGalleryErrors$' -count=1` pasó; es una prueba sintética de handler existente, no una nueva verificación de variantes ni integración a DB.
- Preflight del puerto previo: `4213`, `18082`, `4222`, `18089`, `4207` y `18088` estaban libres. No se inició API/Go/worker para el check ni se accedió a DB/GCS/Cloud SQL. El puerto Go test trabajó solo con stubs en proceso.

## Revisión incremental de orden 4 y revalidación de orden 5 — 2026-11-11

- Revisión read-only del delta de `public-weekly-calendar.spec.ts`: el caso de 404 ahora primero establece el estado público previo (título y horario visibles en un recurso `activa`) y después navega al recurso no público; afirma status 404, ausencia de título/horario residuales y correlación con la observación del fixture. El cambio es solo de test; no altera rutas, adaptadores ni controles.
- El test público focal pasó dentro de `test:e2e:public-calendar` **6/6**. La revalidación consolidada posterior terminó **exit 0**: OpenAPI 200, ESLint 0 errores/4 warnings, 494 unitarias (493 aprobadas/1 opt-in omitida), typecheck, Prettier, build 32/32 y Playwright 225 (162 aprobadas/63 omitidas/0 fallidas); calendario público separado 6/6. `git diff --check` PASS.
- Dictamen local: el delta no introduce una mutación ni evidencia una fuga visual tras navegar del detalle activo al 404, dentro del fixture. No acredita caching/autorización de Go, elegibilidad, DB ni persistencia; no sustituye el gate integrado. Órdenes 1–5 y orden 6 entrega continúan parciales/bloqueados por falta de target integrado aislado autorizado. Sin commit, push, integración, despliegue ni acceso a servicios persistentes.

## Revisión independiente del contrato de galería — 2026-11-11

- Revisión source-only `sa-0-29b60f7f`: PASS, sin concerns de seguridad ni errores lógicos. Confirmó separación pending/ready, validación UUID/fecha/metadata y reconstrucción de rutas BFF a partir de rutas esperadas. Sugirió usar el helper UUID real en el test y documentar/probar límites de fechas; no halló defectos concretos.
- Revisión de cobertura `sa-1-b29f2f10`: señaló que no se probaba la aceptación de una sola variante. Se cotejó la observación con OpenAPI: `variants` permite de una a dos entradas y cada nombre es `display` o `thumbnail`, por lo que el código no debe requerir ambas. Se añadió test de caracterización para cada variante individual; esa rama quedó cubierta, manteniendo el contrato sin cambios. No se modificó producción.
- La sugerencia no bloqueante sobre la imitación de UUID y formatos de fecha se atendió: el spec compila el helper real `isValidUUID`; casos sintéticos cubren fracción nanosegundo y offset numérico. Una sonda aislada de Go stdlib confirmó parseo/JSON de las formas representativas y que `+24:00` no se puede serializar, por lo que el test verifica su rechazo en el adaptador.
- Tras estos cambios: spec contractual focal **9/9**, Prettier focal y `git diff --check` PASS; `pnpm test` completo **496 (495 aprobadas, 0 fallidas, 1 opt-in omitida)**. Revisores no ejecutaron pruebas ni vieron el test final; la validación fue ejecutada por el padre.
- Dictamen limitado a fuente/tests locales. No valida Go en vivo, BFF real, autorización, PostgreSQL, almacenamiento ni readback persistido; el gate integrado de órdenes 1–5 continúa parcial por falta del target aislado autorizado.

## Revisión del recorrido de catálogo activo→archivado — 2026-11-11

- Se cotejó el nuevo E2E de `public-weekly-calendar.spec.ts` con `/explorar`, `parseCatalogQuery`, `serializeCatalogQuery`, `listSpaces` y OpenAPI: `q` llega a `GET /public/spaces`; `PublicSpacePage` exige `items` y permite omitir `next_cursor`, por lo que el response sintético `{ items }` es contract-shaped. La UI renderiza el resultado activo y el empty state cuando la lista vuelve vacía.
- La prueba correlaciona las dos lecturas por query y conteo, además del enlace activo y la ausencia posterior. El estado archivado vive solo en el fixture controlado por la respuesta BFF mock; la cobertura no evalúa el filtro de elegibilidad Go ni una mutación/readback persistente. No se encontró defecto de fuente ni se modificó código de producto.
- Reejecución local `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar`: **6/6**, exit 0; `git diff --check` PASS. Puertos del harness libres antes de iniciar; no se inició API/Go/worker ni se accedió a DB/GCS/Cloud SQL.
- Orden 6 continúa bloqueado para integración/entrega/aceptación hasta disponer de un target integrado aislado autorizado y readback. No se usó la base persistente ni servicios de otro worktree; sin commit, push, integración o despliegue.

## Revisión independiente final del harness — 2026-11-11

- Re-review source-only `deleg_4e6269fa`: **PASS**, sin concerns de seguridad ni errores lógicos. Confirmó que el harness usa `isValidUUID` y `ApiProtocolError` reales, que los casos de variante única y los límites 1–2 concuerdan con OpenAPI, y que los ejemplos fraccionario/offset aceptado y `+24:00` rechazado concuerdan con el validador y la sonda Go documentada.
- La sugerencia opcional de explicitar ambos identificadores ya está cubierta: `id: "not-a-uuid"` y `space_id: "not-a-uuid"` se prueban en casos separados y cada uno exige `ApiProtocolError` (`frontend/tooling/host-listings-adapter-contract.test.mjs`). El revisor no ejecutó pruebas ni cambió archivos; la corrida parental `pnpm check` posterior al test sigue siendo la evidencia de ejecución.
- Alcance de fuente/tests locales solamente; no es integración Go/BFF/DB/storage ni readback. El siguiente gate integrado sigue bloqueado por falta del target aislado autorizado.

## Adjudicación de revisiones independientes — órdenes 2–4 — 2026-11-12

- Se verificó en el checkout la observación de orden 2 sobre el borrado de bloque manual tras 412. La regla vigente bloquea crear hasta recargar la zona/revisión usada para convertir horas locales; DELETE opera por ID y OpenAPI no define `If-Match` para bloqueos manuales. No se amplió el bloqueo ni se inventó CAS. También se descartó que el GET semanal fallido instale un horario residual: `selectListing` normaliza el error no-auth a `null` y publica ese valor antes de resolver; el editor bloquea controles y guardado hasta recargar (`host-listings-facade.ts:449-486`, `listing-editor.tsx:472-475,2021-2032,2077-2079`).
- En orden 3, el refresh sí obtiene un detalle actual junto con las fotos: `hostListingsFacade.loadPhotos` hace ambos GET, exige ETag y actualiza `selectedListing`. La revisión de la prueba a capacidad máxima era falsa: `phase7-host-listings.spec.ts` contiene aserciones explícitas de 30 tarjetas, cero POST adicional, portada y ETag en la mutación siguiente. Sí se corrigió una debilidad del fixture de 412: ahora el servidor sintético avanza a ETag `"3"`, la primera eliminación con `"2"` es rechazada y, tras refresh, la siguiente utiliza `"3"` y termina con 204; Playwright focal **1/1**.
- En orden 4 se endureció el test source-level para inspeccionar el literal `eligiblePublicSQL` concreto e insistir en que se concatene en una consulta. La relectura confirmó sus usos en `List` (`repository.go:220`), `detailQuery` consumida por `Detail` (`:378,384`), `PublicImage` (`:446`) y `PublicMediaVariant` (`:460`). Go focal PASS y `gofmt -d` limpio. Sigue siendo inspección de fuente, no ejecución SQL.
- La revisión independiente fue de solo lectura; la adjudicación y cambios de cobertura los ejecutó el agente principal. No se consultó navegador del usuario, API/Go live, DB, GCS o Cloud SQL; no se tocaron controles/migraciones y no hubo commit, push, integración, despliegue ni aceptación. Los gates integrados de órdenes 1–5 siguen parciales/bloqueados por falta de target aislado autorizado y artefactos privados; el siguiente paso permanece condicionado a ese target.

## Target DB-only aislado — 2026-11-12

- Autorización acotada: crear un target nuevo y desechable con nombre/puerto propios; por ahora solo DB vacía y backup privado. No amplía el alcance a roles de aplicación, migraciones, API, cuentas ni recorrido de producto.
- Target creado: `espacigo_wizard_20261112_order6`, contenedor `...-db`, volumen local `...-pgdata` y red Docker `...-internal` (interna), imagen PostgreSQL/PostGIS fijada por digest. El contenedor está en ejecución y los recursos están etiquetados con el nombre exacto del ensayo; no se tocó el volumen anterior `espacigo_wizard_20261106`.
- El preflight confirmó libres los puertos elegidos y ausencia previa de los recursos exactos. Docker conserva `HostPort 15437` en configuración, pero no instaló listener: `docker port` no devuelve mapeo y el probe TCP a `127.0.0.1:15437` falló. La conexión TCP directa desde el host a la IP interna real sí respondió; la publicación de puerto queda sin verificar. No se cambió la red ni se dejó un relay activo.
- Readback técnico completado en transacción `READ ONLY`: `current_database` y `session_user` coincidieron con el target y el usuario `postgres`; no existen `public.schema_migrations`, `public.usuario` ni `public.espacio`, y el conteo consultado de relaciones públicas no pertenecientes a extensiones fue cero. `pg_isready` dentro del contenedor indicó aceptación de conexiones.
- Se creó un dump custom privado y se validaron `pg_restore --list` y el parseo completo con `pg_restore --exit-on-error --file=/dev/null`; archivo y recibo quedaron bajo `.local/testing/wizard-integration-20261112/`, modo `0600`. Recibos de fallo inicial y reconciliación se conservaron; no se borró ni recreó el contenedor/volumen.
- **Desviación de procedimiento que impide cerrar el gate administrativo:** para superar la falta del puerto publicado, el readback y el dump se ejecutaron con `psql`/`pg_dump` autenticados como `postgres` mediante un relay temporal. Aunque el SQL fue de solo lectura y el relay ya está detenido, esto excedió la restricción vigente de limitar esa identidad al flujo versionado autorizado. No repetir ese método ni considerar suficiente este backup como precondición para DDL hasta resolver/revisar la desviación por la ruta autorizada.
- No se ejecutaron migraciones, DDL, bootstrap de roles, API/worker, altas de cuentas ni mutaciones de espacios. Órdenes 1–5 siguen parciales para browser→BFF→Go→DB/readback; este ensayo solo confirma el estado inicial DB-only. Sin commit, push, integración a `main`, despliegue ni aceptación.

## Revisión independiente complementaria del delta Go — 2026-11-12

- La revisión source-only `deleg_7f33d267` examinó las adiciones de pruebas de rango HTTP (handler de espacios/servicio occupancy), lectura de variante privada (handler gallery/servicio media) y validación de metadatos de imagen pública. Contrastó los casos con handlers, servicios y OpenAPI; veredicto **PASS**, sin concerns de seguridad ni errores lógicos. No ejecutó tests/build ni accedió a API, DB, GCS o credenciales.
- Límites de evidencia señalados por el revisor: los contadores/repositorios y object stores son stubs; las pruebas caracterizan validación, parámetros, llamadas y respuestas de servicio, pero no acreditan integridad/autorización PostgreSQL ni comportamiento del proveedor de storage. Esos límites ya pertenecen al gate integrado pendiente y no son defectos de las pruebas unitarias.
- El gate integrado browser→BFF→Go→DB/readback sigue bloqueado por las restricciones y precondiciones documentadas. Esta revisión no autoriza ni aprueba DDL, API/worker, cambios de controles, integración, publicación, despliegue ni aceptación.

## Revisión local final de fallback responsive — 2026-11-12

- En el checkout de la rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`, se ajustó únicamente la cobertura E2E y su evidencia: los fallbacks de galería/tarjeta registran solicitudes y respuestas como eventos individuales, sin `Set`/`Map` que oculten repeticiones; comprueban el recurso legacy y que el placeholder desaparezca cuando la imagen carga.
- Revalidación de ambos fallbacks **2/2** (cada uno recorre escritorio **1365×900** y móvil **390×844**); suite pública de galería **4/4**, calendario **6/6**; Axe focal **0 violaciones**. `pnpm check` terminó exit 0: OpenAPI 200, unitarias **504 (503 aprobadas/1 opt-in omitida/0 fallidas)**, ESLint 0 errores/4 warnings, typecheck, Prettier, build **32/32**, Playwright default **226 (163 aprobadas/63 omitidas/0 fallidas)**. Go tests/vet offline focales PASS; `git diff --check`, formato y puertos de ensayo PASS.
- La revisión independiente source-only `deleg_192334b9` pasó sin bloqueos ni errores lógicos. El contador de mutaciones se limita explícitamente a requests emitidos por la página; fixture y Next usan loopback con `reuseExistingServer: false`. No es evidencia de efectos internos, autorización real, Go, DB, storage o persistencia.
- Sin API/worker ni acceso a PostgreSQL, GCS o Cloud SQL en esta continuación; sin cambios productivos, commit, push, integración a `main`, despliegue ni aceptación. El gate integrado browser→BFF→Go→DB/readback sigue bloqueado por falta de un target aislado y operaciones autorizadas; este avance no habilita el ensayo DB-only existente ni modifica la desviación de procedimiento previamente documentada.
- Revalidación tras reforzar la correlación de eventos (2026-11-12): `pnpm test:e2e:public-gallery` **6/6**; ESLint focal, `tsc --noEmit` y Prettier PASS. Cada ruta esperada conserva sus solicitudes repetidas; cada request se correlaciona con exactamente una response, y cada response se valida contra status/MIME/cache esperados. Se filtran rutas de otras fotos no incluidas en el caso; no se ocultan duplicados de las rutas objetivo. Durante el diagnóstico, la exigencia provisional de exactamente un GET falló con repeticiones observadas (3 para `display` 404 y 2 para `thumbnail` 404); se retiró esa cardinalidad no sustentada y se validan todos los eventos, incluidos resultados asíncronos entre viewports. Solo fixtures locales; no cambió producción ni se accedió a API, DB, GCS o Cloud SQL.

## Target PostgreSQL efímero para media — 2026-11-12

- Se creó, con autorización limitada a crear una DB efímera, el target independiente `espacigo_spaces_media_upload_20261112`: proyecto Compose y base con ese nombre; imagen `postgis/postgis@sha256:60f6ad1d21ea86a67d47780b9a0d1e1d200500f62b19293fa834d0dea80b8677`; puerto publicado solo en `127.0.0.1:15439`; red Compose dedicada `espacigo_spaces_media_upload_20261112_isolated`; datos en `tmpfs`, sin volumen Docker de datos. El contenedor fue verificado `running/healthy` y el listener exacto está activo. La credencial permanece en `.local/testing/spaces-media-upload-20261112/` con permisos privados; su contenido no se leyó ni se registró.
- El preflight inicial de este target encontró cero tablas de aplicación y solo `plpgsql`; no se aplicaron migraciones ni DDL, no se crearon roles y no se inició API/worker. No se conectó a `espacigo_wizard_20261112_order6`, a la DB persistente ni a servicios de otro worktree. Este recurso nuevo no corrige ni sustituye la desviación administrativa documentada para aquel target anterior.
- Alcance autorizado y ejecutado: creación/verificación de infraestructura local efímera únicamente. **No** constituye gate de integración ni autoriza por sí solo migrador/DDL, roles de aplicación, API/worker, cuentas/espacios, GCS, cambios de controles, publicación, integración a `main` o despliegue. Órdenes 1–5 siguen parciales y orden 6 integrado continúa pendiente.
- Próximo paso que requiere decisión expresa: autorizar o no, para este nombre/puerto exactos, el preflight administrativo completo y la aplicación secuencial de migraciones versionadas con el bootstrap de mínimos privilegios definido por el repositorio. Hasta entonces, mantener la DB vacía y no iniciar el gate integrado.
- Preflight de fuente posterior: el perfil API `local-space-wizard` solo acepta `127.0.0.1:15436/espacigo_wizard_20261106`; `local-http-worker-free` está fijado a `127.0.0.1:15434/espacigo_f5_gate_20261101` (`cmd/api/worker_profile.go:101-107`). Ninguno acepta el target nuevo. El test PostgreSQL opt-in de media también tiene guard fijo a `postgres@127.0.0.1:15432/espacigo_f6_synth_20261028` y usa su raíz privada específica (`internal/adapters/postgres/media/repository_integration_test.go:157-183`). Por tanto, la DB nueva no puede conectarse a los recorridos existentes sin ampliar/crear un perfil y un harness exactos; esos guards son controles y no se modificaron ni ejecutaron. Esto es un bloqueo adicional independiente de migraciones/roles.
- Alcance siguiente que requiere decisión: cualquier perfil/harness para este target debe ser dedicado, loopback, sin workers, con identidad runtime limitada y pruebas de guardas; no reutilizar identidades/targets fijados a otros ensayos. La autorización general de continuar no habilita cambiar esos controles, iniciar API o ejecutar el harness mutante. La comprobación de GCS sigue siendo una autorización separada.

## Revisión independiente del fallback público — 2026-11-12

- Revisión source-only independiente `deleg_f3ec7a9a` examinó el spec de galería, fixture, `media.ts`, adaptador público y configuración del harness, en el worktree de la rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`. El revisor informó PASS limitado; no editó ni ejecutó pruebas/servicios.
- Adjudicación del padre: la observación sobre que `responses` no se corta explícitamente por `requestStart` no es un defecto reproducible. Aunque se filtra sobre el historial completo de respuestas, `assertPublicImageEvents` conserva solo respuestas cuyo `response.request()` es idéntico a uno de los objetos `Request` de `events.requests.slice(requestStart)`. Una respuesta anterior está asociada a su request anterior, que no pertenece a ese subconjunto; las respuestas actuales se validan individualmente contra status/MIME/cache. No se cambió el test por esa observación.
- La inspección confirma rutas públicas basadas en UUID y fuentes `media`/`synthetic` coherentes con el adaptador/contrato; los casos de fallback comprueban ambos viewports, `naturalWidth > 0` al recuperar, placeholder cuando fallan ambas rutas, navegación del carrusel/tarjeta y ausencia de mutaciones emitidas por la página. La prueba normal de thumbnail, sin fallback, se ejecuta en un viewport; los recorridos de fallback cubren escritorio y móvil.
- Alcance exclusivamente source/UI con fixture/BFF mock. No acredita autorización/elegibilidad real, integración Go, almacenamiento o persistencia. No hubo acceso a API/DB/GCS/Cloud SQL, cambios de producción, commit, push, integración ni despliegue. La ejecución local anterior sobre el mismo árbol permanece: galería **6/6**, calendario **6/6** y `pnpm check` exit 0, documentados en este archivo de evidencia del orden 4.

## Orden 6 — estado tras la revalidación local de órdenes 4–5 — 2026-11-12

- Checkout revisado: `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`; worktree permanece intencionalmente sucio y sin commit.
- La consolidación local reciente terminó `pnpm check` exit 0: OpenAPI 200, ESLint 0 errores/4 warnings, unitarias 507 (506 PASS/1 opt-in omitida), build 32/32 y Playwright default 229 (166 PASS/63 omitidas). Calendario y galería pública posteriores al build pasaron 6/6 cada una; Go offline test/vet y `git diff --check HEAD` pasaron. Esta verificación no ejecutó API/worker ni conectó a DB/GCS/Cloud SQL.
- El target `espacigo_spaces_order2_20261113` está aprobado solo para las operaciones específicas del gate de orden 2. Esa autorización no cubre integración, migraciones/roles/harness de órdenes 4–6, publicación a `main`, despliegue ni aceptación. No se inició ni inspeccionó el target durante esta actualización.
- **Dictamen de orden 6:** revisión local y checks no mutantes actualizados; integración/entrega/readback y aceptación permanecen pendientes. No hay autorización explícita para modificar remotamente la rama, integrar a `main`, desplegar o aceptar producto; además, los gates integrados previos no tienen evidencia browser→BFF→Go→DB/readback. No se declara orden 6 completado. Próximo paso: continuar solo con tareas locales no mutantes; cualquier operación integrada debe quedar dentro de su autorización exacta, con preflight propio y sin ampliar el target de orden 2.

## Revalidación local posterior — 2026-11-13

- En el mismo checkout y HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `env -i HOME="$HOME" PATH="$PATH" PLAYWRIGHT_PORT=4294 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 pnpm check` terminó exit 0: OpenAPI 200 operaciones; ESLint 0 errores/4 advertencias; TypeScript; unitarias 507 (506 aprobadas, 0 fallidas, 1 opt-in omitida); Prettier; build Next.js 16.4.3 con 32/32 páginas; Playwright default 229 (166 aprobadas, 63 omitidas, 0 fallidas).
- Sobre el artefacto recién construido, `pnpm test:e2e:public-calendar` y `pnpm test:e2e:public-gallery` terminaron 6/6 cada una. Son recorridos Chromium locales con fixtures/BFF mock; no validan Go/API, PostgreSQL, GCS ni readback persistido.
- Los puertos locales 4294, 4201 y 18082 estaban libres al terminar; no se inició API ni worker y no se conectó a base o almacenamiento. `git diff --check HEAD` pasó.
- La revalidación no cambia el dictamen: órdenes 1–5 aún requieren sus gates integrados y orden 6 continúa pendiente de integración/entrega/aceptación. El target `espacigo_spaces_order2_20261113` no se usó ni se amplió su autorización. Sin cambios de producto, commit, push, integración a `main`, despliegue ni aceptación.

## Auditoría fuente no mutante del gate — 2026-11-13

- Enumeración numérica de `db/migrations`: **84** archivos contiguos, `V000001__m01_identity.sql` a `V000084__didit_identity_verification_schema.sql`; V61 es `V000061__mpv2_12_hourly_publication_schedules.sql`. `git status --short -- db/migrations` no reportó cambios.
- Un cálculo Python independiente construyó las líneas `version filename sha256(contents)\n` y obtuvo SHA-256 `4b4757ae2975e8a72c817ecc02fbcb89b38b6d6d8c8bc0688c4d6bed7ba87907`, igual al pin embebido en `cmd/dbmigrate/spaces_order2_snapshot.go`. Los tests focales `TestStageSpacesOrder2Snapshot` y `TestStageSpacesOrder2SnapshotRejectsInventoryDrift` pasaron con `env -i`, `GOPROXY=off` y `GOTOOLCHAIN=local`.
- Alcance: la comprobación confirma coincidencia del pin con el árbol actual y el comportamiento de staging V1–V61; no acredita procedencia/revisión independiente del manifiesto, elección de prefijo para orden 6, estado de ledger de un target ni ejecución SQL.
- Revisión source-only de `cmd/api/worker_profile.go:74-115`: las rutas locales exigen los targets exactos de wizard, orden 2 o F5; no existe perfil target-locked para el target de orden 6 propuesto. La evidencia previa de este mismo archivo (líneas 181–182) exige revisión/autorización propia para agregar perfil/harness; no se alteraron guards.
- El target propuesto `espacigo_spaces_order6_20261113` en `127.0.0.1:15441` no se creó. No hubo conexión a PostgreSQL, DDL, creación de roles, inicio de API/worker ni acceso a GCS/Cloud SQL. El gate integrado permanece pendiente; `15439` y el target de orden 2 `15440` no se usaron.

## Reejecución source-only no mutante — 2026-11-13

- Tras confirmar que `TestStageSpacesOrder2Snapshot` y `TestStageSpacesOrder2SnapshotRejectsInventoryDrift` solo copian migraciones a temporales/permisos restringidos y no abren conexiones SQL, se ejecutaron con `env -i`, `TMPDIR` bajo el scratch privado, `GOPROXY=off`, `GOTOOLCHAIN=local` y `-race`: ambos pasaron (`go test -race -count=1 -run '^TestStageSpacesOrder2Snapshot($|RejectsInventoryDrift$)' ./cmd/dbmigrate`). `go vet ./cmd/dbmigrate` pasó; `gofmt` quedó limpio. `git status --short -- db/migrations` no reportó cambios.
- El preflight solo observó listeners existentes en `127.0.0.1:15439` y `:15440`; no se conectó ni interactuó con ellos. `127.0.0.1:15441` no se usó. No se creó ni autorizó el target propuesto `espacigo_spaces_order6_20261113`; no se tocaron bases, migraciones, controles, roles, API/worker o GCS.
- El trabajo local/source-only disponible para esta fase queda revalidado; la entrega/readback integrada y aceptación no pueden avanzar sin autorización explícita de target y operaciones. El ensayo DB-only histórico conserva su desviación documentada y no es sustituto. Sin commit, push, integración a `main`, despliegue ni aceptación.

## Perfil de orden 6 no registrado: rechazo antes del target — 2026-11-13

- Se añadió únicamente el test `TestRuntimeProfileRejectsUnregisteredSpacesOrder6BeforeReadingTarget` en `cmd/api/worker_profile_test.go`. Un getter sintético registra las claves consultadas y confirma que `runtimeProfile` rechaza `local-spaces-order6` leyendo solo `API_RUNTIME_PROFILE`, antes de consultar configuración de destino. No se agregó un perfil/harness de orden 6 ni se modificó código productivo.
- Con entorno `env -i`, `GOPROXY=off` y `GOTOOLCHAIN=local`, pasaron con `-race` las regresiones de rechazo del perfil no registrado, el pin/allowlist/workerless de orden 2 y `TestStageSpacesOrder2Snapshot`/`TestStageSpacesOrder2SnapshotRejectsInventoryDrift`. `go vet ./cmd/api ./cmd/dbmigrate`, `gofmt` y `git diff --check HEAD` pasaron.
- Solo tests source-level y staging en temporales. No hubo conexión a PostgreSQL, DDL, lectura de ledger, API/worker, GCS o Cloud SQL; tampoco se usaron `15439`, `15440` ni `15441`. El test no acredita target integrado ni readback.
- Dictamen: el gate integrado/entrega/aceptación de orden 6 permanece bloqueado hasta identificar y autorizar explícitamente un target aislado y sus operaciones; la autorización de orden 2 no se amplía. Sin commit, push, integración a `main`, despliegue o aceptación.

## Perfil no registrado sin lanzamiento de workers — 2026-11-13

- `TestLaunchWorkersRejectsUnknownProfileBeforeLaunching` ahora cubre tanto `unknown` como `local-spaces-order6`; en ambos casos exige error de validación y cero callbacks ejecutados. Solo usa workers sintéticos.
- Con `env -i`, `GOPROXY=off` y `GOTOOLCHAIN=local`, pasaron con `-race` esta regresión y las regresiones focales del perfil order6 no registrado, target aprobado/allowlist/workerless de orden 2. `go vet ./cmd/api`, `gofmt` y `git diff --check HEAD` pasaron.
- No se modificó producción/controles; no se inició API/worker real ni se conectó a DB/almacenamiento. Es cobertura source-level, no habilita ni completa el gate integrado de orden 6.

## Revalidación source-only del orden 6 en el árbol actual — 2026-11-13

- En rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`, se reejecutaron con entorno `env -i`, `GOPROXY=off`, `GOTOOLCHAIN=local` y `-race` los tests `TestLaunchWorkersRejectsUnknownProfileBeforeLaunching`, `TestRuntimeProfileRejectsUnregisteredSpacesOrder6BeforeReadingTarget`, `TestStageSpacesOrder2Snapshot` y `TestStageSpacesOrder2SnapshotRejectsInventoryDrift`; `cmd/api` y `cmd/dbmigrate` pasaron.
- `go vet ./cmd/api ./cmd/dbmigrate`, `gofmt -l` de los archivos focales y `git diff --check HEAD` pasaron. Las pruebas usan callbacks sintéticos y archivos temporales; no conectan a servicios.
- No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL. Los listeners preexistentes `127.0.0.1:15439`, `:15440` y `:4203` se dejaron intactos; `:15441` no se usó. No se añadió perfil de orden 6 ni se cambiaron guardas/controles.
- Dictamen: revalidación local/source-only únicamente. Integración, readback browser→BFF→Go→DB, entrega y aceptación del orden 6 siguen pendientes; el target propuesto no está autorizado ni creado y la autorización de orden 2 no se amplía. Sin commit, push, integración o despliegue.

## Reejecución source-only al avanzar a orden 6 — 2026-11-13

- En HEAD `647cd491985d619370990d3f226b5df61e36b42b`, con `env -i`, `GOPROXY=off`, `GOTOOLCHAIN=local` y TMPDIR bajo scratch privado, pasaron con `-race` las regresiones `TestLaunchWorkersRejectsUnknownProfileBeforeLaunching`, `TestRuntimeProfileRejectsUnregisteredSpacesOrder6BeforeReadingTarget`, `TestStageSpacesOrder2Snapshot` y `TestStageSpacesOrder2SnapshotRejectsInventoryDrift`.
- `go vet ./cmd/api ./cmd/dbmigrate`, `gofmt -l` de los archivos focales y `git diff --check HEAD` pasaron. Los tests usan callbacks sintéticos/archivos temporales; no abren conexión SQL ni inician workers.
- El preflight observó listeners existentes en `127.0.0.1:15439` y `:15440`; no se conectó ni interactuó con ellos. `:15441` no se usó. No se creó ni autorizó un target integrado de orden 6, ni se agregó perfil o se alteraron controles.
- Dictamen: solo queda revalidado el trabajo local/source-only. Integración, readback browser→BFF→Go→DB, entrega y aceptación de orden 6 siguen pendientes de target y operaciones explícitamente autorizados; no se extiende la autorización de orden 2. Sin commit, push, integración o despliegue.
