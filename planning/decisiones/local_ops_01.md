# LOCAL-OPS-01 — reglas locales ratificadas

Esta decisión solo define el recorrido sintético local de LOCAL-OPS-01 (#218). No modifica ES1/ES2, contratos comerciales generales ni estados históricos.

## Entrega, uso y recepción

- Check-in: lo registra el arrendatario de la reserva con contrato cuyos dos firmantes están en estado `firmada`; solo se admite durante la fecha calendario de `inicio` interpretada en la zona IANA snapshot de la reserva. Se escribe `en_curso` y un hecho histórico en una sola transacción.
- Check-out: lo registra ese arrendatario sobre `en_curso`. Guarda `finalizada`, evidencia e historial en una transacción. El término del intervalo no completa automáticamente el arriendo.
- La ventana sintética de reclamos es de 24 horas desde `operacion_arriendo_ensayo_local.ocurrio_en` del check-out, no desde el término planificado del intervalo. El check-out fija ese instante según el reloj Backend bajo los bloqueos compartidos.
- Recepción: solo anfitrión, después del check-out. Se guarda fecha y ubicación sintéticas. Puede incluir una observación; su registro no equivale a un reclamo formal, no modifica el plazo ni liquida fondos.
- Para los actos de M08 el Backend crea un PNG fijo sintético; el cliente no carga archivos arbitrarios. La ubicación es `santiago-demo-center-v1`, con coordenadas sintéticas explícitas y aviso de ensayo; no se consulta GPS ni se implica presencia real.

## Reclamo formal y descargo — decisión conjunta 2026-10-09

- Existe discrepancia documental: ES1 CU-39/HU35 asigna el reclamo por daños al anfitrión; la definición ES2 de participantes permite abrir disputas a cualquier parte. Para este prototipo, ratificación del usuario: **solo el anfitrión** abre el reclamo formal por daños antes de cumplirse 24 horas desde el check-out persistido. El arrendatario participante puede consultar el reclamo y dejar un descargo.
- Este límite se aplica solo al reclamo formal por daños. No cambia la incidencia local de privacidad M02, que conserva su propósito y autorización propios, ni establece una prohibición general sobre otros reclamos.
- El reclamo se persiste en M10, referencia check-out/evidencia, cambia la reserva `finalizada → en_disputa` y conserva historial; no decide responsabilidad ni mueve fondos. El descargo de este slice es texto sintético idempotente. La evidencia propia del descargo y notificación durable quedan pendientes de M09/M10.
- Integración con supresión: las reservas activas y un reclamo de daños todavía abierto son bloqueadores distintos; el estado histórico `finalizada` por sí solo no bloquea. La integración consulta tanto la incidencia de privacidad M02 como los reclamos abiertos M10 y conserva la reevaluación dentro de la ejecución protegida. El reclamo abierto continúa bloqueando a ambos participantes hasta la futura resolución M10.
- La observación de recepción queda en M08 y no abre el reclamo formal. El cierre administrativo de la incidencia de privacidad tampoco cierra un reclamo por daños.

## Trazabilidad y límites

- ES1 CU-33/34/48 y HU35 permanecen inalterados. La base local aplica expresamente el reloj/zona y el origen de plazo aquí ratificados.
- La inmutabilidad de operaciones y su secuencia se conserva en PostgreSQL. Se reutilizan la reserva/ocupación únicas, contrato M07, evidencia privada y mecanismos de cleanup.
- Ningún worker altera la reserva únicamente por hora. No hay garantía/ledger, adjudicación, liquidación, cobro o devolución en M08/M10 dentro de este corte.
- Las notificaciones de CU-33/M10 necesitan outbox durable general; no se atribuye a Mailpit ni a este registro una notificación enviada.
- CU-40 queda parcial porque su requisito de fotos del descargo no se implementa aquí. M10 conserva resolución, garantías y efectos financieros pendientes.
