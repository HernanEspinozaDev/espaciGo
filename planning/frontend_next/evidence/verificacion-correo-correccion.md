# Corrección de verificación de correo — diagnóstico y ejecución

## Encargo y límites

El usuario aprobó el diseño publicado y reportó error visible al confirmar el enlace, seguido de acceso posible. Pidió garantizar bloqueo mientras el correo siga pendiente, explicar cómo verificar antes de entrar, reconciliar estados ya confirmados y agregar el logo real al correo. Se conserva diseño/paleta/Tailwind y la autoridad de Go. No se usa cuenta, enlace ni navegador del usuario; no se crean cuentas o envíos reales para probar. No se alteran esquema, documentos legales, IAM o infraestructura por inferencia.

Base verificada: `2d2cd2afe76aa128f9449db979790a423087efd0`, local/remoto iguales y árbol limpio antes del trabajo. Fecha local del preflight: 2026-10-26 23:35 -03.

## Evidencia primaria

- Lectura agregada y sanitizada de logs operativos: una confirmación llegó a Go y devolvió **204**, mientras la web devolvió **502**. Se observaron además cuatro 400 en el gateway y tres login 200 en ambas capas. Se conservaron únicamente servicio/ruta/status/conteos, sin cuerpos, direcciones, cuentas, IP ni credenciales.
- El handler Go llama `VerifyEmail` antes de responder 204. La consumición del enlace y activación se confirman atómicamente. Login verifica contraseña y rechaza `AccountEmailPending` mediante `ErrEmailUnverified` antes de crear sesión.
- Hipótesis de constructor `Response` refutada: el gateway ya usa cuerpo null para 204/205/304 y HEAD.
- Reproducción local sobre el handler gateway real, transpilado sin editarlo: upstream 204 sin Content-Type o con application/json produce 204; el mismo 204 con text/plain u application/octet-stream produce **502 / upstream_content_type_not_allowed**. La comprobación de media type se aplica indebidamente a una respuesta que no admite contenido. No se obtuvo la cabecera exacta de la operación humana y no se presenta como capturada.

El error visual no prueba que la cuenta siguiera pendiente: el estado pudo haberse confirmado antes del rechazo de transporte. No se desactiva acceso de cuentas verificadas por esa discrepancia ni se inventa una segunda bandera de verificación en React.

## Prueba Go/PostgreSQL previa al cambio

Se ejecutaron seis pruebas focalizadas mediante `scripts/test-m04-attributes-postgres.sh`, exclusivamente en PostGIS desechable `--network none`/tmpfs, sin volumen ni conexión a las dos instancias locales retenidas. Cinco pruebas principales pasaron: registro/login no verificado, enlaces reemitidos/vencidos/usados, seguridad de desbloqueo de pendientes, atomicidad/entrega y conservación de bloqueo previo. La prueba legacy de roles/actividad falló en cinco subcasos por asumir rol arrendatario automático, contrario al contrato vigente de registro sin rol. Se corregirá únicamente su fixture explícito, sin otorgar roles en producción ni eliminar la aserción.

## Cortes en ejecución

1. Gateway: regresiones RED/GREEN sobre el handler real para respuestas sin cuerpo; mantener rechazo de cuerpos con media type no autorizado, CSRF, cookies e IAM. Compatibilidad DNT solo como metadato descartado, nunca reenviado ni tratado como autenticación.
2. Acceso/verificación: mensaje claro para correo pendiente y enlace para instrucciones; resultado 204 sin error contradictorio; enlace usado ofrece acceso o reemisión; respuesta ambigua ofrece comprobar acceso sin reintentar automáticamente un token de un solo uso. No autologin por un flag cliente ni confusión entre cuenta de sesión y cuenta del enlace.
3. Prueba visible local sin interceptar la API: Chromium → Next BFF → handler Go → PostgreSQL desechable, desde registro hasta login bloqueado, confirmación, login permitido, restauración y logout. Mailbox solo del ensayo y sin trazas/capturas de credenciales.
4. Correo: rasterizar el SVG real y embeber PNG con CID, MIME alternativo texto/HTML y recurso relacionado, sin imágenes externas ni cambios de buzón/transporte/TLS.
5. Padre: contrastar entregas, construir una vez, ejecutar gates afectados/globales, revisión independiente y release pertinente con esquema real vigente si cambia el mailer Go. No afirmar pruebas o publicación antes de su ejecución.

## Contraste integrado del padre

El gateway fue inspeccionado y sus 14 regresiones nuevas pasaron en el padre. Se construyó el frontend integrado y pasaron tipos, lint y 258/258 unitarias. La primera ejecución browser detectó tres problemas de las pruebas entregadas: CTA legacy con nombre anterior, selector de contraseña ambiguo y sesión controlada no establecida mediante su login. Se corrigieron los fixtures/selectores preservando las aserciones y el comportamiento productivo; se ajustaron los estados simulados al contrato real 403 email_unverified y 422 invalid_token.

En el mismo build, la repetición global produjo **103 E2E/Axe PASS / 45 SKIP / 0 FAIL**, más **11/11 HTTPS navbar y 14/14 HTTPS BFF**. No se cambió código productivo para satisfacer esos selectores o simular una sesión inexistente.

El PNG del logo fue inspeccionado y la vista previa inerte se revisó en Chromium local a 390×844 y 1000×900: imagen decodificada, 40×40 visibles, sin overflow ni assets remotos. Se cerró el servidor de preview propio y se comprobó liberación del puerto. Se pidió corregir dos detalles de interoperabilidad de la primera entrega MIME (tipo related/root y sintaxis/unicidad CID) antes de publicación; la prueba de SMTP TLS/race inicial pasó, pero no sustituye esa corrección ni implica prueba de bandeja real.

El ensayo nativo entregado inicialmente se rechazó: la ejecución real del padre falló por imports inexistentes; la inspección detectó además enlace con credenciales en query, nombres de controles/cookies erróneos y alternativas de logout que no probaban el flujo solicitado. No se aceptó su afirmación de GREEN. Se acotó la reparación al runner y se reforzó el driver Go para fallar si falta el build solicitado y exigir estados por endpoint (registro, login pendiente, confirmación, replay, login válido, sesión y logout), no solo tres contadores globales.

El padre repitió las seis regresiones Go/PostgreSQL en el contenedor desechable después de corregir exclusivamente el fixture explícito de rol: **6/6 pruebas principales PASS**, incluidos todos los subcasos de sesión/roles/expiración/logout. Esta prueba de dominio no sustituye el recorrido Chromium aún pendiente.

La revisión independiente de frontend/gateway resultó aprobada, sin hallazgos de seguridad o lógica. El padre acotó además el consejo «si ya confirmaste tu correo» al contexto de verificación, preservando el error genérico para recuperación de contraseña; la regresión nueva se observó RED y luego GREEN (9 pruebas de dominio) sin alterar el flujo de recuperación.

## Cierre local verificado y publicación

El padre reparó los dos defectos restantes del runner entregado (espera de endpoint inventado y `process.exit` que omitía el finally), reprodujo sus fallos y ejecutó el flujo correcto. Sobre el **último build** repitió con éxito la prueba real Chromium → Next BFF → handler/servicio Go → PostgreSQL desechable: registro 201; login pendiente 403 con copy visible y sin cookies; confirmación 204 pese a metadata text/plain; replay 422 con CTA de acceso; login 200; sesión tras reload; logout mediante el botón visible y ambas cookies eliminadas. El driver Go exige estados por endpoint y un único éxito de verificación; no hay interceptación de API, caminos alternativos de logout ni cuentas reales.

Gates finales del padre: tipos/lint/formato/build PASS; **258 unitarias, 103 E2E/Axe PASS / 45 SKIP, 11 HTTPS navbar, 14 HTTPS BFF**, seis regresiones Go/PostgreSQL principales y el nuevo recorrido nativo PASS; mailer **10 pruebas/race PASS** y `go test ./...` PASS. Frontend/gateway y mailer cuentan con revisiones independientes aprobadas sin hallazgos. MIME final usa PNG real embebido, root related/alternative coherente y CID addr-spec único, sin cambios de transporte. Se verificó que los puertos propios del ensayo nativo/preview quedaron libres y que solo permanecen los dos contenedores locales retenidos, que no se tocaron.

La corrección está validada y se publica por push directo autorizado. Como incluye el mailer Go, requiere modo full y evidencia real de esquema vigente fijada por URI/generación/hash; no un web que eluda la paridad. No se anticipa éxito remoto: el reporte por SHA bajo `.local/deployments/` consignará CI, build/child, proveniencia, digest, revisión y tráfico leídos de vuelta. El usuario conserva la prueba funcional online con su propia cuenta; no se envían confirmaciones reales para ensayar ni se marca una cuenta como verificada por intervención administrativa.
