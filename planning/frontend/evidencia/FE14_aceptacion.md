# FE-14 — Registro de aceptación visual y artefactos exactos

Source commit E2E: `6277140b8fef7226fe29d10284b67d36d943d780` (runner Angular/Playwright construido desde este árbol).

## Reensayo de compatibilidad Compose — 2026-10-12

- Cambio del hilo de compatibilidad: commit `5610369` mueve `angular-e2e` y sus tres bind mounts obligatorios desde `compose.frontend.yaml` a `compose.frontend-e2e.yaml`. El stack normal carga solo el primero; `scripts/test-frontend-e2e.sh` carga ambos overlays para FE-14. `accounts.json` sigue `read_only`; artefactos/resultados son los únicos directorios escribibles del runner. No se montan secretos, TLS, evidencia privada ni datos de replay.
- Compatibilidad base comprobada con `FE14_*` y `LOCAL_FRONTEND_E2E` ausentes: `config-frontend` terminó correctamente y su configuración no contiene `angular-e2e`. Al cargar explícitamente el overlay E2E sin esas rutas, Compose falla como corresponde con `FE14_ACCOUNT_FILE_HOST is required`; el requisito queda confinado a ese flujo.
- Ensayo Docker FE-13 repetido: proyecto/volumen desechables `espacigo_fe13_20261012215917_1723834` / `espacigo_fe13_20261012215917_1723834_pgdata`; puertos API/Angular 18680/18683. Pasaron build y healthcheck, Angular sin credenciales DB, solo red API y sin mock; `verify-frontend` confirmó URL pública, readiness y CORS. La caída dejó Go sin readiness y Angular saludable; la restauración volvió a pasar readiness y verify. La limpieza retiró el proyecto y volumen de prueba; no se tocó el volumen persistente.
- E2E autenticada repetida: proyecto/volumen `espacigo_fe14_13641c8c087f` / `espacigo_fe14_13641c8c087f_pgdata`; puertos API/Angular/Mailpit 18780/18783/18782. Resultado Playwright: 3/3. Verificó el aislamiento del runner, roles/tercero/descargas privadas y fallo/restauración real de Go con la misma cuenta, HTTP 200 autenticado, respuesta demorada controlada y sin nuevo login. Logs backend sin correos ni contraseñas sintéticas. El volumen y archivos temporales fueron eliminados al terminar.
- Imágenes locales exactas (image ID): FE-13 Angular `sha256:a3c9bffe71dba413b3b3c3d2979fb6d7f2d132f7f16bdafbcfc6038676bd3ef8`, runner `sha256:52aa5956d348949582efedbbbc636661a845419d59b097d4afcdd76fa572e8b6`, API `sha256:5c635f63c6454685280054cc533aa4ea073b26a54a84c8822b8604f7bac2ea0a`, DB `sha256:868c4276a780e3f9f83c0a499c6d5c25cada346de9662bce5d206bfc694061f6`. FE-14 Angular `sha256:01cae4e060f623d0724bf49f84d9b189add4b6cd9e74f5c7ba9e4b6c7442e29c`, Playwright runner `sha256:6909d47498a9498bc59796a701ca1df8430b0c30710d4a2c20621020659a11e1`, API `sha256:d0770f51f3eb2ca5a9b36a22ba6be6a930dcb698452e3851a645a2c6e58496e2`, DB `sha256:ed19243529d03bf682461aacc021bef2497abd40f75844a0a815aae576be032a`.
- Capturas JPEG revalidadas con Chromium. Todas las seis vistas de rol/viewport mantienen ancho de documento igual a su viewport. Se actualizaron hashes y altura de documento para las capturas de arrendatario y administrador móvil regeneradas en esta corrida.

## Imágenes reproducibles

| Vista | Viewport CSS | Documento CSS | Archivo JPEG | SHA-256 |
| --- | ---: | ---: | --- | --- |
| renter mobile | 390×844 | 390×3176 | [`fe14_renter_mobile.jpg`](fe14_renter_mobile.jpg) | `8ca6f49bbf633e09e54c5d923b620ca557e8f830c3ccbf3915a61799080f45be` |
| renter desktop | 1365×900 | 1365×2339 | [`fe14_renter_desktop.jpg`](fe14_renter_desktop.jpg) | `2c6b42eb1058f39d6d00ba67f1286f43279592f1a8cc2ae61e5189046bbaa04c` |
| host mobile | 390×844 | 390×3387 | [`fe14_host_mobile.jpg`](fe14_host_mobile.jpg) | `396566a29b5f96cfd4af17d1386aba7ac33bbbb4cb10954dc1d1e8a5e9859766` |
| host desktop | 1365×900 | 1365×2230 | [`fe14_host_desktop.jpg`](fe14_host_desktop.jpg) | `89d316021e6b4076d65f47873e9127d6a01183c913feb557bee018d9b385fa0b` |
| admin mobile | 390×844 | 390×1253 | [`fe14_admin_mobile.jpg`](fe14_admin_mobile.jpg) | `b858f2076cffa9178e254f47b3989ef3881b654f0cf1da52e560b94791850aa9` |
| admin desktop | 1365×900 | 1365×1069 | [`fe14_admin_desktop.jpg`](fe14_admin_desktop.jpg) | `e202625432b1d44a55a79fb7933fe56fadf92df4fef726de14bbaf823a26d00c` |
| restored session desktop | 1365×900 | viewport capturado | [`fe14_restored_session_desktop.jpg`](fe14_restored_session_desktop.jpg) | `f77a47fe13b0e8b5c0f47b83ce81222b31dce50d5df3b7458a3756ef65ad679e` |

Cada JPEG es una captura real de Chromium con sesión sintética ya autenticada. En los seis documentos con medición el ancho del documento coincide con el viewport; el test falla si hay overflow horizontal. Los JSON conservan medidas CSS exactas. `restored_session_desktop` es una captura de viewport 1365×900 tomada tras restaurar Go en la misma pestaña y sesión.

## Ejecución y contenido

- Base: `main` posterior a PR #288, `4eaf98fcd0a46326c884e4f3236d7e55ff39e0df`.
- E2E source: `6277140b8fef7226fe29d10284b67d36d943d780`; Compose compatibility fix: `5610369`.
- Imagen Angular local: `sha256:5b519c5e467d15fa8de3dad5ea871cbcc05a25b65c2537dcab15657b5a3e0a81`.
- Imagen runner E2E: `sha256:e0ea89b0f3dcf39dd201e3791b0beb489b74d5976eaa2751cb98ae5fa6b550ee`.
- Base Playwright: `mcr.microsoft.com/playwright:v1.56.1-noble@sha256:f1e7e01021efd65dd1a2c56064be399f3e4de00fd021ac561325f2bfbb2b837a`.
- Stack desechable: `espacigo_fe14_98d604e66706`; API `18780`, Angular `18783`, Mailpit `18782`; readiness Go y health Angular HTTP 200 antes de login.
- Resultado: 3 pruebas E2E Playwright aprobadas; JSON y ZIP descargados desde Go por controles Angular y validados (scope propio, ausencia de token/contraseña, estructura ZIP y archivos); tercero sin acceso host/admin; caída real observada como `requestfailed` autenticada, alerta/error y cero tarjetas; tras restaurar, el mismo UUID y rol sobreviven sin segundo login, y `GET /verifications/eligibility` con Bearer responde HTTP 200. Un gate de respuesta demorada mantiene spinner y cero tarjetas hasta liberar la respuesta; luego la UI renderiza las dos entradas iguales a Go. Backend logs sin correos ni contraseñas sintéticas.
- Accounts: cuatro usuarios de dominio `.invalid`, password aleatorio y archivo temporal modo 0600; roles host/admin sembrados solo en el PostgreSQL nuevo descartable para probar guards. Go confirmó login, sesión y roles efectivos. No se guardó Bearer en almacenamiento web ni en credenciales persistentes.
- Montajes runner: Docker inspect confirmó solo `accounts.json` `rw=false`, `artifacts` y `results` `rw=true`. Prueba dentro del runner confirma cuenta legible pero no escribible, solo tres entradas visibles en `/run/fe14`, secretos PostgreSQL/secretos backend, `private-evidence`, `privacy-replay` y TLS ausentes e ilegibles.
- Limpieza de la ejecución E2E final: se eliminaron contenedores, red y volumen únicos; no quedan recursos `espacigo_fe14_98d604e66706`.
- Incidencia previa: un primer `up-frontend` abortó al intentar publicar Mailpit en un puerto ocupado, antes de aprovisionar cuentas o lanzar E2E. Faltaban variables explícitas de Compose y el proyecto por defecto montó `espacigo_pgdata`; el contenedor DB persistente se reinició (su volumen existente no fue eliminado). No se ejecutaron comandos de prueba destructivos sobre esa base. El contenedor persistente fue verificado saludable. La corrida E2E exitosa descrita arriba usó otro proyecto y un volumen FE-14 nuevo, sin referencia al persistente.
- Gate consolidado sobre el mismo corte: Angular 169 pruebas en 28 archivos; `api:check` pasó; build/prerender pasó con 3 rutas.

## Límites

Estas capturas cubren estado de cuenta/privacidad, panel de anfitrión, cola administrativa y KYC después de restauración; las pruebas específicas por operación y sus capturas permanecen en FE-02–FE-13. FE-14 no repite todas las mutaciones de cada dominio. No es certificación WCAG. El mock sigue presente y no se retira antes de la aceptación explícita del usuario.


## Demo de aceptación sobre main con la adecuación de #290

- PR #290: aprobado y fusionado. SHA exacto de `main`: `b94dba37d80fa63c92d3e8aedffe8d6740ce2e0d`. La imagen Angular del stack se reconstruyó desde ese checkout y prerenderizó 3 rutas.
- Stack aislado y activo: proyecto `espacigo_fe14_demo_18a028644c87`; volumen exclusivo `espacigo_fe14_demo_18a028644c87_pgdata`; Angular `http://127.0.0.1:18883/`, Go `http://127.0.0.1:18880/api/v1`, Mailpit `http://127.0.0.1:18882/`. Verificaciones: frontend HTTP 200, `/health/ready` de Go HTTP 200, CORS preflight HTTP 204, runtime config con `apiBaseUrl=http://127.0.0.1:18880/api/v1`.
- Cuatro cuentas `.invalid` creadas por registro/verificación con Go y Mailpit. Go confirma `arrendatario`, `arrendador` y `administrador`; los roles especiales se añadieron únicamente al PostgreSQL exclusivo de demo. El tercero recibe HTTP 404 al consultar recursos ajenos.
- El anfitrión y el arrendatario completaron KYC sintético por los formularios normales; la revisión/decisión fue realizada desde la cola Angular de administración, Go confirmó el resultado y ambas elegibilidades aparecieron como `Elegible`. El anfitrión guardó una tarifa horaria CLP 12.000, zona `America/Santiago` y publicó el borrador desde su panel normal; Go respondió HTTP 200 y la publicación apareció en búsqueda/detalle/horarios. No se asignaron roles por la UI.
- Los secretos están en `.local/testing/fe14-demo/cuentas.md` y el JSON temporal local, ambos con modo `0600`, excluidos por `.git/info/exclude`/`.gitignore`; Bearer solo se usó en memoria durante las verificaciones. La guía local es `.local/testing/fe14-demo/README.md`; la pestaña del navegador quedó abierta en `/acceso`. No se publicaron secretos ni se cambió el worktree principal con cambios ajenos.
- El mock permanece en el repositorio. FE-15 y la aceptación de FE-14 siguen pendientes de la decisión explícita del usuario.
- Imágenes exactas reconstruidas desde el merge `b94dba37d80fa63c92d3e8aedffe8d6740ce2e0d`: Angular `sha256:9d7b0d000819b4596e8a5871d246aad8c5858519ac9e0ff16b338ae69e98f7a8`; API Go `sha256:44515588d20e8b551b083b7b10ee4e6d34d3fffb972f1ad5467551e353cebd6c`; PostgreSQL/PostGIS `sha256:1e1155e20be35662342e7edf68ed835da05f7bde5721a4d0119f0a2d8eefec1e`. La configuración local de endpoints y el inventario sintético permanecen bajo `.local/testing/fe14-demo/`; el inventario no contiene credenciales.


## Recorrido Angular conectado a Go — 2026-10-13, HEAD `b94dba37`

**Alcance demostrado:** se ejecutó sobre el merge exacto de #290, con Angular `http://127.0.0.1:18883` y Go `http://127.0.0.1:18880/api/v1`, proyecto/volumen desechables `espacigo_fe14_demo_18a028644c87` / `espacigo_fe14_demo_18a028644c87_pgdata`. Angular respondió HTTP 200, readiness Go HTTP 200 y CORS preflight HTTP 204. La cuenta runtime apuntó al API local. Las credenciales permanecen solo en `.local/testing/fe14-demo/cuentas.md` y `accounts.json` (0600, localmente excluidos); el runner solo puede leer el fixture de cuentas y no monta secretos del backend. El Bearer no se persistió en `localStorage` ni `sessionStorage`.

| Paso | Resultado observado |
| --- | --- |
| KYC y elegibilidad | Dos participantes cargaron evidencia PNG fixture permitida; la cuenta admin revisó/aprobó en Angular. Go confirmó ambas elegibilidades. |
| Publicación | El anfitrión guardó tarifa horaria CLP 12.000 y `America/Santiago`, luego publicó desde el panel normal. Go respondió 200; la búsqueda autenticada, el detalle y la disponibilidad de Go devolvieron datos. |
| Búsqueda, cotización, reserva y pago | Angular consultó búsqueda/horarios, cotizó HTTP 200, creó reserva HTTP 201 y confirmó pago fake HTTP 200. Se usaron fechas/intervalos que devolvió Go. |
| Garantía y decisión | La operación de autorización de garantía devolvió HTTP 202 con operación confirmada; la lectura posterior de Go mostró estado `autorizada`. La aprobación del anfitrión respondió HTTP 200. |
| Contrato y PDF privado | El anfitrión y luego el arrendatario firmaron por Angular; Go dejó la reserva `lista_para_checkin` y el contrato `firmado` con dos firmas. La descarga autenticada de PDF comienza con `%PDF`. |
| Exportaciones/tercero | JSON y ZIP se descargaron desde la vista privada del titular; se validaron alcance propio, `PK`, `manifest.json`/`data.json`, y ausencia de Bearer/contraseña en el JSON. La cuenta tercera obtuvo rechazo/404 y no vio el panel de contrato. |
| Operación y finanzas | **Límite real:** el POST de check-in desde la UI recibió HTTP 409. Go exige que el check-in ocurra en el mismo día IANA del inicio; la reserva seleccionada inicia el 13-10-2026 a las 02:00 America/Santiago y la demo se ejecutó el 12-10 local a las 23:xx. La consulta Angular de intervalos para el día 12 no ofreció horarios. No se alteró reloj, inicio/fin, volumen persistente ni estado para sortear esta regla. Sin check-out confirmado no se solicita liquidación: Go aplica un plazo de 24 horas desde check-out. No se declara completada recepción, operación ni liquidación. |

La reserva anterior del ensayo, cuyo inicio ya había pasado durante la ejecución prolongada, quedó `cancelada_por_firma` por expiración normal de Go. Se conserva como historial y no se reescribió. La reserva nueva para el 13-10 quedó firmada y lista para check-in, sin forzar operaciones. Los ensayos UI que validan 409 no cambiaron el estado.

### Capturas de Chromium (JPEG real)

Todas las medidas son CSS; el test comprobó que el ancho del documento no excediera el viewport. Los JPEG y JSON están en [`aceptacion-fe14-b94dba37/`](aceptacion-fe14-b94dba37/), capturados sobre el stack anterior y la vista señalada, sin correo ni contraseña.

| Vista | Viewport | Documento | SHA-256 JPEG |
| --- | ---: | ---: | --- |
| Búsqueda móvil | 390×844 | 390×2461 | `37cedfae84b50da519b18f6e408d67083e3a2dcb257a294e3af2b4d4a820eb48` |
| Búsqueda escritorio | 1365×900 | 1365×1672 | `467fbfcfa51726733975c1e1377c64dc885a02bec5cebd5dc69abfa37499fbb2` |
| Detalle/horarios móvil | 390×844 | 390×2073 | `d15f2ee7b4a855c5a5c217df8a314efc038a5a162e6cd8064f14caa7a5aa7439` |
| Detalle/horarios escritorio | 1365×900 | 1365×1320 | `e7db7714c8c58877d2f580ff4ec6164d11e069f4b85de9e5c8228f9c0f36f701` |
| Pago fake móvil | 390×844 | 390×3643 | `6aa0b796bbd48ead3e81cb852b1eae841a758bfb8b5854e3a2dbd61733b893d8` |
| Contrato/PDF privado móvil | 390×844 | 390×4688 | `c56665aae8e577e3fe3105d4a20a8f3facc9d0da25b288eeb0a233db58c349a2` |
| Contrato/PDF privado escritorio | 1365×900 | 1365×4177 | `24154801bfb72b423044f009fd3fa10a192668491f6a143a911a995f0314afd0` |
| Finanzas móvil | 390×844 | 390×3155 | `00a1b5e8fb65e6f0d069e02299397543c6cfaf420622718f11679687305cce15` |
| Finanzas escritorio | 1365×900 | 1365×2144 | `b18e6b4d42db8b0e64b290d06943f4a5175c3fe06cac05e338eed8bb958d5aab` |
| Privacidad/descargas móvil | 390×844 | 390×3216 | `19a3b3f020aa49c639bd15d79c2a2b04ec3c1b7e715c440ad14db2f7cad6caae` |
| Privacidad/descargas escritorio | 1365×900 | 1365×2379 | `638a164d3c48a3c35f46ed6df665954cb10bcc66eaa55323bb46185c8ca366fa` |
| Check-in rechazado por fecha | 390×844 | 390×4213 | `5caab99c973c7f17e9b7930efed7678500cfc18b7c74fe94a599f67799728949` |
| Reserva `lista_para_checkin`, pago conservado y HTTP 409 vigente | 390×844 | 390×4213 | `801032ba7c8ba9b2e22956c6d4083b5d371d754b2b19a508766571ca0ecb48f8` |

**Estado de aceptación:** evidencias y límites listos para revisión del usuario. La secuencia de operación posterior a la firma está pendiente porque el calendario de Go no habilita un intervalo local hoy; no se reemplaza con mocks ni con manipulación de fechas. No se declara FE-14 aceptado. El mock sigue presente y FE-15 no comenzó.


## Revisión adicional de FE-14 — pago confirmado y operaciones (2026-10-13)

El inventario inicial que decía `borrador`/KYC `en_revision` quedó supersedido por el recorrido conectado. `.local/testing/fe14-demo/fixture-manifest.md` ahora enumera KYC aprobado/elegible para ambos participantes, publicación normal activa y reserva `2e80f28e-6bbf-4213-bb59-a06079dd31c3` en `lista_para_checkin`. No contiene credenciales; las credenciales siguen solo en los archivos locales ignorados con modo 0600.

Se corrigió el resumen del detalle para que no pierda el pago ya confirmado por Go cuando la reserva avanza. La regresión de componente con `IdentitySession` real recorre `pagada → aprobada_host → firma_parcial → lista_para_checkin`, refresca desde Go en cada avance y mantiene “Confirmado por Go”. En la sesión Angular conectada el resumen aparece también para la reserva viva en `lista_para_checkin`.

El runtime local de Angular se corrigió fuera del código del PR: `/config.json` anuncia `apiBaseUrl=http://127.0.0.1:18880/api/v1`. Readiness Go y Angular devolvieron HTTP 200. A las 23:xx del 12-10 en `America/Santiago`, Go rechazó el check-in desde la UI con HTTP 409: la reserva inicia el 13-10 a las 02:00 IANA. No se cambió fecha, hora, reloj, estado ni volumen. Por la misma regla no se registraron check-out ni recepción; ambos siguen pendientes hasta que Go autorice el check-in y la operación. La captura real de Chromium/Angular es [`fe14_acceptance_checkin_waiting_for_iana_day_mobile.jpg`](aceptacion-fe14-b94dba37/fe14_acceptance_checkin_waiting_for_iana_day_mobile.jpg); su JSON asociado registra viewport 390×844 CSS px y documento 390×4213. SHA-256 JPEG `801032ba7c8ba9b2e22956c6d4083b5d371d754b2b19a508766571ca0ecb48f8`. La imagen muestra el pago confirmado, reserva lista y el resultado del check-in; no muestra correo ni contraseña.

La liquidación viva no es elegible sin check-out y 24 horas reales. Se ejecutó, en cambio, `TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay` con un PostgreSQL/PostGIS nuevo efímero sobre `tmpfs`, ajeno al stack de demo y al volumen persistente. Pasó el avance de reloj sustituible hasta el límite exacto, liquidación fake, cierre, comprobante único/replay y conciliación; el contenedor temporal se eliminó al terminar. Esto cubre el camino aislado previsto en el plan y no modifica las fechas del fixture vivo.

**Gates del cambio:** Angular 179 pruebas/32 archivos; `npm run api:check` pasó; `npm run build` pasó y prerenderizó 3 rutas; la prueba Go aislada indicada pasó. E2E de roles, descargas y caída/restauración siguen como evidencia previa del merge #290; el recorrido vivo de check-in quedó bloqueado por el día local IANA, así que no se reportan check-out/recepción completados.

Se solicita revisión adicional antes de cerrar la aceptación FE-14. El mock permanece y FE-15 no se inicia hasta la aceptación explícita del usuario.
