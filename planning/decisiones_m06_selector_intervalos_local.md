# M06-LOCAL: selector de intervalos disponibles

Issue #166, subentrega vertical sobre #70–#72. Implementa una consulta por fecha de candidatos libres para fixtures autorizados y conecta la selección con el endpoint de cotización existente. No concluye M06 ni cambia la reserva/pago simulado.

## Contrato

- `GET /api/v1/local/booking-trial/catalog/{spaceId}/availability-options?date=YYYY-MM-DD&duration=N` exige autenticación y autoriza únicamente las dos cuentas de su fixture activo. El Backend toma tarifa, modalidad y zona del espacio; no acepta modalidad declarada por cliente. Espacio no autorizado/no habilitado produce 404 y una entrada inválida produce 422.
- La fecha se interpreta en la zona IANA del fixture, de hoy a hoy + 90 días calendario, ambos inclusive. El endpoint evalúa una fecha por solicitud. El mock ofrece atajos para los próximos siete días y un selector de fecha acotado al horizonte.
- Hora: `N ∈ {1,2,4}`, tiempo transcurrido; candidatos cada 30 minutos locales. Día: `N ∈ {1,2,3}`, desde medianoche local hasta el inicio de la fecha siguiente, no múltiplos de 24 horas. Mes: solo `N=1`, inicio de la fecha seleccionada hasta su aniversario mensual, ajustado al último día disponible.
- Se excluyen inicios pasados o iguales al reloj Backend. El horizonte solo limita el inicio; el término puede quedar más allá.
- Candidatos DST inexistentes se omiten. Una hora repetida genera dos instantes UTC distintos; el mock muestra el desplazamiento para distinguirlos.
- La fecha solicitada es una etiqueta calendario, no un instante local a medianoche. Si esta no existe por un cambio de zona, día/mes comienzan en el primer instante válido que pertenece a la fecha. El horizonte compara fechas calendario locales.
- `ocupacion` sigue siendo la única fuente de bloqueos y ocupaciones. El servicio aplica el vencimiento existente, luego el repositorio autorizado evalúa solapamientos `[inicio, término)` por lote. La respuesta no muestra otros IDs, titulares ni motivos.
- Consultar no crea una cotización ni ocupación. Seleccionar completa el intervalo del formulario actual. Cotizar/reservar revalidan con las reglas existentes y un conflicto permite consultar de nuevo. Cambiar espacio, fecha, duración o sesión invalida selección y respuestas pendientes.

## Decisión local por ausencia de horario de apertura

El esquema existente no define un calendario de horas comerciales/recurrentes por fixture; el único dato vigente son intervalos ocupados en `ocupacion`. Por ello este selector considera candidatos todas las horas locales que no solapen una ocupación. No afirma que un anfitrión opere 24/7. Esta simplificación se limita al prototipo sintético y queda pendiente de un contrato de disponibilidad comercial antes de aplicarla a publicaciones reales.

## Persistencia y fixtures

No se necesita migración ni tabla de calendario alternativa. La consulta usa `reserva_ensayo_local_fixture`, `espacio`, `tarifa_espacio` y `ocupacion`, con las concesiones runtime existentes. El helper `scripts/enable-local-interval-selector-fixtures.sh --host-email ... --renter-email ...` agrega idempotentemente fixtures sintéticos por hora/día/mes y un bloqueo manual identificado por su espacio (sin UUID global compartido); no cambia ni elimina fixtures preexistentes. Solo habilita datos para las dos cuentas indicadas. Se comprobó con dos parejas y al repetir el comando.

## Pruebas

- Candidatos y restricciones por duración/modo; inicio futuro, horizonte y fecha inválida, incluida la fecha calendario `2026-09-06` en `America/Santiago` aunque su medianoche no exista.
- Zona horaria, DST de primavera/otoño, duración real de fechas y aniversario mensual.
- PostgreSQL desechable como `espacigo_runtime`: solapes parciales y adyacencia, bloques, tarifas por unidad, privacidad por participante/tercero, retención vencida, consulta sin escrituras y conflicto al ocupar después de consultar.
- Mock: proteger respuestas de selección anterior y rellenar cotización con el UTC seleccionado, incluso durante hora repetida.

La guía para probar el corte y la evidencia aceptada se registran en [evidencia M06 selector](evidencia_m06_selector_intervalos_local_20261006.md).
