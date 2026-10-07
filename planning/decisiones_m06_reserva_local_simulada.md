# M06-LOCAL-01 — Reserva sintética local con pago simulado

**Issue operativa:** [#148](https://github.com/HernanEspinozaDev/espaciGo/issues/148), subentrega trazable de [#73](https://github.com/HernanEspinozaDev/espaciGo/issues/73). Estas decisiones rigen solo el prototipo local; no reemplazan el diseño general M06 ni cierran las Issues de reserva/pago.

## Alcance habilitado

Un comando administrativo local habilita exactamente un borrador sintético privado para dos cuentas activas con correo verificado: anfitrión propietario y arrendatario. La tabla `reserva_ensayo_local_fixture` es una allowlist de singleton. Solo el par autorizado puede leer su fixture mediante API; no hay endpoint de búsqueda ni se enumeran borradores de otros titulares. Esta selección no publica el espacio ni deriva permisos de KYC.

Se reutilizan tarifas versionadas, snapshot de cotización propio del arrendatario y la tabla única M06 `ocupacion`. La solicitud inserta reserva, transición inicial y retención `[inicio, fin)` en una transacción; el índice de exclusión existente arbitra carreras. La tarifa, zona y reglas de uso del espacio quedan capturadas en cotización y reserva. CLP usa enteros; la cotización no ocupa.

## Reglas ratificadas

- Estados: `pendiente_de_pago → pagada → aprobada_host`; finales alternativos `cancelada_por_pago`, `rechazada_arrendador`, `vencida_pago`, `vencida_host` y `cancelada_arrendatario`.
- Cotización: 15 minutos, parámetro local ajustable; no implica retención.
- Pago: 15 minutos desde la solicitud. Adaptador fake disponible solo con `LOCAL_BOOKING_TRIAL=1`. `exito` avanza a `pagada`, `rechazo` cancela y libera la retención, `sin_respuesta` registra timeout simulado y conserva el estado hasta vencimiento. Todo resultado muestra **ENSAYO LOCAL — SIN COBRO REAL**.
- Anfitrión: 24 horas desde pago para aprobar o rechazar. Aprobación conserva la ocupación como `reserva`; rechazo/vencimiento la libera. Reembolso del fake, si aplica, queda registrado como `devolucion_simulada`.
- El arrendatario puede cancelar solo `pendiente_de_pago`; no se inventa política posterior al pago.
- Idempotencia: misma clave/cuerpo entrega la reserva existente; misma clave/cuerpo distinto es 409. La restricción de exclusión prohíbe solapes activos concurrentes.

## Reloj y vencimientos

Los casos de uso reciben un `func() time.Time` sustituible. El servicio procesa vencimientos persistidos de forma transaccional al consultar/listar o al ejecutar una transición; no depende de esperar ni de una goroutine. Los deadline se guardan en PostgreSQL y sobreviven reinicios. La entrega inicial #148 no incluía una conciliación durable de pagos. La subentrega autorizada de #74 añade para el fake local operación persistida, inbox HMAC deduplicado y reconciliación al arrancar y periódica; no acredita garantías ni contrato de proveedor real.

## Dependencias y separación

Reutiliza trabajo fusionado: identidad/verificación/sesión #124; borradores #134; perfiles versionados #141; disponibilidad y `ocupacion` #145; tarifa versionada y snapshots #147. Los criterios amplios de #69–#79 siguen abiertos. Este flujo no requiere el catálogo/búsqueda pública (#52–#68): lo sustituye un fixture privado único. No requiere permiso comercial ni KYC productivo porque no publica el borrador, no cobra ni confirma fondos reales.

La Issue #148 concentra DDL incremental, casos de uso, API, test PostgreSQL y mock de dos participantes. El mock local de este recorrido no es la prueba de pasarela. La subentrega #74 aborda persistencia de eventos autenticados, deduplicación/idempotencia y recuperación con el fake; integración real/sandbox, credenciales y contrato del proveedor siguen pendientes, al igual que #76/#78 por sus criterios de API y pruebas de proveedor y #79 por el mock amplio. La #73 general queda abierta aunque cortes locales se acepten.

## Fuera de alcance

Pagos/proveedor real, movimientos de fondos, devolución real, webhook, conciliación, reintentos ante pasarela, workers durables, contratos, mensajes, reservas comerciales y publicación. No se resuelven M05/M06 global, DB02-09, #123, ni los pendientes M02/M03.
