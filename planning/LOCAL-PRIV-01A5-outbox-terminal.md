# LOCAL-PRIV-01A5 — ciclo terminal y recuperación del outbox de credenciales

Fecha: 2026-10-08. Política ratificada para el prototipo local sintético; no define SLA de producción.

## Decisión

Cada ciclo tiene como máximo ocho intentos automáticos, primero incluido, y conserva el backoff exponencial existente. El octavo fallo persiste `fallo_terminal`, `fallo_terminal_en`, código estructurado y vencimiento de retención a 30 días. No se registra contenido SMTP, credenciales, tokens ni respuesta SMTP completa. Estado, contador acumulado y ciclos quedan en PostgreSQL, por lo que sobreviven reinicios.

La reapertura administrativa autenticada reutiliza el evento, requiere `smtp_restaurado` o `reintento_operativo`, una clave idempotente y auditoría con actor, recurso, resultado, motivo, correlación y fecha. Crea un ciclo nuevo con contador propio en cero y conserva el total acumulado y los ciclos anteriores. Una operación concurrente con igual clave reutiliza un solo ciclo. Eventos entregados/cancelados y destinatarios inactivos no se reabren. El despacho vuelve a comprobar que el destinatario tiene cuenta activa. La retención suspende el purgado mientras se procesa un ciclo abierto; al terminar, el evento recibe un vencimiento nuevo de 30 días. El rol runtime ejecuta la purga acotada a través de una función `SECURITY DEFINER`; no tiene permiso de borrado directo sobre toda la tabla.

El transporte SMTP no ofrece garantía de exactamente una vez: una caída después de que Mailpit/SMTP acepte el mensaje y antes de confirmar la entrega en PostgreSQL puede producir un duplicado al reintentar. La interfaz y la API comunican este límite.

## Alcance

Migración incremental V27, repositorio/servicio, endpoints administrativos locales, contrato OpenAPI, purgador existente y controles mínimos del mock. El corte trata solo `identidad.credencial_cambiada` y Mailpit. Los padres #185/#40 quedan abiertos, así como proveedor/canales productivos y el resto del cierre de privacidad. No incluye GCP.

## Verificación publicada

Ver [evidencia LOCAL-PRIV-01A5](evidence/local-priv-01a5-outbox-terminal-20261008.md).
