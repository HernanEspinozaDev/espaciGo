# Recuperación y reconocimiento de enlaces usados

## Encargo y base

El usuario confirmó que el flujo anterior funciona y pidió simplificar el mensaje de solicitud de recuperación y reconocer enlaces ya usados de recuperación/verificación antes de volver a mostrar los formularios. Base local/remota verificada: `226a9102c608d6140f35bfd8d6b0ad5fe8abafef`, sin cambios pendientes al inicio. Preflight local 2026-10-27 01:28 -03. Se conserva el diseño aprobado.

## Diagnóstico previo

`ResetPassword` ya rechaza enlaces con `ConsumedAt`, invalidación, caducidad o intentos agotados dentro de la transacción del usuario. `VerifyEmail` también rechaza replay. La UI actual muestra el formulario simplemente por haber capturado un fragmento bien formado, sin consultar si Go ya consumió el enlace. Eso permite volver a intentar desde la pantalla, pero por sí solo no demuestra un segundo cambio efectivo de contraseña.

El padre ejecutó tres regresiones principales Go/PostgreSQL con sus subcasos en el contenedor desechable del proyecto: enlaces de verificación/replay, recuperación/cambio de contraseña/revocación y agotamiento de intentos. PASS. La nueva prueba exigirá específicamente un segundo reset con el mismo enlace y otra contraseña, su rechazo y la conservación de la contraseña válida.

## Decisiones de seguridad y contrato

- El servidor ya determina si una cuenta es elegible. Se conserva la respuesta pública uniforme de solicitud para no crear un buscador de correos registrados ni afirmar entrega SMTP que el 202 no confirma. Se retira del texto visible «Si existe una cuenta elegible» y la explicación técnica. Nuevo copy: «Solicitud recibida. Revisa tu correo y la carpeta de spam para continuar. Si no recibes el mensaje, comprueba la dirección e inténtalo de nuevo.»
- Reconocer el enlace requiere prueba de posesión completa, no email ni token ID solos. Dos operaciones read-only semánticas, transportadas por POST para mantener credenciales fuera de URLs: `/auth/verification/status` y `/auth/password/recovery/status` bajo `/api/v1` en Go y `/api/backend` en el BFF.
- Request exacto: `{token_id, token}`. Response 200: `{status}` con `ready | used | expired | invalid`, sin cuenta, correo, hash u otros datos. Validar hash en tiempo constante, usuario vinculado y propósito antes de revelar un estado histórico. Identificador desconocido, secreto incorrecto o propósito distinto producen estado genérico `invalid`.
- La consulta no consume el token ni cambia intentos del token, contraseña, sesión o estado de la cuenta; conserva el control de tasa por IP. No se introducen migraciones ni persistencia de estado paralela en Next.
- Solo `ready` permite mostrar el formulario/confirmación. Mientras se comprueba o si falla la consulta, no se habilita la operación. `used` muestra «Enlace de recuperación ya utilizado» o «Verificación ya realizada», sin botón que vuelva a ejecutar el consumo. Caducidad/invalidez tienen estado honesto y nueva solicitud.
- Una consulta inicial no sustituye la comprobación atómica del consumo: otra pestaña puede usar el enlace entre consulta y envío. Se conserva el rechazo de replay en Go y se reconcilia ese error mediante consulta read-only, sin reintentar automáticamente la mutación.
- Fragmento capturado y purgado como antes. No tokens en query, logs, SSR, Web Storage, capturas/trazas o mensajes de error. Estados ligados a la generación del enlace, con cancelación y rechazo de respuestas tardías.

## Cortes y prueba de aceptación

1. Go/contrato: consultas de estado por posesión, OpenAPI y generación reproducible; casos PG válidos/usados/caducados/incorrectos y ausencia de mutación.
2. UI/adaptador: mensaje simple, comprobación previa y estados usados sin formularios; errores fail-closed y reemplazo A→B seguro.
3. Ensayo real local Chromium → Next BFF → Go → PostgreSQL desechable: reabrir verificación usada sin botón; recuperar una vez, reabrir sin campos; replay directo 422; iniciar sesión con la contraseña correcta y verificar sesión/logout.
4. Padre: contrastar código/ejecución, build único integrado, globales/Axe/HTTPS, revisión independiente y publicación full autorizada con evidencia real vigente. No probar mediante cuenta o correo real del usuario ni tocar bases/volúmenes retenidos.

## Estado de contraste local

- Go/contrato: revisión independiente aprobada, sin hallazgos de seguridad o lógica. El padre repitió los tres nuevos tests PostgreSQL y `go test ./...`: PASS. Se reforzó la prueba con secreto incorrecto contra token ya usado/expirado y límite de 2048 bytes; el segundo reset con distinta contraseña se rechaza y solo la primera contraseña válida permite login.
- UI: revisión independiente aprobada después de corregir respuestas tardías de la reconsulta tras 422 y consultas duplicadas en Retry. El padre ejecutó tipos, lint, formato, `api:check` (181 operaciones), 259 unitarias y primer build: PASS.
- RED visible acreditado sobre el artefacto anterior: dos tests de enlace usado fallan porque no aparece el resultado esperado.
- Artefacto final de UI: tipos/lint/formato y 259 unitarias PASS; build final2 PASS. Tras quitar descripciones y CTA repetidos sin debilitar los tests, el padre obtuvo **34/34 focalizadas**, **108 E2E/Axe PASS / 45 SKIP / 0 FAIL**, **11/11 HTTPS navbar** y **14/14 HTTPS BFF**. Ambas revisiones independientes aprobadas.
- Prueba nativa completa cerrada por el padre: **PASS 9.06 s**, Chromium → Next HTTPS/BFF → Go → PostgreSQL desechable, con solicitud de recuperación 202, primer cambio 204, reapertura de ambos enlaces mostrando estado usado sin formulario, segundo consumo 422 y acceso únicamente con la nueva contraseña válida; login/sesión/logout reales y ambas cookies Secure/HttpOnly.
- Diagnóstico contrastado del runner: la hipótesis delegada de un bloqueo de `APIRequestContext`/TLS no se aceptó sin evidencia. Las fases añadidas por el padre mostraron que el timeout ocurría en `page.goto(..., waitUntil: networkidle)` **antes** del POST de replay, pese a dos consultas Go 200 y un consumo 204. Se sustituyó esa espera, solo en las dos reaperturas, por carga DOM más el encabezado terminal confirmado; el mismo cliente directo obtuvo el 422 esperado. También se corrigió el assert delegado que esperaba erróneamente `expiryCookie.httpOnly === false`: el contrato exige true para ambas cookies. Se mantuvieron las comprobaciones y se restauraron logs sin secretos; no se cambió el runtime para fabricar el PASS.
- Revisión visual controlada: cuatro estados usados (móvil/escritorio, verificación/recuperación), cuatro métricas agregadas por el padre, cero inputs/botones de consumo, CTA acceso único de 44 px, sin overflow y cero violaciones Axe. Capturas sanitizadas inspeccionadas; es UI controlada, no sustituye el ensayo Go anterior.
- Evidencia de esquema renovada mediante captura real read-only/TLS del operador existente, 75 migraciones. El padre leyó el objeto GCS de generación exacta y verificó hash `49931127a2d5ab6a8cb08876244457778fafd048b34028cb3ddec30ccde3a77c`, captura `2026-10-27T05:04:57.687455+00:00`; puerto del forward liberado. Referencia privada `.local/deployments/recuperacion-schema-readonly.json`.

Las tres revisiones independientes (Go/contrato, UI/carreras y ensayo nativo final) quedaron aprobadas sin hallazgos bloqueantes. La validación local está cerrada. Publicación `main` + full con evidencia real vigente pendiente de verificación remota; este documento no la da por concluida. No se ha alterado la verificación o contraseña de ninguna cuenta real.
