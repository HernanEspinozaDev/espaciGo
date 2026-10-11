# LOCAL-CLOSE-01E — exportación contractual y conservación sintética

Fecha: 2026-10-11. Rama `codex/local-close-01-contract-export`; base `main` #265, `6358b26cc891977b8503f96bece033d34ebc631b`. Commit funcional probado: `03e9c77fb08815b63f2c8d8b3cc96ffe635ca491`. Alcance LOCAL-1 sintético; #246 permanece abierta hasta aceptación explícita.

### Corrección adicional del purgador (PR #266)

El purgador ahora recalcula el plazo efectivo con el último cierre relacionado después de bloquear participantes y reserva. Si el `vinculos_retirar_en` almacenado quedó viejo, lo actualiza y difiere el retiro en vez de depender de un lote anterior. Para historiales de espacio, primero selecciona y revalida los espacios elegibles tras locks de usuario/reserva; el mismo conjunto se usa para eliminar el historial de publicación y el de eliminación dentro de la transacción.

Pruebas focalizadas en PostgreSQL desechable:

```sh
GO_TEST_RUN='^TestLocal(ContractRetentionRevalidatesRecentlyClosedDisputeOutsideFirstBatch|PublicationHistoryBatchSkipsBlockedDeletionAndPurgesSameEligibleSet|ContractRetentionDefersObligationsAndRecoversAfterRollback|SignatureExpiryRetentionSchedulesNullDeadlineDefersAndPurgesIdempotently)$' \
  bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity
```

Resultado: PASS en las tres pruebas previas. La primera coloca una reserva con fecha almacenada vencida detrás de otra reserva en el primer lote; al llegar al segundo lote, el cierre reciente de una disputa actualiza el vencimiento a `cerrada_en + 24 meses calendario` y conserva contrato/PDF. La segunda usa `p_limit=1`: una eliminación antigua con pago pendiente queda intacta y no consume el lote; la eliminación posterior elegible retira ambos historiales, mientras ambas filas del espacio bloqueado permanecen. La regresión de firma vencida está documentada a continuación. El manifiesto de integraciones obligatorias incluye las cuatro pruebas. La actualización de V52→V54 se ejecutó en migrador desechable.

### V54 — retención por vencimiento de firmas

V53 permanece intacta. V54 añade `cancelada_por_firma` a la programación e índice de vencimientos de vínculos, y repara fechas nulas heredadas. El purgador también admite `vinculos_retirar_en IS NULL`: vuelve a leer la fecha después de bloquear participantes y reserva, calcula y persiste el plazo efectivo antes de decidir si elimina. La purga de vínculos reconoce el mismo estado terminal.

`TestLocalSignatureExpiryRetentionSchedulesNullDeadlineDefersAndPurgesIdempotently` verifica programación a 24 meses calendario, conservación antes del plazo, reparación de una fecha nula vencida, aplazamiento por devolución pendiente, extensión desde el cierre posterior de la devolución y retirada final de contrato, PDF y vínculos. Repetir ambas purgas no duplica efectos. La prueba incremental V52→V54 verifica además la inclusión del estado en el índice.

## Decisiones ratificadas e implementadas

- Los almacenes especializados por dominio son la adaptación local de MAP-02/DB02-02: un recurso propietario por documento, autorización del módulo dueño, almacenamiento privado y limpieza recuperable. No se creó un almacén global ni se modificaron ES1/ES2.
- Contratos, firmas/historial y PDF sintético: retención hasta 24 meses calendario desde el último cierre terminal relacionado. Una obligación abierta difiere la eliminación.
- Historiales de publicación y eliminación: 24 meses calendario desde la eliminación definitiva, salvo obligaciones abiertas. Consultas, reintentos y exportaciones no reinician vencimientos.
- Estas son reglas del prototipo sintético local, no plazos legales o productivos.

## Entrega de datos propios

El endpoint ZIP de exportación incluye la sección versionada de contratos para anfitrión y arrendatario autorizados. Usa el repositorio propietario para validar participación y exporta snapshot minimizado, estado, firmas e historial minimizados y PDF privado solo si sigue disponible. No incluye claves de cifrado, secretos, identificadores personales de terceros ni documentos de otras reservas/cuentas. Si el PDF fue purgado antes, el manifiesto lo identifica como retirado y no intenta reconstruirlo. Exportar no muta el contrato ni reinicia su vencimiento. La exportación sigue disponible aunque la solicitud de supresión esté bloqueada.

## Persistencia y limpieza

V53 permite representar documentos ya retirados sin perder los hechos contractuales, añade fechas de retención a historiales de publicación/eliminación y backfillea filas anteriores desde la fecha de eliminación con meses calendario. El purgador conserva contratos/historiales mientras detecta obligaciones pendientes; elimina metadata, referencias y PDF privado en la misma unidad transaccional y puede reintentarse después de un fallo. El rol runtime no recibe modificación/borrado directo de ledgers append-only; solo la función controlada puede retirar filas vencidas.

## Pruebas

Pruebas PostgreSQL focalizadas ejecutadas contra recursos desechables:

```sh
GO_TEST_RUN='^TestLocalContract(ExportIsParticipantScopedMinimizedAndReportsRetiredPDF|RetentionDefersObligationsAndRecoversAfterRollback)$' \
  bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity
GO_TEST_RUN='^TestUpgradeFromV52BackfillsLocalDomainRetention$' \
  bash scripts/test-local-comm-mailpit.sh ./internal/migrator
go test ./internal/adapters/postgres/contracts ./internal/adapters/postgres/ownerexport \
  ./internal/privacy ./internal/dbbootstrap ./internal/migrator ./cmd/api
```

Resultados: ambas pruebas de exportación/retención y la actualización V52→V54 pasaron. La exportación se validó como ZIP real: cuenta anfitriona y arrendataria reciben solo su parte autorizada; un tercero no accede; la respuesta incluye JSON minimizado y PDF válido disponible, omite secretos/IDs ajenos, y con PDF retirado informa el estado en el manifiesto sin reconstituir archivo. Los plazos se mantienen ante exportación. La prueba de purga cubre estado previo al vencimiento, obligación pendiente, permisos runtime, rollback/reintento y retirada de archivo/referencias después de resolver la obligación.

La regresión integral final se ejecutó en el commit corregido `03e9c77fb08815b63f2c8d8b3cc96ffe635ca491`:

```sh
bash scripts/test-local-qa.sh
```

Resultado: **PASS**. 100/100 pruebas mock; gate 4/4; `go test -p 1 ./...`; `go vet ./...`; 89/89 integraciones obligatorias, con 127 eventos de pruebas Go; cero integraciones omitidas o `SKIP`; check-in estable 5/5. El runner también verificó la limpieza de sus contenedores/red/directorio temporales. El commit ya está en `scripts/local-required-integrations.txt` para estas integraciones.

## Protección de datos de desarrollo

V53/V54 se migraron desde V52 solo en PostgreSQL desechable. No se aplicaron al volumen persistente `espacigo_pgdata`; no se borraron volúmenes, datos sintéticos o archivos de secretos, ni se regeneraron secretos. No se ejecutó ni preparó trabajo de GCP.

## Cierre LOCAL-1

La matriz vigente no identifica brechas funcionales locales obligatorias restantes para LOCAL-1. Los Issues padre y #185/#40 continúan abiertos por criterios generales, productivos o externos; un slice local no los cierra. Permanecen separados proveedores reales/sandbox, validez jurídica, documentos fiscales válidos, movimientos reales de fondos, campañas/promociones/NPS diferidos por MAP-07 y ubicaciones de backups no comprobadas. Esto no equivale a supresión integral o anonimización. #246 continúa abierta a la espera de revisión de esta entrega y aceptación explícita del propietario.
