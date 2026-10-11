# LOCAL-CLOSE-01A — avisos durables locales

Estado: implementación en `codex/local-close-01`; requiere revisión y merge antes de aceptación. Subentrega trazada en #247, hija de LOCAL-CLOSE-01/#246.

## Productores incluidos en esta rama

V46 añade eventos tipados y productores transaccionales idempotentes en los dueños de cada transición:

| Requisito | Tipo persistido | Productor |
|---|---|---|
| RQF-016 | `cuenta_bloqueada_login` | actualización de `bloqueado_hasta` al registrar el bloqueo por fallos de inicio de sesión. La alerta dejó de depender del envío best-effort de Login. |
| RQF-047/194 | `verificacion_aprobada` / `verificacion_rechazada` | transición de caso sintético terminal. |
| RQF-121/122/201 | `pago_arriendo_confirmado`, `pago_arriendo_rechazado`, `nueva_reserva_pagada` | transición de reserva a `pagada` o `cancelada_por_pago`; se dirige al arrendatario y al anfitrión según corresponda. |
| RQF-134/135/139 | `invitacion_firma`, `contrato_firmado` | alta de cada firma pendiente y transición del contrato a firmado por ambas partes. |
| RQF-210 | `reclamo_resuelto` | ya existía desde V38: una intención por participante insertada con la resolución. Se reutiliza, sin duplicar el productor. |

Los eventos mantienen la política existente de hasta ocho intentos por ciclo, backoff exponencial, fallo terminal, reapertura admin motivada y 30 días desde estado terminal. Mailpit es el único transporte local. SMTP puede duplicar si acepta el mensaje y el proceso cae antes de persistir el resultado; no se promete exactly-once.

Las invitaciones llevan `?contract=<id>` como identificador de recurso, nunca token. Tras iniciar sesión, el mock consulta el contrato y el Backend vuelve a comprobar que la cuenta sea anfitrión o arrendatario antes de mostrarlo. La consulta sin sesión o desde terceros no otorga acceso.

## Evidencia enfocada ejecutada

| Comando | Resultado |
|---|---|
| `npm --prefix mock run build` | PASS; TypeScript compila el flujo de invitación con descarga de contrato autenticada. |
| `go test ./...` | PASS; suite unitaria completa sin `TEST_DATABASE_URL`, sin dependencias sobre la base persistente. |
| `GO_TEST_RUN='^TestAuthLoginFailureBlockResetAndPendingAccountSafety$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/identity` | PASS con PostgreSQL desechable; el quinto fallo crea un solo aviso durable para cuentas verificadas y no verificadas. |
| `GO_TEST_RUN='^TestVerificationRepositoryOwnershipIdempotencyReviewAndRetry$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/verification` | PASS con PostgreSQL desechable y `espacigo_runtime`; aprobación sintética crea un aviso único. |
| `GO_TEST_RUN='^TestSyntheticReviewReciprocityModerationRetentionAndRuntimePermissions$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/reputation` | PASS con PostgreSQL/PostGIS desechable y Mailpit; pago confirmado/rechazado, nueva reserva pagada, y entrega real Mailpit del reporte y bloqueo temporal. |
| `GO_TEST_RUN='^TestLocalBookingTrialPostgresLifecycleAndConcurrentRetry$' bash scripts/test-local-comm-mailpit.sh ./internal/adapters/postgres/booking` | PASS con PostgreSQL desechable; invitaciones a ambos firmantes y aviso de firma completa, sin duplicar en el reintento de firma. |
| `go test ./internal/localnotice` | PASS; cada tipo tiene asunto/cuerpo específico, aviso de ensayo y comprobaciones que excluyen bearer tokens/credenciales del texto. |
| Validación local de referencias OpenAPI y schemas de respuestas de operaciones | PASS; también corrigió dos referencias preexistentes a `ErrorEnvelope` inexistente para usar el schema común `Error`. |

Cada ejecución del wrapper creó y retiró su propio PostgreSQL/PostGIS y Mailpit desechables. No se usó ni modificó `espacigo_pgdata`, secretos o datos persistentes. No se ejecutó GCP.

## Pendientes antes de LOCAL-1

- Esta evidencia cubre los eventos listados, no un outbox general ni entrega exactamente una vez.
- RQF-210 ya está implementado/probado en V38/#223; #247 no lo reimplementa.
- Los mensajes/invitaciones comerciales reales, proveedor de firma y sus credenciales siguen excluidos; no bloquean la invitación Mailpit de ensayo.
- LOCAL-CLOSE-01/#246 sigue abierto hasta completar #248–#250 y ejecutar regresión final QA-03.
