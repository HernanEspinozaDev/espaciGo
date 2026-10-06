# Evidencia M05-LOCAL-01 — catálogo sintético local

Issue #151, PR de implementación pendiente de revisión. El recorrido se ejecutó contra `main` más los cambios del checkout de trabajo en el entorno local; no equivale a aceptación ni a completar M05/M06.

## Persistencia y privacidad

- V15 se aplicó incrementalmente desde V14. `espacigo_pgdata` conservó el mismo volumen; credenciales locales sin cambios.
- El catálogo respondió únicamente fixtures habilitados explícitamente para la pareja sintética autorizada. Se habilitó un segundo espacio sintético de categoría bodega para probar selección múltiple; no se publicó ni modificó KYC.
- Un borrador privado no habilitado no aparece en `GET /catalog`, no se puede consultar por ID y no se puede cotizar. El detalle no incluye dirección privada ni IDs de participantes.
- Se mantuvieron disponibles los ocho códigos/etiquetas de categoría definidos en el catálogo vigente.

## Recorrido desde el mock

- Sesión de arrendataria sintética; búsqueda por categoría `bodega` para `2030-04-01 12:00–13:00` interpretado en `America/Santiago`: el mock mostró un espacio y estado disponible.
- El detalle mostró `space_id=87e2d9a8-42a0-47d4-af7c-8e84edbb5913`, categoría `bodega` / `Bodega`, perfil v1, tarifa CLP 8.000/h y zona `America/Santiago`.
- Cotización `55ae3ac7-6e2a-499c-bbd4-5cad27d241a5` quedó asociada a ese espacio; capturó categoría `bodega`, perfil v1/valores, tarifa CLP 8.000/h y zona `America/Santiago`.
- Reserva `b999ce48-ead6-4734-b14f-9723180a513f` quedó `pendiente_de_pago` para el mismo `space_id` y el intervalo de la cotización.
- Búsqueda posterior de ese intervalo ya no mostró el espacio disponible. La solicitud revalidó la ocupación dentro de la transacción.
- Para no dejar una retención temporal de prueba en la base de desarrollo, cancelé esa solicitud desde el mock antes del pago (`cancelada_arrendatario`); una nueva búsqueda del mismo intervalo volvió a mostrar el espacio disponible.
- Todas las vistas mantienen el aviso `ENSAYO LOCAL — SIN COBRO REAL`.

## Pruebas y límites

La integración PostgreSQL aislada ejecuta con `espacigo_runtime` y cubre múltiples fixtures, aislamiento de borradores, filtros de categoría/intervalo, detalle autorizado, snapshots de tarifa/perfil/zona y revalidación de disponibilidad al reservar. El recorrido del mock se verificó manualmente en el entorno local persistente.

Este corte no implementa catálogo público, geografía/ranking, publicación comercial, KYC productivo, cobro real ni búsqueda general M05. Issues #69–#79, DB02-09 y #123 mantienen sus criterios pendientes.

## Correcciones del PR #152

- Con reloj sustituible, la integración crea una retención, avanza hasta el vencimiento y consulta directamente el catálogo para el intervalo retenido. La búsqueda ejecuta el mecanismo de vencimientos antes de calcular disponibilidad: el espacio reaparece, la ocupación queda inactiva y la reserva/historial registran `vencida_pago` sin consultar ni cotizar previamente.
- Las pruebas del estado del mock cubren cotizar A → seleccionar B → impedir la solicitud con la cotización de A, descartar la respuesta de cotización de A si llega después del cambio a B, e invalidar cotizaciones al comenzar una búsqueda.
- Comprobaciones: `bash scripts/test-m06-local-booking-postgres.sh` pasó con PostgreSQL desechable y rol runtime; `npm --prefix mock run test:profile-races` pasó (5 pruebas); `go test ./...`, `go vet ./...`, parseo de OpenAPI y `git diff --check` pasaron.
