# Lectura local de conversaciones — M09-LOCAL-02

Issue de subentrega: [#158](https://github.com/HernanEspinozaDev/espaciGo/issues/158). Depende funcionalmente de la conversación local aceptada en #156/PR #157. Mantiene trazabilidad con #95–#98 y CU-37/CU-38/HU34; las Issues generales siguen abiertas.

## Decisiones aprobadas para el prototipo

- `reserva_mensaje_lectura` guarda un cursor independiente por `(reserva_id, participante_id)`. No guarda confirmaciones visibles para la contraparte.
- Al abrir o refrescar un hilo, el mock primero carga y muestra correctamente la página reciente; después solicita marcar como leído hasta la mayor `secuencia` de esa página. La marca atiende también las secuencias anteriores. Una carga fallida o una respuesta descartada por selección obsoleta no llama al endpoint de marca.
- El cursor es un high-water mark persistente y monotónico. El upsert del Backend conserva `GREATEST(actual, solicitado)`, de modo que reintentos concurrentes/fuera de orden nunca retroceden la lectura.
- El contador de cada bandeja cuenta únicamente mensajes de otros participantes cuya secuencia supera el cursor propio. Los mensajes enviados por la cuenta actual no cuentan. Una inserción posterior a la página mostrada queda por encima del cursor y continúa pendiente.
- Los estados terminales siguen permitiendo consulta y conservan su contador hasta que la persona abre el hilo.
- No hay tiempo real, polling automático, notificaciones externas ni proveedor de avisos.

## Implementación

V17 añade el cursor con referencias restringidas a reserva y usuario. El rol runtime obtiene `SELECT`, `INSERT` y `UPDATE`, sin `DELETE`. `POST /reservations/{id}/messages/read` valida que la persona participa y que la secuencia pertenece al hilo. Las listas de reservas devuelven `unread_count` calculado desde mensajes/cursor. El mock marca la página ya renderizada y vuelve a consultar las dos listas para refrescar sus contadores.

No cambia el contrato de persistencia de mensajes V16. La limpieza, retención y tratamiento productivo continúan conforme a [decisiones de conversación](decisiones_m09_conversacion_local.md); no se añade borrado automático ni plazo legal.
