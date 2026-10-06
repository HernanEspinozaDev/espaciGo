# Conversación local por reserva — M09-LOCAL-01

Issue de subentrega: [#156](https://github.com/HernanEspinozaDev/espaciGo/issues/156). Trazabilidad conservada con #95–#98 y CU-37/CU-38/HU34. Estas tareas generales mantienen abiertos sus criterios más amplios de M09.

## Contrato de este prototipo

- La conversación se identifica por `reserva_id`; el Backend consulta el anfitrión, arrendatario y estado de esa fila antes de listar o insertar. Un tercero recibe `404` y no obtiene información sobre la existencia del hilo.
- Lectura solo para participantes, también después de cancelación, rechazo o vencimiento. Envío solamente en `pendiente_de_pago`, `pagada` y `aprobada_host`. Una transición simultánea serializa con el envío mediante el bloqueo de la fila de reserva.
- El estado `aprobada_host` no cierra la conversación al terminar el intervalo. Este corte no modela el fin del arriendo ni disputas.
- El mensaje es texto plano de 1–2.000 puntos de código Unicode, sin adjuntos ni edición. El mock usa nodos DOM y `textContent`; el texto nunca se inserta como HTML.
- `Idempotency-Key` se limita por reserva y autor. Un retry con cuerpo idéntico devuelve el mismo mensaje; reutilizar la clave con cuerpo distinto da `409`.
- `GET .../messages` devuelve las 30 entradas más recientes (máximo 100), en orden ascendente por `secuencia`. `older_cursor` contiene la menor secuencia de la página si hay mensajes previos; `before` es exclusivo. El IDENTITY global permite cursores estables aunque filas de otros hilos dejen huecos y aunque sus timestamps coincidan.
- El rol `espacigo_runtime` tiene `SELECT, INSERT`; no puede modificar ni borrar mensajes. El esquema es sintético/local. No hay TTL ni borrado automático.

## Persistencia y limpieza local

La migración incremental V16 crea `mensaje_reserva_ensayo`, huella para comparar retries y unicidad por `(reserva_id, autor_id, clave_idempotencia)`. `ON DELETE RESTRICT` evita que borrar una reserva borre el hilo accidentalmente. Los mensajes persisten en la base local hasta una acción explícita.

Para retirar únicamente el hilo local deseado, se entrega `scripts/clean-local-booking-thread-messages.sh <reservation-uuid>`. Solicita escribir `borrar-hilo` y elimina solo filas cuyo `reserva_id` coincide; opera contra el contenedor local con el administrador de desarrollo. No se ejecuta automáticamente ni se ejecuta durante pruebas.

La finalidad, retención, acceso administrativo, moderación, solicitudes de titulares y eventual borrado productivo siguen pendientes en el diseño general COMM #95–#98. Este corte no propone un plazo legal ni registra mensajes en proveedores externos, correo, analítica o logs.
