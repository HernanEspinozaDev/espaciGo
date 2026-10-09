# LOCAL-CONT-01 — reglas de ensayo ratificadas

Fecha: 2026-10-09. Alcance: reservas, cuentas y documentos sintéticos del prototipo local. Esta política no declara validez jurídica ni cambia los requisitos históricos.

## Conflicto conservado entre fuentes

ES1 CU-30 y HU33 indican registrar el rechazo de una parte, conservar el contrato parcial hasta el plazo y no guardar todavía un contrato final. El Anexo B de ES2 agrega cancelación al rechazo. La regla de esta entrega sigue ES1: el rechazo es terminal para esa firma y versión, queda en el historial y **no cancela** la reserva. M06 cancela únicamente cuando Backend llega a `start_at` y falta al menos una firma. Las referencias históricas ES1/ES2 permanecen sin editar.

## Regla local ratificada

- Los únicos firmantes son el anfitrión y el arrendatario registrados en la reserva aprobada. La firma fake no atribuye facultades ni identidad legal.
- Las acciones de firma, rechazo y vencimiento serializan con el bloqueo de la fila de reserva y leen el reloj inyectable después de obtenerlo.
- Cada acción de firma/rechazo requiere `now < start_at`. Al llegar `now >= start_at`, si falta una firma, M06 cambia la reserva a `cancelada_por_firma`, libera la única ocupación y crea en la misma transacción una obligación de devolución fake única por la suma efectivamente confirmada en el fake. No se descuentan comisiones.
- Si ambas firmas están persistidas antes del inicio, el paso del tiempo no cancela la reserva. Los reintentos devuelven el resultado persistido o conflicto sin repetir transiciones ni obligaciones.
- Un rechazo no libera la ocupación de inmediato. El contrato queda parcial y el vencimiento aplica en `start_at`.
- Toda pantalla, respuesta de ensayo y PDF identifica `ENSAYO SINTÉTICO LOCAL — SIN VALIDEZ JURÍDICA`.

## Integración con cancelación `local_flexible_v1`

El arrendatario conserva el derecho local ratificado de cancelar antes de `start_at` cuando la reserva está `pagada`, `aprobada_host`, `firma_parcial` o `lista_para_checkin`. El importe simulado sigue limitado al 100 % efectivamente confirmado, sin comisión. Firma y cancelación bloquean primero la misma fila de reserva y consultan el reloj tras adquirir el bloqueo.

Si se cancela después de una firma parcial, la versión incompleta pasa a `anulado`, registra la cancelación en su historial y rechaza firmas posteriores. Si ya estaba completamente firmada, conserva firmas y documento como hechos históricos; la reserva igualmente se cancela y libera su ocupación. En ambos casos solo la transacción ganadora puede crear la devolución. En `start_at` o después, el arrendatario no puede cancelar; si faltan firmas, el vencimiento cancela y crea una obligación única. Si ambas firmas se completaron, ese vencimiento no afecta la reserva.

La evaluación de supresión trata `firma_parcial` y `lista_para_checkin` como `reserva_activa`, además de los estados activos previos. Una devolución pendiente conserva su bloqueador independiente.

## Ownership y protección local

M07 guarda snapshot, firmas e historial en `contrato_ensayo_*`. M09 posee `documento_privado_sintetico_local` para metadata y bytes; su fila se inserta en la transacción que crea el contrato. El PDF se cifra con AES-256-GCM usando el secreto local persistente `local_contract_encryption_key`, creado por `scripts/dev-env.sh config`; conservar ese archivo para poder abrir documentos ya emitidos. El API solo entrega texto/metadata a participantes y descifra el PDF al descargarlo después de ambas firmas. M06 conserva reservas, ocupaciones, transición e importe de devolución.

El snapshot copia los valores de reserva, condiciones y política ya persistidos; nunca vuelve a consultar perfil o tarifa para reconstruir un contrato anterior. El acceso de terceros se responde como no encontrado. Los cuerpos, documentos y secretos no se registran en logs.

## Evolución pendiente

La integración de firma externa/sandbox, consultas legales reales, notificaciones durables de contrato, conciliación de callbacks externos, firma electrónica con validez jurídica y despliegue cloud siguen pendientes. El almacenamiento privado común para otros módulos y la inclusión del nuevo contrato en la exportación completa de titular deben integrarse según sus owners antes de declarar completos M07/M02. La decisión de retención productiva de contratos y mensajes no se fija aquí; aplica únicamente el tratamiento sintético local vigente.
