# Decisión técnica — catálogo local de ensayo M05

Fecha: 2026-10-06. Issue #151. PR local vertical vinculado a #69; la búsqueda comercial y el resto de M05 siguen pendientes.

## Límites de producto

- Una fila de `reserva_ensayo_local_fixture` significa habilitación explícita de un espacio sintético. El espacio permanece `borrador`; el endpoint de catálogo nunca consulta el conjunto general de borradores.
- La consulta y el detalle requieren que la cuenta autenticada sea anfitrión o arrendatario de la fila allowlisted. La cotización exige el arrendatario autorizado para el `space_id` elegido.
- El catálogo local no deriva permisos de KYC, no publica, no permite ubicación/ranking ni sustituye criterios generales de M05.
- El detalle expone datos sintéticos de uso seguro, perfil/atributos guardados y tarifa/zona del espacio; omite dirección privada e identidad de los participantes.
- Se conservan las ocho categorías y los perfiles guardados en su versión original. Las consultas no convierten borradores a versiones nuevas.

## Contratos y disponibilidad

- `GET /api/v1/local/booking-trial/catalog` permite categoría y rango ISO-8601 opcionales; la disponibilidad se computa por ausencia de ocupaciones activas solapadas en la tabla única `ocupacion`. El mock interpreta fechas locales con una zona explícita de búsqueda y manda instantes RFC 3339.
- El detalle se consulta mediante `/catalog/{space_id}`. Cotizar requiere `space_id`; Backend selecciona allowlist, tarifa, categoría, versión/valores del perfil, condiciones y zona del mismo registro y guarda un snapshot en la cotización.
- La solicitud continúa usando la cotización concreta y revalida estado, allowlist, intervalo futuro y ocupación dentro de la transacción; una búsqueda o cotización nunca retiene disponibilidad.
- El mantenimiento de muestras es una operación de desarrollo local. El comando recibe anfitrión, arrendatario y código de categoría; el perfil vigente de esa categoría se copia sin convertir valores ni habilitar datos de usuario reales.

## Persistencia

V15 transforma el singleton de M06 a allowlist por espacio, preserva la fila existente y añade a la cotización el código de categoría, versión de perfil y valores snapshot con FK al perfil inmutable. V1–V14 y sus datos permanecen intactos. La prueba PostgreSQL usa base desechable con `espacigo_runtime`; el entorno de desarrollo aplica V15 incrementalmente y conserva volumen/secrets.

Los pagos siguen siendo fake local; ninguna cotización/reserva del catálogo constituye publicación comercial, disponibilidad garantizada futura ni confirmación de cobro real.
