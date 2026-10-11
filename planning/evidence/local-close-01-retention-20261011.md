# LOCAL-CLOSE-01B — purgadores de retención local

Estado: #248 aceptada/Hecho tras merge de PR #252 (`6e96d8f`) y aplicación incremental de V47 al volumen local. La regresión integral de LOCAL-1 se difiere hasta resolver #249/#250. No se ejecutó purga forzada ni se adelantaron vencimientos. Los plazos son los ya ratificados en `privacidad_local_v1`; este cambio no agrega ni generaliza periodos.

## Comportamiento implementado

- El purgador por lotes procesa cotizaciones no convertidas después de `vence_en + 90 días`, aceptaciones cinco años después de la baja y solicitudes de derechos cinco años después de su resolución. Las versiones compartidas de términos y las cotizaciones convertidas quedan fuera. Una solicitud de supresión solo se elimina junto a su ejecución completada y los jobs de archivo ya completados; una limpieza pendiente no obtiene una fecha artificial ni se purga.
- Para verificaciones conserva la fecha terminal y `retirar_en` originales. El archivo sintético se retira a los 90 días del estado terminal, sin exceder una baja elegible. El nuevo `archivo_id` es la referencia de almacenamiento separada del ID de metadata. Una cola con lease persiste intentos y próximo vencimiento. Si falla `Delete`, el job se reintenta y la referencia se conserva. Tras confirmar el borrado, se pone `archivo_id = NULL`; durante el periodo restante la metadata indica `file_available: false` y los endpoints de contenido responden not-found.
- Al cumplir dos años, el purgador borra metadata, historial, proyección de elegibilidad y caso en ese orden. También desliga reintentos descendientes para respetar las FKs. El runtime no puede borrar directamente esas tablas: ejecuta funciones acotadas `SECURITY DEFINER`.
- La limpieza de evidencia de baja por privacidad continúa en su worker V25 separado. El nuevo purgador excluye expedientes `retirada_privacidad` al programar archivos, por lo que ambas colas no compiten. Workers y altas KYC conservan el orden cuenta → verificación donde se eliminan expedientes.
- La ruta administrativa y el worker existente invocan ahora las clases ratificadas; los contadores detallan términos, solicitudes, cotizaciones, verificaciones y archivos en cola/retirados/diferidos. No se purgan auditorías ni hechos financieros con el plazo de otra tabla.

## Validación enfocada

Las integraciones se ejecutaron en PostgreSQL desechable creado por `scripts/test-local-comm-mailpit.sh`; no se usó ni se migró `espacigo_pgdata` para V47. Ambos recorridos obligatorios están registrados en `scripts/local-required-integrations.txt`, por lo que `scripts/test-local-qa.sh` falla si los omite.

- `GO_TEST_RUN='^TestRatifiedLocalRetentionPurgesDueRowsAndRecoversPrivateFileDeletion$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity` — PASS. Con reloj inyectado prueba instante previo, vencimiento, fallo simulado de borrado, referencia intacta, reconstrucción del servicio (recuperación tras reinicio), éxito posterior, referencia limpiada y metadata retenida hasta los dos años. Al segundo vencimiento verifica eliminación de proyección, historial, evidencia y caso; además purga solicitud/términos/cotización no convertida y conserva reserva y cotización convertida.
- `GO_TEST_RUN='^TestUpgradeFromV46BackfillsVerificationEvidenceFileReferences$' bash scripts/test-local-comm-mailpit.sh ./internal/migrator` — PASS. Parte de V46 con un expediente terminal, una fila de evidencia preexistente y su PNG privado; aplica únicamente V47 y verifica referencia/archivo intactos, vencimiento calculado desde la fecha terminal original y restricción instalada y validada. No purga antes de la fecha.
- `GO_TEST_RUN='^TestVerificationRepositoryOwnershipIdempotencyReviewAndRetry$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/verification` — PASS. La migración desde la base desechable y operaciones de evidencia subyacentes siguen funcionando con la nueva referencia de archivo.
- `GO_TEST_RUN='^TestLocalCompleteExportZIPIsOwnerScopedReadOnlyAndAvailableWithBlockedSuppression$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity` — PASS. Reutiliza la prueba de exportación ZIP con evidencia privada propia.
- `npm --prefix mock run build` — PASS.
- `go test ./internal/privacy ./internal/verification/... ./internal/adapters/postgres/verification ./internal/adapters/postgres/identity ./internal/adapters/postgres/ownerexport ./internal/identity/transport/http ./cmd/api` — PASS. El test PostgreSQL sin variable de conexión se ejecuta/omite según su guardia; los escenarios de integración citados arriba sí corrieron con el wrapper desechable.
- `git diff --check` — PASS.

V47 añade columnas, completa filas preexistentes y luego incorpora la restricción `NOT VALID` para ejecutar `VALIDATE CONSTRAINT` sobre datos backfilleados.

V47 está fusionada y aplicada incrementalmente. Se verificaron migración, permisos runtime y salud HTTP sin ejecutar la purga. #248 está cerrada/Hecho por su alcance. #246 sigue abierta/En curso; #185/#40 y los padres de módulos mantienen sus criterios generales pendientes. No se ejecutó trabajo de GCP.
