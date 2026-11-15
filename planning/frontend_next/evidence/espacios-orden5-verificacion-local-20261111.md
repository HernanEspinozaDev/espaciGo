# Orden 5 — revalidación local final, 2026-11-11

- Rama: `codex/frontend/fix/space-01-create-postcommit-recovery`.
- HEAD base: `647cd491985d619370990d3f226b5df61e36b42b`; la verificación cubrió el worktree local intencionalmente sucio, sin commit.
- Alcance: revalidación local no mutante de los cambios reunidos y de las dos correcciones de E2E descritas abajo. No se consultó ni modificó una base, API real, cuentas, GCS, Cloud SQL, `main` o despliegues.
- Entorno saneado con `env -i`; Node `v26.12.1`, pnpm `12.15.0`, Go `go1.27.1`. Go se ejecutó con `GOPROXY=off GOTOOLCHAIN=local`. Para Playwright controlado, `API_BACKEND_BASE_URL` apuntó solo a `http://127.0.0.1:18082/api/v1` (sin API escuchando) y Next usó el puerto loopback 4213. El calendario público usó su fixture loopback en 18088 y Next en 4207. Los puertos de prueba estaban libres al iniciar y quedaron liberados al terminar.

## Gates de código y build

`pnpm api:check`, `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...`, `pnpm test`, `pnpm typecheck`, `pnpm lint`, `pnpm format:check` y `pnpm build` terminaron con código 0. Resultados: OpenAPI vigente con 200 operaciones; unitarias **486 totales, 485 aprobadas, 0 fallidas y 1 opt-in omitida**; lint **0 errores y 4 warnings preexistentes**. La build incluyó el cambio semántico de encabezado de la ruta de verificaciones. No se generó código.

La primera secuencia saneada se detuvo antes de ejecutar Go porque su `PATH` no incluía `/usr/local/go/bin` (`go: command not found`, exit 127). Se corrigió únicamente el `PATH` y se volvió a ejecutar la secuencia completa; no fue un fallo de código.

## Playwright/Axe local

- Suites controladas relevantes de órdenes 2–4: **42/42 aprobadas** en 11 archivos (horario/bloqueos/CAS, unidades día/mes, cambio de cuenta, galería/cola/incertidumbre y `phase7-host-listings`, incluido Axe desktop/móvil).
- Fixture de ficha/calendario público: **4/4 aprobadas**, incluido el 404 correlacionado, click-through SSR y Axe responsive.
- La reejecución en puerto explícito reprodujo los dos fallos iniciales: el test esperaba la etiqueta técnica `Verificación KYC`, que no coincidía con los textos visibles de las tarjetas ni con la política de verificaciones; y `/mi-cuenta/verificaciones` exponía un segundo `h1` dentro del shell de cuenta. Se corrigió la prueba para comprobar los encabezados visibles de las tarjetas (`Verificación de usuario (Identidad)` y `Verificación de espacios`); esto demuestra el texto renderizado, no aprobación de producto del copy. Por separado, se cambió el título de esa subpágina a `h2`, preservando estilos y alineándolo con el `h1` del shell. El test de accesibilidad comprueba explícitamente el título `h2`, un único `h1`, overflow y Axe.
- RED/GREEN: la aserción del título `h2` falló antes del cambio productivo; después de `pnpm build`, ambos tests de `phase6-account.spec.ts` pasaron **2/2** (encabezados visibles de tarjetas/flujo sintético y rutas de cuenta con Axe desktop/móvil). La suite global `pnpm test:e2e` ejecutó **220 tests: 157 aprobados, 63 omitidos y 0 fallidos**. El omitido corresponde a gates opt-in, no se cuenta como integración.
- `pnpm test:e2e:public-calendar` sobre el mismo artefacto reconstruido terminó **4/4** con fixture loopback.

## Revalidación E2E tras ampliar Axe de galería — 2026-11-11

- Suite completa en `PLAYWRIGHT_PORT=4213` con `API_BACKEND_BASE_URL` loopback sin API escuchando: **220 tests, 157 aprobados, 63 omitidos y 0 fallidos**. Incluye la aserción Axe de galería poblada a 1280×800 y 390×844.
- Node/pnpm ejecutados mediante Node `v26.12.1` y pnpm fijado `12.15.0`. Los puertos de ensayo estaban libres antes y quedaron libres después; el recorrido sigue usando respuestas BFF mockeadas.
- No se reconstruyó Next: la única fuente cambiada después de la build fueron tests/documentación, no código de aplicación. No se inició Go/API ni se accedió a DB/GCS.

## Límites y estado

- Orden 5 **parcial**: los gates locales de código/build y Playwright/Axe pasan, incluida la suite global; permanece bloqueado solo el recorrido integrado browser→BFF→Go→DB.
- Sigue bloqueado el recorrido integrado browser→BFF→Go→DB: no existe target integrado aislado autorizado disponible. No se sustituyó por la base persistente compartida, no se inició API/worker y no se hicieron escrituras.
- Estas pruebas no demuestran ownership, autorización, publicación/elegibilidad, CAS backend, privacidad de respuestas reales, almacenamiento GCS ni persistencia/readback.
- No hubo commit, push, integración a `main`, despliegue ni aceptación online.
- Siguiente acción posible dentro del plan: mantener cerrado el gate integrado hasta disponer de un target aislado autorizado y compatible con la política de datos. No se usará la base persistente compartida ni se convertirá evidencia mock en aceptación de producto.

## Corrida consolidada posterior — 2026-11-11

- Comando: `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` desde `frontend/`. Resultado **exit 0**: API check 200 operaciones; lint 0 errores/4 warnings; typecheck; unitarias **489 total, 488 aprobadas, 0 fallidas, 1 opt-in omitida**; formato; build; Playwright completo **220 total, 157 aprobados, 63 omitidos, 0 fallidos**.
- Después del build consolidado, `npx --yes pnpm@12.15.0 test:e2e:public-calendar` terminó **4/4**. La prueba vuelve a comprobar ausencia de marcadores privados tras interacción/hidratación en ambas dimensiones de viewport.
- El target API configurado en el ensayo (`127.0.0.1:18082`) no tenía listener; todo el recorrido fue Playwright/BFF mock. Los puertos 4213, 18082, 4207 y 18088 quedaron libres al finalizar; no se inició Go/API ni se accedió a DB/GCS.
- Esta corrida cierra los gates locales no mutantes reunidos del orden 5, no el gate browser→BFF→Go→DB. No se modificó código productivo, no hubo commit/push, integración, despliegue ni aceptación online.

## Corrida posterior a la comparación schema-driven — 2026-11-11

- Tras añadir en `tooling/public-verification-refresh.test.mjs` la comparación dinámica de claves con `PublicSpace`/`PublicSpaceDetail`, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: API check **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck PASS; unitarias **489 total, 488 aprobadas, 0 fallidas y 1 opt-in omitida**; Prettier PASS; build Next.js **16.4.3** compilado y estáticas **32/32** generadas; Playwright **220 total, 157 aprobadas, 63 omitidas y 0 fallidas**.
- `npx --yes pnpm@12.15.0 test:e2e:public-calendar` volvió a pasar **4/4** sobre el standalone recién construido, con fixture sintética loopback `18088` y Next `4207`. Los puertos `4213`, `18082`, `4207` y `18088` estaban libres antes y quedaron libres después.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` pasó para ambos paquetes (caché). No se inició API/worker ni se consultó o modificó base, Cloud SQL o GCS. El API configurado en `18082` no tuvo listener; el recorrido E2E fue controlado/mock.
- Esta revalidación local no mutante incorpora la nueva regresión, pero no acredita el gate browser→BFF→Go→DB, persistencia/readback ni autorización real. Sin cambios productivos permanentes, commit, push, integración, despliegue o aceptación online.

## Corrida final reunida tras continuar órdenes 2–5 — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` — **exit 0** sobre el artefacto actual: OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **489 total, 488 aprobadas, 0 fallidas, 1 opt-in omitida**; Prettier; build; Playwright **220 total, 157 aprobadas, 63 omitidas, 0 fallidas**.
- Tras esa build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **4/4**. Las cuatro suites de galería contract-guard/queue/uncertain/forbidden-purge también pasaron **12/12** en Chromium local.
- `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` pasó ambos paquetes desde caché. No se inició API/worker; los puertos API `18082`/`18088` se reservaron únicamente para fixtures mock/loopback; ninguna base, GCS o Cloud SQL fue consultada o modificada.
- No hubo commit, push, integración a `main`, despliegue ni aceptación online. Siguen pendientes los gates integrados de órdenes 1–5 hasta disponer de un target aislado autorizado.

## Revalidación local después de revisión — 2026-11-11

- La revisión de cambios posteriores amplió el contrato de fotos `ready` y ajustó el botón de bloqueos para zona IANA inválida. En la ejecución de `pnpm check`, API/OpenAPI (200 operaciones), ESLint (0 errores/4 warnings), typecheck, unitarias (**492 total, 491 aprobadas, 0 fallidas, 1 opt-in omitida**), Prettier y build pasaron; la etapa E2E terminó con un fallo de fixture en el hash sintético de `phase7-host-listings.spec.ts`, no en el código de la aplicación.
- Se corrigió el generador del fixture para producir SHA-256 hexadecimal de 64 caracteres. La prueba afectada pasó aislada (**1/1**), `pnpm typecheck` pasó y la reejecución `pnpm test:e2e` terminó **220 tests: 157 aprobados, 63 omitidos, 0 fallidos**. Luego, la corrida consolidada `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: API 200 operaciones, ESLint 0 errores/4 warnings, typecheck, unitarias **492 total (491 aprobadas, 0 fallidas, 1 opt-in omitida)**, Prettier, build Next.js 16.4.3 con páginas estáticas 32/32 y Playwright **220 (157 aprobados, 63 omitidos, 0 fallidos)**.
- La ejecución fue Playwright local con BFF mock. No demuestra API/Go/DB/GCS ni readback persistido y no cierra el gate integrado; no hubo commit, push, integración, despliegue ni aceptación de producto.

## Revalidación del orden 5 tras avanzar órdenes 2–4 — 2026-11-11

- La última corrida consolidada sobre el código actual (`pnpm check`) terminó **exit 0**: OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; Next.js 16.4.3 con 32/32 páginas; Playwright **220 (157 aprobados/63 omitidos/0 fallidos)**.
- Después de esa build: galería **12/12**, recorrido del editor con fotos **1/1**, calendario público CTA/SSR **4/4**. La suite consolidada incluyó CAS de publicación del listado **1/1** y el recorrido del editor hasta publicar **1/1**.
- `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` terminó **exit 0** para ambos paquetes (caché); la inspección del paquete no encontró clientes SQL/PGX. No se inició API/worker ni se abrió conexión a bases.
- Verificación de solo lectura de puertos `4213`, `18082`, `4207`, `18088`: libres al iniciar y al terminar las suites locales. Todo sigue siendo mock/fixture; el gate integrado browser→BFF→Go→DB permanece parcial por falta de target aislado autorizado.

## Corrida consolidada después de ampliar orden 2 — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI 200 operaciones, ESLint 0 errores/4 warnings, typecheck, unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js 16.4.3 y Playwright **220 (157 aprobados/63 omitidos/0 fallidos)**.
- La suite ahora incluye el cambio mockeado de zona `America/Santiago`→`America/Coyhaique`, cadena ETag de zona a horario, reload/readback y CAS 412 sin replay; focal `weekly-schedule-editor.spec.ts` **1/1**.
- El calendario de bloqueos por fecha pasó dentro de la corrida consolidada (**1/1**). El target API loopback no tenía listener; no se inició Go/API/worker ni se consultó o modificó ninguna base o almacenamiento.

## Corrida posterior a la recuperación de refresh del orden 2 — 2026-11-11

- Corrida `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check`: **exit 0**; OpenAPI 200 operaciones; lint 0 errores/4 warnings; typecheck; unitarias **492 (491 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3; Playwright **220 (157 aprobados, 63 omitidos, 0 fallidos)**.
- La prueba de bloqueos incluye 503 temporal en refresh, mensaje seguro, conservación del último resultado confirmado, reintento 200 y reconciliación de una fila externa sintética tras GET y reload; ningún POST/PUT/DELETE adicional. Pasó focal **1/1** y en la suite consolidada.
- Todo fue Chromium/standalone con BFF mocks. Los puertos de ensayo `4213` y `18082` quedaron libres; no se inició API/worker ni se accedió a datos persistentes.

## Revalidación de continuación — 2026-11-11

- `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` terminó exit 0 para ambos paquetes desde caché. La inspección de las pruebas confirma repositorios/fakes y `httptest`; no se inició Go/API ni se abrió conexión a PostgreSQL.
- Desde `frontend/`, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**, ESLint **0 errores/4 warnings**, typecheck, unitarias **492 (491 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js **16.4.3 con 32/32 páginas** y Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**.
- Tras esa build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` terminó **4/4** con fixtures loopback.
- `git diff --check` terminó sin errores. Al finalizar, los puertos `4213`, `18082`, `4207` y `18088` quedaron libres. Se observó un listener preexistente en `127.0.0.1:5432`; no se conectó, inspeccionó ni modificó.
- Orden 5 permanece **parcial únicamente** por el gate integrado browser→BFF→Go→DB y readback persistido: no hay target aislado autorizado disponible. Todo lo ejecutado fue local/no mutante; no hubo API/worker, acceso a base/GCS/Cloud SQL, commit, push, integración ni despliegue.

## Corrida consolidada tras endurecer zona IANA del horario público — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` desde `frontend/`: **exit 0**. OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **493 total (492 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 con **32/32** páginas; Playwright **222 (159 aprobados, 63 omitidos, 0 fallidos)**.
- Después del build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **4/4** con fixture local. El test verifica privacidad de marcadores tras interacción/hidratación, pero solo sobre datos sintéticos.
- TDD del adaptador público: la regresión para `time_zone: "../UTC"` falló antes de la corrección y la suite `node --test tooling/public-verification-refresh.test.mjs` terminó **8/8** después. `typecheck`, Prettier focal, ESLint focal (0 errores/1 warning preexistente `queryPath`) y `git diff --check` pasaron.
- No se inició API/Go/worker ni se abrió conexión a PostgreSQL. El único listener observado fue el preexistente `127.0.0.1:5432`, no utilizado; las pruebas fueron BFF mocks/fixtures. El orden 5 sigue parcial por falta del target integrado aislado autorizado y readback persistido.
- Después de la corrida frontend consolidada, `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` pasó ambos paquetes (`internal/publiccatalog` y `internal/publiccatalog/transport/http`), usando caché, sin iniciar API ni conectar a base.
- Rebanada posterior solo de fixture/prueba visible: `npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **5/5**, incluyendo la ficha SSR con `../UTC` y el fallback visible `Horario no informado.`; typecheck, ESLint focal y Prettier focal también pasaron. No se cambió fuente productiva desde el build de la corrida consolidada. El gate browser→BFF→Go→DB/readback sigue pendiente.
- Continuación de cobertura solo test/fixture: dominio anfitrión confirma aceptación de `EST` por la validación basada en Intl; la ficha SSR sintética también conserva/renderiza el horario con ese alias. `node --test tooling/host-listings-domain.test.mjs` pasó **7/7** y calendario público **6/6**, typecheck/Prettier/ESLint focal PASS. No cambió producción ni se reconstruyó; se reutilizó el build de la corrida consolidada. La integración real sigue pendiente.
- El caso `EST` afirma los períodos visibles `14:15`/`16:45` después de seleccionar martes. La primera corrida falló porque el test aún no había seleccionado el día; la interacción se añadió y la suite final pasó 6/6, sin cambio productivo.

## Corrida final local tras reunir las regresiones de zona horaria — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check`: **exit 0** en este checkout. OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **493 (492 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright default **222 (159 aprobados, 63 omitidos, 0 fallidos)**. La configuración default excluye `public-weekly-calendar.spec.ts`.
- Después del build, `npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** sobre el fixture loopback; cubre horarios activo/ausente/inactivo, zona inválida `../UTC`, alias `EST`, privacidad/404 y click-through.
- Se verificó que los puertos `4213`, `18082`, `4207` y `18088` quedaron libres tras las corridas. El listener preexistente `127.0.0.1:5432` fue el único observado y no se utilizó. Sin API/Go/worker live, DB, GCS, Cloud SQL, commit, push, integración ni despliegue; orden 5 continúa parcial por falta de target aislado autorizado y readback.
- Tras la consolidación, se añadió al test del alias la aserción visible `Zona horaria: EST`; la suite pública focal quedó **6/6** y typecheck, lint focal y formato pasaron. Es un cambio solo de test, sin fuente productiva.
- Revisión read-only `deleg_1dccfe61`: PASS sin bloqueantes. Sus sugerencias de afirmar cambio `aria-pressed` y cero escrituras en el caso alias se implementaron; calendario público se volvió a ejecutar **6/6** y typecheck/ESLint/Prettier focal pasaron.

## Corrida final consolidada posterior — 2026-11-11

- En HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `PLAYWRIGHT_PORT=4222 API_BACKEND_BASE_URL=http://127.0.0.1:18089/api/v1 npx --yes pnpm@12.15.0 check` desde `frontend/` terminó **exit 0**: OpenAPI 200 operaciones; ESLint 0 errores/4 warnings; typecheck; unitarias **493 (492 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright **222 (159 aprobados, 63 omitidos, 0 fallidos)**.
- Sobre el artefacto recién construido, `PLAYWRIGHT_PORT=4222 API_BACKEND_BASE_URL=http://127.0.0.1:18089/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` terminó **6/6**. Ambos recorridos usaron fixtures/BFF mocks; no había backend escuchando en `18089`.
- `git diff --check` pasó; los puertos de ensayo `4222` y `18089` quedaron libres. Los listeners preexistentes `127.0.0.1:5432` y `127.0.0.1:18081` se conservaron sin conexión ni interacción. No se inició Go/API/worker, no se accedió a DB/GCS/Cloud SQL y no hubo mutaciones persistentes.
- Esto completa la revalidación local no mutante del orden 5 para el worktree actual, no el gate integrado browser→BFF→Go→DB/readback. Órdenes 1–5 siguen parciales sin target aislado autorizado. Sin commit, push, integración a `main`, despliegue ni aceptación online.

## Revalidación consolidada tras el archivo del orden 4 — 2026-11-11

- En el HEAD de este worktree, `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` terminó exit 0 (ambos paquetes; caché). A continuación, con entorno saneado, `PLAYWRIGHT_PORT=4277 API_BACKEND_BASE_URL=http://127.0.0.1:18117/api/v1 npx --yes pnpm@12.15.0 check` desde `frontend/` terminó **exit 0**: OpenAPI **200 operaciones**, ESLint **0 errores/4 warnings**, typecheck, unitarias **493 (492 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js 16.4.3 **32/32 páginas** y Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**.
- Tras esa build, `npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** con fixture sintético local. El API backend configurado en `18117` no tenía listener; la suite pública lanzó solo su fixture en `18088` y Next local en `4207`.
- Los puertos `4277`, `18117`, `4207` y `18088` estaban libres al terminar. Los listeners preexistentes `127.0.0.1:5432` y `127.0.0.1:18081` se conservaron sin conexión ni interacción. No se inició Go/API/worker live ni se accedió a DB/GCS/Cloud SQL.
- Incluye la regresión de UI de archivar que espera el estado `oculta`, el badge «Archivado» y la ausencia del CTA público; al ser BFF mock, no demuestra CAS/elegibilidad ni persistencia backend. Orden 5 continúa **parcial** por falta del gate integrado browser→BFF→Go→DB/readback y de un target aislado autorizado. Sin commit, push, integración, despliegue o aceptación online.

## Revalidación consolidada tras el click-through desde editor — 2026-11-11

- En HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `PLAYWRIGHT_PORT=4285 API_BACKEND_BASE_URL=http://127.0.0.1:18127/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **493 (492 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **222 (159 aprobados/63 omitidos/0 fallidos)**.
- Después de la build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6**, incluido el click-through del CTA del editor a la ficha SSR fixture. `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` pasó ambos paquetes con Go cache.
- Los puertos de ensayo `4285`, `18127`, `4207` y `18088` quedaron libres. No se conectó a los listeners preexistentes `127.0.0.1:5432` o `127.0.0.1:18081`. Se observó `127.0.0.1:4203` con `next-server` cuyo cwd pertenece a `espaciGo-main-sync/frontend/.next/standalone`; es ajeno a este worktree y quedó intacto. No se inició ni consultó API/Go live, worker, DB, GCS o Cloud SQL.
- Orden 5 sigue **parcial**: estas son pruebas unitarias, BFF mock y fixtures; no acreditan el recorrido integrado browser→BFF→Go→DB ni readback persistido. Sin commit, push, integración a `main`, despliegue o aceptación online.

## Revalidación local tras el contrato de variantes de galería — 2026-11-11

- En HEAD `647cd491985d619370990d3f226b5df61e36b42b`, con entorno saneado y puertos `4288`/`18130` verificados libres, `PLAYWRIGHT_PORT=4288 API_BACKEND_BASE_URL=http://127.0.0.1:18130/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **494 (493 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **222 (159 aprobados/63 omitidos/0 fallidos)**.
- Después de esa build, `npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** con Chromium, standalone y fixture BFF local. Los puertos fijos `4207`/`18088` se verificaron libres antes de iniciar; al terminar quedaron libres los cuatro puertos `4207`, `18088`, `4288`, `18130`.
- `PATH=/usr/local/go/bin:$PATH GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/...` terminó exit 0 para ambos paquetes desde caché. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL.
- La corrida incluye la corrección OpenAPI de variantes `[display, thumbnail]`, el cliente regenerado y la rehidratación mock del wizard tras reload. Los E2E usan BFF mocks/fixtures: esto completa la revalidación local no mutante del orden 5, no el readback integrado browser→BFF→Go→DB. Sin commit, push, integración, despliegue ni aceptación online; el gate integrado sigue bloqueado por falta de target aislado autorizado.

## Orden 5 — consolidación posterior al archivado público de orden 4 — 2026-11-11

- Primera corrida de `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check`: exit 1 por timeout de 30 s en `phase9-admin.spec.ts` (`@a11y` rutas administrativas); no fallaron otros tests reportados. El mismo test pasó aislado con `--workers=1` en **11.2 s**. Sin editar código ni aumentar el timeout, se repitió la consolidación desde el inicio y terminó **exit 0**.
- Corrida consolidada final: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **496 total (495 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **225 total (162 aprobadas, 63 omitidas, 0 fallidas)**.
- Tras la build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** con el caso de archivado/404 incluido. `git diff --check` y consulta de puertos de ensayo libres pasaron.
- Solo standalone/Chromium y fixtures/BFF mocks. El gate browser→BFF→Go→DB/readback permanece parcial por falta de target aislado autorizado. No se accedió a DB/GCS/Cloud SQL, ni hubo mutaciones persistentes, commit, push, integración a `main`, despliegue o aceptación online.

## Revalidación consolidada tras el stress test de galería — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **494 (493 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 con **32/32** páginas; Playwright **222 (159 aprobados/63 omitidos/0 fallidos)**.
- La primera corrida consolidada encontró el test de carga límite 30/31 esperando 5 s para confirmar un lote sintético de 26 archivos; falló con 28 listos al vencer el timeout, y la captura posterior mostró las 31 cargas completadas con galería en 30/30. Se amplió solo el timeout de esas aserciones a 20 s, manteniendo intactos el conteo, los 26 POST esperados y el límite 30/30. El recorrido focal pasó **5/5** en repeticiones; la corrida consolidada completa posterior pasó.
- La ejecución utilizó standalone local, Chromium y BFF mocks; `API_BACKEND_BASE_URL` loopback no tenía listener. Los puertos `4213`, `4214` y `18082` estaban libres al terminar. No se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL.
- Sobre el standalone recién construido, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** con fixtures. Los puertos `4207` y `18088` estaban libres antes de arrancar; esta suite también es local/mock.
- Es revalidación local no mutante; el gate integrado browser→BFF→Go→DB/readback sigue parcial por falta de target aislado autorizado. No hubo commit, push, integración a `main`, despliegue ni aceptación online.

## Orden 5 — revalidación final posterior al caso activo→404 — 2026-11-11

- Con puertos `4213`, `18082`, `4207` y `18088` libres, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **494 (493 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright **225 total: 162 aprobadas, 63 omitidas, 0 fallidas**.
- Después del build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6**, incluyendo navegación activa→404 sin contenido residual. `git diff --check` pasó.
- Ejecución local con standalone/Chromium y fixtures BFF. Sin API/Go live, worker, DB, GCS o Cloud SQL; el gate integrado de órdenes 1–5/readback sigue parcial por falta del target aislado autorizado. Sin commit, push, integración, despliegue ni aceptación online.

## Orden 5 — consolidación final del trabajo local reunido — 2026-11-11

- Tras las reejecuciones focales de órdenes 1–4, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **494 (493 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright default **225 (162 aprobadas, 63 omitidas, 0 fallidas)**.
- Sobre la build recién generada, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6**. `git diff --check` pasó después de registrar resultados.
- Solo standalone/Chromium y fixtures/BFF mocks. No se inició Go/API/worker ni se accedió a DB/GCS/Cloud SQL. El recorrido browser→BFF→Go→DB/readback sigue parcial por falta de target aislado autorizado; sin commit, push, integración o despliegue.

## Addendum test-only posterior a la corrida consolidada — 2026-11-11

- Después de `pnpm check`, se añadió solo una prueba unitaria de caracterización del contrato para `display` individual y `thumbnail` individual; no cambió código de producto/OpenAPI. El spec pasó **8/8**, Prettier focal pasó y `pnpm test` completo pasó **495 (494 aprobadas, 0 fallidas, 1 opt-in omitida)**.
- La corrida consolidada OpenAPI/lint/typecheck/format/build/E2E y el calendario público 6/6 registrados arriba son anteriores a esta edición test-only; no se repitieron build/E2E porque la fuente productiva quedó intacta. `git diff --check` se revalida al cerrar este addendum.

## Consolidación final después del test-only — 2026-11-11

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0** sobre el árbol actual: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **496 total (495 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright **225 (162 aprobadas, 63 omitidas, 0 fallidas)**.
- Tras esa build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6**. Los puertos `4213`, `18082`, `4207` y `18088` estaban libres antes de la corrida final y volvieron a quedar libres al terminar.
- El spec contractual focal quedó en **9/9** y usa el helper UUID real; el full check incluyó la suite actualizada. Solo standalone/fixtures/BFF mocks y sonda Go stdlib local; no se inició API/worker ni se accedió a PostgreSQL/GCS/Cloud SQL. Orden 5 continúa parcial por browser→BFF→Go→DB/readback sin target aislado autorizado. Sin commit, push, integración, despliegue ni aceptación online.

## Consolidación con cobertura de catálogo activo→archivado — 2026-11-11

- Con el test de catálogo y fixture incorporados, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **496 (495 aprobadas, 1 opt-in omitida)**; build Next.js **32/32 páginas**; Playwright **225 (162 aprobadas, 63 omitidas, 0 fallidas)**.
- La suite pública `test:e2e:public-calendar` pasó **6/6** y el caso `active owner listing CTA` se reejecutó después del ajuste final (**1/1**). Prettier focal, ESLint focal, `node --check` del fixture y `git diff --check` también pasaron.
- Evidencia exclusivamente local con Chromium, Next standalone, fixtures y BFF mocks. No se inició API/worker ni se accedió a PostgreSQL/GCS/Cloud SQL; no prueba backend real, autorización, privacidad, persistencia ni readback. El gate integrado continúa bloqueado hasta disponer de target aislado autorizado. Sin commit, push, integración o despliegue.

## Revalidación de órdenes 4–5 después de orden 3 — 2026-11-12

- En la rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`, `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **503 total (502 aprobadas, 0 fallidas, 1 omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright **225 total (162 aprobadas, 63 omitidas, 0 fallidas)**. El recorrido phase7 actualizado de galería/portada/reordenamiento quedó incluido y pasó.
- Revalidaciones locales separadas: `test:e2e:public-calendar` **6/6**; `host-listings-publication-cas.spec.ts` **1/1**; `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/... -count=1` pasó ambos paquetes. Puertos `4207`, `4213`, `18082` y `18088` comprobados libres antes/después; `git diff --check` PASS.
- Solo Chromium/Next standalone, fixtures/BFF mocks y Go offline; no hubo API/worker ni acceso a DB/GCS/Cloud SQL. Orden 5 sigue parcial para browser→BFF→Go→DB/readback por falta del target aislado autorizado y artefactos privados requeridos. Sin commit, push, integración, despliegue ni aceptación online.

## Corrida final reunida tras la cobertura de elegibilidad de orden 4 — 2026-11-12

- `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0** en el árbol de trabajo actual: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck PASS; unitarias **503 (502 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier PASS; build Next.js 16.4.3 **32/32 páginas**; Playwright default **225 (162 aprobadas/63 omitidas/0 fallidas)**.
- Tras el build, `API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6**. Go offline: `GOPROXY=off GOTOOLCHAIN=local go test ./internal/publiccatalog/... -count=1` y `go test ./internal/adapters/postgres/publiccatalog -run 'TestPublicVerificationSQL' -count=1` PASS, incluidos los subtests de decisión de identidad y espacio.
- Se observaron 4 warnings de ESLint (0 errores) y 4 avisos Node `MODULE_TYPELESS_PACKAGE_JSON`; ninguna prueba falló. Puertos de fixtures/browser `4213`, `18082`, `4207`, `18088` estaban libres antes de sus corridas y quedaron libres después. `git diff --check` PASS.
- Todo fue local con Next standalone, Chromium, BFF/fixtures sintéticos y Go offline. `@native`/`@native-public` se omitieron; no se inició API/worker ni se accedió a DB/GCS/Cloud SQL. Esta corrida completa los gates locales de orden 5, no el gate integrado browser→BFF→Go→DB/readback, que sigue parcial por falta de target aislado autorizado y artefactos privados requeridos. Sin commit, push, integración, despliegue ni aceptación online.

## Revalidación tras revisión independiente de órdenes 2–4 — 2026-11-12

- Con el fixture de galería ajustado para conflicto ETag `"2"`→`"3"` y el test de elegibilidad SQL limitado al literal `eligiblePublicSQL`, la corrida `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **503 (502 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **225 (162 aprobadas, 63 omitidas, 0 fallidas)**. La regresión de incertidumbre quedó incluida y pasó.
- Después del build, `PLAYWRIGHT_PORT=4207 API_BACKEND_BASE_URL=http://127.0.0.1:18088/api/v1 npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** y `PLAYWRIGHT_PORT=4213 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 npx --yes pnpm@12.15.0 exec playwright test tests/weekly-schedule-editor.spec.ts --reporter=list` pasó **1/1**. Go offline: `go test ./internal/publiccatalog/... ./internal/adapters/postgres/publiccatalog -count=1` y `go test ./internal/occupancy ./internal/spaces/transport/http -count=1` pasaron; `gofmt -d` quedó sin diferencias. Prettier focal de test/evidencias y `git diff --check HEAD` PASS.
- Los puertos locales `4213`, `18082`, `4207` y `18088` estaban libres antes de las corridas y quedaron libres después. Solo Chromium/Next standalone, mocks/fixtures y tests Go sin DB; no se inició API/worker ni se accedió a PostgreSQL, GCS o Cloud SQL. Los gates integrados de órdenes 1–5 siguen parciales por falta de target aislado autorizado y artefactos privados requeridos; no commit, push, integración, despliegue ni aceptación.

## Corrida local final tras órdenes 3–4 — 2026-11-12

- En el árbol actual, `PLAYWRIGHT_PORT=4233 API_BACKEND_BASE_URL=http://127.0.0.1:18093/api/v1 npx --yes pnpm@12.15.0 check` terminó **exit 0**: API/OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck PASS; unitarias **503 (502 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier PASS; build Next.js 16.4.3 **32/32 páginas**; Playwright default **225 (162 aprobadas/63 omitidas/0 fallidas)**.
- Tras la build, `npx --yes pnpm@12.15.0 test:e2e:public-calendar` pasó **6/6** con Chromium/standalone y fixture BFF; usó los puertos fijos `4207` y `18088`. Go offline: pasaron `go test ./internal/publiccatalog/... ./internal/occupancy ./internal/spaces/transport/http ./internal/gallery/transport/http ./internal/gallery ./internal/media -count=1`, el test source-level `TestPublicVerificationSQLUsesLatestFinalizedDecision`, y `go vet` de los paquetes unitarios correspondientes.
- Se observaron 4 avisos Node `MODULE_TYPELESS_PACKAGE_JSON`; no hubo fallos. Los puertos usados `4207`, `4233`, `18088` y `18093` quedaron libres al terminar.
- Todo fue local, sintético y no mutante; los tests `@native`/`@native-public` permanecen omitidos. Sin API/worker, acceso a DB/GCS/Cloud SQL, ni cambios de controles. Quedan parciales los gates integrados browser→BFF→Go→DB/readback de órdenes 1–5; no hubo commit, push, integración, despliegue ni aceptación online.

## Corrida consolidada posterior a thumbnail público — 2026-11-12

- Con la prueba de thumbnail, la aserción de readback de la primera foto tras crear el borrador, la caracterización BFF sin cookie/Bearer y el handler Go de ambas variantes públicas, la última `PLAYWRIGHT_PORT=4228 API_BACKEND_BASE_URL=http://127.0.0.1:18092/api/v1 pnpm check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **504 (503 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **226 (163 aprobadas/63 omitidas/0 fallidas)**. Esta cuenta sustituye la corrida anterior registrada arriba.
- Tras ese build, Chromium local pasó `test:e2e:public-calendar` **6/6** y `test:e2e:public-gallery` **2/2**. Go sin DB: tests y `go vet` de `internal/publiccatalog/...`, `internal/occupancy`, `internal/spaces/transport/http`, `internal/gallery/transport/http`, `internal/gallery` e `internal/media` PASS; `gofmt -d` y `git diff --check HEAD` PASS.
- Fixture/Next standalone local; puertos `4228`, `18092`, `4207`, `18088`, `4208` y `18089` libres tras las corridas. No API/worker, PostgreSQL, GCS ni Cloud SQL; no migraciones, grants, identidades ni credenciales; no commit, push, integración o despliegue. Los gates integrados de órdenes 1–5 siguen parciales por falta de target aislado autorizado.

## Extensión Go local de orden 4: headers y límites de ruta — 2026-11-12

- Tras la corrida consolidada frontend anterior, se reforzó únicamente `internal/publiccatalog/transport/http/handler_test.go`: además de POST 405, query y variante `avatar` 404 sin consultas al repositorio/object store, los éxitos validan ETag SHA-256, `Content-Length` y `Content-Disposition`; generación desalineada devuelve 404 sin bytes. Errores de repositorio/storage no exponen detalles ni contenido; CORS cubre preflight, GET permitido y origen no confiable antes de lecturas. La revocación revalida antes del objeto. Tests focales, `go vet` de HTTP, `gofmt` y `git diff --check` pasaron.
- No se modificó frontend desde la `pnpm check` registrada arriba. Esta prueba usa fakes in-memory; no ejecuta API, DB, GCS ni demuestra persistencia/ownership real.

## E2E local de fallback visual público — 2026-11-12

- Solo cambió `frontend/tests/public-gallery-media.spec.ts`; sin cambios de fuente UI, se reutilizó el standalone ya construido. La prueba de `display 404 → legacy original 404` afirma el placeholder accesible vigente, zoom oculto, navegación a la segunda foto y cero mutaciones.
- `pnpm exec playwright test --config=playwright.public-gallery.config.ts --grep 'shows its current placeholder'`: **1/1 PASS**. Suite completa `pnpm exec playwright test --config=playwright.public-gallery.config.ts`: **3/3 PASS**.
- Fixture local; el comportamiento visual queda caracterizado, no aceptado por producto. No demuestra BFF/Go/storage/persistencia reales. El frontend completo no se volvió a ejecutar en esta rebanada.

## E2E local de fallback en tarjeta pública — 2026-11-12

- Solo cambió el spec `frontend/tests/public-gallery-media.spec.ts`; en Chromium local, `thumbnail 404 → legacy original 404` conserva el placeholder, el enlace de la tarjeta y la navegación a la ficha, sin solicitudes mutantes emitidas por la página.
- Revalidación focal de fallback de ficha y tarjeta **2/2 PASS** (cada recorrido ejecuta ambos viewports); suite de galería dedicada **4/4 PASS**. `pnpm lint` 0 errores/4 warnings, `pnpm typecheck` y Prettier PASS. Reutilizado el standalone existente; no se modificó UI productiva.
- Caracterización de fixtures únicamente; placeholder sin aceptación visual de producto, y sin evidencia de BFF/Go/storage/persistencia integrados.
- Se agregó Axe-core focalizado al placeholder de galería y al de tarjeta pública; ambas regiones reportan cero violaciones en el estado de fallback. Después de una imagen alternativa cargada, la galería verifica que el placeholder desaparezca. En ambos recorridos, cada viewport exige el conjunto deduplicado de pares método/ruta `GET` de variante + recurso legacy; se toleran reintentos GET y se rechazan otros métodos en esas rutas. El contador de mutaciones cubre solicitudes de página, no efectos internos del servidor.
- La revisión detectó que `arrayContaining` aceptaba métodos o repeticiones inesperados. El intento de exigir conteo bruto exacto observó GET repetidos de `display` (3) y `thumbnail` (2), además de un GET legacy en cada caso; no se infiere la causa. La aserción final compara pares únicos método/ruta, permite esos GET repetidos y falla ante otros métodos en las dos rutas verificadas.
- La re-revisión independiente final, estrictamente read-only y sin ejecutar tests/navegador, no encontró bloqueos ni errores lógicos pendientes. Verificó ambos viewports, navegación de tarjeta, desaparición del placeholder y alcance del contador de solicitudes de página. La fixture sintética y Next se enlazan a `127.0.0.1`; el config desactiva `reuseExistingServer`. Esta revisión de fuente no convierte el fixture en evidencia de integración real.

## Corrida consolidada de orden 5 tras fallback y accesibilidad — 2026-11-12

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`. `pnpm check` se ejecutó con entorno aislado y puertos locales comprobados: API/OpenAPI **200**; ESLint **0 errores/4 warnings**; typecheck; `node --test` **504 (503 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; Next build **32/32**; Playwright default **226 (163 aprobadas/63 omitidas/0 fallidas)**.
- Tras el build, Playwright público calendario **6/6** y galería **4/4**. Los dos fallbacks de galería/tarjeta se recorrieron a **1365×900** y **390×844**; Axe-core focal en ambas regiones reportó **0 violaciones**. Los asserts de las rutas de imagen exigen el conjunto esperado de métodos/rutas, tolerando reintentos GET repetidos; el contador solo afirma cero solicitudes mutantes de la página. Go `test` y `vet` offline PASS en catálogo público, adaptador PostgreSQL de catálogo, ocupación, espacios HTTP, galería HTTP/servicio y media; `gofmt -d` y `git diff --check HEAD` PASS. Puertos `4207`, `18088`, `4208`, `18089` libres al terminar.
- Solo se usaron Next standalone, Chromium y fixtures; ejecución Go sin variables de conexión DB. No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o navegador/CUA del usuario. Tests `@native`/`@native-public` omitidos. Orden 5 local verificado, pero el gate integrado browser→BFF→Go→DB/readback permanece parcial/bloqueado; orden 6, publicación e integración siguen fuera de este resultado. Sin commit, push, merge o despliegue.

## Revalidación consolidada después del fallback legacy exitoso — 2026-11-12

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`. `pnpm check` exit 0: OpenAPI **200 operaciones**, ESLint **0 errores/4 warnings**, typecheck, unitarias **504 (503 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js **32/32** y Playwright default **226 (163 aprobadas/63 omitidas/0 fallidas)**.
- Sobre el build final: calendario público **6/6** y galería pública **5/5**. El nuevo caso `display 404 → legacy WebP 200` pasó y recorre escritorio **1365×900** y móvil **390×844**; valida `naturalWidth > 0`, ausencia de placeholder, pares GET esperados sin rutas/métodos extra, cero solicitudes mutantes emitidas por la página y Axe sin violaciones.
- Go offline tests/vet pasaron para `internal/publiccatalog/...`, `internal/adapters/postgres/publiccatalog`, `internal/occupancy`, `internal/spaces/transport/http`, `internal/gallery/transport/http`, `internal/gallery` e `internal/media`; `gofmt -d`, diff-check y puertos de ensayo sin errores/libres.
- Solo Next standalone, Chromium, fixtures y Go en proceso. No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o al navegador del usuario. Los tests `@native`/`@native-public` quedaron omitidos. No se acredita fallback integrado, ownership, autorización ni persistencia; el gate browser→BFF→Go→DB/readback sigue bloqueado: falta designar un target aislado y autorizar explícitamente las operaciones y readbacks integrados. Sin commit, push, integración a `main` o despliegue.

## Revalidación tras fallback legacy exitoso de tarjeta — 2026-11-12

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD base `647cd491985d619370990d3f226b5df61e36b42b`; `frontend/tests/public-gallery-media.spec.ts` sigue sin seguimiento, sin commit. `pnpm check` pasó exit 0 en la corrida final con `PLAYWRIGHT_PORT=4238` y `API_BACKEND_BASE_URL=http://127.0.0.1:18098/api/v1`: OpenAPI **200**, unitarias **504 (503 aprobadas/0 fallidas/1 opt-in omitida)**, ESLint **0 errores/4 warnings**, typecheck, Prettier, build **32/32**, Playwright default **226 (163 aprobadas/63 omitidas/0 fallidas)**.
- Tras el build: calendario público **6/6** y galería **6/6**. El nuevo caso `thumbnail 404 → legacy WebP 200` verifica todas las respuestas observadas por viewport (thumbnail 404; original 200 `image/webp`), carga decodificada, ausencia de placeholder, pares GET esperados, cero solicitudes mutantes de página y Axe 0 en **1365×900** y **390×844**. Go offline tests/vet PASS para los paquetes de catálogo, occupancy, spaces HTTP, gallery y media listados en la sección anterior; puertos propios libres.
- La primera invocación de `pnpm check` se detuvo antes de ejecutar Playwright porque el puerto predeterminado **4203** estaba ocupado; no se interactuó con ese listener. La repetición usó puerto propio **4238** y target API configurado solo a loopback sin listener, y completó los gates. No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o navegador/CUA del usuario.
- Evidencia sintética local; no prueba fallback BFF→Go/storage, autorización, ownership ni readback persistido. El gate integrado y el orden 6 permanecen bloqueados por falta de designación de target aislado y autorización explícita de operaciones/readbacks. Sin commit, push, integración a `main` o despliegue.
- Revisión source-only: `deleg_fb475b97` detectó que el mapa comprobaba solo la última respuesta y aclaró que el spec es untracked, no parte de HEAD. Se cambió a capturar cada respuesta por viewport y verificar todos sus status/MIME; la comparación deduplicada de pares conserva intencionalmente la tolerancia a GET repetidos. La aclaración worktree/HEAD quedó registrada arriba. Re-revisión focal `deleg_b118c20a`: **PASS**, sin concerns ni errores lógicos. Ningún revisor ejecutó tests ni accedió a servicios; el test sigue siendo fixture sintética.

## Orden 5 — consolidación posterior a la cobertura de horario parcial del orden 4 — 2026-11-12

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`. `pnpm check` terminó **exit 0** con entorno de DB/native opt-ins sin configurar, `PLAYWRIGHT_PORT=4238` y API backend solo como loopback sin listener. API/OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck, unitarias, formato, build y Playwright default pasaron. E2E default: **228** (165 aprobadas, 63 omitidas, 0 fallidas). `pnpm test` confirmó **505** (504 aprobadas, 0 fallidas, 1 opt-in omitida).
- Tras el build de esa corrida, `playwright.public-calendar.config.ts` pasó **6/6** —incluye la distinción accesible `mixed`/«Parcial» para 14:00–15:00 y 16:00–17:00 frente a 15:00–16:00 completo— y `playwright.public-gallery.config.ts` pasó **6/6**. Puertos loopback dedicados estuvieron disponibles antes y quedaron libres después.
- Go offline `test` y `vet` PASS para `internal/publiccatalog/...`, `internal/adapters/postgres/publiccatalog`, `internal/occupancy`, `internal/spaces/transport/http`, `internal/gallery/transport/http`, `internal/gallery` e `internal/media`; `gofmt -d` y `git diff --check HEAD` PASS. Se desactivaron por entorno los opt-ins de integración SQL; no se inició API/worker ni hubo conexión o escritura a PostgreSQL, GCS, Cloud SQL o servicios online.
- Orden 5 permanece **parcial** solo por el recorrido integrado browser→BFF→Go→DB/readback: no hay target aislado autorizado. No se publica ni se integra a `main`; orden 6 entrega/aceptación continúa bloqueado por sus gates integrados y autorizaciones pendientes.

## Orden 5 — consolidación tras la revalidación de publicación — 2026-11-12

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD base `647cd491985d619370990d3f226b5df61e36b42b`. `PLAYWRIGHT_PORT=4294 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 pnpm check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **507 total (506 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **229 total (166 aprobadas/63 omitidas/0 fallidas)**.
- Después del build, `pnpm test:e2e:public-calendar` pasó **6/6** y `pnpm test:e2e:public-gallery` **6/6**, en Chromium local con standalone y fixtures loopback. Go offline pasó `go test -count=1` y `go vet` para booking, adaptadores PostgreSQL de booking/catálogo público, occupancy, spaces HTTP, catálogo público, gallery y media; `gofmt -l` no reportó archivos y `git diff --check HEAD` pasó. Los listeners propios `4207`, `18088`, `4208` y `18089` estaban libres antes y después.
- No se inició API/worker ni se usó PostgreSQL, GCS, Cloud SQL o servicios online durante estas pruebas. El target aprobado `espacigo_spaces_order2_20261113` y sus operaciones autorizadas son de orden 2 solamente; no se ejecutaron migraciones/roles/harness/readback integrados y no se extiende esa autorización a los gates integrados de orden 4/5. Orden 5 conserva pendiente su aceptación integrada browser→BFF→Go→DB/readback; orden 4 y 5 siguen parciales. Sin commit, push, integración o despliegue.

## Revalidación consolidada posterior al endurecimiento de envelopes — 2026-11-13

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD base `647cd491985d619370990d3f226b5df61e36b42b`. Con entorno saneado, `PLAYWRIGHT_PORT=4294 API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1 pnpm check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **508 (507 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32**; Playwright default **229 (166 aprobadas/63 omitidas/0 fallidas)**. La nueva regresión de cinco envelopes malformados quedó incluida.
- Después del build, `pnpm exec playwright test --config=playwright.public-calendar.config.ts` pasó **6/6** y `pnpm exec playwright test --config=playwright.public-gallery.config.ts` pasó **6/6**. Solo Next standalone/Chromium y fixtures loopback; el target de API configurado no tuvo listener. Puertos `4294`, `18082`, `4207`, `18088`, `4208` y `18089` quedaron libres al terminar.
- `git diff --check HEAD` pasó. Go offline test/vet no se repitieron en esta rebanada; sus resultados anteriores permanecen fechados por separado en las evidencias previas. No se inició Go/API/worker ni se consultó o modificó PostgreSQL/GCS/Cloud SQL.
- Revalidación local no mutante solamente: los gates integrados browser→BFF→Go→DB/readback de órdenes 1–5 continúan parciales; la autorización del target de orden 2 no se extiende a órdenes 4–5. Sin commit, push, integración, despliegue ni aceptación online.

## Follow-up Go offline de orden 5 — 2026-11-13

- Con entorno `env -i` y `GOPROXY=off GOTOOLCHAIN=local`, `go test -count=1` pasó para `./internal/adapters/postgres/booking`, `./internal/booking/...`, `./internal/occupancy`, `./internal/spaces/transport/http`, `./internal/publiccatalog/...`, `./internal/gallery/transport/http`, `./internal/gallery` y `./internal/media`. El test focal `go test -count=1 -run '^TestPublicVerificationSQLUsesLatestFinalizedDecision$' ./internal/adapters/postgres/publiccatalog` también pasó.
- `go vet` pasó para los mismos paquetes, incluido el paquete de catálogo PostgreSQL; `gofmt -l` no reportó archivos entre los Go revisados y `git diff --check HEAD` pasó.
- El filtrado del test PostgreSQL de catálogo fue intencional: solo se ejecutó la caracterización source-level indicada, no los tests nativos/opt-in del paquete. En el paquete booking las pruebas de PostgreSQL exigen `TEST_DATABASE_URL`, ausente en el entorno limpio. No se conectó a PostgreSQL ni se inició API/worker; tampoco se usaron GCS/Cloud SQL.
- Esta evidencia es Go unit/fake/source-level y no prueba ejecución de SQL, autorización integrada ni readback persistido. Completa el complemento Go no mutante de la revalidación local, no el gate integrado browser→BFF→Go→DB de órdenes 1–5. Sin commit, push, integración, despliegue ni aceptación online.

## Revalidación consolidada de orden 5 — 2026-11-13

- Worktree `/home/nandev/Documentos/projects/espaciGo-spaces-wizard-recovery`, rama `codex/frontend/fix/space-01-create-postcommit-recovery`, HEAD `647cd491985d619370990d3f226b5df61e36b42b`. Con entorno `env -i`, API URL configurada a loopback sin listener y `PLAYWRIGHT_PORT=4294`, `pnpm check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **508 (507 aprobadas/0 fallidas/1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **229 (166 aprobadas/63 omitidas/0 fallidas)**.
- Después del build, calendario público y galería pública pasaron **6/6** cada una en Chromium/Next standalone con fixtures loopback. Go offline `go test -count=1` pasó en `internal/booking`, `internal/occupancy`, `internal/booking/transport/http`, `internal/spaces/transport/http`, `internal/publiccatalog/...`, `internal/gallery`, `internal/gallery/transport/http` e `internal/media`; pasó además el test source-only `TestPublicVerificationSQLUsesLatestFinalizedDecision`. `go vet` pasó en esos paquetes Go offline; `gofmt -l` no reportó archivos.
- Los opt-ins de DB/GCS quedaron deshabilitados mediante entorno limpio. En `internal/adapters/postgres/publiccatalog` solo se ejecutó el test source-only nombrado, no los tests nativos que conectan a PostgreSQL. No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o servicios online. Los puertos `4294`, `18082`, `4207`, `18088`, `4208` y `18089` quedaron libres al finalizar.
- Esta cobertura valida contrato/API inventory, UI/build y suites locales con fixtures/fakes; no demuestra autorización real, ejecución SQL, fallback backend, persistencia ni readback integrado. Órdenes 4–5 siguen parciales en browser→BFF→Go→DB/readback. Sin commit, push, integración a `main`, despliegue o aceptación online.

## Reejecución final de orden 5 tras la continuación de orden 4 — 2026-11-13

- En entorno saneado (`env -i`), `PLAYWRIGHT_PORT=4294`, `API_BACKEND_BASE_URL=http://127.0.0.1:18082/api/v1` sin listener y `pnpm check` terminó **exit 0**: API/OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **508 (507 aprobadas, 0 fallidas, 1 opt-in omitida)**; Prettier; build Next.js 16.4.3 **32/32 páginas**; Playwright default **229 (166 aprobadas, 63 omitidas, 0 fallidas)**.
- Sobre esa build, las suites públicas de calendario y galería pasaron **6/6** cada una en Chromium/Next standalone con fixtures loopback.
- Con `GOPROXY=off GOTOOLCHAIN=local`, `go test -race -count=1` pasó en `internal/booking`, booking HTTP, occupancy, spaces HTTP, `internal/publiccatalog/...`, `internal/gallery/...` e `internal/media`; `go vet` pasó en esos paquetes. Pasó también únicamente `TestPublicVerificationSQLUsesLatestFinalizedDecision` en el adaptador PostgreSQL de catálogo; no se ejecutaron los otros tests que podrían conectar a DB. `gofmt -l` no reportó archivos.
- Los puertos `4294`, `18082`, `4207`, `4208`, `18088` y `18089` estaban libres tras las suites. No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o servicios online.
- Evidencia local/mock/source-only; no acredita autorización real, SQL ejecutado, persistencia o readback browser→BFF→Go→DB. Órdenes 1–5 y el gate integrado siguen parciales. Sin commit, push, integración, despliegue o aceptación online.

## Reejecución de orden 5 tras la última cobertura de orden 4 — 2026-11-13

- Sobre HEAD `647cd491985d619370990d3f226b5df61e36b42b`, con `env -i`, `PLAYWRIGHT_PORT=4294` y API loopback `18082` sin listener, `pnpm check` terminó **exit 0**: OpenAPI **200 operaciones**; ESLint **0 errores/4 warnings**; typecheck; unitarias **508 (507 aprobadas/0 fallidas/1 omitida opt-in)**; formato; build Next.js 16.4.3 **32/32**; Playwright default **229 (166 aprobadas/63 omitidas/0 fallidas)**.
- Tras el build, calendario público y galería pública pasaron **6/6** cada una sobre fixtures loopback. Preflight y postflight confirmaron libres los puertos locales reservados.
- Con entorno limpio y `GOPROXY=off GOTOOLCHAIN=local`, `go test -race -count=1` y `go vet` pasaron para booking, occupancy, booking/spaces HTTP, publiccatalog, gallery y media; el test source-only `TestPublicVerificationSQLUsesLatestFinalizedDecision` también pasó. La corrida incluyó `internal/publiccatalog/service_media_input_test.go`; `gofmt -l` y `git diff --check HEAD` no reportaron problemas.
- No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o servicios online. Los gates integrados browser→BFF→Go→DB/readback siguen parciales; las corridas locales no los sustituyen. Sin commit, push, integración o despliegue.

## Reejecución adicional de orden 5 tras avanzar orden 4 — 2026-11-13

- Corrida independiente de `pnpm check` con `env -i`, `PLAYWRIGHT_PORT=4294` y API loopback `18082` sin listener: exit **0**, OpenAPI **200 operaciones**, ESLint **0 errores/4 warnings**, typecheck, unitarias **508 (507 aprobadas/0 fallidas/1 opt-in omitida)**, Prettier, build Next.js 16.4.3 **32/32**, Playwright default **229 (166 aprobadas/63 omitidas/0 fallidas)**.
- Tras el build, `playwright.public-calendar.config.ts` y `playwright.public-gallery.config.ts` pasaron **6/6** cada una con Chromium, standalone y fixtures loopback.
- Con entorno limpio y `GOPROXY=off GOTOOLCHAIN=local`, `go test -count=1` y `go vet` pasaron en booking, occupancy, booking/spaces HTTP, gallery HTTP/gallery, media y `publiccatalog/...`; `gofmt -l` fue vacío y `git diff --check HEAD` pasó. Esta reejecución Go fue sin `-race`; las revalidaciones `-race` previas quedan en sus entradas fechadas.
- Los puertos locales `4294`, `18082`, `4207`, `18088`, `4208` y `18089` quedaron libres después. No se inició API/worker ni se accedió a PostgreSQL, GCS, Cloud SQL o servicios online. Sigue sin probarse browser→BFF→Go→DB/readback; orden 5 permanece parcial. Sin commit, push, integración o despliegue.
