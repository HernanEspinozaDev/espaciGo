# M06-LOCAL: horario semanal para fixtures sintéticos por hora

Issue #168 · ampliación local autorizada del prototipo. No completa M06 ni sustituye los criterios generales de #70–#72.

## Alcance y ownership

- Solo fixtures sintéticos explícitamente habilitados y con tarifa vigente `hora`.
- Solo el anfitrión del fixture consulta o reemplaza la configuración. Los participantes mantienen sus consultas de disponibilidad y cotización normales; un tercero ve 404.
- El horario semanal se persiste en tablas propias de configuración, no en `ocupacion`. `ocupacion` conserva bloqueos manuales, retenciones y reservas como fuente única de intervalos ocupados.
- El horario puede estar desactivado o activo. Si está desactivado (incluida la ausencia de fila), se conserva el comportamiento anterior. Si está activo, los días sin tramos están cerrados.
- Cada día ISO admite hasta dos intervalos half-open de hora civil local, sin solapes, de apertura estrictamente anterior al cierre y sin cruce de medianoche. `00:00` se acepta al abrir. `24:00` solo se acepta al cerrar y es el límite final exclusivo de ese día; no se representa como otro día ni se edita como hora de apertura.
- Las tarifas `dia` y `mes` no consultan ni aplican esta configuración.

## Fechas, zona y DST

La zona IANA configurada en el espacio determina días ISO y horas locales. Los candidatos siguen siendo intervalos UTC y deben tener duración estrictamente positiva. El Backend aplica el mismo predicado al selector, catálogo con intervalo, cotización y solicitud de reserva.

Se omiten inicios locales inexistentes. En una hora repetida, los dos inicios UTC se consideran candidatos distintos; una duración transcurrida se ofrece solo cuando cada tramo de desplazamiento UTC queda dentro de un mismo tramo local configurado. Si la transición hace que parte del intervalo se desplace fuera del horario (por ejemplo, una reversión horaria que cruza el cierre), se descarta. La prueba de `America/Santiago` comprueba el salto de primavera del 6-sep-2026 y la hora repetida del 4-abr-2026.

Mientras la configuración esté activa, una actualización de `zona_horaria` se rechaza por una guarda PostgreSQL. El anfitrión puede desactivar el horario, revisar la nueva zona y volver a configurarlo después.

## Consistencia y datos históricos

Guardar la configuración toma un bloqueo exclusivo de la fila de `espacio`; cotizar y crear reserva toman bloqueo compartido de esa misma fila y consultan la configuración dentro de la transacción. Si una actualización precede a cotización/solicitud, se valida con la nueva versión; si la cotización previa deja de caber, la solicitud devuelve conflicto y no crea reserva ni ocupación. Consultar intervalos no persiste cotizaciones ni retenciones.

Cambiar, activar o desactivar un horario no reescribe, cancela ni altera reservas o historiales ya existentes. La revalidación protege solo nuevas cotizaciones y solicitudes.

## Decisión de alcance

HU24 describe la disponibilidad recurrente como no implementada/fuera de alcance en ES1. Esta entrega amplía únicamente el prototipo local para fixtures sintéticos por hora por decisión explícita, sin convertirla en requisito comercial/general ni habilitar publicaciones reales. Festivos, temporadas, excepciones, cierres comerciales y operación real quedan fuera.

## Verificación

La evidencia de pruebas y el recorrido del mock se mantiene en [evidencia M06 horario semanal](evidencia_m06_horario_semanal_local.md). La migración V19 es incremental; conserva las migraciones aplicadas y no modifica los datos existentes.
