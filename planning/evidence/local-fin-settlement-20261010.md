# Evidencia — LOCAL-FIN-SETTLEMENT-01 (#236)

**Estado:** listo para revisión en [PR #237](https://github.com/HernanEspinozaDev/espaciGo/pull/237); el ajuste final de minimización del ledger y su prueba están en `54a55a3`. No fusionado ni aceptado. Baseline integrado #235 / `5c6be649967b794f862bf2457eaacd18e104448b`. Project 1 incluye #236 en `En revisión`.

## Recorrido implementado

- V44 añade snapshots de política/tasa en cotización y reserva, una liquidación por reserva, operación fake idempotente, resultado fake persistente separado, historial append-only y comprobante JSON inmutable. V45 admite `liquidacion_pendiente` en las evaluaciones auditadas de privacidad. Reservas anteriores quedan sin asignación retroactiva.
- El Backend requiere anfitrión, reserva `finalizada`, checkout persistido + 24 h, ausencia de disputa abierta y de operaciones financieras/devoluciones pendientes; lee pagos y devoluciones confirmados, excluyendo autorización/liberación/captura de garantía del neto. La operación queda transaccional y auditable.
- Baja y purga detectan también una reserva finalizada con snapshot local y saldo positivo confirmado aunque todavía no exista intención. La evaluación devuelve `liquidacion_pendiente`; la purga difiere y conserva participantes, permitiendo iniciar la operación más tarde. Reservas históricas sin snapshot no reciben política retroactiva.
- Al vencer la retención, se retiran los actores participantes de los historiales de liquidación y del ledger financiero, incluido `operacion_pendiente_conciliacion` con `detalle_codigo='liquidacion_fake_incierta'`; tipo/acción, código, importe, moneda, estado y fechas permanecen. El actor administrativo que no es participante se conserva. Runtime tiene únicamente permiso `UPDATE(actor_id)` en esas tablas, no UPDATE general ni DELETE.
- Timeout se conserva como resultado incierto; el reconciliador tras reinicio consulta la misma operación. Reconciliación administrativa está autorizada y no inicia una segunda transferencia. Solo el éxito confirmado produce comprobante.
- Endpoints y OpenAPI: consultar/iniciar liquidación, conciliar solo como administrador y obtener comprobante solo por participante. El mock muestra política, importes separados, aviso de ensayo, resultado incierto y descarga JSON. Limpia e invalida respuestas al cambiar reserva o sesión.
- Exportación propia cubre snapshots/liquidación/historial/comprobante. Supresión bloquea obligaciones pendientes. Retención usa el plazo de transacciones locales ya ratificado y conserva la referencia mínima del hecho financiero; no se declara anonimización.

## Verificación ejecutada

- `go test ./cmd/api ./internal/booking/... ./internal/adapters/postgres/booking ./internal/adapters/fakebooking ./internal/dbbootstrap/...` — PASS.
- `GO_TEST_RUN=TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` — PASS sobre PostgreSQL/PostGIS desechable; no usó `espacigo_pgdata`.
- `GO_TEST_RUN='TestSuppressionAndRetentionPreserveUnstartedSettlementObligation|TestLocalSettlementHistoryActorLinksAreUnlinkedAtRetention|TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity ./internal/adapters/postgres/booking` — PASS sobre PostgreSQL/PostGIS desechable. Comprueba bloqueo de baja/purga antes de intención, inicio posterior posible, minimización actor-only con permisos runtime y carrera controlada de consulta → registro → confirmación/reuso; otro resultado conserva conflicto.
- La integración crea una reserva de prueba, comprueba rechazo un segundo antes de checkout+24 h sin intención parcial, mantiene el bloqueo de reserva mientras comienza una solicitud y avanza el reloj hasta el límite, y confirma que la toma de hora ocurre tras el lock.
- También prueba dos solicitudes concurrentes con la misma clave (una liquidación/resultado/historial/comprobante), pago fake timeout → Backend/adapter nuevos → estado incierto persistido → conciliación administrativa → una sola fila de resultado, rechazo sin comprobante, devolución pendiente bloqueante y neto después de confirmar devolución. Comprueba lectura por participante, rechazo de participante a conciliación, autorización administrativa, contrato de error 401 y permisos runtime que impiden modificar/borrar recibo e historial.
- La ampliación enfocada vuelve a recorrer timeout → conciliación administrativa exitosa → vencimiento de 24 meses → purga. Compara antes y después todas las filas de historial de liquidación y ledger financiero: los `actor_id` del anfitrión/arrendatario quedan nulos incluso en `liquidacion_fake_incierta`, mientras actores no participantes, códigos, secuencias implícitas, fechas e importes permanecen; el snapshot conserva política/moneda/estado e importes y la relación del anfitrión se minimiza. El mismo caso verifica en la conexión `espacigo_runtime` que solo `actor_id` admite UPDATE, sin permiso de columna adicional ni UPDATE de tabla.
- `npm --prefix mock run test:local-settlement` — PASS, 3 pruebas de clave repetida, exclusión de envíos simultáneos y descarte de respuestas tras cambio de selección o sesión (incluido logout/relogin).
- `python3` con PyYAML analizó `planning/openapi.yaml` como OpenAPI 3.1.0; las tres nuevas rutas de liquidación/conciliación/receipt están presentes.

## Pendientes y límites

- El adaptador fake no mueve dinero. Proveedor real/sandbox, contrato externo, comisión productiva, liquidación real y documento fiscal válido siguen abiertos bajo #109/#110.
- La comisión 0 % está versionada y es exclusiva del ensayo.
- El comprobante es JSON de ensayo, no una boleta. No se ejecutó prueba contra proveedor externo ni GCP.
- MAP-07 difiere campañas, promociones y NPS; no se marcan como implementados/satisfechos.
- La investigación del fallo intermitente de check-in informado previamente queda para la consolidación del runner de pruebas, posterior a esta entrega.

## Ajuste final de minimización del ledger

- Prueba ejecutada tras el último ajuste: `GO_TEST_RUN='TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/booking` — PASS en PostgreSQL/PostGIS desechable, incluyendo el recorrido de timeout, conciliación, purga de retención y comprobación de permisos de columna del rol runtime.
- `git diff --check` — PASS. No se ejecutaron suites ajenas al cambio ni se usó la base persistente o secretos locales.
