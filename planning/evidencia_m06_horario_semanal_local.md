# Evidencia M06-LOCAL: horario semanal

Issue #168 · PR de implementación en preparación para revisión.

## Recorrido

Desde el catálogo autenticado, elegir un fixture por hora muestra la administración del horario solo a su anfitrión. El anfitrión consulta/activa/desactiva siete días ISO y define cero, uno o dos tramos por día; la API valida la forma y ownership. Con horario activo, el mock consulta el selector existente y carga sus intervalos en el formulario de cotización. Las tarifas de día/mes conservan su comportamiento previo.

La selección no crea cotización ni retención. La cotización y solicitud vuelven a verificar el horario actual. Al guardar cambios, el mock limpia selección y cotización anterior para requerir una nueva consulta.

## Decisiones reproducibles

- `24:00` significa cierre exclusivo al final del día; la apertura `00:00` es válida. Los tramos no cruzan medianoche.
- Domingo 6-sep-2026 en `America/Santiago`: se omite 00:00–00:59 inexistente y comienza en el primer candidato 01:00. La condición del intervalo completo también se comprueba con pausas y cierres.
- Sábado 4-abr-2026 en `America/Santiago`: 23:00 ocurre dos veces; se conservan dos intervalos UTC diferenciables cuando ambos caben completamente en el mismo tramo local.
- Sin horario activo, el prototipo conserva la lista previa de candidatos; día activo sin períodos no entrega intervalos.
- Cambios de horario usan el bloqueo exclusivo de `espacio`; cotización y solicitud usan bloqueo compartido. Las reservas/historias existentes no se cambian.

## Resultados

Validación ejecutada en la rama de entrega:

- `bash scripts/test-m06-local-booking-postgres.sh`: pasó en PostgreSQL desechable con el rol runtime `espacigo_runtime`. Incluye ownership entre anfitrión/arrendatario/tercero; pausas y días cerrados; filtro de catálogo por intervalo; rechazo de cotización y solicitud incompatibles; no creación de reserva/ocupación tras solicitud rechazada; modificación de horario concurrente sincronizada con solicitud; zona bloqueada mientras está activo y permitida tras desactivar; candidato del salto de primavera y las dos ocurrencias de la hora repetida de otoño que satisfacen el tramo local.
- `go test ./...`: pasó para todos los paquetes.
- `go vet ./...`: pasó.
- `npm run test:profile-races` en `mock/`: pasó compilación TypeScript y 21 pruebas de estado/control de respuestas asíncronas existentes.
- `python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml"))'`: pasó.
- `git diff --check`: pasó.
- `bash scripts/dev-env.sh up`: migró V19 incrementalmente y dejó backend/mock/PostgreSQL locales arriba. La base persistente `espacigo_pgdata` y los archivos locales de secretos `db_admin_password` y `runtime_password` siguieron presentes; no se ejecutó `clean`, no se borró el volumen ni se alteraron registros de prueba.
- `bash scripts/dev-env.sh verify-http`: pasaron las 9 comprobaciones HTTP de salud, readiness, CORS/preflight y entrega del mock.

### Recorrido manual del editor en navegador (2026-10-06)

La sesión previa había expirado y la consulta explícita devolvió 401; el mock limpió los datos transitorios. No se tocó el horario de ningún fixture previo ni se restableció ninguna contraseña. Con autorización posterior se crearon dos cuentas sintéticas nuevas, verificadas por Mailpit, y se habilitó un solo fixture adicional de oficina por hora para esa pareja:

- Anfitrión: `m06-weekly-host-1791296032582@ejemplo.invalid`.
- Arrendatario: `m06-weekly-renter-1791296032582@ejemplo.invalid`.
- Fixture aislado: `Espacio sintético · Oficina`, UUID `feb18bfd-4bda-4581-83de-79053ae01a32`, autorizado solo para las dos cuentas anteriores.
- Las contraseñas temporales se entregaron al usuario por el chat; no se guardaron en el repositorio ni en esta evidencia.

Resultados visibles en el mock:

1. Como anfitrión, activé el horario del lunes con `09:00–12:00` y `13:00–17:00` y guardé correctamente.
2. Recargué la página, volví a iniciar sesión como anfitrión, reabrí el detalle y comprobé que la casilla seguía activa y los cuatro valores reaparecieron en sus campos.
3. Con el horario activo, consulté el martes `2026-10-06`, sin tramos configurados: el selector mostró **0 intervalos**. Para el lunes `2026-10-12` mostró **12 inicios**: `09:00`, `09:30`, `10:00`, `10:30`, `11:00`, `13:00`, `13:30`, `14:00`, `14:30`, `15:00`, `15:30` y `16:00`. No ofreció inicios en la pausa `12:00–13:00` ni intervalos que la atravesaran.
4. Desactivé el horario y consulté nuevamente el martes: el selector volvió a ofrecer **25 intervalos**, confirmando el comportamiento anterior sin configuración activa. La fila queda desactivada; no hay reglas activas.
5. Cerré la sesión de anfitrión, inicié como arrendatario, busqué el fixture y abrí el detalle. `#booking-weekly-hours-form` tuvo cero elementos y el contenedor del editor quedó vacío; la edición no aparece para el participante.

Mailpit no estaba escuchando inicialmente; se inició solo su servicio local con `docker compose up --no-build --wait mailpit`. No se ejecutó `clean`, no se reinició/eliminó `espacigo_pgdata`, no se tocaron secretos ni las cuentas/fixtures previos. Se añadió únicamente la pareja y fixture sintéticos descritos arriba. El horario del nuevo fixture quedó desactivado al cerrar la prueba. No se repitió la suite completa.

La Issue #168 queda abierta y en revisión hasta la aceptación de HernanEspinozaDev. El PR no se ha fusionado; Issues generales/M06 no se marcan como completadas.
