# #74 — pagos fake con inbox persistente y conciliación recuperable

Esta subentrega de BOOK-BE-03 implementa el tramo local fake/durable autorizado. No conecta pasarelas ni define su contrato. Las dependencias declaradas #71, #72 y #22 están satisfechas. #74 sigue abierta hasta aceptar su entrega; #76/#78/#79 mantienen sus dependencias.

## Operación e idempotencia

`V000021__m06_local_payment_inbox.sql` añade una operación durable por reserva/`Idempotency-Key`, con huella del resultado solicitado y estado. La API inserta la intención antes de invocar `StartPayment`. Una repetición con igual clave/cuerpo consulta el mismo ID; una huella diferente o una nueva clave mientras hay una operación ambigua devuelve conflicto. Ningún reintento invoca un segundo inicio de pago.

Los callback fake se firman con HMAC-SHA256 sobre `event_id`, `operation_id` y `outcome`. El secreto local estable se monta desde `.local/secrets/local_payment_webhook_secret`; `scripts/dev-env.sh` solo lo genera si falta y no reemplaza los otros secretos. La ruta `POST /api/v1/local/booking-trial/payment-events` valida `X-Local-Payment-Signature` antes de guardar. La firma no se persiste. El inbox guarda únicamente eventos autenticados e inmutables, con huella de payload; un registro de aplicación separado lleva el estado del proceso. Repetir el mismo evento es idempotente; reutilizar el ID con contenido diferente entra en conflicto.

La conciliación procesa primero vencimientos e inbox pendiente y luego hace consultas de estado (`LookupPayment`) por ID persistido. Nunca llama a `StartPayment` desde una repetición o reconciliación. El mismo servicio reconcilia al iniciar el Backend y vuelve a intentarlo cada cinco segundos. Aplicar un resultado mantiene en una transacción la reserva, ocupación, fila de pago, historial, operación y marca de procesamiento. Los vencimientos se comprueban bajo el bloqueo de la reserva; vencer el pago también cierra la operación ambigua como `vencida`.

Un fake `sin_respuesta` conserva la operación `pendiente` y responde con timeout. Repetir la misma clave vuelve a consultar el estado; otra clave se rechaza mientras el resultado anterior siga ambiguo. Un evento tardío autenticado puede resolverlo. El fake no acredita fondos ni produce movimientos reales.

## Migración, entorno y pruebas

La migración V21 es incremental y no modifica las migraciones aplicadas. El rol `espacigo_runtime` obtiene lectura/escritura del estado operacional, lectura/inserción del inbox inmutable y lectura/escritura del estado de aplicación; no puede modificar ni borrar el evento autenticado.

Pruebas enfocadas:

- `go test ./internal/booking/... ./internal/adapters/fakebooking ./cmd/api -count=1` cubre la firma fake y el endpoint HMAC/replay sin sesión de usuario.
- `bash scripts/test-m06-payment-inbox-postgres.sh` usa una base PostgreSQL desechable y el rol runtime. Verifica dos solicitudes concurrentes con la misma clave, ausencia de segundo inicio tras timeout, conflicto para nueva clave, callback autenticado, replay, rechazo de firma inválida y reconstrucción del servicio/adaptador que aplica el inbox persistido sin duplicar pago, historial ni ocupación.
- `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` ejecuta la integración existente de reservas junto con la nueva integración para comprobar compatibilidad del flujo.

Estas pruebas no usan `espacigo_pgdata`. El cambio no reinició el entorno persistente ni aplicó V21 a la base local durante su desarrollo. Al integrar la entrega, `scripts/dev-env.sh up` aplicará la migración de manera incremental.

## Pendientes excluidos

Integración real/sandbox, credenciales y firma contractual del proveedor continúan pendientes. También permanecen pendientes los alcances de API/proveedor de #76, sandbox/webhooks de #78 y el mock amplio de #79. No se declara #74 Hecho ni se completa M06.
