# LOCAL-PRIV-01A3 — retención y reaplicación tras restore

Subentrega de #194 bajo LOCAL-PRIV-01A / #185 y PRIV-BE-02 / #40. Alcance exclusivo de cuentas y hechos sintéticos locales sujetos a `privacidad_local_v1`. No declara supresión integral ni anonimización y no comprende datos reales, backups productivos o GCP.

## Decisiones y comportamiento

- Una reserva terminal recibe `vinculos_retirar_en` a 24 meses calendario desde el último cierre terminal relacionado. El purgador usa lotes acotados, vuelve a revisar plazo y obligaciones bajo bloqueo, y solo retira referencias identificables cuando no queda reserva, pago, devolución o disputa abierta. También difiere resultados del fake sin evento autenticado coincidente, eventos sin fila de procesamiento y aplicaciones en estado `pendiente` o `pendiente_conciliacion`; retoma el purgado cuando la conciliación llega a un estado terminal. Preserva los hechos financieros, snapshots, ocupaciones e historiales sujetos a su retención.
- La migración V26 es incremental: registra vencimientos de reservas terminales existentes y crea un marcador idempotente de purga, un índice parcial de vencimiento y una bitácora de reaplicación. No reconstruye tablas ni modifica V25.
- Un registro técnico de ejecuciones completadas se guarda fuera de PostgreSQL y del volumen `espacigo_pgdata`: `${XDG_STATE_HOME:-~/.local/state}/espacigo/privacy-replay/completed-suppressions-v1.json`. Contiene UUIDs técnicos de ejecución, solicitud y cuenta, más fechas; excluye correos, credenciales, hashes, tokens, archivos y texto libre. El directorio es `0700` y el archivo `0600`.
- La sincronización **une** las ejecuciones de la base vigente con las del archivo externo; nunca sustituye el registro por un subconjunto. Tras restaurar una base anterior, el arranque del worker conserva las bajas que solo existan en el registro lateral. Un archivo inválido, con versión desconocida o entradas contradictorias detiene la exportación y no se sobrescribe.
- Tras completar una baja, el Backend actualiza ese registro por reemplazo atómico. Si la escritura falla, la respuesta informa que la baja ya se completó y que el registro requiere reintento; repetir la operación idempotente vuelve a sincronizarlo. El worker y el comando de exportación realizan la misma unión. La frontera entre commit PostgreSQL y escritura en filesystem no es una transacción distribuida: ante una caída abrupta entre ambos pasos, exporta desde la base vigente antes de restaurar una copia anterior.
- Antes de crear un backup local, exporta el registro. Después de restaurar una copia anterior en el entorno local, genera un UUID nuevo para esa restauración y reaplica el registro usando una cuenta administradora activa. La reaplicación vuelve a comprobar los bloqueadores de la baja; no reactiva cuentas ni elimina registros históricos.
- El procedimiento solo protege respaldos que incluyan el registro exportado y que se reapliquen mediante el comando. Copias antiguas, externas o no inventariadas no se consideran corregidas ni eliminadas. No se borra el volumen ni los secretos.

## Procedimiento local

```bash
# Exportar antes de producir/copiar un backup local.
scripts/privacy-local-replay.sh export

# Después de restaurar un backup anterior mediante el procedimiento local,
# proporcionar un UUID nuevo para identificar esa restauración y el UUID de
# una cuenta administradora activa.
scripts/privacy-local-replay.sh replay <restore-uuid> <admin-account-uuid>

# Ejecutar el purgado vencido en lotes acotados desde la API local:
scripts/privacy-local-replay.sh purge
```

No pases credenciales en argumentos. El registro predeterminado vive fuera del repositorio y de `pgdata`; conserva sus permisos y copia ese archivo por el mecanismo local de backup que corresponda. No ejecutes una restauración de prueba sobre el volumen persistente.

## Evidencia de implementación

- `V000026__local_privacy_retention_purge_and_replay.sql` añade retención, marcadores idempotentes y ledger de reaplicación de forma incremental.
- `internal/adapters/postgres/identity/retention.go` purga únicamente vencidos elegibles, conserva hechos históricos y reaplica bajas bajo las comprobaciones existentes.
- `internal/privacy/service.go`, `cmd/api/local_privacy.go`, `scripts/privacy-local-replay.sh` y el montaje separado de Compose mantienen/exportan el registro de recuperación fuera del volumen.
- Pruebas PostgreSQL aisladas ejecutadas con el rol `espacigo_runtime`:
  - `TestLocalReservationRetentionPurgeRespectsDeadlineAndKeepsHistoricalFacts`
  - `TestLocalReservationRetentionDefersOpenDisputeAndRecalculatesFromClosure`
  - `TestLocalReservationRetentionWaitsForFakePaymentReconciliation`
  - `TestLocalSuppressionReplayRestoresAnOlderDatabaseSnapshotIdempotently`
  - `TestLocalSuppressionReplayAfterPgDumpRestore` (dump y restore en una base desechable).
- Las pruebas verifican que antes del vencimiento no se purga, que la disputa abierta y una conciliación fake pendiente difieren el trabajo, que el purgador reanuda después del cierre de conciliación, que se preservan hechos, que una restauración anterior seguida del arranque del worker conserva el archivo externo y reaplica la baja leyéndolo de disco, y que repetir reaplicación no duplica efectos. También comprueban permisos `0600` y que el registro no contiene correo, hashes ni tokens.

## Pendientes

La sincronización filesystem/PostgreSQL no es atómica ante una caída en la ventana commit/export; el procedimiento recuperable es exportar desde la base vigente antes de restaurar un backup anterior. No se ha inventariado ni purgado el conjunto de respaldos ya existentes. La operación local no completa los criterios integrales de #185/#40, no establece retención productiva/legal y no equivale a supresión integral ni anonimización.
