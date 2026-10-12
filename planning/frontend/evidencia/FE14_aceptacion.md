# FE-14 — Registro de aceptación visual y artefactos exactos

Source commit E2E corregido: `6277140b8fef7226fe29d10284b67d36d943d780` (runner Angular/Playwright construido desde este árbol).

## Imágenes reproducibles

| Vista | Viewport CSS | Documento CSS | Archivo JPEG | SHA-256 |
| --- | ---: | ---: | --- | --- |
| renter mobile | 390×844 | 390×3152 | [`fe14_renter_mobile.jpg`](fe14_renter_mobile.jpg) | `ca6cab6457877c47bb567c929ea13c1d3fb1389db7bfaba517a9bfb2bcc51157` |
| renter desktop | 1365×900 | 1365×2339 | [`fe14_renter_desktop.jpg`](fe14_renter_desktop.jpg) | `4db8780673dd2722f82d02a2fff4bc04c42adf10337fb259a07c71e86843a2d9` |
| host mobile | 390×844 | 390×3387 | [`fe14_host_mobile.jpg`](fe14_host_mobile.jpg) | `396566a29b5f96cfd4af17d1386aba7ac33bbbb4cb10954dc1d1e8a5e9859766` |
| host desktop | 1365×900 | 1365×2230 | [`fe14_host_desktop.jpg`](fe14_host_desktop.jpg) | `89d316021e6b4076d65f47873e9127d6a01183c913feb557bee018d9b385fa0b` |
| admin mobile | 390×844 | 390×1377 | [`fe14_admin_mobile.jpg`](fe14_admin_mobile.jpg) | `b858f2076cffa9178e254f47b3989ef3881b654f0cf1da52e560b94791850aa9` |
| admin desktop | 1365×900 | 1365×1069 | [`fe14_admin_desktop.jpg`](fe14_admin_desktop.jpg) | `e202625432b1d44a55a79fb7933fe56fadf92df4fef726de14bbaf823a26d00c` |
| restored session desktop | 1365×900 | viewport capturado | [`fe14_restored_session_desktop.jpg`](fe14_restored_session_desktop.jpg) | `f77a47fe13b0e8b5c0f47b83ce81222b31dce50d5df3b7458a3756ef65ad679e` |

Cada JPEG es una captura real de Chromium con sesión sintética ya autenticada. En los seis documentos con medición el ancho del documento coincide con el viewport; el test falla si hay overflow horizontal. Los JSON conservan medidas CSS exactas. `restored_session_desktop` es una captura de viewport 1365×900 tomada tras restaurar Go en la misma pestaña y sesión.

## Ejecución y contenido

- Base: `main` posterior a PR #288, `4eaf98fcd0a46326c884e4f3236d7e55ff39e0df`.
- E2E source: `6277140b8fef7226fe29d10284b67d36d943d780`.
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
