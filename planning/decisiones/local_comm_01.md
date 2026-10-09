# D-COMM para LOCAL-COMM-01

Estado: ratificada por la titular del producto el 2026-10-09, exclusivamente para el prototipo local con reservas sintéticas. No establece reglas legales ni productivas. Issue hija #220; trazabilidad a #95–#102 y CU-35–38/49.

## Reseñas

- Una reseña por participante y reserva, con unicidad `(reserva, autor)`: el arrendatario califica el espacio y el anfitrión al arrendatario.
- Nota entera obligatoria entre 1 y 5; comentario opcional. No se agrega ventana de 14 días. Se crea solo en `finalizada`; en `en_disputa` no se crean nuevas y las ya existentes se conservan.
- No hay edición. El reintento idéntico devuelve el registro persistido; contenido diferente entra en conflicto.
- Publicaciones activas muestran solo la calificación y comentario sintéticos del espacio, sin correo, UUID de reserva, autor ni datos de contacto. La vista pública minimiza correos/UUID que aparezcan en texto. Ocultas no se listan ni promedian. La reputación del arrendatario solo está disponible para la propia cuenta autenticada.

## Reporte y moderación

- El anfitrión solo reporta reseñas del espacio de su reserva/espacio, con `insultos_acoso`, `datos_personales`, `spam` o `ajeno_experiencia`.
- Reportar marca `reportada`; no oculta ni cambia el promedio. Un reporte queda pendiente hasta decisión administrativa.
- Administrador activo puede desestimar (`sin_infraccion`, `duplicado`, `error_registro`) u ocultar (`contenido_inadecuado` u otro código admitido por el contrato) con motivo estructurado, actor y auditoría append-only. El corte no incluye apelación.

## Avisos durables

| Evento | Destinatario |
| --- | --- |
| Check-in | Anfitrión |
| Reseña reportada | Administradores activos autorizados |
| Reclamo formal abierto | Arrendatario |
| Reserva cancelada | Ambas partes |

La intención por evento/destinatario se confirma con la transacción de origen mediante trigger de base de datos; no incluye texto de conversación ni secretos. Mailpit es el transporte local. El worker conserva ciclo, intentos y resultado estructurado tras reinicio; máximo ocho intentos por ciclo, fallo terminal, reapertura solo por administrador con motivo/auditoría. Una caída después de que SMTP acepte el mensaje pero antes de confirmar la entrega puede producir duplicado: no se promete exactamente una vez. No se emite aviso por mensaje.

## Privacidad y retención local

- Reseñas: 24 meses calendario desde la creación.
- Reportes: mientras estén pendientes y 24 meses calendario desde resolución.
- Avisos pendientes se mantienen; entregados o fallidos terminalmente se conservan 30 días desde el cierre del ciclo; una reapertura suspende purga mientras siga pendiente. Auditoría de recuperación permanece cinco años conforme al tratamiento de auditoría ya ratificado.
- El ZIP propio incluye reseñas propias y avisos del titular con minimización de identificadores y contenido de terceros. En baja, se retiran vínculos directos y texto libre de reseñas asociadas; se conserva puntuación/hecho sintético hasta su vencimiento. Los reportes pendientes conservan solo códigos e historial sin vínculo directo al titular retirado.
- La baja de una cuenta reseñada convierte la reseña recibida en `arrendatario_retirado` con destinatario nulo; mantiene puntuación y hecho, y elimina comentario/texto libre. Para impedir una segunda reseña luego de purgar el contenido, se guarda solo SHA-256 de reserva+autor sin identificadores ni contenido, hasta el vencimiento de los vínculos de esa reserva; no extiende por sí sola la retención de la reseña.
- Altas de reseña y reporte bloquean las cuentas participantes/actoras y revalidan estado activo antes de insertar. El despacho toma el mismo bloqueo por cuenta que la baja y lo conserva hasta confirmar éxito/fallo SMTP y finalizar el ciclo. Si el despacho gana la carrera, el aviso se vuelve terminal antes de la baja; si la baja gana, cancela la intención y el worker no envía.
- El tratamiento de mensajes no cambia: política local existente de limpieza explícita por hilo. Estos plazos son decisiones del prototipo, no plazos legales.

## Integración y límites

Se conserva el outbox V22/V27 de cambios de credencial, sin reutilizarlo para cambiar su semántica. El nuevo outbox M09 modela solo los cuatro eventos anteriores. Los padres generales M08/M09/M10 permanecen abiertos; no cubre avisos de contratos, descargos, resolución, canales externos, perfiles públicos de arrendatarios o requisitos productivos.
