# Consolidación de criterios originales M06 de reservas

Fecha: 2026-10-06. Criterios revisados: #69, #70, #71, #72, #73, #75 y #77. Este corte consolida solo el recorrido local con fixtures sintéticos y adaptadores fake; no certifica proveedores ni completa M06 general.

## Matriz por criterio original antes de esta entrega

| Issue | Estado | Implementación y evidencia reutilizable | Brecha concreta al inicio / dependencia |
| --- | --- | --- | --- |
| #69 estados por actor, vencimientos, idempotencia, conflictos, transacciones y pagos separados | Parcial | `internal/booking/domain.go`, `internal/adapters/postgres/booking/repository.go`, integración PostgreSQL; PR #149/#152/#157/#161 | Faltaba persistir motivo del rechazo y definir una regla local de cancelación pospago. Webhook, conciliación y proveedor requieren #74/#76/#78. Depende de #68 y #19. |
| #70 modelo: ocupación única, `[inicio,fin)`, snapshots, historial y expiración | Parcial | V9 `ocupacion`, V11–V14 reservas/snapshots/historial, V19 horario; PR #149/#152/#157 | Faltaba un snapshot de política y estado separado para devolución. Contratos y auditoría productiva siguen fuera. Depende de #69/#21. |
| #71 migración incremental, integridad, adyacencia y concurrencia | Parcial | `scripts/test-m06-local-booking-postgres.sh`, integración con `espacigo_runtime`; PR #149/#152/#157 | No había integración del nuevo manejo pospago. La matriz completa MD-01–MD-13 aún exige aceptación por criterio. Depende de #70. |
| #72 repositorio transaccional: estados, ocupación, historial y expiración | Parcial | `repository.go`, `expiry/expire.go`, pruebas PostgreSQL; PR #149/#152/#157/#161/#169 | Se requerían revalidación tras bloqueo para cancelación pospago, coordinación con aprobación, devolución atómica y reintentos. Depende de #71/#22. |
| #73 solicitud, aprobación/rechazo, consulta/historial y cancelación | Parcial | `internal/booking/service.go`, handlers y mock; PR #149/#150 | Rechazo no exigía motivo; cancelación solo cubría pre-pago, mientras CU-51/HU19 requería una regla explícita para pagadas/aprobadas. Depende de #72/#65. |
| #75 API protegida, conflicto 409 y OpenAPI | Parcial | Handler HTTP y `planning/openapi.yaml`; PR #149/#150 | Faltaban contrato de preview/cancelación pospago/devolución y motivo de rechazo. Depende de #73/#23. |
| #77 integración PostgreSQL, concurrencia, rangos, estados, expiración y actor | Parcial | `repository_integration_test.go` y script desechable; PR #149/#152/#157 | Faltaban pruebas deterministas de espera de bloqueo para cancelación y carrera aprobar/cancelar, resultado fake de devolución y rol runtime. Depende de #75/#24. |

## Decisión de producto local

El usuario ratificó durante esta entrega la regla `local_flexible_v1`, exclusivamente para fixtures sintéticos y pagos fake. La especificación completa y sus límites están en [`decisiones_m06_cancelacion_local.md`](decisiones_m06_cancelacion_local.md). En resumen: las reservas pagadas/aprobadas se pueden cancelar por su titular antes del inicio estricto; el fake devuelve 100 % del importe confirmado. Al inicio o después se rechaza. La obligación de devolución es independiente del estado de la reserva, usa identidad estable e idempotente y permite reintentos tras fallo/timeout. Esto no declara completo CU-51/HU19 ni define política comercial.

## Matriz tras implementar el corte

| Issue | Estado | Evidencia de esta entrega | Brecha y dependencia que permanecen |
| --- | --- | --- | --- |
| #69 | Parcial | Motivo obligatorio y persistido al rechazar; reloj leído tras bloqueos para pago, decisión y cancelación; pruebas de lock wait y carrera aprobar/cancelar. | Fake solamente. Webhook firmado/deduplicado, conciliación durable y pasarela dependen de #74/#76/#78. Check-in, curso y disputa aún sin contrato. |
| #70 | Parcial | V20 aditiva añade snapshot `local_flexible_v1`, importe efectivamente pagado, cancelación idempotente y obligación/intentos de devolución. Se conserva `ocupacion` e historial existente. | No cubre contrato/garantía ni modelo productivo de devolución. Depende de #69/#21. |
| #71 | Parcial | Script ejecuta migraciones en Postgres desechable; prueba operación bajo `espacigo_runtime`, grants, lock wait, adyacencia y unicidad de devolución. | No certifica toda MD-01–MD-13 ni el volumen persistente. Depende de #70. |
| #72 | Parcial | Pruebas verifican límites temporales, conflicto sin mutación, liberación, historial, reintentos fallo→timeout→éxito, carrera con aprobación y reintentos concurrentes que convergen al resultado persistido. | Estados de check-in, curso, disputa y fallos de proveedor real pendientes. Depende de #71/#22. |
| #73 | Parcial | Preview y cancelación desde API/mock; pago fake; estado separado de devolución; motivo opcional; avisos Mailpit a ambas partes. | Cancelación al inicio/en curso, check-in, disputa y política comercial real quedan fuera. Depende de #72/#65. |
| #75 | Parcial | OpenAPI, decodificación JSON estricta, tests de conflicto/validación y respuestas para preview, cancelación, devolución y timeout. | No cubre integración de pasarela ni todos los criterios generales del Issue. Depende de #73/#23. |
| #77 | Parcial | Integración PostgreSQL desechable con runtime; límites temporales, reintento, permiso, historial, carrera cancelar/aprobar, éxito/timeout concurrentes, dos éxitos concurrentes y avisos únicos; recorrido navegador/API con fake y Mailpit. | Pruebas locales no sustituyen criterios generales ni proveedor real. Depende de #75/#24. |

## Dependencias y pendientes

- #19/#21/#22/#23/#24 están cerradas y aportan base técnica suficiente.
- Se conserva la cadena abierta #68 → #69 → #70 → #71 → #72 → #73 → #75 → #77 y #65 → #73. No se cierra un padre para desbloquear una hija. El catálogo/cotización de fixtures aceptado permite probar este recorrido local, pero no completa #65 ni #68. La dependencia general de #73 a #65 sigue pendiente de ajuste trazable en Projects.
- #74/#76/#78 siguen abiertas: falta integración real/sandbox contractual, webhooks firmados y deduplicados, manejo durable, conciliación y garantías ante timeout/replay de proveedor.
- #79 no se declara completo: esta evidencia cubre solo el slice local de reservas, no toda su aceptación visual/general.
- No se cierra ninguna Issue original en esta publicación. La aceptación del PR debe preceder cualquier cambio a Hecho.

## Validación y evidencia

- `go test ./...` y `go vet ./...`: ejecutados al finalizar esta entrega; resultados registrados en el PR.
- `bash scripts/test-m06-local-booking-postgres.sh`: ejecutado en PostgreSQL desechable. Las migraciones corrieron con administrador efímero y el flujo de aplicación con `espacigo_runtime`. La base de desarrollo no se usó.
- `npm --prefix mock run build` y `npm --prefix mock run test:profile-races`: ejecutados al finalizar la entrega.
- OpenAPI cargado con PyYAML y referencias de respuesta verificadas; `git diff --check` pasó.
- Evidencia de recorrido, Mailpit, resultados precisos, y límites del entorno: [`evidence/m06-booking-consolidation-20261006.md`](evidence/m06-booking-consolidation-20261006.md).
