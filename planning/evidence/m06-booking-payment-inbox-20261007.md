# Evidencia #74 — pagos fake durables

Implementación en el PR #172, separada de la aceptación de PR #170 y habilitada después del merge #171. Dependencias declaradas: #71, #72 y #22, satisfechas. #74 permanece abierta hasta aceptación; esto no habilita ni completa la integración real/sandbox ni #76/#78/#79.

## Recorrido y garantías probadas

- La intención de pago se inserta antes de invocar el fake y queda vinculada a una clave idempotente estable y su huella. Dos solicitudes concurrentes con la misma clave/cuerpo produjeron un solo inicio del fake; ambas observaron el timeout pendiente.
- Reintentar con la misma clave consultó el estado sin iniciar otro pago. Una clave distinta mientras el resultado era ambiguo recibió conflicto.
- El fake emitió un evento firmado HMAC. Un evento con firma incorrecta no se persistió; el original quedó guardado antes de aplicar dominio. Repetirlo devolvió el resultado idempotente sin duplicar el inbox.
- Se reconstruyeron el servicio y el adaptador con la misma base y clave local, simulando reinicio del Backend. El reconciliador aplicó el evento del inbox sin volver a invocar el adaptador. La reserva quedó `pagada`, con una sola fila de pago, una transición, ocupación activa y operación/evento marcados como aplicados. Reproducir el callback después del reinicio no duplicó efectos.
- La integración existente además verificó que el vencimiento marque como `vencida` una operación de pago ambigua y que la retención se libere.
- `espacigo_runtime` pudo operar la tabla de estado y la tabla de aplicación; el inbox autenticado permite SELECT/INSERT, sin UPDATE/DELETE.

## Comprobaciones ejecutadas

- `bash scripts/test-m06-payment-inbox-postgres.sh` — pasó en PostgreSQL desechable, con `espacigo_runtime`.
- `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` — pasó: integración nueva y ciclo existente de reservas.
- `go test ./...` — pasó. Las integraciones que requieren `TEST_DATABASE_URL` no se consideran cubiertas por esa invocación; se ejecutaron con los scripts PostgreSQL desechables anteriores.
- `go vet ./...` — pasó.
- PyYAML cargó `planning/openapi.yaml` y `compose.yaml`; `bash -n` verificó los scripts modificados; `git diff --check` pasó.

## Entorno y límites

Las pruebas PostgreSQL usaron una instancia temporal aislada. No se reinició el entorno local, no se aplicó V21 a `espacigo_pgdata` y no se alteraron los secretos o datos existentes. Se agregó a Compose la referencia a un secreto HMAC fake independiente; `scripts/dev-env.sh` lo genera solo si no existe, sin reemplazar credenciales locales anteriores.

No se ejecutó una pasarela real ni sandbox. Proveedor, contrato, credenciales y firma real siguen pendientes. Tampoco se alteró el estado de #76/#78/#79 ni se cierra #74 con la publicación del PR.
