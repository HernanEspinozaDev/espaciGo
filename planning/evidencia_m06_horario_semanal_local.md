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

### Seguimiento manual de navegador (2026-10-06)

Se intentó iniciar la comprobación desde la pestaña local que conservaba una vista previa con resultados/cotización. Al consultar explícitamente la sesión, la API respondió `401 unauthenticated` y el mock invalidó esos datos transitorios. No se guardó ni modificó el horario; tampoco se alteraron reservas, historiales u otros datos persistentes. La captura mostraba el formulario de sesión sin sesión consultada y el aviso 401. No se recuperó ni restableció la contraseña.

El recorrido manual solicitado queda **pendiente** de reautenticar la cuenta anfitriona sintética en el navegador y después la arrendataria: como anfitrión, activar lunes con `09:00–12:00` y `13:00–17:00`, guardar, recargar el mock y comprobar persistencia; en el selector, comprobar que no ofrece candidatos durante la pausa ni en días cerrados; desactivar el horario y confirmar que vuelve la disponibilidad anterior. Como arrendataria, comprobar que el editor no aparece. Para no cambiar credenciales ni añadir datos, esta sesión se detuvo ante la falta de una sesión anfitriona válida. La integración PostgreSQL automatizada cubre selector, catálogo, cotización, solicitud y permisos con el rol runtime.

La Issue #168 queda abierta y en revisión hasta la aceptación de HernanEspinozaDev. El PR no se ha fusionado; Issues generales/M06 no se marcan como completadas.
