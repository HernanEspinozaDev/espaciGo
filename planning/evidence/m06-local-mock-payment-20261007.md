# Evidencia — M06-LOCAL pago fake desde la bandeja

Issue de subentrega: #175, vinculada como hija de #79. Dependencias satisfechas: #24, #74 (alcance fake/durable) y #173. La entrega cubre la interfaz local de pago; no resuelve proveedor real/sandbox ni los criterios generales aún abiertos de #77, #78 y #79.

## Alcance verificado

- Desde la bandeja, el arrendatario selecciona una reserva propia en `pendiente_de_pago`; anfitrión, terceros y estados incompatibles no pueden ejecutar el pago.
- La interfaz muestra siempre `ENSAYO LOCAL — SIN COBRO REAL` y mensajes distintos para pago confirmado, rechazo del fake y resultado incierto/timeout.
- Un timeout conserva en memoria de la pestaña la clave idempotente y el resultado original. El botón pasa a «Consultar / reintentar pago (misma clave)» y bloquea cambios del resultado. Un envío concurrente para la misma reserva se ignora.
- Al reintentar, se reutilizan clave y contenido; el Backend consulta la misma operación durable. Si la conciliación ya actualizó el pago, la bandeja recarga detalle e historial desde la API. La bandeja también permite consultar de nuevo el estado explícitamente.
- Las respuestas se aplican solo si siguen vigentes la reserva seleccionada, la cuenta y la sesión capturadas al iniciar la solicitud.
- Logout y respuesta 401 invalidan solicitudes pendientes y limpian bandeja, selección, detalle/historial, estado de pago, conversación y selección/cotización del catálogo. La clave incierta permanece solo en memoria para evitar fabricar otro intento mientras siga abierta la pestaña; no se muestra tras logout.
- El secreto HMAC no está incluido en el mock ni se transmite desde el navegador.

## Validación

Pruebas enfocadas de estado y concurrencia del mock:

```sh
npm --prefix mock run test:payment-flow
```

Compilación más 5 pruebas: clave/resultado estables ante timeout y concurrencia, separación por reserva, descarte de respuestas obsoletas por reserva/cuenta/sesión, recuperación tras conciliación y deduplicación de clicks.

Comprobación de regresión de estados de bandeja y carreras existentes:

```sh
npm --prefix mock run test:profile-races
```

Resultado: compilación y 22 pruebas existentes aprobadas. El entorno se actualizó sin reiniciar ni eliminar el volumen:

```sh
scripts/dev-env.sh up -d
scripts/dev-env.sh verify-http
```

Resultado: API, mock y assets locales disponibles; base persistente V21 conservada. No se ejecutó una suite Backend adicional porque el comportamiento del Backend no cambió y se reutilizan las verificaciones de pago durable de #173.

## Recorrido de navegador

Se usó una pareja y un fixture sintéticos autorizados exclusivamente para este ensayo local; los registros anteriores se conservaron. Desde la cuenta arrendataria se crearon reservas sintéticas distintas para comprobar:

1. **Éxito:** el pago fake quedó `pagada`; detalle, listado e historial mostraron la transición `pendiente_de_pago → pagada`.
2. **Rechazo:** la reserva quedó `cancelada_por_pago`; el detalle y el historial indicaron el rechazo y la liberación correspondiente.
3. **Timeout:** el Backend devolvió 504; se mantuvo `pendiente_de_pago`, se mostró la explicación de conciliación y el control conservó el mismo resultado y clave para reintentar. El reintento volvió a consultar esa operación y la bandeja/detalle/historial se recargaron desde la API. La recuperación del resultado fake ya persistido está cubierta por las pruebas Backend de #173 y por la prueba enfocada de estado del mock.
4. **Logout:** la sesión terminó y la interfaz vació las reservas, el detalle/historial, el estado de pago, la conversación y el catálogo/cotización visibles.

La pantalla no expone tokens HMAC. Los correos y credenciales sintéticos, tokens de verificación, claves de idempotencia y secretos locales se omiten de esta evidencia.
