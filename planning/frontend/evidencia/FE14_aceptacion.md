# FE-14 — Registro de aceptación visual y artefactos exactos

Source commit E2E: `251b8722164640fba819bbdde9e1a8a4efd51d4c` (runner Angular/Playwright construido desde este árbol).

## Imágenes reproducibles

| Vista | Viewport CSS | Documento CSS | Archivo JPEG | SHA-256 |
| --- | ---: | ---: | --- | --- |
| renter mobile | 390×844 | 390×3152 | [`fe14_renter_mobile.jpg`](fe14_renter_mobile.jpg) | `18f349f8f799c6d009a0d3cf8319b6957074e0601eec71cf89f7977548b21052` |
| renter desktop | 1365×900 | 1365×2339 | [`fe14_renter_desktop.jpg`](fe14_renter_desktop.jpg) | `cf7c5b11a62c90f833a9be303c2cfd81f2a96c9203a9265c925227ed44902fc8` |
| host mobile | 390×844 | 390×3387 | [`fe14_host_mobile.jpg`](fe14_host_mobile.jpg) | `396566a29b5f96cfd4af17d1386aba7ac33bbbb4cb10954dc1d1e8a5e9859766` |
| host desktop | 1365×900 | 1365×2230 | [`fe14_host_desktop.jpg`](fe14_host_desktop.jpg) | `89d316021e6b4076d65f47873e9127d6a01183c913feb557bee018d9b385fa0b` |
| admin mobile | 390×844 | 390×1377 | [`fe14_admin_mobile.jpg`](fe14_admin_mobile.jpg) | `b858f2076cffa9178e254f47b3989ef3881b654f0cf1da52e560b94791850aa9` |
| admin desktop | 1365×900 | 1365×1069 | [`fe14_admin_desktop.jpg`](fe14_admin_desktop.jpg) | `e202625432b1d44a55a79fb7933fe56fadf92df4fef726de14bbaf823a26d00c` |
| restored session desktop | 1365×900 | viewport capturado | [`fe14_restored_session_desktop.jpg`](fe14_restored_session_desktop.jpg) | `f77a47fe13b0e8b5c0f47b83ce81222b31dce50d5df3b7458a3756ef65ad679e` |

Cada JPEG es una captura real de Chromium con sesión sintética ya autenticada. En los seis documentos con medición el ancho del documento coincide con el viewport; el test falla si hay overflow horizontal. Los JSON conservan medidas CSS exactas. `restored_session_desktop` es una captura de viewport 1365×900 tomada tras restaurar Go en la misma pestaña y sesión.

## Ejecución y contenido

- Base: `main` posterior a PR #288, `4eaf98fcd0a46326c884e4f3236d7e55ff39e0df`.
- E2E source: `251b8722164640fba819bbdde9e1a8a4efd51d4c`.
- Imagen Angular local: `sha256:59e8db601369b6845763857de041a725660264eb3458d621248a6d748855a006`.
- Imagen runner E2E: `sha256:9f12ba89d6ddc0f35385251de0d016234aa3fe186166102ca8e8bff33ab1c3d2`.
- Base Playwright: `mcr.microsoft.com/playwright:v1.56.1-noble@sha256:f1e7e01021efd65dd1a2c56064be399f3e4de00fd021ac561325f2bfbb2b837a`.
- Stack desechable: `espacigo_fe14_916a8cf5c305`; API `18780`, Angular `18783`, Mailpit `18782`; readiness Go y health Angular HTTP 200 antes de login.
- Resultado: 2 pruebas E2E Playwright aprobadas; JSON y ZIP descargados desde Go por controles Angular y validados (scope propio, ausencia de token/contraseña, estructura ZIP y archivos); tercero sin acceso host/admin; estado privado recuperado al restaurar backend en el mismo documento. Backend logs sin correos ni contraseñas sintéticas.
- Accounts: cuatro usuarios de dominio `.invalid`, password aleatorio y archivo temporal modo 0600; roles host/admin sembrados solo en el PostgreSQL nuevo descartable para probar guards. Go confirmó login, sesión y roles efectivos. No se guardó Bearer en almacenamiento web ni en credenciales persistentes.
- Limpieza de la ejecución E2E final: se eliminaron contenedores, red y volumen únicos; no quedan recursos `espacigo_fe14_916a8cf5c305`.
- Incidencia previa: un primer `up-frontend` abortó al intentar publicar Mailpit en un puerto ocupado, antes de aprovisionar cuentas o lanzar E2E. Faltaban variables explícitas de Compose y el proyecto por defecto montó `espacigo_pgdata`; el contenedor DB persistente se reinició (su volumen existente no fue eliminado). No se ejecutaron comandos de prueba destructivos sobre esa base. El contenedor persistente fue verificado saludable. La corrida E2E exitosa descrita arriba usó otro proyecto y un volumen FE-14 nuevo, sin referencia al persistente.
- Gate consolidado sobre el mismo corte: Angular 169 pruebas en 28 archivos; `api:check` pasó; build/prerender pasó con 3 rutas.

## Límites

Estas capturas cubren estado de cuenta/privacidad, panel de anfitrión, cola administrativa y KYC después de restauración; las pruebas específicas por operación y sus capturas permanecen en FE-02–FE-13. FE-14 no repite todas las mutaciones de cada dominio. No es certificación WCAG. El mock sigue presente y no se retira antes de la aceptación explícita del usuario.
