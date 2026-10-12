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
