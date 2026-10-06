# Evidencia — M09-LOCAL-01 conversación por reserva

Issue #156, trazable a #95–#98 y CU-37/CU-38/HU34. Corte local sintético; las Issues generales y sus criterios productivos continúan abiertos.

## Validación ejecutada

- `bash scripts/dev-env.sh up`: entorno local saludable y V16 aplicada de forma incremental sobre `espacigo_pgdata`.
- `bash scripts/test-m06-local-booking-postgres.sh`: pasó en PostgreSQL desechable, con el rol runtime usado por el Backend. La prueba recorre envío idempotente, autorización de anfitrión/arrendatario, rechazo de terceros, límites, paginación por cursor, envío en los tres estados activos y lectura sin envío tras cancelación, rechazo y vencimiento. También comprueba que `aprobada_host` permite conversar aunque el intervalo ya haya terminado.
- `go test ./...`: pasó.
- `go vet ./...`: pasó.
- `npm --prefix mock run test:profile-races`: pasó, 8/8.
- `npm --prefix mock run build`: pasó.
- `bash -n scripts/clean-local-booking-thread-messages.sh scripts/test-m04-attributes-postgres.sh` y `git diff --check`: pasaron.

## Comprobación con ambas cuentas

En `http://127.0.0.1:8081/`, se abrió una reserva aprobada desde la bandeja de arrendatario, se envió un mensaje sintético y se inició sesión como su anfitrión. El anfitrión vio el mismo mensaje en el hilo y pudo escribir una respuesta. El texto de ensayo incluyó una etiqueta HTML literal: el DOM mostró ese texto y no creó un elemento `<b>`. La bandeja del anfitrión no mostró reservas del arrendatario; la prueba PostgreSQL valida además lectura y escritura denegadas para una tercera cuenta.

La comprobación detectó que el wrapper de formularios dejaba deshabilitado el botón de envío después de seleccionar una reserva. Se corrigió el refresco de controles y se repitió el recorrido exitosamente.

## Datos y pendientes

La prueba interactiva dejó mensajes sintéticos en el hilo local a propósito para conservar la evidencia de persistencia. No se ejecutó el script de limpieza. `espacigo_pgdata` conserva su volumen preexistente (creado el `2026-10-05T01:49:09-03:00`); no se reinició ni se borró. La limpieza de un hilo requiere ejecutar explícitamente el comando documentado en `planning/decisiones_m09_conversacion_local.md`.

La retención productiva, moderación, acceso administrativo y solicitudes de borrado siguen pendientes en #95–#98. No hay expiración automática ni se atribuye plazo legal a esos mensajes.
