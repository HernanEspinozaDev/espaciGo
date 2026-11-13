# Orden 4 — CTA a la ficha pública (cobertura local)

- Fecha: 2026-11-11.
- Worktree: `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`.
- Rama: `codex/frontend/fix/space-01-create-postcommit-recovery`.
- HEAD base inspeccionado: `647cd491985d619370990d3f226b5df61e36b42b`; cambios locales conservados, sin commit.
- Alcance: permitir que el anfitrión abra la ruta de ficha pública desde la tarjeta del listado y el editor, únicamente cuando el estado recibido es `activa`.

## Cambio y regresión

- `frontend/src/features/host-listings/ui/host-listings-list.tsx`: las tarjetas activas muestran `Ver publicación` con enlace `/espacios/{id}` y nombre accesible contextual al título. Borradores y archivados no reciben este CTA; la acción `Archivar` se conserva.
- `frontend/src/features/host-listings/ui/listing-editor.tsx`: después de que el estado mostrado cambia a `activa`, el editor presenta `Ver publicación` hacia `/espacios/{id}`. En borrador no se muestra; `Archivar` permanece disponible.
- `frontend/tests/phase7-host-listings.spec.ts`: las regresiones comprueban la ausencia del enlace para borrador, el destino/visibilidad en tarjeta activa, y el destino tras publicación simulada en el editor, además de la acción de archivo.
- TDD: el caso del editor falló antes de la implementación porque no existía el enlace después del `PUT` mock de publicación; el caso del listado falló porque no existía el enlace en una tarjeta activa. Ambos pasaron con la implementación y un build nuevo.

## Verificación local

- `api:check`: PASS, inventario de 200 operaciones.
- `test`: 481 casos; 480 passed, 0 failed, 1 opt-in skipped.
- `typecheck`: PASS.
- `lint`: PASS, 0 errores y 4 advertencias existentes en otros archivos.
- `format:check`: PASS.
- `build`: PASS (Next.js 16.4.3).
- Playwright posterior al build: 2 passed (listado con filtros/estados y editor hasta publicación); la prueba de editor conserva los checks de contenido, tarifas, horario, galería y publicación de su escenario controlado.
- `test:e2e:public-calendar`: **4 passed** sobre el fixture sintético loopback definido en `tooling/public-calendar-fixture.mjs` (sin DB ni Go). Comprueba ficha pública con horario semanal read-only a 1365×900 y 390×844, estados de horario ausente/inactivo, y que marcadores privados sintéticos presentes en la respuesta del fixture no aparecen en el HTML SSR. Para el 404 de ID no público, toma baseline de observaciones (0 lecturas), comprueba incremento posterior, status 404 y el ID solicitado; también afirma ausencia de título/horario residual. Verifica ausencia de escrituras, Axe sin violaciones en ambas vistas y click-through desde la tarjeta activa del anfitrión hasta la ficha SSR fixture. No acredita política de la API real.
- `host-listings-publication-cas.spec.ts`: **1 passed**; la regresión controlada comprueba el enlace mientras el espacio está activo, su desaparición tras archivar y que no reaparece tras la reactivación fallida/lectura no confirmada. Las transiciones son mocks, no operaciones persistidas.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...`: **PASS** en ambos paquetes. Son pruebas locales del servicio/handler con repositorios de memoria; no conectan una base ni acreditan elegibilidad real de publicación.
- Las comprobaciones `typecheck`, lint global y `format:check` pasaron antes del último cambio, que solo modifica este test `.mjs`; después, el test unitario focal pasó 4/4, `node_modules/.bin/eslint` sobre el archivo dio 0 errores/1 advertencia preexistente (`queryPath`), Prettier pasó y `git diff --check` pasó.
- En una corrida focal separada, las cuatro suites de galería pasaron 12/12.
- Revisión independiente del editor (`deleg_3a1cd8c5`) y revisión combinada listado+editor (`deleg_e41d7adc`): **PASS**, sin hallazgos de seguridad o lógica; el revisor combinado verificó estado `activa`, destino, accesibilidad, ausencia en borrador y preservación de `Archivar`. Ambas fueron revisiones de fuente; no ejecutaron pruebas ni acreditan efectividad online.
- Revisión independiente de las nuevas regresiones de click-through y visibilidad (`deleg_e3f87564`): **PASS**, sin bloqueantes de seguridad o lógica. Confirmó que el test de navegación hace click y llega a la ficha SSR fixture, y que la regresión CAS comprueba ausencia del enlace tras archivar y reactivación no confirmada. Sugerencias no bloqueantes: registrar/afirmar más explícitamente los estados del fixture y mantener claro que el resumen de listado es sintético, no validado por schema; ninguna de ambas pruebas demuestra denegación de ruta pública ni estado persistido.
- Revisión read-only inicial del 404 (`deleg_d17cb8f4`): **PASS**, sin bloqueantes; sugirió identificar explícitamente el caso en el fixture y comprobar que el SSR consumió el 404. Se atendieron ambas sugerencias con una respuesta explícita para el ID sintético no público y un contador/status consultable solo en el fixture loopback. RED/GREEN del canal: antes del endpoint de observaciones, la prueba falló al recibir 404 en vez de 200 del endpoint diagnóstico; con el endpoint y el caso explícito, la prueba focal pasó 1/1 y la suite completa 4/4.
- Privacidad de proyección pública: el contrato OpenAPI de `PublicSpace`/`PublicSpaceDetail` declara `additionalProperties: false`; el adaptador copia campos allowlist distintos para listado/detalle y sanea características, imágenes, verificación y horario semanal anidados. Las regresiones unitarias afirman conjuntos exactos de claves, preservación de campos incluidos en el fixture, exclusión de marcadores privados y fallbacks ante valores anidados malformados; TDD RED previo al adaptador: `PRIVATE_OWNER_ID_MARKER leaked`. `node --test tooling/public-verification-refresh.test.mjs` — 4/4 en la corrida original; el nuevo seguimiento dinámico se detalla abajo. La lista exacta cubre las propiedades opcionales declaradas (región/comuna y `cover_image_id` solo en listado), preserva sus valores cuando están presentes y comprueba que se omiten cuando faltan. La comparación schema-driven de nombres de propiedades ya está añadida; OpenAPI no declara reglas semánticas de zona IANA ni orden de periodos.
- Revisión estática de `internal/publiccatalog/domain.go:57-85` y `internal/adapters/postgres/publiccatalog/repository.go:381-420` confirma que Go define una proyección pública explícita y que el detalle usa el filtro SQL de elegibilidad vigente. Es lectura de fuente, no ejecución Go/DB ni readback.
- Corrida posterior a la ampliación de regresiones: `api:check` — 200 operaciones; `test` — 483 casos (482 pasaron, 0 fallaron, 1 opt-in omitido); `typecheck` — PASS; `lint` — 0 errores y 4 advertencias preexistentes; `format:check` — PASS. El build Next.js 16.4.3 del código de producción, sin cambios posteriores, pasó; la última `test:e2e:public-calendar` sobre ese build — 4/4.
- Review del adaptador (`deleg_735e81ca`): **FAIL** por un gap de cobertura, sin security concerns; se añadieron assertions de claves exactas y casos malformados, sin cambiar código de producción. La revisión read-only `deleg_618c28a2` (tarea 2) aprobó el adaptador sin bloqueantes y sugirió cobertura schema-driven y límites semánticos; esa revisión precede a las nuevas assertions. La review de cobertura (`deleg_70e2f3bb`) devolvió **FAIL** por la ausencia en el fixture/expected keys de los opcionales OpenAPI `region_code`, `region_name`, `comuna_code`, `comuna_name` y `cover_image_id`; el adaptador ya los permitía. Se añadieron valores sintéticos válidos, expectativas exactas list/detail (incluido `cover_image_id` solo en listado) y comprobación de omisión cuando no vienen, sin cambiar producción; la suite unitaria completa quedó 482/483 (1 opt-in omitido), el archivo enfocado 4/4. La observación sobre orden horario/zona IANA no está respaldada como requisito por el OpenAPI inspeccionado y se deja como sugerencia de dominio, no se cambia el adaptador. La re-review read-only de cobertura (`deleg_7cc32013`) dio **PASS**, sin security concerns ni logic errors: confirmó que los conjuntos exactos coinciden con `PublicSpace`/`PublicSpaceDetail` de OpenAPI, que los campos opcionales de ubicación se conservan en ambos DTOs, `cover_image_id` solo en listado y que los valores sintéticos respetan tipos/formatos; no encontró falso positivo ni fuga.
- Re-review read-only anterior del 404 (`deleg_07ab2200`): **FAIL** (sin security concerns; 2 logic_errors informados): posible divergencia de puerto y contador acumulativo. La primera objeción no se reproduce en el config actual: el child del fixture hereda el mismo entorno leído por el test, `reuseExistingServer:false` exige servidor nuevo y `fullyParallel:false` serializa casos. Para la segunda, se añadió baseline cero, delta de lecturas y aserción del ID observado; también se añadieron marcadores privados al detalle sintético y se comprueba su ausencia en el HTML SSR. La revisión read-only `deleg_618c28a2` (tarea 1) aprobó la versión anterior sin bloqueantes y recomendó estas correlaciones; su sugerencia del CTA mock queda expresamente como flujo UI/SSR sintético, no autenticación real. La revisión final del fixture/SSR (`deleg_0d1bcd41`) dio **PASS**, sin bloqueantes: baseline 0, navegación 404, incremento de lectura con mismo ID/estado 404, y marcadores presentes en respuesta sintética upstream pero ausentes del HTML SSR. Sugirió preservar la limitación de observaciones en memoria y evitar inferir privacidad del backend real; la evidencia queda explícitamente acotada al fixture aislado/local. El PASS `deleg_d17cb8f4` cubre una versión anterior; no se extrapola a la final.

## Revalidación de marcadores privados tras hidratación — 2026-11-11

- `tests/public-weekly-calendar.spec.ts` ahora vuelve a buscar los marcadores privados en `document.documentElement.innerHTML` después de una interacción que confirma la hidratación del calendario, en escritorio y móvil.
- `npx --yes pnpm@12.15.0 test:e2e:public-calendar` — **4/4 aprobadas**, incluidos el flujo actualizado, 404 correlacionado y Axe responsive.
- Se añadió una aserción visible `Publicado` al recorrido del CTA, alineada con el fixture explícito `state: "activa"`. RED/GREEN: al variar solo el estado sintético a `borrador`, falló la aserción de badge; al restaurar `activa`, el caso focal pasó **1/1** y la suite **4/4**. Atiende la sugerencia no bloqueante de `deleg_e3f87564`; el fixture se tipa estáticamente en la sección siguiente, pero no se valida en runtime ni demuestra el DTO real.
- El mock de `GET /spaces` se completó y tipó con los tipos generados `SpaceDraft` y `SpaceDraftList`. RED de `typecheck` mostró que faltaban `description`, `usage_rules`, `address`, `attribute_schema_version` y `attributes`; al completar el fixture contrato-shaped, `typecheck` pasó. Esto comprueba tipos/campos requeridos estáticamente, no restricciones runtime como `minLength` o patrones OpenAPI.
- `tooling/public-verification-refresh.test.mjs` ahora deriva con el parser YAML las propiedades de `PublicSpace` y `PublicSpaceDetail` desde `planning/openapi.yaml` y compara con ellas las claves emitidas por cada adaptador, conservando también los snapshots explícitos y las pruebas de campos anidados.
- Sensibilidad RED/GREEN: al quitar temporalmente `comuna_name` del allowlist de listado, la aserción dinámica falló con el nombre faltante; restaurado el adaptador, `node --test tooling/public-verification-refresh.test.mjs` pasó **7/7** y la suite `node --test tooling/*.test.mjs` pasó **488/489** (1 opt-in omitida). ESLint focal: 0 errores/1 warning `queryPath`; Prettier PASS.
- Esta comparación detecta faltas/extras de claves respecto del OpenAPI vigente, no valida valores JSON Schema, restricciones de dominio, autorización o respuesta real.
- Se usaron únicamente el fixture loopback en `127.0.0.1:18088` y Next standalone en `127.0.0.1:4207`; no se inició Go/API ni se consultó DB/GCS. Es evidencia de SSR/hidratación con respuesta sintética, no de privacidad de la API real.
- Sin cambios productivos ni mutaciones persistentes.

## Límites y estado

- Solo evidencia local con BFF/fixture sintéticos. Los marcadores privados solo existen en respuestas sintéticas y su ausencia en el HTML verifica el límite del adaptador SSR, no la privacidad del API real. El baseline/delta y el ID observado confirman la rama 404 del fixture durante la suite; un proceso local externo que accediese deliberadamente al mismo puerto/ID aún podría contaminar el contador. No se consulta una fila de borrador ni se demuestra que Go aplique elegibilidad/ownership. Los cambios de publicación/CAS son mocks; el DTO resumen del listado no tiene validación runtime. No se consultó base de datos o almacenamiento ni se probó persistencia/readback.
- No se alteraron migraciones, bases persistentes, controles de seguridad, cuentas, recursos persistidos, `main` ni despliegue.
- Orden 4 sigue **parcial**: la proyección allowlist del adaptador y el no-renderizado SSR de marcadores desconocidos se probaron localmente; faltan elegibilidad, CAS/visibilidad y privacidad en una respuesta autorizada real, autorización backend y lectura pública en un target integrado aislado. La navegación SSR/transiciones visibles y el login del CTA usan mocks/fixtures. Los gates integrados de órdenes 1–3 también siguen pendientes.

## Revalidación focal posterior — 2026-11-11

- `npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **4/4** con fixture loopback y standalone local, incluido click-through desde CTA de anfitrión al detalle SSR, 404 correlacionado, privacidad de marcadores y Axe responsive.
- La última corrida `pnpm check` también incluyó el caso de CAS de publicación del listado (**1 passed**) y el recorrido del editor hasta publicación (**1 passed**); el recorrido de galería/portada en ese editor pasó por separado **1/1**.
- Todo continúa en fixtures/BFF mocks; no se accedió a Go/API real, bases ni GCS. Esta revalidación no cambia el gate integrado parcial ni acredita elegibilidad/privacidad real.

## Validación de referencia de portada pública — 2026-11-11

- Se encontró que `publicCoverUrl` rechazaba un `space_id` inválido, pero podía construir la ruta BFF de imagen con un `cover_image_id` no UUID cuando faltaba la referencia correspondiente en `images`.
- TDD: la regresión falló primero mostrando la ruta `/api/backend/public/spaces/{spaceId}/images/not-a-uuid`; tras exigir UUID válido para espacio y portada, `publicCoverUrl` devuelve `null` para la referencia inválida y conserva la ruta para un UUID válido sin referencia en la galería.
- `node --test tooling/public-catalog-domain.test.mjs`: **7/7**. La suite `test:e2e:public-calendar`: **4/4**, sobre el build local nuevo; mantiene click-through CTA→ficha SSR, 404 no público, privacidad de marcadores y Axe responsive.
- `pnpm check` consolidado: **exit 0**; OpenAPI 200 operaciones, ESLint 0 errores/4 warnings, typecheck, unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js 16.4.3 **32/32** y Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**.
- Cambio local en `src/features/public-catalog/domain/media.ts`, probado sobre HEAD `647cd491985d619370990d3f226b5df61e36b42b` con datos sintéticos. No se inició Go/API ni se accedió a DB/GCS; no acredita enforcement, elegibilidad o privacidad del backend real. Orden 4 permanece parcial en su gate integrado.

## Validación defensiva de zona IANA en horario público — 2026-11-11

- En `publicWeeklySchedule`, el adaptador solo comprobaba que `time_zone` fuera string, por lo que proyectaba horarios con identificadores imposibles como `../UTC`. Se añadió una regresión contra el adaptador que exige omitir el horario inválido; RED observado antes del cambio (`weekly_schedule` seguía presente).
- El adaptador ahora acepta solo strings no vacíos/sin whitespace externo que `Intl.DateTimeFormat` reconoce como `timeZone`; conserva el texto exacto y omite el horario completo si la zona es inválida. No impone una allowlist de nombres ni normaliza aliases aceptados por `Intl`.
- `node --test tooling/public-verification-refresh.test.mjs`: **8/8**. Typecheck, Prettier focal y `pnpm check` consolidado: exit 0. Corrida consolidada: OpenAPI 200 operaciones, unitarias **493 (492 aprobadas/0 fallidas/1 opt-in omitida)**, build Next.js 16.4.3 **32/32**, Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**; ESLint 0 errores/4 warnings. La corrida separada posterior de calendario público pasó **4/4**.
- Solo datos sintéticos y BFF fixtures; no hubo Go/API real, DB, GCS, ni prueba de persistencia. Orden 4 sigue parcial por integración/autorización/elegibilidad reales.
- Revisión independiente read-only `deleg_b9853dff`: **PASS**, sin observaciones de seguridad ni errores lógicos; confirmó que la validación coincide con la validación IANA local basada en `Intl` y que el adaptador omite horarios malformados. No ejecutó pruebas; el GREEN aquí registrado corresponde a las corridas parentales.

## Regresión visible para horario con zona inválida — 2026-11-11

- El fixture loopback ahora sirve un detalle sintético con estructura de horario válida y `time_zone: "../UTC"`. La prueba browser navega a la ficha SSR y verifica `Horario no informado.`, ausencia del encabezado de horario semanal/periodo inválido y cero escrituras.
- `npx --yes pnpm@12.15.0 test:e2e:public-calendar`: **5/5**. Prettier focal, ESLint focal y `pnpm typecheck`: PASS. No hubo cambio productivo en esta rebanada; Playwright usó el build local actualizado tras la validación IANA.
- Esto acredita la presentación visible para un fixture sintético, no el filtrado del backend real ni persistencia. Orden 4 permanece parcial por los gates integrados de elegibilidad, autorización y privacidad.
- Compatibilidad positiva complementaria: se verificó en runtime que `Intl.DateTimeFormat` acepta `EST`; el fixture con ese alias conserva el horario y la ficha SSR lo renderiza. El fixture invoca el mismo adaptador de catálogo que procesa la respuesta pública. Suite browser final **6/6**, incluyendo fallback inválido y alias aceptado; no hay allowlist ni normalización local añadida.
- La regresión del alias comprueba además los períodos exactos visibles `14:15` y `16:45` tras elegir martes. Su primer intento no halló los campos, ya que la UI los monta al seleccionar el día; se corrigió solo la secuencia del test y la reejecución de la suite pasó **6/6**.
- Tras el build consolidado final, `test:e2e:public-calendar` se repitió con el mismo resultado **6/6** en el artefacto vigente.
- La prueba del alias verifica también el texto exacto de zona expuesto por la UI (`Zona horaria: EST`) junto con los períodos. Tras añadir esta aserción, calendario público volvió a **6/6**; typecheck, ESLint focal y Prettier focal pasaron. Solo se modificó el test, sin rebuild requerido.
- Revisión independiente read-only `deleg_1dccfe61`: **PASS**, sin bloqueantes de seguridad/lógica; confirmó que ambos IDs recorren la ficha SSR y su adaptador. Sugerencias no bloqueantes para asegurar `aria-pressed` y ausencia de escrituras en el escenario alias ya fueron incorporadas; la reejecución posterior quedó **6/6**. El reviewer no ejecutó pruebas; el GREEN aquí registrado corresponde a las corridas parentales.

## CTA del editor tras archivar — 2026-11-11

- Se amplió la navegación controlada del editor en `phase7-host-listings.spec.ts`: tras publicar y comprobar el enlace `Ver publicación`, el flujo archiva el mismo espacio mediante el PUT mock de publicación con estado `oculta`.
- La prueba exige respuesta visible de archivado, badge `Archivado` y desaparición del enlace público del editor. Esto cubre la transición visible del CTA ante la respuesta de estado, además de la cobertura previa del listado/CAS.
- Playwright focal del editor: **1/1**; Prettier, ESLint focal, typecheck y `git diff --check` PASS. El standalone existente se reutilizó porque solo cambió el test.
- Transición BFF mock; no prueba elegibilidad, CAS ni persistencia backend. No se inició Go/API ni se accedió a base/GCS; el orden 4 continúa parcial por sus gates integrados.

## Click-through público desde el editor — 2026-11-11

- Se amplió `public-weekly-calendar.spec.ts`: tras llegar desde el listado a la ficha SSR, el caso abre directamente la ruta del editor activo como precondición, verifica el enlace `Ver publicación`, lo pulsa y comprueba la navegación al título y calendario de la ficha pública. Así se prueba el click del CTA del editor, aunque no el trayecto UI hacia el editor.
- El BFF mock responde `GET /spaces/{id}` con el `SpaceDraft` activo y ETag sintético requerido por el adaptador. Después de la revisión independiente, la prueba amplió el contador: registra todos los métodos de escritura bajo `/api/backend/`, exceptuando el `POST /api/backend/auth/login` sintético esperado; la aserción final exige cero mutaciones BFF restantes.
- RED inicial: no había respuesta mock para el detalle del editor y no aparecía el CTA; se añadió únicamente el mock contract-shaped, sin cambios de producto. Tras reforzar la aserción de mutaciones, la suite pública completa pasó **6/6**, incluido este escenario. Prettier, ESLint focal, typecheck y `git diff --check` PASS.
- Es navegación real de UI contra Next standalone local y datos fixture/BFF mock; no demuestra elegibilidad, autorización, privacidad o persistencia Go/DB. Se reutilizó el build existente porque solo cambió la prueba; no se inició backend ni se accedió a DB/GCS.

## Cambio de ficha activa a 404 — 2026-11-11

- Se amplió el caso de detalle no disponible para navegar primero a una ficha activa y confirmar título y horario presentes; después navega a un espacio no público y mantiene las aserciones de HTTP 404, título/horario ausentes e identificación correlacionada del lookup en el fixture.
- Es una caracterización browser de navegación secuencial en Next standalone y fixture local, no una respuesta autorizada de Go ni prueba de cache/visibilidad persistida.
- `npx --yes pnpm@12.15.0 test:e2e:public-calendar` — **6/6 aprobadas**; Prettier y ESLint focal PASS. No cambió código de producción ni se requirió rebuild.
- Sin API/DB/GCS/Cloud SQL ni mutaciones persistentes. Orden 4 continúa parcial para elegibilidad, CAS, privacidad y autorización reales; el gate integrado requiere target aislado autorizado.

## Reejecución focal de navegación pública — 2026-11-11

- Con los puertos locales `4207` y `18088` libres, se ejecutó `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar`; **6/6 aprobadas** sobre standalone local.
- Revalida ficha pública minimizada, horarios/zonas, transición activa→404 sin contenido residual y click-through del CTA. Fixture y BFF mocks únicamente; no demuestra autorización, privacidad ni elegibilidad Go/DB.

## Archivar desde editor y confirmar 404 público — 2026-11-11

- `tests/public-weekly-calendar.spec.ts` amplía el recorrido activo: desde el editor se pulsa `Archivar`; el BFF mock registra `PUT /spaces/{id}/publication` con `state: "oculta"` e `If-Match: "active-editor-v1"`, y el editor muestra confirmación y deja de ofrecer `Ver publicación`.
- El fixture local marca el ID como archivado; al abrir después la ficha pública, el SSR recibe 404. La prueba comprueba ausencia de título y horario residual, y correlaciona el ID/status con observaciones del fixture. Las escrituras BFF esperadas son únicamente login sintético y un PUT de publicación.
- RED inicial: `pnpm check` terminó exit 1 por timeout de 30 s en el test Axe de rutas de administración durante la corrida paralela; la prueba focal de ese test pasó aislada en **11.2 s**. Sin cambios posteriores, la consolidación repetida terminó exit 0. Esto no se atribuye al cambio de orden 4 ni se modificó el timeout de la prueba ajena.
- `test:e2e:public-calendar`: **6/6**. Es solo Next standalone, Chromium y fixture/BFF mocks; no acredita CAS, elegibilidad, autorización, privacidad o persistencia Go/DB real. Orden 4 sigue parcial en su gate integrado.
- No se inició API/Go/worker ni se accedió a DB/GCS/Cloud SQL; no hubo mutaciones persistentes, commit, push, integración ni despliegue.

## Catálogo público activo→archivado — 2026-11-11

- El recorrido mock de `tests/public-weekly-calendar.spec.ts` busca `Espacio sintético` en `/explorar`: confirma el enlace al espacio activo y observa el GET del catálogo con un resultado. Después archiva desde el editor, vuelve a consultar la búsqueda y confirma estado vacío; las observaciones del fixture verifican que ambas lecturas ocurrieron con la misma consulta y que el resultado final tiene cero elementos.
- `tooling/public-calendar-fixture.mjs` ahora modela `GET /api/v1/public/spaces?q=...` y omite el espacio tras su transición sintética a archivado. No cambia API real ni acredita persistencia o reglas Go/DB.
- La suite `test:e2e:public-calendar` pasó **6/6**; el caso `active owner listing CTA` pasó **1/1** después del ajuste final. Solo Chromium/Next standalone y fixtures/BFF mocks. Sin API, DB, GCS, credenciales, commit, push, integración ni despliegue.

## Revalidación focal tras continuar el plan — 2026-11-12

- `playwright test --config=playwright.public-calendar.config.ts` pasó **6/6** en Chromium local; `host-listings-publication-cas.spec.ts` pasó **1/1**, incluyendo publicación/archivo, CAS 412 y lectura fallida sin replay. `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/... -count=1` pasó los dos paquetes.
- La corrida consolidada de orden 5 del mismo árbol también pasó; su resultado completo queda en `evidence/espacios-orden5-verificacion-local-20261111.md`.
- No se inició API/worker ni se accedió a DB/GCS/Cloud SQL. Son pruebas BFF/fixture y Go offline; no demuestran elegibilidad, autorización, privacidad o persistencia reales. Orden 4 continúa parcial en el gate integrado.

## Elegibilidad de verificación: última decisión finalizada — 2026-11-12

- Se añadió `TestPublicVerificationSQLUsesLatestFinalizedDecision` en `internal/adapters/postgres/publiccatalog/testnuevoverification_sql_test.go`. La regresión exige para identidad y espacio la selección determinística de la última decisión entre `aprobada`, `revocada` y `vencida` (`ORDER BY updated_at DESC,id DESC LIMIT 1`), de modo que una decisión anterior aprobada no reaparezca por cambiar el orden o quitar la selección de la decisión vigente.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/adapters/postgres/publiccatalog -run 'TestPublicVerificationSQL' -count=1` — PASS. `gofmt -d` y `git diff --check` — PASS. Solo se modificó el test; no hay cambios de producción ni build requerido.
- Es una comprobación de la estructura del SQL fuente, no ejecuta el query ni demuestra elegibilidad efectiva en PostgreSQL, autorización, privacidad o readback. No se inició API/worker ni se accedió a DB/GCS/Cloud SQL. Orden 4 sigue parcial y su gate integrado continúa bloqueado por falta de target aislado autorizado.

## Revisión source-level del guard SQL — 2026-11-12

- La revisión señaló que buscar fragmentos sobre todo `repository.go` podría pasar por coincidencias ajenas al SQL usado por catálogo. `TestPublicVerificationSQLUsesLatestFinalizedDecision` ahora extrae específicamente el literal `eligiblePublicSQL`, exige que el literal se concatene en una consulta y busca dentro de esa expresión las decisiones finalizadas y el orden/desempate determinístico.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/adapters/postgres/publiccatalog -run '^TestPublicVerificationSQLUsesLatestFinalizedDecision$' -count=1` — PASS; `gofmt -d` — sin diferencias. Esto sigue siendo un guard de fuente: no parsea ni ejecuta SQL y no acredita elegibilidad efectiva, autorización, privacidad ni persistencia PostgreSQL.

## Variantes públicas: metadatos, tamaño y digest inválidos — 2026-11-12

- Se añadió `TestMediaImageContentRejectsInvalidMetadataSizeAndDigest` en `internal/publiccatalog/service_test.go`. Con el servicio real y repositorio/object store in-memory comprueba cinco rechazos: MIME no WebP, tamaño cero y superior al límite (cero lecturas de objeto), tamaño almacenado discordante y SHA-256 incorrecto (una lectura, sin devolver bytes); todos se exponen como `ErrNotFound`.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/... -count=1` y `go vet ./internal/publiccatalog/...` PASS; `gofmt -d` y `git diff --check` limpios.
- Es evidencia unitaria de servicio con fakes; no ejecuta SQL ni valida que PostgreSQL filtre elegibilidad, que la revocación sea efectiva en datos reales o que GCS/storage persista correctamente. Sin API/worker/DB/GCS ni cambios de producción/seguridad; orden 4 sigue parcial en su gate integrado.

## Galería pública de imágenes — revalidación local, 2026-11-12

- `pnpm build` terminó exit 0 con Next.js 16.4.3 y generó las 32 páginas estáticas. Sobre ese build, `pnpm test:e2e:public-gallery` pasó **1/1** en Chromium local: carga la ficha sintética, verifica las dos variantes `display` devueltas como `image/webp`/`no-store`, navega por ambas fotos y ejecuta Axe en escritorio y móvil; también exige cero escrituras BFF y ausencia de marcadores privados en el HTML SSR.
- La suite usa `playwright.public-gallery.config.ts`, un fixture loopback y Next standalone local. Los puertos `4208` y `18089` estaban libres antes de la corrida y quedaron libres al terminar; no se inició Go/API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL.
- Prettier focal, ESLint focal, `pnpm typecheck` y `node --test tooling/public-catalog-domain.test.mjs` (**7/7**) pasaron. El comando cubre las imágenes display de la ficha, no thumbnails de tarjetas del catálogo ni elegibilidad/autorización/privacidad del backend real o persistencia/readback. Orden 4 continúa parcial en el gate integrado; sin commit, push, integración, despliegue ni aceptación online.

## Tarjeta del catálogo público: thumbnail — 2026-11-12

- Se amplió `public-gallery-media.spec.ts` con un recorrido sintético desde `/explorar`: la tarjeta selecciona `thumbnail`, la imagen se carga con `image/webp` y `Cache-Control: no-store`, y el enlace conserva el ID de ficha; el navegador no emite escrituras. `pnpm test:e2e:public-gallery` pasó **2/2** en Chromium local: galería `display` de ficha y thumbnail de tarjeta.
- La fixture loopback sirve únicamente una ficha, categorías, ubicaciones y bytes de imagen sintéticos. Esto prueba representación/ruta BFF en el entorno local del fixture; no elegibilidad, autorización, ownership, privacidad backend, storage ni persistencia. Sin API/worker/DB/GCS/Cloud SQL, sin cambios de seguridad, y gate integrado del orden 4 sigue parcial.

## Transporte BFF de variante pública — 2026-11-12

- `frontend/tooling/backend-gateway-credentials.test.mjs` caracteriza `GET /public/spaces/{spaceID}/images/{imageID}/variants/thumbnail` con cookie de sesión sintética en la petición del navegador. El handler conserva el path permitido y transmite bytes `image/webp` exactos con `Cache-Control: no-store`, `credentials: omit` y `redirect: manual`; ni la cookie ni un Bearer de sesión se reenvían al upstream. La suite focal de gateway pasó **7/7**, incluida esta caracterización.
- El `fetch` upstream y la identidad Cloud Run están stubbeados; por tanto, esto prueba el límite de transporte de la ruta BFF en una prueba unitaria, no identidad de servicio real, elegibilidad, autorización Go, privacidad de datos, almacenamiento ni persistencia. No se cambió producción ni se inició API/worker/DB/GCS/Cloud SQL.

## Handler Go: ambas variantes públicas y revocación — 2026-11-12

- `TestPublicMediaImageIsNotCachedAndRevocationTakesEffect` envía solicitudes HTTP al handler para `display` y `thumbnail`; el repositorio in-memory devuelve datos solo para la variante exacta y el object store valida clave/generación. Ambas responden `image/webp`, bytes esperados, `Cache-Control: no-store`, ETag fuerte derivado del SHA-256 del fixture, `Content-Length` y `Content-Disposition: inline`. POST devuelve **405**, GET con query **404** y variante no permitida `avatar` **404**, todos antes de consultar repositorio/storage. Un desajuste sintético de generación devuelve 404 sin exponer bytes; tras la revocación, el GET revalida y responde 404 sin leer el objeto.
- `TestPublicMediaImageHidesRepositoryError` inyecta un error interno sintético: el handler devuelve 500 `internal_error` con `no-store`, omite el detalle sentinel y no consulta el object store.
- `TestPublicMediaImageHidesObjectStoreError` inyecta un error sintético del object store: el servicio lo normaliza a 404 `no-store`, sin filtrar el sentinel ni bytes de imagen; repositorio consultado una vez y object store leído una vez.
- `TestPublicMediaVariantCORSOriginPolicy` comprueba en la ruta binaria un preflight permitido sin lecturas, un GET permitido que devuelve WebP con `Access-Control-Allow-Origin` específico, `Vary: Origin` y sin credenciales CORS, y un GET de origen no confiable con 403 previo a repositorio/storage.
- Focal `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/transport/http -run '^TestPublicMedia(Image|VariantCORS)' -count=1 -v`: PASS, incluidos los subtests `display`/`thumbnail`. Después, `go test ./internal/publiccatalog/... ./internal/occupancy ./internal/spaces/transport/http ./internal/gallery/transport/http ./internal/gallery ./internal/media -count=1` y `go vet` del mismo conjunto pasaron. `gofmt` y `git diff --check HEAD` limpios.
- El handler/service usan repositorio y storage en memoria; esto no ejecuta SQL ni prueba elegibilidad/ownership/storage persistente. Sin API/worker, DB/GCS/Cloud SQL ni cambios productivos.

## Fallback visual de la ficha pública — 2026-11-12

- `public-gallery-media.spec.ts` añade un recorrido en Chromium donde la variante `display` sintética responde 404 y el endpoint legacy original devuelve 404 desde el fixture local. La UI presenta el texto accesible “Imagen no disponible”, oculta el control de zoom para la imagen fallida, conserva “Siguiente imagen”, permite cargar la segunda imagen y confirma que el placeholder desaparece; no se emiten solicitudes mutantes desde la página.
- Prueba focal PASS **1/1**; suite dedicada `pnpm exec playwright test --config=playwright.public-gallery.config.ts` PASS **3/3**. Se reutilizó el standalone local existente porque solo cambió el spec; puertos 4208/18089 estaban libres antes del arranque.
- Esto caracteriza el placeholder existente y continuidad del carrusel. No constituye aceptación visual de producto ni integración con BFF/Go/storage real.

## Fallback de tarjeta de catálogo — 2026-11-12

- Se amplió `public-gallery-media.spec.ts`: `thumbnail` recibe 404 sintético y el endpoint legacy original 404 desde el fixture; la tarjeta conserva el placeholder accesible y su enlace a la ficha sigue funcionando al navegar a `/espacios/{spaceID}`. No se emiten solicitudes mutantes desde la página.
- Revalidación focal de fallback de ficha y tarjeta PASS **2/2** (cada recorrido en escritorio y móvil); suite dedicada de galería PASS **4/4**. `pnpm lint` terminó con 0 errores/4 warnings existentes, `pnpm typecheck` y Prettier PASS.
- Axe-core focalizado en la región de galería con la imagen fallida y en la tarjeta con su placeholder reportó cero violaciones en escritorio **1365×900** y móvil **390×844**; ambas interacciones fallback se recorrieron en esos dos viewports. Por viewport se compara el conjunto deduplicado de pares método/ruta para variante + recurso legacy, exigiendo solo `GET` en esas rutas y permitiendo reintentos GET repetidos. Este contador observa solicitudes del navegador/página; el fixture no acredita efectos internos de servicios reales.
- Solo Playwright local con fixture/BFF; no prueba la respuesta real de Go/storage ni constituye aceptación de la decisión visual del placeholder.

## Fallback legacy exitoso de galería — 2026-11-12

- `frontend/tests/public-gallery-media.spec.ts` caracteriza `display 404 → recurso legacy 200`: el fixture responde bytes WebP sintéticos y la primera imagen alcanza `naturalWidth > 0`; el placeholder no aparece. En 1365×900 y 390×844 exige el conjunto deduplicado de pares método/ruta GET de variante + legacy, tolera GET repetidos, y comprueba cero solicitudes mutantes emitidas por la página y cero violaciones Axe en la galería.
- El caso focal pasó **1/1** recorriendo ambos viewports; la suite dedicada de galería pasó **5/5**. La consolidación final del mismo árbol y sus resultados están en `evidence/espacios-orden5-verificacion-local-20261111.md`.
- Evidencia exclusivamente local con bytes sintéticos: no prueba el fallback del servidor real, autorización/ownership, almacenamiento ni persistencia. No constituye aceptación visual del comportamiento.

## Fallback legacy exitoso de tarjeta pública — 2026-11-12

- `frontend/tests/public-gallery-media.spec.ts` caracteriza `thumbnail 404 → recurso legacy 200`: el GET de variante devuelve 404 y el endpoint original entrega WebP sintético 200. El test valida todas las respuestas observadas en cada viewport (thumbnail 404; original 200 `image/webp`), espera `naturalWidth > 0`, ausencia del placeholder y el conjunto deduplicado exacto de pares GET método/ruta.
- El caso recorre escritorio **1365×900** y móvil **390×844**, afirma cero solicitudes mutantes emitidas por la página, Axe sin violaciones en la tarjeta y el enlace a `/espacios/{id}`. Test focal **1/1**; la suite pública de galería tras la consolidación pasó **6/6**.
- Fixture local únicamente; no demuestra el fallback en BFF/Go/storage real, autorización, ownership ni persistencia. No es aceptación visual del producto.

## Orden 5 — consolidación local tras revisión de órdenes — 2026-11-12

- Sobre el worktree basado en HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `PLAYWRIGHT_PORT=4238 API_BACKEND_BASE_URL=http://127.0.0.1:18098/api/v1 npx --yes pnpm@12.15.0 check` terminó exit 0: OpenAPI **200 operaciones**, ESLint **0 errores/4 warnings**, typecheck, unitarias **505 (504 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js **32/32** y Playwright default **228 (165 aprobadas/63 omitidas/0 fallidas)**.
- Después del build, `pnpm exec playwright test --config=playwright.public-gallery.config.ts` pasó **6/6** y `pnpm exec playwright test --config=playwright.public-calendar.config.ts` pasó **6/6**. Ambos usan Next standalone local y fixtures loopback; los puertos de ambos harness estaban libres antes de la ejecución.
- No se inició Go/API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o servicios online. Estos gates son checks locales y mocks; no acreditan elegibilidad, autorización, privacidad ni persistencia integradas. No hubo cambios de producto, commit, push, integración o despliegue.
- El gate integrado permanece parcial y no se ejecutó: requiere un target aislado aceptado y autorización específica para los pasos protegidos que aún faltan. La ejecución de checks locales no sustituye esa autorización.

## Presentación pública de horas parciales — 2026-11-12

- Se amplió `frontend/tests/public-weekly-calendar.spec.ts` para el período fixture martes **14:15–16:45**: los tramos horarios de borde **14:00–15:00** y **16:00–17:00** se exponen como `aria-pressed="mixed"` y «Parcial», incluyendo el rango exacto en el nombre accesible; **15:00–16:00** se expone como completo sin etiqueta parcial.
- La regresión focal pasó **1/1**; `test:e2e:public-calendar` pasó **6/6** sobre Next standalone local y fixture/BFF sintético. `pnpm lint` terminó con **0 errores/4 warnings**, `pnpm typecheck` y `pnpm format:check` PASS.
- Test-only; no cambió presentación/producción. Los assertions caracterizan la interfaz del fixture, no la respuesta Go, autorización/privacidad integrada, persistencia ni disponibilidad real. No se inició API/worker ni se accedió a DB/GCS o servicios online; orden 4 integrado sigue parcial.

## Orden 4 — endurecimiento del adaptador público y revalidación — 2026-11-12

- El adaptador de catálogo copia campos de lista/detalle desde allowlists contractuales y limpia features, imágenes, resumen de verificación y horario semanal anidados; horarios con zona inválida o días/intervalos malformados, invertidos, vacíos o solapados se omiten sin alterar el orden de intervalos válidos. `publicCoverUrl` rechaza IDs de portada que no sean UUID.
- `node --test tooling/public-verification-refresh.test.mjs tooling/public-catalog-domain.test.mjs` pasó **15/15**. Go offline `go test ./internal/publiccatalog/... ./internal/adapters/postgres/publiccatalog -count=1`, `go vet` de esos paquetes y `gofmt -d` pasaron.
- Sobre el árbol local basado en HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `PLAYWRIGHT_PORT=4294 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 pnpm check` terminó **exit 0**: OpenAPI **200**, ESLint **0 errores/4 warnings**, typecheck, unitarias **507 total (506 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js 16.4.3 **32/32** y Playwright default **166 PASS/63 omitidas/0 fallidas**. Tras el build, calendario público **6/6** y galería pública **6/6**; `git diff --check HEAD` limpio.
- Solo pruebas locales con fixtures sintéticos y BFF; no se inició Go/API/worker, no se accedió a PostgreSQL/GCS/Cloud SQL ni a servicios online. Esto no demuestra la consulta SQL de elegibilidad, autorización/ownership reales, privacidad de datos reales ni persistencia/readback integrado. Orden 4 y su gate integrado siguen parciales; sin commit, push, integración o despliegue.

## Envelopes de éxito malformados del catálogo — 2026-11-13

- `frontend/tooling/public-verification-refresh.test.mjs` agrega cinco casos: raíz nula de listado, `items: null`, entrada nula de lista, `data: null` y raíz nula de detalle. La regresión observó RED antes de la protección del adaptador; después, cada respuesta HTTP 2xx malformada produce `{ kind: "unavailable", status: 502 }`, sin lanzar excepción ni aceptar una página parcial.
- `frontend/src/features/public-catalog/infrastructure/go-public-catalog-adapter.ts` valida la forma de raíz/envelope, que `items` sea una lista y que cada elemento sea un objeto antes de adaptarlo; detalle valida `data`. Las respuestas no exitosas conservan el resultado de error original.
- `PLAYWRIGHT_PORT=4294 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 pnpm check` terminó **exit 0**: API/OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **508 total (507 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright default **229 total (166 aprobadas, 63 omitidas, 0 fallidas)**. El test nuevo aparece en la corrida unitaria.
- Sobre el build recién generado, `playwright.public-calendar.config.ts` pasó **6/6** y `playwright.public-gallery.config.ts` **6/6** con Chromium, Next standalone y fixtures loopback. `git diff --check HEAD` pasó; los puertos locales `4294`, `18082`, `4207`, `18088`, `4208` y `18089` estaban libres después de las corridas.
- No se inició API/Go/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o servicios online. Esto sigue siendo cobertura local de adaptador/BFF/fixtures; no prueba elegibilidad, autorización/ownership, privacidad backend ni persistencia/readback. Orden 4 continúa parcial. Sin commit, push, integración a `main` o despliegue.

## Revalidación no mutante de orden 4 — 2026-11-13

- `playwright.public-calendar.config.ts`: **6/6 passed**; `playwright.public-gallery.config.ts`: **6/6 passed**. Chromium se ejecutó contra Next standalone y fixtures loopback aislados; calendario público, CTA/SSR, horarios informativos y fallback de imágenes quedaron cubiertos por sus suites vigentes.
- `node --test tooling/public-verification-refresh.test.mjs tooling/public-catalog-domain.test.mjs`: **16/16 passed**. Se ejercitaron allowlists/privacidad, envelopes 2xx malformados, horarios públicos e IANA.
- `env -i`, `GOPROXY=off`, `GOTOOLCHAIN=local`: `go test -race -count=1 ./internal/publiccatalog/...`, `go vet ./internal/publiccatalog/...` y el test fuente-only `TestPublicVerificationSQLUsesLatestFinalizedDecision` pasaron. `gofmt -l` no reportó archivos. Se excluyó expresamente el test de integración PostgreSQL que conecta a un target local; no hubo conexión DB.
- ESLint focal: 0 errores y 1 warning preexistente (`queryPath` asignado sin uso en `tooling/public-verification-refresh.test.mjs:53`); Prettier focal y `git diff --check HEAD` pasaron. No se cambió código productivo en esta revalidación.
- Sin API/worker, PostgreSQL, GCS, Cloud SQL ni servicios online. La evidencia es fixture/adapter/unitaria/source-only y no demuestra elegibilidad, autorización/ownership, privacidad backend ni readback/persistencia integrados. Orden 4 continúa parcial; no hereda la autorización del orden 2.

## Cancelación durante lectura de variante pública — 2026-11-13

- `TestMediaImageContentStopsAfterCancelledRepositoryLookup` usa repositorio y object store sintéticos. Con contexto ya cancelado, el repositorio fake devuelve `context.Canceled`; el servicio conserva el error, devuelve cero bytes y no invoca el object store.
- El test focal con `-race`, `go test -race -count=1 ./internal/publiccatalog/...`, `go vet ./internal/publiccatalog/...`, `gofmt` y `git diff --check HEAD` pasaron. Solo se modificó `internal/publiccatalog/service_test.go` y esta evidencia; no hay cambio productivo.
- Límite: caracteriza el traspaso de contexto y evita la lectura posterior usando fakes; no valida cancelación efectiva del adaptador PostgreSQL/storage real, elegibilidad, autorización ni persistencia. Sin API/worker, DB/GCS/Cloud SQL. Orden 4 integrado sigue parcial.

## Rechazo temprano de entradas inválidas para variantes públicas — 2026-11-13

- `TestMediaImageContentRejectsInvalidInputBeforeRepositoryOrObjectStore` prueba UUID de espacio/foto inválidos y variante no permitida; los tres casos devuelven `ErrNotFound` vacío sin consultar el repositorio ni leer el object store. `TestMediaImageContentRequiresPublicMediaRepositoryBeforeObjectRead` verifica el mismo cierre cuando el repositorio no implementa la interfaz de variantes públicas.
- Focal: ambos tests y cuatro escenarios pasaron. `go test -race -count=1 ./internal/publiccatalog ./internal/publiccatalog/transport/http` y `go vet` de ambos paquetes pasaron. `go test -coverprofile` midió `MediaImageContent` en **100%** de sentencias; `gofmt -l` y `git diff --check HEAD` limpios.
- Test-only fake-backed; no demuestra filtro SQL, autorización/elegibilidad real, GCS ni persistencia. Sin API/worker, DB, GCS o Cloud SQL; sin cambio productivo ni de controles. Orden 4 integrado sigue parcial.

## Error de lectura de variante con metadata parcial — caracterización local — 2026-11-13

- `TestMediaImageContentDiscardsMetadataReturnedWithRepositoryError` hace que el repositorio fake devuelva metadata sintética completa junto con un error. El servicio conserva el error, devuelve `PublicImage{}` y cero bytes, y no consulta el object store aunque la key/generation fake existan.
- El primer intento falló por fixture incompleto: el fake no implementaba el método `PublicImage` requerido por `PublicImageRepository`, así que el servicio rechazó el puerto antes de la llamada prevista. Tras completar la interfaz fake, el test focal pasó; `go test -race -count=1 ./internal/publiccatalog/...`, `go vet ./internal/publiccatalog/...`, `gofmt` y `git diff --check HEAD` pasaron.
- Es caracterización fake-backed de error más valor parcial; no prueba comportamiento del adaptador PostgreSQL/storage, elegibilidad, autorización o readback persistido. No hubo API/worker ni acceso a DB/GCS/Cloud SQL; orden 4 integrado sigue parcial.
