# Evidencia de consolidación local M06 — 2026-10-06

Esta es una ampliación del prototipo de reservas para revisión en el PR de consolidación. No constituye aceptación de #69/#70/#71/#72/#73/#75/#77 ni completa M06. La política `local_flexible_v1` aplica solamente a fixtures sintéticos y pagos fake; no hubo movimiento de dinero.

## Pruebas de Backend y PostgreSQL

- `bash scripts/test-m06-local-booking-postgres.sh` — pasó contra PostgreSQL desechable. Migraciones con rol administrador efímero y operaciones con `espacigo_runtime`.
- La integración verifica: pago fake registrado al importe confirmado; motivo de rechazo preservado; snapshot de política en cotización/reserva; titularidad; cancelación estrictamente antes del inicio; rechazo exacto al inicio y al cruzar el límite mientras espera el bloqueo; ausencia de mutación ante conflicto; competencia entre aprobación y cancelación; liberación atómica de `ocupacion`; historial; una sola obligación de devolución.
- Se probaron resultados fake fallo → timeout/sin respuesta → éxito, con estado de reserva cancelada y devolución pendiente después de los primeros intentos, misma identidad de operación y sin duplicar la obligación. El rol runtime puede leer/insertar/actualizar únicamente lo necesario; no obtiene permisos de modificación/borrado de historial o intentos.
- El aviso local se envió a ambas partes para cancelación y para estado de devolución. El test de integración comprueba destinatarios mediante el adaptador de avisos; Mailpit se comprobó además en el recorrido de navegador descrito abajo.

## Recorrido en mock/API

Se levantó un Compose temporal con nombre de proyecto `espacigo-m06-review`, puertos aislados y volumen efímero. Se registraron/verificaron por Mailpit dos cuentas sintéticas y se habilitó un único fixture de ensayo para esa pareja. No se usaron contraseñas ni tokens de cuentas persistentes. Secuencia comprobada como arrendatario:

1. Buscar fixture sintético, consultar intervalos disponibles y crear cotización snapshot de 8.000 CLP por una hora con política `local_flexible_v1`.
2. Solicitar reserva, completar pago fake exitoso y cargar vista previa de cancelación.
3. Verificar que la vista previa muestre política, fecha límite estricta, monto confirmado y “Devolución simulada — sin movimiento de dinero”. Confirmar cancelación con motivo opcional antes del inicio.
4. Comprobar que la reserva queda cancelada, la ocupación se libera y el detalle conserva historial más obligación de devolución pendiente.
5. Ejecutar una devolución fake fallida; el detalle presenta estado pendiente y `fallo_simulado`. Reintentar con éxito: el detalle muestra devolución completada por 8.000 CLP y el mismo ID de operación.
6. Mailpit mostró avisos de cancelación pendiente y de devolución completada enviados a ambas cuentas sintéticas. El aviso es correo local de desarrollo; no implica entrega durable/productiva.

El directorio temporal, las cuentas y el volumen desechable no contienen los datos del prototipo persistente. La base persistente `espacigo_pgdata`, secretos y datos sintéticos preexistentes se conservaron; no se reiniciaron ni borraron.

## Suite final y omisiones

- `go test ./...` — pasó. En esta invocación los tests condicionados a `TEST_DATABASE_URL` se omitieron; el script PostgreSQL separado los ejecutó contra una instancia desechable.
- `go vet ./...` — pasó.
- `npm --prefix mock run build` — pasó.
- `npm --prefix mock run test:profile-races` — pasó, 22/22.
- `git diff --check` — pasó.
- OpenAPI cargó con PyYAML y referencias de esquema de respuesta verificadas.

## Límites

Cancelación desde el inicio/en curso, check-in, disputas, política comercial, devolución de proveedor real, webhooks firmados/deduplicados, conciliación y avisos durables continúan pendientes. #74/#76/#78 siguen abiertas; #79 no queda completa por este recorrido. No se afirma ninguna actualización de GitHub Projects ni se cierran Issues en esta evidencia.
