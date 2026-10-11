# Evidencia — LOCAL-M04-PUBLICATION-DELETE-01 y LOCAL-M10-BOOKING-CLOSE-01

Fecha: 2026-10-11. Base integrada `main` después de PR #261 (`d5f8668da7747270115c5191dfb81de6af5d13ba`). #258 está en PR #262. #259 se publica en el PR apilado [#263](https://github.com/HernanEspinozaDev/espaciGo/pull/263), con base temporal `codex/local-m04-publication-delete-01`; retargetearlo a `main` tras integrar #262. La implementación inicial de cierre fue `c2107475578b967e63dd79cf30ba504b19d1e407`; la corrección de reseñas posterior a liquidación quedó probada en `4a37314669808930fdf53a06413943a78135dc06`. #246 permanece abierta. Este documento no acepta LOCAL-1.

## LOCAL-M10-BOOKING-CLOSE-01 (#259, RQF-177)

V51 amplía los estados terminales con `cerrada` y el cálculo de retención de vínculos desde `actualizada_en` al completar el cierre. Un trigger de PostgreSQL impide una transición directa a `cerrada` si no existe una liquidación en estado `liquidada`, resultado `exito_simulado` tanto en el agregado como en el evento fake persistido. El cambio de reserva, transición secuencial, liquidación, historial financiero y comprobante se confirma en la misma transacción. Los reintentos reutilizan el resultado persistido y no crean nuevas transiciones ni comprobantes. Los snapshots de precio y ocupación permanecen sin alteraciones.

La aplicación de un éxito revalida reserva, disputa, devolución, pago, garantía, decisión financiera y otras liquidaciones pendientes bajo los bloqueos comunes. Una disputa abierta produce conflicto y no aplica parcialmente la liquidación ni cierra la reserva; al cerrarla administrativamente, la conciliación del mismo evento puede completar el proceso. La retención incorpora `cerrada` sin adelantar vencimientos ni cambiar políticas previas.

El mock etiqueta el estado cerrado, el filtro administrativo acepta `cerrada`, OpenAPI enumera el estado y el hilo de conversación mantiene lectura solamente en `cerrada`. El purgador de vínculos considera `cerrada` terminal, respetando la fecha individual de 24 meses y las obligaciones existentes.

Una reserva `cerrada` tras una liquidación fake confirmada conserva el permiso de reseña vigente para `finalizada`: el repositorio acepta a sus participantes, mantiene la ventana de retención, unicidad e idempotencia. La elegibilidad del mock usa el mismo par de estados; estados en disputa y otros estados no elegibles siguen rechazados. El test de integración de liquidación efectúa el recorrido real éxito fake → transición a `cerrada` → reseñas del arrendatario y anfitrión → reintento idéntico reutilizado. También rechaza una segunda reseña distinta del mismo participante y una reseña de tercero.

## Comprobaciones ejecutadas

- `GO_TEST_RUN='^TestLocalFakeSettlementBoundaryTimeoutRestartAndConcurrentReplay$' bash scripts/test-m06-local-booking-postgres.sh` — PASS en PostgreSQL/PostGIS desechable con rol `espacigo_runtime`. El test aplica migraciones desde V1 hasta V51 en una base temporal eliminada al salir del runner. Comprueba límite checkout+24h, éxito/rechazo, timeout/reinicio/reconciliación, dos reintentos concurrentes, una sola historia/comprobante/transición a `cerrada`, protección SQL contra cierre con liquidación rechazada o resultado aún no aplicado, conflicto mientras una disputa está abierta, aplicación posterior del mismo evento al cerrar la disputa, preservación de snapshots/ocupación y retención independiente de 24 meses para el estado cerrado. Añade las reseñas de ambos participantes sobre la reserva liquidada y cerrada, el reintento idempotente, el conflicto para otra reseña del mismo autor y el rechazo de un tercero.
- `go test ./internal/adapters/postgres/booking ./internal/adapters/postgres/identity ./internal/booking/transport/http ./internal/booking/...` — PASS. La integración que necesita PostgreSQL no se considera cubierta por esta ejecución; se ejecutó aparte mediante el runner anterior.
- `go vet ./internal/adapters/postgres/booking ./internal/adapters/postgres/identity ./internal/booking/transport/http ./internal/booking/...` — PASS.
- `npm --prefix mock run test:rental-operations` — PASS, 8 casos; incluye `cerrada` en estados de solo lectura y conserva pruebas de sesión/selección asíncrona.
- `npm --prefix mock run test:local-settlement` — PASS, 3 casos; estado de intento y reintento de liquidación fake.
- `npm --prefix mock run test:review-eligibility` — PASS; habilita solo `finalizada` y `cerrada`, excluyendo disputa, estados activos y cancelados.
- La compilación TypeScript corre en ambos scripts npm anteriores. `git diff --check` — PASS.

Las pruebas corren en entornos desechables. V50/V51 no se aplican al volumen de desarrollo mientras #262/#263 estén pendientes de revisión/merge. Se conserva `espacigo_pgdata`, secretos y datos. No se ejecuta el runner integral de LOCAL-1 hasta completar y aceptar #258 y #259; sin GCP.
