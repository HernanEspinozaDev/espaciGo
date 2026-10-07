# #74 — pagos fake con inbox persistente y conciliación recuperable

Esta subentrega de BOOK-BE-03 implementa el tramo local fake/durable autorizado. No conecta pasarelas ni define su contrato. Las dependencias declaradas #71, #72 y #22 están satisfechas. #74 sigue abierta hasta aceptar su entrega; #76/#78/#79 mantienen sus dependencias.

## Operación e idempotencia

`V000021__m06_local_payment_inbox.sql` añade una intención durable por reserva/`Idempotency-Key`, con huella del resultado solicitado y estado, separada del resultado durable del fake. La API inserta la intención antes de invocar `StartPayment`. Una repetición con igual clave/cuerpo conserva el ID; una huella diferente o una nueva clave mientras hay una operación ambigua devuelve conflicto. El fake registra como máximo un resultado por ID de operación; los inicios repetidos son idempotentes y no crean un segundo cobro.

Los callback fake se firman con HMAC-SHA256 sobre `event_id`, `operation_id` y `outcome`. El secreto local estable se monta desde `.local/secrets/local_payment_webhook_secret`; `scripts/dev-env.sh` solo lo genera si falta y no reemplaza los otros secretos. La ruta `POST /api/v1/local/booking-trial/payment-events` valida `X-Local-Payment-Signature` antes de guardar. La firma no se persiste. El inbox guarda únicamente eventos autenticados e inmutables, con huella de payload; un registro de aplicación separado lleva el estado del proceso. Repetir el mismo evento es idempotente; reutilizar el ID con contenido diferente entra en conflicto.

La consulta de una operación fake desconocida devuelve “sin resultado”; nunca infiere éxito desde la intención local. La conciliación aplica primero los eventos autenticados persistidos, consulta el resultado fake y, si el fake aún no registra inicio/resultado, lo inicia con el ID idempotente. Esto recupera tanto una caída antes de iniciar como una caída después de registrar el resultado y antes de guardar el evento en el inbox. El servicio reconcilia al arrancar y cada cinco segundos. Aplicar un resultado mantiene en una transacción la reserva, ocupación, fila de pago, historial, operación y marca de procesamiento.

Un fake `sin_respuesta` conserva una marca de timeout separada y responde con timeout. Repetir la misma clave consulta el resultado; otra clave se rechaza mientras la operación siga ambigua. Los eventos HMAC válidos se deduplican y se conservan aunque lleguen después de vencer o cancelar la reserva: quedan `pendiente_conciliacion`, sin reactivar reserva ni ocupación. Un evento recibido antes del vencimiento sigue siendo elegible si su procesamiento se retrasa; el barrido de expiración respeta eventos autenticados pendientes recibidos a tiempo. La creación/pago en el fake no acredita fondos reales.

## Migración, entorno y pruebas

La migración V21 es incremental y no modifica migraciones ya aplicadas. `espacigo_runtime` puede actualizar la marca/resultado del fake, cambiar estado operacional y de aplicación, y leer/insertar el inbox inmutable; el inbox no admite UPDATE/DELETE. Una reserva inexistente o ajena conserva la respuesta 404 al iniciar pago.

Pruebas enfocadas:

- `go test ./internal/booking/... ./internal/adapters/fakebooking ./cmd/api -count=1` cubre la firma fake y el endpoint HMAC/replay sin sesión de usuario.
- `bash scripts/test-m06-payment-inbox-postgres.sh` usa PostgreSQL desechable y el rol runtime. Verifica lookup desconocido sin resultado inventado, recuperación tras caída antes de iniciar y después del resultado fake pero antes del inbox, timeout idempotente, callback/replay, rechazo de firma inválida, 404 para reserva inexistente/ajena, evento tardío tras cancelación y vencimiento sin reactivar reserva/ocupación, y evento recibido antes del límite procesado después de un barrido de expiración.
- `bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` ejecuta la integración existente de reservas junto con la nueva integración para comprobar compatibilidad del flujo.

Estas pruebas no usan `espacigo_pgdata`. El cambio no reinició el entorno persistente ni aplicó V21 a la base local durante su desarrollo. Al integrar la entrega, `scripts/dev-env.sh up` aplicará la migración de manera incremental.

## Pendientes excluidos

Integración real/sandbox, credenciales y firma contractual del proveedor continúan pendientes. También permanecen pendientes los alcances de API/proveedor de #76, sandbox/webhooks de #78 y el mock amplio de #79. No se declara #74 Hecho ni se completa M06.
