# LOCAL-OPS-01 — matriz y evidencia

Estado: implementación del PR #218 y correcciones focalizadas del PR #219 para revisión. Issue #218, hija de #88; Issues originales M08/M10 permanecen abiertas. No es aceptación de los criterios generales.

## Requisitos y cobertura

| Criterio | Implementación | Evidencia/verificación | Estado y límite |
|---|---|---|---|
| Check-in solo arrendatario, contrato totalmente firmado y día local del inicio | `internal/adapters/postgres/operations/repository.go`; valida estado `lista_para_checkin`, ambas filas de firma y `time_zone` snapshot; comparte lock de cuentas → reserva → candidato. | Integración PostgreSQL usa hora inyectada para intento en día posterior (409 sin cambio) y registra al inicio; prueba de dos requests concurrentes con misma clave. | Parcial local; no representa lectura de acceso físico. |
| PNG privado, actor, hora Backend, ubicación sintética | `internal/operation/service.go`; `evidencefs.Store` compartido; metadata/bytes separados y candidato recuperable; el servidor crea `synthetic-png-v1` y persiste `santiago-demo-center-v1`. | Integración verifica una evidencia, descarga por participante y 404 a tercero. Los tests de evidencia privada existentes cubren permisos/directorio privado. | Sintético; sin GPS, fotos reales ni geolocalización probatoria. |
| Check-out desde `en_curso` y gracia desde instante guardado | Repositorio cambia estado, inserta transición y operación; plazo calculado desde `ocurrio_en` del check-out. | Integración avanza reloj inyectado, consulta la fecha persistida + 24h y comprueba estado `finalizada`. | Regla local explícita en `decisiones/local_ops_01.md`, anclada al check-out, no al término planificado. |
| Recepción host posterior, sin liquidación temprana | Tipo `recepcion`, `recepcion_conforme` o `recepcion_con_observaciones`; la reserva queda `finalizada`, no se actualiza pago/refund/plazo. | PostgreSQL: recepción tras checkout, replay sin duplicar, listado secuencial y ocupación conservada. | No implementa disputa/resolución financiera. |
| Observación distinta de reclamo formal | Las observaciones quedan en M08. `internal/damageclaim/` persiste el ingreso formal M10 por separado; actor host, antes del plazo desde checkout; arrendatario puede registrar un descargo textual. | Integración: abrir reclamo cambia a `en_disputa`, participante consulta, descargo idempotente, tercero 404. | M10 parcial; CU-40 requiere fotos de descargo, y quedan avisos durables, evidencia adicional, resolución/adjudicación y fondos. Incidencia privacy M02 no se reutiliza. |
| Idempotencia/concurrencia | Idempotencia ligada a actor/clave/payload; lock común serializa estado. Las transiciones/history/inserciones se confirman atómicamente. | Dos check-ins concurrentes con misma clave devuelven mismo ID y un solo evento/state change. M06/contract integration vigente cubre el orden de lock compartido. | La repetición incompatible/conflicto se verifica en pruebas enfocadas listadas abajo. |
| Privacidad y conversación | Se conserva reserva, ocupación, snapshot, mensajes y contrato; actividad `en_curso` y reclamo formal abierto son bloqueadores de baja independientes. `finalizada` sola es un hecho histórico. | Prueba PostgreSQL `TestM02SuppressionReviewListsOnlyLiveReservationAndPaymentObligations`: estado terminal sin reclamo es elegible; reclamo M10 abierto devuelve `disputa_abierta` en ejecución. El recorrido M08 conserva ocupación, evidencia e historial. | No se marca reserva completa por tiempo y no se borra evidencia/conversación automáticamente. |

## Contratos de API y mock

- API: `POST .../check-in`, `POST .../check-out`, `POST .../reception`, `GET .../operations`, descarga privada de evidencia; M10 local tiene `GET/POST .../damage-claim` y `POST .../damage-claim/defense`.
- Clave `Idempotency-Key` obligatoria en acciones; respuestas usan el sobre de error común y `X-Request-ID`, CORS y autenticación Bearer.
- Mock: bandeja existente, tras elegir una reserva se cargan operaciones, y los botones se habilitan según sesión/participante/estado. El API crea imagen y ubicación; el mock muestra aviso sintético y no acepta coordenadas ni archivos del navegador. La salida de observación explica que no es reclamo formal.
- Ensayo manual en navegador: con reserva aprobada y ambos firmantes, seleccionar como arrendatario → firmar completo → registrar check-in el día local de inicio → registrar check-out → observar estado/plazo → entrar como anfitrión, registrar recepción/observación → abrir reclamo dentro de ventana → volver como arrendatario y registrar descargo. Verificar que finalización no libera ocupación, recepción no cambia plazo y el reclamo no crea movimientos de dinero.

## Validación

Comprobaciones de esta rama, todas satisfactorias:

- `go test ./internal/operation/... ./internal/adapters/postgres/operations ./internal/damageclaim/... ./internal/adapters/postgres/damageclaim ./cmd/api -count=1` — PASS; valida paquetes afectados, contrato de errores y router ensamblado.
- `bash scripts/test-m06-local-booking-postgres.sh ./internal/adapters/postgres/booking -run '^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$'` — PASS; migración/fixture en PostgreSQL desechable y rol runtime. Incluye check-in en fecha incorrecta sin efectos, actor/tercero 404, idempotencia concurrente, check-out, recepción sin alterar plazo, evidencia privada, apertura/consulta/descargo de reclamo, exclusión de dinero y vencimiento exactamente al límite mientras espera el bloqueo.
- `GO_TEST_RUN='^TestM02SuppressionReviewListsOnlyLiveReservationAndPaymentObligations$' bash scripts/test-m04-attributes-postgres.sh ./internal/adapters/postgres/identity` — PASS; terminal histórico solo no bloquea y reclamo M10 abierto sí bloquea como `disputa_abierta`.
- `cd mock && npm run test:rental-operations` — PASS (3/3); compila TypeScript y cubre estados/actores del panel, misma clave en reintento, invalidación por sesión/selección y respuestas tardías.
- `python3 -c 'import yaml; yaml.safe_load(open("planning/openapi.yaml")); print("OpenAPI YAML parse: PASS")'` — PASS.
- `git diff --check` — PASS.

### Correcciones y recorrido del PR #219 (2026-10-09)

- La conversación permite enviar a los dos participantes en `en_curso` y `en_disputa`; la verificación Backend conserva idempotencia, 404 a terceros y solo lectura para estados terminales. La integración PostgreSQL del ciclo incluye mensajes de anfitrión y arrendatario después del check-in, reintento con la misma clave, tercero rechazado, nuevo envío rechazado después del check-out y envío permitido durante el reclamo abierto.
- `cd mock && npm run test:rental-operations` — PASS, compila el TypeScript y ejecuta 8 pruebas; incluye el estado de conversación `en_curso`, selección de otra reserva durante la última recarga, logout/relogin durante esa espera y el caso válido cuando solo avanza la revisión interna de la bandeja.
- `GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' bash scripts/test-m06-local-booking-postgres.sh` — PASS en PostgreSQL desechable. `go vet ./internal/conversation/... ./internal/adapters/postgres/conversation` y `git diff --check` — PASS.
- Recorrido verificado por navegador contra API/PostgreSQL local con las dos cuentas sintéticas existentes: firma pendiente completada → check-in (reserva `en_curso`) → mensaje del arrendatario → check-out (`finalizada`, plazo calculado desde la hora persistida) → recepción/observación del anfitrión sin liquidación → reclamo formal sintético del anfitrión (`en_disputa`) → descargo del arrendatario visible para participantes. La conversación dejó de aceptar mensajes al quedar `finalizada` y volvió a ser escribible al quedar abierta la disputa, según el contrato acordado. Evidencia e imágenes fueron sintéticas; no hubo fondos ni decisiones sobre daños.
- El entorno persistente se actualizó sin eliminar `espacigo_pgdata`, secretos ni datos previos. Las pruebas PostgreSQL focalizadas usaron su clúster desechable.

El ciclo interactivo navegador→API se comprobó como parte de la evidencia del PR #219; el resultado anterior que decía que no se afirmaba ese recorrido queda reemplazado por el registro anterior. Las Issues generales de los padres siguen abiertas.

## Pendientes deliberados

- Notificación durable al anfitrión/arrendatario en outbox general (M09); no se declara enviada por API ni Mailpit.
- CU-40: fotos privadas propias del descargo.
- M10: revisión, resolución, adjudicación, cobro/reembolso de garantía, ledger y liquidación.
- Proveedor real, reglas comerciales generales, validez jurídica y política productiva de conservación.
- Las Issues #88–94, #103–110 y padres relacionados permanecen abiertas hasta criterios completos aceptados.
