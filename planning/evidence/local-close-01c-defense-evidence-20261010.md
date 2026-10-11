# LOCAL-CLOSE-01C — evidencia privada sintética en descargos

Estado: implementación propuesta para revisión en el PR de #249. V48 se probó en bases desechables mediante el runner; no se aplicó a `espacigo_pgdata`. #250 sigue esperando la aceptación de #249. #246 y los padres #185/#40 permanecen abiertos.

## Recorrido y tratamiento

- El arrendatario que ya presentó un descargo puede solicitar una única operación idempotente que crea una PNG fija en Backend. La solicitud no acepta bytes ni ubicación del navegador. Solo procede mientras el reclamo siga abierto y la reserva esté en `en_disputa`; la operación y su historial comparten los bloqueos existentes de cuenta/reserva.
- Anfitrión y arrendatario consultan el contenido por el endpoint privado de evidencia de la reserva. La API devuelve 404 a terceros. Administración puede consultar el adjunto por la ruta de evidencia del reclamo con rol activo. El mock presenta el botón en el descargo y vuelve a validar reserva, cuenta, token y generación después de leer el blob.
- El ZIP añade `synthetic_damage_claim_defenses`, limitado a descargos escritos por el titular, y el archivo PNG propio. Excluye adjuntos escritos por otra cuenta, contactos y datos privados de la contraparte. Si el archivo no está disponible, deja una exclusión explícita en el manifiesto.
- La baja elegible reutiliza el job durable de archivos; el contenido se elimina antes de registrar la finalización. Si falla el borrado, queda pendiente recuperable. Al vencer la retención ya ratificada de 24 meses del conjunto reserva/disputa, V48 encola la misma limpieza durable y posterga retirar vínculos hasta confirmarla. No se introduce plazo nuevo ni se purgan metadatos/historial antes de lo ratificado.
- Tras limpiar el blob, las APIs, el ZIP y los listados de evidencia lo excluyen, mientras los hechos mínimos permanecen. Los identificadores y vínculos se retiran después en la transacción existente de purga. No se declara anonimización ni validez probatoria.

## Validación enfocada

Las integraciones PostgreSQL usaron el entorno desechable con PostGIS y Mailpit previsto en `scripts/test-local-comm-mailpit.sh`; no tocaron el volumen persistente.

- `GO_TEST_RUN='^TestSyntheticDamageDefenseEvidence(ParticipantAdminExportIsolation|RetentionUsesRecoverableCleanup|SuppressionQueuesRecoverableCleanup)$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity` — PASS. Cubre creación HTTP idempotente con runtime, lectura por ambas partes y administrador, 404 a tercero, PNG real dentro del ZIP propio y fuera del ZIP del anfitrión; simula el vencimiento con reloj sustituible y verifica cola durable, borrado, metadatos mínimos y posterior purga; verifica también baja, fallo de borrado y recuperación.
- `GO_TEST_RUN='^TestResolvedDamageClaimPrivacyEvaluationMinimizesTextAndPurgesLinksAtRatifiedDeadline$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity` — PASS. Comprueba la respuesta administrativa de reclamo posterior a purga contra el esquema OpenAPI vigente, incluida la lista vacía de evidencia.
- `go test ./internal/operation/... ./internal/damageclaim/... ./internal/adapters/postgres/ownerexport` — PASS.
- `npm --prefix mock run build` — PASS.
- `node --test mock/test/damage-claim-request-state.test.mjs` — PASS (5 pruebas), incluida respuesta tardía al cambiar reserva y al cerrar/iniciar otra sesión de la misma cuenta.
- `git diff --check` — PASS.

## Límites

Solo archivos PNG sintéticos generados en servidor. No se acepta foto real ni archivo proporcionado por cliente. No se añaden efectos financieros, adjudicación ni avisos. La baja continúa siendo minimización con retención residual; no completa supresión integral. #250, los criterios generales de M08/M10, #185/#40 y el cierre LOCAL-1 quedan pendientes. No se ejecutó GCP ni se modificó el volumen o los secretos locales.
