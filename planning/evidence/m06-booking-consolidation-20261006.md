# Evidencia de consolidación local M06 — 2026-10-06

Esta evidencia respalda la entrega local del PR #170 y su aceptación posterior. Solo se aceptan los criterios específicos completados de #70/#71/#72; no completa M06 ni los criterios generales de #69/#73/#75/#77. La política `local_flexible_v1` aplica solamente a fixtures sintéticos y pagos fake; no hubo movimiento de dinero.

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

Cancelación desde el inicio/en curso, check-in, disputas, política comercial y proveedor real continúan pendientes. #69/#73/#75/#77 permanecen abiertas por criterios generales. Para #74 ya están satisfechas las dependencias de su tramo fake/durable; persisten como trabajo futuro la persistencia de eventos autenticados, deduplicación, idempotencia y conciliación recuperable al reiniciar Backend. El adaptador real/sandbox, credenciales y contrato de proveedor quedan separados. #76/#78 siguen bloqueadas por esas dependencias de proveedor/API y #79 sigue bloqueada por sus dependencias generales; este corte no las habilita.

## Ajuste de reintentos concurrentes de devolución

En la rama del PR #170 se añadió `reused` a la respuesta para distinguir una operación resuelta por otra solicitud concurrente. Si la transacción encuentra la obligación ya completada después de tomar el bloqueo, el repositorio devuelve `operation_id`, monto, moneda, `last_result` y `updated_at` desde PostgreSQL. El servicio no vuelve a notificar y no convierte un resultado fake timeout/fallo descartado en error si prevaleció la finalización guardada.

Pruebas deterministas en la integración PostgreSQL desechable:

- Éxito concurrente con timeout: se mantienen ambas solicitudes en el adaptador fake, se deja confirmar y persistir el éxito, y recién entonces se libera el timeout. Las dos respuestas son completadas con los campos idénticos a PostgreSQL; la del timeout descartado lleva `reused=true`, responde sin 504 y no añade avisos.
- Dos éxitos concurrentes: ambas alcanzan el adaptador, se libera una y se espera su commit antes de liberar la otra. Hay una sola finalización/intento, una respuesta original y otra `reused=true`, ambas con el resultado y timestamp persistidos, y un único par de avisos.

Comando focal ejecutado nuevamente: `bash scripts/test-m06-local-booking-postgres.sh` — pasó. No se alteró V20, el volumen persistente ni los secretos.

## Aceptación posterior al merge

PR #170 se fusionó en `main` el 2026-10-06 (merge `b9d337435252129f489ef990c2086854d9bc9b6f`). El checkout quedó sincronizado por fast-forward. `scripts/dev-env.sh up` actualizó API/mock y aplicó V20 incrementalmente sobre la base existente; `schema_migrations` confirma `V000020__m06_local_flexible_cancellation.sql`. Comprobación posterior mínima: API `/health/ready` respondió `ready`, el mock respondió HTTP 200 y los servicios quedaron saludables. Se confirmó la presencia del volumen `espacigo_pgdata` y de los archivos de secretos locales sin leer ni mostrar su contenido. No se borraron datos ni se repitieron suites.

Con la aceptación del slice local se cierran #70 (BOOK-DB-01), #71 (BOOK-DB-02) y #72 (BOOK-BE-01), cuyos criterios originales específicos quedaron satisfechos. #69, #73, #75, #77 y los pendientes productivos/de mock permanecen abiertos; esta aceptación no declara completo M06.

## Preparación de #74 — alcance fake/durable

Tras cerrar #70–#72, y con #22 también cerrado, todas las dependencias declaradas por #74 para iniciar su parte Backend están satisfechas. Projects deja #74 en Listo para implementar con el adaptador fake: eventos autenticados persistidos, deduplicación/idempotencia y conciliación recuperable tras reinicio. Esa implementación todavía no forma parte de esta evidencia ni de PR #170. La pasarela real/sandbox, credenciales, contrato del proveedor y las tareas #76/#78/#79 permanecen pendientes conforme a sus propias dependencias.
