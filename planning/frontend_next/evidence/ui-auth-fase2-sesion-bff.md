# UI-AUTH fase 2 — evidencia de ejecución

**Estado: fase 2 completada y validada localmente el 26 de octubre de 2026. Publicación y despliegue se verifican por separado; no se inicia fase 3.**

## Alcance y base

- Corte local sobre `main`, HEAD de partida `8a87e38d1b27c384d054ff4bf463c46bdc49c9d2`.
- Mantener los cambios cromáticos de fase 1 y el diseño Next/Tailwind existente. Sin fase 3, cabeceras nuevas, cambios de infraestructura o backend productivo.
- Contrato: `../contrato_ejecucion_ui_auth_fase2.md`; decisión aprobada: `../adr_ui_auth_001_persistencia_bff.md`.

## Preflight nativo verificado

El harness privado monta los handlers y servicios Go de identidad reales sobre PostgreSQL nativo autorizado, sin iniciar `cmd/api` o workers. Evidencia privada: `.local/testing/ui-auth-phase2-native/preflight-report.json`.

Resultado leído y comprobado: DB/rol coincidentes, credencial sintética existente comprobada offline, login **200**, sesión **200**, logout **204**, consulta posterior **401**, expiración dentro de ocho horas. Cero cuentas nuevas y cero recuperaciones; workers no iniciados. El contador de un intento corresponde exclusivamente a esta ejecución del harness principal, no al total histórico de intentos de toda la conversación.

El dominio reservado para crear cuentas en la política no coincide con el inventario existente: no se interpreta esa discrepancia como permiso de creación. Se comprobó por separado la autorización histórica del inventario sintético existente y verificado.

Se compiló además un modo servidor privado del mismo harness, limitado a identidad/perfil y a `127.0.0.1:18084`, con limpieza de las sesiones que emita al recibir SIGTERM. Compilarlo no equivale a haber ejecutado el gate HTTPS nativo: ese gate sigue pendiente en este punto.

## Tracer HTTPS RED real

`frontend/tooling/bff-session-browser.mjs` inicia dos procesos Next propios, un proxy TLS loopback y un navegador Chromium con perfil nuevo y privado. No usa el navegador del usuario. Los endpoints BFF no se interceptan: se ejercita la aplicación real contra un upstream sintético, claramente distinto de la prueba Go/PostgreSQL.

Contra el build anterior, la prueba falló por el motivo esperado: **el login todavía devolvía el DTO antiguo con campos ajenos a la whitelist BFF**. Salida real: 0 PASS / 1 FAIL, exit 1. No se usaron credenciales nativas en este RED. Evidencia privada conservada en `.local/testing/ui-auth-phase2/controlled-browser-red-report.json`.

Los listeners propios de ese RED se cerraron y se verificaron libres `4330`, `4331`, `18443` y `18085`. Se compiló el harness Go privado y se comprobó sin diferencias el alcance protegido `AGENTS.md`, `frontend/AGENTS.md`, `cmd/` e `internal/`.

## Fixtures de regresión

Los interceptores controlados de las suites UI 5–9 se adaptaron al DTO BFF sin token, conservando una única expiración absoluta por login. Son fixtures de UI, **no prueba de cookies o seguridad del BFF**. Se eliminaron campos `access_token`/`token_type` y se reemplazaron las expectativas de Bearer del navegador por ausencia de Authorization. Los helpers de acceso modificados usan namespaces `.invalid`.

## Gates iniciales (histórico; actualización al final)

- Integración y revisión del servidor BFF y núcleo cliente delegados.
- Tipos, lint, formato, unitarias, build y regresión E2E/Axe sobre el nuevo build.
- HTTPS GREEN, continuidad entre procesos, reapertura de Chromium, caducidad/revocación, 401/403/503, logout remoto fallido, marcador pendiente y sincronización entre pestañas.
- Gate HTTPS separado con Go real y la cuenta sintética existente, sin workers.
- Revisión independiente, documentación final y comprobación de limpieza. Sin commit/push/despliegue de este corte por ahora.

## Corrección verificada de reservas tras entrega no acreditada

La entrega `deleg_14b01f31` afirmó aislamiento completo, pero el padre leyó el archivo real y encontró el constructor de un argumento y ausencia de `clear()`. Las siete regresiones de sesión fallaron; no se aceptó ese reporte. También se corrigió el doble de prueba que llamaba `getReservationDetail` en lugar del puerto real `getReservation`, sin cambiar las expectativas de aislamiento.

El padre implementó la barrera sobre el código real: constructor conectado a AuthSource, purga íntegra, epoch y AbortController de sesión, combinación de señales y guardas después de awaits/antes de retornar datos. Un transformador AST aplicó el mismo patrón a los métodos Promise sin reescribir payloads manualmente. La comprobación AST registró 36 llamadas de puerto antes y después, con argumentos de negocio preservados.

Verificación real posterior: **13 PASS / 0 FAIL** en las suites de reglas y aislamiento de reservas; ESLint y Prettier pasan; **typecheck global pasa** en ese corte. Los snapshots de código retenidos se guardaron como `.ts.txt` en el directorio privado para no tratarlos como fuentes compilables. Este resultado no reemplaza los gates de build, HTTPS, nativo y revisión final todavía abiertos.

## Ampliación verificada del aislamiento administrativo

La entrega administrativa fue explícitamente parcial: cinco pruebas pasaban, pero las guardas post-await solo cubrían `loadGlobalMe` y `loadOverview`. El padre añadió dos regresiones reales para KPI tardío tras purga y KPI sin sesión confirmada; ambas fallaron antes del cambio.

Se extendió mediante AST la guarda de sesión a los métodos Promise restantes, combinando señales de aborto y preservando retornos void/boolean/nullable, los motivos y reglas 409/428. La purga también limpia errores anteriores; la revalidación best-effort no deja rechazos sin capturar. La comparación registró 37 llamadas de puerto antes y después con argumentos de negocio idénticos.

Resultado posterior del padre: **16 PASS / 0 FAIL** en aislamiento y dominio administrativo; ESLint, Prettier y typecheck global pasan en este corte. El aislamiento ampliado no se declara probado contra Go nativo ni desplegado: los gates de integración y revisión final siguen abiertos.

## Bloqueo por revisión independiente de rutas

La revisión `deleg_a48610c3` devolvió `passed: false`. El padre contrastó los hallazgos con las rutas y con el handler Go real (`Authorize(token, "", AutomaticPolling)`). No se acepta el corte ni se publica mientras siga abierto este gate.

Correcciones delegadas por endpoint y con pruebas de ruta (no solo helpers):

- Login: límite de bytes durante lectura, no después de `request.text()`; validación de token/expiry antes de construir cookies y de DTO profundo; rechazo de Authorization del navegador; redirecciones upstream rechazadas; rollback esperado y acotado; aborto antes de emitir cookies.
- Logout: revocación usando un token único y válido aun si el metadato de expiry falta o venció. No confundir ausencia de transporte válido para restaurar con ausencia de credencial que todavía pueda revocarse. Duplicados ambiguos no se adivinan ni se reportan como revocación confirmada.
- Sesión: rechazo de Authorization del navegador, DTO acotado/profundo y redirecciones bloqueadas. Se precisó el contrato: `403` no confirma revocación y debe conservar cookies, sin publicar una identidad autenticada; `401` sí invalida el transporte.

Las pruebas unitarias previas en verde no subsanan estos hallazgos. Continúan pendientes build y gate HTTPS GREEN, prueba nativa BFF→Go y revisión posterior de las correcciones.

## Revisión intermedia del corte delegado

El padre ejecutó la suite completa de tooling: **81 PASS / 0 FAIL**, con dos advertencias `MODULE_TYPELESS_PACKAGE_JSON`. Esto no equivale a typecheck o integración HTTPS aprobados. La migración general se interrumpió preservando el trabajo parcial; reservas y administración conservaban consumidores del contrato antiguo y se dividieron por capas.

La inspección directa del servidor encontró un incumplimiento no cubierto por sus cinco pruebas de helpers: el parser admitía expiry ausente/inválida y el GET de sesión sustituía ese metadato por `Date.now() + 30 min`. **Corrección aplicada y verificada por el padre:** ahora se exige exactamente un token y un metadato RFC3339 válido y futuro; el GET devuelve exclusivamente esa fecha original, sin ventana nueva. Las cinco pruebas de helpers, formato y lint pasan tras la corrección. Esto no sustituye el gate HTTPS completo.

**Guard IAM local aplicado y verificado:** los cinco callsites de servidor/SSR usan `getCloudRunIdentityHeadersForBackend`. La prueba controlada confirma cero invocaciones de metadata para loopback incluso con audience remoto, y conservación de IAM para el destino remoto ensayado. El padre ejecutó después la suite completa: **97 PASS / 0 FAIL / 0 SKIP**, con advertencias `MODULE_TYPELESS_PACKAGE_JSON`. El typecheck de ese momento todavía falla en `use-admin.ts`, cuya migración está en curso. Permanecen abiertas la revisión de rutas BFF (abortos, validación profunda y errores upstream) y la integración HTTPS.

## Actualización verificada — 26 de octubre de 2026

- **HTTPS con upstream controlado: 14 PASS / 0 FAIL, exit 0.** Se ejecutó el BFF Next real, sin interceptarlo, incluyendo recarga, pestaña nueva, cierre/reapertura de Chromium privado, continuidad entre dos procesos Next, CSRF, expiración, logout y superficies sin credencial. Reporte privado: `.local/testing/ui-auth-phase2/controlled-browser-report.json`.
- La sonda de idempotencia original usaba `POST /spaces`, cuyo contrato generado no admite ese header. Se corrigió **solo la sonda y su upstream sintético** para usar `POST /local/booking-trial/reservations`, operación que sí lo declara. No se amplió la allowlist productiva ni se ejecutaron reservas contra Go nativo.
- **HTTPS BFF→Go real→PostgreSQL nativo autorizado: 7 PASS / 0 FAIL, exit 0.** Mismo runner en modo nativo separado. Se comprobó login, restauración por recarga/pestaña/reapertura, cambio de proceso Next, CSRF, logout remoto y ausencia de credencial en superficies JS. Reporte privado: `.local/testing/ui-auth-phase2/native-browser-report.json`.
- Se terminó con SIGTERM el harness nativo propio. Su `server-report.json` confirma DB/usuario coincidentes, **un login en esta ejecución**, login 200, sesiones válidas 200, logout 204 y consulta posterior 401. Una sesión propia comprobada inactiva; cero cuentas nuevas, cero recuperaciones y ningún worker iniciado. Puertos `18084`, `18085`, `18443`, `4330` y `4331` libres al verificar. No se modificaron backend productivo, SQL ni archivos AGENTS.
- **Calidad antes de añadir nuevas regresiones:** `pnpm typecheck`, `pnpm lint`, `pnpm format:check` y `pnpm test` exit 0; **150 pruebas PASS / 0 FAIL / 0 SKIP**. Persisten avisos Node `MODULE_TYPELESS_PACKAGE_JSON`; no se cambia el tipo global de módulos para ocultarlos.
- **Regresión E2E/Axe: 62 PASS / 2 FAIL / 45 SKIP**, de 109 escenarios. Los skips corresponden a suites nativas/entornos desactivados; no cuentan como integración probada. Fallos abiertos: overflow móvil en `/anfitrion/reservas/res-test-1` y pérdida de datos/autorización al navegar de moderación KYC a espacios antes de probar el 409. Diagnóstico delegado en slices independientes; no se debilitan asserts ni se suprimen reglas Axe.
- Inspección posterior detectó que `AccountFacade` y `HostListingsFacade` todavía no tienen barrera post-await equivalente a reservas/administración/verificaciones: sus runtimes siguen construyéndolas solo con el puerto. Se encargaron regresiones RED específicas de respuestas tardías tras `clear()`, antes de implementar cualquier arreglo. El éxito de transporte y las 150 pruebas anteriores **no acreditan aislamiento completo** de estos dos dominios.

### RED de aislamiento cuenta/publicaciones reproducido por el padre

Se leyeron los dos archivos de pruebas delegados y se ejecutó `node --test tooling/account-session.test.mjs tooling/host-listings-session.test.mjs`: **0 PASS / 4 FAIL, exit 1**. Son fallos de aserción sobre las fachadas reales transpiladas, no errores de imports o compilación. Los puertos sintéticos ignoran deliberadamente AbortSignal.

- Cuenta: `loadOverview()` repuebla el perfil después de `clear()`; `exportIdentityBlob()` entrega un Blob privado después de la limpieza.
- Publicaciones: `loadListings()` repuebla el listado y `selectListing()` el detalle después de `clear()`; las pruebas también exigen que no se retorne el payload privado tardío.

Se delegó GREEN limitado a esos métodos y a la invalidación de generación en `clear()`. La suscripción al cambio de sesión y el aislamiento de los demás métodos siguen pendientes: no deben confundirse con el arreglo de estas cuatro regresiones. Se despachó además una revisión independiente read-only del servidor BFF, sin acceso a credenciales o base de datos.

### Revisión independiente del servidor BFF

La revisión `deleg_d6ab100b` devolvió JSON válido con `passed: true`, sin hallazgos de seguridad o lógica en su alcance servidor. El padre repitió las cinco suites indicadas (`bff-session`, `bff-login-route`, `bff-session-route`, `bff-logout-route` y `bff-auth-client`): **58 PASS / 0 FAIL / 0 SKIP, exit 0**.

La sugerencia no bloqueante propone responder explícitamente 502 a 3xx del gateway genérico. La inspección del padre confirma `redirect: "manual"` y ausencia de `Location` en la allowlist de headers de respuesta; las rutas auth ya rechazan redirecciones. No se ha aplicado un cambio adicional al gateway a partir de esta sugerencia. La aprobación se limita al servidor revisado: no subsana las regresiones de cuenta/publicaciones ni los dos fallos E2E, y no equivale a cierre integral o autorización de despliegue.

### Diagnóstico E2E y excepción responsive autorizada

- El padre inspeccionó el fixture administrativo y repitió el escenario KYC→Espacios en `127.0.0.1:4335`: **1 PASS, exit 0**. Causa: no se interceptaba `GET /api/backend/admin/local/published-spaces`; la petición alcanzaba el upstream controlado y recibía 401, disparando correctamente la purga. El fix solo añade la ruta correcta y el envelope `{ data: { items: [] } }` al fixture. No se debilitó la purga ni se modificó AdminFacade para ocultar el fallo.
- El diagnóstico delegado midió el botón «Consultar Política de Reembolso» hasta `right: 407.06px` en viewport de 390px: cabecera rígida `flex items-center justify-between`. Los UUIDs largos no son la causa; el problema ya existía. El E2E podía medir el loader en lugar del contenido y dar un falso positivo.
- **Autorización explícita del usuario, 26 de octubre de 2026:** «Sí, aplicar la corrección responsive mínima». Excepción estrictamente limitada a apilar título y botón de esa cabecera en móvil usando Tailwind (`flex-col`, `gap-2`, con `sm:flex-row sm:items-center sm:justify-between`), conservando la disposición de escritorio. No habilita otro rediseño ni fase 3.
- Corrección entregada: una sola className en la cabecera de reembolso. El padre comparó por AST todas las className del componente contra `HEAD`: misma cantidad y **exactamente una diferencia**, coincidente con la excepción autorizada. También leyó la espera de botón/fuentes en el E2E y ejecutó Prettier y ESLint de ambos archivos: exit 0. El RED delegado, ya esperando contenido real, detectó overflow en `/reservas/res-test-1` móvil, confirmando que ambas variantes de detalle compartían el defecto. **GREEN todavía pendiente:** no se ha reconstruido mientras otros agentes modifican las fachadas; fuente corregida no implica standalone actualizado.

### GREEN parcial cuenta/publicaciones verificado por el padre

El padre releyó las fachadas entregadas y ejecutó conjuntamente `account-session`, `host-listings-session`, `account-domain` y `host-listings-domain`: **16 PASS / 0 FAIL / 0 SKIP, exit 0**; `pnpm typecheck` global también exit 0. Son cinco regresiones de sesión y once pruebas de dominio en este corte.

- Cuenta: `loadOverview()` y `exportIdentityBlob()` capturan generación/controlador local; no publican perfil ni entregan Blob tardío tras `clear()`. Se añadió una tercera regresión de foto tardía, que verifica que tampoco se crea Blob URL después de la limpieza.
- Publicaciones: `loadListings()` y `selectListing()` descartan resultados tardíos y sus `finally` no borran el controlador de otra operación.
- **Alcance todavía parcial:** los runtimes aún necesitan suscripción de identidad/permisos y las otras operaciones privadas requieren guardas. Se delegó ese enlace reactivo solo para los métodos ya corregidos y se encargaron RED adicionales acotados para mutación/descarga de cuenta y creación/tarifa de publicaciones. Las nuevas pruebas RED pueden hacer fallar temporalmente el glob global; no se confunde ese estado con una suite completa en verde.

### Segundo RED de operaciones privadas verificado

El padre leyó `account-operations-session.test.mjs` y `host-listings-operations-session.test.mjs` y ejecutó ambos archivos juntos: **0 PASS / 4 FAIL, exit 1**, por aserciones reales, sin errores de importación. Tras `clear()`, `updateProfile()` vuelve a publicar el perfil, `downloadIdentityArchive()` devuelve el ZIP privado, `createDraft()` repuebla el listado y `loadPricing()` repuebla la tarifa activa. No se alteraron las expectativas para ocultar estas fugas.

Se encoló a los agentes propietarios de las fachadas un siguiente slice limitado a esos dos métodos por dominio, después de verificar el enlace reactivo de sesión que ya ejecutan. No hay dos escritores concurrentes sobre cada fachada. Los demás métodos siguen fuera de ese arreglo parcial.

### Ciclo reactivo y segundo GREEN verificados

El padre releyó constructores, suscripciones y ambos runtimes. Estos ahora inyectan `authFacade` y la purga de blobs. Se ejecutaron las seis suites de sesión/operaciones/dominio de cuenta y publicaciones: **31 PASS / 0 FAIL / 0 SKIP, exit 0**; typecheck global exit 0. Se comprobó purga por cuenta/permisos/expiry, no-fetch sin sesión confirmada y diferenciación 401/403 frente a 503 para los métodos cubiertos.

Las cuatro regresiones del segundo RED ya pasan: perfil, ZIP, creación de borrador y tarifa no se publican/entregan después de `clear()`. En `selectListing`, los 401/403 de consultas secundarias ya no se ocultan como ausencia opcional. **No es aislamiento integral todavía**: se delegó, con un escritor por fachada y prueba de invariancia de argumentos, extender las barreras al resto de operaciones async privadas.

La inspección de UI también encontró contraseñas en estado local de `SecurityCard` sin invalidación por cambio de sesión, fuera del alcance de la purga de fachada. Se delegó una regresión E2E controlada y corrección estrictamente funcional de ese componente, sin cambios de DOM/clases/diseño. No se han usado contraseñas reales ni se ha cambiado ninguna en Go. El nuevo build y los gates finales esperan estas entregas.

### Aislamiento local de SecurityCard: entrega revisada, GREEN de navegador pendiente

El padre leyó el componente y las dos regresiones Playwright controladas. La entrega conserva el árbol DOM original mediante un límite de componente virtual con key de sesión, aborta solicitudes propias al desmontar y usa `authFacade.runAuthenticated` para la operación. Prettier y ESLint de ambos archivos pasan al repetirlos en el padre. Los dos RED delegados detectaron contraseñas sintéticas retenidas después del cambio A→B; no se usaron credenciales reales.

La revisión del padre identificó además ambigüedad en la huella basada en `join`: un probe sobre la función real transpilada reprodujo colisión entre arrays de roles distintos y entre delimitadores de roles/capabilities. Se delegó sustituirla por serialización estructurada sin normalización, verificar igualdad de identidad fresca antes del envío y descartar callbacks de operación abortada/sustituida. **Todavía no se declara GREEN browser:** las dos pruebas deben ejecutarse contra el próximo standalone reconstruido, no contra el anterior.

### Hardening de SecurityCard verificado por el padre

Se leyó la corrección entregada: huella por `JSON.stringify` de la tupla de seis campos con arrays anidados, comparación con identidad fresca antes de enviar y descarte de callbacks abortados/sustituidos. El padre repitió `security-session-key.test.mjs`: **4 PASS / 0 FAIL**, y Prettier/ESLint de los archivos afectados: exit 0.

La comparación AST contra `HEAD` confirma iguales etiquetas y atributos literales/clases del árbol renderizado. No se afirma igualdad textual completa del JSX: el `disabled` del botón cambia legítimamente de comprobar `accessToken` a comprobar sesión autenticada por la migración cookie. Los E2E de limpieza de contraseñas y respuesta tardía aún esperan el standalone nuevo, al terminar los dos trabajos propietarios de fachadas.

### Gate consolidado tras ampliar las fachadas

El padre ejecutó de nuevo `pnpm typecheck && pnpm lint && pnpm format:check && pnpm test` y luego `pnpm build` contra el fixture público loopback: **todos exit 0; 213 unitarias PASS / 0 FAIL / 0 SKIP**. Persisten avisos Node de tipo de módulo no declarado; no hay errores de lint ni tipos. Estos resultados reemplazan los conteos parciales reportados por los implementadores, que no se suman entre sí.

Se verificó independientemente por AST la invariancia de publicaciones contra los dos backups retenidos: **24 llamadas directas a `this.#port` antes y después**, nombres/orden/argumentos de negocio iguales; solo se agregan o sustituyen señales. El reporte delegado de 27 no se adopta como conteo de llamadas directas.

Sobre el standalone nuevo:

- **HTTPS BFF real controlado: 14 PASS / 0 FAIL**, exit 0, repetido después del build.
- **E2E/Axe: 64 PASS / 2 FAIL / 45 SKIP**, de 111 escenarios. Los fallos administrativos y de overflow anteriores ya pasan, incluida la medición del contenido real en móvil.
- Los dos nuevos E2E de SecurityCard ya comprueban correctamente campos vacíos tras A→B; fallan al contar el `role="alert"` global de Next que anuncia el título de ruta, no un error del componente. El padre leyó el snapshot que identifica ese anunciador y delegó limitar las aserciones al contenedor de la tarjeta, sin debilitar las comprobaciones de contraseñas/errores.
- Se delegó también reemplazar dos textos técnicos obsoletos sobre token en memoria/firma propia por la descripción real BFF/HttpOnly, sin rediseño, y se solicitó revisión independiente read-only de las dos fachadas completas y sus runtimes.

### Consolidado GREEN con copy y selectores corregidos

Después de releer los selectores acotados a la tarjeta, el padre volvió a ejecutar calidad, unitarias, build y E2E completos: **214 unitarias PASS / 0 FAIL / 0 SKIP; 66 E2E/Axe PASS / 0 FAIL / 45 SKIP**, de 111 escenarios. Tipos, lint, formato y build exit 0. La suite HTTPS del BFF real también se repitió: **14 PASS / 0 FAIL**, exit 0. Los 45 skips siguen sin contar como integración Go o Docker.

Este consolidado incluye los textos BFF/HttpOnly en el bundle y confirma tanto la limpieza de contraseñas A→B como el descarte del resultado pendiente. `git diff --check` y la comprobación de no modificar `AGENTS.md`, `cmd/` e `internal/` no mostraron diferencias protegidas.

**Pendiente detectado en la inspección final:** el estado React local del asistente de publicación (título/dirección/borrador) no está ligado a identidad, mientras la fachada sí se purga. Se delegó una regresión A→B y un límite virtual de sesión alrededor de los hijos del workspace, sin añadir nodos DOM ni alterar cabecera, clases o composición. El agente posee en exclusiva ese layout/componente/test y reconstruirá para su GREEN; el padre no hará builds concurrentes. La revisión independiente de fachadas continúa separada. El consolidado anterior no cierra este caso nuevo.

### Revisión independiente de fachadas aprobada

La revisión read-only `deleg_b67cd464` devolvió JSON válido con `passed: true`, `security_concerns: []` y `logic_errors: []` para `AccountFacade`, `HostListingsFacade` y ambos runtimes. Confirma guardas antes/después de awaits, purga reactiva, cancelación y diferenciación 401/403 frente a 503. Su comprobación AST coincide con la del padre: 24 llamadas directas al puerto; los tres `.catch()` encadenados explican el conteo previo de 27.

Esta aprobación cierra el gate independiente de esas fachadas, no el del nuevo límite de estado React local del workspace que sigue en ejecución. No se atribuye al revisor aceptación visual ni prueba de operaciones de negocio contra Go nativo.

### Límite virtual de sesión: comprobación global y regresión pendiente

El padre leyó el componente virtual de 20 líneas y volvió a ejecutar tipos/lint/formato, las unitarias (**214 PASS / 0 FAIL**) y HTTPS (**14 PASS / 0 FAIL**). Las regresiones del borrador A→B y de permisos pasan en el standalone reconstruido por el implementador.

La regresión global del padre terminó **67 E2E PASS / 1 FAIL / 45 SKIP**, exit 1: al abrir el menú móvil de dashboard desaparece el enlace Seguridad. Se detectó que la huella incluye la transición inicial `checking → anonymous`, que puede remontar el menú aunque nunca haya existido una identidad confirmada. No se acepta el cierre del componente ni el de fase 2 por sus pruebas aisladas.

Se delegó reproducir esa carrera con GET de sesión diferido y corregir únicamente la estabilidad de la huella sin identidad. Los cambios desde una identidad autenticada y los cambios A→B/permisos deben seguir desmontando todo el estado local privado. El agente mantiene propiedad exclusiva del componente/test y del siguiente build; no se cambian cabecera, navegación o estilos para ocultar el fallo.

## Cierre técnico local — 26 de octubre de 2026

El padre releyó la corrección del límite virtual: huella estable `guest` mientras no existe identidad confirmada; huella estructurada de los seis campos para autenticados. Se conserva así el menú durante el GET inicial diferido y se descarta el estado privado al perder identidad, cambiar de cuenta o de permisos.

| Gate final repetido por el padre | Resultado |
| --- | --- |
| Typecheck, ESLint, Prettier y build de producción | PASS, exit 0 |
| Suite unitaria completa | 214 PASS / 0 FAIL / 0 SKIP |
| Playwright/E2E/Axe, 114 escenarios | 69 PASS / 0 FAIL / 45 SKIP |
| HTTPS, BFF Next real con upstream controlado | 14 PASS / 0 FAIL |
| HTTPS, BFF Next real → Go identidad real → PostgreSQL nativo autorizado | 7 PASS / 0 FAIL |
| Revisión independiente de servidor BFF y de cuenta/publicaciones | Aprobadas, sin bloqueos |
| `git diff --check` y archivos protegidos/backend sin cambios | PASS |

El gate nativo se repitió contra el build final usando exclusivamente el inventario sintético existente. Tras SIGTERM se leyó `server-report.json`: mismo destino/usuario autorizados, un intento de login en esa corrida, login 200, sesiones 200, logout 204 y posterior 401; una sesión propia comprobada inactiva. Cero cuentas creadas, cero recuperaciones, sin workers. Los reportes nativos anteriores se copiaron a archivos privados `*-before-final-20261026.json` antes de la nueva corrida.

Se detuvieron únicamente el harness nativo y el fixture propios; los runners cerraron sus Next/TLS/Chromium. Se verificaron libres los puertos de fase `18084`, `18085`, `18088`, `18443`, `4329`, `4330`–`4340` utilizados. No se borraron secretos, reportes, bases, volúmenes ni recursos retenidos. PostgreSQL y servicios previos no se detuvieron.

### Límites y publicación

- Los 45 escenarios omitidos pertenecen a suites nativas/Docker históricas desactivadas; no se contabilizan como probados. El gate de identidad Go se acredita por el runner HTTPS nativo separado, no por esos skips.
- No se certifican operaciones de negocio de todos los dominios contra Go nativo, login con cuentas de producción, acceso privado IAP interactivo ni aceptación visual de la paleta. Persisten avisos Node no bloqueantes `MODULE_TYPELESS_PACKAGE_JSON`.
- La excepción responsive de reembolso fue autorizada explícitamente y conserva escritorio; no se reconstruyeron vistas ni se inició fase 3. Las cabeceras/identidad visible y el logo de acceso siguen siendo entregas posteriores.
- La identidad de publicación se comprobó como `HernanEspinozaDev` tanto en `gh` como en Git. La paridad read-only con la API desplegada comprobó 447 archivos backend idénticos; API `espacigo-test-api-00021-55v` intacta.
- **Fase 2 cerrada técnicamente en local.** La autorización previa de push directo a `main` y el trigger regional existente se gestionan como entrega separada. Este documento no afirma anticipadamente push o despliegue: consultar el SHA remoto, su build y el reporte de release bajo `.local/deployments/`, conforme a `planning/gcp/operacion_cloud_build_next.md`. No crear un segundo pipeline ni ejecutar publicación completa de Go/SQL.
