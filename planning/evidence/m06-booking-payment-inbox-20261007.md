# Evidencia #74 — pagos fake durables

Implementación en el PR #172, separada de la aceptación de PR #170 y habilitada después del merge #171. Dependencias declaradas: #71, #72 y #22, satisfechas. #74 permanece abierta hasta aceptación; esto no habilita ni completa la integración real/sandbox ni #76/#78/#79.

## Recorrido y garantías probadas

- La intención de pago se inserta antes de invocar el fake y queda vinculada a una clave idempotente estable y su huella. Dos solicitudes concurrentes con la misma clave/cuerpo conservaron una sola operación/registro fake de timeout; ambas observaron el estado pendiente.
- La intención local y el resultado del fake son filas separadas. Una consulta para operación desconocida devolvió ausencia de resultado. El fake tiene como máximo un resultado por ID de operación; los reintentos usan esa identidad sin duplicar el cobro. Una clave distinta mientras el resultado era ambiguo recibió conflicto.
- El fake emitió un evento firmado HMAC. Un evento con firma incorrecta no se persistió; el original quedó guardado antes de aplicar dominio. Repetirlo devolvió el resultado idempotente sin duplicar el inbox.
- Se reconstruyeron el servicio y el adaptador con la misma base y clave local, simulando reinicio del Backend. El reconciliador aplicó el evento del inbox sin volver a invocar el adaptador. La reserva quedó `pagada`, con una sola fila de pago, una transición, ocupación activa y operación/evento marcados como aplicados. Reproducir el callback después del reinicio no duplicó efectos.
- Se simuló caída después de persistir la intención y antes de iniciar el fake: `LookupPayment` no fabricó éxito; al reiniciar, la conciliación inició el fake con el ID persistido y completó una sola operación. También se simuló caída después de que el fake persistiera su resultado y antes de insertar en el inbox: al reiniciar, el resultado se recuperó sin otro inicio y se aplicó una sola vez.
- Éxitos autenticados posteriores a la cancelación y al vencimiento quedaron en el inbox con estado `pendiente_conciliacion`. Se confirmó que no reactivaron la reserva ni la ocupación, y que el replay fue deduplicado.
- Un evento autenticado antes del deadline, pero procesado después, sobrevivió incluso a un barrido de expiración intermedio y se aplicó sin perder su secuencia de historial.
- `BeginPayment` devolvió `ErrNotFound` para una reserva inexistente y para una reserva de otro titular, manteniendo el 404 de API.
- La integración existente además verificó que el vencimiento marque como `vencida` una operación de pago ambigua y que la retención se libere.
- `espacigo_runtime` pudo operar estado, resultado fake e inbox; el inbox autenticado permite SELECT/INSERT, sin UPDATE/DELETE.

## Comprobaciones ejecutadas

- En esta corrección de seguimiento: `bash scripts/test-m06-payment-inbox-postgres.sh` — pasó en PostgreSQL desechable con `espacigo_runtime`, incluidos los casos nuevos.
- `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` — pasó la integración de inbox y el ciclo de reservas existente.
- `go test ./internal/adapters/fakebooking ./internal/booking/... ./cmd/api -count=1` — pasó. `go vet ./...` — pasó. PyYAML, `bash -n` y `git diff --check` — pasaron.
- `go test ./...` había pasado en la implementación inicial del PR; no se repitió para esta corrección enfocada.

## Entorno y límites

Las pruebas PostgreSQL usaron una instancia temporal aislada. No se reinició el entorno local, no se aplicó V21 a `espacigo_pgdata` y no se alteraron los secretos o datos existentes. Se agregó a Compose la referencia a un secreto HMAC fake independiente; `scripts/dev-env.sh` lo genera solo si no existe, sin reemplazar credenciales locales anteriores.

No se ejecutó una pasarela real ni sandbox. Proveedor, contrato, credenciales y firma real siguen pendientes. Tampoco se alteró el estado de #76/#78/#79 ni se cierra #74 con la publicación del PR.
