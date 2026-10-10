# LOCAL-ADMIN-01D — cobertura financiera de auditoría local

Fecha: 2026-10-10. Base conciliada: `main` `cfdbcc1941de58c4db07b554560c877cd01ea1ea` (merge #234). PR de esta rama pendiente de revisión/merge.

## Propósito y límite

Esta subentrega vertical de #112–#114 reutiliza `evento_auditoria_local`, el ownership financiero de M10 y las pruebas de outbox/inbox aceptadas. La inspección de productores encontró tres mutaciones administrativas financieras sin evento central transaccional: decisión de deducción, operación fake de captura/liberación y conciliación administrativa de un resultado fake. V43 permite claves idempotentes para esas acciones; el Backend escribe sus eventos dentro de las mismas transacciones que cambian la decisión/operación/historial.

La autorización fake iniciada por el arrendatario y los vencimientos automáticos siguen registrados por sus mecanismos/historiales dueños; no son acciones de administrador. La auditoría no duplica importes, resultado del proveedor fake, cuerpos, motivos libres ni payloads. `resultado=exito` describe que se aceptó y persistió la petición administrativa; el resultado financiero fake permanece en su tabla dueña. Las claves y correlaciones del ledger se derivan mediante SHA-256 de la clave/operación, no guardan la clave idempotente enviada en claro.

No se creó un publicador, tabla ni retención universal: credenciales conservan `outbox_evento_local`/ciclos V22/V27; avisos de ensayo conservan `aviso_local`/ciclos V36/V37 y su política local; los resultados entrantes de pago fake conservan su inbox/eventos V40. Cada owner mantiene su proceso de reintento y vencimiento. RNF-043 de cinco años se aplica solo a `evento_auditoria_local`.

## Clasificación de #112–#114

| Criterio | Estado | Evidencia / pendiente |
|---|---|---|
| Ledger local, retención, consulta/exportación y minimización | Aceptado localmente como base; consulta/exportación aceptada en #228 | V22+ y evidencia `local-admin-01b-20261009.md`; no prueba la cobertura de todo productor posible. |
| Productores existentes de credenciales, privacidad, KYC, moderación, reclamos, gobierno y consultas admin | Aceptado localmente por owner/slice | Inventario en `diseno_local_admin_arch_01.md`; reutiliza historial y writers actuales. |
| Auditoría de decisión financiera, operación de captura/liberación y conciliación admin | Brecha funcional implementada en este PR; pendiente de aceptación | V43 + `internal/adapters/postgres/booking/guarantee.go`; prueba de rollback y deduplicación incluida. |
| Tablas/grants para un outbox universal | Decisión local: no justificado por productores/consumidores de la base integrada | Los tres mecanismos dueños ya dan entrega/conciliación a sus usos. V43 solo amplía el catálogo de acciones idempotentes de auditoría; reusa grants de runtime existentes. No se asigna un plazo compartido. |
| Campañas/promociones/NPS | Decisión pendiente/diferida | MAP-07 no define finalidad, productores, permiso ni retención; no se implementa. |
| Recuperación de workers existentes | Aceptado localmente y evidencia reutilizada, sin cambio en esta entrega | Outbox credenciales #198/#199, avisos #220 y fake payment #74; sus suites no se repitieron porque no se modificaron. |
| Pub/Sub/BigQuery, almacenamiento inmutable productivo, proveedores externos | Integración externa | Fuera del cierre local y de esta entrega; sin GCP. |

No se cierran #112, #113, #114 ni #111: el alcance general de administración conserva reportes/gobierno/acciones futuras, la matriz de productores debe revisarse cuando aparezca un nuevo owner o consumidor, y el padre mantiene su dependencia nativa #110. La API de Projects siguió sin ser necesaria para implementar el código; el estado operativo del tablero no se afirma actualizado.

## Prueba enfocada

Comando ejecutado:

```sh
GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' \
  bash scripts/test-m04-attributes-postgres.sh \
  ./internal/adapters/postgres/booking \
  ./internal/dbbootstrap \
  ./internal/migrator
```

Resultado: **PASS**. El script crea una base PostgreSQL/PostGIS desechable en tmpfs, aplica las migraciones desde V1 hasta V43, crea el rol `espacigo_runtime`, verifica que el repositorio usa ese rol y elimina el contenedor/socket al terminar. En el recorrido del booking integrado:

- la inserción de auditoría fallida revierte la decisión financiera y su historial, sin filas parciales;
- decisión/replay, captura fake, conciliación y liberación fake dejan exactamente 1/2/1 eventos respectivos, todos del administrador y la reserva correctos;
- los eventos conservan códigos mínimos, `detalle_codigos` vacío de obligaciones, correlación/clave derivadas y vencimiento exacto a cinco años;
- los reintentos no agregan eventos duplicados, y los resultados/montos siguen en las tablas financieras dueñas;
- migración V43 pasa sobre esquema vacío usando grants de runtime ya configurados.

Una ejecución de repetición anterior falló una sola vez en la carrera preexistente de check-in (primer resultado `ErrConflict`, reintento `nil`) antes de los asertos financieros. La ejecución siguiente pasó completa; el síntoma no se repitió y no toca la ruta de auditoría agregada. Se conserva como observación de estabilidad del test integrado, no se oculta como un resultado PASS adicional.

No se ejecutaron suites de los workers M01/M09/M06 ya aceptados porque esta entrega no los modifica. No se accedió ni alteró la base persistente `espacigo_pgdata` ni secretos locales. No se ejecutó GCP.

Se comprobó también que V43 conserva los writers existentes afectados por el constraint compartido:

```sh
GO_TEST_RUN='^(TestCredentialNoticeTerminalCycleAdminRecoveryAndRetention|TestSyntheticReviewReciprocityModerationRetentionAndRuntimePermissions)$' \
  bash scripts/test-m04-attributes-postgres.sh \
  ./internal/adapters/postgres/identity \
  ./internal/adapters/postgres/reputation
```

Resultado: **PASS** en bases desechables desde cero; outbox de credenciales/reapertura y auditoría de moderación siguen aceptando sus acciones existentes. El camino SMTP real se omitió en este comando; su entrega a Mailpit ya está evidenciada en el corte #220 y no cambió aquí.

## Reconciliación de planificación

`matriz_cierre_backend_local.md`, `backlog_cierre_backend_local.md`, `trazabilidad_requisitos_backend_local.md`, `plan_cierre_backend_local.md` y `diseno_local_admin_arch_01.md` se actualizaron contra `main` #234. Se corrigió que publicación local M04, contrato/firma M07 y garantía/deducción fake M10 aparecieran como no implementados. Sus padres siguen parciales/abiertos, sin convertir estos cortes en aceptación general.
