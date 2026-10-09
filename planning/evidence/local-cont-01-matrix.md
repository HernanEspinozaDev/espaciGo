# LOCAL-CONT-01 — matriz de implementación y evidencia

Estado: listo para revisión en PR. Issue local #216, hija de #80; los padres #80–#87 siguen abiertos.

| Criterio | Resultado del corte | Evidencia nueva |
|---|---|---|
| Contrato inicia desde reserva aprobada, snapshot conserva intervalo, monto, moneda, zona, condiciones y política | Implementado localmente; el recuperador idempotente termina creaciones interrumpidas después de aprobación | `V000034`, `internal/adapters/postgres/contracts/repository.go`, `EnsureApproved` |
| Solo anfitrión/arrendatario leen; terceros reciben 404 | Implementado en repositorio y handler | Prueba PostgreSQL de acceso de tercero añadida al lifecycle M06 |
| Firma parcial y final por los dos participantes; repetición sin historial duplicado | Implementado con bloqueo común de reserva | Pruebas PostgreSQL de ambas firmas y replay |
| Rechazo terminal del firmante, sin cancelación inmediata | Implementado; M06 queda `firma_parcial` hasta el límite | Prueba PostgreSQL de rechazo y consulta posterior |
| Inicio estricto anterior para las firmas; vencimiento `>= start_at` | Implementado con reloj consultado después del lock | Prueba en límite exacto y carreras concurrentes signo/worker |
| Cancelación y liberación de ocupación atómicas; devolución única por importe fake confirmado | Implementado en M06; estado `cancelada_por_firma`; la devolución queda como obligación fake reutilizando el endpoint existente | Pruebas PostgreSQL de una transición, una devolución e importe total |
| Cancelación local_flexible_v1 antes del inicio tras firma parcial o completa | M06 y M07 coordinan por bloqueo de reserva; cancelación invalida solo contratos incompletos, mantiene completos como hechos históricos y registra una devolución única del importe pagado | Integración PostgreSQL: después de 1 y 2 firmas, autorización del renter, importe completo, ocupación liberada y reintentos sin firma/refund duplicados; carreras firma→cancel y cancel→firma serializadas |
| Bloqueador de baja con reserva en estado de contrato parcial o completo | Evaluación de privacidad incluye `firma_parcial` y `lista_para_checkin` como reserva activa | Integración identity/PostgreSQL con cada estado y titular independiente |
| CORS de rutas de contrato | Política de orígenes permitidos aplicada antes de auth y servicio, incluyendo errores, preflight y PDF | Prueba HTTP del handler con origen permitido/no permitido, error 401, OPTIONS y descarga |
| Conversación M09 durante contrato parcial/completo | Participantes escriben en `firma_parcial` y `lista_para_checkin`; terminalización conserva lectura sin nuevos mensajes | PostgreSQL verifica envío/replay en ambos, 404 a terceros y rechazo después de cancelación o expiración directa |
| Artefacto privado | PDF sintético, metadata/bytes bajo owner M09, cifrado AES-256-GCM; descarga autenticada solo tras ambas firmas | Pruebas de magic/EOF, cifrado, descifrado y detección de manipulación; prueba PostgreSQL valida PDF descifrado |
| Mock y sesión | Controles de participante, rechazo, firma, estado, historial y descarga; invalida operaciones al cambiar reserva/sesión | `npm --prefix mock run test:contract-actions`; compilación TS |
| OpenAPI | Rutas de creación/consulta/firma/rechazo/descarga y esquemas añadidos | `planning/openapi.yaml`; parse YAML correcto y 5 rutas de contrato |

## Límite de aceptación

El corte es exclusivamente sintético y cada interfaz/artefacto lo identifica como ensayo sin validez jurídica. No prueba un proveedor de firma, callback externo ni validez legal. El recuperador local busca contratos faltantes tras aprobación y cancela reservas aprobadas sin contrato/firma cuando el reloj Backend llega al inicio. Los avisos durables de contrato, exportación del nuevo artefacto por participante y retención productiva quedan fuera y no cierran M07 ni privacidad.

## Comprobaciones

Validación completada en esta rama:

- `GO_TEST_RUN=TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry bash scripts/test-m06-local-booking-postgres.sh` — PASS sobre PostgreSQL/PostGIS desechable con rol runtime. Incluye ciclo de reserva/pago fake, participantes y tercero, aprobación/rechazo, firmas parcial/completa, cancelación después de 1 y 2 firmas, conflicto a inicio, devolución única, locks cruzados y liberación de ocupación.
- `GO_TEST_RUN=TestSuppressionTreatsSignedContractReservationStatesAsActive bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity` — PASS en PostgreSQL desechable para `firma_parcial` y `lista_para_checkin`.
- `go test ./internal/contract/transport/http ./internal/adapters/postgres/contracts ./internal/adapters/postgres/booking ./internal/adapters/postgres/identity ./cmd/api` y `go vet` sobre los mismos paquetes — PASS.
- `npm --prefix mock run test:contract-actions` — PASS, 8 pruebas de pago/cancelación, permisos del contrato y respuesta tardía.
- `npm --prefix mock run test:contract-actions` — PASS, 10 pruebas; agrega permisos de envío de conversación para ambas partes durante las dos fases de firma y conserva solo lectura en estados terminales.
- El envío nuevo en `firma_parcial` procesa el vencimiento M07 bajo el orden de locks cuenta→reserva→contrato antes de responder conflicto; no agrega mensaje, conserva el replay idempotente previo y deja una sola transición/devolución.
- Recorrido real del mock contra API/PostgreSQL local persistente (2026-10-09): dos cuentas sintéticas verificadas y con KYC local aprobado; catálogo → detalle → cotización CLP 8.000 → solicitud/retención → pago fake → aprobación del anfitrión → firma anfitrión → firma arrendatario → vista previa y cancelación `local_flexible_v1` antes del inicio → devolución fake completa de CLP 8.000. El panel mostró `firmado`, `cancelada_arrendatario`, devolución `completada` y el historial secuencial. No se ejecutó ninguna transferencia real. Las cuentas, sus dos casos KYC, el fixture autorizado y la reserva sintética de recorrido permanecen en la base persistente; no se borraron datos ni se regeneraron secretos.
- Compilación TypeScript del mock y parseo de `planning/openapi.yaml` — PASS.
- `git diff --check` — PASS.

La base persistente conservó su volumen, datos previos y secretos; V34 se aplicó incrementalmente al levantar el entorno para esta entrega. El recorrido anterior sí se ejecutó desde el mock en navegador contra ese entorno. Las carreras y casos de privacidad se comprobaron en PostgreSQL desechable con reloj/controles deterministas. No se ejecutaron suites ajenas a este corte ni pruebas/despliegues de GCP.
