# M06-LOCAL-01 — tiempo de inicio e historial secuencial

Fecha: 2026-10-05. Rama: `feat/m06-local-booking-trial`, PR #149.

## Cambio

- El Backend rechaza cotizaciones cuyo inicio sea pasado o igual a su reloj. El adaptador repite la validación al comenzar la transacción de cotización.
- En una solicitud nueva, después del bloqueo de idempotencia y de comprobar que no existe una reserva previa, el Backend vuelve a leer su reloj dentro de la transacción. La cotización debe seguir vigente y su inicio debe ser estrictamente posterior a ese instante. Un reintento con la misma clave y contenido devuelve la reserva previa antes de esta validación.
- V13 añade a cada transición una `secuencia` positiva y única por reserva; V14 reconstruye el orden de filas históricas empatadas desde la cadena de estados, sin usar UUID como criterio. Las escrituras nuevas calculan la secuencia mientras mantienen el bloqueo de la reserva; la consulta ordena por ella. La API/OpenAPI expone `sequence`.

## Comprobaciones ejecutadas

- `go test ./...` — pasó. La integración PostgreSQL no recibe `TEST_DATABASE_URL` en esta ejecución general; se ejecutó aparte con el script desechable indicado abajo.
- `bash scripts/test-m06-local-booking-postgres.sh` — pasó `TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry` en PostgreSQL desechable. El harness aplicó migraciones hasta V14 y comprobó conexión/flujo con `espacigo_runtime`.
- En esa integración: rechazos de cotización con inicio pasado e igual a ahora; una cotización aún vigente cuyo intervalo ya había comenzado fue rechazada dentro de la solicitud; la petición rechazada dejó cero reservas para esa cotización y cero ocupaciones para el intervalo; creación, pago y aprobación con el mismo instante devolvieron secuencia/estados `1 pendiente_de_pago`, `2 pagada`, `3 aprobada_host`; el reintento idempotente posterior al inicio del intervalo devolvió la reserva original.
- `go vet ./...` y `git diff --check` — pasaron.
- `npm --prefix mock run build` y `npm --prefix mock run test:profile-races` — pasaron (2 pruebas del mock; no se modificó su alcance).
- `bash scripts/dev-env.sh up -d` aplicó V13 y V14 incrementalmente en el entorno persistente. La versión registrada quedó en 14; el volumen `espacigo_pgdata` conservó su fecha de creación `2026-10-05T01:49:09-03:00`; los secretos locales conservaron modo `600`, propietario `1000:1000`, tamaño y fecha de modificación. No se borró el volumen ni se regeneraron secretos.
- Conexión PostgreSQL usando `espacigo_runtime`: permisos del historial `SELECT/INSERT/UPDATE/DELETE = true/true/false/false`.
- `bash scripts/dev-env.sh verify-http` — backend, mock, readiness, CORS y recursos estáticos pasaron.

## Alcance de la integración

Se ejecutó el único test PostgreSQL del adaptador de reservas en base y contenedor desechables; su `trap` elimina la base, el rol de prueba, el contenedor y el socket temporal. No se ejecutó una suite PostgreSQL global de todos los módulos. No se cambió el estado operativo de #148: continúa En revisión hasta aceptación humana.
