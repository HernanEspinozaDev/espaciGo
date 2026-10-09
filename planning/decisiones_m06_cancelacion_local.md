# Política local de cancelación M06

Fecha: 2026-10-06. Versión: `local_flexible_v1`.

Esta política se aplica exclusivamente a fixtures sintéticos habilitados y al adaptador fake del entorno local. No representa una política comercial ni autoriza cobros o devoluciones reales.

## Regla

- `pendiente_de_pago`: se conserva la cancelación existente antes de que venza el plazo de pago; no genera devolución.
- `pagada`, `aprobada_host`, `firma_parcial` y `lista_para_checkin`: el arrendatario titular puede cancelar solo mientras el reloj del Backend sea estrictamente anterior al inicio. La devolución fake corresponde al 100 % del importe CLP confirmado por el pago fake, limitado al subtotal de la reserva. No se agregan comisiones, descuentos ni garantías.
- Al alcanzar o superar el inicio, la cancelación se rechaza. Check-in, reservas en curso, disputas y políticas comerciales siguen pendientes.
- La versión se toma de la cotización y se copia a la reserva. Una modificación futura del valor por defecto del fixture no cambia cotizaciones ni reservas existentes.
- Antes de confirmar, el mock presenta política, plazo, importe y el texto **“Devolución simulada — sin movimiento de dinero”**. El motivo es opcional.
- El Backend vuelve a validar titular, estado e inicio dentro de la transacción. Firma, rechazo y cancelación adquieren las mismas filas de participantes y reserva en orden común, y consultan el reloj después de bloquear. Cancelar anula una versión todavía incompleta antes de que una firma pueda continuar; conserva un contrato firmado como hecho histórico. La transición, la liberación de `ocupacion`, el historial y la obligación única de devolución se confirman atómicamente.
- El estado de devolución es independiente del estado terminal de la reserva. El fake admite éxito, fallo y timeout. Fallo/timeout conservan la reserva cancelada y la devolución pendiente. Los reintentos explícitos reutilizan el mismo `operation_id`; una obligación por reserva y la identidad estable impiden una segunda devolución.
- Mailpit recibe avisos de ensayo para ambos participantes. El resultado identifica el envío local no durable; no se afirma entrega productiva ni notificación durable.

## Persistencia y límites

V20 es aditiva. Guarda la versión de política en fixture, cotización y reserva; registra importe confirmado en cada pago fake; agrega una fila de cancelación idempotente, una obligación de devolución por reserva y los resultados secuenciales de cada intento. No altera filas de reserva ni historiales previos salvo el valor por defecto de la nueva columna snapshot. Las migraciones anteriores no se editan.

Las nuevas reglas no modifican ni cancelan reservas existentes. La tabla única `ocupacion` sigue siendo el calendario de ocupación y se desactiva en la misma transacción que cancela.

Faltan proveedor real, autorización de devoluciones externas, firma/replay de webhooks, conciliación, fallos ambiguos del proveedor, contratos, check-in, ocupación en curso, disputa y política comercial aprobada. Las pruebas fake validan el contrato local, no esos criterios productivos de CU-51/HU19 ni cierran #74/#76/#78.
